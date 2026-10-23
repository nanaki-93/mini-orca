package agentruntime

import (
	"context"
	"fmt"
	"iter"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/workflow"
)

type RepairGraphInput struct {
	Snapshot   ChangeSnapshot `json:"snapshot"`
	MaxRepairs int            `json:"max_repairs"`
}

type RepairAttemptResult struct {
	Attempt int    `json:"attempt"`
	Next    string `json:"next"`
}

// The host decides whether current evidence warrants another attempt. A node
// never retries a failed host call; recovery from an error requires admission.
type RepairAttemptHost func(context.Context, int) (repair bool, err error)

func RunRepairGraph(ctx context.Context, input RepairGraphInput, host RepairAttemptHost) error {
	if host == nil || input.MaxRepairs < 0 || input.MaxRepairs > 2 {
		return fmt.Errorf("repair graph requires a host and at most two repairs")
	}
	if err := validateChangeSnapshot(input.Snapshot); err != nil {
		return err
	}
	previous := workflow.Start
	var edges []workflow.Edge
	for attempt := 0; attempt <= input.MaxRepairs; attempt++ {
		node := &repairAttemptNode{BaseNode: workflow.NewBaseNode(fmt.Sprintf("attempt_%d", attempt), "", workflow.NodeConfig{}), attempt: attempt, limit: input.MaxRepairs, host: host}
		edge := workflow.Edge{From: previous, To: node}
		if attempt > 0 {
			edge.Route = workflow.StringRoute("repair")
		}
		edges = append(edges, edge)
		previous = node
	}
	worker, err := graphAgent("checked_change_repairs", edges, 1, nil)
	if err != nil {
		return err
	}
	_, err = runGraph(ctx, "repair_graph", "repair-graph-v1", input, input.MaxRepairs, worker, collectRepairGraph)
	return err
}

type repairAttemptNode struct {
	workflow.BaseNode
	attempt, limit int
	host           RepairAttemptHost
}

func (n *repairAttemptNode) Run(ctx agent.Context, _ any) iter.Seq2[*session.Event, error] {
	return func(yield func(*session.Event, error) bool) {
		started := session.NewEvent(ctx, ctx.InvocationID())
		started.CustomMetadata = map[string]any{"mini_orca.attempt": n.attempt, "mini_orca.node_status": "running"}
		if !yield(started, nil) {
			return
		}
		if err := n.admit(ctx); err != nil {
			yield(nil, err)
			return
		}
		repair, err := n.host(ctx, n.attempt)
		if err == nil {
			err = context.Cause(ctx)
		}
		if err != nil {
			yield(nil, err)
			return
		}
		if repair && n.attempt >= n.limit {
			yield(nil, &BudgetError{Limit: "repairs"})
			return
		}
		next := "complete"
		if repair {
			next = "repair"
		}
		finished := session.NewEvent(ctx, ctx.InvocationID())
		finished.Output = RepairAttemptResult{Attempt: n.attempt, Next: next}
		finished.Routes = []string{next}
		yield(finished, nil)
	}
}

func (n *repairAttemptNode) admit(ctx context.Context) error {
	if err := context.Cause(ctx); err != nil {
		return err
	}
	if n.attempt > 0 {
		budget := BudgetFromContext(ctx)
		if budget == nil {
			return fmt.Errorf("repair requires a shared run budget")
		}
		return budget.AdmitRepair(ctx)
	}
	return nil
}

func collectRepairGraph(ctx context.Context, events iter.Seq2[*session.Event, error]) (int, error) {
	completed, next := false, 0
	for event, err := range events {
		if err != nil {
			return 0, err
		}
		if event == nil {
			return 0, fmt.Errorf("repair graph returned an empty event")
		}
		if event.Output == nil {
			continue
		}
		result, ok := event.Output.(RepairAttemptResult)
		if !ok || completed || result.Attempt != next || result.Next != "complete" && result.Next != "repair" {
			return 0, fmt.Errorf("repair graph returned an unexpected transition")
		}
		next++
		completed = result.Next == "complete"
	}
	if err := context.Cause(ctx); err != nil {
		return 0, err
	}
	if !completed {
		return 0, fmt.Errorf("repair graph ended before completing an attempt")
	}
	return next, nil
}
