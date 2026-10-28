// Command mini-orca opens Mini-Orca on the project in the current folder (or
// the folder given as its argument): it starts the daemon beside it when none
// is running, makes that folder the daemon's project, and opens the Web UI in
// the browser. A daemon it started runs until Ctrl+C; one that was already
// running is only switched to the folder and keeps running.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdout); err != nil && !errors.Is(err, flag.ErrHelp) {
		fmt.Fprintf(os.Stderr, "mini-orca: %v\n", err)
		os.Exit(1)
	}
}

type options struct {
	url     string
	project string
	open    bool
}

func parseOptions(args []string, out io.Writer) (options, error) {
	flags := flag.NewFlagSet("mini-orca", flag.ContinueOnError)
	flags.SetOutput(out)
	flags.Usage = func() {
		fmt.Fprintln(out, "Usage: mini-orca [flags] [project folder]")
		fmt.Fprintln(out, "Opens Mini-Orca on the project folder (default: the current folder).")
		flags.PrintDefaults()
	}
	defaultURL := os.Getenv("MINI_ORCA_URL")
	if defaultURL == "" {
		defaultURL = defaultDaemonURL
	}
	url := flags.String("url", defaultURL, "daemon address (MINI_ORCA_URL)")
	noOpen := flags.Bool("no-open", false, "do not open the Web UI in the browser")
	if err := flags.Parse(args); err != nil {
		return options{}, err
	}
	if flags.NArg() > 1 {
		return options{}, fmt.Errorf("give at most one project folder")
	}
	project, err := projectFolder(flags.Arg(0))
	if err != nil {
		return options{}, err
	}
	return options{url: *url, project: project, open: !*noOpen}, nil
}

func run(ctx context.Context, args []string, out io.Writer) error {
	opts, err := parseOptions(args, out)
	if err != nil {
		return err
	}
	client := newDaemonClient(opts.url)
	var daemon *ownedDaemon
	if !client.healthy(ctx) {
		if opts.url != defaultDaemonURL {
			return fmt.Errorf("no daemon answers at %s; start it there or leave --url unset", opts.url)
		}
		fmt.Fprintf(out, "Starting the Mini-Orca daemon for %s…\n", opts.project)
		if daemon, err = startDaemon(ctx, opts.project, out); err != nil {
			return err
		}
		defer daemon.stop()
		if err := client.waitHealthy(ctx, daemon); err != nil {
			return err
		}
	}
	if err := client.openProject(ctx, opts.project); err != nil {
		return err
	}
	page := client.base + "/ui"
	if opts.open {
		if err := openBrowser(page); err != nil {
			fmt.Fprintf(out, "Could not open the browser (%v); open %s yourself.\n", err, page)
		}
	}
	if daemon == nil {
		fmt.Fprintf(out, "Opened %s in the running daemon: %s\n", opts.project, page)
		fmt.Fprintln(out, client.configNote(ctx))
		return nil
	}
	fmt.Fprintf(out, "Mini-Orca is running for %s at %s — press Ctrl+C to stop it.\n", opts.project, page)
	select {
	case <-ctx.Done():
		fmt.Fprintln(out, "\nStopping the daemon…")
		return nil
	case <-daemon.done:
		return fmt.Errorf("the daemon stopped: %v", daemon.err)
	}
}
