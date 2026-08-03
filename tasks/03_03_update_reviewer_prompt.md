# Task 3.3: Update Reviewer Prompt

## Goal
Update the reviewer prompt builder to compare code against user's original prompt instead of a structured plan.

## Files to Modify
- `/Users/marcoandreose/DEV/lab/mini-orca/internal/agent/prompts/reviewer.go`

## Detailed Steps

### Step 1: Add BuildReviewerPromptFromRequest()
```go
func BuildReviewerPromptFromRequest(code string, userRequest string, skills []string) ([]model.ChatMessage, error) {
	if code == "" {
		return nil, fmt.Errorf("reviewer prompt: code is required")
	}
	if userRequest == "" {
		return nil, fmt.Errorf("reviewer prompt: user request is required")
	}
	systemMessage := buildReviewerSystemMessage(skills)
	userMessage := buildReviewerUserMessageFromRequest(code, userRequest)
	return []model.ChatMessage{
		{Role: "system", Content: systemMessage},
		{Role: "user", Content: userMessage},
	}, nil
}
```

### Step 2: Add buildReviewerUserMessageFromRequest()
```go
func buildReviewerUserMessageFromRequest(code string, userRequest string) string {
	var sb promptBuilder
	sb.AppendLine("## User Request")
	sb.AppendLine(userRequest)
	sb.AppendLine("")
	sb.AppendLine("## Generated Code")
	sb.AppendLine("Review this code against the user's request above:")
	sb.AppendLine("```go")
	sb.AppendLine(code)
	sb.AppendLine("```")
	sb.AppendLine("")
	sb.AppendLine("## Review Criteria")
	sb.AppendLine("1. Does the code fulfill the user's request?")
	sb.AppendLine("2. Is the code clean, idiomatic, and well-structured?")
	sb.AppendLine("3. Are there any bugs or logical errors?")
	sb.AppendLine("4. Is error handling appropriate?")
	sb.AppendLine("5. Are there any security concerns?")
	sb.AppendLine("")
	sb.AppendLine("Provide your review in the structured format specified in the system prompt.")
	return sb.String()
}
```

### Step 3: Mark old functions as deprecated
Keep `BuildReviewerPrompt()` and `buildReviewerUserMessage()` with deprecation comments.

## Verification
- `go build ./internal/agent/prompts/` — compiles
- `BuildReviewerPromptFromRequest()` exists
- Old `BuildReviewerPrompt()` still exists for backwards compatibility
