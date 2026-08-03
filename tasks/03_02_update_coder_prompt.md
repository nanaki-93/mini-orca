# Task 3.2: Update Coder Prompt

## Goal
Update the coder prompt builder to accept a user feature request string instead of `PlanUnit`.

## Files to Modify
- `/Users/marcoandreose/DEV/lab/mini-orca/internal/agent/prompts/coder.go`

## Detailed Steps

### Step 1: Add FeatureRequest struct
```go
type FeatureRequest struct {
	Prompt         string
	ProjectContext string
	TargetFile     string
	Language       string
}
```

### Step 2: Add BuildCoderPromptFromRequest()
```go
func BuildCoderPromptFromRequest(req FeatureRequest) ([]model.ChatMessage, error) {
	if req.Prompt == "" {
		return nil, fmt.Errorf("coder prompt: feature request prompt is required")
	}
	systemMessage := buildCoderSystemMessage(nil)
	userMessage := buildCoderUserMessageFromRequest(req)
	return []model.ChatMessage{
		{Role: "system", Content: systemMessage},
		{Role: "user", Content: userMessage},
	}, nil
}
```

### Step 3: Add buildCoderUserMessageFromRequest()
```go
func buildCoderUserMessageFromRequest(req FeatureRequest) string {
	var sb promptBuilder
	sb.AppendLine("## Feature Request")
	sb.AppendLine(req.Prompt)
	sb.AppendLine("")
	if req.TargetFile != "" {
		sb.AppendLine("## Target File")
		sb.AppendLine(fmt.Sprintf("Write the code to: %s", req.TargetFile))
		sb.AppendLine("")
	}
	if req.ProjectContext != "" {
		sb.AppendLine("## Project Context")
		sb.AppendLine("Reference this existing project context:")
		sb.AppendLine("```")
		sb.AppendLine(req.ProjectContext)
		sb.AppendLine("```")
		sb.AppendLine("")
	}
	sb.AppendLine("## Instructions")
	sb.AppendLine("1. Generate clean, idiomatic code for this feature")
	sb.AppendLine("2. Include a comment at the very top: // target: path/to/target_file.go")
	sb.AppendLine("3. Return ONLY the code in a code block")
	sb.AppendLine("4. Do NOT include tests — testing is a separate phase")
	return sb.String()
}
```

### Step 4: Mark old BuildCoderPrompt() as deprecated
Keep the function but add a deprecation comment. Same for `buildCoderUserMessage()`.

## Verification
- `go build ./internal/agent/prompts/` — compiles
- `BuildCoderPromptFromRequest()` exists
- Old `BuildCoderPrompt()` still exists for backwards compatibility
