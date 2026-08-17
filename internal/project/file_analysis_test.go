package project

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestFileAnalysisCacheRoundTripAndSourceFreePersistence(t *testing.T) {
	root := t.TempDir()
	writeIndexFixture(t, root, "main.go", "package main\nfunc Run() {}\n")
	cache, err := NewFileAnalysisCache(root)
	if err != nil {
		t.Fatal(err)
	}
	input := analysisCacheInput("main.go", "sha256:one")
	analysis := newFreshAnalysis(input)
	analysis.Purpose = "Runs the command."
	analysis.Responsibilities = []string{"dispatches work"}
	analysis.SymbolExplanations = map[string]string{"Run": "Starts the command."}
	if err := cache.Store(analysis); err != nil {
		t.Fatal(err)
	}
	loaded, err := cache.Load(input)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Status != AnalysisStatusFresh || loaded.Purpose != analysis.Purpose || loaded.SymbolExplanations["Run"] == "" {
		t.Fatalf("loaded analysis = %+v", loaded)
	}
	data, err := os.ReadFile(cache.cachePath("main.go"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "package main") || strings.Contains(string(data), "func Run") {
		t.Fatalf("cache contains original source: %s", data)
	}
}

func TestFileAnalysisCacheInvalidatesOnlyChangedInputs(t *testing.T) {
	root := t.TempDir()
	writeIndexFixture(t, root, "one.go", "package main\nfunc One() {}\n")
	writeIndexFixture(t, root, "two.go", "package main\nfunc Two() {}\n")
	cache, err := NewFileAnalysisCache(root)
	if err != nil {
		t.Fatal(err)
	}
	one := analysisCacheInput("one.go", "sha256:one")
	two := analysisCacheInput("two.go", "sha256:two")
	if err := cache.Store(newFreshAnalysis(one)); err != nil {
		t.Fatal(err)
	}
	if err := cache.Store(newFreshAnalysis(two)); err != nil {
		t.Fatal(err)
	}

	changedOne := one
	changedOne.ContentHash = "sha256:one-changed"
	changedOne.ProjectRevision = "sha256:revision-after-one-change"
	loaded, err := cache.Load(changedOne)
	if err != nil || loaded.Status != AnalysisStatusStale {
		t.Fatalf("changed analysis = %+v, %v; want stale", loaded, err)
	}
	updatedTwo := two
	updatedTwo.ProjectRevision = changedOne.ProjectRevision
	loaded, err = cache.Load(updatedTwo)
	if err != nil || loaded.Status != AnalysisStatusFresh {
		t.Fatalf("unchanged analysis = %+v, %v; want fresh", loaded, err)
	}

	for _, mutate := range []func(*FileAnalysisInput){
		func(value *FileAnalysisInput) { value.Model = "new-model" },
		func(value *FileAnalysisInput) { value.Profile = "new-profile" },
		func(value *FileAnalysisInput) { value.ContextPolicyVersion = "policy-v2" },
		func(value *FileAnalysisInput) { value.PromptVersion = "prompt-v2" },
	} {
		candidate := two
		mutate(&candidate)
		loaded, err := cache.Load(candidate)
		if err != nil || loaded.Status != AnalysisStatusStale {
			t.Fatalf("changed validity input = %+v, %v; want stale", loaded, err)
		}
	}
}

func TestFileAnalysisCacheRecoversCorruptionAndConcurrentAccess(t *testing.T) {
	root := t.TempDir()
	writeIndexFixture(t, root, "main.go", "package main\nfunc Run() {}\n")
	cache, err := NewFileAnalysisCache(root)
	if err != nil {
		t.Fatal(err)
	}
	input := analysisCacheInput("main.go", "sha256:one")
	if err := os.MkdirAll(filepath.Dir(cache.cachePath(input.Path)), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cache.cachePath(input.Path), []byte("not-json"), 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := cache.Load(input)
	if err != nil || loaded.Status != AnalysisStatusMissing {
		t.Fatalf("recovered analysis = %+v, %v", loaded, err)
	}
	corrupt, err := filepath.Glob(cache.cachePath(input.Path) + ".corrupt-*")
	if err != nil || len(corrupt) != 1 {
		t.Fatalf("corrupt backup = %v, %v", corrupt, err)
	}

	var group sync.WaitGroup
	for i := 0; i < 20; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			if err := cache.Store(newFreshAnalysis(input)); err != nil {
				t.Error(err)
			}
			if _, err := cache.Load(input); err != nil {
				t.Error(err)
			}
		}()
	}
	group.Wait()
	loaded, err = cache.Load(input)
	if err != nil || loaded.Status != AnalysisStatusFresh {
		t.Fatalf("concurrent cache result = %+v, %v", loaded, err)
	}
}

func analysisCacheInput(path, hash string) FileAnalysisInput {
	return FileAnalysisInput{ProjectID: "sha256:project", ProjectRevision: "sha256:revision", Path: path, ContentHash: hash, Language: "Go", Model: "local-model", Profile: "coder", PromptVersion: "prompt-v1", ContextPolicyVersion: "policy-v1"}
}

func newFreshAnalysis(input FileAnalysisInput) FileAnalysis {
	return FileAnalysis{SchemaVersion: fileAnalysisSchemaVersion, ProjectID: input.ProjectID, ProjectRevision: input.ProjectRevision, Path: input.Path, ContentHash: input.ContentHash, Language: input.Language, Status: AnalysisStatusFresh, Model: input.Model, Profile: input.Profile, PromptVersion: input.PromptVersion, ContextPolicyVersion: input.ContextPolicyVersion}
}
