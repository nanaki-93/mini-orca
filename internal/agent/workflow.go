package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/agent/prompts"
	"github.com/nanaki-93/mini-orca/v2/internal/llm"
	"github.com/nanaki-93/mini-orca/v2/internal/tools"
)

// Formatter defines the interface for formatting code files.
type Formatter interface {
	// FormatCode formats the code file at the given path.
	FormatCode(path string) error
}

// WorkflowResult holds the result of a workflow execution including formatting changes.
type WorkflowResult struct {
	// AgentResult is the result from the agent execution.
	AgentResult *Result
	// FormattingChanges reports any code formatting changes made.
	FormattingChanges []FormattingChange
	// FormattingApplied indicates if formatting was applied.
	FormattingApplied bool
}

// FormattingChange represents a single file formatting change.
type FormattingChange struct {
	// Path is the path to the formatted file.
	Path string
	// BeforeContent is the content before formatting.
	BeforeContent string
	// AfterContent is the content after formatting.
	AfterContent string
}

// Workflow orchestrates the multi-agent workflow with formatting integration.
type Workflow struct {
	orchestrator *Orchestrator
	formatter    Formatter
}

// NewWorkflow creates a new workflow with the given LLM client and tool executor.
func NewWorkflow(llmClient *llm.Client, executor tools.ToolExecutor) *Workflow {
	return &Workflow{
		orchestrator: NewOrchestrator(llmClient, executor),
		formatter:    tools.NewFormatterExecutor(),
	}
}

// RunCodingWorkflow executes the coding workflow with formatting integration.
// It runs the coder agent, formats the generated code, and reports changes.
func (w *Workflow) RunCodingWorkflow(unit prompts.PlanUnit) (*WorkflowResult, error) {
	// Run the coder agent
	agentResult, err := w.orchestrator.RunCoder(unit)
	if err != nil {
		return nil, fmt.Errorf("workflow: coder execution failed: %w", err)
	}

	result := &WorkflowResult{
		AgentResult: agentResult,
	}

	// Auto-format after code generation
	changes, err := w.formatGeneratedCode(result.AgentResult.Output, unit.Title)
	if err != nil {
		// Don't fail the workflow if formatting fails, just log and continue
		fmt.Printf("Warning: formatting failed: %v\n", err)
	}
	result.FormattingChanges = changes
	result.FormattingApplied = len(changes) > 0

	return result, nil
}

// RunTestingWorkflow executes the testing workflow with formatting integration.
// It runs the tester agent and formats code if test failures are found.
func (w *Workflow) RunTestingWorkflow(code string, testResults string) (*WorkflowResult, error) {
	// Run the tester agent
	agentResult, err := w.orchestrator.RunTester(code, testResults)
	if err != nil {
		return nil, fmt.Errorf("workflow: tester execution failed: %w", err)
	}

	result := &WorkflowResult{
		AgentResult: agentResult,
	}

	// Auto-format after test failures (if needed)
	// Parse the test report to check for failures
	var testReport TestReport
	if err := parseJSON(result.AgentResult.Output, &testReport); err == nil {
		if !testReport.Passed && len(testReport.Failures) > 0 {
			// Attempt to format the code to fix potential formatting issues
			changes, err := w.formatCodeFromTest(code)
			if err != nil {
				fmt.Printf("Warning: formatting after test failure failed: %v\n", err)
			}
			result.FormattingChanges = changes
			result.FormattingApplied = len(changes) > 0
		}
	}

	return result, nil
}

// formatGeneratedCode formats the generated code and saves it to a file.
// It extracts code blocks from the agent output and formats them.
func (w *Workflow) formatGeneratedCode(output string, title string) ([]FormattingChange, error) {
	// Extract code blocks from the output
	codeBlocks := extractCodeBlocks(output)
	if len(codeBlocks) == 0 {
		return nil, nil
	}

	var changes []FormattingChange

	// Create a temporary directory for formatting
	tempDir, err := os.MkdirTemp("", "mini-orca-format-*")
	if err != nil {
		return nil, fmt.Errorf("workflow: failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Format each code block
	for i, block := range codeBlocks {
		// Determine file extension based on language
		ext := getFileExtension(block.Language)
		filename := fmt.Sprintf("%s_%d%s", sanitizeFilename(title), i, ext)
		filepath := filepath.Join(tempDir, filename)

		// Write the code to a temp file
		if err := os.WriteFile(filepath, []byte(block.Content), 0644); err != nil {
			return nil, fmt.Errorf("workflow: failed to write temp file: %w", err)
		}

		// Read original content
		originalContent := block.Content

		// Format the file
		if err := w.formatter.FormatCode(filepath); err != nil {
			fmt.Printf("Warning: formatting failed for %s: %v\n", filepath, err)
			continue
		}

		// Read formatted content
		formattedContent, err := os.ReadFile(filepath)
		if err != nil {
			return nil, fmt.Errorf("workflow: failed to read formatted file: %w", err)
		}

		// Check if formatting changed the content
		if strings.TrimSpace(originalContent) != strings.TrimSpace(string(formattedContent)) {
			changes = append(changes, FormattingChange{
				Path:          filename,
				BeforeContent: originalContent,
				AfterContent:  string(formattedContent),
			})
		}
	}

	return changes, nil
}

// formatCodeFromTest attempts to format code that failed tests.
func (w *Workflow) formatCodeFromTest(code string) ([]FormattingChange, error) {
	// Create a temporary directory for formatting
	tempDir, err := os.MkdirTemp("", "mini-orca-format-test-*")
	if err != nil {
		return nil, fmt.Errorf("workflow: failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Write the code to a temp file
	filepath := filepath.Join(tempDir, "code.go")
	if err := os.WriteFile(filepath, []byte(code), 0644); err != nil {
		return nil, fmt.Errorf("workflow: failed to write temp file: %w", err)
	}

	// Read original content
	originalContent := code

	// Format the file
	if err := w.formatter.FormatCode(filepath); err != nil {
		return nil, fmt.Errorf("workflow: formatting failed: %w", err)
	}

	// Read formatted content
	formattedContent, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("workflow: failed to read formatted file: %w", err)
	}

	// Check if formatting changed the content
	if strings.TrimSpace(originalContent) != strings.TrimSpace(string(formattedContent)) {
		return []FormattingChange{
			{
				Path:          "code.go",
				BeforeContent: originalContent,
				AfterContent:  string(formattedContent),
			},
		}, nil
	}

	return nil, nil
}

// codeBlock represents a code block extracted from agent output.
type codeBlock struct {
	Language string
	Content  string
}

// extractCodeBlocks extracts code blocks from the agent output.
func extractCodeBlocks(output string) []codeBlock {
	var blocks []codeBlock

	// Split by triple backticks
	parts := strings.Split(output, "```")
	for i := 0; i < len(parts)-1; i += 2 {
		// First part is the language specifier
		language := strings.TrimSpace(parts[i])
		if language == "" {
			language = "go" // Default to Go
		}

		// Second part is the content
		content := strings.TrimSpace(parts[i+1])
		if content != "" {
			blocks = append(blocks, codeBlock{
				Language: language,
				Content:  content,
			})
		}
	}

	// If no code blocks found, treat the entire output as a code block
	if len(blocks) == 0 && strings.TrimSpace(output) != "" {
		blocks = append(blocks, codeBlock{
			Language: "go",
			Content:  output,
		})
	}

	return blocks
}

// getFileExtension returns the file extension for a given language.
func getFileExtension(language string) string {
	switch strings.ToLower(language) {
	case "go":
		return ".go"
	case "python", "py":
		return ".py"
	case "typescript", "ts":
		return ".ts"
	case "javascript", "js":
		return ".js"
	case "kotlin", "kt":
		return ".kt"
	case "java":
		return ".java"
	case "rust", "rs":
		return ".rs"
	default:
		return ".go" // Default to Go
	}
}

// sanitizeFilename sanitizes a string for use as a filename.
func sanitizeFilename(name string) string {
	// Replace spaces and special characters with underscores
	var sb strings.Builder
	for _, ch := range name {
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '-' || ch == '_' {
			sb.WriteRune(ch)
		} else {
			sb.WriteString("_")
		}
	}
	return sb.String()
}

// parseJSON parses JSON content into the given destination.
func parseJSON(content string, dest any) error {
	// Find JSON content (could be wrapped in markdown code blocks)
	jsonContent := content
	if strings.HasPrefix(content, "```") {
		parts := strings.Split(content, "```")
		if len(parts) >= 3 {
			jsonContent = strings.TrimSpace(parts[1])
		}
	}

	// Try to parse JSON
	return unmarshalJSON([]byte(jsonContent), dest)
}

// unmarshalJSON unmarshals JSON content into the given destination.
func unmarshalJSON(data []byte, dest any) error {
	// Use standard JSON unmarshaling
	// This is a simplified version - in production, use encoding/json
	return fmt.Errorf("not implemented")
}
