package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/nanaki-93/mini-orca/v2/internal/project"
	"github.com/nanaki-93/mini-orca/v2/internal/storage"
)

const (
	legacyFeatureSchemaVersion       = 1
	featureHistorySchemaVersion      = 2
	maxFeatureHistoryBytes           = 64 * 1024
	maxExistingFeatureIdeasBytes     = 8 * 1024
	maxFeatureSuggestionsPerResponse = 5
	legacyFeatureGenerationID        = "legacy"
)

// errFeatureHistoryFull keeps the saved report unchanged instead of evicting ideas.
var errFeatureHistoryFull = errors.New("feature history is full; existing ideas were kept")

var errUnsupportedFeatureHistory = errors.New("unsupported feature history")

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

func emptyFeatureReport(index *project.ProjectIndex) *FeatureReport {
	return &FeatureReport{SchemaVersion: featureHistorySchemaVersion, ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision, Hash: "empty",
		Status: "not_generated", Freshness: "current", Suggestions: []FeatureSuggestion{}, Generations: []FeatureGeneration{}}
}

func readFeatures(root string, index *project.ProjectIndex) (*FeatureReport, error) {
	path, err := changeMetadataPath(root, "features.json")
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return emptyFeatureReport(index), nil
	}
	if err != nil {
		return nil, err
	}
	if len(data) > maxFeatureHistoryBytes {
		return nil, fmt.Errorf("feature history exceeds 64 KiB")
	}
	var report FeatureReport
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&report); err != nil {
		return nil, fmt.Errorf("read feature history: %w", err)
	}
	if decoder.Decode(new(any)) != io.EOF || report.ProjectID != index.ProjectID {
		return nil, errUnsupportedFeatureHistory
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
	case legacyFeatureSchemaVersion:
		if err := convertLegacyFeatureHistory(report); err != nil {
			return err
		}
	case featureHistorySchemaVersion:
		if report.Generations == nil {
			return errUnsupportedFeatureHistory
		}
	default:
		return errUnsupportedFeatureHistory
	}
	return validateFeatureReferences(report)
}

// Legacy ideas share one generation built from the v1 report context. Goals are
// trusted only when v1 still marked them current; otherwise the ideas stay stale.
func convertLegacyFeatureHistory(report *FeatureReport) error {
	if report.Generations != nil || report.LastGeneration != nil {
		return errUnsupportedFeatureHistory
	}
	report.Generations = []FeatureGeneration{}
	if len(report.Suggestions) == 0 {
		return nil
	}
	legacy := FeatureGeneration{ID: legacyFeatureGenerationID, GeneratedAt: report.UpdatedAt, ProjectRevision: report.ProjectRevision, WorkspaceHash: report.WorkspaceHash}
	if report.Freshness == "current" {
		legacy.GoalsHash = contentHash([]byte(report.Goals))
	}
	for i := range report.Suggestions {
		if report.Suggestions[i].GenerationID != "" {
			return errUnsupportedFeatureHistory
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
			return errUnsupportedFeatureHistory
		}
		generations[generation.ID] = true
	}
	ideas := make(map[string]bool, len(report.Suggestions))
	for i := range report.Suggestions {
		idea := &report.Suggestions[i]
		if idea.ID == "" || ideas[idea.ID] || !generations[idea.GenerationID] {
			return errUnsupportedFeatureHistory
		}
		ideas[idea.ID] = true
		// Freshness is derived for each response and never trusted from storage.
		idea.Freshness = ""
	}
	if last := report.LastGeneration; last != nil && (!generations[last.GenerationID] || !validFeatureOutcomeCounts(*last)) {
		return errUnsupportedFeatureHistory
	}
	return nil
}

func validFeatureOutcomeCounts(outcome FeatureGenerationOutcome) bool {
	return outcome.AddedCount >= 0 && outcome.DuplicateCount >= 0 && outcome.AddedCount+outcome.DuplicateCount <= maxFeatureSuggestionsPerResponse
}

// writeFeatures persists schema v2 without derived freshness. A report that
// would exceed the history bound is rejected before any write.
func writeFeatures(root string, report *FeatureReport) error {
	path, err := changeMetadataPath(root, "features.json")
	if err != nil {
		return err
	}
	report.SchemaVersion = featureHistorySchemaVersion
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
	report.Hash = contentHash(data)
	data, err = json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if len(data) > maxFeatureHistoryBytes {
		return fmt.Errorf("%w: the saved report would exceed 64 KiB", errFeatureHistoryFull)
	}
	return storage.WriteFile(path, data, 0600)
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

// deriveFeatureFreshness marks an idea current only when its own generation
// matches the current project revision, workspace and saved goals. Report-level
// freshness keeps describing only the most recent generation context.
func deriveFeatureFreshness(report *FeatureReport, projectRevision, workspaceHash string) {
	goalsHash := contentHash([]byte(report.Goals))
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

// featureWorkspaceHash avoids a source walk when nothing can be stale.
func featureWorkspaceHash(ctx context.Context, root string, report *FeatureReport) (string, error) {
	if report.Status == "not_generated" && len(report.Suggestions) == 0 {
		return "", nil
	}
	return benchmarkSourceFingerprint(ctx, root)
}

// normalizedFeatureTitle folds only case, spacing and punctuation differences.
func normalizedFeatureTitle(title string) string {
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

// mergeFeatureSuggestions appends only new candidates. Existing ideas keep
// their order, IDs and statuses; dismissed ideas still suppress duplicates.
func mergeFeatureSuggestions(existing, candidates []FeatureSuggestion, generationID string) ([]FeatureSuggestion, FeatureGenerationOutcome) {
	merged := append(make([]FeatureSuggestion, 0, len(existing)+len(candidates)), existing...)
	ids, titles := map[string]bool{}, map[string]bool{}
	remember := func(idea FeatureSuggestion) {
		ids[idea.ID] = true
		if key := normalizedFeatureTitle(idea.Title); key != "" {
			titles[key] = true
		}
	}
	for _, idea := range existing {
		remember(idea)
	}
	outcome := FeatureGenerationOutcome{GenerationID: generationID}
	for _, candidate := range candidates {
		key := normalizedFeatureTitle(candidate.Title)
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

// existingFeatureIdeas lists stored titles newest first in whole lines within
// a fixed bound, so generation can avoid repeats without unbounded context.
func existingFeatureIdeas(suggestions []FeatureSuggestion) string {
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
		if builder.Len()+len(line)+omissionReserve > maxExistingFeatureIdeasBytes {
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

func newFeatureGenerationID(existing []FeatureGeneration) (string, error) {
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

func featureModelSummary(runtime modelRuntime) *FeatureModelSummary {
	return &FeatureModelSummary{Profile: runtime.effective.Profile, Model: runtime.effective.Model,
		ProviderOrigin: runtime.effective.ProviderOrigin, RemoteProvider: runtime.effective.RemoteProvider}
}
