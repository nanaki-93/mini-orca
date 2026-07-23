# Mini-Orca v2.0 — Agent Skills System

## 1. Overview

Each agent has its own **configurable set of skills**. Skills come in two types:

- **Knowledge Skills** — Guidelines and principles that shape how the agent thinks and generates output (passed via prompts)
- **Tool Skills** — Capabilities the agent can execute (shell commands, file operations, etc.)

Skills are **configurable per agent** via `config.yaml`. They can be simple flat lists now, with hierarchical structure possible in the future.

---

## 2. Skill Types

```go
type SkillType string

const (
    SkillKnowledge SkillType = "knowledge"  // Guidelines for agent behavior
    SkillTool      SkillType = "tool"       // Executable capabilities
)

type Skill struct {
    ID          string       `json:"id"`           // Unique identifier
    Name        string       `json:"name"`         // Human-readable name
    Description string       `json:"description"`  // What this skill does
    Type        SkillType    `json:"type"`         // knowledge | tool
    Enabled     bool         `json:"enabled"`      // Whether this skill is active
    Priority    int          `json:"priority"`     // Order of application (lower = first)
    
    // For knowledge skills
    PromptTemplate string `json:"prompt_template,omitempty"` // How to include in agent prompt
    
    // For tool skills
    ToolName     string   `json:"tool_name,omitempty"`     // Tool identifier
    ToolArgs     []string `json:"tool_args,omitempty"`     // Default arguments
}
```

---

## 3. Predefined Skills Library

### 3.1 Knowledge Skills

#### SOLID Principles
```yaml
skills:
  solid_principles:
    id: "solid_principles"
    name: "SOLID Principles"
    description: "Follow SOLID principles in all code generation"
    type: knowledge
    priority: 1
    prompt_template: |
      SOLID PRINCIPLES - Apply to ALL code:
      1. Single Responsibility: Each function/struct/class should have ONE reason to change
      2. Open/Closed: Design for extension without modification
      3. Liskov Substitution: Subtypes must be substitutable for base types
      4. Interface Segregation: Prefer many specific interfaces over one general-purpose
      5. Dependency Inversion: Depend on abstractions, not concretions
```

#### Clean Code
```yaml
skills:
  clean_code:
    id: "clean_code"
    name: "Clean Code"
    description: "Write clean, readable, maintainable code"
    type: knowledge
    priority: 2
    prompt_template: |
      CLEAN CODE GUIDELINES:
      - Use meaningful, descriptive names for functions, variables, and types
      - Functions should be small (max 20 lines) and do one thing well
      - Use consistent formatting and indentation
      - Add comments only when code is not self-explanatory
      - Follow language-specific conventions and idioms
      - Handle errors explicitly and gracefully
      - Avoid magic numbers and strings - use constants
```

#### KISS Principle
```yaml
skills:
  kiss_principle:
    id: "kiss_principle"
    name: "KISS Principle"
    description: "Keep It Simple, Stupid"
    type: knowledge
    priority: 3
    prompt_template: |
      KISS PRINCIPLE:
      - Prefer simple solutions over complex ones
      - Avoid over-engineering
      - Use the simplest approach that solves the problem
      - Don't use design patterns unless they clearly simplify the code
      - If a solution requires extensive explanation, it's probably too complex
```

#### No Repetition (DRY)
```yaml
skills:
  no_repetition:
    id: "no_repetition"
    name: "No Repetition (DRY)"
    description: "Avoid code duplication"
    type: knowledge
    priority: 4
    prompt_template: |
      DRY PRINCIPLE (Don't Repeat Yourself):
      - Extract common patterns into reusable functions, structs, or classes
      - If you see similar code in two places, factor it out
      - Use generics or interfaces when multiple types need similar behavior
      - Create helper functions for repeated logic
      - Maintain a single source of truth for any piece of knowledge
```

#### Business Logic Adherence
```yaml
skills:
  business_logic_adherence:
    id: "business_logic_adherence"
    name: "Business Logic Adherence"
    description: "Always follow the business logic defined in the business_logic.md file"
    type: knowledge
    priority: 0  // Highest priority - always first
    prompt_template: |
      BUSINESS LOGIC ADHERENCE:
      - ALWAYS read and follow the business_logic.md file in the project root
      - All code must conform to the business rules defined in that file
      - If there's a conflict between business logic and other considerations, business logic wins
      - If business_logic.md doesn't exist, ask for clarification before proceeding
      - Document any business logic decisions in comments
```

### 3.2 Tool Skills

#### Shell Command Execution
```yaml
skills:
  shell_execute:
    id: "shell_execute"
    name: "Shell Command Execution"
    description: "Execute shell commands"
    type: tool
    tool_name: "shell"
    tool_args: ["timeout", "30"]
```

#### File Read
```yaml
skills:
  file_read:
    id: "file_read"
    name: "File Read"
    description: "Read file contents"
    type: tool
    tool_name: "file_read"
```

#### File Write (Atomic)
```yaml
skills:
  file_write_atomic:
    id: "file_write_atomic"
    name: "Atomic File Write"
    description: "Write ONE function/struct/class to file atomically"
    type: tool
    tool_name: "file_write_atomic"
```

#### Run Tests
```yaml
skills:
  run_tests:
    id: "run_tests"
    name: "Run Tests"
    description: "Execute project test suite"
    type: tool
    tool_name: "test_runner"
```

#### Format Code
```yaml
skills:
  format_code:
    id: "format_code"
    name: "Format Code"
    description: "Format code using language-specific formatter"
    type: tool
    tool_name: "code_formatter"
```

---

## 4. Agent-Specific Skills

### 4.1 Planner Agent Skills

```yaml
agents:
  planner:
    model: "lm-studio"
    model_name: "qwen/qwen3-coder-30b"
    temperature: 0.3
    max_tokens: 4096
    skills:
      # Knowledge skills
      - solid_principles
      - clean_code
      - kiss_principle
      - business_logic_adherence
      
      # Tool skills
      - file_read
      - shell_execute
      
      # Planner-specific knowledge
      - architecture_design
      - task_breakdown
      - dependency_mapping
```

**Planner Prompt Template:**
```
You are an expert software architect. Your skills include:

{solid_principles_prompt}
{clean_code_prompt}
{kiss_principle_prompt}
{business_logic_adherence_prompt}

Additional capabilities:
- Architecture Design: Create clear system architecture with module boundaries
- Task Breakdown: Decompose requirements into atomic units (functions, structs, classes)
- Dependency Mapping: Identify and document component dependencies

Your task: Analyze the user's project goal and create a detailed implementation plan.
```

### 4.2 Coder Agent Skills

```yaml
agents:
  coder:
    model: "lm-studio"
    model_name: "qwen/qwen3-coder-30b"
    temperature: 0.1
    max_tokens: 8192
    skills:
      # Knowledge skills
      - solid_principles
      - clean_code
      - kiss_principle
      - no_repetition
      
      # Tool skills
      - file_read
      - file_write_atomic
      - format_code
      
      # Coder-specific knowledge
      - function_generation
      - struct_design
      - class_creation
```

**Coder Prompt Template:**
```
You are an expert software developer. Your skills include:

{solid_principles_prompt}
{clean_code_prompt}
{kiss_principle_prompt}
{no_repetition_prompt}

Additional capabilities:
- Function Generation: Write well-structured functions with proper signatures
- Struct Design: Create appropriate data structures
- Class Creation: Build well-encapsulated classes (for OOP languages)

Your task: Implement ONE atomic unit (function, struct, or class) based on the plan.
Output ONLY the code for this single unit.
```

### 4.3 Tester Agent Skills

```yaml
agents:
  tester:
    model: "lm-studio"
    model_name: "qwen/qwen3-coder-30b"
    temperature: 0.2
    max_tokens: 2048
    skills:
      # Knowledge skills
      - clean_code
      
      # Tool skills
      - run_tests
      - file_read
      - shell_execute
      
      # Tester-specific knowledge
      - unit_testing
      - integration_testing
      - coverage_analysis
      - test_generation
```

**Tester Prompt Template:**
```
You are an expert QA engineer. Your skills include:

{clean_code_prompt}

Additional capabilities:
- Unit Testing: Write and execute unit tests
- Integration Testing: Verify component interactions
- Coverage Analysis: Analyze test coverage
- Test Generation: Generate comprehensive test cases

Your task: Run tests on the generated code and provide a detailed report.
```

### 4.4 Reviewer Agent Skills

```yaml
agents:
  reviewer:
    model: "lm-studio"
    model_name: "qwen/qwen3-coder-30b"
    temperature: 0.2
    max_tokens: 4096
    skills:
      # Knowledge skills
      - solid_principles
      - clean_code
      - business_logic_adherence
      
      # Tool skills
      - file_read
      
      # Reviewer-specific knowledge
      - style_check
      - logic_review
      - security_audit
```

**Reviewer Prompt Template:**
```
You are an expert code reviewer. Your skills include:

{solid_principles_prompt}
{clean_code_prompt}
{business_logic_adherence_prompt}

Additional capabilities:
- Style Check: Verify code style consistency
- Logic Review: Check code logic for correctness
- Security Audit: Review for security vulnerabilities

Your task: Review the generated code and provide a detailed assessment.
```

---

## 5. Skills System Architecture

### 5.1 Skills Registry

```go
// internal/agent/skills/skills.go

type SkillsRegistry struct {
    library map[string]Skill  // Skill ID -> Skill definition
    enabled map[string]bool   // Skill ID -> enabled status
}

func (r *SkillsRegistry) GetEnabledSkills(agentType string) []Skill {
    // Return enabled skills for the given agent type
    // ...
}

func (r *SkillsRegistry) GetSkillPrompt(skillID string) string {
    // Get the prompt template for a knowledge skill
    // ...
}
```

### 5.2 Skills Integration with Agents

```go
// internal/agent/planner.go

type PlannerAgent struct {
    Model    model.Config
    Skills   *skills.SkillsRegistry
    Prompt   PromptTemplate
}

func (a *PlannerAgent) Execute(ctx context.Context, input Input) (*Plan, error) {
    // Get enabled skills
    enabledSkills := a.Skills.GetEnabledSkills("planner")
    
    // Build prompt with skill descriptions
    prompt := a.buildPrompt(input, enabledSkills)
    
    // Execute with model
    response, err := a.Model.Execute(ctx, prompt)
    if err != nil {
        return nil, err
    }
    
    // Parse response
    plan, err := a.parseResponse(response)
    if err != nil {
        return nil, err
    }
    
    return plan, nil
}

func (a *PlannerAgent) buildPrompt(input Input, skills []Skill) string {
    prompt := "You are an expert software architect.\n\n"
    
    // Add skill prompts
    for _, skill := range skills {
        if skill.Type == SkillKnowledge {
            skillPrompt := a.Skills.GetSkillPrompt(skill.ID)
            prompt += skillPrompt + "\n\n"
        }
    }
    
    // Add task-specific instructions
    prompt += input.TaskDescription
    
    return prompt
}
```

---

## 6. Configuring Skills

### 6.1 Enable/Disable Skills

```yaml
# config.yaml - Enable/disable skills per agent
agents:
  planner:
    skills:
      - solid_principles: enabled: true
      - clean_code: enabled: true
      - kiss_principle: enabled: false  # Disabled for this agent
      - business_logic_adherence: enabled: true
```

### 6.2 Add Custom Skills

```yaml
# Add custom skills to the library
skills:
  my_custom_skill:
    id: "my_custom_skill"
    name: "My Custom Skill"
    description: "A custom skill for my project"
    type: knowledge
    priority: 5
    prompt_template: |
      MY CUSTOM SKILL:
      - Always do X
      - Never do Y
      - Prefer Z
```

### 6.3 Skill Priority

Skills are applied in priority order (lower number = applied first):

```yaml
skills:
  business_logic_adherence:
    priority: 0  # Always applied first
  
  solid_principles:
    priority: 1
  
  clean_code:
    priority: 2
  
  kiss_principle:
    priority: 3
```

---

## 7. Future: Hierarchical Skills

The system is designed to support hierarchical skills in the future:

```yaml
# Future structure (not implemented yet)
skills:
  solid_principles:
    type: knowledge
    children:
      - single_responsibility
      - open_closed
      - liskov_substitution
      - interface_segregation
      - dependency_inversion
```

This would allow:
- Enabling/disabling parent skills to enable/disable all children
- Different priority levels for sub-skills
- More granular control

---

## 8. Skills Validation

```go
// Validate that all enabled skills exist in the library
func (r *SkillsRegistry) Validate(agentSkills []string) error {
    for _, skillID := range agentSkills {
        if _, exists := r.library[skillID]; !exists {
            return fmt.Errorf("skill not found in library: %s", skillID)
        }
    }
    return nil
}

// Check for conflicting skills
func (r *SkillsRegistry) CheckConflicts(agentSkills []string) []string {
    conflicts := []string{}
    
    // Example: business_logic_adherence should always be enabled if present
    // If disabled, warn the user
    for _, skillID := range agentSkills {
        if skillID == "business_logic_adherence" {
            if !r.enabled[skillID] {
                conflicts = append(conflicts, 
                    "business_logic_adherence is disabled - business logic may be ignored")
            }
        }
    }
    
    return conflicts
}
```
