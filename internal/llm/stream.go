package llm

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

const maxStreamFrameBytes = 256 * 1024

type generationChunk struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			ChatMessage
			ToolCalls []struct {
				Index *int `json:"index"`
				generationWireCall
			} `json:"tool_calls"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage *ChatUsage      `json:"usage"`
	Error json.RawMessage `json:"error"`
}

type generationAccumulator struct {
	id, model, finish                   string
	message                             generationWireMessage
	usage                               *ChatUsage
	calls                               map[int]*generationCallAccumulator
	text, reasoning, alternateReasoning strings.Builder
}

type generationCallAccumulator struct {
	call            generationWireCall
	name, arguments strings.Builder
}

func (c *Client) consumeGenerationStream(ctx context.Context, response *http.Response, schemas map[string]*jsonschema.Schema, yield func(GenerationEvent, error) bool) {
	contentType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil || contentType != "text/event-stream" {
		yield(GenerationEvent{}, fmt.Errorf("%w: provider did not return an event stream", ErrUnusableResponse))
		return
	}
	limited := &io.LimitedReader{R: response.Body, N: maxProviderResponseBytes + 1}
	reader := bufio.NewReaderSize(limited, maxStreamFrameBytes)
	state := generationAccumulator{calls: make(map[int]*generationCallAccumulator)}
	for {
		if err := ctx.Err(); err != nil {
			yield(GenerationEvent{}, err)
			return
		}
		data, err := readGenerationFrame(reader)
		if err == nil && limited.N == 0 {
			err = fmt.Errorf("%w: stream exceeds byte limit", ErrUnusableResponse)
		}
		if err != nil {
			yield(GenerationEvent{}, generationStreamError(ctx, err))
			return
		}
		if string(data) == "[DONE]" {
			result, err := state.complete(c, schemas)
			if ctx.Err() != nil {
				result, err = nil, ctx.Err()
			}
			yield(GenerationEvent{Result: result}, err)
			return
		}
		text, err := state.append(data)
		if err != nil {
			yield(GenerationEvent{}, err)
			return
		}
		if text != "" && !yield(GenerationEvent{TextDelta: text}, nil) {
			return
		}
	}
}

func generationStreamError(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err == io.EOF {
		return fmt.Errorf("%w: stream ended before its completion marker", ErrUnusableResponse)
	}
	return fmt.Errorf("%w: cannot read complete stream frame", ErrUnusableResponse)
}

func readGenerationFrame(reader *bufio.Reader) ([]byte, error) {
	var data []byte
	frameBytes := 0
	for {
		line, err := reader.ReadSlice('\n')
		if err != nil {
			return nil, err
		}
		frameBytes += len(line)
		if frameBytes > maxStreamFrameBytes {
			return nil, fmt.Errorf("stream frame exceeds byte limit")
		}
		value := strings.TrimSuffix(strings.TrimSuffix(string(line), "\n"), "\r")
		if value == "" && len(data) != 0 {
			return data, nil
		}
		if value == "" {
			frameBytes = 0
		}
		if strings.HasPrefix(value, "event:") && strings.TrimSpace(strings.TrimPrefix(value, "event:")) == "error" {
			return nil, fmt.Errorf("provider reported a stream error")
		}
		if strings.HasPrefix(value, "data:") {
			if len(data) != 0 {
				data = append(data, '\n')
			}
			data = append(data, strings.TrimPrefix(strings.TrimPrefix(value, "data:"), " ")...)
		}
	}
}

func (a *generationAccumulator) append(data []byte) (string, error) {
	var chunk generationChunk
	if err := json.Unmarshal(data, &chunk); err != nil || len(chunk.Error) != 0 && string(chunk.Error) != "null" {
		return "", fmt.Errorf("%w: invalid generation stream event", ErrUnusableResponse)
	}
	if err := a.acceptIdentity(chunk); err != nil {
		return "", err
	}
	if err := a.acceptUsage(chunk); err != nil {
		return "", err
	}
	if a.isTrailingUsageChunk(chunk) {
		return "", nil
	}
	if len(chunk.Choices) != 1 || chunk.Choices[0].Index != 0 || a.finish != "" {
		return "", fmt.Errorf("%w: invalid or late generation delta", ErrUnusableResponse)
	}
	choice := chunk.Choices[0]
	if err := a.appendMessage(choice.Delta.ChatMessage); err != nil {
		return "", err
	}
	for _, call := range choice.Delta.ToolCalls {
		if call.Index == nil {
			return "", fmt.Errorf("%w: missing tool-call stream index", ErrUnusableResponse)
		}
		if err := a.appendCall(*call.Index, call.generationWireCall); err != nil {
			return "", err
		}
	}
	a.finish = choice.FinishReason
	return choice.Delta.Content, nil
}

func (a *generationAccumulator) isTrailingUsageChunk(chunk generationChunk) bool {
	if chunk.Usage == nil {
		return false
	}
	if len(chunk.Choices) == 0 {
		return true
	}
	if a.finish == "" || len(chunk.Choices) != 1 || chunk.Choices[0].Index != 0 || chunk.Choices[0].FinishReason != "" {
		return false
	}
	delta := chunk.Choices[0].Delta
	return (delta.ChatMessage == (ChatMessage{}) || delta.ChatMessage == (ChatMessage{Role: "assistant"})) && len(delta.ToolCalls) == 0
}

func (a *generationAccumulator) acceptUsage(chunk generationChunk) error {
	if chunk.Usage == nil {
		return nil
	}
	if a.usage != nil || a.finish == "" && (len(chunk.Choices) != 1 || chunk.Choices[0].FinishReason == "") {
		return fmt.Errorf("%w: invalid stream usage position", ErrUnusableResponse)
	}
	if err := validateGenerationUsage(chunk.Usage); err != nil {
		return err
	}
	a.usage = chunk.Usage
	return nil
}

func (a *generationAccumulator) acceptIdentity(chunk generationChunk) error {
	if a.id != "" && chunk.ID != "" && a.id != chunk.ID || a.model != "" && chunk.Model != "" && a.model != chunk.Model {
		return fmt.Errorf("%w: stream changed response identity", ErrUnusableResponse)
	}
	if chunk.ID != "" {
		a.id = chunk.ID
	}
	if chunk.Model != "" {
		a.model = chunk.Model
	}
	return nil
}

func (a *generationAccumulator) appendMessage(delta ChatMessage) error {
	if delta.Role != "" && delta.Role != "assistant" || delta.Refusal != "" {
		return fmt.Errorf("%w: stream refused or returned an invalid role", ErrUnusableResponse)
	}
	a.message.Role = "assistant"
	a.text.WriteString(delta.Content)
	a.reasoning.WriteString(delta.ReasoningContent)
	a.alternateReasoning.WriteString(delta.Reasoning)
	return nil
}

func (a *generationAccumulator) appendCall(index int, delta generationWireCall) error {
	if index < 0 || index >= maxGenerationCalls {
		return fmt.Errorf("%w: tool-call stream index exceeds limit", ErrUnusableResponse)
	}
	call := a.calls[index]
	if call == nil {
		call = &generationCallAccumulator{}
		a.calls[index] = call
	}
	if call.call.ID != "" && delta.ID != "" && call.call.ID != delta.ID || call.call.Type != "" && delta.Type != "" && call.call.Type != delta.Type {
		return fmt.Errorf("%w: tool-call stream identity changed", ErrUnusableResponse)
	}
	if delta.ID != "" {
		call.call.ID = delta.ID
	}
	if delta.Type != "" {
		call.call.Type = delta.Type
	}
	call.name.WriteString(delta.Function.Name)
	call.arguments.WriteString(delta.Function.Arguments)
	if call.name.Len() > 64 || call.arguments.Len() > maxToolJSONBytes {
		return fmt.Errorf("%w: tool-call stream exceeds argument limit", ErrUnusableResponse)
	}
	return nil
}

func (a *generationAccumulator) complete(c *Client, schemas map[string]*jsonschema.Schema) (*GenerationResult, error) {
	if a.finish == "" {
		return nil, fmt.Errorf("%w: stream has no terminal choice", ErrUnusableResponse)
	}
	a.message.Content = a.text.String()
	a.message.ReasoningContent = a.reasoning.String()
	a.message.Reasoning = a.alternateReasoning.String()
	for index := range len(a.calls) {
		call, exists := a.calls[index]
		if !exists {
			return nil, fmt.Errorf("%w: stream has a gap in tool-call indices", ErrUnusableResponse)
		}
		value := call.call
		value.Function.Name, value.Function.Arguments = call.name.String(), call.arguments.String()
		a.message.ToolCalls = append(a.message.ToolCalls, value)
	}
	return c.validateGenerationResult(a.message, a.model, a.finish, a.usage, schemas, true)
}
