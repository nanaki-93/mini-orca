package agentruntime

import (
	"context"
	"fmt"
	"slices"
)

type SkillInspection struct {
	Name          string      `json:"name"`
	Description   string      `json:"description"`
	License       string      `json:"license,omitempty"`
	Compatibility string      `json:"compatibility,omitempty"`
	RequiredTools []string    `json:"required_tools"`
	Manifest      SkillFile   `json:"manifest"`
	Resources     []SkillFile `json:"resources"`
}

// InspectSkill validates captured text with the same native parser as runtime
// loading. It grants no tools; each invocation checks its actual tool inventory.
func InspectSkill(ctx context.Context, input SkillPackage, read func(context.Context, string, string) ([]byte, error)) (*SkillInspection, error) {
	if read == nil {
		return nil, fmt.Errorf("skill inspection requires a scoped reader")
	}
	if err := validateSkillPackage(input); err != nil {
		return nil, err
	}
	catalog := &skillCatalog{read: read}
	metadata, _, err := catalog.parse(ctx, input.Manifest)
	if err != nil {
		return nil, err
	}
	required := map[string]bool{}
	for _, name := range metadata.AllowedTools {
		if !toolNamePattern.MatchString(name) {
			return nil, fmt.Errorf("skill has an unsupported required tool name")
		}
		required[name] = true
	}
	if err := validateSkillMetadata(metadata, input.Directory, required); err != nil {
		return nil, err
	}
	total := input.Manifest.SizeBytes
	for _, resource := range input.Resources {
		total += resource.SizeBytes
		if total > 128<<10 {
			return nil, fmt.Errorf("skill inventory exceeds 128 KiB")
		}
		if _, err := catalog.readFile(ctx, resource); err != nil {
			return nil, err
		}
	}
	tools := append([]string{}, metadata.AllowedTools...)
	return &SkillInspection{Name: metadata.Name, Description: metadata.Description, License: metadata.License, Compatibility: metadata.Compatibility, RequiredTools: tools, Manifest: input.Manifest, Resources: slices.Clone(input.Resources)}, nil
}
