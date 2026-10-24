package client

type AgentUsage struct {
	ModelCalls           int   `json:"model_calls"`
	ToolCalls            int   `json:"tool_calls"`
	Repairs              int   `json:"repairs"`
	InputTokens          int64 `json:"input_tokens"`
	OutputTokens         int64 `json:"output_tokens"`
	UnknownUsageCalls    int   `json:"unknown_usage_calls"`
	ReservedInputTokens  int64 `json:"reserved_input_tokens"`
	ReservedOutputTokens int64 `json:"reserved_output_tokens"`
	ActiveCalls          int   `json:"active_calls"`
	ElapsedMillis        int64 `json:"elapsed_millis"`
}

type AgentLimits struct {
	ModelCalls      int   `json:"model_calls"`
	ToolCalls       int   `json:"tool_calls"`
	InputTokens     int64 `json:"input_tokens"`
	OutputTokens    int64 `json:"output_tokens"`
	ElapsedSeconds  int   `json:"elapsed_seconds"`
	ConcurrentCalls int   `json:"concurrent_calls"`
	Repairs         int   `json:"repairs"`
}

type AgentCheckpoint struct {
	RequestID    string `json:"request_id"`
	NodeID       string `json:"node_id"`
	InvocationID string `json:"invocation_id"`
	ExpiresAt    string `json:"expires_at"`
	Decision     string `json:"decision,omitempty"`
	ReceivedAt   string `json:"received_at,omitempty"`
}

type AgentRun struct {
	ChangeSessionID string           `json:"change_session_id,omitempty"`
	WorkflowID      string           `json:"workflow_id,omitempty"`
	ID              string           `json:"id"`
	RootID          string           `json:"root_id"`
	ParentID        string           `json:"parent_id,omitempty"`
	ProjectID       string           `json:"project_id"`
	ProjectRevision string           `json:"project_revision"`
	Kind            string           `json:"kind"`
	Status          string           `json:"status"`
	Active          bool             `json:"active"`
	Stale           bool             `json:"stale"`
	CanCancel       bool             `json:"can_cancel"`
	StartedAt       string           `json:"started_at"`
	UpdatedAt       string           `json:"updated_at"`
	Usage           AgentUsage       `json:"usage"`
	Limits          AgentLimits      `json:"limits"`
	Input           *AgentCheckpoint `json:"input,omitempty"`
}

type AgentRunList struct {
	ProjectID       string     `json:"project_id"`
	ProjectRevision string     `json:"project_revision"`
	Runs            []AgentRun `json:"runs"`
	NextCursor      string     `json:"next_cursor,omitempty"`
}

type AgentSessionKey struct {
	App  string `json:"app"`
	User string `json:"user"`
	ID   string `json:"id"`
}

type AgentArtifactReference struct {
	Session AgentSessionKey `json:"session"`
	Name    string          `json:"name"`
	Version int64           `json:"version"`
}

type AgentPrivateText struct {
	ToolRecords    []AgentToolRecord       `json:"tool_records,omitempty"`
	ToolsTruncated bool                    `json:"tools_truncated,omitempty"`
	Available      bool                    `json:"available"`
	State          string                  `json:"state"`
	Text           string                  `json:"text,omitempty"`
	Truncated      bool                    `json:"truncated"`
	Artifact       *AgentArtifactReference `json:"artifact,omitempty"`
}

type AgentToolRecord struct {
	EventID   string `json:"event_id"`
	CallID    string `json:"call_id,omitempty"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Value     string `json:"value"`
	Truncated bool   `json:"truncated"`
}

type AgentRunEvent struct {
	Sequence        uint64                   `json:"sequence"`
	RunID           string                   `json:"run_id"`
	InvocationID    string                   `json:"invocation_id"`
	ParentID        string                   `json:"parent_id,omitempty"`
	ProjectID       string                   `json:"project_id"`
	ProjectRevision string                   `json:"project_revision"`
	NodeID          string                   `json:"node_id,omitempty"`
	Kind            string                   `json:"kind"`
	State           string                   `json:"state"`
	Time            string                   `json:"time"`
	Usage           AgentUsage               `json:"usage"`
	Evidence        []AgentArtifactReference `json:"evidence,omitempty"`
	RequestID       string                   `json:"request_id,omitempty"`
}

type AgentRunDetail struct {
	Run             AgentRun          `json:"run"`
	Invocations     []AgentRun        `json:"invocations"`
	HasMore         bool              `json:"has_more"`
	Events          []AgentRunEvent   `json:"events"`
	EventsTruncated bool              `json:"events_truncated"`
	Input           *ChangeInput      `json:"input,omitempty"`
	Text            *AgentPrivateText `json:"text,omitempty"`
}

type AgentEventBatch struct {
	ProjectID       string          `json:"project_id"`
	ProjectRevision string          `json:"project_revision"`
	Epoch           string          `json:"epoch"`
	Cursor          string          `json:"cursor"`
	Reset           bool            `json:"reset"`
	HasMore         bool            `json:"has_more"`
	Events          []AgentRunEvent `json:"events"`
	Snapshot        *AgentRunList   `json:"snapshot,omitempty"`
}
