package agentstate

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/config"
)

var ErrMissionConflict = errors.New("mission catalog changed; reload before editing or reviewing")

const (
	MaxMissions        = 128
	MaxMissionChildren = 16
	MaxMissionSources  = 64
	maxMissionCatalog  = 8 << 20
)

// MissionLimits requires explicit values, including zero repairs. It cannot
// silently inherit a larger operator budget when a field is missing.
type MissionLimits config.AgentLimits

func (limits *MissionLimits) UnmarshalJSON(data []byte) error {
	type plain MissionLimits
	var value plain
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if !validJSON(data) || decoder.Decode(&value) != nil {
		return fmt.Errorf("invalid mission limits")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	for _, name := range []string{"model_calls", "tool_calls", "input_tokens", "output_tokens", "elapsed_seconds", "concurrent_calls", "repairs"} {
		if raw, ok := fields[name]; !ok || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return fmt.Errorf("mission limits require every budget value")
		}
	}
	*limits = MissionLimits(value)
	return config.AgentLimits(*limits).Validate()
}

func (limits MissionLimits) Within(ceiling config.AgentLimits) error {
	if err := config.AgentLimits(limits).Validate(); err != nil {
		return err
	}
	for _, pair := range [][2]int64{
		{int64(limits.ModelCalls), int64(ceiling.ModelCalls)}, {int64(limits.ToolCalls), int64(ceiling.ToolCalls)},
		{limits.InputTokens, ceiling.InputTokens}, {limits.OutputTokens, ceiling.OutputTokens},
		{int64(limits.ElapsedSeconds), int64(ceiling.ElapsedSeconds)}, {int64(limits.ConcurrentCalls), int64(ceiling.ConcurrentCalls)},
		{int64(limits.Repairs), int64(ceiling.Repairs)},
	} {
		if pair[0] > pair[1] {
			return fmt.Errorf("mission limits exceed the available budget")
		}
	}
	return nil
}

type MissionSource struct {
	Path   string `json:"path"`
	Hash   string `json:"hash"`
	Exists bool   `json:"exists"`
}

type MissionChild struct {
	ID                 string        `json:"id"`
	Title              string        `json:"title"`
	Approach           string        `json:"approach"`
	AcceptanceCriteria []string      `json:"acceptance_criteria"`
	Dependencies       []string      `json:"dependencies"`
	WritePaths         []string      `json:"write_paths"`
	ReadPaths          []string      `json:"read_paths"`
	Limits             MissionLimits `json:"limits"`
}

type MissionPlan struct {
	Goal             string          `json:"goal"`
	Sources          []MissionSource `json:"sources"`
	Limits           MissionLimits   `json:"limits"`
	Children         []MissionChild  `json:"children"`
	IntegrationOrder []string        `json:"integration_order"`
}

// Review refers to this complete plan hash. It never grants execution or
// proposal Apply authority; subsequent content edits clear every review.
type MissionRecord struct {
	ID              string            `json:"id"`
	ProjectID       string            `json:"project_id"`
	ProjectRevision string            `json:"project_revision"`
	Policy          string            `json:"policy"`
	Configuration   string            `json:"configuration"`
	Instructions    map[string]string `json:"instructions"`
	Plan            MissionPlan       `json:"plan"`
	PlanHash        string            `json:"plan_hash"`
	Reviewed        []string          `json:"reviewed"`
	Approved        bool              `json:"approved"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

type MissionSnapshot struct {
	CatalogRevision int64         `json:"catalog_revision"`
	Mission         MissionRecord `json:"mission"`
}

type MissionCatalog struct {
	Revision int64           `json:"catalog_revision"`
	Missions []MissionRecord `json:"missions"`
}

type missionCatalog struct {
	Version   int                      `json:"version"`
	Workspace string                   `json:"workspace"`
	Revision  int64                    `json:"revision"`
	Records   map[string]MissionRecord `json:"records"`
}

func (plan MissionPlan) Validate() error {
	if !validMemoryText(plan.Goal, 4096, true) || len(plan.Children) == 0 || len(plan.Children) > MaxMissionChildren {
		return fmt.Errorf("mission requires a goal up to 4096 bytes and 1–16 children")
	}
	if err := config.AgentLimits(plan.Limits).Validate(); err != nil {
		return err
	}
	sources, err := missionSources(plan.Sources)
	if err != nil {
		return err
	}
	children := make(map[string]MissionChild, len(plan.Children))
	for _, child := range plan.Children {
		if _, exists := children[child.ID]; exists {
			return fmt.Errorf("duplicate mission child")
		}
		if err := validateMissionChild(child, sources, plan.Limits); err != nil {
			return err
		}
		children[child.ID] = child
	}
	ancestors, err := missionAncestors(plan.IntegrationOrder, children)
	if err != nil {
		return err
	}
	if err := validateMissionAccess(plan.Children, sources, ancestors); err != nil {
		return err
	}
	return validateMissionTotal(plan)
}

func missionSources(sources []MissionSource) (map[string]MissionSource, error) {
	if len(sources) == 0 || len(sources) > MaxMissionSources {
		return nil, fmt.Errorf("mission requires 1–64 captured paths")
	}
	result := make(map[string]MissionSource, len(sources))
	for _, source := range sources {
		if !validMemoryPath(source.Path) || source.Exists != (source.Hash != "") {
			return nil, fmt.Errorf("invalid mission source identity")
		}
		if source.Exists && (MemorySource{Path: source.Path, Hash: source.Hash}).Validate() != nil {
			return nil, fmt.Errorf("mission source requires a content hash")
		}
		for path := range result {
			if path == source.Path || strings.HasPrefix(path, source.Path+"/") || strings.HasPrefix(source.Path, path+"/") {
				return nil, fmt.Errorf("mission paths must be distinct files")
			}
		}
		result[source.Path] = source
	}
	return result, nil
}

func validateMissionChild(child MissionChild, sources map[string]MissionSource, ceiling MissionLimits) error {
	if !validMissionChildID(child.ID) || !validMemoryText(child.Title, 120, false) || !validMemoryText(child.Approach, 4096, true) {
		return fmt.Errorf("mission child requires an ID, title and bounded approach")
	}
	if len(child.AcceptanceCriteria) == 0 || len(child.AcceptanceCriteria) > 16 || len(child.Dependencies) >= MaxMissionChildren {
		return fmt.Errorf("mission child requires 1–16 criteria and at most 15 dependencies")
	}
	for _, criterion := range child.AcceptanceCriteria {
		if !validMemoryText(criterion, 512, true) {
			return fmt.Errorf("mission criteria must be nonempty and at most 512 bytes")
		}
	}
	if err := validateMissionPaths(child, sources); err != nil {
		return err
	}
	return child.Limits.Within(config.AgentLimits(ceiling))
}

func validMissionChildID(id string) bool {
	return len(id) > 0 && len(id) <= 64 && strings.IndexFunc(id, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-')
	}) < 0
}

func validateMissionPaths(child MissionChild, sources map[string]MissionSource) error {
	if len(child.WritePaths) == 0 || len(child.WritePaths) > 8 || len(child.ReadPaths) > 8 {
		return fmt.Errorf("mission child requires 1–8 writes and at most eight reads")
	}
	seen := map[string]bool{}
	for _, path := range append(slices.Clone(child.WritePaths), child.ReadPaths...) {
		if _, exists := sources[path]; !exists || seen[path] {
			return fmt.Errorf("child paths must be distinct captured sources")
		}
		seen[path] = true
	}
	return nil
}

// Requiring a complete topological order also rejects cycles and missing edges.
func missionAncestors(order []string, children map[string]MissionChild) (map[string]map[string]bool, error) {
	if len(order) != len(children) {
		return nil, fmt.Errorf("integration order must include every child exactly once")
	}
	ancestors := map[string]map[string]bool{}
	for _, id := range order {
		child, exists := children[id]
		if !exists || ancestors[id] != nil {
			return nil, fmt.Errorf("invalid integration order")
		}
		parents := map[string]bool{}
		seen := map[string]bool{}
		for _, dependency := range child.Dependencies {
			if seen[dependency] || ancestors[dependency] == nil {
				return nil, fmt.Errorf("dependencies must be distinct earlier children in the integration order")
			}
			seen[dependency], parents[dependency] = true, true
			for ancestor := range ancestors[dependency] {
				parents[ancestor] = true
			}
		}
		ancestors[id] = parents
	}
	return ancestors, nil
}

func validateMissionAccess(children []MissionChild, sources map[string]MissionSource, ancestors map[string]map[string]bool) error {
	for i, child := range children {
		for _, other := range children[:i] {
			ordered := ancestors[child.ID][other.ID] || ancestors[other.ID][child.ID]
			if !ordered && missionAccessConflict(child, other) {
				return fmt.Errorf("children %s and %s need a dependency for shared write/read access", child.ID, other.ID)
			}
		}
		for _, path := range child.ReadPaths {
			created := slices.ContainsFunc(children, func(parent MissionChild) bool {
				return ancestors[child.ID][parent.ID] && slices.Contains(parent.WritePaths, path)
			})
			if !sources[path].Exists && !created {
				return fmt.Errorf("absent read path requires an ancestor creator")
			}
		}
	}
	return nil
}

func missionAccessConflict(a, b MissionChild) bool {
	for _, path := range a.WritePaths {
		if slices.Contains(b.WritePaths, path) || slices.Contains(b.ReadPaths, path) {
			return true
		}
	}
	for _, path := range a.ReadPaths {
		if slices.Contains(b.WritePaths, path) {
			return true
		}
	}
	return false
}

func validateMissionTotal(plan MissionPlan) error {
	total := MissionLimits{ElapsedSeconds: 1, ConcurrentCalls: 1}
	for _, child := range plan.Children {
		total.ModelCalls += child.Limits.ModelCalls
		total.ToolCalls += child.Limits.ToolCalls
		total.InputTokens += child.Limits.InputTokens
		total.OutputTokens += child.Limits.OutputTokens
		total.Repairs += child.Limits.Repairs
	}
	if err := total.Within(config.AgentLimits(plan.Limits)); err != nil {
		return fmt.Errorf("cumulative child budgets: %w", err)
	}
	return nil
}

func missionPlanHash(record MissionRecord) (string, error) {
	// Identity and admission bindings are part of the reviewed document. Review
	// state and timestamps are deliberately outside its content identity.
	record.PlanHash, record.Reviewed, record.Approved = "", nil, false
	record.CreatedAt, record.UpdatedAt = time.Time{}, time.Time{}
	data, err := json.Marshal(record)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
}

func (record MissionRecord) validate() error {
	for _, id := range []string{record.ID, record.ProjectID, record.ProjectRevision, record.Policy, record.Configuration} {
		if !validID(id) {
			return fmt.Errorf("invalid mission binding")
		}
	}
	if err := record.Plan.Validate(); err != nil {
		return err
	}
	if len(record.Instructions) != len(record.Plan.Sources) {
		return fmt.Errorf("mission requires instruction identities for every source")
	}
	for _, source := range record.Plan.Sources {
		if (MemorySource{Path: source.Path, Hash: record.Instructions[source.Path]}).Validate() != nil {
			return fmt.Errorf("invalid mission instruction identity")
		}
	}
	hash, err := missionPlanHash(record)
	if err != nil || record.PlanHash != hash || record.CreatedAt.IsZero() || record.UpdatedAt.Before(record.CreatedAt) {
		return fmt.Errorf("invalid mission content identity or timestamps")
	}
	return validateMissionReview(record)
}

func validateMissionReview(record MissionRecord) error {
	seen := map[string]bool{}
	for _, id := range record.Reviewed {
		if seen[id] || !slices.ContainsFunc(record.Plan.Children, func(child MissionChild) bool { return child.ID == id }) {
			return fmt.Errorf("invalid mission child review")
		}
		seen[id] = true
	}
	if record.Approved && len(seen) != len(record.Plan.Children) {
		return fmt.Errorf("mission approval requires review of every child")
	}
	return nil
}

func (s *Store) loadMissions() (*missionCatalog, error) {
	location, err := s.path("missions.json")
	if err != nil {
		return nil, fmt.Errorf("%w: locate missions: %w", ErrPersistence, err)
	}
	data, err := readJournalFile(location)
	if errors.Is(err, os.ErrNotExist) {
		return &missionCatalog{Version: 1, Workspace: s.workspace, Records: map[string]MissionRecord{}}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%w: read missions: %w", ErrPersistence, err)
	}
	var catalog missionCatalog
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if len(data) > maxMissionCatalog || !validJSON(data) || decoder.Decode(&catalog) != nil || s.validateMissionCatalog(&catalog) != nil {
		return nil, ErrCorrupt
	}
	return &catalog, nil
}

func (s *Store) validateMissionCatalog(catalog *missionCatalog) error {
	if catalog.Version != 1 || catalog.Workspace != s.workspace || catalog.Revision < int64(len(catalog.Records)) || catalog.Revision > 1<<53 || catalog.Records == nil || len(catalog.Records) > MaxMissions {
		return ErrCorrupt
	}
	for id, record := range catalog.Records {
		if id != record.ID || record.validate() != nil {
			return ErrCorrupt
		}
	}
	return nil
}

func (s *Store) Missions(ctx context.Context, projectID string) (MissionCatalog, error) {
	result := MissionCatalog{Missions: []MissionRecord{}}
	if err := context.Cause(ctx); err != nil {
		return result, err
	}
	if !validID(projectID) {
		return result, fmt.Errorf("mission project identity is required")
	}
	catalog, err := s.loadMissions()
	if err != nil {
		return result, err
	}
	result.Revision = catalog.Revision
	for _, record := range catalog.Records {
		if record.ProjectID == projectID {
			result.Missions = append(result.Missions, record)
		}
	}
	slices.SortFunc(result.Missions, func(a, b MissionRecord) int {
		if order := b.UpdatedAt.Compare(a.UpdatedAt); order != 0 {
			return order
		}
		return strings.Compare(a.ID, b.ID)
	})
	return result, context.Cause(ctx)
}

func authorizeMissionWrite(ctx context.Context, authorize func() error) error {
	if err := context.Cause(ctx); err != nil {
		return err
	}
	if authorize == nil {
		return fmt.Errorf("mission mutation requires publication authority")
	}
	if err := authorize(); err != nil {
		return err
	}
	// Source/settings checks can outlast a deadline or observe cancellation.
	// Recheck after them, before the atomic rename publishes the decision.
	return context.Cause(ctx)
}

func (s *Store) mutateMissions(ctx context.Context, revision int64, authorize func() error, change func(*missionCatalog, time.Time) error) (err error) {
	if err := authorizeMissionWrite(ctx, authorize); err != nil {
		return err
	}
	parent := ctx
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	lock, err := s.lock(ctx)
	if err != nil {
		return persistenceOperationError(parent, "lock missions", err)
	}
	defer func() {
		if closeErr := lock.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("%w: release mission lock: %w", ErrPersistence, closeErr))
		}
	}()
	catalog, err := s.loadMissions()
	if err != nil {
		return err
	}
	if catalog.Revision != revision {
		return ErrMissionConflict
	}
	if err := authorizeMissionWrite(ctx, authorize); err != nil {
		return err
	}
	if err := change(catalog, time.Now().UTC()); err != nil {
		return err
	}
	catalog.Revision++
	if err := s.validateMissionCatalog(catalog); err != nil {
		return err
	}
	data, err := json.Marshal(catalog)
	if err != nil {
		return fmt.Errorf("encode missions: %w", err)
	}
	if len(data) > maxMissionCatalog {
		return fmt.Errorf("mission catalog exceeds its 8 MiB limit")
	}
	location, err := s.path("missions.json")
	if err != nil {
		return err
	}
	if err := s.write(location, data, 0600, func() error { return authorizeMissionWrite(ctx, authorize) }); err != nil {
		return persistenceOperationError(parent, "save missions", err)
	}
	return nil
}

func (s *Store) SaveMission(ctx context.Context, record MissionRecord, revision int64, previousHash string, authorize func() error) (MissionSnapshot, error) {
	data, err := json.Marshal(record)
	if err != nil {
		return MissionSnapshot{}, err
	}
	if len(data) > 256<<10 {
		return MissionSnapshot{}, fmt.Errorf("mission exceeds 256 KiB")
	}
	// Own nested slices/maps throughout publication, including caller callbacks.
	var owned MissionRecord
	if err := json.Unmarshal(data, &owned); err != nil {
		return MissionSnapshot{}, err
	}
	record = owned
	err = s.mutateMissions(ctx, revision, authorize, func(catalog *missionCatalog, now time.Time) error {
		previous, exists := catalog.Records[record.ID]
		if exists && (previous.ProjectID != record.ProjectID || previous.PlanHash != previousHash) || !exists && previousHash != "" {
			return ErrMissionConflict
		}
		if !exists && len(catalog.Records) >= MaxMissions {
			return fmt.Errorf("mission catalog has reached its 128-record limit")
		}
		record.Reviewed, record.Approved = []string{}, false
		record.CreatedAt, record.UpdatedAt = now, now
		if exists {
			record.CreatedAt = previous.CreatedAt
		}
		hash, err := missionPlanHash(record)
		if err != nil {
			return err
		}
		record.PlanHash = hash
		if err := record.validate(); err != nil {
			return err
		}
		catalog.Records[record.ID] = record
		return nil
	})
	if err != nil {
		return MissionSnapshot{}, err
	}
	return MissionSnapshot{CatalogRevision: revision + 1, Mission: record}, nil
}

func (s *Store) ReviewMission(ctx context.Context, projectID, id string, revision int64, hash, childID string, authorize func() error) (result MissionSnapshot, err error) {
	err = s.mutateMissions(ctx, revision, authorize, func(catalog *missionCatalog, now time.Time) error {
		record, exists := catalog.Records[id]
		if !exists || record.ProjectID != projectID {
			return ErrNotFound
		}
		if record.PlanHash != hash {
			return ErrMissionConflict
		}
		if err := reviewMissionChild(&record, childID); err != nil {
			return err
		}
		record.UpdatedAt = now
		catalog.Records[id] = record
		result = MissionSnapshot{CatalogRevision: revision + 1, Mission: record}
		return nil
	})
	if err != nil {
		return MissionSnapshot{}, err
	}
	return result, nil
}

func reviewMissionChild(record *MissionRecord, childID string) error {
	if record.Approved {
		return fmt.Errorf("mission is already approved; revise it before another review")
	}
	if childID == "" {
		if len(record.Reviewed) != len(record.Plan.Children) {
			return fmt.Errorf("review every child before approving the mission")
		}
		record.Approved = true
		return nil
	}
	if !slices.ContainsFunc(record.Plan.Children, func(child MissionChild) bool { return child.ID == childID }) {
		return fmt.Errorf("unknown mission child")
	}
	if slices.Contains(record.Reviewed, childID) {
		return fmt.Errorf("child already reviewed")
	}
	record.Reviewed = append(record.Reviewed, childID)
	return nil
}
