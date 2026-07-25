# Mini-Orca v2.0 — Detailed Task Breakdown

This document splits every milestone into small, actionable tasks ordered by dependency.
Each task is designed to be completable in 1-3 hours.

---

## Milestone 1: Model Abstraction & LM Studio Provider (Week 1)

### 1.1 Model Abstraction Layer

#### 1.1.1 Create package structure
- [ ] Create `internal/model/` directory
- [ ] Create `internal/model/router.go` file
- [ ] Create `internal/model/provider.go` file
- [ ] Create `internal/model/types.go` file

#### 1.1.2 Define types in `types.go`
- [ ] Define `Provider` struct (name, base URL, config)
- [ ] Define `Model` struct (id, object, owned_by)
- [ ] Define `ModelConfig` struct (provider, model_id, temperature, max_tokens)
- [ ] Define `ChatRequest` struct (messages, model, temperature, stream)
- [ ] Define `ChatResponse` struct (choices, usage, model)
- [ ] Define `ChatMessage` struct (role, content)
- [ ] Define `PhaseConfig` struct (model config + phase-specific overrides)
- [ ] Define `Config` struct (top-level config with phases, providers, agents)
- [ ] Define `Router` struct (map of provider name → Provider, active config)

#### 1.1.3 Define Provider interface in `provider.go`
- [ ] Define `Provider` interface with methods:
  - `ListModels(ctx context.Context) ([]Model, error)`
  - `Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)`
  - `Name() string`
  - `IsStreamingSupported() bool` (optional, for future)

#### 1.1.4 Create Router in `router.go`
- [ ] Define `NewRouter()` constructor
- [ ] Implement `RegisterProvider(name string, p Provider)` method
- [ ] Implement `GetProvider(name string) (Provider, error)` method
- [ ] Implement `GetPhaseConfig(phase string) (*PhaseConfig, error)` method
- [ ] Implement `Chat(phase string, messages []ChatMessage) (*ChatResponse, error)` — routing logic
- [ ] Add error handling for missing providers/phases

### 1.2 LM Studio Provider

#### 1.2.1 Create LM Studio provider file
- [ ] Create `internal/model/lm_studio.go`
- [ ] Define `LMStudioProvider` struct (baseURL, httpClient)

#### 1.2.2 Implement ListModels
- [ ] Implement `ListModels()` that calls `GET /v1/models`
- [ ] Parse JSON response into `[]Model`
- [ ] Handle HTTP errors and malformed responses
- [ ] Add logging for debugging

#### 1.2.3 Implement Chat
- [ ] Implement `Chat()` that calls `POST /v1/chat/completions`
- [ ] Serialize request to JSON
- [ ] Parse response JSON into `ChatResponse`
- [ ] Handle non-200 status codes
- [ ] Handle streaming flag (return error for now, support later)
- [ ] Add context timeout (e.g., 5 minutes)

#### 1.2.4 Implement helper methods
- [ ] Implement `Name()` → returns `"lm-studio"`
- [ ] Implement `IsStreamingSupported()` → returns `false` (v1)

#### 1.2.5 Test LM Studio provider
- [ ] Create `internal/model/lm_studio_test.go`
- [ ] Write unit test for `ListModels` (mock HTTP server)
- [ ] Write unit test for `Chat` (mock HTTP server)
- [ ] Write integration test (requires local LM Studio)

### 1.3 Configuration System

#### 1.3.1 Create config package structure
- [ ] Create `internal/config/` directory
- [ ] Create `internal/config/config.go`
- [ ] Create `internal/config/default.go`

#### 1.3.2 Define config schema
- [ ] Define `Config` struct with fields:
  - `Models` (ModelsConfig)
  - `Agents` (AgentsConfig)
  - `Skills` (SkillsConfig)
- [ ] Define `ModelsConfig` with:
  - `ActiveProvider string`
  - `Providers map[string]ProviderConfig`
  - `Phases map[string]PhaseModelConfig`
- [ ] Define `ProviderConfig` with:
  - `BaseURL string`
  - `APIKey string` (optional, for future providers)
- [ ] Define `PhaseModelConfig` with:
  - `Provider string`
  - `Model string`
  - `Temperature float64`
  - `MaxTokens int`
- [ ] Define `AgentsConfig` with:
  - `Planner AgentConfig`
  - `Coder AgentConfig`
  - `Tester AgentConfig`
  - `Reviewer AgentConfig`
- [ ] Define `AgentConfig` with:
  - `Skills []string`
  - `Model string` (optional override)
- [ ] Define `SkillsConfig` with:
  - `Knowledge map[string]SkillDef`
  - `Tools map[string]ToolDef`

#### 1.3.3 Implement config loader
- [ ] Implement `LoadConfig(path string) (*Config, error)`
- [ ] Use `gopkg.in/yaml.v3` for YAML parsing
- [ ] Handle file not found error
- [ ] Handle YAML parse errors
- [ ] Validate required fields (base_url, phases)
- [ ] Return sensible defaults for optional fields

#### 1.3.4 Implement default configs
- [ ] Create `DefaultPhaseConfigs()` → returns map of phase → default PhaseModelConfig
- [ ] Create `DefaultAgentConfigs()` → returns default agent configs with skill lists
- [ ] Create `DefaultSkills()` → returns predefined skill definitions

#### 1.3.5 Create sample config file
- [ ] Create `config.example.yaml` with full example
- [ ] Include all phases, agents, and skills
- [ ] Add comments explaining each section

### 1.4 Integrate Router into Existing Code

#### 1.4.1 Audit existing LLM calls
- [ ] Find all places in codebase that call LLM (likely in `internal/agent/client.go`)
- [ ] Document each call site with its purpose
- [ ] Identify hardcoded model names

#### 1.4.2 Update agent client
- [ ] Modify `internal/agent/client.go` to accept `*model.Router`
- [ ] Replace direct HTTP calls with `router.Chat(phase, messages)`
- [ ] Remove hardcoded model name
- [ ] Add phase parameter to `Generate()` or equivalent method

#### 1.4.3 Update main/daemon entry point
- [ ] In `cmd/daemon/main.go`, load config at startup
- [ ] Create Router from config
- [ ] Register LM Studio provider
- [ ] Pass Router to agent initialization
- [ ] Handle startup errors gracefully

#### 1.4.4 Test full flow
- [ ] Run daemon with config
- [ ] Verify Router is initialized correctly
- [ ] Verify agent calls go through router
- [ ] Test with actual LM Studio instance

---

## Milestone 2: Multi-Agent Architecture with Skills (Week 2)

### 2.1 Agent Package Structure

#### 2.1.1 Create agent package structure
- [ ] Create `internal/agent/agent.go` — Agent interface
- [ ] Create `internal/agent/registry.go` — Agent registry
- [ ] Create `internal/agent/context.go` — Agent context (session info, history)

#### 2.1.2 Define Agent interface
- [ ] Define `Agent` interface with:
  - `Name() string`
  - `Execute(ctx context.Context, input string) (*AgentResult, error)`
  - `GetSkills() []string`
  - `SetSkills(skills []string)`
- [ ] Define `AgentResult` struct:
  - `Output string`
  - `Metadata map[string]string`
  - `Phase string`

#### 2.1.3 Create Agent Registry
- [ ] Define `Registry` struct (map of name → Agent)
- [ ] Implement `Register(name string, agent Agent)`
- [ ] Implement `Get(name string) (Agent, error)`
- [ ] Implement `List() []string`
- [ ] Pre-register the 4 agents (planner, coder, tester, reviewer)

### 2.2 Skills System

#### 2.2.1 Define skill types
- [ ] Create `internal/agent/skills/skills.go`
- [ ] Define `SkillType` enum: `Knowledge`, `Tool`
- [ ] Define `Skill` struct:
  - `Name string`
  - `Type SkillType`
  - `Description string`
  - `PromptTemplate string` (the skill's contribution to the system prompt)
  - `Parameters map[string]string` (optional skill-specific params)
- [ ] Define `SkillCategory` enum: `Design`, `Coding`, `Testing`, `Review`, `Principles`

#### 2.2.2 Implement skills library
- [ ] Create `internal/agent/skills/library.go`
- [ ] Define all knowledge skills:
  - `solid_principles` — S.O.L.I.D. principles guidance
  - `clean_code` — Clean Code principles
  - `kiss_principle` — Keep It Simple, Stupid
  - `no_repetition` — Avoid code duplication
  - `business_logic_adherence` — Follow business requirements
  - `architecture_design` — Architectural design patterns
  - `task_breakdown` — Task decomposition
  - `dependency_mapping` — Dependency analysis
- [ ] Define all coding skills:
  - `function_generation` — Function generation best practices
  - `struct_design` — Struct/POJO design
  - `class_creation` — Class creation patterns
- [ ] Define all testing skills:
  - `unit_testing` — Unit test best practices
  - `integration_testing` — Integration test patterns
  - `coverage_analysis` — Test coverage analysis
  - `test_generation` — Test case generation
- [ ] Define all review skills:
  - `style_check` — Code style consistency
  - `logic_review` — Logic correctness
  - `security_audit` — Security vulnerability check
- [ ] Each skill has a `PromptTemplate` field with the skill's text contribution

#### 2.2.3 Implement skills registry
- [ ] Create `internal/agent/skills/registry.go`
- [ ] Define `SkillsRegistry` struct (map of name → Skill)
- [ ] Implement `Register(skill Skill)`
- [ ] Implement `Get(name string) (*Skill, error)`
- [ ] Implement `GetByCategory(category SkillCategory) []Skill`
- [ ] Implement `GetForAgent(agentName string) []Skill`
- [ ] Pre-register all skills from library

#### 2.2.4 Create skills utility functions
- [ ] `BuildSkillPrompt(skillNames []string) string` — Combines skill templates
- [ ] `ValidateSkillNames(names []string) ([]string, []string)` — Returns valid + invalid
- [ ] `MergeSkills(agentSkills []string, globalSkills []string) []string` — Deduplicate

### 2.3 Planner Agent

#### 2.3.1 Create planner agent file
- [ ] Create `internal/agent/planner.go`
- [ ] Define `PlannerAgent` struct embedding `Agent`
- [ ] Implement `Name()` → `"planner"`
- [ ] Implement `GetSkills()` / `SetSkills()`

#### 2.3.2 Implement planner execution
- [ ] Implement `Execute()` method:
  - Receive goal description as input
  - Build system prompt with planner skills
  - Call `router.Chat()` with planning phase config
  - Parse response into plan format
  - Return `AgentResult` with plan output
- [ ] Define plan output format (JSON or structured text):
  ```
  Plan:
  - Unit 1: [name]
    - Description
    - Dependencies: []
    - File: path/to/file.go
    - Functions: [func1, func2]
  - Unit 2: ...
  ```

#### 2.3.3 Create planner prompt template
- [ ] Create `internal/agent/prompts/planner.go`
- [ ] Define `BuildPlannerPrompt(goal string, skills []string, context string) ([]ChatMessage, error)`
- [ ] System message includes:
  - Role definition (expert software architect)
  - Skill templates (from skills system)
  - Output format specification
  - Constraints (atomic units, no code yet)
- [ ] User message includes:
  - Goal description
  - Project context (file structure, existing code)
  - Any constraints from user

### 2.4 Coder Agent

#### 2.4.1 Create coder agent file
- [ ] Create `internal/agent/coder.go`
- [ ] Define `CoderAgent` struct embedding `Agent`
- [ ] Implement `Name()` → `"coder"`
- [ ] Implement `GetSkills()` / `SetSkills()`

#### 2.4.2 Implement coder execution
- [ ] Implement `Execute()` method:
  - Receive atomic unit description as input
  - Build system prompt with coder skills
  - Call `router.Chat()` with coding phase config
  - Parse response into code
  - Return `AgentResult` with code output
- [ ] Enforce ONE atomic unit per execution (function, struct, or class)
- [ ] Include context: existing code, plan reference

#### 2.4.3 Create coder prompt template
- [ ] Create `internal/agent/prompts/coder.go`
- [ ] Define `BuildCoderPrompt(unit PlanUnit, existingCode string, skills []string) ([]ChatMessage, error)`
- [ ] System message includes:
  - Role definition (expert software engineer)
  - Skill templates (from skills system)
  - Output format specification
  - Constraints (ONE unit, no repetition, clean code)
- [ ] User message includes:
  - Unit description
  - Existing file content (for context)
  - Dependency information

### 2.5 Tester Agent

#### 2.5.1 Create tester agent file
- [ ] Create `internal/agent/tester.go`
- [ ] Define `TesterAgent` struct embedding `Agent`
- [ ] Implement `Name()` → `"tester"`
- [ ] Implement `GetSkills()` / `SetSkills()`

#### 2.5.2 Implement tester execution
- [ ] Implement `Execute()` method:
  - Receive code + test results as input
  - Build system prompt with tester skills
  - Call `router.Chat()` with testing phase config
  - Analyze failures
  - Return `TestReport` with:
    - Pass/Fail status
    - Failed test descriptions
    - Suggested fixes
    - Coverage assessment

#### 2.5.3 Create tester prompt template
- [ ] Create `internal/agent/prompts/tester.go`
- [ ] Define `BuildTesterPrompt(code string, testResults string, skills []string) ([]ChatMessage, error)`
- [ ] System message includes:
  - Role definition (expert QA engineer)
  - Skill templates (from skills system)
  - Analysis framework
- [ ] User message includes:
  - Code to test
  - Test output (stdout/stderr)
  - Coverage report (if available)

### 2.6 Reviewer Agent

#### 2.6.1 Create reviewer agent file
- [ ] Create `internal/agent/reviewer.go`
- [ ] Define `ReviewerAgent` struct embedding `Agent`
- [ ] Implement `Name()` → `"reviewer"`
- [ ] Implement `GetSkills()` / `SetSkills()`

#### 2.6.2 Implement reviewer execution
- [ ] Implement `Execute()` method:
  - Receive code + context as input
  - Build system prompt with reviewer skills
  - Call `router.Chat()` with review phase config
  - Analyze code quality
  - Return `ReviewReport` with:
    - Issues found (severity: critical/warning/info)
    - Suggestions for improvement
    - Overall quality score
    - Pass/Fail recommendation

#### 2.6.3 Create reviewer prompt template
- [ ] Create `internal/agent/prompts/reviewer.go`
- [ ] Define `BuildReviewerPrompt(code string, plan string, skills []string) ([]ChatMessage, error)`
- [ ] System message includes:
  - Role definition (expert code reviewer)
  - Skill templates (from skills system)
  - Review checklist
- [ ] User message includes:
  - Code to review
  - Original plan/spec
  - Previous review feedback (if any)

### 2.7 Wire Everything Together

#### 2.7.1 Register agents in main
- [ ] In `cmd/daemon/main.go`, register all 4 agents
- [ ] Load agent configs from YAML (skills per agent)
- [ ] Initialize each agent with its skill set

#### 2.7.2 Create agent orchestration helper
- [ ] Create `internal/agent/orchestrator.go`
- [ ] Implement `RunPlanner(goal string) (*AgentResult, error)`
- [ ] Implement `RunCoder(unit PlanUnit) (*AgentResult, error)`
- [ ] Implement `RunTester(code string, testResults string) (*AgentResult, error)`
- [ ] Implement `RunReviewer(code string, plan string) (*AgentResult, error)`

#### 2.7.3 Test agents individually
- [ ] Write simple tests for each agent's prompt building
- [ ] Verify skills are correctly included in prompts
- [ ] Test with LM Studio for end-to-end

---

## Milestone 3: Language-Agnostic Tool Executor (Week 2)

### 3.1 Tool Executor Package Structure

#### 3.1.1 Create tools package structure
- [ ] Create `internal/tools/` directory
- [ ] Create `internal/tools/executor.go` — Main executor interface
- [ ] Create `internal/tools/shell.go` — Shell command executor
- [ ] Create `internal/tools/file_ops.go` — File operations
- [ ] create `internal/tools/formatter.go` — Code formatting
- [ ] Create `internal/tools/project_types.go` — Project detection

#### 3.1.2 Define ToolExecutor interface
- [ ] Define `ToolExecutor` interface with:
  - `Execute(cmd string, args []string, timeout time.Duration) (*ExecResult, error)`
  - `FormatCode(path string) error`
  - `WriteFile(path string, content string) error`
  - `ReadFile(path string) (string, error)`
  - `AppendToFile(path string, content string) error`
- [ ] Define `ExecResult` struct:
  - `ExitCode int`
  - `Stdout string`
  - `Stderr string`
  - `Duration time.Duration`

### 3.2 Project Type Detection

#### 3.2.1 Define project types
- [ ] Create `internal/tools/project_types.go`
- [ ] Define `ProjectType` enum: `Go`, `Kotlin`, `Java`, `Rust`, `TypeScript`, `Python`, `Unknown`
- [ ] Define `ProjectInfo` struct:
  - `Type ProjectType`
  - `RootDir string`
  - `SrcDir string`
  - `TestDir string`
  - `BuildFile string` (go.mod, build.gradle, Cargo.toml, package.json, etc.)

#### 3.2.2 Implement detection logic
- [ ] Implement `DetectProjectType(rootDir string) (*ProjectInfo, error)`
- [ ] Detection rules (check in order):
  - `go.mod` → Go
  - `build.gradle` or `build.gradle.kts` → Kotlin/Java
  - `Cargo.toml` → Rust
  - `package.json` → TypeScript
  - `requirements.txt` or `pyproject.toml` → Python
- [ ] Set appropriate SrcDir, TestDir, BuildFile for each type
- [ ] Return error if no known project type found

### 3.3 Language-Specific Executors

#### 3.3.1 Implement Go executor
- [ ] Create `internal/tools/go_executor.go`
- [ ] Implement `FormatCode()` → runs `go fmt ./...`
- [ ] Implement `RunTests()` → runs `go test ./... -v -cover`
- [ ] Implement `Build()` → runs `go build ./...`
- [ ] Implement `Lint()` → runs `golangci-lint run` (if available), fallback to `go vet`

#### 3.3.2 Implement Kotlin executor
- [ ] Create `internal/tools/kotlin_executor.go`
- [ ] Implement `FormatCode()` → runs `./gradlew ktlintFormat`
- [ ] Implement `RunTests()` → runs `./gradlew test`
- [ ] Implement `Build()` → runs `./gradlew build`
- [ ] Implement `Lint()` → runs `./gradlew ktlintCheck`

#### 3.3.3 Implement Java executor
- [ ] Create `internal/tools/java_executor.go`
- [ ] Implement `FormatCode()` → runs `./gradlew spotlessApply`
- [ ] Implement `RunTests()` → runs `./gradlew test`
- [ ] Implement `Build()` → runs `./gradlew build`
- [ ] Implement `Lint()` → runs `./gradlew spotlessCheck`

#### 3.3.4 Implement Rust executor
- [ ] Create `internal/tools/rust_executor.go`
- [ ] Implement `FormatCode()` → runs `cargo fmt`
- [ ] Implement `RunTests()` → runs `cargo test`
- [ ] Implement `Build()` → runs `cargo build`
- [ ] Implement `Lint()` → runs `cargo clippy`

#### 3.3.5 Implement TypeScript executor
- [ ] Create `internal/tools/typescript_executor.go`
- [ ] Implement `FormatCode()` → runs `npx prettier --write`
- [ ] Implement `RunTests()` → runs `npx jest`
- [ ] Implement `Build()` → runs `npx tsc --noEmit`
- [ ] Implement `Lint()` → runs `npx eslint`

#### 3.3.6 Implement Python executor
- [ ] Create `internal/tools/python_executor.go`
- [ ] Implement `FormatCode()` → runs `black .`
- [ ] Implement `RunTests()` → runs `pytest`
- [ ] Implement `Build()` → runs `python -m py_compile`
- [ ] Implement `Lint()` → runs `flake8` or `pylint`

#### 3.3.7 Create executor factory
- [ ] In `executor.go`, implement `NewExecutor(projectInfo *ProjectInfo) ToolExecutor`
- [ ] Factory dispatches to correct language-specific executor
- [ ] Return generic executor for unknown types (shell-only)

### 3.4 Shell Command Executor

#### 3.4.1 Implement shell executor
- [ ] Create `internal/tools/shell.go`
- [ ] Implement `Execute(cmd string, args []string, timeout time.Duration) (*ExecResult, error)`
- [ ] Use `exec.CommandContext` for timeout support
- [ ] Capture stdout and stderr
- [ ] Return structured result with exit code

#### 3.4.2 Implement safe command execution
- [ ] Add command allowlist for safety
- [ ] Block dangerous commands (rm -rf, mkfs, etc.)
- [ ] Log all commands executed
- [ ] Add configurable timeout (default 60 seconds)

### 3.5 Atomic File Operations

#### 3.5.1 Implement file read
- [ ] In `file_ops.go`, implement `ReadFile(path string) (string, error)`
- [ ] Handle file not found
- [ ] Handle permission errors
- [ ] Return file content as string

#### 3.5.2 Implement atomic write
- [ ] Implement `WriteFile(path string, content string) error`
- [ ] Write to temp file first
- [ ] Atomically rename temp → target
- [ ] Create parent directories if needed
- [ ] Handle write errors gracefully

#### 3.5.3 Implement append function
- [ ] Implement `AppendFunctionToFile(path string, functionName string, functionBody string) error`
- [ ] Read existing file
- [ ] Check if function already exists (avoid duplicates)
- [ ] Append function with proper formatting
- [ ] Write back atomically

#### 3.5.4 Implement struct/class writer
- [ ] Implement `WriteStructToFile(path string, structName string, structBody string) error`
- [ ] Implement `WriteClassToFile(path string, className string, classBody string) error`
- [ ] Check for existing definitions
- [ ] Insert at appropriate location (after imports, before main)

#### 3.5.5 Implement file update helper
- [ ] Implement `UpdateFile(path string, updates []FileUpdate) error`
- [ ] `FileUpdate` struct: `{Type string, Name string, Content string}`
- [ ] Apply all updates in order
- [ ] Single atomic write at the end

### 3.6 Code Formatting

#### 3.6.1 Implement formatter
- [ ] In `formatter.go`, implement `FormatCode(path string) error`
- [ ] Detect project type from path
- [ ] Delegate to language-specific formatter
- [ ] Handle formatter not installed (warn, don't fail)

#### 3.6.2 Integrate formatting into workflow
- [ ] Auto-format after code generation
- [ ] Auto-format after test failures (if needed)
- [ ] Report formatting changes

### 3.7 Wire into Daemon

#### 3.7.1 Update daemon initialization
- [ ] In `cmd/daemon/main.go`, detect project type at startup
- [ ] Create ToolExecutor from project info
- [ ] Pass executor to orchestrator

#### 3.7.2 Create tools package tests
- [ ] Write tests for project detection
- [ ] Write tests for file operations
- [ ] Write tests for shell executor (mock commands)

---

## Milestone 4: Orchestrator & State Machine (Week 3)

### 4.1 Orchestrator Package Structure

#### 4.1.1 Create orchestrator package structure
- [ ] Create `internal/orchestrator/` directory
- [ ] Create `internal/orchestrator/orchestrator.go`
- [ ] Create `internal/orchestrator/phase_router.go`
- [ ] Create `internal/orchestrator/human_gate.go`
- [ ] Create `internal/orchestrator/errors.go`

#### 4.1.2 Define orchestrator struct
- [ ] Create `internal/orchestrator/orchestrator.go`
- [ ] Define `Orchestrator` struct with:
  - `agents map[string]agent.Agent`
  - `router *model.Router`
  - `executor tools.ToolExecutor`
  - `stateStore *state.Store`
  - `config *config.Config`
  - `currentPhase Phase`
  - `currentSession *Session`
  - `ctx context.Context`
  - `cancel context.CancelFunc`
  - `humanGate *HumanGate`

### 4.2 State Models

#### 4.2.1 Define session model
- [ ] Create `internal/state/session.go`
- [ ] Define `Session` struct:
  - `ID string` (UUID)
  - `Goal string`
  - `ProjectPath string`
  - `ProjectType string`
  - `CreatedAt time.Time`
  - `UpdatedAt time.Time`
  - `CurrentPhase Phase`
  - `Status SessionStatus` (pending, running, paused, completed, failed, cancelled)
  - `Plan *Plan`
  - `AtomicUnits []AtomicUnit`
  - `History []PhaseHistory`
  - `Error string`

#### 4.2.2 Define plan model
- [ ] Define `Plan` struct:
  - `ID string`
  - `SessionID string`
  - `GeneratedAt time.Time`
  - `GeneratedBy string` (agent name)
  - `Units []PlanUnit`
  - `Status PlanStatus` (draft, approved, rejected, in_progress, completed)
  - `ApprovedAt time.Time`
  - `ApprovedBy string`
  - `RejectionReason string`

#### 4.2.3 Define plan unit model
- [ ] Define `PlanUnit` struct:
  - `ID string`
  - `Name string`
  - `Description string`
  - `Dependencies []string` (references to other unit IDs)
  - `File string`
  - `UnitType string` (function, struct, class)
  - `GeneratedCode string`
  - `Status UnitStatus` (pending, coding, testing, review, completed, failed)
  - `Tests []TestDefinition`
  - `ReviewComments []ReviewComment`
  - `RetryCount int`

#### 4.2.4 Define phase history model
- [ ] Define `PhaseHistory` struct:
  - `ID string`
  - `SessionID string`
  - `Phase string`
  - `StartedAt time.Time`
  - `CompletedAt time.Time`
  - `Status PhaseStatus`
  - `Output string`
  - `Error string`
  - `RetryCount int`

#### 4.2.5 Define enums and statuses
- [ ] Define `Phase` enum: `Planning`, `PlanningReview`, `Coding`, `Testing`, `Review`, `HumanReview`
- [ ] Define `SessionStatus` enum: `Pending`, `Running`, `Paused`, `Completed`, `Failed`, `Cancelled`
- [ ] Define `PlanStatus` enum: `Draft`, `Approved`, `Rejected`, `InProgress`, `Completed`
- [ ] Define `UnitStatus` enum: `Pending`, `Coding`, `Testing`, `Review`, `Completed`, `Failed`
- [ ] Define `PhaseStatus` enum: `Pending`, `InProgress`, `Completed`, `Failed`, `Skipped`

### 4.3 Phase Router & Transitions

#### 4.3.1 Define phase transition rules
- [ ] Create `internal/orchestrator/phase_router.go`
- [ ] Define valid transitions:
  ```
  Planning → PlanningReview (human gate)
  PlanningReview → Planning (reject)
  PlanningReview → Coding (approve)
  Coding → Testing
  Testing → Coding (fail, retry)
  Testing → Review (pass)
  Review → Coding (reject)
  Review → HumanReview (pass)
  HumanReview → Coding (request edits)
  HumanReview → Completed (approve)
  ```
- [ ] Define `Transition` struct: `{From Phase, To Phase, RequiresHuman bool}`
- [ ] Implement `GetValidTransitions(current Phase) []Phase`
- [ ] Implement `ValidateTransition(from, to Phase) error`

#### 4.3.2 Implement phase router
- [ ] Implement `TransitionTo(phase Phase) error`
- [ ] Update session state
- [ ] Log transition
- [ ] Trigger phase handler
- [ ] Handle invalid transitions

### 4.4 Human Gate

#### 4.4.1 Implement human gate
- [ ] Create `internal/orchestrator/human_gate.go`
- [ ] Define `HumanGate` struct:
  - `sessionID string`
  - `phase Phase`
  - `output string`
  - `approved bool`
  - `feedback string`
  - `responseChan chan GateResponse`
- [ ] Define `GateResponse` struct:
  - `Action string` (approve, reject, edit)
  - `Feedback string`
  - `Timestamp time.Time`
- [ ] Implement `RequestApproval(output string) (*GateResponse, error)`
- [ ] Implement `Respond(action string, feedback string) error`
- [ ] Implement `Timeout(timeout time.Duration) error`

#### 4.4.2 Integrate with API
- [ ] Create API endpoint for gate responses: `POST /api/sessions/:id/gate`
- [ ] Create API endpoint to check gate status: `GET /api/sessions/:id/gate`
- [ ] Gate response triggers phase transition

### 4.5 Orchestrator Phase Handlers

#### 4.5.1 Implement planning phase handler
- [ ] In `orchestrator.go`, implement `runPlanning() error`
- [ ] Call planner agent with goal
- [ ] Parse plan output
- [ ] Save plan to state store
- [ ] Transition to PlanningReview
- [ ] Request human approval

#### 4.5.2 Implement coding phase handler
- [ ] Implement `runCoding() error`
- [ ] Get next pending unit from plan
- [ ] Call coder agent with unit details
- [ ] Execute atomic file operation (write ONE unit)
- [ ] Format code
- [ ] Update unit status
- [ ] Transition to Testing

#### 4.5.3 Implement testing phase handler
- [ ] Implement `runTesting() error`
- [ ] Run language-specific tests
- [ ] Capture test output
- [ ] Call tester agent to analyze
- [ ] If pass → transition to Review
- [ ] If fail → transition to Coding (with feedback)

#### 4.5.4 Implement review phase handler
- [ ] Implement `runReview() error`
- [ ] Call reviewer agent
- [ ] Parse review report
- [ ] If pass → transition to HumanReview
- [ ] If fail → transition to Coding (with feedback)

#### 4.5.5 Implement human review phase handler
- [ ] Implement `runHumanReview() error`
- [ ] Present review results to human
- [ ] Wait for approval/edit request
- [ ] If approve → transition to Completed
- [ ] If edit → transition to Coding (with feedback)

### 4.6 Retry Logic

#### 4.6.1 Implement retry configuration
- [ ] Add retry config to `Config`:
  ```yaml
  retry:
    max_retries: 3
    backoff_base: 1000  # ms
    backoff_max: 30000  # ms
  ```
- [ ] Define `RetryConfig` struct

#### 4.6.2 Implement retry wrapper
- [ ] Create `internal/orchestrator/retry.go`
- [ ] Implement `WithRetry(fn func() error, maxRetries int) error`
- [ ] Implement exponential backoff
- [ ] Log each retry attempt
- [ ] Return final error after all retries exhausted

#### 4.6.3 Integrate retry into phase handlers
- [ ] Wrap LLM calls with retry
- [ ] Wrap test execution with retry (once)
- [ ] Track retry count in state

### 4.7 State Store Updates

#### 4.7.1 Enhance state store
- [ ] Update `internal/state/store.go` for new models
- [ ] Implement `SaveSession(session *Session) error`
- [ ] Implement `GetSession(id string) (*Session, error)`
- [ ] Implement `SavePlan(plan *Plan) error`
- [ ] Implement `GetPlan(sessionID string) (*Plan, error)`
- [ ] Implement `UpdateUnitStatus(unitID string, status UnitStatus) error`
- [ ] Implement `GetPendingUnits(planID string) ([]PlanUnit, error)`
- [ ] Implement `SavePhaseHistory(history *PhaseHistory) error`
- [ ] Implement `GetSessionHistory(sessionID string) ([]PhaseHistory, error)`

#### 4.7.2 Implement session history tracking
- [ ] Log all phase transitions
- [ ] Log all LLM calls (input/output summary)
- [ ] Log all file operations
- [ ] Store in state store

#### 4.7.3 Implement plan persistence
- [ ] Save plan as soon as generated
- [ ] Save each unit's generated code
- [ ] Save test results
- [ ] Save review reports

### 4.8 Wire into Daemon

#### 4.8.1 Update daemon entry point
- [ ] In `cmd/daemon/main.go`:
  - Load config
  - Create model router
  - Register agents
  - Detect project type
  - Create tool executor
  - Create state store
  - Create orchestrator
  - Start HTTP server

#### 4.8.2 Create API endpoints
- [ ] `POST /api/sessions` — Create new session
- [ ] `GET /api/sessions/:id` — Get session status
- [ ] `POST /api/sessions/:id/start` — Start session
- [ ] `POST /api/sessions/:id/pause` — Pause session
- [ ] `POST /api/sessions/:id/resume` — Resume session
- [ ] `POST /api/sessions/:id/stop` — Stop session
- [ ] `GET /api/sessions` — List sessions
- [ ] `POST /api/sessions/:id/gate` — Respond to human gate

#### 4.8.3 Maintain backward compatibility
- [ ] Keep existing v1 API endpoints working
- [ ] Add feature flag or version detection
- [ ] Log deprecation warnings for v1 endpoints

---

## Milestone 5: IDE-like HTMX Frontend (Week 4)

### 5.1 Template Structure

#### 5.1.1 Create template directory structure
- [ ] Create `internal/api/templates/` directory
- [ ] Create `internal/api/templates/base.html`
- [ ] Create `internal/api/templates/ide.html`
- [ ] Create `internal/api/templates/components/` directory
- [ ] Create `internal/api/templates/phases/` directory
- [ ] Create `internal/api/templates/editors/` directory

#### 5.1.2 Create base layout
- [ ] Create `internal/api/templates/base.html`
- [ ] Define HTML5 doctype, charset, viewport
- [ ] Include CSS (Tailwind via CDN for dev, or embedded)
- [ ] Include HTMX via CDN
- [ ] Define block sections:
  - `{{define "header"}}`
  - `{{define "sidebar"}}`
  - `{{define "main"}}`
  - `{{define "footer"}}`
- [ ] Dark theme by default (CSS variables)

### 5.2 IDE Layout

#### 5.2.1 Build file tree component
- [ ] Create `internal/api/templates/components/file-tree.html`
- [ ] Recursive file tree display
- [ ] Expand/collapse folders (HTMX or CSS-only)
- [ ] Highlight current file
- [ ] Show file icons based on extension
- [ ] Click to view file content

#### 5.2.2 Build code editor component
- [ ] Create `internal/api/templates/editors/code-editor.html`
- [ ] Display code with syntax highlighting
- [ ] Use simple CSS-based highlighting (no JS library)
- [ ] Show line numbers
- [ ] Support function-level and full-file views
- [ ] Edit mode with textarea (HTMX submit)

#### 5.2.3 Build phase tracker component
- [ ] Create `internal/api/templates/components/phase-tracker.html`
- [ ] Horizontal progress bar with phases
- [ ] Color coding: pending, in-progress, completed, failed, waiting
- [ ] Show current phase prominently
- [ ] Show retry count
- [ ] Click to see phase details

#### 5.2.4 Build activity log component
- [ ] Create `internal/api/templates/components/activity-log.html`
- [ ] Scrollable log panel
- [ ] Show timestamp, phase, action, status
- [ ] Color-code by type (LLM call, file op, test, review)
- [ ] Auto-scroll to latest
- [ ] Filter by phase/type

#### 5.2.5 Build header bar component
- [ ] Create `internal/api/templates/components/header.html`
- [ ] Project name and path
- [ ] Current phase indicator
- [ ] Model info (provider, model name)
- [ ] Session controls (pause, resume, stop buttons)
- [ ] Settings icon (opens config panel)

### 5.3 Phase-Specific Templates

#### 5.3.1 Build planning phase template
- [ ] Create `internal/api/templates/phases/planning.html`
- [ ] Show "Planning in progress..." with spinner
- [ ] Show goal description
- [ ] Show estimated time
- [ ] Loading animation

#### 5.3.2 Build planning review template
- [ ] Create `internal/api/templates/phases/planning-review.html`
- [ ] Display full plan
- [ ] Show atomic units as cards
- [ ] Show dependencies graph (simple text-based)
- [ ] Approve button (HTMX form)
- [ ] Reject button (HTMX form with reason)
- [ ] Edit goal field

#### 5.3.3 Build coding phase template
- [ ] Create `internal/api/templates/phases/coding.html`
- [ ] Show current unit being coded
- [ ] Show generated code in editor
- [ ] Show "Applying changes..." indicator
- [ ] Show file operation progress
- [ ] Show next unit in queue

#### 5.3.4 Build testing phase template
- [ ] Create `internal/api/templates/phases/testing.html`
- [ ] Show test command being run
- [ ] Show test output (scrollable)
- [ ] Show coverage percentage
- [ ] Show tester agent analysis
- [ ] Pass/fail indicator

#### 5.3.5 Build review phase template
- [ ] Create `internal/api/templates/phases/review.html`
- [ ] Show review report
- [ ] List issues by severity
- [ ] Show reviewer suggestions
- [ ] Overall quality score
- [ ] Pass/fail recommendation

#### 5.3.6 Build human review template
- [ ] Create `internal/api/templates/phases/human-review.html`
- [ ] Show final code
- [ ] Show all review feedback
- [ ] Approve button
- [ ] Request edit button (opens edit form)
- [ ] Show edit feedback area

### 5.4 Editor Components

#### 5.4.1 Build function editor
- [ ] Create `internal/api/templates/editors/code-editor.html`
- [ ] Display single function
- [ ] Editable textarea mode
- [ ] Syntax highlighting
- [ ] Submit changes via HTMX form

#### 5.4.2 Build full file editor
- [ ] Create `internal/api/templates/editors/full-file-editor.html`
- [ ] Display entire file
- [ ] Full editing mode
- [ ] Save button (HTMX POST)
- [ ] Cancel button

### 5.5 HTMX Interactivity

#### 5.5.1 Model configuration panel
- [ ] Create config panel component
- [ ] Dropdown for provider selection
- [ ] Input for model name
- [ ] Slider for temperature
- [ ] HTMX form to save
- [ ] Server-side validation

#### 5.5.2 Session controls
- [ ] Pause button → `hx-post /api/sessions/:id/pause`
- [ ] Resume button → `hx-post /api/sessions/:id/resume`
- [ ] Stop button → `hx-post /api/sessions/:id/stop`
- [ ] Disable buttons based on session state
- [ ] Show confirmation for stop

#### 5.5.3 Feedback input fields
- [ ] Textarea for feedback
- [ ] HTMX form submission
- [ ] Show feedback history
- [ ] Character limit with counter

#### 5.5.4 Search with debounce
- [ ] Search input in file tree
- [ ] `hx-trigger="input changed delay:300ms"`
- [ ] `hx-get /api/files?search=...`
- [ ] Replace file tree results

#### 5.5.5 Mobile sidebar
- [ ] CSS class toggle for sidebar visibility
- [ ] Hamburger menu button
- [ ] No JavaScript frameworks
- [ ] CSS transitions for smooth toggle

### 5.6 Skills Management UI

#### 5.6.1 Skills library view
- [ ] Create `internal/api/templates/components/skills-library.html`
- [ ] Server-rendered list of all available skills
- [ ] Group by category (Design, Coding, Testing, Review)
- [ ] Show skill description
- [ ] HTMX search/filter

#### 5.6.2 Add/Edit skill form
- [ ] Create `internal/api/templates/components/skill-form.html`
- [ ] Server-rendered form partial
- [ ] Fields: name, type, description, prompt template
- [ ] HTMX form submission
- [ ] Validation errors displayed inline

#### 5.6.3 Agent-skill association view
- [ ] Create `internal/api/templates/components/agent-skills.html`
- [ ] Show each agent's assigned skills
- [ ] Checkbox list for adding/removing skills
- [ ] HTMX form submission
- [ ] Visual feedback on change

#### 5.6.4 Bulk actions
- [ ] Reset button → `hx-post /api/skills/reset`
- [ ] Export button → `hx-post /api/skills/export` (download JSON)
- [ ] Import button → `hx-post /api/skills/import` (file upload)
- [ ] Confirmation dialogs (CSS/HTMX only)

#### 5.6.5 Skills API endpoints
- [ ] `GET /api/skills` — List all skills
- [ ] `POST /api/skills` — Create skill
- [ ] `PUT /api/skills/:id` — Update skill
- [ ] `DELETE /api/skills/:id` — Delete skill
- [ ] `GET /api/skills/export` — Export as JSON
- [ ] `POST /api/skills/import` — Import from JSON
- [ ] `GET /api/agents/:id/skills` — Get agent skills
- [ ] `PUT /api/agents/:id/skills` — Set agent skills

### 5.7 API Handlers

#### 5.7.1 Project handlers
- [ ] Create `internal/api/handlers/project.go`
- [ ] `GET /api/projects` — List projects
- [ ] `POST /api/projects` — Create/open project
- [ ] `GET /api/projects/:id/files` — List files
- [ ] `GET /api/projects/:id/files/:path` — Get file content

#### 5.7.2 Approval handlers
- [ ] Create `internal/api/handlers/approve.go`
- [ ] `POST /api/sessions/:id/gate` — Respond to human gate
- [ ] `GET /api/sessions/:id/gate` — Get current gate
- [ ] Validate response format
- [ ] Trigger phase transition

#### 5.7.3 Config handlers
- [ ] Create `internal/api/handlers/config.go`
- [ ] `GET /api/config` — Get current config
- [ ] `PUT /api/config` — Update config
- [ ] `GET /api/config/models` — List available models
- [ ] `GET /api/config/phases` — Get phase configs

#### 5.7.4 Session handlers
- [ ] Create `internal/api/handlers/session.go`
- [ ] `GET /api/sessions` — List sessions
- [ ] `GET /api/sessions/:id` — Get session details
- [ ] `POST /api/sessions` — Create session
- [ ] `POST /api/sessions/:id/start` — Start session
- [ ] `POST /api/sessions/:id/pause` — Pause session
- [ ] `POST /api/sessions/:id/resume` — Resume session
- [ ] `POST /api/sessions/:id/stop` — Stop session
- [ ] `DELETE /api/sessions/:id` — Delete session
- [ ] `GET /api/sessions/:id/history` — Get phase history

#### 5.7.5 HTMX partial rendering endpoints
- [ ] `GET /api/render/phase/:phase` — Render phase partial
- [ ] `GET /api/render/file-tree` — Render file tree partial
- [ ] `GET /api/render/activity-log` — Render activity log partial
- [ ] `GET /api/render/phase-tracker` — Render phase tracker partial

### 5.8 CSS Styling

#### 5.8.1 Base styles
- [ ] Create `internal/api/static/css/main.css`
- [ ] CSS variables for theming
- [ ] Dark theme defaults
- [ ] Layout grid/flexbox definitions
- [ ] Typography styles

#### 5.8.2 Component styles
- [ ] File tree styles
- [ ] Code editor styles
- [ ] Phase tracker styles
- [ ] Activity log styles
- [ ] Button styles
- [ ] Form styles
- [ ] Modal styles

#### 5.8.3 Responsive styles
- [ ] Mobile breakpoint (< 768px)
- [ ] Tablet breakpoint (< 1024px)
- [ ] Hide sidebar on mobile
- [ ] Stack layout on small screens

### 5.9 Polish & UX

#### 5.9.1 Loading states
- [ ] Global loading spinner
- [ ] Phase-specific loading indicators
- [ ] Button disabled states during operations
- [ ] Skeleton screens for slow loads

#### 5.9.2 Error handling
- [ ] Error toast notifications
- [ ] Error page for 404/500
- [ ] Graceful degradation (show error message)
- [ ] Retry button on failed requests

#### 5.9.3 Responsive design
- [ ] Test on mobile viewport
- [ ] Test on tablet viewport
- [ ] Ensure all interactions work with touch
- [ ] Adjust font sizes for readability

---

## Milestone 6: Polish & Release (Week 5-6)

### 6.1 Backend Polish

#### 6.1.1 Error handling improvements
- [ ] Create `internal/errors/` package
- [ ] Define custom error types
- [ ] Add error wrapping with context
- [ ] Consistent error responses in API
- [ ] User-friendly error messages

#### 6.1.2 Logging improvements
- [ ] Create `internal/logging/` package
- [ ] Structured logging (JSON format)
- [ ] Log levels (debug, info, warn, error)
- [ ] Log sensitive data filtering
- [ ] Log rotation (if needed)

#### 6.1.3 Documentation
- [ ] Update README with v2 features
- [ ] Add architecture diagram
- [ ] Document API endpoints
- [ ] Document configuration options
- [ ] Add usage examples
- [ ] Create CONTRIBUTING.md

#### 6.1.4 Unit tests
- [ ] Write tests for model router
- [ ] Write tests for config loader
- [ ] Write tests for skills system
- [ ] Write tests for agent prompts
- [ ] Write tests for tool executor
- [ ] Write tests for state store
- [ ] Write tests for orchestrator transitions
- [ ] Target 70%+ coverage

### 6.2 Frontend Polish

#### 6.2.1 Bug fixes
- [ ] Test all phase flows
- [ ] Test human gate interactions
- [ ] Test skills management
- [ ] Test config updates
- [ ] Fix any UI bugs found

#### 6.2.2 Performance optimization
- [ ] Optimize HTMX requests (batch if possible)
- [ ] Lazy-load file tree
- [ ] Cache phase history
- [ ] Optimize CSS (remove unused)

#### 6.2.3 Accessibility
- [ ] Add ARIA labels
- [ ] Keyboard navigation
- [ ] Screen reader support
- [ ] Color contrast checks
- [ ] Focus management

### 6.3 Prepare for Kotlin Native

#### 6.3.1 Document API contract
- [ ] Create `docs/api-contract.md`
- [ ] Document all endpoints
- [ ] Document request/response schemas
- [ ] Document authentication (none for v2)
- [ ] Document rate limits (if any)

#### 6.3.2 Create API documentation
- [ ] Generate OpenAPI/Swagger spec
- [ ] Create interactive API docs
- [ ] Document error codes
- [ ] Document versioning strategy

#### 6.3.3 Create Kotlin native plan
- [ ] Create `plan/kotlin-native.md`
- [ ] Architecture overview
- [ ] UI component mapping
- [ ] State management approach
- [ ] Networking layer
- [ ] Testing strategy
- [ ] Estimated timeline

### 6.4 Release v2.0

#### 6.4.1 Update project files
- [ ] Update README.md
- [ ] Update go.mod version
- [ ] Update version constant in code
- [ ] Update config.example.yaml

#### 6.4.2 Create release notes
- [ ] List all new features
- [ ] List all bug fixes
- [ ] List breaking changes
- [ ] Migration guide from v1

#### 6.4.3 Docker support (optional)
- [ ] Create Dockerfile
- [ ] Create docker-compose.yml
- [ ] Add build targets to Makefile
- [ ] Document Docker usage

---

## Task Dependencies Summary

```
Milestone 1 (Model Abstraction)
  ├── Must complete before: Milestone 2, 3, 4
  └── Independent: Yes

Milestone 2 (Multi-Agent)
  ├── Depends on: Milestone 1
  ├── Must complete before: Milestone 4
  └── Independent tasks: 2.2 (Skills) can be parallelized with 2.3-2.6 (Agents)

Milestone 3 (Tool Executor)
  ├── Depends on: None (parallel with M1-M2)
  └── Must complete before: Milestone 4

Milestone 4 (Orchestrator)
  ├── Depends on: Milestone 1, 2, 3
  ├── Must complete before: Milestone 5
  └── Independent tasks: 4.2 (State Models), 4.6 (Retry), 4.7 (State Store) can be parallelized

Milestone 5 (Frontend)
  ├── Depends on: Milestone 4
  └── Independent tasks: 5.1-5.2 (Layout), 5.6-5.8 (Skills UI) can be parallelized

Milestone 6 (Polish)
  ├── Depends on: All previous milestones
  └── Independent tasks: 6.1 (Backend), 6.2 (Frontend), 6.3 (Kotlin prep) can be parallelized
```

## Parallelization Opportunities

| Parallel Group | Tasks |
|----------------|-------|
| Group A | M1.1-1.4 (Model layer) |
| Group B | M2.2 (Skills system) |
| Group C | M2.3-2.6 (Agents) — can be done in parallel after B |
| Group D | M3.1-3.2 (Tools structure + detection) |
| Group E | M3.3 (Language executors) — parallel after D |
| Group F | M3.4-3.5 (Shell + file ops) — parallel after D |
| Group G | M4.2, M4.6, M4.7 (State, retry, store) — after M1 |
| Group H | M5.1-5.2 (Layout) |
| Group I | M5.3-5.5 (Phase templates + HTMX) — after H |
| Group J | M5.6-5.8 (Skills UI + API) — parallel with I |
| Group K | M6.1, M6.2, M6.3 (Polish) — all parallel |

## Recommended Execution Order

1. **Week 1**: M1 (Model Abstraction) — Foundation for everything
2. **Week 2**: M2 (Multi-Agent) + M3 (Tool Executor) — Can overlap
3. **Week 3**: M4 (Orchestrator) — Integrates M1, M2, M3
4. **Week 4**: M5 (Frontend) — Depends on M4 working
5. **Week 5-6**: M6 (Polish) — All parallel workstreams
