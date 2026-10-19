package symbols

import (
	"path/filepath"
	"strings"
	"time"
)

// ProjectIndex holds deterministic project facts. It never contains source text.
type ProjectIndex struct {
	SchemaVersion   string      `json:"schema_version"`
	ProjectID       string      `json:"project_id"`
	ProjectRevision string      `json:"project_revision"`
	GeneratedAt     time.Time   `json:"generated_at"`
	Files           []IndexFile `json:"files"`
}

// IndexFile is the persisted deterministic metadata for one eligible file.
type IndexFile struct {
	Path           string       `json:"path"`
	ContentHash    string       `json:"content_hash"`
	Language       string       `json:"language"`
	SizeBytes      int64        `json:"size_bytes"`
	LineCount      int          `json:"line_count"`
	ModifiedAt     time.Time    `json:"modified_at"`
	Binary         bool         `json:"binary"`
	Imports        []string     `json:"imports"`
	Symbols        []SymbolInfo `json:"symbols"`
	Diagnostics    []Diagnostic `json:"diagnostics,omitempty"`
	AnalysisStatus string       `json:"analysis_status"`
}

// SymbolInfo describes a deterministic or approximate source declaration span.
type SymbolInfo struct {
	Name         string `json:"name"`
	Kind         string `json:"kind"`
	Signature    string `json:"signature"`
	StartLine    int    `json:"start_line"`
	EndLine      int    `json:"end_line"`
	Visibility   string `json:"visibility"`
	Confidence   string `json:"confidence"`
	AtomicTarget bool   `json:"atomic_target"`
}

// Diagnostic records non-fatal deterministic extraction issues.
type Diagnostic struct {
	Message string `json:"message"`
	Line    int    `json:"line,omitempty"`
}

var languageByExtension = map[string]string{
	".go": "Go", ".kt": "Kotlin", ".kts": "Kotlin", ".java": "Java",
	".rs": "Rust", ".ts": "TypeScript", ".tsx": "TypeScript", ".mts": "TypeScript", ".cts": "TypeScript",
	".js": "JavaScript", ".jsx": "JavaScript", ".mjs": "JavaScript", ".cjs": "JavaScript",
	".py": "Python", ".pyi": "Python", ".rb": "Ruby", ".php": "PHP", ".swift": "Swift",
	".c": "C", ".h": "C", ".cpp": "C++", ".cc": "C++", ".cxx": "C++", ".hpp": "C++", ".hh": "C++",
	".cs": "C#", ".scala": "Scala", ".dart": "Dart", ".lua": "Lua", ".zig": "Zig",
	".ex": "Elixir", ".exs": "Elixir", ".erl": "Erlang", ".hs": "Haskell", ".ml": "OCaml", ".mli": "OCaml",
	".r": "R", ".jl": "Julia", ".clj": "Clojure", ".groovy": "Groovy", ".gradle": "Groovy", ".sol": "Solidity",
	".vue": "Vue", ".svelte": "Svelte", ".astro": "Astro",
	".html": "HTML", ".css": "CSS", ".scss": "SCSS", ".sql": "SQL",
	".graphql": "GraphQL", ".gql": "GraphQL", ".proto": "Protobuf", ".tf": "Terraform", ".hcl": "HCL",
	".sh": "Shell", ".bash": "Shell", ".zsh": "Shell", ".fish": "Shell", ".ps1": "PowerShell", ".bat": "Batch", ".cmd": "Batch",
	".yaml": "YAML", ".yml": "YAML", ".json": "JSON", ".toml": "TOML", ".ini": "INI", ".cfg": "INI", ".conf": "Config",
	".md": "Markdown", ".xml": "XML",
}

// DetectLanguage maps a file path or special filename to its canonical language name.
func DetectLanguage(path string) string {
	if language := languageByExtension[strings.ToLower(filepath.Ext(path))]; language != "" {
		return language
	}
	switch strings.ToLower(filepath.Base(path)) {
	case "dockerfile":
		return "Dockerfile"
	case "makefile", "gnumakefile":
		return "Makefile"
	case "gemfile", "rakefile":
		return "Ruby"
	case "cmakelists.txt":
		return "CMake"
	}
	return "Text"
}

// HasKnownExtension reports whether the file path's extension maps to a known language.
func HasKnownExtension(path string) bool {
	return languageByExtension[strings.ToLower(filepath.Ext(path))] != ""
}
