package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestAgentWorkerCommandRequiresExplicitStartup(t *testing.T) {
	t.Setenv("PORT", "invalid")
	for _, tc := range []struct {
		args []string
		code int
	}{{nil, 0}, {[]string{"--help"}, 0}, {[]string{"--config", "private.yaml"}, 2}, {[]string{"--serve"}, 2}, {[]string{"--serve", "--config", "private.yaml", "--scope", "private.json"}, 1}} {
		var out, diagnostics bytes.Buffer
		if code := run(t.Context(), tc.args, &out, &diagnostics); code != tc.code || out.Len() != 0 {
			t.Fatal(tc, code, diagnostics.String())
		}
	}
	for _, tc := range []struct{ address, port, want string }{{"", "", "127.0.0.1:9091"}, {"", "8080", "0.0.0.0:8080"}, {"127.0.0.1:9092", "bad", "127.0.0.1:9092"}} {
		got, err := workerAddress(tc.address, tc.port)
		if err != nil || got != tc.want {
			t.Fatal(got, err)
		}
	}
	for _, address := range []string{"localhost:80", ":80", "0.0.0.0:0", "127.0.0.1:65536"} {
		if _, err := workerAddress(address, ""); err == nil {
			t.Fatal("invalid address", address)
		}
	}
}

func TestAgentWorkerManifestRejectsAmbiguousOrOversizedJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scope.json")
	for _, text := range []string{`{"project_id":"a","project_id":"b"}`, `{"unknown":true}`, `{} {}`, strings.Repeat(" ", 17<<10)} {
		if err := os.WriteFile(path, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := loadScope(path); err == nil {
			t.Fatal("manifest accepted")
		}
	}
}

type workerShutdownFixture struct {
	done   chan struct{}
	closed chan struct{}
	once   sync.Once
}

func (w *workerShutdownFixture) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	<-w.closed
	response.WriteHeader(http.StatusServiceUnavailable)
}
func (w *workerShutdownFixture) Done() <-chan struct{} { return w.done }
func (w *workerShutdownFixture) Close(context.Context) error {
	w.once.Do(func() { close(w.closed) })
	return nil
}

type failedWorkerOutput struct{}

func (failedWorkerOutput) Write([]byte) (int, error) { return 0, errors.New("output unavailable") }

func TestAgentWorkerShutdownJoinsServerAndPreservesOutputFailure(t *testing.T) {
	for _, failing := range []bool{false, true} {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		worker := &workerShutdownFixture{done: make(chan struct{}), closed: make(chan struct{})}
		ctx, cancel := context.WithCancel(t.Context())
		var output io.Writer = io.Discard
		if failing {
			output = failedWorkerOutput{}
		} else {
			cancel()
		}
		finished := make(chan error, 1)
		go func() { finished <- serveWorker(ctx, listener, worker, output) }()
		select {
		case err := <-finished:
			if (err != nil) != failing {
				t.Fatal("shutdown result", err)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("server shutdown did not join")
		}
		cancel()
		select {
		case <-worker.closed:
		default:
			t.Fatal("worker work retained")
		}
	}
}
