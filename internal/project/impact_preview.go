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
		for _, imported := range file.Imports {
			if targetDir != "." && strings.HasSuffix(strings.TrimSuffix(imported, "/"), targetDir) {
				preview.References = append(preview.References, ImpactReference{Path: file.Path, Confidence: "medium", Reason: "imports the target package"})
				break
			}
		}
	}
	return preview
}
