package project

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestVerifyProjectSourcesPreservesCancellationAndReadOnlyState(t *testing.T) {
	root := newStateFixture(t, "sample.go", "package fixture\nfunc Run() {}\n")
	manager, err := NewManager(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Set(root, &Analysis{}); err != nil {
		t.Fatal(err)
	}
	index, err := manager.Index()
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(root, indexRelativePath))
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyProjectSources(context.Background(), root, index); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := VerifyProjectSources(ctx, root, index); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled verification=%v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "added.go"), []byte("package fixture\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := VerifyProjectSources(context.Background(), root, index); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("changed source verification=%v", err)
	}
	after, err := os.ReadFile(filepath.Join(root, indexRelativePath))
	if err != nil || string(before) != string(after) {
		t.Fatalf("verification rewrote the index: %v", err)
	}
}

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
