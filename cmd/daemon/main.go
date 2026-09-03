package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/api/handlers"
	"github.com/nanaki-93/mini-orca/v2/internal/app"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/logging"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
	"github.com/nanaki-93/mini-orca/v2/internal/version"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		logging.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	// Initialize logger
	logging.Init(logging.Config{
		Level:         cfg.Logging.Level,
		Format:        cfg.Logging.Format,
		Filename:      cfg.Logging.Filename,
		SensitiveKeys: cfg.Logging.SensitiveKeys,
	})

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		logging.Error("Invalid configuration", "error", err)
		os.Exit(1)
	}

	projectManager, err := project.NewManager(cfg.ProjectPath)
	if err != nil {
		logging.Error("Invalid initial project path", "error", err)
		os.Exit(1)
	}

	application, err := app.New(cfg, projectManager)
	if err != nil {
		logging.Error("Failed to initialize application service", "error", err)
		os.Exit(1)
	}

	// Log successful startup
	logging.Info("Mini-Orca daemon started successfully")
	for _, model := range application.EffectiveModels() {
		logging.Info("Model scope initialized", "scope", model.Scope, "model", model.Model, "reasoning_effort", model.ReasoningEffort, "provider_origin", model.ProviderOrigin, "remote_provider", model.RemoteProvider)
	}
	logging.Info("Workflow: single-coder preview with explicit review")

	// Start HTTP server with all API endpoints
	server := startHTTPServer(application, projectManager)

	// Wait for shutdown signal
	quit := waitForShutdown()
	logging.Info("Received shutdown signal", "signal", quit)

	// Shutdown HTTP server
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logging.Error("HTTP server shutdown error", "error", err)
	}

	logging.Info("Shutdown complete")
}

// startHTTPServer creates and starts the HTTP server with all API endpoints.
func startHTTPServer(
	application *app.Service,
	projectManager *project.Manager,
) *http.Server {
	mux := newHTTPMux(application, projectManager)

	server := &http.Server{
		Addr:         daemonAddress(),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 6 * time.Minute,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		logging.Info("HTTP server starting", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logging.Error("HTTP server error", "error", err)
			os.Exit(1)
		}
	}()

	return server
}

// daemonAddress is loopback-only unless a local deployment explicitly opts in
// to another bind address (for example, a container port mapping).
func daemonAddress() string {
	if address := os.Getenv("MINI_ORCA_BIND_ADDRESS"); address != "" {
		return address
	}
	return "127.0.0.1:9090"
}

const (
	routeRetained = "retained"
	routeRetired  = "retired"
)

// routeSpec records the pre-cleanup HTTP surface and its current owner. It is
// intentionally local to the daemon: tests use it to keep registration and the
// cleanup inventory aligned without introducing a routing abstraction.
type routeSpec struct {
	Method      string `json:"method"`
	Path        string `json:"path"`
	Disposition string `json:"disposition"`
	Consumer    string `json:"consumer,omitempty"`
}

func registeredRoutes() []routeSpec {
	return []routeSpec{
		{http.MethodGet, "/health", routeRetained, "container health check"},
		{http.MethodGet, "/status", routeRetained, "Desktop ApiClient.status"},
		{http.MethodGet, "/api/system/info", routeRetired, ""},
		{http.MethodPost, "/api/projects/current/chat/sessions", routeRetained, "Desktop ApiClient.openChatSession"},
		{http.MethodGet, "/api/projects/current/chat/sessions/{sessionID}", routeRetired, ""},
		{http.MethodPost, "/api/projects/current/chat/sessions/{sessionID}/messages", routeRetained, "Desktop ApiClient.sendChatMessage"},
		{http.MethodGet, "/api/projects/current/activity", routeRetired, ""},
		{http.MethodPost, "/api/chat/message", routeRetired, ""},
		{http.MethodGet, "/api/chat/history", routeRetired, ""},
		{http.MethodGet, "/api/models/current", routeRetained, "Desktop ApiClient.modelCatalog"},
		{http.MethodGet, "/api/projects/current/context", routeRetained, "Desktop ApiClient.context"},
		{http.MethodPost, "/api/projects/import", routeRetained, "Desktop ApiClient.importProject"},
		{http.MethodPost, "/api/projects/restore", routeRetained, "Desktop ApiClient.restoreProject"},
		{http.MethodGet, "/api/projects/current", routeRetired, ""},
		{http.MethodGet, "/api/projects/current/overview", routeRetained, "Desktop ApiClient.overview"},
		{http.MethodGet, "/api/projects/current/findings", routeRetained, "Desktop ApiClient.findings"},
		{http.MethodPatch, "/api/projects/current/findings/{findingID}", routeRetained, "Desktop ApiClient.updateFindingStatus"},
		{http.MethodGet, "/api/projects/current/scan", routeRetained, "Desktop ApiClient.goScan"},
		{http.MethodPost, "/api/projects/current/scan", routeRetained, "Desktop ApiClient.startGoScan"},
		{http.MethodDelete, "/api/projects/current/scan", routeRetained, "Desktop ApiClient.cancelGoScan"},
		{http.MethodGet, "/api/projects/current/index", routeRetained, "Desktop ApiClient.index"},
		{http.MethodGet, "/api/projects/current/files/info", routeRetained, "Desktop ApiClient.fileInfo"},
		{http.MethodGet, "/api/projects/current/files/symbols", routeRetained, "Desktop ApiClient.symbols"},
		{http.MethodGet, "/api/projects/current/impact", routeRetained, "Desktop ApiClient.impact"},
		{http.MethodGet, "/api/projects/current/git", routeRetained, "Desktop ApiClient.gitStatus"},
		{http.MethodGet, "/api/projects/current/files/analysis", routeRetained, "Desktop ApiClient.analysis"},
		{http.MethodPost, "/api/projects/current/files/analysis", routeRetained, "Desktop ApiClient.analyze"},
		{http.MethodDelete, "/api/projects/current/files/analysis", routeRetired, ""},
		{http.MethodGet, "/api/projects/current/analysis-job", routeRetained, "Desktop ApiClient.analyzeAllJob"},
		{http.MethodPost, "/api/projects/current/analysis-job", routeRetained, "Desktop ApiClient.startAnalyzeAll"},
		{http.MethodPost, "/api/projects/current/analysis-job/pause", routeRetained, "Desktop ApiClient.pauseAnalyzeAll"},
		{http.MethodPost, "/api/projects/current/analysis-job/resume", routeRetained, "Desktop ApiClient.resumeAnalyzeAll"},
		{http.MethodPost, "/api/projects/current/analysis-job/cancel", routeRetained, "Desktop ApiClient.cancelAnalyzeAll"},
		{http.MethodPost, "/api/projects/current/reindex", routeRetained, "Desktop ApiClient.reindex"},
		{http.MethodGet, "/api/projects/current/drafts/{draftID}", routeRetired, ""},
		{http.MethodPatch, "/api/projects/current/drafts/{draftID}", routeRetained, "Desktop ApiClient.updateDraft"},
		{http.MethodPost, "/api/projects/current/drafts/{draftID}/validate", routeRetained, "Desktop ApiClient.validateDraft"},
		{http.MethodPost, "/api/projects/current/drafts/{draftID}/checks", routeRetained, "Desktop ApiClient.checkDraft"},
		{http.MethodGet, "/api/projects/current/drafts/{draftID}/review", routeRetired, ""},
		{http.MethodPost, "/api/projects/current/candidates/checks", routeRetired, ""},
		{http.MethodPost, "/api/projects/current/candidates/compare", routeRetired, ""},
		{http.MethodPost, "/api/projects/current/candidates/export", routeRetired, ""},
		{http.MethodPost, "/api/projects/current/apply", routeRetained, "Desktop ApiClient.applyDraft"},
		{http.MethodPost, "/api/projects/current/undo", routeRetained, "Desktop ApiClient.undo"},
		{http.MethodGet, "/api/projects/current/audit", routeRetired, ""},
	}
}

// newHTTPMux registers only the local desktop API routes.
func newHTTPMux(
	application *app.Service,
	projectManager *project.Manager,
) *http.ServeMux {
	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","version":"` + version.Version + `"}`))
	})

	// Status endpoint
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"running","version":"` + version.Version + `","workflow":"single_coder_preview"}`))
	})

	// System info endpoint
	systemHandler := handlers.NewSystemHandler()
	mux.HandleFunc("GET /api/system/info", systemHandler.GetSystemInfo)

	// Initialize chat handler
	chatHandler := handlers.NewChatHandler(application)

	// File-scoped chat session endpoints. A message has no target fields; its
	// immutable project/file/symbol identity is established at session creation.
	mux.HandleFunc("POST /api/projects/current/chat/sessions", chatHandler.OpenSession)
	mux.HandleFunc("GET /api/projects/current/chat/sessions/{sessionID}", chatHandler.Session)
	mux.HandleFunc("POST /api/projects/current/chat/sessions/{sessionID}/messages", chatHandler.SendSessionMessage)
	mux.HandleFunc("GET /api/projects/current/activity", chatHandler.Activity)
	// Keep the retired discovery route from silently accepting an unsafe one-shot
	// request until the desktop client moves to the typed session contract.
	mux.HandleFunc("POST /api/chat/message", chatHandler.RemovedMessageEndpoint)
	mux.HandleFunc("GET /api/chat/history", chatHandler.Activity)

	modelHandler := handlers.NewModelHandler(application)
	mux.HandleFunc("GET /api/models/current", modelHandler.Current)
	contextHandler := handlers.NewContextHandler(application)
	mux.HandleFunc("GET /api/projects/current/context", contextHandler.Preview)

	projectHandler := handlers.NewProjectHandler(projectManager, application)
	candidateHandler := handlers.NewCandidateHandler(application, projectManager)
	mux.HandleFunc("POST /api/projects/import", projectHandler.Import)
	mux.HandleFunc("POST /api/projects/restore", projectHandler.Restore)
	mux.HandleFunc("GET /api/projects/current", projectHandler.Current)
	mux.HandleFunc("GET /api/projects/current/overview", projectHandler.Overview)
	mux.HandleFunc("GET /api/projects/current/findings", projectHandler.Findings)
	mux.HandleFunc("PATCH /api/projects/current/findings/{findingID}", projectHandler.UpdateFindingStatus)
	mux.HandleFunc("GET /api/projects/current/scan", projectHandler.GoScanProgress)
	mux.HandleFunc("POST /api/projects/current/scan", projectHandler.StartGoScan)
	mux.HandleFunc("DELETE /api/projects/current/scan", projectHandler.CancelGoScan)
	mux.HandleFunc("GET /api/projects/current/index", projectHandler.Index)
	mux.HandleFunc("GET /api/projects/current/files/info", projectHandler.FileInfo)
	mux.HandleFunc("GET /api/projects/current/files/symbols", projectHandler.Symbols)
	mux.HandleFunc("GET /api/projects/current/impact", projectHandler.Impact)
	mux.HandleFunc("GET /api/projects/current/git", projectHandler.GitStatus)
	mux.HandleFunc("GET /api/projects/current/files/analysis", projectHandler.FileAnalysis)
	mux.HandleFunc("POST /api/projects/current/files/analysis", projectHandler.AnalyzeFile)
	mux.HandleFunc("DELETE /api/projects/current/files/analysis", projectHandler.DeleteFileAnalysis)
	mux.HandleFunc("GET /api/projects/current/analysis-job", projectHandler.AnalyzeAllJob)
	mux.HandleFunc("POST /api/projects/current/analysis-job", projectHandler.StartAnalyzeAll)
	mux.HandleFunc("POST /api/projects/current/analysis-job/pause", projectHandler.PauseAnalyzeAll)
	mux.HandleFunc("POST /api/projects/current/analysis-job/resume", projectHandler.ResumeAnalyzeAll)
	mux.HandleFunc("POST /api/projects/current/analysis-job/cancel", projectHandler.CancelAnalyzeAll)
	mux.HandleFunc("POST /api/projects/current/reindex", projectHandler.Reindex)
	mux.HandleFunc("GET /api/projects/current/drafts/{draftID}", candidateHandler.Draft)
	mux.HandleFunc("PATCH /api/projects/current/drafts/{draftID}", candidateHandler.UpdateDraft)
	mux.HandleFunc("POST /api/projects/current/drafts/{draftID}/validate", candidateHandler.ValidateDraft)
	mux.HandleFunc("POST /api/projects/current/drafts/{draftID}/checks", candidateHandler.CheckDraft)
	mux.HandleFunc("GET /api/projects/current/drafts/{draftID}/review", candidateHandler.ReviewDraft)
	mux.HandleFunc("POST /api/projects/current/candidates/checks", candidateHandler.Check)
	mux.HandleFunc("POST /api/projects/current/candidates/compare", candidateHandler.Compare)
	mux.HandleFunc("POST /api/projects/current/candidates/export", candidateHandler.Export)
	mux.HandleFunc("POST /api/projects/current/apply", candidateHandler.Apply)
	mux.HandleFunc("POST /api/projects/current/undo", candidateHandler.Undo)
	mux.HandleFunc("GET /api/projects/current/audit", candidateHandler.Audit)

	return mux
}

// waitForShutdown blocks until a SIGINT or SIGTERM signal is received.
func waitForShutdown() os.Signal {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigChan
	signal.Stop(sigChan)
	return sig
}
