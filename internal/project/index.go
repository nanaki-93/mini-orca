package project

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/storage"
)

const (
	indexSchemaVersion = "3"
	indexRelativePath  = ".mini-orca/index.json"
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

// SymbolInfo is populated by language extractors in later tasks. Defining the
// stable index shape now keeps persisted facts and API clients forward-compatible.
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

// BuildIndex scans only context-policy-eligible project files and writes the
// complete index atomically. Existing unchanged entries are reused by hash.
func BuildIndex(root, id, revision string) (*ProjectIndex, error) {
	return buildIndex(root, id, revision, true)
}

// buildIndex can refresh an in-memory index for startup restoration without
// rewriting the persisted cache.
func buildIndex(root, id, revision string, persist bool) (*ProjectIndex, error) {
	canonical, err := CanonicalRoot(root)
	if err != nil {
		return nil, err
	}
	policy, err := NewContextPolicy(canonical)
	if err != nil {
		return nil, err
	}
	previous, _ := loadIndex(canonical)
	previousFiles := make(map[string]IndexFile)
	if previous != nil && previous.SchemaVersion == indexSchemaVersion {
		for _, file := range previous.Files {
			previousFiles[file.Path] = file
		}
	}
	paths, err := WalkProjectFiles(context.Background(), ProjectWalkOptions{
		Root: canonical, IgnoredDirectories: ignoredProjectDirectories, IncludeSymlinkFiles: true, MaxFiles: maxProjectFiles,
	})
	if err != nil {
		return nil, err
	}
	index := &ProjectIndex{
		SchemaVersion: indexSchemaVersion, ProjectID: id, ProjectRevision: revision,
		GeneratedAt: time.Now().UTC(), Files: make([]IndexFile, 0, len(paths)),
	}
	for _, relative := range paths {
		if !policy.Decide(relative).Include {
			continue
		}
		entry, err := buildIndexFile(canonical, relative, previousFiles[relative])
		if err != nil {
			return nil, err
		}
		index.Files = append(index.Files, entry)
	}
	if persist {
		if err := writeIndex(canonical, index); err != nil {
			return nil, err
		}
	}
	return index, nil
}

func buildIndexFile(root, relative string, previous IndexFile) (IndexFile, error) {
	fullPath, err := ResolveFile(root, relative)
	if err != nil {
		return IndexFile{}, fmt.Errorf("resolve index file %s: %w", relative, err)
	}
	info, err := os.Stat(fullPath)
	if err != nil {
		return IndexFile{}, fmt.Errorf("stat index file %s: %w", relative, err)
	}
	content, err := readIndexContent(fullPath, info)
	if err != nil {
		return IndexFile{}, fmt.Errorf("read index file %s: %w", relative, err)
	}
	entry := IndexFile{
		Path: relative, ContentHash: content.hash, Language: detectLanguage(relative), SizeBytes: content.size,
		ModifiedAt: info.ModTime().UTC(), Binary: content.binary, Imports: []string{}, Symbols: []SymbolInfo{}, AnalysisStatus: "missing",
	}
	if !entry.Binary {
		entry.LineCount = content.lines
	}
	if previous.Path == relative && previous.ContentHash == content.hash {
		entry.Imports = append([]string(nil), previous.Imports...)
		entry.Symbols = append([]SymbolInfo(nil), previous.Symbols...)
		entry.Diagnostics = append([]Diagnostic(nil), previous.Diagnostics...)
		entry.AnalysisStatus = previous.AnalysisStatus
	} else if !entry.Binary && content.size > maxFileViewBytes {
		entry.Diagnostics = []Diagnostic{{Message: fmt.Sprintf("File exceeds %d bytes; symbol extraction is unavailable.", maxFileViewBytes)}}
	} else if entry.Language == "Go" && !entry.Binary {
		entry.Imports, entry.Symbols, entry.Diagnostics = extractGoFacts(relative, content.source)
	} else if !entry.Binary {
		entry.Imports, entry.Symbols = extractGenericFacts(entry.Language, content.source)
	}
	return entry, nil
}

func loadIndex(root string) (*ProjectIndex, error) {
	data, err := os.ReadFile(filepath.Join(root, indexRelativePath))
	if err != nil {
		return nil, err
	}
	var index ProjectIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, err
	}
	if index.SchemaVersion != indexSchemaVersion {
		return nil, fmt.Errorf("unsupported index schema %q", index.SchemaVersion)
	}
	return &index, nil
}

func writeIndex(root string, index *ProjectIndex) error {
	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return fmt.Errorf("encode index: %w", err)
	}
	if err := storage.WriteFile(filepath.Join(root, indexRelativePath), data, 0600); err != nil {
		return fmt.Errorf("store index: %w", err)
	}
	return nil
}
