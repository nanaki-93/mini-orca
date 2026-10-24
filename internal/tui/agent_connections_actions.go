package tui

import (
	"context"
	"fmt"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
)

type connectionKey struct {
	Kind client.ConnectionKind
	ID   string
}

type connectionFailure struct{ State, Message string }
type connectionReceipt struct{ Configuration, Action string }

type agentConnectionsView struct {
	Parent    ProjectIdentity
	Scope     client.AgentSettingsScope
	Path      string
	Records   map[client.ConnectionKind][]client.AgentConnection
	Failures  map[client.ConnectionKind]connectionFailure
	Receipts  map[connectionKey]connectionReceipt
	Pending   string
	Operation string
	Key       connectionKey
}

type agentConnectionAction struct {
	ID, Operation string
	Parent        ProjectIdentity
	Scope         client.AgentSettingsScope
	Key           connectionKey
	Control       client.ConnectionControl
}

type connectionListResult struct {
	Data []client.AgentConnection
	Err  error
}

type agentConnectionResult struct {
	Action agentConnectionAction
	Lists  map[client.ConnectionKind]connectionListResult
	Data   *client.AgentConnection
	Err    error
}

func (s *State) connectionsCurrent() bool {
	v := s.AgentConnections
	return v != nil && s.checkoutScopeCurrent(v.Parent, v.Scope, v.Path)
}

func (s *State) selectConnectionScope(scope string) {
	if s.Project == nil || s.Project.ProjectID == "" {
		s.Error = "Open a project before loading connections."
		return
	}
	selection := client.AgentSettingsScope{ProjectIdentity: s.Project.ProjectIdentity}
	path := s.Project.Path
	switch scope {
	case "main":
	case "task":
		if s.Change == nil || s.Change.WorktreePath == "" || s.Change.ProjectID == "" || s.Change.ProjectRevision == "" {
			s.Error = "Select a task with an indexed worktree first."
			return
		}
		selection = client.AgentSettingsScope{ProjectIdentity: s.Change.ProjectIdentity, WorktreePath: s.Change.WorktreePath}
		path = s.Change.WorktreePath
	default:
		s.Error = "Use :connection-scope main or :connection-scope task."
		return
	}
	s.cancelConnectionRequest()
	s.AgentConnections = newConnectionsView(s.Project.ProjectIdentity, selection, path)
	s.SelectedIdx = 0
	s.refreshConnections()
}

func (s *State) refreshConnections() {
	if s.AgentConnections == nil {
		s.selectConnectionScope("main")
		return
	}
	s.queueConnectionAction(agentConnectionAction{Operation: "read"})
}

func (s *State) queueConnectionAction(action agentConnectionAction) {
	v := s.AgentConnections
	if !s.connectionsCurrent() || v.Pending != "" {
		s.Error = "Select the current checkout and finish or cancel its pending connection request first."
		return
	}
	action.ID, action.Parent, action.Scope = nextTUIReqID("connection"), v.Parent, v.Scope
	v.Pending, v.Operation, v.Key = action.ID, action.Operation, action.Key
	s.enqueueAction(ActionRequest{Kind: "agent-connection", AgentConnection: &action})
}

func (s *State) acceptsConnectionAction(action *agentConnectionAction) bool {
	v := s.AgentConnections
	return action != nil && s.connectionsCurrent() && v.Pending == action.ID && v.Parent == action.Parent && v.Scope == action.Scope
}

func (s *State) cancelConnectionRequest() {
	v := s.AgentConnections
	if v == nil || v.Pending == "" {
		return
	}
	s.enqueueAction(ActionRequest{Kind: "cancel-agent-connection", Target: v.Pending})
	message := "Request canceled. Refresh local inventory to establish whether discovery or a grant completed."
	if v.Operation == "read" {
		for _, kind := range client.ConnectionKinds() {
			v.Failures[kind] = connectionFailure{State: "unavailable", Message: message}
		}
	} else {
		v.Failures[v.Key.Kind] = connectionFailure{State: "unavailable", Message: message}
	}
	v.Pending = ""
	s.CmdOutput = message
}

func executeConnectionAction(ctx context.Context, cl *Client, action agentConnectionAction) agentConnectionResult {
	result := agentConnectionResult{Action: action}
	if cl == nil {
		result.Err = fmt.Errorf("daemon is unavailable")
		return result
	}
	if action.Operation == "read" {
		result.Lists = make(map[client.ConnectionKind]connectionListResult)
		for _, kind := range client.ConnectionKinds() {
			data, err := cl.ReadConnections(ctx, action.ID+"-"+string(kind), action.Scope, kind)
			result.Lists[kind] = connectionListResult{Data: data, Err: err}
		}
	} else if action.Operation == "control" {
		result.Data, result.Err = cl.ControlConnection(ctx, action.ID, action.Scope, action.Key.Kind, action.Key.ID, action.Control)
	} else {
		result.Err = fmt.Errorf("unsupported connection action")
	}
	return result
}

func (s *State) applyConnectionResult(result agentConnectionResult) {
	if !s.acceptsConnectionAction(&result.Action) {
		return
	}
	v := s.AgentConnections
	selected := s.selectedConnectionKey()
	v.Pending = ""
	if result.Action.Operation == "read" {
		for _, kind := range client.ConnectionKinds() {
			list := result.Lists[kind]
			if result.Err != nil {
				list.Err = result.Err
			}
			if list.Err != nil {
				v.Failures[kind] = connectionFailure{State: "unavailable", Message: list.Err.Error()}
			} else {
				v.Records[kind] = list.Data
				delete(v.Failures, kind)
			}
		}
	} else {
		s.applyConnectionControl(result)
	}
	s.restoreConnectionSelection(selected)
}

func (s *State) applyConnectionControl(result agentConnectionResult) {
	v, key := s.AgentConnections, result.Action.Key
	if result.Err != nil {
		v.Failures[key.Kind] = connectionFailure{State: "failed", Message: "Action outcome unavailable. Refresh local inventory: " + result.Err.Error()}
		s.Error = settingsText(v.Failures[key.Kind].Message)
		return
	}
	if result.Data == nil {
		v.Failures[key.Kind] = connectionFailure{State: "unavailable", Message: "No connection receipt returned. Refresh local inventory."}
		return
	}
	for i, item := range v.Records[key.Kind] {
		if item.Config.ID == key.ID {
			v.Records[key.Kind][i] = *result.Data
			delete(v.Failures, key.Kind)
			v.Receipts[key] = connectionReceipt{Configuration: result.Data.Configuration, Action: result.Action.Control.Action}
			s.CmdOutput = "Connection " + key.ID + ": " + result.Data.State + ". Select connections explicitly for each investigation."
			return
		}
	}
	v.Failures[key.Kind] = connectionFailure{State: "unavailable", Message: "Connection list changed. Refresh to inspect the returned grant."}
}

func (s *State) dispatchConnectionAction(ctx context.Context, cl *Client, action ActionRequest) bool {
	switch action.Kind {
	case "agent-connection":
		if s.acceptsConnectionAction(action.AgentConnection) {
			s.applyConnectionResult(executeConnectionAction(ctx, cl, *action.AgentConnection))
		}
	case "cancel-agent-connection":
		if cl != nil {
			cl.Cancel(action.Target)
			for _, kind := range client.ConnectionKinds() {
				cl.Cancel(action.Target + "-" + string(kind))
			}
		}
	default:
		return false
	}
	return true
}

func (s *State) loadAgentConnections(ctx context.Context, cl *Client) {
	if s.Project == nil || s.Project.ProjectID == "" {
		return
	}
	s.AgentConnections = newConnectionsView(s.Project.ProjectIdentity, client.AgentSettingsScope{ProjectIdentity: s.Project.ProjectIdentity}, s.Project.Path)
	v := s.AgentConnections
	v.Pending = nextTUIReqID("connection-sync")
	s.applyConnectionResult(executeConnectionAction(ctx, cl, agentConnectionAction{ID: v.Pending, Parent: v.Parent, Scope: v.Scope, Operation: "read"}))
}

func newConnectionsView(parent ProjectIdentity, scope client.AgentSettingsScope, path string) *agentConnectionsView {
	return &agentConnectionsView{Parent: parent, Scope: scope, Path: path, Records: map[client.ConnectionKind][]client.AgentConnection{}, Failures: map[client.ConnectionKind]connectionFailure{}, Receipts: map[connectionKey]connectionReceipt{}}
}
