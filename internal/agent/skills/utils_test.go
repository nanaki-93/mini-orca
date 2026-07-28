package skills

import (
	"strings"
	"testing"
)

func TestBuildSkillPrompt_ValidSkills(t *testing.T) {
	registry := NewSkillsRegistry()

	prompt := registry.BuildSkillPrompt([]string{"solid_principles", "clean_code"})
	if prompt == "" {
		t.Fatal("expected non-empty prompt")
	}
	if !strings.Contains(prompt, "SOLID principles") {
		t.Error("expected prompt to contain 'SOLID principles'")
	}
	if !strings.Contains(prompt, "clean code practices") {
		t.Error("expected prompt to contain 'clean code practices'")
	}
	if !strings.Contains(prompt, "---") {
		t.Error("expected prompt to contain separator between templates")
	}
}

func TestBuildSkillPrompt_MixedValidInvalid(t *testing.T) {
	registry := NewSkillsRegistry()

	prompt := registry.BuildSkillPrompt([]string{"solid_principles", "nonexistent", "clean_code"})
	if prompt == "" {
		t.Fatal("expected non-empty prompt")
	}
	if !strings.Contains(prompt, "SOLID principles") {
		t.Error("expected prompt to contain 'SOLID principles'")
	}
	if !strings.Contains(prompt, "clean code practices") {
		t.Error("expected prompt to contain 'clean code practices'")
	}
}

func TestBuildSkillPrompt_AllInvalid(t *testing.T) {
	registry := NewSkillsRegistry()

	prompt := registry.BuildSkillPrompt([]string{"nonexistent1", "nonexistent2"})
	if prompt != "" {
		t.Errorf("expected empty prompt for all invalid skills, got %q", prompt)
	}
}

func TestBuildSkillPrompt_EmptyInput(t *testing.T) {
	registry := NewSkillsRegistry()

	prompt := registry.BuildSkillPrompt(nil)
	if prompt != "" {
		t.Errorf("expected empty prompt for nil input, got %q", prompt)
	}
}

func TestValidateSkillNames_AllValid(t *testing.T) {
	registry := NewSkillsRegistry()

	valid, invalid := registry.ValidateSkillNames([]string{"solid_principles", "clean_code", "function_generation"})
	if len(valid) != 3 {
		t.Errorf("expected 3 valid skills, got %d", len(valid))
	}
	if len(invalid) != 0 {
		t.Errorf("expected 0 invalid skills, got %d", len(invalid))
	}
}

func TestValidateSkillNames_AllInvalid(t *testing.T) {
	registry := NewSkillsRegistry()

	valid, invalid := registry.ValidateSkillNames([]string{"nonexistent1", "nonexistent2"})
	if len(valid) != 0 {
		t.Errorf("expected 0 valid skills, got %d", len(valid))
	}
	if len(invalid) != 2 {
		t.Errorf("expected 2 invalid skills, got %d", len(invalid))
	}
}

func TestValidateSkillNames_Mixed(t *testing.T) {
	registry := NewSkillsRegistry()

	valid, invalid := registry.ValidateSkillNames([]string{"solid_principles", "nonexistent", "clean_code"})
	if len(valid) != 2 {
		t.Errorf("expected 2 valid skills, got %d", len(valid))
	}
	if len(invalid) != 1 {
		t.Errorf("expected 1 invalid skill, got %d", len(invalid))
	}

	if valid[0] != "solid_principles" || valid[1] != "clean_code" {
		t.Errorf("expected [solid_principles, clean_code], got %v", valid)
	}
	if invalid[0] != "nonexistent" {
		t.Errorf("expected [nonexistent], got %v", invalid)
	}
}

func TestValidateSkillNames_EmptyInput(t *testing.T) {
	registry := NewSkillsRegistry()

	valid, invalid := registry.ValidateSkillNames(nil)
	if len(valid) != 0 {
		t.Errorf("expected 0 valid skills, got %d", len(valid))
	}
	if len(invalid) != 0 {
		t.Errorf("expected 0 invalid skills, got %d", len(invalid))
	}
}

func TestMergeSkills_NoOverlap(t *testing.T) {
	agent := []string{"solid_principles", "clean_code"}
	global := []string{"function_generation", "struct_design"}

	result := MergeSkills(agent, global)
	if len(result) != 4 {
		t.Errorf("expected 4 skills, got %d", len(result))
	}
	if result[0] != "solid_principles" || result[1] != "clean_code" ||
		result[2] != "function_generation" || result[3] != "struct_design" {
		t.Errorf("unexpected order: %v", result)
	}
}

func TestMergeSkills_WithOverlap(t *testing.T) {
	agent := []string{"solid_principles", "clean_code"}
	global := []string{"clean_code", "function_generation", "solid_principles"}

	result := MergeSkills(agent, global)
	if len(result) != 3 {
		t.Errorf("expected 3 skills (deduplicated), got %d", len(result))
	}
	if result[0] != "solid_principles" || result[1] != "clean_code" || result[2] != "function_generation" {
		t.Errorf("unexpected result: %v", result)
	}
}

func TestMergeSkills_EmptyAgent(t *testing.T) {
	result := MergeSkills(nil, []string{"solid_principles", "clean_code"})
	if len(result) != 2 {
		t.Errorf("expected 2 skills, got %d", len(result))
	}
}

func TestMergeSkills_EmptyGlobal(t *testing.T) {
	result := MergeSkills([]string{"solid_principles", "clean_code"}, nil)
	if len(result) != 2 {
		t.Errorf("expected 2 skills, got %d", len(result))
	}
}

func TestMergeSkills_BothEmpty(t *testing.T) {
	result := MergeSkills(nil, nil)
	if len(result) != 0 {
		t.Errorf("expected 0 skills, got %d", len(result))
	}
}

func TestMergeSkills_PreservesOrder(t *testing.T) {
	agent := []string{"a", "b", "c"}
	global := []string{"d", "e", "f"}

	result := MergeSkills(agent, global)
	expected := []string{"a", "b", "c", "d", "e", "f"}
	if len(result) != len(expected) {
		t.Fatalf("expected %d skills, got %d", len(expected), len(result))
	}
	for i, exp := range expected {
		if result[i] != exp {
			t.Errorf("at index %d: expected %q, got %q", i, exp, result[i])
		}
	}
}
