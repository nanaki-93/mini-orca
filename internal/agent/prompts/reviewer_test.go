package prompts

import (
	"strings"
	"testing"
)

func TestBuildReviewerPromptFromRequest_EmptyCode(t *testing.T) {
	_, err := BuildReviewerPromptFromRequest("", "Create User struct", nil)
	if err == nil {
		t.Fatal("expected error for empty code, got nil")
	}
}

func TestBuildReviewerPromptFromRequest_EmptyUserRequest(t *testing.T) {
	_, err := BuildReviewerPromptFromRequest("package user", "", nil)
	if err == nil {
		t.Fatal("expected error for empty user request, got nil")
	}
}

func TestBuildReviewerPromptFromRequest_NoSkills(t *testing.T) {
	code := `package user

type User struct {
	ID    string
	Name  string
	Email string
}

func NewUser(id, name, email string) *User {
	return &User{ID: id, Name: name, Email: email}
}`
	userRequest := "Create a User struct with ID, Name, Email fields and a NewUser constructor"

	messages, err := BuildReviewerPromptFromRequest(code, userRequest, nil)
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
	if !strings.Contains(systemContent, "expert code reviewer") {
		t.Error("system message should contain role description")
	}
	// When no skills provided, skills section should not be present
	if strings.Contains(systemContent, "## Available Skills") {
		t.Error("system message should not contain skills section when no skills provided")
	}
	if !strings.Contains(systemContent, "## Output Format") {
		t.Error("system message should contain output format section")
	}
	if !strings.Contains(systemContent, "## Review Checklist") {
		t.Error("system message should contain review checklist section")
	}
	if !strings.Contains(systemContent, "## Constraints") {
		t.Error("system message should contain constraints section")
	}
	if !strings.Contains(systemContent, "SOLID") {
		t.Error("system message should mention SOLID principles")
	}
	if !strings.Contains(systemContent, "Do NOT rewrite the code") {
		t.Error("system message should contain constraint about not rewriting code")
	}
	if !strings.Contains(systemContent, "## Issues") {
		t.Error("system message should mention issues section")
	}
	if !strings.Contains(systemContent, "## Score") {
		t.Error("system message should mention score section")
	}
	if !strings.Contains(systemContent, "## Recommendation") {
		t.Error("system message should mention recommendation section")
	}

	// Check user message
	if messages[1].Role != "user" {
		t.Errorf("expected user role, got %s", messages[1].Role)
	}

	userContent := messages[1].Content
	if !strings.Contains(userContent, "## User Request") {
		t.Error("user message should contain user request section")
	}
	if !strings.Contains(userContent, "Create a User struct") {
		t.Error("user message should contain the user request")
	}
	if !strings.Contains(userContent, "## Generated Code") {
		t.Error("user message should contain generated code section")
	}
	if !strings.Contains(userContent, "package user") {
		t.Error("user message should contain the code")
	}
	if !strings.Contains(userContent, "```go") {
		t.Error("user message should wrap code in code block")
	}
	if !strings.Contains(userContent, "## Review Criteria") {
		t.Error("user message should contain review criteria section")
	}
}

func TestBuildReviewerPromptFromRequest_WithSkills(t *testing.T) {
	code := `func Add(a, b int) int { return a + b }`
	userRequest := "Implement Add function that takes two integers and returns their sum."
	skills := []string{"security_audit", "performance_review", "clean_code"}

	messages, err := BuildReviewerPromptFromRequest(code, userRequest, skills)
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

func TestBuildReviewerPromptFromRequest_Full(t *testing.T) {
	code := `package user

type User struct {
	ID    string ` + "`json:\"id\"`" + `
	Name  string ` + "`json:\"name\"`" + `
	Email string ` + "`json:\"email\"`" + `
}

func NewUser(id, name, email string) *User {
	return &User{ID: id, Name: name, Email: email}
}`
	userRequest := "Create a User struct with ID, Name, Email fields and JSON tags, implement NewUser constructor, add email validation, and add error handling for invalid inputs"
	skills := []string{"security_audit", "performance_review", "clean_code"}

	messages, err := BuildReviewerPromptFromRequest(code, userRequest, skills)
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
		"expert code reviewer",
		"## Available Skills",
		"security_audit",
		"performance_review",
		"clean_code",
		"## Output Format",
		"## Summary",
		"## Issues",
		"## Suggestions",
		"## Score",
		"## Recommendation",
		"## Review Checklist",
		"Plan Alignment",
		"Code Quality",
		"Security",
		"Performance",
		"Testing Readiness",
		"Documentation",
		"## Constraints",
		"SOLID",
		"DRY",
		"Do NOT rewrite the code",
		"Approve, Request Changes, or Reject",
	}
	for _, check := range systemChecks {
		if !strings.Contains(systemContent, check) {
			t.Errorf("system message should contain %q", check)
		}
	}

	// Verify user message content
	userContent := messages[1].Content
	userChecks := []string{
		"## User Request",
		"Create a User struct",
		"## Generated Code",
		"package user",
		"NewUser",
		"```go",
		"## Review Criteria",
	}
	for _, check := range userChecks {
		if !strings.Contains(userContent, check) {
			t.Errorf("user message should contain %q", check)
		}
	}
}

func TestBuildReviewerPromptFromRequest_OutputType(t *testing.T) {
	code := `package user

type User struct {
	ID string
}`
	userRequest := "Create User struct with ID field"
	messages, err := BuildReviewerPromptFromRequest(code, userRequest, nil)
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

func TestBuildReviewerPromptFromRequest_EmptySkills(t *testing.T) {
	code := `package user

func Add(a, b int) int { return a + b }`
	userRequest := "Implement Add function"

	messages, err := BuildReviewerPromptFromRequest(code, userRequest, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	systemContent := messages[0].Content
	if strings.Contains(systemContent, "## Available Skills") {
		t.Error("system message should not contain skills section when skills are empty")
	}
}

func TestBuildReviewerPromptFromRequest_ComplexRequest(t *testing.T) {
	code := `package user

type User struct {
	ID    string ` + "`json:\"id\"`" + `
	Name  string ` + "`json:\"name\"`" + `
	Email string ` + "`json:\"email\"`" + `
}

func (u *User) Validate() error {
	if u.ID == "" {
		return errors.New("id is required")
	}
	return nil
}`
	userRequest := "1. Create User struct with JSON tags\n2. Implement NewUser constructor with validation\n3. Add Validate method for business rules\n4. Add error handling for all public methods\n5. Add unit tests for all functions\n6. Add integration tests for database operations"

	messages, err := BuildReviewerPromptFromRequest(code, userRequest, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	userContent := messages[1].Content
	if !strings.Contains(userContent, "Validate method") {
		t.Error("user message should contain complex request details")
	}
	if !strings.Contains(userContent, "unit tests") {
		t.Error("user message should contain testing requirements")
	}
}

func TestBuildReviewerPromptFromRequest_ReviewChecklist(t *testing.T) {
	code := `func Add(a, b int) int { return a + b }`
	userRequest := "Implement Add function"

	messages, err := BuildReviewerPromptFromRequest(code, userRequest, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	systemContent := messages[0].Content

	// Verify all checklist items are present
	checklistItems := []string{
		"Plan Alignment",
		"Code Quality",
		"Security",
		"Performance",
		"Testing Readiness",
		"Documentation",
	}
	for _, item := range checklistItems {
		if !strings.Contains(systemContent, item) {
			t.Errorf("system message should contain checklist item %q", item)
		}
	}

	// Verify specific checklist details
	details := []string{
		"SOLID",
		"DRY",
		"KISS",
		"error handling",
		"hardcoded secrets",
		"Input validation",
		"Efficient algorithms",
		"concurrency",
		"testable",
		"godoc comments",
	}
	for _, detail := range details {
		if !strings.Contains(systemContent, detail) {
			t.Errorf("system message should contain checklist detail %q", detail)
		}
	}
}

func TestBuildReviewerPromptFromRequest_OutputFormat(t *testing.T) {
	code := `func Add(a, b int) int { return a + b }`
	userRequest := "Implement Add function"

	messages, err := BuildReviewerPromptFromRequest(code, userRequest, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	systemContent := messages[0].Content

	// Verify all output format sections are present
	outputSections := []string{
		"## Summary",
		"## Issues",
		"## Suggestions",
		"## Score",
		"## Recommendation",
	}
	for _, section := range outputSections {
		if !strings.Contains(systemContent, section) {
			t.Errorf("system message should contain output section %q", section)
		}
	}

	// Verify score range is specified
	if !strings.Contains(systemContent, "0-100") {
		t.Error("system message should specify score range 0-100")
	}

	// Verify recommendation options
	if !strings.Contains(systemContent, "Approve") {
		t.Error("system message should mention Approve recommendation")
	}
	if !strings.Contains(systemContent, "Request Changes") {
		t.Error("system message should mention Request Changes recommendation")
	}
	if !strings.Contains(systemContent, "Reject") {
		t.Error("system message should mention Reject recommendation")
	}
}

func TestBuildReviewerPromptFromRequest_CodeAndRequestSeparation(t *testing.T) {
	code := `package user

type User struct {
	ID string
}`
	userRequest := "Create User struct with ID field and validation"

	messages, err := BuildReviewerPromptFromRequest(code, userRequest, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	userContent := messages[1].Content

	// Verify code is in its own section
	codeSectionIdx := strings.Index(userContent, "## Generated Code")
	requestSectionIdx := strings.Index(userContent, "## User Request")

	if codeSectionIdx == -1 {
		t.Error("user message should contain code section")
	}
	if requestSectionIdx == -1 {
		t.Error("user message should contain user request section")
	}
	if requestSectionIdx >= codeSectionIdx {
		t.Error("user request section should come before code section")
	}
}
