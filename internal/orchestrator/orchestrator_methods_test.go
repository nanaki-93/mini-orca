package orchestrator

import (
	"github.com/nanaki-93/mini-orca/v2/internal/state"
	"os"
	"path/filepath"
	"testing"
)

func TestOrchestrator_DetermineTargetFile(t *testing.T) {
	o := &Orchestrator{}

	unit := &state.PlanUnit{Name: "Task One", File: "task1.go"}
	if o.determineTargetFile(unit) != "task1.go" {
		t.Errorf("expected task1.go, got %s", o.determineTargetFile(unit))
	}

	unit = &state.PlanUnit{Name: "Task Two"}
	if o.determineTargetFile(unit) != "task_two.go" {
		t.Errorf("expected task_two.go, got %s", o.determineTargetFile(unit))
	}
}

func TestExtractCodeBlocks(t *testing.T) {
	output := "Some text\n```go\nfunc main() {}\n```\nOther text\n```python\ndef main(): pass\n```"
	blocks := extractCodeBlocks(output)
	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(blocks))
	}
	if blocks[0].Language != "go" || blocks[0].Content != "func main() {}" {
		t.Errorf("unexpected block 0: %+v", blocks[0])
	}
	if blocks[1].Language != "python" || blocks[1].Content != "def main(): pass" {
		t.Errorf("unexpected block 1: %+v", blocks[1])
	}
}

func TestOrchestrator_ExtractCodeFromResult(t *testing.T) {
	o := &Orchestrator{}
	unit := &state.PlanUnit{Name: "test"}

	output := "```go\ncode\n```"
	code, file, err := o.extractCodeFromResult(output, unit)
	if err != nil || code != "code" || file != "test.go" {
		t.Errorf("unexpected result: %q, %q, %v", code, file, err)
	}
}

func TestOrchestrator_WriteFileAtomic(t *testing.T) {
	o := &Orchestrator{}
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "subdir/test.txt")

	err := o.writeFileAtomic(path, "hello world")
	if err != nil {
		t.Fatalf("writeFileAtomic failed: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(content) != "hello world" {
		t.Errorf("expected hello world, got %q", string(content))
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Hello World", "hello_world"},
		{"Task-1: Setup", "task-1_setup"},
		{"file!@#$%^&*()name", "file_name"},
	}

	for _, tt := range tests {
		if got := sanitizeFilename(tt.input); got != tt.want {
			t.Errorf("sanitizeFilename(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestOrchestrator_UpdateUnitStatus(t *testing.T) {
	o := &Orchestrator{}
	plan := &state.Plan{
		Units: []state.PlanUnit{
			{ID: "u1", Status: state.UnitStatusPending},
		},
	}

	err := o.updateUnitStatus(plan, "u1", state.UnitStatusCoding)
	if err != nil {
		t.Fatalf("updateUnitStatus failed: %v", err)
	}
	if plan.Units[0].Status != state.UnitStatusCoding {
		t.Errorf("expected StatusCoding, got %s", plan.Units[0].Status)
	}

	err = o.updateUnitStatus(plan, "unknown", state.UnitStatusCoding)
	if err == nil {
		t.Error("expected error for unknown unit")
	}
}
