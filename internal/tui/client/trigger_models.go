package client

import "time"

type TriggerBinding struct {
	ProjectIdentity
	Configuration   string `json:"configuration"`
	SettingsVersion int64  `json:"settings_version"`
	Policy          string `json:"policy"`
	Model           string `json:"model"`
	ProviderOrigin  string `json:"provider_origin"`
}

type AgentTriggers struct {
	TriggerBinding
	State       string          `json:"state"`
	Owned       bool            `json:"owned"`
	Fault       bool            `json:"fault"`
	ScopeHash   string          `json:"scope_hash"`
	Record      TriggerRecord   `json:"record"`
	Ceilings    TriggerCeilings `json:"ceilings"`
	RunLimits   AgentLimits     `json:"run_limits"`
	Destination Model           `json:"destination"`
}

type TriggerControl struct {
	TriggerBinding
	ControlRevision       int64           `json:"control_revision"`
	Action                string          `json:"action"`
	Confirm               bool            `json:"confirm"`
	Options               *TriggerOptions `json:"options,omitempty"`
	ScopeHash             string          `json:"scope_hash,omitempty"`
	GrantSeconds          int             `json:"grant_seconds,omitempty"`
	ConfirmSourceSharing  bool            `json:"confirm_source_sharing,omitempty"`
	ConfirmRemoteProvider bool            `json:"confirm_remote_provider,omitempty"`
}

type TriggerOptions struct {
	Mode            string   `json:"mode"`
	IntervalSeconds int      `json:"interval_seconds"`
	Paths           []string `json:"paths"`
}

type TriggerCeilings struct {
	PollSeconds     int   `json:"poll_seconds"`
	DebounceSeconds int   `json:"debounce_seconds"`
	QuietSeconds    int   `json:"quiet_seconds"`
	WindowSeconds   int   `json:"window_seconds"`
	MaxRuns         int   `json:"max_runs"`
	MaxModelCalls   int   `json:"max_model_calls"`
	MaxInputTokens  int64 `json:"max_input_tokens"`
	MaxOutputTokens int64 `json:"max_output_tokens"`
	RunSeconds      int   `json:"run_seconds"`
	GrantSeconds    int   `json:"grant_seconds"`
}

type TriggerAllowance struct {
	Runs         int   `json:"runs"`
	ModelCalls   int   `json:"model_calls"`
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
	Seconds      int   `json:"seconds"`
}

type TriggerWindow struct {
	StartedAt time.Time        `json:"started_at"`
	EndsAt    time.Time        `json:"ends_at"`
	Limits    TriggerAllowance `json:"limits"`
	Charged   TriggerAllowance `json:"charged"`
}

type TriggerGrant struct {
	ID              string      `json:"id"`
	ProjectID       string      `json:"project_id"`
	Configuration   string      `json:"configuration"`
	SettingsVersion int64       `json:"settings_version"`
	Policy          string      `json:"policy"`
	ScopeHash       string      `json:"scope_hash"`
	Model           string      `json:"model"`
	ProviderOrigin  string      `json:"provider_origin"`
	CreatedAt       time.Time   `json:"created_at"`
	ExpiresAt       time.Time   `json:"expires_at"`
	RunLimits       AgentLimits `json:"run_limits"`
}

type TriggerAttempt struct {
	ID              string           `json:"id"`
	ProjectRevision string           `json:"project_revision"`
	Path            string           `json:"path"`
	SourceHash      string           `json:"source_hash"`
	State           string           `json:"state"`
	StartedAt       time.Time        `json:"started_at"`
	FinishedAt      time.Time        `json:"finished_at"`
	Reserved        TriggerAllowance `json:"reserved"`
	Usage           AgentUsage       `json:"usage"`
}

type TriggerRecord struct {
	Version         int64             `json:"version"`
	ControlRevision int64             `json:"control_revision"`
	ProjectID       string            `json:"project_id"`
	Options         TriggerOptions    `json:"options"`
	Grant           *TriggerGrant     `json:"grant"`
	Window          TriggerWindow     `json:"window"`
	Observed        map[string]string `json:"observed"`
	Handled         map[string]string `json:"handled"`
	ChangedAt       time.Time         `json:"changed_at"`
	NextEligibleAt  time.Time         `json:"next_eligible_at"`
	LastAttempt     *TriggerAttempt   `json:"last_attempt"`
	State           string            `json:"state"`
	Reason          string            `json:"reason"`
	UpdatedAt       time.Time         `json:"updated_at"`
}
