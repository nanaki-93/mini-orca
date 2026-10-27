package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentruntime"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

const repositoryQuerySchema = `{"type":"object","properties":{"path":{"type":"string","minLength":1,"maxLength":512},"query":{"type":"string","maxLength":256},"start_line":{"type":"integer","minimum":1,"maximum":1000000},"end_line":{"type":"integer","minimum":1,"maximum":1000000}},"required":["path"],"additionalProperties":false}`
const repositoryEvidenceSchema = `{"type":"object","properties":{"status":{"type":"string","enum":["ok","scope_required","unavailable","stale"]},"evidence":{"type":"array","maxItems":20,"items":{"type":"object","properties":{"path":{"type":"string"},"hash":{"type":"string"},"start_line":{"type":"integer"},"end_line":{"type":"integer"},"kind":{"type":"string"},"text":{"type":"string"},"confidence":{"type":"string"}},"required":["path","hash","start_line","end_line","kind","text","confidence"],"additionalProperties":false}},"requested":{"type":"object"},"truncated":{"type":"boolean"}},"required":["status","evidence","truncated"],"additionalProperties":false}`

func (s *Service) declarationInvestigation(ctx context.Context, request DeclarationExplanationRequest, prepared preparedDeclarationExplanation) (*agentruntime.Investigation, error) {
	root := prepared.root
	policy, err := project.NewContextPolicy(root)
	if err != nil {
		return nil, err
	}
	instructions, err := project.ResolveInstructions(root, request.TargetPath)
	if err != nil {
		return nil, err
	}
	if policy.Version() != prepared.policy || instructions.Fingerprint != prepared.instructions {
		return nil, fmt.Errorf("%w: investigation context changed", agentruntime.ErrPolicyDenied)
	}
	scope := project.AgentScope{Path: prepared.file.Path, Hash: prepared.file.ContentHash, StartLine: 1, EndLine: max(1, prepared.file.LineCount)}
	reader, err := project.NewAgentContext(&project.ProjectIndex{Files: []project.IndexFile{prepared.file}}, []project.AgentScope{scope}, policy.ReadSource)
	if err != nil {
		return nil, err
	}
	authorize := func(ctx context.Context) error { return s.validateExplanationContext(ctx, request, prepared) }
	data, err := json.Marshal([]any{request.ProjectID, request.ProjectRevision, scope, policy.Version(), instructions.Fingerprint})
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(data)
	tools := repositoryTools(reader)
	tools = append(tools, agentruntime.ToolSpec{Name: "current_checks", Description: "Read source-free checks for a currently bound proposal; unavailable or stale evidence is not a pass. Never runs checks.", InputSchema: json.RawMessage(repositoryQuerySchema), OutputSchema: json.RawMessage(repositoryEvidenceSchema), Run: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
		var query project.AgentQuery
		if err := json.Unmarshal(args, &query); err != nil {
			return nil, err
		}
		if query.Path != scope.Path {
			return json.Marshal(project.AgentEvidenceResult{Status: "scope_required", Evidence: []project.AgentEvidence{}, Requested: &query})
		}
		result, err := s.agentCheckEvidence(ctx, root, request, scope, policy.Version())
		if err != nil {
			return nil, err
		}
		return json.Marshal(result)
	}})
	return s.skillInvestigation(ctx, root, hex.EncodeToString(hash[:]), tools, authorize)
}

func (s *Service) validateExplanationContext(ctx context.Context, request DeclarationExplanationRequest, prepared preparedDeclarationExplanation) error {
	if err := context.Cause(ctx); err != nil {
		return err
	}
	if s.manager.Root() != prepared.root {
		return project.ErrRevisionConflict
	}
	current, err := project.NewContextPolicy(prepared.root)
	if err != nil {
		return err
	}
	// Read through the bounded, cancellable reader rather than reusing the
	// mutation guard's unrestricted hash pass on every agent turn.
	if _, err := current.ReadSource(ctx, request.TargetPath, request.BaseFileHash); err != nil {
		return err
	}
	if err := s.validateInvestigationIndex(request, prepared); err != nil {
		return err
	}
	if current.Version() != prepared.policy {
		return fmt.Errorf("context policy changed")
	}
	return project.ValidateInstructions(prepared.root, request.TargetPath, prepared.instructions)
}

func (s *Service) validateInvestigationIndex(request DeclarationExplanationRequest, prepared preparedDeclarationExplanation) error {
	index, err := s.manager.Index()
	if err != nil {
		return err
	}
	if index.ProjectID != request.ProjectID || index.ProjectRevision != request.ProjectRevision {
		return project.ErrRevisionConflict
	}
	file, err := s.manager.IndexedFile(request.TargetPath)
	if err != nil {
		return err
	}
	if s.manager.Root() != prepared.root || file.ContentHash != request.BaseFileHash {
		return project.ErrRevisionConflict
	}
	if !sameExplanationAnchor(prepared.file, *file) {
		return project.ErrRevisionConflict
	}
	return nil
}

func repositoryTools(reader *project.AgentContext) []agentruntime.ToolSpec {
	descriptions := map[string]string{
		"read_snippet":      "Read only the admitted source span. A wider path or line range returns scope_required.",
		"search_repository": "Search literal text within an admitted file; returns at most 20 hash-bound matches.",
		"inspect_imports":   "Inspect admitted indexed imports. An import is not permission to read its destination.",
		"inspect_impact":    "Inspect heuristic import relationships within admitted scope; this is not a verified call graph.",
	}
	var result []agentruntime.ToolSpec
	for _, name := range slices.Sorted(maps.Keys(descriptions)) {
		result = append(result, agentruntime.ToolSpec{Name: name, Description: descriptions[name], InputSchema: json.RawMessage(repositoryQuerySchema), OutputSchema: json.RawMessage(repositoryEvidenceSchema), Run: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
			var query project.AgentQuery
			if err := json.Unmarshal(args, &query); err != nil {
				return nil, err
			}
			evidence, err := reader.Retrieve(ctx, name, query)
			if err != nil {
				return nil, err
			}
			return json.Marshal(evidence)
		}})
	}
	return result
}

func (s *Service) agentCheckEvidence(ctx context.Context, root string, request DeclarationExplanationRequest, scope project.AgentScope, policy string) (project.AgentEvidenceResult, error) {
	result := project.AgentEvidenceResult{Status: "unavailable", Evidence: []project.AgentEvidence{}}
	s.changesMu.Lock()
	authority := maps.Clone(s.changeAuthority)
	s.changesMu.Unlock()
	ids := slices.Sorted(maps.Keys(authority))
	matched := 0
	for _, key := range ids {
		if !strings.HasPrefix(key, request.ProjectID+"/") {
			continue
		}
		if matched == 16 {
			result.Truncated = true
			break
		}
		matched++
		if err := context.Cause(ctx); err != nil {
			return result, err
		}
		stored, err := loadChangeSession(root, strings.TrimPrefix(key, request.ProjectID+"/"))
		if err != nil {
			return result, err
		}
		if !matchesAgentCheckScope(stored, request, scope, policy, authority[key]) {
			continue
		}
		workspace, err := workspaceFingerprint(ctx, root)
		if err != nil {
			return result, err
		}
		s.changesMu.Lock()
		current := s.changeAuthority[key] == authority[key]
		s.changesMu.Unlock()
		if !current || workspace != stored.WorkspaceHash {
			result.Status = "stale"
			continue
		}
		result.Status = "ok"
		appendAgentChecks(&result, scope, stored)
		break
	}
	return result, context.Cause(ctx)
}

func matchesAgentCheckScope(stored *ChangeSession, request DeclarationExplanationRequest, scope project.AgentScope, policy, authority string) bool {
	if stored.ProjectID != request.ProjectID || stored.ProjectRevision != request.ProjectRevision || stored.PolicyFingerprint != policy || len(stored.Checks) == 0 {
		return false
	}
	if authority != "checks:"+stored.Hash && authority != "review:"+stored.Hash {
		return false
	}
	for _, target := range stored.Targets {
		if target.Path == scope.Path && target.Hash == scope.Hash {
			return true
		}
	}
	return false
}

func appendAgentChecks(result *project.AgentEvidenceResult, scope project.AgentScope, stored *ChangeSession) {
	bytes := 0
	for _, check := range stored.Checks {
		if len(result.Evidence) == 20 {
			result.Truncated = true
			break
		}
		// Commands, output and diagnostics can contain source from other paths.
		data, _ := json.Marshal(struct {
			ProposalID, ProposalHash, Name, State, Outcome string
			Required                                       bool
		}{stored.ID, stored.Hash, check.Name, check.State, check.Outcome, check.Required})
		if len(data) > 1024 || bytes+len(data) > 8192 {
			result.Truncated = true
			continue
		}
		bytes += len(data)
		result.Evidence = append(result.Evidence, project.AgentEvidence{AgentScope: scope, Kind: "proposal_check", Text: string(data), Confidence: "recorded_proposal_check"})
	}
	if len(result.Evidence) == 0 {
		result.Status = "unavailable"
	}
}
