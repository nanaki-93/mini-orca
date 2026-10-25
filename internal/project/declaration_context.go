package project

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

// FunctionContextOptions describes the bounded file-focused context for
// one code-scope request. It intentionally contains no unrelated source.
type FunctionContextOptions struct {
	TargetPath string
	Index      *ProjectIndex
	MaxTokens  int
	Context    context.Context
}

// BuildFunctionWithManifest retains policy and manifest semantics while
// emitting only deterministic facts plus the selected file source.
func (b *ContextBuilder) BuildFunctionWithManifest(root string, options FunctionContextOptions) (string, ContextManifest, error) {
	manifest := ContextManifest{Included: []ContextFile{}, Excluded: []ContextDecision{}, ByteLimit: maxContextBytes, TokenLimit: options.MaxTokens}
	if options.MaxTokens <= 0 {
		manifest.TokenLimit = maxContextTokens
	}
	canonical, err := CanonicalRoot(root)
	if err != nil {
		return "", manifest, err
	}
	policy, err := NewContextPolicy(canonical)
	if err != nil {
		return "", manifest, err
	}
	path := filepath.ToSlash(strings.TrimSpace(options.TargetPath))
	decision := policy.Decide(path)
	if !decision.Include {
		manifest.Excluded = append(manifest.Excluded, decision)
		return "", manifest, fmt.Errorf("file context target is not eligible: %s", decision.Reason)
	}
	file := functionContextFile(options.Index, path)
	if file == nil {
		return "", manifest, fmt.Errorf("file context target is not indexed")
	}
	ctx := options.Context
	if ctx == nil {
		ctx = context.Background()
	}
	data, err := policy.ReadSource(ctx, path, file.ContentHash)
	if err != nil {
		return "", manifest, fmt.Errorf("read file context target: %w", err)
	}
	source := string(data)
	instructions, err := ResolveInstructions(canonical, path)
	if err != nil {
		return "", manifest, err
	}
	guidance := ""
	if instructions.Text != "" {
		guidance = InstructionPromptGuidance + instructions.Text + "\n"
		if estimateTokens(guidance) >= manifest.TokenLimit {
			return "", manifest, fmt.Errorf("project instructions exceed the file context token limit")
		}
	}
	text := guidance + renderFunctionContext(path, *file, source)
	for _, instruction := range instructions.Files {
		manifest.Included = append(manifest.Included, ContextFile{Path: instruction.Path, SizeBytes: int64(len(instruction.Content)), Hash: instruction.Hash, Tokens: estimateTokens(instruction.Content)})
	}
	manifest.Excluded = append(manifest.Excluded, instructions.Excluded...)
	text, truncated := limitContextTokens(text, manifest.TokenLimit)
	manifest.Truncated = truncated
	manifest.EstimatedTokens = estimateTokens(text)
	manifest.Included = append(manifest.Included, ContextFile{Path: path, SizeBytes: int64(len(source)), Hash: file.ContentHash, Tokens: estimateTokens(source), Truncated: truncated})
	return text, manifest, nil
}

func functionContextFile(index *ProjectIndex, path string) *IndexFile {
	if index == nil {
		return nil
	}
	for fileIndex := range index.Files {
		if index.Files[fileIndex].Path == path {
			copy := index.Files[fileIndex]
			return &copy
		}
	}
	return nil
}

func sourceLines(source string, start, end int) string {
	if start <= 0 || end < start {
		return ""
	}
	lines := strings.Split(source, "\n")
	if start > len(lines) {
		return ""
	}
	if end > len(lines) {
		end = len(lines)
	}
	return strings.Join(lines[start-1:end], "\n")
}

func renderFunctionContext(path string, file IndexFile, source string) string {
	var result strings.Builder
	result.WriteString("## File task context\n")
	result.WriteString("Target path: " + path + "\n")
	result.WriteString("Package/import facts: " + strings.Join(file.Imports, ", ") + "\n")
	fenceLang := strings.ToLower(file.Language)
	if fenceLang == "" || fenceLang == "unknown" || fenceLang == "text" {
		fenceLang = ""
	}
	result.WriteString("Target file source:\n```" + fenceLang + "\n" + source + "\n```\n")
	return result.String()
}

func limitContextTokens(value string, maxTokens int) (string, bool) {
	if estimateTokens(value) <= maxTokens {
		return value, false
	}
	marker := "\n[context truncated]\n"
	if maxTokens <= estimateTokens(marker) {
		return "", true
	}
	limit := (maxTokens - estimateTokens(marker)) * 4
	if limit <= 0 {
		return "", true
	}
	runes := []rune(value)
	if limit > len(runes) {
		limit = len(runes)
	}
	return string(runes[:limit]) + marker, true
}
