// Package project contains active-project state, analysis, and safe file access.
package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// CanonicalRoot validates a project directory and resolves symlinks.
func CanonicalRoot(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("project path is required")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve project path: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", fmt.Errorf("resolve project symlinks: %w", err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("stat project path: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("project path is not a directory")
	}
	return filepath.Clean(resolved), nil
}

// ResolveFile safely resolves a relative path inside root, including symlinks.
func ResolveFile(root, relative string) (string, error) {
	if strings.TrimSpace(relative) == "" {
		return "", fmt.Errorf("file path is required")
	}
	if filepath.IsAbs(relative) {
		return "", fmt.Errorf("file path must be relative to the project")
	}
	root, err := CanonicalRoot(root)
	if err != nil {
		return "", err
	}
	candidate := filepath.Join(root, filepath.Clean(relative))
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", fmt.Errorf("resolve file path: %w", err)
	}
	rel, err := filepath.Rel(root, resolved)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("file path escapes project directory")
	}
	return resolved, nil
}
