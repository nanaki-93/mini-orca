package tui

import (
	"strings"
	"testing"
)

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
