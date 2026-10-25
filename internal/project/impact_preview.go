package project

import (
	"path/filepath"
	"strings"
)

// ImpactReference is advisory index metadata; it never adds source to a model request.
type ImpactReference struct {
	Path       string `json:"path"`
	Symbol     string `json:"symbol,omitempty"`
	Confidence string `json:"confidence"`
	Reason     string `json:"reason"`
}

type ImpactPreview struct {
	TargetPath   string            `json:"target_path"`
	TargetSymbol string            `json:"target_symbol,omitempty"`
	References   []ImpactReference `json:"references"`
}

// BuildImpactPreview uses only deterministic imports and declaration names.
func BuildImpactPreview(index *ProjectIndex, targetPath, targetSymbol string) ImpactPreview {
	preview := ImpactPreview{TargetPath: targetPath, TargetSymbol: targetSymbol, References: []ImpactReference{}}
	if index == nil {
		return preview
	}
	targetDir := filepath.ToSlash(filepath.Dir(targetPath))
	for _, file := range index.Files {
		if file.Path == targetPath {
			continue
		}
		if ref, ok := fileImpactReference(file, targetDir, targetSymbol); ok {
			preview.References = append(preview.References, ref)
		}
	}
	return preview
}

func fileImpactReference(file IndexFile, targetDir, targetSymbol string) (ImpactReference, bool) {
	matchedSymbol := matchingImpactSymbol(file.Symbols, targetSymbol)
	for _, imported := range file.Imports {
		trimmed := strings.Trim(imported, `"'/`)
		if targetDir != "." && (trimmed == targetDir || strings.HasSuffix(trimmed, "/"+targetDir)) {
			return ImpactReference{Path: file.Path, Symbol: matchedSymbol, Confidence: "medium", Reason: "imports the target package"}, true
		}
	}
	if matchedSymbol != "" && filepath.ToSlash(filepath.Dir(file.Path)) == targetDir {
		return ImpactReference{Path: file.Path, Symbol: matchedSymbol, Confidence: "medium", Reason: "references the target symbol in package"}, true
	}
	return ImpactReference{}, false
}

func matchingImpactSymbol(symbols []SymbolInfo, targetSymbol string) string {
	if targetSymbol == "" {
		return ""
	}
	for _, symbol := range symbols {
		if symbol.Name == targetSymbol || strings.Contains(symbol.Signature, targetSymbol) {
			return symbol.Name
		}
	}
	return ""
}
