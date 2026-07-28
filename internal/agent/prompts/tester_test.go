package prompts

import (
	"strings"
	"testing"
)

func TestBuildTesterPrompt_EmptyCode(t *testing.T) {
	_, err := BuildTesterPrompt("", "test results", nil)
	if err == nil {
		t.Fatal("expected error for empty code, got nil")
	}
}

func TestBuildTesterPrompt_EmptyTestResults(t *testing.T) {
	_, err := BuildTesterPrompt("code", "", nil)
	if err == nil {
		t.Fatal("expected error for empty test results, got nil")
	}
}

func TestBuildTesterPrompt_NoSkills(t *testing.T) {
	code := `package user

type User struct {
	ID    string
	Name  string
	Email string
}

func NewUser(id, name, email string) *User {
	return &User{ID: id, Name: name, Email: email}
}`
	testResults := `=== RUN TestUser_Create
--- PASS: TestUser_Create (0.00s)
=== RUN TestUser_Update
--- FAIL: TestUser_Update (0.01s)
    user_test.go:45: expected status 200, got 500`

	messages, err := BuildTesterPrompt(code, testResults, nil)
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
	if !strings.Contains(systemContent, "expert QA tester") {
		t.Error("system message should contain role description")
	}
	// When no skills provided, skills section should not be present
	if strings.Contains(systemContent, "## Available Skills") {
		t.Error("system message should not contain skills section when no skills provided")
	}
	if !strings.Contains(systemContent, "## Output Format") {
		t.Error("system message should contain output format section")
	}
	if !strings.Contains(systemContent, "## Analysis Framework") {
		t.Error("system message should contain analysis framework section")
	}
	if !strings.Contains(systemContent, "## Constraints") {
		t.Error("system message should contain constraints section")
	}
	if !strings.Contains(systemContent, "SOLID") {
		t.Error("system message should mention SOLID principles")
	}
	if !strings.Contains(systemContent, "table-driven tests") {
		t.Error("system message should mention table-driven tests")
	}
	if !strings.Contains(systemContent, "Do NOT write test code") {
		t.Error("system message should contain constraint about not writing tests")
	}

	// Check user message
	if messages[1].Role != "user" {
		t.Errorf("expected user role, got %s", messages[1].Role)
	}

	userContent := messages[1].Content
	if !strings.Contains(userContent, "## Code to Test") {
		t.Error("user message should contain code section")
	}
	if !strings.Contains(userContent, "package user") {
		t.Error("user message should contain the code")
	}
	if !strings.Contains(userContent, "```go") {
		t.Error("user message should wrap code in code block")
	}
	if !strings.Contains(userContent, "## Test Results") {
		t.Error("user message should contain test results section")
	}
	if !strings.Contains(userContent, "```") {
		t.Error("user message should wrap test results in code block")
	}
}

func TestBuildTesterPrompt_WithSkills(t *testing.T) {
	code := `func Add(a, b int) int { return a + b }`
	testResults := `=== RUN TestAdd
--- PASS: TestAdd (0.00s)`
	skills := []string{"test_automation", "edge_cases", "performance_testing"}

	messages, err := BuildTesterPrompt(code, testResults, skills)
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

func TestBuildTesterPrompt_WithCoverage(t *testing.T) {
	code := `package user

type User struct {
	ID    string
	Name  string
	Email string
}`
	testResults := `=== RUN TestUser_Create
--- PASS: TestUser_Create (0.00s)
coverage: 85%`

	messages, err := BuildTesterPrompt(code, testResults, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	userContent := messages[1].Content
	if !strings.Contains(userContent, "## Code to Test") {
		t.Error("user message should contain code section")
	}
	if !strings.Contains(userContent, "## Test Results") {
		t.Error("user message should contain test results section")
	}
	if !strings.Contains(userContent, "coverage: 85%") {
		t.Error("user message should contain coverage information")
	}
}

func TestBuildTesterPrompt_Full(t *testing.T) {
	code := `package user

type User struct {
	ID    string ` + "`json:\"id\"`" + `
	Name  string ` + "`json:\"name\"`" + `
	Email string ` + "`json:\"email\"`" + `
}

func NewUser(id, name, email string) *User {
	return &User{ID: id, Name: name, Email: email}
}`
	testResults := `=== RUN TestUser_Create
--- PASS: TestUser_Create (0.00s)
=== RUN TestUser_Update
--- FAIL: TestUser_Update (0.01s)
    user_test.go:45: expected status 200, got 500
=== RUN TestUser_Delete
--- PASS: TestUser_Delete (0.00s)
coverage: 78%`
	skills := []string{"test_automation", "edge_cases", "performance_testing"}

	messages, err := BuildTesterPrompt(code, testResults, skills)
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
		"expert QA tester",
		"## Available Skills",
		"test_automation",
		"edge_cases",
		"performance_testing",
		"## Output Format",
		"## Summary",
		"## Failures",
		"## Suggestions",
		"## Coverage",
		"## Analysis Framework",
		"Code Review",
		"Test Analysis",
		"Coverage Assessment",
		"Quality Recommendations",
		"## Constraints",
		"SOLID",
		"table-driven tests",
		"Do NOT write test code",
	}
	for _, check := range systemChecks {
		if !strings.Contains(systemContent, check) {
			t.Errorf("system message should contain %q", check)
		}
	}

	// Verify user message content
	userContent := messages[1].Content
	userChecks := []string{
		"## Code to Test",
		"package user",
		"NewUser",
		"```go",
		"## Test Results",
		"TestUser_Create",
		"TestUser_Update",
		"TestUser_Delete",
		"coverage: 78%",
		"```",
	}
	for _, check := range userChecks {
		if !strings.Contains(userContent, check) {
			t.Errorf("user message should contain %q", check)
		}
	}
}

func TestBuildTesterPrompt_OutputType(t *testing.T) {
	code := `package user

type User struct {
	ID string
}`
	testResults := `--- PASS: TestUser (0.00s)`
	messages, err := BuildTesterPrompt(code, testResults, nil)
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

func TestBuildTesterPrompt_EmptySkills(t *testing.T) {
	code := `package user

func Add(a, b int) int { return a + b }`
	testResults := `--- PASS: TestAdd (0.00s)`

	messages, err := BuildTesterPrompt(code, testResults, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	systemContent := messages[0].Content
	if strings.Contains(systemContent, "## Available Skills") {
		t.Error("system message should not contain skills section when skills are empty")
	}
}

func TestBuildTesterPrompt_CodeOnly(t *testing.T) {
	code := `package user

type User struct {
	ID    string
	Name  string
	Email string
}`
	testResults := `=== RUN TestUser_Create
--- PASS: TestUser_Create (0.00s)`

	messages, err := BuildTesterPrompt(code, testResults, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	userContent := messages[1].Content
	if !strings.Contains(userContent, "## Code to Test") {
		t.Error("user message should contain code section")
	}
	if !strings.Contains(userContent, "## Test Results") {
		t.Error("user message should contain test results section")
	}
}

func TestBuildTesterPrompt_ComplexTestResults(t *testing.T) {
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
	testResults := `=== RUN TestUser_Create
--- PASS: TestUser_Create (0.00s)
=== RUN TestUser_Validate
--- FAIL: TestUser_Validate (0.01s)
    user_test.go:45: expected error, got nil
=== RUN TestUser_Validate_InvalidEmail
--- SKIP: TestUser_Validate_InvalidEmail (0.00s)
=== RUN TestUser_Update
--- FAIL: TestUser_Update (0.02s)
    user_test.go:78: expected status 200, got 500
    user_test.go:79: expected body to contain "success", got "{\"error\":\"internal server error\"}"
coverage: 65%`

	messages, err := BuildTesterPrompt(code, testResults, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	userContent := messages[1].Content
	if !strings.Contains(userContent, "TestUser_Validate") {
		t.Error("user message should contain detailed test results")
	}
	if !strings.Contains(userContent, "expected error, got nil") {
		t.Error("user message should contain specific failure details")
	}
	if !strings.Contains(userContent, "coverage: 65%") {
		t.Error("user message should contain coverage information")
	}
}

func TestBuildTesterPrompt_AnalysisFramework(t *testing.T) {
	code := `func Add(a, b int) int { return a + b }`
	testResults := `--- PASS: TestAdd (0.00s)`

	messages, err := BuildTesterPrompt(code, testResults, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	systemContent := messages[0].Content

	// Verify all four analysis framework steps are present
	frameworkSteps := []string{
		"Code Review",
		"Test Analysis",
		"Coverage Assessment",
		"Quality Recommendations",
	}
	for _, step := range frameworkSteps {
		if !strings.Contains(systemContent, step) {
			t.Errorf("system message should contain framework step %q", step)
		}
	}

	// Verify specific framework details
	details := []string{
		"edge cases",
		"SOLID",
		"test coverage",
		"untested code paths",
		"Prioritize fixes by severity",
	}
	for _, detail := range details {
		if !strings.Contains(systemContent, detail) {
			t.Errorf("system message should contain framework detail %q", detail)
		}
	}
}
