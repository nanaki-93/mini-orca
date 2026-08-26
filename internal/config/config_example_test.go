package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/config"
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

	cfg, err := config.LoadFromYAML(examplePath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	if cfg.LLM.BaseURL != "http://localhost:1234" {
		t.Errorf("expected base_url http://localhost:1234, got %s", cfg.LLM.BaseURL)
	}

	if len(cfg.Agents.Coder.Skills) != 3 {
		t.Errorf("expected 3 coder skills, got %d", len(cfg.Agents.Coder.Skills))
	}
}

func TestRepositoryConfigurationPolicy(t *testing.T) {
	repositoryRoot := filepath.Join("..", "..")
	ignoreFile, err := os.ReadFile(filepath.Join(repositoryRoot, ".gitignore"))
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	if !containsIgnoreRule(string(ignoreFile), "config.yaml") {
		t.Fatal(".gitignore must ignore local config.yaml")
	}

	example, err := config.LoadFromYAML(filepath.Join(repositoryRoot, "config.example.yaml"))
	if err != nil {
		t.Fatalf("load config example: %v", err)
	}
	if err := example.Validate(); err != nil {
		t.Fatalf("validate config example: %v", err)
	}
}

func containsIgnoreRule(contents, want string) bool {
	for _, line := range strings.Split(contents, "\n") {
		if strings.TrimSpace(line) == want {
			return true
		}
	}
	return false
}

func TestLoadConfigNotExists(t *testing.T) {
	_, err := config.LoadFromYAML("nonexistent.yaml")
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

	_, err := config.LoadFromYAML(tmpPath)
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

	loaded, err := config.LoadFromYAML(tmpPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if loaded.LLM.BaseURL != defaultCfg.LLM.BaseURL {
		t.Errorf("base_url mismatch: got %s, want %s",
			loaded.LLM.BaseURL, defaultCfg.LLM.BaseURL)
	}
}
