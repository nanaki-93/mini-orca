package handlers

import (
	"fmt"
	"strings"
	"unicode"
)

// ─── String Utility Functions ─────────────────────────────────────────────────

// lower converts a string to lowercase.
func lower(s string) string {
	return strings.ToLower(s)
}

// title capitalizes the first letter of a string.
func title(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

// fileExtension extracts the file extension from a filename (without the dot).
func fileExtension(name string) string {
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] == '.' {
			return name[i+1:]
		}
	}
	return ""
}

// fileIcon returns the icon name for a given file extension.
func fileIcon(ext string) string {
	switch ext {
	case "ts", "tsx":
		return "typescript"
	case "js", "jsx":
		return "javascript"
	case "go":
		return "go"
	case "py":
		return "python"
	case "rs":
		return "rust"
	case "java":
		return "java"
	case "kt":
		return "kotlin"
	case "html":
		return "html"
	case "css":
		return "css"
	case "md":
		return "markdown"
	case "yaml", "yml":
		return "yaml"
	case "json":
		return "json"
	case "sh", "bash":
		return "shell"
	case "gitignore", "gitattributes":
		return "git"
	default:
		return "file"
	}
}

// ─── Template Utility Functions ───────────────────────────────────────────────

func dict(values ...interface{}) (map[string]interface{}, error) {
	if len(values)%2 != 0 {
		return nil, fmt.Errorf("dict: invalid number of values")
	}
	dict := make(map[string]interface{}, len(values)/2)
	for i := 0; i < len(values); i += 2 {
		key, ok := values[i].(string)
		if !ok {
			return nil, fmt.Errorf("dict: keys must be strings")
		}
		dict[key] = values[i+1]
	}
	return dict, nil
}

func defFunc(val, fallback interface{}) interface{} {
	if val == nil {
		return fallback
	}
	return val
}
