package project

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestReindexRefreshesInventoryAndBuildFactsWithoutReplacingInterpretation(t *testing.T) {
	root := t.TempDir()
	writeIndexFixture(t, root, "go.mod", "module example\ngo 1.22\n")
	writeIndexFixture(t, root, "main.go", "package main\nfunc main() {}\n")
	analyzer := &Analyzer{}
	analysis, err := analyzer.scan(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	analysis.AIStatus = ProjectAnalysisStatusFresh
	analysis.Report = ProjectAnalysisReport{Status: ProjectAnalysisStatusFresh, ProjectRevision: analysis.ProjectRevision, Purpose: "Previously analyzed purpose"}
	manager, err := NewManager(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Set(root, analysis); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"main.go", "go.mod"} {
		if err := os.Remove(filepath.Join(root, path)); err != nil {
			t.Fatal(err)
		}
	}
	writeIndexFixture(t, root, "app.py", "def run():\n    return 1\n")
	writeIndexFixture(t, root, "helper.py", "answer = 42")
	writeIndexFixture(t, root, "requirements.txt", "example\n")
	writeIndexFixture(t, root, "ignored.py", "hidden = True\n")
	writeIndexFixture(t, root, ".gitignore", "ignored.py\n")
	index, err := manager.Reindex()
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := manager.Analysis()
	if err != nil {
		t.Fatal(err)
	}
	if fresh.FileCount != 4 || fresh.SourceFileCount != 2 || fresh.TotalLines != 3 || !reflect.DeepEqual(fresh.Languages, map[string]int{"Python": 2}) {
		t.Fatalf("stale inventory after reindex: %+v", fresh)
	}
	if !reflect.DeepEqual(fresh.Files, []string{".gitignore", "app.py", "helper.py", "requirements.txt"}) || fresh.Type != "python" || fresh.BuildFile != "requirements.txt" || fresh.ProjectRevision != index.ProjectRevision {
		t.Fatalf("stale paths or project identity after reindex: %+v", fresh)
	}
	if fresh.Report.Status != ProjectAnalysisStatusStale || fresh.AIStatus != ProjectAnalysisStatusStale || fresh.Report.Purpose != "Previously analyzed purpose" {
		t.Fatalf("reindex must retain and mark historical interpretation stale: %+v", fresh.Report)
	}
	// An unchanged refresh must not accumulate counts or reinterpret the project.
	if _, err := manager.Reindex(); err != nil {
		t.Fatal(err)
	}
	repeated, err := manager.Analysis()
	if err != nil || !reflect.DeepEqual(fresh, repeated) {
		t.Fatalf("repeated reindex changed facts: %+v, %v", repeated, err)
	}
}
