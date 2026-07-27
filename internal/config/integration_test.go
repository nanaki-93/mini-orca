package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_FullFlow(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Write a minimal config
	data := []byte(`
models:
  active_provider: "lm-studio"
  providers:
    lm-studio:
      base_url: "http://localhost:1234"
  phases:
    planning:
      provider: "lm-studio"
      model: "test-model"
      temperature: 0.5
      max_tokens: 1024
    coding:
      provider: "lm-studio"
      model: "code-model"
      temperature: 0.7
      max_tokens: 2048
`)
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	if cfg.Models.ActiveProvider != "lm-studio" {
		t.Errorf("expected active_provider lm-studio, got %s", cfg.Models.ActiveProvider)
	}

	if len(cfg.Models.Providers) != 1 {
		t.Errorf("expected 1 provider, got %d", len(cfg.Models.Providers))
	}

	if len(cfg.Models.Phases) != 2 {
		t.Errorf("expected 2 phases, got %d", len(cfg.Models.Phases))
	}

	planning := cfg.Models.Phases["planning"]
	if planning.Model != "test-model" {
		t.Errorf("expected planning model test-model, got %s", planning.Model)
	}
	if planning.Temperature != 0.5 {
		t.Errorf("expected planning temperature 0.5, got %f", planning.Temperature)
	}
}

func TestDefault_FullFlow(t *testing.T) {
	cfg := Default()

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Default config failed validation: %v", err)
	}

	if cfg.Models.ActiveProvider != "lm-studio" {
		t.Errorf("expected lm-studio, got %s", cfg.Models.ActiveProvider)
	}

	if len(cfg.Models.Phases) != 5 {
		t.Errorf("expected 5 phases, got %d", len(cfg.Models.Phases))
	}

	if len(cfg.Models.Providers) != 1 {
		t.Errorf("expected 1 provider, got %d", len(cfg.Models.Providers))
	}

	if len(cfg.Agents.Planner.Skills) == 0 {
		t.Error("expected planner to have skills")
	}

	if len(cfg.Skills.Knowledge) == 0 {
		t.Error("expected knowledge entries")
	}
}

func TestSaveAndReload_KeepData(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.yaml")

	original := Default()
	if err := original.Save(path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if loaded.Models.ActiveProvider != original.Models.ActiveProvider {
		t.Errorf("active_provider mismatch: got %s, want %s",
			loaded.Models.ActiveProvider, original.Models.ActiveProvider)
	}

	if len(loaded.Models.Phases) != len(original.Models.Phases) {
		t.Errorf("phases count mismatch: got %d, want %d",
			len(loaded.Models.Phases), len(original.Models.Phases))
	}
}
