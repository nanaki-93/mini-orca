package tui

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
)

type agentCloudView struct {
	Parent                                                           ProjectIdentity
	Scope                                                            client.AgentSettingsScope
	Path, Epoch                                                      string
	Data                                                             *client.CloudCatalog
	Preview                                                          *client.CloudPreview
	Draft, SourceHint, Pending, Operation, Status, Failure, Recovery string
	Details                                                          bool
	Cancel                                                           context.CancelFunc
}

func (s *State) cloudCurrent() bool {
	v := s.AgentCloud
	return v != nil && v.Scope.WorktreePath == "" && s.checkoutScopeCurrent(v.Parent, v.Scope, v.Path)
}

func (s *State) reconcileCloudScope() {
	v := s.AgentCloud
	if v == nil {
		return
	}
	if s.Project == nil || s.Project.ProjectID != v.Parent.ProjectID || s.Project.Path != v.Path {
		s.cancelCloudRequest()
		s.AgentCloud = nil
		return
	}
	if s.Project.ProjectIdentity != v.Parent {
		s.cancelCloudRequest()
		v.Parent, v.Scope.ProjectIdentity = s.Project.ProjectIdentity, s.Project.ProjectIdentity
		v.Epoch, v.Status, v.Preview = nextTUIReqID("cloud-view"), "stale", nil
		v.Failure = "Project revision changed. Refresh local receipts before another action."
	}
}

func (s *State) refreshCloudCopies() {
	hint := s.cloudSourceHint()
	s.Navigate(PageCloud)
	s.reconcileCloudScope()
	if s.Project == nil || s.Project.ProjectID == "" {
		s.Error = "Open a project before inspecting cloud copies."
		return
	}
	if !s.cloudCurrent() {
		s.AgentCloud = &agentCloudView{Parent: s.Project.ProjectIdentity, Scope: client.AgentSettingsScope{ProjectIdentity: s.Project.ProjectIdentity}, Path: s.Project.Path, Epoch: nextTUIReqID("cloud-view"), SourceHint: hint}
	}
	s.queueCloudAction(agentCloudAction{Operation: "read"})
}

func (s *State) cloudSourceHint() string {
	if s.EffectivePage() == PageArtifacts && s.libraryCurrent() && s.AgentLibrary.Scope.WorktreePath == "" {
		if key := s.selectedLibraryKey(); key.ID != "" {
			return "artifact " + key.ID
		}
	}
	if s.EffectivePage() == PageAgents && s.AgentDetailOpen {
		if detail := s.currentAgentDetail(); detail != nil && s.AgentProgress.Request.WorktreePath == "" {
			return "session " + detail.Run.ID
		}
	}
	return ""
}

func (s *State) execCloudCommand(command, arg string) bool {
	switch command {
	case "cloud":
		if s.cloudCurrent() && s.AgentCloud.Data != nil {
			s.Navigate(PageCloud)
		} else {
			s.refreshCloudCopies()
		}
	case "cloud-refresh":
		s.refreshCloudCopies()
	case "cloud-preview":
		s.previewCloudCopy(arg)
	case "cloud-copy":
		s.confirmCloudControl("copy")
	case "cloud-check":
		s.confirmCloudControl("check")
	case "cloud-delete":
		s.confirmCloudControl("delete")
	case "cloud-cancel":
		s.cancelCloudRequest()
	default:
		return false
	}
	return true
}

func (s *State) handleCloudKey(key string) bool {
	switch key {
	case "r":
		s.refreshCloudCopies()
	case "e":
		draft := ""
		if s.cloudCurrent() {
			draft = s.AgentCloud.Draft
		}
		s.openCommandInput("cloud-preview " + draft)
	case "c":
		s.confirmCloudControl("copy")
	case "i":
		s.confirmCloudControl("check")
	case "d":
		s.confirmCloudControl("delete")
	case "x":
		s.cancelCloudRequest()
	case "Enter":
		if !s.cloudCurrent() || s.AgentCloud.Data == nil {
			s.refreshCloudCopies()
			return true
		}
		s.AgentCloud.Details = !s.AgentCloud.Details
		s.DetailScroll = 0
	default:
		return false
	}
	return true
}

func (s *State) editableCloud() error {
	s.reconcileCloudScope()
	v := s.AgentCloud
	if !s.cloudCurrent() || s.EffectivePage() != PageCloud || v.Data == nil || v.Status != "current" || v.Data.ProjectIdentity != v.Scope.ProjectIdentity {
		return fmt.Errorf("refresh cloud copies for the current main checkout first")
	}
	if v.Pending != "" || v.Recovery != "" {
		return fmt.Errorf("finish or cancel the pending request, then refresh local receipts before another action")
	}
	return nil
}

func (s *State) previewCloudCopy(arg string) {
	if err := s.editableCloud(); err != nil {
		s.Error = err.Error()
		return
	}
	v := s.AgentCloud
	v.Draft, v.Preview = strings.TrimSpace(arg), nil
	parts := strings.Fields(arg)
	if len(parts) != 3 || !slices.Contains([]string{"artifact", "session"}, parts[1]) {
		s.Error = "Use :cloud-preview <destination> artifact|session <source-id>."
		return
	}
	selection := client.CloudSelection{ProjectIdentity: v.Scope.ProjectIdentity, TargetID: parts[0], Kind: parts[1], SourceID: parts[2]}
	if s.cloudDestination(selection.TargetID) == nil {
		s.Error = "Select a configured destination from this view."
		return
	}
	s.queueCloudAction(agentCloudAction{Operation: "preview", Selection: selection})
}

func (s *State) cloudDestination(id string) *client.CloudDestination {
	if !s.cloudCurrent() || s.AgentCloud.Data == nil {
		return nil
	}
	for i := range s.AgentCloud.Data.Destinations {
		item := &s.AgentCloud.Data.Destinations[i]
		if item.Target.ID == id {
			return item
		}
	}
	return nil
}

func (s *State) cloudControl(action string) (agentCloudAction, error) {
	v := s.AgentCloud
	a := agentCloudAction{Operation: "control", Control: client.CloudControl{ProjectIdentity: v.Scope.ProjectIdentity, Action: action, Confirm: true}}
	if action == "copy" {
		if v.Preview == nil {
			return a, fmt.Errorf("preview the selected source before copying")
		}
		preview := *v.Preview
		a.Control.ID, a.Control.Preview, a.Control.ConfirmSourceSharing = preview.ID, &preview, true
		return a, nil
	}
	row := s.selectedCloudRow()
	if row.Copy == nil {
		return a, fmt.Errorf("select a retained copy to check or delete")
	}
	before := *row.Copy
	destination := s.cloudDestination(before.Target.ID)
	if destination == nil || destination.Target != before.Target || destination.Configuration != before.Configuration {
		return a, fmt.Errorf("restore this copy's recorded destination configuration before remote recovery")
	}
	if before.State == "deleted" || before.LeaseUntil.After(time.Now()) {
		return a, fmt.Errorf("this copy is deleted or its operation lease is still active; refresh after the lease expires")
	}
	a.Before, a.Control.ID, a.Control.Version = &before, before.ID, before.Version
	return a, nil
}

func (s *State) confirmCloudControl(action string) {
	if err := s.editableCloud(); err != nil {
		s.Error = err.Error()
		return
	}
	a, err := s.cloudControl(action)
	if err != nil {
		s.Error = err.Error()
		return
	}
	a.Expected, err = s.cloudFingerprint()
	if err != nil {
		s.Error = err.Error()
		return
	}
	target := client.CloudTarget{}
	title, label, message := "Check remote copy", "Check", "Contact the recorded destination to verify this copy. This does not repeat an upload."
	if a.Before != nil {
		target = a.Before.Target
		message += "\nCopy: " + a.Before.ID + "\nSource: " + a.Before.SourceID + "\nOriginal revision: " + a.Before.SourceRevision + "\nSHA-256: " + a.Before.Hash
	}
	if action == "copy" {
		p := a.Control.Preview
		target = p.Target
		title, label = "Copy private data to cloud", "Copy"
		message = fmt.Sprintf("Share this selected private %s with the destination below.\nSource: %s\nSource revision: %s\nSource version: %d\nArchive: %d bytes\nSHA-256: %s\nPrompts, tool output or artifact content may be included. This archive cannot resume a workflow.", p.Kind, p.SourceID, p.SourceRevision, p.SourceVersion, p.Size, p.Hash)
	}
	if action == "delete" {
		title, label = "Delete remote copy", "Delete"
		message = strings.Replace(message, "Contact the recorded destination to verify this copy. This does not repeat an upload.", "Delete the live remote copy. Provider retention, soft-delete or backups may retain data.", 1)
	}
	message += "\n" + cloudDestinationText(target) + "\nLocal originals remain intact. Cleanup is manual, not an automatic provider TTL.\nMain checkout: " + s.AgentCloud.Path + "\nCurrent revision: " + s.AgentCloud.Scope.ProjectRevision
	s.Confirmation = &Confirmation{Title: title, AcceptLabel: label, Message: externalSafeText(message), OnConfirm: func(state *State) { state.queueCloudAction(a) }}
}
