package changes

import (
	"os"
	"path/filepath"
	"strings"
)

// ReadGoModulePath reads the module path declared in root/go.mod, if present.
func ReadGoModulePath(root string) string {
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "module ") {
			fields := strings.Fields(trimmed)
			if len(fields) >= 2 {
				return strings.Trim(fields[1], `"`)
			}
		}
	}
	return ""
}

// LocalImportedGoDirs returns repository-relative directories of local Go
// packages imported by the given Go source contents.
func LocalImportedGoDirs(root, modPath string, goContents []string) []string {
	if modPath == "" {
		return nil
	}
	prefix := modPath + "/"
	var dirs []string
	seen := map[string]bool{}
	for _, content := range goContents {
		for _, line := range strings.Split(content, "\n") {
			sub := extractModuleSubpath(line, prefix)
			if sub == "" || seen[sub] {
				continue
			}
			info, err := os.Stat(filepath.Join(root, filepath.FromSlash(sub)))
			if err == nil && info.IsDir() {
				seen[sub] = true
				dirs = append(dirs, sub)
			}
		}
	}
	return dirs
}

func extractModuleSubpath(line, prefix string) string {
	idx := strings.Index(line, `"`+prefix)
	if idx < 0 {
		return ""
	}
	rest := line[idx+1+len(prefix):]
	end := strings.IndexByte(rest, '"')
	if end <= 0 {
		return ""
	}
	return filepath.ToSlash(strings.TrimSpace(rest[:end]))
}
