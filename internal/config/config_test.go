package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	content := `
models:
  active_provider: "p1"
  providers:
    p1:
      base_url: "http://localhost:1234"
  phases:
    coding:
      provider: "p1"
      model: "gpt-4"
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

		if cfg.Models.ActiveProvider != "p1" {
			t.Errorf("expected active provider p1, got %s", cfg.Models.ActiveProvider)
		}

		if cfg.Models.Phases["coding"].Model != "gpt-4" {
			t.Errorf("expected model gpt-4, got %s", cfg.Models.Phases["coding"].Model)
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
				Models: ModelsConfig{
					ActiveProvider: "p1",
					Providers: map[string]ProviderConfig{
						"p1": {BaseURL: "http://localhost"},
					},
					Phases: map[string]PhaseModelConfig{
						"p1": {Provider: "p1"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Missing active provider",
			config: Config{
				Models: ModelsConfig{
					Providers: map[string]ProviderConfig{
						"p1": {BaseURL: "http://localhost"},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Active provider not in providers",
			config: Config{
				Models: ModelsConfig{
					ActiveProvider: "p2",
					Providers: map[string]ProviderConfig{
						"p1": {BaseURL: "http://localhost"},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Provider missing base_url",
			config: Config{
				Models: ModelsConfig{
					ActiveProvider: "p1",
					Providers: map[string]ProviderConfig{
						"p1": {BaseURL: ""},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "No phases",
			config: Config{
				Models: ModelsConfig{
					ActiveProvider: "p1",
					Providers: map[string]ProviderConfig{
						"p1": {BaseURL: "http://localhost"},
					},
					Phases: nil,
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
		Models: ModelsConfig{
			ActiveProvider: "p1",
			Providers: map[string]ProviderConfig{
				"p1": {BaseURL: "http://localhost"},
			},
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

	if loaded.Models.ActiveProvider != "p1" {
		t.Errorf("expected p1, got %s", loaded.Models.ActiveProvider)
	}
}
