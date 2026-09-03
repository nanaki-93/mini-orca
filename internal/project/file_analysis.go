package project

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	fileAnalysisSchemaVersion = "1"
	fileAnalysisRelativeDir   = ".mini-orca/file-analysis"

	AnalysisStatusFresh   = "fresh"
	AnalysisStatusMissing = "missing"
	AnalysisStatusStale   = "stale"
	AnalysisStatusFailed  = "failed"
	AnalysisStatusRunning = "running"
)

// Finding is model-derived advice, not a deterministic project fact.
type Finding struct {
	Severity string       `json:"severity"`
	Summary  string       `json:"summary"`
	TaskSpec *BugTaskSpec `json:"task_spec,omitempty"`
}

// Suggestion is an optional, user-reviewed atomic improvement proposal.
type Suggestion struct {
	Title        string `json:"title"`
	Summary      string `json:"summary"`
	TargetSymbol string `json:"target_symbol,omitempty"`
	Action       string `json:"action,omitempty"`
}

// FileAnalysis is a source-free, versioned semantic summary for one file.
// Deterministic symbols and imports remain in ProjectIndex; this record only
// stores model interpretation and its validity inputs.
type FileAnalysis struct {
	SchemaVersion        string            `json:"schema_version"`
	ProjectID            string            `json:"project_id"`
	ProjectRevision      string            `json:"project_revision"`
	Path                 string            `json:"path"`
	ContentHash          string            `json:"content_hash"`
	Language             string            `json:"language"`
	Purpose              string            `json:"purpose,omitempty"`
	Responsibilities     []string          `json:"responsibilities,omitempty"`
	Symbols              []SymbolInfo      `json:"symbols,omitempty"`
	Imports              []string          `json:"imports,omitempty"`
	Dependencies         []string          `json:"dependencies,omitempty"`
	SideEffects          []string          `json:"side_effects,omitempty"`
	Risks                []Finding         `json:"risks,omitempty"`
	Suggestions          []Suggestion      `json:"suggestions,omitempty"`
	SymbolExplanations   map[string]string `json:"symbol_explanations,omitempty"`
	Status               string            `json:"status"`
	Failure              string            `json:"failure,omitempty"`
	Model                string            `json:"model,omitempty"`
	ConfiguredModel      string            `json:"configured_model,omitempty"`
	Profile              string            `json:"profile,omitempty"`
	Scope                string            `json:"scope,omitempty"`
	ProviderOrigin       string            `json:"provider_origin,omitempty"`
	ReasoningEffort      string            `json:"reasoning_effort,omitempty"`
	PromptVersion        string            `json:"prompt_version"`
	ContextPolicyVersion string            `json:"context_policy_version"`
	GeneratedAt          time.Time         `json:"generated_at"`
}

// FileAnalysisInput identifies the exact inputs that make an analysis valid.
type FileAnalysisInput struct {
	ProjectID            string
	ProjectRevision      string
	Path                 string
	ContentHash          string
	Language             string
	Model                string
	Profile              string
	Scope                string
	ProviderOrigin       string
	ReasoningEffort      string
	PromptVersion        string
	ContextPolicyVersion string
}

// FileAnalysisCache persists semantic summaries under one project root.
type FileAnalysisCache struct {
	root string
	mu   sync.Mutex
}

func NewFileAnalysisCache(root string) (*FileAnalysisCache, error) {
	canonical, err := CanonicalRoot(root)
	if err != nil {
		return nil, err
	}
	return &FileAnalysisCache{root: canonical}, nil
}

// Load returns a cache entry whose status is fresh, stale, missing, failed, or
// running. It never reads source content and recovers corrupt cache files.
func (c *FileAnalysisCache) Load(input FileAnalysisInput) (*FileAnalysis, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.validateInput(input); err != nil {
		return nil, err
	}
	path := c.cachePath(input.Path)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return newMissingAnalysis(input), nil
	}
	if err != nil {
		return nil, fmt.Errorf("read file analysis cache: %w", err)
	}
	var analysis FileAnalysis
	if err := json.Unmarshal(data, &analysis); err != nil || !validStoredAnalysis(analysis) {
		if err := c.recoverCorrupt(path); err != nil {
			return nil, err
		}
		return newMissingAnalysis(input), nil
	}
	if !analysisMatches(analysis, input) {
		analysis.Status = AnalysisStatusStale
	}
	return cloneFileAnalysis(&analysis), nil
}

// Store atomically persists a source-free analysis whose cache key is derived
// from its normalized project-relative path.
func (c *FileAnalysisCache) Store(analysis FileAnalysis) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	analysis = *cloneFileAnalysis(&analysis)
	for index := range analysis.Risks {
		analysis.Risks[index].TaskSpec = SanitizeBugTaskSpec(analysis.Risks[index].TaskSpec)
		if !validPersistedBugTaskSpec(analysis.Risks[index].TaskSpec) {
			return fmt.Errorf("file analysis task specification is invalid")
		}
	}
	input := inputFromAnalysis(analysis)
	if err := c.validateInput(input); err != nil {
		return err
	}
	if !validAnalysisStatus(analysis.Status) || analysis.Status == AnalysisStatusMissing || analysis.Status == AnalysisStatusStale {
		return fmt.Errorf("cannot persist file analysis status %q", analysis.Status)
	}
	analysis.SchemaVersion = fileAnalysisSchemaVersion
	analysis.Path = normalizedAnalysisPath(analysis.Path)
	analysis.GeneratedAt = analysis.GeneratedAt.UTC()
	if analysis.GeneratedAt.IsZero() {
		analysis.GeneratedAt = time.Now().UTC()
	}
	data, err := json.MarshalIndent(analysis, "", "  ")
	if err != nil {
		return fmt.Errorf("encode file analysis cache: %w", err)
	}
	directory := filepath.Join(c.root, fileAnalysisRelativeDir)
	if err := os.MkdirAll(directory, 0700); err != nil {
		return fmt.Errorf("create file analysis cache directory: %w", err)
	}
	temp, err := os.CreateTemp(directory, ".analysis-*.tmp")
	if err != nil {
		return fmt.Errorf("create file analysis cache temp file: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return fmt.Errorf("write file analysis cache: %w", err)
	}
	if err := temp.Chmod(0600); err != nil {
		temp.Close()
		return fmt.Errorf("set file analysis cache permissions: %w", err)
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return fmt.Errorf("sync file analysis cache: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close file analysis cache: %w", err)
	}
	if err := os.Rename(tempPath, c.cachePath(analysis.Path)); err != nil {
		return fmt.Errorf("replace file analysis cache: %w", err)
	}
	return nil
}

// Delete removes one analysis cache entry after the same path-policy checks as
// Load. A missing entry is already absent and therefore succeeds.
func (c *FileAnalysisCache) Delete(relative string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	input := FileAnalysisInput{Path: relative, ProjectID: "cache", ProjectRevision: "cache", ContentHash: "cache", Language: "Text", PromptVersion: "cache", ContextPolicyVersion: contextPolicyVersion}
	if err := c.validateInput(input); err != nil {
		return err
	}
	err := os.Remove(c.cachePath(relative))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("delete file analysis cache: %w", err)
	}
	return nil
}

func (c *FileAnalysisCache) validateInput(input FileAnalysisInput) error {
	policy, err := NewContextPolicy(c.root)
	if err != nil {
		return err
	}
	decision := policy.Decide(input.Path)
	if decision.Reason == "unsafe path" || !decision.Include {
		return fmt.Errorf("file analysis path is not eligible: %s", decision.Reason)
	}
	if _, err := ResolveFile(c.root, input.Path); err != nil {
		return err
	}
	for _, value := range []string{input.ProjectID, input.ProjectRevision, input.ContentHash, input.Language, input.PromptVersion, input.ContextPolicyVersion} {
		if value == "" {
			return fmt.Errorf("file analysis validity inputs are required")
		}
	}
	return nil
}

func (c *FileAnalysisCache) cachePath(relative string) string {
	return filepath.Join(c.root, fileAnalysisRelativeDir, analysisPathKey(normalizedAnalysisPath(relative))+".json")
}

func (c *FileAnalysisCache) recoverCorrupt(path string) error {
	corrupt := path + ".corrupt-" + time.Now().UTC().Format("20060102T150405.000000000")
	if err := os.Rename(path, corrupt); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("recover corrupt file analysis cache: %w", err)
	}
	return nil
}

func newMissingAnalysis(input FileAnalysisInput) *FileAnalysis {
	return &FileAnalysis{SchemaVersion: fileAnalysisSchemaVersion, ProjectID: input.ProjectID, ProjectRevision: input.ProjectRevision, Path: normalizedAnalysisPath(input.Path), ContentHash: input.ContentHash, Language: input.Language, Status: AnalysisStatusMissing, Model: input.Model, ConfiguredModel: input.Model, Profile: input.Profile, Scope: input.Scope, ProviderOrigin: input.ProviderOrigin, ReasoningEffort: input.ReasoningEffort, PromptVersion: input.PromptVersion, ContextPolicyVersion: input.ContextPolicyVersion}
}

func inputFromAnalysis(analysis FileAnalysis) FileAnalysisInput {
	model := analysis.ConfiguredModel
	if model == "" {
		model = analysis.Model
	}
	return FileAnalysisInput{ProjectID: analysis.ProjectID, ProjectRevision: analysis.ProjectRevision, Path: analysis.Path, ContentHash: analysis.ContentHash, Language: analysis.Language, Model: model, Profile: analysis.Profile, Scope: analysis.Scope, ProviderOrigin: analysis.ProviderOrigin, ReasoningEffort: analysis.ReasoningEffort, PromptVersion: analysis.PromptVersion, ContextPolicyVersion: analysis.ContextPolicyVersion}
}

func analysisMatches(analysis FileAnalysis, input FileAnalysisInput) bool {
	// A project revision changes for every eligible source edit. Content hashes
	// keep invalidation local to the edited file while the stored revision still
	// records the project state that informed the original summary.
	return analysis.SchemaVersion == fileAnalysisSchemaVersion && analysis.ProjectID == input.ProjectID && analysis.Path == normalizedAnalysisPath(input.Path) && analysis.ContentHash == input.ContentHash && analysis.Language == input.Language && modelsMatch(analysis.ConfiguredModel, input.Model) && analysis.Profile == input.Profile && analysis.Scope == input.Scope && analysis.ProviderOrigin == input.ProviderOrigin && analysis.ReasoningEffort == input.ReasoningEffort && analysis.PromptVersion == input.PromptVersion && analysis.ContextPolicyVersion == input.ContextPolicyVersion
}

func modelsMatch(stored, requested string) bool {
	return requested == "" || stored == "" || stored == requested
}

func validStoredAnalysis(analysis FileAnalysis) bool {
	return analysis.SchemaVersion == fileAnalysisSchemaVersion && analysis.Path != "" && analysis.ProjectID != "" && analysis.ProjectRevision != "" && analysis.ContentHash != "" && analysis.Language != "" && analysis.PromptVersion != "" && analysis.ContextPolicyVersion != "" && validAnalysisStatus(analysis.Status)
}

func validAnalysisStatus(status string) bool {
	switch status {
	case AnalysisStatusFresh, AnalysisStatusMissing, AnalysisStatusStale, AnalysisStatusFailed, AnalysisStatusRunning:
		return true
	default:
		return false
	}
}

func normalizedAnalysisPath(path string) string { return filepath.ToSlash(filepath.Clean(path)) }

func analysisPathKey(path string) string {
	sum := sha256.Sum256([]byte(path))
	return hex.EncodeToString(sum[:])
}

func cloneFileAnalysis(source *FileAnalysis) *FileAnalysis {
	copy := *source
	copy.Responsibilities = append([]string(nil), source.Responsibilities...)
	copy.Symbols = append([]SymbolInfo(nil), source.Symbols...)
	copy.Imports = append([]string(nil), source.Imports...)
	copy.Dependencies = append([]string(nil), source.Dependencies...)
	copy.SideEffects = append([]string(nil), source.SideEffects...)
	copy.Risks = append([]Finding(nil), source.Risks...)
	for index := range copy.Risks {
		copy.Risks[index].TaskSpec = cloneBugTaskSpec(source.Risks[index].TaskSpec)
	}
	copy.Suggestions = append([]Suggestion(nil), source.Suggestions...)
	if source.SymbolExplanations != nil {
		copy.SymbolExplanations = make(map[string]string, len(source.SymbolExplanations))
		for name, explanation := range source.SymbolExplanations {
			copy.SymbolExplanations[name] = explanation
		}
	}
	return &copy
}
