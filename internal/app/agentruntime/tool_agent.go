package agentruntime

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/tool"

	"github.com/nanaki-93/mini-orca/v2/internal/llm"
)

func newInvestigationAgent(client CompletionClient, request Request) (agent.Agent, error) {
	if len(request.Messages) != 1 || request.Messages[0].Role != "user" || request.Stream || request.Schema != nil {
		return nil, fmt.Errorf("%w: investigation requires one admitted user prompt", llm.ErrRequestRejected)
	}
	var adapter model.LLM
	if provider, ok := client.(*ProviderClient); ok && provider.profile.IsGoogle() {
		adapter = &googleModel{client: provider}
	} else if transport, ok := client.(generationClient); ok {
		adapter = &compatibleModel{client: transport}
	} else {
		return nil, fmt.Errorf("%w: selected provider cannot use investigation tools", llm.ErrRequestRejected)
	}
	var tools []tool.Tool
	for _, item := range request.Investigation.tools {
		tools = append(tools, item)
	}
	worker, err := llmagent.New(llmagent.Config{
		Name: "investigation", Description: "Investigate the explicitly admitted repository scope",
		Model: &investigationModel{inner: adapter, scope: request.Investigation, contextTokens: request.ContextMaxTokens}, Tools: tools,
		DisallowTransferToParent: true, DisallowTransferToPeers: true,
	})
	if err != nil {
		return nil, err
	}
	return agent.New(agent.Config{Name: "investigation_runner", Description: "Bounded repository investigation", Run: investigationRun(worker)})
}

func investigationRun(worker agent.Agent) func(agent.InvocationContext) iter.Seq2[*session.Event, error] {
	return func(ctx agent.InvocationContext) iter.Seq2[*session.Event, error] {
		return func(yield func(*session.Event, error) bool) {
			for event, err := range worker.Run(ctx) {
				if err == nil && event != nil && !event.Partial && event.Content != nil && event.IsFinalResponse() {
					event.Output = completionResult(&event.LLMResponse)
					event.TurnComplete = true
				}
				if !yield(event, err) || err != nil {
					return
				}
			}
		}
	}
}

type investigationModel struct {
	inner         model.LLM
	scope         *Investigation
	contextTokens int
}

func (m *investigationModel) Name() string { return m.inner.Name() }
func (m *investigationModel) GenerateContent(ctx context.Context, request *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		if err := m.validateContext(request); err != nil {
			CancelRun(ctx, err)
			yield(nil, err)
			return
		}
		if err := m.scope.admitModel(ctx); err != nil {
			CancelRun(ctx, err)
			yield(nil, err)
			return
		}
		for response, err := range m.inner.GenerateContent(ctx, request, stream) {
			if err == nil {
				err = authorizeInvestigation(ctx)
			}
			if !yield(response, err) || err != nil {
				return
			}
		}
	}
}

func (m *investigationModel) validateContext(request *model.LLMRequest) error {
	limit := 128 << 10
	if m.contextTokens > 0 && m.contextTokens < limit/4 {
		limit = m.contextTokens * 4
	}
	data, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("encode investigation context: %w", err)
	}
	if len(data) > limit {
		return fmt.Errorf("%w: investigation context exceeds %d bytes", ErrBudgetExceeded, limit)
	}
	return nil
}
