package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type BootstrapExecStep struct {
	Command string `json:"command"`
	State   string `json:"state"`
	Output  string `json:"output,omitempty"`
}

type BootstrapExecResult struct {
	ProjectID       string              `json:"project_id"`
	ProjectRevision string              `json:"project_revision"`
	Steps           []BootstrapExecStep `json:"steps"`
}

// ExecuteBootstrapToolchain runs `mise trust && mise install` and `lefthook install`
// in the active project root when the corresponding configuration files and binaries exist.
func (s *Service) ExecuteBootstrapToolchain(ctx context.Context, revision string, confirm bool) (*BootstrapExecResult, error) {
	if confirm {
		if _, err := s.TrustProjectExecution(revision, true); err != nil {
			return nil, err
		}
	} else if err := s.requireProjectExecutionTrust(revision); err != nil {
		return nil, err
	}
	analysis, err := s.manager.Analysis()
	if err != nil {
		return nil, err
	}
	root := s.manager.Root()
	result := &BootstrapExecResult{
		ProjectID:       analysis.ProjectID,
		ProjectRevision: analysis.ProjectRevision,
		Steps:           []BootstrapExecStep{},
	}
	if hasAnyFile(root, ".mise.toml", "mise.toml") {
		result.Steps = append(result.Steps, runMiseBootstrapStep(ctx, root))
	}
	if hasAnyFile(root, "lefthook.yml", ".lefthook.yml", "lefthook.yaml") {
		result.Steps = append(result.Steps, runLefthookBootstrapStep(ctx, root))
	}
	return result, nil
}

func hasAnyFile(root string, names ...string) bool {
	for _, name := range names {
		if info, err := os.Stat(filepath.Join(root, name)); err == nil && !info.IsDir() {
			return true
		}
	}
	return false
}

func runMiseBootstrapStep(ctx context.Context, root string) BootstrapExecStep {
	step := BootstrapExecStep{Command: "mise trust && mise install"}
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

func runLefthookBootstrapStep(ctx context.Context, root string) BootstrapExecStep {
	step := BootstrapExecStep{Command: "lefthook install"}
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
	cmd := exec.CommandContext(runCtx, name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(out))
	if len(text) > 2048 {
		text = text[:2048]
	}
	if err != nil {
		if text == "" {
			text = fmt.Sprintf("%s %s: %v", name, strings.Join(args, " "), err)
		}
		return text, err
	}
	return text, nil
}
