package project

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	maxContextBytes = 512 * 1024
	maxSnippetBytes = 24 * 1024
	maxTargetBytes  = 192 * 1024
)

// ContextBuilder creates bounded project-wide context for analysis and generation.
type ContextBuilder struct{}

func NewContextBuilder() *ContextBuilder { return &ContextBuilder{} }

func (b *ContextBuilder) Build(root, target string) (string, error) {
	canonical, err := CanonicalRoot(root)
	if err != nil {
		return "", err
	}
	files, err := listFiles(canonical)
	if err != nil {
		return "", err
	}
	if target != "" {
		if _, err := ResolveFile(canonical, target); err != nil {
			return "", fmt.Errorf("invalid target file: %w", err)
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
			break
		}
		fullPath := filepath.Join(canonical, filepath.FromSlash(relative))
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
		if remaining <= 0 {
			break
		}
		if len(data) > remaining {
			data = data[:remaining]
		}
		result.WriteString("## File: " + relative + "\n```\n")
		result.Write(data)
		result.WriteString("\n```\n\n")
	}
	return result.String(), nil
}

func listFiles(root string) ([]string, error) {
	files := make([]string, 0)
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
		if len(files) >= maxProjectFiles {
			return fmt.Errorf("project contains more than %d files", maxProjectFiles)
		}
		relative, err := filepath.Rel(root, path)
		if err == nil {
			files = append(files, filepath.ToSlash(relative))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

func prioritize(files []string, target string) []string {
	ordered := make([]string, 0, len(files)+1)
	seen := make(map[string]bool, len(files))
	for _, file := range []string{target, analysisRelativePath, "README.md", "go.mod", "build.gradle.kts", "build.gradle", "package.json", "Cargo.toml", "pyproject.toml"} {
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
