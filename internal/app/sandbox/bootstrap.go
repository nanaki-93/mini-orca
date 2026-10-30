package sandbox

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/textutil"
)

type BootstrapStep struct {
	Command string `json:"command"`
	State   string `json:"state"`
	Output  string `json:"output,omitempty"`
}

type BootstrapResult struct {
	ProjectID       string          `json:"project_id"`
	ProjectRevision string          `json:"project_revision"`
	Steps           []BootstrapStep `json:"steps"`
}

func RunBootstrapSteps(ctx context.Context, root string) []BootstrapStep {
	steps := []BootstrapStep{}
	if hasAnyFile(root, ".mise.toml", "mise.toml") {
		steps = append(steps, runMiseBootstrapStep(ctx, root))
	}
	if hasAnyFile(root, "lefthook.yml", ".lefthook.yml", "lefthook.yaml") {
		steps = append(steps, runLefthookBootstrapStep(ctx, root))
	}
	return steps
}

func hasAnyFile(root string, names ...string) bool {
	for _, name := range names {
		if info, err := os.Stat(filepath.Join(root, name)); err == nil && !info.IsDir() {
			return true
		}
	}
	return false
}

func runMiseBootstrapStep(ctx context.Context, root string) BootstrapStep {
	step := BootstrapStep{Command: "mise trust && mise install"}
	if _, err := exec.LookPath("mise"); err != nil {
		step.State = "skipped"
		step.Output = "mise binary not found in PATH"
		return step
	}
	trustOut, trustErr := runBootstrapCmd(ctx, root, "mise", "trust")
	if trustErr != nil {
		step.State = "failed"
		step.Output = trustOut
		return step
	}
	installOut, installErr := runBootstrapCmd(ctx, root, "mise", "install")
	combined := strings.TrimSpace(strings.TrimSpace(trustOut) + "\n" + strings.TrimSpace(installOut))
	if installErr != nil {
		step.State = "failed"
		step.Output = combined
		return step
	}
	step.State = "passed"
	step.Output = combined
	return step
}

func runLefthookBootstrapStep(ctx context.Context, root string) BootstrapStep {
	step := BootstrapStep{Command: "lefthook install"}
	if _, err := exec.LookPath("lefthook"); err != nil {
		step.State = "skipped"
		step.Output = "lefthook binary not found in PATH"
		return step
	}
	out, err := runBootstrapCmd(ctx, root, "lefthook", "install")
	if err != nil {
		step.State = "failed"
		step.Output = out
		return step
	}
	step.State = "passed"
	step.Output = out
	return step
}

func runBootstrapCmd(ctx context.Context, dir, name string, args ...string) (string, error) {
	runCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	result, err := RunCommandWithStart(runCtx, dir, append([]string{name}, args...), true, nil)
	text := strings.TrimSpace(result.Output)
	text = textutil.Truncate(text, 2048)
	if err != nil {
		if text == "" {
			text = fmt.Sprintf("%s %s: %v", name, strings.Join(args, " "), err)
		}
		return text, err
	}
	return text, nil
}
