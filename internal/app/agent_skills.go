package app

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentruntime"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

func (s *Service) skillInvestigation(ctx context.Context, root, scope string, specs []agentruntime.ToolSpec, authorize func(context.Context) error) (*agentruntime.Investigation, error) {
	if len(s.agentRuntime.Skills) == 0 {
		return agentruntime.NewInvestigation(scope, specs, authorize)
	}
	if err := authorize(ctx); err != nil {
		return nil, err
	}
	index, err := s.manager.Index()
	if err != nil {
		return nil, err
	}
	policy, err := project.NewContextPolicy(root)
	if err != nil {
		return nil, err
	}
	var packages []agentruntime.SkillPackage
	for _, directory := range s.agentRuntime.Skills {
		input, err := captureSkillPackage(index, directory, policy)
		if err != nil {
			return nil, err
		}
		packages = append(packages, input)
	}
	reader := func(ctx context.Context, path, hash string) ([]byte, error) {
		current, err := project.NewContextPolicy(root)
		if err != nil {
			return nil, err
		}
		if s.manager.Root() != root || current.Version() != policy.Version() {
			return nil, project.ErrRevisionConflict
		}
		return current.ReadSource(ctx, path, hash)
	}
	return agentruntime.NewSkillInvestigation(ctx, scope, specs, authorize, packages, reader)
}

func captureSkillPackage(index *project.ProjectIndex, directory string, policy *project.ContextPolicy) (agentruntime.SkillPackage, error) {
	result := agentruntime.SkillPackage{Directory: directory, Resources: []agentruntime.SkillFile{}}
	for _, file := range index.Files {
		relative, found := strings.CutPrefix(file.Path, directory+"/")
		if !found || !skillInventoryPath(relative) {
			continue
		}
		if decision := policy.Decide(file.Path); !decision.Include {
			return result, fmt.Errorf("%w: configured skill resource %s", project.ErrExcludedFile, file.Path)
		}
		if file.SizeBytes < 1 || file.SizeBytes > 8192 {
			return result, fmt.Errorf("skill file %s must contain 1–8192 bytes", file.Path)
		}
		captured := agentruntime.SkillFile{Path: file.Path, Hash: file.ContentHash, SizeBytes: file.SizeBytes}
		if relative == "SKILL.md" {
			result.Manifest = captured
		} else {
			result.Resources = append(result.Resources, captured)
		}
	}
	if result.Manifest.Hash == "" {
		return result, fmt.Errorf("configured skill %s/SKILL.md is missing, excluded, or unindexed; reindex after checking its policy", directory)
	}
	if len(result.Resources) > 16 {
		return result, fmt.Errorf("configured skill %s exceeds 16 indexed resources", directory)
	}
	slices.SortFunc(result.Resources, func(a, b agentruntime.SkillFile) int { return strings.Compare(a.Path, b.Path) })
	return result, nil
}

func skillInventoryPath(relative string) bool {
	if relative == "SKILL.md" {
		return true
	}
	directory, _, found := strings.Cut(relative, "/")
	return found && slices.Contains([]string{"references", "assets", "scripts"}, directory)
}

func addSkillContext(ctx context.Context, manifest *project.ContextManifest, investigation *agentruntime.Investigation) error {
	if investigation == nil {
		return nil
	}
	files, err := investigation.SkillFiles(ctx)
	if err != nil {
		return err
	}
	for _, file := range files {
		if slices.ContainsFunc(manifest.Included, func(item project.ContextFile) bool { return item.Path == file.Path && item.Hash == file.Hash }) {
			continue
		}
		tokens := int((file.SizeBytes + 3) / 4)
		manifest.Included = append(manifest.Included, project.ContextFile{Path: file.Path, Hash: file.Hash, SizeBytes: file.SizeBytes, Tokens: tokens})
		manifest.EstimatedTokens += tokens
	}
	return nil
}
