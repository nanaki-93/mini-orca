package prompts

import (
	"strings"
	"testing"
)

func TestBuildCoderPromptFromRequest_EmptyPrompt(t *testing.T) {
	req := FeatureRequest{
		Prompt: "",
	}
	_, err := BuildCoderPromptFromRequest(req)
	if err == nil {
		t.Fatal("expected error for empty prompt, got nil")
	}
}

func TestBuildCoderPromptFromRequest_Valid(t *testing.T) {
	req := FeatureRequest{
		Prompt: "Create a User struct with ID, Name, Email fields",
	}

	messages, err := BuildCoderPromptFromRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}

	// Check system message
	if messages[0].Role != "system" {
		t.Errorf("expected system role, got %s", messages[0].Role)
	}

	systemContent := messages[0].Content
	if !strings.Contains(systemContent, "expert Go coder") {
		t.Error("system message should contain role description")
	}
	if !strings.Contains(systemContent, "Output Format") {
		t.Error("system message should contain output format section")
	}
	if !strings.Contains(systemContent, "Constraints") {
		t.Error("system message should contain constraints section")
	}
	if !strings.Contains(systemContent, "SOLID") {
		t.Error("system message should mention SOLID principles")
	}
	if !strings.Contains(systemContent, "DRY") {
		t.Error("system message should mention DRY principle")
	}
	if !strings.Contains(systemContent, "KISS") {
		t.Error("system message should mention KISS principle")
	}
	if !strings.Contains(systemContent, "context.Context") {
		t.Error("system message should mention context.Context")
	}
	if !strings.Contains(systemContent, "gofmt") {
		t.Error("system message should mention gofmt standards")
	}

	// Check user message
	if messages[1].Role != "user" {
		t.Errorf("expected user role, got %s", messages[1].Role)
	}

	userContent := messages[1].Content
	if !strings.Contains(userContent, "Create a User struct") {
		t.Error("user message should contain the feature request")
	}
	if !strings.Contains(userContent, "## Feature Request") {
		t.Error("user message should contain feature request section")
	}
	if !strings.Contains(userContent, "## Instructions") {
		t.Error("user message should contain instructions section")
	}
}

func TestBuildCoderPromptFromRequest_WithProjectContext(t *testing.T) {
	req := FeatureRequest{
		Prompt:         "Create a User struct",
		ProjectContext: "module github.com/example/project\ngo 1.21",
	}

	messages, err := BuildCoderPromptFromRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}

	userContent := messages[1].Content
	if !strings.Contains(userContent, "## Project Context") {
		t.Error("user message should contain project context section")
	}
	if !strings.Contains(userContent, "module github.com/example/project") {
		t.Error("user message should contain project context")
	}
}

func TestBuildCoderPromptFromRequest_WithTargetFile(t *testing.T) {
	req := FeatureRequest{
		Prompt:     "Create a User struct",
		TargetFile: "models/user.go",
	}

	messages, err := BuildCoderPromptFromRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}

	userContent := messages[1].Content
	if !strings.Contains(userContent, "## Target File") {
		t.Error("user message should contain target file section")
	}
	if !strings.Contains(userContent, "models/user.go") {
		t.Error("user message should contain target file path")
	}
}

func TestBuildCoderPromptFromRequest_Full(t *testing.T) {
	req := FeatureRequest{
		Prompt:         "Create a User struct with ID, Name, Email fields and JSON tags",
		ProjectContext: "module github.com/example/project\ngo 1.21",
		TargetFile:     "models/user.go",
		Language:       "go",
	}

	messages, err := BuildCoderPromptFromRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify message types
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}
	if messages[0].Role != "system" {
		t.Errorf("first message should be system, got %s", messages[0].Role)
	}
	if messages[1].Role != "user" {
		t.Errorf("second message should be user, got %s", messages[1].Role)
	}

	// Verify system message content
	systemContent := messages[0].Content
	systemChecks := []string{
		"expert Go coder",
		"## Output Format",
		"## Constraints",
		"SOLID",
		"DRY",
		"KISS",
		"context.Context",
		"gofmt",
	}
	for _, check := range systemChecks {
		if !strings.Contains(systemContent, check) {
			t.Errorf("system message should contain %q", check)
		}
	}

	// Verify user message content
	userContent := messages[1].Content
	userChecks := []string{
		"## Feature Request",
		"Create a User struct",
		"## Project Context",
		"module github.com/example/project",
		"## Target File",
		"models/user.go",
		"## Instructions",
	}
	for _, check := range userChecks {
		if !strings.Contains(userContent, check) {
			t.Errorf("user message should contain %q", check)
		}
	}
}

func TestBuildCoderPromptFromRequest_OutputType(t *testing.T) {
	req := FeatureRequest{
		Prompt: "Test description",
	}
	messages, err := BuildCoderPromptFromRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify return type is []model.ChatMessage
	for i, msg := range messages {
		if msg.Role == "" {
			t.Errorf("message %d has empty role", i)
		}
		if msg.Content == "" {
			t.Errorf("message %d has empty content", i)
		}
	}
}

func TestBuildCoderPromptFromRequest_OnlyPrompt(t *testing.T) {
	req := FeatureRequest{
		Prompt: "Implement a constructor function",
	}

	messages, err := BuildCoderPromptFromRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	userContent := messages[1].Content
	if !strings.Contains(userContent, "Implement a constructor function") {
		t.Error("user message should contain the feature request")
	}
	// Should not have project context or target file sections
	if strings.Contains(userContent, "## Project Context") {
		t.Error("user message should not contain project context when not provided")
	}
	if strings.Contains(userContent, "## Target File") {
		t.Error("user message should not contain target file when not provided")
	}
}

func TestBuildCoderPromptFromRequest_EmptyTargetFile(t *testing.T) {
	req := FeatureRequest{
		Prompt:     "Create a User struct",
		TargetFile: "",
	}

	messages, err := BuildCoderPromptFromRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	userContent := messages[1].Content
	if strings.Contains(userContent, "## Target File") {
		t.Error("user message should not contain target file section when empty")
	}
}

func TestBuildCoderPromptFromRequest_EmptyProjectContext(t *testing.T) {
	req := FeatureRequest{
		Prompt:         "Create a User struct",
		ProjectContext: "",
	}

	messages, err := BuildCoderPromptFromRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	userContent := messages[1].Content
	if strings.Contains(userContent, "## Project Context") {
		t.Error("user message should not contain project context section when empty")
	}
}
