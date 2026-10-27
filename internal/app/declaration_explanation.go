package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentruntime"
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
// project and indexed source file.
type DeclarationExplanationRequest struct {
	ProjectID             string `json:"project_id"`
	ProjectRevision       string `json:"project_revision"`
	BaseFileHash          string `json:"base_file_hash"`
	TargetPath            string `json:"target_path"`
	ConfirmRemoteProvider bool   `json:"confirm_remote_provider,omitempty"`
}

// SourceAnchor identifies the bounded source range that the explanation covers.
type SourceAnchor struct {
	Path      string `json:"path"`
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
	file         project.IndexFile
	context      string
	manifest     project.ContextManifest
	policy       string
	instructions string
	root         string
}

type currentDeclarationExplanationTarget struct {
	file project.IndexFile
}

// ExplainDeclaration investigates an admitted file without creating a
// proposal or granting check, Apply or Undo authority.
func (s *Service) ExplainDeclaration(ctx context.Context, request DeclarationExplanationRequest) (*DeclarationExplanation, error) {
	if err := s.RequireRemoteConfirmation(config.CodeModelScope, request.ConfirmRemoteProvider); err != nil {
		return nil, err
	}
	ctx, err := s.freezeAgentSettings(ctx, s.manager.Root())
	if err != nil {
		return nil, err
	}
	runtime := s.runtimes.function
	prepared, err := s.prepareDeclarationExplanation(ctx, request, runtime)
	if err != nil {
		return nil, err
	}
	messages := []llm.ChatMessage{{Role: "user", Content: declarationExplanationPrompt(prepared.context)}}
	input := agentruntime.Request{Messages: messages, ContextMaxTokens: runtime.effective.ContextMaxTokens}
	if llm.ProviderCapabilities(runtime.profile).Tools {
		input.Investigation, err = s.declarationInvestigation(ctx, request, prepared)
		if err != nil {
			return nil, err
		}
	}
	return runKnowledge(ctx, s, runtime, agentruntime.KnowledgeExplanation, func(ctx context.Context) (*DeclarationExplanation, string, error) {
		explanation, err := s.explainDeclarationInRun(ctx, request, prepared, runtime, input)
		return explanation, "completed", err
	})
}

func (s *Service) explainDeclarationInRun(ctx context.Context, request DeclarationExplanationRequest, prepared preparedDeclarationExplanation, runtime modelRuntime, input agentruntime.Request) (*DeclarationExplanation, error) {
	timed, cancel := context.WithTimeout(ctx, duration(runtime.effective.Timeout))
	defer cancel()
	authorize := func(ctx context.Context) error { return s.validateExplanationContext(ctx, request, prepared) }
	result, err := s.retryAgentRequest(timed, runtime, input, authorize)
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
	// reindex, or source edit cannot publish against a superseded file.
	if err := s.validateExplanationContext(ctx, request, prepared); err != nil {
		return nil, err
	}
	if err := addSkillContext(ctx, &prepared.manifest, input.Investigation); err != nil {
		return nil, err
	}
	if err := input.Investigation.Validate(ctx); err != nil {
		return nil, err
	}
	return &DeclarationExplanation{
		Version: parsed.Version, ProjectID: request.ProjectID, ProjectRevision: request.ProjectRevision,
		BaseFileHash: request.BaseFileHash,
		Anchor:       SourceAnchor{Path: prepared.file.Path, StartLine: 1, EndLine: max(1, prepared.file.LineCount)},
		Summary:      parsed.Summary, Behavior: parsed.Behavior, Inputs: parsed.Inputs, Outputs: parsed.Outputs,
		SideEffects: parsed.SideEffects, ErrorBehavior: parsed.ErrorBehavior,
		EngineeringInsight: project.CloneEngineeringInsight(parsed.EngineeringInsight), ContextManifest: prepared.manifest,
	}, nil
}

func (s *Service) prepareDeclarationExplanation(ctx context.Context, request DeclarationExplanationRequest, runtime modelRuntime) (preparedDeclarationExplanation, error) {
	target, index, err := s.currentDeclarationExplanationTargetWithIndex(request)
	if err != nil {
		return preparedDeclarationExplanation{}, err
	}
	root := s.manager.Root()
	policy, err := project.NewContextPolicy(root)
	if err != nil {
		return preparedDeclarationExplanation{}, err
	}
	instructions, err := project.ResolveInstructions(root, request.TargetPath)
	if err != nil {
		return preparedDeclarationExplanation{}, err
	}
	functionContext, manifest, err := s.buildDeclarationContext(s.manager.Root(), project.FunctionContextOptions{
		TargetPath: request.TargetPath,
		Index:      index,
		MaxTokens:  declarationContextLimit(runtime.effective.ContextMaxTokens),
		Context:    ctx,
	})
	if err != nil {
		return preparedDeclarationExplanation{}, fmt.Errorf("build declaration explanation context: %w", err)
	}
	if err := s.validatePreparedExplanationTarget(request, target.file); err != nil {
		return preparedDeclarationExplanation{}, err
	}
	if err := project.ValidateInstructions(root, request.TargetPath, instructions.Fingerprint); err != nil {
		return preparedDeclarationExplanation{}, err
	}
	return preparedDeclarationExplanation{file: target.file, context: functionContext, manifest: s.contextManifestForRuntime(manifest, runtime), policy: policy.Version(), instructions: instructions.Fingerprint, root: root}, nil
}

func (s *Service) validatePreparedExplanationTarget(request DeclarationExplanationRequest, preparedFile project.IndexFile) error {
	current, err := s.currentDeclarationExplanationTarget(request)
	if err != nil {
		return err
	}
	if !sameExplanationAnchor(preparedFile, current.file) {
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
	if file.Binary || file.Language == "Text" || file.Language == "Markdown" {
		return currentDeclarationExplanationTarget{}, nil, fmt.Errorf("%w: declaration explanation requires an indexed source file", project.ErrUnsupportedFile)
	}
	return currentDeclarationExplanationTarget{file: *file}, index, nil
}

func sameExplanationAnchor(preparedFile project.IndexFile, currentFile project.IndexFile) bool {
	return preparedFile.Path == currentFile.Path && preparedFile.ContentHash == currentFile.ContentHash && preparedFile.LineCount == currentFile.LineCount
}

func declarationExplanationPrompt(functionContext string) string {
	return "Explain the supplied source file. Return one JSON object only, without Markdown or code fences. " +
		"Required fields: version (\"v1\"), summary (string), behavior (string array), inputs (string array), outputs (string array), side_effects (string array), error_behavior (string array). " +
		"Each array must contain at most six concise items. Optionally include engineering_insight with exactly mechanism, why_it_matters_here, and optional tradeoff_or_failure_mode and transferable_lesson. " + project.EngineeringInsightPromptInstructions +
		"Ground every statement in the supplied source file and indexed facts. Do not propose edits, invent callers, or reproduce the file wholesale.\n\n" + functionContext
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

func declarationContextLimit(limit int) int {
	if limit <= 0 {
		return 4000
	}
	return limit
}
