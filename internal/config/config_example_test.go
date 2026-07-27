package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nanaki-93/mini-orca/internal/config"
)

func TestLoadConfigExample(t *testing.T) {
	tmpDir := t.TempDir()
	examplePath := filepath.Join(tmpDir, "config.example.yaml")

	// Read the actual example file
	data, err := os.ReadFile("../../config.example.yaml")
	if err != nil {
		t.Fatalf("read example file: %v", err)
	}
	if err := os.WriteFile(examplePath, data, 0644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	cfg, err := config.LoadConfig(examplePath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	if len(cfg.Models.Providers) != 1 {
		t.Errorf("expected 1 provider, got %d", len(cfg.Models.Providers))
	}

	if len(cfg.Models.Phases) != 5 {
		t.Errorf("expected 5 phases, got %d", len(cfg.Models.Phases))
	}

	if len(cfg.Skills.Knowledge) != 12 {
		t.Errorf("expected 12 knowledge entries, got %d", len(cfg.Skills.Knowledge))
	}

	if len(cfg.Skills.Tools) != 3 {
		t.Errorf("expected 3 tools, got %d", len(cfg.Skills.Tools))
	}

	if cfg.Models.ActiveProvider != "lm-studio" {
		t.Errorf("expected active_provider lm-studio, got %s", cfg.Models.ActiveProvider)
	}

	// Check a specific phase config
	planning := cfg.Models.Phases["planning"]
	if planning.Temperature != 0.3 {
		t.Errorf("expected planning temperature 0.3, got %f", planning.Temperature)
	}

	// Check agent skills
	if len(cfg.Agents.Planner.Skills) != 3 {
		t.Errorf("expected planner 3 skills, got %d", len(cfg.Agents.Planner.Skills))
	}
}

func TestLoadConfigNotExists(t *testing.T) {
	_, err := config.LoadConfig("nonexistent.yaml")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestLoadConfigInvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	tmpPath := filepath.Join(tmpDir, "invalid.yaml")
	if err := os.WriteFile(tmpPath, []byte("{{invalid yaml:::"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := config.LoadConfig(tmpPath)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestSaveAndReload(t *testing.T) {
	tmpDir := t.TempDir()
	tmpPath := filepath.Join(tmpDir, "config.yaml")

	defaultCfg := config.Default()
	if err := defaultCfg.Save(tmpPath); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := config.LoadConfig(tmpPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if loaded.Models.ActiveProvider != defaultCfg.Models.ActiveProvider {
		t.Errorf("active_provider mismatch: got %s, want %s",
			loaded.Models.ActiveProvider, defaultCfg.Models.ActiveProvider)
	}

	if len(loaded.Models.Phases) != len(defaultCfg.Models.Phases) {
		t.Errorf("phases count mismatch: got %d, want %d",
			len(loaded.Models.Phases), len(defaultCfg.Models.Phases))
	}
}
