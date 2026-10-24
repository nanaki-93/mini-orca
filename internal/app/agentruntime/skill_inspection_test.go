package agentruntime

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestSkillInspectionValidatesWithoutGrantingUnavailableTools(t *testing.T) {
	source, input := newSkillSource()
	inspection, err := InspectSkill(t.Context(), input, source.read)
	if err != nil || inspection.Name != "review-code" || len(inspection.RequiredTools) != 2 || len(inspection.Resources) != 1 {
		t.Fatal(inspection, err)
	}
	if source.reads[input.Manifest.Path] != 1 || source.reads[input.Resources[0].Path] != 1 {
		t.Fatal("inspection did not validate bounded captured resources", source.reads)
	}
	base := fixtureInvestigation(t, nil, nil)
	spec := base.tools[0].spec
	spec.Name = "different_lookup"
	if _, err := NewSkillInvestigation(t.Context(), "inspection-is-not-a-grant", []ToolSpec{spec}, func(context.Context) error { return nil }, []SkillPackage{input}, source.read); !errors.Is(err, ErrPolicyDenied) {
		t.Fatal("inspection granted the unavailable lookup tool", err)
	}
}

func TestSkillInspectionRejectsChangedAndMalformedResources(t *testing.T) {
	for _, kind := range []string{"changed-manifest", "changed-resource", "invalid-name", "duplicate-tool", "invalid-tool", "empty-body", "canceled"} {
		t.Run(kind, func(t *testing.T) {
			source, input := newSkillSource()
			ctx := t.Context()
			manifest := fixtureSkillManifest
			switch kind {
			case "changed-manifest":
				source.replace(input.Manifest.Path, "changed")
			case "changed-resource":
				source.replace(input.Resources[0].Path, "changed")
			case "invalid-name":
				manifest = strings.Replace(manifest, "name: review-code", "name: another-skill", 1)
			case "duplicate-tool":
				manifest = strings.Replace(manifest, "lookup load_skill_resource", "lookup lookup", 1)
			case "invalid-tool":
				manifest = strings.Replace(manifest, "lookup load_skill_resource", "../../execute", 1)
			case "empty-body":
				manifest = strings.Split(manifest, "SKILL_BODY")[0]
			case "canceled":
				canceled, cancel := context.WithCancel(ctx)
				cancel()
				ctx = canceled
			}
			if manifest != fixtureSkillManifest {
				source.replace(input.Manifest.Path, manifest)
				input.Manifest = skillFile(input.Manifest.Path, manifest)
			}
			if _, err := InspectSkill(ctx, input, source.read); err == nil {
				t.Fatal("invalid inspection succeeded")
			}
		})
	}
}
