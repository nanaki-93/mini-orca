package main

import (
	"fmt"
	"log"
	"net/http"

	"mini-orca/internal/config"
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

	// TODO: Initialize agents, orchestrator, and API handlers
	// TODO: Start HTTP server

	fmt.Println("\n[TODO] Agents, orchestrator, and API handlers not yet implemented")
	fmt.Println("Press Ctrl+C to exit")

	// Keep the process running
	select {}
}

// healthCheck handles the /health endpoint
func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"ok","version":"2.0.0"}`)
}
