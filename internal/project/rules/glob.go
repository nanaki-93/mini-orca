package rules

import (
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

// compileGlob turns a slash-separated glob into a case-insensitive regular
// expression: * matches within one path segment, ** matches any number of
// segments and ? matches one character. A glob without a slash matches the
// file name in any directory, as in .gitignore. The Web UI mirrors these
// semantics in web/src/lib/rules.ts.
func compileGlob(glob string) (*regexp.Regexp, bool) {
	glob = strings.TrimPrefix(strings.TrimSpace(glob), "./")
	if glob == "" || strings.HasPrefix(glob, "/") || strings.Contains(glob, "\\") {
		return nil, false
	}
	if !strings.Contains(glob, "/") {
		glob = "**/" + glob
	}
	var pattern strings.Builder
	pattern.WriteString("(?i)^")
	for i := 0; i < len(glob); i++ {
		switch {
		case strings.HasPrefix(glob[i:], "**/"):
			pattern.WriteString("(?:.*/)?")
			i += 2
		case strings.HasPrefix(glob[i:], "**"):
			pattern.WriteString(".*")
			i++
		case glob[i] == '*':
			pattern.WriteString("[^/]*")
		case glob[i] == '?':
			pattern.WriteString("[^/]")
		default:
			pattern.WriteString(regexp.QuoteMeta(glob[i : i+1]))
		}
	}
	pattern.WriteString("$")
	compiled, err := regexp.Compile(pattern.String())
	return compiled, err == nil
}

// joinGlob makes a glob declared in dir's AGENTS.md project-relative.
func joinGlob(dir, glob string) string {
	glob = strings.TrimPrefix(strings.TrimSpace(glob), "./")
	if dir == "" {
		return glob
	}
	if !strings.Contains(glob, "/") {
		glob = "**/" + glob
	}
	return path.Join(dir, glob)
}

// conventionalTestFiles are the test file names and folders of common
// ecosystems. A project adds its own with test_files in AGENTS.md.
var conventionalTestFiles = []string{
	"*_test.go",
	"*_test.py", "test_*.py", "conftest.py",
	"*.test.ts", "*.spec.ts", "*.test.tsx", "*.spec.tsx", "*_test.ts",
	"*.test.js", "*.spec.js", "*.test.jsx", "*.spec.jsx", "*_test.js",
	"*.test.mjs", "*.spec.mjs", "*.test.cjs", "*.spec.cjs",
	"*test.kt", "*tests.kt", "*spec.kt", "*test.java", "*tests.java", "*test.scala", "*spec.scala",
	"*_test.rs", "*_test.rb", "*_spec.rb", "*test.php", "*test.cs", "*tests.cs",
	"*test.swift", "*tests.swift", "*_test.c", "*_test.cpp", "*_test.cc", "*_test.dart", "*_test.exs",
	"**/test/**", "**/tests/**", "**/__tests__/**", "**/spec/**", "**/specs/**",
}

// CompanionTest names where a test for a source file conventionally lives,
// preferring one that exists under root, or "" when the language keeps
// tests elsewhere (in the same file, or with no single place).
func CompanionTest(root, file string) string {
	file = strings.TrimPrefix(filepath.ToSlash(file), "./")
	dir, name := path.Split(file)
	ext := path.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	var candidates []string
	switch strings.ToLower(ext) {
	case ".go":
		candidates = []string{dir + stem + "_test.go"}
	case ".py":
		candidates = []string{dir + "test_" + stem + ".py", dir + "tests/test_" + stem + ".py", "tests/test_" + stem + ".py"}
	case ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs":
		candidates = []string{dir + stem + ".test" + ext, dir + stem + ".spec" + ext, dir + "__tests__/" + stem + ".test" + ext}
	case ".java", ".kt", ".scala":
		if strings.Contains(file, "src/main/") {
			candidates = []string{strings.Replace(dir, "src/main/", "src/test/", 1) + stem + "Test" + ext}
		}
	case ".rb":
		candidates = []string{"spec/" + strings.TrimPrefix(dir, "lib/") + stem + "_spec.rb", "test/" + strings.TrimPrefix(dir, "lib/") + stem + "_test.rb"}
	}
	if len(candidates) == 0 || IsConventionalTestFile(file) {
		return ""
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(filepath.Join(root, filepath.FromSlash(candidate))); err == nil && info.Mode().IsRegular() {
			return candidate
		}
	}
	return candidates[0]
}
