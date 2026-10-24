package main

import (
	"context"
	"net"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/app"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

func agentDaemonFixture(t *testing.T, configuration config.AgentServer) *app.Service {
	t.Helper()
	manager, err := project.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Integrations.A2AServer = configuration
	service, err := app.New(cfg, manager)
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func TestAgentServerListenerIsAbsentByDefault(t *testing.T) {
	server, err := startAgentHTTPServer(agentDaemonFixture(t, config.AgentServer{}))
	if err != nil || server != nil || server.failures() != nil {
		t.Fatal("default A2A exposure", server, err)
	}
	if err := server.shutdown(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestAgentServerListenerReusesLocalRequestPolicy(t *testing.T) {
	configuration := config.AgentServer{Enabled: true, Principals: []config.AgentPrincipal{{ID: "reviewer", ProjectID: "fixture", BearerTokenEnv: "DAEMON_AGENT_TOKEN", Capabilities: []string{"analysis"}}}}
	server, err := newAgentHTTPServer(agentDaemonFixture(t, configuration))
	if err != nil {
		t.Fatal(err)
	}
	defer server.closeAgent(context.Background())
	for _, item := range []struct {
		path, host, origin string
		status             int
	}{
		{"/.well-known/agent-card.json", "127.0.0.1:9091", "", 200},
		{"/.well-known/agent-card.json", "attacker.example", "", 403},
		{"/.well-known/agent-card.json", "127.0.0.1:9091", "https://attacker.example", 403},
		{"/api/projects/current", "127.0.0.1:9091", "", 404},
		{"/dev-ui", "127.0.0.1:9091", "", 404},
	} {
		r := httptest.NewRequest("GET", item.path, nil)
		r.Host = item.host
		r.Header.Set("Origin", item.origin)
		w := httptest.NewRecorder()
		server.server.Handler.ServeHTTP(w, r)
		if w.Code != item.status {
			t.Fatal("local policy", item.path, w.Code, w.Body.String())
		}
		if w.Code == 200 && (strings.Contains(w.Body.String(), "DAEMON_AGENT_TOKEN") || strings.Contains(w.Body.String(), `"project_id"`)) {
			t.Fatal("card disclosed private configuration")
		}
	}
}

func TestAgentServerListenerBindFailureIsReported(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	configuration := config.AgentServer{Enabled: true, Bind: listener.Addr().String(), Principals: []config.AgentPrincipal{{ID: "reviewer", ProjectID: "fixture", BearerTokenEnv: "DAEMON_AGENT_TOKEN", Capabilities: []string{"analysis"}}}}
	if server, err := startAgentHTTPServer(agentDaemonFixture(t, configuration)); err == nil || server != nil {
		t.Fatal("bind failure hidden", server, err)
	}
}
