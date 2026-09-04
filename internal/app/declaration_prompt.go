package app

import (
	"fmt"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/llm"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

const functionRequestPreamble = "You are an expert software engineer. Generate clean, idiomatic code in the target project's existing language and style.\n\n---\n\nAtomic Unit Description:\n"

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
	input.WriteString("## Output contract\nReturn exactly one JSON object and no Markdown. Its fields must be version (\"v1\"), declaration, imports (an optional array of import specs), explanation (a concise assistant explanation), and optional engineering_insight ({mechanism,why_it_matters_here,tradeoff_or_failure_mode?,transferable_lesson?}). An insight is advisory 50-90 word prose grounded in the supplied context; omit it when there is no non-obvious mechanism or trade-off. Do not include target paths, symbols, complete file content, patches, or extra fields.\n")
	return functionRequestMessages(input.String()), nil
}

func functionRequestMessages(input string) []llm.ChatMessage {
	return []llm.ChatMessage{{Role: "user", Content: functionRequestPreamble + input}}
}
