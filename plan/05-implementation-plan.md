# Mini-Orca v2.0 — Implementation Plan

## Overview

This document outlines the step-by-step implementation plan to evolve mini-orca from v1.0 to v2.0. The plan is organized into **6 milestones**, each building on the previous one.

**Estimated Timeline:** 4-6 weeks (solo developer, part-time)

---

## Milestone 1: Model Abstraction & LM Studio Provider (Week 1)

### Goal: Create the model abstraction layer with LM Studio as the only provider

### Tasks

- [ ] **1.1** Create `internal/model/` package structure
  - [ ] Define `Provider` interface (extensible for future providers)
  - [ ] Define `ChatRequest`, `ChatResponse`, `ModelConfig` types
  - [ ] Create `Router` struct

- [ ] **1.2** Implement LM Studio provider
  - [ ] `internal/model/lm_studio.go`
  - [ ] `ListModels()` — query `/v1/models`
  - [ ] `Chat()` — POST to `/v1/chat/completions`
  - [ ] Test with local LM Studio instance

- [ ] **1.3** Create configuration system
  - [ ] `config.yaml` schema (LM Studio only initially)
  - [ ] Config loader (`internal/config/`)
  - [ ] Default phase configs

- [ ] **1.4** Update existing agent calls to use the new model router
  - [ ] Modify `internal/agent/client.go` to use `model.Router`
  - [ ] Remove hardcoded model name

### Deliverables
- Model router with Provider interface (LM Studio only)
- YAML configuration file
- All existing LLM calls go through the router

---

## Milestone 2: Multi-Agent Architecture (Week 2)

### Goal: Split the single agent into specialized agents

### Tasks

- [ ] **2.1** Create `internal/agent/` package structure
  - [ ] Define `Agent` interface
  - [ ] Create `Registry` for agents

- [ ] **2.2** Implement Planner agent
  - [ ] `internal/agent/planner.go`
  - [ ] Specialized prompts for planning
  - [ ] Output: Plan with atomic units (functions, structs, classes)

- [ ] **2.3** Implement Coder agent
  - [ ] `internal/agent/coder.go`
  - [ ] Specialized prompts for code generation
  - [ ] Output: ONE atomic unit per execution

- [ ] **2.4** Implement Tester agent
  - [ ] `internal/agent/tester.go`
  - [ ] Specialized prompts for test analysis
  - [ ] Output: TestReport

- [ ] **2.5** Implement Reviewer agent
  - [ ] `internal/agent/reviewer.go`
  - [ ] Specialized prompts for code review
  - [ ] Output: ReviewReport

- [ ] **2.6** Create prompt templates
  - [ ] `internal/agent/prompts/planner.go`
  - [ ] `internal/agent/prompts/coder.go`
  - [ ] `internal/agent/prompts/tester.go`
  - [ ] `internal/agent/prompts/reviewer.go`

### Deliverables
- 4 specialized agents
- Prompt templates for each phase
- Agents can be configured with different models

---

## Milestone 3: Language-Agnostic Tool Executor (Week 2)

### Goal: Redesign tools to be language-agnostic with auto-detection

### Tasks

- [ ] **3.1** Create `internal/tools/` package structure
  - [ ] `executor.go` — Main executor with auto-detection
  - [ ] `shell.go` — Universal shell command executor
  - [ ] `file_ops.go` — Atomic file operations (ONE unit at a time)
  - [ ] `project_types.go` — Project type detection

- [ ] **3.2** Implement project type detection
  - [ ] Detect: Go, Kotlin, Java, Rust, TypeScript, Python, etc.
  - [ ] `DetectProjectType(path string) ProjectType`

- [ ] **3.3** Implement language-specific executors
  - [ ] `go_executor.go` — `go test`, `go fmt`, `go build`
  - [ ] `kotlin_executor.go` — `./gradlew test`, `ktlint`
  - [ ] `rust_executor.go` — `cargo test`, `cargo fmt`
  - [ ] `typescript_executor.go` — `npx jest`, `npx prettier`
  - [ ] `python_executor.go` — `pytest`, `black`

- [ ] **3.4** Implement atomic file operations
  - [ ] `AppendFunctionToFile()` — Append ONE function
  - [ ] `WriteStructToFile()` — Write ONE struct
  - [ ] `WriteClassToFile()` — Write ONE class
  - [ ] `ReadFile()` — Read file content
  - [ ] All operations target ONE atomic unit

- [ ] **3.5** Update daemon to use new tool executor
  - [ ] `cmd/daemon/main.go`
  - [ ] Wire up language-agnostic executor

### Deliverables
- Language-agnostic tool executor
- Auto-detection of project type
- Atomic file operations (one function/struct/class at a time)

---

## Milestone 4: Orchestrator & State Machine (Week 3)

### Goal: Build the new orchestrator with the 5-phase flow

### Tasks

- [ ] **4.1** Create `internal/orchestrator/` package
  - [ ] `orchestrator.go` — Main orchestrator struct
  - [ ] `phase_router.go` — Phase management
  - [ ] `human_gate.go` — User approval handling

- [ ] **4.2** Define new state models
  - [ ] `internal/state/session.go`
  - [ ] Session, Plan, AtomicUnit structs
  - [ ] Phase enum types

- [ ] **4.3** Implement phase transitions
  - [ ] Planning → PlanningReview → Coding → Testing → Review → HumanReview
  - [ ] Auto-loops for Coding ↔ Testing ↔ Review
  - [ ] Human gates at Planning and HumanReview

- [ ] **4.4** Implement retry logic
  - [ ] Configurable retries per phase
  - [ ] Exponential backoff
  - [ ] Error handling and recovery

- [ ] **4.5** Update state store
  - [ ] Enhance `internal/state/store.go` for new models
  - [ ] Add session history tracking
  - [ ] Add plan persistence

- [ ] **4.6** Update daemon entry point
  - [ ] Wire up new orchestrator
  - [ ] Keep backward compatibility with v1 states

### Deliverables
- Working 5-phase orchestrator
- Human-in-the-loop at Planning and HumanReview
- Retry logic for failed phases

---

## Milestone 5: HTMX Frontend Enhancement (Week 4)

### Goal: Enhance the existing HTMX dashboard with new features

### Tasks

- [ ] **5.1** Create new template structure
  - [ ] `internal/api/templates/base.html` — Base layout
  - [ ] `internal/api/templates/components/` — Reusable components
  - [ ] `internal/api/templates/phases/` — Phase-specific views

- [ ] **5.2** Build phase-specific templates
  - [ ] `phases/planning.html` — Planning in progress
  - [ ] `phases/planning-review.html` — Plan approval with Alpine.js
  - [ ] `phases/coding.html` — Code display
  - [ ] `phases/testing.html` — Test results
  - [ ] `phases/review.html` — Review report
  - [ ] `phases/human-review.html` — Human approval

- [ ] **5.3** Build components
  - [ ] `components/phase-tracker.html` — Phase progress
  - [ ] `components/activity-log.html` — Activity feed
  - [ ] `components/header.html` — Header bar
  - [ ] `components/sidebar.html` — Session sidebar

- [ ] **5.4** Enhance API handlers
  - [ ] `api/handlers/session.go` — Session CRUD
  - [ ] `api/handlers/approve.go` — Approval handling
  - [ ] `api/handlers/config.go` — Model config
  - [ ] Add HTMX partial rendering endpoints

- [ ] **5.5** Add Alpine.js interactivity
  - [ ] Model configuration panel
  - [ ] Session controls (pause/resume/stop)
  - [ ] Feedback input fields

- [ ] **5.6** Polish & UX
  - [ ] Loading states
  - [ ] Error handling
  - [ ] Responsive design

### Deliverables
- Enhanced HTMX dashboard
- Phase-specific views
- Model configuration UI
- Responsive design

---

## Milestone 6: Polish & Kotlin Future (Week 5-6)

### Goal: Final polish and prepare for Kotlin native app

### Tasks

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

### Deliverables
- Production-ready v2.0
- API documentation
- Kotlin native preparation document

---

## Risk Assessment

| Risk | Impact | Mitigation |
|------|--------|------------|
| LLM response quality | High | Iterate prompts, allow model switching |
| Project detection accuracy | Medium | Fallback to shell commands |
| Atomic unit boundary issues | Medium | Clear prompts, human review catches issues |
| Scope creep | Medium | Stick to plan, defer non-essential features |

---

## Success Criteria

- [ ] All 5 phases work correctly with human gates
- [ ] Each phase can use a different model/provider
- [ ] HTMX dashboard shows real-time phase progress
- [ ] Code review UI displays generated code
- [ ] Retry logic handles failures gracefully
- [ ] Configuration is fully customizable via YAML
- [ ] Tool executor auto-detects project types
- [ ] Each agent modifies exactly ONE function/struct/class
- [ ] No authentication required (local environment)

---

## Quick Start Commands (After Implementation)

```bash
# 1. Clone and setup
git clone <repo>
cd mini-orca

# 2. Configure LM Studio
cat > config.yaml << EOF
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
EOF

# 3. Start daemon
go run cmd/daemon/main.go

# 4. Open dashboard
open http://localhost:8080/dashboard

# 5. Enter project goal → Review plan → Approve → Watch agents work!
```
