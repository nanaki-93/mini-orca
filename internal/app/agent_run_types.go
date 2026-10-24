package app

import (
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
)

type AgentRunQuery struct {
	ProjectID       string
	ProjectRevision string
	Cursor          string
	Limit           int
}

type AgentRun struct {
	ChangeSessionID string                      `json:"change_session_id,omitempty"`
	WorkflowID      string                      `json:"workflow_id,omitempty"`
	ID              string                      `json:"id"`
	RootID          string                      `json:"root_id"`
	ParentID        string                      `json:"parent_id,omitempty"`
	ProjectID       string                      `json:"project_id"`
	ProjectRevision string                      `json:"project_revision"`
	Kind            string                      `json:"kind"`
	Status          string                      `json:"status"`
	Active          bool                        `json:"active"`
	Stale           bool                        `json:"stale"`
	CanCancel       bool                        `json:"can_cancel"`
	StartedAt       time.Time                   `json:"started_at"`
	UpdatedAt       time.Time                   `json:"updated_at"`
	Usage           agentstate.BudgetUsage      `json:"usage"`
	Limits          config.AgentLimits          `json:"limits"`
	Input           *agentstate.InputCheckpoint `json:"input,omitempty"`
}

type AgentRunList struct {
	ProjectID       string     `json:"project_id"`
	ProjectRevision string     `json:"project_revision"`
	Runs            []AgentRun `json:"runs"`
	NextCursor      string     `json:"next_cursor,omitempty"`
}

type AgentPrivateText struct {
	ToolRecords    []AgentToolRecord             `json:"tool_records,omitempty"`
	ToolsTruncated bool                          `json:"tools_truncated,omitempty"`
	Available      bool                          `json:"available"`
	State          string                        `json:"state"`
	Text           string                        `json:"text,omitempty"`
	Truncated      bool                          `json:"truncated"`
	Artifact       *agentstate.ArtifactReference `json:"artifact,omitempty"`
}

type AgentToolRecord struct {
	EventID   string `json:"event_id"`
	CallID    string `json:"call_id,omitempty"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Value     string `json:"value"`
	Truncated bool   `json:"truncated"`
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

type AgentRunEvent struct {
	Sequence        uint64                         `json:"sequence"`
	RunID           string                         `json:"run_id"`
	InvocationID    string                         `json:"invocation_id"`
	ParentID        string                         `json:"parent_id,omitempty"`
	ProjectID       string                         `json:"project_id"`
	ProjectRevision string                         `json:"project_revision"`
	NodeID          string                         `json:"node_id,omitempty"`
	Kind            string                         `json:"kind"`
	State           string                         `json:"state"`
	Time            time.Time                      `json:"time"`
	Usage           agentstate.BudgetUsage         `json:"usage"`
	Evidence        []agentstate.ArtifactReference `json:"evidence,omitempty"`
	RequestID       string                         `json:"request_id,omitempty"`
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

type AgentRunControl struct {
	ProjectID       string `json:"project_id"`
	ProjectRevision string `json:"project_revision"`
	Action          string `json:"action"`
}
