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
	semanticAnalysisPromptVersion = "file-analysis-v14"
	maxSemanticAnalysisBytes      = 64 * 1024
	fileAnalysisInsightFieldCount = 4
	fileAnalysisInsightMaxChars   = 250
)

const fileAnalysisResponseSchemaName = "file_analysis_response"

// fileAnalysisResponseSchemaDocument mirrors the wire response accepted by
// parseSemanticAnalysis. Target-specific checks (such as an exact symbol name)
// remain in the parser because they depend on the selected file.
const fileAnalysisResponseSchemaDocument = `{
  "type":"object",
  "additionalProperties":false,
  "required":["purpose","responsibilities","dependencies","side_effects","risks","suggestions","symbol_explanations"],
  "properties":{
    "purpose":{"type":"string","minLength":1},
    "responsibilities":{"type":"array","maxItems":3,"items":{"type":"string","minLength":1}},
    "dependencies":{"type":"array","maxItems":3,"items":{"type":"string","minLength":1}},
    "side_effects":{"type":"array","maxItems":3,"items":{"type":"string","minLength":1}},
    "risks":{"type":"array","maxItems":3,"items":{"$ref":"#/$defs/risk"}},
    "suggestions":{"type":"array","maxItems":3,"items":{"$ref":"#/$defs/suggestion"}},
    "symbol_explanations":{"type":"object","additionalProperties":{"type":"string","minLength":1}},
    "engineering_insight":{"$ref":"#/$defs/optionalInsight"}
  },
  "$defs":{
    "insight":{
      "type":"object",
      "additionalProperties":false,
      "required":["mechanism","why_it_matters_here","tradeoff_or_failure_mode","transferable_lesson"],
      "properties":{
        "mechanism":{"$ref":"#/$defs/insightText"},
        "why_it_matters_here":{"$ref":"#/$defs/insightText"},
        "tradeoff_or_failure_mode":{"$ref":"#/$defs/insightText"},
        "transferable_lesson":{"$ref":"#/$defs/insightText"}
      }
    },
    "insightText":{"type":"string","minLength":1,"maxLength":250},
    "optionalInsight":{"anyOf":[{"$ref":"#/$defs/insight"},{"type":"null"}]},
    "taskSpec":{
      "type":"object",
      "additionalProperties":false,
      "required":["schema_version","target_path","target_symbol","target_signature","acceptance_criteria","non_goals"],
      "properties":{
        "schema_version":{"enum":["1"]},
        "target_path":{"type":"string","minLength":1,"maxLength":1024},
        "target_symbol":{"type":"string","minLength":1,"maxLength":512},
        "target_signature":{"type":"string","minLength":1,"maxLength":1024},
        "acceptance_criteria":{"type":"array","minItems":1,"maxItems":8,"items":{"type":"string","minLength":1,"maxLength":512}},
        "non_goals":{"type":"array","maxItems":8,"items":{"type":"string","minLength":1,"maxLength":512}},
        "go_test_candidate":{"anyOf":[{"$ref":"#/$defs/goTestCandidate"},{"type":"null"}]}
      }
    },
    "goTestCandidate":{
      "type":"object",
      "additionalProperties":false,
      "required":["name","content"],
      "properties":{"name":{"type":"string","minLength":1,"maxLength":512},"content":{"type":"string","minLength":1,"maxLength":16384}}
    },
    "risk":{
      "type":"object",
      "additionalProperties":false,
      "required":["category","severity","summary"],
      "properties":{
        "category":{"enum":["bugs","performance","security"]},
        "severity":{"enum":["low","medium","high"]},
        "summary":{"type":"string","minLength":1},
        "task_spec":{"anyOf":[{"$ref":"#/$defs/taskSpec"},{"type":"null"}]},
        "engineering_insight":{"$ref":"#/$defs/optionalInsight"}
      }
    },
    "suggestion":{
      "type":"object",
      "additionalProperties":false,
      "required":["title","summary"],
      "properties":{
        "title":{"type":"string","minLength":1},
        "summary":{"type":"string","minLength":1},
        "target_symbol":{"type":"string","minLength":1},
        "action":{"type":"string","minLength":1},
        "engineering_insight":{"$ref":"#/$defs/optionalInsight"}
      }
    }
  }
}`

const fileAnalysisInsightGuidance = "Ground selected-file behavior and engineering insights in TARGET_SOURCE. Use TARGET_FACTS, PROJECT_FACTS, and CONTEXT_MANIFEST only for the identity and context they supply. Apply these factual boundaries to the whole final response, including purpose, risks, suggestions, explanations, task specs, and engineering insights. " +
	"State visible behavior as fact only when TARGET_SOURCE demonstrates it. Separate local guarantees from downstream behavior that this file cannot establish, and state conditional concerns as conditions rather than facts. You may explain consequences demonstrated by visible local control or data flow. Do not infer unshown downstream safeguards, callers, implementations, or consequences; do not invent files or source. " +
	"Use the shared four-field contract to explain an observable relationship or invariant, why it matters locally, a real limitation or failure mode, and a concrete test with expected observations. An engineering insight can describe a correct existing behavior and does not require a proposed change or discovered defect. " +
	"Default to tests that characterize current behavior: state what the code currently does and the expected observation. If you suggest a proposed-change test, label it proposed-change and first state the current behavior it would change; do not portray a failing proposed expectation as a present contract. " +
	"Prefer one concise file-level insight where it is genuinely useful. Intentionally omit it for trivial code without a meaningful observable relationship or when the evidence cannot support a specific explanation; do not give generic tests or advice. Do not duplicate that lesson in risks or suggestions. "

const fileAnalysisInsightSchema = "At every supported engineering_insight location (the top level, each risks[] item, and each suggestions[] item), either omit engineering_insight or use null, or provide exactly one object with exactly these four non-empty string fields and no other keys: mechanism, why_it_matters_here, tradeoff_or_failure_mode, and transferable_lesson. Each field is limited to 250 Unicode characters, intentionally keeping generated insight prose concise; the four fields together fit the parser's 1,000-normalized-rune budget. Shape: {\"mechanism\":\"...\",\"why_it_matters_here\":\"...\",\"tradeoff_or_failure_mode\":\"...\",\"transferable_lesson\":\"...\"}. Never use a string, array, or partial object. "

// EngineeringInsightPromptVersion returns the production selected-file prompt
// identity used by evaluation; callers cannot supply an unrelated label.
func EngineeringInsightPromptVersion() string { return semanticAnalysisPromptVersion }

// FileAnalysisResponseSchema returns the one strict structured-output contract
// shared by production selected-file analysis and its evaluation runner.
func FileAnalysisResponseSchema() llm.JSONSchema {
	return llm.JSONSchema{Name: fileAnalysisResponseSchemaName, Schema: json.RawMessage(fileAnalysisResponseSchemaDocument)}
}

type semanticAnalysisResponse struct {
	Diagnostics        FileAnalysisDiagnostics     `json:"-"`
	Purpose            string                      `json:"purpose"`
	Responsibilities   []string                    `json:"responsibilities"`
	Dependencies       []string                    `json:"dependencies"`
	SideEffects        []string                    `json:"side_effects"`
	Risks              []project.Finding           `json:"risks"`
	Suggestions        []project.Suggestion        `json:"suggestions"`
	SymbolExplanations map[string]string           `json:"symbol_explanations"`
	EngineeringInsight *project.EngineeringInsight `json:"engineering_insight,omitempty"`
}

// semanticAnalysisWireResponse keeps the optional task_spec untrusted until its
// target has been checked against the indexed file. A malformed suggestion must
// not discard the rest of an otherwise usable file summary.
type semanticAnalysisWireResponse struct {
	Purpose            string                       `json:"purpose"`
	Responsibilities   []string                     `json:"responsibilities"`
	Dependencies       []string                     `json:"dependencies"`
	SideEffects        []string                     `json:"side_effects"`
	Risks              []semanticAnalysisFinding    `json:"risks"`
	Suggestions        []semanticAnalysisSuggestion `json:"suggestions"`
	SymbolExplanations map[string]string            `json:"symbol_explanations"`
	EngineeringInsight json.RawMessage              `json:"engineering_insight"`
}

type semanticAnalysisFinding struct {
	Category project.FindingCategory `json:"category"`
	Severity string                  `json:"severity"`
	Summary  string                  `json:"summary"`
	TaskSpec json.RawMessage         `json:"task_spec"`
	Insight  json.RawMessage         `json:"engineering_insight"`
}

type semanticAnalysisSuggestion struct {
	Title        string          `json:"title"`
	Summary      string          `json:"summary"`
	TargetSymbol string          `json:"target_symbol"`
	Action       string          `json:"action"`
	Insight      json.RawMessage `json:"engineering_insight"`
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
	result, err := s.retryWithJSONSchema(timed, runtime, []llm.ChatMessage{{Role: "user", Content: prompt}}, FileAnalysisResponseSchema())
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
		SideEffects: parsed.SideEffects, Risks: parsed.Risks, Suggestions: parsed.Suggestions, EngineeringInsight: project.CloneEngineeringInsight(parsed.EngineeringInsight),
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
		"Required fields: purpose (string), responsibilities (string array), dependencies (string array), side_effects (string array), risks ({category,severity,summary,task_spec?,engineering_insight?} array), suggestions ({title,summary,target_symbol?,action?,engineering_insight?} array), symbol_explanations (object keyed only by supplied symbol names), engineering_insight? (see the shared engineering insight contract below). Keep each array to at most three concise items. " +
		fileAnalysisInsightSchema + project.EngineeringInsightPromptInstructions +
		"For symbol_explanations, copy keys verbatim from TARGET_FACTS.symbols[].name. Do not explain parameters, local variables, fields, imported names, or referenced types unless their exact name appears in that list. An empty object is valid. Risk severity must be low, medium, or high. " +
		fileAnalysisInsightGuidance +
		"Every risk must have exactly one category: bugs for incorrect behavior, performance for avoidable resource costs, or security for a trust-boundary weakness. Choose the category from the demonstrated failure mechanism, independently of severity. State the relevant local control/data flow and conditions in summary. Performance concerns are hypotheses unless measurements are supplied; never invent measured impact. For security, identify the visible input and sensitive operation; do not infer missing safeguards in an unseen callee. Omit concerns unsupported by TARGET_SOURCE. General cleanup, explanations and best-practice advice belong in suggestions, not risks; an empty risks array is valid. " +
		"Keep each insight field at or below 250 Unicode characters. Generic advice to use defer, handle errors, or follow best practices is not an insight. " +
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
	return validateSemanticAnalysisResponse(output, target, source, false)
}

func validateSemanticAnalysisResponse(output string, target project.IndexFile, source string, collectRejectedParentDiagnostics bool) (semanticAnalysisResponse, error) {
	var sizeErr error
	if len(output) == 0 || len(output) > maxSemanticAnalysisBytes {
		sizeErr = fmt.Errorf("semantic analysis response is empty or too large")
		if !collectRejectedParentDiagnostics {
			return semanticAnalysisResponse{}, sizeErr
		}
	}
	var wire semanticAnalysisWireResponse
	decodeErr := decodeSemanticAnalysis(output, &wire)
	if decodeErr != nil {
		if sizeErr != nil {
			return semanticAnalysisResponse{}, sizeErr
		}
		return semanticAnalysisResponse{}, decodeErr
	}
	parentErr := sizeErr
	if parentErr == nil {
		parentErr = validateSemanticAnalysisLimits(wire)
	}
	if parentErr != nil && !collectRejectedParentDiagnostics {
		return semanticAnalysisResponse{}, parentErr
	}
	// Evaluation historically records bounded field metadata for any decoded
	// parent, including parents rejected by size or collection limits. Normal
	// production requests retain their cheap rejection before section parsing.
	parsed := parseSemanticSections(wire, target, source)
	if parentErr != nil {
		return semanticAnalysisResponse{Diagnostics: parsed.Diagnostics}, parentErr
	}

	for _, risk := range parsed.Risks {
		if !risk.Category.Valid() || risk.Severity != "low" && risk.Severity != "medium" && risk.Severity != "high" || risk.Summary == "" {
			return semanticAnalysisResponse{Diagnostics: parsed.Diagnostics}, fmt.Errorf("semantic analysis contains an invalid risk")
		}
	}
	limitFileAnalysisInsights(&parsed.EngineeringInsight, parsed.Risks, parsed.Suggestions)
	return parsed, nil
}

func parseSemanticSections(wire semanticAnalysisWireResponse, target project.IndexFile, source string) semanticAnalysisResponse {
	parsed := semanticAnalysisResponse{
		Purpose: wire.Purpose, Responsibilities: wire.Responsibilities,
		Dependencies: wire.Dependencies, SideEffects: wire.SideEffects,
		Diagnostics: FileAnalysisDiagnostics{Insights: make([]FileAnalysisInsightDiagnostic, 0, 1+len(wire.Risks)+len(wire.Suggestions))},
	}
	var err error
	parsed.SymbolExplanations, err = normalizeSymbolExplanations(wire.SymbolExplanations, target.Symbols)
	if err != nil {
		// Optional explanations cannot grant target authority. Omit the entire
		// section on an invalid key instead of losing an otherwise valid summary.
		parsed.SymbolExplanations = map[string]string{}
		parsed.Diagnostics.SymbolExplanationsDegraded = true
	}
	parsed.EngineeringInsight = parsed.Diagnostics.parseInsight("top_level", nil, wire.EngineeringInsight)
	parsed.Risks = parseSemanticRisks(wire.Risks, target, source, &parsed.Diagnostics)
	parsed.Suggestions = parseSemanticSuggestions(wire.Suggestions, &parsed.Diagnostics)
	return parsed
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

func parseSemanticRisks(risks []semanticAnalysisFinding, target project.IndexFile, source string, diagnostics *FileAnalysisDiagnostics) []project.Finding {
	parsed := make([]project.Finding, 0, len(risks))
	for index, risk := range risks {
		finding := project.Finding{
			Category:           risk.Category,
			Severity:           strings.ToLower(strings.TrimSpace(risk.Severity)),
			Summary:            strings.TrimSpace(risk.Summary),
			TaskSpec:           parseOptionalBugTaskSpec(risk.TaskSpec, target, source),
			EngineeringInsight: diagnostics.parseInsight("risk", &index, risk.Insight),
		}
		if len(risk.TaskSpec) > 0 && strings.TrimSpace(string(risk.TaskSpec)) != "null" && finding.TaskSpec == nil {
			diagnostics.TaskSpecDegradedRiskIndices = append(diagnostics.TaskSpecDegradedRiskIndices, index)
		}
		parsed = append(parsed, finding)
	}
	return parsed
}

func parseSemanticSuggestions(suggestions []semanticAnalysisSuggestion, diagnostics *FileAnalysisDiagnostics) []project.Suggestion {
	parsed := make([]project.Suggestion, 0, len(suggestions))
	for index, suggestion := range suggestions {
		insight := diagnostics.parseInsight("suggestion", &index, suggestion.Insight)
		parsed = append(parsed, project.Suggestion{Title: suggestion.Title, Summary: suggestion.Summary, TargetSymbol: suggestion.TargetSymbol, Action: suggestion.Action, EngineeringInsight: insight})
	}
	return parsed
}

// parseFileAnalysisEngineeringInsightDiagnostic applies the selected-file
// requirement that all four insight fields be present and non-empty. The
// shared project parser remains the single source for JSON shape,
// normalization, and aggregate-size validation.
func parseFileAnalysisEngineeringInsightDiagnostic(raw json.RawMessage) (*project.EngineeringInsight, project.OptionalEngineeringInsightDiagnostic) {
	insight, diagnostic := project.ParseOptionalEngineeringInsightDiagnostic(raw)
	if diagnostic.Reason != project.OptionalEngineeringInsightAccepted {
		return nil, diagnostic
	}
	if !diagnostic.TradeoffOrFailureMode.Present || !diagnostic.TradeoffOrFailureMode.RuneCountKnown || diagnostic.TradeoffOrFailureMode.Runes == 0 || !diagnostic.TransferableLesson.Present || !diagnostic.TransferableLesson.RuneCountKnown || diagnostic.TransferableLesson.Runes == 0 {
		diagnostic.Reason = project.OptionalEngineeringInsightEmptyRequiredField
		return nil, diagnostic
	}
	return insight, diagnostic
}

func limitFileAnalysisInsights(report **project.EngineeringInsight, risks []project.Finding, suggestions []project.Suggestion) {
	remaining := 3
	keep := func(insight **project.EngineeringInsight) {
		if *insight == nil {
			return
		}
		if remaining == 0 {
			*insight = nil
			return
		}
		remaining--
	}
	keep(report)
	for index := range risks {
		keep(&risks[index].EngineeringInsight)
	}
	for index := range suggestions {
		keep(&suggestions[index].EngineeringInsight)
	}
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
