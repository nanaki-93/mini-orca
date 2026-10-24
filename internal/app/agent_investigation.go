package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/json/jsontext"
	"fmt"
	"os"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentruntime"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/llm"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

// PlanEvidence contains an exact source quotation and a model interpretation.
// Valid provenance does not make the interpretation a verified finding.
type PlanEvidence struct {
	Path        string `json:"path"`
	Hash        string `json:"hash"`
	StartLine   int    `json:"start_line"`
	EndLine     int    `json:"end_line"`
	Quote       string `json:"quote"`
	Observation string `json:"observation"`
}

type PlanInvestigationSource struct {
	Path   string `json:"path"`
	Hash   string `json:"hash"`
	Exists bool   `json:"exists"`
}

type PlanInvestigationRequest struct {
	ProjectID             string                    `json:"project_id"`
	ProjectRevision       string                    `json:"project_revision"`
	Plan                  ChangePlanInput           `json:"plan"`
	Paths                 []string                  `json:"paths"`
	AcceptanceCriteria    []string                  `json:"acceptance_criteria"`
	Sources               []PlanInvestigationSource `json:"sources"`
	ConfirmRemoteProvider bool                      `json:"confirm_remote_provider"`
}

type planInvestigationOutput struct {
	Version            string         `json:"version"`
	Approach           string         `json:"approach"`
	Evidence           []PlanEvidence `json:"evidence"`
	Assumptions        []string       `json:"assumptions"`
	SuggestedScope     []string       `json:"suggested_scope"`
	AcceptanceCriteria []string       `json:"acceptance_criteria"`
}

type PlanInvestigation struct {
	planInvestigationOutput
	ProjectID       string                  `json:"project_id"`
	ProjectRevision string                  `json:"project_revision"`
	InputHash       string                  `json:"input_hash"`
	ContextManifest project.ContextManifest `json:"context_manifest"`
}

type preparedPlanInvestigation struct {
	request      PlanInvestigationRequest
	root         string
	policy       *project.ContextPolicy
	index        *project.ProjectIndex
	sources      map[string]string
	instructions map[string]project.EffectiveInstructions
	scopes       []project.AgentScope
	inputHash    string
	manifest     project.ContextManifest
	sourceBytes  int
}

// InvestigatePlan returns suggestions without storing or approving a change plan.
func (s *Service) InvestigatePlan(ctx context.Context, request PlanInvestigationRequest) (*PlanInvestigation, error) {
	if err := s.RequireRemoteConfirmation(config.FunctionModelScope, request.ConfirmRemoteProvider); err != nil {
		return nil, err
	}
	runtime := s.runtimes.function
	timed, cancel := context.WithTimeout(ctx, duration(runtime.effective.Timeout))
	defer cancel()
	prepared, err := s.preparePlanInvestigation(timed, request)
	if err != nil {
		return nil, err
	}
	input, err := s.planInvestigationInput(prepared, runtime)
	if err != nil {
		return nil, err
	}
	authorize := func(ctx context.Context) error { return s.validatePlanInvestigation(ctx, prepared) }
	timed, err = withProjectMemory(timed, prepared.root, request.ProjectID, request.ProjectRevision, request.Plan.Problem, manifestMemorySources(prepared.manifest), authorize)
	if err != nil {
		return nil, err
	}
	output, err := s.retryAgentRequest(timed, runtime, input, authorize)
	if err != nil {
		return nil, err
	}
	result, err := parsePlanInvestigation(output.Content, prepared)
	if err != nil {
		return nil, err
	}
	if err := authorize(timed); err != nil {
		return nil, err
	}
	return &PlanInvestigation{planInvestigationOutput: result, ProjectID: request.ProjectID, ProjectRevision: request.ProjectRevision, InputHash: prepared.inputHash, ContextManifest: s.contextManifestForRuntime(prepared.manifest, runtime)}, nil
}

func (s *Service) preparePlanInvestigation(ctx context.Context, request PlanInvestigationRequest) (*preparedPlanInvestigation, error) {
	request.Plan = cloneChangePlanInput(request.Plan)
	request.Paths = slices.Clone(request.Paths)
	request.Sources = slices.Clone(request.Sources)
	request.AcceptanceCriteria = slices.Clone(request.AcceptanceCriteria)
	if err := validatePlanInvestigationRequest(request); err != nil {
		return nil, err
	}
	index, err := s.manager.Index()
	if err != nil {
		return nil, err
	}
	if index.ProjectID != request.ProjectID || index.ProjectRevision != request.ProjectRevision {
		return nil, project.ErrRevisionConflict
	}
	root := s.manager.Root()
	policy, err := project.NewContextPolicy(root)
	if err != nil {
		return nil, err
	}
	p := &preparedPlanInvestigation{request: request, root: root, policy: policy, index: index, sources: map[string]string{}, instructions: map[string]project.EffectiveInstructions{}, manifest: project.ContextManifest{Included: []project.ContextFile{}, Excluded: []project.ContextDecision{}, ByteLimit: maxChangeBytes}}
	for _, source := range request.Sources {
		if err := p.capture(ctx, source); err != nil {
			return nil, err
		}
	}
	for _, citation := range request.Plan.Citations {
		if err := validatePlanCitation(citation, p.sources[citation.Path]); err != nil {
			return nil, err
		}
	}
	data, err := json.Marshal([]any{request, policy.Version(), p.instructions})
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(data)
	p.inputHash = hex.EncodeToString(digest[:])
	if err := s.validatePlanInvestigation(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

func validatePlanInvestigationRequest(request PlanInvestigationRequest) error {
	if request.ProjectID == "" || request.ProjectRevision == "" || len(request.Paths) == 0 || len(request.Paths) > 8 {
		return fmt.Errorf("provide a project identity and 1–8 write paths")
	}
	if err := validateChangePlanInput(request.Plan); err != nil {
		return err
	}
	if err := validateChangeCriteria(request.AcceptanceCriteria); err != nil {
		return err
	}
	paths := append(slices.Clone(request.Paths), request.Plan.ReadPaths...)
	if len(paths) != len(request.Sources) {
		return fmt.Errorf("provide captured identity for every read and write path")
	}
	seen := map[string]bool{}
	for _, source := range request.Sources {
		if seen[source.Path] || !slices.Contains(paths, source.Path) || source.Exists != (source.Hash != "") {
			return fmt.Errorf("invalid or duplicate investigation source identity")
		}
		seen[source.Path] = true
		if !source.Exists && slices.Contains(request.Plan.ReadPaths, source.Path) {
			return fmt.Errorf("read context must refer to existing source")
		}
	}
	if len(seen) != len(paths) {
		return fmt.Errorf("read and write paths must be distinct")
	}
	return nil
}

func (p *preparedPlanInvestigation) capture(ctx context.Context, source PlanInvestigationSource) error {
	if err := context.Cause(ctx); err != nil {
		return err
	}
	if _, err := project.ResolveWritePath(p.root, source.Path); err != nil {
		return err
	}
	if decision := p.policy.Decide(source.Path); !decision.Include {
		return fmt.Errorf("%w: %s", project.ErrExcludedFile, decision.Reason)
	}
	if source.Exists {
		if err := p.captureExisting(ctx, source); err != nil {
			return err
		}
	} else if err := validateAbsentInvestigationSource(p.root, source.Path); err != nil {
		return err
	}
	instructions, err := project.ResolveInstructions(p.root, source.Path)
	if err != nil {
		return err
	}
	p.instructions[source.Path] = instructions
	for _, guide := range instructions.Files {
		if !slices.ContainsFunc(p.manifest.Included, func(file project.ContextFile) bool { return file.Path == guide.Path }) {
			p.manifest.Included = append(p.manifest.Included, project.ContextFile{Path: guide.Path, Hash: guide.Hash, SizeBytes: int64(len(guide.Content)), Tokens: (len(guide.Content) + 3) / 4})
		}
	}
	return nil
}

func (p *preparedPlanInvestigation) captureExisting(ctx context.Context, source PlanInvestigationSource) error {
	var indexed *project.IndexFile
	for _, file := range p.index.Files {
		if file.Path == source.Path {
			indexed = &file
			break
		}
	}
	if indexed == nil || indexed.ContentHash != source.Hash {
		return project.ErrRevisionConflict
	}
	data, err := p.policy.ReadSource(ctx, source.Path, source.Hash)
	if err != nil {
		return err
	}
	if p.sourceBytes+len(data) > maxChangeBytes {
		return fmt.Errorf("investigation source exceeds 256 KiB; reduce scope")
	}
	p.sourceBytes += len(data)
	p.sources[source.Path] = string(data)
	if indexed.LineCount > 0 {
		p.scopes = append(p.scopes, project.AgentScope{Path: source.Path, Hash: source.Hash, StartLine: 1, EndLine: indexed.LineCount})
	}
	p.manifest.Included = append(p.manifest.Included, project.ContextFile{Path: source.Path, Hash: source.Hash, SizeBytes: int64(len(data)), Tokens: (len(data) + 3) / 4})
	return nil
}

func validateAbsentInvestigationSource(root, path string) error {
	full, err := project.ResolveWritePath(root, path)
	if err != nil {
		return err
	}
	_, err = os.Lstat(full)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect new investigation path: %w", err)
	}
	return fmt.Errorf("%w: a new target now exists", project.ErrRevisionConflict)
}

func (s *Service) validatePlanInvestigation(ctx context.Context, p *preparedPlanInvestigation) error {
	if err := context.Cause(ctx); err != nil {
		return err
	}
	index, err := s.manager.Index()
	if err != nil {
		return err
	}
	if s.manager.Root() != p.root || index.ProjectID != p.request.ProjectID || index.ProjectRevision != p.request.ProjectRevision {
		return project.ErrRevisionConflict
	}
	policy, err := project.NewContextPolicy(p.root)
	if err != nil {
		return err
	}
	if policy.Version() != p.policy.Version() {
		return fmt.Errorf("%w: context policy changed", project.ErrRevisionConflict)
	}
	for _, source := range p.request.Sources {
		if err := p.validateSource(ctx, policy, source); err != nil {
			return err
		}
	}
	return nil
}

func (p *preparedPlanInvestigation) validateSource(ctx context.Context, policy *project.ContextPolicy, source PlanInvestigationSource) error {
	if source.Exists {
		if _, err := policy.ReadSource(ctx, source.Path, source.Hash); err != nil {
			return err
		}
	} else if err := validateAbsentInvestigationSource(p.root, source.Path); err != nil {
		return err
	}
	return project.ValidateInstructions(p.root, source.Path, p.instructions[source.Path].Fingerprint)
}

func (s *Service) planInvestigationInput(p *preparedPlanInvestigation, runtime modelRuntime) (agentruntime.Request, error) {
	input := agentruntime.Request{Schema: &llm.JSONSchema{Name: "plan_investigation", Schema: json.RawMessage(planInvestigationSchema)}, ContextMaxTokens: declarationContextLimit(runtime.effective.ContextMaxTokens)}
	tools := llm.ProviderCapabilities(runtime.profile).Tools && len(p.scopes) > 0
	prompt, err := p.prompt(tools)
	if err != nil {
		return input, err
	}
	if len(prompt) > input.ContextMaxTokens*4 || len(prompt) > 128<<10 {
		return input, fmt.Errorf("plan and context exceed the Function profile's limit; reduce scope")
	}
	input.Messages = []llm.ChatMessage{{Role: "user", Content: prompt}}
	p.manifest.EstimatedTokens, p.manifest.TokenLimit = (len(prompt)+3)/4, input.ContextMaxTokens
	if !tools {
		return input, nil
	}
	reader, err := project.NewAgentContext(p.index, p.scopes, p.policy.ReadSource)
	if err != nil {
		return input, err
	}
	input.Investigation, err = agentruntime.NewInvestigation(p.inputHash, repositoryTools(reader), func(ctx context.Context) error { return s.validatePlanInvestigation(ctx, p) })
	return input, err
}

func (p *preparedPlanInvestigation) prompt(tools bool) (string, error) {
	var text strings.Builder
	text.WriteString("Investigate this editable task plan. Return only JSON matching the supplied schema. Propose an approach and testable acceptance criteria. Separate assumptions from source evidence. Every evidence item must quote exact admitted source within its cited line range and hash. Suggested scope is advisory; do not treat it as permission. Do not execute commands or change source.\n")
	data, err := json.Marshal(p.request)
	if err != nil {
		return "", err
	}
	text.Write(data)
	seen := map[string]bool{}
	for _, source := range p.request.Sources {
		for _, guide := range p.instructions[source.Path].Files {
			if !seen[guide.Path] {
				text.WriteString("\n" + project.InstructionPromptGuidance + "\n" + guide.Path + "\n" + guide.Content)
				seen[guide.Path] = true
			}
		}
		if !tools && source.Exists {
			text.WriteString("\nCaptured source " + source.Path + " (" + source.Hash + "):\n" + p.sources[source.Path])
		}
	}
	return text.String(), nil
}

func parsePlanInvestigation(content string, p *preparedPlanInvestigation) (planInvestigationOutput, error) {
	var output planInvestigationOutput
	if len(content) > 32<<10 || !jsontext.Value(content).IsValid() {
		return output, fmt.Errorf("invalid bounded planning response")
	}
	decoder := json.NewDecoder(bytes.NewBufferString(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&output); err != nil {
		return output, fmt.Errorf("decode planning response: %w", err)
	}
	if output.Version != "v1" || !validInvestigationText(output.Approach, 4096) || len(output.Evidence) > 8 || len(output.Assumptions) > 8 || len(output.SuggestedScope) > 8 {
		return output, fmt.Errorf("invalid planning response fields")
	}
	if err := validateInvestigationCollections(output); err != nil {
		return output, err
	}
	for _, evidence := range output.Evidence {
		if err := validatePlanCitation(evidence, p.sources[evidence.Path]); err != nil {
			return output, err
		}
	}
	for _, path := range output.SuggestedScope {
		if _, err := project.ResolveWritePath(p.root, path); err != nil {
			return output, err
		}
		if !p.policy.Decide(path).Include {
			return output, fmt.Errorf("suggested path is excluded by context policy")
		}
	}
	return output, nil
}

func validateInvestigationCollections(output planInvestigationOutput) error {
	if output.Evidence == nil || output.Assumptions == nil || output.SuggestedScope == nil || len(output.AcceptanceCriteria) == 0 {
		return fmt.Errorf("planning response is missing required collections")
	}
	if err := validateChangeCriteria(output.AcceptanceCriteria); err != nil {
		return err
	}
	for _, criterion := range output.AcceptanceCriteria {
		if !validInvestigationText(criterion, 512) {
			return fmt.Errorf("invalid planning acceptance criterion")
		}
	}
	for _, assumption := range output.Assumptions {
		if !validInvestigationText(assumption, 512) {
			return fmt.Errorf("invalid planning assumption")
		}
	}
	return nil
}

func validInvestigationText(value string, limit int) bool {
	return strings.TrimSpace(value) != "" && len(value) <= limit && utf8.ValidString(value) && !strings.ContainsRune(value, 0)
}

func validatePlanCitation(citation PlanEvidence, source string) error {
	if source == "" || citation.Hash != contentHash([]byte(source)) || !validInvestigationText(citation.Quote, 1024) || !validInvestigationText(citation.Observation, 512) {
		return fmt.Errorf("planning citation lacks valid captured provenance")
	}
	lines := strings.Split(strings.TrimSuffix(source, "\n"), "\n")
	if citation.StartLine < 1 || citation.EndLine < citation.StartLine || citation.EndLine > len(lines) {
		return fmt.Errorf("planning citation is outside the captured source")
	}
	if !strings.Contains(strings.Join(lines[citation.StartLine-1:citation.EndLine], "\n"), citation.Quote) {
		return fmt.Errorf("planning citation quote does not match captured source")
	}
	return nil
}

const planInvestigationSchema = `{"type":"object","properties":{"version":{"type":"string","const":"v1"},"approach":{"type":"string","minLength":1,"maxLength":4096},"evidence":{"type":"array","maxItems":8,"items":{"type":"object","properties":{"path":{"type":"string"},"hash":{"type":"string"},"start_line":{"type":"integer","minimum":1},"end_line":{"type":"integer","minimum":1},"quote":{"type":"string","minLength":1,"maxLength":1024},"observation":{"type":"string","minLength":1,"maxLength":512}},"required":["path","hash","start_line","end_line","quote","observation"],"additionalProperties":false}},"assumptions":{"type":"array","maxItems":8,"items":{"type":"string","maxLength":512}},"suggested_scope":{"type":"array","maxItems":8,"items":{"type":"string","maxLength":512}},"acceptance_criteria":{"type":"array","minItems":1,"maxItems":16,"items":{"type":"string","minLength":1,"maxLength":512}}},"required":["version","approach","evidence","assumptions","suggested_scope","acceptance_criteria"],"additionalProperties":false}`
