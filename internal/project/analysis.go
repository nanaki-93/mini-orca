package project

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/nanaki-93/mini-orca/v2/internal/llm"
)

const (
	maxFileViewBytes = 1024 * 1024
	maxProjectFiles  = 20000
)

var ignoredProjectDirectories = map[string]bool{
	".git": true, ".gradle": true, ".idea": true, ".mini-orca": true,
	"node_modules": true, "vendor": true, "build": true, "dist": true,
	"target": true, "out": true, ".next": true, ".cache": true,
}

var languageByExtension = map[string]string{
	".go": "Go", ".kt": "Kotlin", ".kts": "Kotlin", ".java": "Java",
	".rs": "Rust", ".ts": "TypeScript", ".tsx": "TypeScript", ".js": "JavaScript",
	".jsx": "JavaScript", ".py": "Python", ".html": "HTML", ".css": "CSS",
	".scss": "SCSS", ".sql": "SQL", ".sh": "Shell", ".yaml": "YAML",
	".yml": "YAML", ".json": "JSON", ".md": "Markdown", ".xml": "XML",
}

type chatClient interface {
	Chat(context.Context, []llm.ChatMessage) (*llm.ChatResponse, error)
}

// Analysis is the persisted and API-facing summary for an imported project.
type Analysis struct {
	ProjectID       string                `json:"project_id"`
	ProjectRevision string                `json:"project_revision"`
	Name            string                `json:"name"`
	Path            string                `json:"path"`
	Type            string                `json:"type"`
	BuildFile       string                `json:"build_file,omitempty"`
	FileCount       int                   `json:"file_count"`
	SourceFileCount int                   `json:"source_file_count"`
	TotalLines      int                   `json:"total_lines"`
	Languages       map[string]int        `json:"languages"`
	Files           []string              `json:"files"`
	Summary         string                `json:"summary"`
	AIStatus        string                `json:"ai_status"`
	AnalyzedAt      time.Time             `json:"analyzed_at"`
	Report          ProjectAnalysisReport `json:"report"`
}

// FileInfo describes one selected project file.
type FileInfo struct {
	Path        string    `json:"path"`
	ContentHash string    `json:"content_hash"`
	Name        string    `json:"name"`
	Extension   string    `json:"extension"`
	Language    string    `json:"language"`
	SizeBytes   int64     `json:"size_bytes"`
	LineCount   int       `json:"line_count"`
	ModifiedAt  time.Time `json:"modified_at"`
	Binary      bool      `json:"binary"`
	Content     string    `json:"content,omitempty"`
}

// Analyzer scans projects and asks the configured model for an architectural summary.
type Analyzer struct {
	llm             chatClient
	model           string
	profile         string
	scope           string
	providerOrigin  string
	reasoningEffort string
}

// NewAnalyzerWithProvenance records the effective scope and sanitized provider
// origin alongside the configured model for cache freshness.
func NewAnalyzerWithProvenance(client chatClient, model, scope, providerOrigin, reasoningEffort string) *Analyzer {
	profile := scope
	if profile == "" {
		profile = "analysis"
	}
	if scope == "" {
		scope = profile
	}
	return &Analyzer{llm: client, model: model, profile: profile, scope: scope, providerOrigin: providerOrigin, reasoningEffort: reasoningEffort}
}

func (a *Analyzer) Analyze(ctx context.Context, root string) (*Analysis, error) {
	analysis, err := a.scan(ctx, root)
	if err != nil {
		return nil, err
	}

	contextText, err := NewContextBuilder().Build(analysis.Path, "")
	if err != nil {
		return nil, fmt.Errorf("build analysis context: %w", err)
	}
	report := newProjectAnalysisReportWithProvenance(analysis.ProjectID, analysis.ProjectRevision, a.model, a.profile, a.scope, a.providerOrigin, a.reasoningEffort)
	if a.llm == nil {
		report.Status = ProjectAnalysisStatusUnavailable
		report.Failure = "AI analysis is unavailable because no LLM client is configured."
	} else {
		response, chatErr := a.llm.Chat(ctx, projectAnalysisMessages(contextText))
		if chatErr != nil || response == nil || len(response.Choices) == 0 {
			report.Status = ProjectAnalysisStatusFailed
			report.Failure = "AI analysis could not be completed. The factual project inventory below is still valid."
		} else {
			parsed, parseErr := parseProjectAnalysisResponse(response.Choices[0].Message.Content)
			if parseErr != nil {
				report.Status = ProjectAnalysisStatusFailed
				report.Failure = "AI analysis returned an unusable structured report. The factual project inventory below is still valid."
			} else {
				report.Purpose = parsed.Purpose
				report.Architecture = parsed.Architecture
				report.Components = parsed.Components
				report.EntryPoints = parsed.EntryPoints
				report.Flows = parsed.Flows
				report.Risks = parsed.Risks
				report.NextSteps = parsed.NextSteps
				if response.Model != "" {
					report.Model = response.Model
				}
			}
		}
	}
	analysis.Report = report
	analysis.AIStatus = report.Status
	analysis.Summary = projectAnalysisSummary(report)

	if err := StoreProjectAnalysisReport(analysis.Path, report); err != nil {
		return nil, err
	}
	return analysis, nil
}

// Restore reloads deterministic facts and the persisted project interpretation
// without contacting a model or rewriting project artifacts.
func (a *Analyzer) Restore(root string) (*Analysis, error) {
	analysis, err := a.scan(context.Background(), root)
	if err != nil {
		return nil, err
	}
	report, err := LoadProjectAnalysisReport(analysis.Path, ProjectAnalysisInput{
		ProjectID:       analysis.ProjectID,
		ProjectRevision: analysis.ProjectRevision,
		Model:           a.model,
		Profile:         a.profile,
		Scope:           a.scope,
		ProviderOrigin:  a.providerOrigin,
		ReasoningEffort: a.reasoningEffort,
		PromptVersion:   projectAnalysisPromptVersion,
	})
	if err != nil {
		return nil, fmt.Errorf("load stored project analysis: %w", err)
	}
	if report == nil {
		return nil, fmt.Errorf("stored project analysis not found")
	}
	analysis.Report = *report
	analysis.AIStatus = report.Status
	analysis.Summary = projectAnalysisSummary(*report)
	analysis.AnalyzedAt = report.GeneratedAt
	return analysis, nil
}

func (a *Analyzer) scan(ctx context.Context, root string) (*Analysis, error) {
	canonical, err := CanonicalRoot(root)
	if err != nil {
		return nil, err
	}
	policy, err := NewContextPolicy(canonical)
	if err != nil {
		return nil, err
	}
	analysis, err := scanWithPolicy(ctx, canonical, policy)
	if err != nil {
		return nil, err
	}
	analysis.ProjectID = projectID(canonical)
	analysis.ProjectRevision, err = projectRevision(canonical)
	if err != nil {
		return nil, err
	}
	return analysis, nil
}

func scanWithPolicy(ctx context.Context, root string, policy *ContextPolicy) (*Analysis, error) {
	result := &Analysis{
		Name: filepath.Base(root), Path: root, Type: "unknown",
		Languages: make(map[string]int), AnalyzedAt: time.Now().UTC(),
	}
	detection := detectProject(root)
	result.Type = detection.Type
	result.BuildFile = detection.BuildFile
	paths, err := WalkProjectFiles(ctx, ProjectWalkOptions{
		Root: root, Policy: policy, IgnoredDirectories: ignoredProjectDirectories, IncludeSymlinkFiles: true, MaxFiles: maxProjectFiles,
	})
	if err != nil {
		return nil, fmt.Errorf("scan project: %w", err)
	}
	result.Files = paths
	result.FileCount = len(paths)
	for _, relativePath := range paths {
		language := detectLanguage(relativePath)
		if language == "Text" {
			continue
		}
		result.SourceFileCount++
		result.Languages[language]++
		path, err := ResolveFile(root, relativePath)
		if err != nil {
			continue
		}
		data, err := readLimited(path, maxFileViewBytes)
		if err == nil && !isBinary(data) {
			result.TotalLines += countLines(data)
		}
	}
	return result, nil
}

func GetFileInfo(root, relative string) (*FileInfo, error) {
	fullPath, err := ResolveFile(root, relative)
	if err != nil {
		return nil, err
	}
	stat, err := os.Stat(fullPath)
	if err != nil {
		return nil, err
	}
	if stat.IsDir() {
		return nil, fmt.Errorf("path is a directory")
	}
	if stat.Size() > maxFileViewBytes {
		return nil, fmt.Errorf("file is larger than %d bytes", maxFileViewBytes)
	}
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, err
	}
	binary := isBinary(data)
	info := &FileInfo{
		Path: filepath.ToSlash(filepath.Clean(relative)), Name: filepath.Base(relative),
		Extension: strings.TrimPrefix(strings.ToLower(filepath.Ext(relative)), "."),
		Language:  detectLanguage(relative), SizeBytes: stat.Size(), ModifiedAt: stat.ModTime(), Binary: binary,
	}
	if !binary {
		info.LineCount = countLines(data)
		info.Content = string(data)
	}
	info.ContentHash = contentHash(data)
	return info, nil
}

func readLimited(path string, limit int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(io.LimitReader(file, limit+1))
}

func isBinary(data []byte) bool {
	sample := data
	if len(sample) > 8192 {
		sample = sample[:8192]
	}
	return bytes.IndexByte(sample, 0) >= 0 || !utf8.Valid(sample)
}

func countLines(data []byte) int {
	if len(data) == 0 {
		return 0
	}
	lines := bytes.Count(data, []byte{'\n'})
	if data[len(data)-1] != '\n' {
		lines++
	}
	return lines
}

func detectLanguage(path string) string {
	if language := languageByExtension[strings.ToLower(filepath.Ext(path))]; language != "" {
		return language
	}
	return "Text"
}
