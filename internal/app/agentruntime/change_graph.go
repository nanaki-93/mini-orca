package agentruntime

import (
	"context"
	"fmt"
	"iter"
	"slices"
	"strings"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/workflow"
)

type ChangeNode string

const (
	ChangeCreate      ChangeNode = "create"
	ChangeTests       ChangeNode = "generate_tests"
	ChangeVerify      ChangeNode = "verify"
	ChangeReview      ChangeNode = "review"
	ChangeHumanReview ChangeNode = "human_review"
)

// ChangeSnapshot carries identity only. Source, edits and check authority remain
// in the host; a graph result cannot be submitted as source mutation authority.
type ChangeSnapshot struct {
	ProjectID       string `json:"project_id"`
	ProjectRevision string `json:"project_revision"`
	SessionID       string `json:"session_id"`
	WorkflowID      string `json:"workflow_id"`
	Revision        int64  `json:"revision"`
	Hash            string `json:"hash"`
}

type ChangeGraphInput struct {
	Snapshot ChangeSnapshot `json:"snapshot"`
	Start    ChangeNode     `json:"start"`
	Models   [3]string      `json:"models"`
}

type ChangeNodeInput struct {
	Node         ChangeNode
	Snapshot     ChangeSnapshot
	PriorVerdict string
}

type ChangeNodeResult struct {
	Node     ChangeNode     `json:"node"`
	Snapshot ChangeSnapshot `json:"snapshot"`
	Status   string         `json:"status"`
	Verdict  string         `json:"verdict,omitempty"`
}

type ChangeGraphResult struct {
	Nodes   []ChangeNodeResult
	Outcome string
}

type ChangeNodeHost func(context.Context, ChangeNodeInput) (ChangeNodeResult, error)

var changeNodes = []ChangeNode{ChangeCreate, ChangeTests, ChangeVerify, ChangeReview, ChangeHumanReview}

// RunChangeGraph executes an admitted, single-attempt graph. Node retries are
// disabled: provider retries and repair admission belong to their host owners.
func RunChangeGraph(ctx context.Context, input ChangeGraphInput, host ChangeNodeHost) (ChangeGraphResult, error) {
	nodes, err := validateChangeGraph(input, host)
	if err != nil {
		return ChangeGraphResult{}, err
	}
	worker, err := newChangeGraphAgent(input, nodes, host)
	if err != nil {
		return ChangeGraphResult{}, err
	}
	return runGraph(ctx, "change_graph", "change-graph-v1", input, changeNodes, worker, func(ctx context.Context, events iter.Seq2[*session.Event, error]) (ChangeGraphResult, error) {
		return collectChangeGraph(ctx, events, nodes)
	})
}

func validateChangeGraph(input ChangeGraphInput, host ChangeNodeHost) ([]ChangeNode, error) {
	if host == nil {
		return nil, fmt.Errorf("change graph requires an admitted host and proposal identity")
	}
	if err := validateChangeSnapshot(input.Snapshot); err != nil {
		return nil, err
	}
	if err := validateGraphLabels(input.Models[:]...); err != nil {
		return nil, err
	}
	if input.Start != ChangeCreate && input.Start != ChangeTests && input.Start != ChangeReview {
		return nil, fmt.Errorf("change graph must start at creation, testing or review")
	}
	return slices.Clone(changeNodes[slices.Index(changeNodes, input.Start):]), nil
}

func validateChangeSnapshot(identity ChangeSnapshot) error {
	if identity.Revision < 0 {
		return fmt.Errorf("change graph requires a non-negative proposal revision")
	}
	return validateGraphLabels(identity.ProjectID, identity.ProjectRevision, identity.SessionID, identity.WorkflowID, identity.Hash)
}

func validateGraphLabels(values ...string) error {
	for _, value := range values {
		if strings.TrimSpace(value) == "" || len(value) > 512 || strings.ContainsRune(value, 0) {
			return fmt.Errorf("change graph identity is missing or invalid")
		}
	}
	return nil
}

func newChangeGraphAgent(input ChangeGraphInput, nodes []ChangeNode, host ChangeNodeHost) (agent.Agent, error) {
	previous := workflow.Start
	var edges []workflow.Edge
	for _, id := range nodes {
		node := workflow.NewEmittingFunctionNode(string(id), changeNodeBody(id, host), workflow.NodeConfig{})
		edges = append(edges, workflow.Edge{From: previous, To: node})
		previous = node
	}
	return graphAgent("checked_change", edges, 1, ChangeNodeResult{Snapshot: input.Snapshot})
}

func changeNodeBody(id ChangeNode, host ChangeNodeHost) workflow.EmittingFunctionFn[ChangeNodeResult, ChangeNodeResult] {
	return func(ctx agent.Context, previous ChangeNodeResult, emit func(*session.Event) error) (ChangeNodeResult, error) {
		input := ChangeNodeInput{Node: id, Snapshot: previous.Snapshot, PriorVerdict: previous.Verdict}
		started := session.NewEvent(ctx, ctx.InvocationID())
		started.CustomMetadata = map[string]any{"mini_orca.change_node": id, "mini_orca.node_status": "running"}
		// Non-partial events use the pinned scheduler's persistence handshake.
		// Check cancellation again after emit: a failed append cancels the node.
		if err := emit(started); err != nil {
			return ChangeNodeResult{}, err
		}
		if err := context.Cause(ctx); err != nil {
			return ChangeNodeResult{}, err
		}
		result, err := host(ctx, input)
		if err != nil {
			return ChangeNodeResult{}, fmt.Errorf("change node %s: %w", id, err)
		}
		if err := context.Cause(ctx); err != nil {
			return ChangeNodeResult{}, err
		}
		return result, validateChangeNodeResult(input, result)
	}
}

func validateChangeNodeResult(input ChangeNodeInput, result ChangeNodeResult) error {
	before, after := input.Snapshot, result.Snapshot
	if result.Node != input.Node || before.ProjectID != after.ProjectID || before.ProjectRevision != after.ProjectRevision || before.SessionID != after.SessionID || before.WorkflowID != after.WorkflowID || after.Hash == "" || len(after.Hash) > 512 {
		return fmt.Errorf("%w: change node returned a different admission", ErrPolicyDenied)
	}
	if input.Node == ChangeCreate || input.Node == ChangeTests {
		if after.Revision < before.Revision || after.Revision == before.Revision && after.Hash != before.Hash {
			return fmt.Errorf("%w: proposal revision did not advance", ErrPolicyDenied)
		}
	} else if before != after {
		return fmt.Errorf("%w: read-only change node changed the proposal", ErrPolicyDenied)
	}
	return validateChangeNodeOutcome(input, result)
}

func validateChangeNodeOutcome(input ChangeNodeInput, result ChangeNodeResult) error {
	if input.Node == ChangeHumanReview {
		if input.PriorVerdict != "approve" && input.PriorVerdict != "changes_requested" {
			return fmt.Errorf("human-review handoff lacks a critic verdict")
		}
		want := "changes_requested"
		if input.PriorVerdict == "approve" {
			want = "awaiting_human_review"
		}
		if result.Status != want || result.Verdict != input.PriorVerdict {
			return fmt.Errorf("change graph handoff disagrees with the checked review")
		}
		return nil
	}
	if result.Status != "completed" {
		return fmt.Errorf("change graph node did not complete")
	}
	if input.Node == ChangeReview {
		if result.Verdict != "approve" && result.Verdict != "changes_requested" {
			return fmt.Errorf("change graph review lacks a valid verdict")
		}
	} else if result.Verdict != "" {
		return fmt.Errorf("only the review node can produce a review verdict")
	}
	return nil
}

func collectChangeGraph(ctx context.Context, events iter.Seq2[*session.Event, error], nodes []ChangeNode) (ChangeGraphResult, error) {
	var result ChangeGraphResult
	for event, err := range events {
		if err != nil {
			return result, err
		}
		if event == nil {
			return result, fmt.Errorf("change graph returned an empty event")
		}
		if event.Output == nil {
			continue
		}
		output, ok := event.Output.(ChangeNodeResult)
		if !ok || len(result.Nodes) >= len(nodes) || output.Node != nodes[len(result.Nodes)] {
			return result, fmt.Errorf("change graph returned an unexpected node result")
		}
		result.Nodes = append(result.Nodes, output)
	}
	if err := context.Cause(ctx); err != nil {
		return result, err
	}
	if len(result.Nodes) != len(nodes) {
		return result, fmt.Errorf("change graph ended before the human-review handoff")
	}
	result.Outcome = result.Nodes[len(result.Nodes)-1].Status
	return result, nil
}
