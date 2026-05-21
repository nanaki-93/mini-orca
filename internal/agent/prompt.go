package agent

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nanaki-93/mini-orca/internal/state"
)

// GenerateProjectAnalysis handles Step 1: High-level architectural options
func (c *Client) GenerateProjectAnalysis(goal string) (string, error) {
	systemPrompt := `You are an expert software architect specialized in Kotlin/Java, Go, Kubernetes, and Terraform.
Analyze the user's project goal and provide a comprehensive architectural solution layout.
Focus on structure, tech stack choices, and decoupling strategies.
Output your response entirely in clean, readable Markdown.`

	return c.CallLMStudio(systemPrompt, fmt.Sprintf("Analyze and design a strategy for: %s", goal))
}

// GenerateTaskBreakdown handles Step 2: Low-level json task generation
func (c *Client) GenerateTaskBreakdown(approvedAnalysis string) ([]state.MicroTask, error) {
	systemPrompt := `You are an expert technical planner. Your job is to break down architectural strategies into granular, sequential micro-activities.
You MUST output your response as a raw, single JSON array matching this exact schema:
[
  {
    "id": "task-01",
    "description": "Short explanation of what to do",
    "target_file": "relative/path/to/file.go",
    "command": "optional shell validation command to execute like 'go test ./...'"
  }
]
Do not wrap your JSON in markdown blocks (like ` + "```json" + `). Output only raw, parseable JSON.`

	userPrompt := fmt.Sprintf("Convert this approved architecture layout into a list of micro tasks:\n\n%s", approvedAnalysis)
	rawJSON, err := c.CallLMStudio(systemPrompt, userPrompt)
	if err != nil {
		return nil, err
	}

	// Clean up formatting edge-cases if the local model accidentally spits out markdown backticks anyway
	rawJSON = strings.TrimPrefix(rawJSON, "```json")
	rawJSON = strings.TrimPrefix(rawJSON, "```")
	rawJSON = strings.TrimSuffix(rawJSON, "```")
	rawJSON = strings.TrimSpace(rawJSON)

	var tasks []state.MicroTask
	if err := json.Unmarshal([]byte(rawJSON), &tasks); err != nil {
		return nil, fmt.Errorf("failed to parse model output into structured task array: %w. Raw text was: %s", err, rawJSON)
	}

	return tasks, nil
}
