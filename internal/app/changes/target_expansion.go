package changes

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

const (
	maxExpandedSessionTargets = 8
	maxAutoCapturedFileBytes  = 24 * 1024
)

// ResolveExpandedTargetPaths inspects failed check outputs and coupled local Go
// imports to discover uncaptured repository files that should be added to a
// change session's target scope for self-repair.
func ResolveExpandedTargetPaths(
	root string,
	existingPaths []string,
	existingContents map[string]string,
	failedOutputs []string,
	isTestPath func(string) bool,
) []string {
	if len(existingPaths) >= maxExpandedSessionTargets || len(failedOutputs) == 0 {
		return nil
	}
	policy, err := project.NewContextPolicy(root)
	if err != nil {
		return nil
	}
	state := newExpansionState(root, existingPaths, policy, isTestPath)
	modPath := ReadGoModulePath(root)
	searchDirs := collectSearchDirs(existingPaths, failedOutputs, modPath)
	for _, token := range extractCheckFileTokens(failedOutputs, modPath) {
		rel := resolveRepoCandidatePath(root, token, searchDirs, policy)
		state.addResolvedFileAndCompanion(rel)
		if state.full() {
			return state.added
		}
	}
	if len(existingPaths)+len(state.added) < 6 && modPath != "" {
		state.expandCoupledLocalGoImports(existingPaths, existingContents, modPath)
	}
	return state.added
}

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

type expansionState struct {
	root                 string
	policy               *project.ContextPolicy
	isTestPath           func(string) bool
	seen                 map[string]bool
	initialNonTestGoDirs map[string]bool
	initialCount         int
	added                []string
}

func newExpansionState(root string, existingPaths []string, policy *project.ContextPolicy, isTestPath func(string) bool) *expansionState {
	seen := make(map[string]bool, len(existingPaths))
	nonTestDirs := map[string]bool{}
	for _, p := range existingPaths {
		norm := filepath.ToSlash(p)
		seen[norm] = true
		if strings.HasSuffix(norm, ".go") && (isTestPath == nil || !isTestPath(norm)) {
			nonTestDirs[filepath.ToSlash(filepath.Dir(norm))] = true
		}
	}
	return &expansionState{
		root:                 root,
		policy:               policy,
		isTestPath:           isTestPath,
		seen:                 seen,
		initialNonTestGoDirs: nonTestDirs,
		initialCount:         len(existingPaths),
	}
}

func (s *expansionState) full() bool {
	return s.initialCount+len(s.added) >= maxExpandedSessionTargets
}

func (s *expansionState) appendPath(rel string) {
	if rel == "" || s.seen[rel] || s.full() || !isValidRepoSourceFile(s.root, rel, s.policy) {
		return
	}
	s.seen[rel] = true
	s.added = append(s.added, rel)
}

func (s *expansionState) addResolvedFileAndCompanion(rel string) {
	if rel == "" || s.seen[rel] || s.full() {
		return
	}
	if strings.HasSuffix(rel, "_test.go") {
		dir := filepath.ToSlash(filepath.Dir(rel))
		if !s.initialNonTestGoDirs[dir] {
			companion := strings.TrimSuffix(rel, "_test.go") + ".go"
			s.appendPath(companion)
		}
		s.appendPath(rel)
		return
	}
	s.appendPath(rel)
	if strings.HasSuffix(rel, ".go") {
		s.appendPath(strings.TrimSuffix(rel, ".go") + "_test.go")
	}
}

func (s *expansionState) expandCoupledLocalGoImports(existingPaths []string, existingContents map[string]string, modPath string) {
	var goContents []string
	for _, p := range existingPaths {
		if strings.HasSuffix(p, ".go") && (s.isTestPath == nil || !s.isTestPath(p)) {
			if content := existingContents[p]; content != "" {
				goContents = append(goContents, content)
			}
		}
	}
	for _, dir := range LocalImportedGoDirs(s.root, modPath, goContents) {
		impls, tests := listSmallPackageGoFiles(s.root, dir, s.policy)
		if len(impls) == 0 || len(impls) > 2 || len(tests) == 0 {
			continue
		}
		for _, impl := range impls {
			s.appendPath(impl)
		}
		for _, testFile := range tests {
			s.appendPath(testFile)
		}
	}
}

func listSmallPackageGoFiles(root, dir string, policy *project.ContextPolicy) ([]string, []string) {
	entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(dir)))
	if err != nil {
		return nil, nil
	}
	var impls, tests []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		rel := entry.Name()
		if dir != "." {
			rel = dir + "/" + entry.Name()
		}
		if !isValidRepoSourceFile(root, rel, policy) {
			continue
		}
		if strings.HasSuffix(entry.Name(), "_test.go") {
			tests = append(tests, rel)
		} else {
			impls = append(impls, rel)
		}
	}
	return impls, tests
}

func collectSearchDirs(existingPaths, failedOutputs []string, modPath string) []string {
	var dirs []string
	seen := map[string]bool{}
	addDir := func(d string) {
		d = filepath.ToSlash(strings.TrimSpace(d))
		if d != "" && !seen[d] {
			seen[d] = true
			dirs = append(dirs, d)
		}
	}
	for _, p := range existingPaths {
		addDir(filepath.Dir(p))
	}
	if modPath == "" {
		return dirs
	}
	prefix := modPath + "/"
	for _, out := range failedOutputs {
		for _, token := range strings.Fields(out) {
			if idx := strings.Index(token, prefix); idx >= 0 {
				sub := strings.Trim(token[idx+len(prefix):], `":;(),[]`)
				addDir(sub)
			}
		}
	}
	return dirs
}

func extractCheckFileTokens(failedOutputs []string, modPath string) []string {
	var out []string
	seen := map[string]bool{}
	for _, text := range failedOutputs {
		for _, field := range strings.Fields(text) {
			cand := parseFileTokenWithLine(field, modPath)
			if cand != "" && !seen[cand] {
				seen[cand] = true
				out = append(out, cand)
			}
		}
	}
	return out
}

func parseFileTokenWithLine(field, modPath string) string {
	trimmed := strings.Trim(field, `"'(),[]{}<>`)
	idx := strings.IndexByte(trimmed, ':')
	if idx <= 0 || idx+1 >= len(trimmed) || trimmed[idx+1] < '0' || trimmed[idx+1] > '9' {
		return ""
	}
	cand := filepath.ToSlash(strings.TrimPrefix(trimmed[:idx], "./"))
	if modPath != "" && strings.HasPrefix(cand, modPath+"/") {
		cand = strings.TrimPrefix(cand, modPath+"/")
	}
	if !hasSourceFileExtension(cand) {
		return ""
	}
	return cand
}

func hasSourceFileExtension(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go", ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".py", ".rs", ".rb", ".java", ".kt", ".c", ".cc", ".cpp", ".cs", ".swift", ".php":
		return true
	default:
		return false
	}
}

func resolveRepoCandidatePath(root, cand string, searchDirs []string, policy *project.ContextPolicy) string {
	if strings.Contains(cand, "/") {
		if resolved := resolveSlashedCandidate(root, cand, policy); resolved != "" {
			return resolved
		}
		cand = filepath.Base(cand)
	}
	for _, dir := range searchDirs {
		rel := cand
		if dir != "." && dir != "" {
			rel = dir + "/" + cand
		}
		if isValidRepoSourceFile(root, rel, policy) {
			return rel
		}
	}
	return findSourceFileByBasename(root, cand, policy)
}

func resolveSlashedCandidate(root, cand string, policy *project.ContextPolicy) string {
	cleaned := strings.TrimPrefix(filepath.ToSlash(cand), "/")
	for sub := cleaned; sub != ""; {
		if isValidRepoSourceFile(root, sub, policy) {
			return sub
		}
		slash := strings.IndexByte(sub, '/')
		if slash < 0 {
			break
		}
		sub = sub[slash+1:]
	}
	return ""
}

func findSourceFileByBasename(root, baseName string, policy *project.ContextPolicy) string {
	var match string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, ".") && path != root {
				return filepath.SkipDir
			}
			if name == "vendor" || name == "node_modules" || name == "build" || name == "dist" {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() != baseName {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if isValidRepoSourceFile(root, rel, policy) {
			match = rel
			return fs.SkipAll
		}
		return nil
	})
	return match
}

func isValidRepoSourceFile(root, rel string, policy *project.ContextPolicy) bool {
	if rel == "" || strings.HasPrefix(rel, "../") || strings.Contains(rel, "/../") {
		return false
	}
	if policy != nil && !policy.Decide(rel).Include {
		return false
	}
	full, err := project.ResolveWritePath(root, rel)
	if err != nil {
		return false
	}
	info, err := os.Stat(full)
	if err != nil || info.IsDir() || info.Size() > maxAutoCapturedFileBytes {
		return false
	}
	return true
}
