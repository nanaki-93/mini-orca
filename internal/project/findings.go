package project

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/storage"
)

const (
	findingStorePath   = ".mini-orca/findings.json"
	findingStoreSchema = "1"

	FindingSourceAI     = "ai"
	FindingSourceParser = "parser"
	FindingSourceVet    = "vet"
	FindingSourceTest   = "test"

	FindingConfidenceSuggested    = "suggested"
	FindingConfidenceToolReported = "tool_reported"

	FindingStatusOpen      = "open"
	FindingStatusDismissed = "dismissed"
	FindingStatusFixed     = "fixed"
	FindingStatusRetired   = "retired"

	FindingFreshnessFresh = "fresh"
	FindingFreshnessStale = "stale"
)

var findingSecret = regexp.MustCompile(`(?i)\b(?:api[_-]?key|access[_-]?token|auth[_-]?token|password|secret|credential|authorization)\b\s*[:=]\s*[^\s,;]+`)

// FindingLocation identifies an optional project-relative location.
type FindingLocation struct {
	Path      string `json:"path,omitempty"`
	StartLine int    `json:"start_line,omitempty"`
	EndLine   int    `json:"end_line,omitempty"`
	Symbol    string `json:"symbol,omitempty"`
}

// UnifiedFinding keeps model suggestions structurally distinct from verified
// tool output while sharing one deterministic persistence and triage contract.
type UnifiedFinding struct {
	ID                  string          `json:"id"`
	Source              string          `json:"source"`
	Confidence          string          `json:"confidence"`
	Severity            string          `json:"severity"`
	Title               string          `json:"title"`
	Message             string          `json:"message"`
	Rule                string          `json:"rule,omitempty"`
	ProjectID           string          `json:"project_id"`
	ProjectRevision     string          `json:"project_revision"`
	FileHash            string          `json:"file_hash,omitempty"`
	Location            FindingLocation `json:"location"`
	Evidence            string          `json:"evidence,omitempty"`
	Status              string          `json:"status"`
	Freshness           string          `json:"freshness"`
	DetectedAt          time.Time       `json:"detected_at"`
	OriginatingAnalysis string          `json:"originating_analysis,omitempty"`
	TaskSpec            *BugTaskSpec    `json:"task_spec,omitempty"`
}

// FindingInput is the active deterministic state used to assess freshness.
type FindingInput struct {
	ProjectID       string
	ProjectRevision string
	FileHashes      map[string]string
}

type findingDocument struct {
	SchemaVersion string           `json:"schema_version"`
	Findings      []UnifiedFinding `json:"findings"`
}

// FindingStore persists only sanitized finding metadata under .mini-orca.
type FindingStore struct {
	root string
	mu   sync.Mutex
}

func NewFindingStore(root string) (*FindingStore, error) {
	canonical, err := CanonicalRoot(root)
	if err != nil {
		return nil, err
	}
	return &FindingStore{root: canonical}, nil
}

func (s *FindingStore) Load(input FindingInput) ([]UnifiedFinding, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	findings, err := s.loadLocked()
	if err != nil {
		return nil, err
	}
	for index := range findings {
		if !findingMatchesInput(findings[index], input) {
			findings[index].Freshness = FindingFreshnessStale
		}
	}
	return cloneFindings(findings), nil
}

// ReconcileSource updates findings from one producer without retiring findings
// from the other producers. This keeps explicit scan results and cached AI
// suggestions independently refreshable.
func (s *FindingStore) ReconcileSource(input FindingInput, source string, reported []UnifiedFinding) ([]UnifiedFinding, error) {
	if !validFindingSource(source) {
		return nil, fmt.Errorf("finding source is invalid")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if input.ProjectID == "" || input.ProjectRevision == "" {
		return nil, fmt.Errorf("finding project identity is required")
	}
	previous, err := s.loadLocked()
	if err != nil {
		return nil, err
	}
	byID := make(map[string]UnifiedFinding, len(previous))
	current := make([]UnifiedFinding, 0, len(previous)+len(reported))
	for _, finding := range previous {
		if finding.Source == source {
			byID[finding.ID] = finding
			continue
		}
		current = append(current, finding)
	}
	seen := make(map[string]bool, len(reported))
	for _, finding := range reported {
		if finding.Source != source {
			return nil, fmt.Errorf("finding source does not match reconciliation source")
		}
		finding = normalizeFinding(finding, input)
		if err := validateFinding(finding); err != nil {
			return nil, err
		}
		if prior, ok := byID[finding.ID]; ok && prior.Status != FindingStatusRetired {
			finding.Status = prior.Status
		}
		seen[finding.ID] = true
		current = append(current, finding)
	}
	for id, finding := range byID {
		if seen[id] {
			continue
		}
		finding.Status = FindingStatusRetired
		finding.Freshness = FindingFreshnessStale
		current = append(current, finding)
	}
	sort.Slice(current, func(i, j int) bool { return current[i].ID < current[j].ID })
	if err := s.storeLocked(current); err != nil {
		return nil, err
	}
	return cloneFindings(current), nil
}

// SetStatus records an explicit user triage decision for one persisted finding.
func (s *FindingStore) SetStatus(id, status string) error {
	if id == "" || !validFindingStatus(status) || status == FindingStatusRetired {
		return fmt.Errorf("finding status is invalid")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	findings, err := s.loadLocked()
	if err != nil {
		return err
	}
	for index := range findings {
		if findings[index].ID == id {
			findings[index].Status = status
			return s.storeLocked(findings)
		}
	}
	return fmt.Errorf("finding not found")
}

func (s *FindingStore) loadLocked() ([]UnifiedFinding, error) {
	path := filepath.Join(s.root, findingStorePath)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return []UnifiedFinding{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read findings: %w", err)
	}
	var document findingDocument
	if err := json.Unmarshal(data, &document); err != nil || document.SchemaVersion != findingStoreSchema {
		if recoverErr := storage.RecoverCorrupt(path); recoverErr != nil {
			return nil, recoverErr
		}
		return []UnifiedFinding{}, nil
	}
	return document.Findings, nil
}

func (s *FindingStore) storeLocked(findings []UnifiedFinding) error {
	data, err := json.MarshalIndent(findingDocument{SchemaVersion: findingStoreSchema, Findings: findings}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode findings: %w", err)
	}
	if err := storage.WriteFile(filepath.Join(s.root, findingStorePath), data, 0600); err != nil {
		return fmt.Errorf("store findings: %w", err)
	}
	return nil
}

func normalizeFinding(finding UnifiedFinding, input FindingInput) UnifiedFinding {
	finding.ProjectID = input.ProjectID
	finding.ProjectRevision = input.ProjectRevision
	finding.Title = sanitizeFindingText(finding.Title, maxProjectAnalysisItemBytes)
	finding.Message = sanitizeFindingText(finding.Message, maxProjectAnalysisItemBytes)
	finding.Evidence = sanitizeFindingText(finding.Evidence, maxProjectAnalysisBytes)
	finding.TaskSpec = SanitizeBugTaskSpec(finding.TaskSpec)
	finding.Location.Path = filepath.ToSlash(strings.TrimSpace(finding.Location.Path))
	finding.Location.Symbol = strings.TrimSpace(finding.Location.Symbol)
	if finding.Status == "" {
		finding.Status = FindingStatusOpen
	}
	finding.Freshness = FindingFreshnessFresh
	if finding.DetectedAt.IsZero() {
		finding.DetectedAt = time.Now().UTC()
	} else {
		finding.DetectedAt = finding.DetectedAt.UTC()
	}
	finding.ID = FindingID(finding)
	return finding
}

// FindingID intentionally excludes revision and hash, preserving triage for an
// unchanged report while Load guards it from being treated as current.
func FindingID(finding UnifiedFinding) string {
	values := []string{finding.Source, finding.Rule, finding.Location.Path, fmt.Sprint(finding.Location.StartLine), fmt.Sprint(finding.Location.EndLine), finding.Location.Symbol, finding.Message}
	sum := sha256.Sum256([]byte(strings.Join(values, "\x00")))
	return "finding:" + hex.EncodeToString(sum[:])
}

func findingMatchesInput(finding UnifiedFinding, input FindingInput) bool {
	if finding.ProjectID != input.ProjectID || finding.ProjectRevision != input.ProjectRevision {
		return false
	}
	if finding.FileHash == "" || finding.Location.Path == "" || input.FileHashes == nil {
		return true
	}
	return input.FileHashes[finding.Location.Path] == finding.FileHash
}

func validateFinding(finding UnifiedFinding) error {
	if finding.ID == "" || !validFindingSource(finding.Source) || !validFindingConfidence(finding.Source, finding.Confidence) || !validProjectAnalysisSeverity(finding.Severity) || finding.Title == "" || finding.Message == "" || finding.ProjectID == "" || finding.ProjectRevision == "" || !validFindingStatus(finding.Status) || finding.Freshness != FindingFreshnessFresh || finding.DetectedAt.IsZero() {
		return fmt.Errorf("finding is invalid")
	}
	if len(finding.Title) > maxProjectAnalysisItemBytes || len(finding.Message) > maxProjectAnalysisItemBytes || len(finding.Evidence) > maxProjectAnalysisBytes || finding.Location.StartLine < 0 || finding.Location.EndLine < finding.Location.StartLine {
		return fmt.Errorf("finding exceeds limits")
	}
	if !validPersistedBugTaskSpec(finding.TaskSpec) {
		return fmt.Errorf("finding task specification is invalid")
	}
	return nil
}

func validPersistedBugTaskSpec(spec *BugTaskSpec) bool {
	if spec == nil {
		return true
	}
	if spec.SchemaVersion != BugTaskSpecSchemaVersion || spec.TargetPath == "" || spec.TargetSymbol == "" || spec.TargetSignature == "" || len(spec.AcceptanceCriteria) == 0 || len(spec.AcceptanceCriteria) > MaxBugTaskItems || len(spec.NonGoals) > MaxBugTaskItems {
		return false
	}
	for _, item := range append(append([]string(nil), spec.AcceptanceCriteria...), spec.NonGoals...) {
		if item == "" || len(item) > MaxBugTaskItemBytes {
			return false
		}
	}
	if candidate := spec.GoTestCandidate; candidate != nil && (candidate.Name == "" || candidate.Content == "" || len(candidate.Name) > MaxBugTaskItemBytes || len(candidate.Content) > MaxBugTaskCandidateBytes) {
		return false
	}
	return true
}

func validFindingSource(value string) bool {
	return value == FindingSourceAI || value == FindingSourceParser || value == FindingSourceVet || value == FindingSourceTest
}
func validFindingConfidence(source, confidence string) bool {
	if source == FindingSourceAI {
		return confidence == FindingConfidenceSuggested
	}
	return confidence == FindingConfidenceToolReported
}
func validFindingStatus(value string) bool {
	return value == FindingStatusOpen || value == FindingStatusDismissed || value == FindingStatusFixed || value == FindingStatusRetired
}
func sanitizeFindingText(value string, limit int) string {
	value = strings.TrimSpace(findingSecret.ReplaceAllString(value, "[redacted]"))
	if len(value) <= limit {
		return value
	}
	return value[:limit-len("[truncated]")] + "[truncated]"
}
func cloneFindings(source []UnifiedFinding) []UnifiedFinding {
	result := append([]UnifiedFinding(nil), source...)
	for index := range result {
		result[index].TaskSpec = cloneBugTaskSpec(source[index].TaskSpec)
	}
	return result
}

// SuggestedFindingsForProject adapts fresh structured AI risks without ever
// claiming they came from a verification tool.
func SuggestedFindingsForProject(report ProjectAnalysisReport) []UnifiedFinding {
	if report.Status != ProjectAnalysisStatusFresh {
		return []UnifiedFinding{}
	}
	findings := make([]UnifiedFinding, 0, len(report.Risks))
	for _, risk := range report.Risks {
		findings = append(findings, UnifiedFinding{Source: FindingSourceAI, Confidence: FindingConfidenceSuggested, Severity: risk.Severity, Title: "Project analysis suggestion", Message: risk.Summary, OriginatingAnalysis: "project"})
	}
	return findings
}

// SuggestedFindingsForFile adapts fresh file-analysis risks with the precise
// file path and hash that must remain current before a fix is prepared.
func SuggestedFindingsForFile(analysis FileAnalysis) []UnifiedFinding {
	if analysis.Status != AnalysisStatusFresh {
		return []UnifiedFinding{}
	}
	findings := make([]UnifiedFinding, 0, len(analysis.Risks))
	for _, risk := range analysis.Risks {
		location := FindingLocation{Path: analysis.Path}
		if spec := risk.TaskSpec; spec != nil {
			location.Symbol = spec.TargetSymbol
			for _, symbol := range analysis.Symbols {
				if symbol.Name == spec.TargetSymbol {
					location.StartLine = symbol.StartLine
					location.EndLine = symbol.EndLine
					break
				}
			}
		}
		findings = append(findings, UnifiedFinding{Source: FindingSourceAI, Confidence: FindingConfidenceSuggested, Severity: risk.Severity, Title: "File analysis suggestion", Message: risk.Summary, FileHash: analysis.ContentHash, Location: location, OriginatingAnalysis: "file", TaskSpec: cloneBugTaskSpec(risk.TaskSpec)})
	}
	return findings
}
