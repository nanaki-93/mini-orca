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
    coding:
      provider: "lm-studio"
      model: "code-model"
      temperature: 0.7
      max_tokens: 2048
`)
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFromYAML(configPath)
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

	if len(cfg.Models.Phases) != 1 {
		t.Errorf("expected 1 phase, got %d", len(cfg.Models.Phases))
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

	if len(cfg.Models.Phases) != 4 {
		t.Errorf("expected 4 phases, got %d", len(cfg.Models.Phases))
	}

	if len(cfg.Models.Providers) != 1 {
		t.Errorf("expected 1 provider, got %d", len(cfg.Models.Providers))
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

	loaded, err := LoadFromYAML(path)
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
