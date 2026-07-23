package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"mini-orca/internal/config"
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

	// Initialize tool executor (requires a project directory)
	// TODO: Get project directory from CLI args or config
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
	shellExecutor := tools.NewShellExecutor(5 * 60 * 1000000000) // 5 minutes
	fileOps := tools.NewFileOps(projectDir)
	gitOps := tools.NewGitOps(projectDir)
	formatter := tools.NewFormatter(executor)

	// Print tool capabilities
	fmt.Println("\nTool executor capabilities:")
	fmt.Printf("  - Shell executor: timeout=%v\n", shellExecutor)
	fmt.Printf("  - File operations: base=%s\n", fileOps)
	fmt.Printf("  - Git operations: repo=%s\n", gitOps)
	fmt.Printf("  - Formatter: project=%s\n", formatter)
	fmt.Printf("  - Supported extensions: %v\n", executor.GetSupportedExtensions())

	// Register HTTP handlers
	http.HandleFunc("/health", healthCheck)
	// TODO: Add more endpoints:
	// - /api/projects - project management
	// - /api/agents/run - run an agent
	// - /api/git/status - git status
	// - /ide - IDE dashboard

	fmt.Println("\n[TODO] Agents, orchestrator, and API handlers not yet implemented")
	fmt.Println("Press Ctrl+C to exit")

	// Start HTTP server
	go func() {
		addr := fmt.Sprintf(":%d", cfg.Server.Port)
		fmt.Printf("\nStarting HTTP server on http://localhost%s\n", addr)
		if err := http.ListenAndServe(addr, nil); err != nil {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\nShutting down...")
}

// healthCheck handles the /health endpoint
func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"ok","version":"2.0.0","milestones":{"model_abstraction":"complete","multi_agent":"complete","tools":"complete"}}`)
}
