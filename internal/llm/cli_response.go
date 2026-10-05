package llm

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/config"
)

type cliResponseDecoder struct {
	profile    config.ModelProfile
	response   *ChatResponse
	structured bool
	started    bool
	finished   bool
}

type agyStepEvent struct {
	Type     string `json:"step_type"`
	ToolName string `json:"tool_name"`
	ToolInfo *struct {
		Name string `json:"name"`
	} `json:"tool_info"`
}

func decodeCLIResponse(profile config.ModelProfile, output []byte, structured bool) (*ChatResponse, error) {
	decoder := cliResponseDecoder{profile: profile, structured: structured}
	scanner := bufio.NewScanner(bytes.NewReader(output))
	scanner.Buffer(make([]byte, 4096), maxProviderResponseBytes)
	for scanner.Scan() {
		if len(bytes.TrimSpace(scanner.Bytes())) == 0 {
			continue
		}
		var err error
		if profile.Provider == config.AgyProvider {
			err = decoder.agyEvent(scanner.Bytes())
		} else {
			err = decoder.piEvent(scanner.Bytes())
		}
		if err != nil {
			return nil, err
		}
	}
	if scanner.Err() != nil || !decoder.started || !decoder.finished || decoder.response == nil {
		return nil, unusableCLIResponse("incomplete CLI response")
	}
	return decoder.response, nil
}

func (d *cliResponseDecoder) agyEvent(data []byte) error {
	var event struct {
		Event string `json:"event"`
		Init  struct {
			Tools *[]string `json:"tools"`
			Agent string    `json:"agent"`
			Model string    `json:"model"`
		} `json:"init"`
		Step   agyStepEvent `json:"step_update"`
		Result struct {
			Status           string          `json:"status"`
			Response         string          `json:"response"`
			StructuredOutput json.RawMessage `json:"structured_output"`
			Usage            struct {
				Input  int `json:"input_tokens"`
				Output int `json:"output_tokens"`
				Total  int `json:"total_tokens"`
			} `json:"usage"`
		} `json:"result"`
	}
	if json.Unmarshal(data, &event) != nil {
		return unusableCLIResponse("malformed agy event")
	}
	switch event.Event {
	case "init":
		// init.tools is the CLI's catalog, not the custom agent's allowed tools.
		if d.started || event.Init.Tools == nil || event.Init.Agent != "mini-orca" || event.Init.Model != d.profile.Model {
			return unusableCLIResponse("agy did not select the configured model and agent")
		}
		d.started = true
	case "step_update":
		return d.agyStep(event.Step)
	case "result":
		if !d.started || d.finished || event.Result.Status != "SUCCESS" {
			return unusableCLIResponse("agy did not complete successfully")
		}
		content, err := agyFinalContent(event.Result.Response, event.Result.StructuredOutput, d.structured)
		if err != nil {
			return err
		}
		d.finished = true
		d.response = cliChatResponse(d.profile.Model, content, ChatUsage{
			PromptTokens: event.Result.Usage.Input, CompletionTokens: event.Result.Usage.Output, TotalTokens: event.Result.Usage.Total,
		})
	default:
		return unusableCLIResponse("unrecognized agy event")
	}
	return nil
}

func (d *cliResponseDecoder) agyStep(step agyStepEvent) error {
	if !d.started || d.finished {
		return unusableCLIResponse("agy progress is outside its model turn")
	}
	if step.Type == "subagent" {
		return unusableCLIResponse("agy attempted tool use")
	}
	if step.Type != "tool" {
		return nil
	}
	// Schema requests enable finish solely to format the native result.
	if !d.structured || step.ToolName != "finish" {
		return unusableCLIResponse("agy attempted tool use")
	}
	if step.ToolInfo != nil && step.ToolInfo.Name != "" && step.ToolInfo.Name != "finish" {
		return unusableCLIResponse("agy reported conflicting tool names")
	}
	return nil
}

func agyFinalContent(response string, output json.RawMessage, structured bool) (string, error) {
	if !structured {
		return response, nil
	}
	if len(output) != 0 {
		return string(output), nil
	}
	// Preserve the existing optional blank-review contract. Nonempty prose is
	// never a substitute for the native schema result, even if it parses as JSON.
	if strings.TrimSpace(response) == "" {
		return response, nil
	}
	return "", unusableCLIResponse("agy did not return the required structured output")
}

func (d *cliResponseDecoder) piEvent(data []byte) error {
	var event struct {
		Type      string          `json:"type"`
		Message   json.RawMessage `json:"message"`
		WillRetry bool            `json:"willRetry"`
	}
	if json.Unmarshal(data, &event) != nil {
		return unusableCLIResponse("malformed pi event")
	}
	switch event.Type {
	case "agent_start":
		if d.started {
			return unusableCLIResponse("pi attempted another model turn")
		}
		d.started = true
	case "message_end":
		return d.piMessage(event.Message)
	case "agent_end":
		if !d.started || d.finished || event.WillRetry {
			return unusableCLIResponse("pi did not complete successfully")
		}
		d.finished = true
	case "tool_execution_start", "tool_execution_update", "tool_execution_end", "auto_retry_start", "compaction_start", "error", "extension_error":
		return unusableCLIResponse("pi attempted tools, additional requests, or reported an error")
	}
	return nil
}

func (d *cliResponseDecoder) piMessage(data json.RawMessage) error {
	var role struct {
		Role string `json:"role"`
	}
	if json.Unmarshal(data, &role) != nil {
		return unusableCLIResponse("malformed pi message")
	}
	if role.Role == "user" {
		return nil
	}
	if role.Role != "assistant" {
		return unusableCLIResponse("pi returned an unexpected message role")
	}
	var message struct {
		StopReason string `json:"stopReason"`
		Model      string `json:"model"`
		Content    []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Usage struct {
			Input  int `json:"input"`
			Output int `json:"output"`
			Total  int `json:"totalTokens"`
		} `json:"usage"`
	}
	if json.Unmarshal(data, &message) != nil || message.StopReason != "stop" || !d.started || d.finished || d.response != nil {
		return unusableCLIResponse("pi assistant response did not complete normally")
	}
	if !matchesPiModel(d.profile.Model, message.Model) {
		return unusableCLIResponse("pi returned a different model than configured; use an exact model ID")
	}
	var content strings.Builder
	for _, block := range message.Content {
		switch block.Type {
		case "text":
			content.WriteString(block.Text)
		case "thinking":
		default:
			return unusableCLIResponse("pi returned non-text assistant content")
		}
	}
	d.response = cliChatResponse(message.Model, content.String(), ChatUsage{
		PromptTokens: message.Usage.Input, CompletionTokens: message.Usage.Output, TotalTokens: message.Usage.Total,
	})
	return nil
}

func matchesPiModel(configured, actual string) bool {
	if actual == "" {
		return false
	}
	if actual == configured {
		return true
	}
	_, model, qualified := strings.Cut(configured, "/")
	return qualified && actual == model
}

func cliChatResponse(model, content string, usage ChatUsage) *ChatResponse {
	return &ChatResponse{Model: model, Usage: usage, Choices: []ChatChoice{{Message: ChatMessage{Role: "assistant", Content: content}, FinishReason: "stop"}}}
}

func unusableCLIResponse(reason string) error {
	return fmt.Errorf("llm client: %w: %s", ErrUnusableResponse, reason)
}
