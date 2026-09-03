package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

func TestApplyDraftRejectsSourceChangedAfterChecksWithoutWriting(t *testing.T) {
	service, root := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	draft := createDraftReadyForApply(t, service)
	external := "package main\n\nfunc Run() { println(\"external\") }\n"
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte(external), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := service.ApplyDraft(context.Background(), applyDraftRequest(draft))
	if !errors.Is(err, project.ErrRevisionConflict) {
		t.Fatalf("ApplyDraft() error = %v, want revision conflict", err)
	}
	after, err := os.ReadFile(filepath.Join(root, "main.go"))
	if err != nil || string(after) != external {
		t.Fatalf("late source change was overwritten: %q, %v", after, err)
	}
}

func TestApplyDraftCancellationAndFailedChecksLeaveSourceUntouched(t *testing.T) {
	service, root := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	draft := createDraftReadyForApply(t, service)
	original, err := os.ReadFile(filepath.Join(root, "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.ApplyDraft(canceled, applyDraftRequest(draft)); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled ApplyDraft() error = %v", err)
	}
	service.draftMu.Lock()
	service.drafts[draft.ID].checks.Report.Applicable = false
	service.draftMu.Unlock()
	if _, err := service.ApplyDraft(context.Background(), applyDraftRequest(draft)); err == nil {
		t.Fatal("expected ApplyDraft() to reject failed required checks")
	}
	after, err := os.ReadFile(filepath.Join(root, "main.go"))
	if err != nil || string(after) != string(original) {
		t.Fatalf("non-writing Apply stages changed source: %q, %v", after, err)
	}
}

func createDraftReadyForApply(t *testing.T, service *Service) *Draft {
	t.Helper()
	analysis, err := service.manager.Analysis()
	if err != nil {
		t.Fatal(err)
	}
	file, err := service.manager.IndexedFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	draft, err := service.CreateDraft(DraftCreateRequest{ID: "apply-stage", ProjectID: analysis.ProjectID, ProjectRevision: analysis.ProjectRevision, BaseFileHash: file.ContentHash, TargetPath: "main.go", Mode: project.DeclarationEditReplaceSymbol, TargetSymbol: "Run", Declaration: "func Run() { println(\"draft\") }"})
	if err != nil {
		t.Fatal(err)
	}
	validated, err := service.ValidateDraft(draft.ID, draft.Revision)
	if err != nil || validated.State != DraftValid {
		t.Fatalf("ValidateDraft() = %+v, %v", validated, err)
	}
	if _, err := service.CheckDraft(context.Background(), DraftCheckRequest{ID: validated.ID, ExpectedRevision: validated.Revision, ExpectedHash: validated.Hash}); err != nil {
		t.Fatalf("CheckDraft() error = %v", err)
	}
	return validated
}
