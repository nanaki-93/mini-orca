package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigExampleUsesTheStrictScopedSchema(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "config.example.yaml"))
	if err != nil {
		t.Fatalf("read config example: %v", err)
	}
	path := filepath.Join(t.TempDir(), "config.example.yaml")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFromYAML(path); err != nil {
		t.Fatalf("LoadFromYAML(config.example.yaml) error = %v", err)
	}
}
