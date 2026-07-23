package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"mini-orca/internal/agent"
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
	shellExecutor := tools.NewShellExecutor(5 * 60 * time.Second)
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

	// Create insertion manager
	insertionMgr := orchestrator.NewInsertionManager(orch, fileOps, gitOps)
	_ = insertionMgr // TODO: Wire up insertion endpoints

	// Print tool capabilities
	fmt.Println("\nTool executor capabilities:")
	fmt.Printf("  - Shell executor: timeout=%v\n", shellExecutor)
	fmt.Printf("  - File operations: base=%s\n", fileOps)
	fmt.Printf("  - Git operations: repo=%s\n", gitOps)
	fmt.Printf("  - Formatter: project=%s\n", formatter)
	fmt.Printf("  - Supported extensions: %v\n", executor.GetSupportedExtensions())

	// Print session info
	fmt.Printf("\nSession: %s (%s)\n", session.Name, session.ID)
	fmt.Printf("Current phase: %s\n", session.Phase)

	// Register HTTP handlers
	http.HandleFunc("/health", healthCheck)
	http.HandleFunc("/api/session", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"session_id":"%s","phase":"%s","agent_count":%d}`,
			session.ID, session.Phase, len(agentRegistry.List()))
	})
	http.HandleFunc("/api/agents", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"agents":%v}`, agentRegistry.List())
	})
	http.HandleFunc("/api/orchestrator/start", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		go func() {
			ctx := context.Background()
			if err := orch.Start(ctx); err != nil {
				log.Printf("Orchestrator error: %v", err)
			}
		}()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"started","session":"%s"}`, session.ID)
	})
	http.HandleFunc("/api/insert", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		// Placeholder for function insertion
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"placeholder","insertion_manager":"initialized"}`)
	})

	fmt.Println("\n[INFO] Orchestrator initialized and ready")
	fmt.Println("Press Ctrl+C to exit")

	// Start HTTP server
	go func() {
		addr := fmt.Sprintf(":%d", cfg.Server.Port)
		fmt.Printf("\nStarting HTTP server on http://localhost%s\n", addr)
		fmt.Println("Available endpoints:")
		fmt.Println("  - GET  /health")
		fmt.Println("  - GET  /api/session")
		fmt.Println("  - GET  /api/agents")
		fmt.Println("  - POST /api/orchestrator/start")
		fmt.Println("  - POST /api/insert")
		if err := http.ListenAndServe(addr, nil); err != nil {
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
	fmt.Fprintf(w, `{"status":"ok","version":"2.0.0","milestones":{"model_abstraction":"complete","multi_agent":"complete","tools":"complete","orchestrator":"complete"}}`)
}
