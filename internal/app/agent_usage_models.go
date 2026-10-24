package app

import "github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"

type AgentTraceSummary struct {
	Export     *AgentTraceExportPreview `json:"export,omitempty"`
	Version    int                      `json:"version"`
	TraceID    string                   `json:"trace_id"`
	Snapshot   string                   `json:"snapshot"`
	Legacy     bool                     `json:"legacy"`
	Truncated  bool                     `json:"truncated"`
	Incomplete bool                     `json:"incomplete"`
	Omitted    uint64                   `json:"omitted"`
	Spans      []agentstate.TraceSpan   `json:"spans"`
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
	ProjectID              string `json:"project_id"`
	ProjectRevision        string `json:"project_revision"`
	Snapshot               string `json:"snapshot"`
	DestinationFingerprint string `json:"destination_fingerprint"`
	Confirm                bool   `json:"confirm"`
}

type AgentTraceExportReceipt struct {
	ProjectID              string `json:"project_id"`
	ProjectRevision        string `json:"project_revision"`
	RunID                  string `json:"run_id"`
	Snapshot               string `json:"snapshot"`
	DestinationFingerprint string `json:"destination_fingerprint"`
	State                  string `json:"state"`
	Sent                   int    `json:"sent"`
	Accepted               *int   `json:"accepted,omitempty"`
	Rejected               *int   `json:"rejected,omitempty"`
	Reason                 string `json:"reason,omitempty"`
}

type AgentCostEstimate struct {
	Status        string   `json:"status"`
	Currency      string   `json:"currency,omitempty"`
	PriceVersion  string   `json:"price_version,omitempty"`
	KnownSubtotal float64  `json:"known_subtotal"`
	Total         *float64 `json:"total,omitempty"`
	Reasons       []string `json:"reasons"`
}
