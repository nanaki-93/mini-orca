package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"time"
)

type ConnectionKind string

const (
	ConnectionMCP    ConnectionKind = "mcp"
	ConnectionAPI    ConnectionKind = "api"
	ConnectionRemote ConnectionKind = "remote"
	ConnectionAccess ConnectionKind = "access"
)

func ConnectionKinds() []ConnectionKind {
	return []ConnectionKind{ConnectionMCP, ConnectionAPI, ConnectionRemote, ConnectionAccess}
}

type ConnectionPermission struct {
	Name   string `json:"name"`
	Effect string `json:"effect"`
}

// ConnectionConfig is the passive public configuration shared by the inventory
// endpoints. Environment values name credential references, never their values.
type ConnectionConfig struct {
	ID             string                 `json:"id"`
	Transport      string                 `json:"transport,omitempty"`
	Endpoint       string                 `json:"endpoint,omitempty"`
	CardURL        string                 `json:"card_url,omitempty"`
	SpecPath       string                 `json:"spec_path,omitempty"`
	BearerTokenEnv string                 `json:"bearer_token_env,omitempty"`
	Command        string                 `json:"command,omitempty"`
	Args           []string               `json:"args,omitempty"`
	Environment    map[string]string      `json:"environment,omitempty"`
	Tools          []ConnectionPermission `json:"tools,omitempty"`
	Operations     []ConnectionPermission `json:"operations,omitempty"`
	Skills         []string               `json:"skills,omitempty"`
	TimeoutSeconds int                    `json:"timeout_seconds,omitempty"`
	ProjectID      string                 `json:"project_id,omitempty"`
	Capabilities   []string               `json:"capabilities,omitempty"`
}

type ConnectionTool struct {
	RemoteName   string          `json:"remote_name"`
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	InputSchema  json.RawMessage `json:"input_schema"`
	OutputSchema json.RawMessage `json:"output_schema"`
}

type ConnectionOperation struct {
	ID          string `json:"id"`
	Method      string `json:"method"`
	Path        string `json:"path"`
	Effect      string `json:"effect"`
	Description string `json:"description"`
}

type ConnectionSkill struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type ConnectionInventory struct {
	Fingerprint string                `json:"fingerprint"`
	Name        string                `json:"name,omitempty"`
	Description string                `json:"description,omitempty"`
	Version     string                `json:"version,omitempty"`
	Streaming   bool                  `json:"streaming,omitempty"`
	Tools       []ConnectionTool      `json:"tools,omitempty"`
	Operations  []ConnectionOperation `json:"operations,omitempty"`
	Skills      []ConnectionSkill     `json:"skills,omitempty"`
}

type AgentConnection struct {
	Config        ConnectionConfig     `json:"config"`
	Configuration string               `json:"configuration"`
	State         string               `json:"state"`
	Grant         string               `json:"grant,omitempty"`
	ExpiresAt     *time.Time           `json:"expires_at,omitempty"`
	Inventory     *ConnectionInventory `json:"inventory,omitempty"`
}

type ConnectionControl struct {
	ProjectIdentity
	Action                string `json:"action"`
	Configuration         string `json:"configuration"`
	Grant                 string `json:"grant,omitempty"`
	ConfirmConnect        bool   `json:"confirm_connect"`
	ConfirmSourceSharing  bool   `json:"confirm_source_sharing,omitempty"`
	ConfirmRemoteProvider bool   `json:"confirm_remote_provider,omitempty"`
}

var connectionIDPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,23}$`)

func connectionTarget(kind ConnectionKind, scope AgentSettingsScope, id string) (string, error) {
	resources := map[ConnectionKind]string{ConnectionMCP: "integrations", ConnectionAPI: "apis", ConnectionRemote: "remote-agents", ConnectionAccess: "agent-access"}
	resource, ok := resources[kind]
	if !ok || scope.ProjectID == "" || scope.ProjectRevision == "" {
		return "", fmt.Errorf("connection request requires a known kind and current checkout identity")
	}
	if id != "" {
		if !connectionIDPattern.MatchString(id) {
			return "", fmt.Errorf("invalid configured connection ID")
		}
		resource += "/" + id + "/control"
	}
	query := url.Values{"project_id": {scope.ProjectID}, "project_revision": {scope.ProjectRevision}}
	if scope.WorktreePath != "" {
		query.Set("worktree_path", scope.WorktreePath)
	}
	return "/api/projects/current/" + resource + "?" + query.Encode(), nil
}

func (c *Client) ReadConnections(ctx context.Context, id string, scope AgentSettingsScope, kind ConnectionKind) ([]AgentConnection, error) {
	target, err := connectionTarget(kind, scope, "")
	if err != nil {
		return nil, err
	}
	var result []AgentConnection
	if err := c.DoJSON(ctx, id, http.MethodGet, target, nil, &result); err != nil {
		return nil, err
	}
	if len(result) > 8 {
		return nil, fmt.Errorf("connection inventory exceeds configured limits")
	}
	seen := map[string]bool{}
	for _, entry := range result {
		if !connectionIDPattern.MatchString(entry.Config.ID) || seen[entry.Config.ID] || entry.Configuration == "" {
			return nil, fmt.Errorf("connection inventory contains an invalid or repeated identity")
		}
		if kind == ConnectionAccess && entry.Config.ProjectID != scope.ProjectID {
			return nil, fmt.Errorf("incoming principal belongs to another project")
		}
		seen[entry.Config.ID] = true
	}
	return result, nil
}

func (c *Client) ControlConnection(ctx context.Context, requestID string, scope AgentSettingsScope, kind ConnectionKind, id string, request ConnectionControl) (*AgentConnection, error) {
	if id == "" || request.ProjectIdentity != scope.ProjectIdentity {
		return nil, fmt.Errorf("connection control requires the selected checkout and configured ID")
	}
	target, err := connectionTarget(kind, scope, id)
	if err != nil {
		return nil, err
	}
	var result AgentConnection
	if err := c.DoJSON(ctx, requestID, http.MethodPost, target, request, &result); err != nil {
		return nil, err
	}
	if result.Config.ID != id || result.Configuration != request.Configuration {
		return nil, fmt.Errorf("connection response belongs to a different configuration")
	}
	return &result, nil
}
