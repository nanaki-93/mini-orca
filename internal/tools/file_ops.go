package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileOps provides atomic file operations for modifying source files.
type FileOps struct {
	baseDir string
}

// NewFileOps creates a new file operations handler.
func NewFileOps(baseDir string) *FileOps {
	return &FileOps{
		baseDir: baseDir,
	}
}

// ReadFile reads the entire content of a file.
func (f *FileOps) ReadFile(relPath string) (string, error) {
	fullPath := filepath.Join(f.baseDir, relPath)

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", relPath, err)
	}

	return string(data), nil
}

// WriteFile writes content to a file atomically (write to temp, then rename).
func (f *FileOps) WriteFile(relPath string, content string) error {
	fullPath := filepath.Join(f.baseDir, relPath)

	// Ensure directory exists
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Write to temporary file first
	tmpPath := fullPath + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, fullPath); err != nil {
		// Clean up temp file on failure
		os.Remove(tmpPath)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// AppendFunctionToFile appends a single function to a file.
// It adds the function before the last closing brace if the file has content.
func (f *FileOps) AppendFunctionToFile(relPath string, functionName string, functionCode string) error {
	existing, err := f.ReadFile(relPath)
	if err != nil {
		// File doesn't exist, create it
		return f.WriteFile(relPath, functionCode+"\n")
	}

	// Check if function already exists
	if strings.Contains(existing, "func "+functionName) {
		return fmt.Errorf("function %s already exists in %s", functionName, relPath)
	}

	// Append the function
	newContent := strings.TrimRight(existing, "\n") + "\n\n" + functionCode + "\n"

	return f.WriteFile(relPath, newContent)
}

// WriteStructToFile writes a struct definition to a file.
func (f *FileOps) WriteStructToFile(relPath string, structName string, structCode string) error {
	existing, err := f.ReadFile(relPath)
	if err != nil {
		// File doesn't exist, create it
		return f.WriteFile(relPath, structCode+"\n")
	}

	if strings.Contains(existing, "type "+structName+" struct") {
		return fmt.Errorf("struct %s already exists in %s", structName, relPath)
	}

	newContent := strings.TrimRight(existing, "\n") + "\n\n" + structCode + "\n"

	return f.WriteFile(relPath, newContent)
}

// WriteClassToFile writes a class definition to a file (Java/Kotlin/TypeScript).
func (f *FileOps) WriteClassToFile(relPath string, className string, classCode string) error {
	return f.WriteStructToFile(relPath, className, classCode)
}

// ReplaceFunction replaces an existing function in a file.
func (f *FileOps) ReplaceFunction(relPath string, functionName string, newCode string) error {
	content, err := f.ReadFile(relPath)
	if err != nil {
		return err
	}

	// Simple replacement - in production, use AST parsing
	prefix := "func " + functionName
	if !strings.Contains(content, prefix) {
		return fmt.Errorf("function %s not found in %s", functionName, relPath)
	}

	// Find the function start
	startIdx := strings.Index(content, prefix)
	if startIdx == -1 {
		return fmt.Errorf("function %s not found in %s", functionName, relPath)
	}

	// Find the end of the function (next function or end of file)
	// This is a simple implementation - production should use AST
	endIdx := len(content)
	for i := startIdx + len(prefix); i < len(content); i++ {
		if content[i] == '\n' && i+1 < len(content) {
			// Check if next line starts a new function/struct/interface
			rest := content[i+1:]
			if strings.HasPrefix(rest, "func ") || strings.HasPrefix(rest, "type ") ||
				strings.HasPrefix(rest, "interface ") || strings.HasPrefix(rest, "class ") {
				endIdx = i
				break
			}
		}
	}

	newContent := content[:startIdx] + newCode + content[endIdx:]

	return f.WriteFile(relPath, newContent)
}

// CreateDirectory creates a directory if it doesn't exist.
func (f *FileOps) CreateDirectory(relPath string) error {
	fullPath := filepath.Join(f.baseDir, relPath)
	return os.MkdirAll(fullPath, 0755)
}

// FileExists checks if a file exists.
func (f *FileOps) FileExists(relPath string) bool {
	fullPath := filepath.Join(f.baseDir, relPath)
	info, err := os.Stat(fullPath)
	if err != nil {
		return false
	}
	return !info.IsDir()
}
