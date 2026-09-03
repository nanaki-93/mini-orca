package project

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	maxContextBytes  = 512 * 1024
	maxSnippetBytes  = 24 * 1024
	maxTargetBytes   = 192 * 1024
	maxContextTokens = 12000
)

// ContextBuilder creates bounded project-wide context for analysis and generation.
type ContextBuilder struct{}

// ContextManifest describes the prompt material without returning source text.
type ContextManifest struct {
	Included        []ContextFile     `json:"included"`
	Excluded        []ContextDecision `json:"excluded"`
	EstimatedTokens int               `json:"estimated_tokens"`
	ByteLimit       int               `json:"byte_limit"`
	TokenLimit      int               `json:"token_limit"`
	Truncated       bool              `json:"truncated"`
	Scope           string            `json:"scope,omitempty"`
	Model           string            `json:"model,omitempty"`
	ProviderOrigin  string            `json:"provider_origin,omitempty"`
	RemoteProvider  bool              `json:"remote_provider,omitempty"`
}

type ContextFile struct {
	Path      string `json:"path"`
	SizeBytes int64  `json:"size_bytes"`
	Hash      string `json:"hash"`
	Tokens    int    `json:"estimated_tokens"`
	Truncated bool   `json:"truncated"`
}

func NewContextBuilder() *ContextBuilder { return &ContextBuilder{} }

func (b *ContextBuilder) Build(root, target string) (string, error) {
	contextText, _, err := b.BuildWithManifest(root, target)
	return contextText, err
}

// BuildWithManifest returns the exact prompt context plus safe inspection metadata.
func (b *ContextBuilder) BuildWithManifest(root, target string) (string, ContextManifest, error) {
	manifest := ContextManifest{Included: []ContextFile{}, Excluded: []ContextDecision{}, ByteLimit: maxContextBytes, TokenLimit: maxContextTokens}
	canonical, files, excluded, err := contextFileInventory(root)
	if err != nil {
		return "", manifest, err
	}
	manifest.Excluded = excluded
	var includedIndex map[string]int
	manifest.Included, includedIndex = contextManifestFiles(canonical, files)
	if err := validateContextTarget(canonical, target); err != nil {
		return "", manifest, err
	}
	var result strings.Builder
	writeContextInventory(&result, files)
	b.appendContextSnippets(&result, canonical, target, files, includedIndex, &manifest)
	manifest.EstimatedTokens = estimateTokens(result.String())
	return result.String(), manifest, nil
}

func contextFileInventory(root string) (string, []string, []ContextDecision, error) {
	canonical, err := CanonicalRoot(root)
	if err != nil {
		return "", nil, nil, err
	}
	policy, err := NewContextPolicy(canonical)
	if err != nil {
		return "", nil, nil, err
	}
	allFiles, err := WalkProjectFiles(context.Background(), ProjectWalkOptions{Root: canonical, IgnoredDirectories: ignoredProjectDirectories, IncludeSymlinkFiles: true, MaxFiles: maxProjectFiles})
	if err != nil {
		return "", nil, nil, err
	}
	files, excluded := make([]string, 0, len(allFiles)), make([]ContextDecision, 0)
	for _, file := range allFiles {
		decision := policy.Decide(file)
		if decision.Include {
			files = append(files, file)
		} else {
			excluded = append(excluded, decision)
		}
	}
	return canonical, files, excluded, nil
}

func contextManifestFiles(root string, files []string) ([]ContextFile, map[string]int) {
	included, includedIndex := make([]ContextFile, 0, len(files)), make(map[string]int, len(files))
	for _, file := range files {
		includedIndex[file] = len(included)
		included = append(included, contextManifestFile(root, file))
	}
	return included, includedIndex
}

func contextManifestFile(root, file string) ContextFile {
	entry := ContextFile{Path: file, Tokens: estimateTokens(file)}
	if fullPath, resolveErr := ResolveFile(root, file); resolveErr == nil {
		if info, statErr := os.Stat(fullPath); statErr == nil {
			entry.SizeBytes = info.Size()
		}
	}
	return entry
}

func validateContextTarget(root, target string) error {
	if target == "" {
		return nil
	}
	if _, err := ResolveFile(root, target); err != nil {
		return fmt.Errorf("invalid target file: %w", err)
	}
	return nil
}

func writeContextInventory(result *strings.Builder, files []string) {
	result.WriteString("## Complete project file inventory\n")
	for _, file := range files {
		result.WriteString("- " + file + "\n")
	}
	result.WriteString("\n")
}

func (b *ContextBuilder) appendContextSnippets(result *strings.Builder, root, target string, files []string, includedIndex map[string]int, manifest *ContextManifest) {
	for _, relative := range prioritize(files, filepath.ToSlash(target)) {
		if result.Len() >= maxContextBytes {
			manifest.Truncated = true
			return
		}
		if b.appendContextSnippet(result, root, target, relative, includedIndex, manifest) {
			return
		}
	}
}

// appendContextSnippet reports whether a context boundary requires traversal to stop.
func (b *ContextBuilder) appendContextSnippet(result *strings.Builder, root, target, relative string, includedIndex map[string]int, manifest *ContextManifest) bool {
	data, originalSize, ok := contextSnippet(root, target, relative)
	if !ok {
		return false
	}
	remaining, availableTokens := maxContextBytes-result.Len(), maxContextTokens-estimateTokens(result.String())
	if remaining <= 0 || availableTokens <= 0 {
		manifest.Truncated = true
		return true
	}
	data = boundedContextData(data, remaining, availableTokens)
	if len(data) < originalSize {
		manifest.Truncated = true
	}
	result.WriteString("## File: " + relative + "\n```\n")
	result.Write(data)
	result.WriteString("\n```\n\n")
	b.updateManifestFile(relative, data, originalSize, includedIndex, manifest)
	return false
}

func contextSnippet(root, target, relative string) ([]byte, int, bool) {
	fullPath, err := ResolveFile(root, relative)
	if err != nil || !contextCandidate(relative) {
		return nil, 0, false
	}
	limit := int64(maxSnippetBytes)
	if relative == filepath.ToSlash(target) {
		limit = maxTargetBytes
	}
	data, err := readLimited(fullPath, limit)
	if err != nil || isBinary(data) {
		return nil, 0, false
	}
	return data, len(data), true
}

func boundedContextData(data []byte, remaining, availableTokens int) []byte {
	if len(data) > remaining {
		data = data[:remaining]
	}
	if estimateTokens(string(data)) > availableTokens {
		data = data[:min(len(data), availableTokens*4)]
	}
	return data
}

func (b *ContextBuilder) updateManifestFile(relative string, data []byte, originalSize int, includedIndex map[string]int, manifest *ContextManifest) {
	index, found := includedIndex[relative]
	if !found {
		return
	}
	hash := sha256.Sum256(data)
	manifest.Included[index] = ContextFile{Path: relative, SizeBytes: int64(len(data)), Hash: "sha256:" + hex.EncodeToString(hash[:]), Tokens: estimateTokens(string(data)), Truncated: len(data) < originalSize}
}

func estimateTokens(text string) int {
	if text == "" {
		return 0
	}
	return (len([]rune(text)) + 3) / 4
}

func min(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func prioritize(files []string, target string) []string {
	ordered := make([]string, 0, len(files)+1)
	seen := make(map[string]bool, len(files))
	for _, file := range []string{target, "README.md", "go.mod", "build.gradle.kts", "build.gradle", "package.json", "Cargo.toml", "pyproject.toml"} {
		if file != "" && !seen[file] {
			ordered = append(ordered, file)
			seen[file] = true
		}
	}
	for _, file := range files {
		if !seen[file] {
			ordered = append(ordered, file)
			seen[file] = true
		}
	}
	return ordered
}

func contextCandidate(path string) bool {
	extension := strings.ToLower(filepath.Ext(path))
	if languageByExtension[extension] != "" {
		return true
	}
	switch filepath.Base(path) {
	case "Dockerfile", "Makefile", "go.mod", "go.sum", "Cargo.toml", "package.json", "pyproject.toml", "requirements.txt", "build.gradle", "build.gradle.kts", "settings.gradle.kts":
		return true
	default:
		return false
	}
}
