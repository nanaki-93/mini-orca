package project

import "testing"

func TestBuildImpactPreviewUsesOnlyIndexImports(t *testing.T) {
	preview := BuildImpactPreview(&ProjectIndex{Files: []IndexFile{{Path: "cmd/main.go", Imports: []string{"example/internal/project"}}, {Path: "internal/project/index.go"}}}, "internal/project/index.go", "BuildIndex")
	if len(preview.References) != 1 || preview.References[0].Path != "cmd/main.go" || preview.References[0].Confidence != "medium" {
		t.Fatalf("preview = %+v", preview)
	}
}
