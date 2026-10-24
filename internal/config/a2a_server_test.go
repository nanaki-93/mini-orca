package config

import (
	"strings"
	"testing"
)

func TestAgentServerConfigurationRequiresExplicitLoopbackPrincipals(t *testing.T) {
	defaults, err := ResolveIntegrations(IntegrationsConfig{})
	if err != nil || defaults.A2AServer.Enabled || defaults.A2AServer.Bind != "127.0.0.1:9091" {
		t.Fatalf("passive defaults: %+v %v", defaults.A2AServer, err)
	}
	valid := AgentServer{Enabled: true, Principals: []AgentPrincipal{{ID: "reviewer", BearerTokenEnv: "REVIEWER_TOKEN", ProjectID: "project-1", Capabilities: []string{"analysis", "proposal"}}}}
	resolved, err := resolveAgentServer(valid)
	if err != nil {
		t.Fatal(err)
	}
	resolved.Principals[0].Capabilities[0] = "mutated"
	if valid.Principals[0].Capabilities[0] != "analysis" {
		t.Fatal("mutable configuration alias")
	}
	for _, name := range []string{"host", "dns", "port", "missing", "duplicate", "credential", "capability", "timeout", "concurrency"} {
		t.Run(name, func(t *testing.T) {
			input, _ := resolveAgentServer(valid)
			switch name {
			case "host":
				input.Bind = "0.0.0.0:9091"
			case "dns":
				input.Bind = "localhost:9091"
			case "port":
				input.Bind = "127.0.0.1:0"
			case "missing":
				input.Principals = nil
			case "duplicate":
				input.Principals = append(input.Principals, input.Principals[0])
			case "credential":
				input.Principals[0].BearerTokenEnv = "inline-token"
			case "capability":
				input.Principals[0].Capabilities = []string{"apply"}
			case "timeout":
				input.TimeoutSeconds = 121
			case "concurrency":
				input.MaxConcurrent = 5
			}
			if _, err := resolveAgentServer(input); err == nil {
				t.Fatal("invalid configuration accepted")
			}
		})
	}
	if err := rejectUnknownFields([]byte("integrations:\n  unrestricted_adk_server: true\n")); err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatal("unrestricted legacy key accepted", err)
	}
}
