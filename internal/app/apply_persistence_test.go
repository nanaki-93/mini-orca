package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

func TestApplyPreflightPersistenceFailuresLeaveSourceUntouched(t *testing.T) {
	for _, failure := range []string{"audit", "journal"} {
		t.Run(failure, func(t *testing.T) {
			service, root := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
			draft := createDraftReadyForApply(t, service)
			original := readApplySource(t, root)
			if failure == "audit" {
				if err := writeJSONAtomic(filepath.Join(root, auditRelativePath), "invalid audit"); err != nil {
					t.Fatal(err)
				}
			} else {
				blockMutationMetadata(t, root, pendingApplyRelativePath)
			}
			result, err := service.ApplyDraft(context.Background(), applyDraftRequest(draft))
			if err == nil || result != nil || readApplySource(t, root) != original {
				t.Fatalf("Apply = %+v, %v; preflight failure must not change source", result, err)
			}
		})
	}
}

func TestApplyMetadataFailureReturnsReceiptAndRecoveryUndoSurvivesRestart(t *testing.T) {
	service, root := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	draft := createDraftReadyForApply(t, service)
	original := readApplySource(t, root)
	blockMutationMetadata(t, root, applyStateRelativePath)
	applied, err := service.ApplyDraft(context.Background(), applyDraftRequest(draft))
	if err != nil || applied == nil || !applied.UndoAvailable || len(applied.Warnings) == 0 {
		t.Fatalf("Apply = %+v, %v; want successful mutation with recovery warning", applied, err)
	}
	if contentHash([]byte(readApplySource(t, root))) != applied.PostApplyHash {
		t.Fatal("receipt does not describe the changed source")
	}
	journal, err := os.ReadFile(filepath.Join(root, pendingApplyRelativePath))
	if err != nil || strings.Contains(string(journal), "println") {
		t.Fatalf("missing or source-bearing recovery journal: %v", err)
	}
	restartedManager, err := project.NewManager(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := restartedManager.Set(root, &project.Analysis{Name: "fixture", Path: root}); err != nil {
		t.Fatal(err)
	}
	restarted, err := New(scopedTestConfig("http://127.0.0.1:1"), restartedManager)
	if err != nil {
		t.Fatal(err)
	}
	request := UndoRequest{ProjectID: draft.ProjectID, ProjectRevision: applied.ProjectRevision, PostApplyHash: applied.PostApplyHash, Confirm: true}
	undone, err := restarted.UndoDraft(context.Background(), request)
	if err != nil || undone == nil || undone.UndoAvailable || len(undone.Warnings) == 0 || readApplySource(t, root) != original {
		t.Fatalf("Undo = %+v, %v; want restored source with metadata cleanup warning", undone, err)
	}
	if _, err := restarted.UndoDraft(context.Background(), request); !errors.Is(err, project.ErrRevisionConflict) {
		t.Fatalf("repeated Undo = %v; want conflict", err)
	}
}

func TestApplyAndUndoIndexFailuresReportActualMutation(t *testing.T) {
	service, root := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	draft := createDraftReadyForApply(t, service)
	original := readApplySource(t, root)
	indexPath := filepath.Join(root, ".mini-orca/index.json")
	if err := os.Remove(indexPath); err != nil {
		t.Fatal(err)
	}
	blockMutationMetadata(t, root, ".mini-orca/index.json")
	if _, err := service.TrustProjectExecution(draft.ProjectRevision, true); err != nil {
		t.Fatal(err)
	}
	applied, err := service.ApplyDraft(context.Background(), applyDraftRequest(draft))
	if err != nil || applied == nil || applied.Index != nil || len(applied.Warnings) == 0 || !applied.UndoAvailable {
		t.Fatalf("Apply = %+v, %v; want a receipt despite failed indexing", applied, err)
	}
	if applied.ProjectRevision != draft.ProjectRevision || contentHash([]byte(readApplySource(t, root))) != applied.PostApplyHash {
		t.Fatal("failed indexing must retain the known revision and report the actual source hash")
	}
	if trust, err := service.ExecutionTrust(applied.ProjectRevision, ""); err != nil || trust.Trusted {
		t.Fatalf("execution trust must expire after source mutation despite failed indexing: %+v, %v", trust, err)
	}
	undone, err := service.UndoDraft(context.Background(), UndoRequest{ProjectID: draft.ProjectID, ProjectRevision: applied.ProjectRevision, PostApplyHash: applied.PostApplyHash, Confirm: true})
	if err != nil || undone == nil || undone.Index != nil || undone.UndoAvailable || len(undone.Warnings) == 0 || readApplySource(t, root) != original {
		t.Fatalf("Undo = %+v, %v; want restored source despite failed indexing", undone, err)
	}
}

func TestUndoAuditFailureStillReturnsRestoredSourceReceipt(t *testing.T) {
	service, root := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	draft := createDraftReadyForApply(t, service)
	original := readApplySource(t, root)
	applied, err := service.ApplyDraft(context.Background(), applyDraftRequest(draft))
	if err != nil {
		t.Fatal(err)
	}
	if err := writeJSONAtomic(filepath.Join(root, auditRelativePath), "invalid audit"); err != nil {
		t.Fatal(err)
	}
	undone, err := service.UndoDraft(context.Background(), UndoRequest{ProjectID: draft.ProjectID, ProjectRevision: applied.ProjectRevision, PostApplyHash: applied.PostApplyHash, Confirm: true})
	if err != nil || undone == nil || undone.Audit.Outcome != "undone" || len(undone.Warnings) == 0 || readApplySource(t, root) != original {
		t.Fatalf("Undo = %+v, %v; audit failure must not hide restoration", undone, err)
	}
}

func TestPendingApplyNeverResurrectsPreviousUndo(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("external"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := writeApplyState(root, applyState{TargetPath: "previous.go", AfterHash: "previous"}); err != nil {
		t.Fatal(err)
	}
	if err := writeJSONAtomic(filepath.Join(root, pendingApplyRelativePath), applyJournal{State: applyState{TargetPath: "main.go", AfterHash: "expected"}}); err != nil {
		t.Fatal(err)
	}
	if state, err := readApplyState(root); state != nil || !errors.Is(err, project.ErrRevisionConflict) {
		t.Fatalf("Undo state = %+v, %v; want conflict rather than previous Apply", state, err)
	}
}

func blockMutationMetadata(t *testing.T, root, relative string) {
	t.Helper()
	path := filepath.Join(root, relative)
	if err := os.MkdirAll(path, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "blocker"), []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
}

func readApplySource(t *testing.T, root string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
