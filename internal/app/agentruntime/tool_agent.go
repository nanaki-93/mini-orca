package agentruntime

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"slices"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/session"
	"google.golang.org/genai"
)

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
	protected     string
}

func (m *investigationModel) Name() string { return m.inner.Name() }
func (m *investigationModel) GenerateContent(ctx context.Context, request *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		request = m.protectContext(request)
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

func (m *investigationModel) protectContext(request *model.LLMRequest) *model.LLMRequest {
	if request == nil || m.protected == "" {
		return request
	}
	for _, content := range request.Contents {
		if content != nil && content.Role == genai.RoleUser && len(content.Parts) == 1 && content.Parts[0] != nil && content.Parts[0].Text == m.protected {
			return request
		}
	}
	copy := *request
	copy.Contents = append([]*genai.Content{genai.NewContentFromText(m.protected, genai.RoleUser)}, slices.Clone(request.Contents)...)
	return &copy
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
