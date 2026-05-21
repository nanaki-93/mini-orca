package tools

import (
	"context"
	"os/exec"
	"strings"
)

type GenericShellExecutor struct{}

func (s *GenericShellExecutor) Execute(ctx context.Context, projectPath, command, arguments string) (string, error) {
	// Split args string safely into execution arrays
	fields := strings.Fields(arguments)

	cmd := exec.CommandContext(ctx, command, fields...)
	cmd.Dir = projectPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), err
	}
	return string(output), nil
}
