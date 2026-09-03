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
	if options.MaxFiles <= 0 {
		return nil, fmt.Errorf("maximum project files must be positive")
	}
	root, err := CanonicalRoot(options.Root)
	if err != nil {
		return nil, err
	}
	walker := projectFileWalker{ctx: walkContext(ctx), root: root, options: options, ignored: walkIgnoredDirectories(options)}
	err = filepath.WalkDir(root, walker.visit)
	if err != nil {
		return nil, err
	}
	sort.Strings(walker.files)
	return walker.files, nil
}

type projectFileWalker struct {
	ctx     context.Context
	root    string
	options ProjectWalkOptions
	ignored map[string]bool
	files   []string
}

func walkContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func walkIgnoredDirectories(options ProjectWalkOptions) map[string]bool {
	if options.IgnoredDirectories != nil {
		return options.IgnoredDirectories
	}
	return ignoredProjectDirectories
}

func (w *projectFileWalker) visit(path string, entry fs.DirEntry, walkErr error) error {
	if err := w.ctx.Err(); err != nil {
		return err
	}
	if walkErr != nil || path == w.root {
		return nil
	}
	if entry.IsDir() {
		return w.directoryVisitResult(entry)
	}
	return w.fileVisitResult(path, entry)
}

func (w *projectFileWalker) directoryVisitResult(entry fs.DirEntry) error {
	if w.ignored[entry.Name()] {
		return filepath.SkipDir
	}
	return nil
}

func (w *projectFileWalker) fileVisitResult(path string, entry fs.DirEntry) error {
	relative, err := filepath.Rel(w.root, path)
	if err != nil {
		return nil
	}
	relative = filepath.ToSlash(relative)
	if !w.includeFile(relative, entry) {
		return nil
	}
	if len(w.files) >= w.options.MaxFiles {
		return fmt.Errorf("project contains more than %d files", w.options.MaxFiles)
	}
	w.files = append(w.files, relative)
	return nil
}

func (w *projectFileWalker) includeFile(relative string, entry fs.DirEntry) bool {
	if !allowedSourceExtension(relative, w.options.AllowedExtensions) {
		return false
	}
	if entry.Type()&fs.ModeSymlink != 0 && !w.options.IncludeSymlinkFiles {
		return false
	}
	return w.options.Policy == nil || w.options.Policy.Decide(relative).Include
}

func allowedSourceExtension(path string, allowed map[string]bool) bool {
	if len(allowed) == 0 {
		return true
	}
	return allowed[strings.ToLower(filepath.Ext(path))]
}
