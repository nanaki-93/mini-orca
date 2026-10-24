package app

import "github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"

type AgentTraceSummary struct {
	Version    int                    `json:"version"`
	TraceID    string                 `json:"trace_id"`
	Snapshot   string                 `json:"snapshot"`
	Legacy     bool                   `json:"legacy"`
	Truncated  bool                   `json:"truncated"`
	Incomplete bool                   `json:"incomplete"`
	Omitted    uint64                 `json:"omitted"`
	Spans      []agentstate.TraceSpan `json:"spans"`
}

type AgentCostEstimate struct {
	Status        string   `json:"status"`
	Currency      string   `json:"currency,omitempty"`
	PriceVersion  string   `json:"price_version,omitempty"`
	KnownSubtotal float64  `json:"known_subtotal"`
	Total         *float64 `json:"total,omitempty"`
	Reasons       []string `json:"reasons"`
}
