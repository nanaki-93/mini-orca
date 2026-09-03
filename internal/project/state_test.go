package project

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestManagerRejectsStaleFileAndProjectState(t *testing.T) {
	root := newStateFixture(t, "sample.go", "package fixture\nfunc Run() {}\n")
	manager, err := NewManager(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Set(root, &Analysis{}); err != nil {
		t.Fatal(err)
	}
	analysis, err := manager.Analysis()
	if err != nil {
		t.Fatal(err)
	}
	info, err := GetFileInfo(root, "sample.go")
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.ValidateMutableRequest(analysis.ProjectID, analysis.ProjectRevision, "sample.go", info.ContentHash); err != nil {
		t.Fatalf("fresh request rejected: %v", err)
	}

	if err := os.WriteFile(filepath.Join(root, "sample.go"), []byte("package fixture\nfunc Run() { println(\"changed\") }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := manager.ValidateMutableRequest(analysis.ProjectID, analysis.ProjectRevision, "sample.go", info.ContentHash); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("stale file error = %v, want revision conflict", err)
	}
	if err := manager.Set(root, &Analysis{}); err != nil {
		t.Fatal(err)
	}
	updated, err := manager.Analysis()
	if err != nil {
		t.Fatal(err)
	}
	if updated.ProjectID != analysis.ProjectID || updated.ProjectRevision == analysis.ProjectRevision {
		t.Fatalf("updated project state = %+v, original = %+v", updated, analysis)
	}
}

func newStateFixture(t *testing.T, name, content string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return root
}
