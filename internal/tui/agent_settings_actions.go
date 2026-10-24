package tui

import (
	"context"
	"fmt"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
)

type agentSettingsView struct {
	Parent      ProjectIdentity
	Scope       client.AgentSettingsScope
	Path        string
	Data        *client.AgentSettings
	Pending     string
	Operation   string
	Status      string
	Failure     string
	Validations map[string]*client.AgentSkillValidation
}

type agentSettingsAction struct {
	ID, Operation string
	Parent        ProjectIdentity
	Scope         client.AgentSettingsScope
	Control       client.AgentSettingsControl
	Validation    client.AgentSkillValidationRequest
}

type agentSettingsResult struct {
	Action     agentSettingsAction
	Data       *client.AgentSettings
	Validation *client.AgentSkillValidation
	Err        error
}

func (s *State) dispatchSettingsAction(ctx context.Context, cl *Client, action ActionRequest) bool {
	switch action.Kind {
	case "agent-settings":
		if s.acceptsSettingsAction(action.AgentSettings) {
			s.applyAgentSettings(executeAgentSettings(ctx, cl, *action.AgentSettings))
		}
	case "cancel-agent-settings":
		if cl != nil {
			cl.Cancel(action.Target)
		}
	default:
		return false
	}
	return true
}

func (s *State) settingsScopeCurrent(view *agentSettingsView) bool {
	if view == nil || s.Project == nil || view.Parent != s.Project.ProjectIdentity {
		return false
	}
	if view.Scope.WorktreePath == "" {
		return view.Scope.ProjectIdentity == s.Project.ProjectIdentity && view.Path == s.Project.Path
	}
	return s.Change != nil && s.Change.WorktreePath == view.Scope.WorktreePath && s.Change.ProjectIdentity == view.Scope.ProjectIdentity
}

func (s *State) acceptsSettingsAction(action *agentSettingsAction) bool {
	v := s.AgentSettings
	return action != nil && s.settingsScopeCurrent(v) && v.Pending == action.ID && v.Scope == action.Scope && v.Parent == action.Parent
}

func (s *State) selectAgentSettings(scope string) {
	if s.Project == nil || s.Project.ProjectID == "" {
		s.Error = "Open a project before loading agent settings."
		return
	}
	v := &agentSettingsView{Parent: s.Project.ProjectIdentity, Scope: client.AgentSettingsScope{ProjectIdentity: s.Project.ProjectIdentity}, Path: s.Project.Path}
	switch scope {
	case "main":
	case "task":
		if s.Change == nil || s.Change.WorktreePath == "" || s.Change.ProjectID == "" || s.Change.ProjectRevision == "" {
			s.Error = "Select a task with an indexed worktree first."
			return
		}
		v.Scope = client.AgentSettingsScope{ProjectIdentity: s.Change.ProjectIdentity, WorktreePath: s.Change.WorktreePath}
		v.Path = s.Change.WorktreePath
	default:
		s.Error = "Use :agent-settings main or :agent-settings task."
		return
	}
	s.cancelAgentSettings()
	s.AgentSettings = v
	s.SelectedIdx = 0
	s.refreshAgentSettings()
}

func (s *State) refreshAgentSettings() {
	if s.AgentSettings == nil {
		s.selectAgentSettings("main")
		return
	}
	if !s.settingsScopeCurrent(s.AgentSettings) {
		s.Error = "Checkout changed. Select :agent-settings main or task again."
		return
	}
	s.queueSettingsAction(agentSettingsAction{Operation: "read"})
}

func (s *State) queueSettingsAction(action agentSettingsAction) {
	v := s.AgentSettings
	if !s.settingsScopeCurrent(v) || v.Pending != "" {
		s.Error = "Load the current checkout settings and finish or cancel its pending request first."
		return
	}
	action.ID, action.Parent, action.Scope = nextTUIReqID("settings"), v.Parent, v.Scope
	v.Pending, v.Operation, v.Status, v.Failure = action.ID, action.Operation, "loading", ""
	s.enqueueAction(ActionRequest{Kind: "agent-settings", AgentSettings: &action})
}

func (s *State) cancelAgentSettings() {
	v := s.AgentSettings
	if v == nil || v.Pending == "" {
		return
	}
	s.enqueueAction(ActionRequest{Kind: "cancel-agent-settings", Target: v.Pending})
	v.Pending, v.Status, v.Failure = "", "canceled", "Request canceled. Refresh to confirm the stored state; a save may already have completed."
	v.Data, v.Validations = nil, nil
	s.CmdOutput = v.Failure
}

func executeAgentSettings(ctx context.Context, cl *Client, action agentSettingsAction) agentSettingsResult {
	result := agentSettingsResult{Action: action}
	if cl == nil {
		result.Err = fmt.Errorf("daemon is unavailable")
		return result
	}
	switch action.Operation {
	case "read":
		result.Data, result.Err = cl.ReadAgentSettings(ctx, action.ID, action.Scope)
	case "control":
		result.Data, result.Err = cl.ControlAgentSettings(ctx, action.ID, action.Scope, action.Control)
	case "validate":
		result.Validation, result.Err = cl.ValidateAgentSkill(ctx, action.ID, action.Scope, action.Validation)
	default:
		result.Err = fmt.Errorf("unsupported agent settings action")
	}
	return result
}

func (s *State) applyAgentSettings(result agentSettingsResult) {
	if !s.acceptsSettingsAction(&result.Action) {
		return
	}
	v := s.AgentSettings
	v.Pending = ""
	if result.Err != nil {
		v.Status, v.Failure = "unavailable", result.Err.Error()
		if result.Action.Operation != "validate" {
			v.Data, v.Validations = nil, nil
		}
		if result.Action.Operation != "read" {
			s.Error = "Agent settings: " + v.Failure
		}
		return
	}
	v.Status, v.Failure = "current", ""
	if result.Data != nil {
		selected := s.selectedSettingsKey()
		v.Data, v.Validations = result.Data, nil
		s.restoreSettingsSelection(selected)
	}
	if result.Validation != nil {
		if v.Validations == nil {
			v.Validations = make(map[string]*client.AgentSkillValidation)
		}
		v.Validations[result.Validation.Directory] = result.Validation
	}
	if result.Action.Operation == "control" {
		s.CmdOutput = "Agent settings saved for future runs in " + settingsText(v.Path) + ". Active runs retain their captured settings."
	}
}

func (s *State) loadAgentSettings(ctx context.Context, cl *Client) {
	if s.Project == nil || s.Project.ProjectID == "" {
		return
	}
	v := &agentSettingsView{Parent: s.Project.ProjectIdentity, Scope: client.AgentSettingsScope{ProjectIdentity: s.Project.ProjectIdentity}, Path: s.Project.Path, Pending: nextTUIReqID("settings-sync"), Status: "loading"}
	s.AgentSettings = v
	s.applyAgentSettings(executeAgentSettings(ctx, cl, agentSettingsAction{ID: v.Pending, Operation: "read", Scope: v.Scope, Parent: v.Parent}))
}
