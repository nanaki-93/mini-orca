package client

import "time"

type LiveBinding struct {
	ProjectIdentity
	Configuration   string `json:"configuration"`
	SettingsVersion int64  `json:"settings_version"`
	Model           string `json:"model"`
	ProviderOrigin  string `json:"provider_origin"`
}
type LiveOpenRequest struct {
	LiveBinding
	ConfirmConnect        bool `json:"confirm_connect"`
	ConfirmRemoteProvider bool `json:"confirm_remote_provider"`
}
type LiveControl struct {
	LiveBinding
	ControlRevision       int64                `json:"control_revision"`
	Action                string               `json:"action"`
	Text                  string               `json:"text,omitempty"`
	Attachment            *AttachmentReference `json:"attachment,omitempty"`
	ConfirmConnect        bool                 `json:"confirm_connect"`
	ConfirmSourceSharing  bool                 `json:"confirm_source_sharing"`
	ConfirmRemoteProvider bool                 `json:"confirm_remote_provider"`
}
type LiveSession struct {
	LiveBinding
	ID               string                   `json:"id"`
	Version          int64                    `json:"version"`
	ControlRevision  int64                    `json:"control_revision"`
	Policy           string                   `json:"policy"`
	State            string                   `json:"state"`
	Connections      int                      `json:"connections"`
	Turns            int                      `json:"turns"`
	CompletedTurns   int                      `json:"completed_turns"`
	SentBytes        int64                    `json:"sent_bytes"`
	ReceivedBytes    int64                    `json:"received_bytes"`
	ReportedInput    int64                    `json:"reported_input_tokens"`
	ReportedOutput   int64                    `json:"reported_output_tokens"`
	Reported         bool                     `json:"usage_reported"`
	Usage            AgentUsage               `json:"usage"`
	RunIDs           []string                 `json:"run_ids"`
	Artifacts        []AgentArtifactReference `json:"artifacts"`
	CreatedAt        time.Time                `json:"created_at"`
	UpdatedAt        time.Time                `json:"updated_at"`
	ExpiresAt        time.Time                `json:"expires_at"`
	Owned            bool                     `json:"owned"`
	Stale            bool                     `json:"stale"`
	CanReconnect     bool                     `json:"can_reconnect"`
	RetentionFailed  bool                     `json:"retention_failed"`
	OutputArtifacts  []AgentArtifact          `json:"output_artifacts"`
	MissingArtifacts int                      `json:"missing_artifacts"`
}
type LiveOverview struct {
	LiveBinding
	State                 string        `json:"state"`
	Destination           Model         `json:"destination"`
	Limits                AgentLimits   `json:"limits"`
	LifetimeSeconds       int           `json:"lifetime_seconds"`
	MaxConnections        int           `json:"max_connections"`
	MaxTurnsPerConnection int           `json:"max_turns_per_connection"`
	MaxInputBytes         int           `json:"max_input_bytes"`
	MaxOutputBytes        int           `json:"max_output_bytes"`
	Sessions              []LiveSession `json:"sessions"`
}
