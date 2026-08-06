package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	content := `
llm:
  base_url: "http://localhost:1234"
  model: "gpt-4"
  temperature: 0.7
  max_tokens: 4096
`
	tmpDir, err := os.MkdirTemp("", "config-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("Valid YAML", func(t *testing.T) {
		cfg, err := LoadFromYAML(configPath)
		if err != nil {
			t.Fatalf("failed to load config: %v", err)
		}

		if cfg.LLM.BaseURL != "http://localhost:1234" {
			t.Errorf("expected base_url http://localhost:1234, got %s", cfg.LLM.BaseURL)
		}

		if cfg.LLM.Model != "gpt-4" {
			t.Errorf("expected model gpt-4, got %s", cfg.LLM.Model)
		}

		// Test defaults
		if cfg.Retry.MaxRetries != 3 {
			t.Errorf("expected default max retries 3, got %d", cfg.Retry.MaxRetries)
		}
		if cfg.Logging.Level != "info" {
			t.Errorf("expected default log level info, got %s", cfg.Logging.Level)
		}
	})

	t.Run("Non-existent file", func(t *testing.T) {
		_, err := LoadFromYAML("non-existent.yaml")
		if err == nil {
			t.Error("expected error for non-existent file")
		}
	})

	t.Run("Invalid YAML", func(t *testing.T) {
		invalidPath := filepath.Join(tmpDir, "invalid.yaml")
		os.WriteFile(invalidPath, []byte("invalid: yaml: :"), 0644)
		_, err := LoadFromYAML(invalidPath)
		if err == nil {
			t.Error("expected error for invalid YAML")
		}
	})
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "Valid config",
			config: Config{
				LLM: LLMConfig{
					BaseURL: "http://localhost",
				},
			},
			wantErr: false,
		},
		{
			name: "Missing base_url",
			config: Config{
				LLM: LLMConfig{
					BaseURL: "",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSaveLoadJSON(t *testing.T) {
	cfg := &Config{
		LLM: LLMConfig{
			BaseURL: "http://localhost",
			Model:   "gpt-4",
		},
	}

	tmpDir, _ := os.MkdirTemp("", "config-json-test")
	defer os.RemoveAll(tmpDir)
	path := filepath.Join(tmpDir, "config.json")

	if err := cfg.Save(path); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	loaded, err := LoadFromJSON(path)
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}

	if loaded.LLM.BaseURL != "http://localhost" {
		t.Errorf("expected http://localhost, got %s", loaded.LLM.BaseURL)
	}
	if loaded.LLM.Model != "gpt-4" {
		t.Errorf("expected gpt-4, got %s", loaded.LLM.Model)
	}
}
