package skills

import (
	"testing"
)

func TestNewSkillsRegistry_PrefillsAllSkills(t *testing.T) {
	registry := NewSkillsRegistry()

	// Should have all 18 skills pre-registered
	allSkills := AllSkills()
	if len(allSkills) != 18 {
		t.Fatalf("expected 18 skills in library, got %d", len(allSkills))
	}

	// Verify we can get each skill by name
	for _, skill := range allSkills {
		s, err := registry.Get(skill.Name)
		if err != nil {
			t.Fatalf("failed to get skill %q: %v", skill.Name, err)
		}
		if s.Name != skill.Name {
			t.Errorf("expected skill name %q, got %q", skill.Name, s.Name)
		}
	}
}

func TestSkillsRegistry_RegisterAndGet(t *testing.T) {
	registry := NewSkillsRegistry()

	skill := Skill{
		Name:           "custom_skill",
		Type:           Knowledge,
		Category:       Principles,
		Description:    "A custom skill for testing",
		PromptTemplate: "Test template",
		Parameters:     map[string]string{"key": "value"},
	}

	err := registry.Register(skill)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	got, err := registry.Get("custom_skill")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if got.Name != "custom_skill" {
		t.Errorf("expected name custom_skill, got %s", got.Name)
	}
	if got.Type != Knowledge {
		t.Errorf("expected type Knowledge, got %s", got.Type)
	}
	if got.Category != Principles {
		t.Errorf("expected category Principles, got %s", got.Category)
	}
	if got.Description != "A custom skill for testing" {
		t.Errorf("expected description 'A custom skill for testing', got %s", got.Description)
	}
	if got.Parameters["key"] != "value" {
		t.Errorf("expected parameter value 'value', got %s", got.Parameters["key"])
	}
}

func TestSkillsRegistry_RegisterDuplicate(t *testing.T) {
	registry := NewSkillsRegistry()

	skill := Skill{
		Name:           "duplicate_skill",
		Type:           Knowledge,
		Category:       Principles,
		Description:    "Test",
		PromptTemplate: "Test",
		Parameters:     map[string]string{},
	}

	// First registration should succeed
	if err := registry.Register(skill); err != nil {
		t.Fatalf("first Register failed: %v", err)
	}

	// Second registration should fail
	err := registry.Register(skill)
	if err == nil {
		t.Fatal("expected error for duplicate registration, got nil")
	}
}

func TestSkillsRegistry_GetNotFound(t *testing.T) {
	registry := NewSkillsRegistry()

	_, err := registry.Get("nonexistent_skill")
	if err == nil {
		t.Fatal("expected error for nonexistent skill, got nil")
	}
}

func TestSkillsRegistry_GetByCategory(t *testing.T) {
	registry := NewSkillsRegistry()

	// Test each category
	categories := []SkillCategory{Design, Coding, Testing, Review, Principles}
	expectedCounts := map[SkillCategory]int{
		Design:     3, // architecture_design, task_breakdown, dependency_mapping
		Coding:     3, // function_generation, struct_design, class_creation
		Testing:    4, // unit_testing, integration_testing, coverage_analysis, test_generation
		Review:     3, // style_check, logic_review, security_audit
		Principles: 5, // solid_principles, clean_code, kiss_principle, no_repetition, business_logic_adherence
	}

	for _, cat := range categories {
		skills := registry.GetByCategory(cat)
		expected := expectedCounts[cat]
		if len(skills) != expected {
			t.Errorf("expected %d skills in category %q, got %d", expected, cat, len(skills))
		}

		// Verify all returned skills belong to the category
		for _, skill := range skills {
			if skill.Category != cat {
				t.Errorf("skill %q has category %q, expected %q", skill.Name, skill.Category, cat)
			}
		}
	}
}

func TestSkillsRegistry_GetForAgent(t *testing.T) {
	registry := NewSkillsRegistry()

	tests := []struct {
		agentName     string
		expectedCat   SkillCategory
		expectedCount int
	}{
		{"coder", Coding, 3},
		{"tester", Testing, 4},
		{"reviewer", Review, 3},
		{"unknown_agent", Principles, 5},
	}

	for _, tt := range tests {
		t.Run(tt.agentName, func(t *testing.T) {
			skills := registry.GetForAgent(tt.agentName)
			if len(skills) != tt.expectedCount {
				t.Errorf("expected %d skills for agent %q, got %d", tt.expectedCount, tt.agentName, len(skills))
			}

			// Verify all skills belong to the expected category
			for _, skill := range skills {
				if skill.Category != tt.expectedCat {
					t.Errorf("skill %q has category %q, expected %q", skill.Name, skill.Category, tt.expectedCat)
				}
			}
		})
	}
}

func TestSkillsRegistry_ThreadSafety(t *testing.T) {
	registry := NewSkillsRegistry()

	done := make(chan bool)

	// Concurrent reads
	go func() {
		for range 100 {
			_, _ = registry.Get("solid_principles")
			_ = registry.GetByCategory(Design)
			_ = registry.GetForAgent("coding")
		}
		done <- true
	}()

	// Concurrent writes
	go func() {
		for range 10 {
			skill := Skill{
				Name:           "dynamic_skill",
				Type:           Knowledge,
				Category:       Principles,
				Description:    "Dynamic",
				PromptTemplate: "Test",
				Parameters:     map[string]string{},
			}
			_ = registry.Register(skill)
		}
		done <- true
	}()

	<-done
	<-done
}
