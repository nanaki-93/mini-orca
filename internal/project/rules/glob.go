package rules

import (
	"path"
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
