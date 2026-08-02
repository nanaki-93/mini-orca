package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/agent"
	"github.com/nanaki-93/mini-orca/v2/internal/agent/skills"
	"github.com/nanaki-93/mini-orca/v2/internal/api"
	"github.com/nanaki-93/mini-orca/v2/internal/api/handlers"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/logging"
	"github.com/nanaki-93/mini-orca/v2/internal/model"
	"github.com/nanaki-93/mini-orca/v2/internal/orchestrator"
	"github.com/nanaki-93/mini-orca/v2/internal/state"
	"github.com/nanaki-93/mini-orca/v2/internal/tools"
	"github.com/nanaki-93/mini-orca/v2/internal/version"
)

func main() {
	// Load configuration
	cfg, err := loadConfig()
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

	// Initialize router
	router, err := initRouter(cfg)
	if err != nil {
		logging.Error("Failed to initialize router", "error", err)
		os.Exit(1)
	}

	// Initialize skills registry and register config skills
	skillsRegistry := initSkillsRegistry(cfg)

	// Detect project type and create tool executor
	projectInfo, executor := initToolExecutor()

	// Initialize agent registry and register agents
	agentRegistry := initAgentRegistry(router)

	// Initialize state store
	store := initStateStore(projectInfo)

	// Initialize API stores and handlers
	sessionStore := api.NewSessionStore()
	gateStore := api.NewGateStore()

	// Initialize project store
	projectStore := handlers.NewProjectStore()

	// Auto-create project from state store session
	if store != nil {
		session := store.GetCurrentSession()
		if session != nil && session.ProjectPath != "" {
			project, err := projectStore.CreateProject("", session.ProjectPath, session.ProjectType)
			if err != nil {
				logging.Warn("Failed to auto-create project from state store", "error", err)
			} else {
				logging.Info("Auto-created project from state store", "project_id", project.ID, "project_name", project.Name, "project_path", project.Path)
			}
		}
	}

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
		htmxRenderHandler = handlers.NewHTMXRenderHandler(templateEngine, sessionStore, projectStore, cache)
	}

	// Initialize orchestrator with router, registry, and executor
	agentOrchestrator := agent.NewOrchestrator(router, skillsRegistry, executor)

	// Create agents for logging purposes
	coder := agent.NewCoderAgent(router, skillsRegistry)
	coder.SetSkills(cfg.Agents.Coder.Skills)

	tester := agent.NewTesterAgent(router, skillsRegistry)
	tester.SetSkills(cfg.Agents.Tester.Skills)

	reviewer := agent.NewReviewerAgent(router, skillsRegistry)
	reviewer.SetSkills(cfg.Agents.Reviewer.Skills)

	// Log successful startup
	logging.Info("Mini-Orca daemon started successfully")
	logging.Info("Startup config",
		"active_provider", cfg.Models.ActiveProvider,
		"registered_providers", router.ListProviders(),
		"configured_phases", len(cfg.Models.Phases))

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
	logging.Info("Agent registered", "name", coder.Name(), "description", coder.Name(), "skills", coder.GetSkills())
	logging.Info("Agent registered", "name", tester.Name(), "description", tester.Description(), "skills", tester.GetSkills())
	logging.Info("Agent registered", "name", reviewer.Name(), "description", reviewer.Description(), "skills", reviewer.GetSkills())

	// Log registered agents from registry
	logging.Info("Registered agents from registry", "agents", agentRegistry.List())

	// List available models
	listClient := agent.NewClient(router)
	models, err := listClient.ListModels()
	if err != nil {
		logging.Warn("Failed to list models", "error", err)
	} else {
		logging.Info("Available models", "count", len(models))
		for _, m := range models {
			logging.Debug("Model detail", "id", m.ID, "owned_by", m.OwnedBy)
		}
	}

	// Start HTTP server with all API endpoints
	server := startHTTPServer(agentOrchestrator, store, sessionStore, gateStore, projectStore, htmxRenderHandler)

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

// loadConfig reads configuration from the specified path or uses defaults.
func loadConfig() (*config.Config, error) {
	configPath := "config.yaml"
	if p := os.Getenv("MINI_ORCA_CONFIG"); p != "" {
		configPath = p
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			logging.Info("Config file not found, using defaults", "path", configPath)
			return config.Default(), nil
		}
		return nil, err
	}

	logging.Info("Loaded config", "path", configPath)
	return cfg, nil
}

// initRouter creates and configures the model router from the provided config.
func initRouter(cfg *config.Config) (*model.Router, error) {
	router := model.NewRouter()

	// Register providers from config
	for name, providerCfg := range cfg.Models.Providers {
		provider, err := createProvider(name, providerCfg)
		if err != nil {
			return nil, fmt.Errorf("create provider %q: %w", name, err)
		}
		router.RegisterProvider(provider)
		logging.Info("Registered provider", "name", name, "base_url", providerCfg.BaseURL)
	}

	// Set phase-specific model configurations
	for phase, phaseCfg := range cfg.Models.Phases {
		modelCfg := model.ModelConfig{
			Provider:    phaseCfg.Provider,
			ModelID:     phaseCfg.Model,
			Temperature: phaseCfg.Temperature,
			MaxTokens:   phaseCfg.MaxTokens,
		}
		router.SetDefaultConfig(string(phase), modelCfg)
	}

	// Set active provider
	if cfg.Models.ActiveProvider != "" {
		if err := router.SetActiveProvider(cfg.Models.ActiveProvider); err != nil {
			return nil, fmt.Errorf("set active provider: %w", err)
		}
	}

	return router, nil
}

// createProvider creates a provider instance from its configuration.
func createProvider(name string, cfg config.ProviderConfig) (model.Provider, error) {
	switch name {
	case "lm-studio":
		return model.NewLMStudioProvider(cfg.BaseURL), nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s", name)
	}
}

// initSkillsRegistry creates a skills registry and registers all skills from the config.
func initSkillsRegistry(cfg *config.Config) *skills.SkillsRegistry {
	registry := skills.NewSkillsRegistry()

	// Register knowledge skills from config
	for name, description := range cfg.Skills.Knowledge {
		s := skills.Skill{
			Name:           name,
			Type:           skills.Knowledge,
			PromptTemplate: description,
		}
		if err := registry.Register(s); err != nil {
			logging.Warn("Failed to register knowledge skill", "name", name, "error", err)
		}
	}

	// Register tool skills from config
	for name, description := range cfg.Skills.Tools {
		s := skills.Skill{
			Name:           name,
			Type:           skills.Tool,
			PromptTemplate: description,
		}
		if err := registry.Register(s); err != nil {
			logging.Warn("Failed to register tool skill", "name", name, "error", err)
		}
	}

	totalSkills := len(cfg.Skills.Knowledge) + len(cfg.Skills.Tools)
	logging.Info("Skills registry initialized", "count", totalSkills)

	return registry
}

// initToolExecutor detects the project type at the current directory and creates
// the appropriate ToolExecutor for the detected project type.
func initToolExecutor() (*tools.ProjectInfo, tools.ToolExecutor) {
	// Detect project type starting from current directory
	detector := tools.NewProjectDetectorExecutor()
	currentDir, err := os.Getwd()
	if err != nil {
		logging.Warn("Failed to get current directory", "error", err)
		currentDir = "."
	}

	projectInfo, err := detector.DetectProjectType(currentDir)
	if err != nil {
		// If project type detection fails, try parent directories
		logging.Warn("Failed to detect project type", "dir", currentDir, "error", err)
		projectInfo, err = detector.DetectProjectType(filepath.Dir(currentDir))
		if err != nil {
			logging.Warn("Failed to detect project type in parent directory", "error", err)
			logging.Info("Using generic executor (shell-only)")
			return nil, tools.NewExecutor(&tools.ProjectInfo{Type: tools.ProjectTypeUnknown, RootDir: currentDir})
		}
	}

	logging.Info("Detected project type", "type", projectInfo.Type)

	// Create ToolExecutor based on detected project type
	executor := tools.NewExecutor(projectInfo)
	return projectInfo, executor
}

// initAgentRegistry creates an agent registry and registers all agents that implement the Agent interface.
func initAgentRegistry(router *model.Router) *agent.Registry {
	registry := agent.NewRegistry()

	// Create and register coder agent
	coder := agent.NewCoderAgent(router, nil)
	if err := registry.Register(coder.Name(), coder); err != nil {
		logging.Warn("Failed to register coder agent", "error", err)
	}

	logging.Info("Agent registry initialized", "count", len(registry.List()))
	return registry
}

// initStateStore creates a state store with a new session.
func initStateStore(projectInfo *tools.ProjectInfo) *state.Store {
	currentDir, _ := os.Getwd()
	if currentDir == "" {
		currentDir = "."
	}

	projectType := "unknown"
	if projectInfo != nil {
		projectType = string(projectInfo.Type)
	}

	session := &state.Session{
		ID:            generateSessionID(),
		Goal:          "",
		ProjectPath:   currentDir,
		ProjectType:   projectType,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		CurrentPhase:  state.PhaseCoding,
		Status:        state.SessionStatusPending,
		AtomicUnits:   nil,
		History:       nil,
		TestResults:   nil,
		ReviewReports: nil,
		Error:         "",
	}

	store := state.NewStore(session)
	logging.Info("State store initialized", "session_id", session.ID)
	return store
}

// startHTTPServer creates and starts the HTTP server with all API endpoints.
func startHTTPServer(
	agentOrchestrator *agent.Orchestrator,
	store *state.Store,
	sessionStore *api.SessionStore,
	gateStore *api.GateStore,
	projectStore *handlers.ProjectStore,
	htmxRenderHandler *handlers.HTMXRenderHandler,
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

	// Create phase router for session lifecycle
	currentSession := &state.Session{
		ID:           "default",
		CurrentPhase: state.PhaseCoding,
		Status:       state.SessionStatusPending,
	}
	phaseRouter := orchestrator.NewPhaseRouter(currentSession, nil)

	// Initialize API handlers
	sessionHandler := api.NewSessionHandler(sessionStore, gateStore, phaseRouter)
	gateHandler := api.NewGateHandler(gateStore, phaseRouter)

	// Register session routes
	mux.HandleFunc("POST /api/sessions", sessionHandler.CreateSession)
	mux.HandleFunc("GET /api/sessions", sessionHandler.ListSessions)
	mux.HandleFunc("GET /api/sessions/", func(w http.ResponseWriter, r *http.Request) {
		parts := api.SplitPath(r.URL.Path)
		if len(parts) >= 5 && parts[4] == "gate" {
			gateHandler.GetGateStatus(w, r)
		} else {
			sessionHandler.GetSessionStatus(w, r)
		}
	})
	mux.HandleFunc("POST /api/sessions/", func(w http.ResponseWriter, r *http.Request) {
		// Route to appropriate lifecycle handler
		parts := api.SplitPath(r.URL.Path)
		if len(parts) >= 5 {
			action := parts[4]
			switch action {
			case "start":
				sessionHandler.StartSession(w, r)
			case "pause":
				sessionHandler.PauseSession(w, r)
			case "resume":
				sessionHandler.ResumeSession(w, r)
			case "stop":
				sessionHandler.StopSession(w, r)
			case "gate":
				gateHandler.RespondToGate(w, r)
			}
		}
	})

	// Register project routes
	projectHandler := handlers.NewProjectHandler(projectStore)
	mux.HandleFunc("GET /api/projects", projectHandler.ListProjects)
	mux.HandleFunc("POST /api/projects", projectHandler.CreateProject)
	mux.HandleFunc("GET /api/projects/", func(w http.ResponseWriter, r *http.Request) {
		// Route to appropriate project handler
		parts := api.SplitPath(r.URL.Path)
		if len(parts) >= 5 {
			action := parts[4]
			switch action {
			case "files":
				projectHandler.ListFiles(w, r)
			}
		}
	})
	mux.HandleFunc("GET /api/projects/files/", func(w http.ResponseWriter, r *http.Request) {
		projectHandler.GetFileContent(w, r)
	})

	// System info endpoint
	systemHandler := handlers.NewSystemHandler()
	mux.HandleFunc("GET /api/system/info", systemHandler.GetSystemInfo)

	// Static files
	fs := http.FileServer(http.Dir("internal/api/static"))
	mux.Handle("/static/", http.StripPrefix("/static/", fs))

	// Register HTMX render endpoints
	if htmxRenderHandler != nil {
		mux.HandleFunc("GET /api/render/phase/", htmxRenderHandler.RenderPhase)
		mux.HandleFunc("GET /api/render/file-tree", htmxRenderHandler.RenderFileTree)
		mux.HandleFunc("GET /api/render/dashboard", htmxRenderHandler.RenderDashboard)
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

// generateSessionID generates a simple session ID.
func generateSessionID() string {
	return fmt.Sprintf("session-%d", time.Now().UnixNano())
}

// waitForShutdown blocks until a SIGINT or SIGTERM signal is received.
func waitForShutdown() os.Signal {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigChan
	signal.Stop(sigChan)
	return sig
}
