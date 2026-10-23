package agentruntime

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"google.golang.org/adk/v2/session"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
)

// Observer owns only live delivery. Durable progress is committed by agentstate
// before callbacks run; missing or slow subscribers cannot erase terminal state.
type Observer struct {
	Started func(agentstate.Run, context.CancelFunc)
	Ended   func(string)
	Changed func()
	Text    func(string, string)
}

type observerKey struct{}
type invocationKey struct{}
type streamingKey struct{}
type changeScopeKey struct{}
type changeRunScope struct{ SessionID, WorkflowID string }

func withGraphScope(ctx context.Context, input any) context.Context {
	var snapshot ChangeSnapshot
	switch input := input.(type) {
	case ChangeGraphInput:
		snapshot = input.Snapshot
	case ReviewGraphInput:
		snapshot = input.Snapshot
	case RepairGraphInput:
		snapshot = input.Snapshot
	case ConfirmationInput:
		snapshot = input.Snapshot
	default:
		return ctx
	}
	return context.WithValue(ctx, changeScopeKey{}, changeRunScope{snapshot.SessionID, snapshot.WorkflowID})
}

func WithObserver(ctx context.Context, observer *Observer) context.Context {
	if ctx.Value(observerKey{}) != nil || observer == nil {
		return ctx
	}
	return context.WithValue(ctx, observerKey{}, observer)
}

func WithStreaming(ctx context.Context) context.Context {
	return context.WithValue(ctx, streamingKey{}, true)
}
func StreamingRequested(ctx context.Context) bool {
	requested, _ := ctx.Value(streamingKey{}).(bool)
	return requested
}

func (s *runtimeSession) activate(ctx context.Context) context.Context {
	if s.runID == "" {
		return ctx
	}
	ctx = context.WithValue(ctx, invocationKey{}, s.runID)
	if s.record.ChangeSessionID != "" {
		ctx = context.WithValue(ctx, changeScopeKey{}, changeRunScope{s.record.ChangeSessionID, s.record.WorkflowID})
	}
	if observer, _ := ctx.Value(observerKey{}).(*Observer); observer != nil && observer.Started != nil {
		observer.Started(s.record, func() { CancelRun(ctx, context.Canceled) })
	}
	notifyProgress(ctx)
	return ctx
}

func observeRunEnd(ctx context.Context, id string) {
	if observer, _ := ctx.Value(observerKey{}).(*Observer); observer != nil && observer.Ended != nil && id != "" {
		observer.Ended(id)
	}
	notifyProgress(ctx)
}

func notifyProgress(ctx context.Context) {
	if observer, _ := ctx.Value(observerKey{}).(*Observer); observer != nil && observer.Changed != nil {
		observer.Changed()
	}
}

func observeText(ctx context.Context, text string) {
	id, _ := ctx.Value(invocationKey{}).(string)
	if observer, _ := ctx.Value(observerKey{}).(*Observer); observer != nil && observer.Text != nil && id != "" && text != "" {
		observer.Text(id, text)
	}
}

func projectEvent(event *session.Event, key agentstate.Key) *agentstate.NodeProgress {
	progress := &agentstate.NodeProgress{NodeID: eventNodeID(event), State: "recorded"}
	switch {
	case event.RequestedInput != nil:
		progress.State = "waiting_input"
	case event.CustomMetadata["mini_orca.node_status"] == "running":
		progress.State = "running"
	case event.Output != nil || event.TurnComplete:
		progress.State = "completed"
	}
	if version := event.Actions.ArtifactDelta["completion.json"]; version > 0 {
		progress.Evidence = []agentstate.ArtifactReference{{Session: key, Name: "completion.json", Version: version}}
	}
	return progress
}

func eventNodeID(event *session.Event) string {
	if event.NodeInfo != nil {
		parts := strings.Split(event.NodeInfo.Path, "/")
		name, _, _ := strings.Cut(parts[len(parts)-1], "@")
		if knownProgressNode(name) {
			return name
		}
	}
	if name := changeNodeMetadata(event.CustomMetadata["mini_orca.change_node"]); name != "" {
		return name
	}
	if role, ok := event.CustomMetadata["mini_orca.review_node"].(string); ok && slices.Contains(reviewRoles, role) {
		return "review_" + role
	}
	if attempt, ok := event.CustomMetadata["mini_orca.attempt"].(int); ok && attempt >= 0 && attempt <= 2 {
		return fmt.Sprintf("attempt_%d", attempt)
	}
	if event.RequestedInput != nil {
		return "human_review"
	}
	if name := outputNodeID(event.Output); name != "" {
		return name
	}
	if event.Author == "completion" {
		return "completion"
	}
	return "agent"
}

func outputNodeID(output any) string {
	switch output := output.(type) {
	case ChangeNodeResult:
		return changeNodeMetadata(output.Node)
	case ReviewNodeResult:
		if slices.Contains(reviewRoles, output.Specialist) {
			return "review_" + output.Specialist
		}
	case ReviewGraphResult:
		return "review_result"
	case RepairAttemptResult:
		if output.Attempt >= 0 && output.Attempt <= 2 {
			return fmt.Sprintf("attempt_%d", output.Attempt)
		}
	case Result:
		return "completion"
	}
	return ""
}

func changeNodeMetadata(value any) string {
	var name string
	switch value := value.(type) {
	case ChangeNode:
		name = string(value)
	case string:
		name = value
	}
	if slices.Contains(changeNodes, ChangeNode(name)) {
		return name
	}
	return ""
}

func knownProgressNode(name string) bool {
	return slices.Contains([]string{"create", "generate_tests", "verify", "review", "human_review", "review_correctness", "review_architecture", "review_test_quality", "review_security", "review_performance", "join_reviews", "review_result", "attempt_0", "attempt_1", "attempt_2", "completion", "agent"}, name)
}
