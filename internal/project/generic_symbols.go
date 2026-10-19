package project

import "github.com/nanaki-93/mini-orca/v2/internal/project/symbols"

func extractGenericFacts(language string, source []byte) ([]string, []SymbolInfo) {
	return symbols.ExtractGenericFacts(language, source)
}

// ExtractFileFacts returns the detected language, raw import lines, and symbol spans
// for any source file.
func ExtractFileFacts(path string, source []byte) (string, []string, []SymbolInfo) {
	return symbols.ExtractFileFacts(path, source)
}

// NormalizeGenericImportPath extracts a slash-normalized module path from a raw
// import statement across TypeScript/JS, Python, Rust, JVM, C/C++, C#, Ruby, PHP, and Elixir.
func NormalizeGenericImportPath(raw string) string {
	return symbols.NormalizeGenericImportPath(raw)
}
