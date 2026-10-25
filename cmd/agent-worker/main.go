// Command agent-worker serves explicitly admitted, in-memory source analysis.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/json/jsontext"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/app"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(run(ctx, os.Args[1:], os.Stdout, os.Stderr))
}

type workerOptions struct {
	serve                         bool
	configPath, scopePath, listen string
	worker                        app.AgentWorkerOptions
}

func parseOptions(args []string, diagnostics io.Writer) (workerOptions, error) {
	var o workerOptions
	f := flag.NewFlagSet("agent-worker", flag.ContinueOnError)
	f.SetOutput(diagnostics)
	f.BoolVar(&o.serve, "serve", false, "explicitly start the authenticated worker")
	f.StringVar(&o.configPath, "config", "", "explicit model and bearer-principal YAML configuration")
	f.StringVar(&o.scopePath, "scope", "", "operator-approved project/revision and path/hash JSON manifest")
	f.StringVar(&o.listen, "listen", "", "literal IP:port; default loopback, or 0.0.0.0:$PORT")
	f.StringVar(&o.worker.Endpoint, "public-url", "", "public HTTPS /a2a URL, or literal loopback HTTP")
	f.DurationVar(&o.worker.Lifetime, "lifetime", 15*time.Minute, "worker lifetime, 1s–15m")
	f.IntVar(&o.worker.MaxJobs, "max-jobs", 32, "maximum admitted jobs per process, 1–128")
	f.BoolVar(&o.worker.ConfirmSourceSharing, "confirm-source-sharing", false, "confirm receiving source from the pinned manifest")
	f.BoolVar(&o.worker.ConfirmRemoteProvider, "confirm-remote-provider", false, "confirm sharing admitted content with the configured remote model")
	if len(args) == 0 {
		f.Usage()
		return o, flag.ErrHelp
	}
	if err := f.Parse(args); err != nil {
		return o, err
	}
	if f.NArg() != 0 || !o.serve || o.configPath == "" || o.scopePath == "" {
		return o, fmt.Errorf("require --serve, --config and --scope, without positional arguments")
	}
	return o, nil
}

func workerAddress(address, port string) (string, error) {
	if address == "" {
		address = "127.0.0.1:9091"
		if port != "" {
			address = net.JoinHostPort("0.0.0.0", port)
		}
	}
	host, port, err := net.SplitHostPort(address)
	number, portErr := strconv.Atoi(port)
	if err != nil || portErr != nil || net.ParseIP(host) == nil || number < 1 || number > 65535 {
		return "", fmt.Errorf("listen requires a literal IP and port 1–65535")
	}
	return address, nil
}

func loadScope(path string) (app.AgentWorkerScope, error) {
	var scope app.AgentWorkerScope
	f, err := os.Open(path)
	if err != nil {
		return scope, fmt.Errorf("cannot open scope manifest")
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, (16<<10)+1))
	if err != nil || len(data) > 16<<10 || !jsontext.Value(data).IsValid() {
		return scope, fmt.Errorf("scope manifest must be valid JSON within 16 KiB")
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&scope); err != nil {
		return scope, fmt.Errorf("invalid scope manifest fields")
	}
	return scope, nil
}

func run(ctx context.Context, args []string, output, diagnostics io.Writer) int {
	o, err := parseOptions(args, diagnostics)
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	if err != nil {
		fmt.Fprintln(diagnostics, err)
		return 2
	}
	if err := startWorker(ctx, o, output); err != nil {
		fmt.Fprintln(diagnostics, err)
		return 1
	}
	return 0
}

func startWorker(ctx context.Context, o workerOptions, output io.Writer) error {
	address, err := workerAddress(o.listen, os.Getenv("PORT"))
	if err != nil {
		return err
	}
	if o.worker.Scope, err = loadScope(o.scopePath); err != nil {
		return err
	}
	cfg, err := config.LoadFromYAML(o.configPath)
	if err != nil {
		return fmt.Errorf("cannot load worker configuration; verify the private YAML file")
	}
	worker, err := app.NewAgentWorker(ctx, cfg, o.worker)
	if err != nil {
		return fmt.Errorf("worker admission rejected: %w", err)
	}
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", address)
	if err != nil {
		closeContext, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		return errors.Join(fmt.Errorf("cannot bind worker listener"), worker.Close(closeContext))
	}
	return serveWorker(ctx, listener, worker, output)
}

type workerService interface {
	http.Handler
	Done() <-chan struct{}
	Close(context.Context) error
}

func serveWorker(ctx context.Context, listener net.Listener, worker workerService, output io.Writer) error {
	server := &http.Server{Handler: worker, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 130 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16 << 10, ErrorLog: log.New(io.Discard, "", 0)}
	finished := make(chan error, 1)
	go func() { finished <- server.Serve(listener) }()
	_, result := fmt.Fprintln(output, `{"state":"ready","capabilities":["analysis"],"advisory":true}`)
	joined := false
	if result == nil {
		select {
		case <-ctx.Done():
		case <-worker.Done():
		case result = <-finished:
			joined = true
			if errors.Is(result, http.ErrServerClosed) {
				result = nil
			}
		}
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	// Stop admission and cancel owned calls before waiting for HTTP handlers.
	closeErr := worker.Close(shutdown)
	shutdownErr := server.Shutdown(shutdown)
	if shutdownErr != nil {
		shutdownErr = errors.Join(shutdownErr, server.Close())
	}
	if !joined {
		serveErr := <-finished
		if !errors.Is(serveErr, http.ErrServerClosed) {
			result = errors.Join(result, serveErr)
		}
	}
	return errors.Join(result, closeErr, shutdownErr)
}
