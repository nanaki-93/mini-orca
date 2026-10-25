package tui

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/api/handlers"
	"github.com/nanaki-93/mini-orca/v2/internal/app"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/theme"
)

func triggerUIFixture(t *testing.T) (*State, *Client, *app.Service, *atomic.Int32, *atomic.Bool) {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc main() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	manager, err := project.NewManager(root)
	if err != nil {
		t.Fatal(err)
	}
	if err = manager.Set(root, &project.Analysis{Name: "ambient fixture", Path: root}); err != nil {
		t.Fatal(err)
	}
	calls := &atomic.Int32{}
	provider := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls.Add(1) }))
	t.Cleanup(provider.Close)
	cfg := config.Default()
	cfg.ModelScopes.Bug.APIBaseURL = provider.URL + "/v1"
	service, err := app.New(cfg, manager)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := service.ShutdownChangeWorkflows(ctx); err != nil {
			t.Error(err)
		}
	})
	h := handlers.NewAgentTriggersHandler(service)
	lost := &atomic.Bool{}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/projects/current/index", func(w http.ResponseWriter, r *http.Request) {
		index, err := manager.Index()
		if err != nil {
			http.Error(w, "index unavailable", 500)
			return
		}
		_ = json.NewEncoder(w).Encode(index)
	})
	mux.HandleFunc("GET /api/projects/current/agent-triggers", h.Get)
	mux.HandleFunc("POST /api/projects/current/agent-triggers/control", func(w http.ResponseWriter, r *http.Request) {
		receipt := httptest.NewRecorder()
		h.Control(receipt, r)
		if lost.Swap(false) {
			http.Error(w, "receipt unavailable", http.StatusBadGateway)
			return
		}
		for key, values := range receipt.Header() {
			w.Header()[key] = values
		}
		w.WriteHeader(receipt.Code)
		_, _ = w.Write(receipt.Body.Bytes())
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	cl, err := NewClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cl.Close() })
	index, err := manager.Index()
	if err != nil {
		t.Fatal(err)
	}
	s := NewState()
	s.Project = &Project{ProjectIdentity: ProjectIdentity{ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision}, Name: "ambient fixture", Path: manager.Root()}
	s.ExecuteCommand(":ambient")
	applyTriggerPending(t, s, cl)
	return s, cl, service, calls, lost
}

func applyTriggerPending(t *testing.T, s *State, cl *Client) {
	t.Helper()
	m, cmd := libraryPending(t, t.Context(), s, cl)
	m.Update(cmd())
	if s.AgentTriggers.Pending != "" || s.AgentTriggers.Failure != "" {
		t.Fatalf("ambient action: %s %s", s.Error, s.AgentTriggers.Failure)
	}
}

func TestAmbientUIRealHandlersRequireSeparateWatchAndGrant(t *testing.T) {
	s, cl, service, calls, _ := triggerUIFixture(t)
	if s.AgentTriggers.Data.State != "disabled" || calls.Load() != 0 {
		t.Fatal("passive entry started work")
	}
	s.HandleKey(KeyEvent{Key: "w"})
	if !s.CmdActive || !strings.HasPrefix(s.CmdInput, "ambient-watch changes") || len(s.PendingActions) != 0 {
		t.Fatal("watch editor was not passive")
	}
	s.HandleKey(KeyEvent{Key: "Escape"})
	s.ExecuteCommand(":ambient-watch changes main.go")
	if s.Confirmation == nil || len(s.PendingActions) != 0 || !strings.Contains(s.Confirmation.Message, "separate grant") {
		t.Fatal("watch skipped concrete confirmation")
	}
	s.HandleKey(KeyEvent{Key: "Enter"})
	applyTriggerPending(t, s, cl)
	if s.AgentTriggers.Data.State != "watch_only" || s.AgentTriggers.Data.Record.Grant != nil {
		t.Fatal("watch silently granted provider work")
	}
	s.HandleKey(KeyEvent{Key: "g"})
	for _, text := range []string{"main.go", "filtered project context", s.AgentTriggers.Data.Model, s.AgentTriggers.Data.ProviderOrigin, "input tokens", "Window", "Revision"} {
		if s.Confirmation == nil || !strings.Contains(s.Confirmation.Message, text) {
			t.Fatal("grant review omitted", text)
		}
	}
	s.HandleKey(KeyEvent{Key: "Enter"})
	applyTriggerPending(t, s, cl)
	if s.AgentTriggers.Data.State != "granted" || s.AgentTriggers.Data.Record.Grant == nil || calls.Load() != 0 {
		t.Fatal("grant receipt missing or past work replayed")
	}
	for _, key := range []string{"v", "d"} {
		s.HandleKey(KeyEvent{Key: key})
		if s.Confirmation == nil {
			t.Fatal("missing stop confirmation")
		}
		s.HandleKey(KeyEvent{Key: "Enter"})
		applyTriggerPending(t, s, cl)
	}
	if s.AgentTriggers.Data.Owned || s.AgentTriggers.Data.State != "disabled" || calls.Load() != 0 {
		t.Fatal("disable retained owner or dispatched provider")
	}
	draft := s.AgentTriggers.Draft
	if err := os.WriteFile(filepath.Join(s.Project.Path, "main.go"), []byte("package main\nfunc Changed() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	index, err := service.Reindex()
	if err != nil {
		t.Fatal(err)
	}
	s.HandleKey(KeyEvent{Key: "r"})
	applyTriggerPending(t, s, cl)
	if s.Project.ProjectRevision != index.ProjectRevision || s.AgentTriggers.Data.ProjectRevision != index.ProjectRevision || s.AgentTriggers.Draft != draft {
		t.Fatal("refresh lost draft or failed to adopt same-project revision")
	}
}

func TestAmbientUILostReceiptPreservesDraftAndRequiresRefresh(t *testing.T) {
	s, cl, _, calls, lost := triggerUIFixture(t)
	lost.Store(true)
	s.ExecuteCommand(":ambient-watch changes main.go")
	s.HandleKey(KeyEvent{Key: "Enter"})
	m, cmd := libraryPending(t, t.Context(), s, cl)
	m.Update(cmd())
	if s.AgentTriggers.Recovery == "" || s.AgentTriggers.Data.State != "disabled" || s.AgentTriggers.Draft != "changes main.go" {
		t.Fatal("unknown outcome discarded healthy metadata or draft")
	}
	s.Error = ""
	s.ExecuteCommand(":ambient-grant")
	if len(s.PendingActions) != 0 || s.Confirmation != nil {
		t.Fatal("unknown watch outcome permitted another control")
	}
	s.Error = ""
	s.ExecuteCommand(":ambient-refresh")
	applyTriggerPending(t, s, cl)
	if s.AgentTriggers.Recovery != "" || s.AgentTriggers.Data.State != "watch_only" || calls.Load() != 0 {
		t.Fatal("refresh failed to reconcile actual retained watch")
	}
}

func TestAmbientUIRejectsChangedConfirmationAndQueuedDispatch(t *testing.T) {
	for _, change := range []string{"draft", "control", "configuration", "window", "project"} {
		for _, queued := range []bool{false, true} {
			t.Run(change+map[bool]string{false: " confirmation", true: " queued"}[queued], func(t *testing.T) {
				s, cl, _, calls, _ := triggerUIFixture(t)
				s.ExecuteCommand(":ambient-watch changes main.go")
				if queued {
					s.HandleKey(KeyEvent{Key: "Enter"})
				}
				switch change {
				case "draft":
					s.AgentTriggers.Draft = "changes other.go"
				case "control":
					s.AgentTriggers.Data.Record.ControlRevision++
				case "configuration":
					s.AgentTriggers.Data.Configuration = strings.Repeat("e", 64)
				case "window":
					s.AgentTriggers.Data.Record.Window.Charged.Runs++
				case "project":
					s.Project.ProjectRevision = "changed"
				}
				if queued {
					m := NewBubbleModel(t.Context(), s, cl, 80, 24)
					cmd, ok := m.prepareAsyncActionCmd("late", s.PendingActions[0])
					if !ok || cmd != nil {
						t.Fatal("changed queued control dispatched")
					}
				} else {
					s.HandleKey(KeyEvent{Key: "Enter"})
					for _, a := range s.PendingActions {
						if a.Kind == "agent-triggers" {
							t.Fatal("changed confirmation dispatched")
						}
					}
				}
				if calls.Load() != 0 {
					t.Fatal("stale control dispatched provider")
				}
			})
		}
	}
}

func TestAmbientUICancelJoinsTransportAndRejectsLateReceipt(t *testing.T) {
	for _, ending := range []string{"navigate", "quit", "project"} {
		t.Run(ending, func(t *testing.T) {
			s, _, _, _, _ := triggerUIFixture(t)
			started, ended, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
			var once sync.Once
			cl := externalTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.Copy(io.Discard, r.Body)
				close(started)
				defer close(ended)
				select {
				case <-r.Context().Done():
				case <-release:
				}
			})
			t.Cleanup(func() { once.Do(func() { close(release) }) })
			s.ExecuteCommand(":ambient-watch changes main.go")
			s.HandleKey(KeyEvent{Key: "Enter"})
			m, cmd := libraryPending(t, t.Context(), s, cl)
			result := make(chan agentTriggerResultMsg, 1)
			go func() { result <- cmd().(agentTriggerResultMsg) }()
			select {
			case <-started:
			case <-time.After(5 * time.Second):
				t.Fatal("request did not start")
			}
			switch ending {
			case "navigate":
				s.Navigate(PageSummary)
			case "quit":
				m.Update(KeyEvent{Key: "c", Ctrl: true})
			case "project":
				s.Project = &Project{ProjectIdentity: ProjectIdentity{ProjectID: "other", ProjectRevision: "other-revision"}, Path: "/other"}
				m.Update(KeyEvent{})
			}
			select {
			case <-ended:
			case <-time.After(5 * time.Second):
				t.Fatal("navigation did not cancel transport")
			}
			select {
			case message := <-result:
				m.Update(message)
			case <-time.After(5 * time.Second):
				t.Fatal("request owner did not finish")
			}
			if ending == "project" {
				if s.AgentTriggers != nil {
					t.Fatal("private ambient state survived project replacement")
				}
				return
			}
			if s.AgentTriggers.Pending != "" || s.AgentTriggers.Recovery == "" || s.AgentTriggers.Data.State != "disabled" || s.AgentTriggers.Draft != "changes main.go" {
				t.Fatal("late/canceled result changed retained state")
			}
		})
	}
}

func TestAmbientUILayoutPalettesAndPassiveDisclosure(t *testing.T) {
	s, _, _, calls, _ := triggerUIFixture(t)
	s.AgentTriggers.Data.Record.Reason = "PRIVATE_ESCAPE\x1b[31m reason"
	for _, palette := range []Palette{PaletteTokyo, PaletteCRT, PaletteAmber, PalettePaper} {
		for _, size := range [][2]int{{52, 16}, {80, 24}, {120, 40}} {
			s.Palette = palette
			s.DetailScroll = 0
			s.AgentTriggers.Details = false
			frame := s.Render(size[0], size[1])
			if !strings.Contains(frame, "AMBIENT") || !strings.Contains(frame, "disabled") || strings.Contains(frame, "PRIVATE_ESCAPE\x1b[31m") {
				t.Fatalf("ambient state inaccessible at %v", size)
			}
			for _, row := range strings.Split(frame, "\n") {
				if theme.VisibleRuneCount(row) != size[0] {
					t.Fatal("ambient layout overflow")
				}
			}
			s.HandleKey(KeyEvent{Key: "Enter"})
			s.HandleKey(KeyEvent{Key: "End"})
			_ = s.Render(size[0], size[1])
			s.HandleKey(KeyEvent{Key: "Home"})
			for _, key := range []string{"a", "c", "u", "f"} {
				s.HandleKey(KeyEvent{Key: key})
			}
			if len(s.PendingActions) != 0 || calls.Load() != 0 {
				t.Fatal("disclosure or unrelated source shortcuts dispatched")
			}
		}
	}
	s.HandleKey(KeyEvent{Key: "w", Pasted: true})
	if s.CmdActive {
		t.Fatal("pasted shortcut opened an action")
	}
	for _, arg := range []string{"changes ../secret", "changes main.go;main.go", "interval 5 main.go", "changes missing.go"} {
		s.Error = ""
		s.ExecuteCommand(":ambient-watch " + arg)
		if s.Error == "" || s.Confirmation != nil || len(s.PendingActions) != 0 {
			t.Fatal("unsafe watch draft accepted", arg)
		}
	}
}

func TestAmbientUIPermissionStatesExpireLocallyAndRetainRecovery(t *testing.T) {
	s, cl, _, calls, _ := triggerUIFixture(t)
	s.Change = &ChangeSession{ProjectIdentity: ProjectIdentity{ProjectID: "task", ProjectRevision: "task-revision"}, WorktreePath: "/task"}
	s.ExecuteCommand(":ambient-watch changes main.go")
	if !strings.Contains(s.Confirmation.Message, "Main checkout: "+s.Project.Path) || strings.Contains(s.Confirmation.Message, "/task") {
		t.Fatal("task selection retargeted ambient scope")
	}
	s.HandleKey(KeyEvent{Key: "Enter"})
	applyTriggerPending(t, s, cl)
	s.ExecuteCommand(":ambient-grant 60")
	s.HandleKey(KeyEvent{Key: "Enter"})
	applyTriggerPending(t, s, cl)
	g := s.AgentTriggers.Data.Record.Grant
	g.CreatedAt, g.ExpiresAt = time.Now().Add(-2*time.Minute), time.Now().Add(-time.Minute)
	if !strings.Contains(strings.ToLower(s.Render(80, 24)), "grant expired") {
		t.Fatalf("elapsed permission still appeared active: %s", s.Render(80, 24))
	}
	for _, state := range []string{"watch_only", "grant_expired", "stale_settings", "stale_grant", "interrupted", "failed"} {
		s.AgentTriggers.Data.State = state
		for _, palette := range []Palette{PaletteTokyo, PaletteCRT, PaletteAmber, PalettePaper} {
			s.Palette = palette
			for _, size := range [][2]int{{52, 16}, {80, 24}, {120, 40}} {
				s.DetailScroll = 0
				frame := s.Render(size[0], size[1])
				if !strings.Contains(strings.ToLower(frame), strings.ReplaceAll(state, "_", " ")) || !strings.Contains(frame, "Revoke") || !strings.Contains(frame, "Disable") {
					t.Fatalf("permission/recovery inaccessible: %s at %v", state, size)
				}
				for _, row := range strings.Split(frame, "\n") {
					if theme.VisibleRuneCount(row) != size[0] {
						t.Fatal("permission layout overflow")
					}
				}
			}
		}
	}
	s.AgentTriggers.Status, s.AgentTriggers.Failure, s.AgentTriggers.Recovery = "unknown", "Receipt unavailable", "Refresh before another control."
	if !strings.Contains(strings.ToLower(s.Render(80, 24)), "last known") || !strings.Contains(s.Render(80, 24), "Refresh before") {
		t.Fatal("uncertain receipt appeared current")
	}
	if calls.Load() != 0 || len(s.PendingActions) != 0 {
		t.Fatal("rendering permission states dispatched")
	}
}
