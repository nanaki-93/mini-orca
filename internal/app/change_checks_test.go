package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
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
