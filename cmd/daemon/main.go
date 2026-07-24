package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"mini-orca/internal/agent"
	"mini-orca/internal/agent/skills"
	"mini-orca/internal/api/handlers"
	"mini-orca/internal/api/templates"
	"mini-orca/internal/config"
	"mini-orca/internal/orchestrator"
	"mini-orca/internal/state"
	"mini-orca/internal/tools"
)

func main() {
	// Load configuration
	cfg, err := config.Load("")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	fmt.Println("=== Mini-Orca v2.0 ===")
	fmt.Printf("Server will start on port %d\n", cfg.Server.Port)

	// Build model router
	router, err := cfg.BuildRouter()
	if err != nil {
		log.Fatalf("Failed to build model router: %v", err)
	}

	// Verify LM Studio provider is registered
	lmProvider := router.GetConfig().Providers["lm-studio"]
	fmt.Printf("LM Studio provider configured at: %s\n", lmProvider.BaseURL)

	// List available phases
	fmt.Println("\nConfigured phases:")
	for phase, pc := range cfg.Models.Phases {
		fmt.Printf("  - %s: model=%s, provider=%s, temp=%.1f\n",
			phase, pc.Model, pc.Provider, pc.Temperature)
	}

	// Initialize tool executor
	projectDir := "."
	executor, err := tools.NewExecutor(projectDir)
	if err != nil {
		fmt.Printf("Note: Could not auto-detect project type in '%s': %v\n", projectDir, err)
		fmt.Println("      Tool executor will be initialized when a project is loaded.")
		executor = tools.NewExecutorWithProjectType(projectDir, tools.ProjectTypeUnknown)
	} else {
		fmt.Printf("\nDetected project type: %s\n", executor.ProjectType())
	}

	// Initialize tools
	fileOps := tools.NewFileOps(projectDir)
	gitOps := tools.NewGitOps(projectDir)
	formatter := tools.NewFormatter(executor)

	// Initialize agents
	agentRegistry := agent.NewRegistry()
	agentConfig := map[string][]string{
		"planner":  cfg.Agents.Planner.Skills,
		"coder":    cfg.Agents.Coder.Skills,
		"tester":   cfg.Agents.Tester.Skills,
		"reviewer": cfg.Agents.Reviewer.Skills,
	}
	agent.InitializeAgentsWithRouter(agentRegistry, agentConfig, router)

	fmt.Printf("\nRegistered agents: %v\n", agentRegistry.List())

	// Initialize skills
	skillRegistry := skills.NewRegistry()
	allSkills := skillRegistry.MergeWithLibrary()
	for _, s := range allSkills {
		skillRegistry.Add(s)
	}

	// Initialize state store
	stateStore, err := state.NewStore(".mini-orca/state")
	if err != nil {
		log.Fatalf("Failed to create state store: %v", err)
	}

	// Create a new session
	session := state.NewSession(
		"",
		"My Project",
		"Example project for Mini-Orca",
		projectDir,
	)

	// Initialize orchestrator
	orch := orchestrator.NewOrchestrator(
		session,
		router,
		agentRegistry,
		executor,
		fileOps,
		gitOps,
		formatter,
		stateStore,
		cfg,
	)

	// Load templates
	ts, err := templates.LoadTemplates()
	if err != nil {
		log.Fatalf("Failed to load templates: %v", err)
	}

	// Create handlers
	projectHandler := handlers.NewProjectHandler(orch, stateStore)
	approveHandler := handlers.NewApproveHandler(orch)
	configHandler := handlers.NewConfigHandler(cfg, router)
	insertionHandler := handlers.NewInsertionHandler(orch, fileOps)
	skillsHandler := handlers.NewSkillsHandler(skillRegistry)

	// Register HTTP handlers
	mux := http.NewServeMux()

	// Static files
	mux.Handle("/css/", http.StripPrefix("/css/", http.FileServer(http.FS(templates.StaticFS))))
	mux.Handle("/js/", http.StripPrefix("/js/", http.FileServer(http.FS(templates.StaticFS))))

	// Templates
	mux.HandleFunc("/ide", func(w http.ResponseWriter, r *http.Request) {
		ts.IDE.Execute(w, nil)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/ide", http.StatusFound)
	})

	// API endpoints
	mux.HandleFunc("/api/session", projectHandler.GetSession)
	mux.HandleFunc("/api/sessions", projectHandler.ListSessions)
	mux.HandleFunc("/api/sessions/create", projectHandler.CreateSession)
	mux.HandleFunc("/api/session/phase", projectHandler.UpdatePhase)

	mux.HandleFunc("/api/approve", approveHandler.Approve)
	mux.HandleFunc("/api/reject", approveHandler.Reject)
	mux.HandleFunc("/api/pending-approvals", approveHandler.GetPendingApprovals)

	mux.HandleFunc("/api/config", configHandler.GetConfig)
	mux.HandleFunc("/api/config/update", configHandler.UpdateConfig)
	mux.HandleFunc("/api/models", configHandler.GetAvailableModels)

	mux.HandleFunc("/api/insert", insertionHandler.InsertFunction)
	mux.HandleFunc("/api/file/functions", insertionHandler.GetFileFunctions)
	mux.HandleFunc("/api/insertion-points", insertionHandler.GetInsertionPoints)

	mux.HandleFunc("/api/skills", skillsHandler.ListSkills)
	mux.HandleFunc("/api/skills/create", skillsHandler.CreateSkill)
	mux.HandleFunc("/api/skills/update", skillsHandler.UpdateSkill)
	mux.HandleFunc("/api/skills/delete", skillsHandler.DeleteSkill)
	mux.HandleFunc("/api/skills/export", skillsHandler.ExportSkills)
	mux.HandleFunc("/api/skills/import", skillsHandler.ImportSkills)
	mux.HandleFunc("/api/agent-skills", skillsHandler.GetAgentSkills)
	mux.HandleFunc("/api/agent-skills/assign", skillsHandler.AssignAgentSkill)
	mux.HandleFunc("/api/skills/bulk-assign", skillsHandler.BulkAssignSkills)

	// Template endpoints
	mux.HandleFunc("/api/templates/header", func(w http.ResponseWriter, r *http.Request) {
		t, _ := templates.ParsePartial("components", "header.html")
		t.Execute(w, nil)
	})
	mux.HandleFunc("/api/templates/file-tree", func(w http.ResponseWriter, r *http.Request) {
		t, _ := templates.ParsePartial("components", "file-tree.html")
		t.Execute(w, nil)
	})
	mux.HandleFunc("/api/templates/phase-tracker", func(w http.ResponseWriter, r *http.Request) {
		t, _ := templates.ParsePartial("components", "phase-tracker.html")
		t.Execute(w, nil)
	})
	mux.HandleFunc("/api/templates/activity-log", func(w http.ResponseWriter, r *http.Request) {
		t, _ := templates.ParsePartial("components", "activity-log.html")
		t.Execute(w, nil)
	})
	mux.HandleFunc("/api/templates/main-view", func(w http.ResponseWriter, r *http.Request) {
		// Render phase-specific template based on current session phase
		session := orch.GetSession()
		templateName := fmt.Sprintf("phases/%s.html", session.Phase)
		t, _ := templates.ParsePartial("phases", templateName)
		if t != nil {
			t.Execute(w, map[string]string{
				"PlanPreview":    "Plan preview will appear here",
				"PlanDetails":    "Plan details will appear here",
				"GeneratedCode":  "Generated code will appear here",
				"TestOutput":     "Test output will appear here",
				"ReviewReport":   "Review report will appear here",
			})
		} else {
			fmt.Fprintf(w, `<div class="bg-gray-800 rounded-lg p-6"><h2 class="text-lg font-semibold text-gray-400">Phase: %s</h2><p class="text-gray-300">No template available for this phase.</p></div>`, session.Phase)
		}
	})

	// Health check
	mux.HandleFunc("/health", healthCheck)

	fmt.Println("\n[INFO] Orchestrator initialized and ready")
	fmt.Println("Press Ctrl+C to exit")

	// Start HTTP server
	go func() {
		addr := fmt.Sprintf(":%d", cfg.Server.Port)
		fmt.Printf("\nStarting HTTP server on http://localhost%s\n", addr)
		fmt.Println("Available endpoints:")
		fmt.Println("  - GET  /health")
		fmt.Println("  - GET  /ide")
		fmt.Println("  - GET  /api/session")
		fmt.Println("  - GET  /api/sessions")
		fmt.Println("  - POST /api/sessions/create")
		fmt.Println("  - POST /api/session/phase")
		fmt.Println("  - POST /api/approve")
		fmt.Println("  - POST /api/reject")
		fmt.Println("  - GET  /api/pending-approvals")
		fmt.Println("  - GET  /api/config")
		fmt.Println("  - POST /api/config/update")
		fmt.Println("  - GET  /api/models")
		fmt.Println("  - POST /api/insert")
		fmt.Println("  - GET  /api/file/functions")
		fmt.Println("  - GET  /api/insertion-points")
		fmt.Println("  - GET  /api/skills")
		fmt.Println("  - POST /api/skills/create")
		fmt.Println("  - PUT  /api/skills/update")
		fmt.Println("  - DELETE /api/skills/delete")
		fmt.Println("  - GET  /api/skills/export")
		fmt.Println("  - POST /api/skills/import")
		fmt.Println("  - GET  /api/agent-skills")
		fmt.Println("  - POST /api/agent-skills/assign")
		fmt.Println("  - POST /api/skills/bulk-assign")
		fmt.Println("  - GET  /api/templates/*")
		if err := http.ListenAndServe(addr, mux); err != nil {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\nShutting down...")
	orch.Stop()
	fmt.Println("Orchestrator stopped.")
}

// healthCheck handles the /health endpoint
func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"ok","version":"2.0.0","milestones":{"model_abstraction":"complete","multi_agent":"complete","tools":"complete","orchestrator":"complete","frontend":"complete"}}`)
}
