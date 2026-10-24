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

// Analysis admission and evidence remain in the host. The graph captures only
// immutable identities for one bounded window; restoring it never dispatches.
type AnalysisSnapshot struct {
	ProjectID           string `json:"project_id"`
	ProjectRevision     string `json:"project_revision"`
	RunID               string `json:"run_id"`
	Generation          string `json:"generation"`
	QueueID             string `json:"queue_id"`
	PolicyFingerprint   string `json:"policy_fingerprint"`
	ProviderFingerprint string `json:"provider_fingerprint"`
}

type AnalysisWork struct {
	Ordinal    int    `json:"ordinal"`
	FileIndex  int    `json:"file_index"`
	StageIndex int    `json:"stage_index"`
	Stage      string `json:"stage"`
	FileHash   string `json:"file_hash"`
}

type AnalysisGraphInput struct {
	Snapshot AnalysisSnapshot `json:"snapshot"`
	Work     []AnalysisWork   `json:"work"`
}

type AnalysisOutcome struct {
	Status   string
	Continue bool
}

type AnalysisNodeResult struct {
	Work   AnalysisWork `json:"work"`
	Status string       `json:"status"`
	Next   string       `json:"next"`
}

type AnalysisGraphResult struct {
	Nodes   []AnalysisNodeResult
	Stopped bool
}

type AnalysisHost func(context.Context, AnalysisWork) (AnalysisOutcome, error)

var analysisNodeStages = []string{"semantic", "performance", "security_rules", "security_ai"}

// No SDK retry is enabled. Host stage errors stop dispatch; a recorded failed
// report may continue to the next stage without losing its failure status.
func RunAnalysisGraph(ctx context.Context, input AnalysisGraphInput, host AnalysisHost) (AnalysisGraphResult, error) {
	if err := validateAnalysisGraph(input, host); err != nil {
		return AnalysisGraphResult{}, err
	}
	input.Work = slices.Clone(input.Work)
	previous := workflow.Start
	var edges []workflow.Edge
	for i, work := range input.Work {
		node := &analysisWorkNode{BaseNode: workflow.NewBaseNode(analysisNodeID(work), "", workflow.NodeConfig{}), work: work, last: i == len(input.Work)-1, host: host}
		edge := workflow.Edge{From: previous, To: node}
		if i > 0 {
			edge.Route = workflow.StringRoute("continue")
		}
		edges = append(edges, edge)
		previous = node
	}
	worker, err := graphAgent("project_analysis", edges, 1, nil)
	if err != nil {
		return AnalysisGraphResult{}, err
	}
	return runGraph(ctx, "analysis_graph", "analysis-graph-v1", input, analysisNodeStages, worker, func(ctx context.Context, events iter.Seq2[*session.Event, error]) (AnalysisGraphResult, error) {
		return collectAnalysisGraph(ctx, events, input.Work)
	})
}

func validateAnalysisGraph(input AnalysisGraphInput, host AnalysisHost) error {
	if host == nil || len(input.Work) == 0 || len(input.Work) > 2000 {
		return fmt.Errorf("analysis graph requires a host and 1-2000 stage nodes")
	}
	s := input.Snapshot
	if err := validateGraphLabels(s.ProjectID, s.ProjectRevision, s.RunID, s.Generation, s.QueueID, s.PolicyFingerprint, s.ProviderFingerprint); err != nil {
		return fmt.Errorf("analysis admission: %w", err)
	}
	files := 0
	previous := AnalysisWork{FileIndex: -1}
	for i, work := range input.Work {
		if err := validateAnalysisWork(work, previous, i); err != nil {
			return err
		}
		if work.FileIndex != previous.FileIndex {
			files++
		}
		previous = work
	}
	if files > 500 {
		return fmt.Errorf("analysis graph exceeds 500 files per window")
	}
	return nil
}

func validateAnalysisWork(work, previous AnalysisWork, ordinal int) error {
	if work.Ordinal != ordinal || work.FileIndex < 0 || work.StageIndex < 0 || work.StageIndex >= len(analysisNodeStages) || work.Stage != analysisNodeStages[work.StageIndex] {
		return fmt.Errorf("analysis graph has an invalid stage identity")
	}
	if work.FileIndex < previous.FileIndex || work.FileIndex == previous.FileIndex && (work.StageIndex <= previous.StageIndex || work.FileHash != previous.FileHash) {
		return fmt.Errorf("analysis graph work is reordered, duplicated or inconsistent")
	}
	return validateGraphLabels(work.FileHash)
}

type analysisWorkNode struct {
	workflow.BaseNode
	work AnalysisWork
	last bool
	host AnalysisHost
}

func (n *analysisWorkNode) Run(ctx agent.Context, _ any) iter.Seq2[*session.Event, error] {
	return func(yield func(*session.Event, error) bool) {
		started := session.NewEvent(ctx, ctx.InvocationID())
		started.CustomMetadata = map[string]any{"mini_orca.analysis_node": analysisNodeID(n.work), "mini_orca.node_status": "running"}
		if !yield(started, nil) {
			return
		}
		if err := context.Cause(ctx); err != nil {
			yield(nil, err)
			return
		}
		outcome, err := tracedHost(ctx, "node", analysisNodeID(n.work), func(ctx context.Context) (AnalysisOutcome, error) { return n.host(ctx, n.work) })
		if err == nil {
			err = context.Cause(ctx)
		}
		if err == nil {
			err = validateAnalysisOutcome(outcome)
		}
		if err != nil {
			yield(nil, fmt.Errorf("analysis node %s: %w", analysisNodeID(n.work), err))
			return
		}
		next := "continue"
		if !outcome.Continue {
			next = "stop"
		} else if n.last {
			next = "complete"
		}
		finished := session.NewEvent(ctx, ctx.InvocationID())
		finished.Output = AnalysisNodeResult{Work: n.work, Status: outcome.Status, Next: next}
		finished.Routes = []string{next}
		yield(finished, nil)
	}
}

func validateAnalysisOutcome(outcome AnalysisOutcome) error {
	if outcome.Status == "stopped" && !outcome.Continue {
		return nil
	}
	if slices.Contains([]string{"completed", "completed_empty", "partial", "failed", "skipped", "unavailable"}, outcome.Status) {
		return nil
	}
	return fmt.Errorf("analysis host returned an unsettled stage")
}

func collectAnalysisGraph(ctx context.Context, events iter.Seq2[*session.Event, error], work []AnalysisWork) (AnalysisGraphResult, error) {
	var result AnalysisGraphResult
	finished := false
	for event, err := range events {
		if err != nil {
			return result, err
		}
		if event == nil {
			return result, fmt.Errorf("analysis graph returned an empty event")
		}
		if event.Output == nil {
			continue
		}
		node, ok := event.Output.(AnalysisNodeResult)
		if !ok || finished || len(result.Nodes) >= len(work) || node.Work != work[len(result.Nodes)] {
			return result, fmt.Errorf("analysis graph returned an unexpected stage")
		}
		result.Nodes = append(result.Nodes, node)
		result.Stopped = node.Next == "stop"
		finished = result.Stopped || node.Next == "complete" && len(result.Nodes) == len(work)
	}
	if err := context.Cause(ctx); err != nil {
		return result, err
	}
	if !finished {
		return result, fmt.Errorf("analysis graph ended before settling its window")
	}
	return result, nil
}

func analysisNodeID(work AnalysisWork) string {
	return fmt.Sprintf("analysis_%04d_%s", work.Ordinal, work.Stage)
}

func knownAnalysisNode(name string) bool {
	value, ok := strings.CutPrefix(name, "analysis_")
	if !ok {
		return false
	}
	ordinal, stage, _ := strings.Cut(value, "_")
	index, err := strconv.Atoi(ordinal)
	return err == nil && index >= 0 && index < 2000 && slices.Contains(analysisNodeStages, stage) && name == analysisNodeID(AnalysisWork{Ordinal: index, Stage: stage})
}

func analysisProgressState(status string) string {
	switch status {
	case "completed", "completed_empty":
		return "completed"
	case "failed":
		return "failed"
	default:
		return "recorded" // Partial, unavailable and stopped work is not success.
	}
}
