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
	profile  config.ModelProfile
	response *ChatResponse
	started  bool
	finished bool
}

func decodeCLIResponse(profile config.ModelProfile, output []byte) (*ChatResponse, error) {
	decoder := cliResponseDecoder{profile: profile}
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
		Step struct {
			Type string `json:"step_type"`
		} `json:"step_update"`
		Result struct {
			Status   string `json:"status"`
			Response string `json:"response"`
			Usage    struct {
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
		if d.started || event.Init.Tools == nil || len(*event.Init.Tools) != 0 || event.Init.Agent != "mini-orca" || event.Init.Model != d.profile.Model {
			return unusableCLIResponse("agy did not select the configured model and tool-free agent")
		}
		d.started = true
	case "step_update":
		if event.Step.Type == "tool" || event.Step.Type == "subagent" {
			return unusableCLIResponse("agy attempted tool use")
		}
	case "result":
		if !d.started || d.finished || event.Result.Status != "SUCCESS" {
			return unusableCLIResponse("agy did not complete successfully")
		}
		d.finished = true
		d.response = cliChatResponse(d.profile.Model, event.Result.Response, ChatUsage{
			PromptTokens: event.Result.Usage.Input, CompletionTokens: event.Result.Usage.Output, TotalTokens: event.Result.Usage.Total,
		})
	default:
		return unusableCLIResponse("unrecognized agy event")
	}
	return nil
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
