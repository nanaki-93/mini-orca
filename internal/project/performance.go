package project

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/storage"
)

const (
	PerformancePromptVersion  = "performance-file-v1"
	maxPerformanceOutputBytes = 64 * 1024
	maxPerformanceFindings    = 5
)

// PerformanceFinding is a source-based opportunity, never a measured result.
type PerformanceFinding struct {
	ID                 string              `json:"id"`
	Category           string              `json:"category"`
	PotentialImpact    string              `json:"potential_impact"`
	Confidence         string              `json:"confidence"`
	Title              string              `json:"title"`
	ObservedPattern    string              `json:"observed_pattern"`
	WorkloadConditions string              `json:"workload_conditions"`
	Recommendation     string              `json:"recommendation"`
	Tradeoff           string              `json:"tradeoff"`
	VerificationPlan   string              `json:"verification_plan"`
	StartLine          int                 `json:"start_line"`
	EndLine            int                 `json:"end_line"`
	Symbol             string              `json:"symbol,omitempty"`
	EngineeringInsight *EngineeringInsight `json:"engineering_insight,omitempty"`
}

type PerformanceFileReport struct {
	SchemaVersion        string               `json:"schema_version"`
	ProjectID            string               `json:"project_id"`
	ProjectRevision      string               `json:"project_revision"`
	Path                 string               `json:"path"`
	ContentHash          string               `json:"content_hash"`
	Status               string               `json:"status"`
	Findings             []PerformanceFinding `json:"findings"`
	Warning              string               `json:"warning,omitempty"`
	Model                string               `json:"model"`
	Profile              string               `json:"profile"`
	Scope                string               `json:"scope"`
	ProviderOrigin       string               `json:"provider_origin,omitempty"`
	ReasoningEffort      string               `json:"reasoning_effort,omitempty"`
	PromptVersion        string               `json:"prompt_version"`
	ContextPolicyVersion string               `json:"context_policy_version"`
	GeneratedAt          time.Time            `json:"generated_at"`
}

type performanceWire struct {
	Findings []performanceFindingWire `json:"findings"`
}
type performanceFindingWire struct {
	Category           string          `json:"category"`
	PotentialImpact    string          `json:"potential_impact"`
	Confidence         string          `json:"confidence"`
	Title              string          `json:"title"`
	ObservedPattern    string          `json:"observed_pattern"`
	WorkloadConditions string          `json:"workload_conditions"`
	Recommendation     string          `json:"recommendation"`
	Tradeoff           string          `json:"tradeoff"`
	VerificationPlan   string          `json:"verification_plan"`
	StartLine          int             `json:"start_line"`
	EndLine            int             `json:"end_line"`
	Symbol             string          `json:"symbol"`
	Insight            json.RawMessage `json:"engineering_insight"`
}

// ParsePerformanceFindings rejects malformed parent output but omits malformed optional insights.
func ParsePerformanceFindings(output, path, source string, symbols []SymbolInfo) ([]PerformanceFinding, string, error) {
	if len(output) == 0 || len(output) > maxPerformanceOutputBytes {
		return nil, "", fmt.Errorf("performance review output is empty or too large")
	}
	var wire performanceWire
	decoder := json.NewDecoder(strings.NewReader(output))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&wire); err != nil {
		return nil, "", fmt.Errorf("parse performance review JSON: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, "", fmt.Errorf("performance review JSON must contain one object")
	}
	if len(wire.Findings) > maxPerformanceFindings {
		return nil, "", fmt.Errorf("performance review has too many findings")
	}
	lines := strings.Count(source, "\n") + 1
	valid := make([]PerformanceFinding, 0, len(wire.Findings))
	dropped := 0
	for _, item := range wire.Findings {
		finding := PerformanceFinding{Category: strings.ToLower(strings.TrimSpace(item.Category)), PotentialImpact: strings.ToLower(strings.TrimSpace(item.PotentialImpact)), Confidence: strings.ToLower(strings.TrimSpace(item.Confidence)), Title: strings.TrimSpace(item.Title), ObservedPattern: strings.TrimSpace(item.ObservedPattern), WorkloadConditions: strings.TrimSpace(item.WorkloadConditions), Recommendation: strings.TrimSpace(item.Recommendation), Tradeoff: strings.TrimSpace(item.Tradeoff), VerificationPlan: strings.TrimSpace(item.VerificationPlan), StartLine: item.StartLine, EndLine: item.EndLine, Symbol: strings.TrimSpace(item.Symbol)}
		if !validPerformanceFinding(finding, lines, symbols) {
			dropped++
			continue
		}
		finding.EngineeringInsight, _ = ParseOptionalEngineeringInsight(item.Insight)
		sum := sha256.Sum256([]byte(path + fmt.Sprintf(":%d:%d:%s:%s", finding.StartLine, finding.EndLine, finding.Category, finding.Title)))
		finding.ID = "performance:" + hex.EncodeToString(sum[:8])
		valid = append(valid, finding)
	}
	if len(wire.Findings) > 0 && len(valid) == 0 {
		return nil, "", fmt.Errorf("performance review had no usable findings")
	}
	warning := ""
	if dropped > 0 {
		warning = "Some model opportunities were omitted because their source anchors were invalid."
	}
	return valid, warning, nil
}

func validPerformanceFinding(f PerformanceFinding, lines int, symbols []SymbolInfo) bool {
	if !oneOf(f.Category, "cpu", "memory", "io", "concurrency", "caching", "ui") || !oneOf(f.PotentialImpact, "high", "medium", "low", "unknown") || !oneOf(f.Confidence, "high", "medium", "low") || f.Title == "" || f.ObservedPattern == "" || f.WorkloadConditions == "" || f.Recommendation == "" || f.Tradeoff == "" || f.VerificationPlan == "" || f.StartLine < 1 || f.EndLine < f.StartLine || f.EndLine > lines {
		return false
	}
	if f.Symbol == "" {
		return true
	}
	for _, symbol := range symbols {
		if symbol.Name == f.Symbol && f.StartLine >= symbol.StartLine && f.EndLine <= symbol.EndLine {
			return true
		}
	}
	return false
}
func oneOf(value string, values ...string) bool {
	for _, candidate := range values {
		if value == candidate {
			return true
		}
	}
	return false
}

func StorePerformanceFileReport(root string, report PerformanceFileReport) error {
	if report.ProjectID == "" || report.Path == "" || report.ContentHash == "" || report.PromptVersion == "" || report.GeneratedAt.IsZero() {
		return fmt.Errorf("performance report is incomplete")
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return storage.WriteFile(performanceCachePath(root, report.Path), data, 0600)
}

// LoadPerformanceFileReport reads only sanitized metadata. Changed identity is
// reported as stale instead of being presented as a current review.
func LoadPerformanceFileReport(root, path, contentHash string) (*PerformanceFileReport, error) {
	cachePath := performanceCachePath(root, path)
	data, err := os.ReadFile(cachePath)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read performance cache: %w", err)
	}
	var report PerformanceFileReport
	if err := json.Unmarshal(data, &report); err != nil || report.SchemaVersion != "1" || report.Path != path || report.ContentHash == "" {
		if recoverErr := storage.RecoverCorrupt(cachePath); recoverErr != nil {
			return nil, recoverErr
		}
		return nil, nil
	}
	if report.ContentHash != contentHash {
		report.Status = "stale"
	}
	return clonePerformanceFileReport(&report), nil
}

func performanceCachePath(root, path string) string {
	key := sha256.Sum256([]byte(path))
	return filepath.Join(root, ".mini-orca", "performance", "files", hex.EncodeToString(key[:])+".json")
}

func clonePerformanceFileReport(source *PerformanceFileReport) *PerformanceFileReport {
	if source == nil {
		return nil
	}
	copy := *source
	copy.Findings = append([]PerformanceFinding(nil), source.Findings...)
	for index := range copy.Findings {
		copy.Findings[index].EngineeringInsight = CloneEngineeringInsight(source.Findings[index].EngineeringInsight)
	}
	return &copy
}
