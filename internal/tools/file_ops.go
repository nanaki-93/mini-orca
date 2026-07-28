package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// fileOpsExecutor implements file operations for the Executor interface.
type fileOpsExecutor struct {
	shell Executor
}

// FileUpdate represents a single update operation to be applied to a file.
type FileUpdate struct {
	// Type is the type of update: "append_function", "append_struct", "append_class", or "replace".
	Type string
	// Name is the name of the entity (function, struct, or class name).
	Name string
	// Content is the content to add or replace.
	Content string
}

// NewFileOpsExecutor creates a new file operations executor instance.
func NewFileOpsExecutor() Executor {
	return &fileOpsExecutor{
		shell: NewSafeShellExecutor(),
	}
}

// ReadFile reads the content of a file at the given path.
func (f *fileOpsExecutor) ReadFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("file_ops: failed to read file %s: %w", path, err)
	}
	return data, nil
}

// ReadFileAsString reads the content of a file at the given path and returns it as a string.
func (f *fileOpsExecutor) ReadFileAsString(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("file_ops: failed to read file %s: %w", path, err)
	}
	return string(data), nil
}

// WriteFile writes content to a file at the given path with the specified permissions.
func (f *fileOpsExecutor) WriteFile(path string, data []byte, perm uint32) error {
	err := os.WriteFile(path, data, os.FileMode(perm))
	if err != nil {
		return fmt.Errorf("file_ops: failed to write file %s: %w", path, err)
	}
	return nil
}

// Shell executes a shell command and returns its output.
func (f *fileOpsExecutor) Shell(ctx context.Context, command string, args ...string) (*ShellResult, error) {
	return f.shell.Shell(ctx, command, args...)
}

// DetectProjectType detects the type of project at the given directory.
func (f *fileOpsExecutor) DetectProjectType(dirPath string) (*ProjectInfo, error) {
	return f.shell.DetectProjectType(dirPath)
}

// Format formats code content using the appropriate formatter.
func (f *fileOpsExecutor) Format(ctx context.Context, content []byte, language string) ([]byte, error) {
	return f.shell.Format(ctx, content, language)
}

// FormatCode formats the code file at the given path.
func (f *fileOpsExecutor) FormatCode(path string) error {
	return f.shell.FormatCode(path)
}

// WriteFileAtomic writes content to a file atomically using temp file + rename.
func (f *fileOpsExecutor) WriteFileAtomic(path string, content string) error {
	// Create parent directories if needed
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("file_ops: failed to create directory %s: %w", dir, err)
	}

	// Create temp file in the same directory (same filesystem for atomic rename)
	tempFile, err := os.CreateTemp(dir, ".tmp-"+filepath.Base(path)+"-*")
	if err != nil {
		return fmt.Errorf("file_ops: failed to create temp file: %w", err)
	}
	tempPath := tempFile.Name()

	// Clean up temp file on error
	defer func() {
		if err != nil {
			os.Remove(tempPath)
		}
	}()

	// Write content to temp file
	if _, err := tempFile.WriteString(content); err != nil {
		tempFile.Close()
		return fmt.Errorf("file_ops: failed to write to temp file: %w", err)
	}

	// Close temp file before rename
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("file_ops: failed to close temp file: %w", err)
	}

	// Atomically rename temp file to target
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("file_ops: failed to rename temp file to %s: %w", path, err)
	}

	return nil
}

// AppendFunctionToFile appends a function to a file, checking for duplicates first.
func (f *fileOpsExecutor) AppendFunctionToFile(path string, functionName string, functionBody string) error {
	// Read existing file content
	existing, err := f.ReadFileAsString(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("file_ops: failed to read file %s: %w", path, err)
	}

	// Check if function already exists
	if f.containsFunction(existing, functionName) {
		return fmt.Errorf("file_ops: function %s already exists in file %s", functionName, path)
	}

	// Append function with proper formatting
	var newContent string
	if existing != "" {
		newContent = existing + "\n\n"
	}
	newContent += functionBody

	// Write back atomically
	return f.WriteFileAtomic(path, newContent)
}

// containsFunction checks if a function with the given name already exists in the content.
func (f *fileOpsExecutor) containsFunction(content string, functionName string) bool {
	// Check for function declaration patterns
	patterns := []string{
		"func " + functionName + "(",
		"func " + functionName + " ",
		"func (" + functionName + " ",
		"def " + functionName + "(",
		"public " + functionName + "(",
		"private " + functionName + "(",
		"protected " + functionName + "(",
	}

	for _, pattern := range patterns {
		if strings.Contains(content, pattern) {
			return true
		}
	}
	return false
}

// WriteStructToFile writes a struct definition to a file, checking for duplicates first.
// Inserts the struct after imports (Go) or at the end of the file.
func (f *fileOpsExecutor) WriteStructToFile(path string, structName string, structBody string) error {
	existing, err := f.ReadFileAsString(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("file_ops: failed to read file %s: %w", path, err)
	}

	if f.containsStruct(existing, structName) {
		return fmt.Errorf("file_ops: struct %s already exists in file %s", structName, path)
	}

	newContent := f.insertStruct(existing, structBody)
	return f.WriteFileAtomic(path, newContent)
}

// WriteClassToFile writes a class definition to a file, checking for duplicates first.
// Inserts the class at an appropriate location.
func (f *fileOpsExecutor) WriteClassToFile(path string, className string, classBody string) error {
	existing, err := f.ReadFileAsString(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("file_ops: failed to read file %s: %w", path, err)
	}

	if f.containsClass(existing, className) {
		return fmt.Errorf("file_ops: class %s already exists in file %s", className, path)
	}

	newContent := f.insertClass(existing, classBody)
	return f.WriteFileAtomic(path, newContent)
}

// containsStruct checks if a struct with the given name already exists in the content.
func (f *fileOpsExecutor) containsStruct(content string, structName string) bool {
	patterns := []string{
		"type " + structName + " struct",
		"type " + structName + " ",
		"struct " + structName,
		"class " + structName + " {",
		"class " + structName + "(",
	}

	for _, pattern := range patterns {
		if strings.Contains(content, pattern) {
			return true
		}
	}
	return false
}

// containsClass checks if a class with the given name already exists in the content.
func (f *fileOpsExecutor) containsClass(content string, className string) bool {
	patterns := []string{
		"class " + className + " {",
		"class " + className + "(",
		"class " + className + " extends",
		"class " + className + " implements",
		"public class " + className,
		"private class " + className,
		"protected class " + className,
		"interface " + className,
	}

	for _, pattern := range patterns {
		if strings.Contains(content, pattern) {
			return true
		}
	}
	return false
}

// insertStruct inserts a struct definition at an appropriate location in the file.
func (f *fileOpsExecutor) insertStruct(existingContent string, structBody string) string {
	// For Go files, insert after imports block
	if strings.Contains(existingContent, "import (") {
		importEnd := strings.Index(existingContent, ")")
		if importEnd != -1 {
			importEnd = strings.Index(existingContent[importEnd:], "\n")
			if importEnd != -1 {
				importEnd += importEnd + 1
				return existingContent[:importEnd] + "\n\n" + structBody + existingContent[importEnd:]
			}
		}
	}

	// For Python files, insert after imports
	if strings.Contains(existingContent, "import ") {
		lines := strings.Split(existingContent, "\n")
		insertIdx := len(lines)
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "import ") || strings.HasPrefix(trimmed, "from ") {
				insertIdx = i + 1
			} else {
				break
			}
		}
		lines = append(lines[:insertIdx], append([]string{"", structBody}, lines[insertIdx:]...)...)
		return strings.Join(lines, "\n")
	}

	// Default: append to end
	if existingContent != "" {
		return existingContent + "\n\n" + structBody
	}
	return structBody
}

// insertClass inserts a class definition at an appropriate location in the file.
func (f *fileOpsExecutor) insertClass(existingContent string, classBody string) string {
	// For Java/C#/Kotlin files, insert after package/import declarations
	if strings.Contains(existingContent, "package ") || strings.Contains(existingContent, "import ") {
		lines := strings.Split(existingContent, "\n")
		insertIdx := len(lines)
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "package ") || strings.HasPrefix(trimmed, "import ") {
				insertIdx = i + 1
			} else {
				break
			}
		}
		lines = append(lines[:insertIdx], append([]string{"", classBody}, lines[insertIdx:]...)...)
		return strings.Join(lines, "\n")
	}

	// Default: append to end
	if existingContent != "" {
		return existingContent + "\n\n" + classBody
	}
	return classBody
}

// UpdateFile applies multiple file updates in order with a single atomic write at the end.
func (f *fileOpsExecutor) UpdateFile(path string, updates []FileUpdate) error {
	// Read existing file content
	content, err := f.ReadFileAsString(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("file_ops: failed to read file %s: %w", path, err)
	}

	// Apply all updates in order
	for i, update := range updates {
		content, err = f.applyUpdate(content, update)
		if err != nil {
			return fmt.Errorf("file_ops: failed to apply update %d: %w", i, err)
		}
	}

	// Single atomic write at the end
	return f.WriteFileAtomic(path, content)
}

// applyUpdate applies a single FileUpdate to the content.
func (f *fileOpsExecutor) applyUpdate(content string, update FileUpdate) (string, error) {
	switch update.Type {
	case "append_function":
		if f.containsFunction(content, update.Name) {
			return content, nil
		}
		if content != "" {
			return content + "\n\n" + update.Content, nil
		}
		return update.Content, nil
	case "append_struct":
		if f.containsStruct(content, update.Name) {
			return content, nil
		}
		return f.insertStruct(content, update.Content), nil
	case "append_class":
		if f.containsClass(content, update.Name) {
			return content, nil
		}
		return f.insertClass(content, update.Content), nil
	case "replace":
		return update.Content, nil
	default:
		return content, fmt.Errorf("file_ops: unknown update type: %s", update.Type)
	}
}

// _ ensures NewSafeShellExecutor is used.
var _ = NewSafeShellExecutor
