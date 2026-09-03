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
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

const (
	semanticAnalysisPromptVersion = "file-analysis-v2"
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

// AnalyzeFile creates or refreshes a semantic summary for exactly one selected
// file. It sends that file's source plus bounded, source-free project facts.
func (s *Service) AnalyzeFile(ctx context.Context, targetFile string, refresh, confirmRemoteProvider bool) (*project.FileAnalysis, error) {
	if err := s.RequireRemoteConfirmation(config.BugModelScope, confirmRemoteProvider); err != nil {
		return nil, err
	}
	indexedFile, err := s.manager.IndexedFile(targetFile)
	if err != nil {
		return nil, err
	}
	if indexedFile.Binary {
		return nil, fmt.Errorf("target file must be text")
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
	cache, input, err := s.fileAnalysisCacheInput(analysis, indexedFile, indexedFile.ContentHash)
	if err != nil {
		return nil, err
	}
	cached, err := cache.Load(input)
	if err != nil {
		return nil, err
	}
	if !refresh && (cached.Status == project.AnalysisStatusFresh || cached.Status == project.AnalysisStatusFailed || cached.Status == project.AnalysisStatusRunning) {
		if err := s.syncFileAnalysisStatus(input, cached.Status); err != nil {
			return nil, err
		}
		return cached, nil
	}
	index, err := s.manager.Index()
	if err != nil {
		return nil, err
	}
	prompt, err := semanticPrompt(fileInfo.Content, *analysis, index, *indexedFile, s.contextManifestForRuntime(semanticManifest(*indexedFile), s.bugRuntime))
	if err != nil {
		return nil, err
	}
	timed, cancel := context.WithTimeout(ctx, s.analysisTimeout)
	defer cancel()
	result, err := s.retry(timed, s.bugRuntime, prompt)
	if timed.Err() != nil {
		return nil, timed.Err()
	}
	if err != nil {
		return s.storeAnalysisFailure(cache, input, err)
	}
	parsed, err := parseSemanticAnalysis(result.Output, *indexedFile, fileInfo.Content)
	if err != nil {
		return s.storeAnalysisFailure(cache, input, err)
	}
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
	if model := result.Metadata["model"]; model != "" {
		fresh.Model = model
	}
	if err := cache.Store(fresh); err != nil {
		return nil, err
	}
	if err := s.syncFileAnalysisStatus(input, fresh.Status); err != nil {
		return nil, err
	}
	return &fresh, nil
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

// ClearFileAnalysis removes one selected file's semantic cache entry.
func (s *Service) ClearFileAnalysis(targetFile string) error {
	file, err := s.manager.IndexedFile(targetFile)
	if err != nil {
		return err
	}
	analysis, err := s.manager.Analysis()
	if err != nil {
		return err
	}
	cache, err := project.NewFileAnalysisCache(s.manager.Root())
	if err != nil {
		return err
	}
	if err := cache.Delete(targetFile); err != nil {
		return err
	}
	return s.syncFileAnalysisStatus(project.FileAnalysisInput{ProjectID: analysis.ProjectID, ProjectRevision: analysis.ProjectRevision, Path: file.Path}, project.AnalysisStatusMissing)
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
	runtime := s.bugRuntime
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
		"Required fields: purpose (string), responsibilities (string array), dependencies (string array), side_effects (string array), risks ({severity,summary,task_spec?} array), suggestions ({title,summary,target_symbol?,action?} array), symbol_explanations (object keyed only by supplied symbol names). " +
		"A task_spec is optional and must use schema_version \"1\", one supplied exact atomic symbol, acceptance_criteria and non_goals arrays, and may include go_test_candidate {name,content}. Never target another file. " +
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
	var parsed semanticAnalysisResponse
	if len(output) == 0 || len(output) > maxSemanticAnalysisBytes {
		return parsed, fmt.Errorf("semantic analysis response is empty or too large")
	}
	decoder := json.NewDecoder(strings.NewReader(output))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&parsed); err != nil {
		return parsed, fmt.Errorf("parse semantic analysis JSON: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return parsed, fmt.Errorf("semantic analysis JSON must contain one object")
	}
	if decoder.More() || strings.TrimSpace(parsed.Purpose) == "" || len(parsed.Responsibilities) > 32 || len(parsed.Dependencies) > 64 || len(parsed.SideEffects) > 32 || len(parsed.Risks) > 32 || len(parsed.Suggestions) > 32 {
		return parsed, fmt.Errorf("semantic analysis JSON is incomplete or exceeds limits")
	}
	allowed := make(map[string]bool, len(target.Symbols))
	shortNames := make(map[string][]string)
	for _, symbol := range target.Symbols {
		allowed[symbol.Name] = true
		if separator := strings.LastIndexByte(symbol.Name, '.'); separator >= 0 {
			shortName := symbol.Name[separator+1:]
			shortNames[shortName] = append(shortNames[shortName], symbol.Name)
		}
	}
	normalizedExplanations := make(map[string]string, len(parsed.SymbolExplanations))
	for name, explanation := range parsed.SymbolExplanations {
		normalizedName := name
		if !allowed[name] {
			candidates := shortNames[name]
			if len(candidates) != 1 {
				return parsed, fmt.Errorf("semantic analysis contains an unknown or ambiguous symbol explanation")
			}
			normalizedName = candidates[0]
		}
		explanation = strings.TrimSpace(explanation)
		if explanation == "" {
			return parsed, fmt.Errorf("semantic analysis contains an unknown or empty symbol explanation")
		}
		if _, exists := normalizedExplanations[normalizedName]; exists {
			return parsed, fmt.Errorf("semantic analysis contains duplicate symbol explanations")
		}
		normalizedExplanations[normalizedName] = explanation
	}
	parsed.SymbolExplanations = normalizedExplanations
	for index := range parsed.Risks {
		risk := &parsed.Risks[index]
		risk.Severity = strings.ToLower(strings.TrimSpace(risk.Severity))
		risk.Summary = strings.TrimSpace(risk.Summary)
		if risk.Severity != "low" && risk.Severity != "medium" && risk.Severity != "high" || risk.Summary == "" {
			return parsed, fmt.Errorf("semantic analysis contains an invalid risk")
		}
		taskSpec, err := validateBugTaskSpec(risk.TaskSpec, target, source)
		if err != nil {
			return parsed, err
		}
		risk.TaskSpec = taskSpec
	}
	return parsed, nil
}

func validateBugTaskSpec(spec *project.BugTaskSpec, target project.IndexFile, source string) (*project.BugTaskSpec, error) {
	if spec == nil {
		return nil, nil
	}
	if spec.SchemaVersion != project.BugTaskSpecSchemaVersion || len(spec.TargetPath) > 1024 || len(spec.TargetSymbol) > project.MaxBugTaskItemBytes || len(spec.TargetSignature) > 1024 || spec.TargetPath != target.Path || strings.TrimSpace(spec.TargetSymbol) == "" {
		return nil, fmt.Errorf("semantic analysis contains an invalid bug task target")
	}
	if err := validateBugTaskItems(spec.AcceptanceCriteria, true); err != nil {
		return nil, err
	}
	if err := validateBugTaskItems(spec.NonGoals, false); err != nil {
		return nil, err
	}
	matching := make([]project.SymbolInfo, 0, 1)
	for _, symbol := range target.Symbols {
		if symbol.Name == spec.TargetSymbol {
			matching = append(matching, symbol)
		}
	}
	if len(matching) != 1 || matching[0].Confidence != "exact" || !matching[0].AtomicTarget || matching[0].Signature == "" {
		return nil, fmt.Errorf("semantic analysis bug task target is not one exact atomic symbol")
	}
	validated := project.SanitizeBugTaskSpec(spec)
	validated.TargetPath = target.Path
	validated.TargetSymbol = matching[0].Name
	validated.TargetSignature = matching[0].Signature
	if validated.GoTestCandidate != nil {
		validated.GoTestCandidate = validateGoTestCandidate(validated.GoTestCandidate, target, source)
	}
	return validated, nil
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
