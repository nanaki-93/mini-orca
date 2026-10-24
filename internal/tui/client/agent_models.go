package client

type MissionSource struct {
	Path   string `json:"path"`
	Hash   string `json:"hash"`
	Exists bool   `json:"exists"`
}

type MissionChild struct {
	ID                 string      `json:"id"`
	Title              string      `json:"title"`
	Approach           string      `json:"approach"`
	AcceptanceCriteria []string    `json:"acceptance_criteria"`
	Dependencies       []string    `json:"dependencies"`
	WritePaths         []string    `json:"write_paths"`
	ReadPaths          []string    `json:"read_paths"`
	Limits             AgentLimits `json:"limits"`
}

type MissionPlan struct {
	Goal             string          `json:"goal"`
	Sources          []MissionSource `json:"sources"`
	Limits           AgentLimits     `json:"limits"`
	Children         []MissionChild  `json:"children"`
	IntegrationOrder []string        `json:"integration_order"`
}

type MissionRecord struct {
	ProjectIdentity
	ID            string            `json:"id"`
	Policy        string            `json:"policy"`
	Configuration string            `json:"configuration"`
	Instructions  map[string]string `json:"instructions"`
	Plan          MissionPlan       `json:"plan"`
	PlanHash      string            `json:"plan_hash"`
	Reviewed      []string          `json:"reviewed"`
	Approved      bool              `json:"approved"`
	CreatedAt     string            `json:"created_at"`
	UpdatedAt     string            `json:"updated_at"`
}

type AgentMission struct {
	CatalogRevision int64         `json:"catalog_revision"`
	Mission         MissionRecord `json:"mission"`
	Stale           bool          `json:"stale"`
	SourceStatus    string        `json:"source_status"`
}

type MissionExecutionChild struct {
	ID               string      `json:"id"`
	State            string      `json:"state"`
	Limits           AgentLimits `json:"limits"`
	WaveID           string      `json:"wave_id,omitempty"`
	Directory        string      `json:"directory,omitempty"`
	Branch           string      `json:"branch,omitempty"`
	BaseHead         string      `json:"base_head,omitempty"`
	ProjectID        string      `json:"project_id,omitempty"`
	ProjectRevision  string      `json:"project_revision,omitempty"`
	SessionID        string      `json:"session_id,omitempty"`
	WorkflowID       string      `json:"workflow_id,omitempty"`
	ProposalHash     string      `json:"proposal_hash,omitempty"`
	ProposalRevision int64       `json:"proposal_revision,omitempty"`
	Reason           string      `json:"reason,omitempty"`
	UpdatedAt        string      `json:"updated_at,omitempty"`
}

type MissionChildOrigin struct {
	MissionID   string `json:"mission_id"`
	ExecutionID string `json:"execution_id"`
	ChildID     string `json:"child_id"`
	PlanHash    string `json:"plan_hash"`
}

type MissionWave struct {
	ID              string   `json:"id"`
	Owner           string   `json:"owner"`
	Children        []string `json:"children"`
	StartedAt       string   `json:"started_at"`
	AllowanceMillis int64    `json:"allowance_millis"`
}

type MissionExecution struct {
	ID              string                  `json:"id"`
	Revision        int64                   `json:"execution_revision"`
	CatalogRevision int64                   `json:"catalog_revision"`
	Mission         MissionRecord           `json:"mission"`
	Base            GitIdentity             `json:"base"`
	State           string                  `json:"state"`
	ElapsedMillis   int64                   `json:"elapsed_millis,omitempty"`
	Wave            *MissionWave            `json:"wave,omitempty"`
	Failure         string                  `json:"failure,omitempty"`
	Children        []MissionExecutionChild `json:"children"`
	CreatedAt       string                  `json:"created_at"`
	UpdatedAt       string                  `json:"updated_at"`
}

type AgentMissionExecution struct {
	Execution     MissionExecution `json:"execution"`
	Stale         bool             `json:"stale"`
	SourceStatus  string           `json:"source_status"`
	ReadyChildren []string         `json:"ready_children"`
	OwnerState    string           `json:"owner_state"`
}

type MissionExecutionRequest struct {
	ProjectIdentity
	Action            string       `json:"action"`
	CatalogRevision   int64        `json:"catalog_revision"`
	PlanHash          string       `json:"plan_hash"`
	ExecutionID       string       `json:"execution_id,omitempty"`
	ExecutionRevision int64        `json:"execution_revision"`
	Base              *GitIdentity `json:"base,omitempty"`
	Confirm           bool         `json:"confirm"`
	ConfirmExecution  bool         `json:"confirm_execution,omitempty"`
	ConfirmedProfiles []string     `json:"confirmed_profiles,omitempty"`
}

type MissionSummary struct {
	ID              string `json:"id"`
	ProjectRevision string `json:"project_revision"`
	Goal            string `json:"goal"`
	PlanHash        string `json:"plan_hash"`
	ChildCount      int    `json:"child_count"`
	ReviewedCount   int    `json:"reviewed_count"`
	Approved        bool   `json:"approved"`
	Stale           bool   `json:"stale"`
	SourceStatus    string `json:"source_status"`
	UpdatedAt       string `json:"updated_at"`
}

type MissionList struct {
	ProjectIdentity
	CatalogRevision int64            `json:"catalog_revision"`
	Missions        []MissionSummary `json:"missions"`
}

type MissionMutation struct {
	ProjectIdentity
	CatalogRevision int64        `json:"catalog_revision"`
	PlanHash        string       `json:"plan_hash"`
	Action          string       `json:"action"`
	ChildID         string       `json:"child_id,omitempty"`
	Confirm         bool         `json:"confirm"`
	Plan            *MissionPlan `json:"plan,omitempty"`
}

type MissionPlanningRequest struct {
	ProjectIdentity
	CatalogRevision       int64           `json:"catalog_revision"`
	Goal                  string          `json:"goal"`
	Sources               []MissionSource `json:"sources"`
	Limits                AgentLimits     `json:"limits"`
	PlanningLimits        AgentLimits     `json:"planning_limits"`
	ConfirmRemoteProvider bool            `json:"confirm_remote_provider"`
}

type MissionPlanningResult struct {
	AgentMission
	InputHash       string          `json:"input_hash"`
	PromptVersion   string          `json:"prompt_version"`
	PlanningLimits  AgentLimits     `json:"planning_limits"`
	Usage           AgentUsage      `json:"usage"`
	ContextManifest ContextManifest `json:"context_manifest"`
}

type ContextManifest struct {
	Included        []ContextFile     `json:"included"`
	Excluded        []ContextDecision `json:"excluded"`
	EstimatedTokens int               `json:"estimated_tokens"`
	ByteLimit       int               `json:"byte_limit"`
	TokenLimit      int               `json:"token_limit"`
	Truncated       bool              `json:"truncated"`
	Scope           string            `json:"scope,omitempty"`
	Model           string            `json:"model,omitempty"`
	ProviderOrigin  string            `json:"provider_origin,omitempty"`
	RemoteProvider  bool              `json:"remote_provider,omitempty"`
}

type ContextFile struct {
	Path      string `json:"path"`
	SizeBytes int64  `json:"size_bytes"`
	Hash      string `json:"hash"`
	Tokens    int    `json:"estimated_tokens"`
	Truncated bool   `json:"truncated"`
}

type ContextDecision struct {
	Path    string `json:"path"`
	Include bool   `json:"include"`
	Reason  string `json:"reason"`
}

type AgentUsage struct {
	Accounting           *AgentUsageAccounting `json:"accounting,omitempty"`
	ModelCalls           int                   `json:"model_calls"`
	ToolCalls            int                   `json:"tool_calls"`
	Repairs              int                   `json:"repairs"`
	InputTokens          int64                 `json:"input_tokens"`
	OutputTokens         int64                 `json:"output_tokens"`
	UnknownUsageCalls    int                   `json:"unknown_usage_calls"`
	ReservedInputTokens  int64                 `json:"reserved_input_tokens"`
	ReservedOutputTokens int64                 `json:"reserved_output_tokens"`
	ActiveCalls          int                   `json:"active_calls"`
	ElapsedMillis        int64                 `json:"elapsed_millis"`
}

type AgentUsageAccounting struct {
	Version   int                  `json:"version"`
	Providers []AgentProviderUsage `json:"providers"`
}

type AgentProviderUsage struct {
	ModelKey             string `json:"model_key"`
	ModelCalls           int    `json:"model_calls"`
	ActiveCalls          int    `json:"active_calls"`
	FailedCalls          int    `json:"failed_calls"`
	UnknownUsageCalls    int    `json:"unknown_usage_calls"`
	ReportedCalls        int    `json:"reported_calls"`
	PartialUsageCalls    int    `json:"partial_usage_calls"`
	CacheKnownCalls      int    `json:"cache_known_calls"`
	InputTokens          int64  `json:"input_tokens"`
	OutputTokens         int64  `json:"output_tokens"`
	ReportedInputTokens  int64  `json:"reported_input_tokens"`
	ReportedOutputTokens int64  `json:"reported_output_tokens"`
	CachedInputTokens    int64  `json:"cached_input_tokens"`
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
	Trace           *AgentTraceSummary `json:"trace,omitempty"`
	Cost            *AgentCostEstimate `json:"cost,omitempty"`
	Run             AgentRun           `json:"run"`
	Invocations     []AgentRun         `json:"invocations"`
	HasMore         bool               `json:"has_more"`
	Events          []AgentRunEvent    `json:"events"`
	EventsTruncated bool               `json:"events_truncated"`
	Input           *ChangeInput       `json:"input,omitempty"`
	Text            *AgentPrivateText  `json:"text,omitempty"`
}

type AgentTraceSpan struct {
	ID           string `json:"id"`
	ParentID     string `json:"parent_id,omitempty"`
	RunKey       string `json:"run_key,omitempty"`
	OperationKey string `json:"operation_key,omitempty"`
	Kind         string `json:"kind"`
	Status       string `json:"status"`
	Started      string `json:"started"`
	Ended        string `json:"ended,omitempty"`
	ElapsedNanos int64  `json:"elapsed_nanos"`
}

type AgentTraceSummary struct {
	Export     *AgentTraceExportPreview `json:"export,omitempty"`
	Version    int                      `json:"version"`
	TraceID    string                   `json:"trace_id"`
	Snapshot   string                   `json:"snapshot"`
	Legacy     bool                     `json:"legacy"`
	Truncated  bool                     `json:"truncated"`
	Incomplete bool                     `json:"incomplete"`
	Omitted    uint64                   `json:"omitted"`
	Spans      []AgentTraceSpan         `json:"spans"`
}

type AgentCostEstimate struct {
	Status        string   `json:"status"`
	Currency      string   `json:"currency,omitempty"`
	PriceVersion  string   `json:"price_version,omitempty"`
	KnownSubtotal float64  `json:"known_subtotal"`
	Total         *float64 `json:"total,omitempty"`
	Reasons       []string `json:"reasons"`
}

type AgentTraceExportPreview struct {
	Enabled     bool   `json:"enabled"`
	Destination string `json:"destination,omitempty"`
	Fingerprint string `json:"fingerprint,omitempty"`
	Finished    int    `json:"finished"`
	Unfinished  int    `json:"unfinished"`
	Reason      string `json:"reason,omitempty"`
}

type AgentTraceExportRequest struct {
	ProjectIdentity
	Snapshot               string `json:"snapshot"`
	DestinationFingerprint string `json:"destination_fingerprint"`
	Confirm                bool   `json:"confirm"`
}

type AgentTraceExportReceipt struct {
	ProjectIdentity
	RunID                  string `json:"run_id"`
	Snapshot               string `json:"snapshot"`
	DestinationFingerprint string `json:"destination_fingerprint"`
	State                  string `json:"state"`
	Sent                   int    `json:"sent"`
	Accepted               *int   `json:"accepted,omitempty"`
	Rejected               *int   `json:"rejected,omitempty"`
	Reason                 string `json:"reason,omitempty"`
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

type AgentArtifact struct {
	ProjectIdentity
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Version        int64             `json:"version"`
	Hash           string            `json:"hash"`
	Kind           string            `json:"kind"`
	MIME           string            `json:"mime"`
	DisplayName    string            `json:"display_name,omitempty"`
	Size           int               `json:"size"`
	Created        string            `json:"created"`
	Owner          AgentSessionKey   `json:"owner"`
	Ownership      string            `json:"ownership"`
	Provenance     map[string]string `json:"provenance"`
	RevisionStatus string            `json:"revision_status"`
	SourceStatus   string            `json:"source_status"`
	Protected      bool              `json:"protected"`
	InUse          bool              `json:"in_use"`
	CanDelete      bool              `json:"can_delete"`
	URI            string            `json:"uri"`
}

type ArtifactList struct {
	ProjectIdentity
	Snapshot   string          `json:"snapshot"`
	Artifacts  []AgentArtifact `json:"artifacts"`
	NextCursor string          `json:"next_cursor,omitempty"`
}

type ArtifactDetail struct {
	AgentArtifact
	PreviewKind string `json:"preview_kind"`
	Text        string `json:"text,omitempty"`
	Truncated   bool   `json:"truncated"`
	ReportID    string `json:"report_id,omitempty"`
}

type ArtifactControl struct {
	ProjectIdentity
	Action  string `json:"action"`
	Version int64  `json:"version"`
	Hash    string `json:"hash"`
}

type ArtifactReceipt struct {
	ProjectIdentity
	ID      string `json:"id"`
	Version int64  `json:"version"`
	Hash    string `json:"hash"`
	State   string `json:"state"`
	Path    string `json:"path,omitempty"`
}

type MemorySource struct {
	Path      string `json:"path"`
	Hash      string `json:"hash"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
}

type MemoryProvenance struct {
	Origin       string `json:"origin"`
	SessionID    string `json:"session_id,omitempty"`
	WorkflowID   string `json:"workflow_id,omitempty"`
	ProposalHash string `json:"proposal_hash,omitempty"`
}

type AgentMemory struct {
	ProjectIdentity
	ID           string           `json:"id"`
	Version      int64            `json:"version"`
	Policy       string           `json:"policy"`
	Kind         string           `json:"kind"`
	State        string           `json:"state"`
	Title        string           `json:"title"`
	Text         string           `json:"text"`
	Sources      []MemorySource   `json:"sources"`
	Provenance   MemoryProvenance `json:"provenance"`
	CreatedAt    string           `json:"created_at"`
	UpdatedAt    string           `json:"updated_at"`
	ExpiresAt    string           `json:"expires_at"`
	Stale        bool             `json:"stale"`
	Expired      bool             `json:"expired"`
	SourceStatus string           `json:"source_status"`
}

type LibraryProposalIdentity struct {
	ProjectIdentity
	Revision int64  `json:"revision"`
	Hash     string `json:"hash"`
}

type MemoryDraft struct {
	Kind          string                   `json:"kind"`
	Title         string                   `json:"title"`
	Text          string                   `json:"text"`
	Sources       []MemorySource           `json:"sources"`
	RetentionDays int                      `json:"retention_days"`
	SessionID     string                   `json:"session_id,omitempty"`
	Identity      *LibraryProposalIdentity `json:"identity,omitempty"`
	LessonIndex   *int                     `json:"lesson_index,omitempty"`
}

type MemoryMutation struct {
	ProjectIdentity
	Action  string       `json:"action"`
	Version int64        `json:"version"`
	Draft   *MemoryDraft `json:"draft,omitempty"`
}

type LibrarySource struct {
	FileInfo
	Truncated bool
}
