package config

import (
	"fmt"
	"maps"
	"net"
	"net/url"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// IntegrationsConfig is passive configuration. It does not grant connection,
// process execution, or permission to disclose project context.
type IntegrationsConfig struct {
	MCP          []MCPServer      `yaml:"mcp"`
	OpenAPI      []OpenAPIService `yaml:"openapi"`
	GoogleSearch bool             `yaml:"google_search"`
}

type OpenAPIService struct {
	ID             string            `yaml:"id" json:"id"`
	SpecPath       string            `yaml:"spec_path" json:"spec_path"`
	Endpoint       string            `yaml:"endpoint" json:"endpoint"`
	BearerTokenEnv string            `yaml:"bearer_token_env" json:"bearer_token_env,omitempty"`
	Operations     []IntegrationTool `yaml:"operations" json:"operations"`
	TimeoutSeconds int               `yaml:"timeout_seconds" json:"timeout_seconds"`
}

type MCPServer struct {
	ID             string            `yaml:"id" json:"id"`
	Transport      string            `yaml:"transport" json:"transport"`
	Endpoint       string            `yaml:"endpoint" json:"endpoint,omitempty"`
	BearerTokenEnv string            `yaml:"bearer_token_env" json:"bearer_token_env,omitempty"`
	Command        string            `yaml:"command" json:"command,omitempty"`
	Args           []string          `yaml:"args" json:"args,omitempty"`
	Environment    map[string]string `yaml:"environment" json:"environment,omitempty"`
	Tools          []IntegrationTool `yaml:"tools" json:"tools"`
	TimeoutSeconds int               `yaml:"timeout_seconds" json:"timeout_seconds"`
}

type IntegrationTool struct {
	Name   string `yaml:"name" json:"name"`
	Effect string `yaml:"effect" json:"effect"`
}

var integrationID = regexp.MustCompile(`^[a-z][a-z0-9_]{0,23}$`)
var integrationToolName = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_.-]{0,63}$`)
var integrationEnvName = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,63}$`)

func ResolveIntegrations(input IntegrationsConfig) (IntegrationsConfig, error) {
	result := IntegrationsConfig{MCP: slices.Clone(input.MCP), GoogleSearch: input.GoogleSearch}
	if len(result.MCP) > 8 {
		return result, fmt.Errorf("integrations.mcp permits at most eight servers")
	}
	seen := map[string]bool{}
	for i, server := range result.MCP {
		if !integrationID.MatchString(server.ID) || seen[server.ID] {
			return result, fmt.Errorf("MCP IDs must be unique lowercase identifiers of at most 24 characters")
		}
		seen[server.ID] = true
		if server.TimeoutSeconds == 0 {
			server.TimeoutSeconds = 30
		}
		if server.TimeoutSeconds < 1 || server.TimeoutSeconds > 60 {
			return result, fmt.Errorf("MCP timeout must be 1–60 seconds")
		}
		if err := validateMCPServer(server); err != nil {
			return result, fmt.Errorf("MCP server %s: %w", server.ID, err)
		}
		server.Args, server.Tools, server.Environment = slices.Clone(server.Args), slices.Clone(server.Tools), maps.Clone(server.Environment)
		result.MCP[i] = server
	}
	var err error
	result.OpenAPI, err = resolveOpenAPIServices(input.OpenAPI, seen)
	return result, err
}

func resolveOpenAPIServices(input []OpenAPIService, seen map[string]bool) ([]OpenAPIService, error) {
	result := slices.Clone(input)
	if len(result) > 8 {
		return nil, fmt.Errorf("integrations.openapi permits at most eight services")
	}
	for i, service := range result {
		if !integrationID.MatchString(service.ID) || seen[service.ID] {
			return nil, fmt.Errorf("integration IDs must be unique lowercase identifiers of at most 24 characters")
		}
		seen[service.ID] = true
		if service.TimeoutSeconds == 0 {
			service.TimeoutSeconds = 30
		}
		if err := validateOpenAPIService(service); err != nil {
			return nil, fmt.Errorf("OpenAPI service %s: %w", service.ID, err)
		}
		service.Operations = slices.Clone(service.Operations)
		result[i] = service
	}
	return result, nil
}

func validateOpenAPIService(service OpenAPIService) error {
	if service.TimeoutSeconds < 1 || service.TimeoutSeconds > 60 {
		return fmt.Errorf("OpenAPI timeout must be 1–60 seconds")
	}
	if !filepath.IsLocal(service.SpecPath) || filepath.ToSlash(filepath.Clean(service.SpecPath)) != service.SpecPath || len(service.SpecPath) > 512 || strings.ContainsAny(service.SpecPath, "\\\x00\r\n") {
		return fmt.Errorf("spec_path must be a canonical project-relative file")
	}
	if service.BearerTokenEnv != "" && !integrationEnvName.MatchString(service.BearerTokenEnv) {
		return fmt.Errorf("bearer_token_env must name an environment variable")
	}
	if err := validateIntegrationEndpoint(service.Endpoint); err != nil {
		return err
	}
	u, _ := url.Parse(service.Endpoint)
	if u.RawPath != "" || strings.ContainsAny(u.Path, "{}\\") || strings.Contains(u.Path, "..") {
		return fmt.Errorf("endpoint must have a fixed, unescaped base path")
	}
	return validateAPIOperations(service.Operations)
}

func validateAPIOperations(operations []IntegrationTool) error {
	if len(operations) < 1 || len(operations) > 4 {
		return fmt.Errorf("configure one to four explicitly classified operations")
	}
	seen := map[string]bool{}
	for _, operation := range operations {
		if !integrationToolName.MatchString(operation.Name) || seen[operation.Name] {
			return fmt.Errorf("operation IDs must be bounded unique identifiers")
		}
		seen[operation.Name] = true
		if operation.Effect != "read_external" && operation.Effect != "mutate_external" {
			return fmt.Errorf("classify operations as read_external or mutate_external")
		}
	}
	return nil
}

func validateMCPServer(server MCPServer) error {
	if err := validateIntegrationTools(server.Tools); err != nil {
		return err
	}
	switch server.Transport {
	case "streamable_http":
		if server.Command != "" || len(server.Args) != 0 || len(server.Environment) != 0 {
			return fmt.Errorf("HTTP transport cannot configure a process")
		}
		if server.BearerTokenEnv != "" && !integrationEnvName.MatchString(server.BearerTokenEnv) {
			return fmt.Errorf("bearer_token_env must name an environment variable")
		}
		return validateIntegrationEndpoint(server.Endpoint)
	case "stdio":
		return validateMCPCommand(server)
	default:
		return fmt.Errorf("transport must be stdio or streamable_http")
	}
}

func validateIntegrationTools(tools []IntegrationTool) error {
	if len(tools) < 1 || len(tools) > 4 {
		return fmt.Errorf("configure one to four explicitly classified tools")
	}
	seen := map[string]bool{}
	for _, tool := range tools {
		if !integrationToolName.MatchString(tool.Name) || seen[tool.Name] {
			return fmt.Errorf("tool names must be bounded unique identifiers")
		}
		seen[tool.Name] = true
		if tool.Effect != "read_external" {
			return fmt.Errorf("only operator-classified read_external MCP tools are supported; mutations, filesystem and command execution are unavailable")
		}
	}
	return nil
}

func validateIntegrationEndpoint(endpoint string) error {
	u, err := url.Parse(endpoint)
	if err != nil || len(endpoint) > 2048 || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" {
		return fmt.Errorf("endpoint must be an absolute HTTP URL without credentials, query or fragment")
	}
	ip := net.ParseIP(u.Hostname())
	if u.Scheme != "https" && !(u.Scheme == "http" && ip != nil && ip.IsLoopback()) {
		return fmt.Errorf("endpoint requires HTTPS, except literal loopback HTTP")
	}
	return nil
}

func validateMCPCommand(server MCPServer) error {
	if server.Endpoint != "" || server.BearerTokenEnv != "" || !filepath.IsAbs(server.Command) || filepath.Clean(server.Command) != server.Command || len(server.Command) > 1024 || strings.ContainsAny(server.Command, "\x00\r\n") {
		return fmt.Errorf("stdio requires an absolute installed command and no HTTP settings")
	}
	if len(server.Args) > 16 || len(server.Environment) > 8 {
		return fmt.Errorf("stdio permits at most 16 fixed arguments and eight environment references")
	}
	for _, arg := range server.Args {
		if len(arg) > 1024 || strings.ContainsAny(arg, "\x00\r\n") {
			return fmt.Errorf("stdio arguments must be bounded single-line strings")
		}
	}
	return validateMCPEnvironment(server.Environment)
}

func validateMCPEnvironment(environment map[string]string) error {
	for name, reference := range environment {
		if !integrationEnvName.MatchString(name) || !integrationEnvName.MatchString(reference) || reservedProcessEnv(name) {
			return fmt.Errorf("stdio environment permits explicit credential references, not process configuration")
		}
	}
	return nil
}

func reservedProcessEnv(name string) bool {
	return slices.Contains([]string{"PATH", "HOME", "TMPDIR", "TMP", "TEMP", "ENV", "BASH_ENV", "SHELLOPTS", "IFS", "CDPATH", "RUBYOPT", "PERL5OPT", "JAVA_TOOL_OPTIONS", "CLASSPATH", "GOENV", "GOPATH", "GOROOT", "GOTOOLCHAIN", "GOCACHE", "GOMODCACHE", "GOPROXY", "GOSUMDB", "GONOSUMDB", "GONOPROXY", "GOFLAGS", "GODEBUG", "CGO_ENABLED", "CC", "CXX"}, name) || strings.HasPrefix(name, "LD_") || strings.HasPrefix(name, "DYLD_") || strings.HasPrefix(name, "PYTHON") || strings.HasPrefix(name, "NODE_")
}
