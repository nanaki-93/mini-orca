package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

func TestScanGoProjectFixtures(t *testing.T) {
	for _, test := range []struct {
		name      string
		files     map[string]string
		wantParse string
		wantVet   string
		wantTests string
		truncated bool
	}{
		{name: "pass and isolate", files: map[string]string{"go.mod": "module fixture\n\ngo 1.22\n", "main.go": "package fixture\nfunc Run() {}\n", "main_test.go": "package fixture\nimport \"os\"\nimport \"testing\"\nfunc TestRun(t *testing.T) { if err := os.WriteFile(\"scan-marker\", []byte(\"x\"), 0600); err != nil { t.Fatal(err) } }\n"}, wantParse: CheckPassed, wantVet: CheckPassed, wantTests: CheckPassed},
		{name: "parse failure", files: map[string]string{"go.mod": "module fixture\n\ngo 1.22\n", "main.go": "package fixture\nfunc Broken( {\n"}, wantParse: CheckFailed},
		{name: "vet failure", files: map[string]string{"go.mod": "module fixture\n\ngo 1.22\n", "main.go": "package fixture\nimport \"fmt\"\nfunc Run() { fmt.Printf(\"%d\", \"wrong\") }\n"}, wantParse: CheckPassed, wantVet: CheckFailed},
		{name: "test failure", files: map[string]string{"go.mod": "module fixture\n\ngo 1.22\n", "main.go": "package fixture\n", "main_test.go": "package fixture\nimport \"testing\"\nfunc TestBroken(t *testing.T) { t.Fatal(\"broken\") }\n"}, wantParse: CheckPassed, wantTests: CheckFailed},
		{name: "test output is truncated", files: map[string]string{"go.mod": "module fixture\n\ngo 1.22\n", "main.go": "package fixture\n", "main_test.go": "package fixture\nimport (\"strings\"; \"testing\")\nfunc TestLoud(t *testing.T) { t.Fatal(strings.Repeat(\"x\", 9000)) }\n"}, wantParse: CheckPassed, wantTests: CheckFailed, truncated: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			service, root, revision := newGoScanService(t, test.files)
			report, err := service.ScanGoProject(context.Background(), revision)
			if err != nil {
				t.Fatal(err)
			}
			states := map[string]string{}
			outputs := map[string]string{}
			for _, phase := range report.Phases {
				states[phase.Name] = phase.State
				outputs[phase.Name] = phase.Output
			}
			if test.wantParse != "" && states["parse"] != test.wantParse {
				t.Fatalf("parse = %q, report = %+v", states["parse"], report)
			}
			if test.wantVet != "" && states["vet"] != test.wantVet {
				t.Fatalf("vet = %q, report = %+v", states["vet"], report)
			}
			if test.wantTests != "" && states["tests"] != test.wantTests {
				t.Fatalf("tests = %q, report = %+v", states["tests"], report)
			}
			if test.truncated && !strings.Contains(outputs["tests"], "[output truncated]") {
				t.Fatalf("test output was not bounded: %d bytes", len(outputs["tests"]))
			}
			if _, err := os.Stat(filepath.Join(root, "scan-marker")); !os.IsNotExist(err) {
				t.Fatalf("scan modified imported project: %v", err)
			}
			if _, err := os.Stat(filepath.Join(root, goScanPath)); err != nil {
				t.Fatalf("scan result not persisted: %v", err)
			}
		})
	}
}

func TestScanGoProjectRejectsStaleRevisionAndCancels(t *testing.T) {
	service, _, revision := newGoScanService(t, map[string]string{"go.mod": "module fixture\n\ngo 1.22\n", "main.go": "package fixture\n", "main_test.go": "package fixture\nimport (\"testing\"; \"time\")\nfunc TestSlow(t *testing.T) { time.Sleep(time.Second) }\n"})
	if _, err := service.ScanGoProject(context.Background(), "stale"); !errors.Is(err, project.ErrRevisionConflict) {
		t.Fatalf("stale scan = %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	report, err := service.ScanGoProject(ctx, revision)
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != "canceled" || report.Phases[len(report.Phases)-1].State != CheckCanceled {
		t.Fatalf("canceled report = %+v", report)
	}
}

func TestGoScanLifecycleStartsReadsAndCancels(t *testing.T) {
	service, _, revision := newGoScanService(t, map[string]string{
		"go.mod":       "module fixture\n\ngo 1.22\n",
		"main.go":      "package fixture\n",
		"main_test.go": "package fixture\nimport (\"testing\"; \"time\")\nfunc TestSlow(t *testing.T) { time.Sleep(time.Second) }\n",
	})
	if _, err := service.StartGoScan("stale"); !errors.Is(err, project.ErrRevisionConflict) {
		t.Fatalf("stale start = %v", err)
	}
	started, err := service.StartGoScan(revision)
	if err != nil || started.Status != "running" {
		t.Fatalf("started scan = %+v, %v", started, err)
	}
	if _, err := service.GoScanProgress(revision); err != nil {
		t.Fatal(err)
	}
	if _, err := service.CancelGoScan(revision); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		report, err := service.GoScanProgress(revision)
		if err != nil {
			t.Fatal(err)
		}
		if report != nil && report.Status == "canceled" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("scan did not report cancellation")
}

func newGoScanService(t *testing.T, files map[string]string) (*Service, string, string) {
	t.Helper()
	root := t.TempDir()
	for path, content := range files {
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	manager, err := project.NewManager(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Set(root, &project.Analysis{Name: "fixture", Path: root}); err != nil {
		t.Fatal(err)
	}
	analysis, err := manager.Analysis()
	if err != nil {
		t.Fatal(err)
	}
	cfg := scopedTestConfig("http://127.0.0.1:1")
	cfg.Timeouts = config.TimeoutConfig{FocusedCheckSeconds: 5}
	service, err := New(cfg, manager)
	if err != nil {
		t.Fatal(err)
	}
	return service, root, analysis.ProjectRevision
}
