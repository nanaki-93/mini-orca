package symbols

import (
	"regexp"
	"strings"
)

// ExtractGenericFacts extracts approximate imports for non-Go languages.
func ExtractGenericFacts(language string, source []byte) []string {
	imports := []string{}
	lines := strings.Split(string(source), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if genericImport(language, trimmed) {
			imports = append(imports, trimmed)
		}
	}
	return imports
}

func genericImport(language, line string) bool {
	switch language {
	case "Kotlin", "Java", "TypeScript", "JavaScript", "Swift", "Scala", "Dart":
		return strings.HasPrefix(line, "import ")
	case "Python":
		return strings.HasPrefix(line, "import ") || strings.HasPrefix(line, "from ")
	case "Rust":
		return strings.HasPrefix(line, "use ")
	case "C", "C++":
		return strings.HasPrefix(line, "#include ")
	case "C#":
		return strings.HasPrefix(line, "using ")
	case "Ruby":
		return strings.HasPrefix(line, "require ") || strings.HasPrefix(line, "require_relative ")
	case "PHP":
		return strings.HasPrefix(line, "use ") || strings.HasPrefix(line, "require ") || strings.HasPrefix(line, "include ")
	case "Elixir":
		return strings.HasPrefix(line, "alias ") || strings.HasPrefix(line, "import ") || strings.HasPrefix(line, "use ")
	default:
		return false
	}
}

// ExtractFileFacts returns the detected language and raw import lines for any source file.
func ExtractFileFacts(path string, source []byte) (string, []string) {
	lang := DetectLanguage(path)
	if lang == "Go" {
		imports, _ := ExtractGoFacts(path, source)
		return lang, imports
	}
	imports := ExtractGenericFacts(lang, source)
	return lang, imports
}

var quotedImportPattern = regexp.MustCompile(`['"<]([^'">]+)['">]`)

// NormalizeGenericImportPath extracts a slash-normalized module path from a raw
// import statement across TypeScript/JS, Python, Rust, JVM, C/C++, C#, Ruby, PHP, and Elixir.
func NormalizeGenericImportPath(raw string) string {
	trimmed := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(raw), ";"))
	if m := quotedImportPattern.FindStringSubmatch(trimmed); len(m) == 2 {
		return cleanRelativeImport(m[1])
	}
	for _, kw := range []string{"from ", "import ", "use ", "using ", "alias ", "require_relative ", "require "} {
		if idx := strings.Index(trimmed, kw); idx >= 0 {
			rest := strings.TrimSpace(trimmed[idx+len(kw):])
			if fields := strings.Fields(rest); len(fields) > 0 {
				token := strings.Trim(fields[0], "{};()")
				token = strings.ReplaceAll(token, "::", "/")
				token = strings.ReplaceAll(token, "\\", "/")
				if !strings.Contains(token, "/") && strings.Contains(token, ".") {
					token = strings.ReplaceAll(token, ".", "/")
				}
				return cleanRelativeImport(strings.ToLower(token))
			}
		}
	}
	return cleanRelativeImport(trimmed)
}

func cleanRelativeImport(spec string) string {
	slash := strings.ReplaceAll(strings.TrimSpace(spec), "\\", "/")
	for strings.HasPrefix(slash, "./") || strings.HasPrefix(slash, "../") {
		if strings.HasPrefix(slash, "./") {
			slash = strings.TrimPrefix(slash, "./")
		} else {
			slash = strings.TrimPrefix(slash, "../")
		}
	}
	return strings.Trim(slash, "/")
}
