package features

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/nanaki-93/mini-orca/v2/internal/app/checkexec"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
	"github.com/nanaki-93/mini-orca/v2/internal/storage"
)

const (
	LegacySchemaVersion       = 1
	HistorySchemaVersion      = 2
	MaxHistoryBytes           = 64 * 1024
	MaxExistingIdeasBytes     = 8 * 1024
	MaxSuggestionsPerResponse = 5
	LegacyGenerationID        = "legacy"
)

// ErrHistoryFull keeps the saved report unchanged instead of evicting ideas.
var ErrHistoryFull = errors.New("feature history is full; existing ideas were kept")

var ErrUnsupportedHistory = errors.New("unsupported feature history")

// FeatureModelSummary names the model that produced a generation without endpoints or credentials.
type FeatureModelSummary struct {
	Profile        string `json:"profile"`
	Model          string `json:"model"`
	ProviderOrigin string `json:"provider_origin"`
	RemoteProvider bool   `json:"remote_provider"`
}

// FeatureGeneration is the provenance shared by ideas admitted from one successful response.
type FeatureGeneration struct {
	ID              string               `json:"id"`
	GeneratedAt     time.Time            `json:"generated_at"`
	ProjectRevision string               `json:"project_revision"`
	WorkspaceHash   string               `json:"workspace_hash"`
	GoalsHash       string               `json:"goals_hash"`
	Model           *FeatureModelSummary `json:"model"`
}

// FeatureGenerationOutcome counts what the most recent successful generation contributed.
type FeatureGenerationOutcome struct {
	GenerationID   string `json:"generation_id"`
	AddedCount     int    `json:"added_count"`
	DuplicateCount int    `json:"duplicate_count"`
}

// MetadataPath disallows metadata-directory aliases before either reads
// or writes. Source-write path policy deliberately does not admit metadata.
func MetadataPath(root, name string) (string, error) {
	current := root
	for _, part := range []string{".mini-orca", "changes", name} {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("change history contains a symlink")
		}
	}
	return current, nil
}

func ContentHash(content []byte) string {
	sum := sha256.Sum256(content)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func EmptyReport(index *project.ProjectIndex) *FeatureReport {
	return &FeatureReport{
		SchemaVersion:   HistorySchemaVersion,
		ProjectID:       index.ProjectID,
		ProjectRevision: index.ProjectRevision,
		Hash:            "empty",
		Status:          "not_generated",
		Freshness:       "current",
		Suggestions:     []FeatureSuggestion{},
		Generations:     []FeatureGeneration{},
	}
}

func Read(root string, index *project.ProjectIndex) (*FeatureReport, error) {
	path, err := MetadataPath(root, "features.json")
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return EmptyReport(index), nil
	}
	if err != nil {
		return nil, err
	}
	if len(data) > MaxHistoryBytes {
		return nil, fmt.Errorf("feature history exceeds 64 KiB")
	}
	var report FeatureReport
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&report); err != nil {
		return nil, fmt.Errorf("read feature history: %w", err)
	}
	if decoder.Decode(new(any)) != io.EOF || report.ProjectID != index.ProjectID {
		return nil, ErrUnsupportedHistory
	}
	if err := upgradeFeatureHistory(&report); err != nil {
		return nil, err
	}
	return &report, nil
}

// upgradeFeatureHistory converts v1 in memory and keeps its stored hash, so
// expected-hash checks still match the unchanged file until the next write.
func upgradeFeatureHistory(report *FeatureReport) error {
	if report.Suggestions == nil {
		report.Suggestions = []FeatureSuggestion{}
	}
	switch report.SchemaVersion {
	case LegacySchemaVersion:
		if err := convertLegacyFeatureHistory(report); err != nil {
			return err
		}
	case HistorySchemaVersion:
		if report.Generations == nil {
			return ErrUnsupportedHistory
		}
	default:
		return ErrUnsupportedHistory
	}
	return validateFeatureReferences(report)
}

// Legacy ideas share one generation built from the v1 report context. Goals are
// trusted only when v1 still marked them current; otherwise the ideas stay stale.
func convertLegacyFeatureHistory(report *FeatureReport) error {
	if report.Generations != nil || report.LastGeneration != nil {
		return ErrUnsupportedHistory
	}
	report.Generations = []FeatureGeneration{}
	if len(report.Suggestions) == 0 {
		return nil
	}
	legacy := FeatureGeneration{ID: LegacyGenerationID, GeneratedAt: report.UpdatedAt, ProjectRevision: report.ProjectRevision, WorkspaceHash: report.WorkspaceHash}
	if report.Freshness == "current" {
		legacy.GoalsHash = ContentHash([]byte(report.Goals))
	}
	for i := range report.Suggestions {
		if report.Suggestions[i].GenerationID != "" {
			return ErrUnsupportedHistory
		}
		report.Suggestions[i].GenerationID = legacy.ID
	}
	report.Generations = append(report.Generations, legacy)
	return nil
}

func validateFeatureReferences(report *FeatureReport) error {
	generations := make(map[string]bool, len(report.Generations))
	for _, generation := range report.Generations {
		if generation.ID == "" || len(generation.ID) > 64 || generations[generation.ID] {
			return ErrUnsupportedHistory
		}
		generations[generation.ID] = true
	}
	ideas := make(map[string]bool, len(report.Suggestions))
	for i := range report.Suggestions {
		idea := &report.Suggestions[i]
		if idea.ID == "" || ideas[idea.ID] || !generations[idea.GenerationID] {
			return ErrUnsupportedHistory
		}
		ideas[idea.ID] = true
		// Freshness is derived for each response and never trusted from storage.
		idea.Freshness = ""
	}
	if last := report.LastGeneration; last != nil && (!generations[last.GenerationID] || !validFeatureOutcomeCounts(*last)) {
		return ErrUnsupportedHistory
	}
	return nil
}

func validFeatureOutcomeCounts(outcome FeatureGenerationOutcome) bool {
	return outcome.AddedCount >= 0 && outcome.DuplicateCount >= 0 && outcome.AddedCount+outcome.DuplicateCount <= MaxSuggestionsPerResponse
}

// Write persists schema v2 without derived freshness. A report that
// would exceed the history bound is rejected before any write.
func Write(root string, report *FeatureReport) error {
	path, err := MetadataPath(root, "features.json")
	if err != nil {
		return err
	}
	report.SchemaVersion = HistorySchemaVersion
	report.Generations = referencedFeatureGenerations(report)
	for i := range report.Suggestions {
		report.Suggestions[i].Freshness = ""
	}
	report.UpdatedAt = time.Now().UTC()
	report.Hash = ""
	data, err := json.Marshal(report)
	if err != nil {
		return err
	}
	report.Hash = ContentHash(data)
	data, err = json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if len(data) > MaxHistoryBytes {
		return fmt.Errorf("%w: the saved report would exceed 64 KiB", ErrHistoryFull)
	}
	return storage.WriteFile(path, data, 0600)
}

func Clear(root string, index *project.ProjectIndex) (*FeatureReport, error) {
	report, err := Read(root, index)
	if err != nil {
		return nil, err
	}
	if report.Goals == "" {
		path, err := MetadataPath(root, "features.json")
		if err != nil {
			return nil, err
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		return EmptyReport(index), nil
	}
	report.ProjectRevision = index.ProjectRevision
	report.Status = "not_generated"
	report.Freshness = "current"
	report.Failure = ""
	report.Suggestions = []FeatureSuggestion{}
	report.Generations = []FeatureGeneration{}
	report.LastGeneration = nil
	report.WorkspaceHash = ""
	report.ContextManifest = project.ContextManifest{}
	if err := Write(root, report); err != nil {
		return nil, err
	}
	return report, nil
}

func referencedFeatureGenerations(report *FeatureReport) []FeatureGeneration {
	referenced := map[string]bool{}
	for _, idea := range report.Suggestions {
		referenced[idea.GenerationID] = true
	}
	if report.LastGeneration != nil {
		referenced[report.LastGeneration.GenerationID] = true
	}
	kept := []FeatureGeneration{}
	for _, generation := range report.Generations {
		if referenced[generation.ID] {
			kept = append(kept, generation)
		}
	}
	return kept
}

// DeriveFreshness marks an idea current only when its own generation
// matches the current project revision, workspace and saved goals. Report-level
// freshness keeps describing only the most recent generation context.
func DeriveFreshness(report *FeatureReport, projectRevision, workspaceHash string) {
	goalsHash := ContentHash([]byte(report.Goals))
	current := make(map[string]bool, len(report.Generations))
	for _, generation := range report.Generations {
		current[generation.ID] = generation.ProjectRevision == projectRevision && generation.WorkspaceHash == workspaceHash &&
			generation.GoalsHash != "" && generation.GoalsHash == goalsHash
	}
	for i := range report.Suggestions {
		report.Suggestions[i].Freshness = "stale"
		if current[report.Suggestions[i].GenerationID] {
			report.Suggestions[i].Freshness = "current"
		}
	}
	if report.Status != "not_generated" && (report.ProjectRevision != projectRevision || report.WorkspaceHash != workspaceHash) {
		report.Freshness = "stale"
	}
}

// WorkspaceHash avoids a source walk when nothing can be stale.
func WorkspaceHash(ctx context.Context, root string, report *FeatureReport) (string, error) {
	if report.Status == "not_generated" && len(report.Suggestions) == 0 {
		return "", nil
	}
	return checkexec.WorkspaceFingerprint(ctx, root)
}

// NormalizedTitle folds only case, spacing and punctuation differences.
func NormalizedTitle(title string) string {
	var builder strings.Builder
	separated := false
	for _, r := range title {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			separated = true
			continue
		}
		if separated && builder.Len() > 0 {
			builder.WriteByte(' ')
		}
		separated = false
		builder.WriteRune(unicode.ToLower(r))
	}
	return builder.String()
}

// MergeSuggestions appends only new candidates. Existing ideas keep
// their order, IDs and statuses; dismissed ideas still suppress duplicates.
func MergeSuggestions(existing, candidates []FeatureSuggestion, generationID string) ([]FeatureSuggestion, FeatureGenerationOutcome) {
	merged := append(make([]FeatureSuggestion, 0, len(existing)+len(candidates)), existing...)
	ids, titles := map[string]bool{}, map[string]bool{}
	remember := func(idea FeatureSuggestion) {
		ids[idea.ID] = true
		if key := NormalizedTitle(idea.Title); key != "" {
			titles[key] = true
		}
	}
	for _, idea := range existing {
		remember(idea)
	}
	outcome := FeatureGenerationOutcome{GenerationID: generationID}
	for _, candidate := range candidates {
		key := NormalizedTitle(candidate.Title)
		if ids[candidate.ID] || key != "" && titles[key] {
			outcome.DuplicateCount++
			continue
		}
		candidate.Status, candidate.GenerationID = "open", generationID
		merged = append(merged, candidate)
		remember(candidate)
		outcome.AddedCount++
	}
	return merged, outcome
}

// ExistingIdeas lists stored titles newest first in whole lines within
// a fixed bound, so generation can avoid repeats without unbounded context.
func ExistingIdeas(suggestions []FeatureSuggestion) string {
	const header = "Existing ideas, newest first (status: title):\n"
	if len(suggestions) == 0 {
		return header + "None.\n"
	}
	const omissionReserve = 64
	var builder strings.Builder
	builder.WriteString(header)
	listed := 0
	for i := len(suggestions) - 1; i >= 0; i-- {
		line := "- " + suggestions[i].Status + ": " + strings.Join(strings.Fields(suggestions[i].Title), " ") + "\n"
		if builder.Len()+len(line)+omissionReserve > MaxExistingIdeasBytes {
			break
		}
		builder.WriteString(line)
		listed++
	}
	if omitted := len(suggestions) - listed; omitted > 0 {
		fmt.Fprintf(&builder, "%d older ideas omitted.\n", omitted)
	}
	return builder.String()
}

func NewGenerationID(existing []FeatureGeneration) (string, error) {
	for {
		var nonce [8]byte
		if _, err := rand.Read(nonce[:]); err != nil {
			return "", fmt.Errorf("create feature generation id: %w", err)
		}
		id := "gen-" + hex.EncodeToString(nonce[:])
		if !slices.ContainsFunc(existing, func(generation FeatureGeneration) bool { return generation.ID == id }) {
			return id, nil
		}
	}
}
