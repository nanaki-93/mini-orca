package config

import (
	"fmt"
	"net/url"
	"slices"
	"strings"
)

// RemoteAgent is passive operator configuration, never a permission inferred
// from an agent card or a remotely supplied task.
type RemoteAgent struct {
	ID             string   `yaml:"id" json:"id"`
	CardURL        string   `yaml:"card_url" json:"card_url"`
	Endpoint       string   `yaml:"endpoint" json:"endpoint"`
	BearerTokenEnv string   `yaml:"bearer_token_env" json:"bearer_token_env,omitempty"`
	Skills         []string `yaml:"skills" json:"skills"`
	TimeoutSeconds int      `yaml:"timeout_seconds" json:"timeout_seconds"`
}

func resolveRemoteAgents(input []RemoteAgent, seen map[string]bool) ([]RemoteAgent, error) {
	if len(input) > 8 {
		return nil, fmt.Errorf("integrations.a2a permits at most eight remote agents")
	}
	result := slices.Clone(input)
	for i, remote := range result {
		if !integrationID.MatchString(remote.ID) || seen[remote.ID] {
			return nil, fmt.Errorf("remote agent IDs must be unique across integrations")
		}
		seen[remote.ID] = true
		if remote.TimeoutSeconds == 0 {
			remote.TimeoutSeconds = 30
		}
		if err := validateRemoteAgent(remote); err != nil {
			return nil, fmt.Errorf("remote agent %s: %w", remote.ID, err)
		}
		remote.Skills = slices.Clone(remote.Skills)
		result[i] = remote
	}
	return result, nil
}

func validateRemoteAgent(remote RemoteAgent) error {
	if remote.TimeoutSeconds < 1 || remote.TimeoutSeconds > 60 {
		return fmt.Errorf("remote agent timeout must be 1–60 seconds")
	}
	for _, endpoint := range []string{remote.CardURL, remote.Endpoint} {
		if err := validateIntegrationEndpoint(endpoint); err != nil {
			return err
		}
		u, _ := url.Parse(endpoint)
		if u.RawPath != "" || strings.ContainsAny(u.Path, "{}\\") || strings.Contains(u.Path, "..") {
			return fmt.Errorf("remote agent URLs require fixed unescaped paths")
		}
	}
	card, _ := url.Parse(remote.CardURL)
	endpoint, _ := url.Parse(remote.Endpoint)
	if card.Scheme != endpoint.Scheme || card.Host != endpoint.Host {
		return fmt.Errorf("agent card and RPC endpoint must have the same exact origin")
	}
	if remote.BearerTokenEnv != "" && !integrationEnvName.MatchString(remote.BearerTokenEnv) {
		return fmt.Errorf("bearer_token_env must name an environment variable")
	}
	return validateRemoteSkills(remote.Skills)
}

func validateRemoteSkills(skills []string) error {
	if len(skills) < 1 || len(skills) > 4 {
		return fmt.Errorf("allowlist one to four remote skill IDs")
	}
	seen := map[string]bool{}
	for _, skill := range skills {
		if !integrationToolName.MatchString(skill) || seen[skill] {
			return fmt.Errorf("remote skill IDs must be distinct bounded identifiers")
		}
		seen[skill] = true
	}
	return nil
}
