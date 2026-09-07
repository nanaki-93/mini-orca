package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/llm"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

const (
	declarationExplanationVersion  = "v1"
	maxDeclarationExplanationBytes = 16 * 1024
	maxDeclarationExplanationItems = 6
	maxDeclarationExplanationRunes = 2_000
)

// DeclarationExplanationRequest pins one read-only request to the exact active
// project, source file, and indexed Go declaration.
type DeclarationExplanationRequest struct {
	ProjectID             string `json:"project_id"`
	ProjectRevision       string `json:"project_revision"`
	BaseFileHash          string `json:"base_file_hash"`
	TargetPath            string `json:"target_path"`
	TargetSymbol          string `json:"target_symbol"`
	ConfirmRemoteProvider bool   `json:"confirm_remote_provider,omitempty"`
}

// SourceAnchor identifies the bounded source range that the explanation covers.
type SourceAnchor struct {
	Path      string `json:"path"`
	Symbol    string `json:"symbol"`
	Signature string `json:"signature"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
}

// DeclarationExplanation is transient read-only data. It is never persisted in
// chat history and cannot be applied as source.
type DeclarationExplanation struct {
	Version            string                      `json:"version"`
	ProjectID          string                      `json:"project_id"`
	ProjectRevision    string                      `json:"project_revision"`
	BaseFileHash       string                      `json:"base_file_hash"`
	Anchor             SourceAnchor                `json:"anchor"`
	Summary            string                      `json:"summary"`
	Behavior           []string                    `json:"behavior"`
	Inputs             []string                    `json:"inputs"`
	Outputs            []string                    `json:"outputs"`
	SideEffects        []string                    `json:"side_effects"`
	ErrorBehavior      []string                    `json:"error_behavior"`
	EngineeringInsight *project.EngineeringInsight `json:"engineering_insight,omitempty"`
	ContextManifest    project.ContextManifest     `json:"context_manifest"`
}

type declarationExplanationModelResponse struct {
	Version            string                      `json:"version"`
	Summary            string                      `json:"summary"`
	Behavior           []string                    `json:"behavior"`
	Inputs             []string                    `json:"inputs"`
	Outputs            []string                    `json:"outputs"`
	SideEffects        []string                    `json:"side_effects"`
	ErrorBehavior      []string                    `json:"error_behavior"`
	EngineeringInsight *project.EngineeringInsight `json:"engineering_insight,omitempty"`
}

type declarationExplanationWireResponse struct {
	Version            string          `json:"version"`
	Summary            string          `json:"summary"`
	Behavior           []string        `json:"behavior"`
	Inputs             []string        `json:"inputs"`
	Outputs            []string        `json:"outputs"`
	SideEffects        []string        `json:"side_effects"`
	ErrorBehavior      []string        `json:"error_behavior"`
	EngineeringInsight json.RawMessage `json:"engineering_insight"`
}

type preparedDeclarationExplanation struct {
	file     project.IndexFile
	symbol   project.SymbolInfo
	context  string
	manifest project.ContextManifest
}

type currentDeclarationExplanationTarget struct {
	file   project.IndexFile
	symbol project.SymbolInfo
}

// ExplainDeclaration performs one Function-scope model request without creating
// a session, draft, check, apply, undo, or persisted analysis record.
func (s *Service) ExplainDeclaration(ctx context.Context, request DeclarationExplanationRequest) (*DeclarationExplanation, error) {
	if err := s.RequireRemoteConfirmation(config.FunctionModelScope, request.ConfirmRemoteProvider); err != nil {
		return nil, err
	}
	runtime := s.runtimes.function
	prepared, err := s.prepareDeclarationExplanation(request, runtime)
	if err != nil {
		return nil, err
	}
	messages := []llm.ChatMessage{{Role: "user", Content: declarationExplanationPrompt(prepared.context)}}
	timed, cancel := context.WithTimeout(ctx, duration(runtime.effective.Timeout))
	defer cancel()
	result, err := s.retry(timed, runtime, messages)
	if timed.Err() != nil {
		return nil, timed.Err()
	}
	if err != nil {
		return nil, err
	}
	parsed, err := ParseDeclarationExplanationResponse(result.Content)
	if err != nil {
		return nil, err
	}
	// Re-read both policy and index state after provider work so a policy change,
	// reindex, or source edit cannot publish against a superseded declaration.
	if err := s.validatePreparedExplanationTarget(request, prepared.file, prepared.symbol); err != nil {
		return nil, err
	}
	return &DeclarationExplanation{
		Version: parsed.Version, ProjectID: request.ProjectID, ProjectRevision: request.ProjectRevision,
		BaseFileHash: request.BaseFileHash,
		Anchor:       SourceAnchor{Path: prepared.file.Path, Symbol: prepared.symbol.Name, Signature: prepared.symbol.Signature, StartLine: prepared.symbol.StartLine, EndLine: prepared.symbol.EndLine},
		Summary:      parsed.Summary, Behavior: parsed.Behavior, Inputs: parsed.Inputs, Outputs: parsed.Outputs,
		SideEffects: parsed.SideEffects, ErrorBehavior: parsed.ErrorBehavior,
		EngineeringInsight: project.CloneEngineeringInsight(parsed.EngineeringInsight), ContextManifest: prepared.manifest,
	}, nil
}

func (s *Service) prepareDeclarationExplanation(request DeclarationExplanationRequest, runtime modelRuntime) (preparedDeclarationExplanation, error) {
	target, index, err := s.currentDeclarationExplanationTargetWithIndex(request)
	if err != nil {
		return preparedDeclarationExplanation{}, err
	}
	functionContext, manifest, err := s.buildDeclarationContext(s.manager.Root(), project.FunctionContextOptions{
		TargetPath: request.TargetPath, TargetSymbol: request.TargetSymbol,
		Mode: project.DeclarationEditReplaceSymbol, Index: index,
		MaxTokens: sessionFunctionContextLimit(runtime.effective.ContextMaxTokens),
	})
	if err != nil {
		return preparedDeclarationExplanation{}, fmt.Errorf("build declaration explanation context: %w", err)
	}
	if err := s.validatePreparedExplanationTarget(request, target.file, target.symbol); err != nil {
		return preparedDeclarationExplanation{}, err
	}
	return preparedDeclarationExplanation{file: target.file, symbol: target.symbol, context: functionContext, manifest: s.contextManifestForRuntime(manifest, runtime)}, nil
}

func (s *Service) validatePreparedExplanationTarget(request DeclarationExplanationRequest, preparedFile project.IndexFile, preparedSymbol project.SymbolInfo) error {
	current, err := s.currentDeclarationExplanationTarget(request)
	if err != nil {
		return err
	}
	if !sameExplanationAnchor(preparedFile, preparedSymbol, current.file, current.symbol) {
		return project.ErrRevisionConflict
	}
	return nil
}

func (s *Service) currentDeclarationExplanationTarget(request DeclarationExplanationRequest) (currentDeclarationExplanationTarget, error) {
	target, _, err := s.currentDeclarationExplanationTargetWithIndex(request)
	return target, err
}

func (s *Service) currentDeclarationExplanationTargetWithIndex(request DeclarationExplanationRequest) (currentDeclarationExplanationTarget, *project.ProjectIndex, error) {
	if err := s.ValidateMutableRequest(request.ProjectID, request.ProjectRevision, request.TargetPath, request.BaseFileHash); err != nil {
		return currentDeclarationExplanationTarget{}, nil, err
	}
	index, err := s.manager.Index()
	if err != nil {
		return currentDeclarationExplanationTarget{}, nil, err
	}
	if index.ProjectID != request.ProjectID || index.ProjectRevision != request.ProjectRevision {
		return currentDeclarationExplanationTarget{}, nil, project.ErrRevisionConflict
	}
	file, err := s.manager.IndexedFile(request.TargetPath)
	if err != nil {
		return currentDeclarationExplanationTarget{}, nil, err
	}
	if file.Language != "Go" || file.Binary {
		return currentDeclarationExplanationTarget{}, nil, fmt.Errorf("%w: declaration explanation requires an indexed Go source file", project.ErrUnsupportedFile)
	}
	if err := validateChatTarget(*file, project.DeclarationEditReplaceSymbol, request.TargetSymbol); err != nil {
		return currentDeclarationExplanationTarget{}, nil, fmt.Errorf("declaration explanation target: %w", err)
	}
	symbol := exactExplanationSymbol(*file, request.TargetSymbol)
	if symbol == nil {
		return currentDeclarationExplanationTarget{}, nil, fmt.Errorf("declaration explanation requires one exact atomic declaration")
	}
	return currentDeclarationExplanationTarget{file: *file, symbol: *symbol}, index, nil
}

func sameExplanationAnchor(preparedFile project.IndexFile, preparedSymbol project.SymbolInfo, currentFile project.IndexFile, currentSymbol project.SymbolInfo) bool {
	return preparedFile.Path == currentFile.Path && preparedSymbol.Name == currentSymbol.Name && preparedSymbol.Signature == currentSymbol.Signature && preparedSymbol.StartLine == currentSymbol.StartLine && preparedSymbol.EndLine == currentSymbol.EndLine
}

func exactExplanationSymbol(file project.IndexFile, target string) *project.SymbolInfo {
	var match *project.SymbolInfo
	for i := range file.Symbols {
		candidate := file.Symbols[i]
		if candidate.Name == target && candidate.AtomicTarget && candidate.Confidence == "exact" {
			if match != nil {
				return nil
			}
			copy := candidate
			match = &copy
		}
	}
	return match
}

func declarationExplanationPrompt(functionContext string) string {
	return "Explain exactly the supplied Go declaration. Return one JSON object only, without Markdown or code fences. " +
		"Required fields: version (\"v1\"), summary (string), behavior (string array), inputs (string array), outputs (string array), side_effects (string array), error_behavior (string array). " +
		"Each array must contain at most six concise items. Optionally include engineering_insight with exactly mechanism, why_it_matters_here, and optional tradeoff_or_failure_mode and transferable_lesson. " + project.EngineeringInsightPromptInstructions +
		"Ground every statement in the supplied declaration and indexed facts. Do not propose edits, invent callers, or reproduce the declaration wholesale.\n\n" + functionContext
}

// ParseDeclarationExplanationResponse accepts one small, closed JSON contract.
func ParseDeclarationExplanationResponse(output string) (declarationExplanationModelResponse, error) {
	if len(output) == 0 || len(output) > maxDeclarationExplanationBytes {
		return declarationExplanationModelResponse{}, fmt.Errorf("declaration explanation response is empty or exceeds the size limit")
	}
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return declarationExplanationModelResponse{}, fmt.Errorf("declaration explanation response is empty or exceeds the size limit")
	}
	var wire declarationExplanationWireResponse
	decoder := json.NewDecoder(strings.NewReader(trimmed))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&wire); err != nil {
		return declarationExplanationModelResponse{}, fmt.Errorf("parse declaration explanation response: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return declarationExplanationModelResponse{}, fmt.Errorf("declaration explanation response must contain one JSON object")
	}
	response := declarationExplanationModelResponse{
		Version: wire.Version, Summary: wire.Summary, Behavior: wire.Behavior, Inputs: wire.Inputs,
		Outputs: wire.Outputs, SideEffects: wire.SideEffects, ErrorBehavior: wire.ErrorBehavior,
	}
	response.EngineeringInsight, _ = project.ParseOptionalEngineeringInsight(wire.EngineeringInsight)
	if err := validateDeclarationExplanation(response); err != nil {
		return declarationExplanationModelResponse{}, err
	}
	return response, nil
}

func validateDeclarationExplanation(response declarationExplanationModelResponse) error {
	if response.Version != declarationExplanationVersion || strings.TrimSpace(response.Summary) == "" {
		return fmt.Errorf("declaration explanation response does not satisfy version and summary requirements")
	}
	groups := [][]string{response.Behavior, response.Inputs, response.Outputs, response.SideEffects, response.ErrorBehavior}
	runes := utf8.RuneCountInString(response.Summary)
	for _, group := range groups {
		if group == nil || len(group) > maxDeclarationExplanationItems {
			return fmt.Errorf("declaration explanation response is incomplete or exceeds item limits")
		}
		for _, item := range group {
			if strings.TrimSpace(item) == "" {
				return fmt.Errorf("declaration explanation response contains an empty item")
			}
			runes += utf8.RuneCountInString(item)
		}
	}
	if runes > maxDeclarationExplanationRunes {
		return fmt.Errorf("declaration explanation response exceeds text limits")
	}
	response.Summary = strings.TrimSpace(response.Summary)
	return nil
}
