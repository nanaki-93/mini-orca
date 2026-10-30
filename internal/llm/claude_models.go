package llm

import (
	"context"
	"fmt"
	"os/exec"
)

// ClaudeModel is a model Claude Code offers by alias. The aliases follow the
// newest model of each family the user's plan includes.
type ClaudeModel struct {
	ID   string
	Name string
}

var claudeModels = []ClaudeModel{
	{ID: "opus", Name: "Claude Opus"},
	{ID: "sonnet", Name: "Claude Sonnet"},
	{ID: "haiku", Name: "Claude Haiku"},
	{ID: "fable", Name: "Claude Fable"},
}

// DiscoverClaudeModels lists the aliases when Claude Code is installed.
// Claude Code has no catalog command, and nothing is started to find out:
// whether the plan includes a model shows when a run starts.
func DiscoverClaudeModels(ctx context.Context, executable string) ([]ClaudeModel, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if _, err := exec.LookPath(executable); err != nil {
		return nil, fmt.Errorf("claude CLI executable is unavailable")
	}
	return append([]ClaudeModel(nil), claudeModels...), nil
}
