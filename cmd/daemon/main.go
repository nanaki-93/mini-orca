package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/nanaki-93/mini-orca/internal/agent"
	"github.com/nanaki-93/mini-orca/internal/config"
	"github.com/nanaki-93/mini-orca/internal/model"
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

	// Initialize agent client with router
	client := agent.NewClient(router)

	// Log successful startup
	log.Println("Mini-Orca daemon started successfully")
	log.Printf("Active provider: %s", cfg.Models.ActiveProvider)
	log.Printf("Registered providers: %v", router.ListProviders())
	log.Printf("Configured phases: %d", len(cfg.Models.Phases))

	// List available models
	models, err := client.ListModels()
	if err != nil {
		log.Printf("Warning: failed to list models: %v", err)
	} else {
		log.Printf("Available models: %d", len(models))
		for _, m := range models {
			log.Printf("  - %s (owned by: %s)", m.ID, m.OwnedBy)
		}
	}

	// Wait for shutdown signal
	quit := waitForShutdown()
	log.Printf("Received signal %v, shutting down...", quit)
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

// waitForShutdown blocks until a SIGINT or SIGTERM signal is received.
func waitForShutdown() os.Signal {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	sig := <-sigChan
	signal.Stop(sigChan)
	return sig
}
