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
	"sync/atomic"
	"testing"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/api/handlers"
	"github.com/nanaki-93/mini-orca/v2/internal/app"
	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/theme"
)

type cloudUIFixture struct {
	s        *State
	cl       *Client
	service  *app.Service
	source   string
	controls atomic.Int32
	lost     atomic.Bool
}

func newCloudUIFixture(t *testing.T) *cloudUIFixture {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc main() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	manager, err := project.NewManager(root)
	if err != nil {
		t.Fatal(err)
	}
	if err = manager.Set(root, &project.Analysis{Name: "cloud fixture", Path: root}); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Integrations.Cloud = []config.CloudTarget{{ID: "archive", Service: "gcs_artifact", Project: "123456789012", Region: "europe-west10", Resource: "archive-bucket", Principal: "archive-agent@example-project.iam.gserviceaccount.com", CredentialFileEnv: "CLOUD_UI_KEY", RetentionDays: 30}}
	backup := cfg.Integrations.Cloud[0]
	backup.ID, backup.Resource = "backup", "backup-bucket"
	cfg.Integrations.Cloud = append(cfg.Integrations.Cloud, backup)
	t.Setenv("CLOUD_UI_KEY", filepath.Join(t.TempDir(), "missing-key"))
	service, err := app.New(cfg, manager)
	if err != nil {
		t.Fatal(err)
	}
	index, err := manager.Index()
	if err != nil {
		t.Fatal(err)
	}
	store, err := agentstate.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.SaveArtifact(t.Context(), agentstate.ArtifactWrite{Key: agentstate.ArtifactKey{Session: agentstate.Key{App: "mini-orca", User: "local", ID: "fixture"}, Name: "result.txt"}, Kind: "text", MIME: "text/plain", Data: []byte("PRIVATE_CLOUD_SOURCE"), Provenance: map[string]string{"project": index.ProjectID, "revision": index.ProjectRevision}})
	if err != nil {
		t.Fatal(err)
	}
	items, err := service.ListArtifacts(t.Context(), app.AgentRunQuery{ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision})
	if err != nil || len(items.Artifacts) != 1 {
		t.Fatal(items, err)
	}
	f := &cloudUIFixture{service: service, source: items.Artifacts[0].ID}
	h := handlers.NewAgentCloudHandler(service)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/projects/current/index", func(w http.ResponseWriter, _ *http.Request) { _ = json.NewEncoder(w).Encode(index) })
	mux.HandleFunc("GET /api/projects/current/cloud-copies", h.List)
	mux.HandleFunc("POST /api/projects/current/cloud-copies/preview", h.Preview)
	mux.HandleFunc("POST /api/projects/current/cloud-copies/control", func(w http.ResponseWriter, r *http.Request) {
		f.controls.Add(1)
		response := httptest.NewRecorder()
		h.Control(response, r)
		if f.lost.Swap(false) {
			http.Error(w, "receipt lost", http.StatusBadGateway)
			return
		}
		for key, values := range response.Header() {
			w.Header()[key] = values
		}
		w.WriteHeader(response.Code)
		_, _ = w.Write(response.Body.Bytes())
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	f.cl, err = NewClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.cl.Close() })
	f.s = NewState()
	f.s.Project = &Project{ProjectIdentity: ProjectIdentity{ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision}, Name: "cloud fixture", Path: root}
	f.s.ExecuteCommand(":cloud")
	applyCloudPending(t, f.s, f.cl)
	return f
}

func applyCloudPending(t *testing.T, s *State, cl *Client) {
	t.Helper()
	m, cmd := libraryPending(t, t.Context(), s, cl)
	m.Update(cmd())
}

func previewCloudFixture(t *testing.T, f *cloudUIFixture) {
	t.Helper()
	f.s.ExecuteCommand(":cloud-preview archive artifact " + f.source)
	applyCloudPending(t, f.s, f.cl)
	if f.s.AgentCloud.Preview == nil {
		t.Fatal("preview absent", f.s.AgentCloud.Failure)
	}
}

func TestCloudUIRealCopyRecoveryAndSeparateSharing(t *testing.T) {
	f := newCloudUIFixture(t)
	s := f.s
	s.HandleKey(KeyEvent{Key: "e"})
	if !s.CmdActive || !strings.HasPrefix(s.CmdInput, "cloud-preview ") || len(s.PendingActions) != 0 {
		t.Fatal("preview editor dispatched")
	}
	s.HandleKey(KeyEvent{Key: "Escape"})
	previewCloudFixture(t, f)
	if f.controls.Load() != 0 || s.AgentCloud.Preview.SourceID != f.source {
		t.Fatal("preview sent cloud control")
	}
	p := *s.AgentCloud.Preview
	s.HandleKey(KeyEvent{Key: "c"})
	if s.Confirmation == nil || len(s.PendingActions) != 0 {
		t.Fatal("copy skipped confirmation")
	}
	for _, value := range []string{p.Hash, p.SourceRevision, p.Target.Principal, p.Target.Project, p.Target.Region, p.Target.Resource, s.Project.Path, "private artifact", "Cleanup is manual"} {
		if !strings.Contains(s.Confirmation.Message, value) {
			t.Fatal("copy review omitted", value)
		}
	}
	s.HandleKey(KeyEvent{Key: "Enter"})
	applyCloudPending(t, s, f.cl)
	if f.controls.Load() != 1 || len(s.AgentCloud.Data.Copies) != 1 || s.AgentCloud.Data.Copies[0].State != "unknown" || s.AgentCloud.Preview != nil || s.AgentCloud.Recovery == "" {
		t.Fatal("unavailable cloud lost its uncertain receipt")
	}
	s.HandleKey(KeyEvent{Key: "i"})
	if s.Confirmation != nil || len(s.PendingActions) != 0 {
		t.Fatal("recovery skipped refresh")
	}
	s.Error = ""
	s.HandleKey(KeyEvent{Key: "r"})
	applyCloudPending(t, s, f.cl)
	if s.selectedCloudRow().Copy == nil || s.AgentCloud.Recovery != "" {
		t.Fatal("refresh lost receipt selection")
	}
	s.HandleKey(KeyEvent{Key: "i"})
	if s.Confirmation == nil || s.Confirmation.AcceptLabel != "Check" || len(s.PendingActions) != 0 {
		t.Fatal("remote check was not explicit")
	}
	s.HandleKey(KeyEvent{Key: "Enter"})
	applyCloudPending(t, s, f.cl)
	if f.controls.Load() != 2 || s.AgentCloud.Data.Copies[0].Version != 4 || s.AgentCloud.Data.Copies[0].State != "unknown" {
		t.Fatal("check rewrote remote uncertainty")
	}
	items, err := f.service.ListArtifacts(t.Context(), app.AgentRunQuery{ProjectID: s.Project.ProjectID, ProjectRevision: s.Project.ProjectRevision})
	if err != nil || len(items.Artifacts) != 1 {
		t.Fatal("cloud action changed originals", err)
	}
	s.ExecuteCommand(":cloud-refresh")
	applyCloudPending(t, s, f.cl)
	s.HandleKey(KeyEvent{Key: "d"})
	if s.Confirmation == nil || !strings.Contains(s.Confirmation.Message, "Delete the live remote copy") || !strings.Contains(s.Confirmation.Message, "backups") {
		t.Fatal("delete omitted retention limits")
	}
	s.HandleKey(KeyEvent{Key: "Escape"})
	s.HandleKey(KeyEvent{Key: "c"})
	if s.Confirmation != nil || len(s.PendingActions) != 0 {
		t.Fatal("copy replay accepted without a new preview")
	}
}

func TestCloudUILostReceiptRequiresLocalRefresh(t *testing.T) {
	f := newCloudUIFixture(t)
	previewCloudFixture(t, f)
	p := *f.s.AgentCloud.Preview
	f.lost.Store(true)
	f.s.ExecuteCommand(":cloud-copy")
	f.s.HandleKey(KeyEvent{Key: "Enter"})
	applyCloudPending(t, f.s, f.cl)
	if f.s.AgentCloud.Status != "unknown" || f.s.AgentCloud.Recovery == "" || f.s.AgentCloud.Preview != nil {
		t.Fatal("lost receipt became safe to retry")
	}
	f.s.Error = ""
	f.s.ExecuteCommand(":cloud-refresh")
	applyCloudPending(t, f.s, f.cl)
	if len(f.s.AgentCloud.Data.Copies) != 1 || f.s.AgentCloud.Data.Copies[0].ID != p.ID || f.controls.Load() != 1 {
		t.Fatal("refresh repeated a copy or lost its receipt")
	}
}

func TestCloudUIStaleConfirmationAndQueuedDispatch(t *testing.T) {
	for _, phase := range []string{"confirmation", "queued"} {
		for _, change := range []string{"draft", "hash", "destination", "revision", "selection"} {
			t.Run(phase+"/"+change, func(t *testing.T) {
				f := newCloudUIFixture(t)
				s := f.s
				previewCloudFixture(t, f)
				s.ExecuteCommand(":cloud-copy")
				if phase == "queued" {
					s.HandleKey(KeyEvent{Key: "Enter"})
				}
				switch change {
				case "draft":
					s.AgentCloud.Draft += " edited"
				case "hash":
					s.AgentCloud.Preview.Hash = strings.Repeat("d", 64)
				case "destination":
					s.AgentCloud.Data.Destinations[0].Target.Resource = "other-bucket"
				case "revision":
					s.Project.ProjectRevision = "changed"
				case "selection":
					s.SelectedIdx = 1
				}
				if phase == "confirmation" {
					s.HandleKey(KeyEvent{Key: "Enter"})
					if len(s.PendingActions) != 0 {
						t.Fatal("stale confirmation dispatched")
					}
				} else {
					a := s.PendingActions[0]
					s.PendingActions = nil
					m := NewBubbleModel(t.Context(), s, f.cl, 80, 24)
					cmd, ok := m.prepareAsyncActionCmd("stale", a)
					if !ok || cmd != nil {
						t.Fatal("stale queued control dispatched")
					}
				}
				if f.controls.Load() != 0 {
					t.Fatal("stale control reached handler")
				}
			})
		}
	}
}

func TestCloudUICancellationRejectsLateResultsAndRetainsDraft(t *testing.T) {
	f := newCloudUIFixture(t)
	s := f.s
	previewCloudFixture(t, f)
	draft := s.AgentCloud.Draft
	s.ExecuteCommand(":cloud-copy")
	s.HandleKey(KeyEvent{Key: "Enter"})
	a := *s.PendingActions[0].AgentCloud
	s.PendingActions = nil
	ctx, cancel := context.WithCancel(t.Context())
	s.AgentCloud.Cancel = cancel
	s.Navigate(PageEditor)
	if ctx.Err() == nil || s.AgentCloud.Pending != "" || s.AgentCloud.Recovery == "" || s.AgentCloud.Draft != draft {
		t.Fatal("navigation lost cancellation or recovery")
	}
	s.applyCloudResult(agentCloudResult{Action: a, Err: context.Canceled})
	if s.AgentCloud.Status != "canceled" || f.controls.Load() != 0 {
		t.Fatal("late result replaced canceled state")
	}
	s.Project = &Project{ProjectIdentity: ProjectIdentity{ProjectID: "replacement", ProjectRevision: "revision"}, Path: "/other"}
	s.reconcileCloudScope()
	if s.AgentCloud != nil {
		t.Fatal("replacement retained private cloud metadata")
	}
}

func TestCloudUILiveTransportCancellationAndQuit(t *testing.T) {
	for _, action := range []string{"navigate", "quit"} {
		t.Run(action, func(t *testing.T) {
			f := newCloudUIFixture(t)
			s := f.s
			previewCloudFixture(t, f)
			started, closed := make(chan struct{}), make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.Copy(io.Discard, r.Body)
				close(started)
				<-r.Context().Done()
				close(closed)
			}))
			t.Cleanup(server.Close)
			cl, err := NewClient(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = cl.Close() })
			s.ExecuteCommand(":cloud-copy")
			s.HandleKey(KeyEvent{Key: "Enter"})
			m, cmd := libraryPending(t, t.Context(), s, cl)
			results := make(chan agentCloudResultMsg, 1)
			go func() { results <- cmd().(agentCloudResultMsg) }()
			select {
			case <-started:
			case <-time.After(5 * time.Second):
				t.Fatal("request did not start")
			}
			if action == "navigate" {
				s.Navigate(PageInstructions)
			} else {
				s.Quitting = true
				m.Update(nil)
			}
			select {
			case <-closed:
			case <-time.After(5 * time.Second):
				t.Fatal("request was not canceled")
			}
			select {
			case result := <-results:
				m.Update(result)
			case <-time.After(5 * time.Second):
				t.Fatal("canceled worker leaked")
			}
			if s.AgentCloud.Status != "canceled" || s.AgentCloud.Recovery == "" || s.AgentCloud.Preview != nil {
				t.Fatal("cancel lost uncertain outcome")
			}
		})
	}
}

func TestCloudUILayoutAndPassiveKeyboardActions(t *testing.T) {
	f := newCloudUIFixture(t)
	s := f.s
	if len(HubTabs()) != 7 {
		t.Fatal("cloud added another workspace")
	}
	for _, palette := range []Palette{PaletteTokyo, PaletteCRT, PaletteAmber, PalettePaper} {
		for _, size := range [][2]int{{52, 16}, {80, 24}, {120, 40}} {
			s.Palette = palette
			s.DetailScroll = 0
			s.AgentCloud.Details = false
			frame := s.Render(size[0], size[1])
			if !strings.Contains(frame, "Cloud copies") || !strings.Contains(frame, "configured") {
				t.Fatalf("missing cloud state at %v", size)
			}
			for _, line := range strings.Split(frame, "\n") {
				if theme.VisibleRuneCount(line) != size[0] {
					t.Fatal("cloud frame overflow", size)
				}
			}
			s.HandleKey(KeyEvent{Key: "Enter"})
			s.HandleKey(KeyEvent{Key: "End"})
			_ = s.Render(size[0], size[1])
			s.HandleKey(KeyEvent{Key: "Home"})
			for _, key := range []string{"a", "u", "f"} {
				s.HandleKey(KeyEvent{Key: key})
			}
			if len(s.PendingActions) != 0 || f.controls.Load() != 0 {
				t.Fatal("passive navigation dispatched work")
			}
		}
	}
	s.HandleKey(KeyEvent{Key: "e", Pasted: true})
	if s.CmdActive {
		t.Fatal("pasted shortcut opened action")
	}
	previewCloudFixture(t, f)
	s.AgentCloud.Details = true
	details := strings.Join(s.renderCloudDetails(52), "\n")
	if strings.Contains(details, "PRIVATE_CLOUD_SOURCE") || !strings.Contains(details, "SHA-256") || !strings.Contains(details, "Cleanup") && !strings.Contains(details, "cleanup") {
		t.Fatal("preview exposed private content or omitted review detail")
	}
	for _, command := range []string{"cloud-copy", "cloud-check", "cloud-delete"} {
		s.openCommandInput(command)
		s.HandleKey(KeyEvent{Key: "Tab"})
		s.HandleKey(KeyEvent{Key: "Escape"})
		if len(s.PendingActions) != 0 || f.controls.Load() != 0 {
			t.Fatal("command browsing dispatched work")
		}
	}
	// A last-known receipt never becomes a claim of current cloud availability.
	s.AgentCloud.Status, s.AgentCloud.Failure = "unavailable", "Remote\x1b[31m diagnostic"
	frame := s.Render(80, 24)
	if !strings.Contains(strings.ToLower(frame), "last known") || strings.Contains(frame, "Remote\x1b[31m") {
		t.Fatal("unsafe or misleading failure state")
	}
}

func TestCloudUIPrefillsInspectedArtifactWithoutUploading(t *testing.T) {
	f := newCloudUIFixture(t)
	s := f.s
	s.AgentCloud = nil
	s.AgentLibrary = newLibraryView(s.Project.ProjectIdentity, client.AgentSettingsScope{ProjectIdentity: s.Project.ProjectIdentity}, s.Project.Path)
	s.AgentLibrary.Artifacts = &client.ArtifactList{Artifacts: []client.AgentArtifact{{ID: f.source, Name: "result.txt", Version: 1}}}
	s.Navigate(PageArtifacts)
	s.ExecuteCommand(":cloud")
	applyCloudPending(t, s, f.cl)
	if s.AgentCloud.Draft != "archive artifact "+f.source || s.AgentCloud.Preview != nil || f.controls.Load() != 0 {
		t.Fatal("source prefill skipped preview or sharing review")
	}
}

func TestCloudUIRecoveryRejectsChangedDestinationsAndActiveLeases(t *testing.T) {
	f := newCloudUIFixture(t)
	s := f.s
	previewCloudFixture(t, f)
	s.ExecuteCommand(":cloud-copy")
	s.HandleKey(KeyEvent{Key: "Enter"})
	applyCloudPending(t, s, f.cl)
	s.ExecuteCommand(":cloud-refresh")
	applyCloudPending(t, s, f.cl)
	copy := &s.AgentCloud.Data.Copies[0]
	copy.State, copy.LeaseUntil = "checking", time.Now().Add(time.Minute)
	s.ExecuteCommand(":cloud-check")
	if s.Confirmation != nil || len(s.PendingActions) != 0 {
		t.Fatal("active lease permitted concurrent control")
	}
	copy.State, copy.LeaseUntil = "unknown", time.Time{}
	s.AgentCloud.Data.Destinations[0].Target.Resource = "different-bucket"
	s.ExecuteCommand(":cloud-delete")
	if s.Confirmation != nil || len(s.PendingActions) != 0 || !strings.Contains(s.Error, "recorded destination") {
		t.Fatal("changed destination permitted recovery")
	}
	if f.controls.Load() != 1 {
		t.Fatal("rejected recovery reached transport")
	}
}

func TestCloudUISelectionChangedAfterDispatchRequiresReceiptRefresh(t *testing.T) {
	f := newCloudUIFixture(t)
	s := f.s
	previewCloudFixture(t, f)
	s.ExecuteCommand(":cloud-copy")
	s.HandleKey(KeyEvent{Key: "Enter"})
	m, cmd := libraryPending(t, t.Context(), s, f.cl)
	result := cmd()
	s.HandleKey(KeyEvent{Key: "j"})
	m.Update(result)
	if s.AgentCloud.Status != "unknown" || s.AgentCloud.Recovery == "" || s.AgentCloud.Preview != nil || len(s.AgentCloud.Data.Copies) != 0 {
		t.Fatal("late receipt was presented under a replaced selection")
	}
	s.ExecuteCommand(":cloud-refresh")
	applyCloudPending(t, s, f.cl)
	if len(s.AgentCloud.Data.Copies) != 1 || f.controls.Load() != 1 {
		t.Fatal("refresh lost or replayed the admitted copy")
	}
}

func TestCloudUIReceiptStatesRemainVisibleAtCompactSizes(t *testing.T) {
	f := newCloudUIFixture(t)
	s := f.s
	previewCloudFixture(t, f)
	s.ExecuteCommand(":cloud-copy")
	s.HandleKey(KeyEvent{Key: "Enter"})
	applyCloudPending(t, s, f.cl)
	s.AgentCloud.Recovery = ""
	s.AgentCloud.Data.Copies[0].Target.ID = "a_very_long_destination"
	for _, state := range []string{"completed", "unknown", "missing", "deleted", "copying", "checking", "deleting"} {
		s.AgentCloud.Data.Copies[0].State = state
		for _, palette := range []Palette{PaletteTokyo, PaletteCRT, PaletteAmber, PalettePaper} {
			s.Palette = palette
			for _, size := range [][2]int{{52, 16}, {80, 24}, {120, 40}} {
				s.DetailScroll, s.AgentCloud.Details = 0, false
				frame := s.Render(size[0], size[1])
				if !strings.Contains(frame, state) {
					t.Fatalf("%s hidden at %v: %s", state, size, frame)
				}
				for _, row := range strings.Split(frame, "\n") {
					if theme.VisibleRuneCount(row) != size[0] {
						t.Fatal("receipt frame overflow")
					}
				}
			}
		}
	}
}
