package project

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// FunctionContextOptions describes the bounded declaration-focused context for
// one Function-scope request. It intentionally contains no unrelated source.
type FunctionContextOptions struct {
	TargetPath       string
	TargetSymbol     string
	Mode             DeclarationEditMode
	Index            *ProjectIndex
	TaskSpec         *BugTaskSpec
	PriorDeclaration string
	MaxTokens        int
}

// BuildFunctionWithManifest retains policy and manifest semantics while
// emitting only deterministic facts plus the selected declaration or stub.
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
		return "", manifest, fmt.Errorf("function context target is not eligible: %s", decision.Reason)
	}
	fullPath, err := ResolveFile(canonical, path)
	if err != nil {
		return "", manifest, err
	}
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return "", manifest, fmt.Errorf("read function context target: %w", err)
	}
	if isBinary(data) {
		return "", manifest, fmt.Errorf("function context target must be text")
	}
	file := functionContextFile(options.Index, path)
	if file == nil {
		return "", manifest, fmt.Errorf("function context target is not indexed")
	}
	declaration, err := functionContextDeclaration(string(data), *file, options.TargetSymbol, options.Mode)
	if err != nil {
		return "", manifest, err
	}
	text := renderFunctionContext(path, *file, declaration, options)
	text, truncated := limitContextTokens(text, manifest.TokenLimit)
	manifest.Truncated = truncated
	manifest.EstimatedTokens = estimateTokens(text)
	manifest.Included = append(manifest.Included, ContextFile{Path: path, SizeBytes: int64(len(declaration)), Hash: file.ContentHash, Tokens: estimateTokens(declaration), Truncated: truncated})
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

func functionContextDeclaration(source string, file IndexFile, symbol string, mode DeclarationEditMode) (string, error) {
	if mode == DeclarationEditCreateSymbol {
		return "// Create a new top-level declaration named " + symbol + ".", nil
	}
	for _, candidate := range file.Symbols {
		if candidate.Name == symbol && candidate.Confidence == "exact" && candidate.AtomicTarget {
			return sourceLines(source, candidate.StartLine, candidate.EndLine), nil
		}
	}
	return "", fmt.Errorf("function context target symbol is not exact")
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

func renderFunctionContext(path string, file IndexFile, declaration string, options FunctionContextOptions) string {
	var result strings.Builder
	result.WriteString("## Function task context\n")
	result.WriteString("Target path: " + path + "\n")
	result.WriteString("Target symbol: " + options.TargetSymbol + "\n")
	result.WriteString("Package/import facts: " + strings.Join(file.Imports, ", ") + "\n")
	if options.TaskSpec != nil {
		task := options.TaskSpec
		result.WriteString("Task signature: " + task.TargetSignature + "\nAcceptance criteria:\n")
		for _, criterion := range task.AcceptanceCriteria {
			result.WriteString("- " + criterion + "\n")
		}
		result.WriteString("Non-goals:\n")
		for _, nonGoal := range task.NonGoals {
			result.WriteString("- " + nonGoal + "\n")
		}
	}
	signatures := directlyRelevantSignatures(file, options.TargetSymbol)
	if len(signatures) > 0 {
		result.WriteString("Relevant indexed signatures:\n")
		for _, signature := range signatures {
			result.WriteString("- " + signature + "\n")
		}
	}
	result.WriteString("Selected declaration or stub:\n```go\n" + declaration + "\n```\n")
	if options.PriorDeclaration != "" {
		result.WriteString("Prior declaration revision (explicit request only):\n```go\n" + options.PriorDeclaration + "\n```\n")
	}
	return result.String()
}

func directlyRelevantSignatures(file IndexFile, target string) []string {
	values := make([]string, 0, len(file.Symbols))
	for _, symbol := range file.Symbols {
		if symbol.Name == target || symbol.Confidence == "exact" {
			values = append(values, symbol.Name+": "+symbol.Signature)
		}
	}
	sort.Strings(values)
	if len(values) > 16 {
		values = values[:16]
	}
	return values
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
