package tui

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
)

func TestResearchBrowserUsesOnlyGuardedURLAndArgumentVector(t *testing.T) {
	cl, _ := NewClient("http://127.0.0.1:8080")
	defer cl.Close()
	scope := client.AgentSettingsScope{ProjectIdentity: ProjectIdentity{ProjectID: "p", ProjectRevision: "r"}, WorktreePath: "/tmp/task $(untrusted) `text`"}
	for _, system := range []string{"darwin", "linux", "windows"} {
		calls := 0
		err := launchResearchReport(t.Context(), cl, scope, externalTestID, system, func(ctx context.Context, command string, args ...string) error {
			calls++
			deadline, bounded := ctx.Deadline()
			if !bounded || time.Until(deadline) > 5*time.Second || command == "sh" || command == "cmd" {
				t.Fatal("browser launch was unbounded or shell-interpolated")
			}
			want := map[string]string{"darwin": "open", "linux": "xdg-open", "windows": "rundll32"}[system]
			if command != want || len(args) != 1 && system != "windows" {
				t.Fatal("unexpected launcher", command, args)
			}
			location, err := url.Parse(args[len(args)-1])
			if err != nil {
				t.Fatal(err)
			}
			if location.Host != "127.0.0.1:8080" || location.Query().Get("worktree_path") != scope.WorktreePath || !strings.HasSuffix(location.Path, externalTestID+"/report") {
				t.Fatal("browser URL changed scope", location)
			}
			return nil
		})
		if err != nil || calls != 1 {
			t.Fatal("explicit report did not launch once", err, calls)
		}
	}
}

func TestResearchBrowserRejectsForeignMalformedAndCanceledRequests(t *testing.T) {
	scope := client.AgentSettingsScope{ProjectIdentity: ProjectIdentity{ProjectID: "p", ProjectRevision: "r"}}
	for _, test := range []struct {
		origin, id, system string
		canceled           bool
	}{
		{"https://outside.example", externalTestID, "darwin", false},
		{"http://127.0.0.1:8080", "../report", "darwin", false},
		{"http://127.0.0.1:8080", externalTestID, "unsupported", false},
		{"http://127.0.0.1:8080", externalTestID, "darwin", true},
	} {
		cl, _ := NewClient(test.origin)
		ctx, cancel := context.WithCancel(t.Context())
		if test.canceled {
			cancel()
		}
		calls := 0
		err := launchResearchReport(ctx, cl, scope, test.id, test.system, func(context.Context, string, ...string) error { calls++; return nil })
		cancel()
		_ = cl.Close()
		if err == nil || calls != 0 {
			t.Fatal("invalid or canceled browser request launched", test, err)
		}
	}
	cl, _ := NewClient("http://127.0.0.1:8080")
	defer cl.Close()
	failure := errors.New("launcher unavailable")
	if err := launchResearchReport(t.Context(), cl, scope, externalTestID, "linux", func(context.Context, string, ...string) error { return failure }); !errors.Is(err, failure) {
		t.Fatal("browser failure was hidden", err)
	}
}
