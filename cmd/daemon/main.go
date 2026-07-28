package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/nanaki-93/mini-orca/internal/agent"
	"github.com/nanaki-93/mini-orca/internal/agent/skills"
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

	// Initialize skills registry and register config skills
	registry := initSkillsRegistry(cfg)

	// Initialize all agents with their skill sets
	planner := agent.NewPlannerAgent(router, registry)
	planner.SetSkills(cfg.Agents.Planner.Skills)

	coder := agent.NewCoderAgent(router, registry)
	coder.SetSkills(cfg.Agents.Coder.Skills)

	tester := agent.NewTesterAgent(router, registry)
	tester.SetSkills(cfg.Agents.Tester.Skills)

	reviewer := agent.NewReviewerAgent(router, registry)
	reviewer.SetSkills(cfg.Agents.Reviewer.Skills)

	// Log successful startup
	log.Println("Mini-Orca daemon started successfully")
	log.Printf("Active provider: %s", cfg.Models.ActiveProvider)
	log.Printf("Registered providers: %v", router.ListProviders())
	log.Printf("Configured phases: %d", len(cfg.Models.Phases))

	// Log registered agents and their skills
	log.Printf("Agent: %s (%s) — skills: %v", planner.Name(), planner.Description(), planner.GetSkills())
	log.Printf("Agent: %s (%s) — skills: %v", coder.Name(), coder.Description(), coder.GetSkills())
	log.Printf("Agent: %s (%s) — skills: %v", tester.Name(), tester.Description(), tester.GetSkills())
	log.Printf("Agent: %s (%s) — skills: %v", reviewer.Name(), reviewer.Description(), reviewer.GetSkills())

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

// waitForShutdown blocks until a SIGINT or SIGTERM signal is received.
func waitForShutdown() os.Signal {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	sig := <-sigChan
	signal.Stop(sigChan)
	return sig
}
