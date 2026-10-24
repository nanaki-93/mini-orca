package config

import "testing"

func TestA2AConfigurationPinsDestinationAndSkillScope(t *testing.T) {
	valid := RemoteAgent{ID: "reviewer", CardURL: "https://agent.example/card", Endpoint: "https://agent.example/rpc", Skills: []string{"review"}, BearerTokenEnv: "REVIEW_TOKEN"}
	resolved, err := ResolveIntegrations(IntegrationsConfig{A2A: []RemoteAgent{valid}})
	if err != nil || resolved.A2A[0].TimeoutSeconds != 30 {
		t.Fatalf("resolve: %+v %v", resolved, err)
	}
	resolved.A2A[0].Skills[0] = "changed"
	if valid.Skills[0] != "review" {
		t.Fatal("configuration slice alias")
	}
	for _, kind := range []string{"origin", "http", "credentials", "query", "path", "skill", "duplicate", "timeout"} {
		t.Run(kind, func(t *testing.T) {
			remote := valid
			switch kind {
			case "origin":
				remote.Endpoint = "https://other.example/rpc"
			case "http":
				remote.CardURL = "http://agent.example/card"
			case "credentials":
				remote.BearerTokenEnv = "inline-secret"
			case "query":
				remote.CardURL += "?token=x"
			case "path":
				remote.Endpoint += "/../write"
			case "skill":
				remote.Skills = nil
			case "duplicate":
				remote.Skills = []string{"review", "review"}
			case "timeout":
				remote.TimeoutSeconds = 61
			}
			if _, err := ResolveIntegrations(IntegrationsConfig{A2A: []RemoteAgent{remote}}); err == nil {
				t.Fatal("unsafe configuration accepted")
			}
		})
	}
	if _, err := ResolveIntegrations(IntegrationsConfig{A2A: []RemoteAgent{valid, valid}}); err == nil {
		t.Fatal("duplicate remote identity accepted")
	}
}
