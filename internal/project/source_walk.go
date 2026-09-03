package project

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

// ProjectWalkOptions makes the source-traversal boundary explicit without
// exposing filesystem callbacks to project callers. Root is canonicalized,
// ignored directories are skipped, a nil extension set allows every extension,
// and symlinked directories are never followed. A nil policy includes every
// traversed file; a policy limits results to prompt/index-eligible files.
type ProjectWalkOptions struct {
	Root                string
	Policy              *ContextPolicy
	IgnoredDirectories  map[string]bool
	AllowedExtensions   map[string]bool
	IncludeSymlinkFiles bool
	MaxFiles            int
}

// WalkProjectFiles returns project-relative paths in stable order. It never
// follows symlinked directories. Callers that need source-policy eligibility
// pass their ContextPolicy; context inspection passes nil so it can report
// policy exclusions separately.
func WalkProjectFiles(ctx context.Context, options ProjectWalkOptions) ([]string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if options.MaxFiles <= 0 {
		return nil, fmt.Errorf("maximum project files must be positive")
	}
	root, err := CanonicalRoot(options.Root)
	if err != nil {
		return nil, err
	}
	ignored := options.IgnoredDirectories
	if ignored == nil {
		ignored = ignoredProjectDirectories
	}

	files := make([]string, 0)
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			// Preserve the previous scan behavior: inaccessible descendants are
			// omitted so callers receive the usable deterministic subset.
			return nil
		}
		if path == root {
			return nil
		}
		if entry.IsDir() {
			if ignored[entry.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		relative = filepath.ToSlash(relative)
		if !allowedSourceExtension(relative, options.AllowedExtensions) {
			return nil
		}
		if len(files) >= options.MaxFiles {
			return fmt.Errorf("project contains more than %d files", options.MaxFiles)
		}
		if entry.Type()&fs.ModeSymlink != 0 && !options.IncludeSymlinkFiles {
			return nil
		}
		if options.Policy != nil && !options.Policy.Decide(relative).Include {
			return nil
		}
		files = append(files, relative)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

func allowedSourceExtension(path string, allowed map[string]bool) bool {
	if len(allowed) == 0 {
		return true
	}
	return allowed[strings.ToLower(filepath.Ext(path))]
}
