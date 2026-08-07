package agent

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/llm"
)

// ReviewReport represents the output of a code review phase.
type ReviewReport struct {
	Issues         []string `json:"issues,omitempty"`
	Suggestions    []string `json:"suggestions,omitempty"`
	Score          int      `json:"score"`
	Recommendation string   `json:"recommendation"`
	Summary        string   `json:"summary"`
}

// ReviewerAgent is the agent responsible for reviewing code.
// It embeds *Client to inherit Agent interface implementation.
type ReviewerAgent struct {
	*Client
}

// NewReviewerAgent creates a new reviewer agent with the given LLM client.
func NewReviewerAgent(llmClient *llm.Client) *ReviewerAgent {
	client := NewClient(llmClient)
	client.name = "reviewer"
	client.description = "Reviews code for quality, security, and adherence to standards"
	client.phase = "review"
	client.skills = make([]string, 0)

	return &ReviewerAgent{
		Client: client,
	}
}

// Execute runs the reviewer agent with the given code and context.
// It builds a system prompt from reviewer skills, calls the LLM, and returns a review report.
func (r *ReviewerAgent) Execute(ctx context.Context, input string) (*ReviewReport, error) {
	if r.llmClient == nil {
		return nil, fmt.Errorf("reviewer agent: LLM client not configured")
	}

	if input == "" {
		return nil, fmt.Errorf("reviewer agent: input is required")
	}

	// Build system prompt from reviewer skills
	systemPrompt := r.buildSkillPrompt(r.GetSkills())
	if systemPrompt == "" {
		systemPrompt = "You are an expert code reviewer. Analyze code for quality, security, and adherence to standards."
	}

	// Combine system prompt with user input (code + context)
	fullPrompt := systemPrompt + "\n\n---\n\nCode to Review:\n" + input

	// Call the LLM
	messages := []llm.ChatMessage{
		{Role: "user", Content: fullPrompt},
	}

	resp, err := r.llmClient.Chat(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("reviewer agent: execution failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("reviewer agent: empty response from model")
	}

	// Parse LLM response into ReviewReport
	report := parseReviewReport(resp.Choices[0].Message.Content)

	return report, nil
}

// buildSkillPrompt constructs a system prompt from the given skill names.
func (r *ReviewerAgent) buildSkillPrompt(skillNames []string) string {
	if len(skillNames) == 0 {
		return ""
	}

	var prompt string
	for _, skillName := range skillNames {
		prompt += "Use the " + skillName + " skill.\n"
	}
	return prompt
}

// parseReviewReport parses the LLM response into a ReviewReport struct.
func parseReviewReport(content string) *ReviewReport {
	report := &ReviewReport{
		Summary: content,
	}

	// Extract issues
	report.Issues = reviewerExtractLines(content, "## Issues")
	if len(report.Issues) == 0 {
		report.Issues = reviewerExtractLines(content, "## Problems")
	}
	if len(report.Issues) == 0 {
		report.Issues = reviewerExtractLines(content, "## Findings")
	}

	// Extract suggestions
	report.Suggestions = reviewerExtractLines(content, "## Suggestions")
	if len(report.Suggestions) == 0 {
		report.Suggestions = reviewerExtractLines(content, "## Improvements")
	}
	if len(report.Suggestions) == 0 {
		report.Suggestions = reviewerExtractLines(content, "## Recommendations")
	}

	// Extract score
	report.Score = extractScore(content)

	// Extract recommendation
	report.Recommendation = extractRecommendation(content)

	return report
}

// reviewerExtractLines extracts bullet-pointed lines after a section header.
func reviewerExtractLines(content, header string) []string {
	var lines []string
	// Find the header
	headerIdx := strings.Index(content, header)
	if headerIdx == -1 {
		return nil
	}

	// Get content after header
	afterHeader := content[headerIdx+len(header):]

	// Split by newlines and extract bullet points
	for _, line := range strings.Split(afterHeader, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "- ") {
			lines = append(lines, strings.TrimPrefix(line, "- "))
		} else if strings.HasPrefix(line, "* ") {
			lines = append(lines, strings.TrimPrefix(line, "* "))
		}
	}

	return lines
}

// extractScore extracts the review score from content.
func extractScore(content string) int {
	// Look for common score patterns
	for _, pattern := range []string{"score:", "score is", "rating:", "rating is", "quality score"} {
		idx := strings.Index(strings.ToLower(content), pattern)
		if idx != -1 {
			// Extract next few characters after pattern
			after := content[idx+len(pattern):]
			var scoreStr string
			for _, ch := range after {
				if ch >= '0' && ch <= '9' {
					scoreStr += string(ch)
				} else if len(scoreStr) > 0 {
					break
				}
			}
			if scoreStr != "" {
				score, err := strconv.Atoi(scoreStr)
				if err == nil && score >= 0 && score <= 100 {
					return score
				}
			}
		}
	}

	// Default score if not found
	return 0
}

// extractRecommendation extracts the final recommendation from content.
func extractRecommendation(content string) string {
	// Look for recommendation section
	recommendationHeader := "## Recommendation"
	headerIdx := strings.Index(content, recommendationHeader)
	if headerIdx != -1 {
		// Get content after header
		afterHeader := content[headerIdx+len(recommendationHeader):]
		// Take first non-empty line as the recommendation
		for _, line := range strings.Split(afterHeader, "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				return line
			}
		}
	}

	// Look for alternative recommendation headers
	for _, header := range []string{"## Verdict", "## Conclusion", "## Final Decision"} {
		headerIdx := strings.Index(content, header)
		if headerIdx != -1 {
			afterHeader := content[headerIdx+len(header):]
			for _, line := range strings.Split(afterHeader, "\n") {
				line = strings.TrimSpace(line)
				if line != "" {
					return line
				}
			}
		}
	}

	// Default recommendation based on score
	return "Review complete. See issues and suggestions above."
}
