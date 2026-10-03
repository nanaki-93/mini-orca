package llm

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/config"
)

func TestCLIAgyRequiresValidNativeStructuredOutput(t *testing.T) {
	for _, mode := range []string{"missing-structured", "invalid-structured", "null-structured"} {
		t.Run(mode, func(t *testing.T) {
			profile, _ := cliFixture(t, config.AgyProvider, mode)
			schema := JSONSchema{Name: "answer", Schema: json.RawMessage(`{"type":"object","properties":{"answer":{"type":"string"}},"required":["answer"],"additionalProperties":false}`)}
			_, err := NewClient(profile).ChatWithJSONSchema(context.Background(), []ChatMessage{{Role: "user", Content: "Explain the supplied source."}}, schema)
			if !errors.Is(err, ErrUnusableResponse) {
				t.Fatalf("invalid native output was accepted or fell back to valid prose JSON: %v", err)
			}
		})
	}
}

func TestCLIAgyRejectsIncorrectAgentsAndToolUse(t *testing.T) {
	profile := config.ModelProfile{Provider: config.AgyProvider, Model: "fixture-model"}
	valid := cliFixtureOutput(config.AgyProvider, "success")
	for name, output := range map[string]string{
		"missing agent": strings.Replace(valid, `"agent":"mini-orca"`, `"agent":""`, 1),
		"wrong agent":   strings.Replace(valid, `"agent":"mini-orca"`, `"agent":"default"`, 1),
		"tool":          cliFixtureOutput(config.AgyProvider, "tool"),
		"subagent":      strings.Replace(cliFixtureOutput(config.AgyProvider, "tool"), `"step_type":"tool"`, `"step_type":"subagent"`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeCLIResponse(profile, []byte(output), true); !errors.Is(err, ErrUnusableResponse) {
				t.Fatalf("unexpected agent or tool use = %v", err)
			}
		})
	}
}

func TestCLIAgyPlainResponsesKeepToolsDisabled(t *testing.T) {
	profile, capture := cliFixture(t, config.AgyProvider, "missing-structured")
	response, err := NewClient(profile).Chat(context.Background(), []ChatMessage{{Role: "user", Content: "Explain the supplied source."}})
	if err != nil || response.Choices[0].Message.Content != cliFixtureContent {
		t.Fatalf("plain response = %+v, %v", response, err)
	}
	var request cliCapture
	readTestJSON(t, capture, &request)
	if !strings.Contains(request.System, "tools: []") || strings.Contains(strings.Join(request.Args, " "), "--json-schema") {
		t.Fatal("plain responses must not enable structured completion")
	}
}
