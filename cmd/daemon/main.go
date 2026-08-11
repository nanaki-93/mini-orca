package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/agent"
	"github.com/nanaki-93/mini-orca/v2/internal/api"
	"github.com/nanaki-93/mini-orca/v2/internal/api/handlers"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/llm"
	"github.com/nanaki-93/mini-orca/v2/internal/logging"
	"github.com/nanaki-93/mini-orca/v2/internal/tools"
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

	// Initialize LLM client
	llmClient := llm.NewClient(
		cfg.LLM.BaseURL,
		cfg.LLM.APIKey,
		cfg.LLM.Model,
		cfg.LLM.Temperature,
		cfg.LLM.MaxTokens,
	)

	// Detect project type and create tool executor
	projectInfo, executor := tools.InitToolExecutorWithPath(cfg.ProjectPath)

	// Initialize agent registry and register agents
	agentRegistry := agent.InitAgentRegistry(llmClient)

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
		projectPath := cfg.ProjectPath
		htmxRenderHandler = handlers.NewHTMXRenderHandler(templateEngine, cache, projectPath)
	}

	// Initialize orchestrator with LLM client and executor
	agentOrchestrator := agent.NewOrchestrator(llmClient, executor)

	// Create agents for logging purposes
	coder := agent.NewCoderAgent(llmClient)
	coder.SetSkills(cfg.Agents.Coder.Skills)

	tester := agent.NewTesterAgent(llmClient)
	tester.SetSkills(cfg.Agents.Tester.Skills)

	reviewer := agent.NewReviewerAgent(llmClient)
	reviewer.SetSkills(cfg.Agents.Reviewer.Skills)

	// Log successful startup
	logging.Info("Mini-Orca daemon started successfully")
	logging.Info("Startup config",
		"base_url", cfg.LLM.BaseURL,
		"model", cfg.LLM.Model,
		"temperature", cfg.LLM.Temperature)

	// Log project type and executor
	if projectInfo != nil {
		logging.Info("Project info", "type", projectInfo.Type, "root_dir", projectInfo.RootDir)
	} else {
		logging.Info("Project type: generic (unknown) - using shell-only executor")
	}
	if executor != nil {
		logging.Info("Tool executor initialized successfully")
	}
	if agentOrchestrator != nil {
		logging.Info("Orchestrator initialized with executor")
	}

	// Log registered agents and their skills
	logging.Info("Agents initialized", "agents", []string{"coder", "tester", "reviewer"})
	logging.Info("Workflow: coding → testing → review → human_gate")

	// Log registered agents from registry
	logging.Info("Registered agents from registry", "agents", agentRegistry.List())

	// List available models
	models, err := llmClient.ListModels(context.Background())
	if err != nil {
		logging.Warn("Failed to list models", "error", err)
	} else {
		logging.Info("Available models", "count", len(models))
		for _, m := range models {
			logging.Debug("Model detail", "id", m.ID, "owned_by", m.OwnedBy)
		}
	}

	// Start HTTP server with all API endpoints
	projectPath := cfg.ProjectPath
	server := startHTTPServer(agentOrchestrator, htmxRenderHandler, llmClient, executor, projectPath)

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
	agentOrchestrator *agent.Orchestrator,
	htmxRenderHandler *handlers.HTMXRenderHandler,
	llmClient *llm.Client,
	executor tools.ToolExecutor,
	projectPath string,
) *http.Server {
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
		_, _ = w.Write([]byte(`{"status":"running","version":"` + version.Version + `","agents":["coder","tester","reviewer"]}`))
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
	chatHandler := handlers.NewChatHandler(llmClient, executor, projectPath)

	// Register chat endpoints
	mux.HandleFunc("POST /api/chat/message", chatHandler.SendMessage)
	mux.HandleFunc("GET /api/chat/history", chatHandler.GetHistory)

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
		Addr:         ":8080",
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
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

// waitForShutdown blocks until a SIGINT or SIGTERM signal is received.
func waitForShutdown() os.Signal {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigChan
	signal.Stop(sigChan)
	return sig
}
