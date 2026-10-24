package client

import (
	"encoding/json"
	"time"
)

// External receipts are private review data. They do not grant local source or
// execution authority, and reported remote usage is not locally measured usage.
type APIAction struct {
	ProjectIdentity
	ID              string          `json:"id"`
	Version         int64           `json:"version"`
	IntegrationID   string          `json:"integration_id"`
	Inventory       string          `json:"inventory"`
	Scope           string          `json:"scope"`
	OperationKey    string          `json:"operation_key"`
	Request         json.RawMessage `json:"request"`
	RequestHash     string          `json:"request_hash"`
	State           string          `json:"state"`
	HTTPStatus      int             `json:"http_status,omitempty"`
	Response        json.RawMessage `json:"response,omitempty"`
	ResponseHash    string          `json:"response_hash,omitempty"`
	ReportedOutcome string          `json:"reported_outcome,omitempty"`
	Observation     string          `json:"observation,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
	ExpiresAt       time.Time       `json:"expires_at"`
	PendingUntil    time.Time       `json:"pending_until,omitempty"`
	CanApprove      bool            `json:"can_approve"`
	CanReconcile    bool            `json:"can_reconcile"`
	Stale           bool            `json:"stale"`
}

type APIActionControl struct {
	ProjectIdentity
	Action          string `json:"action"`
	Version         int64  `json:"version"`
	RequestHash     string `json:"request_hash"`
	Confirm         bool   `json:"confirm"`
	ReportedOutcome string `json:"reported_outcome,omitempty"`
	Observation     string `json:"observation,omitempty"`
}

type ExternalSource struct {
	Path string `json:"path"`
	Hash string `json:"hash"`
}

type ResearchRequest struct {
	ProjectIdentity
	Question              string           `json:"question"`
	Sources               []ExternalSource `json:"sources"`
	ConfirmRemoteProvider bool             `json:"confirm_remote_provider"`
	ConfirmSearch         bool             `json:"confirm_search"`
	ConfirmSourceSharing  bool             `json:"confirm_source_sharing"`
}

type ResearchReport struct {
	ProjectIdentity
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	State     string    `json:"state"`
	Model     string    `json:"model"`
	CreatedAt time.Time `json:"created_at"`
	Stale     bool      `json:"stale"`
}

type ResearchOverview struct {
	State   string           `json:"state"`
	Model   string           `json:"model"`
	Reports []ResearchReport `json:"reports"`
}

type DelegationRequest struct {
	ProjectIdentity
	Remote          IntegrationUse   `json:"remote"`
	Kind            string           `json:"kind"`
	Skill           string           `json:"skill"`
	Question        string           `json:"question"`
	Sources         []ExternalSource `json:"sources"`
	Stream          bool             `json:"stream"`
	ConfirmDelegate bool             `json:"confirm_delegate"`
}

type DelegationControl struct {
	ProjectIdentity
	Version     int64          `json:"version"`
	Action      string         `json:"action"`
	Remote      IntegrationUse `json:"remote"`
	Reply       string         `json:"reply,omitempty"`
	Observation string         `json:"observation,omitempty"`
	Stream      bool           `json:"stream"`
	Confirm     bool           `json:"confirm"`
}

type RemoteArtifact struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Text     string `json:"text"`
	Complete bool   `json:"complete"`
}

type RemoteUsage struct {
	InputTokens  int64  `json:"input_tokens"`
	OutputTokens int64  `json:"output_tokens"`
	Source       string `json:"source"`
}

type RemoteProgress struct {
	TaskID    string           `json:"task_id,omitempty"`
	ContextID string           `json:"context_id,omitempty"`
	State     string           `json:"state"`
	Text      string           `json:"text,omitempty"`
	Artifacts []RemoteArtifact `json:"artifacts"`
	Usage     RemoteUsage      `json:"usage"`
}

type Delegation struct {
	ProjectIdentity
	ID              string           `json:"id"`
	Version         int64            `json:"version"`
	RemoteID        string           `json:"remote_id"`
	EndpointHash    string           `json:"endpoint_hash"`
	CardFingerprint string           `json:"card_fingerprint"`
	Skill           string           `json:"skill"`
	Kind            string           `json:"kind"`
	Question        string           `json:"question"`
	Sources         []ExternalSource `json:"sources"`
	InputHash       string           `json:"input_hash"`
	MessageID       string           `json:"message_id"`
	Operation       string           `json:"operation"`
	State           string           `json:"state"`
	Progress        RemoteProgress   `json:"progress"`
	Observation     string           `json:"observation,omitempty"`
	CreatedAt       time.Time        `json:"created_at"`
	UpdatedAt       time.Time        `json:"updated_at"`
	LeaseUntil      time.Time        `json:"lease_until"`
	Stale           bool             `json:"stale"`
	Active          bool             `json:"active"`
	Advisory        bool             `json:"advisory"`
}
