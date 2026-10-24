package config

import (
	"fmt"
	"net"
	"slices"
	"strconv"
	"strings"
)

// AgentServer configures authentication, never permission to use a project.
type AgentServer struct {
	Enabled        bool             `yaml:"enabled" json:"enabled"`
	Bind           string           `yaml:"bind" json:"bind"`
	TimeoutSeconds int              `yaml:"timeout_seconds" json:"timeout_seconds"`
	MaxConcurrent  int              `yaml:"max_concurrent" json:"max_concurrent"`
	Principals     []AgentPrincipal `yaml:"principals" json:"principals"`
}

type AgentPrincipal struct {
	ID             string   `yaml:"id" json:"id"`
	BearerTokenEnv string   `yaml:"bearer_token_env" json:"bearer_token_env"`
	ProjectID      string   `yaml:"project_id" json:"project_id"`
	Capabilities   []string `yaml:"capabilities" json:"capabilities"`
}

func resolveAgentServer(input AgentServer) (AgentServer, error) {
	if input.Bind == "" {
		input.Bind = "127.0.0.1:9091"
	}
	if input.TimeoutSeconds == 0 {
		input.TimeoutSeconds = 60
	}
	if input.MaxConcurrent == 0 {
		input.MaxConcurrent = 2
	}
	if err := validateAgentListener(input); err != nil {
		return input, err
	}
	if len(input.Principals) > 8 || (input.Enabled && len(input.Principals) == 0) {
		return input, fmt.Errorf("enabled a2a_server requires one to eight principals")
	}
	input.Principals = slices.Clone(input.Principals)
	seen, credentials := map[string]bool{}, map[string]bool{}
	for i, principal := range input.Principals {
		if err := validateAgentPrincipal(principal); err != nil {
			return input, err
		}
		if seen[principal.ID] || credentials[principal.BearerTokenEnv] {
			return input, fmt.Errorf("a2a_server principals require distinct IDs and credential references")
		}
		seen[principal.ID], credentials[principal.BearerTokenEnv] = true, true
		principal.Capabilities = slices.Clone(principal.Capabilities)
		input.Principals[i] = principal
	}
	return input, nil
}

func validateAgentListener(input AgentServer) error {
	host, port, err := net.SplitHostPort(input.Bind)
	number, portErr := strconv.Atoi(port)
	ip := net.ParseIP(host)
	if err != nil || portErr != nil || number < 1 || number > 65535 || ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("a2a_server.bind requires a literal loopback IP and port 1–65535")
	}
	if input.TimeoutSeconds < 1 || input.TimeoutSeconds > 120 || input.MaxConcurrent < 1 || input.MaxConcurrent > 4 {
		return fmt.Errorf("a2a_server permits 1–120 seconds and 1–4 concurrent executions")
	}
	return nil
}

func validateAgentPrincipal(principal AgentPrincipal) error {
	if !integrationID.MatchString(principal.ID) || !integrationEnvName.MatchString(principal.BearerTokenEnv) {
		return fmt.Errorf("a2a_server principals require a bounded ID and an explicit credential environment reference")
	}
	if principal.ProjectID == "" || len(principal.ProjectID) > 128 || strings.ContainsAny(principal.ProjectID, "\x00\r\n\t ") {
		return fmt.Errorf("a2a_server principal requires a fixed project ID")
	}
	if len(principal.Capabilities) == 0 || len(principal.Capabilities) > 2 {
		return fmt.Errorf("a2a_server capabilities must select analysis, proposal, or both")
	}
	seen := map[string]bool{}
	for _, capability := range principal.Capabilities {
		if (capability != "analysis" && capability != "proposal") || seen[capability] {
			return fmt.Errorf("a2a_server capabilities must be distinct analysis or proposal entries")
		}
		seen[capability] = true
	}
	return nil
}
