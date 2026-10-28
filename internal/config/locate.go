package config

import (
	"os"
	"path/filepath"
	"slices"
)

// ConfigEnv names an explicit configuration file; it replaces the search.
const ConfigEnv = "MINI_ORCA_CONFIG"

// Candidates lists, in order, the configuration files a daemon started in
// workDir from executable reads: MINI_ORCA_CONFIG alone when set, otherwise
// the project's config.yaml, the user's ~/.config/mini-orca/config.yaml, the
// platform configuration directory's mini-orca/config.yaml, and the
// config.yaml beside the executable (its symlinks resolved).
func Candidates(workDir, executable string) []string {
	if explicit := os.Getenv(ConfigEnv); explicit != "" {
		return []string{explicit}
	}
	list := []string{filepath.Join(workDir, "config.yaml")}
	if home, err := os.UserHomeDir(); err == nil {
		list = append(list, filepath.Join(home, ".config", "mini-orca", "config.yaml"))
	}
	if dir, err := os.UserConfigDir(); err == nil {
		list = append(list, filepath.Join(dir, "mini-orca", "config.yaml"))
	}
	if executable != "" {
		if resolved, err := filepath.EvalSymlinks(executable); err == nil {
			list = append(list, filepath.Join(filepath.Dir(resolved), "config.yaml"))
		}
		list = append(list, filepath.Join(filepath.Dir(executable), "config.yaml"))
	}
	unique := make([]string, 0, len(list))
	for _, path := range list {
		if path = filepath.Clean(path); !slices.Contains(unique, path) {
			unique = append(unique, path)
		}
	}
	return unique
}

// Find returns the first candidate that is a regular file, or "" when none
// is and the built-in local defaults apply. An explicit MINI_ORCA_CONFIG is
// returned even when missing, so loading it reports the error.
func Find(workDir, executable string) string {
	candidates := Candidates(workDir, executable)
	if os.Getenv(ConfigEnv) != "" {
		return candidates[0]
	}
	for _, path := range candidates {
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
			return path
		}
	}
	return ""
}
