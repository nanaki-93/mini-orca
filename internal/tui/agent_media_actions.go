package tui

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
)

type agentMediaResult struct {
	Action      agentMediaAction
	Attachments *client.AttachmentOverview
	Live        *client.LiveOverview
	Session     *client.LiveSession
	Import      *mediaImportDraft
	Artifact    *client.AgentArtifact
	Advice      *client.AttachmentAdviceReceipt
	Private     *client.ArtifactDetail
	Export      *client.ArtifactReceipt
	Failures    map[string]error
	Err         error
}

func (s *State) mediaActionFingerprint(action agentMediaAction) (string, error) {
	if !s.mediaCurrent() {
		return "", fmt.Errorf("media checkout changed")
	}
	payload, err := s.mediaActionPayload(action)
	if err != nil {
		return "", err
	}
	v := s.AgentMedia
	return externalFingerprint(struct {
		Epoch, Group, Row string
		Selected          map[string]client.AttachmentReference
		Payload           any
	}{v.Epoch, v.Group, s.selectedMediaRow().ID, v.Selected, payload})
}

func (s *State) mediaActionPayload(action agentMediaAction) (any, error) {
	v := s.AgentMedia
	switch action.Operation {
	case "import":
		if v.Import == nil {
			return nil, fmt.Errorf("capture the file before importing")
		}
		return v.Import.Upload, nil
	case "advice":
		if v.Attachments == nil || v.Attachments.State != "available" || v.Failures["attachments"] != "" {
			return nil, fmt.Errorf("refresh attachment availability")
		}
		a := v.Attachments
		refs, _ := s.selectedMediaRefs()
		return struct {
			Configuration, Model, Origin, Question string
			Version                                int64
			Refs                                   []client.AttachmentReference
		}{a.Configuration, a.Model.Model, a.Model.ProviderOrigin, v.Question, a.SettingsVersion, refs}, nil
	case "open":
		if v.Live == nil || v.Live.State != "available" || v.Failures["sessions"] != "" {
			return nil, fmt.Errorf("refresh live availability")
		}
		return v.Live.LiveBinding, nil
	case "control":
		return s.mediaControlPayload(action)
	case "inspect", "export":
		item := s.selectedMediaArtifact()
		if item == nil {
			return nil, fmt.Errorf("select an artifact")
		}
		return client.AttachmentReference{ID: item.ID, Version: item.Version, Hash: item.Hash}, nil
	default:
		return action.Target, nil
	}
}

func (s *State) mediaControlPayload(action agentMediaAction) (any, error) {
	item := s.selectedMediaSession()
	if item == nil || item.ID != action.Target {
		return nil, fmt.Errorf("live selection changed")
	}
	if action.Control.Action != "close" && s.AgentMedia.Failures["sessions"] != "" {
		return nil, fmt.Errorf("refresh live state before sending another control")
	}
	return struct {
		Binding                 client.LiveBinding
		Revision                int64
		State, Draft            string
		Owned, Stale, Reconnect bool
	}{item.LiveBinding, item.ControlRevision, item.State, s.AgentMedia.LiveDraft, item.Owned, item.Stale, item.CanReconnect}, nil
}

func (s *State) queueMediaAction(action agentMediaAction) {
	if err := s.mediaReady(); err != nil {
		s.Error = err.Error()
		return
	}
	v := s.AgentMedia
	if v.Recovery != "" && mediaMayHaveOutcome(action) {
		s.Error = "Refresh local media history before repeating an operation with an uncertain outcome."
		return
	}
	if action.Operation != "read" {
		expected, err := s.mediaActionFingerprint(action)
		if err != nil {
			s.Error = err.Error()
			return
		}
		if action.Expected != "" && action.Expected != expected {
			s.Error = "Media selection or settings changed. Review them again."
			return
		}
		action.Expected = expected
	}
	action.ID, action.Epoch, action.Parent, action.Scope, action.Path = nextTUIReqID("media"), v.Epoch, v.Parent, v.Scope, v.Path
	action.Upload.Data = bytes.Clone(action.Upload.Data)
	action.Advice.Attachments = slices.Clone(action.Advice.Attachments)
	v.Pending, v.Operation = action.ID, action.Operation
	s.enqueueAction(ActionRequest{Kind: "agent-media", AgentMedia: &action})
}

func (s *State) acceptsMediaAction(action *agentMediaAction) bool {
	v := s.AgentMedia
	return action != nil && s.mediaCurrent() && v.Epoch == action.Epoch && v.Pending == action.ID && v.Scope == action.Scope && v.Parent == action.Parent
}

func (s *State) prepareMediaAction(action *agentMediaAction) bool {
	if !s.acceptsMediaAction(action) {
		return false
	}
	if action.Expected != "" {
		current, err := s.mediaActionFingerprint(*action)
		if err != nil || current != action.Expected {
			s.AgentMedia.Pending, s.Error = "", "Media selection or settings changed before dispatch. Review them again."
			return false
		}
	}
	return true
}

func (m *BubbleModel) prepareMediaCmd(action *agentMediaAction) tea.Cmd {
	if !m.state.prepareMediaAction(action) {
		return nil
	}
	captured := *action
	ctx, cancel := context.WithCancel(m.ctx)
	m.state.AgentMedia.Cancel = cancel
	return func() tea.Msg {
		defer cancel()
		return agentMediaResultMsg{executeMediaAction(ctx, m.client, captured)}
	}
}

func (s *State) cancelMediaRequest() {
	v := s.AgentMedia
	if v == nil || v.Pending == "" {
		return
	}
	if v.Cancel != nil {
		v.Cancel()
		v.Cancel = nil
	}
	s.enqueueAction(ActionRequest{Kind: "cancel-agent-media", Target: v.Pending})
	if mediaMayHaveOutcome(agentMediaAction{Operation: v.Operation}) {
		v.Recovery = "Canceled request may have a retained outcome."
	}
	v.Pending, v.Operation = "", ""
	v.Notice = "Local request canceled. Refresh to inspect any retained outcome; nothing will be resent automatically."
}

func executeMediaAction(ctx context.Context, cl *Client, action agentMediaAction) agentMediaResult {
	r := agentMediaResult{Action: action, Failures: map[string]error{}}
	if action.Operation == "capture" {
		r.Import, r.Err = readMediaImport(ctx, action.Scope.ProjectIdentity, action.Path, action.Target)
		return r
	}
	if cl == nil {
		r.Err = fmt.Errorf("daemon connection unavailable")
		return r
	}
	switch action.Operation {
	case "read":
		r.Attachments, r.Failures["attachments"] = cl.ReadAttachments(ctx, action.ID+"-attachments", action.Scope)
		r.Live, r.Failures["sessions"] = cl.ReadLiveSessions(ctx, action.ID+"-sessions", action.Scope)
	case "import":
		r.Artifact, r.Err = cl.ImportAttachment(ctx, action.ID, action.Scope, action.Upload)
	case "advice":
		r.Advice, r.Err = cl.AttachmentAdvice(ctx, action.ID, action.Scope, action.Advice)
	case "open":
		r.Session, r.Err = cl.OpenLiveSession(ctx, action.ID, action.Scope, action.Open)
	case "control":
		r.Session, r.Err = cl.ControlLiveSession(ctx, action.ID, action.Scope, action.Target, action.Control)
	case "session":
		r.Session, r.Err = cl.LiveSessionDetail(ctx, action.ID, action.Scope, action.Target)
	case "inspect":
		r.Private, r.Err = cl.ArtifactDetail(ctx, action.ID, action.Scope, action.Target)
	case "export":
		r.Export, r.Err = cl.ControlArtifact(ctx, action.ID, action.Scope, action.Target, action.Artifact)
	default:
		r.Err = fmt.Errorf("unsupported media action")
	}
	return r
}

func (s *State) applyMediaResult(result agentMediaResult) {
	if !s.acceptsMediaAction(&result.Action) {
		return
	}
	v := s.AgentMedia
	v.Pending, v.Operation, v.Cancel = "", "", nil
	if result.Action.Expected != "" {
		current, err := s.mediaActionFingerprint(result.Action)
		if err != nil || current != result.Action.Expected {
			v.Notice = "Selection changed. Refresh retained history before another action."
			if mediaMayHaveOutcome(result.Action) {
				v.Recovery = "Selection changed while a request was pending."
			}
			return
		}
	}
	if result.Err != nil {
		s.Error = externalSafeText(result.Err.Error())
		if mediaMayHaveOutcome(result.Action) {
			v.Recovery = "The request may have a retained outcome."
		}
		v.Notice = "Request failed. Prior progress and drafts are retained; refresh to inspect the outcome before another attempt."
		return
	}
	s.applyMediaSuccess(result)
}

func mediaMayHaveOutcome(action agentMediaAction) bool {
	return slices.Contains([]string{"import", "advice", "open", "control"}, action.Operation) && action.Control.Action != "close"
}

func (s *State) applyMediaSuccess(result agentMediaResult) {
	v := s.AgentMedia
	switch result.Action.Operation {
	case "read":
		s.applyMediaOverview(result)
	case "capture":
		v.Import = result.Import
		if v.Import == nil {
			s.Error = "Captured attachment unavailable."
			return
		}
		draft := v.Import
		request := draft.Upload
		request.ConfirmImport = true
		message := fmt.Sprintf("Import captured bytes into the private local library.\nCheckout: %s\nRevision: %s\nFile: %s\nName: %s\nMIME: %s\nSize: %d bytes\nSHA-256: %s\nProvider sharing requires a separate confirmation.", v.Path, v.Scope.ProjectRevision, draft.Path, request.Name, request.MIME, len(request.Data), draft.Hash)
		s.confirmMediaAction(agentMediaAction{Operation: "import", Upload: request}, "Import attachment", message, "Import")
	case "import":
		if result.Artifact == nil {
			s.Error = "Import receipt unavailable."
			return
		}
		s.mergeImportedMedia(*result.Artifact)
		v.Import, v.Group, s.SelectedIdx = nil, "attachments", 0
		v.Notice = "Imported locally. Use Space to select a version for a separately confirmed provider request."
	case "advice":
		v.Advice, v.Group, v.Private, s.SelectedIdx = result.Advice, "outputs", nil, 0
		v.Notice = "Advisory answer saved privately. Enter discloses it; older answers remain in Code / Artifacts."
	case "open", "control", "session":
		s.applyMediaSession(result)
	case "inspect":
		v.Private = result.Private
		s.DetailScroll = 0
	case "export":
		v.Export = result.Export
		if result.Export != nil {
			v.Notice = "Private copy: " + externalSafeText(result.Export.Path)
		}
	}
}

func (s *State) mergeImportedMedia(item client.AgentArtifact) {
	v := s.AgentMedia
	if v.Attachments == nil {
		v.Attachments = &client.AttachmentOverview{ProjectIdentity: v.Scope.ProjectIdentity, State: "unread"}
	}
	items := v.Attachments.Attachments
	items = slices.DeleteFunc(items, func(old client.AgentArtifact) bool { return old.ID == item.ID })
	v.Attachments.Attachments = append([]client.AgentArtifact{item}, items...)
	if len(v.Attachments.Attachments) > 128 {
		v.Attachments.Attachments = v.Attachments.Attachments[:128]
		v.Attachments.Truncated = true
	}
}

func (s *State) applyMediaOverview(result agentMediaResult) {
	v, selected := s.AgentMedia, s.selectedMediaRow().ID
	for _, group := range []string{"attachments", "sessions"} {
		v.Failures[group] = ""
		if err := result.Failures[group]; err != nil {
			v.Failures[group] = externalSafeText(err.Error())
		}
	}
	if result.Attachments != nil {
		v.Attachments = result.Attachments
		s.reconcileMediaSelection()
	}
	if result.Live != nil {
		v.Live = result.Live
		s.reconcileMediaSession()
	}
	if result.Failures["attachments"] == nil && result.Failures["sessions"] == nil {
		v.Recovery = ""
	}
	s.restoreMediaSelection(selected)
	v.Notice = "Local metadata refreshed. Private text requires explicit inspection."
}

func (s *State) reconcileMediaSession() {
	v := s.AgentMedia
	if v.Session == nil {
		return
	}
	previous := v.Session
	v.Session = nil
	for _, item := range v.Live.Sessions {
		if item.ID == previous.ID {
			// A list omits private output metadata. Keep inspected metadata only
			// while its exact receipt version remains current.
			if item.Version == previous.Version && item.LiveBinding == previous.LiveBinding {
				item.OutputArtifacts = previous.OutputArtifacts
				item.MissingArtifacts = previous.MissingArtifacts
			}
			v.Session = &item
			return
		}
	}
}

func (s *State) reconcileMediaSelection() {
	v := s.AgentMedia
	available := map[string]client.AttachmentReference{}
	for _, item := range v.Attachments.Attachments {
		if item.RevisionStatus == "current" {
			available[item.ID] = client.AttachmentReference{ID: item.ID, Version: item.Version, Hash: item.Hash}
		}
	}
	for id, selected := range v.Selected {
		if available[id] != selected {
			delete(v.Selected, id)
		}
	}
}

func (s *State) applyMediaSession(result agentMediaResult) {
	v := s.AgentMedia
	if result.Session == nil {
		s.Error = "Live session receipt unavailable. Refresh history."
		return
	}
	v.Session, v.Private = result.Session, nil
	s.mergeMediaSession(*result.Session)
	if result.Action.Operation == "open" {
		v.Group = "sessions"
		s.restoreMediaSelection(result.Session.ID)
	}
	if result.Action.Control.Action == "send" {
		v.LiveDraft = ""
	}
	v.Notice = "Live state: " + result.Session.State + ". Transcript/audio artifacts are retained when the connection ends."
}

func (s *State) mergeMediaSession(item client.LiveSession) {
	v := s.AgentMedia
	if v.Live == nil {
		return
	}
	for i := range v.Live.Sessions {
		if v.Live.Sessions[i].ID == item.ID {
			v.Live.Sessions[i] = item
			return
		}
	}
	v.Live.Sessions = append([]client.LiveSession{item}, v.Live.Sessions...)
}

func (s *State) dispatchMediaAction(ctx context.Context, cl *Client, action ActionRequest) bool {
	switch action.Kind {
	case "agent-media":
		if s.prepareMediaAction(action.AgentMedia) {
			s.applyMediaResult(executeMediaAction(ctx, cl, *action.AgentMedia))
		}
	case "cancel-agent-media":
		if cl != nil {
			for _, suffix := range []string{"", "-attachments", "-sessions"} {
				cl.Cancel(action.Target + suffix)
			}
		}
	default:
		return false
	}
	return true
}

func readMediaImport(ctx context.Context, identity client.ProjectIdentity, base, input string) (*mediaImportDraft, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	base, err := filepath.Abs(base)
	if err != nil {
		return nil, fmt.Errorf("locate selected checkout: %w", err)
	}
	canonical, err := filepath.EvalSymlinks(base)
	if err != nil {
		return nil, fmt.Errorf("locate media checkout: %w", err)
	}
	path := input
	if !filepath.IsAbs(path) {
		path = filepath.Join(base, path)
	}
	rel, err := filepath.Rel(base, filepath.Clean(path))
	if err != nil || !filepath.IsLocal(rel) || rel == "." {
		return nil, fmt.Errorf("select a file inside the current checkout")
	}
	root, err := os.OpenRoot(canonical)
	if err != nil {
		return nil, fmt.Errorf("open media checkout: %w", err)
	}
	defer root.Close()
	data, err := readMediaBytes(ctx, root, rel)
	if err != nil {
		return nil, err
	}
	return capturedMedia(identity, filepath.Clean(path), data)
}

func readMediaBytes(ctx context.Context, root *os.Root, rel string) ([]byte, error) {
	info, err := mediaFileInfo(root, rel)
	if err != nil {
		return nil, err
	}
	file, err := root.Open(rel)
	if err != nil {
		return nil, fmt.Errorf("open selected media: %w", err)
	}
	defer file.Close()
	stop := context.AfterFunc(ctx, func() { _ = file.Close() })
	defer stop()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return nil, fmt.Errorf("selected media changed before reading")
	}
	data, err := io.ReadAll(io.LimitReader(file, (512<<10)+1))
	if ctx.Err() != nil {
		return nil, context.Cause(ctx)
	}
	if err != nil {
		return nil, fmt.Errorf("read selected media: %w", err)
	}
	after, err := mediaFileInfo(root, rel)
	if err != nil || !os.SameFile(info, after) || !info.ModTime().Equal(after.ModTime()) || info.Size() != int64(len(data)) {
		return nil, fmt.Errorf("selected media changed while reading")
	}
	return data, nil
}

func mediaFileInfo(root *os.Root, relative string) (os.FileInfo, error) {
	parts := strings.Split(filepath.ToSlash(relative), "/")
	var info os.FileInfo
	for i := range parts {
		var err error
		info, err = root.Lstat(filepath.Join(parts[:i+1]...))
		if err != nil {
			return nil, fmt.Errorf("inspect selected media: %w", err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("media paths must not contain symlinks")
		}
	}
	if !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > 512<<10 {
		return nil, fmt.Errorf("select a regular media file containing 1–524288 bytes")
	}
	return info, nil
}

func capturedMedia(identity client.ProjectIdentity, path string, data []byte) (*mediaImportDraft, error) {
	mime := map[string]string{".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".pdf": "application/pdf", ".wav": "audio/wav", ".pcm": "audio/pcm;rate=16000"}[strings.ToLower(filepath.Ext(path))]
	if mime == "" {
		return nil, fmt.Errorf("select PNG, JPEG, PDF, 16 kHz mono PCM WAV or raw PCM media")
	}
	hash := sha256.Sum256(data)
	return &mediaImportDraft{Upload: client.AttachmentImport{ProjectIdentity: identity, Name: filepath.Base(path), MIME: mime, Data: data}, Hash: hex.EncodeToString(hash[:]), Path: path}, nil
}
