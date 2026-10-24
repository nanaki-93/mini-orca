package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
)

func renderAgentTrace(trace *client.AgentTraceSummary) []string {
	if trace == nil || trace.Legacy {
		return []string{"", "Local trace unavailable for this historical run."}
	}
	lines := []string{"", "Local spans · shared across attempts and child work", "Trace: " + trace.TraceID}
	for _, span := range trace.Spans {
		duration := "duration unfinished"
		if span.Ended != "" {
			duration = time.Duration(span.ElapsedNanos).Round(time.Microsecond).String()
		}
		lines = append(lines, fmt.Sprintf("%s · %s · %s · %s", span.ID, span.Kind, span.Status, duration))
		if span.ParentID != "" {
			lines = append(lines, "  Parent: "+span.ParentID)
		}
	}
	if trace.Truncated {
		lines = append(lines, fmt.Sprintf("%d spans omitted by retention or the 256-span view limit.", trace.Omitted))
	}
	if trace.Incomplete {
		lines = append(lines, "Unfinished spans have no measured end time; interrupted work is not success.")
	}
	return append(lines, "Span completion describes execution; findings, checks and approval retain their own outcomes.")
}

func renderAgentCost(cost *client.AgentCostEstimate) []string {
	if cost == nil || cost.PriceVersion == "" {
		return []string{"Cost: unknown · a versioned price table is required."}
	}
	lines := []string{fmt.Sprintf("Price table: %s · %s", cost.PriceVersion, cost.Currency)}
	if cost.Status == "estimated" && cost.Total != nil {
		return append(lines, fmt.Sprintf("Estimated token cost: %.6f %s · not an invoice", *cost.Total, cost.Currency))
	}
	lines = append(lines, fmt.Sprintf("Total cost: unknown · known subtotal %.6f %s", cost.KnownSubtotal, cost.Currency))
	for _, reason := range cost.Reasons {
		lines = append(lines, "  "+strings.ReplaceAll(reason, "_", " "))
	}
	return append(lines, "Subtotal prices only measured tokens with known rates; it excludes unmeasured consumption.")
}
