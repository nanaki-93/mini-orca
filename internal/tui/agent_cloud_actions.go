package tui

import (
	"context"
	"fmt"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
)

type agentCloudAction struct {
	ID, Epoch, Operation, Expected string
	Parent                         ProjectIdentity
	Scope                          client.AgentSettingsScope
	Selection                      client.CloudSelection
	Control                        client.CloudControl
	Before                         *client.CloudCopy
}
type agentCloudResult struct {
	Action  agentCloudAction
	Data    *client.CloudCatalog
	Index   *ProjectIndex
	Preview *client.CloudPreview
	Copy    *client.CloudCopy
	Err     error
}
type agentCloudResultMsg struct{ result agentCloudResult }

func (s *State) cloudFingerprint() (string, error) {
	if !s.cloudCurrent() || s.AgentCloud.Data == nil {
		return "", fmt.Errorf("refresh cloud copies first")
	}
	v := s.AgentCloud
	return externalFingerprint(struct {
		Parent                       ProjectIdentity
		Path, Epoch, Draft, Selected string
		Data                         *client.CloudCatalog
		Preview                      *client.CloudPreview
	}{v.Parent, v.Path, v.Epoch, v.Draft, s.selectedCloudRow().Key, v.Data, v.Preview})
}

func (s *State) queueCloudAction(a agentCloudAction) {
	v := s.AgentCloud
	if !s.cloudCurrent() || s.EffectivePage() != PageCloud || v.Pending != "" {
		s.Error = "Refresh cloud copies and finish or cancel the pending request first."
		return
	}
	if a.Operation != "read" {
		if err := s.editableCloud(); err != nil {
			s.Error = err.Error()
			return
		}
		fingerprint, err := s.cloudFingerprint()
		if err != nil || a.Expected != "" && a.Expected != fingerprint {
			s.Error = "Cloud selection, source or destination changed. Review again."
			return
		}
		a.Expected = fingerprint
	}
	a.ID, a.Epoch, a.Parent, a.Scope = nextTUIReqID("cloud"), v.Epoch, v.Parent, v.Scope
	v.Pending, v.Operation, v.Status = a.ID, a.Operation, "loading"
	s.enqueueAction(ActionRequest{Kind: "agent-cloud", AgentCloud: &a})
}

func (s *State) acceptsCloudAction(a *agentCloudAction) bool {
	v := s.AgentCloud
	return a != nil && s.cloudCurrent() && s.EffectivePage() == PageCloud && a.ID == v.Pending && a.Epoch == v.Epoch && a.Parent == v.Parent && a.Scope == v.Scope
}

func (s *State) prepareCloudAction(a *agentCloudAction) bool {
	if !s.acceptsCloudAction(a) {
		return false
	}
	if a.Operation != "read" {
		fingerprint, err := s.cloudFingerprint()
		if err != nil || fingerprint != a.Expected {
			s.AgentCloud.Pending, s.AgentCloud.Operation, s.AgentCloud.Status = "", "", "stale"
			s.Error = "Cloud scope or selection changed before dispatch. Refresh and review again."
			return false
		}
	}
	return true
}

func (m *BubbleModel) prepareCloudCmd(a *agentCloudAction) tea.Cmd {
	if !m.state.prepareCloudAction(a) {
		return nil
	}
	captured := *a
	ctx, cancel := context.WithCancel(m.ctx)
	m.state.AgentCloud.Cancel = cancel
	return func() tea.Msg {
		defer cancel()
		return agentCloudResultMsg{executeCloudAction(ctx, m.client, captured)}
	}
}

func (s *State) cancelCloudRequest() {
	v := s.AgentCloud
	if v == nil || v.Pending == "" {
		return
	}
	if v.Cancel != nil {
		v.Cancel()
		v.Cancel = nil
	}
	s.enqueueAction(ActionRequest{Kind: "cancel-agent-cloud", Target: v.Pending})
	if v.Operation == "control" {
		v.Recovery, v.Preview = "The remote outcome may be retained. Refresh local receipts before recovery.", nil
	}
	v.Pending, v.Operation, v.Status = "", "", "canceled"
	v.Failure = "Local request canceled. Refresh to inspect retained state."
}

func executeCloudAction(ctx context.Context, cl *Client, a agentCloudAction) agentCloudResult {
	r := agentCloudResult{Action: a}
	if cl == nil {
		r.Err = fmt.Errorf("daemon connection unavailable")
		return r
	}
	switch a.Operation {
	case "read":
		r.Data, r.Index, r.Err = cl.RefreshCloudCopies(ctx, a.ID, a.Scope)
	case "preview":
		r.Preview, r.Err = cl.PreviewCloudCopy(ctx, a.ID, a.Scope, a.Selection)
	case "control":
		r.Copy, r.Err = cl.ControlCloudCopy(ctx, a.ID, a.Scope, a.Control, a.Before)
	default:
		r.Err = fmt.Errorf("unsupported cloud operation")
	}
	return r
}

func (s *State) applyCloudResult(r agentCloudResult) {
	if !s.acceptsCloudAction(&r.Action) {
		return
	}
	v := s.AgentCloud
	if v.Cancel != nil {
		v.Cancel()
		v.Cancel = nil
	}
	v.Pending, v.Operation = "", ""
	if r.Err != nil {
		v.Status, v.Failure = "unavailable", r.Err.Error()
		if r.Action.Operation == "control" {
			v.Status, v.Preview = "unknown", nil
			v.Recovery = "The remote outcome may be retained. Refresh local receipts before recovery."
			s.Error = v.Recovery
		}
		return
	}
	switch r.Action.Operation {
	case "read":
		s.applyCloudCatalog(r)
	case "preview":
		s.applyCloudPreview(r)
	case "control":
		s.applyCloudReceipt(r)
	}
}

func (s *State) applyCloudCatalog(r agentCloudResult) {
	v := s.AgentCloud
	if r.Data == nil || r.Index == nil {
		v.Status, v.Failure = "unavailable", "Cloud catalog unavailable. Refresh local receipts."
		return
	}
	selected := s.selectedCloudRow().Key
	s.Project.ProjectIdentity, s.Index = r.Index.ProjectIdentity, r.Index
	v.Parent, v.Scope.ProjectIdentity = r.Index.ProjectIdentity, r.Index.ProjectIdentity
	v.Data, v.Preview, v.Status, v.Failure, v.Recovery = r.Data, nil, "current", "", ""
	for i, row := range s.cloudRows() {
		if row.Key == selected {
			s.SelectedIdx = i
			break
		}
	}
	if v.Draft == "" && v.SourceHint != "" {
		for _, destination := range r.Data.Destinations {
			if strings.HasPrefix(v.SourceHint, "session ") == (destination.Target.Service == "vertex_session") {
				v.Draft = destination.Target.ID + " " + v.SourceHint
				break
			}
		}
	}
}

func (s *State) applyCloudPreview(r agentCloudResult) {
	v := s.AgentCloud
	fingerprint, err := s.cloudFingerprint()
	if err != nil || fingerprint != r.Action.Expected || r.Preview == nil {
		v.Status, v.Failure = "stale", "Cloud selection changed while previewing. Refresh and select the source again."
		return
	}
	destination := s.cloudDestination(r.Preview.TargetID)
	if destination == nil || destination.Target != r.Preview.Target || destination.Configuration != r.Preview.Configuration {
		v.Status, v.Failure = "stale", "Destination changed while previewing. Refresh before copying."
		return
	}
	v.Preview, v.Status, v.Failure, v.Details = r.Preview, "current", "", true
	s.DetailScroll = 0
	s.CmdOutput = "Local preview ready. Review the private data and destination, then choose Copy."
}

func (s *State) applyCloudReceipt(r agentCloudResult) {
	v := s.AgentCloud
	fingerprint, err := s.cloudFingerprint()
	if err != nil || fingerprint != r.Action.Expected {
		v.Status, v.Preview = "unknown", nil
		v.Recovery = "Selection changed while the remote operation was pending. Refresh its retained outcome."
		return
	}
	if r.Copy == nil {
		v.Status, v.Recovery = "unknown", "Receipt unavailable. Refresh before recovery."
		v.Preview = nil
		return
	}
	v.Preview, v.Status, v.Failure = nil, "current", ""
	index := slices.IndexFunc(v.Data.Copies, func(c client.CloudCopy) bool { return c.ID == r.Copy.ID })
	if index < 0 {
		v.Data.Copies = append(v.Data.Copies, *r.Copy)
		index = len(v.Data.Copies) - 1
	} else {
		v.Data.Copies[index] = *r.Copy
	}
	s.SelectedIdx, v.Details = len(v.Data.Destinations)+index, true
	s.CmdOutput = "Remote copy outcome: " + r.Copy.State + ". Local originals retained."
	if r.Copy.State == "unknown" {
		v.Recovery = "Remote outcome unknown. Refresh local receipts, then explicitly Check or Delete."
	}
}

func (s *State) dispatchCloudAction(ctx context.Context, cl *Client, a ActionRequest) bool {
	switch a.Kind {
	case "agent-cloud":
		if s.prepareCloudAction(a.AgentCloud) {
			s.applyCloudResult(executeCloudAction(ctx, cl, *a.AgentCloud))
		}
	case "cancel-agent-cloud":
		if cl != nil {
			cl.Cancel(a.Target)
		}
	default:
		return false
	}
	return true
}
