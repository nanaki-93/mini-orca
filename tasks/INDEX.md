# Mini-Orca v2.0 — Task Index

**Total tasks: 136**

Each task is a standalone markdown file in the `tasks/` directory with:
- Checklist of sub-items
- Dependencies
- Deliverables
- Status tracking

---

## Milestone 1: Model Abstraction & LM Studio (18 tasks)

| # | File | Description |
|---|------|-------------|
| 1.1.1 | `01-1-1-create-package-structure.md` | Create `internal/model/` directory structure |
| 1.1.2 | `01-1-2-define-types.md` | Define all core types (Provider, Model, Config, etc.) |
| 1.1.3 | `01-1-3-define-provider-interface.md` | Define Provider interface |
| 1.1.4 | `01-1-4-create-router.md` | Create Router struct with phase-based config |
| 1.2.1 | `01-2-1-create-lm-studio-provider-file.md` | Create LM Studio provider file |
| 1.2.2 | `01-2-2-implement-list-models.md` | Implement ListModels (`GET /v1/models`) |
| 1.2.3 | `01-2-3-implement-chat.md` | Implement Chat (`POST /v1/chat/completions`) |
| 1.2.4 | `01-2-4-implement-helper-methods.md` | Implement Name() and IsStreamingSupported() |
| 1.2.5 | `01-2-5-test-lm-studio-provider.md` | Test LM Studio provider |
| 1.3.1 | `01-3-1-create-config-package.md` | Create `internal/config/` directory structure |
| 1.3.2 | `01-3-2-define-config-schema.md` | Define all config structs |
| 1.3.3 | `01-3-3-implement-config-loader.md` | Implement YAML config loader |
| 1.3.4 | `01-3-4-implement-default-configs.md` | Implement default config helpers |
| 1.3.5 | `01-3-5-create-sample-config-file.md` | Create `config.example.yaml` |
| 1.4.1 | `01-4-1-audit-existing-llm-calls.md` | Audit existing LLM call sites |
| 1.4.2 | `01-4-2-update-agent-client.md` | Update agent client to use router |
| 1.4.3 | `01-4-3-update-main-daemon-entry-point.md` | Wire router into daemon |
| 1.4.4 | `01-4-4-test-full-flow.md` | End-to-end flow test |

---

## Milestone 2: Multi-Agent Architecture with Skills (22 tasks)

| # | File | Description |
|---|------|-------------|
| 2.1.1 | `02-1-1-create-agent-package-structure.md` | Create agent package structure |
| 2.1.2 | `02-1-2-define-agent-interface.md` | Define Agent interface and AgentResult |
| 2.1.3 | `02-1-3-create-agent-registry.md` | Create Agent Registry |
| 2.2.1 | `02-2-1-define-skill-types.md` | Define Skill types and categories |
| 2.2.2 | `02-2-2-implement-skills-library.md` | Define all 17 predefined skills |
| 2.2.3 | `02-2-3-implement-skills-registry.md` | Create SkillsRegistry |
| 2.2.4 | `02-2-4-create-skills-utility-functions.md` | BuildSkillPrompt, ValidateSkillNames, MergeSkills |
| 2.3.1 | `02-3-1-create-planner-agent-file.md` | Create PlannerAgent struct |
| 2.3.2 | `02-3-2-implement-planner-execution.md` | Implement Planner Execute() |
| 2.3.3 | `02-3-3-create-planner-prompt-template.md` | Create planner prompt template |
| 2.4.1 | `02-4-1-create-coder-agent-file.md` | Create CoderAgent struct |
| 2.4.2 | `02-4-2-implement-coder-execution.md` | Implement Coder Execute() |
| 2.4.3 | `02-4-3-create-coder-prompt-template.md` | Create coder prompt template |
| 2.5.1 | `02-5-1-create-tester-agent-file.md` | Create TesterAgent struct |
| 2.5.2 | `02-5-2-implement-tester-execution.md` | Implement Tester Execute() |
| 2.5.3 | `02-5-3-create-tester-prompt-template.md` | Create tester prompt template |
| 2.6.1 | `02-6-1-create-reviewer-agent-file.md` | Create ReviewerAgent struct |
| 2.6.2 | `02-6-2-implement-reviewer-execution.md` | Implement Reviewer Execute() |
| 2.6.3 | `02-6-3-create-reviewer-prompt-template.md` | Create reviewer prompt template |
| 2.7.1 | `02-7-1-register-agents-in-main.md` | Register all 4 agents in daemon |
| 2.7.2 | `02-7-2-create-agent-orchestration-helper.md` | Create RunPlanner, RunCoder, RunTester, RunReviewer |
| 2.7.3 | `02-7-3-test-agents-individually.md` | Test all 4 agents |

---

## Milestone 3: Language-Agnostic Tool Executor (22 tasks)

| # | File | Description |
|---|------|-------------|
| 3.1.1 | `03-1-1-create-tools-package-structure.md` | Create `internal/tools/` directory structure |
| 3.1.2 | `03-1-2-define-toolexecutor-interface.md` | Define ToolExecutor interface |
| 3.2.1 | `03-2-1-define-project-types.md` | Define ProjectType enum and ProjectInfo |
| 3.2.2 | `03-2-2-implement-detection-logic.md` | Implement DetectProjectType() |
| 3.3.1 | `03-3-1-implement-go-executor.md` | Create Go executor (go fmt, go test, go build) |
| 3.3.2 | `03-3-2-implement-kotlin-executor.md` | Create Kotlin executor (gradlew, ktlint) |
| 3.3.3 | `03-3-3-implement-java-executor.md` | Create Java executor (spotless, gradlew) |
| 3.3.4 | `03-3-4-implement-rust-executor.md` | Create Rust executor (cargo test, cargo fmt) |
| 3.3.5 | `03-3-5-implement-typescript-executor.md` | Create TS executor (jest, prettier) |
| 3.3.6 | `03-3-6-implement-python-executor.md` | Create Python executor (pytest, black) |
| 3.3.7 | `03-3-7-create-executor-factory.md` | Create NewExecutor() factory |
| 3.4.1 | `03-4-1-implement-shell-executor.md` | Implement universal shell executor |
| 3.4.2 | `03-4-2-implement-safe-command-execution.md` | Add command allowlist and safety |
| 3.5.1 | `03-5-1-implement-file-read.md` | Implement ReadFile() |
| 3.5.2 | `03-5-2-implement-atomic-write.md` | Implement atomic WriteFile() |
| 3.5.3 | `03-5-3-implement-append-function.md` | Implement AppendFunctionToFile() |
| 3.5.4 | `03-5-4-implement-struct-class-writer.md` | Implement WriteStructToFile/WriteClassToFile |
| 3.5.5 | `03-5-5-implement-file-update-helper.md` | Implement UpdateFile() batch operation |
| 3.6.1 | `03-6-1-implement-formatter.md` | Implement FormatCode() |
| 3.6.2 | `03-6-2-integrate-formatting-into-workflow.md` | Auto-format after code generation |
| 3.7.1 | `03-7-1-update-daemon-initialization.md` | Wire executor into daemon |
| 3.7.2 | `03-7-2-create-tools-package-tests.md` | Tests for tools package |

---

## Milestone 4: Orchestrator & State Machine (25 tasks)

| # | File | Description |
|---|------|-------------|
| 4.1.1 | `04-1-1-create-orchestrator-package-structure.md` | Create `internal/orchestrator/` directory |
| 4.1.2 | `04-1-2-define-orchestrator-struct.md` | Define Orchestrator struct |
| 4.2.1 | `04-2-1-define-session-model.md` | Define Session struct |
| 4.2.2 | `04-2-2-define-plan-model.md` | Define Plan struct |
| 4.2.3 | `04-2-3-define-plan-unit-model.md` | Define PlanUnit struct |
| 4.2.4 | `04-2-4-define-phase-history-model.md` | Define PhaseHistory struct |
| 4.2.5 | `04-2-5-define-enums-and-statuses.md` | Define all enums (Phase, SessionStatus, etc.) |
| 4.3.1 | `04-3-1-define-phase-transition-rules.md` | Define valid phase transitions |
| 4.3.2 | `04-3-2-implement-phase-router.md` | Implement TransitionTo() |
| 4.4.1 | `04-4-1-implement-human-gate.md` | Implement HumanGate |
| 4.4.2 | `04-4-2-integrate-human-gate-with-api.md` | Gate API endpoints |
| 4.5.1 | `04-5-1-implement-planning-phase-handler.md` | Implement runPlanning() |
| 4.5.2 | `04-5-2-implement-coding-phase-handler.md` | Implement runCoding() |
| 4.5.3 | `04-5-3-implement-testing-phase-handler.md` | Implement runTesting() |
| 4.5.4 | `04-5-4-implement-review-phase-handler.md` | Implement runReview() |
| 4.5.5 | `04-5-5-implement-human-review-phase-handler.md` | Implement runHumanReview() |
| 4.6.1 | `04-6-1-implement-retry-configuration.md` | Add retry config |
| 4.6.2 | `04-6-2-implement-retry-wrapper.md` | Implement WithRetry() |
| 4.6.3 | `04-6-3-integrate-retry-into-phase-handlers.md` | Wire retry into all phases |
| 4.7.1 | `04-7-1-enhance-state-store.md` | CRUD for Session, Plan, Unit, History |
| 4.7.2 | `04-7-2-implement-session-history-tracking.md` | Log all phase transitions |
| 4.7.3 | `04-7-3-implement-plan-persistence.md` | Persist plan and unit data |
| 4.8.1 | `04-8-1-update-daemon-entry-point.md` | Wire orchestrator into daemon |
| 4.8.2 | `04-8-2-create-api-endpoints.md` | Session and gate API endpoints |
| 4.8.3 | `04-8-3-maintain-backward-compatibility.md` | Keep v1 API working |

---

## Milestone 5: IDE-like HTMX Frontend (37 tasks)

| # | File | Description |
|---|------|-------------|
| 5.1.1 | `05-1-1-create-template-directory-structure.md` | Create template directories |
| 5.1.2 | `05-1-2-create-base-layout.md` | Create base.html with dark theme |
| 5.2.1 | `05-2-1-build-file-tree-component.md` | File tree component |
| 5.2.2 | `05-2-2-build-code-editor-component.md` | Code editor component |
| 5.2.3 | `05-2-3-build-phase-tracker-component.md` | Phase tracker component |
| 5.2.4 | `05-2-4-build-activity-log-component.md` | Activity log component |
| 5.2.5 | `05-2-5-build-header-bar-component.md` | Header bar component |
| 5.3.1 | `05-3-1-build-planning-phase-template.md` | Planning phase view |
| 5.3.2 | `05-3-2-build-planning-review-template.md` | Planning review (approve/reject) |
| 5.3.3 | `05-3-3-build-coding-phase-template.md` | Coding phase view |
| 5.3.4 | `05-3-4-build-testing-phase-template.md` | Testing phase view |
| 5.3.5 | `05-3-5-build-review-phase-template.md` | Review phase view |
| 5.3.6 | `05-3-6-build-human-review-template.md` | Human review view |
| 5.4.1 | `05-4-1-build-function-editor.md` | Function-level editor |
| 5.4.2 | `05-4-2-build-full-file-editor.md` | Full file editor |
| 5.5.1 | `05-5-1-model-configuration-panel.md` | Model config panel |
| 5.5.2 | `05-5-2-session-controls.md` | Pause/resume/stop buttons |
| 5.5.3 | `05-5-3-feedback-input-fields.md` | Feedback textarea |
| 5.5.4 | `05-5-4-search-with-debounce.md` | File tree search |
| 5.5.5 | `05-5-5-mobile-sidebar.md` | Responsive sidebar |
| 5.6.1 | `05-6-1-skills-library-view.md` | Skills library view |
| 5.6.2 | `05-6-2-add-edit-skill-form.md` | Add/edit skill form |
| 5.6.3 | `05-6-3-agent-skill-association-view.md` | Agent-skill checkboxes |
| 5.6.4 | `05-6-4-bulk-actions.md` | Reset/export/import skills |
| 5.6.5 | `05-6-5-skills-api-endpoints.md` | Skills CRUD API |
| 5.7.1 | `05-7-1-project-handlers.md` | Project API handlers |
| 5.7.2 | `05-7-2-approval-handlers.md` | Approval API handlers |
| 5.7.3 | `05-7-3-config-handlers.md` | Config API handlers |
| 5.7.4 | `05-7-4-session-handlers.md` | Session API handlers |
| 5.7.5 | `05-7-5-htmx-partial-rendering-endpoints.md` | HTMX partial endpoints |
| 5.8.1 | `05-8-1-base-styles.md` | Base CSS with dark theme |
| 5.8.2 | `05-8-2-component-styles.md` | Component CSS |
| 5.8.3 | `05-8-3-responsive-styles.md` | Responsive CSS |
| 5.9.1 | `05-9-1-loading-states.md` | Loading spinners and indicators |
| 5.9.2 | `05-9-2-error-handling.md` | Error toasts and pages |
| 5.9.3 | `05-9-3-responsive-design.md` | Mobile/tablet testing |

---

## Milestone 6: Polish & Release (13 tasks)

| # | File | Description |
|---|------|-------------|
| 6.1.1 | `06-1-1-error-handling-improvements.md` | Centralized error handling |
| 6.1.2 | `06-1-2-logging-improvements.md` | Structured logging |
| 6.1.3 | `06-1-3-documentation.md` | README, API docs, CONTRIBUTING.md |
| 6.1.4 | `06-1-4-unit-tests.md` | Unit tests (70%+ coverage) |
| 6.2.1 | `06-2-1-bug-fixes.md` | Test all flows, fix bugs |
| 6.2.2 | `06-2-2-performance-optimization.md` | Optimize HTMX, lazy-load, cache |
| 6.2.3 | `06-2-3-accessibility.md` | ARIA, keyboard nav, contrast |
| 6.3.1 | `06-3-1-document-api-contract.md` | API contract documentation |
| 6.3.2 | `06-3-2-create-api-documentation.md` | OpenAPI/Swagger spec |
| 6.3.3 | `06-3-3-create-kotlin-native-plan.md` | Kotlin native app plan |
| 6.4.1 | `06-4-1-update-project-files.md` | Update README, go.mod, version |
| 6.4.2 | `06-4-2-create-release-notes.md` | v2.0 release notes |
| 6.4.3 | `06-4-3-docker-support.md` | Dockerfile + docker-compose (optional) |

---

## Quick Reference by Dependency

### Start Here (no dependencies)
- `01-1-1-create-package-structure.md`
- `01-4-1-audit-existing-llm-calls.md`

### After Milestone 1
- All Milestone 2 tasks (start with `02-1-1`)
- All Milestone 3 tasks (start with `03-1-1`)

### After Milestones 1-3
- All Milestone 4 tasks (start with `04-1-1`)

### After Milestone 4
- All Milestone 5 tasks (start with `05-1-1`)

### After All Previous
- All Milestone 6 tasks (start with `06-1-1`)
