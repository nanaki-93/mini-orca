package project

import (
	"regexp"
	"strings"
)

var genericDeclarationPatterns = map[string]*regexp.Regexp{
	"Kotlin":     regexp.MustCompile(`^\s*(?:public\s+|private\s+|internal\s+|protected\s+)?(?:data\s+|sealed\s+|abstract\s+|suspend\s+)?(class|interface|object|fun)\s+([A-Za-z_][A-Za-z0-9_]*)`),
	"Java":       regexp.MustCompile(`^\s*(?:public\s+|private\s+|protected\s+)?(?:static\s+|final\s+|abstract\s+)*(class|interface|enum|record|[A-Za-z_][A-Za-z0-9_<>, ?\[\]]*)\s+([A-Za-z_][A-Za-z0-9_]*)\s*(?:\(|\{)`),
	"TypeScript": regexp.MustCompile(`^\s*(?:export\s+)?(?:default\s+)?(?:async\s+)?(class|interface|type|function|const)\s+([A-Za-z_$][A-Za-z0-9_$]*)`),
	"JavaScript": regexp.MustCompile(`^\s*(?:export\s+)?(?:default\s+)?(?:async\s+)?(class|function|const)\s+([A-Za-z_$][A-Za-z0-9_$]*)`),
	"Python":     regexp.MustCompile(`^\s*(?:async\s+)?(class|def)\s+([A-Za-z_][A-Za-z0-9_]*)`),
	"Rust":       regexp.MustCompile(`^\s*(?:pub(?:\([^)]*\))?\s+)?(?:async\s+)?(struct|enum|trait|fn|impl)\s+([A-Za-z_][A-Za-z0-9_]*)`),
	"Ruby":       regexp.MustCompile(`^\s*(class|module|def)\s+([A-Za-z_][A-Za-z0-9_?!]*)`),
	"PHP":        regexp.MustCompile(`^\s*(?:public\s+|private\s+|protected\s+|static\s+|abstract\s+|final\s+)*(class|interface|trait|function)\s+([A-Za-z_][A-Za-z0-9_]*)`),
	"Swift":      regexp.MustCompile(`^\s*(?:public\s+|private\s+|internal\s+|fileprivate\s+|open\s+|final\s+)*(class|struct|enum|protocol|actor|func)\s+([A-Za-z_][A-Za-z0-9_]*)`),
	"C#":         regexp.MustCompile(`^\s*(?:public\s+|private\s+|protected\s+|internal\s+|static\s+|sealed\s+|abstract\s+|async\s+)*(class|interface|struct|enum|record)\s+([A-Za-z_][A-Za-z0-9_]*)`),
	"C":          regexp.MustCompile(`^\s*(?:static\s+|inline\s+|extern\s+)*(struct|enum|typedef|[A-Za-z_][A-Za-z0-9_*\s]*)\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(`),
	"C++":        regexp.MustCompile(`^\s*(?:template\s*<[^>]*>\s*)?(?:static\s+|inline\s+|virtual\s+|constexpr\s+)*(class|struct|enum|namespace|[A-Za-z_][A-Za-z0-9_:<>,*\s]*)\s+([A-Za-z_][A-Za-z0-9_]*)\s*(?:\(|\{)`),
	"Scala":      regexp.MustCompile(`^\s*(?:private\s+|protected\s+|final\s+|sealed\s+|case\s+|abstract\s+)*(class|trait|object|def)\s+([A-Za-z_][A-Za-z0-9_]*)`),
	"Dart":       regexp.MustCompile(`^\s*(?:abstract\s+)?(class|mixin|enum|void|Future)\s+([A-Za-z_][A-Za-z0-9_]*)`),
	"Lua":        regexp.MustCompile(`^\s*(?:local\s+)?(function)\s+([A-Za-z_][A-Za-z0-9_.:]*)`),
	"Zig":        regexp.MustCompile(`^\s*(?:pub\s+)?(fn|const)\s+([A-Za-z_][A-Za-z0-9_]*)`),
	"Elixir":     regexp.MustCompile(`^\s*(defmodule|def|defp)\s+([A-Za-z_][A-Za-z0-9_.?!]*)`),
	"Shell":      regexp.MustCompile(`^\s*(?:(function)\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*\(\)\s*\{?`),
}

func extractGenericFacts(language string, source []byte) ([]string, []SymbolInfo) {
	imports := []string{}
	symbols := []SymbolInfo{}
	declaration := genericDeclarationPatterns[language]
	lines := strings.Split(string(source), "\n")
	for number, line := range lines {
		trimmed := strings.TrimSpace(line)
		if genericImport(language, trimmed) {
			imports = append(imports, trimmed)
		}
		if declaration == nil {
			continue
		}
		match := declaration.FindStringSubmatch(line)
		if len(match) != 3 {
			continue
		}
		kind, ok := normalizeGenericSymbolKind(language, match[1])
		if !ok {
			continue
		}
		endLine := estimateGenericSymbolEndLine(lines, number)
		symbols = append(symbols, SymbolInfo{Name: match[2], Kind: kind, Signature: trimmed, StartLine: number + 1, EndLine: endLine, Visibility: "unknown", Confidence: "approximate", AtomicTarget: kind != "impl"})
	}
	return imports, symbols
}

var genericTypeKinds = map[string]bool{
	"class": true, "interface": true, "enum": true, "record": true, "struct": true, "namespace": true, "typedef": true,
}

func normalizeGenericSymbolKind(language, kind string) (string, bool) {
	if kind == "" {
		return "function", true
	}
	if (language == "Java" || language == "C" || language == "C++") && !genericTypeKinds[kind] {
		if !genericJavaDeclaration(kind) {
			return "", false
		}
		return "method", true
	}
	return kind, true
}

func estimateGenericSymbolEndLine(lines []string, startIdx int) int {
	if startIdx < 0 || startIdx >= len(lines) {
		return startIdx + 1
	}
	first := lines[startIdx]
	openBraces := strings.Count(first, "{") - strings.Count(first, "}")
	if openBraces <= 0 {
		return startIdx + 1
	}
	maxScan := min(len(lines), startIdx+200)
	for i := startIdx + 1; i < maxScan; i++ {
		openBraces += strings.Count(lines[i], "{") - strings.Count(lines[i], "}")
		if openBraces <= 0 {
			return i + 1
		}
	}
	return startIdx + 1
}

func genericJavaDeclaration(prefix string) bool {
	switch prefix {
	case "new", "return", "throw", "case", "else", "for", "while", "switch", "catch", "try", "do", "assert", "yield", "if", "sizeof":
		return false
	default:
		return true
	}
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

// ExtractFileFacts returns the detected language, raw import lines, and symbol spans
// for any non-Go source file.
func ExtractFileFacts(path string, source []byte) (string, []string, []SymbolInfo) {
	lang := detectLanguage(path)
	if lang == "Go" {
		imports, symbols, _ := extractGoFacts(path, source)
		return lang, imports, symbols
	}
	imports, symbols := extractGenericFacts(lang, source)
	return lang, imports, symbols
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
