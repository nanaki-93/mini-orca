package skills

// Library provides the complete set of predefined skills available to agents.

// Knowledge skills provide domain expertise and guidance.
var KnowledgeSkills = []Skill{
	{
		Name:        "solid_principles",
		Type:        Knowledge,
		Category:    Principles,
		Description: "Applies SOLID principles (Single Responsibility, Open/Closed, Liskov Substitution, Interface Segregation, Dependency Inversion) to software design.",
		PromptTemplate: `Apply SOLID principles to the following code/task:
- Single Responsibility: Each module/class should have one reason to change
- Open/Closed: Design for extension without modification
- Liskov Substitution: Subtypes must be substitutable for base types
- Interface Segregation: Prefer specific interfaces over general-purpose ones
- Dependency Inversion: Depend on abstractions, not concretions

Task: {{task}}`,
		Parameters: map[string]string{"domain": "software_design"},
	},
	{
		Name:        "clean_code",
		Type:        Knowledge,
		Category:    Principles,
		Description: "Enforces clean code practices including meaningful naming, small functions, consistent formatting, and readable structure.",
		PromptTemplate: `Review the following code for clean code practices:
- Use meaningful, intention-revealing names
- Functions should be small and do one thing
- Follow consistent formatting and style
- Minimize cognitive load through clear structure
- Avoid magic numbers and strings

Code: {{code}}`,
		Parameters: map[string]string{"domain": "code_quality"},
	},
	{
		Name:        "kiss_principle",
		Type:        Knowledge,
		Category:    Principles,
		Description: "Enforces the KISS principle — keep it simple, stupid. Avoid over-engineering and unnecessary complexity.",
		PromptTemplate: `Apply the KISS principle to the following design/task:
- Choose the simplest solution that works
- Avoid premature optimization
- Prefer readability over cleverness
- Minimize dependencies and abstractions
- Question every line of code — does it add value?

Task: {{task}}`,
		Parameters: map[string]string{"domain": "simplicity"},
	},
	{
		Name:        "no_repetition",
		Type:        Knowledge,
		Category:    Principles,
		Description: "Enforces DRY (Don't Repeat Yourself) principle — eliminate duplication through abstraction and reuse.",
		PromptTemplate: `Apply DRY principles to eliminate repetition:
- Identify duplicated code patterns
- Extract common logic into reusable functions/modules
- Use appropriate abstractions without over-engineering
- Maintain a single source of truth for each piece of knowledge

Code/Design: {{code}}`,
		Parameters: map[string]string{"domain": "code_quality"},
	},
	{
		Name:        "business_logic_adherence",
		Type:        Knowledge,
		Category:    Principles,
		Description: "Ensures implementation strictly adheres to business requirements and domain rules.",
		PromptTemplate: `Verify that the implementation adheres to business requirements:
- Every business rule must be explicitly implemented
- Domain terms should map directly to code names (ubiquitous language)
- Business invariants must be enforced
- Edge cases related to business rules must be handled

Requirements: {{requirements}}
Implementation: {{code}}`,
		Parameters: map[string]string{"domain": "domain_driven_design"},
	},
	{
		Name:        "architecture_design",
		Type:        Knowledge,
		Category:    Design,
		Description: "Guides architectural decisions including layering, separation of concerns, and system boundaries.",
		PromptTemplate: `Design the architecture following these principles:
- Separate concerns into clear layers (presentation, business, data)
- Minimize coupling between modules
- Define clear interfaces between components
- Consider scalability and maintainability
- Document architectural decisions and rationale

System: {{system_description}}`,
		Parameters: map[string]string{"domain": "architecture"},
	},
	{
		Name:        "task_breakdown",
		Type:        Knowledge,
		Category:    Design,
		Description: "Breaks complex tasks into smaller, manageable, and logically ordered subtasks.",
		PromptTemplate: `Break down the following task into smaller subtasks:
- Identify independent units of work
- Define clear dependencies between subtasks
- Ensure each subtask is testable and verifiable
- Order subtasks by dependency and priority
- Estimate complexity for each subtask

Task: {{task}}`,
		Parameters: map[string]string{"domain": "project_management"},
	},
	{
		Name:        "dependency_mapping",
		Type:        Knowledge,
		Category:    Design,
		Description: "Maps and analyzes dependencies between modules, packages, and external services.",
		PromptTemplate: `Analyze and map dependencies for the following system:
- Identify internal module dependencies
- List external package/service dependencies
- Detect circular dependencies
- Assess dependency risk (versioning, maintenance, security)
- Suggest dependency reductions where possible

System: {{system_description}}`,
		Parameters: map[string]string{"domain": "system_analysis"},
	},
}

// Coding skills provide implementation capabilities.
var CodingSkills = []Skill{
	{
		Name:        "function_generation",
		Type:        Tool,
		Category:    Coding,
		Description: "Generates well-structured, documented functions with proper error handling and edge case coverage.",
		PromptTemplate: `Generate a function with the following specification:
- Name: {{function_name}}
- Input: {{input_signature}}
- Output: {{output_signature}}
- Purpose: {{purpose}}
- Constraints: {{constraints}}

Requirements:
- Add clear documentation/comments
- Handle all edge cases and errors
- Use appropriate error handling patterns
- Keep the function focused and small`,
		Parameters: map[string]string{"language": "go", "style": "idiomatic"},
	},
	{
		Name:        "struct_design",
		Type:        Tool,
		Category:    Coding,
		Description: "Designs Go structs with proper fields, embedded types, and interface implementations.",
		PromptTemplate: `Design a Go struct with the following specification:
- Name: {{struct_name}}
- Purpose: {{purpose}}
- Required fields: {{fields}}
- Methods needed: {{methods}}

Requirements:
- Use appropriate visibility (exported vs unexported)
- Consider embedding for composition
- Follow Go naming conventions
- Document public fields and methods`,
		Parameters: map[string]string{"language": "go", "style": "idiomatic"},
	},
	{
		Name:        "class_creation",
		Type:        Tool,
		Category:    Coding,
		Description: "Creates Go structs with methods and interfaces that form cohesive, well-encapsulated types.",
		PromptTemplate: `Create a Go type (struct) with methods:
- Name: {{type_name}}
- Purpose: {{purpose}}
- State: {{fields}}
- Behavior: {{methods}}
- Interfaces to implement: {{interfaces}}

Requirements:
- Encapsulate internal state
- Provide clear public API
- Implement relevant interfaces
- Keep related methods grouped together`,
		Parameters: map[string]string{"language": "go", "style": "idiomatic"},
	},
}

// Testing skills provide verification capabilities.
var TestingSkills = []Skill{
	{
		Name:        "unit_testing",
		Type:        Tool,
		Category:    Testing,
		Description: "Creates focused unit tests that verify individual functions and methods in isolation.",
		PromptTemplate: `Generate unit tests for the following code:
- Function/Method: {{target}}
- Input cases: {{input_cases}}
- Expected outputs: {{expected_outputs}}

Requirements:
- Use table-driven tests (Go convention)
- Cover happy path, edge cases, and error cases
- Keep tests independent and deterministic
- Use meaningful test names that describe the scenario`,
		Parameters: map[string]string{"framework": "testing", "style": "table_driven"},
	},
	{
		Name:        "integration_testing",
		Type:        Tool,
		Category:    Testing,
		Description: "Creates integration tests that verify interactions between multiple components or services.",
		PromptTemplate: `Generate integration tests for the following components:
- Components: {{components}}
- Interactions: {{interactions}}
- Expected outcomes: {{expected_outcomes}}

Requirements:
- Test real interactions, not mocks
- Use test databases or services where appropriate
- Clean up test fixtures after each test
- Assert on complete workflows, not individual steps`,
		Parameters: map[string]string{"framework": "testing", "style": "end_to_end"},
	},
	{
		Name:        "coverage_analysis",
		Type:        Tool,
		Category:    Testing,
		Description: "Analyzes test coverage and identifies untested code paths and branches.",
		PromptTemplate: `Analyze test coverage for the following code:
- Code: {{code}}
- Existing tests: {{tests}}

Identify:
- Uncovered branches and conditions
- Missing edge case tests
- Areas with low logical coverage
- Recommendations for additional tests`,
		Parameters: map[string]string{"tool": "go test -cover", "threshold": "80"},
	},
	{
		Name:        "test_generation",
		Type:        Tool,
		Category:    Testing,
		Description: "Generates comprehensive test suites including unit, integration, and property-based tests.",
		PromptTemplate: `Generate a complete test suite for the following code:
- Code: {{code}}
- Type: {{type}} (function/struct/method/interface)
- Dependencies: {{dependencies}}

Include:
- Unit tests with table-driven test cases
- Error path tests
- Concurrency tests if applicable
- Benchmark tests for performance-critical code
- Test helpers and fixtures`,
		Parameters: map[string]string{"framework": "testing", "style": "comprehensive"},
	},
}

// Review skills provide quality assurance capabilities.
var ReviewSkills = []Skill{
	{
		Name:        "style_check",
		Type:        Tool,
		Category:    Review,
		Description: "Checks code against style guidelines including formatting, naming conventions, and idiomatic patterns.",
		PromptTemplate: `Review the following code for style compliance:
- Code: {{code}}
- Style guide: {{style_guide}}

Check for:
- Naming conventions (Go idioms)
- Formatting and indentation
- Import ordering
- Consistent error handling patterns
- Unnecessary complexity or verbosity`,
		Parameters: map[string]string{"linter": "gofmt,golint,gocritic", "strict": "true"},
	},
	{
		Name:        "logic_review",
		Type:        Tool,
		Category:    Review,
		Description: "Reviews code logic for correctness, edge cases, and potential bugs.",
		PromptTemplate: `Review the following code for logical correctness:
- Code: {{code}}
- Purpose: {{purpose}}
- Input constraints: {{constraints}}

Check for:
- Correctness of business logic
- Edge case handling
- Off-by-one errors and boundary conditions
- Race conditions and concurrency issues
- Resource leaks (goroutines, connections, files)`,
		Parameters: map[string]string{"depth": "thorough", "focus": "correctness"},
	},
	{
		Name:        "security_audit",
		Type:        Tool,
		Category:    Review,
		Description: "Audits code for security vulnerabilities including injection, authentication, and data exposure.",
		PromptTemplate: `Perform a security audit on the following code:
- Code: {{code}}
- Context: {{context}}

Check for:
- SQL/Command injection vulnerabilities
- Improper input validation and sanitization
- Authentication and authorization issues
- Sensitive data exposure (logs, errors, responses)
- Unsafe crypto usage
- Dependency vulnerabilities`,
		Parameters: map[string]string{"standard": "OWASP", "severity": "high"},
	},
}

// AllSkills returns every predefined skill in the library.
func AllSkills() []Skill {
	var all []Skill
	all = append(all, KnowledgeSkills...)
	all = append(all, CodingSkills...)
	all = append(all, TestingSkills...)
	all = append(all, ReviewSkills...)
	return all
}

// SkillsByType returns skills filtered by type.
func SkillsByType(st SkillType) []Skill {
	var result []Skill
	for _, s := range AllSkills() {
		if s.Type == st {
			result = append(result, s)
		}
	}
	return result
}

// SkillsByCategory returns skills filtered by category.
func SkillsByCategory(cat SkillCategory) []Skill {
	var result []Skill
	for _, s := range AllSkills() {
		if s.Category == cat {
			result = append(result, s)
		}
	}
	return result
}
