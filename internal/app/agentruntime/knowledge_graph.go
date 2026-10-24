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

type KnowledgeKind string

const (
	KnowledgeOverview    KnowledgeKind = "overview"
	KnowledgeFeatures    KnowledgeKind = "features"
	KnowledgeExplanation KnowledgeKind = "explanation"
	KnowledgeSemantic    KnowledgeKind = "semantic"
	KnowledgeSecurity    KnowledgeKind = "security"
)

var knowledgeKinds = []KnowledgeKind{KnowledgeOverview, KnowledgeFeatures, KnowledgeExplanation, KnowledgeSemantic, KnowledgeSecurity}

// The host owns evidence, retries and publication. Only the operation kind and
// settled outcome cross this graph; model text stays in private child sessions.
type KnowledgeOutcome struct {
	Kind   KnowledgeKind `json:"kind"`
	Status string        `json:"status"`
}

func RunKnowledgeGraph(ctx context.Context, kind KnowledgeKind, host func(context.Context) (string, error)) (KnowledgeOutcome, error) {
	if !slices.Contains(knowledgeKinds, kind) || host == nil {
		return KnowledgeOutcome{}, fmt.Errorf("knowledge graph requires a known operation and host")
	}
	node := &knowledgeNode{BaseNode: workflow.NewBaseNode("knowledge_"+string(kind), "", workflow.NodeConfig{}), kind: kind, host: host}
	worker, err := graphAgent("knowledge", []workflow.Edge{{From: workflow.Start, To: node}}, 1, nil)
	if err != nil {
		return KnowledgeOutcome{}, err
	}
	return runGraph(ctx, "knowledge_graph", "knowledge-graph-v1", kind, knowledgeKinds, worker, func(ctx context.Context, events iter.Seq2[*session.Event, error]) (KnowledgeOutcome, error) {
		return collectKnowledge(ctx, events, kind)
	})
}

type knowledgeNode struct {
	workflow.BaseNode
	kind KnowledgeKind
	host func(context.Context) (string, error)
}

func (n *knowledgeNode) Run(ctx agent.Context, _ any) iter.Seq2[*session.Event, error] {
	return func(yield func(*session.Event, error) bool) {
		started := session.NewEvent(ctx, ctx.InvocationID())
		started.CustomMetadata = map[string]any{"mini_orca.knowledge_node": "knowledge_" + string(n.kind), "mini_orca.node_status": "running"}
		if !yield(started, nil) {
			return
		}
		if err := context.Cause(ctx); err != nil {
			yield(nil, err)
			return
		}
		status, err := tracedHost(ctx, "node", "knowledge_"+string(n.kind), n.host)
		if err == nil {
			err = context.Cause(ctx)
		}
		if err == nil && !validKnowledgeStatus(status) {
			err = fmt.Errorf("knowledge host returned an unsettled outcome")
		}
		if err != nil {
			yield(nil, fmt.Errorf("knowledge %s: %w", n.kind, err))
			return
		}
		finished := session.NewEvent(ctx, ctx.InvocationID())
		finished.Output = KnowledgeOutcome{Kind: n.kind, Status: status}
		yield(finished, nil)
	}
}

func collectKnowledge(ctx context.Context, events iter.Seq2[*session.Event, error], kind KnowledgeKind) (KnowledgeOutcome, error) {
	var result KnowledgeOutcome
	for event, err := range events {
		if err != nil {
			return result, err
		}
		if event == nil {
			return result, fmt.Errorf("knowledge graph returned an empty event")
		}
		if event.Output == nil {
			continue
		}
		outcome, ok := event.Output.(KnowledgeOutcome)
		if !ok || outcome.Kind != kind || !validKnowledgeStatus(outcome.Status) || result.Kind != "" {
			return result, fmt.Errorf("knowledge graph returned an unexpected outcome")
		}
		result = outcome
	}
	if err := context.Cause(ctx); err != nil {
		return result, err
	}
	if result.Kind == "" {
		return result, fmt.Errorf("knowledge graph ended without an outcome")
	}
	return result, nil
}

func validKnowledgeStatus(status string) bool {
	return slices.Contains([]string{"completed", "partial", "failed", "unavailable"}, status)
}

func knownKnowledgeNode(name string) bool {
	value, ok := strings.CutPrefix(name, "knowledge_")
	return ok && slices.Contains(knowledgeKinds, KnowledgeKind(value))
}
