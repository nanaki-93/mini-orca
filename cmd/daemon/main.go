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
	logging.Info("Startup config",
		"base_url", cfg.LLM.BaseURL,
		"model", cfg.LLM.Model,
		"temperature", cfg.LLM.Temperature)

	logging.Info("Generation profile initialized", "profile", application.EffectiveModel().Profile, "model", application.EffectiveModel().Model)
	logging.Info("Workflow: single-coder preview with explicit review")

	// List available models
	modelsCtx, cancelModels := context.WithTimeout(context.Background(), 10*time.Second)
	models, err := application.AnalysisClient().ListModels(modelsCtx)
	cancelModels()
	if err != nil {
		logging.Warn("Failed to list models", "error", err)
	} else {
		logging.Info("Available models", "count", len(models))
		for _, m := range models {
			logging.Debug("Model detail", "id", m.ID, "owned_by", m.OwnedBy)
		}
	}

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

	// Register chat endpoints
	mux.HandleFunc("POST /api/chat/message", chatHandler.SendMessage)
	mux.HandleFunc("GET /api/chat/history", chatHandler.GetHistory)

	modelHandler := handlers.NewModelHandler(application)
	mux.HandleFunc("GET /api/models/current", modelHandler.Current)
	contextHandler := handlers.NewContextHandler(application)
	mux.HandleFunc("GET /api/projects/current/context", contextHandler.Preview)

	projectHandler := handlers.NewProjectHandler(projectManager, application)
	candidateHandler := handlers.NewCandidateHandler(application, projectManager)
	mux.HandleFunc("POST /api/projects/import", projectHandler.Import)
	mux.HandleFunc("GET /api/projects/current", projectHandler.Current)
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
