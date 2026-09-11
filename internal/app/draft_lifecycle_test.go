package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

func TestDraftCreationPreviewApplyUndoPreservesExistingSourceAndImports(t *testing.T) {
	for _, source := range []string{
		"package main\n",
		"package main\n\nimport \"fmt\"\n\n// Existing remains unchanged.\nfunc Existing() { fmt.Println(\"keep\") }\n",
	} {
		t.Run(source, func(t *testing.T) {
			service, root, generated := createGeneratedFunctionFixture(t, source, "Build", `func Build() string { return strings.TrimSpace(" ready ") }`, []string{"strings"})
			otherPath := filepath.Join(root, "notes.txt")
			if err := os.WriteFile(otherPath, []byte("untouched"), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := service.ApplyDraft(context.Background(), applyDraftRequest(generated)); err == nil {
				t.Fatal("applied creation before validation and checks")
			}
			validated, err := service.ValidateDraft(generated.ID, generated.Revision)
			if err != nil || validated.State != DraftValid {
				t.Fatalf("validation = %+v, %v", validated, err)
			}
			if _, err := service.ApplyDraft(context.Background(), applyDraftRequest(validated)); err == nil {
				t.Fatal("applied creation before checks")
			}
			checks, err := service.CheckDraft(context.Background(), DraftCheckRequest{ID: validated.ID, ExpectedRevision: validated.Revision, ExpectedHash: validated.Hash})
			if err != nil || !checks.Applicable || checks.CompositionHash != validated.CompositionHash {
				t.Fatalf("creation checks = %+v, %v", checks, err)
			}
			assertCreationSource(t, root, source)
			request := applyDraftRequest(validated)
			request.Confirm = false
			if _, err := service.ApplyDraft(context.Background(), request); err == nil {
				t.Fatal("applied creation without confirmation")
			}
			assertCreationSource(t, root, source)
			applied, err := service.ApplyDraft(context.Background(), applyDraftRequest(validated))
			if err != nil {
				t.Fatal(err)
			}
			content, err := os.ReadFile(filepath.Join(root, "main.go"))
			if err != nil || !strings.Contains(string(content), `func Build() string { return strings.TrimSpace(" ready ") }`) || !strings.Contains(string(content), `"strings"`) {
				t.Fatalf("applied creation = %q, %v", content, err)
			}
			if strings.Contains(source, "Existing") && (!strings.Contains(string(content), "// Existing remains unchanged.\nfunc Existing() { fmt.Println(\"keep\") }") || !strings.Contains(string(content), `import "fmt"`)) {
				t.Fatalf("unrelated declarations/imports changed: %s", content)
			}
			if _, err := service.UndoDraft(context.Background(), UndoRequest{ProjectID: validated.ProjectID, ProjectRevision: applied.ProjectRevision, PostApplyHash: applied.PostApplyHash, Confirm: true}); err != nil {
				t.Fatal(err)
			}
			assertCreationSource(t, root, source)
			other, err := os.ReadFile(otherPath)
			if err != nil || string(other) != "untouched" {
				t.Fatalf("unrelated file changed: %q, %v", other, err)
			}
		})
	}
}

func TestDraftCreationRejectsSourceChangeAfterChecks(t *testing.T) {
	service, root, generated := createGeneratedFunctionFixture(t, "package main\n", "Build", "func Build() {}", nil)
	validated, err := service.ValidateDraft(generated.ID, generated.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.CheckDraft(context.Background(), DraftCheckRequest{ID: validated.ID, ExpectedRevision: validated.Revision, ExpectedHash: validated.Hash}); err != nil {
		t.Fatal(err)
	}
	external := "package main\n\nfunc AddedOutside() {}\n"
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte(external), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ApplyDraft(context.Background(), applyDraftRequest(validated)); !errors.Is(err, project.ErrRevisionConflict) {
		t.Fatalf("stale creation Apply = %v", err)
	}
	assertCreationSource(t, root, external)
	stale, err := service.Draft(generated.ID)
	if err != nil || stale.State != DraftStale {
		t.Fatalf("stale creation draft = %+v, %v", stale, err)
	}
}

func TestDraftLifecycleInvalidatesApprovalEvidenceAndRejectsLateValidation(t *testing.T) {
	service, _ := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	draft := createFixtureDraft(t, service, "draft-lifecycle", "")

	valid := validateFixtureDraft(t, service, draft.ID, draft.Revision, true)
	if valid.State != DraftValid || valid.Validation == nil {
		t.Fatalf("validated draft = %+v", valid)
	}
	service.drafts.mu.Lock()
	service.drafts.records[draft.ID].checks = &draftCheckEvidence{Revision: valid.Revision, CompositionHash: "composition", Report: DraftCheckReport{Applicable: true}}
	service.drafts.mu.Unlock()

	dirty, err := service.UpdateDraft(DraftUpdateRequest{ID: draft.ID, ExpectedRevision: valid.Revision, Declaration: "func Run( {", Imports: []string{"fmt"}})
	if err != nil {
		t.Fatal(err)
	}
	service.drafts.mu.Lock()
	checksCleared := service.drafts.records[draft.ID].checks == nil
	service.drafts.mu.Unlock()
	if dirty.State != DraftDirty || dirty.Validation != nil || !checksCleared || dirty.PreviousHash == "" || dirty.Hash == valid.Hash {
		t.Fatalf("edited draft did not clear approval evidence: %+v", dirty)
	}
	invalid := validateFixtureDraft(t, service, dirty.ID, dirty.Revision, false)
	if invalid.State != DraftInvalid || invalid.Declaration != "func Run( {" {
		t.Fatalf("invalid draft should remain editable: %+v", invalid)
	}

	recovered, err := service.UpdateDraft(DraftUpdateRequest{ID: draft.ID, ExpectedRevision: invalid.Revision, Declaration: "func Run() { println(\"recovered\") }"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.CompleteDraftValidation(draft.ID, invalid.Revision, project.DeclarationValidation{Applicable: true}); !errors.Is(err, project.ErrRevisionConflict) {
		t.Fatalf("late validation error = %v, want revision conflict", err)
	}
	if got := validateFixtureDraft(t, service, recovered.ID, recovered.Revision, true); got.State != DraftValid {
		t.Fatalf("recovered draft = %+v", got)
	}
}

func TestDraftUpdatesAreRevisionGuardedUnderConcurrency(t *testing.T) {
	service, _ := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	draft := createFixtureDraft(t, service, "draft-concurrent", "")

	var group sync.WaitGroup
	errorsByUpdate := make(chan error, 2)
	for _, declaration := range []string{"func Run() { println(\"left\") }", "func Run() { println(\"right\") }"} {
		group.Add(1)
		go func(declaration string) {
			defer group.Done()
			_, err := service.UpdateDraft(DraftUpdateRequest{ID: draft.ID, ExpectedRevision: draft.Revision, Declaration: declaration})
			errorsByUpdate <- err
		}(declaration)
	}
	group.Wait()
	close(errorsByUpdate)

	successes := 0
	conflicts := 0
	for err := range errorsByUpdate {
		if err == nil {
			successes++
		} else if errors.Is(err, project.ErrRevisionConflict) {
			conflicts++
		} else {
			t.Fatalf("update error = %v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("successes=%d conflicts=%d", successes, conflicts)
	}
	current, err := service.Draft(draft.ID)
	if err != nil {
		t.Fatal(err)
	}
	if current.Revision != draft.Revision+1 || current.State != DraftDirty {
		t.Fatalf("current draft = %+v", current)
	}
}

func TestDraftLineageAndOpenFileInvalidation(t *testing.T) {
	service, _ := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	parent := createFixtureDraft(t, service, "draft-parent", "")
	child := createFixtureDraft(t, service, "draft-child", parent.ID)
	if child.ParentDraftID != parent.ID {
		t.Fatalf("draft lineage = %+v", child)
	}
	analysis, err := service.manager.Analysis()
	if err != nil {
		t.Fatal(err)
	}
	service.ExpireDraftsForOpenFile(analysis.ProjectID, analysis.ProjectRevision, "other.go", "other-hash")
	stale, err := service.Draft(parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stale.State != DraftStale {
		t.Fatalf("open-file switch did not stale draft: %+v", stale)
	}
}

func TestDraftStalesOnFileChangeAndProjectSwitchClearsSource(t *testing.T) {
	service, root := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	draft := createFixtureDraft(t, service, "draft-switch", "")
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc Run() { println(\"external\") }\n"), 0600); err != nil {
		t.Fatal(err)
	}
	stale, err := service.Draft(draft.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stale.State != DraftStale {
		t.Fatalf("file change did not stale draft: %+v", stale)
	}

	newRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(newRoot, "next.go"), []byte("package next\nfunc Next() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := service.ActivateProject(newRoot, &project.Analysis{Name: "next", Path: newRoot}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Draft(draft.ID); err == nil {
		t.Fatal("project switch retained editable draft source")
	}
}

func createFixtureDraft(t *testing.T, service *Service, id, parentID string) *Draft {
	t.Helper()
	analysis, err := service.manager.Analysis()
	if err != nil {
		t.Fatal(err)
	}
	file, err := service.manager.IndexedFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	draft, err := service.CreateDraft(DraftCreateRequest{ID: id, ProjectID: analysis.ProjectID, ProjectRevision: analysis.ProjectRevision, BaseFileHash: file.ContentHash, TargetPath: "main.go", Mode: project.DeclarationEditReplaceSymbol, TargetSymbol: "Run", Declaration: "func Run() { println(\"generated\") }", Imports: []string{"fmt"}, ParentDraftID: parentID})
	if err != nil {
		t.Fatal(err)
	}
	return draft
}

func validateFixtureDraft(t *testing.T, service *Service, id string, revision int64, applicable bool) *Draft {
	t.Helper()
	if _, err := service.BeginDraftValidation(id, revision); err != nil {
		t.Fatal(err)
	}
	draft, err := service.CompleteDraftValidation(id, revision, project.DeclarationValidation{Applicable: applicable})
	if err != nil {
		t.Fatal(err)
	}
	return draft
}
