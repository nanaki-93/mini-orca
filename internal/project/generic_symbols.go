package project

import (
	"regexp"
	"strings"
)

var genericDeclarationPatterns = map[string]*regexp.Regexp{
	"Kotlin":     regexp.MustCompile(`^\s*(?:public\s+|private\s+|internal\s+|protected\s+)?(?:data\s+|sealed\s+|abstract\s+)?(class|interface|object|fun)\s+([A-Za-z_][A-Za-z0-9_]*)`),
	"Java":       regexp.MustCompile(`^\s*(?:public\s+|private\s+|protected\s+)?(?:static\s+|final\s+|abstract\s+)*(class|interface|enum|record|[A-Za-z_][A-Za-z0-9_<>, ?\[\]]*)\s+([A-Za-z_][A-Za-z0-9_]*)\s*(?:\(|\{)`),
	"TypeScript": regexp.MustCompile(`^\s*(?:export\s+)?(?:default\s+)?(?:async\s+)?(class|interface|type|function)\s+([A-Za-z_$][A-Za-z0-9_$]*)`),
	"Python":     regexp.MustCompile(`^\s*(class|def)\s+([A-Za-z_][A-Za-z0-9_]*)`),
	"Rust":       regexp.MustCompile(`^\s*(?:pub\s+)?(struct|enum|trait|fn|impl)\s+([A-Za-z_][A-Za-z0-9_]*)`),
}

func extractGenericFacts(language string, source []byte) ([]string, []SymbolInfo) {
	imports := []string{}
	symbols := []SymbolInfo{}
	declaration := genericDeclarationPatterns[language]
	for number, line := range strings.Split(string(source), "\n") {
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
		kind, name := match[1], match[2]
		if language == "Java" && kind != "class" && kind != "interface" && kind != "enum" && kind != "record" {
			if !genericJavaDeclaration(kind) {
				continue
			}
			kind = "method"
		}
		symbols = append(symbols, SymbolInfo{Name: name, Kind: kind, Signature: trimmed, StartLine: number + 1, EndLine: number + 1, Visibility: "unknown", Confidence: "approximate", AtomicTarget: kind != "impl"})
	}
	return imports, symbols
}

func genericJavaDeclaration(prefix string) bool {
	switch prefix {
	case "new", "return", "throw", "case", "else", "for", "while", "switch", "catch", "try", "do", "assert", "yield":
		return false
	default:
		return true
	}
}

func genericImport(language, line string) bool {
	switch language {
	case "Kotlin", "Java", "TypeScript":
		return strings.HasPrefix(line, "import ")
	case "Python":
		return strings.HasPrefix(line, "import ") || strings.HasPrefix(line, "from ")
	case "Rust":
		return strings.HasPrefix(line, "use ")
	default:
		return false
	}
}
