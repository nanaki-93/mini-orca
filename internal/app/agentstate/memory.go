package agentstate

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

var ErrMemoryConflict = errors.New("curated memory changed; reload before editing")

const (
	MaxMemories      = 512
	MaxMemorySources = 64
	MaxMemoryText    = 2048
	maxMemoryCatalog = 8 << 20
)

// A zero line range denotes the whole file. Hashes bind retained advice to the
// source that was explicitly selected when it was created or revised.
type MemorySource struct {
	Path      string `json:"path"`
	Hash      string `json:"hash"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
}

type MemoryProvenance struct {
	Origin       string `json:"origin"`
	SessionID    string `json:"session_id,omitempty"`
	WorkflowID   string `json:"workflow_id,omitempty"`
	ProposalHash string `json:"proposal_hash,omitempty"`
}

type MemoryRecord struct {
	ID              string           `json:"id"`
	Version         int64            `json:"version"`
	ProjectID       string           `json:"project_id"`
	ProjectRevision string           `json:"project_revision"`
	Policy          string           `json:"policy"`
	Kind            string           `json:"kind"`
	State           string           `json:"state"`
	Title           string           `json:"title"`
	Text            string           `json:"text"`
	Sources         []MemorySource   `json:"sources"`
	Provenance      MemoryProvenance `json:"provenance"`
	CreatedAt       time.Time        `json:"created_at"`
	UpdatedAt       time.Time        `json:"updated_at"`
	ExpiresAt       time.Time        `json:"expires_at"`
}

type MemoryVersion struct {
	ID      string
	Version int64
}

type MemoryMatch struct {
	Record MemoryRecord
	Stale  bool
	Score  int
}

type memoryCatalog struct {
	Version   int                     `json:"version"`
	Workspace string                  `json:"workspace"`
	Records   map[string]MemoryRecord `json:"records"`
}

func validMemoryText(value string, limit int, multiline bool) bool {
	if strings.TrimSpace(value) == "" || len(value) > limit || !utf8.ValidString(value) {
		return false
	}
	return strings.IndexFunc(value, func(r rune) bool {
		return unicode.IsControl(r) && !(multiline && (r == '\n' || r == '\t'))
	}) < 0
}

func (source MemorySource) Validate() error {
	if !validMemoryPath(source.Path) {
		return fmt.Errorf("memory source requires a bounded relative path")
	}
	if !strings.HasPrefix(source.Hash, "sha256:") || !validHash(strings.TrimPrefix(source.Hash, "sha256:")) {
		return fmt.Errorf("memory source requires a content hash")
	}
	if source.StartLine < 0 || source.EndLine < source.StartLine || source.EndLine > 1000000 || (source.StartLine == 0) != (source.EndLine == 0) {
		return fmt.Errorf("memory source has an invalid line range")
	}
	return nil
}

func validMemoryPath(value string) bool {
	return value != "" && len(value) <= 512 && utf8.ValidString(value) && !path.IsAbs(value) && path.Clean(value) == value && value != "." && value != ".." && !strings.HasPrefix(value, "../") && !strings.Contains(value, "\\") && strings.IndexFunc(value, unicode.IsControl) < 0
}

func (record MemoryRecord) validate() error {
	for _, value := range []string{record.ID, record.ProjectID, record.ProjectRevision, record.Policy} {
		if !validID(value) {
			return fmt.Errorf("memory identity is invalid")
		}
	}
	if record.Version < 1 || record.Version > 1<<53 || record.CreatedAt.IsZero() || record.UpdatedAt.Before(record.CreatedAt) || !record.ExpiresAt.After(record.UpdatedAt) {
		return fmt.Errorf("memory version or retention is invalid")
	}
	if !slices.Contains([]string{"decision", "lesson", "verified_outcome"}, record.Kind) || !slices.Contains([]string{"candidate", "accepted"}, record.State) {
		return fmt.Errorf("memory kind or state is invalid")
	}
	if !validMemoryText(record.Title, 120, false) || !validMemoryText(record.Text, MaxMemoryText, true) {
		return fmt.Errorf("memory requires a title up to 120 bytes and text up to 2048 bytes")
	}
	if err := validateMemorySources(record.Sources); err != nil {
		return err
	}
	return validateMemoryProvenance(record)
}

func validateMemorySources(sources []MemorySource) error {
	if len(sources) == 0 || len(sources) > MaxMemorySources {
		return fmt.Errorf("memory requires 1-64 captured source scopes")
	}
	seen := make(map[string]bool, len(sources))
	for _, source := range sources {
		if err := source.Validate(); err != nil {
			return err
		}
		if seen[source.Path] {
			return fmt.Errorf("memory repeats a source path")
		}
		seen[source.Path] = true
	}
	return nil
}

func validateMemoryProvenance(record MemoryRecord) error {
	p := record.Provenance
	if p.Origin == "user" && record.Kind != "verified_outcome" && p.SessionID == "" && p.WorkflowID == "" && p.ProposalHash == "" {
		return nil
	}
	if p.Origin != "proposal_checks" && p.Origin != "harness_lesson" {
		return fmt.Errorf("memory provenance is invalid")
	}
	if p.Origin == "proposal_checks" && record.Kind != "verified_outcome" || p.Origin == "harness_lesson" && record.Kind != "lesson" {
		return fmt.Errorf("memory kind does not match its evidence")
	}
	if !validMemoryProposalOrigin(p) {
		return fmt.Errorf("memory proposal provenance is incomplete")
	}
	return nil
}

func validMemoryProposalOrigin(p MemoryProvenance) bool {
	return validID(p.SessionID) && (p.WorkflowID == "" || validID(p.WorkflowID)) && validID(p.ProposalHash) && (p.Origin != "harness_lesson" || p.WorkflowID != "")
}

func (s *Store) loadMemory() (*memoryCatalog, error) {
	location, err := s.path("memory.json")
	if err != nil {
		return nil, err
	}
	data, err := readJournalFile(location)
	if errors.Is(err, os.ErrNotExist) {
		return &memoryCatalog{Version: 1, Workspace: s.workspace, Records: map[string]MemoryRecord{}}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read curated memory: %w", err)
	}
	var catalog memoryCatalog
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if len(data) > maxMemoryCatalog || !validJSON(data) || decoder.Decode(&catalog) != nil || s.validateMemoryCatalog(&catalog) != nil {
		return nil, ErrCorrupt
	}
	return &catalog, nil
}

func (s *Store) validateMemoryCatalog(catalog *memoryCatalog) error {
	if catalog.Version != 1 || catalog.Workspace != s.workspace || catalog.Records == nil || len(catalog.Records) > MaxMemories {
		return ErrCorrupt
	}
	for id, record := range catalog.Records {
		if id != record.ID || record.validate() != nil {
			return ErrCorrupt
		}
	}
	return nil
}

// Memories reads private records without creating files, pruning or accepting.
func (s *Store) Memories(ctx context.Context, projectID string) ([]MemoryRecord, error) {
	if err := context.Cause(ctx); err != nil {
		return nil, err
	}
	if !validID(projectID) {
		return nil, fmt.Errorf("memory project identity is required")
	}
	catalog, err := s.loadMemory()
	if err != nil {
		return nil, err
	}
	result := []MemoryRecord{}
	for _, record := range catalog.Records {
		if record.ProjectID == projectID {
			result = append(result, record)
		}
	}
	slices.SortFunc(result, func(a, b MemoryRecord) int {
		if order := b.UpdatedAt.Compare(a.UpdatedAt); order != 0 {
			return order
		}
		return strings.Compare(a.ID, b.ID)
	})
	return result, context.Cause(ctx)
}

func (s *Store) mutateMemory(ctx context.Context, authorize func() error, change func(*memoryCatalog, time.Time) error) (err error) {
	if err := authorizeMemoryWrite(ctx, authorize); err != nil {
		return err
	}
	parent := ctx
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	lock, err := s.lock(ctx)
	if err != nil {
		return persistenceOperationError(parent, "lock curated memory", err)
	}
	defer func() {
		if closeErr := lock.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("%w: release curated memory lock: %w", ErrPersistence, closeErr))
		}
	}()
	catalog, err := s.loadMemory()
	if err != nil {
		return err
	}
	if err := authorizeMemoryWrite(ctx, authorize); err != nil {
		return err
	}
	if err := change(catalog, time.Now().UTC()); err != nil {
		return err
	}
	if err := s.validateMemoryCatalog(catalog); err != nil {
		return err
	}
	data, err := json.Marshal(catalog)
	if err != nil {
		return fmt.Errorf("encode curated memory: %w", err)
	}
	if len(data) > maxMemoryCatalog {
		return fmt.Errorf("curated memory exceeds its 8 MiB limit")
	}
	location, err := s.path("memory.json")
	if err != nil {
		return err
	}
	if err := s.write(location, data, 0600, func() error { return authorizeMemoryWrite(ctx, authorize) }); err != nil {
		return persistenceOperationError(parent, "save curated memory", err)
	}
	return nil
}

func authorizeMemoryWrite(ctx context.Context, authorize func() error) error {
	if err := context.Cause(ctx); err != nil {
		return err
	}
	if authorize == nil {
		return fmt.Errorf("curated memory requires publication authority")
	}
	return authorize()
}

// SaveMemory creates or edits a candidate. Accepted content cannot be changed
// silently: every edit requires a fresh, separate acceptance of its new version.
func (s *Store) SaveMemory(ctx context.Context, record MemoryRecord, expectedVersion int64, authorize func() error) (MemoryRecord, error) {
	record.Sources = slices.Clone(record.Sources)
	err := s.mutateMemory(ctx, authorize, func(catalog *memoryCatalog, now time.Time) error {
		previous, exists := catalog.Records[record.ID]
		if exists && (previous.ProjectID != record.ProjectID || previous.Version != expectedVersion) || !exists && expectedVersion != 0 {
			return ErrMemoryConflict
		}
		record.Version, record.State = expectedVersion+1, "candidate"
		record.CreatedAt, record.UpdatedAt = now, now
		if exists {
			record.CreatedAt = previous.CreatedAt
		}
		if !record.ExpiresAt.After(now) || record.ExpiresAt.After(now.Add(365*24*time.Hour)) {
			return fmt.Errorf("memory retention must end within the next 365 days")
		}
		if err := record.validate(); err != nil {
			return err
		}
		pruneExpiredMemory(catalog, record.ProjectID, now, record.ID)
		if !exists && len(catalog.Records) >= MaxMemories {
			return fmt.Errorf("curated memory has reached its 512-record limit")
		}
		catalog.Records[record.ID] = record
		return nil
	})
	return record, err
}

func (s *Store) AcceptMemory(ctx context.Context, projectID, id string, expectedVersion int64, authorize func() error) (result MemoryRecord, err error) {
	err = s.mutateMemory(ctx, authorize, func(catalog *memoryCatalog, now time.Time) error {
		value, err := currentMemory(catalog, projectID, id, expectedVersion)
		if err != nil {
			return err
		}
		if value.State != "candidate" || !value.ExpiresAt.After(now) {
			return fmt.Errorf("only an unexpired candidate can be accepted")
		}
		value.State, value.Version, value.UpdatedAt = "accepted", value.Version+1, now
		catalog.Records[id], result = value, value
		pruneExpiredMemory(catalog, projectID, now, id)
		return nil
	})
	return result, err
}

func (s *Store) ForgetMemory(ctx context.Context, projectID, id string, expectedVersion int64, authorize func() error) error {
	return s.mutateMemory(ctx, authorize, func(catalog *memoryCatalog, now time.Time) error {
		if _, err := currentMemory(catalog, projectID, id, expectedVersion); err != nil {
			return err
		}
		delete(catalog.Records, id)
		pruneExpiredMemory(catalog, projectID, now, "")
		return nil
	})
}

func currentMemory(catalog *memoryCatalog, projectID, id string, expectedVersion int64) (MemoryRecord, error) {
	record, found := catalog.Records[id]
	if !found || record.ProjectID != projectID {
		return MemoryRecord{}, ErrNotFound
	}
	if record.Version != expectedVersion {
		return MemoryRecord{}, ErrMemoryConflict
	}
	return record, nil
}

func pruneExpiredMemory(catalog *memoryCatalog, projectID string, now time.Time, keep string) {
	for id, record := range catalog.Records {
		if id != keep && record.ProjectID == projectID && !record.ExpiresAt.After(now) {
			delete(catalog.Records, id)
		}
	}
}

// SearchMemory ranks only host-admitted records. The caller must build the
// source scopes from current policy and actual source hashes, never model input.
func (s *Store) SearchMemory(ctx context.Context, projectID, revision, policy, query string, sources []MemorySource, now time.Time) ([]MemoryMatch, error) {
	terms, err := memorySearchTerms(query)
	if err != nil {
		return nil, err
	}
	records, err := s.Memories(ctx, projectID)
	if err != nil {
		return nil, err
	}
	allowed := make(map[string]MemorySource, len(sources))
	for _, source := range sources {
		if err := source.Validate(); err != nil {
			return nil, err
		}
		if _, duplicate := allowed[source.Path]; duplicate {
			return nil, fmt.Errorf("memory admission repeats a path")
		}
		allowed[source.Path] = source
	}
	matches := []MemoryMatch{}
	for _, record := range records {
		if !memoryEligible(record, policy, allowed, now) {
			continue
		}
		score := memorySearchScore(record, terms)
		if score > 0 {
			matches = append(matches, MemoryMatch{Record: record, Stale: record.ProjectRevision != revision, Score: score})
		}
	}
	slices.SortStableFunc(matches, func(a, b MemoryMatch) int { return cmp.Compare(b.Score, a.Score) })
	return matches[:min(len(matches), 8)], context.Cause(ctx)
}

func memoryEligible(record MemoryRecord, policy string, sources map[string]MemorySource, now time.Time) bool {
	if record.State != "accepted" || !record.ExpiresAt.After(now) || record.Policy != policy {
		return false
	}
	for _, required := range record.Sources {
		allowed, found := sources[required.Path]
		if !found || allowed.Hash != required.Hash || allowed.StartLine != 0 && (required.StartLine < allowed.StartLine || required.EndLine > allowed.EndLine) {
			return false
		}
	}
	return true
}

func memorySearchTerms(query string) ([]string, error) {
	if len(query) > 512 || !utf8.ValidString(query) {
		return nil, fmt.Errorf("memory query must be at most 512 UTF-8 bytes")
	}
	terms := strings.FieldsFunc(strings.ToLower(query), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
	terms = slices.Compact(slices.Sorted(slices.Values(terms)))
	if len(terms) > 64 {
		return nil, fmt.Errorf("memory query has more than 64 terms")
	}
	return terms, nil
}

func memorySearchScore(record MemoryRecord, terms []string) int {
	text := strings.ToLower(record.Title + " " + record.Text)
	score := 0
	for _, term := range terms {
		if len(term) >= 2 && strings.Contains(text, term) {
			score++
		}
	}
	return score
}
