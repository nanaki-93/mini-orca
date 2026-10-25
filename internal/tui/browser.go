package tui

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
)

type browserRunner func(context.Context, string, ...string) error

func openResearchReport(ctx context.Context, cl *Client, scope client.AgentSettingsScope, id string) error {
	return launchResearchReport(ctx, cl, scope, id, runtime.GOOS, func(ctx context.Context, command string, args ...string) error {
		return exec.CommandContext(ctx, command, args...).Run()
	})
}

func openWebUI(ctx context.Context, cl *Client, tab, sessionID, filePath, worktreePath string) error {
	return launchWebUI(ctx, cl, tab, sessionID, filePath, worktreePath, runtime.GOOS, func(ctx context.Context, command string, args ...string) error {
		return exec.CommandContext(ctx, command, args...).Run()
	})
}

func launchResearchReport(ctx context.Context, cl *Client, scope client.AgentSettingsScope, id, system string, run browserRunner) error {
	location, err := cl.ResearchReportURL(scope, id)
	if err != nil {
		return err
	}
	return launchSystemBrowser(ctx, location, "open retained research report", system, run)
}

func launchWebUI(ctx context.Context, cl *Client, tab, sessionID, filePath, worktreePath, system string, run browserRunner) error {
	location, err := cl.WebUIURL(tab, sessionID, filePath, worktreePath)
	if err != nil {
		return err
	}
	return launchSystemBrowser(ctx, location, "open local web UI", system, run)
}

func launchSystemBrowser(ctx context.Context, location, action, system string, run browserRunner) error {
	var command string
	var args []string
	switch system {
	case "darwin":
		command, args = "open", []string{location}
	case "linux":
		command, args = "xdg-open", []string{location}
	case "windows":
		command, args = "rundll32", []string{"url.dll,FileProtocolHandler", location}
	default:
		return fmt.Errorf("system browser opening is unavailable on %s", system)
	}
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := context.Cause(bounded); err != nil {
		return err
	}
	if err := run(bounded, command, args...); err != nil {
		return fmt.Errorf("%s: %w", action, err)
	}
	return nil
}
