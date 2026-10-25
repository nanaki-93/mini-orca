package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nanaki-93/mini-orca/v2/internal/api/handlers"
	"github.com/nanaki-93/mini-orca/v2/internal/app"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

func mediaClientFixture(t *testing.T) (*Client, AgentSettingsScope, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	var workers sync.WaitGroup
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if strings.HasSuffix(r.URL.Path, ":generateContent") {
			_, _ = w.Write([]byte(`{"modelVersion":"gemini-2.5-flash","candidates":[{"finishReason":"STOP","content":{"role":"model","parts":[{"text":"PRIVATE_MEDIA_ADVICE"}]}}]}`))
			return
		}
		workers.Add(1)
		defer workers.Done()
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
		if err := conn.WriteJSON(map[string]any{"setupComplete": map[string]any{}}); err != nil {
			return
		}
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	t.Cleanup(func() { provider.Close(); workers.Wait() })
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	manager, err := project.NewManager(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Set(root, &project.Analysis{Name: "media fixture", Path: root}); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.AgentRuntime.LiveModel = "gemini-3.8-live"
	cfg.ModelScopes.Function.Provider, cfg.ModelScopes.Function.APIKey, cfg.ModelScopes.Function.Model, cfg.ModelScopes.Function.APIBaseURL = config.GeminiProvider, "explicit-test-key", "gemini-2.5-flash", provider.URL
	service, err := app.New(cfg, manager)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		if err := service.ShutdownChangeWorkflows(ctx); err != nil {
			t.Error(err)
		}
	})
	index, err := manager.Index()
	if err != nil {
		t.Fatal(err)
	}
	a, l, library := handlers.NewAgentAttachmentHandler(service), handlers.NewAgentLiveHandler(service), handlers.NewAgentLibraryHandler(service)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/projects/current/attachments", a.Overview)
	mux.HandleFunc("POST /api/projects/current/attachments", a.Import)
	mux.HandleFunc("POST /api/projects/current/attachments/advice", a.Advice)
	mux.HandleFunc("GET /api/projects/current/live-sessions", l.Overview)
	mux.HandleFunc("POST /api/projects/current/live-sessions", l.Open)
	mux.HandleFunc("GET /api/projects/current/live-sessions/{liveID}", l.Detail)
	mux.HandleFunc("POST /api/projects/current/live-sessions/{liveID}/control", l.Control)
	mux.HandleFunc("GET /api/projects/current/artifacts/{artifactID}", library.Detail)
	mux.HandleFunc("POST /api/projects/current/artifacts/{artifactID}/control", library.Control)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	cl, err := NewClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cl.Close() })
	return cl, AgentSettingsScope{ProjectIdentity: ProjectIdentity{ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision}}, &calls
}

func TestAttachmentsClientActualImportAdviceAndDisclosure(t *testing.T) {
	cl, scope, calls := mediaClientFixture(t)
	view, err := cl.ReadAttachments(t.Context(), "read", scope)
	if err != nil || view.State != "available" || calls.Load() != 0 {
		t.Fatalf("availability: %+v %v", view, err)
	}
	upload := AttachmentImport{ProjectIdentity: scope.ProjectIdentity, Name: "brief.pdf", MIME: "application/pdf", Data: []byte("%PDF-1.7\n1 0 obj\nPRIVATE_DOCUMENT\nendobj\n%%EOF"), ConfirmImport: true}
	item, err := cl.ImportAttachment(t.Context(), "import", scope, upload)
	if err != nil || item.Size != len(upload.Data) || calls.Load() != 0 {
		t.Fatalf("import: %+v %v", item, err)
	}
	request := AttachmentAdviceRequest{ProjectIdentity: scope.ProjectIdentity, Configuration: view.Configuration, SettingsVersion: view.SettingsVersion, Model: view.Model.Model, ProviderOrigin: view.Model.ProviderOrigin, Question: "Explain this document", Attachments: []AttachmentReference{{ID: item.ID, Version: item.Version, Hash: item.Hash}}, ConfirmRemoteProvider: true}
	if _, err := cl.AttachmentAdvice(t.Context(), "no-consent", scope, request); err == nil || calls.Load() != 0 {
		t.Fatal("missing sharing consent dispatched")
	}
	request.ConfirmSourceSharing = true
	receipt, err := cl.AttachmentAdvice(t.Context(), "advice", scope, request)
	if err != nil || receipt.State != "advisory" || receipt.UsageKnown || calls.Load() != 1 {
		t.Fatalf("advice: %+v %v", receipt, err)
	}
	detail, err := cl.ArtifactDetail(t.Context(), "disclose", scope, receipt.Artifact.ID)
	if err != nil || !strings.Contains(detail.Text, "PRIVATE_MEDIA_ADVICE") || calls.Load() != 1 {
		t.Fatalf("disclosure: %+v %v", detail, err)
	}
	request.Attachments[0].Hash = strings.Repeat("b", 64)
	if _, err := cl.AttachmentAdvice(t.Context(), "changed", scope, request); err == nil || calls.Load() != 1 {
		t.Fatal("changed media accepted")
	}
}

func TestAttachmentsClientRejectsMismatchedImportReceiptAndForeignCatalog(t *testing.T) {
	scope, artifact, _, _ := libraryClientFixture()
	upload := AttachmentImport{ProjectIdentity: scope.ProjectIdentity, Name: "brief.pdf", MIME: "application/pdf", Data: []byte("reviewed bytes"), ConfirmImport: true}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("worktree_path") != scope.WorktreePath {
			t.Error("lost worktree selector")
		}
		if r.Method == "POST" {
			item := artifact.AgentArtifact
			item.Kind, item.MIME, item.DisplayName = "inline", upload.MIME, upload.Name
			_ = json.NewEncoder(w).Encode(item)
			return
		}
		_ = json.NewEncoder(w).Encode(AttachmentOverview{ProjectIdentity: ProjectIdentity{ProjectID: "other", ProjectRevision: scope.ProjectRevision}, State: "available", Configuration: strings.Repeat("a", 64)})
	}))
	defer server.Close()
	cl, _ := NewClient(server.URL)
	defer cl.Close()
	if _, err := cl.ImportAttachment(t.Context(), "import", scope, upload); err == nil {
		t.Fatal("different bytes accepted")
	}
	if _, err := cl.ReadAttachments(t.Context(), "catalog", scope); err == nil {
		t.Fatal("foreign catalog accepted")
	}
	for _, target := range []string{"/api/projects/current/attachments/advice", "/api/projects/current/attachments/path"} {
		if allowedRequest(http.MethodGet, target, "") {
			t.Fatalf("unsupported route allowed: %s", target)
		}
	}
}
