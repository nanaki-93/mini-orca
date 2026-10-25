package tui

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
)

type agentTriggerView struct {
	Parent                                               ProjectIdentity
	Scope                                                client.AgentSettingsScope
	Path, Epoch                                          string
	Data                                                 *client.AgentTriggers
	Index                                                *ProjectIndex
	Draft, Pending, Operation, Status, Failure, Recovery string
	Details                                              bool
	Cancel                                               context.CancelFunc
}

func (s *State) triggersCurrent() bool {
	v := s.AgentTriggers
	return v != nil && v.Scope.WorktreePath == "" && s.checkoutScopeCurrent(v.Parent, v.Scope, v.Path)
}

func (s *State) reconcileTriggerScope() {
	v := s.AgentTriggers
	if v == nil {
		return
	}
	if s.Project == nil || s.Project.ProjectID != v.Parent.ProjectID || s.Project.Path != v.Path {
		s.cancelTriggerRequest()
		s.AgentTriggers = nil
		return
	}
	if s.Project.ProjectIdentity != v.Parent {
		s.cancelTriggerRequest()
		v.Parent, v.Scope.ProjectIdentity = s.Project.ProjectIdentity, s.Project.ProjectIdentity
		v.Epoch, v.Status = nextTUIReqID("ambient-view"), "stale"
		v.Failure = "Project revision changed. Refresh local state before granting provider work."
		v.Index = nil
	}
}

func (s *State) refreshTriggers() {
	s.reconcileTriggerScope()
	if s.Project == nil || s.Project.ProjectID == "" {
		s.Error = "Open a project before inspecting ambient analysis."
		return
	}
	if !s.triggersCurrent() {
		s.AgentTriggers = &agentTriggerView{Parent: s.Project.ProjectIdentity, Scope: client.AgentSettingsScope{ProjectIdentity: s.Project.ProjectIdentity}, Path: s.Project.Path, Epoch: nextTUIReqID("ambient-view"), Draft: "changes "}
	}
	s.Navigate(PageAmbient)
	s.queueTriggerAction(agentTriggerAction{Operation: "read"})
}

func (s *State) execTriggerCommand(command, arg string) bool {
	switch command {
	case "ambient":
		s.Navigate(PageAmbient)
		if !s.triggersCurrent() || s.AgentTriggers.Data == nil {
			s.refreshTriggers()
		}
	case "ambient-refresh":
		s.refreshTriggers()
	case "ambient-cancel":
		s.cancelTriggerRequest()
	case "ambient-watch":
		s.confirmTriggerWatch(arg)
	case "ambient-grant":
		s.confirmTriggerGrant(arg)
	case "ambient-revoke":
		s.confirmTriggerStop("revoke")
	case "ambient-disable":
		s.confirmTriggerStop("disable")
	default:
		return false
	}
	return true
}

func (s *State) handleTriggerKey(key string) bool {
	switch key {
	case "r":
		s.refreshTriggers()
	case "w":
		draft := "changes "
		if s.triggersCurrent() {
			draft = s.AgentTriggers.Draft
		}
		s.openCommandInput("ambient-watch " + draft)
	case "g":
		s.confirmTriggerGrant("")
	case "v":
		s.confirmTriggerStop("revoke")
	case "d":
		s.confirmTriggerStop("disable")
	case "x":
		s.cancelTriggerRequest()
	case "Enter":
		if !s.triggersCurrent() || s.AgentTriggers.Data == nil {
			s.refreshTriggers()
			return true
		}
		s.AgentTriggers.Details = !s.AgentTriggers.Details
		s.DetailScroll = 0
	default:
		return false
	}
	return true
}

func (s *State) editableTriggers(stopping bool) *client.AgentTriggers {
	s.reconcileTriggerScope()
	v := s.AgentTriggers
	if !s.triggersCurrent() || v.Data == nil || v.Pending != "" {
		s.Error = "Refresh ambient state for the main checkout and finish or cancel its pending request first."
		return nil
	}
	if v.Recovery != "" {
		s.Error = "Refresh ambient state before another control; the previous request may have completed."
		return nil
	}
	if !stopping && (v.Status != "current" || v.Data.ProjectIdentity != v.Scope.ProjectIdentity) {
		s.Error = "Refresh current ambient metadata before watching or granting."
		return nil
	}
	return v.Data
}

func parseTriggerWatch(arg string) (client.TriggerOptions, error) {
	mode, rest, _ := strings.Cut(strings.TrimSpace(arg), " ")
	o := client.TriggerOptions{Mode: mode}
	if mode == "interval" {
		var seconds string
		seconds, rest, _ = strings.Cut(strings.TrimSpace(rest), " ")
		value, err := strconv.Atoi(seconds)
		if err != nil {
			return o, fmt.Errorf("use :ambient-watch interval <seconds> <path; path>")
		}
		o.IntervalSeconds = value
	}
	for _, path := range strings.Split(rest, ";") {
		path = strings.TrimSpace(path)
		if path != "" {
			o.Paths = append(o.Paths, path)
		}
	}
	if mode == "disabled" {
		return o, fmt.Errorf("use :ambient-disable to stop watching")
	}
	return o, o.Validate()
}

func (s *State) confirmTriggerWatch(arg string) {
	d := s.editableTriggers(false)
	if d == nil {
		return
	}
	s.AgentTriggers.Draft = arg
	o, err := parseTriggerWatch(arg)
	if err != nil {
		s.Error = err.Error()
		return
	}
	if err = s.checkTriggerPaths(o.Paths); err != nil {
		s.Error = err.Error()
		return
	}
	r := client.TriggerControl{TriggerBinding: d.TriggerBinding, ControlRevision: d.Record.ControlRevision, Action: "watch", Confirm: true, Options: &o}
	message := fmt.Sprintf("Watch these main-checkout sources in %s mode", o.Mode)
	if o.Mode == "interval" {
		message += fmt.Sprintf(" every %d seconds", o.IntervalSeconds)
	}
	message += ".\n" + strings.Join(o.Paths, "\n") + "\nDetection records suggestions. Provider permission requires a separate grant. Any previous grant is revoked. A fresh baseline skips past changes."
	s.confirmTriggerAction(r, "Set ambient watch", "Watch", message)
}

func (s *State) checkTriggerPaths(paths []string) error {
	index := s.AgentTriggers.Index
	if index == nil || index.ProjectIdentity != s.AgentTriggers.Scope.ProjectIdentity {
		return fmt.Errorf("refresh the current source index before choosing paths")
	}
	for _, path := range paths {
		found := slices.ContainsFunc(index.Files, func(f client.IndexedFile) bool {
			return f.Path == path && !f.Binary && f.Language != "Text" && f.Language != "Markdown"
		})
		if !found {
			return fmt.Errorf("select an indexed source path: %s", settingsText(path))
		}
	}
	return nil
}

func (s *State) confirmTriggerGrant(arg string) {
	d := s.editableTriggers(false)
	if d == nil {
		return
	}
	if d.Record.Options.Mode == "disabled" || d.State == "stale_settings" {
		s.Error = "Set a watch and resolve current agent settings before granting automatic analysis."
		return
	}
	seconds := min(3600, d.Ceilings.GrantSeconds)
	if arg != "" {
		value, err := strconv.Atoi(strings.TrimSpace(arg))
		if err != nil {
			s.Error = "Use :ambient-grant <seconds>."
			return
		}
		seconds = value
	}
	if seconds < 60 || seconds > d.Ceilings.GrantSeconds {
		s.Error = fmt.Sprintf("Grant lifetime must be 60–%d seconds.", d.Ceilings.GrantSeconds)
		return
	}
	r := client.TriggerControl{TriggerBinding: d.TriggerBinding, ControlRevision: d.Record.ControlRevision, Action: "grant", Confirm: true, ScopeHash: d.ScopeHash, GrantSeconds: seconds, ConfirmSourceSharing: true, ConfirmRemoteProvider: d.Destination.RemoteProvider}
	l := d.RunLimits
	message := fmt.Sprintf("Allow automatic semantic advice for %d seconds.\nSelected paths:\n%s\nShare these sources and filtered project context with %s at %s.\nPer attempt: %d model calls, %d input tokens, %d output tokens, %ds.\nWindow: at most %d attempts in %ds. Existing window charges remain.\nNo source changes or project commands. Every attempt remains advisory.", seconds, strings.Join(d.Record.Options.Paths, "\n"), d.Model, d.ProviderOrigin, l.ModelCalls, l.InputTokens, l.OutputTokens, l.ElapsedSeconds, d.Ceilings.MaxRuns, d.Ceilings.WindowSeconds)
	message += "\n" + triggerWindowSummary(d.Record.Window)
	s.confirmTriggerAction(r, "Grant automatic analysis", "Grant", message)
}

func (s *State) confirmTriggerStop(action string) {
	d := s.editableTriggers(true)
	if d == nil {
		return
	}
	r := client.TriggerControl{TriggerBinding: d.TriggerBinding, ControlRevision: d.Record.ControlRevision, Action: action, Confirm: true}
	r.ProjectIdentity = s.AgentTriggers.Scope.ProjectIdentity
	title, accept, message := "Revoke automatic permission", "Revoke", "Stop and join automatic analysis, remove permission and retain a fresh watch-only baseline."
	if action == "disable" {
		title, accept, message = "Disable ambient analysis", "Disable", "Stop and join automatic analysis, remove permission and clear the watch selection."
	}
	s.confirmTriggerAction(r, title, accept, message+"\nRetained attempt receipts and window charges are preserved.")
}

func (s *State) confirmTriggerAction(r client.TriggerControl, title, accept, message string) {
	s.Navigate(PageAmbient)
	a := agentTriggerAction{Operation: "control", Control: r}
	expected, err := s.triggerFingerprint()
	if err != nil {
		s.Error = err.Error()
		return
	}
	a.Expected = expected
	message += "\nMain checkout: " + s.AgentTriggers.Path + "\nRevision: " + s.AgentTriggers.Scope.ProjectRevision
	s.Confirmation = &Confirmation{Title: title, AcceptLabel: accept, Message: externalSafeText(message), OnConfirm: func(state *State) { state.queueTriggerAction(a) }}
}
