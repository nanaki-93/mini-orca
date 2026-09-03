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
	canonical, err := CanonicalRoot(root)
	if err != nil {
		return "", manifest, err
	}
	policy, err := NewContextPolicy(canonical)
	if err != nil {
		return "", manifest, err
	}
	allFiles, err := WalkProjectFiles(context.Background(), ProjectWalkOptions{
		Root: canonical, IgnoredDirectories: ignoredProjectDirectories, IncludeSymlinkFiles: true, MaxFiles: maxProjectFiles,
	})
	if err != nil {
		return "", manifest, err
	}
	files := make([]string, 0, len(allFiles))
	for _, file := range allFiles {
		decision := policy.Decide(file)
		if decision.Include {
			files = append(files, file)
		} else {
			manifest.Excluded = append(manifest.Excluded, decision)
		}
	}
	includedIndex := make(map[string]int, len(files))
	for _, file := range files {
		entry := ContextFile{Path: file, Tokens: estimateTokens(file)}
		if fullPath, resolveErr := ResolveFile(canonical, file); resolveErr == nil {
			if info, statErr := os.Stat(fullPath); statErr == nil {
				entry.SizeBytes = info.Size()
			}
		}
		includedIndex[file] = len(manifest.Included)
		manifest.Included = append(manifest.Included, entry)
	}
	if target != "" {
		if _, err := ResolveFile(canonical, target); err != nil {
			return "", manifest, fmt.Errorf("invalid target file: %w", err)
		}
	}

	var result strings.Builder
	result.WriteString("## Complete project file inventory\n")
	for _, file := range files {
		result.WriteString("- " + file + "\n")
	}
	result.WriteString("\n")

	ordered := prioritize(files, filepath.ToSlash(target))
	for _, relative := range ordered {
		if result.Len() >= maxContextBytes {
			manifest.Truncated = true
			break
		}
		fullPath, err := ResolveFile(canonical, relative)
		if err != nil {
			continue
		}
		if !contextCandidate(relative) {
			continue
		}
		limit := int64(maxSnippetBytes)
		if relative == filepath.ToSlash(target) {
			limit = maxTargetBytes
		}
		data, err := readLimited(fullPath, limit)
		if err != nil || isBinary(data) {
			continue
		}
		remaining := maxContextBytes - result.Len()
		if remaining <= 0 || estimateTokens(result.String()) >= maxContextTokens {
			manifest.Truncated = true
			break
		}
		originalSize := len(data)
		if len(data) > remaining {
			data = data[:remaining]
		}
		availableTokens := maxContextTokens - estimateTokens(result.String())
		if tokens := estimateTokens(string(data)); tokens > availableTokens {
			data = data[:min(len(data), availableTokens*4)]
		}
		if len(data) < originalSize {
			manifest.Truncated = true
		}
		result.WriteString("## File: " + relative + "\n```\n")
		result.Write(data)
		result.WriteString("\n```\n\n")
		hash := sha256.Sum256(data)
		entry := ContextFile{Path: relative, SizeBytes: int64(len(data)), Hash: "sha256:" + hex.EncodeToString(hash[:]), Tokens: estimateTokens(string(data)), Truncated: len(data) < originalSize}
		if index, found := includedIndex[relative]; found {
			manifest.Included[index] = entry
		}
	}
	manifest.EstimatedTokens = estimateTokens(result.String())
	return result.String(), manifest, nil
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
