package tools

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type TerraformExecutor struct{}

func (t *TerraformExecutor) Execute(ctx context.Context, projectPath, action, args string) (string, error) {
	var cmd *exec.Cmd

	switch strings.ToLower(action) {
	case "init":
		cmd = exec.CommandContext(ctx, "terraform", "init", "-no-color")
	case "validate":
		cmd = exec.CommandContext(ctx, "terraform", "validate", "-no-color")
	case "plan":
		cmd = exec.CommandContext(ctx, "terraform", "plan", "-no-color")
	default:
		return "", fmt.Errorf("unknown terraform action instruction: %s", action)
	}

	cmd.Dir = projectPath // Execute inside your target project repository folder

	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("terraform %s failed: %w", action, err)
	}

	return string(output), nil
}
