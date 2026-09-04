package app

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"strings"
	"time"
	"unicode"

	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/llm"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

const (
	semanticAnalysisPromptVersion = "file-analysis-v3"
	maxSemanticAnalysisBytes      = 64 * 1024
)

type semanticAnalysisResponse struct {
	Purpose            string               `json:"purpose"`
	Responsibilities   []string             `json:"responsibilities"`
	Dependencies       []string             `json:"dependencies"`
	SideEffects        []string             `json:"side_effects"`
	Risks              []project.Finding    `json:"risks"`
	Suggestions        []project.Suggestion `json:"suggestions"`
	SymbolExplanations map[string]string    `json:"symbol_explanations"`
}

// semanticAnalysisWireResponse keeps the optional task_spec untrusted until its
// target has been checked against the indexed file. A malformed suggestion must
// not discard the rest of an otherwise usable file summary.
type semanticAnalysisWireResponse struct {
	Purpose            string                    `json:"purpose"`
	Responsibilities   []string                  `json:"responsibilities"`
	Dependencies       []string                  `json:"dependencies"`
	SideEffects        []string                  `json:"side_effects"`
	Risks              []semanticAnalysisFinding `json:"risks"`
	Suggestions        []project.Suggestion      `json:"suggestions"`
	SymbolExplanations map[string]string         `json:"symbol_explanations"`
}

type semanticAnalysisFinding struct {
	Severity string          `json:"severity"`
	Summary  string          `json:"summary"`
	TaskSpec json.RawMessage `json:"task_spec"`
}

type preparedFileAnalysis struct {
	analysis    *project.Analysis
	indexedFile *project.IndexFile
	source      string
	cache       *project.FileAnalysisCache
	input       project.FileAnalysisInput
}

// AnalyzeFile creates or refreshes a semantic summary for exactly one selected
// file. It sends that file's source plus bounded, source-free project facts.
func (s *Service) AnalyzeFile(ctx context.Context, targetFile string, refresh, confirmRemoteProvider bool) (*project.FileAnalysis, error) {
	if err := s.RequireRemoteConfirmation(config.BugModelScope, confirmRemoteProvider); err != nil {
		return nil, err
	}
	prepared, err := s.prepareFileAnalysis(targetFile)
	if err != nil {
		return nil, err
	}
	cached, err := prepared.cache.Load(prepared.input)
	if err != nil {
		return nil, err
	}
	if !refresh && reusableFileAnalysis(cached) {
		return s.syncCachedFileAnalysis(prepared.input, cached)
	}
	return s.generateFileAnalysis(ctx, prepared)
}

func (s *Service) prepareFileAnalysis(targetFile string) (preparedFileAnalysis, error) {
	indexedFile, err := s.manager.IndexedFile(targetFile)
	if err != nil {
		return preparedFileAnalysis{}, err
	}
	if indexedFile.Binary {
		return preparedFileAnalysis{}, fmt.Errorf("target file must be text")
	}
	analysis, err := s.manager.Analysis()
	if err != nil {
		return preparedFileAnalysis{}, err
	}
	fileInfo, err := project.GetFileInfo(s.manager.Root(), indexedFile.Path)
	if err != nil {
		return preparedFileAnalysis{}, err
	}
	if fileInfo.ContentHash != indexedFile.ContentHash {
		return preparedFileAnalysis{}, project.ErrRevisionConflict
	}
	cache, input, err := s.fileAnalysisCacheInput(analysis, indexedFile, indexedFile.ContentHash)
	if err != nil {
		return preparedFileAnalysis{}, err
	}
	return preparedFileAnalysis{analysis: analysis, indexedFile: indexedFile, source: fileInfo.Content, cache: cache, input: input}, nil
}

func reusableFileAnalysis(cached *project.FileAnalysis) bool {
	return cached.Status == project.AnalysisStatusFresh || cached.Status == project.AnalysisStatusFailed || cached.Status == project.AnalysisStatusRunning
}

func (s *Service) syncCachedFileAnalysis(input project.FileAnalysisInput, cached *project.FileAnalysis) (*project.FileAnalysis, error) {
	if err := s.syncFileAnalysisStatus(input, cached.Status); err != nil {
		return nil, err
	}
	return cached, nil
}

func (s *Service) generateFileAnalysis(ctx context.Context, prepared preparedFileAnalysis) (*project.FileAnalysis, error) {
	index, err := s.manager.Index()
	if err != nil {
		return nil, err
	}
	runtime := s.runtimes.bug
	prompt, err := semanticPrompt(prepared.source, *prepared.analysis, index, *prepared.indexedFile, s.contextManifestForRuntime(semanticManifest(*prepared.indexedFile), runtime))
	if err != nil {
		return nil, err
	}
	timed, cancel := context.WithTimeout(ctx, s.analysisTimeout)
	defer cancel()
	result, err := s.retry(timed, runtime, []llm.ChatMessage{{Role: "user", Content: prompt}})
	if timed.Err() != nil {
		return nil, timed.Err()
	}
	if err != nil {
		return s.storeAnalysisFailure(prepared.cache, prepared.input, err)
	}
	parsed, err := parseSemanticAnalysis(result.Content, *prepared.indexedFile, prepared.source)
	if err != nil {
		return s.storeAnalysisFailure(prepared.cache, prepared.input, err)
	}
	fresh := newFileAnalysis(prepared, parsed)
	if model := result.Model; model != "" {
		fresh.Model = model
	}
	if err := prepared.cache.Store(fresh); err != nil {
		return nil, err
	}
	if err := s.syncFileAnalysisStatus(prepared.input, fresh.Status); err != nil {
		return nil, err
	}
	return &fresh, nil
}

func newFileAnalysis(prepared preparedFileAnalysis, parsed semanticAnalysisResponse) project.FileAnalysis {
	input, indexedFile := prepared.input, prepared.indexedFile
	fresh := project.FileAnalysis{
		SchemaVersion: "1", ProjectID: input.ProjectID, ProjectRevision: input.ProjectRevision,
		Path: input.Path, ContentHash: input.ContentHash, Language: input.Language,
		Purpose: parsed.Purpose, Responsibilities: parsed.Responsibilities, Symbols: append([]project.SymbolInfo(nil), indexedFile.Symbols...),
		Imports: append([]string(nil), indexedFile.Imports...), Dependencies: parsed.Dependencies,
		SideEffects: parsed.SideEffects, Risks: parsed.Risks, Suggestions: parsed.Suggestions,
		SymbolExplanations: parsed.SymbolExplanations, Status: project.AnalysisStatusFresh,
		Model: input.Model, ConfiguredModel: input.Model, Profile: input.Profile, Scope: input.Scope, ProviderOrigin: input.ProviderOrigin, ReasoningEffort: input.ReasoningEffort, PromptVersion: input.PromptVersion, ContextPolicyVersion: input.ContextPolicyVersion,
		GeneratedAt: time.Now().UTC(),
	}
	return fresh
}

// CachedFileAnalysis returns the existing semantic state without contacting the model.
func (s *Service) CachedFileAnalysis(targetFile string) (*project.FileAnalysis, error) {
	indexedFile, err := s.manager.IndexedFile(targetFile)
	if err != nil {
		return nil, err
	}
	analysis, err := s.manager.Analysis()
	if err != nil {
		return nil, err
	}
	info, err := project.GetFileInfo(s.manager.Root(), indexedFile.Path)
	if err != nil {
		return nil, err
	}
	cache, input, err := s.fileAnalysisCacheInput(analysis, indexedFile, info.ContentHash)
	if err != nil {
		return nil, err
	}
	cached, err := cache.Load(input)
	if err != nil {
		return nil, err
	}
	if err := s.syncFileAnalysisStatus(input, cached.Status); err != nil {
		return nil, err
	}
	return cached, nil
}

func (s *Service) fileAnalysisCacheInput(analysis *project.Analysis, file *project.IndexFile, contentHash string) (*project.FileAnalysisCache, project.FileAnalysisInput, error) {
	policy, err := project.NewContextPolicy(s.manager.Root())
	if err != nil {
		return nil, project.FileAnalysisInput{}, err
	}
	cache, err := project.NewFileAnalysisCache(s.manager.Root())
	if err != nil {
		return nil, project.FileAnalysisInput{}, err
	}
	runtime := s.runtimes.bug
	return cache, project.FileAnalysisInput{ProjectID: analysis.ProjectID, ProjectRevision: analysis.ProjectRevision, Path: file.Path, ContentHash: contentHash, Language: file.Language, Model: runtime.profile.Model, Profile: runtime.effective.Profile, Scope: runtime.effective.Scope, ProviderOrigin: runtime.effective.ProviderOrigin, ReasoningEffort: runtime.effective.ReasoningEffort, PromptVersion: semanticAnalysisPromptVersion, ContextPolicyVersion: policy.Version()}, nil
}

func (s *Service) syncFileAnalysisStatus(input project.FileAnalysisInput, status string) error {
	return s.manager.UpdateFileAnalysisStatus(input.ProjectID, input.ProjectRevision, input.Path, status)
}

func (s *Service) storeAnalysisFailure(cache *project.FileAnalysisCache, input project.FileAnalysisInput, cause error) (*project.FileAnalysis, error) {
	if strings.Contains(cause.Error(), "context canceled") || strings.Contains(cause.Error(), "deadline exceeded") {
		return nil, cause
	}
	failed := project.FileAnalysis{SchemaVersion: "1", ProjectID: input.ProjectID, ProjectRevision: input.ProjectRevision, Path: input.Path, ContentHash: input.ContentHash, Language: input.Language, Status: project.AnalysisStatusFailed, Failure: "The model returned an unusable file summary. Retry the analysis.", Model: input.Model, ConfiguredModel: input.Model, Profile: input.Profile, Scope: input.Scope, ProviderOrigin: input.ProviderOrigin, ReasoningEffort: input.ReasoningEffort, PromptVersion: input.PromptVersion, ContextPolicyVersion: input.ContextPolicyVersion, GeneratedAt: time.Now().UTC()}
	if err := cache.Store(failed); err != nil {
		return nil, err
	}
	if err := s.syncFileAnalysisStatus(input, failed.Status); err != nil {
		return nil, err
	}
	return &failed, nil
}

func semanticPrompt(source string, analysis project.Analysis, index *project.ProjectIndex, target project.IndexFile, manifest project.ContextManifest) (string, error) {
	if len(source) > maxSemanticAnalysisBytes {
		return "", fmt.Errorf("selected file is too large for semantic analysis")
	}
	projectFacts := struct {
		Name       string   `json:"name"`
		Type       string   `json:"type"`
		Summary    string   `json:"summary"`
		Signatures []string `json:"relevant_signatures"`
	}{Name: analysis.Name, Type: analysis.Type, Summary: analysis.Summary, Signatures: boundedSignatures(index, target.Path)}
	facts, err := json.Marshal(projectFacts)
	if err != nil {
		return "", err
	}
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		return "", err
	}
	targetJSON, err := json.Marshal(struct {
		Path     string               `json:"path"`
		Language string               `json:"language"`
		Imports  []string             `json:"imports"`
		Symbols  []project.SymbolInfo `json:"symbols"`
	}{target.Path, target.Language, target.Imports, target.Symbols})
	if err != nil {
		return "", err
	}
	return "You summarize exactly one selected source file. Return one JSON object only; do not use Markdown or code fences. " +
		"Required fields: purpose (string), responsibilities (string array), dependencies (string array), side_effects (string array), risks ({severity,summary,task_spec?} array), suggestions ({title,summary,target_symbol?,action?} array), symbol_explanations (object keyed only by supplied symbol names). Keep each array to at most three concise items. " +
		"Usually omit task_spec. If you include one, it must have only these fields: schema_version \"1\", target_path copied exactly from TARGET_FACTS.path, target_symbol copied exactly from one exact atomic TARGET_FACTS.symbols name, target_signature copied exactly from that symbol's TARGET_FACTS signature, acceptance_criteria (array), non_goals (array), and optional go_test_candidate {name,content}. Do not use a symbol field. Never target another file. " +
		"Treat all interpretations as suggestions. Do not quote source wholesale, invent files, or include source from another file.\n\n" +
		"PROJECT_FACTS:\n" + string(facts) + "\n\nCONTEXT_MANIFEST:\n" + string(manifestJSON) + "\n\nTARGET_FACTS:\n" + string(targetJSON) + "\n\nTARGET_SOURCE (the only source content supplied):\n```\n" + source + "\n```\n", nil
}

func semanticManifest(target project.IndexFile) project.ContextManifest {
	tokens := int((target.SizeBytes + 3) / 4)
	return project.ContextManifest{Included: []project.ContextFile{{Path: target.Path, SizeBytes: target.SizeBytes, Hash: target.ContentHash, Tokens: tokens}}, Excluded: []project.ContextDecision{}, EstimatedTokens: tokens, ByteLimit: maxSemanticAnalysisBytes, TokenLimit: maxSemanticAnalysisBytes / 4}
}

func boundedSignatures(index *project.ProjectIndex, targetPath string) []string {
	const limit = 64
	result := make([]string, 0, limit)
	for _, file := range index.Files {
		for _, symbol := range file.Symbols {
			if file.Path == targetPath || len(result) < limit {
				result = append(result, file.Path+": "+symbol.Signature)
			}
			if len(result) == limit {
				return result
			}
		}
	}
	return result
}

func parseSemanticAnalysis(output string, target project.IndexFile, source string) (semanticAnalysisResponse, error) {
	var wire semanticAnalysisWireResponse
	if len(output) == 0 || len(output) > maxSemanticAnalysisBytes {
		return semanticAnalysisResponse{}, fmt.Errorf("semantic analysis response is empty or too large")
	}
	if err := decodeSemanticAnalysis(output, &wire); err != nil {
		return semanticAnalysisResponse{}, err
	}
	if err := validateSemanticAnalysisLimits(wire); err != nil {
		return semanticAnalysisResponse{}, err
	}
	explanations, err := normalizeSymbolExplanations(wire.SymbolExplanations, target.Symbols)
	if err != nil {
		return semanticAnalysisResponse{}, err
	}
	risks, err := parseSemanticRisks(wire.Risks, target, source)
	if err != nil {
		return semanticAnalysisResponse{}, err
	}
	return semanticAnalysisResponse{
		Purpose:            wire.Purpose,
		Responsibilities:   wire.Responsibilities,
		Dependencies:       wire.Dependencies,
		SideEffects:        wire.SideEffects,
		Risks:              risks,
		Suggestions:        wire.Suggestions,
		SymbolExplanations: explanations,
	}, nil
}

func decodeSemanticAnalysis(output string, parsed *semanticAnalysisWireResponse) error {
	decoder := json.NewDecoder(strings.NewReader(output))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(parsed); err != nil {
		return fmt.Errorf("parse semantic analysis JSON: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("semantic analysis JSON must contain one object")
	}
	return nil
}

func validateSemanticAnalysisLimits(parsed semanticAnalysisWireResponse) error {
	if strings.TrimSpace(parsed.Purpose) == "" || len(parsed.Responsibilities) > 32 || len(parsed.Dependencies) > 64 || len(parsed.SideEffects) > 32 || len(parsed.Risks) > 32 || len(parsed.Suggestions) > 32 {
		return fmt.Errorf("semantic analysis JSON is incomplete or exceeds limits")
	}
	return nil
}

func normalizeSymbolExplanations(explanations map[string]string, symbols []project.SymbolInfo) (map[string]string, error) {
	allowed := make(map[string]bool, len(symbols))
	shortNames := make(map[string][]string)
	for _, symbol := range symbols {
		allowed[symbol.Name] = true
		if separator := strings.LastIndexByte(symbol.Name, '.'); separator >= 0 {
			shortName := symbol.Name[separator+1:]
			shortNames[shortName] = append(shortNames[shortName], symbol.Name)
		}
	}
	normalizedExplanations := make(map[string]string, len(explanations))
	for name, explanation := range explanations {
		normalizedName := name
		if !allowed[name] {
			candidates := shortNames[name]
			if len(candidates) != 1 {
				return nil, fmt.Errorf("semantic analysis contains an unknown or ambiguous symbol explanation")
			}
			normalizedName = candidates[0]
		}
		explanation = strings.TrimSpace(explanation)
		if explanation == "" {
			return nil, fmt.Errorf("semantic analysis contains an unknown or empty symbol explanation")
		}
		if _, exists := normalizedExplanations[normalizedName]; exists {
			return nil, fmt.Errorf("semantic analysis contains duplicate symbol explanations")
		}
		normalizedExplanations[normalizedName] = explanation
	}
	return normalizedExplanations, nil
}

func parseSemanticRisks(risks []semanticAnalysisFinding, target project.IndexFile, source string) ([]project.Finding, error) {
	parsed := make([]project.Finding, 0, len(risks))
	for _, risk := range risks {
		finding := project.Finding{
			Severity: strings.ToLower(strings.TrimSpace(risk.Severity)),
			Summary:  strings.TrimSpace(risk.Summary),
		}
		if finding.Severity != "low" && finding.Severity != "medium" && finding.Severity != "high" || finding.Summary == "" {
			return nil, fmt.Errorf("semantic analysis contains an invalid risk")
		}
		finding.TaskSpec = parseOptionalBugTaskSpec(risk.TaskSpec, target, source)
		parsed = append(parsed, finding)
	}
	return parsed, nil
}

func parseOptionalBugTaskSpec(raw json.RawMessage, target project.IndexFile, source string) *project.BugTaskSpec {
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "null" {
		return nil
	}
	var spec project.BugTaskSpec
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&spec); err != nil {
		return nil
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil
	}
	validated, err := validateBugTaskSpec(&spec, target, source)
	if err != nil {
		return nil
	}
	return validated
}

func validateBugTaskSpec(spec *project.BugTaskSpec, target project.IndexFile, source string) (*project.BugTaskSpec, error) {
	if spec == nil {
		return nil, nil
	}
	if !validBugTaskTarget(spec, target.Path) {
		return nil, fmt.Errorf("semantic analysis contains an invalid bug task target")
	}
	if err := validateBugTaskItems(spec.AcceptanceCriteria, true); err != nil {
		return nil, err
	}
	if err := validateBugTaskItems(spec.NonGoals, false); err != nil {
		return nil, err
	}
	matching, ok := exactAtomicSymbol(target.Symbols, spec.TargetSymbol)
	if !ok {
		return nil, fmt.Errorf("semantic analysis bug task target is not one exact atomic symbol")
	}
	validated := project.SanitizeBugTaskSpec(spec)
	validated.TargetPath = target.Path
	validated.TargetSymbol = matching.Name
	validated.TargetSignature = matching.Signature
	if validated.GoTestCandidate != nil {
		validated.GoTestCandidate = validateGoTestCandidate(validated.GoTestCandidate, target, source)
	}
	return validated, nil
}

func validBugTaskTarget(spec *project.BugTaskSpec, targetPath string) bool {
	return spec.SchemaVersion == project.BugTaskSpecSchemaVersion && len(spec.TargetPath) <= 1024 &&
		len(spec.TargetSymbol) <= project.MaxBugTaskItemBytes && len(spec.TargetSignature) <= 1024 &&
		spec.TargetPath == targetPath && strings.TrimSpace(spec.TargetSymbol) != ""
}

func exactAtomicSymbol(symbols []project.SymbolInfo, name string) (project.SymbolInfo, bool) {
	matching := make([]project.SymbolInfo, 0, 1)
	for _, symbol := range symbols {
		if symbol.Name == name {
			matching = append(matching, symbol)
		}
	}
	if len(matching) != 1 || matching[0].Confidence != "exact" || !matching[0].AtomicTarget || matching[0].Signature == "" {
		return project.SymbolInfo{}, false
	}
	return matching[0], true
}

func validateBugTaskItems(items []string, required bool) error {
	if len(items) > project.MaxBugTaskItems || required && len(items) == 0 {
		return fmt.Errorf("semantic analysis bug task items exceed limits")
	}
	for _, item := range items {
		if strings.TrimSpace(item) == "" || len(item) > project.MaxBugTaskItemBytes {
			return fmt.Errorf("semantic analysis bug task item is invalid")
		}
	}
	return nil
}

func validateGoTestCandidate(candidate *project.GoTestCandidateSpec, target project.IndexFile, source string) *project.GoTestCandidateSpec {
	if candidate == nil || target.Language != "Go" || len(candidate.Name) > project.MaxBugTaskItemBytes || len(candidate.Content) > project.MaxBugTaskCandidateBytes || !validGoTestName(candidate.Name) {
		return nil
	}
	fset := token.NewFileSet()
	targetFile, err := parser.ParseFile(fset, target.Path, source, parser.PackageClauseOnly)
	if err != nil {
		return nil
	}
	testFile, err := parser.ParseFile(fset, candidate.Name+"_test.go", candidate.Content, parser.AllErrors)
	if err != nil || testFile.Name == nil || testFile.Name.Name != targetFile.Name.Name {
		return nil
	}
	count := 0
	for _, declaration := range testFile.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if ok && function.Recv == nil && function.Name.Name == candidate.Name {
			count++
		}
	}
	if count != 1 {
		return nil
	}
	copy := *candidate
	return &copy
}

func validGoTestName(name string) bool {
	runes := []rune(name)
	return len(runes) > len("Test") && strings.HasPrefix(name, "Test") && unicode.IsUpper(runes[len("Test")])
}
