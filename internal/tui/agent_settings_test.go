package tui

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/theme"
)

func TestCanceledSettingsSaveRefreshesActualPersistedOutcome(t *testing.T) {
	s := settingsTestState()
	stored := *s.AgentSettings.Data
	var version atomic.Int64
	version.Store(stored.Version)
	admitted, canceled := make(chan struct{}), make(chan struct{})
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			// Reading the request body starts net/http's disconnect detection.
			if _, err := io.Copy(io.Discard, r.Body); err != nil {
				t.Error(err)
				return
			}
			version.Add(1)
			close(admitted)
			select {
			case <-r.Context().Done():
			case <-ctx.Done():
			}
			close(canceled)
			return
		}
		value := stored
		value.Version = version.Load()
		_ = json.NewEncoder(w).Encode(value)
	}))
	defer server.Close()
	defer cancel()
	cl, _ := NewClient(server.URL)
	defer cl.Close()
	s.ExecuteCommand(":agent-budget model_calls 2")
	s.Confirmation.OnConfirm(s)
	s.Confirmation = nil
	model := NewBubbleModel(ctx, s, cl, 80, 24)
	cmd, ok := model.prepareAsyncActionCmd("ignored", s.PendingActions[0])
	if !ok || cmd == nil {
		t.Fatal("missing asynchronous settings command")
	}
	s.PendingActions = nil
	result := make(chan agentSettingsResultMsg, 1)
	go func() { result <- cmd().(agentSettingsResultMsg) }()
	select {
	case <-admitted:
	case <-ctx.Done():
		t.Fatal("save was not admitted")
	}
	s.cancelAgentSettings()
	s.FlushActions(ctx, cl)
	select {
	case <-canceled:
	case <-ctx.Done():
		t.Fatal("cancel did not close the HTTP request")
	}
	model.Update(<-result)
	if ctx.Err() != nil {
		t.Fatal("request cancellation required the test deadline")
	}
	if s.AgentSettings.Data != nil || s.AgentSettings.Status != "canceled" {
		t.Fatal("late mutation response replaced uncertain outcome")
	}
	s.refreshAgentSettings()
	s.FlushActions(ctx, cl)
	if s.AgentSettings.Data == nil || s.AgentSettings.Data.Version != stored.Version+1 {
		t.Fatal("refresh did not discover the save that completed before cancellation")
	}
}

func settingsTestState() *State {
	s := NewState()
	s.Project = &Project{ProjectIdentity: ProjectIdentity{ProjectID: "main", ProjectRevision: "r"}, Path: "/repo"}
	limits := client.AgentLimits{ModelCalls: 12, ToolCalls: 24, InputTokens: 60000, OutputTokens: 8000, ElapsedSeconds: 600, ConcurrentCalls: 3, Repairs: 2}
	s.AgentSettings = &agentSettingsView{Parent: s.Project.ProjectIdentity, Scope: client.AgentSettingsScope{ProjectIdentity: s.Project.ProjectIdentity}, Path: "/repo", Status: "current", Data: &client.AgentSettings{ProjectIdentity: s.Project.ProjectIdentity, Version: 4, Configuration: "configuration", State: "overridden", Limits: limits, Ceilings: limits, Roles: client.AgentRoles{Creation: "code", Testing: "analyze", Review: "bug"}, Skills: []client.AgentSkillSetting{{Directory: "skills/review", Enabled: true}, {Directory: "skills/test", Enabled: false}}}}
	s.Models = &ModelCatalog{Scopes: map[string]Model{
		"analyze": {Scope: "analyze", Model: "review-local", ProviderOrigin: "http://127.0.0.1:11434"},
		"bug":     {Scope: "bug", Model: "bug-local", ProviderOrigin: "http://127.0.0.1:11434"},
		"code":    {Scope: "code", Model: "code-local", ProviderOrigin: "http://127.0.0.1:11434"},
	}}
	return s
}

func TestAgentSettingsControlsRequireCurrentConfirmedSnapshot(t *testing.T) {
	s := settingsTestState()
	s.Navigate(PageRoles)
	s.ExecuteCommand(":agent-role creation bug")
	if s.Confirmation == nil || len(s.PendingActions) != 0 || !strings.Contains(s.Confirmation.Message, "/repo") {
		t.Fatal("role assignment bypassed scoped confirmation")
	}
	s.AgentSettings.Data.Version++
	s.Confirmation.OnConfirm(s)
	if len(s.PendingActions) != 0 || !strings.Contains(s.Error, "changed") {
		t.Fatal("stale settings confirmation dispatched")
	}
	s.ExecuteCommand(":agent-budget model_calls 13")
	if len(s.PendingActions) != 0 || s.Error == "" {
		t.Fatal("budget ceiling ignored")
	}
	s.ExecuteCommand(":agent-budget repairs 0")
	s.Confirmation.OnConfirm(s)
	if len(s.PendingActions) != 1 || s.PendingActions[0].AgentSettings.Control.Values.Limits.Repairs != 0 || !slices.Equal(s.PendingActions[0].AgentSettings.Control.Values.DisabledSkills, []string{"skills/test"}) {
		t.Fatal("budget edit lost complete settings")
	}
	action := *s.PendingActions[0].AgentSettings
	s.Project.ProjectRevision = "new"
	copy := *s.AgentSettings.Data
	copy.Version++
	s.applyAgentSettings(agentSettingsResult{Action: action, Data: &copy})
	if s.AgentSettings.Data.Version != 5 {
		t.Fatal("late save replaced another revision")
	}
}

func TestAgentSettingsAsyncCancellationAndCheckoutIsolation(t *testing.T) {
	s := settingsTestState()
	s.ExecuteCommand(":agent-budget model_calls 2")
	s.Confirmation.OnConfirm(s)
	action := s.PendingActions[0]
	model := NewBubbleModel(t.Context(), s, nil, 80, 24)
	cmd, ok := model.prepareAsyncActionCmd("unused", action)
	if !ok || cmd == nil {
		t.Fatal("settings mutation must run asynchronously")
	}
	s.cancelAgentSettings()
	if s.AgentSettings.Data != nil || !strings.Contains(s.CmdOutput, "may already have completed") {
		t.Fatal("cancel claimed a known save outcome")
	}
	model.Update(cmd())
	if s.AgentSettings.Status != "canceled" || s.AgentSettings.Pending != "" {
		t.Fatal("late cancellation result resurrected request")
	}
	if _, err := s.captureWorkflowChoice(&ChangeSeed{}, nil); err == nil {
		t.Fatal("uncertain saved settings admitted a workflow with guessed defaults")
	}
	s.Change = &ChangeSession{ID: "task", ProjectIdentity: ProjectIdentity{ProjectID: "child", ProjectRevision: "c"}, WorktreePath: "/repo/task"}
	s.PendingActions = nil
	s.selectAgentSettings("task")
	request := s.PendingActions[0].AgentSettings
	if request.Scope.WorktreePath != "/repo/task" || request.Scope.ProjectID != "child" || s.AgentSettings.Path != "/repo/task" {
		t.Fatal("task settings inherited main checkout identity")
	}
	s.Change.WorktreePath = "/repo/other-task"
	if s.acceptsSettingsAction(request) {
		t.Fatal("changed task accepted previous response")
	}
}

func TestAgentSettingsRefreshAndValidationUseOnlyLocalRoutes(t *testing.T) {
	s := settingsTestState()
	s.Navigate(PageSkills)
	s.SelectedIdx = 1
	stored := *s.AgentSettings.Data
	stored.Skills = []client.AgentSkillSetting{stored.Skills[1], stored.Skills[0]}
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.URL.Path)
		switch r.URL.Path {
		case "/api/projects/current/agent-settings":
			_ = json.NewEncoder(w).Encode(stored)
		case "/api/projects/current/agent-settings/skills/validate":
			var request client.AgentSkillValidationRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Error(err)
			}
			_ = json.NewEncoder(w).Encode(client.AgentSkillValidation{AgentSkillValidationRequest: request, State: "valid", Inspection: &client.AgentSkillInspection{Name: "test", Description: "Reviewed metadata", RequiredTools: []string{"unavailable-tool"}}})
		default:
			t.Errorf("settings requested non-local operation: %s", r.URL)
			w.WriteHeader(500)
		}
	}))
	defer server.Close()
	cl, _ := NewClient(server.URL)
	defer cl.Close()
	s.refreshAgentSettings()
	s.FlushActions(t.Context(), cl)
	if s.SelectedIdx != 0 || s.selectedSettingsKey() != "skills/test" {
		t.Fatal("refresh lost selected skill identity")
	}
	s.validateSelectedSkill()
	s.FlushActions(t.Context(), cl)
	if len(calls) != 2 || s.AgentSettings.Validations["skills/test"] == nil || s.AgentSettings.Data.Skills[0].Enabled {
		t.Fatalf("validation enabled a skill or failed: %+v %s", calls, s.Error)
	}
	s.toggleSelectedSkill()
	if s.Confirmation == nil || len(s.PendingActions) != 0 {
		t.Fatal("skill enablement bypassed confirmation")
	}
	s.Confirmation.OnConfirm(s)
	values := s.PendingActions[0].AgentSettings.Control.Values
	if values.DisabledSkills == nil || len(values.DisabledSkills) != 0 {
		t.Fatal("enablement must retain an explicit empty disabled list")
	}
}

func TestAgentSettingsViewsArePassiveBoundedAndReachable(t *testing.T) {
	for _, palette := range []Palette{PaletteTokyo, PaletteCRT, PaletteAmber, PalettePaper} {
		for _, size := range [][2]int{{52, 16}, {80, 24}, {120, 36}} {
			for _, page := range []Page{PageRoles, PageSkills} {
				s := settingsTestState()
				s.Palette = palette
				s.Navigate(page)
				s.AgentSettings.Validations = map[string]*client.AgentSkillValidation{"skills/review": {State: "valid", Inspection: &client.AgentSkillInspection{Name: "safe-name", Description: "metadata\x1b[2J\nsecond line", Manifest: client.AgentSkillFile{Path: "skills/review/SKILL.md", Hash: "captured-hash"}}}}
				var rendered strings.Builder
				for range 30 {
					frame := s.Render(size[0], size[1])
					rendered.WriteString(frame)
					for _, row := range strings.Split(frame, "\n") {
						if theme.VisibleRuneCount(row) != size[0] {
							t.Fatalf("settings overflow at %v: %q", size, row)
						}
					}
					s.HandleKey(KeyEvent{Key: "PageDown"})
				}
				for _, token := range []string{"CHECKOUT", "/repo", "SELECTED SETTING", "Future runs"} {
					if !strings.Contains(rendered.String(), token) {
						t.Fatalf("%s inaccessible in %s at %v", token, page, size)
					}
				}
				if page == PageSkills && (!strings.Contains(rendered.String(), "captured-hash") || strings.Contains(rendered.String(), "\x1b[2J")) {
					t.Fatal("skill metadata hidden or unsanitized")
				}
				if len(s.PendingActions) != 0 || s.Confirmation != nil || s.SelectedIdx != 0 {
					t.Fatal("settings browsing dispatched or changed selection")
				}
				for _, key := range []string{"a", "c", "t", "b", "u"} {
					s.HandleKey(KeyEvent{Key: key})
				}
				if len(s.PendingActions) != 0 || s.Confirmation != nil {
					t.Fatal("settings inherited unrelated mutation shortcuts")
				}
			}
		}
	}
}

func TestAgentSettingsStaleResetAndUnavailableRemainDistinct(t *testing.T) {
	s := settingsTestState()
	s.AgentSettings.Data.State = "stale"
	s.ExecuteCommand(":agent-role creation bug")
	if s.Confirmation != nil || s.Error == "" {
		t.Fatal("stale overrides were editable")
	}
	s.ExecuteCommand(":settings-reset")
	s.Confirmation.OnConfirm(s)
	request := s.PendingActions[0].AgentSettings.Control
	if request.Action != "reset" || request.Values != nil || request.Version != 4 {
		t.Fatal("reset did not preserve guarded settings identity")
	}
	s.Navigate(PageRoles)
	s.Confirmation, s.Error = nil, ""
	s.Project.ProjectRevision = "different"
	if frame := s.Render(80, 24); !strings.Contains(frame, "unavailable for this checkout") {
		t.Fatal("old settings presented as current")
	}
}
