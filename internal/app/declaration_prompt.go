package app

import (
	"fmt"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/llm"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
	"github.com/nanaki-93/mini-orca/v2/internal/workflow"
)

const functionRequestPreamble = "You are an expert software engineer. Generate clean, idiomatic code in the target project's existing language and style.\n\n---\n\nAtomic Unit Description:\n"

func generationMessages(userPrompt, projectContext, targetFile, targetSymbol string, scope workflow.ScopeMode) ([]llm.ChatMessage, error) {
	if strings.TrimSpace(userPrompt) == "" {
		return nil, fmt.Errorf("generation prompt is required")
	}
	if strings.TrimSpace(targetFile) == "" || strings.TrimSpace(targetSymbol) == "" {
		return nil, fmt.Errorf("generation target file and symbol are required")
	}
	var input strings.Builder
	input.WriteString("## Atomic code request\n" + userPrompt + "\n\n")
	input.WriteString("## Immutable scope\n")
	input.WriteString("Target file: " + targetFile + "\n")
	input.WriteString("Target function or class: " + targetSymbol + "\n")
	input.WriteString("Action: fix\n")
	input.WriteString("Scope mode: " + string(scope) + "\n")
	input.WriteString("You may change only this named symbol in this one file. Do not create, rename, or modify any other file or symbol. Preserve unrelated target-file code exactly.\n\n")
	input.WriteString("## Project-wide context\n" + projectContext + "\n\n")
	input.WriteString("## Output contract\nReturn exactly one JSON object and no Markdown or prose. Its fields must be version (\"v1\"), target_path (\"" + targetFile + "\"), target_symbol (\"" + targetSymbol + "\"), scope_mode (\"" + string(scope) + "\"), candidate_content (the complete updated content of " + targetFile + "), and optional rationale. Do not return patches, explanations outside rationale, or additional files.\n")
	return functionRequestMessages(input.String()), nil
}

func declarationDraftMessages(userPrompt, conversation, projectContext, targetFile, targetSymbol string, mode project.DeclarationEditMode) ([]llm.ChatMessage, error) {
	if strings.TrimSpace(userPrompt) == "" {
		return nil, fmt.Errorf("declaration request is required")
	}
	if strings.TrimSpace(targetFile) == "" || strings.TrimSpace(targetSymbol) == "" {
		return nil, fmt.Errorf("declaration target file and symbol are required")
	}
	if mode != project.DeclarationEditReplaceSymbol && mode != project.DeclarationEditCreateSymbol {
		return nil, fmt.Errorf("unsupported declaration edit mode %q", mode)
	}
	var input strings.Builder
	input.WriteString("## File-scoped declaration request\n" + userPrompt + "\n\n")
	input.WriteString("## Immutable session scope\n")
	input.WriteString("Target file: " + targetFile + "\n")
	input.WriteString("Target symbol: " + targetSymbol + "\n")
	input.WriteString("Edit mode: " + string(mode) + "\n")
	input.WriteString("Return exactly one complete Go function, method, type, or single top-level var declaration for this symbol. Do not return package clauses, a full file, a patch, or any declaration for another symbol. Imports must be listed separately.\n\n")
	if strings.TrimSpace(conversation) != "" {
		input.WriteString("## Earlier file-scoped conversation\n" + conversation + "\n\n")
	}
	input.WriteString("## One-file context\n" + projectContext + "\n\n")
	input.WriteString("## Output contract\nReturn exactly one JSON object and no Markdown. Its fields must be version (\"v1\"), declaration, imports (an optional array of import specs), and explanation (a concise assistant explanation). Do not include target paths, symbols, complete file content, patches, or extra fields.\n")
	return functionRequestMessages(input.String()), nil
}

func functionRequestMessages(input string) []llm.ChatMessage {
	return []llm.ChatMessage{{Role: "user", Content: functionRequestPreamble + input}}
}
