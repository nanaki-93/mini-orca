package agentruntime

import (
	"context"
	"fmt"
	"iter"
	"slices"
	"strconv"
	"strings"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/workflow"
)

// Only host-admitted ready children enter this wave. Dependency, writer, budget,
// worktree and proposal authority remains in the application and its journal.
type MissionGraphInput struct {
	ProjectID       string   `json:"project_id"`
	ProjectRevision string   `json:"project_revision"`
	ExecutionID     string   `json:"execution_id"`
	WaveID          string   `json:"wave_id"`
	PlanHash        string   `json:"plan_hash"`
	Children        []string `json:"children"`
	Concurrency     int      `json:"concurrency"`
}

type MissionNodeResult struct {
	ExecutionID      string `json:"execution_id"`
	WaveID           string `json:"wave_id"`
	ChildID          string `json:"child_id"`
	State            string `json:"state"`
	SessionID        string `json:"session_id,omitempty"`
	WorkflowID       string `json:"workflow_id,omitempty"`
	ProposalHash     string `json:"proposal_hash,omitempty"`
	ProposalRevision int64  `json:"proposal_revision,omitempty"`
}

type MissionGraphResult struct {
	Children []MissionNodeResult `json:"children"`
}

type MissionChildHost func(context.Context, string) (MissionNodeResult, error)

func RunMissionGraph(ctx context.Context, input MissionGraphInput, host MissionChildHost) (MissionGraphResult, error) {
	if err := validateMissionGraph(input, host); err != nil {
		return MissionGraphResult{}, err
	}
	input.Children = slices.Clone(input.Children)
	worker, err := newMissionGraph(input, host)
	if err != nil {
		return MissionGraphResult{}, err
	}
	return runGraph(ctx, "mission_graph", "mission-graph-v1", input, input.Children, worker, collectMissionGraph)
}

func validateMissionGraph(input MissionGraphInput, host MissionChildHost) error {
	if host == nil || len(input.Children) < 1 || len(input.Children) > 16 || input.Concurrency < 1 || input.Concurrency > len(input.Children) {
		return fmt.Errorf("mission graph requires 1–16 admitted children and bounded concurrency")
	}
	if err := validateGraphLabels(input.ProjectID, input.ProjectRevision, input.ExecutionID, input.WaveID, input.PlanHash); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, child := range input.Children {
		if seen[child] || !validMissionChildID(child) {
			return fmt.Errorf("mission children must have distinct bounded identifiers")
		}
		seen[child] = true
	}
	return nil
}

func validMissionChildID(child string) bool {
	return len(child) > 0 && len(child) <= 64 && strings.IndexFunc(child, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_')
	}) < 0
}

func newMissionGraph(input MissionGraphInput, host MissionChildHost) (agent.Agent, error) {
	join := workflow.NewJoinNode("join_mission")
	var edges []workflow.Edge
	for index, child := range input.Children {
		node := workflow.NewEmittingFunctionNode(missionNodeName(index), missionNodeBody(input, index, child, host), workflow.NodeConfig{})
		edges = append(edges, workflow.Edge{From: workflow.Start, To: node}, workflow.Edge{From: node, To: join})
	}
	aggregate := workflow.NewFunctionNode("mission_result", func(_ agent.Context, results map[string]any) (MissionGraphResult, error) {
		return joinMissionResults(input, results)
	}, workflow.NodeConfig{})
	edges = append(edges, workflow.Edge{From: join, To: aggregate})
	return graphAgent("mission_wave", edges, input.Concurrency, input.WaveID)
}

func missionNodeBody(input MissionGraphInput, index int, child string, host MissionChildHost) workflow.EmittingFunctionFn[string, MissionNodeResult] {
	return func(ctx agent.Context, wave string, emit func(*session.Event) error) (MissionNodeResult, error) {
		if wave != input.WaveID {
			return MissionNodeResult{}, ErrPolicyDenied
		}
		started := session.NewEvent(ctx, ctx.InvocationID())
		started.CustomMetadata = map[string]any{"mini_orca.mission_node": missionNodeName(index), "mini_orca.node_status": "running"}
		if err := emit(started); err != nil {
			return MissionNodeResult{}, err
		}
		if err := context.Cause(ctx); err != nil {
			return MissionNodeResult{}, err
		}
		result, err := tracedHost(ctx, "node", missionNodeName(index), func(ctx context.Context) (MissionNodeResult, error) { return host(ctx, child) })
		if err != nil {
			return MissionNodeResult{}, fmt.Errorf("mission child %s: %w", child, err)
		}
		if err := context.Cause(ctx); err != nil {
			return MissionNodeResult{}, err
		}
		return result, validateMissionNode(input, child, result)
	}
}

func validateMissionNode(input MissionGraphInput, child string, result MissionNodeResult) error {
	if result.ExecutionID != input.ExecutionID || result.WaveID != input.WaveID || result.ChildID != child {
		return fmt.Errorf("%w: mission child returned another execution identity", ErrPolicyDenied)
	}
	if !slices.Contains([]string{"awaiting_review", "rejected", "failed", "interrupted"}, result.State) {
		return fmt.Errorf("mission child returned an invalid outcome")
	}
	if result.ProposalRevision < 0 {
		return fmt.Errorf("mission child returned an invalid proposal revision")
	}
	if result.State == "awaiting_review" || result.State == "rejected" || result.SessionID != "" {
		return validateGraphLabels(result.SessionID, result.WorkflowID, result.ProposalHash)
	}
	return nil
}

func joinMissionResults(input MissionGraphInput, results map[string]any) (MissionGraphResult, error) {
	result := MissionGraphResult{Children: []MissionNodeResult{}}
	if len(results) != len(input.Children) {
		return result, fmt.Errorf("mission wave is incomplete")
	}
	for index, child := range input.Children {
		value, ok := results[missionNodeName(index)].(MissionNodeResult)
		if !ok {
			return result, fmt.Errorf("mission child has no typed outcome")
		}
		if err := validateMissionNode(input, child, value); err != nil {
			return result, err
		}
		result.Children = append(result.Children, value)
	}
	return result, nil
}

func collectMissionGraph(ctx context.Context, events iter.Seq2[*session.Event, error]) (MissionGraphResult, error) {
	var result MissionGraphResult
	for event, err := range events {
		if err != nil {
			return result, err
		}
		if event == nil {
			return result, fmt.Errorf("mission graph returned an empty event")
		}
		if joined, ok := event.Output.(MissionGraphResult); ok {
			if len(result.Children) > 0 {
				return result, fmt.Errorf("mission graph joined more than once")
			}
			result = joined
		}
	}
	if err := context.Cause(ctx); err != nil {
		return result, err
	}
	if len(result.Children) == 0 {
		return result, fmt.Errorf("mission graph ended without a complete join")
	}
	return result, nil
}

func missionNodeName(index int) string { return "mission_child_" + strconv.Itoa(index) }

func knownMissionNode(name string) bool {
	if name == "join_mission" || name == "mission_result" {
		return true
	}
	suffix, found := strings.CutPrefix(name, "mission_child_")
	if !found {
		return false
	}
	index, err := strconv.Atoi(suffix)
	return err == nil && index >= 0 && index < 16 && strconv.Itoa(index) == suffix
}
