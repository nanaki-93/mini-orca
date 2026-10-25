package tui

import (
	"context"
	"fmt"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
)

type agentTriggerAction struct {
	ID, Epoch, Operation, Expected string
	Parent                         ProjectIdentity
	Scope                          client.AgentSettingsScope
	Control                        client.TriggerControl
}
type agentTriggerResult struct {
	Action agentTriggerAction
	Data   *client.AgentTriggers
	Index  *ProjectIndex
	Err    error
}
type agentTriggerResultMsg struct{ result agentTriggerResult }

func (s *State) triggerFingerprint() (string, error) {
	if !s.triggersCurrent() || s.AgentTriggers.Data == nil {
		return "", fmt.Errorf("refresh ambient state first")
	}
	v := s.AgentTriggers
	d := v.Data
	return externalFingerprint(struct {
		Parent       ProjectIdentity
		Epoch, Draft string
		Binding      client.TriggerBinding
		Revision     int64
		Scope        string
		Options      client.TriggerOptions
		Window       client.TriggerWindow
		Limits       client.AgentLimits
		Ceilings     client.TriggerCeilings
	}{v.Parent, v.Epoch, v.Draft, d.TriggerBinding, d.Record.ControlRevision, d.ScopeHash, d.Record.Options, d.Record.Window, d.RunLimits, d.Ceilings})
}

func (s *State) queueTriggerAction(a agentTriggerAction) {
	v := s.AgentTriggers
	if !s.triggersCurrent() || v.Pending != "" {
		s.Error = "Load current ambient state and finish or cancel the pending request first."
		return
	}
	if a.Operation == "control" {
		if s.editableTriggers(a.Control.Action == "revoke" || a.Control.Action == "disable") == nil {
			return
		}
		fingerprint, err := s.triggerFingerprint()
		if err != nil || fingerprint != a.Expected {
			s.Error = "Ambient scope, draft, settings or budget changed. Review the current control again."
			return
		}
		if a.Control.Options != nil {
			copy := *a.Control.Options
			copy.Paths = slices.Clone(copy.Paths)
			a.Control.Options = &copy
		}
	}
	a.ID, a.Epoch, a.Parent, a.Scope = nextTUIReqID("ambient"), v.Epoch, v.Parent, v.Scope
	v.Pending, v.Operation, v.Status = a.ID, a.Operation, "loading"
	s.enqueueAction(ActionRequest{Kind: "agent-triggers", AgentTriggers: &a})
}

func (s *State) acceptsTriggerAction(a *agentTriggerAction) bool {
	v := s.AgentTriggers
	return a != nil && s.triggersCurrent() && a.ID == v.Pending && a.Epoch == v.Epoch && a.Parent == v.Parent && a.Scope == v.Scope
}

func (s *State) prepareTriggerAction(a *agentTriggerAction) bool {
	if !s.acceptsTriggerAction(a) {
		return false
	}
	if a.Operation == "control" {
		fingerprint, err := s.triggerFingerprint()
		if err != nil || fingerprint != a.Expected {
			s.AgentTriggers.Pending, s.AgentTriggers.Status = "", "stale"
			s.Error = "Ambient scope, draft or limits changed before dispatch. Refresh and review again."
			return false
		}
	}
	return true
}

func (m *BubbleModel) prepareTriggerCmd(a *agentTriggerAction) tea.Cmd {
	if !m.state.prepareTriggerAction(a) {
		return nil
	}
	captured := *a
	ctx, cancel := context.WithCancel(m.ctx)
	m.state.AgentTriggers.Cancel = cancel
	return func() tea.Msg {
		defer cancel()
		return agentTriggerResultMsg{executeTriggerAction(ctx, m.client, captured)}
	}
}

func (s *State) cancelTriggerRequest() {
	v := s.AgentTriggers
	if v == nil || v.Pending == "" {
		return
	}
	if v.Cancel != nil {
		v.Cancel()
		v.Cancel = nil
	}
	s.enqueueAction(ActionRequest{Kind: "cancel-agent-triggers", Target: v.Pending})
	if v.Operation == "control" {
		v.Recovery = "The control may have completed. Refresh before another control."
	}
	v.Pending, v.Operation, v.Status = "", "", "canceled"
	v.Failure = "Local request canceled. Refresh to inspect current daemon permission and any retained outcome."
}

func executeTriggerAction(ctx context.Context, cl *Client, a agentTriggerAction) agentTriggerResult {
	r := agentTriggerResult{Action: a}
	if cl == nil {
		r.Err = fmt.Errorf("daemon connection unavailable")
		return r
	}
	switch a.Operation {
	case "read":
		r.Data, r.Index, r.Err = cl.RefreshAgentTriggers(ctx, a.ID, a.Scope)
	case "control":
		r.Data, r.Err = cl.ControlAgentTriggers(ctx, a.ID, a.Scope, a.Control)
	default:
		r.Err = fmt.Errorf("unsupported ambient operation")
	}
	return r
}

func (s *State) applyTriggerResult(r agentTriggerResult) {
	if !s.acceptsTriggerAction(&r.Action) {
		return
	}
	v := s.AgentTriggers
	if v.Cancel != nil {
		v.Cancel()
		v.Cancel = nil
	}
	v.Pending, v.Operation = "", ""
	if r.Err != nil || r.Data == nil {
		v.Status, v.Failure = "unavailable", "Ambient metadata is unavailable. Refresh to inspect retained state."
		if r.Err != nil {
			v.Failure = r.Err.Error()
		}
		if r.Action.Operation == "control" {
			v.Status = "unknown"
			v.Recovery = "The control may have completed. Refresh before another control."
			s.Error = v.Recovery + " " + v.Failure
		}
		return
	}
	if r.Index != nil {
		s.Project.ProjectIdentity = r.Index.ProjectIdentity
		s.Index, v.Index = r.Index, r.Index
		v.Parent, v.Scope.ProjectIdentity = r.Index.ProjectIdentity, r.Index.ProjectIdentity
	}
	v.Data, v.Status, v.Failure, v.Recovery = r.Data, "current", "", ""
	if v.Draft == "changes " && r.Data.Record.Options.Mode != "disabled" {
		v.Draft = triggerWatchDraft(r.Data.Record.Options)
	}
	if r.Action.Operation == "control" {
		s.CmdOutput = "Ambient control recorded: " + r.Action.Control.Action + ". Window charges are retained."
	}
}

func triggerWatchDraft(o client.TriggerOptions) string {
	prefix := o.Mode + " "
	if o.Mode == "interval" {
		prefix += fmt.Sprintf("%d ", o.IntervalSeconds)
	}
	return prefix + strings.Join(o.Paths, "; ")
}

func (s *State) dispatchTriggerAction(ctx context.Context, cl *Client, a ActionRequest) bool {
	switch a.Kind {
	case "agent-triggers":
		if s.prepareTriggerAction(a.AgentTriggers) {
			s.applyTriggerResult(executeTriggerAction(ctx, cl, *a.AgentTriggers))
		}
	case "cancel-agent-triggers":
		if cl != nil {
			cl.Cancel(a.Target)
		}
	default:
		return false
	}
	return true
}
