package agentruntime

import (
	"context"
	"fmt"
	"iter"
	"slices"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/workflow"
)

var reviewRoles = []string{"correctness", "architecture", "test_quality", "security", "performance"}

type ReviewGraphInput struct {
	Snapshot    ChangeSnapshot `json:"snapshot"`
	Specialists []string       `json:"specialists"`
	Concurrency int            `json:"concurrency"`
}

// ReviewNodeResult deliberately excludes model prose. The host retains that
// evidence in its private proposal history before returning this receipt.
type ReviewNodeResult struct {
	Specialist string         `json:"specialist"`
	Snapshot   ChangeSnapshot `json:"snapshot"`
	Verdict    string         `json:"verdict"`
	Findings   int            `json:"findings"`
}

type ReviewGraphResult struct {
	Reviews []ReviewNodeResult `json:"reviews"`
	Verdict string             `json:"verdict"`
}

type ReviewNodeHost func(context.Context, string, ChangeSnapshot) (ReviewNodeResult, error)

func RunReviewGraph(ctx context.Context, input ReviewGraphInput, host ReviewNodeHost) (ReviewGraphResult, error) {
	if err := validateReviewGraph(input, host); err != nil {
		return ReviewGraphResult{}, err
	}
	input.Specialists = slices.Clone(input.Specialists)
	worker, err := newReviewGraph(input, host)
	if err != nil {
		return ReviewGraphResult{}, err
	}
	return runGraph(ctx, "review_graph", "review-graph-v1", input, input.Specialists, worker, collectReviewGraph)
}

func validateReviewGraph(input ReviewGraphInput, host ReviewNodeHost) error {
	if host == nil || input.Concurrency < 1 || input.Concurrency > 32 || len(input.Specialists) == 0 || len(input.Specialists) > len(reviewRoles) {
		return fmt.Errorf("invalid specialist graph admission")
	}
	if err := validateChangeSnapshot(input.Snapshot); err != nil {
		return err
	}
	last := -1
	for _, role := range input.Specialists {
		index := slices.Index(reviewRoles, role)
		if index <= last {
			return fmt.Errorf("specialists must be unique admitted roles in canonical order")
		}
		last = index
	}
	return nil
}

func newReviewGraph(input ReviewGraphInput, host ReviewNodeHost) (agent.Agent, error) {
	join := workflow.NewJoinNode("join_reviews")
	var edges []workflow.Edge
	for _, role := range input.Specialists {
		node := workflow.NewEmittingFunctionNode("review_"+role, reviewNodeBody(role, host), workflow.NodeConfig{})
		edges = append(edges, workflow.Edge{From: workflow.Start, To: node}, workflow.Edge{From: node, To: join})
	}
	aggregate := workflow.NewFunctionNode("review_result", func(_ agent.Context, results map[string]any) (ReviewGraphResult, error) {
		return joinReviewResults(input, results)
	}, workflow.NodeConfig{})
	edges = append(edges, workflow.Edge{From: join, To: aggregate})
	return graphAgent("specialist_review", edges, input.Concurrency, input.Snapshot)
}

func reviewNodeBody(role string, host ReviewNodeHost) workflow.EmittingFunctionFn[ChangeSnapshot, ReviewNodeResult] {
	return func(ctx agent.Context, snapshot ChangeSnapshot, emit func(*session.Event) error) (ReviewNodeResult, error) {
		started := session.NewEvent(ctx, ctx.InvocationID())
		started.CustomMetadata = map[string]any{"mini_orca.review_node": role, "mini_orca.node_status": "running"}
		if err := emit(started); err != nil {
			return ReviewNodeResult{}, err
		}
		if err := context.Cause(ctx); err != nil {
			return ReviewNodeResult{}, err
		}
		result, err := tracedHost(ctx, "node", "review_"+role, func(ctx context.Context) (ReviewNodeResult, error) { return host(ctx, role, snapshot) })
		if err != nil {
			return ReviewNodeResult{}, fmt.Errorf("specialist %s: %w", role, err)
		}
		if err := context.Cause(ctx); err != nil {
			return ReviewNodeResult{}, err
		}
		return result, validateReviewResult(role, snapshot, result)
	}
}

func validateReviewResult(role string, snapshot ChangeSnapshot, result ReviewNodeResult) error {
	if result.Specialist != role || result.Snapshot != snapshot {
		return fmt.Errorf("%w: specialist returned a different proposal", ErrPolicyDenied)
	}
	if result.Findings < 0 || result.Findings > 16 || result.Verdict != "approve" && result.Verdict != "changes_requested" || (result.Verdict == "approve") != (result.Findings == 0) {
		return fmt.Errorf("specialist returned an invalid verdict")
	}
	return nil
}

func joinReviewResults(input ReviewGraphInput, results map[string]any) (ReviewGraphResult, error) {
	joined := ReviewGraphResult{Verdict: "approve"}
	if len(results) != len(input.Specialists) {
		return joined, fmt.Errorf("specialist review is incomplete")
	}
	for _, role := range input.Specialists {
		result, ok := results["review_"+role].(ReviewNodeResult)
		if !ok {
			return joined, fmt.Errorf("specialist %s has no typed result", role)
		}
		if err := validateReviewResult(role, input.Snapshot, result); err != nil {
			return joined, err
		}
		joined.Reviews = append(joined.Reviews, result)
		if result.Verdict == "changes_requested" {
			joined.Verdict = result.Verdict
		}
	}
	return joined, nil
}

func collectReviewGraph(ctx context.Context, events iter.Seq2[*session.Event, error]) (ReviewGraphResult, error) {
	var result ReviewGraphResult
	for event, err := range events {
		if err != nil {
			return result, err
		}
		if event == nil {
			return result, fmt.Errorf("specialist graph returned an empty event")
		}
		if joined, ok := event.Output.(ReviewGraphResult); ok {
			if result.Verdict != "" {
				return result, fmt.Errorf("specialist graph joined more than once")
			}
			result = joined
		}
	}
	if err := context.Cause(ctx); err != nil {
		return result, err
	}
	if result.Verdict == "" {
		return result, fmt.Errorf("specialist graph ended without a complete join")
	}
	return result, nil
}
