package agentruntime

import (
	"context"
	"fmt"
	"iter"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/session"
	"google.golang.org/genai"

	"github.com/nanaki-93/mini-orca/v2/internal/llm"
)

// CompletionClient is the existing transport contract, including CLI providers.
// Implementations validate provider output before returning it to the runtime.
type CompletionClient interface {
	Chat(context.Context, []llm.ChatMessage) (*llm.ChatResponse, error)
	ChatWithJSONSchema(context.Context, []llm.ChatMessage, llm.JSONSchema) (*llm.ChatResponse, error)
}

func newCompletionAgent(client CompletionClient, request Request) (agent.Agent, error) {
	return agent.New(agent.Config{
		Name:        "completion",
		Description: "Execute one admitted model completion",
		Run: func(ctx agent.InvocationContext) iter.Seq2[*session.Event, error] {
			return func(yield func(*session.Event, error) bool) {
				result, err := requestCompletion(ctx, client, request)
				if err != nil {
					yield(nil, err)
					return
				}
				event := session.NewEvent(ctx, ctx.InvocationID())
				event.Author = ctx.Agent().Name()
				event.LLMResponse = model.LLMResponse{
					Content:      genai.NewContentFromText(result.Content, genai.RoleModel),
					ModelVersion: result.Model,
					TurnComplete: true,
				}
				event.Output = result
				yield(event, nil)
			}
		},
	})
}

func requestCompletion(ctx context.Context, client CompletionClient, request Request) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	var response *llm.ChatResponse
	var err error
	if request.Schema == nil {
		response, err = client.Chat(ctx, request.Messages)
	} else {
		response, err = client.ChatWithJSONSchema(ctx, request.Messages, *request.Schema)
	}
	if err != nil {
		return Result{}, err
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if response == nil || len(response.Choices) == 0 {
		return Result{}, fmt.Errorf("%w: completion has no choice", llm.ErrUnusableResponse)
	}
	choice := response.Choices[0]
	return Result{
		Content: choice.Message.Content, Model: response.Model,
		FinishReason: choice.FinishReason, Usage: response.Usage,
	}, nil
}
