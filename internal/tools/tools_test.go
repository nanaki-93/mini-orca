package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ============================================================================
// Common Test Helpers
// ============================================================================

// createTempDir creates a temporary directory for tests
func createTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "tools-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	return dir
}

// cleanupTempDir removes a temporary directory
func cleanupTempDir(t *testing.T, dir string) {
	t.Helper()
	if err := os.RemoveAll(dir); err != nil {
		t.Logf("failed to cleanup temp dir: %v", err)
	}
}

// createTestFile creates a test file with given content and returns its path
func createTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file %s: %v", path, err)
	}
	return path
}

// readTestFile reads a test file and returns its content
func readTestFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read test file %s: %v", path, err)
	}
	return string(data)
}

// assertEqual asserts two values are equal
func assertEqual(t *testing.T, expected, actual interface{}, msg ...string) {
	t.Helper()
	if expected != actual {
		msgStr := ""
		if len(msg) > 0 {
			msgStr = msg[0]
		}
		t.Errorf("expected %v, got %v%s", expected, actual, msgStr)
	}
}

// assertError asserts an error is returned
func assertError(t *testing.T, err error, msg ...string) {
	t.Helper()
	if err == nil {
		msgStr := ""
		if len(msg) > 0 {
			msgStr = msg[0]
		}
		t.Error("expected error, got nil" + msgStr)
	}
}

// assertNoError asserts no error is returned
func assertNoError(t *testing.T, err error, msg ...string) {
	t.Helper()
	if err != nil {
		msgStr := ""
		if len(msg) > 0 {
			msgStr = msg[0]
		}
		t.Errorf("unexpected error: %v%s", err, msgStr)
	}
}

// assertContains asserts that a string contains a substring
func assertContains(t *testing.T, s, substr string, msg ...string) {
	t.Helper()
	if !strings.Contains(s, substr) {
		msgStr := ""
		if len(msg) > 0 {
			msgStr = msg[0]
		}
		t.Errorf("expected %q to contain %q%s", s, substr, msgStr)
	}
}

// assertNotContains asserts that a string does not contain a substring
func assertNotContains(t *testing.T, s, substr string, msg ...string) {
	t.Helper()
	if strings.Contains(s, substr) {
		msgStr := ""
		if len(msg) > 0 {
			msgStr = msg[0]
		}
		t.Errorf("expected %q to not contain %q%s", s, substr, msgStr)
	}
}
