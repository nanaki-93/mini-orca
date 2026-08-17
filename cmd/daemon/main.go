package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/api"
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

	// Initialize API stores and handlers

	// Initialize template engine
	templatesPath := "internal/api/templates"
	templateEngine, err := handlers.NewTemplateEngine(templatesPath)
	if err != nil {
		logging.Warn("Failed to initialize template engine", "error", err)
		templateEngine = nil
	}

	// Initialize HTMX render handler
	var htmxRenderHandler *handlers.HTMXRenderHandler
	if templateEngine != nil {
		cache := api.NewResponseCache(10 * time.Second)
		htmxRenderHandler = handlers.NewHTMXRenderHandler(templateEngine, cache, projectManager)
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
	server := startHTTPServer(application, htmxRenderHandler, projectManager)

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
	htmxRenderHandler *handlers.HTMXRenderHandler,
	projectManager *project.Manager,
) *http.Server {
	mux := newHTTPMux(application, htmxRenderHandler, projectManager)

	// Initialize error handler
	templatesPath := "internal/api/templates"
	errorHandler, err := api.NewErrorHandler(templatesPath)
	if err != nil {
		logging.Warn("Failed to initialize error handler", "error", err)
		errorHandler = nil
	}

	// Wrap with error handler middleware
	var handler http.Handler = mux
	if errorHandler != nil {
		handler = errorHandler.Next(handler)
	}

	server := &http.Server{
		Addr:         ":9090",
		Handler:      handler,
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

// newHTTPMux registers the daemon routes. Browser UI routes remain temporary
// compatibility routes until the desktop replacement is complete.
func newHTTPMux(
	application *app.Service,
	htmxRenderHandler *handlers.HTMXRenderHandler,
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

	// Static files
	fs := http.FileServer(http.Dir("internal/api/static"))
	mux.Handle("/static/", http.StripPrefix("/static/", fs))

	// Register HTMX render endpoints
	if htmxRenderHandler != nil {
		mux.HandleFunc("GET /api/render/file-tree", htmxRenderHandler.RenderFileTree)
		mux.HandleFunc("GET /api/tree/expand", htmxRenderHandler.ExpandFolder)
		mux.HandleFunc("GET /api/files/view", htmxRenderHandler.ViewFile)
	}

	// Register main page
	if htmxRenderHandler != nil {
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			// Only handle the root path, let other routes handle their own paths
			if r.URL.Path == "/" {
				htmxRenderHandler.RenderMainPage(w, r)
			} else {
				http.NotFound(w, r)
			}
		})
	}

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
	mux.HandleFunc("POST /api/projects/import", projectHandler.Import)
	mux.HandleFunc("GET /api/projects/current", projectHandler.Current)
	mux.HandleFunc("GET /api/projects/current/files/info", projectHandler.FileInfo)

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
