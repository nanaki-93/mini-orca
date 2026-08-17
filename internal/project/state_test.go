package project

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestManagerScopesActivityAndPersistsAcrossRestart(t *testing.T) {
	rootA := newStateFixture(t, "a.go", "package fixture\nfunc A() {}\n")
	rootB := newStateFixture(t, "b.go", "package fixture\nfunc B() {}\n")
	manager, err := NewManager(rootA)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Set(rootA, &Analysis{Name: "a"}); err != nil {
		t.Fatal(err)
	}
	if err := manager.RecordActivity(Activity{Role: "user", Content: "Requested focused generation", Phase: "coding", TargetFile: "a.go", TargetSymbol: "A"}); err != nil {
		t.Fatal(err)
	}
	activity, err := manager.Activity()
	if err != nil || len(activity) != 1 {
		t.Fatalf("activity = %+v, %v", activity, err)
	}
	if activity[0].ProjectID == "" || activity[0].ProjectRevision == "" {
		t.Fatalf("activity missing project state: %+v", activity[0])
	}

	if err := manager.Set(rootB, &Analysis{Name: "b"}); err != nil {
		t.Fatal(err)
	}
	activity, err = manager.Activity()
	if err != nil || len(activity) != 0 {
		t.Fatalf("project B activity = %+v, %v; project A activity leaked", activity, err)
	}

	restarted, err := NewManager(rootA)
	if err != nil {
		t.Fatal(err)
	}
	if err := restarted.Set(rootA, &Analysis{Name: "a"}); err != nil {
		t.Fatal(err)
	}
	activity, err = restarted.Activity()
	if err != nil || len(activity) != 1 || activity[0].TargetSymbol != "A" {
		t.Fatalf("restarted activity = %+v, %v", activity, err)
	}
	if _, err := os.Stat(filepath.Join(rootA, ".mini-orca", "sessions", "activity.json")); err != nil {
		t.Fatalf("activity was not persisted: %v", err)
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

func TestManagerRecoversCorruptActivityFile(t *testing.T) {
	root := newStateFixture(t, "sample.go", "package fixture\n")
	if err := os.MkdirAll(filepath.Join(root, ".mini-orca", "sessions"), 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ".mini-orca", "sessions", "activity.json")
	if err := os.WriteFile(path, []byte("not-json"), 0600); err != nil {
		t.Fatal(err)
	}
	manager, err := NewManager(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Set(root, &Analysis{}); err != nil {
		t.Fatal(err)
	}
	activity, err := manager.Activity()
	if err != nil || len(activity) != 0 {
		t.Fatalf("recovered activity = %+v, %v", activity, err)
	}
	matches, err := filepath.Glob(path + ".corrupt-*")
	if err != nil || len(matches) != 1 {
		t.Fatalf("corrupt activity backup = %v, %v", matches, err)
	}
}

func TestManagerRejectsActivityForProjectSwitchedDuringGeneration(t *testing.T) {
	rootA := newStateFixture(t, "a.go", "package fixture\nfunc A() {}\n")
	rootB := newStateFixture(t, "b.go", "package fixture\nfunc B() {}\n")
	manager, err := NewManager(rootA)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Set(rootA, &Analysis{}); err != nil {
		t.Fatal(err)
	}
	analysisA, err := manager.Analysis()
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Set(rootB, &Analysis{}); err != nil {
		t.Fatal(err)
	}
	if err := manager.RecordActivityFor(analysisA.ProjectID, analysisA.ProjectRevision, Activity{Role: "assistant", Content: "Generated preview", Phase: "coding"}); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("record error = %v, want revision conflict", err)
	}
	activity, err := manager.Activity()
	if err != nil || len(activity) != 0 {
		t.Fatalf("project B activity = %+v, %v; project A activity leaked", activity, err)
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
