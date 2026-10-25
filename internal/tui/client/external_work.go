package client

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"slices"
)

var externalRecordID = regexp.MustCompile(`^[A-Za-z0-9_-]{20,64}$`)

func externalTarget(scope AgentSettingsScope, resource, id, action string) (string, error) {
	if scope.ProjectID == "" || scope.ProjectRevision == "" {
		return "", fmt.Errorf("external work requires a current checkout identity")
	}
	path := "/api/projects/current/" + resource
	if id != "" {
		if !externalRecordID.MatchString(id) {
			return "", fmt.Errorf("invalid external receipt identity")
		}
		path += "/" + id
	}
	if action != "" {
		path += "/" + action
	}
	query := url.Values{"project_id": {scope.ProjectID}, "project_revision": {scope.ProjectRevision}}
	if scope.WorktreePath != "" {
		query.Set("worktree_path", scope.WorktreePath)
	}
	return path + "?" + query.Encode(), nil
}

func (c *Client) externalJSON(ctx context.Context, requestID string, scope AgentSettingsScope, resource, id, action string, body, out any) error {
	target, err := externalTarget(scope, resource, id, action)
	if err != nil {
		return err
	}
	method := http.MethodGet
	if body != nil {
		method = http.MethodPost
	}
	return c.DoJSON(ctx, requestID, method, target, body, out)
}

func externalIdentity(scope AgentSettingsScope, identity ProjectIdentity, id string) error {
	if !externalRecordID.MatchString(id) || identity.ProjectID != scope.ProjectID || identity.ProjectRevision == "" {
		return fmt.Errorf("external receipt is invalid or belongs to another project")
	}
	return nil
}

func validateAPIReceipt(scope AgentSettingsScope, item *APIAction) error {
	if err := externalIdentity(scope, item.ProjectIdentity, item.ID); err != nil {
		return err
	}
	hash := sha256.Sum256(item.Request)
	if item.Version < 1 || len(item.Request) > 24<<10 || !json.Valid(item.Request) || hex.EncodeToString(hash[:]) != item.RequestHash {
		return fmt.Errorf("external request does not match its retained version and hash")
	}
	item.Stale = item.Stale || item.ProjectIdentity != scope.ProjectIdentity
	item.CanApprove = item.CanApprove && !item.Stale
	return nil
}

func (c *Client) ReadAPIActions(ctx context.Context, requestID string, scope AgentSettingsScope) ([]APIAction, error) {
	var result []APIAction
	if err := c.externalJSON(ctx, requestID, scope, "api-actions", "", "", nil, &result); err != nil {
		return nil, err
	}
	if len(result) > 512 {
		return nil, fmt.Errorf("external action history exceeds the retained limit")
	}
	seen := map[string]bool{}
	for i := range result {
		if err := validateAPIReceipt(scope, &result[i]); err != nil {
			return nil, err
		}
		if seen[result[i].ID] {
			return nil, fmt.Errorf("external action history repeats a receipt")
		}
		seen[result[i].ID] = true
	}
	return result, nil
}

func (c *Client) ControlAPIAction(ctx context.Context, requestID string, scope AgentSettingsScope, id string, request APIActionControl) (*APIAction, error) {
	if request.ProjectIdentity != scope.ProjectIdentity || id == "" {
		return nil, fmt.Errorf("external action control scope changed")
	}
	var result APIAction
	if err := c.externalJSON(ctx, requestID, scope, "api-actions", id, "control", request, &result); err != nil {
		return nil, err
	}
	if err := validateAPIReceipt(scope, &result); err != nil {
		return nil, err
	}
	if result.ID != id || result.Version <= request.Version || result.RequestHash != request.RequestHash {
		return nil, fmt.Errorf("external action response does not match the reviewed receipt")
	}
	return &result, nil
}

func (c *Client) ReadResearch(ctx context.Context, requestID string, scope AgentSettingsScope) (*ResearchOverview, error) {
	var result ResearchOverview
	if err := c.externalJSON(ctx, requestID, scope, "research", "", "", nil, &result); err != nil {
		return nil, err
	}
	if !slices.Contains([]string{"available", "disabled", "unsupported_profile"}, result.State) || result.Model == "" {
		return nil, fmt.Errorf("research availability metadata is incomplete")
	}
	seen := map[string]bool{}
	for i := range result.Reports {
		item := &result.Reports[i]
		if err := externalIdentity(scope, item.ProjectIdentity, item.ID); err != nil {
			return nil, err
		}
		if seen[item.ID] {
			return nil, fmt.Errorf("research history repeats a report")
		}
		seen[item.ID] = true
		item.Stale = item.Stale || item.ProjectIdentity != scope.ProjectIdentity
	}
	return &result, nil
}

func (c *Client) RunResearch(ctx context.Context, requestID string, scope AgentSettingsScope, request ResearchRequest) (*ResearchReport, error) {
	if request.ProjectIdentity != scope.ProjectIdentity {
		return nil, fmt.Errorf("research scope changed")
	}
	var result ResearchReport
	if err := c.externalJSON(ctx, requestID, scope, "research", "", "", request, &result); err != nil {
		return nil, err
	}
	if err := externalIdentity(scope, result.ProjectIdentity, result.ID); err != nil {
		return nil, err
	}
	if result.ProjectIdentity != request.ProjectIdentity {
		return nil, fmt.Errorf("research returned a different source revision")
	}
	return &result, nil
}

func (c *Client) ReadDelegations(ctx context.Context, requestID string, scope AgentSettingsScope) ([]Delegation, error) {
	var result []Delegation
	if err := c.externalJSON(ctx, requestID, scope, "delegations", "", "", nil, &result); err != nil {
		return nil, err
	}
	if len(result) > 128 {
		return nil, fmt.Errorf("delegation history exceeds the retained limit")
	}
	seen := map[string]bool{}
	for i := range result {
		if err := validateDelegationReceipt(scope, &result[i]); err != nil {
			return nil, err
		}
		if seen[result[i].ID] {
			return nil, fmt.Errorf("delegation history repeats a receipt")
		}
		seen[result[i].ID] = true
	}
	return result, nil
}

func validateDelegationReceipt(scope AgentSettingsScope, result *Delegation) error {
	if err := externalIdentity(scope, result.ProjectIdentity, result.ID); err != nil {
		return err
	}
	if result.Version < 1 || !result.Advisory {
		return fmt.Errorf("remote receipt lacks its version or advisory provenance")
	}
	result.Stale = result.Stale || result.ProjectIdentity != scope.ProjectIdentity
	return nil
}

func (c *Client) Delegation(ctx context.Context, requestID string, scope AgentSettingsScope, id string) (*Delegation, error) {
	if id == "" {
		return nil, fmt.Errorf("select a retained delegation")
	}
	return c.delegation(ctx, requestID, scope, id, "", nil)
}

func (c *Client) StartDelegation(ctx context.Context, requestID string, scope AgentSettingsScope, request DelegationRequest) (*Delegation, error) {
	if request.ProjectIdentity != scope.ProjectIdentity {
		return nil, fmt.Errorf("delegation scope changed")
	}
	result, err := c.delegation(ctx, requestID, scope, "", "", request)
	if err == nil && (result.ProjectIdentity != request.ProjectIdentity || result.RemoteID != request.Remote.ID || result.Kind != request.Kind || result.Skill != request.Skill) {
		return nil, fmt.Errorf("delegation receipt differs from the captured destination or scope")
	}
	return result, err
}

func (c *Client) ControlDelegation(ctx context.Context, requestID string, scope AgentSettingsScope, id string, request DelegationControl) (*Delegation, error) {
	if request.ProjectIdentity != scope.ProjectIdentity || id == "" {
		return nil, fmt.Errorf("delegation control scope changed")
	}
	result, err := c.delegation(ctx, requestID, scope, id, "control", request)
	if err == nil && result.Version <= request.Version {
		return nil, fmt.Errorf("delegation response did not advance the reviewed version")
	}
	return result, err
}

func (c *Client) delegation(ctx context.Context, requestID string, scope AgentSettingsScope, id, action string, body any) (*Delegation, error) {
	var result Delegation
	if err := c.externalJSON(ctx, requestID, scope, "delegations", id, action, body, &result); err != nil {
		return nil, err
	}
	if err := validateDelegationReceipt(scope, &result); err != nil {
		return nil, err
	}
	if id != "" && result.ID != id {
		return nil, fmt.Errorf("delegation response belongs to another receipt")
	}
	return &result, nil
}

func (c *Client) ExternalIndex(ctx context.Context, requestID string, scope AgentSettingsScope) (*ProjectIndex, error) {
	var result ProjectIndex
	if err := c.externalJSON(ctx, requestID, scope, "index", "", "", nil, &result); err != nil {
		return nil, err
	}
	if result.ProjectIdentity != scope.ProjectIdentity {
		return nil, fmt.Errorf("source index belongs to another checkout revision")
	}
	return &result, nil
}

func (c *Client) validateLoopbackBrowserOrigin() error {
	origin, err := url.Parse(c.base)
	if err != nil || origin == nil {
		return fmt.Errorf("report browser requires a loopback daemon origin")
	}
	ip := net.ParseIP(origin.Hostname())
	if origin.Hostname() != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return fmt.Errorf("report browser requires a loopback daemon origin")
	}
	return nil
}

// ResearchReportURL only constructs a local, identity-guarded report location.
// It neither fetches the report nor permits caller-supplied destinations.
func (c *Client) ResearchReportURL(scope AgentSettingsScope, id string) (string, error) {
	if err := c.validateLoopbackBrowserOrigin(); err != nil {
		return "", err
	}
	if id == "" {
		return "", fmt.Errorf("select a retained research report")
	}
	target, err := externalTarget(scope, "research", id, "report")
	if err != nil {
		return "", err
	}
	return c.base + target, nil
}

// WebUIURL constructs a local loopback URL for the daemon's embedded Web UI and diff viewer.
func (c *Client) WebUIURL(tab, sessionID, filePath, worktreePath string) (string, error) {
	if err := c.validateLoopbackBrowserOrigin(); err != nil {
		return "", err
	}
	query := url.Values{}
	if tab != "" && tab != "diff" {
		query.Set("tab", tab)
	}
	if sessionID != "" {
		query.Set("session_id", sessionID)
	}
	if filePath != "" {
		query.Set("file", filePath)
	}
	if worktreePath != "" {
		query.Set("worktree_path", worktreePath)
	}
	target := c.base + "/ui"
	if encoded := query.Encode(); encoded != "" {
		target += "?" + encoded
	}
	return target, nil
}
