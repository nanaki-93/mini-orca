// Package skills provides the skills system for Mini-Orca agents.
//
// Skills are reusable capabilities that enhance agent behavior. Each skill
// has a name, description, type (knowledge or tool), and optional prompt
// guidance that is injected into the agent's context.
//
// # Predefined Skills
//
// The library package contains 20 predefined skills:
//
// Knowledge Skills:
//   - solid_principles: SOLID design principles
//   - clean_code: Clean code practices
//   - kiss_principle: Keep It Simple, Stupid
//   - business_logic_adherence: Domain logic correctness
//   - security_audit: Security best practices
//   - style_check: Code style consistency
//   - logic_review: Logical correctness
//   - architecture_design: System architecture
//
// Tool Skills:
//   - function_generation: Generate standalone functions
//   - struct_design: Design Go structs
//   - class_creation: Create Java/Kotlin classes
//   - unit_testing: Write unit tests
//   - integration_testing: Write integration tests
//   - coverage_analysis: Analyze test coverage
//   - test_generation: Generate test cases
//   - dependency_mapping: Map dependencies
//   - task_breakdown: Break tasks into units
//
// # Custom Skills
//
// Users can add custom skills via the API or by editing the configuration.
// Custom skills are stored in the skills registry and can be exported/imported
// as JSON.
//
// # Example
//
//	library := skills.NewLibrary()
//	skills := library.GetSkills("solid_principles", "clean_code")
//
//	registry := skills.NewRegistry()
//	registry.Register(library)
//
//	skill, _ := registry.Get("solid_principles")
package skills
