package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/tui"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "mini-orca: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, in io.Reader, out io.Writer) error {
	fs := flag.NewFlagSet("mini-orca", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	defaultURL := strings.TrimSpace(os.Getenv("MINI_ORCA_URL"))
	if defaultURL == "" {
		defaultURL = "http://127.0.0.1:9090"
	}
	urlFlag := fs.String("url", defaultURL, "Mini-Orca daemon HTTP origin")
	projectFlag := fs.String("project", "", "Project path to open or restore")
	paletteFlag := fs.String("palette", "tokyo", "Initial TUI color palette (tokyo, crt, amber, paper)")
	cmdFlag := fs.String("cmd", "", "Optional colon command to run on startup")
	onceFlag := fs.Bool("once", false, "Render a single frame and exit")
	widthFlag := fs.Int("width", 120, "Terminal width in columns")
	heightFlag := fs.Int("height", 36, "Terminal height in rows")

	if err := fs.Parse(args); err != nil {
		return err
	}

	client, err := tui.NewClient(*urlFlag)
	if err != nil {
		return err
	}
	defer client.Close()

	projectPath := resolveProjectPath(*projectFlag, *onceFlag)
	stopDaemon := ensureLocalDaemon(ctx, client, *onceFlag)
	defer stopDaemon()

	state := tui.NewState()
	applyPaletteFlag(state, *paletteFlag)
	_ = state.SyncFromDaemon(ctx, client, projectPath)

	if strings.TrimSpace(*cmdFlag) != "" {
		state.ExecuteCommand(*cmdFlag)
		state.FlushActions(ctx, client)
	}
	if *onceFlag {
		if root := client.Root(); root != "" && state.Project != nil && state.Project.Path == "" {
			state.Project.Path = root
		}
		_, err := fmt.Fprintln(out, state.Render(*widthFlag, *heightFlag))
		return err
	}
	go func() {
		<-ctx.Done()
		client.Cancel("sync-status")
	}()
	return state.RunSession(ctx, client, in, out, *widthFlag, *heightFlag)
}

func resolveProjectPath(explicit string, once bool) string {
	if strings.TrimSpace(explicit) != "" || once {
		return explicit
	}
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}
	return ""
}

func ensureLocalDaemon(ctx context.Context, client *tui.Client, once bool) func() {
	if once || daemonReachable(ctx, client) {
		return func() {}
	}
	daemonBin := findDaemonBinary()
	if daemonBin == "" {
		return func() {}
	}
	cmd := exec.CommandContext(ctx, daemonBin)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return func() {}
	}
	waitForDaemon(ctx, client)
	return func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			_, _ = cmd.Process.Wait()
		}
	}
}

func daemonReachable(ctx context.Context, client *tui.Client) bool {
	pingCtx, cancel := context.WithTimeout(ctx, 250*time.Millisecond)
	defer cancel()
	resp, err := client.Request(pingCtx, "health-ping", http.MethodGet, "/health", "")
	return err == nil && resp.Status == http.StatusOK
}

func findDaemonBinary() string {
	if exe, err := os.Executable(); err == nil {
		sibling := filepath.Join(filepath.Dir(exe), "mini-orca-daemon")
		if _, err := os.Stat(sibling); err == nil {
			return sibling
		}
	}
	if path, err := exec.LookPath("mini-orca-daemon"); err == nil {
		return path
	}
	return ""
}

func waitForDaemon(ctx context.Context, client *tui.Client) {
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if daemonReachable(ctx, client) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func applyPaletteFlag(state *tui.State, raw string) {
	switch tui.Palette(strings.ToLower(strings.TrimSpace(raw))) {
	case tui.PaletteCRT:
		state.Palette = tui.PaletteCRT
	case tui.PaletteAmber:
		state.Palette = tui.PaletteAmber
	case tui.PalettePaper:
		state.Palette = tui.PalettePaper
	default:
		state.Palette = tui.PaletteTokyo
	}
}
