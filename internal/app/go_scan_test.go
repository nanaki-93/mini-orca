package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/logging"
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

func TestGoScanLifecyclePersistsWorkspaceLimitFailure(t *testing.T) {
	service, root, revision := newGoScanService(t, map[string]string{
		"go.mod":  "module fixture\n\ngo 1.22\n",
		"main.go": "package fixture\n",
	})
	largePath := filepath.Join(root, "workspace-limit.bin")
	if err := os.WriteFile(largePath, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(largePath, checkLimits.maxWorkspaceBytes+1); err != nil {
		t.Fatal(err)
	}
	if _, err := service.StartGoScan(revision); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		report, err := service.GoScanProgress(revision)
		if err != nil {
			t.Fatal(err)
		}
		if report != nil && report.Status == "failed" {
			if len(report.Phases) != 1 || report.Phases[0].Name != "workspace" || report.Phases[0].State != CheckFailed || !strings.Contains(report.Phases[0].Output, "byte limit") {
				t.Fatalf("workspace limit report = %+v", report)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("scan did not persist workspace limit failure")
}

func TestGoScanFailureReportStorageFailurePreservesOriginalFailure(t *testing.T) {
	service, root, revision := newGoScanService(t, map[string]string{
		"go.mod":  "module fixture\n\ngo 1.22\n",
		"main.go": "package fixture\n",
	})
	analysis, err := service.manager.Analysis()
	if err != nil {
		t.Fatal(err)
	}
	largePath := filepath.Join(root, "workspace-limit.bin")
	if err := os.WriteFile(largePath, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(largePath, checkLimits.maxWorkspaceBytes+1); err != nil {
		t.Fatal(err)
	}
	initial := newGoScanReport(analysis.ProjectID, revision)
	initialData, err := marshalGoScanReport(initial)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeGoScanReport(root, initialData); err != nil {
		t.Fatal(err)
	}
	reportPath := filepath.Join(root, goScanPath)
	writes := 0
	service.goScan.writeFailureReport = func(path string, data []byte) error {
		writes++
		var failed GoScanReport
		if err := json.Unmarshal(data, &failed); err != nil {
			t.Fatal(err)
		}
		if path != root || failed.Status != "failed" || len(failed.Phases) != 1 || !strings.Contains(failed.Phases[0].Output, "byte limit") {
			t.Fatalf("storage failure did not follow the original scan failure: %s, %+v", path, failed)
		}
		return errors.New("store scan report: " + root + " api_key=private-token " + strings.Repeat("x", checkLimits.maxOutputBytes+1))
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	service.goScan.report = cloneGoScanReport(initial)
	service.goScan.cancel = cancel
	var output bytes.Buffer
	logging.Init(logging.Config{Format: "json", Output: &output})
	t.Cleanup(func() { logging.Init(logging.Config{Format: "json"}) })

	service.runStartedGoScan(ctx, analysis.ProjectID, revision)

	report := service.goScan.report
	if report == nil || report.Status != "failed" || report.ProjectID != analysis.ProjectID || report.ProjectRevision != revision {
		t.Fatalf("in-memory failure report = %+v", report)
	}
	if len(report.Phases) != 1 || report.Phases[0].Name != "workspace" || report.Phases[0].State != CheckFailed || !strings.Contains(report.Phases[0].Output, "byte limit") {
		t.Fatalf("original scan failure was replaced: %+v", report)
	}
	if service.goScan.cancel != nil || writes != 1 {
		t.Fatalf("failed scan completion: cancel retained = %v, storage attempts = %d", service.goScan.cancel != nil, writes)
	}
	var diagnostic struct {
		Level           string `json:"level"`
		Message         string `json:"msg"`
		ProjectID       string `json:"project_id"`
		ProjectRevision string `json:"project_revision"`
		Error           string `json:"error"`
	}
	if err := json.Unmarshal(output.Bytes(), &diagnostic); err != nil {
		t.Fatalf("expected one persistence diagnostic: %v: %s", err, output.String())
	}
	if diagnostic.Level != "ERROR" || diagnostic.Message != "Failed to persist Go scan failure report" || diagnostic.ProjectID != analysis.ProjectID || diagnostic.ProjectRevision != revision {
		t.Fatalf("persistence diagnostic = %+v", diagnostic)
	}
	if !strings.Contains(diagnostic.Error, "store scan report") || !strings.Contains(diagnostic.Error, "<project>") || !strings.Contains(diagnostic.Error, "[redacted]") || strings.Contains(output.String(), root) || strings.Contains(output.String(), "private-token") {
		t.Fatalf("missing or unsanitized persistence failure: %s", output.String())
	}
	if len(diagnostic.Error) > checkLimits.maxOutputBytes+len("\n[output truncated]") || !strings.HasSuffix(diagnostic.Error, "[output truncated]") {
		t.Fatalf("persistence diagnostic was not bounded: %d bytes", len(diagnostic.Error))
	}
	stored, err := os.ReadFile(reportPath)
	if err != nil || !bytes.Equal(stored, initialData) {
		t.Fatalf("failed storage changed durable report: %s, %v", stored, err)
	}
	if persisted, err := service.GoScanProgress(revision); err != nil || !reflect.DeepEqual(persisted, cloneGoScanReport(initial)) {
		t.Fatalf("failed storage reported new durable progress: %+v, %v", persisted, err)
	}
}

func TestGoScanFailedDetachedWorkerPreservesActiveScan(t *testing.T) {
	for _, change := range []string{"project", "revision"} {
		t.Run(change, func(t *testing.T) {
			service, root, revision := newGoScanService(t, map[string]string{
				"go.mod":  "module fixture\n\ngo 1.22\n",
				"main.go": "package fixture\n",
			})
			original, err := service.manager.Analysis()
			if err != nil {
				t.Fatal(err)
			}
			activeRoot := root
			if change == "project" {
				activeRoot = t.TempDir()
			}
			if err := os.WriteFile(filepath.Join(activeRoot, "main.go"), []byte("package replacement\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := service.manager.Set(activeRoot, &project.Analysis{Name: "replacement", Path: activeRoot}); err != nil {
				t.Fatal(err)
			}
			active, err := service.manager.Analysis()
			if err != nil {
				t.Fatal(err)
			}
			if active.ProjectID == original.ProjectID && active.ProjectRevision == revision {
				t.Fatal("fixture did not detach the original scan")
			}
			report := newGoScanReport(active.ProjectID, active.ProjectRevision)
			data, err := marshalGoScanReport(report)
			if err != nil {
				t.Fatal(err)
			}
			if err := writeGoScanReport(activeRoot, data); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			service.goScan.report = cloneGoScanReport(report)
			service.goScan.cancel = cancel
			service.goScan.writeFailureReport = func(string, []byte) error {
				t.Fatal("detached worker attempted to persist a failure report")
				return nil
			}

			service.runStartedGoScan(context.Background(), original.ProjectID, revision)

			if !reflect.DeepEqual(service.goScan.report, cloneGoScanReport(report)) || service.goScan.cancel == nil || ctx.Err() != nil {
				t.Fatalf("detached worker changed active scan: %+v", service.goScan.report)
			}
			stored, err := os.ReadFile(filepath.Join(activeRoot, goScanPath))
			if err != nil || !bytes.Equal(stored, data) {
				t.Fatalf("detached worker changed persisted scan: %s, %v", stored, err)
			}
		})
	}
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
	if _, err := service.TrustProjectExecution(analysis.ProjectRevision, true); err != nil {
		t.Fatal(err)
	}
	return service, root, analysis.ProjectRevision
}
