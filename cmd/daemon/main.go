package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/nanaki-93/mini-orca/internal/agent"
	"github.com/nanaki-93/mini-orca/internal/agent/skills"
	"github.com/nanaki-93/mini-orca/internal/api"
	"github.com/nanaki-93/mini-orca/internal/api/handlers"
	"github.com/nanaki-93/mini-orca/internal/config"
	"github.com/nanaki-93/mini-orca/internal/model"
	"github.com/nanaki-93/mini-orca/internal/orchestrator"
	"github.com/nanaki-93/mini-orca/internal/state"
	"github.com/nanaki-93/mini-orca/internal/tools"
)

func main() {
	// Load configuration
	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// Initialize router
	router, err := initRouter(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize router: %v", err)
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

	// Initialize template engine
	templatesPath := "internal/api/templates"
	templateEngine, err := handlers.NewTemplateEngine(templatesPath)
	if err != nil {
		log.Printf("Warning: failed to initialize template engine: %v", err)
		templateEngine = nil
	}

	// Initialize HTMX render handler
	var htmxRenderHandler *handlers.HTMXRenderHandler
	if templateEngine != nil {
		htmxRenderHandler = handlers.NewHTMXRenderHandler(templateEngine, sessionStore, projectStore)
	}

	// Initialize orchestrator with router, registry, and executor
	agentOrchestrator := agent.NewOrchestrator(router, skillsRegistry, executor)

	// Create agents for logging purposes
	planner := agent.NewPlannerAgent(router, skillsRegistry)
	planner.SetSkills(cfg.Agents.Planner.Skills)

	coder := agent.NewCoderAgent(router, skillsRegistry)
	coder.SetSkills(cfg.Agents.Coder.Skills)

	tester := agent.NewTesterAgent(router, skillsRegistry)
	tester.SetSkills(cfg.Agents.Tester.Skills)

	reviewer := agent.NewReviewerAgent(router, skillsRegistry)
	reviewer.SetSkills(cfg.Agents.Reviewer.Skills)

	// Log successful startup
	log.Println("Mini-Orca daemon started successfully")
	log.Printf("Active provider: %s", cfg.Models.ActiveProvider)
	log.Printf("Registered providers: %v", router.ListProviders())
	log.Printf("Configured phases: %d", len(cfg.Models.Phases))

	// Log project type and executor
	if projectInfo != nil {
		log.Printf("Detected project type: %s (root: %s)", projectInfo.Type, projectInfo.RootDir)
	} else {
		log.Println("Project type: generic (unknown) - using shell-only executor")
	}
	if executor != nil {
		log.Printf("Tool executor initialized successfully")
	}
	if agentOrchestrator != nil {
		log.Println("Orchestrator initialized with executor")
	}

	// Log registered agents and their skills
	log.Printf("Agent: %s (%s) — skills: %v", planner.Name(), planner.Description(), planner.GetSkills())
	log.Printf("Agent: %s (%s) — skills: %v", coder.Name(), coder.Description(), coder.GetSkills())
	log.Printf("Agent: %s (%s) — skills: %v", tester.Name(), tester.Description(), tester.GetSkills())
	log.Printf("Agent: %s (%s) — skills: %v", reviewer.Name(), reviewer.Description(), reviewer.GetSkills())

	// Log registered agents from registry
	log.Printf("Registered agents: %v", agentRegistry.List())

	// List available models
	listClient := agent.NewClient(router)
	models, err := listClient.ListModels()
	if err != nil {
		log.Printf("Warning: failed to list models: %v", err)
	} else {
		log.Printf("Available models: %d", len(models))
		for _, m := range models {
			log.Printf("  - %s (owned by: %s)", m.ID, m.OwnedBy)
		}
	}

	// Start HTTP server with all API endpoints
	server := startHTTPServer(agentOrchestrator, store, sessionStore, gateStore, projectStore, htmxRenderHandler, cfg.API)

	// Wait for shutdown signal
	quit := waitForShutdown()
	log.Printf("Received signal %v, shutting down...", quit)

	// Shutdown HTTP server
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	log.Println("Shutdown complete")
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
			log.Printf("Config file %s not found, using defaults", configPath)
			return config.Default(), nil
		}
		return nil, err
	}

	log.Printf("Loaded config from %s", configPath)
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
		log.Printf("Registered provider: %s (base_url: %s)", name, providerCfg.BaseURL)
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
			log.Printf("Warning: failed to register knowledge skill %q: %v", name, err)
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
			log.Printf("Warning: failed to register tool skill %q: %v", name, err)
		}
	}

	totalSkills := len(cfg.Skills.Knowledge) + len(cfg.Skills.Tools)
	log.Printf("Skills registry initialized with %d skills from config", totalSkills)

	return registry
}

// initToolExecutor detects the project type at the current directory and creates
// the appropriate ToolExecutor for the detected project type.
func initToolExecutor() (*tools.ProjectInfo, tools.ToolExecutor) {
	// Detect project type starting from current directory
	detector := tools.NewProjectDetectorExecutor()
	currentDir, err := os.Getwd()
	if err != nil {
		log.Printf("Warning: failed to get current directory: %v", err)
		currentDir = "."
	}

	projectInfo, err := detector.DetectProjectType(currentDir)
	if err != nil {
		// If project type detection fails, try parent directories
		log.Printf("Warning: failed to detect project type in %s: %v", currentDir, err)
		projectInfo, err = detector.DetectProjectType(filepath.Dir(currentDir))
		if err != nil {
			log.Printf("Warning: failed to detect project type in parent directory: %v", err)
			log.Println("Using generic executor (shell-only)")
			return nil, tools.NewExecutor(&tools.ProjectInfo{Type: tools.ProjectTypeUnknown, RootDir: currentDir})
		}
	}

	log.Printf("Detected project type: %s", projectInfo.Type)

	// Create ToolExecutor based on detected project type
	executor := tools.NewExecutor(projectInfo)
	return projectInfo, executor
}

// initAgentRegistry creates an agent registry and registers all agents that implement the Agent interface.
func initAgentRegistry(router *model.Router) *agent.Registry {
	registry := agent.NewRegistry()

	// Create and register planner agent
	planner := agent.NewPlannerAgent(router, nil)
	if err := registry.Register(planner.Name(), planner); err != nil {
		log.Printf("Warning: failed to register planner agent: %v", err)
	}

	// Create and register coder agent
	coder := agent.NewCoderAgent(router, nil)
	if err := registry.Register(coder.Name(), coder); err != nil {
		log.Printf("Warning: failed to register coder agent: %v", err)
	}

	log.Printf("Agent registry initialized with %d agents", len(registry.List()))
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
		CurrentPhase:  state.PhasePlanning,
		Status:        state.SessionStatusPending,
		Plan:          nil,
		AtomicUnits:   nil,
		History:       nil,
		TestResults:   nil,
		ReviewReports: nil,
		Error:         "",
	}

	store := state.NewStore(session)
	log.Printf("State store initialized for session: %s", session.ID)
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
	apiConfig config.APIConfig,
) *http.Server {
	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	// Status endpoint
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"running","agents":["planner","coder","tester","reviewer"]}`))
	})

	// Create phase router for session lifecycle
	currentSession := &state.Session{
		ID:           "default",
		CurrentPhase: state.PhasePlanning,
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
		sessionHandler.GetSessionStatus(w, r)
	})
	mux.HandleFunc("POST /api/sessions/", func(w http.ResponseWriter, r *http.Request) {
		// Route to appropriate lifecycle handler
		parts := splitPath(r.URL.Path)
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

	// Register gate status route
	mux.HandleFunc("GET /api/sessions/", func(w http.ResponseWriter, r *http.Request) {
		if len(splitPath(r.URL.Path)) >= 5 && splitPath(r.URL.Path)[4] == "gate" {
			gateHandler.GetGateStatus(w, r)
		} else {
			sessionHandler.GetSessionStatus(w, r)
		}
	})

	// Register project routes
	projectHandler := handlers.NewProjectHandler(projectStore)
	mux.HandleFunc("GET /api/projects", projectHandler.ListProjects)
	mux.HandleFunc("POST /api/projects", projectHandler.CreateProject)
	mux.HandleFunc("GET /api/projects/", func(w http.ResponseWriter, r *http.Request) {
		// Route to appropriate project handler
		parts := splitPath(r.URL.Path)
		if len(parts) >= 5 {
			action := parts[4]
			switch action {
			case "files":
				projectHandler.ListFiles(w, r)
			}
		}
	})
	mux.HandleFunc("GET /api/projects//files/", func(w http.ResponseWriter, r *http.Request) {
		projectHandler.GetFileContent(w, r)
	})

	// Register HTMX render endpoints
	if htmxRenderHandler != nil {
		mux.HandleFunc("GET /api/render/phase/", htmxRenderHandler.RenderPhase)
		mux.HandleFunc("GET /api/render/file-tree", htmxRenderHandler.RenderFileTree)
		mux.HandleFunc("GET /api/render/activity-log", htmxRenderHandler.RenderActivityLog)
		mux.HandleFunc("GET /api/render/phase-tracker", htmxRenderHandler.RenderPhaseTracker)
	}

	// Wrap with version middleware for backward compatibility
	versionMiddleware := api.NewVersionMiddleware(apiConfig, log.Printf)
	server := &http.Server{
		Addr:         ":8080",
		Handler:      versionMiddleware.Next(mux),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("HTTP server starting on %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
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
	<-sigChan
	sig := <-sigChan
	signal.Stop(sigChan)
	return sig
}

// splitPath splits a URL path into its components.
func splitPath(path string) []string {
	if path == "/" {
		return []string{""}
	}
	path = cleanPath(path)
	if path[0] == '/' {
		path = path[1:]
	}
	if path == "" {
		return []string{""}
	}
	return split(path, '/')
}

// cleanPath removes redundant slashes from the path.
func cleanPath(path string) string {
	if path == "" {
		return "/"
	}
	if path[0] != '/' {
		path = "/" + path
	}
	n := len(path)
	for i := 1; i < n-1; {
		if path[i] == '/' && path[i+1] == '/' {
			path = path[:i+1] + path[i+2:]
			n--
		} else {
			i++
		}
	}
	return path
}

// split splits a string by a separator into a slice of substrings.
func split(s string, sep rune) []string {
	var result []string
	var current []rune
	for _, r := range s {
		if r == sep {
			result = append(result, string(current))
			current = nil
		} else {
			current = append(current, r)
		}
	}
	result = append(result, string(current))
	return result
}
