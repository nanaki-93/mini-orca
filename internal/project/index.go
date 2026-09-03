package project

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	indexSchemaVersion = "2"
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
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return IndexFile{}, fmt.Errorf("read index file %s: %w", relative, err)
	}
	hash := contentHash(data)
	entry := IndexFile{
		Path: relative, ContentHash: hash, Language: detectLanguage(relative), SizeBytes: info.Size(),
		ModifiedAt: info.ModTime().UTC(), Binary: isBinary(data), Imports: []string{}, Symbols: []SymbolInfo{}, AnalysisStatus: "missing",
	}
	if !entry.Binary {
		entry.LineCount = countLines(data)
	}
	if previous.Path == relative && previous.ContentHash == hash {
		entry.Imports = append([]string(nil), previous.Imports...)
		entry.Symbols = append([]SymbolInfo(nil), previous.Symbols...)
		entry.Diagnostics = append([]Diagnostic(nil), previous.Diagnostics...)
		entry.AnalysisStatus = previous.AnalysisStatus
	} else if entry.Language == "Go" && !entry.Binary {
		entry.Imports, entry.Symbols, entry.Diagnostics = extractGoFacts(relative, data)
	} else if !entry.Binary {
		entry.Imports, entry.Symbols = extractGenericFacts(entry.Language, data)
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
	directory := filepath.Join(root, filepath.Dir(indexRelativePath))
	if err := os.MkdirAll(directory, 0700); err != nil {
		return fmt.Errorf("create index directory: %w", err)
	}
	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return fmt.Errorf("encode index: %w", err)
	}
	temp, err := os.CreateTemp(directory, ".index-*.tmp")
	if err != nil {
		return fmt.Errorf("create index temp file: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return fmt.Errorf("write index temp file: %w", err)
	}
	if err := temp.Chmod(0600); err != nil {
		temp.Close()
		return fmt.Errorf("set index permissions: %w", err)
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return fmt.Errorf("sync index temp file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close index temp file: %w", err)
	}
	if err := os.Rename(tempPath, filepath.Join(root, indexRelativePath)); err != nil {
		return fmt.Errorf("replace index: %w", err)
	}
	return nil
}
