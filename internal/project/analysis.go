package project

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/nanaki-93/mini-orca/v2/internal/llm"
	"github.com/nanaki-93/mini-orca/v2/internal/tools"
)

const (
	analysisRelativePath = ".mini-orca/analysis.md"
	maxFileViewBytes     = 1024 * 1024
	maxProjectFiles      = 20000
)

var ignoredDirectories = map[string]bool{
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
	ProjectID       string         `json:"project_id"`
	ProjectRevision string         `json:"project_revision"`
	Name            string         `json:"name"`
	Path            string         `json:"path"`
	Type            string         `json:"type"`
	BuildFile       string         `json:"build_file,omitempty"`
	FileCount       int            `json:"file_count"`
	SourceFileCount int            `json:"source_file_count"`
	TotalLines      int            `json:"total_lines"`
	Languages       map[string]int `json:"languages"`
	Files           []string       `json:"files"`
	AnalysisFile    string         `json:"analysis_file"`
	Summary         string         `json:"summary"`
	AIStatus        string         `json:"ai_status"`
	AnalyzedAt      time.Time      `json:"analyzed_at"`
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
	llm chatClient
}

func NewAnalyzer(client chatClient) *Analyzer {
	return &Analyzer{llm: client}
}

func (a *Analyzer) Analyze(ctx context.Context, root string) (*Analysis, error) {
	canonical, err := CanonicalRoot(root)
	if err != nil {
		return nil, err
	}
	policy, err := NewContextPolicy(canonical)
	if err != nil {
		return nil, err
	}
	analysis, err := scanWithPolicy(canonical, policy)
	if err != nil {
		return nil, err
	}

	contextText, err := NewContextBuilder().Build(canonical, "")
	if err != nil {
		return nil, fmt.Errorf("build analysis context: %w", err)
	}
	analysis.AIStatus = "complete"
	if a.llm == nil {
		analysis.AIStatus = "unavailable"
		analysis.Summary = "AI analysis is unavailable because no LLM client is configured."
	} else {
		response, chatErr := a.llm.Chat(ctx, []llm.ChatMessage{
			{Role: "system", Content: "You are a software architect. Analyze the supplied project facts and context. Be concise and factual. Return Markdown with: Purpose, Architecture, Entry points, Data/control flow, Risks, and Suggested next steps. Do not invent files or dependencies."},
			{Role: "user", Content: contextText},
		})
		if chatErr != nil || len(response.Choices) == 0 {
			analysis.AIStatus = "failed"
			analysis.Summary = "AI analysis could not be completed. The factual project inventory below is still valid."
		} else {
			analysis.Summary = strings.TrimSpace(response.Choices[0].Message.Content)
		}
	}

	if err := writeAnalysis(canonical, analysis); err != nil {
		return nil, err
	}
	return analysis, nil
}

func scan(root string) (*Analysis, error) {
	policy, err := NewContextPolicy(root)
	if err != nil {
		return nil, err
	}
	return scanWithPolicy(root, policy)
}

func scanWithPolicy(root string, policy *ContextPolicy) (*Analysis, error) {
	result := &Analysis{
		Name: filepath.Base(root), Path: root, Type: string(tools.ProjectTypeUnknown),
		Languages: make(map[string]int), AnalysisFile: analysisRelativePath, AnalyzedAt: time.Now().UTC(),
	}
	if info, err := tools.NewProjectDetectorExecutor().DetectProjectType(root); err == nil {
		result.Type = string(info.Type)
		result.BuildFile = info.BuildFile
	}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if entry.IsDir() {
			if path != root && ignoredDirectories[entry.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if len(result.Files) >= maxProjectFiles {
			return fmt.Errorf("project contains more than %d files", maxProjectFiles)
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		relativePath := filepath.ToSlash(relative)
		if !policy.Decide(relativePath).Include {
			return nil
		}
		result.Files = append(result.Files, relativePath)
		result.FileCount++
		language := detectLanguage(path)
		if language == "Text" {
			return nil
		}
		result.SourceFileCount++
		result.Languages[language]++
		data, err := readLimited(path, maxFileViewBytes)
		if err == nil && !isBinary(data) {
			result.TotalLines += countLines(data)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan project: %w", err)
	}
	sort.Strings(result.Files)
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

func writeAnalysis(root string, analysis *Analysis) error {
	directory := filepath.Join(root, filepath.Dir(analysisRelativePath))
	if err := os.MkdirAll(directory, 0755); err != nil {
		return fmt.Errorf("create analysis directory: %w", err)
	}
	var markdown strings.Builder
	markdown.WriteString("# Project analysis: " + analysis.Name + "\n\n")
	markdown.WriteString(fmt.Sprintf("Generated: %s  \nAI status: %s  \nProject type: %s  \nBuild file: %s  \nFiles: %d (%d source)  \nLines: %d\n\n", analysis.AnalyzedAt.Format(time.RFC3339), analysis.AIStatus, analysis.Type, analysis.BuildFile, analysis.FileCount, analysis.SourceFileCount, analysis.TotalLines))
	markdown.WriteString("## Languages\n\n")
	languages := make([]string, 0, len(analysis.Languages))
	for language := range analysis.Languages {
		languages = append(languages, language)
	}
	sort.Strings(languages)
	for _, language := range languages {
		markdown.WriteString(fmt.Sprintf("- %s: %d files\n", language, analysis.Languages[language]))
	}
	markdown.WriteString("\n## AI architecture summary\n\n" + analysis.Summary + "\n\n## File inventory\n\n")
	for _, file := range analysis.Files {
		markdown.WriteString("- `" + strings.ReplaceAll(file, "`", "") + "`\n")
	}
	if err := os.WriteFile(filepath.Join(root, analysisRelativePath), []byte(markdown.String()), 0644); err != nil {
		return fmt.Errorf("write analysis file: %w", err)
	}
	return nil
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
