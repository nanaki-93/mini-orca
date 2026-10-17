package app

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

func prepareChangeFixture(t *testing.T, service *Service, paths ...string) *ChangeSession {
	t.Helper()
	session := openChangeFixture(t, service, paths...)
	proposal, err := service.SendChangeMessage(context.Background(), session.ID, ChangeMessageRequest{ChangeIdentity: changeIdentity(session), Message: "Implement the change."})
	if err != nil {
		t.Fatal(err)
	}
	return proposal
}

func approveChangeFixture(t *testing.T, service *Service, session *ChangeSession) *ChangeSession {
	t.Helper()
	checked, err := service.CheckChange(context.Background(), session.ID, ChangeCheckRequest{ChangeIdentity: changeIdentity(session)})
	if err != nil || !requiredChecksPassed(checked.Checks) {
		t.Fatalf("checks: %+v, %v", checked, err)
	}
	reviewed, err := service.ReviewChange(context.Background(), session.ID, changeIdentity(checked))
	if err != nil {
		t.Fatal(err)
	}
	return reviewed
}

func TestChangeChecksRequireTrustAndProveNewTests(t *testing.T) {
	server := changeProvider(t, func() string {
		return `{"explanation":"Add helper and regression test.","changes":[{"path":"new.go","content":"package main\nfunc Helper() int { return 2 }\n"},{"path":"new_test.go","content":"package main\nimport \"testing\"\nfunc TestHelper(t *testing.T) { if Helper() != 2 { t.Fatal(\"wrong result\") } }\n"}]}`
	})
	service, root := newSemanticAnalysisService(t, server.URL, 0)
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.test/change\n\ngo 1.22\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Reindex(); err != nil {
		t.Fatal(err)
	}
	session := prepareChangeFixture(t, service, "new.go", "new_test.go")
	request := ChangeCheckRequest{ChangeIdentity: changeIdentity(session), CheckOptions: CheckOptions{RunTests: true}}
	if _, err := service.CheckChange(context.Background(), session.ID, request); err == nil {
		t.Fatal("checks ran without trust")
	}
	if _, err := service.TrustProjectExecution(session.ProjectRevision, true); err != nil {
		t.Fatal(err)
	}
	checked, err := service.CheckChange(context.Background(), session.ID, request)
	if err != nil || !requiredChecksPassed(checked.Checks) {
		t.Fatalf("trusted checks: %+v, %v", checked, err)
	}
	var baseline, tests bool
	for _, check := range checked.Checks {
		if check.Name == "regression baseline" {
			baseline = check.State == CheckPassed
		}
		if check.Name == "tests" {
			tests = check.State == CheckPassed
		}
	}
	if !baseline || !tests {
		t.Fatalf("missing baseline/candidate evidence: %+v", checked.Checks)
	}
	if _, err := os.Stat(filepath.Join(root, "new_test.go")); !os.IsNotExist(err) {
		t.Fatal("checks wrote project test")
	}
}

func TestChangeFailedChecksRepairAndApprovalInvalidation(t *testing.T) {
	var reply atomic.Value
	reply.Store(`{"explanation":"Incomplete candidate.","changes":[{"path":"new.go","content":"package main\nfunc Broken( {\n"}]}`)
	server := changeProvider(t, func() string { return reply.Load().(string) })
	service, _ := newSemanticAnalysisService(t, server.URL, 0)
	session := prepareChangeFixture(t, service, "new.go")
	checked, err := service.CheckChange(context.Background(), session.ID, ChangeCheckRequest{ChangeIdentity: changeIdentity(session)})
	if err != nil || requiredChecksPassed(checked.Checks) {
		t.Fatalf("failed checks lost: %+v, %v", checked, err)
	}
	if _, err := service.ReviewChange(context.Background(), session.ID, changeIdentity(checked)); err == nil {
		t.Fatal("failed checks authorized review")
	}
	reply.Store(`{"explanation":"Complete candidate.","changes":[{"path":"new.go","content":"package main\nfunc Ready() {}\n"}]}`)
	repaired, err := service.RepairChangeMessage(context.Background(), session.ID, ChangeMessageRequest{ChangeIdentity: changeIdentity(checked), Repair: true})
	if err != nil || repaired.RepairAttempts != 1 || len(repaired.Checks) != 0 || repaired.ReviewedHash != "" {
		t.Fatalf("repair: %+v, %v", repaired, err)
	}
	approved := approveChangeFixture(t, service, repaired)
	if _, err := service.ResumeChange(context.Background(), session.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ApplyChange(context.Background(), session.ID, ChangeApplyRequest{ChangeIdentity: changeIdentity(approved), Confirm: true}); err == nil || !strings.Contains(err.Error(), "review") {
		t.Fatalf("historical approval reused: %v", err)
	}
}

func TestReviewChangeAcrossRestartAndDirectHunkEdits(t *testing.T) {
	server := changeProvider(t, func() string { return groupedChangeResponse })
	service, _ := newSemanticAnalysisService(t, server.URL, 0)
	session := prepareChangeFixture(t, service, "main.go", "docs/change.md")
	checked, err := service.CheckChange(context.Background(), session.ID, ChangeCheckRequest{ChangeIdentity: changeIdentity(session)})
	if err != nil || !requiredChecksPassed(checked.Checks) {
		t.Fatalf("checks: %+v, %v", checked, err)
	}

	restarted, err := New(scopedTestConfig(server.URL), service.manager)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := restarted.ApplyChange(context.Background(), checked.ID, ChangeApplyRequest{ChangeIdentity: changeIdentity(checked), Confirm: true}); err == nil {
		t.Fatal("apply succeeded without review after restart")
	}
	reviewed, err := restarted.ReviewChange(context.Background(), checked.ID, changeIdentity(checked))
	if err != nil || reviewed.ReviewedHash != checked.Hash {
		t.Fatalf("review after restart failed: %+v, %v", reviewed, err)
	}
	reviewedAgain, err := restarted.ReviewChange(context.Background(), reviewed.ID, changeIdentity(reviewed))
	if err != nil || reviewedAgain.ReviewedHash != reviewed.Hash {
		t.Fatalf("repeat review failed: %+v, %v", reviewedAgain, err)
	}
}

func TestNonGoJustfileChecksAndTestsExecuteUnderTrust(t *testing.T) {
	binDir := t.TempDir()
	fakeJust := filepath.Join(binDir, "just")
	script := "#!/bin/sh\nif [ \"$1\" = \"test\" ]; then\n  grep -q \"return 42\" app.ts || { echo \"FAIL: expected 42 in app.ts\"; exit 1; }\n  echo \"PASS: app.test.ts\"\n  exit 0\nfi\nif [ \"$1\" = \"check\" ]; then\n  echo \"check ok\"\n  exit 0\nfi\nexit 1\n"
	if err := os.WriteFile(fakeJust, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	var reply atomic.Value
	reply.Store(`{"explanation":"Initial implementation.","changes":[{"path":"app.ts","content":"export function run() { return 0; }\n"},{"path":"app.test.ts","content":"import { run } from './app';\n"}]}`)
	server := changeProvider(t, func() string { return reply.Load().(string) })

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"name":"ts-just"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "justfile"), []byte("setup:\n  npm install\ndev:\n  npm run dev\ntest:\n  npm test\ncheck:\n  npm run check\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "app.ts"), []byte("export function run() { return 0; }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	manager, err := project.NewManager(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Set(root, &project.Analysis{Name: "ts-just", Path: root, Type: "typescript"}); err != nil {
		t.Fatal(err)
	}
	service, err := New(scopedTestConfig(server.URL), manager)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(service.CancelChangeWorkflows)
	analysis, err := service.manager.Analysis()
	if err != nil {
		t.Fatal(err)
	}
	trust, err := service.ExecutionTrust(analysis.ProjectRevision, "")
	if err != nil || len(trust.Commands) != 2 || strings.Join(trust.Commands[0], " ") != "just test" || strings.Join(trust.Commands[1], " ") != "just check" {
		t.Fatalf("expected just test and just check in execution trust commands, got %+v, err=%v", trust, err)
	}
	if _, err := service.TrustProjectExecution(analysis.ProjectRevision, true); err != nil {
		t.Fatal(err)
	}
	session := prepareChangeFixture(t, service, "app.ts", "app.test.ts")
	failedCheck, err := service.CheckChange(context.Background(), session.ID, ChangeCheckRequest{
		ChangeIdentity: changeIdentity(session),
		CheckOptions:   CheckOptions{RunTests: true, RunLint: true},
	})
	if err != nil || requiredChecksPassed(failedCheck.Checks) {
		t.Fatalf("expected just test to fail when app.ts returns 0, got %+v, err=%v", failedCheck, err)
	}
	reply.Store(`{"explanation":"Return 42.","changes":[{"path":"app.ts","content":"export function run() { return 42; }\n"},{"path":"app.test.ts","content":"import { run } from './app';\n"}]}`)
	repaired, err := service.RepairChangeMessage(context.Background(), failedCheck.ID, ChangeMessageRequest{
		ChangeIdentity: changeIdentity(failedCheck),
		Repair:         true,
	})
	if err != nil {
		t.Fatal(err)
	}
	passedCheck, err := service.CheckChange(context.Background(), repaired.ID, ChangeCheckRequest{
		ChangeIdentity: changeIdentity(repaired),
		CheckOptions:   CheckOptions{RunTests: true, RunLint: true},
	})
	if err != nil || !requiredChecksPassed(passedCheck.Checks) {
		t.Fatalf("expected just test and just check to pass after repair, got %+v, err=%v", passedCheck, err)
	}
}

func TestChangeChecksEnforceArchitectureBoundariesAndComplexityGate(t *testing.T) {
	overlyComplexGo := "package api\n\nimport \"example.test/internal/storage\"\n\nfunc Handle(x int) int {\n\t_ = storage.Name\n\tif x == 1 {\n\t\treturn 1\n\t}\n\tif x == 2 {\n\t\treturn 2\n\t}\n\tif x == 3 {\n\t\treturn 3\n\t}\n\tif x == 4 {\n\t\treturn 4\n\t}\n\tif x == 5 {\n\t\treturn 5\n\t}\n\tif x == 6 {\n\t\treturn 6\n\t}\n\tif x == 7 {\n\t\treturn 7\n\t}\n\tif x == 8 {\n\t\treturn 8\n\t}\n\tif x == 9 {\n\t\treturn 9\n\t}\n\tif x == 10 {\n\t\treturn 10\n\t}\n\tif x == 11 {\n\t\treturn 11\n\t}\n\tif x == 12 {\n\t\treturn 12\n\t}\n\tif x == 13 {\n\t\treturn 13\n\t}\n\tif x == 14 {\n\t\treturn 14\n\t}\n\tif x == 15 {\n\t\treturn 15\n\t}\n\treturn 0\n}\n"
	var reply atomic.Value
	reply.Store(`{"explanation":"Violating candidate.","changes":[{"path":"internal/api/handler.go","content":` + strconv.Quote(overlyComplexGo) + `}]}`)
	server := changeProvider(t, func() string { return reply.Load().(string) })
	service, root := newSemanticAnalysisService(t, server.URL, 0)
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("# Agent Contract\n- `internal/api/` must never import `internal/storage` or `internal/llm`.\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Reindex(); err != nil {
		t.Fatal(err)
	}
	session := prepareChangeFixture(t, service, "internal/api/handler.go")
	checked, err := service.CheckChange(context.Background(), session.ID, ChangeCheckRequest{ChangeIdentity: changeIdentity(session)})
	if err != nil || requiredChecksPassed(checked.Checks) {
		t.Fatalf("expected architecture and complexity checks to fail, got %+v, err=%v", checked, err)
	}
	var archFailed, complexityFailed bool
	for _, check := range checked.Checks {
		if check.Name == "architecture boundaries" && check.State == CheckFailed && strings.Contains(check.Output, "internal/storage") {
			archFailed = true
		}
		if check.Name == "complexity gate" && check.State == CheckFailed && strings.Contains(check.Output, "cyclomatic complexity 16") {
			complexityFailed = true
		}
	}
	if !archFailed || !complexityFailed {
		t.Fatalf("expected both architecture boundaries and complexity gate to fail with diagnostics, got %+v", checked.Checks)
	}

	cleanGo := "package api\n\nfunc Handle(x int) int {\n\tif x == 1 {\n\t\treturn 1\n\t}\n\treturn 0\n}\n"
	reply.Store(`{"explanation":"Clean candidate.","changes":[{"path":"internal/api/handler.go","content":` + strconv.Quote(cleanGo) + `}]}`)
	repaired, err := service.RepairChangeMessage(context.Background(), checked.ID, ChangeMessageRequest{
		ChangeIdentity: changeIdentity(checked),
		Repair:         true,
	})
	if err != nil {
		t.Fatal(err)
	}
	passed, err := service.CheckChange(context.Background(), repaired.ID, ChangeCheckRequest{ChangeIdentity: changeIdentity(repaired)})
	if err != nil || !requiredChecksPassed(passed.Checks) {
		t.Fatalf("expected clean candidate to pass architecture and complexity checks, got %+v, err=%v", passed, err)
	}
}
