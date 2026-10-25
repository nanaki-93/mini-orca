package tui

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
)

func failedWorktreeRetryState() *State {
	s := settingsTestState()
	s.Change = &ChangeSession{
		ID: "failed-task", ProjectIdentity: ProjectIdentity{ProjectID: "child", ProjectRevision: "child-revision"},
		WorktreePath: "/repo/task", Title: "Fix result", Kind: "fix", State: "failed", Freshness: "current",
		Revision: 2, Hash: "failed-proposal", Targets: []ChangeTarget{{Path: "main.go"}, {Path: "main_test.go"}},
		AcceptanceCriteria: []string{"Returns the correct result"},
		Plan:               &ChangePlan{Hash: "approved-plan", ChangePlanInput: ChangePlanInput{Problem: "Wrong result", Approach: "Correct the return value"}},
		Workflow:           &ChangeWorkflow{ID: "failed-workflow", Status: "failed", Stages: []ChangeWorkflowStage{{Name: "create", Status: "failed"}}},
	}
	s.ActiveChanges = []*ChangeSession{s.Change}
	s.Navigate(PageAgents)
	return s
}

func TestRepairFailedCreationKeepsTaskAndLoadsCheckoutSettings(t *testing.T) {
	s := failedWorktreeRetryState()
	s.HandleKey(KeyEvent{Key: "r"})
	if s.Change == nil || s.Change.ID != "failed-task" || s.Change.Hash != "failed-proposal" || s.PlanDraft != nil || s.Error != "" {
		t.Fatalf("repair discarded the failed task or reopened generation with the wrong settings: change=%+v error=%q", s.Change, s.Error)
	}
	if len(s.PendingActions) != 1 || s.PendingActions[0].Kind != "agent-settings" || s.PendingActions[0].AgentSettings.Scope.WorktreePath != "/repo/task" {
		t.Fatalf("repair did not first request the task's local settings: %+v", s.PendingActions)
	}
	if s.Change.Workflow.Status != "failed" || s.Confirmation != nil {
		t.Fatal("settings preparation started work or requested consent before reading task settings")
	}
}

func TestRetryLoadsTaskRolesBeforeProviderConsent(t *testing.T) {
	s := failedWorktreeRetryState()
	remote := s.Models.Scopes["bug"]
	remote.RemoteProvider, remote.ProviderOrigin = true, "https://task-provider.example"
	s.Models.Scopes["bug"] = remote
	settings := *s.AgentSettings.Data
	settings.ProjectIdentity = s.Change.ProjectIdentity
	settings.Roles = client.AgentRoles{Creation: "bug", Testing: "function", Review: "analyze"}
	updated := cloneChangeSessionSnapshot(s.Change)
	updated.Workflow.Status = "running"
	var calls atomic.Int32
	cl := investigationClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Query().Get("worktree_path") != "/repo/task" {
			t.Errorf("retry lost its task checkout: %s", r.URL)
		}
		switch r.URL.Path {
		case "/api/projects/current/agent-settings":
			if r.Method != http.MethodGet || r.URL.Query().Get("project_id") != "child" || r.URL.Query().Get("project_revision") != "child-revision" {
				t.Errorf("settings read used another checkout: %s %s", r.Method, r.URL)
			}
			_ = json.NewEncoder(w).Encode(settings)
		case "/api/projects/current":
			_ = json.NewEncoder(w).Encode(Project{ProjectIdentity: settings.ProjectIdentity, Path: "/repo/task"})
		case "/api/projects/current/changes/failed-task/workflow":
			var body struct {
				ProjectIdentity
				Revision   int               `json:"revision"`
				Hash       string            `json:"hash"`
				PlanHash   string            `json:"plan_hash"`
				ResumeFrom string            `json:"resume_from"`
				Models     map[string]string `json:"models"`
				Confirmed  []string          `json:"confirmed_profiles"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			if r.Method != http.MethodPost || body.ProjectIdentity != settings.ProjectIdentity || body.Revision != 2 || body.Hash != "failed-proposal" || body.PlanHash != "approved-plan" || body.ResumeFrom != "create" || body.Models["create"] != "bug" || len(body.Confirmed) != 1 || body.Confirmed[0] != "bug" {
				t.Errorf("retry lost the saved task, scoped roles or fresh consent: %+v", body)
			}
			_ = json.NewEncoder(w).Encode(updated)
		default:
			t.Errorf("retry made an unexpected request: %s %s", r.Method, r.URL)
			http.Error(w, "unexpected request", http.StatusBadRequest)
		}
	})
	s.ExecuteCommand(":retry create")
	if s.Error != "" || len(s.PendingActions) != 1 || s.PendingActions[0].Kind != "agent-settings" {
		t.Fatalf("retry did not prepare scoped settings: %s %+v", s.Error, s.PendingActions)
	}
	s.FlushActions(t.Context(), cl)
	if s.Error != "" || calls.Load() != 1 || s.Confirmation == nil || len(s.PendingActions) != 0 || !strings.Contains(s.Confirmation.Message, "create: bug") || !strings.Contains(s.Confirmation.Message, "https://task-provider.example") {
		t.Fatalf("settings load failed or bypassed fresh provider consent: calls=%d error=%q confirmation=%+v", calls.Load(), s.Error, s.Confirmation)
	}
	s.HandleKey(KeyEvent{Key: "Enter"})
	s.FlushActions(t.Context(), cl)
	if s.Error != "" || calls.Load() != 3 || s.Change.ID != "failed-task" || s.Change.Workflow.Status != "running" || len(s.ActiveChanges) != 1 {
		t.Fatalf("repair did not resume the original workflow: calls=%d change=%+v error=%q", calls.Load(), s.Change, s.Error)
	}
}

func TestRetrySettingsReadContinuesThroughAsyncResult(t *testing.T) {
	s := failedWorktreeRetryState()
	settings := *s.AgentSettings.Data
	settings.ProjectIdentity = s.Change.ProjectIdentity
	var calls atomic.Int32
	cl := investigationClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != http.MethodGet || r.URL.Path != "/api/projects/current/agent-settings" {
			t.Errorf("settings preparation dispatched workflow I/O: %s %s", r.Method, r.URL)
		}
		_ = json.NewEncoder(w).Encode(settings)
	})
	m := NewBubbleModel(t.Context(), s, cl, 80, 24)
	m.nativeStreams = false
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if cmd == nil || calls.Load() != 0 || s.Change.Workflow.Status != "failed" {
		t.Fatal("repair blocked the UI or ran before its settings request")
	}
	message := cmd()
	next := m.applyAgentRunMessage(message)
	if next == nil || calls.Load() != 1 || s.Error != "" || s.Change.ID != "failed-task" || s.Change.Workflow.Status != "running" || len(s.PendingActions) != 0 {
		t.Fatalf("settings completion did not schedule the retry asynchronously: calls=%d error=%q change=%+v", calls.Load(), s.Error, s.Change)
	}
}

func TestRetrySettingsCompletionRejectsChangedRequest(t *testing.T) {
	for _, scenario := range []struct {
		name   string
		change func(*State)
	}{
		{"task", func(s *State) { s.Change.ID = "another-task" }},
		{"proposal", func(s *State) { s.Change.Hash = "new-proposal" }},
		{"revision", func(s *State) { s.Change.Revision++ }},
		{"plan", func(s *State) { s.Change.Plan.Hash = "new-plan" }},
		{"checkout", func(s *State) { s.Change.WorktreePath = "/repo/another-task" }},
		{"project", func(s *State) { s.Project.ProjectRevision = "new-parent" }},
		{"routing", func(s *State) { s.ScopeModels["bug"] = "function" }},
		{"navigation", func(s *State) { s.Navigate(PageSummary) }},
		{"command editor", func(s *State) { s.openCommandInput("agent-settings ") }},
		{"confirmation", func(s *State) { s.Confirmation = &Confirmation{Title: "Another decision"} }},
		{"quit", func(s *State) { s.Quitting = true }},
		{"cancel", func(s *State) { s.cancelAgentSettings() }},
		{"replacement", func(s *State) { s.selectAgentSettings("main") }},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			s := failedWorktreeRetryState()
			settings := *s.AgentSettings.Data
			settings.ProjectIdentity = s.Change.ProjectIdentity
			s.execRetryCommand("create")
			if len(s.PendingActions) != 1 || s.PendingActions[0].AgentSettings == nil {
				t.Fatalf("missing settings request: %s %+v", s.Error, s.PendingActions)
			}
			action := *s.PendingActions[0].AgentSettings
			s.PendingActions = nil
			scenario.change(s)
			s.PendingActions = nil
			confirmation, view, pending := s.Confirmation, s.AgentSettings, s.AgentSettings.Pending
			s.applyAgentSettings(agentSettingsResult{Action: action, Data: &settings})
			if len(s.PendingActions) != 0 || s.Confirmation != confirmation || s.Change.Workflow.Status != "failed" || s.AgentSettings != view {
				t.Fatal("late settings completion resumed a changed or canceled retry")
			}
			if scenario.name == "replacement" && s.AgentSettings.Pending != pending {
				t.Fatal("late settings completion replaced another request")
			}
		})
	}
}

func TestRetrySettingsFailuresNeverGenerateWithMainDefaults(t *testing.T) {
	for _, outcome := range []string{"unavailable", "wrong checkout", "stale"} {
		t.Run(outcome, func(t *testing.T) {
			s := failedWorktreeRetryState()
			settings := *s.AgentSettings.Data
			settings.ProjectIdentity = s.Change.ProjectIdentity
			if outcome == "wrong checkout" {
				settings.ProjectIdentity = s.Project.ProjectIdentity
			}
			if outcome == "stale" {
				settings.State = "stale"
			}
			var calls atomic.Int32
			cl := investigationClient(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.Method != http.MethodGet || r.URL.Path != "/api/projects/current/agent-settings" {
					t.Errorf("failed settings load dispatched work: %s %s", r.Method, r.URL)
				}
				if outcome == "unavailable" {
					http.Error(w, "settings unavailable", http.StatusServiceUnavailable)
					return
				}
				_ = json.NewEncoder(w).Encode(settings)
			})
			s.execRetryCommand("create")
			s.FlushActions(t.Context(), cl)
			if s.Error == "" || calls.Load() != 1 || len(s.PendingActions) != 0 || s.Confirmation != nil || s.Change.Workflow.Status != "failed" {
				t.Fatalf("settings failure supplied defaults or hid its error: calls=%d error=%q change=%+v", calls.Load(), s.Error, s.Change)
			}
			if outcome == "unavailable" {
				s.execRetryCommand("create")
				s.FlushActions(t.Context(), cl)
				if calls.Load() != 2 || s.Error == "" || s.Change.Workflow.Status != "failed" {
					t.Fatal("an explicit retry could not refresh unavailable checkout settings")
				}
			}
		})
	}
}

func TestRetrySettingsDoesNotCancelPendingSave(t *testing.T) {
	s := failedWorktreeRetryState()
	v := s.AgentSettings
	v.Pending, v.Operation = "pending-save", "control"
	s.execRetryCommand("create")
	if s.Error == "" || s.AgentSettings != v || v.Pending != "pending-save" || len(s.PendingActions) != 0 || s.Confirmation != nil {
		t.Fatal("retry canceled an unresolved settings save or dispatched work")
	}
}

func TestRetrySettingsRejectsMissingCheckoutIdentity(t *testing.T) {
	s := failedWorktreeRetryState()
	s.Change.ProjectRevision = ""
	s.execRetryCommand("create")
	if !strings.Contains(s.Error, "checkout identity") || len(s.PendingActions) != 0 || s.Confirmation != nil || s.Change.Workflow.Status != "failed" {
		t.Fatal("retry guessed a task checkout identity or dispatched work")
	}
}

func TestRetrySettingsSelectsMainForMainCheckout(t *testing.T) {
	s := failedWorktreeRetryState()
	view, err := s.workflowRetrySettingsView()
	if err != nil {
		t.Fatal(err)
	}
	settings := *s.AgentSettings.Data
	settings.ProjectIdentity = s.Change.ProjectIdentity
	view.Data, view.Status = &settings, "current"
	s.AgentSettings = view
	s.Change.ProjectIdentity, s.Change.WorktreePath = s.Project.ProjectIdentity, ""
	s.execRetryCommand("create")
	if s.Error != "" || len(s.PendingActions) != 1 || s.PendingActions[0].AgentSettings == nil {
		t.Fatalf("main checkout retry did not prepare settings: %s %+v", s.Error, s.PendingActions)
	}
	scope := s.PendingActions[0].AgentSettings.Scope
	if scope.WorktreePath != "" || scope.ProjectIdentity != s.Project.ProjectIdentity || s.AgentSettings.Path != s.Project.Path {
		t.Fatalf("main checkout retry inherited task settings: %+v", scope)
	}
}

func TestRetrySettingsReplacesReadForAnotherCheckout(t *testing.T) {
	s := failedWorktreeRetryState()
	settings := *s.AgentSettings.Data
	settings.ProjectIdentity = s.Change.ProjectIdentity
	s.execRetryCommand("create")
	old := *s.PendingActions[0].AgentSettings
	s.PendingActions = nil
	s.Change.ID, s.Change.WorktreePath = "another-task", "/repo/another-task"
	s.Change.ProjectIdentity = ProjectIdentity{ProjectID: "another-child", ProjectRevision: "another-revision"}
	s.execRetryCommand("create")
	if s.Error != "" || len(s.PendingActions) != 2 || s.PendingActions[0].Kind != "cancel-agent-settings" || s.PendingActions[0].Target != old.ID || s.PendingActions[1].AgentSettings == nil {
		t.Fatalf("new task retry did not replace the old metadata read: error=%q actions=%+v", s.Error, s.PendingActions)
	}
	current := s.AgentSettings
	if current.Scope.WorktreePath != "/repo/another-task" || current.Scope.ProjectIdentity != s.Change.ProjectIdentity {
		t.Fatal("replacement settings read inherited the old task scope")
	}
	s.PendingActions = nil
	s.applyAgentSettings(agentSettingsResult{Action: old, Data: &settings})
	if s.AgentSettings != current || current.Pending == "" || current.Data != nil || s.Confirmation != nil || len(s.PendingActions) != 0 {
		t.Fatal("old settings completion replaced the new retry or started work")
	}
}
