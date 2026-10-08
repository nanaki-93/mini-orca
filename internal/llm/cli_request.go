package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

type cliInvocation struct {
	directory string
	args      []string
	input     string
}

func (c *Client) chatCLI(ctx context.Context, messages []ChatMessage, format *ResponseFormat) (response *ChatResponse, err error) {
	schema, err := compileCLISchema(format)
	if err != nil {
		return nil, err
	}
	directory, err := os.MkdirTemp("", "mini-orca-provider-")
	if err != nil {
		return nil, fmt.Errorf("llm client: cannot create private CLI working directory")
	}
	defer func() {
		if cleanupErr := os.RemoveAll(directory); cleanupErr != nil && err == nil {
			response, err = nil, fmt.Errorf("llm client: could not remove private CLI request files")
		}
	}()
	invocation, err := prepareCLIInvocation(c.profile, messages, format, directory)
	if err != nil {
		return nil, err
	}
	output, err := runCLI(ctx, c.profile, invocation)
	if err != nil {
		return nil, err
	}
	response, err = decodeCLIResponse(c.profile, output)
	if err != nil {
		return nil, err
	}
	if err := c.validateFinalResponse(*response); err != nil {
		return nil, err
	}
	if schema != nil && !(c.optionalFinalContent && strings.TrimSpace(response.Choices[0].Message.Content) == "") {
		instance, decodeErr := jsonschema.UnmarshalJSON(strings.NewReader(response.Choices[0].Message.Content))
		if decodeErr != nil || schema.Validate(instance) != nil {
			return nil, fmt.Errorf("llm client: %w: CLI output does not match the required JSON schema", ErrUnusableResponse)
		}
	}
	return response, nil
}

func prepareCLIInvocation(profile config.ModelProfile, messages []ChatMessage, format *ResponseFormat, directory string) (cliInvocation, error) {
	system, prompt, err := cliPrompt(messages, format)
	if err != nil {
		return cliInvocation{}, err
	}
	invocation := cliInvocation{directory: directory, input: prompt}
	if profile.Provider == config.PiProvider {
		invocation.args, err = preparePi(directory, profile, system)
	} else {
		invocation.args, err = prepareAgy(directory, profile, system)
		input, _ := json.Marshal(map[string]any{"event": "user", "message": map[string]string{"content": prompt}})
		invocation.input = string(input) + "\n"
	}
	return invocation, err
}

func cliPrompt(messages []ChatMessage, format *ResponseFormat) (string, string, error) {
	system := "You are Mini-Orca's model provider. Answer the final user message in the supplied JSON conversation. Use only the supplied context. Return the requested answer, without file access, commands, or external tools."
	conversation := make([]ChatMessage, 0, len(messages))
	for _, message := range messages {
		switch message.Role {
		case "system":
			system += "\n\n" + message.Content
		case "user", "assistant":
			conversation = append(conversation, ChatMessage{Role: message.Role, Content: message.Content})
		default:
			return "", "", fmt.Errorf("llm client: %w: CLI messages must use system, user, or assistant roles", ErrRequestRejected)
		}
	}
	if len(conversation) == 0 {
		return "", "", fmt.Errorf("llm client: %w: CLI conversation is empty", ErrRequestRejected)
	}
	if format != nil {
		system += "\n\nReturn only JSON conforming to this schema:\n" + string(format.JSONSchema.Schema)
	}
	prompt, err := json.Marshal(conversation)
	if err != nil {
		return "", "", fmt.Errorf("llm client: cannot encode CLI conversation")
	}
	return system, "Conversation:\n" + string(prompt), nil
}

func preparePi(directory string, profile config.ModelProfile, system string) ([]string, error) {
	if err := writeCLIFile(directory, "system.txt", system); err != nil {
		return nil, err
	}
	// This is our own empty workspace, never the imported project. Disable inner
	// retries/compaction so the application owns its attempt and context budgets.
	settings := `{"retry":{"enabled":false,"provider":{"maxRetries":0}},"compaction":{"enabled":false}}`
	if err := writeCLIFile(directory, ".pi/settings.json", settings); err != nil {
		return nil, err
	}
	args := []string{"--print", "--mode", "json", "--no-session", "--no-tools", "--no-extensions", "--no-mcp", "--no-skills", "--no-prompt-templates", "--no-themes", "--no-context-files", "--offline", "--approve", "--system-prompt", filepath.Join(directory, "system.txt"), "--model", profile.Model}
	if profile.ReasoningEffort != "" {
		effort := profile.ReasoningEffort
		if effort == "none" {
			effort = "off"
		}
		args = append(args, "--thinking", effort)
	}
	return args, nil
}

func prepareAgy(directory string, profile config.ModelProfile, system string) ([]string, error) {
	// Native --json-schema requires finish, which user tool hooks can deny.
	// Keep this a text completion, with the prompted schema enforced by chatCLI.
	agent := "---\nname: mini-orca\ndescription: Mini-Orca completion provider\ntools: []\nmainAgent: true\nsubagent: false\ninheritMcp: false\ncommandExecutionPolicy: off\nmcpServers: []\nskills: []\nplugins: []\n---\n# System Prompt\n" + system
	if err := writeCLIFile(directory, ".agents/agents/mini-orca/agent.md", agent); err != nil {
		return nil, err
	}
	args := []string{"--input-format", "stream-json", "--output-format", "stream-json", "--agent", "mini-orca", "--disable-slash-commands", "--model", profile.Model, "--log-file", os.DevNull}
	if profile.ReasoningEffort != "" {
		args = append(args, "--effort", profile.ReasoningEffort)
	}
	return args, nil
}

func writeCLIFile(directory, name, content string) error {
	path := filepath.Join(directory, name)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("llm client: cannot create private CLI request directory")
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		return fmt.Errorf("llm client: cannot write private CLI request file")
	}
	return nil
}

type cliSchemaLoader struct{}

func (cliSchemaLoader) Load(string) (any, error) {
	return nil, fmt.Errorf("external schema references are not allowed")
}

func compileCLISchema(format *ResponseFormat) (*jsonschema.Schema, error) {
	if format == nil {
		return nil, nil
	}
	document, err := jsonschema.UnmarshalJSON(strings.NewReader(string(format.JSONSchema.Schema)))
	if err != nil {
		return nil, fmt.Errorf("llm client: %w: invalid CLI response schema", ErrStructuredRequestRejected)
	}
	compiler := jsonschema.NewCompiler()
	compiler.UseLoader(cliSchemaLoader{})
	const location = "urn:mini-orca:response"
	if err := compiler.AddResource(location, document); err != nil {
		return nil, fmt.Errorf("llm client: %w: invalid CLI response schema", ErrStructuredRequestRejected)
	}
	schema, err := compiler.Compile(location)
	if err != nil {
		return nil, fmt.Errorf("llm client: %w: unsupported CLI response schema", ErrStructuredRequestRejected)
	}
	return schema, nil
}
