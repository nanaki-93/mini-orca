package tui

import (
	"strings"
	"testing"
)

func TestWorkflowRecoveryUsesNamedStagesAndFreshConsentGuidance(t *testing.T) {
	if resumableWorkflowStage(nil) != "create" {
		t.Fatal("missing workflow skipped generation")
	}
	session := &ChangeSession{Changes: []FileChange{{Path: "main.go"}}, Workflow: &ChangeWorkflow{Status: "interrupted", Stages: []ChangeWorkflowStage{{Name: "review", Status: "pending"}, {Name: "test", Status: "pending"}, {Name: "create", Status: "completed"}}}}
	if got := resumableWorkflowStage(session); got != "test" {
		t.Fatal("named generation evidence lost", got)
	}
	session.Checks = []Check{{Name: "tests", State: "passed", Required: true}}
	session.Workflow.Stages[1].Status = "completed"
	if got := resumableWorkflowStage(session); got != "review" {
		t.Fatal("named checked evidence lost", got)
	}
	session.Workflow.Stages = append(session.Workflow.Stages, session.Workflow.Stages[1])
	if got := resumableWorkflowStage(session); got != "create" {
		t.Fatal("ambiguous stage authorized retry", got)
	}
	for _, state := range []string{"interrupted", "outdated", "stale"} {
		session.Workflow.Status = state
		if !strings.Contains(workflowTimingSummary(session), "fresh consent and checks") {
			t.Fatal("recovery hid fresh admission", state)
		}
	}
	s := NewState()
	s.Change = session
	s.execRetryCommand("unknown-native-node")
	if len(s.PendingActions) != 0 || s.Confirmation != nil || s.Error == "" {
		t.Fatal("unknown node dispatched")
	}
}

func TestRetryConsentRejectsChangedTaskAndUnconfirmedRemoteDispatch(t *testing.T) {
	s := settingsTestState()
	remote := s.Models.Scopes["code"]
	remote.RemoteProvider, remote.ProviderOrigin = true, "https://provider.example"
	s.Models.Scopes["code"] = remote
	s.Change = &ChangeSession{ID: "task", ProjectIdentity: s.Project.ProjectIdentity, Hash: "original", Targets: []ChangeTarget{{Path: "main.go"}}}
	s.execRetryCommand("review")
	if s.Confirmation == nil || len(s.PendingActions) != 0 {
		t.Fatal("retry bypassed provider confirmation")
	}
	s.Change.Hash = "changed"
	s.Confirmation.OnConfirm(s)
	if len(s.PendingActions) != 0 || !strings.Contains(s.Error, "changed") {
		t.Fatal("retry reused consent for a different proposal")
	}
	if _, ok := s.prepareChangeActionTask("unconfirmed", ActionRequest{Kind: "repair-change", Target: "task"}); ok {
		t.Fatal("direct action silently confirmed all remote profiles")
	}
	s.AgentSettings = nil
	s.Models = nil
	choice, err := s.captureWorkflowChoice(nil, s.Change)
	if err != nil || len(choice.Confirmed) != 0 {
		t.Fatal("missing model metadata supplied remote consent")
	}
}

func TestTestingStageWithoutChecksShowsBusyAndResumeGuidance(t *testing.T) {
	s := NewState()
	s.AgentStage = 2
	s.Change = &ChangeSession{
		ID: "change-timeout", Title: "Fix encryption key persistence",
		Changes: []FileChange{{Path: "vault.go", Content: "package vault\n"}},
		Workflow: &ChangeWorkflow{Status: "running", Stages: []ChangeWorkflowStage{
			{Name: "create", Status: "completed"}, {Name: "test", Status: "running", Model: &Model{Model: "test-model", Timeout: "15m0s"}},
			{Name: "review", Status: "pending"}, {Name: "human_review", Status: "blocked"},
		}},
	}
	th := themeForPalette(s.Palette)
	busy := strings.Join(s.renderAgentDebugPage(th, 140), "\n")
	if !strings.Contains(busy, "Testing is in progress") || strings.Contains(busy, "[r] Retry") || !strings.Contains(busy, "[x] Cancel") {
		t.Fatalf("busy testing guidance = %s", busy)
	}
	s.Change.Workflow.Status = "failed"
	s.Change.Workflow.Stages[1].Status = "failed"
	failed := strings.Join(s.renderAgentDebugPage(th, 140), "\n")
	if !strings.Contains(failed, "Use :retry test to reuse the saved proposal") || !strings.Contains(failed, "[r] Retry") {
		t.Fatalf("failed testing recovery = %s", failed)
	}
}
