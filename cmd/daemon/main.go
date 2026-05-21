package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nanaki-93/mini-orca/internal/agent"
	"github.com/nanaki-93/mini-orca/internal/api"
	"github.com/nanaki-93/mini-orca/internal/operation"
	"github.com/nanaki-93/mini-orca/internal/state"
	"github.com/nanaki-93/mini-orca/internal/tools"
)

func main() {
	log.Println("Initializing Step-by-Step Agent Mini-Orca Daemon...")

	// 1. Initialize shared core infrastructure
	// Set up local SQLite or file-based tracking for persistence
	stateStore := state.NewFileStore("~/.local/share/mini-orca/db.json")

	// Create wrappers for Go, Terraform, and Kubernetes client-go
	toolRegistry := tools.NewRegistry()

	// Set up client pointing to LM Studio at localhost:1234
	lmStudioClient := agent.NewClient("http://127.0.0.1:1234/v1")

	// 2. Create the State Engine and synchronization channels
	// Channels act as the "blocking gates" that wait for your UI confirmations
	approvalChan := make(chan bool)
	stateMachine := operation.NewStateMachine(stateStore, lmStudioClient, toolRegistry, approvalChan)

	// 3. Start the State Machine Loop in a background thread (Goroutine)
	ctx, cancelStateMachine := context.WithCancel(context.Background())
	defer cancelStateMachine()

	go func() {
		log.Println("State Engine Loop activated.")
		stateMachine.StartLoop(ctx)
	}()

	// 4. Initialize and start the HTTP API Server
	// We pass the stateMachine instance and channels so HTTP handlers can interact with it
	router := api.NewRouter(stateMachine, approvalChan)
	httpServer := &http.Server{
		Addr:         "127.0.0.1:8080",
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	go func() {
		log.Printf("API Server listening on %s", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("API Server crash: %v", err)
		}
	}()

	// 5. Graceful Shutdown Management
	// Ensure that hitting Ctrl+C stops everything cleanly without corrupting local files
	stopSignal := make(chan os.Signal, 1)
	signal.Notify(stopSignal, os.Interrupt, syscall.SIGTERM)

	<-stopSignal // Execution blocks here until you trigger an OS kill signal
	log.Println("Shutdown signal received. Wrapping up tasks...")

	// Give active network requests or running local tool processes 5 seconds to finish up safely
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("Error shutting down HTTP server: %v", err)
	}

	log.Println("Daemon cleanly terminated. Goodbye.")
}
