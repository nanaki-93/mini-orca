package prompts

import (
	"strings"
	"testing"
)

func TestBuildCoderPrompt_EmptyTitle(t *testing.T) {
	_, err := BuildCoderPrompt(PlanUnit{Title: "", Description: "Test"}, "", nil)
	if err == nil {
		t.Fatal("expected error for empty title, got nil")
	}
}

func TestBuildCoderPrompt_EmptyDescription(t *testing.T) {
	_, err := BuildCoderPrompt(PlanUnit{Title: "Test"}, "", nil)
	if err == nil {
		t.Fatal("expected error for empty description, got nil")
	}
}

func TestBuildCoderPrompt_NoSkills_NoExistingCode(t *testing.T) {
	unit := PlanUnit{
		Title:        "User Struct",
		Description:  "Implement a User struct with ID, Name, Email fields",
		Dependencies: nil,
	}

	messages, err := BuildCoderPrompt(unit, "", nil)
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
	if !strings.Contains(userContent, "User Struct") {
		t.Error("user message should contain the unit title")
	}
	if !strings.Contains(userContent, "Implement a User struct") {
		t.Error("user message should contain the unit description")
	}
	if !strings.Contains(userContent, "## Atomic Unit") {
		t.Error("user message should contain atomic unit section")
	}
	// When no dependencies, should not have "## Dependencies" section
	if strings.Contains(userContent, "## Dependencies") {
		t.Error("user message should not contain dependencies section when no dependencies")
	}
	// When no existing code, should not have "## Existing Code" section
	if strings.Contains(userContent, "## Existing Code") {
		t.Error("user message should not contain existing code section when empty")
	}
}

func TestBuildCoderPrompt_WithSkills(t *testing.T) {
	unit := PlanUnit{
		Title:       "HTTP Handler",
		Description: "Implement an HTTP handler for user registration",
	}
	skills := []string{"http_handlers", "error_handling", "clean_code"}

	messages, err := BuildCoderPrompt(unit, "", skills)
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

func TestBuildCoderPrompt_WithExistingCode(t *testing.T) {
	unit := PlanUnit{
		Title:       "User Repository",
		Description: "Implement a PostgreSQL repository for User",
	}
	existingCode := `package user

type User struct {
	ID    string ` + "`json:\"id\"`" + `
	Name  string ` + "`json:\"name\"`" + `
	Email string ` + "`json:\"email\"`" + `
}`

	messages, err := BuildCoderPrompt(unit, existingCode, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	userContent := messages[1].Content
	if !strings.Contains(userContent, "## Existing Code") {
		t.Error("user message should contain existing code section")
	}
	if !strings.Contains(userContent, "package user") {
		t.Error("user message should contain the existing code")
	}
	if !strings.Contains(userContent, "```go") {
		t.Error("user message should wrap existing code in code block")
	}
}

func TestBuildCoderPrompt_WithDependencies(t *testing.T) {
	unit := PlanUnit{
		Title:        "User Service",
		Description:  "Implement service layer for User operations",
		Dependencies: []string{"User Struct", "User Repository"},
	}

	messages, err := BuildCoderPrompt(unit, "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	userContent := messages[1].Content
	if !strings.Contains(userContent, "## Dependencies") {
		t.Error("user message should contain dependencies section")
	}
	for _, dep := range unit.Dependencies {
		if !strings.Contains(userContent, dep) {
			t.Errorf("user message should mention dependency %q", dep)
		}
	}
}

func TestBuildCoderPrompt_Full(t *testing.T) {
	unit := PlanUnit{
		Title:        "User Service",
		Description:  "Implement service layer for User CRUD operations with validation",
		Dependencies: []string{"User Struct", "User Repository"},
	}
	existingCode := `package user

type User struct {
	ID    string ` + "`json:\"id\"`" + `
	Name  string ` + "`json:\"name\"`" + `
	Email string ` + "`json:\"email\"`" + `
}`
	skills := []string{"service_layer", "validation", "clean_code"}

	messages, err := BuildCoderPrompt(unit, existingCode, skills)
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
		"## Available Skills",
		"service_layer",
		"validation",
		"clean_code",
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
		"User Service",
		"Implement service layer",
		"## Atomic Unit",
		"## Dependencies",
		"User Struct",
		"User Repository",
		"## Existing Code",
		"package user",
		"```go",
	}
	for _, check := range userChecks {
		if !strings.Contains(userContent, check) {
			t.Errorf("user message should contain %q", check)
		}
	}
}

func TestBuildCoderPrompt_OutputType(t *testing.T) {
	unit := PlanUnit{
		Title:       "Test Unit",
		Description: "Test description",
	}
	messages, err := BuildCoderPrompt(unit, "", nil)
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

func TestBuildCoderPrompt_EmptyDependencies(t *testing.T) {
	unit := PlanUnit{
		Title:        "Test Unit",
		Description:  "Test description",
		Dependencies: []string{},
	}
	messages, err := BuildCoderPrompt(unit, "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	userContent := messages[1].Content
	if strings.Contains(userContent, "## Dependencies") {
		t.Error("user message should not contain dependencies section when dependencies are empty")
	}
}

func TestBuildCoderPrompt_EmptyExistingCode(t *testing.T) {
	unit := PlanUnit{
		Title:       "Test Unit",
		Description: "Test description",
	}
	messages, err := BuildCoderPrompt(unit, "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	userContent := messages[1].Content
	if strings.Contains(userContent, "## Existing Code") {
		t.Error("user message should not contain existing code section when existing code is empty")
	}
}

func TestBuildCoderPrompt_EmptySkills(t *testing.T) {
	unit := PlanUnit{
		Title:       "Test Unit",
		Description: "Test description",
	}
	messages, err := BuildCoderPrompt(unit, "", []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	systemContent := messages[0].Content
	if strings.Contains(systemContent, "## Available Skills") {
		t.Error("system message should not contain skills section when skills are empty")
	}
}
