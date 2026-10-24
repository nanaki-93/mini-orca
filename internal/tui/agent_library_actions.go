package tui

import (
	"context"
	"fmt"
	"slices"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
)

const libraryArtifacts, libraryMemory = "artifacts", "memory"

type libraryKey struct{ Kind, ID string }

type agentLibraryView struct {
	Parent             ProjectIdentity
	Scope              client.AgentSettingsScope
	Path               string
	Artifacts          *client.ArtifactList
	Memories           []client.AgentMemory
	Artifact           *client.ArtifactDetail
	Memory             *client.AgentMemory
	Source             *client.LibrarySource
	SourceFor          string
	Export             *client.ArtifactReceipt
	Loaded             map[string]bool
	Failures           map[string]string
	Pending, Operation string
	Key                libraryKey
	Cancel             context.CancelFunc
}

type agentLibraryAction struct {
	ID, Operation, Cursor, Snapshot, Expected string
	ReportID                                  string
	Parent                                    ProjectIdentity
	Scope                                     client.AgentSettingsScope
	Key                                       libraryKey
	Artifact                                  client.ArtifactControl
	Memory                                    client.MemoryMutation
	Draft                                     *libraryDraft
	Source                                    client.MemorySource
}

type agentLibraryResult struct {
	Action    agentLibraryAction
	Artifacts *client.ArtifactList
	Memories  []client.AgentMemory
	Artifact  *client.ArtifactDetail
	Memory    *client.AgentMemory
	Receipt   *client.ArtifactReceipt
	Index     *ProjectIndex
	Source    *client.LibrarySource
	Failures  map[string]error
	Err       error
}

func newLibraryView(parent ProjectIdentity, scope client.AgentSettingsScope, path string) *agentLibraryView {
	return &agentLibraryView{Parent: parent, Scope: scope, Path: path, Loaded: map[string]bool{}, Failures: map[string]string{}}
}

func (s *State) libraryCurrent() bool {
	v := s.AgentLibrary
	return v != nil && s.checkoutScopeCurrent(v.Parent, v.Scope, v.Path)
}

func (s *State) libraryReady(kind string) error {
	if !s.libraryCurrent() || !s.AgentLibrary.Loaded[kind] || s.AgentLibrary.Failures[kind] != "" {
		return fmt.Errorf("refresh the library for this checkout before acting")
	}
	if s.AgentLibrary.Pending != "" {
		return fmt.Errorf("finish or cancel the pending library request first")
	}
	return nil
}

func (s *State) selectLibraryScope(selection string) {
	if s.Project == nil || s.Project.ProjectID == "" {
		s.Error = "Open a project before loading its library."
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
		s.Error = "Use :library-scope main or :library-scope task."
		return
	}
	s.cancelLibraryRequest()
	s.AgentLibrary = newLibraryView(s.Project.ProjectIdentity, scope, path)
	s.SelectedIdx = 0
	s.refreshLibrary()
}

func (s *State) refreshLibrary() {
	if s.AgentLibrary == nil {
		s.selectLibraryScope("main")
		return
	}
	s.queueLibraryAction(agentLibraryAction{Operation: "read"})
}

func (s *State) queueLibraryAction(action agentLibraryAction) {
	if !s.libraryCurrent() || s.AgentLibrary.Pending != "" {
		s.Error = "Load the current library checkout and finish or cancel its pending request first."
		return
	}
	v := s.AgentLibrary
	action.ID, action.Parent, action.Scope = nextTUIReqID("library"), v.Parent, v.Scope
	v.Pending, v.Operation, v.Key = action.ID, action.Operation, action.Key
	s.enqueueAction(ActionRequest{Kind: "agent-library", AgentLibrary: &action})
}

func (s *State) acceptsLibraryAction(action *agentLibraryAction) bool {
	return action != nil && s.libraryCurrent() && s.AgentLibrary.Pending == action.ID && s.AgentLibrary.Parent == action.Parent && s.AgentLibrary.Scope == action.Scope
}

func (s *State) prepareLibraryAction(action *agentLibraryAction) bool {
	if !s.acceptsLibraryAction(action) {
		return false
	}
	if err := s.libraryAdmission(*action); err != nil {
		s.AgentLibrary.Pending, s.Error = "", externalSafeText(err.Error())
		return false
	}
	return true
}

func (m *BubbleModel) prepareLibraryCmd(action *agentLibraryAction) tea.Cmd {
	if !m.state.prepareLibraryAction(action) {
		return nil
	}
	captured := *action
	owned, cancel := context.WithCancel(m.ctx)
	m.state.AgentLibrary.Cancel = cancel
	return func() tea.Msg {
		defer cancel()
		return agentLibraryResultMsg{result: executeLibraryAction(owned, m.client, captured)}
	}
}

func (s *State) cancelLibraryRequest() {
	v := s.AgentLibrary
	if v == nil || v.Pending == "" {
		return
	}
	if v.Cancel != nil {
		v.Cancel()
		v.Cancel = nil
	}
	s.enqueueAction(ActionRequest{Kind: "cancel-agent-library", Target: v.Pending})
	message := "Local read canceled. Refresh to load current records."
	if v.Operation == "artifact-control" || v.Operation == "memory-control" {
		message = "Request canceled. A local write may have completed; refresh before another action."
	}
	s.markLibraryUnavailable(v.Key.Kind, message)
	v.Pending = ""
	s.CmdOutput = message
}

func executeLibraryAction(ctx context.Context, cl *Client, action agentLibraryAction) agentLibraryResult {
	r := agentLibraryResult{Action: action}
	if cl == nil {
		r.Err = fmt.Errorf("daemon is unavailable")
		return r
	}
	switch action.Operation {
	case "read":
		r.Failures = map[string]error{}
		r.Artifacts, r.Failures[libraryArtifacts] = cl.ReadArtifacts(ctx, action.ID+"-artifacts", action.Scope, "")
		r.Memories, r.Failures[libraryMemory] = cl.ReadMemories(ctx, action.ID+"-memory", action.Scope)
	case "artifact-page":
		r.Artifacts, r.Err = cl.ReadArtifacts(ctx, action.ID, action.Scope, action.Cursor)
	case "artifact-detail":
		r.Artifact, r.Err = cl.ArtifactDetail(ctx, action.ID, action.Scope, action.Key.ID)
	case "artifact-control":
		r.Receipt, r.Err = cl.ControlArtifact(ctx, action.ID, action.Scope, action.Key.ID, action.Artifact)
	case "memory-detail":
		r.Memory, r.Err = cl.MemoryDetail(ctx, action.ID, action.Scope, action.Key.ID)
	case "memory-control":
		r.Memory, r.Err = cl.MutateMemory(ctx, action.ID, action.Scope, action.Key.ID, action.Memory)
	case "capture":
		r.Index, r.Err = cl.ExternalIndex(ctx, action.ID, action.Scope)
	case "source":
		r.Source, r.Err = cl.LibrarySource(ctx, action.ID, action.Scope, action.Source)
	case "report":
		r.Err = openResearchReport(ctx, cl, action.Scope, action.ReportID)
	default:
		r.Err = fmt.Errorf("unsupported library action")
	}
	return r
}

func (s *State) applyLibraryResult(r agentLibraryResult) {
	if !s.acceptsLibraryAction(&r.Action) {
		return
	}
	v, selected := s.AgentLibrary, s.selectedLibraryKey()
	v.Pending, v.Cancel = "", nil
	if err := s.libraryAdmission(r.Action); err != nil {
		s.markLibraryUnavailable(r.Action.Key.Kind, "A late library response no longer matches the reviewed record. Refresh its outcome.")
		s.Error = externalSafeText(err.Error())
		return
	}
	if r.Action.Operation == "read" {
		s.applyLibraryInventory(r)
	} else if r.Err != nil {
		s.markLibraryUnavailable(r.Action.Key.Kind, "Outcome unavailable; refresh the library. "+r.Err.Error())
		s.Error = externalSafeText(r.Err.Error())
	} else {
		s.applyLibraryReceipt(r)
	}
	if r.Action.Operation == "memory-control" && r.Action.Memory.Action == "create" && r.Err == nil && r.Memory != nil {
		selected = libraryKey{libraryMemory, r.Memory.ID}
	}
	s.restoreLibrarySelection(selected)
}

func (s *State) applyLibraryInventory(r agentLibraryResult) {
	v := s.AgentLibrary
	if r.Err != nil {
		s.markLibraryUnavailable("", r.Err.Error())
		return
	}
	for _, kind := range []string{libraryArtifacts, libraryMemory} {
		if err := r.Failures[kind]; err != nil {
			s.markLibraryUnavailable(kind, err.Error())
			continue
		}
		v.Loaded[kind], v.Failures[kind] = true, ""
		if kind == libraryArtifacts {
			v.Artifacts, v.Artifact = r.Artifacts, nil
		} else {
			v.Memories, v.Memory, v.Source = r.Memories, nil, nil
		}
	}
}

func (s *State) markLibraryUnavailable(kind, message string) {
	v := s.AgentLibrary
	for _, group := range []string{libraryArtifacts, libraryMemory} {
		if kind != "" && kind != group {
			continue
		}
		v.Failures[group], v.Loaded[group] = externalSafeText(message), false
		if group == libraryArtifacts {
			v.Artifact = nil
		} else {
			v.Memory, v.Source = nil, nil
		}
	}
}

func (s *State) applyLibraryReceipt(r agentLibraryResult) {
	v := s.AgentLibrary
	switch r.Action.Operation {
	case "artifact-page":
		if r.Artifacts == nil || r.Action.Snapshot != "" && r.Artifacts.Snapshot != r.Action.Snapshot {
			s.markLibraryUnavailable(libraryArtifacts, "Artifact catalog changed. Refresh from the first page.")
			return
		}
		v.Artifacts, v.Artifact, s.SelectedIdx, s.DetailScroll = r.Artifacts, nil, 0, 0
	case "artifact-detail":
		v.Artifact = r.Artifact
	case "memory-detail":
		v.Memory, v.Source = r.Memory, nil
		if r.Memory != nil {
			v.Memories = replaceLibraryMemory(v.Memories, *r.Memory)
		}
	case "artifact-control":
		s.applyLibraryArtifactReceipt(r)
	case "memory-control":
		s.applyLibraryMemoryReceipt(r)
	case "capture":
		s.finishLibraryCapture(r)
	case "source":
		v.Source, v.SourceFor = r.Source, r.Action.Key.ID
		s.CmdOutput = "Current source matches the retained hash. Read-only preview; PgUp/PgDn scrolls."
	case "report":
		s.CmdOutput = "Sent the retained local research report to the system browser."
	}
}

func (s *State) applyLibraryArtifactReceipt(r agentLibraryResult) {
	v := s.AgentLibrary
	if r.Receipt == nil {
		s.markLibraryUnavailable(libraryArtifacts, "Artifact action returned no receipt; refresh before another action.")
		return
	}
	if r.Receipt.State == "exported" {
		v.Export = r.Receipt
		s.CmdOutput = "Exported private copy: " + externalSafeText(r.Receipt.Path)
		return
	}
	if v.Artifacts != nil {
		v.Artifacts.Artifacts = slices.DeleteFunc(v.Artifacts.Artifacts, func(item client.AgentArtifact) bool { return item.ID == r.Receipt.ID })
		v.Artifacts.NextCursor = ""
	}
	v.Artifact = nil
	s.CmdOutput = "Deleted the exact artifact version. Exported copies remain separate. Refresh to restart pagination."
}

func replaceLibraryMemory(items []client.AgentMemory, item client.AgentMemory) []client.AgentMemory {
	item.Text, item.SourceStatus = "", "unchecked"
	for i := range items {
		if items[i].ID == item.ID {
			items[i] = item
			return items
		}
	}
	return append([]client.AgentMemory{item}, items...)
}

func (s *State) applyLibraryMemoryReceipt(r agentLibraryResult) {
	v := s.AgentLibrary
	if r.Action.Memory.Action == "forget" {
		v.Memories = slices.DeleteFunc(v.Memories, func(item client.AgentMemory) bool { return item.ID == r.Action.Key.ID })
		v.Memory, v.Source = nil, nil
		s.CmdOutput = "Removed the retained advice. Prior invocation history remains separate."
		return
	}
	if r.Memory == nil {
		s.markLibraryUnavailable(libraryMemory, "Memory action returned no receipt; refresh before another action.")
		return
	}
	v.Memory, v.Source = r.Memory, nil
	v.Memories = replaceLibraryMemory(v.Memories, *r.Memory)
	s.CmdOutput = "Memory retained as " + r.Memory.State + ". Candidate advice requires separate acceptance before retrieval."
}

func (s *State) dispatchLibraryAction(ctx context.Context, cl *Client, action ActionRequest) bool {
	switch action.Kind {
	case "agent-library":
		if s.prepareLibraryAction(action.AgentLibrary) {
			s.applyLibraryResult(executeLibraryAction(ctx, cl, *action.AgentLibrary))
		}
	case "cancel-agent-library":
		if cl != nil {
			for _, suffix := range []string{"", "-artifacts", "-memory", "-before", "-after"} {
				cl.Cancel(action.Target + suffix)
			}
		}
	default:
		return false
	}
	return true
}

func (s *State) loadLibrary(ctx context.Context, cl *Client) {
	if s.Project == nil || s.Project.ProjectID == "" {
		return
	}
	s.cancelLibraryRequest()
	s.AgentLibrary = newLibraryView(s.Project.ProjectIdentity, client.AgentSettingsScope{ProjectIdentity: s.Project.ProjectIdentity}, s.Project.Path)
	v := s.AgentLibrary
	v.Pending = nextTUIReqID("library-sync")
	s.applyLibraryResult(executeLibraryAction(ctx, cl, agentLibraryAction{ID: v.Pending, Parent: v.Parent, Scope: v.Scope, Operation: "read"}))
}
