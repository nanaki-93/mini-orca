package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/nanaki-93/mini-orca/v2/internal/tui"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "mini-orca-tui: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, in io.Reader, out io.Writer) error {
	fs := flag.NewFlagSet("mini-orca-tui", flag.ContinueOnError)
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

	state := tui.NewState()
	applyPaletteFlag(state, *paletteFlag)
	_ = state.SyncFromDaemon(ctx, client, *projectFlag)

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
