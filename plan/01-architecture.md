# Mini-Orca v2.0 — System Architecture

## 1. High-Level Architecture

```
┌──────────────────────────────────────────────────────────────────────────────────┐
│                    IDE-LIKE HTMX FRONTEND                                        │
│                                                                                  │
│  ┌──────────────┐  ┌──────────────────────────────────────────────────────────┐  │
│  │  FILE TREE   │  │  MAIN WORKSPACE (IDE-like)                                │  │
│  │              │  │                                                           │  │
│  │  📁 src/     │  │  ┌─────────────────────────────────────────────────────┐  │  │
│  │  📁 auth/    │  │  │  Code Editor (single function OR full file)          │  │  │
│  │  📁 models/  │  │  │                                                     │  │  │
│  │  📄 main.go  │  │  │  func ValidateToken(token string) bool {            │  │  │
│  │  📄 user.go  │  │  │      // generated code...                           │  │  │
│  │  📄 ...      │  │  │  }                                                  │  │  │
│  │              │  │  │                                                     │  │  │
│  │  [Insert]    │  │  │  [Edit Function] [Edit Full File] [Submit]          │  │  │
│  │  [Write Fn]  │  │  └─────────────────────────────────────────────────────┘  │  │
│  └──────────────┘  │                                                           │  │
│                    │  ┌─────────────────────────────────────────────────────┐  │  │
│                    │  │  Phase Tracker: [Plan] → [Code] → [Test] → [Rev] → [Human] │  │
│                    │  └─────────────────────────────────────────────────────┘  │  │
│                    │                                                           │  │
│                    │  ┌─────────────────────────────────────────────────────┐  │  │
│                    │  │  Activity Log (scrollable)                          │  │  │
│                    │  │  [12:34] Planning started                           │  │  │
│                    │  │  [12:35] Plan generated                             │  │  │
│                    │  │  [12:36] Code generated: ValidateToken              │  │  │
│                    │  └─────────────────────────────────────────────────────┘  │  │
│                    └───────────────────────────────────────────────────────────┘  │
│  ┌──────────────────────────────────────────────────────────────────────────────┐  │
│  │  HEADER: Project Path | Goal | Model Config | [New Session]              │  │
│  └──────────────────────────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────────────────────┘
                            │ REST API
┌───────────────────────────▼───────────────────────────────────────────────────────┐
│                        ORCHESTRATOR DAEMON                                         │
│                                                                                  │
│  ┌────────────────────────────────────────────────────────────────────────────┐  │
│  │                    API Server (Go)                                          │  │
│  │  - REST endpoints for session management                                   │  │
│  │  - Server-side HTML rendering (HTMX templates)                             │  │
│  │  - No authentication (local environment)                                   │  │
│  └──────────────────────────────┬─────────────────────────────────────────────┘  │  │
│                                 │                                                 │  │
│  ┌──────────────────────────────▼─────────────────────────────────────────────┐  │
│  │               Orchestrator Engine                                          │  │
│  │  - Phase Router (manages A→B→C→D→E flow)                                  │  │  │
│  │  - Human Gate Manager (handles user approvals)                             │  │  │
│  │  - State Manager (persistent state store)                                  │  │  │
│  └──────────────────────────────┬─────────────────────────────────────────────┘  │  │
│                                 │                                                 │  │
│  ┌──────────────────────────────▼─────────────────────────────────────────────┐  │
│  │              Agent Registry                                                │  │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐                     │  │  │
│  │  │ Planner  │ │  Coder   │ │  Tester  │ │ Reviewer │                     │  │  │
│  │  │ +Skills  │ │ +Skills  │ │ +Skills  │ │ +Skills  │                     │  │  │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘                     │  │  │
│  └──────────────────────────────┬─────────────────────────────────────────────┘  │  │
│                                 │                                                 │  │
│  ┌──────────────────────────────▼─────────────────────────────────────────────┐  │
│  │              Model Router                                                  │  │
│  │  ┌────────────────────────────────────────────────────────────────────┐    │  │  │
│  │  │  Provider Interface (extensible)                                   │    │  │  │
│  │  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐         │    │  │  │
│  │  │  │ LM Studio│  │ Ollama   │  │ OpenAI   │  │ Anthropic│ (future)│    │  │  │
│  │  │  └──────────┘  └──────────┘  └──────────┘  └──────────┘         │    │  │  │
│  │  └────────────────────────────────────────────────────────────────────┘    │  │  │
│  └─────────────────────────────────────────────────────────────────────────────┘  │
│                                                                                  │
│  ┌────────────────────────────────────────────────────────────────────────────┐  │
│  │        LANGUAGE-AGNOSTIC TOOL EXECUTOR                                     │  │
│  │  - Auto-detect project type (Go, Kotlin, Java, Rust, TS, Python)          │  │  │
│  │  - Shell commands, file ops, build/test/run                               │  │  │
│  │  - Code formatting (per-language formatters)                              │  │  │
│  │  - Atomic modifications: ONE function/struct/class at a time              │  │  │
│  │  - Error display when LM Studio is unreachable                            │  │  │
│  └────────────────────────────────────────────────────────────────────────────┘  │
│                                                                                  │
│  ┌────────────────────────────────────────────────────────────────────────────┐  │
│  │              State Store                                                   │  │
│  │  - JSON file                                                               │  │
│  └────────────────────────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────────────────────┘
```

## 2. Component Design

### 2.1 Orchestrator Engine (`internal/orchestrator/`)

The core state machine that manages the 5-phase flow.

```go
type Orchestrator struct {
    StateManager   *state.Manager
    AgentRegistry  *agent.Registry
    ModelRouter    *model.Router
    ToolExecutor   *tools.Executor
    HumanGate      chan HumanDecision // User approval channel
}

type HumanDecision struct {
    Approved bool
    Feedback string
}
```

### 2.2 Agent Registry (`internal/agent/`)

Each agent is a struct with its own prompt, model config, and set of skills.

```go
type Agent interface {
    Execute(ctx context.Context, input Input) (Output, error)
}

type PlannerAgent struct {
    Model   model.Config
    Prompt  PromptTemplate
    Skills  []Skill   // Configurable skills for this agent
}

type CoderAgent struct {
    Model   model.Config
    Prompt  PromptTemplate
    Skills  []Skill   // Configurable skills for this agent
}

type TesterAgent struct {
    Model   model.Config
    Prompt  PromptTemplate
    Skills  []Skill   // Configurable skills for this agent
}

type ReviewerAgent struct {
    Model   model.Config
    Prompt  PromptTemplate
    Skills  []Skill   // Configurable skills for this agent
}
```

### 2.3 Model Router (`internal/model/`)

Abstracts away LLM providers with a unified interface. **Starts with LM Studio only**, but the interface is designed to support other providers.

```go
// Provider interface - designed for future expansion
type Provider interface {
    Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
    ListModels(ctx context.Context) ([]string, error)
    Name() string
}

type Router struct {
    providers map[string]Provider  // providerName -> Provider instance
    defaults  map[Phase]model.Config // Per-phase default config
}
```

### 2.4 Language-Agnostic Tool Executor (`internal/tools/`)

Universal tool executor that detects project type and runs appropriate commands.

```go
type Executor struct {
    // Shell commands (always available)
    Shell *ShellExecutor
    
    // File operations (atomic: one function/struct/class at a time)
    FileOps *FileOpsExecutor
    
    // Auto-detected project executors
    projectType ProjectType
    executors   map[ProjectType]ProjectExecutor
}

type ProjectExecutor interface {
    // Detect if this project type matches the given path
    Detect(projectPath string) bool
    
    // Get the build command for this project type
    BuildCommand(projectPath string) []string
    
    // Get the test command for this project type
    TestCommand(projectPath string) []string
    
    // Get the format/lint command
    FormatCommand(projectPath string) []string
}
```

### 2.5 Project Type Detection

```go
type ProjectType string

const (
    ProjectGo          ProjectType = "go"
    ProjectKotlin      ProjectType = "kotlin"
    ProjectJava        ProjectType = "java"
    ProjectRust        ProjectType = "rust"
    ProjectTypeScript  ProjectType = "typescript"
    ProjectPython      ProjectType = "python"
    ProjectUnknown     ProjectType = "unknown"
)

func DetectProjectType(projectPath string) ProjectType {
    // Check for markers in order of priority
    if exists(filepath.Join(projectPath, "go.mod")) {
        return ProjectGo
    }
    if exists(filepath.Join(projectPath, "build.gradle.kts")) ||
       exists(filepath.Join(projectPath, "build.gradle")) {
        return ProjectKotlin
    }
    if exists(filepath.Join(projectPath, "pom.xml")) {
        return ProjectJava
    }
    if exists(filepath.Join(projectPath, "Cargo.toml")) {
        return ProjectRust
    }
    if exists(filepath.Join(projectPath, "package.json")) {
        return ProjectTypeScript
    }
    if exists(filepath.Join(projectPath, "requirements.txt")) ||
       exists(filepath.Join(projectPath, "pyproject.toml")) {
        return ProjectPython
    }
    return ProjectUnknown
}
```

## 3. Data Flow

### 3.1 Session Lifecycle (Single Project)

```
1. User initializes project via IDE
   └─> POST /api/project
   └─> State: INITIALIZED

2. Planner runs (Phase A)
   └─> State: PLANNING
   └─> State: WAITING_FOR_PLAN_APPROVAL

3. User approves plan in IDE
   └─> POST /api/project/approve
   └─> State: PLANNING_APPROVED

4. Coding loop (Phase B→C→D→E)
   └─> For each atomic unit (function/struct/class):
       - CODING → TESTING → REVIEW → HUMAN_REVIEW
       - If rejected: loop back to CODING
       - If edited: TESTING → REVIEW → HUMAN_REVIEW
       - If accepted: next unit

5. Session completed
   └─> State: COMPLETED
```

### 3.2 Atomic Modification Rule

**Every agent modification is limited to ONE atomic unit:**

```
Coder Agent Output:
┌─────────────────────────────────────────────┐
│  ONE function:                              │
│    func ValidateToken(token string) bool    │
│                                             │
│  OR ONE struct:                             │
│    type User struct {                       │
│        ID    string                        │
│        Name  string                        │
│    }                                        │
│                                             │
│  OR ONE class (Kotlin/Java):               │
│    class AuthService {                     │
│        fun authenticate(...) {...}         │
│    }                                        │
└─────────────────────────────────────────────┘
```

This ensures:
- Easy review
- Focused testing
- Clear version control diffs
- Predictable state management

## 4. Configuration System

### 4.1 Config File (`config.yaml`)

```yaml
server:
  port: 8080
  host: "127.0.0.1"
  # No authentication - local environment only

models:
  # Active provider (only LM Studio for now)
  active_provider: "lm-studio"
  
  # Provider configurations
  providers:
    lm-studio:
      base_url: "http://127.0.0.1:1234"
      # Models loaded in LM Studio
      available_models:
        - "qwen/qwen3-coder-30b"
        - "microsoft/phi-3"
    
    # Future providers (interface ready, not yet implemented)
    # ollama:
    #   base_url: "http://localhost:11434"
    # openai:
    #   api_key: "${OPENAI_API_KEY}"
    #   base_url: "https://api.openai.com/v1"
  
  # Per-phase model configuration
  phases:
    planning:
      provider: "lm-studio"
      model: "qwen/qwen3-coder-30b"
      temperature: 0.3
      max_tokens: 4096
    
    coding:
      provider: "lm-studio"
      model: "qwen/qwen3-coder-30b"
      temperature: 0.1
      max_tokens: 8192
    
    testing:
      provider: "lm-studio"
      model: "qwen/qwen3-coder-30b"
      temperature: 0.2
      max_tokens: 2048
    
    review:
      provider: "lm-studio"
      model: "qwen/qwen3-coder-30b"
      temperature: 0.2
      max_tokens: 4096

# Agent Skills Configuration
agents:
  planner:
    skills:
      - architecture_design
      - task_breakdown
      - dependency_mapping
      - solid_principles
      - clean_code
      - business_logic_adherence
  
  coder:
    skills:
      - function_generation
      - struct_design
      - class_creation
      - solid_principles
      - clean_code
      - kiss_principle
      - no_repetition
  
  tester:
    skills:
      - unit_testing
      - integration_testing
      - coverage_analysis
      - test_generation
  
  reviewer:
    skills:
      - style_check
      - logic_review
      - security_audit
      - solid_principles
      - clean_code
      - business_logic_adherence

# Skills Library (predefined skills with descriptions)
skills:
  solid_principles:
    description: "Follow SOLID principles (Single Responsibility, Open/Closed, Liskov Substitution, Interface Segregation, Dependency Inversion)"
    type: knowledge
    
  clean_code:
    description: "Write clean, readable, maintainable code with meaningful names and proper structure"
    type: knowledge
    
  kiss_principle:
    description: "Keep It Simple, Stupid - prefer simple solutions over complex ones"
    type: knowledge
    
  no_repetition:
    description: "Avoid code duplication - extract common patterns into reusable components"
    type: knowledge
    
  business_logic_adherence:
    description: "Always follow the business logic defined in the business_logic.md file"
    type: knowledge
    requires_file: "business_logic.md"
    
  architecture_design:
    description: "Design system architecture with clear module boundaries and dependencies"
    type: knowledge
    
  task_breakdown:
    description: "Break down complex requirements into manageable atomic units (functions, structs, classes)"
    type: knowledge
    
  dependency_mapping:
    description: "Map and manage dependencies between components"
    type: knowledge
    
  function_generation:
    description: "Generate well-structured functions with proper signatures and documentation"
    type: knowledge
    
  struct_design:
    description: "Design appropriate data structures with proper fields and methods"
    type: knowledge
    
  class_creation:
    description: "Create well-encapsulated classes with proper constructors and methods"
    type: knowledge
    
  unit_testing:
    description: "Write comprehensive unit tests for individual functions and methods"
    type: knowledge
    
  integration_testing:
    description: "Write integration tests that verify component interactions"
    type: knowledge
    
  coverage_analysis:
    description: "Analyze test coverage and identify gaps"
    type: knowledge
    
  test_generation:
    description: "Generate test cases based on function signatures and business logic"
    type: knowledge
    
  style_check:
    description: "Check code style consistency and adherence to language conventions"
    type: knowledge
    
  logic_review:
    description: "Review code logic for correctness, edge cases, and potential bugs"
    type: knowledge
    
  security_audit:
    description: "Review code for security vulnerabilities and best practices"
    type: knowledge

tools:
  shell_timeout: 30s
  auto_detect_project: true  # Auto-detect Go, Kotlin, Java, Rust, TS, Python, etc.
  code_formatting: true      # Auto-format code after generation
  
  # Language-specific settings (optional overrides)
  languages:
    go:
      test_runner: "go test ./..."
      format_runner: "go fmt ./..."
      build_runner: "go build ./..."
    kotlin:
      test_runner: "./gradlew test"
      format_runner: "./gradlew ktlintFormat"
      build_runner: "./gradlew build"
    java:
      test_runner: "./gradlew test"
      format_runner: "./gradlew spotlessApply"
      build_runner: "./gradlew build"
    rust:
      test_runner: "cargo test"
      format_runner: "cargo fmt"
      build_runner: "cargo build"
    typescript:
      test_runner: "npx jest"
      format_runner: "npx prettier --write"
      build_runner: "npx tsc"
    python:
      test_runner: "pytest"
      format_runner: "black ."
      build_runner: "python -m py_compile"

state:
  store_type: "json"
  json_path: "~/.mini-orca"

# Theme configuration (dark by default, extensible)
theme:
  default: "dark"
  available:
    - "dark"
    # - "light"      # Future
    # - "high-contrast" # Future
```

## 5. Directory Structure (After Refactor)

```
mini-orca/
├── cmd/
│   ├── daemon/
│   │   └── main.go              # Entry point
│   └── cli/
│       └── main.go              # CLI tool (legacy, keep for compat)
├── internal/
│   ├── orchestrator/            # NEW: Phase management
│   │   ├── orchestrator.go
│   │   ├── phase_router.go
│   │   └── human_gate.go
│   ├── agent/
│   │   ├── registry.go          # Agent registry
│   │   ├── planner.go           # Planner agent
│   │   ├── coder.go             # Coder agent
│   │   ├── tester.go            # Tester agent
│   │   ├── reviewer.go          # Reviewer agent
│   │   └── skills/              # NEW: Skills system
│   │       ├── skills.go        # Skill definitions
│   │       └── library.go       # Predefined skills
│   ├── model/                   # NEW: Model abstraction
│   │   ├── router.go            # Provider router
│   │   ├── provider.go          # Provider interface
│   │   └── lm_studio.go         # LM Studio implementation
│   ├── state/
│   │   ├── manager.go           # Enhanced state manager
│   │   ├── session.go           # Session model
│   │   └── store/
│   │       └── json_store.go    # JSON file store
│   ├── tools/                   # REDESIGNED: Language-agnostic
│   │   ├── executor.go          # Main executor
│   │   ├── shell.go             # Shell commands
│   │   ├── file_ops.go          # Atomic file operations
│   │   └── formatter.go         # Code formatting
│   │   └── project_types.go     # Project detection & executors
│   │       ├── go_executor.go
│   │       ├── kotlin_executor.go
│   │       ├── java_executor.go
│   │       ├── rust_executor.go
│   │       ├── typescript_executor.go
│   │       └── python_executor.go
│   └── api/
│       ├── router.go
│       ├── handlers/
│       │   ├── project.go       # Project management
│       │   ├── approve.go       # Approval handling
│       │   └── config.go        # Model/skills config
│       └── templates/
│           ├── base.html        # Base layout
│           ├── ide.html         # IDE dashboard
│           ├── components/
│           │   ├── file-tree.html
│           │   ├── phase-tracker.html
│           │   └── activity-log.html
│           └── phases/
│               ├── planning.html
│               ├── planning-review.html
│               ├── coding.html
│               ├── testing.html
│               ├── review.html
│               └── human-review.html
├── config.yaml                  # NEW: Default config
├── go.mod
└── go.sum
```

## 6. Migration Strategy

| Component | v1.0 | v2.0 | Notes |
|-----------|------|------|-------|
| State Machine | `internal/operation/engine.go` | `internal/orchestrator/` | Extract to new package |
| Agent | `internal/agent/client.go` | `internal/agent/*.go` | Split into specialized agents |
| Skills | None | `internal/agent/skills/` | NEW: Configurable skills system |
| Model | Hardcoded in client.go | `internal/model/` | Interface-first, LM Studio only initially |
| API | `internal/api/*.go` | `internal/api/handlers/` | Split handlers |
| Frontend | HTMX templates (keep) | `internal/api/templates/` | Enhance to IDE-like layout |
| State Store | JSON file | JSON file (keep) | No SQLite needed |
| Tools | Language-specific | Language-agnostic | Auto-detect + universal shell |
| Formatting | None | `internal/tools/formatter.go` | NEW: Code formatting |

## 7. Dependencies

### Backend (Go)
- No new major dependencies needed
- May add `github.com/BurntSushi/toml` for config parsing (optional)

### Frontend (HTMX - existing)
- HTMX.org (already used)
- TailwindCSS (already used)
- **Zero client-side JS frameworks** — all interactivity via HTMX + server-side rendering
