package skills

import (
	"mini-orca/internal/types"
)

// Library contains all predefined skills for Mini-Orca agents.
func Library() []types.Skill {
	return []types.Skill{
		// Knowledge Skills
		{
			Name:        "solid_principles",
			Description: "Apply SOLID principles (Single Responsibility, Open/Closed, Liskov Substitution, Interface Segregation, Dependency Inversion) in code design.",
			Type:        types.SkillTypeKnowledge,
			Priority:    1,
			Prompt:      "Ensure all code follows SOLID principles. Each component should have a single responsibility, be open for extension but closed for modification, and dependencies should be inverted where appropriate.",
		},
		{
			Name:        "clean_code",
			Description: "Write clean, readable, maintainable code following industry best practices.",
			Type:        types.SkillTypeKnowledge,
			Priority:    1,
			Prompt:      "Write code that is clean and readable. Use meaningful names, keep functions small, avoid magic numbers, and write self-documenting code.",
		},
		{
			Name:        "kiss_principle",
			Description: "Keep It Simple, Stupid - prefer simple solutions over complex ones.",
			Type:        types.SkillTypeKnowledge,
			Priority:    2,
			Prompt:      "Apply the KISS principle. Choose the simplest solution that works. Avoid over-engineering and unnecessary abstractions.",
		},
		{
			Name:        "no_repetition",
			Description: "Don't Repeat Yourself (DRY) - eliminate code duplication.",
			Type:        types.SkillTypeKnowledge,
			Priority:    2,
			Prompt:      "Follow the DRY principle. Extract common code into reusable functions or components. Never duplicate logic.",
		},
		{
			Name:        "business_logic_adherence",
			Description: "Ensure code accurately implements the specified business requirements.",
			Type:        types.SkillTypeKnowledge,
			Priority:    1,
			Prompt:      "Always prioritize correct implementation of business logic. The code must accurately reflect the requirements and handle business rules correctly.",
		},
		{
			Name:        "architecture_design",
			Description: "Design coherent system architecture with proper component relationships.",
			Type:        types.SkillTypeKnowledge,
			Priority:    1,
			Prompt:      "Design architecture that is modular, scalable, and maintainable. Consider component relationships, data flow, and separation of concerns.",
		},
		{
			Name:        "task_breakdown",
			Description: "Break complex problems into manageable, atomic units of work.",
			Type:        types.SkillTypeKnowledge,
			Priority:    1,
			Prompt:      "Break down complex requirements into atomic, independent units. Each unit should be implementable, testable, and reviewable independently.",
		},
		{
			Name:        "dependency_mapping",
			Description: "Identify and map dependencies between components and modules.",
			Type:        types.SkillTypeKnowledge,
			Priority:    2,
			Prompt:      "Map all dependencies between components. Identify circular dependencies and resolve them. Plan implementation order based on dependency chains.",
		},
		{
			Name:        "error_handling",
			Description: "Implement robust error handling and recovery mechanisms.",
			Type:        types.SkillTypeKnowledge,
			Priority:    1,
			Prompt:      "Implement comprehensive error handling. Use appropriate error types, provide meaningful error messages, and handle edge cases gracefully.",
		},
		{
			Name:        "performance_considerations",
			Description: "Consider performance implications of design and implementation choices.",
			Type:        types.SkillTypeKnowledge,
			Priority:    2,
			Prompt:      "Consider time and space complexity. Avoid unnecessary allocations, optimize hot paths, and choose appropriate data structures.",
		},

		// Tool Skills
		{
			Name:        "function_generation",
			Description: "Generate complete, production-ready functions.",
			Type:        types.SkillTypeTool,
			Priority:    1,
			Prompt:      "Generate complete functions with proper signatures, documentation, error handling, and edge case coverage. Each function should be self-contained and well-documented.",
		},
		{
			Name:        "struct_design",
			Description: "Design efficient and well-structured data structures.",
			Type:        types.SkillTypeTool,
			Priority:    1,
			Prompt:      "Design structs that are well-organized, use appropriate types, follow naming conventions, and support the required operations efficiently.",
		},
		{
			Name:        "class_creation",
			Description: "Create well-structured classes with proper encapsulation.",
			Type:        types.SkillTypeTool,
			Priority:    1,
			Prompt:      "Create classes with proper encapsulation, clear interfaces, and well-defined responsibilities. Follow object-oriented best practices.",
		},
		{
			Name:        "unit_testing",
			Description: "Write comprehensive unit tests for individual components.",
			Type:        types.SkillTypeTool,
			Priority:    1,
			Prompt:      "Write unit tests that cover happy paths, edge cases, and error conditions. Use descriptive test names and follow the Arrange-Act-Assert pattern.",
		},
		{
			Name:        "integration_testing",
			Description: "Write tests that verify component interactions.",
			Type:        types.SkillTypeTool,
			Priority:    2,
			Prompt:      "Write integration tests that verify correct behavior when components interact. Test data flow between components and handle test setup/teardown properly.",
		},
		{
			Name:        "coverage_analysis",
			Description: "Analyze and improve test coverage.",
			Type:        types.SkillTypeTool,
			Priority:    2,
			Prompt:      "Analyze test coverage and identify gaps. Focus on untested branches, error paths, and edge cases. Aim for meaningful coverage, not just high percentages.",
		},
		{
			Name:        "test_generation",
			Description: "Generate test code from specifications.",
			Type:        types.SkillTypeTool,
			Priority:    1,
			Prompt:      "Generate comprehensive test code from specifications. Include tests for normal operation, error conditions, and edge cases.",
		},
		{
			Name:        "style_check",
			Description: "Check code against style guides and formatting standards.",
			Type:        types.SkillTypeTool,
			Priority:    2,
			Prompt:      "Check code against established style guides. Ensure consistent formatting, naming conventions, and code organization.",
		},
		{
			Name:        "logic_review",
			Description: "Review code logic for correctness and efficiency.",
			Type:        types.SkillTypeTool,
			Priority:    1,
			Prompt:      "Review code logic thoroughly. Check for correctness, efficiency, and potential bugs. Verify that the implementation matches the requirements.",
		},
		{
			Name:        "security_audit",
			Description: "Check code for security vulnerabilities.",
			Type:        types.SkillTypeTool,
			Priority:    1,
			Prompt:      "Audit code for common security vulnerabilities: injection attacks, improper input validation, insecure defaults, information leakage, and proper authentication/authorization.",
		},
	}
}

// GetSkillsByName returns skills matching the given names.
func GetSkillsByName(names []string) []types.Skill {
	allSkills := Library()
	result := make([]types.Skill, 0, len(names))
	
	for _, name := range names {
		for _, s := range allSkills {
			if s.Name == name {
				result = append(result, s)
				break
			}
		}
	}
	
	return result
}

// GetSkillsByType returns skills of a specific type.
func GetSkillsByType(t types.SkillType) []types.Skill {
	allSkills := Library()
	result := make([]types.Skill, 0)
	
	for _, s := range allSkills {
		if s.Type == t {
			result = append(result, s)
		}
	}
	
	return result
}
