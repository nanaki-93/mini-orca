package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestMCPConfigurationIsExplicitBoundedAndImmutable(t *testing.T) {
	base := MCPServer{ID: "docs", Transport: "streamable_http", Endpoint: "https://docs.example/mcp", BearerTokenEnv: "DOCS_TOKEN", Tools: []IntegrationTool{{Name: "search", Effect: "read_external"}}}
	resolved, err := ResolveIntegrations(IntegrationsConfig{MCP: []MCPServer{base}})
	if err != nil || resolved.MCP[0].TimeoutSeconds != 30 {
		t.Fatalf("resolve: %+v %v", resolved, err)
	}
	resolved.MCP[0].Tools[0].Name = "changed"
	if base.Tools[0].Name != "search" {
		t.Fatal("caller configuration mutated")
	}
	for name, change := range map[string]func(*MCPServer){
		"unsafe HTTP":        func(s *MCPServer) { s.Endpoint = "http://example.com/mcp" },
		"credentials in URL": func(s *MCPServer) { s.Endpoint = "https://user:secret@example.com/mcp" },
		"query":              func(s *MCPServer) { s.Endpoint += "?token=secret" },
		"fragment":           func(s *MCPServer) { s.Endpoint += "#fragment" },
		"missing tools":      func(s *MCPServer) { s.Tools = nil },
		"mutation":           func(s *MCPServer) { s.Tools = []IntegrationTool{{Name: "write", Effect: "write"}} },
		"shell":              func(s *MCPServer) { s.Tools = []IntegrationTool{{Name: "run", Effect: "execute"}} },
		"bad env":            func(s *MCPServer) { s.BearerTokenEnv = "token value" },
		"mixed":              func(s *MCPServer) { s.Command = "/usr/bin/server" },
		"timeout":            func(s *MCPServer) { s.TimeoutSeconds = 61 },
	} {
		t.Run(name, func(t *testing.T) {
			copy := base
			change(&copy)
			if _, err := ResolveIntegrations(IntegrationsConfig{MCP: []MCPServer{copy}}); err == nil {
				t.Fatal("unsafe configuration accepted")
			}
		})
	}
	process := MCPServer{ID: "local", Transport: "stdio", Command: "/usr/local/bin/docs-mcp", Args: []string{"--stdio"}, Environment: map[string]string{"DOCS_TOKEN": "PRIVATE_DOCS_TOKEN"}, Tools: base.Tools}
	if _, err := ResolveIntegrations(IntegrationsConfig{MCP: []MCPServer{process}}); err != nil {
		t.Fatal(err)
	}
	process.Environment = map[string]string{"GOOGLE_API_KEY": "MCP_GOOGLE_API_KEY"}
	if _, err := ResolveIntegrations(IntegrationsConfig{MCP: []MCPServer{process}}); err != nil {
		t.Fatalf("ordinary credential name rejected: %v", err)
	}
	for _, name := range []string{"HOME", "PATH", "LD_PRELOAD", "DYLD_INSERT_LIBRARIES", "NODE_OPTIONS", "BASH_ENV"} {
		process.Environment = map[string]string{name: "TOKEN"}
		if _, err := ResolveIntegrations(IntegrationsConfig{MCP: []MCPServer{process}}); err == nil {
			t.Fatalf("process configuration environment accepted: %s", name)
		}
	}
	if _, err := ResolveIntegrations(IntegrationsConfig{MCP: []MCPServer{base, base}}); err == nil {
		t.Fatal("duplicate connection accepted")
	}
}

func TestMCPConfigurationRejectsUnknownNestedFields(t *testing.T) {
	cfg := Default()
	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Replace(string(data), "integrations:\n    mcp: []", "integrations:\n    mcp:\n      - id: docs\n        automatic_install: true", 1)
	if text == string(data) {
		t.Fatal("fixture did not replace integration config")
	}
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFromYAML(path); err == nil || !strings.Contains(err.Error(), "automatic_install") {
		t.Fatalf("unknown field: %v", err)
	}
}

func TestOpenAPIConfigurationPinsScopeAndEffect(t *testing.T) {
	base := OpenAPIService{ID: "items", SpecPath: "docs/api.json", Endpoint: "https://api.example/v1", BearerTokenEnv: "API_TOKEN", Operations: []IntegrationTool{{Name: "lookup", Effect: "read_external"}, {Name: "update", Effect: "mutate_external"}}}
	resolved, err := ResolveIntegrations(IntegrationsConfig{OpenAPI: []OpenAPIService{base}, GoogleSearch: true})
	if err != nil || !resolved.GoogleSearch || resolved.OpenAPI[0].TimeoutSeconds != 30 {
		t.Fatalf("configuration: %+v %v", resolved, err)
	}
	resolved.OpenAPI[0].Operations[0].Name = "changed"
	if base.Operations[0].Name != "lookup" {
		t.Fatal("configuration alias")
	}
	for name, change := range map[string]func(*OpenAPIService){
		"outside":  func(s *OpenAPIService) { s.SpecPath = "../api.json" },
		"absolute": func(s *OpenAPIService) { s.SpecPath = "/tmp/api.json" },
		"insecure": func(s *OpenAPIService) { s.Endpoint = "http://example.com" },
		"auth":     func(s *OpenAPIService) { s.Endpoint = "https://user:secret@example.com" },
		"escape":   func(s *OpenAPIService) { s.Endpoint = "https://api.example/%2e%2e/private" },
		"effect":   func(s *OpenAPIService) { s.Operations = []IntegrationTool{{Name: "update", Effect: "execute"}} },
		"timeout":  func(s *OpenAPIService) { s.TimeoutSeconds = 61 },
	} {
		t.Run(name, func(t *testing.T) {
			copy := base
			change(&copy)
			if _, err := ResolveIntegrations(IntegrationsConfig{OpenAPI: []OpenAPIService{copy}}); err == nil {
				t.Fatal("invalid API configuration accepted")
			}
		})
	}
	if _, err := ResolveIntegrations(IntegrationsConfig{MCP: []MCPServer{{ID: "items", Transport: "streamable_http", Endpoint: "https://api.example/mcp", Tools: []IntegrationTool{{Name: "lookup", Effect: "read_external"}}}}, OpenAPI: []OpenAPIService{base}}); err == nil {
		t.Fatal("ambiguous integration ID accepted")
	}
}
