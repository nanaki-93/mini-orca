package agentruntime

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"reflect"
	"sync"
	"testing"

	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/session"

	"github.com/nanaki-93/mini-orca/v2/internal/llm"
)

type completionFixture func(context.Context, []llm.ChatMessage, *llm.JSONSchema) (*llm.ChatResponse, error)

func (f completionFixture) Chat(ctx context.Context, messages []llm.ChatMessage) (*llm.ChatResponse, error) {
	return f(ctx, messages, nil)
}

func (f completionFixture) ChatWithJSONSchema(ctx context.Context, messages []llm.ChatMessage, schema llm.JSONSchema) (*llm.ChatResponse, error) {
	return f(ctx, messages, &schema)
}

func fixtureResponse(content string) *llm.ChatResponse {
	return &llm.ChatResponse{
		Model: "fixture-model", Usage: llm.ChatUsage{PromptTokens: 31, CompletionTokens: 7, TotalTokens: 38},
		Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Role: "assistant", Content: content}, FinishReason: "stop"}},
	}
}

func TestCompletionRunnerPreservesConversationSchemaAndMetadata(t *testing.T) {
	messages := []llm.ChatMessage{
		{Role: "system", Content: "Keep this exact policy."},
		{Role: "user", Content: "Original request"},
		{Role: "assistant", Content: "Prior answer", ReasoningContent: "retained field", Reasoning: "alternate field"},
		{Role: "user", Content: "Follow-up request"},
	}
	for _, schema := range []*llm.JSONSchema{nil, {Name: "answer", Schema: []byte(`{"type":"object"}`)}} {
		calls := 0
		client := completionFixture(func(_ context.Context, got []llm.ChatMessage, gotSchema *llm.JSONSchema) (*llm.ChatResponse, error) {
			calls++
			if !reflect.DeepEqual(got, messages) || !reflect.DeepEqual(gotSchema, schema) {
				t.Fatalf("request changed: messages=%+v schema=%+v", got, gotSchema)
			}
			return fixtureResponse(`{"answer":"done"}`), nil
		})
		result, err := Complete(context.Background(), client, Request{Messages: messages, Schema: schema})
		want := Result{Content: `{"answer":"done"}`, Model: "fixture-model", FinishReason: "stop", Usage: fixtureResponse("").Usage}
		if err != nil || result != want || calls != 1 {
			t.Fatalf("result=%+v, error=%v, calls=%d", result, err, calls)
		}
	}
}

func TestCompletionDoesNotRetryProviderFailures(t *testing.T) {
	for _, cause := range []error{llm.ErrRedirectRejected, llm.ErrStructuredRequestRejected, llm.ErrRequestRejected, context.DeadlineExceeded} {
		t.Run(cause.Error(), func(t *testing.T) {
			calls := 0
			client := completionFixture(func(context.Context, []llm.ChatMessage, *llm.JSONSchema) (*llm.ChatResponse, error) {
				calls++
				return nil, cause
			})
			result, err := Complete(context.Background(), client, Request{Messages: []llm.ChatMessage{{Content: "request"}}})
			if !errors.Is(err, cause) || calls != 1 || result != (Result{}) {
				t.Fatalf("result=%+v, error=%v, calls=%d", result, err, calls)
			}
		})
	}
}

func TestCompletionCancellationEndsOwnedInvocation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	entered := make(chan struct{})
	done := make(chan error, 1)
	client := completionFixture(func(ctx context.Context, _ []llm.ChatMessage, _ *llm.JSONSchema) (*llm.ChatResponse, error) {
		close(entered)
		<-ctx.Done()
		return nil, ctx.Err()
	})
	go func() {
		_, err := Complete(ctx, client, Request{Messages: []llm.ChatMessage{{Content: "request"}}})
		done <- err
	}()
	<-entered
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation error=%v", err)
	}
}

func TestCompletionRejectsLateResultAfterCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client := completionFixture(func(context.Context, []llm.ChatMessage, *llm.JSONSchema) (*llm.ChatResponse, error) {
		cancel()
		return fixtureResponse("late answer"), nil
	})
	result, err := Complete(ctx, client, Request{Messages: []llm.ChatMessage{{Content: "request"}}})
	if !errors.Is(err, context.Canceled) || result != (Result{}) {
		t.Fatalf("late result=%+v, error=%v", result, err)
	}
}

func TestCompletionRejectsMissingChoiceButPreservesTransportEmptyPolicy(t *testing.T) {
	for _, test := range []struct {
		response *llm.ChatResponse
		valid    bool
	}{{nil, false}, {&llm.ChatResponse{}, false}, {fixtureResponse(""), true}} {
		client := completionFixture(func(context.Context, []llm.ChatMessage, *llm.JSONSchema) (*llm.ChatResponse, error) {
			return test.response, nil
		})
		_, err := Complete(context.Background(), client, Request{Messages: []llm.ChatMessage{{Content: "request"}}})
		if test.valid && err != nil || !test.valid && !errors.Is(err, llm.ErrUnusableResponse) {
			t.Fatalf("valid=%v, error=%v", test.valid, err)
		}
	}
}

func TestCompletionSnapshotsCallerInput(t *testing.T) {
	request := Request{
		Messages: []llm.ChatMessage{{Content: "original"}},
		Schema:   &llm.JSONSchema{Name: "original", Schema: []byte(`{"type":"object"}`)},
	}
	client := completionFixture(func(_ context.Context, messages []llm.ChatMessage, schema *llm.JSONSchema) (*llm.ChatResponse, error) {
		messages[0].Content = "mutated"
		schema.Schema[0] = 'X'
		return fixtureResponse("done"), nil
	})
	if _, err := Complete(context.Background(), client, request); err != nil {
		t.Fatal(err)
	}
	if request.Messages[0].Content != "original" || string(request.Schema.Schema) != `{"type":"object"}` {
		t.Fatalf("caller input mutated: %+v", request)
	}
}

func TestCompletionInvocationsHaveIndependentHistory(t *testing.T) {
	var workers sync.WaitGroup
	for i := range 4 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			want := fmt.Sprintf("request-%d", i)
			client := completionFixture(func(_ context.Context, messages []llm.ChatMessage, _ *llm.JSONSchema) (*llm.ChatResponse, error) {
				return fixtureResponse(messages[0].Content), nil
			})
			result, err := Complete(context.Background(), client, Request{Messages: []llm.ChatMessage{{Content: want}}})
			if err != nil || result.Content != want {
				t.Errorf("result=%+v, error=%v, want=%s", result, err, want)
			}
		}()
	}
	workers.Wait()
}

func TestCompletionRequiresOneTerminalResultAndDrainsErrors(t *testing.T) {
	final := &session.Event{LLMResponse: model.LLMResponse{TurnComplete: true}, Output: Result{Content: "done"}}
	partial := &session.Event{LLMResponse: model.LLMResponse{Partial: true}, Output: Result{Content: "unfinished"}}
	for _, events := range [][]*session.Event{nil, {nil}, {partial}, {final, final}, {{LLMResponse: model.LLMResponse{TurnComplete: true}}}} {
		stream := func(yield func(*session.Event, error) bool) {
			for _, event := range events {
				if !yield(event, nil) {
					return
				}
			}
		}
		if result, err := collectCompletion(context.Background(), stream); err == nil || result != (Result{}) {
			t.Fatalf("invalid stream accepted: result=%+v error=%v", result, err)
		}
	}
	cause := errors.New("terminal persistence failed")
	stream := iter.Seq2[*session.Event, error](func(yield func(*session.Event, error) bool) {
		if yield(final, nil) {
			yield(nil, cause)
		}
	})
	if result, err := collectCompletion(context.Background(), stream); !errors.Is(err, cause) || result != (Result{}) {
		t.Fatalf("post-result error lost: result=%+v error=%v", result, err)
	}
}

func TestInvalidOrCanceledCompletionDoesNotDispatch(t *testing.T) {
	client := completionFixture(func(context.Context, []llm.ChatMessage, *llm.JSONSchema) (*llm.ChatResponse, error) {
		t.Fatal("invalid request reached provider")
		return nil, nil
	})
	for _, request := range []Request{{}, {Messages: []llm.ChatMessage{{Content: " "}}}} {
		if _, err := Complete(context.Background(), client, request); err == nil {
			t.Fatal("invalid request accepted")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Complete(ctx, client, Request{Messages: []llm.ChatMessage{{Content: "request"}}}); !errors.Is(err, context.Canceled) {
		t.Fatalf("pre-canceled error=%v", err)
	}
}
