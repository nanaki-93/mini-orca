package agentruntime

import (
	"context"
	"fmt"
	"iter"
	"net/http"

	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/llm"
)

// ProviderClient keeps provider selection immutable and exposes the existing
// project-analysis contract without leaking SDK types out of this package.
type ProviderClient struct {
	profile    config.ModelProfile
	compatible *llm.Client
	transport  http.RoundTripper
	optional   bool
}

func NewProviderClient(profile config.ModelProfile) *ProviderClient {
	client := &ProviderClient{profile: profile, transport: http.DefaultTransport}
	if !profile.IsGoogle() {
		client.compatible = llm.NewClient(profile)
	}
	return client
}

func (c *ProviderClient) ModelName() string { return c.profile.Model }

func (c *ProviderClient) WithOptionalFinalContent() *ProviderClient {
	copy := *c
	copy.optional = true
	if copy.compatible != nil {
		copy.compatible = copy.compatible.WithOptionalFinalContent()
	}
	return &copy
}

func (c *ProviderClient) Chat(ctx context.Context, messages []llm.ChatMessage) (*llm.ChatResponse, error) {
	if c.compatible != nil {
		return c.compatible.Chat(ctx, messages)
	}
	return c.chat(ctx, Request{Messages: messages})
}

func (c *ProviderClient) ChatWithJSONSchema(ctx context.Context, messages []llm.ChatMessage, schema llm.JSONSchema) (*llm.ChatResponse, error) {
	if c.compatible != nil {
		return c.compatible.ChatWithJSONSchema(ctx, messages, schema)
	}
	return c.chat(ctx, Request{Messages: messages, Schema: &schema})
}

func (c *ProviderClient) chat(ctx context.Context, request Request) (*llm.ChatResponse, error) {
	result, err := Complete(ctx, c, request)
	if err != nil {
		return nil, err
	}
	return &llm.ChatResponse{Model: result.Model, Usage: result.Usage, Choices: []llm.ChatChoice{{
		Message: llm.ChatMessage{Role: "assistant", Content: result.Content}, FinishReason: result.FinishReason,
	}}}, nil
}

// Generate is the compatibility protocol. Google agents use native model parts
// so their thought signatures survive tool turns without an intermediate wire format.
func (c *ProviderClient) Generate(ctx context.Context, request llm.GenerationRequest) iter.Seq2[llm.GenerationEvent, error] {
	if c.compatible != nil {
		return c.compatible.Generate(ctx, request)
	}
	return func(yield func(llm.GenerationEvent, error) bool) {
		yield(llm.GenerationEvent{}, fmt.Errorf("%w: Google models require the native ADK protocol", llm.ErrRequestRejected))
	}
}
