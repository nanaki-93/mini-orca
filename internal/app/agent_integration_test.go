package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
)

// This crosses the native workflow, private journal, human checkpoint and
// source-mutation owners. Provider replies are fixtures; project checks are real.
func TestAgentIntegrationFromNativeWorkflowThroughRestartApplyAndUndo(t *testing.T) {
	var calls atomic.Int32
	outputs := [3]func() string{}
	for i, output := range []string{workflowImplementation, workflowTests, workflowApproval} {
		outputs[i] = func() string { calls.Add(1); return output }
	}
	s, root, initial := workflowService(t, outputs)
	original, err := os.ReadFile(filepath.Join(root, "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.StartChangeWorkflow(t.Context(), initial.ID, workflowRequest(initial)); err != nil {
		t.Fatal(err)
	}
	ready := finishedWorkflow(t, s, initial.ID)
	if ready.Workflow.Status != "awaiting_human_review" || ready.Workflow.Input == nil || !requiredChecksPassed(ready.Checks) {
		t.Fatal("native workflow did not reach a checked human checkpoint")
	}
	modelCalls := calls.Load()
	assertIntegratedInspectionIsPassive(t, root)
	request := ChangeApplyRequest{ChangeIdentity: identityForChange(ready), Confirm: true}
	if _, err := s.ApplyChange(t.Context(), ready.ID, request); err == nil {
		t.Fatal("native completion or inspection authorized source mutation")
	}
	approved, err := s.RespondChangeInput(t.Context(), ready.ID, inputResponse(ready, "approve"))
	if err != nil {
		t.Fatal(err)
	}
	received := approved.Workflow.Input.ReceivedAt
	assertGeneratedFilesAbsent(t, root)

	restarted, err := New(s.config, s.manager)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(restarted.CancelChangeWorkflows)
	if _, err := restarted.ApplyChange(t.Context(), ready.ID, request); err == nil {
		t.Fatal("restart inherited mutation authority from journal history")
	}
	if _, err := restarted.ResumeChange(t.Context(), ready.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := restarted.TrustProjectExecution(t.Context(), ready.ProjectRevision, true); err != nil {
		t.Fatal(err)
	}
	if _, err := restarted.CheckChange(t.Context(), ready.ID, ChangeCheckRequest{ChangeIdentity: identityForChange(ready), CheckOptions: CheckOptions{RunTests: true}}); err != nil {
		t.Fatal(err)
	}
	reviewed, err := restarted.RespondChangeInput(t.Context(), ready.ID, inputResponse(ready, "approve"))
	if err != nil || !reviewed.Workflow.Input.ReceivedAt.Equal(received) {
		t.Fatal("fresh checks lost the original input receipt", err)
	}
	if _, err := restarted.RespondChangeInput(t.Context(), ready.ID, inputResponse(ready, "reject")); err == nil {
		t.Fatal("conflicting approval replay was accepted")
	}
	assertGeneratedFilesAbsent(t, root)
	receipt, err := restarted.ApplyChange(t.Context(), ready.ID, request)
	if err != nil || receipt.State != "applied" || !receipt.UndoAvailable {
		t.Fatal("explicit Apply failed", err)
	}
	request.ProjectRevision = receipt.ProjectRevision
	assertIntegratedUndoGuards(t, restarted, root, ready.ID, request)
	after, err := os.ReadFile(filepath.Join(root, "main.go"))
	if err != nil || !bytes.Equal(original, after) {
		t.Fatal("workflow recovery changed unrelated source", err)
	}
	assertIntegratedInspectionIsPassive(t, root)
	if calls.Load() != modelCalls {
		t.Fatal("inspection, checkpoint replay, restart or mutation recovery called a provider")
	}
}

func assertGeneratedFilesAbsent(t *testing.T, root string) {
	t.Helper()
	for _, name := range []string{"new.go", "new_test.go"} {
		if _, err := os.Stat(filepath.Join(root, name)); !os.IsNotExist(err) {
			t.Fatal("a proposal became source without Apply", name, err)
		}
	}
}

func assertIntegratedInspectionIsPassive(t *testing.T, root string) {
	t.Helper()
	before := inspectionFiles(t, root)
	result, err := InspectAgentState(t.Context(), AgentInspectionOptions{Root: root, Limit: 500})
	if err != nil || result.AvailableRuns < 4 || result.AvailableArtifacts == 0 || result.PrivateSession != nil || result.PrivateArtifact != nil {
		t.Fatal("native history or private-data boundary lost", err)
	}
	data, err := json.Marshal(result)
	if err != nil || strings.Contains(string(data), "func Helper") || strings.Contains(string(data), "TestHelper") {
		t.Fatal("source escaped metadata inspection", err)
	}
	if !reflect.DeepEqual(before, inspectionFiles(t, root)) {
		t.Fatal("metadata inspection modified source or durable runtime data")
	}
}

func assertIntegratedUndoGuards(t *testing.T, s *Service, root, id string, request ChangeApplyRequest) {
	t.Helper()
	path := filepath.Join(root, "new.go")
	applied, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(applied), "return 2") {
		t.Fatal("Apply omitted checked implementation", err)
	}
	edited := []byte("package main\nfunc Helper() int { return 99 }\n")
	if err := os.WriteFile(path, edited, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UndoChange(t.Context(), id, request); err == nil {
		t.Fatal("Undo overwrote an edit after Apply")
	}
	retained, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(retained, edited) {
		t.Fatal("failed Undo damaged current source", err)
	}
	if _, err := os.Stat(filepath.Join(root, "new_test.go")); err != nil {
		t.Fatal("failed Undo partially removed the proposal", err)
	}
	if err := os.WriteFile(path, applied, 0644); err != nil {
		t.Fatal(err)
	}
	receipt, err := s.UndoChange(t.Context(), id, request)
	if err != nil || receipt.State != "undone" || receipt.UndoAvailable {
		t.Fatal("explicit recovery did not restore the captured source", err)
	}
	assertGeneratedFilesAbsent(t, root)
}
