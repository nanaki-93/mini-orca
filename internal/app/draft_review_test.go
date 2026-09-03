package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

func TestDraftValidateCheckApplyAndUndoRequireCurrentEvidence(t *testing.T) {
	service, root := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	original, err := os.ReadFile(filepath.Join(root, "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	draft := createFixtureDraft(t, service, "draft-review", "")
	updated, err := service.UpdateDraft(DraftUpdateRequest{ID: draft.ID, ExpectedRevision: draft.Revision, Declaration: "func Run() { println(\"ready\") }"})
	if err != nil {
		t.Fatal(err)
	}
	validated, err := service.ValidateDraft(updated.ID, updated.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if validated.State != DraftValid || validated.CompositionHash == "" {
		t.Fatalf("validated draft = %+v", validated)
	}
	checks, err := service.CheckDraft(context.Background(), DraftCheckRequest{ID: validated.ID, ExpectedRevision: validated.Revision, ExpectedHash: validated.Hash})
	if err != nil {
		t.Fatal(err)
	}
	if !checks.Applicable || checks.DraftID != validated.ID || checks.DraftRevision != validated.Revision || checks.DraftHash != validated.Hash || checks.CompositionHash != validated.CompositionHash {
		t.Fatalf("checks are not bound to the draft: %+v", checks)
	}
	dirty, err := service.UpdateDraft(DraftUpdateRequest{ID: validated.ID, ExpectedRevision: validated.Revision, Declaration: "func Run() { println(\"edited\") }"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ApplyDraft(context.Background(), applyDraftRequest(validated)); !errors.Is(err, project.ErrRevisionConflict) {
		t.Fatalf("stale apply error = %v", err)
	}
	current, err := os.ReadFile(filepath.Join(root, "main.go"))
	if err != nil || string(current) != string(original) {
		t.Fatalf("stale Apply changed source: %q, %v", current, err)
	}
	validated, err = service.ValidateDraft(dirty.ID, dirty.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.CheckDraft(context.Background(), DraftCheckRequest{ID: validated.ID, ExpectedRevision: validated.Revision, ExpectedHash: "sha256:stale"}); !errors.Is(err, project.ErrRevisionConflict) {
		t.Fatalf("hash-mismatched checks error = %v", err)
	}
	if _, err := service.CheckDraft(context.Background(), DraftCheckRequest{ID: validated.ID, ExpectedRevision: validated.Revision, ExpectedHash: validated.Hash}); err != nil {
		t.Fatal(err)
	}
	applied, err := service.ApplyDraft(context.Background(), applyDraftRequest(validated))
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(root, "main.go"))
	if err != nil || string(content) != "package main\n\nimport \"fmt\"\n\nfunc Run() { println(\"edited\") }\n" {
		t.Fatalf("applied content = %q, %v", content, err)
	}
	if _, err := service.UndoDraft(context.Background(), UndoRequest{ProjectID: validated.ProjectID, ProjectRevision: applied.ProjectRevision, PostApplyHash: applied.PostApplyHash, Confirm: true}); err != nil {
		t.Fatal(err)
	}
}

func TestInvalidDraftCannotBeCheckedOrApplied(t *testing.T) {
	service, _ := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	draft := createFixtureDraft(t, service, "draft-invalid-review", "")
	dirty, err := service.UpdateDraft(DraftUpdateRequest{ID: draft.ID, ExpectedRevision: draft.Revision, Declaration: "func Run( {"})
	if err != nil {
		t.Fatal(err)
	}
	invalid, err := service.ValidateDraft(dirty.ID, dirty.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if invalid.State != DraftInvalid {
		t.Fatalf("invalid draft = %+v", invalid)
	}
	if _, err := service.CheckDraft(context.Background(), DraftCheckRequest{ID: invalid.ID, ExpectedRevision: invalid.Revision, ExpectedHash: invalid.Hash}); err == nil {
		t.Fatal("expected invalid draft checks to fail")
	}
	if _, err := service.ApplyDraft(context.Background(), applyDraftRequest(invalid)); err == nil {
		t.Fatal("expected invalid draft Apply to fail")
	}
}

func applyDraftRequest(draft *Draft) ApplyRequest {
	return ApplyRequest{DraftID: draft.ID, DraftRevision: draft.Revision, DraftHash: draft.Hash, ProjectID: draft.ProjectID, ProjectRevision: draft.ProjectRevision, BaseFileHash: draft.BaseFileHash, Confirm: true}
}
