package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParsePerformanceFindingsKeepsOnlyAnchoredSourceBasedOpportunities(t *testing.T) {
	source := "package main\nfunc Run() {\n for range []int{1} {}\n}\n"
	symbols := []SymbolInfo{{Name: "Run", StartLine: 2, EndLine: 4}}
	valid := `{"findings":[{"category":"cpu","potential_impact":"medium","confidence":"low","title":"Repeated work","observed_pattern":"The loop repeats local work.","workload_conditions":"Only if the input collection grows.","recommendation":"Measure then cache safely.","tradeoff":"Caching retains memory.","verification_plan":"Benchmark representative input sizes.","start_line":3,"end_line":3,"symbol":"Run"}]}`
	findings, warning, err := ParsePerformanceFindings(valid, "main.go", source, symbols)
	if err != nil || warning != "" || len(findings) != 1 || findings[0].ID == "" {
		t.Fatalf("findings = %+v, %q, %v", findings, warning, err)
	}
	partial := strings.Replace(valid, `"start_line":3,"end_line":3,"symbol":"Run"`, `"start_line":30,"end_line":30`, 1)
	if _, _, err := ParsePerformanceFindings(partial, "main.go", source, symbols); err == nil {
		t.Fatal("all-invalid nonempty output became empty success")
	}
}

func TestPerformanceCacheIsSeparateSourceFreeAndFreshnessBound(t *testing.T) {
	root := t.TempDir()
	report := PerformanceFileReport{SchemaVersion: "1", ProjectID: "p", ProjectRevision: "r", Path: "main.go", ContentHash: "one", Status: "completed", Findings: []PerformanceFinding{}, Model: "model", Profile: "analyze", Scope: "analyze", PromptVersion: PerformancePromptVersion, ContextPolicyVersion: "policy", GeneratedAt: time.Now().UTC()}
	if err := StorePerformanceFileReport(root, report); err != nil {
		t.Fatal(err)
	}
	stored, err := os.ReadFile(performanceCachePath(root, "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(stored), "SOURCE:") || filepath.Base(performanceCachePath(root, "main.go")) == "main.go" {
		t.Fatalf("unsafe cache = %s", stored)
	}
	loaded, err := LoadPerformanceFileReport(root, "main.go", "two")
	if err != nil || loaded == nil || loaded.Status != "stale" {
		t.Fatalf("stale load = %+v, %v", loaded, err)
	}
}
