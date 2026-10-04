package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// ResolveWritePath admits a canonical project-relative spelling and rejects
// symlink components even when their destination is inside the project. Missing
// components are allowed for a new file; this function never creates them.
func ResolveWritePath(root, relative string) (string, error) {
	canonical, err := CanonicalRoot(root)
	if err != nil {
		return "", err
	}
	if err := validateWritePath(relative); err != nil {
		return "", err
	}
	current := canonical
	parts := strings.Split(relative, "/")
	for index, part := range parts {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("inspect write path: %w", err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("write path contains a symlink")
		}
		if index < len(parts)-1 && !info.IsDir() || index == len(parts)-1 && !info.Mode().IsRegular() {
			return "", fmt.Errorf("write path is not a regular file path")
		}
	}
	return current, nil
}

func validateWritePath(relative string) error {
	if relative == "" || len(relative) > 512 || strings.TrimSpace(relative) != relative || filepath.IsAbs(relative) || strings.Contains(relative, "\\") || filepath.ToSlash(filepath.Clean(relative)) != relative || relative == "." {
		return fmt.Errorf("provide a canonical project-relative file path")
	}
	if strings.IndexFunc(relative, unicode.IsControl) >= 0 {
		return fmt.Errorf("file path contains control characters")
	}
	for _, part := range strings.Split(relative, "/") {
		switch part {
		case "..", ".git", ".mini-orca", ".aws", ".codex", ".agents":
			return fmt.Errorf("file path targets protected metadata")
		}
	}
	return nil
}
