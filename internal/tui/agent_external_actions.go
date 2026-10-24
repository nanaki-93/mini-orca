package tui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
)

func (m *BubbleModel) prepareExternalCmd(action *agentExternalAction) tea.Cmd {
	if !m.state.prepareExternalAction(action) {
		return nil
	}
	captured := *action
	owned, cancel := context.WithCancel(m.ctx)
	m.state.AgentExternal.Cancel = cancel
	return func() tea.Msg {
		defer cancel()
		return agentExternalResultMsg{result: executeExternalAction(owned, m.client, captured)}
	}
}

const (
	externalAPI      = "api"
	externalResearch = "research"
	externalRemote   = "remote"
)

type externalKey struct{ Kind, ID string }
type externalReview struct {
	ID, Hash string
	Version  int64
}

type agentExternalView struct {
	Parent             ProjectIdentity
	Scope              client.AgentSettingsScope
	Path               string
	Actions            []client.APIAction
	Research           *client.ResearchOverview
	Delegations        []client.Delegation
	Detail             *client.Delegation
	Reviewed           externalReview
	Loaded             map[string]bool
	Failures           map[string]string
	Pending, Operation string
	Key                externalKey
	Cancel             context.CancelFunc
}

type agentExternalAction struct {
	ID, Operation string
	Parent        ProjectIdentity
	Scope         client.AgentSettingsScope
	Key           externalKey
	Expected      string
	APIControl    client.APIActionControl
	RemoteControl client.DelegationControl
	Draft         *externalDraft
}

type agentExternalResult struct {
	Action      agentExternalAction
	Actions     []client.APIAction
	Research    *client.ResearchOverview
	Delegations []client.Delegation
	Failures    map[string]error
	API         *client.APIAction
	Report      *client.ResearchReport
	Remote      *client.Delegation
	Index       *ProjectIndex
	Err         error
}

func newExternalView(parent ProjectIdentity, scope client.AgentSettingsScope, path string) *agentExternalView {
	return &agentExternalView{Parent: parent, Scope: scope, Path: path, Loaded: map[string]bool{}, Failures: map[string]string{}}
}

func (s *State) externalCurrent() bool {
	v := s.AgentExternal
	return v != nil && s.checkoutScopeCurrent(v.Parent, v.Scope, v.Path)
}

func (s *State) selectExternalScope(selection string) {
	if s.Project == nil || s.Project.ProjectID == "" {
		s.Error = "Open a project before loading external work."
		return
	}
	scope, path := client.AgentSettingsScope{ProjectIdentity: s.Project.ProjectIdentity}, s.Project.Path
	switch selection {
	case "main":
	case "task":
		if s.Change == nil || s.Change.WorktreePath == "" || s.Change.ProjectID == "" || s.Change.ProjectRevision == "" {
			s.Error = "Select an indexed task worktree first."
			return
		}
		scope, path = client.AgentSettingsScope{ProjectIdentity: s.Change.ProjectIdentity, WorktreePath: s.Change.WorktreePath}, s.Change.WorktreePath
	default:
		s.Error = "Use :external-scope main or :external-scope task."
		return
	}
	s.cancelExternalRequest()
	s.AgentExternal = newExternalView(s.Project.ProjectIdentity, scope, path)
	s.SelectedIdx = 0
	s.refreshExternalWork()
}

func (s *State) refreshExternalWork() {
	if s.AgentExternal == nil {
		s.selectExternalScope("main")
		return
	}
	s.queueExternalAction(agentExternalAction{Operation: "read"})
}

func (s *State) queueExternalAction(action agentExternalAction) {
	if !s.externalCurrent() || s.AgentExternal.Pending != "" {
		s.Error = "Load the current external-work checkout and finish or cancel its pending request first."
		return
	}
	v := s.AgentExternal
	action.ID, action.Parent, action.Scope = nextTUIReqID("external"), v.Parent, v.Scope
	v.Pending, v.Operation, v.Key = action.ID, action.Operation, action.Key
	s.enqueueAction(ActionRequest{Kind: "agent-external", AgentExternal: &action})
}

func (s *State) acceptsExternalAction(action *agentExternalAction) bool {
	return action != nil && s.externalCurrent() && s.AgentExternal.Pending == action.ID && s.AgentExternal.Parent == action.Parent && s.AgentExternal.Scope == action.Scope
}

func (s *State) prepareExternalAction(action *agentExternalAction) bool {
	if !s.acceptsExternalAction(action) {
		return false
	}
	if err := s.externalAdmission(*action); err != nil {
		s.AgentExternal.Pending, s.Error = "", err.Error()
		return false
	}
	return true
}

func (s *State) cancelExternalRequest() {
	v := s.AgentExternal
	if v == nil || v.Pending == "" {
		return
	}
	if v.Cancel != nil {
		v.Cancel()
		v.Cancel = nil
	}
	s.enqueueAction(ActionRequest{Kind: "cancel-agent-external", Target: v.Pending})
	s.markExternalUnavailable(v.Operation, v.Key, "Request canceled. Refresh local history to establish the outcome before another action.")
	v.Pending = ""
	s.CmdOutput = "Local request canceled. A remote task or external effect may still exist; refresh local history for recovery."
}

func executeExternalAction(ctx context.Context, cl *Client, action agentExternalAction) agentExternalResult {
	r := agentExternalResult{Action: action}
	if cl == nil {
		r.Err = fmt.Errorf("daemon is unavailable")
		return r
	}
	switch action.Operation {
	case "read":
		r.Failures = map[string]error{}
		r.Actions, r.Failures[externalAPI] = cl.ReadAPIActions(ctx, action.ID+"-api", action.Scope)
		r.Research, r.Failures[externalResearch] = cl.ReadResearch(ctx, action.ID+"-research", action.Scope)
		r.Delegations, r.Failures[externalRemote] = cl.ReadDelegations(ctx, action.ID+"-remote", action.Scope)
	case "capture":
		r.Index, r.Err = cl.ExternalIndex(ctx, action.ID, action.Scope)
	case "api-control":
		r.API, r.Err = cl.ControlAPIAction(ctx, action.ID, action.Scope, action.Key.ID, action.APIControl)
	case "remote-detail":
		r.Remote, r.Err = cl.Delegation(ctx, action.ID, action.Scope, action.Key.ID)
	case "remote-control":
		r.Remote, r.Err = cl.ControlDelegation(ctx, action.ID, action.Scope, action.Key.ID, action.RemoteControl)
	case "research", "delegate":
		executeExternalDraft(ctx, cl, &r)
	case "report":
		r.Err = openResearchReport(ctx, cl, action.Scope, action.Key.ID)
	default:
		r.Err = fmt.Errorf("unsupported external work action")
	}
	return r
}

func executeExternalDraft(ctx context.Context, cl *Client, r *agentExternalResult) {
	a, d := r.Action, r.Action.Draft
	if d == nil {
		r.Err = fmt.Errorf("external work has no captured input")
		return
	}
	if a.Operation == "research" {
		r.Report, r.Err = cl.RunResearch(ctx, a.ID, a.Scope, client.ResearchRequest{ProjectIdentity: a.Scope.ProjectIdentity, Question: d.Question, Sources: d.Sources, ConfirmRemoteProvider: d.Model.RemoteProvider, ConfirmSearch: true, ConfirmSourceSharing: true})
	} else {
		r.Remote, r.Err = cl.StartDelegation(ctx, a.ID, a.Scope, client.DelegationRequest{ProjectIdentity: a.Scope.ProjectIdentity, Question: d.Question, Sources: d.Sources, Kind: d.Kind, Skill: d.Skill, Remote: externalRemoteUse(d.Remote, true), Stream: d.Remote.Inventory.Streaming, ConfirmDelegate: true})
	}
}

func (s *State) applyExternalResult(r agentExternalResult) {
	if !s.acceptsExternalAction(&r.Action) {
		return
	}
	v, selected := s.AgentExternal, s.selectedExternalKey()
	v.Pending, v.Cancel = "", nil
	if r.Action.Operation == "read" {
		s.applyExternalHistory(r)
	} else if r.Err != nil {
		s.markExternalUnavailable(r.Action.Operation, r.Action.Key, "Outcome unavailable; refresh local history. "+r.Err.Error())
		s.Error = settingsText(r.Err.Error())
	} else {
		s.applyExternalReceipt(r)
	}
	s.restoreExternalSelection(selected)
}

func (s *State) applyExternalHistory(r agentExternalResult) {
	v := s.AgentExternal
	for _, kind := range []string{externalAPI, externalResearch, externalRemote} {
		err := r.Failures[kind]
		if r.Err != nil {
			err = r.Err
		}
		if err != nil {
			v.Failures[kind] = err.Error()
			continue
		}
		delete(v.Failures, kind)
		v.Loaded[kind] = true
		switch kind {
		case externalAPI:
			v.Actions, v.Reviewed = r.Actions, externalReview{}
		case externalResearch:
			v.Research = r.Research
		case externalRemote:
			v.Delegations, v.Detail = r.Delegations, nil
		}
	}
}

func (s *State) markExternalUnavailable(operation string, key externalKey, message string) {
	if operation == "capture" || operation == "report" {
		return
	}
	if operation == "read" {
		for _, kind := range []string{externalAPI, externalResearch, externalRemote} {
			s.AgentExternal.Failures[kind] = message
		}
	} else {
		s.AgentExternal.Failures[key.Kind] = message
	}
}

func (s *State) applyExternalReceipt(r agentExternalResult) {
	v := s.AgentExternal
	switch r.Action.Operation {
	case "capture":
		s.confirmCapturedExternalWork(r)
	case "api-control":
		if r.API == nil {
			s.Error = "External action returned no receipt. Refresh local history."
			return
		}
		for i := range v.Actions {
			if v.Actions[i].ID == r.API.ID {
				v.Actions[i] = *r.API
			}
		}
		v.Reviewed = externalReview{}
		s.CmdOutput = "API action outcome: " + r.API.State + "."
	case "remote-detail", "remote-control", "delegate":
		if r.Remote == nil {
			s.Error = "Remote work returned no receipt. Refresh local history."
			return
		}
		v.Detail = r.Remote
		v.Delegations = replaceDelegation(v.Delegations, *r.Remote)
		s.CmdOutput = "Remote advisory receipt: " + r.Remote.State + "."
	case "research":
		if r.Report == nil || v.Research == nil {
			s.Error = "Research receipt unavailable. Refresh local history."
			return
		}
		v.Research.Reports = append([]client.ResearchReport{*r.Report}, v.Research.Reports...)
		s.CmdOutput = "Research retained: " + r.Report.State + ". Select the report and press Enter to open its citations in the browser."
	case "report":
		s.CmdOutput = "Sent the retained local research report to the system browser."
	}
}

func replaceDelegation(items []client.Delegation, receipt client.Delegation) []client.Delegation {
	for i := range items {
		if items[i].ID == receipt.ID {
			items[i] = receipt
			return items
		}
	}
	return append([]client.Delegation{receipt}, items...)
}

func (s *State) dispatchExternalAction(ctx context.Context, cl *Client, action ActionRequest) bool {
	switch action.Kind {
	case "agent-external":
		if s.prepareExternalAction(action.AgentExternal) {
			s.applyExternalResult(executeExternalAction(ctx, cl, *action.AgentExternal))
		}
	case "cancel-agent-external":
		if cl != nil {
			for _, suffix := range []string{"", "-api", "-research", "-remote"} {
				cl.Cancel(action.Target + suffix)
			}
		}
	default:
		return false
	}
	return true
}

func (s *State) dispatchGovernanceAction(ctx context.Context, cl *Client, action ActionRequest) bool {
	return s.dispatchSettingsAction(ctx, cl, action) || s.dispatchConnectionAction(ctx, cl, action) || s.dispatchExternalAction(ctx, cl, action)
}

func (s *State) loadExternalWork(ctx context.Context, cl *Client) {
	if s.Project == nil || s.Project.ProjectID == "" {
		return
	}
	s.cancelExternalRequest()
	s.AgentExternal = newExternalView(s.Project.ProjectIdentity, client.AgentSettingsScope{ProjectIdentity: s.Project.ProjectIdentity}, s.Project.Path)
	v := s.AgentExternal
	v.Pending = nextTUIReqID("external-sync")
	s.applyExternalResult(executeExternalAction(ctx, cl, agentExternalAction{ID: v.Pending, Parent: v.Parent, Scope: v.Scope, Operation: "read"}))
}
