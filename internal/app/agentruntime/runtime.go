// Package agentruntime isolates ADK execution from application authority and wire types.
package agentruntime

import (
	"context"
	"fmt"
	"iter"
	"strings"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/adk/v2/session"

	"github.com/nanaki-93/mini-orca/v2/internal/llm"
)

// Request is an admitted completion. The caller owns retries and scope checks.
type Request struct {
	Messages []llm.ChatMessage
	Schema   *llm.JSONSchema
}

// Result retains provider metadata without exposing ADK types to callers.
type Result struct {
	Content      string
	Model        string
	FinishReason string
	Usage        llm.ChatUsage
}

// Complete runs exactly one provider attempt through an ADK custom agent.
// A private invocation session prevents unrelated requests from sharing history.
// The provider transport retains ownership of sampling, schemas and CLI isolation.
func Complete(ctx context.Context, client CompletionClient, request Request) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if client == nil {
		return Result{}, fmt.Errorf("model client is not configured")
	}
	if len(request.Messages) == 0 || strings.TrimSpace(request.Messages[0].Content) == "" {
		return Result{}, fmt.Errorf("model request is required")
	}
	completion, err := newCompletionAgent(client, snapshotRequest(request))
	if err != nil {
		return Result{}, fmt.Errorf("create completion agent: %w", err)
	}
	return runCompletion(ctx, completion)
}

func runCompletion(ctx context.Context, completion agent.Agent) (Result, error) {
	const appName, userID = "mini-orca", "local"
	sessions := session.InMemoryService()
	created, err := sessions.Create(ctx, &session.CreateRequest{AppName: appName, UserID: userID})
	if err != nil {
		return Result{}, fmt.Errorf("create completion session: %w", err)
	}
	runtime, err := runner.New(runner.Config{AppName: appName, Agent: completion, SessionService: sessions})
	if err != nil {
		return Result{}, fmt.Errorf("create completion runner: %w", err)
	}
	return collectCompletion(ctx, runtime.Run(ctx, userID, created.Session.ID(), nil, agent.RunConfig{}))
}

func collectCompletion(ctx context.Context, events iter.Seq2[*session.Event, error]) (Result, error) {
	var result Result
	completed := false
	for event, err := range events {
		if err != nil {
			return Result{}, fmt.Errorf("model request failed: %w", err)
		}
		if event == nil || event.Partial || !event.TurnComplete || !event.IsFinalResponse() || completed {
			return Result{}, fmt.Errorf("completion runner returned an invalid terminal event")
		}
		result, completed = event.Output.(Result)
		if !completed {
			return Result{}, fmt.Errorf("completion runner returned an invalid result")
		}
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if !completed {
		return Result{}, fmt.Errorf("completion runner ended without a result")
	}
	return result, nil
}

func snapshotRequest(request Request) Request {
	request.Messages = append([]llm.ChatMessage(nil), request.Messages...)
	if request.Schema != nil {
		schema := *request.Schema
		schema.Schema = append([]byte(nil), schema.Schema...)
		request.Schema = &schema
	}
	return request
}
