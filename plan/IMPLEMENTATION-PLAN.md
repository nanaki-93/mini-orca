# Mini-Orca v2.0 — Step-by-Step Implementation Plan

> This plan consolidates all existing planning documents into a single, executable roadmap.
> **Estimated Timeline:** 8-12 weeks (solo developer, part-time)

## 📊 Progress Summary

| Milestone | Status | Progress |
|-----------|--------|----------|
| **1. Model Abstraction & LM Studio** | ✅ Complete | 75% (3/4 tasks) |
| **2. Multi-Agent Architecture with Skills** | ✅ Complete | 100% (7/7 tasks) |
| **3. Language-Agnostic Tool Executor** | ⏳ Pending | 0% |
| **4. Orchestrator & State Machine** | ⏳ Pending | 0% |
| **5. IDE-like HTMX Frontend** | ⏳ Pending | 0% |
| **6. Polish & Kotlin Future** | ⏳ Pending | 0% |

**Last Updated:** Implementation in progress

---

## 📋 Quick Reference: What We're Building

| Feature | Description |
|---------|-------------|
| **5-Phase Workflow** | Planning → Coding → Testing → Review → User Approval |
| **Configurable Models** | Each phase uses a different AI model (GPT-4, Claude, etc.) |
| **IDE-like Dashboard** | HTMX + Alpine.js frontend for real-time monitoring |
| **Git Integration** | Auto-commit, diff, and status tracking |
| **Skills System** | Predefined + custom skills per agent |
| **Kotlin Desktop** | Future native app (Phase 6) |

---

## 🗂️ Final Project Structure

```
mini-orca/
├── cmd/
│   ├── daemon/
│   │   └── main.go              # Entry point
│   └── cli/
│       └── main.go              # CLI interface (optional)
├── internal/
│   ├── api/
│   │   ├── handlers/
│   │   │   ├── project.go       # Project management
│   │   │   ├── approve.go       # Approval handling
│   │   │   ├── config.go        # Model/skills config
│   │   │   └── insertion.go     # Function insertion
│   │   └── templates/
│   │       ├── base.html        # Base layout
│   │       ├── ide.html         # Main IDE page
│   │       ├── components/      # Reusable components
│   │       │   ├── file-tree.html
│   │       │   ├── phase-tracker.html
│   │       │   ├── activity-log.html
│   │       │   └── header.html
│   │       └── phases/          # Phase-specific views
│   │           ├── planning.html
│   │           ├── planning-review.html
│   │           ├── coding.html
│   │           ├── testing.html
│   │           ├── review.html
│   │           └── human-review.html
│   ├── agent/
│   │   ├── agent.go             # Agent interface
│   │   ├── registry.go          # Agent registry
│   │   ├── planner.go           # Planning agent
│   │   ├── coder.go             # Coding agent
│   │   ├── tester.go            # Testing agent
│   │   ├── reviewer.go          # Review agent
│   │   ├── skills/
│   │   │   ├── skills.go        # Skill definitions
│   │   │   ├── library.go       # Predefined skills
│   │   │   └── registry.go      # Skills registry
│   │   └── prompts/
│   │       ├── planner.go       # Planning prompts
│   │       ├── coder.go         # Coding prompts
│   │       ├── tester.go        # Testing prompts
│   │       └── reviewer.go      # Review prompts
│   ├── model/
│   │   ├── provider.go          # Provider interface
│   │   ├── router.go            # Phase-based model routing
│   │   └── lm_studio.go         # LM Studio provider
│   ├── orchestrator/
│   │   ├── orchestrator.go      # Main orchestrator
│   │   ├── phase_router.go      # Phase management
│   │   ├── human_gate.go        # User approval handling
│   │   └── insertion_manager.go # Standalone function insertion
│   ├── tools/
│   │   ├── executor.go          # Main executor with auto-detection
│   │   ├── shell.go             # Shell command executor
│   │   ├── file_ops.go          # Atomic file operations
│   │   ├── git_ops.go           # Git integration
│   │   ├── formatter.go         # Code formatting
│   │   └── project_types.go     # Project type detection
│   ├── state/
│   │   ├── session.go           # Session, Plan, AtomicUnit structs
│   │   └── store.go             # State persistence
│   └── config/
│       ├── config.go            # Configuration loader
│       └── config.yaml          # Default config
├── config/
│   └── config.yaml              # User configuration
├── plan/
│   └── [existing planning docs]
├── tests/
│   └── [test files]
├── go.mod
└── README.md
```

---

## 🚀 Implementation Milestones

### Milestone 1: Model Abstraction & LM Studio Provider (Week 1) ✅ COMPLETED
**Goal:** Create the model abstraction layer with LM Studio as the only provider

#### Tasks
- [x] **1.1** Create `internal/model/` package structure
  - [x] Define `Provider` interface (extensible for future providers)
  - [x] Define `ChatRequest`, `ChatResponse`, `ModelConfig` types
  - [x] Create `Router` struct with phase-based config

- [x] **1.2** Implement LM Studio provider
  - [x] `internal/model/lm_studio.go`
  - [x] `ListModels()` — query `/v1/models`
  - [x] `Chat()` — POST to `/v1/chat/completions`
  - [ ] Test with local LM Studio instance

- [x] **1.3** Create configuration system
  - [x] `config.yaml` schema (LM Studio only initially)
  - [x] Config loader (`internal/config/`)
  - [x] Default phase configs
  - [x] Skills configuration

- [ ] **1.4** Update existing agent calls to use the new model router
  - [ ] Modify `internal/agent/client.go` to use `model.Router`
  - [ ] Remove hardcoded model name

#### Deliverables
- ✅ Model router with Provider interface (LM Studio only)
- ✅ YAML configuration file
- ⏳ All existing LLM calls go through the router (agents created, wiring in progress)

---

### Milestone 2: Multi-Agent Architecture with Skills (Week 2) ✅ COMPLETED
**Goal:** Split the single agent into specialized agents with configurable skills

#### Tasks
- [x] **2.1** Create `internal/agent/` package structure
  - [x] Define `Agent` interface
  - [x] Create `Registry` for agents

- [x] **2.2** Implement skills system
  - [x] `internal/agent/skills/skills.go` — Skill definitions (moved to types package)
  - [x] `internal/agent/skills/library.go` — Predefined skills (20 skills)
  - [x] `internal/agent/skills/registry.go` — Skills registry
  - [x] Skills types: knowledge + tool
  - [x] Flat structure (hierarchical possible later)

- [x] **2.3** Implement Planner agent
  - [x] `internal/agent/planner.go`
  - [x] Skills: `architecture_design`, `task_breakdown`, `dependency_mapping`, `solid_principles`, `clean_code`, `business_logic_adherence`
  - [x] Specialized prompts for planning
  - [x] Output: Plan with atomic units (functions, structs, classes)

- [x] **2.4** Implement Coder agent
  - [x] `internal/agent/coder.go`
  - [x] Skills: `function_generation`, `struct_design`, `class_creation`, `solid_principles`, `clean_code`, `kiss_principle`, `no_repetition`
  - [x] Specialized prompts for code generation
  - [x] Output: ONE atomic unit per execution

- [x] **2.5** Implement Tester agent
  - [x] `internal/agent/tester.go`
  - [x] Skills: `unit_testing`, `integration_testing`, `coverage_analysis`, `test_generation`
  - [x] Specialized prompts for test analysis
  - [x] Output: TestReport

- [x] **2.6** Implement Reviewer agent
  - [x] `internal/agent/reviewer.go`
  - [x] Skills: `style_check`, `logic_review`, `security_audit`, `solid_principles`, `clean_code`, `business_logic_adherence`
  - [x] Specialized prompts for code review
  - [x] Output: ReviewReport

- [x] **2.7** Create prompt templates
  - [x] `internal/agent/prompts/planner.go`
  - [x] `internal/agent/prompts/coder.go`
  - [x] `internal/agent/prompts/tester.go`
  - [x] `internal/agent/prompts/reviewer.go`
  - [x] Skills integrated into prompts

#### Deliverables
- ✅ 4 specialized agents
- ✅ Skills system (flat, configurable, 20 predefined skills)
- ✅ Prompt templates for each phase
- ✅ Agents can be configured with different models

---

### Milestone 3: Language-Agnostic Tool Executor (Week 2-3)
**Goal:** Redesign tools to be language-agnostic with auto-detection and git integration

#### Tasks
- [ ] **3.1** Create `internal/tools/` package structure
  - [ ] `executor.go` — Main executor with auto-detection
  - [ ] `shell.go` — Universal shell command executor
  - [ ] `file_ops.go` — Atomic file operations (ONE unit at a time)
  - [ ] `git_ops.go` — Git integration (add, commit, diff, status)
  - [ ] `formatter.go` — Code formatting (per-language)
  - [ ] `project_types.go` — Project type detection

- [ ] **3.2** Implement project type detection
  - [ ] Detect: Go, Kotlin, Java, Rust, TypeScript, Python
  - [ ] `DetectProjectType(path string) ProjectType`

- [ ] **3.3** Implement language-specific executors
  - [ ] `go_executor.go` — `go test`, `go fmt`, `go build`
  - [ ] `kotlin_executor.go` — `./gradlew test`, `ktlint`
  - [ ] `java_executor.go` — `./gradlew test`, `spotless`
  - [ ] `rust_executor.go` — `cargo test`, `cargo fmt`
  - [ ] `typescript_executor.go` — `npx jest`, `npx prettier`
  - [ ] `python_executor.go` — `pytest`, `black`

- [ ] **3.4** Implement atomic file operations
  - [ ] `AppendFunctionToFile()` — Append ONE function
  - [ ] `WriteStructToFile()` — Write ONE struct
  - [ ] `WriteClassToFile()` — Write ONE class
  - [ ] `ReadFile()` — Read file content
  - [ ] All operations target ONE atomic unit

- [ ] **3.5** Implement Git operations
  - [ ] `GitAdd(path string)` — Add file to git
  - [ ] `GitCommit(message string)` — Commit changes
  - [ ] `GitDiff(path string)` — Show diff
  - [ ] `GitStatus()` — Show git status

- [ ] **3.6** Implement code formatting
  - [ ] `FormatCode(path string)` — Format using language formatter
  - [ ] Auto-format after code generation

- [ ] **3.7** Update daemon to use new tool executor
  - [ ] `cmd/daemon/main.go`
  - [ ] Wire up language-agnostic executor

#### Deliverables
- Language-agnostic tool executor
- Auto-detection of project type
- Git integration (add, commit, diff, status)
- Code formatting
- Atomic file operations (one function/struct/class at a time)

---

### Milestone 4: Orchestrator & State Machine (Week 3-4)
**Goal:** Build the new orchestrator with the 5-phase flow and human gates

#### Tasks
- [ ] **4.1** Create `internal/orchestrator/` package
  - [ ] `orchestrator.go` — Main orchestrator struct
  - [ ] `phase_router.go` — Phase management
  - [ ] `human_gate.go` — User approval handling
  - [ ] `insertion_manager.go` — Standalone function insertion

- [ ] **4.2** Define new state models
  - [ ] `internal/state/session.go`
  - [ ] Session, Plan, AtomicUnit structs
  - [ ] Phase enum types

- [ ] **4.3** Implement phase transitions
  - [ ] Planning → PlanningReview → Coding → Testing → Review → HumanReview
  - [ ] Auto-loops for Coding ↔ Testing ↔ Review
  - [ ] Human gates at Planning and HumanReview
  - [ ] Separate Reject (loop to Coding) vs Edit (Testing → Review → HumanReview)

- [ ] **4.4** Implement standalone function insertion
  - [ ] `InsertFunction(path, functionCode, insertionPoint)`
  - [ ] Always goes through Testing → Review
  - [ ] Insertion point: before/after specific function

- [ ] **4.5** Implement retry logic
  - [ ] Configurable retries per phase
  - [ ] Exponential backoff
  - [ ] Error handling and recovery

- [ ] **4.6** Update state store
  - [ ] Enhance `internal/state/store.go` for new models
  - [ ] Add session history tracking
  - [ ] Add plan persistence

- [ ] **4.7** Update daemon entry point
  - [ ] Wire up new orchestrator
  - [ ] Keep backward compatibility with v1 states

#### Deliverables
- Working 5-phase orchestrator
- Human-in-the-loop at Planning and HumanReview
- Standalone function insertion
- Retry logic for failed phases

---

### Milestone 5: IDE-like HTMX Frontend (Week 4-5)
**Goal:** Create the HTMX-based dashboard with real-time monitoring

#### Tasks
- [ ] **5.1** Create new template structure
  - [ ] `internal/api/templates/base.html` — Base layout
  - [ ] `internal/api/templates/ide.html` — Main IDE page
  - [ ] `internal/api/templates/components/` — Reusable components
  - [ ] `internal/api/templates/phases/` — Phase-specific views

- [ ] **5.2** Build IDE layout
  - [ ] File tree component
  - [ ] Code editor (single function + full file)
  - [ ] Phase tracker
  - [ ] Activity log
  - [ ] Header bar (project info, model config, git status)

- [ ] **5.3** Build phase-specific templates
  - [ ] `phases/planning.html` — Planning in progress
  - [ ] `phases/planning-review.html` — Plan approval with Alpine.js
  - [ ] `phases/coding.html` — Code display
  - [ ] `phases/testing.html` — Test results
  - [ ] `phases/review.html` — Review report
  - [ ] `phases/human-review.html` — Human approval

- [ ] **5.4** Build editor templates
  - [ ] `editors/code-editor.html` — Function editor
  - [ ] `editors/full-file-editor.html` — Full file editor
  - [ ] `editors/standalone-function.html` — Standalone function writer
  - [ ] `editors/insertion-picker.html` — Insertion point picker

- [ ] **5.5** Build components
  - [ ] `components/file-tree.html` — File tree
  - [ ] `components/phase-tracker.html` — Phase progress
  - [ ] `components/activity-log.html` — Activity feed
  - [ ] `components/header.html` — Header bar

- [ ] **5.6** Enhance API handlers
  - [ ] `api/handlers/project.go` — Project management
  - [ ] `api/handlers/approve.go` — Approval handling
  - [ ] `api/handlers/config.go` — Model/skills config
  - [ ] `api/handlers/insertion.go` — Function insertion
  - [ ] Add HTMX partial rendering endpoints

- [ ] **5.7** Add Alpine.js interactivity
  - [ ] Model configuration panel
  - [ ] Session controls (pause/resume/stop)
  - [ ] Feedback input fields
  - [ ] Insertion picker radio buttons

- [ ] **5.8** Skills Management UI
  - [ ] Skills library view (search, filter, enable/disable)
  - [ ] Add/Edit skill dialog (name, description, type, priority, prompt template)
  - [ ] Agent-skill association view (checkboxes per agent)
  - [ ] Bulk actions (reset to defaults, export, import)
  - [ ] API endpoints: CRUD for skills, agent-skill mappings
  - [ ] Alpine.js interactivity for skills management

- [ ] **5.9** Polish & UX
  - [ ] Loading states
  - [ ] Error handling
  - [ ] Responsive design

#### Deliverables
- IDE-like HTMX dashboard
- File tree + code editor
- Phase-specific views
- Standalone function writer + insertion
- Skills management UI (add/edit/remove skills, associate with agents)
- Model configuration UI

---

### Milestone 6: Polish & Kotlin Future (Week 6-8)
**Goal:** Final polish and prepare for Kotlin native app

#### Tasks
- [ ] **6.1** Backend polish
  - [ ] Error handling improvements
  - [ ] Logging improvements
  - [ ] Documentation
  - [ ] Unit tests

- [ ] **6.2** Frontend polish
  - [ ] Bug fixes
  - [ ] Performance optimization
  - [ ] Accessibility improvements

- [ ] **6.3** Prepare for Kotlin native
  - [ ] Document API contract
  - [ ] Create API documentation
  - [ ] `plan/kotlin-native.md` (detailed plan)

- [ ] **6.4** Release v2.0
  - [ ] Update README
  - [ ] Update go.mod
  - [ ] Create release notes
  - [ ] Docker support (optional)

#### Deliverables
- Production-ready v2.0
- API documentation
- Kotlin native preparation document

---

## ✅ Success Criteria

- [ ] All 5 phases work correctly with human gates
- [ ] Each phase can use a different model/provider
- [ ] IDE dashboard shows real-time phase progress
- [ ] Code review UI displays generated code with edit options
- [ ] Retry logic handles failures gracefully
- [ ] Configuration is fully customizable via YAML
- [ ] Tool executor auto-detects project types
- [ ] Each agent modifies exactly ONE function/struct/class
- [ ] Standalone function writer works correctly
- [ ] Skills management UI allows adding/editing/removing skills
- [ ] Skills can be associated with agents via checkbox UI
- [ ] Skills can be exported/imported as JSON config
- [ ] Dark theme by default, extensible for more themes
- [ ] Git integration works (add, commit, diff)
- [ ] No authentication required (local environment)

---

## 📝 Sample Configuration (config.yaml)

```yaml
models:
  active_provider: "lm-studio"
  providers:
    lm-studio:
      base_url: "http://127.0.0.1:1234"
  phases:
    planning:
      provider: "lm-studio"
      model: "qwen/qwen3-coder-30b"
      temperature: 0.3
    coding:
      provider: "lm-studio"
      model: "qwen/qwen3-coder-30b"
      temperature: 0.1
    testing:
      provider: "lm-studio"
      model: "qwen/qwen3-coder-30b"
      temperature: 0.2
    review:
      provider: "lm-studio"
      model: "qwen/qwen3-coder-30b"
      temperature: 0.2

agents:
  planner:
    skills:
      - solid_principles
      - clean_code
      - kiss_principle
      - business_logic_adherence
      - architecture_design
      - task_breakdown
      - dependency_mapping
  
  coder:
    skills:
      - solid_principles
      - clean_code
      - kiss_principle
      - no_repetition
      - function_generation
      - struct_design
      - class_creation
  
  tester:
    skills:
      - clean_code
      - unit_testing
      - integration_testing
      - coverage_analysis
      - test_generation
  
  reviewer:
    skills:
      - solid_principles
      - clean_code
      - business_logic_adherence
      - style_check
      - logic_review
      - security_audit
```

---

## 🏁 Quick Start Commands (After Implementation)

```bash
# 1. Clone and setup
git clone <repo>
cd mini-orca

# 2. Configure LM Studio (see config.yaml above)

# 3. Start daemon
go run cmd/daemon/main.go

# 4. Open IDE dashboard
open http://localhost:8080/ide

# 5. Enter project goal → Review plan → Approve → Code in IDE → Accept/Reject
```

---

## ⚠️ Risk Assessment

| Risk | Impact | Mitigation |
|------|--------|------------|
| LLM response quality | High | Iterate prompts, allow model switching |
| Project detection accuracy | Medium | Fallback to shell commands |
| Atomic unit boundary issues | Medium | Clear prompts, human review catches issues |
| Scope creep | Medium | Stick to plan, defer non-essential features |

---

## 📚 Reference Documents

All planning documents are in the `plan/` directory:
- `plan/01-architecture.md` - Architecture overview
- `plan/02-orchestrator-design.md` - Orchestrator design
- `plan/03-frontend-dashboard.md` - Dashboard design
- `plan/04-agent-workflow.md` - Agent workflow specification
- `plan/04-model-configuration.md` - Model configuration
- `plan/05-technical-architecture.md` - Technical architecture
- `plan/06-implementation-plan.md` - Detailed implementation plan (this file)
- `plan/06-implementation-roadmap.md` - High-level roadmap
- `plan/kotlin-native.md` - Kotlin desktop app plan

---

## 📅 Next Steps

1. **Start with Milestone 1** - Model abstraction layer
2. **Follow the checklist** - Complete each task before moving to the next
3. **Test frequently** - Ensure each milestone works before proceeding
4. **Document as you go** - Update this plan with any changes or discoveries
