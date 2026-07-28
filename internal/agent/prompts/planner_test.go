package prompts

import (
	"strings"
	"testing"
)

func TestBuildPlannerPrompt_EmptyGoal(t *testing.T) {
	_, err := BuildPlannerPrompt("", nil, "")
	if err == nil {
		t.Fatal("expected error for empty goal, got nil")
	}
}

func TestBuildPlannerPrompt_NoSkills_NoContext(t *testing.T) {
	messages, err := BuildPlannerPrompt("Build a REST API", nil, "")
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
	if !strings.Contains(systemContent, "task planning agent") {
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
	if !strings.Contains(systemContent, "KISS") {
		t.Error("system message should mention KISS principle")
	}
	if !strings.Contains(systemContent, "DRY") {
		t.Error("system message should mention DRY principle")
	}

	// Check user message
	if messages[1].Role != "user" {
		t.Errorf("expected user role, got %s", messages[1].Role)
	}

	userContent := messages[1].Content
	if !strings.Contains(userContent, "Build a REST API") {
		t.Error("user message should contain the goal")
	}
	if !strings.Contains(userContent, "## Goal") {
		t.Error("user message should contain goal section")
	}
}

func TestBuildPlannerPrompt_WithSkills(t *testing.T) {
	skills := []string{"task_breakdown", "solid_principles", "architecture_design"}
	messages, err := BuildPlannerPrompt("Build a microservice", skills, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}

	systemContent := messages[0].Content
	if !strings.Contains(systemContent, "## Available Skills") {
		t.Error("system message should contain skills section")
	}
	for _, skill := range skills {
		if !strings.Contains(systemContent, skill) {
			t.Errorf("system message should mention skill %q", skill)
		}
	}
}

func TestBuildPlannerPrompt_WithContext(t *testing.T) {
	context := "This is a Go project using PostgreSQL and Redis for caching."
	messages, err := BuildPlannerPrompt("Build an e-commerce platform", nil, context)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	userContent := messages[1].Content
	if !strings.Contains(userContent, "## Project Context") {
		t.Error("user message should contain context section")
	}
	if !strings.Contains(userContent, context) {
		t.Error("user message should contain the provided context")
	}
}

func TestBuildPlannerPrompt_Full(t *testing.T) {
	skills := []string{"task_breakdown", "solid_principles"}
	context := "Go project with PostgreSQL."
	messages, err := BuildPlannerPrompt("Build a user management API", skills, context)
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
	checks := []string{
		"task planning agent",
		"## Available Skills",
		"task_breakdown",
		"solid_principles",
		"## Output Format",
		"## Constraints",
		"SOLID",
		"KISS",
		"DRY",
	}
	for _, check := range checks {
		if !strings.Contains(systemContent, check) {
			t.Errorf("system message should contain %q", check)
		}
	}

	// Verify user message content
	userContent := messages[1].Content
	checks = []string{
		"Build a user management API",
		"## Goal",
		"## Project Context",
		"Go project with PostgreSQL",
	}
	for _, check := range checks {
		if !strings.Contains(userContent, check) {
			t.Errorf("user message should contain %q", check)
		}
	}
}

func TestBuildPlannerPrompt_OutputType(t *testing.T) {
	messages, err := BuildPlannerPrompt("Test goal", nil, "")
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

func TestBuildPlannerPrompt_EmptyContext(t *testing.T) {
	messages, err := BuildPlannerPrompt("Build something", nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	userContent := messages[1].Content
	// When context is empty, there should be no "## Project Context" section
	if strings.Contains(userContent, "## Project Context") {
		t.Error("user message should not contain context section when context is empty")
	}
}

func TestBuildPlannerPrompt_EmptySkills(t *testing.T) {
	messages, err := BuildPlannerPrompt("Build something", []string{}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	systemContent := messages[0].Content
	// When skills are empty, there should be no "## Available Skills" section
	if strings.Contains(systemContent, "## Available Skills") {
		t.Error("system message should not contain skills section when skills are empty")
	}
}

func TestSystemPromptBuilder(t *testing.T) {
	var sb systemPromptBuilder
	sb.AppendLine("Line 1")
	sb.AppendLine("")
	sb.AppendLine("Line 3")

	result := sb.String()
	if !strings.Contains(result, "Line 1") {
		t.Error("result should contain Line 1")
	}
	if !strings.Contains(result, "Line 3") {
		t.Error("result should contain Line 3")
	}
}
