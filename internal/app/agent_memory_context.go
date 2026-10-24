package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentruntime"
	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

type memoryScope struct {
	root, projectID, revision, policy string
	sources                           []agentstate.MemorySource
	store                             *agentstate.Store
	authorize                         func(context.Context) error
}

func withProjectMemory(ctx context.Context, root, projectID, revision, query string, sources []project.AgentScope, authorize func(context.Context) error) (context.Context, error) {
	store, err := memoryStore(root)
	if err != nil {
		return nil, err
	}
	records, err := readMemories(ctx, store, projectID)
	if err != nil {
		return nil, err
	}
	if len(sources) == 0 || !slices.ContainsFunc(records, func(r agentstate.MemoryRecord) bool { return r.State == "accepted" && r.ExpiresAt.After(time.Now()) }) {
		return ctx, nil
	}
	if authorize == nil {
		return nil, fmt.Errorf("%w: memory requires captured source authority", agentruntime.ErrPolicyDenied)
	}
	policy, err := project.NewContextPolicy(root)
	if err != nil {
		return nil, err
	}
	scope := memoryScope{root: root, projectID: projectID, revision: revision, policy: policy.Version(), store: store, authorize: authorize}
	for _, source := range sources {
		scope.sources = append(scope.sources, agentstate.MemorySource(source))
	}
	return agentruntime.WithMemory(ctx, &agentruntime.MemoryAccess{Query: query, Search: scope.search, Validate: scope.validate})
}

func (scope memoryScope) search(ctx context.Context, query string) ([]agentstate.MemoryMatch, error) {
	if err := scope.authorize(ctx); err != nil {
		return nil, err
	}
	matches, err := scope.store.SearchMemory(ctx, scope.projectID, scope.revision, scope.policy, query, scope.sources, time.Now())
	if err != nil {
		if cause := context.Cause(ctx); cause != nil {
			return nil, cause
		}
		return nil, memoryStorageError(err)
	}
	result := make([]agentstate.MemoryMatch, 0, len(matches))
	for _, match := range matches {
		eligible, err := scope.eligible(ctx, match.Record)
		if err != nil {
			return nil, err
		}
		if eligible {
			result = append(result, match)
		}
	}
	return result, nil
}

func (scope memoryScope) eligible(ctx context.Context, record agentstate.MemoryRecord) (bool, error) {
	if record.ProjectID != scope.projectID || record.State != "accepted" || !record.ExpiresAt.After(time.Now()) || record.Policy != scope.policy {
		return false, nil
	}
	policy, err := project.NewContextPolicy(scope.root)
	if err != nil {
		return false, err
	}
	if policy.Version() != scope.policy {
		return false, nil
	}
	for _, source := range record.Sources {
		if !memorySourceAdmitted(source, scope.sources) {
			return false, nil
		}
		if err := validateMemorySource(ctx, policy, source); err != nil {
			if memorySourceChanged(err) {
				return false, nil
			}
			return false, err
		}
	}
	return true, nil
}

func memorySourceChanged(err error) bool {
	return errors.Is(err, project.ErrRevisionConflict) || errors.Is(err, project.ErrExcludedFile) || errors.Is(err, os.ErrNotExist)
}

func memorySourceAdmitted(source agentstate.MemorySource, admitted []agentstate.MemorySource) bool {
	return slices.ContainsFunc(admitted, func(grant agentstate.MemorySource) bool {
		return source.Path == grant.Path && source.Hash == grant.Hash && (grant.StartLine == 0 || source.StartLine >= grant.StartLine && source.EndLine <= grant.EndLine)
	})
}

func (scope memoryScope) validate(ctx context.Context, versions []agentstate.MemoryVersion) error {
	if err := scope.authorize(ctx); err != nil {
		return err
	}
	records, err := readMemories(ctx, scope.store, scope.projectID)
	if err != nil {
		return err
	}
	for _, version := range versions {
		index := slices.IndexFunc(records, func(r agentstate.MemoryRecord) bool { return r.ID == version.ID && r.Version == version.Version })
		if index < 0 {
			return ErrMemoryConflict
		}
		eligible, err := scope.eligible(ctx, records[index])
		if err != nil {
			return err
		}
		if !eligible {
			return ErrMemoryConflict
		}
	}
	return nil
}

func manifestMemorySources(manifest project.ContextManifest) []project.AgentScope {
	sources := make([]project.AgentScope, 0, len(manifest.Included))
	for _, file := range manifest.Included {
		// Inventory-only entries and truncated snippets do not admit whole-file advice.
		if !file.Truncated && file.Tokens > 0 {
			sources = append(sources, project.AgentScope{Path: file.Path, Hash: file.Hash})
		}
	}
	return sources
}

func changeMemorySources(session *ChangeSession, omitTests bool) []project.AgentScope {
	targets := slices.Clone(session.Targets)
	if session.Plan != nil {
		targets = append(targets, session.Plan.ReadTargets...)
	}
	result := make([]project.AgentScope, 0, len(targets))
	for _, target := range targets {
		if target.Exists && !(omitTests && isTestFilePath(target.Path)) {
			result = append(result, project.AgentScope{Path: target.Path, Hash: target.Hash})
		}
	}
	return result
}

func withOverviewMemory(ctx context.Context, snapshot *project.Analysis) (context.Context, error) {
	_, manifest, err := project.NewContextBuilder().BuildWithManifest(snapshot.Path, "")
	if err != nil {
		return nil, err
	}
	authorize := func(ctx context.Context) error {
		current, err := project.ScanProject(ctx, snapshot.Path)
		if err != nil {
			return err
		}
		if current.ProjectID != snapshot.ProjectID || current.ProjectRevision != snapshot.ProjectRevision {
			return project.ErrRevisionConflict
		}
		return nil
	}
	return withProjectMemory(ctx, snapshot.Path, snapshot.ProjectID, snapshot.ProjectRevision, "architecture decisions project overview lessons", manifestMemorySources(manifest), authorize)
}

func (s *Service) securityReviewMemoryContext(ctx context.Context, snapshot securityReviewSnapshot) (context.Context, error) {
	if err := s.validateSecurityReviewSnapshot(ctx, snapshot); err != nil {
		return nil, err
	}
	sources := []project.AgentScope{{Path: snapshot.file.Path, Hash: snapshot.file.ContentHash}}
	if snapshot.symbol != nil {
		sources[0].StartLine, sources[0].EndLine = snapshot.symbol.StartLine, snapshot.symbol.EndLine
	}
	return withProjectMemory(ctx, snapshot.root, snapshot.analysis.ProjectID, snapshot.analysis.ProjectRevision, snapshot.file.Path+" security review lessons", sources, func(ctx context.Context) error { return s.validateSecurityReviewSnapshot(ctx, snapshot) })
}
