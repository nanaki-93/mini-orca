package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/json/jsontext"
	"errors"
	"fmt"
	"io"
	"iter"
	"math"
	"net/http"
	"regexp"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

const (
	maxGenerationTools = 32
	maxGenerationCalls = 16
	maxToolJSONBytes   = 64 * 1024
)

var toolNamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// These diagnostics retain refusal/finish meaning without retaining provider
// explanation text. Both are also unusable responses for existing callers.
var (
	ErrGenerationRefused    = fmt.Errorf("%w: provider declined generation", ErrUnusableResponse)
	ErrGenerationIncomplete = fmt.Errorf("%w: generation did not finish normally", ErrUnusableResponse)
)

// ToolDefinition declares the complete function schema admitted for a request.
type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters"`
}

// ToolCall preserves provider call identity and validated JSON arguments.
type ToolCall struct {
	ID        string
	Name      string
	Arguments json.RawMessage
}

// GenerationMessage supports text, assistant calls and matching tool responses.
type GenerationMessage struct {
	ChatMessage
	ToolCalls  []ToolCall
	ToolCallID string
	ToolName   string
}

type GenerationRequest struct {
	Messages []GenerationMessage
	Tools    []ToolDefinition
	Schema   *JSONSchema
	Stream   bool
}

// GenerationResult exists only after the complete response has been validated.
// Nil Usage means the provider did not report usage.
type GenerationResult struct {
	Message      GenerationMessage
	Model        string
	FinishReason string
	Usage        *ChatUsage
}

// GenerationEvent carries either inert partial text or one complete result.
type GenerationEvent struct {
	TextDelta string
	Result    *GenerationResult
}

type generationWireCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type generationWireMessage struct {
	ChatMessage
	ToolCalls  []generationWireCall `json:"tool_calls,omitempty"`
	ToolCallID string               `json:"tool_call_id,omitempty"`
}

type generationWireRequest struct {
	ChatRequest
	Messages []generationWireMessage `json:"messages"`
	Tools    []struct {
		Type     string         `json:"type"`
		Function ToolDefinition `json:"function"`
	} `json:"tools,omitempty"`
	StreamOptions *struct {
		IncludeUsage bool `json:"include_usage"`
	} `json:"stream_options,omitempty"`
}

type generationWireResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		Index        int                   `json:"index"`
		Message      generationWireMessage `json:"message"`
		FinishReason string                `json:"finish_reason"`
	} `json:"choices"`
	Usage *ChatUsage `json:"usage"`
}

// ModelName identifies the immutable model selected for this transport.
func (c *Client) ModelName() string { return c.profile.Model }

// Generate performs one bounded exchange. Stopping iteration closes the body;
// no provider retry, tool execution or background reader is hidden here.
func (c *Client) Generate(ctx context.Context, input GenerationRequest) iter.Seq2[GenerationEvent, error] {
	return func(yield func(GenerationEvent, error) bool) {
		if err := ctx.Err(); err != nil {
			yield(GenerationEvent{}, err)
			return
		}
		schemas, err := validateGenerationRequest(input)
		if err != nil {
			yield(GenerationEvent{}, err)
			return
		}
		if c.profile.IsCLI() {
			result, err := c.generateCLI(ctx, input)
			yield(GenerationEvent{Result: result}, err)
			return
		}
		request, err := c.generationHTTPRequest(ctx, input)
		if err != nil {
			yield(GenerationEvent{}, err)
			return
		}
		c.generateHTTP(request, input, schemas, func(event GenerationEvent, err error) bool {
			if err == nil && event.Result != nil {
				err = validateFreshCallIDs(event.Result.Message.ToolCalls, input.Messages)
			}
			if err != nil {
				yield(GenerationEvent{}, err)
				return false
			}
			return yield(event, nil)
		})
	}
}

func (c *Client) generateHTTP(request *http.Request, input GenerationRequest, schemas map[string]*jsonschema.Schema, yield func(GenerationEvent, error) bool) {
	response, cancel, err := c.openResponse(request)
	if err != nil {
		if input.Schema != nil && errors.Is(err, ErrRequestRejected) {
			err = fmt.Errorf("%w: %v", ErrStructuredRequestRejected, err)
		}
		yield(GenerationEvent{}, err)
		return
	}
	defer response.Body.Close()
	defer cancel()
	if input.Stream {
		c.consumeGenerationStream(response.Request.Context(), response, schemas, yield)
		return
	}
	result, err := c.readGeneration(response.Body, schemas)
	if err == nil {
		err = response.Request.Context().Err()
	}
	if err != nil {
		result = nil
	}
	yield(GenerationEvent{Result: result}, err)
}

func (c *Client) generateCLI(ctx context.Context, input GenerationRequest) (*GenerationResult, error) {
	if input.Stream || len(input.Tools) != 0 {
		return nil, fmt.Errorf("%w: CLI providers support completions without tools or streaming", ErrRequestRejected)
	}
	messages := make([]ChatMessage, len(input.Messages))
	for i, message := range input.Messages {
		if len(message.ToolCalls) != 0 || message.ToolCallID != "" {
			return nil, fmt.Errorf("%w: CLI providers do not accept tool history", ErrRequestRejected)
		}
		messages[i] = message.ChatMessage
	}
	var response *ChatResponse
	var err error
	if input.Schema == nil {
		response, err = c.Chat(ctx, messages)
	} else {
		response, err = c.ChatWithJSONSchema(ctx, messages, *input.Schema)
	}
	if err != nil {
		return nil, err
	}
	choice := response.Choices[0]
	result := &GenerationResult{Message: GenerationMessage{ChatMessage: choice.Message}, Model: response.Model, FinishReason: choice.FinishReason}
	if response.Usage != (ChatUsage{}) {
		usage := response.Usage
		result.Usage = &usage
	}
	return result, nil
}

func (c *Client) generationHTTPRequest(ctx context.Context, input GenerationRequest) (*http.Request, error) {
	var format *ResponseFormat
	var err error
	if input.Schema != nil {
		format, err = input.Schema.responseFormat()
		if err != nil {
			return nil, err
		}
	}
	wire := generationWireRequest{ChatRequest: c.chatRequest(nil, format)}
	for _, message := range input.Messages {
		wire.Messages = append(wire.Messages, generationMessageToWire(message))
	}
	for _, tool := range input.Tools {
		wire.Tools = append(wire.Tools, struct {
			Type     string         `json:"type"`
			Function ToolDefinition `json:"function"`
		}{"function", tool})
	}
	wire.Stream = input.Stream
	if input.Stream {
		wire.StreamOptions = &struct {
			IncludeUsage bool `json:"include_usage"`
		}{true}
	}
	body, err := json.Marshal(wire)
	if err != nil || len(body) > maxProviderResponseBytes {
		return nil, fmt.Errorf("%w: generation request is invalid or too large", ErrRequestRejected)
	}
	request, err := c.newRequest(ctx, http.MethodPost, "chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	return request, nil
}

func generationMessageToWire(message GenerationMessage) generationWireMessage {
	wire := generationWireMessage{ChatMessage: message.ChatMessage, ToolCallID: message.ToolCallID}
	for _, call := range message.ToolCalls {
		value := generationWireCall{ID: call.ID, Type: "function"}
		value.Function.Name, value.Function.Arguments = call.Name, string(call.Arguments)
		wire.ToolCalls = append(wire.ToolCalls, value)
	}
	return wire
}

func (c *Client) readGeneration(body io.Reader, schemas map[string]*jsonschema.Schema) (*GenerationResult, error) {
	data, err := readProviderBody(body)
	if err != nil {
		return nil, err
	}
	var response generationWireResponse
	if err := json.Unmarshal(data, &response); err != nil || len(response.Choices) != 1 || response.Choices[0].Index != 0 {
		return nil, fmt.Errorf("%w: expected one valid generation choice", ErrUnusableResponse)
	}
	choice := response.Choices[0]
	return c.validateGenerationResult(choice.Message, response.Model, choice.FinishReason, response.Usage, schemas, false)
}

func (c *Client) validateGenerationResult(wire generationWireMessage, model, finish string, usage *ChatUsage, schemas map[string]*jsonschema.Schema, streaming bool) (*GenerationResult, error) {
	result := &GenerationResult{Message: GenerationMessage{ChatMessage: wire.ChatMessage}, Model: model, FinishReason: finish, Usage: usage}
	if wire.Refusal != "" {
		return nil, ErrGenerationRefused
	}
	if err := validateGenerationUsage(usage); err != nil {
		return nil, err
	}
	if len(wire.ToolCalls) == 0 {
		if streaming && finish != "stop" || finish == "tool_calls" {
			return nil, ErrGenerationIncomplete
		}
		err := c.validateFinalResponse(ChatResponse{Choices: []ChatChoice{{Message: wire.ChatMessage, FinishReason: finish}}})
		if err != nil {
			return nil, err
		}
		return result, nil
	}
	if wire.Role != "assistant" || finish != "tool_calls" || len(wire.ToolCalls) > maxGenerationCalls {
		return nil, fmt.Errorf("%w: invalid tool-call completion", ErrUnusableResponse)
	}
	calls, err := validatedGenerationCalls(wire.ToolCalls, schemas)
	if err != nil {
		return nil, err
	}
	result.Message.ToolCalls = calls
	return result, nil
}

func validatedGenerationCalls(wire []generationWireCall, schemas map[string]*jsonschema.Schema) ([]ToolCall, error) {
	seen := make(map[string]bool)
	var calls []ToolCall
	for _, call := range wire {
		value := ToolCall{ID: call.ID, Name: call.Function.Name, Arguments: json.RawMessage(call.Function.Arguments)}
		if call.Type != "function" || seen[value.ID] {
			return nil, fmt.Errorf("%w: invalid or repeated tool call", ErrUnusableResponse)
		}
		if err := validateToolCall(value, schemas); err != nil {
			return nil, fmt.Errorf("%w: invalid tool call", ErrUnusableResponse)
		}
		seen[value.ID] = true
		calls = append(calls, value)
	}
	return calls, nil
}

func validateFreshCallIDs(calls []ToolCall, messages []GenerationMessage) error {
	for _, message := range messages {
		for _, previous := range message.ToolCalls {
			for _, call := range calls {
				if call.ID == previous.ID {
					return fmt.Errorf("%w: provider reused a tool-call identity", ErrUnusableResponse)
				}
			}
		}
	}
	return nil
}

func validateGenerationUsage(usage *ChatUsage) error {
	if usage == nil {
		return nil
	}
	for _, count := range []int{usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens} {
		if count < 0 || count > math.MaxInt32 {
			return fmt.Errorf("%w: invalid token usage", ErrUnusableResponse)
		}
	}
	if details := usage.PromptTokensDetails; details != nil && (details.CachedTokens < 0 || details.CachedTokens > usage.PromptTokens) {
		return fmt.Errorf("%w: invalid cached token usage", ErrUnusableResponse)
	}
	return nil
}

func validateGenerationRequest(input GenerationRequest) (map[string]*jsonschema.Schema, error) {
	if len(input.Messages) == 0 || len(input.Tools) > maxGenerationTools {
		return nil, fmt.Errorf("%w: invalid generation scope", ErrRequestRejected)
	}
	encoded, err := json.Marshal(input)
	if err != nil || len(encoded) > maxProviderResponseBytes {
		return nil, fmt.Errorf("%w: generation input is invalid or too large", ErrRequestRejected)
	}
	schemas := make(map[string]*jsonschema.Schema)
	for _, tool := range input.Tools {
		if !toolNamePattern.MatchString(tool.Name) || schemas[tool.Name] != nil || len(tool.Parameters) > maxToolJSONBytes || len(tool.Description) > 8192 {
			return nil, fmt.Errorf("%w: invalid tool declaration", ErrRequestRejected)
		}
		document, err := decodeToolObject(tool.Parameters)
		if err != nil || document["type"] != nil && document["type"] != "object" {
			return nil, fmt.Errorf("%w: invalid tool schema", ErrRequestRejected)
		}
		schema, err := compileCLISchema(&ResponseFormat{JSONSchema: &ResponseFormatJSONSchema{Name: tool.Name, Schema: tool.Parameters}})
		if err != nil {
			return nil, fmt.Errorf("%w: unsupported tool schema", ErrRequestRejected)
		}
		schemas[tool.Name] = schema
	}
	if err := validateToolHistory(input.Messages, schemas); err != nil {
		return nil, err
	}
	return schemas, nil
}

func validateToolHistory(messages []GenerationMessage, schemas map[string]*jsonschema.Schema) error {
	pending, seen := make(map[string]string), make(map[string]bool)
	for _, message := range messages {
		if message.Role == "tool" {
			if pending[message.ToolCallID] == "" || len(message.ToolCalls) != 0 || message.ToolName != "" && message.ToolName != pending[message.ToolCallID] {
				return fmt.Errorf("%w: unmatched tool response", ErrRequestRejected)
			}
			delete(pending, message.ToolCallID)
			continue
		}
		if len(pending) != 0 || message.ToolCallID != "" || message.ToolName != "" {
			return fmt.Errorf("%w: tool responses must precede further messages", ErrRequestRejected)
		}
		if err := validateMessageCalls(message, schemas, seen, pending); err != nil {
			return err
		}
	}
	if len(pending) != 0 {
		return fmt.Errorf("%w: missing tool responses", ErrRequestRejected)
	}
	return nil
}

func validateMessageCalls(message GenerationMessage, schemas map[string]*jsonschema.Schema, seen map[string]bool, pending map[string]string) error {
	switch message.Role {
	case "system", "developer", "user", "assistant":
	default:
		return fmt.Errorf("%w: unsupported message role", ErrRequestRejected)
	}
	if len(message.ToolCalls) > maxGenerationCalls || len(message.ToolCalls) != 0 && message.Role != "assistant" {
		return fmt.Errorf("%w: invalid tool-call history", ErrRequestRejected)
	}
	for _, call := range message.ToolCalls {
		if seen[call.ID] {
			return fmt.Errorf("%w: repeated tool-call identity", ErrRequestRejected)
		}
		if err := validateToolCall(call, schemas); err != nil {
			return err
		}
		seen[call.ID], pending[call.ID] = true, call.Name
	}
	return nil
}

func validateToolCall(call ToolCall, schemas map[string]*jsonschema.Schema) error {
	if !validToolCallID(call.ID) || schemas[call.Name] == nil {
		return fmt.Errorf("%w: unknown tool or invalid call identity", ErrRequestRejected)
	}
	arguments, err := decodeToolObject(call.Arguments)
	if err != nil || schemas[call.Name].Validate(arguments) != nil {
		return fmt.Errorf("%w: tool arguments do not match the admitted schema", ErrRequestRejected)
	}
	return nil
}

func validToolCallID(id string) bool {
	if len(id) == 0 || len(id) > 128 {
		return false
	}
	for _, value := range id {
		if value < '!' || value > '~' {
			return false
		}
	}
	return true
}

func decodeToolObject(data []byte) (map[string]any, error) {
	if len(data) == 0 || len(data) > maxToolJSONBytes {
		return nil, fmt.Errorf("invalid tool JSON size")
	}
	decoder := jsontext.NewDecoder(bytes.NewReader(data))
	if _, err := decoder.ReadValue(); err != nil {
		return nil, fmt.Errorf("invalid tool JSON")
	}
	if _, err := decoder.ReadValue(); err != io.EOF {
		return nil, fmt.Errorf("trailing tool JSON")
	}
	value, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	object, ok := value.(map[string]any)
	if err != nil || !ok || object == nil {
		return nil, fmt.Errorf("tool JSON must be an object")
	}
	return object, nil
}
