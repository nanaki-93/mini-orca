package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Git never inherits host Git overrides, runs hooks, opens an editor or signs.
// Checkout filters and merge drivers still require explicit execution consent.
func gitCommand(ctx context.Context, root, index string, input io.Reader, args ...string) (string, error) {
	timed, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	flags := []string{"-C", root, "--no-pager", "-c", "core.hooksPath=" + os.DevNull, "-c", "core.fsmonitor=false", "-c", "commit.gpgSign=false", "-c", "merge.gpgSign=false"}
	command := exec.CommandContext(timed, "git", append(flags, args...)...)
	command.Env = append(checkChildEnvironment(os.Environ()), "GIT_TERMINAL_PROMPT=0", "GIT_EDITOR=true", "GIT_MERGE_AUTOEDIT=no", "GIT_LITERAL_PATHSPECS=1", "LC_ALL=C")
	if index != "" {
		command.Env = append(command.Env, "GIT_INDEX_FILE="+index)
	}
	if err := configureCheckCommand(command); err != nil {
		return "", err
	}
	command.WaitDelay = time.Second
	command.Stdin = input
	out, diagnostics := &boundedCheckOutput{limit: 1024 * 1024}, &boundedCheckOutput{limit: 8192}
	command.Stdout, command.Stderr = out, diagnostics
	err := command.Run()
	if timed.Err() != nil {
		return out.String(), timed.Err()
	}
	if err != nil {
		return out.String(), fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(diagnostics.String()))
	}
	if out.Truncated() || diagnostics.Truncated() {
		return out.String(), fmt.Errorf("git %s output exceeded its limit; refresh Git status", args[0])
	}
	return out.String(), nil
}

func gitRead(ctx context.Context, root string, args ...string) (string, error) {
	return gitCommand(ctx, root, "", nil, args...)
}
