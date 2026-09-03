package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
	"github.com/nanaki-93/mini-orca/v2/internal/workflow"
)

const generationResponseVersion = "v1"

var fencedCandidate = regexp.MustCompile("(?s)^\\s*```[[:alnum:]_+.-]*\\r?\\n(.*?)\\r?\\n?```\\s*$")

// GenerationResponse is the versioned contract accepted from a model.
type GenerationResponse struct {
	Version          string             `json:"version"`
	TargetPath       string             `json:"target_path"`
	TargetSymbol     string             `json:"target_symbol"`
	ScopeMode        workflow.ScopeMode `json:"scope_mode"`
	CandidateContent string             `json:"candidate_content"`
	Rationale        string             `json:"rationale,omitempty"`
}

// GenerationPreview is an in-memory, unvalidated candidate. It is returned for
// review only and is never persisted or applied by this package.
type GenerationPreview struct {
	GenerationID      string                       `json:"generation_id"`
	Version           string                       `json:"version"`
	ProjectID         string                       `json:"project_id"`
	ProjectRevision   string                       `json:"project_revision"`
	BaseFileHash      string                       `json:"base_file_hash"`
	TargetPath        string                       `json:"target_path"`
	TargetSymbol      string                       `json:"target_symbol"`
	ScopeMode         workflow.ScopeMode           `json:"scope_mode"`
	CandidateContent  string                       `json:"candidate_content"`
	CandidateHash     string                       `json:"candidate_hash"`
	Rationale         string                       `json:"rationale,omitempty"`
	Action            string                       `json:"action,omitempty"`
	TemplateID        string                       `json:"template_id,omitempty"`
	TemplateInputHash string                       `json:"template_input_hash,omitempty"`
	EffectiveModel    EffectiveModel               `json:"effective_model"`
	ContextManifest   project.ContextManifest      `json:"context_manifest"`
	Validation        project.GenerationValidation `json:"validation"`
}

// Generate produces an in-memory candidate preview only. It captures the base
// project and file state before calling the model and rejects malformed output.
func (s *Service) Generate(ctx context.Context, userPrompt, targetFile, targetSymbol string, scope workflow.ScopeMode, confirmRemoteProvider bool) (*GenerationPreview, error) {
	if err := s.RequireRemoteConfirmation(config.FunctionModelScope, confirmRemoteProvider); err != nil {
		return nil, err
	}
	if scope == "" {
		scope = workflow.ScopeStrictSymbol
	}
	if scope != workflow.ScopeStrictSymbol && scope != workflow.ScopeSymbolPlusImports {
		return nil, fmt.Errorf("unsupported generation scope %q", scope)
	}
	indexedFile, err := s.manager.IndexedFile(targetFile)
	if err != nil {
		return nil, err
	}
	if indexedFile.Binary {
		return nil, fmt.Errorf("target file must be a text source file")
	}
	analysis, err := s.manager.Analysis()
	if err != nil {
		return nil, err
	}
	fileInfo, err := project.GetFileInfo(s.manager.Root(), indexedFile.Path)
	if err != nil {
		return nil, err
	}
	if fileInfo.ContentHash != indexedFile.ContentHash {
		return nil, project.ErrRevisionConflict
	}
	index, err := s.manager.Index()
	if err != nil {
		return nil, err
	}
	projectContext, manifest, err := project.NewContextBuilder().BuildFunctionWithManifest(s.manager.Root(), project.FunctionContextOptions{
		TargetPath: indexedFile.Path, TargetSymbol: targetSymbol, Mode: project.DeclarationEditReplaceSymbol,
		Index: index, MaxTokens: sessionFunctionContextLimit(s.functionRuntime.effective.ContextMaxTokens),
	})
	if err != nil {
		return nil, fmt.Errorf("build project context: %w", err)
	}
	manifest = s.contextManifestForRuntime(manifest, s.functionRuntime)
	messages, err := generationMessages(userPrompt, projectContext, indexedFile.Path, targetSymbol, scope)
	if err != nil {
		return nil, err
	}

	timed, cancel := context.WithTimeout(ctx, duration(s.functionRuntime.effective.Timeout))
	defer cancel()
	result, err := s.retry(timed, s.functionRuntime, messages)
	if timed.Err() != nil {
		return nil, timed.Err()
	}
	if err != nil {
		return nil, err
	}
	response, err := ParseGenerationResponse(result.Content, indexedFile.Path, targetSymbol, scope)
	if err != nil {
		return nil, err
	}
	if err := s.manager.ValidateMutableRequest(analysis.ProjectID, analysis.ProjectRevision, indexedFile.Path, fileInfo.ContentHash); err != nil {
		return nil, err
	}
	validation := project.ValidateGeneration(indexedFile.Path, fileInfo.Content, response.CandidateContent, selectedSymbol(*indexedFile, targetSymbol), scope)
	preview := &GenerationPreview{
		GenerationID:     newGenerationID(),
		Version:          generationResponseVersion,
		ProjectID:        analysis.ProjectID,
		ProjectRevision:  analysis.ProjectRevision,
		BaseFileHash:     fileInfo.ContentHash,
		TargetPath:       response.TargetPath,
		TargetSymbol:     response.TargetSymbol,
		ScopeMode:        response.ScopeMode,
		CandidateContent: response.CandidateContent,
		CandidateHash:    candidateHash(response.CandidateContent),
		Rationale:        response.Rationale,
		EffectiveModel:   s.EffectiveModel(),
		ContextManifest:  manifest,
		Validation:       validation,
	}
	if validation.Applicable {
		s.rememberCandidate(preview)
	}
	return preview, nil
}

func selectedSymbol(file project.IndexFile, name string) project.SymbolInfo {
	for _, symbol := range file.Symbols {
		if symbol.Name == name {
			return symbol
		}
	}
	return project.SymbolInfo{Name: name}
}

// ParseGenerationResponse accepts exactly one structured response, or one
// complete-file fenced block for compatible models. It never accepts prose,
// multiple blocks, extra response fields, or mismatched target metadata.
func ParseGenerationResponse(output, targetPath, targetSymbol string, scope workflow.ScopeMode) (GenerationResponse, error) {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return GenerationResponse{}, fmt.Errorf("generation response is empty")
	}
	if strings.Count(trimmed, "```") == 2 {
		block := fencedCandidate.FindStringSubmatch(trimmed)
		if len(block) != 2 {
			return GenerationResponse{}, fmt.Errorf("generation response must contain exactly one complete fenced code block")
		}
		return validGenerationResponse(GenerationResponse{
			Version: generationResponseVersion, TargetPath: targetPath, TargetSymbol: targetSymbol,
			ScopeMode: scope, CandidateContent: block[1],
		}, targetPath, targetSymbol, scope)
	}
	var response GenerationResponse
	decoder := json.NewDecoder(strings.NewReader(trimmed))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&response); err != nil {
		return GenerationResponse{}, fmt.Errorf("parse generation response: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return GenerationResponse{}, fmt.Errorf("generation response must contain one JSON object")
	}
	return validGenerationResponse(response, targetPath, targetSymbol, scope)
}

func validGenerationResponse(response GenerationResponse, targetPath, targetSymbol string, scope workflow.ScopeMode) (GenerationResponse, error) {
	if response.Version != generationResponseVersion {
		return GenerationResponse{}, fmt.Errorf("unsupported generation response version %q", response.Version)
	}
	if response.TargetPath != targetPath || response.TargetSymbol != targetSymbol || response.ScopeMode != scope {
		return GenerationResponse{}, fmt.Errorf("generation response target does not match the requested file, symbol, and scope")
	}
	if strings.TrimSpace(response.CandidateContent) == "" {
		return GenerationResponse{}, fmt.Errorf("generation response candidate_content is required")
	}
	if len(response.CandidateContent) > maxSemanticAnalysisBytes {
		return GenerationResponse{}, fmt.Errorf("generation response candidate_content exceeds the size limit")
	}
	return response, nil
}

func candidateHash(content string) string {
	sum := sha256.Sum256([]byte(content))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func newGenerationID() string {
	data := make([]byte, 16)
	if _, err := rand.Read(data); err != nil {
		return "generation-" + candidateHash(fmt.Sprintf("%p", &data))[7:]
	}
	return "generation-" + hex.EncodeToString(data)
}
