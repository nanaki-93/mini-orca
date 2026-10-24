package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

type AgentRoles struct {
	Creation string `json:"creation"`
	Testing  string `json:"testing"`
	Review   string `json:"review"`
}

type AgentPreferences struct {
	Roles          AgentRoles  `json:"roles"`
	Limits         AgentLimits `json:"limits"`
	DisabledSkills []string    `json:"disabled_skills"`
}

type AgentSkillSetting struct {
	Directory string `json:"directory"`
	Enabled   bool   `json:"enabled"`
}

type AgentSettings struct {
	ProjectIdentity
	Version       int64               `json:"version"`
	Configuration string              `json:"configuration"`
	State         string              `json:"state"`
	Roles         AgentRoles          `json:"roles"`
	Limits        AgentLimits         `json:"limits"`
	Ceilings      AgentLimits         `json:"ceilings"`
	Skills        []AgentSkillSetting `json:"skills"`
	Models        []Model             `json:"models"`
}

type AgentSettingsControl struct {
	ProjectIdentity
	Version       int64             `json:"version"`
	Configuration string            `json:"configuration"`
	Action        string            `json:"action"`
	Confirm       bool              `json:"confirm"`
	Values        *AgentPreferences `json:"values,omitempty"`
}

type AgentSkillValidationRequest struct {
	ProjectIdentity
	Version       int64  `json:"version"`
	Configuration string `json:"configuration"`
	Directory     string `json:"directory"`
}

type AgentSkillFile struct {
	Path      string `json:"path"`
	Hash      string `json:"hash"`
	SizeBytes int64  `json:"size_bytes"`
}

type AgentSkillInspection struct {
	Name          string           `json:"name"`
	Description   string           `json:"description"`
	License       string           `json:"license,omitempty"`
	Compatibility string           `json:"compatibility,omitempty"`
	RequiredTools []string         `json:"required_tools"`
	Manifest      AgentSkillFile   `json:"manifest"`
	Resources     []AgentSkillFile `json:"resources"`
}

type AgentSkillValidation struct {
	AgentSkillValidationRequest
	State      string                `json:"state"`
	Inspection *AgentSkillInspection `json:"inspection"`
}

type AgentSettingsScope struct {
	ProjectIdentity
	WorktreePath string
}

func (scope AgentSettingsScope) target(suffix string) (string, error) {
	if scope.ProjectID == "" || scope.ProjectRevision == "" {
		return "", fmt.Errorf("settings require a current checkout identity")
	}
	query := url.Values{"project_id": {scope.ProjectID}, "project_revision": {scope.ProjectRevision}}
	if scope.WorktreePath != "" {
		query.Set("worktree_path", scope.WorktreePath)
	}
	return "/api/projects/current/agent-settings" + suffix + "?" + query.Encode(), nil
}

func (c *Client) ReadAgentSettings(ctx context.Context, id string, scope AgentSettingsScope) (*AgentSettings, error) {
	return c.agentSettings(ctx, id, scope, "", nil)
}

func (c *Client) ControlAgentSettings(ctx context.Context, id string, scope AgentSettingsScope, request AgentSettingsControl) (*AgentSettings, error) {
	if request.ProjectIdentity != scope.ProjectIdentity {
		return nil, fmt.Errorf("settings control scope changed")
	}
	return c.agentSettings(ctx, id, scope, "/control", request)
}

func (c *Client) agentSettings(ctx context.Context, id string, scope AgentSettingsScope, suffix string, body any) (*AgentSettings, error) {
	target, err := scope.target(suffix)
	if err != nil {
		return nil, err
	}
	method := http.MethodGet
	if body != nil {
		method = http.MethodPost
	}
	var result AgentSettings
	if err := c.DoJSON(ctx, id, method, target, body, &result); err != nil {
		return nil, err
	}
	if result.ProjectIdentity != scope.ProjectIdentity || result.Configuration == "" || result.Version < 0 || (result.State != "configured" && result.State != "overridden" && result.State != "stale") {
		return nil, fmt.Errorf("settings response is unavailable or belongs to another checkout revision")
	}
	return &result, nil
}

func (c *Client) ValidateAgentSkill(ctx context.Context, id string, scope AgentSettingsScope, request AgentSkillValidationRequest) (*AgentSkillValidation, error) {
	target, err := scope.target("/skills/validate")
	if err != nil {
		return nil, err
	}
	if request.ProjectIdentity != scope.ProjectIdentity {
		return nil, fmt.Errorf("skill validation scope changed")
	}
	var result AgentSkillValidation
	if err := c.DoJSON(ctx, id, http.MethodPost, target, request, &result); err != nil {
		return nil, err
	}
	if result.AgentSkillValidationRequest != request || result.State != "valid" || result.Inspection == nil {
		return nil, fmt.Errorf("skill validation did not match the captured settings")
	}
	return &result, nil
}
