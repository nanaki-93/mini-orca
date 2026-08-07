package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_FullFlow(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Write a minimal config with LLM settings
	data := []byte(`
llm:
  base_url: "http://localhost:1234"
  model: "code-model"
  temperature: 0.7
  max_tokens: 4096
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

	if cfg.LLM.BaseURL != "http://localhost:1234" {
		t.Errorf("expected base_url http://localhost:1234, got %s", cfg.LLM.BaseURL)
	}

	if cfg.LLM.Model != "code-model" {
		t.Errorf("expected model code-model, got %s", cfg.LLM.Model)
	}

	if cfg.LLM.Temperature != 0.7 {
		t.Errorf("expected temperature 0.7, got %f", cfg.LLM.Temperature)
	}

	if cfg.LLM.MaxTokens != 4096 {
		t.Errorf("expected max_tokens 4096, got %d", cfg.LLM.MaxTokens)
	}
}

func TestDefault_FullFlow(t *testing.T) {
	cfg := Default()

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Default config failed validation: %v", err)
	}

	if cfg.LLM.BaseURL != DefaultProviderURL {
		t.Errorf("expected base_url %s, got %s", DefaultProviderURL, cfg.LLM.BaseURL)
	}

	if len(cfg.Agents.Coder.Skills) == 0 {
		t.Error("expected coder skills")
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

	if loaded.LLM.BaseURL != original.LLM.BaseURL {
		t.Errorf("base_url mismatch: got %s, want %s",
			loaded.LLM.BaseURL, original.LLM.BaseURL)
	}
}

// TestConfig_LLM verifies config accepts LLM settings
func TestConfig_LLM(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	data := []byte(`
llm:
  base_url: "http://localhost:1234"
  model: "gpt-4"
  temperature: 0.5
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

	if cfg.LLM.BaseURL != "http://localhost:1234" {
		t.Errorf("expected base_url http://localhost:1234, got %s", cfg.LLM.BaseURL)
	}
	if cfg.LLM.Model != "gpt-4" {
		t.Errorf("expected model gpt-4, got %s", cfg.LLM.Model)
	}
	if cfg.LLM.Temperature != 0.5 {
		t.Errorf("expected temperature 0.5, got %f", cfg.LLM.Temperature)
	}
	if cfg.LLM.MaxTokens != 2048 {
		t.Errorf("expected max_tokens 2048, got %d", cfg.LLM.MaxTokens)
	}
}
