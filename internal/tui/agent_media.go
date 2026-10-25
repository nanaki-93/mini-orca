package tui

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
)

type mediaImportDraft struct {
	Upload     client.AttachmentImport
	Hash, Path string
}
type agentMediaView struct {
	Parent                                                    ProjectIdentity
	Scope                                                     client.AgentSettingsScope
	Path, Epoch, Group                                        string
	Attachments                                               *client.AttachmentOverview
	Live                                                      *client.LiveOverview
	Session                                                   *client.LiveSession
	Advice                                                    *client.AttachmentAdviceReceipt
	Import                                                    *mediaImportDraft
	Private                                                   *client.ArtifactDetail
	Export                                                    *client.ArtifactReceipt
	Selected                                                  map[string]client.AttachmentReference
	Failures                                                  map[string]string
	Question, LiveDraft, Pending, Operation, Notice, Recovery string
	Cancel                                                    context.CancelFunc
}

type agentMediaAction struct {
	ID, Epoch, Operation, Target, Expected, Path string
	Parent                                       ProjectIdentity
	Scope                                        client.AgentSettingsScope
	Upload                                       client.AttachmentImport
	Advice                                       client.AttachmentAdviceRequest
	Open                                         client.LiveOpenRequest
	Control                                      client.LiveControl
	Artifact                                     client.ArtifactControl
}

func (s *State) mediaCurrent() bool {
	v := s.AgentMedia
	return v != nil && s.checkoutScopeCurrent(v.Parent, v.Scope, v.Path)
}

func (s *State) selectMediaScope(choice string) {
	if s.Project == nil || s.Project.ProjectID == "" {
		s.Error = "Open a project before using media."
		return
	}
	scope, path := client.AgentSettingsScope{ProjectIdentity: s.Project.ProjectIdentity}, s.Project.Path
	switch choice {
	case "main":
	case "task":
		if s.Change == nil || s.Change.WorktreePath == "" || s.Change.ProjectRevision == "" {
			s.Error = "Select an indexed task worktree first."
			return
		}
		scope, path = client.AgentSettingsScope{ProjectIdentity: s.Change.ProjectIdentity, WorktreePath: s.Change.WorktreePath}, s.Change.WorktreePath
	default:
		s.Error = "Use :media-scope main or :media-scope task."
		return
	}
	s.cancelMediaRequest()
	s.AgentMedia = &agentMediaView{Parent: s.Project.ProjectIdentity, Scope: scope, Path: path, Epoch: nextTUIReqID("media-view"), Group: "attachments", Selected: map[string]client.AttachmentReference{}, Failures: map[string]string{}}
	s.SelectedIdx = 0
	s.Navigate(PageMedia)
	s.queueMediaAction(agentMediaAction{Operation: "read"})
}

func (s *State) refreshMedia() {
	if !s.mediaCurrent() {
		s.selectMediaScope("main")
		return
	}
	s.queueMediaAction(agentMediaAction{Operation: "read"})
}

func (s *State) execMediaCommand(command, arg string) bool {
	switch command {
	case "media":
		s.Navigate(PageMedia)
		if !s.mediaCurrent() {
			s.selectMediaScope("main")
		}
	case "media-scope":
		s.selectMediaScope(arg)
	case "media-refresh":
		s.refreshMedia()
	case "media-cancel":
		s.cancelMediaRequest()
	case "media-view":
		s.selectMediaGroup(arg)
	case "media-select":
		s.toggleMediaSelection()
	case "media-inspect":
		s.inspectMedia()
	case "media-hide":
		s.hideMedia()
	case "media-export":
		s.confirmMediaExport()
	case "media-import":
		s.captureMediaImport(arg)
	case "media-ask":
		s.confirmMediaAdvice(arg)
	default:
		return s.execLiveCommand(command, arg)
	}
	return true
}

func (s *State) execLiveCommand(command, arg string) bool {
	switch command {
	case "live-open":
		s.confirmMediaOpen()
	case "live-send":
		s.prepareLiveControl("send", arg, false)
	case "live-send-media":
		s.prepareLiveControl("send", "", true)
	case "live-interrupt", "live-close", "live-reconnect":
		s.prepareLiveControl(strings.TrimPrefix(command, "live-"), "", false)
	default:
		return false
	}
	return true
}

func (s *State) selectMediaGroup(group string) {
	if !s.mediaCurrent() {
		s.Error = "Load media for the current checkout first."
		return
	}
	if !slices.Contains([]string{"attachments", "sessions", "outputs"}, group) {
		s.Error = "Use :media-view attachments, sessions or outputs."
		return
	}
	s.AgentMedia.Group, s.SelectedIdx, s.DetailScroll = group, 0, 0
	s.hideMedia()
}

func (s *State) hideMedia() {
	if s.AgentMedia != nil {
		s.AgentMedia.Private = nil
	}
	s.DetailScroll = 0
}

func (s *State) handleMediaPageKey(ev KeyEvent) bool {
	return s.Page == PageMedia && s.handleMediaKey(ev.Key)
}

func (s *State) handleMediaKey(key string) bool {
	switch key {
	case "r":
		s.refreshMedia()
	case "x":
		s.cancelMediaRequest()
	case "a":
		s.selectMediaGroup("attachments")
	case "l":
		s.selectMediaGroup("sessions")
	case "o":
		s.selectMediaGroup("outputs")
	case " ", "Space":
		s.toggleMediaSelection()
	case "n":
		s.openCommandInput("media-import ")
	case "q":
		s.openCommandInput("media-ask " + s.mediaDraft(false))
	case "s":
		s.openCommandInput("live-send " + s.mediaDraft(true))
	case "e":
		s.confirmMediaExport()
	case "Escape":
		s.hideMedia()
	default:
		return false
	}
	return true
}

func (s *State) mediaDraft(live bool) string {
	if s.AgentMedia == nil {
		return ""
	}
	if live {
		return s.AgentMedia.LiveDraft
	}
	return s.AgentMedia.Question
}

func (s *State) mediaReady() error {
	if !s.mediaCurrent() {
		return fmt.Errorf("load media for the current checkout first")
	}
	if s.AgentMedia.Pending != "" {
		return fmt.Errorf("finish or cancel the pending media request first")
	}
	return nil
}

func (s *State) selectedMediaRefs() ([]client.AttachmentReference, int) {
	if !s.mediaCurrent() || s.AgentMedia.Attachments == nil {
		return nil, 0
	}
	var refs []client.AttachmentReference
	total := 0
	for _, item := range s.AgentMedia.Attachments.Attachments {
		if ref, ok := s.AgentMedia.Selected[item.ID]; ok && ref.Hash == item.Hash && ref.Version == item.Version && item.RevisionStatus == "current" {
			refs, total = append(refs, ref), total+item.Size
		}
	}
	return refs, total
}

func (s *State) toggleMediaSelection() {
	if err := s.mediaReady(); err != nil {
		s.Error = err.Error()
		return
	}
	item := s.selectedMediaArtifact()
	if s.AgentMedia.Group != "attachments" || item == nil || item.RevisionStatus != "current" {
		s.Error = "Select a current imported attachment."
		return
	}
	if _, selected := s.AgentMedia.Selected[item.ID]; selected {
		delete(s.AgentMedia.Selected, item.ID)
		return
	}
	refs, total := s.selectedMediaRefs()
	if len(refs) >= 4 || total+item.Size > 1<<20 {
		s.Error = "Select at most four attachments totaling 1 MiB."
		return
	}
	s.AgentMedia.Selected[item.ID] = client.AttachmentReference{ID: item.ID, Version: item.Version, Hash: item.Hash}
}

func (s *State) captureMediaImport(path string) {
	if err := s.mediaReady(); err != nil {
		s.Error = err.Error()
		return
	}
	if strings.TrimSpace(path) == "" {
		s.Error = "Supply a file inside the selected checkout with :media-import <path>."
		return
	}
	s.AgentMedia.Import = nil
	s.queueMediaAction(agentMediaAction{Operation: "capture", Target: path})
}

func (s *State) confirmMediaAdvice(question string) {
	if err := s.mediaReady(); err != nil {
		s.Error = err.Error()
		return
	}
	v := s.AgentMedia
	v.Question = question
	if v.Attachments == nil || v.Attachments.State != "available" || v.Failures["attachments"] != "" {
		s.Error = "Refresh attachments and configure a supported native Function model for advice."
		return
	}
	refs, _ := s.selectedMediaRefs()
	if len(refs) == 0 || strings.TrimSpace(question) == "" || len(question) > 2048 {
		s.Error = "Select attachments with Space and supply a question of at most 2048 bytes."
		return
	}
	a := v.Attachments
	request := client.AttachmentAdviceRequest{ProjectIdentity: v.Scope.ProjectIdentity, Configuration: a.Configuration, SettingsVersion: a.SettingsVersion, Model: a.Model.Model, ProviderOrigin: a.Model.ProviderOrigin, Question: question, Attachments: refs, ConfirmSourceSharing: true, ConfirmRemoteProvider: true}
	message := s.mediaDestinationMessage(a.Model.Model, a.Model.ProviderOrigin, a.Configuration, a.SettingsVersion) + "\nQuestion: " + question + "\nShare only these imported versions:\n" + s.mediaSelectionMessage() + "\nOne advisory response. Source changes require the separate proposal workflow."
	s.confirmMediaAction(agentMediaAction{Operation: "advice", Advice: request}, "Share attachments for advice", message, "Send")
}

func (s *State) confirmMediaOpen() {
	if err := s.mediaReady(); err != nil {
		s.Error = err.Error()
		return
	}
	v := s.AgentMedia
	if v.Live == nil || v.Failures["sessions"] != "" {
		s.Error = "Refresh live availability first."
		return
	}
	if v.Live.State != "available" {
		s.Error = "Live is " + v.Live.State + ". Configure agent_runtime.live_model: gemini-3.8-live and an explicit Gemini API Function profile, then refresh."
		return
	}
	b := v.Live.LiveBinding
	message := s.mediaDestinationMessage(b.Model, b.ProviderOrigin, b.Configuration, b.SettingsVersion) + "\nOpen an advisory connection for at most five minutes and two connections.\nEach input requires separate sharing consent. Usage is charged conservatively when final usage is unavailable.\nNo capture or playback. Leaving this view keeps the session owned by the daemon; use :live-close explicitly."
	message += fmt.Sprintf("\nLimits: %d model calls, %d input and %d output tokens.\nTransport: 128 KiB input / 512 KiB output; eight send/interrupt controls per connection.", v.Live.Limits.ModelCalls, v.Live.Limits.InputTokens, v.Live.Limits.OutputTokens)
	s.confirmMediaAction(agentMediaAction{Operation: "open", Open: client.LiveOpenRequest{LiveBinding: b, ConfirmConnect: true, ConfirmRemoteProvider: true}}, "Connect live advice", message, "Connect")
}

func (s *State) prepareLiveControl(action, text string, media bool) {
	if err := s.mediaReady(); err != nil {
		s.Error = err.Error()
		return
	}
	item := s.selectedMediaSession()
	if item == nil || !item.Owned {
		s.Error = "Inspect an owned live session first. Restarted sessions require a new explicit connection."
		return
	}
	if action != "close" && item.Stale {
		s.Error = "This live binding is stale. Close it and refresh availability."
		return
	}
	request := client.LiveControl{LiveBinding: item.LiveBinding, ControlRevision: item.ControlRevision, Action: action, Text: text, ConfirmConnect: true, ConfirmSourceSharing: true, ConfirmRemoteProvider: true}
	if err := s.prepareLiveInput(&request, media); err != nil {
		s.Error = err.Error()
		return
	}
	op := agentMediaAction{Operation: "control", Target: item.ID, Control: request}
	if action == "close" || action == "interrupt" {
		s.queueMediaAction(op)
		return
	}
	message := s.mediaDestinationMessage(item.Model, item.ProviderOrigin, item.Configuration, item.SettingsVersion) + fmt.Sprintf("\nSession: %s\nControl revision: %d\n", item.ID, item.ControlRevision)
	if action == "reconnect" {
		message += "Start a fresh connection with the remaining lifetime and budget. Previous input will not be replayed."
	} else if media {
		message += "Share this imported media:\n" + s.mediaSelectionMessage()
	} else {
		message += "Share text: " + text
	}
	s.confirmMediaAction(op, "Confirm live "+action, message, action)
}

func (s *State) prepareLiveInput(request *client.LiveControl, media bool) error {
	item := s.selectedMediaSession()
	if request.Action == "reconnect" {
		if !item.CanReconnect {
			return fmt.Errorf("this session cannot reconnect; inspect its retained state and remaining budget")
		}
		return nil
	}
	if request.Action == "close" {
		return nil
	}
	if item.State != "connected" {
		return fmt.Errorf("live input requires a connected session")
	}
	if request.Action == "interrupt" {
		return nil
	}
	if !media {
		s.AgentMedia.LiveDraft = request.Text
		if strings.TrimSpace(request.Text) == "" || len(request.Text) > 2048 {
			return fmt.Errorf("supply 1–2048 UTF-8 bytes of live text")
		}
		return nil
	}
	refs, _ := s.selectedMediaRefs()
	if len(refs) != 1 {
		return fmt.Errorf("select exactly one imported JPEG or raw PCM version")
	}
	for _, file := range s.AgentMedia.Attachments.Attachments {
		if file.ID == refs[0].ID && (file.Size > 64<<10 || !slices.Contains([]string{"image/jpeg", "audio/pcm;rate=16000"}, file.MIME)) {
			return fmt.Errorf("live input accepts JPEG or raw 16 kHz PCM up to 64 KiB; no format conversion")
		}
	}
	request.Attachment = &refs[0]
	return nil
}

func (s *State) mediaDestinationMessage(model, origin, configuration string, version int64) string {
	v := s.AgentMedia
	return fmt.Sprintf("Checkout: %s\nProject: %s\nRevision: %s\nSettings: %s / %d\nModel: %s\nDestination: %s", v.Path, v.Scope.ProjectID, v.Scope.ProjectRevision, configuration, version, model, origin)
}

func (s *State) mediaSelectionMessage() string {
	var lines []string
	for _, item := range s.AgentMedia.Attachments.Attachments {
		if _, selected := s.AgentMedia.Selected[item.ID]; selected {
			lines = append(lines, fmt.Sprintf("%s · %s · %d bytes · v%d\nSHA-256: %s", client.FirstNonEmpty(item.DisplayName, item.Name), item.MIME, item.Size, item.Version, item.Hash))
		}
	}
	return strings.Join(lines, "\n")
}

func (s *State) confirmMediaAction(action agentMediaAction, title, message, accept string) {
	expected, err := s.mediaActionFingerprint(action)
	if err != nil {
		s.Error = err.Error()
		return
	}
	action.Expected = expected
	parent, scope, epoch := s.AgentMedia.Parent, s.AgentMedia.Scope, s.AgentMedia.Epoch
	s.Confirmation = &Confirmation{Title: title, Message: externalSafeText(message), AcceptLabel: accept, OnConfirm: func(current *State) {
		v := current.AgentMedia
		if !current.mediaCurrent() || v.Parent != parent || v.Scope != scope || v.Epoch != epoch {
			current.Error = "Checkout changed. Review the current selection again."
			return
		}
		current.queueMediaAction(action)
	}}
}
