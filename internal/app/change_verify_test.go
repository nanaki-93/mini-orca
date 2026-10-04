package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

func appliedVerificationFixture(t *testing.T, failTests bool) (*Service, string, *ChangeSession, ChangeIdentity) {
	t.Helper()
	server := changeProvider(t, func() string { return groupedChangeResponse })
	service, root := newSemanticAnalysisService(t, server.URL, 0)
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.test/verification\n\ngo 1.22\n"), 0644); err != nil {
		t.Fatal(err)
	}
	test := "package main\nimport \"testing\"\nfunc TestRun(t *testing.T) { Run() }\n"
	if failTests {
		test = "package main\nimport \"testing\"\nfunc TestRun(t *testing.T) { t.Fatal(\"regression\") }\n"
	}
	if err := os.WriteFile(filepath.Join(root, "main_test.go"), []byte(test), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Reindex(); err != nil {
		t.Fatal(err)
	}
	session := approveChangeFixture(t, service, prepareChangeFixture(t, service, "main.go", "docs/change.md"))
	receipt, err := service.ApplyChange(context.Background(), session.ID, ChangeApplyRequest{ChangeIdentity: changeIdentity(session), Confirm: true})
	if err != nil {
		t.Fatal(err)
	}
	identity := changeIdentity(session)
	identity.ProjectRevision = receipt.ProjectRevision
	return service, root, session, identity
}

func TestChangeVerifyRequiresFreshTrustAndPersistsEvidence(t *testing.T) {
	service, root, session, identity := appliedVerificationFixture(t, false)
	if _, err := service.VerifyChange(context.Background(), session.ID, identity); err == nil {
		t.Fatal("verification ran without post-Apply execution trust")
	}
	if _, err := service.TrustProjectExecution(identity.ProjectRevision, true); err != nil {
		t.Fatal(err)
	}
	stale := identity
	stale.Hash = "old-proposal"
	if _, err := service.VerifyChange(context.Background(), session.ID, stale); !errors.Is(err, project.ErrRevisionConflict) {
		t.Fatalf("stale proposal accepted: %v", err)
	}
	verified, err := service.VerifyChange(context.Background(), session.ID, identity)
	if err != nil || verified.Status != "verified" || !requiredChecksPassed(verified.Checks) {
		t.Fatalf("verification: %+v, %v", verified, err)
	}
	restarted, err := New(scopedTestConfig("http://127.0.0.1:1"), service.manager)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := restarted.ChangeRecovery()
	if err != nil || receipt.Verification == nil || receipt.Verification.Status != "verified" {
		t.Fatalf("persisted evidence: %+v, %v", receipt, err)
	}
	if _, err := restarted.VerifyChange(context.Background(), session.ID, identity); err == nil {
		t.Fatal("restored verification granted execution trust")
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc Run() { println(\"independent\") }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	receipt, err = restarted.ChangeRecovery()
	if err != nil || receipt.Verification.Status != "stale" {
		t.Fatalf("changed source retained verification: %+v, %v", receipt, err)
	}
}

func TestChangeVerifyFailedCanceledAndUnavailableKeepAppliedFiles(t *testing.T) {
	service, root, session, identity := appliedVerificationFixture(t, true)
	if _, err := service.TrustProjectExecution(identity.ProjectRevision, true); err != nil {
		t.Fatal(err)
	}
	evidence, err := service.VerifyChange(context.Background(), session.ID, identity)
	if err != nil || evidence.Status != "failed" {
		t.Fatalf("failed tests reported success: %+v, %v", evidence, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	evidence, err = service.verifyChangeWithChecks(ctx, session.ID, identity, func(context.Context, string, *changeJournal, string) ([]DraftCheck, error) {
		cancel()
		return []DraftCheck{{Name: "tests", Required: true, State: CheckCanceled}}, ctx.Err()
	})
	if err != nil || evidence.Status != "canceled" {
		t.Fatalf("cancellation lost: %+v, %v", evidence, err)
	}
	evidence, err = service.verifyChangeWithChecks(context.Background(), session.ID, identity, func(context.Context, string, *changeJournal, string) ([]DraftCheck, error) {
		return nil, errors.New("workspace unavailable")
	})
	if err != nil || evidence.Status != "unavailable" {
		t.Fatalf("unavailable checks reported success: %+v, %v", evidence, err)
	}
	if evidence.Checks == nil {
		t.Fatal("unavailable evidence has no readable check list")
	}
	data, _ := os.ReadFile(filepath.Join(root, "main.go"))
	if string(data) != session.Changes[0].Content {
		t.Fatal("verification changed source")
	}
	receipt, err := service.ChangeRecovery()
	if err != nil || receipt.State != "applied" || receipt.Verification.Status != "unavailable" {
		t.Fatalf("receipt: %+v, %v", receipt, err)
	}
}

func TestChangeVerifyRejectsLateEvidenceAfterSourceChanges(t *testing.T) {
	service, root, session, identity := appliedVerificationFixture(t, false)
	if _, err := service.TrustProjectExecution(identity.ProjectRevision, true); err != nil {
		t.Fatal(err)
	}
	_, err := service.verifyChangeWithChecks(context.Background(), session.ID, identity, func(context.Context, string, *changeJournal, string) ([]DraftCheck, error) {
		if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n// independent work\n"), 0644); err != nil {
			t.Fatal(err)
		}
		return []DraftCheck{{Name: "tests", Required: true, State: CheckPassed}}, nil
	})
	if !errors.Is(err, project.ErrRevisionConflict) {
		t.Fatalf("late verification became current evidence: %v", err)
	}
	receipt, _ := service.ChangeRecovery()
	if receipt.Verification != nil {
		t.Fatal("late evidence persisted")
	}
}
