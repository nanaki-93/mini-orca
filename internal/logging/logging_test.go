package logging

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestLogging(t *testing.T) {
	var buf bytes.Buffer
	cfg := Config{
		Level:         "debug",
		Format:        "json",
		Output:        &buf,
		SensitiveKeys: []string{"password"},
	}

	Init(cfg)

	Info("info message", "password", "secret123")
	Warn("warn message")
	Error("error message")

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 3 {
		t.Errorf("expected 3 log lines, got %d", len(lines))
	}

	var infoEntry map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &infoEntry); err != nil {
		t.Fatal(err)
	}
	if infoEntry["password"] != "[REDACTED]" {
		t.Errorf("expected password to be redacted, got %v", infoEntry["password"])
	}
}

func TestDefaultSensitiveKeys(t *testing.T) {
	var buf bytes.Buffer
	cfg := Config{
		Level:  "info",
		Format: "json",
		Output: &buf,
	}

	Init(cfg)

	Info("info message", "api_key", "super-secret")

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	if entry["api_key"] != "[REDACTED]" {
		t.Errorf("expected api_key to be redacted by default, got %v", entry["api_key"])
	}
}
