package project

import "github.com/nanaki-93/mini-orca/v2/internal/project/symbols"

func extractGoFacts(filename string, source []byte) ([]string, []SymbolInfo, []Diagnostic) {
	return symbols.ExtractGoFacts(filename, source)
}
