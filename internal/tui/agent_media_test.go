package tui

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/theme"
)

func mediaTestState() *State {
	s := libraryTestState()
	scope := client.AgentSettingsScope{ProjectIdentity: s.Project.ProjectIdentity}
	item := s.AgentLibrary.Artifacts.Artifacts[0]
	item.Name, item.DisplayName, item.Kind, item.MIME, item.Size = "input.jpg", "input.jpg", "inline", "image/jpeg", 16
	binding := client.LiveBinding{ProjectIdentity: scope.ProjectIdentity, Configuration: strings.Repeat("b", 64), SettingsVersion: 2, Model: "gemini-3.8-live", ProviderOrigin: "https://generativelanguage.googleapis.com"}
	now := time.Now().UTC()
	session := client.LiveSession{LiveBinding: binding, ID: externalTestID, Version: 1, ControlRevision: 1, Policy: "3:" + strings.Repeat("c", 64), State: "connected", Connections: 1, Owned: true, CreatedAt: now, UpdatedAt: now, ExpiresAt: now.Add(5 * time.Minute)}
	v := &agentMediaView{Parent: s.Project.ProjectIdentity, Scope: scope, Path: s.Project.Path, Epoch: "media-fixture", Group: "attachments", Selected: map[string]client.AttachmentReference{}, Failures: map[string]string{}}
	v.Attachments = &client.AttachmentOverview{ProjectIdentity: scope.ProjectIdentity, Configuration: binding.Configuration, SettingsVersion: binding.SettingsVersion, State: "available", Attachments: []client.AgentArtifact{item}, Model: client.Model{Model: "gemini-2.5-flash", ProviderOrigin: binding.ProviderOrigin}}
	v.Attachments.Model.Capabilities.Attachments = true
	v.Live = &client.LiveOverview{LiveBinding: binding, State: "available", Destination: client.Model{Model: binding.Model, ProviderOrigin: binding.ProviderOrigin}, LifetimeSeconds: 300, MaxConnections: 2, MaxTurnsPerConnection: 8, MaxInputBytes: 128 << 10, MaxOutputBytes: 512 << 10, Sessions: []client.LiveSession{session}, Limits: client.AgentLimits{ModelCalls: 8, InputTokens: 500000, OutputTokens: 2000000}}
	v.Live.Destination.Capabilities.LiveSessions = true
	v.Session = &session
	s.AgentMedia = v
	s.Navigate(PageMedia)
	return s
}

func mediaPending(t *testing.T, s *State, cl *Client) (*BubbleModel, tea.Cmd) {
	t.Helper()
	m, cmd := libraryPending(t, t.Context(), s, cl)
	m.nativeStreams = false
	return m, cmd
}

func mediaApplyPending(t *testing.T, s *State, cl *Client) {
	t.Helper()
	m, cmd := mediaPending(t, s, cl)
	m.Update(cmd())
	if s.Error != "" || s.AgentMedia.Pending != "" {
		t.Fatalf("media action failed: %s / %s", s.Error, s.AgentMedia.Notice)
	}
}

func TestMediaImportAndAdviceRequireSeparateExactConsent(t *testing.T) {
	s := mediaTestState()
	s.Project.Path = t.TempDir()
	s.AgentMedia.Path = s.Project.Path
	data := []byte("captured JPEG fixture")
	path := filepath.Join(s.Project.Path, "design.jpg")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	var imports, advice, disclosures atomic.Int32
	answer := s.AgentLibrary.Artifacts.Artifacts[0]
	cl := externalTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/attachments"):
			imports.Add(1)
			var request client.AttachmentImport
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil || !request.ConfirmImport || string(request.Data) != string(data) {
				t.Error("import lost captured bytes or consent", err)
			}
			item := s.AgentMedia.Attachments.Attachments[0]
			hash := sha256.Sum256(data)
			item.Hash, item.Size, item.Name, item.DisplayName = hex.EncodeToString(hash[:]), len(data), request.Name, request.Name
			_ = json.NewEncoder(w).Encode(item)
		case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/advice"):
			advice.Add(1)
			var request client.AttachmentAdviceRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil || !request.ConfirmSourceSharing || !request.ConfirmRemoteProvider || len(request.Attachments) != 1 || request.Question != "Explain the design" {
				t.Error("advice lost exact sharing consent", err)
			}
			_ = json.NewEncoder(w).Encode(client.AttachmentAdviceReceipt{State: "advisory", Artifact: answer, Model: request.Model})
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/artifacts/"):
			disclosures.Add(1)
			_ = json.NewEncoder(w).Encode(client.ArtifactDetail{AgentArtifact: answer, PreviewKind: "text", Text: "PRIVATE_ARTIFACT_TEXT"})
		default:
			t.Error("unexpected media request", r.Method, r.URL)
			http.Error(w, "unexpected", http.StatusBadRequest)
		}
	})
	s.ExecuteCommand(":media-import " + path)
	mediaApplyPending(t, s, cl)
	if s.Confirmation == nil || imports.Load() != 0 || advice.Load() != 0 {
		t.Fatal("capture performed an import or provider request")
	}
	for _, field := range []string{path, "image/jpeg", s.AgentMedia.Import.Hash, "separate confirmation"} {
		if !strings.Contains(s.Confirmation.Message, field) {
			t.Fatalf("import review omitted %s", field)
		}
	}
	// The user reviews a captured version, even if the file subsequently changes.
	if err := os.WriteFile(path, []byte("new local bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	s.HandleKey(KeyEvent{Key: "Enter"})
	mediaApplyPending(t, s, cl)
	if imports.Load() != 1 || advice.Load() != 0 || len(s.AgentMedia.Attachments.Attachments) != 1 {
		t.Fatal("import duplicated a version or implicitly shared it")
	}
	s.HandleKey(KeyEvent{Key: "Space"})
	s.ExecuteCommand(":media-ask Explain the design")
	if s.Confirmation == nil || len(s.PendingActions) != 0 || !strings.Contains(s.Confirmation.Message, "Explain the design") || !strings.Contains(s.Confirmation.Message, s.AgentMedia.Attachments.Model.ProviderOrigin) {
		t.Fatal("provider sharing bypassed exact review")
	}
	s.HandleKey(KeyEvent{Key: "Enter"})
	mediaApplyPending(t, s, cl)
	if s.AgentMedia.Group != "outputs" || advice.Load() != 1 || disclosures.Load() != 0 || strings.Contains(s.Render(100, 30), "PRIVATE_ARTIFACT_TEXT") {
		t.Fatal("advice was not retained privately")
	}
	s.HandleKey(KeyEvent{Key: "Enter"})
	mediaApplyPending(t, s, cl)
	if disclosures.Load() != 1 || s.AgentMedia.Private == nil {
		t.Fatal("explicit inspection did not disclose the answer")
	}
	s.HandleKey(KeyEvent{Key: "Escape"})
	if s.AgentMedia.Private != nil || s.Page != PageMedia {
		t.Fatal("Escape did not hide private text in place")
	}
}

func TestMediaLocalCaptureRejectsUnsafeFilesAndCancellation(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	write := func(path string, size int) {
		t.Helper()
		if err := os.WriteFile(path, make([]byte, size), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(root, "ok.pcm"), 16)
	write(filepath.Join(root, "empty.jpg"), 0)
	write(filepath.Join(root, "large.pdf"), (512<<10)+1)
	write(filepath.Join(root, "secret.txt"), 10)
	write(filepath.Join(outside, "external.jpg"), 16)
	if err := os.Symlink(filepath.Join(outside, "external.jpg"), filepath.Join(root, "link.jpg")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
		t.Fatal(err)
	}
	id := ProjectIdentity{ProjectID: "project", ProjectRevision: "revision"}
	for _, input := range []string{".", "..", outside + "/external.jpg", "link.jpg", "linked/external.jpg", "empty.jpg", "large.pdf", "secret.txt", "missing.jpg"} {
		t.Run(input, func(t *testing.T) {
			if value, err := readMediaImport(t.Context(), id, root, input); err == nil || value != nil {
				t.Fatal("unsafe capture accepted", input)
			}
		})
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := readMediaImport(ctx, id, root, "ok.pcm"); !errors.Is(err, context.Canceled) {
		t.Fatal("canceled capture proceeded", err)
	}
	value, err := readMediaImport(t.Context(), id, root, "ok.pcm")
	if err != nil || value.Upload.MIME != "audio/pcm;rate=16000" || value.Upload.ConfirmImport || len(value.Upload.Data) != 16 {
		t.Fatal("bounded capture unavailable", err)
	}
}

func TestMediaConfirmationAndQueuedDispatchRejectChangedBindings(t *testing.T) {
	for _, change := range []string{"settings", "artifact", "question", "scope", "epoch", "selection"} {
		for _, queued := range []bool{false, true} {
			t.Run(change+map[bool]string{false: "/confirmation", true: "/queue"}[queued], func(t *testing.T) {
				s := mediaTestState()
				s.HandleKey(KeyEvent{Key: "Space"})
				s.ExecuteCommand(":media-ask Review")
				if s.Confirmation == nil {
					t.Fatal(s.Error)
				}
				if queued {
					s.HandleKey(KeyEvent{Key: "Enter"})
				}
				switch change {
				case "settings":
					s.AgentMedia.Attachments.SettingsVersion++
				case "artifact":
					s.AgentMedia.Attachments.Attachments[0].Hash = strings.Repeat("d", 64)
				case "question":
					s.AgentMedia.Question = "Different question"
				case "scope":
					s.Project.ProjectRevision = "replacement"
				case "epoch":
					s.AgentMedia.Epoch = "replacement"
				case "selection":
					s.AgentMedia.Selected = map[string]client.AttachmentReference{}
				}
				if queued {
					if s.prepareMediaAction(s.PendingActions[0].AgentMedia) {
						t.Fatal("stale queued request admitted")
					}
				} else {
					s.HandleKey(KeyEvent{Key: "Enter"})
					if len(s.PendingActions) != 0 {
						t.Fatal("stale confirmation dispatched")
					}
				}
			})
		}
	}
}

func TestMediaFailurePreservesDraftAndRequiresRefreshWithoutReplay(t *testing.T) {
	s := mediaTestState()
	s.selectMediaGroup("sessions")
	s.ExecuteCommand(":live-send retained draft")
	s.HandleKey(KeyEvent{Key: "Enter"})
	var writes atomic.Int32
	cl := externalTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			writes.Add(1)
			http.Error(w, "receipt lost", http.StatusBadGateway)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/attachments") {
			_ = json.NewEncoder(w).Encode(s.AgentMedia.Attachments)
		} else {
			_ = json.NewEncoder(w).Encode(s.AgentMedia.Live)
		}
	})
	m, cmd := mediaPending(t, s, cl)
	m.Update(cmd())
	if s.AgentMedia.LiveDraft != "retained draft" || s.AgentMedia.Recovery == "" || s.AgentMedia.Session.Turns != 0 {
		t.Fatal("failure lost draft or fabricated progress")
	}
	s.Error = ""
	s.ExecuteCommand(":live-send retained draft")
	s.HandleKey(KeyEvent{Key: "Enter"})
	if len(s.PendingActions) != 0 || writes.Load() != 1 {
		t.Fatal("unknown outcome was retried")
	}
	s.Error = ""
	s.ExecuteCommand(":media-refresh")
	mediaApplyPending(t, s, cl)
	if s.AgentMedia.Recovery != "" || s.AgentMedia.LiveDraft != "retained draft" || writes.Load() != 1 {
		t.Fatal("refresh replayed input or discarded draft")
	}
}

func TestMediaLiveControlsKeepConsentAndCloseAvailable(t *testing.T) {
	for _, action := range []string{"open", "send", "send-media", "interrupt", "close", "reconnect"} {
		t.Run(action, func(t *testing.T) {
			s := mediaTestState()
			s.HandleKey(KeyEvent{Key: "Space"})
			s.HandleKey(KeyEvent{Key: "l"})
			if action == "reconnect" {
				s.AgentMedia.Session.State, s.AgentMedia.Session.CanReconnect = "disconnected", true
			}
			if action == "close" {
				s.AgentMedia.Session.Stale, s.AgentMedia.Recovery = true, "unknown"
			}
			command := ":live-" + action
			if action == "send" {
				command += " exact live question"
			}
			s.ExecuteCommand(command)
			if action != "interrupt" && action != "close" {
				if s.Confirmation == nil || len(s.PendingActions) != 0 || !strings.Contains(s.Confirmation.Message, s.AgentMedia.Live.ProviderOrigin) {
					t.Fatal("missing exact live review", s.Error)
				}
				s.HandleKey(KeyEvent{Key: "Enter"})
			}
			if len(s.PendingActions) != 1 {
				t.Fatal("live control unavailable", s.Error)
			}
			request := s.PendingActions[0].AgentMedia
			if action == "open" {
				if !request.Open.ConfirmConnect || request.Open.LiveBinding != s.AgentMedia.Live.LiveBinding {
					t.Fatal("wrong open binding")
				}
			} else if request.Control.ControlRevision != 1 || request.Target != s.AgentMedia.Session.ID {
				t.Fatal("wrong control identity")
			}
			if action == "send-media" && (request.Control.Attachment == nil || request.Control.Attachment.Hash != s.AgentMedia.Attachments.Attachments[0].Hash) {
				t.Fatal("wrong media version")
			}
		})
	}
}

func TestMediaPassiveNavigationAndUnsupportedProfiles(t *testing.T) {
	s := mediaTestState()
	for _, key := range []string{"l", "o", "a", "PageDown", "PageUp", "?", "Escape"} {
		s.HandleKey(KeyEvent{Key: key})
	}
	if len(s.PendingActions) != 0 || s.Confirmation != nil {
		t.Fatal("navigation dispatched work")
	}
	s.AgentMedia.Live.State = "disabled"
	s.ExecuteCommand(":live-open")
	if !strings.Contains(s.Error, "agent_runtime.live_model") || s.Confirmation != nil || len(s.PendingActions) != 0 {
		t.Fatal("disabled live had inert or executable control")
	}
	s.Error = ""
	s.AgentMedia.Attachments.State = "unsupported_profile"
	s.ExecuteCommand(":media-ask Review")
	if !strings.Contains(s.Error, "native Code") || s.Confirmation != nil {
		t.Fatal("unsupported advice missing recovery")
	}
	s.Error = ""
	s.AgentMedia.Attachments = nil
	s.AgentMedia.Failures["attachments"] = "network unavailable"
	view := strings.Join(s.renderMedia(themeForPalette(s.Palette), 100), "\n")
	if strings.Contains(view, "No imported") || !strings.Contains(view, "Records unavailable") {
		t.Fatal("unavailable metadata became empty success")
	}
}

func TestMediaRefreshReconcilesExactSelectionAndInspectedHistory(t *testing.T) {
	s := mediaTestState()
	s.toggleMediaSelection()
	selected := *s.AgentMedia.Session
	selected.OutputArtifacts = []client.AgentArtifact{s.AgentLibrary.Artifacts.Artifacts[0]}
	s.AgentMedia.Session = &selected
	live := *s.AgentMedia.Live
	attachments := *s.AgentMedia.Attachments
	s.applyMediaOverview(agentMediaResult{Live: &live, Attachments: &attachments})
	if len(s.AgentMedia.Session.OutputArtifacts) != 1 {
		t.Fatal("same-version refresh lost inspected output metadata")
	}
	live.Sessions = nil
	attachments.Attachments = nil
	s.applyMediaOverview(agentMediaResult{Live: &live, Attachments: &attachments})
	if s.AgentMedia.Session != nil || len(s.AgentMedia.Selected) != 0 {
		t.Fatal("removed session or attachment remained actionable")
	}
}

func TestMediaTaskScopeAndPartialRefreshStayIsolated(t *testing.T) {
	s := mediaTestState()
	s.Change = &ChangeSession{ProjectIdentity: ProjectIdentity{ProjectID: "task-project", ProjectRevision: "task-revision"}, WorktreePath: "/repo/.mini-orca/worktrees/task"}
	attachments, live := *s.AgentMedia.Attachments, *s.AgentMedia.Live
	attachments.ProjectIdentity, attachments.Attachments = s.Change.ProjectIdentity, nil
	live.ProjectIdentity, live.Sessions = s.Change.ProjectIdentity, nil
	s.ExecuteCommand(":media-scope task")
	var reads atomic.Int32
	cl := externalTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		reads.Add(1)
		if r.Method != "GET" || r.URL.Query().Get("worktree_path") != s.Change.WorktreePath || r.URL.Query().Get("project_id") != s.Change.ProjectID {
			t.Error("task scope leaked", r.Method, r.URL)
		}
		if strings.HasSuffix(r.URL.Path, "/attachments") {
			_ = json.NewEncoder(w).Encode(attachments)
			return
		}
		http.Error(w, "live metadata unavailable", http.StatusServiceUnavailable)
	})
	mediaApplyPending(t, s, cl)
	if reads.Load() != 2 || s.AgentMedia.Attachments == nil || s.AgentMedia.Live != nil || s.AgentMedia.Failures["sessions"] == "" {
		t.Fatal("partial read fabricated live state or erased healthy group")
	}
	s.ExecuteCommand(":media-refresh")
	m, cmd := mediaPending(t, s, cl)
	result := cmd()
	s.Project.ProjectRevision = "replacement"
	m.Update(result)
	if s.mediaCurrent() || s.AgentMedia.Live != nil || s.AgentMedia.Private != nil {
		t.Fatal("late task result crossed project replacement")
	}
}

func TestMediaAudioExportUsesExactPrivateVersionWithoutPlayback(t *testing.T) {
	s := mediaTestState()
	item := s.AgentMedia.Attachments.Attachments[0]
	item.MIME, item.Name, item.DisplayName = "audio/pcm;rate=24000", "reply.pcm", "reply.pcm"
	s.AgentMedia.Session.OutputArtifacts = []client.AgentArtifact{item}
	s.selectMediaGroup("outputs")
	var reads, writes atomic.Int32
	cl := externalTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			reads.Add(1)
			_ = json.NewEncoder(w).Encode(client.ArtifactDetail{AgentArtifact: item, PreviewKind: "binary"})
			return
		}
		writes.Add(1)
		var request client.ArtifactControl
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request.Action != "export" || request.Version != item.Version || request.Hash != item.Hash {
			t.Error("export lost exact media version", err)
		}
		_ = json.NewEncoder(w).Encode(client.ArtifactReceipt{ProjectIdentity: item.ProjectIdentity, ID: item.ID, Version: item.Version, Hash: item.Hash, State: "exported", Path: "/repo/.mini-orca/exports/artifacts/artifact-reply.pcm"})
	})
	if len(s.PendingActions) != 0 {
		t.Fatal("audio navigation read or played data")
	}
	s.HandleKey(KeyEvent{Key: "Enter"})
	mediaApplyPending(t, s, cl)
	fields := s.mediaArtifactDisclosure(&item)
	if reads.Load() != 1 || writes.Load() != 0 || !strings.Contains(fields[0][1], "No automatic playback") {
		t.Fatal("audio inspection exposed payload or playback")
	}
	s.HandleKey(KeyEvent{Key: "e"})
	if s.Confirmation == nil || !strings.Contains(s.Confirmation.Message, item.Hash) || writes.Load() != 0 {
		t.Fatal("audio export bypassed confirmation")
	}
	s.HandleKey(KeyEvent{Key: "Enter"})
	mediaApplyPending(t, s, cl)
	if writes.Load() != 1 || s.AgentMedia.Export == nil || !strings.Contains(s.AgentMedia.Notice, "reply.pcm") {
		t.Fatal("export hid actual private copy")
	}
}

func TestMediaLiveInputRejectsUnsupportedMediaAndStaleSessions(t *testing.T) {
	for _, invalid := range []string{"empty-selection", "format", "size", "stale", "disconnected"} {
		t.Run(invalid, func(t *testing.T) {
			s := mediaTestState()
			s.toggleMediaSelection()
			switch invalid {
			case "empty-selection":
				s.AgentMedia.Selected = map[string]client.AttachmentReference{}
			case "format":
				s.AgentMedia.Attachments.Attachments[0].MIME = "audio/wav"
			case "size":
				s.AgentMedia.Attachments.Attachments[0].Size = (64 << 10) + 1
			case "stale":
				s.AgentMedia.Session.Stale = true
			case "disconnected":
				s.AgentMedia.Session.State = "disconnected"
			}
			s.ExecuteCommand(":live-send-media")
			if s.Error == "" || s.Confirmation != nil || len(s.PendingActions) != 0 {
				t.Fatal("unsafe live media input admitted")
			}
		})
	}
}

func TestMediaViewsStayBoundedPrivateAndAccessibleAcrossPalettes(t *testing.T) {
	for _, palette := range []Palette{PaletteTokyo, PaletteCRT, PaletteAmber, PalettePaper} {
		for _, size := range [][2]int{{52, 16}, {80, 24}, {120, 40}} {
			for _, group := range []string{"attachments", "sessions", "outputs"} {
				s := mediaTestState()
				s.Palette = palette
				s.AgentMedia.Advice = &client.AttachmentAdviceReceipt{Artifact: s.AgentLibrary.Artifacts.Artifacts[0]}
				s.selectMediaGroup(group)
				if group == "outputs" {
					s.AgentMedia.Private = &client.ArtifactDetail{AgentArtifact: s.AgentMedia.Advice.Artifact, PreviewKind: "text", Text: "unsafe\x1b[2J\n" + strings.Repeat("retained advice ", 50) + "end-of-media"}
				}
				var visited strings.Builder
				for range 35 {
					frame := s.Render(size[0], size[1])
					visited.WriteString(frame)
					for _, line := range strings.Split(frame, "\n") {
						if theme.VisibleRuneCount(line) != size[0] {
							t.Fatalf("media overflow %s %v: %q", group, size, line)
						}
					}
					s.HandleKey(KeyEvent{Key: "PageDown"})
				}
				if !strings.Contains(visited.String(), "SELECTED MEDIA") || strings.Contains(visited.String(), "\x1b[2J") || len(s.PendingActions) != 0 {
					t.Fatal("media view inaccessible, unsafe or active", group, size)
				}
				if group == "outputs" && !strings.Contains(visited.String(), "end-of-media") {
					t.Fatal("private text cannot be reached", size)
				}
				if group == "sessions" && !strings.Contains(visited.String(), ":live-close") {
					t.Fatal("session close undiscoverable", size)
				}
			}
		}
	}
}
