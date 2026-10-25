package project

import (
	"path/filepath"
	"strings"
)

// ImpactReference is advisory index metadata; it never adds source to a model request.
type ImpactReference struct {
	Path       string `json:"path"`
	Confidence string `json:"confidence"`
	Reason     string `json:"reason"`
}

type ImpactPreview struct {
	TargetPath string            `json:"target_path"`
	References []ImpactReference `json:"references"`
}

// BuildImpactPreview uses only deterministic imports.
func BuildImpactPreview(index *ProjectIndex, targetPath string) ImpactPreview {
	preview := ImpactPreview{TargetPath: targetPath, References: []ImpactReference{}}
	if index == nil {
		return preview
	}
	targetDir := filepath.ToSlash(filepath.Dir(targetPath))
	for _, file := range index.Files {
		if file.Path == targetPath {
			continue
		}
		if ref, ok := fileImpactReference(file, targetDir); ok {
			preview.References = append(preview.References, ref)
		}
	}
	return preview
}

func fileImpactReference(file IndexFile, targetDir string) (ImpactReference, bool) {
	for _, imported := range file.Imports {
		trimmed := strings.Trim(imported, `"'/`)
		if targetDir != "." && (trimmed == targetDir || strings.HasSuffix(trimmed, "/"+targetDir)) {
			return ImpactReference{Path: file.Path, Confidence: "medium", Reason: "imports the target package"}, true
		}
	}
	return ImpactReference{}, false
}
