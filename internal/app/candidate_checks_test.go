package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

func TestCandidateChecksUseIsolatedWorkspace(t *testing.T) {
	service, root := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	originalPath := filepath.Join(root, "main.go")
	original, err := os.ReadFile(originalPath)
	if err != nil {
		t.Fatal(err)
	}
	candidate := "package main\n\nimport \"fmt\"\n\nfunc Run() { fmt.Println(\"changed\") }\n"
	report, err := runFixtureDraftChecks(service, context.Background(), candidate, CandidateCheckOptions{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Applicable || len(report.Checks) != 4 || report.Checks[0].State != CheckPassed || report.Checks[1].State != CheckPassed {
		t.Fatalf("report = %+v", report)
	}
	after, err := os.ReadFile(originalPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(original) {
		t.Fatal("candidate checks modified the imported project")
	}
}

func TestCandidateChecksReportFormatterFailure(t *testing.T) {
	service, _ := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	candidate := "package main\nimport \"fmt\"\nfunc Run(){fmt.Println(\"changed\")}\n"
	report, err := runFixtureDraftChecks(service, context.Background(), candidate, CandidateCheckOptions{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if report.Applicable || report.Checks[1].State != CheckFailed {
		t.Fatalf("formatting failure report = %+v", report)
	}
}

func TestRunCheckCommandHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, _, err := runCheckCommand(ctx, t.TempDir(), []string{"sleep", "1"})
	if err == nil || !errors.Is(ctx.Err(), context.DeadlineExceeded) {
		t.Fatalf("command error = %v, context = %v", err, ctx.Err())
	}
}

func TestTaskTestChecksRequireBaseFailureAndCandidatePassWithoutWritingProject(t *testing.T) {
	service, root := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module fixture\n\ngo 1.22\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n\nfunc Run() bool { return false }\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Reindex(); err != nil {
		t.Fatal(err)
	}
	test := project.GoTestCandidateSpec{Name: "TestRun", Content: "package main\n\nimport \"testing\"\n\nfunc TestRun(t *testing.T) { if !Run() { t.Fatal(\"expected true\") } }\n"}
	report, err := runFixtureDraftChecks(service, context.Background(), "package main\n\nfunc Run() bool { return true }\n", CandidateCheckOptions{}, &test)
	checks := taskChecks(report.Checks)
	if err != nil || len(checks) != 2 || checks[0].State != CheckPassed || checks[1].State != CheckPassed {
		t.Fatalf("task checks = %+v, err = %v", checks, err)
	}
	if _, err := os.Stat(filepath.Join(root, "mini_orca_task_test.go")); !os.IsNotExist(err) {
		t.Fatalf("task test escaped temporary workspace: %v", err)
	}

	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n\nfunc Run() bool { return true }\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Reindex(); err != nil {
		t.Fatal(err)
	}
	report, err = runFixtureDraftChecks(service, context.Background(), "package main\n\nfunc Run() bool { return true }\n", CandidateCheckOptions{}, &test)
	checks = taskChecks(report.Checks)
	if err != nil || len(checks) != 1 || checks[0].State != CheckFailed {
		t.Fatalf("unexpectedly passing baseline = %+v, err = %v", checks, err)
	}
}

func TestTaskTestChecksReportCandidateFailureAndUseNonConflictingFilename(t *testing.T) {
	service, root := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module fixture\n\ngo 1.22\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "mini_orca_task_test.go"), []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n\nfunc Run() bool { return false }\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Reindex(); err != nil {
		t.Fatal(err)
	}
	test := project.GoTestCandidateSpec{Name: "TestRun", Content: "package main\n\nimport \"testing\"\n\nfunc TestRun(t *testing.T) { if !Run() { t.Fatal(\"expected true\") } }\n"}
	report, err := runFixtureDraftChecks(service, context.Background(), "package main\n\nfunc Run() bool { return false }\n", CandidateCheckOptions{}, &test)
	checks := taskChecks(report.Checks)
	if err != nil || len(checks) != 2 || checks[0].State != CheckPassed || checks[1].State != CheckFailed {
		t.Fatalf("candidate failure checks = %+v, err = %v", checks, err)
	}
	workspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspace, "nested"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "nested", "mini_orca_task_test.go"), []byte("package nested\n"), 0600); err != nil {
		t.Fatal(err)
	}
	path, err := taskTestPath(workspace, "nested/main.go")
	if err != nil || filepath.Base(path) != "mini_orca_task_1_test.go" {
		t.Fatalf("task filename = %q, err = %v", path, err)
	}
}

func taskChecks(checks []CandidateCheck) []CandidateCheck {
	result := make([]CandidateCheck, 0, 2)
	for _, check := range checks {
		if strings.HasPrefix(check.Name, "task test ") {
			result = append(result, check)
		}
	}
	return result
}

func TestTaskTestCheckCancellationAndEvidenceSanitization(t *testing.T) {
	service, _ := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	check := service.runTaskTestCheck(ctx, t.TempDir(), "task", []string{"go", "test", "./..."}, true)
	if check.State != CheckCanceled {
		t.Fatalf("canceled task check = %+v", check)
	}
	evidence := sanitizeCheckOutput("/tmp/work/api_key=private\npassword: hidden\n"+strings.Repeat("x", maxCheckOutputBytes+1), "/tmp/work", "/project")
	if strings.Contains(evidence, "private") || strings.Contains(evidence, "hidden") || !strings.Contains(evidence, "[output truncated]") || !strings.Contains(evidence, "<workspace>") {
		t.Fatalf("sanitized evidence = %q", evidence)
	}
}

func TestApplyUndoAndAuditAreConflictSafeAndSourceFree(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []map[string]any{{"message": map[string]string{"content": `{"version":"v1","target_path":"main.go","target_symbol":"Run","scope_mode":"strict_symbol","candidate_content":"package main\n\nimport \"fmt\"\n\nfunc Run() { fmt.Println(\"changed\") }\n"}`}}}})
	}))
	defer server.Close()
	service, root := newSemanticAnalysisService(t, server.URL, 0)
	analysis, err := service.manager.Analysis()
	if err != nil {
		t.Fatal(err)
	}
	file, err := service.manager.IndexedFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	preview, err := service.Generate(context.Background(), "change Run", "main.go", "Run", "strict_symbol", false)
	if err != nil || !preview.Validation.Applicable {
		t.Fatalf("preview = %+v, %v", preview, err)
	}
	if _, err := service.CheckCandidate(context.Background(), preview.GenerationID, CandidateCheckOptions{}); err != nil {
		t.Fatal(err)
	}
	draft, err := service.Draft(preview.GenerationID)
	if err != nil {
		t.Fatal(err)
	}
	applied, err := service.ApplyDraft(context.Background(), ApplyRequest{DraftID: preview.GenerationID, DraftRevision: draft.Revision, DraftHash: draft.Hash, ProjectID: analysis.ProjectID, ProjectRevision: analysis.ProjectRevision, BaseFileHash: file.ContentHash, Confirm: true})
	if err != nil {
		t.Fatal(err)
	}
	current, err := os.ReadFile(filepath.Join(root, "main.go"))
	if err != nil || string(current) != preview.CandidateContent {
		t.Fatalf("applied content = %q, %v", current, err)
	}
	audits, err := service.AuditHistory()
	if err != nil || len(audits) != 1 || stringMustContain(jsonAudit(t, audits), "changed") {
		t.Fatalf("audit = %+v, %v", audits, err)
	}
	restartedManager, err := project.NewManager(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := restartedManager.Set(root, &project.Analysis{Name: "fixture", Path: root}); err != nil {
		t.Fatal(err)
	}
	restarted, err := New(scopedTestConfig(server.URL), restartedManager)
	if err != nil {
		t.Fatal(err)
	}
	undone, err := restarted.UndoDraft(context.Background(), UndoRequest{ProjectID: analysis.ProjectID, ProjectRevision: applied.ProjectRevision, PostApplyHash: applied.PostApplyHash, Confirm: true})
	if err != nil || undone.UndoAvailable {
		t.Fatalf("undo = %+v, %v", undone, err)
	}
	restored, err := os.ReadFile(filepath.Join(root, "main.go"))
	if err != nil || string(restored) == preview.CandidateContent {
		t.Fatalf("restored content = %q, %v", restored, err)
	}
}

func TestApplyRejectsFileConflict(t *testing.T) {
	service, root := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	analysis, err := service.manager.Analysis()
	if err != nil {
		t.Fatal(err)
	}
	file, err := service.manager.IndexedFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	service.rememberCandidate(&GenerationPreview{GenerationID: "candidate", ProjectID: analysis.ProjectID, ProjectRevision: analysis.ProjectRevision, BaseFileHash: file.ContentHash, TargetPath: "main.go", TargetSymbol: "Run", CandidateContent: "package main\nfunc Run() {}\n", Validation: project.GenerationValidation{Applicable: true}})
	preview, err := service.Candidate("candidate")
	if err != nil {
		t.Fatal(err)
	}
	service.drafts["candidate"].checks = &draftCheckEvidence{Revision: 1, CandidateHash: preview.CandidateHash, Report: CandidateCheckReport{Applicable: true}}
	draft, err := service.Draft("candidate")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc Run() { println(\"external\") }\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ApplyDraft(context.Background(), ApplyRequest{DraftID: "candidate", DraftRevision: draft.Revision, DraftHash: draft.Hash, ProjectID: analysis.ProjectID, ProjectRevision: analysis.ProjectRevision, BaseFileHash: file.ContentHash, Confirm: true}); !errors.Is(err, project.ErrRevisionConflict) {
		t.Fatalf("apply conflict = %v", err)
	}
}

func jsonAudit(t *testing.T, audits []AuditEntry) string {
	t.Helper()
	data, err := json.Marshal(audits)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func stringMustContain(value, unwanted string) bool { return strings.Contains(value, unwanted) }

func runFixtureDraftChecks(service *Service, ctx context.Context, source string, options CandidateCheckOptions, taskTest *project.GoTestCandidateSpec) (CandidateCheckReport, error) {
	file, err := service.manager.IndexedFile("main.go")
	if err != nil {
		return CandidateCheckReport{}, err
	}
	return service.runDraftChecks(ctx, draftCheckInput{file: *file, source: source, taskTest: taskTest}, options)
}
