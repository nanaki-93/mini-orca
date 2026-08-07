# Mini-Orca Cleanup - Unit Tasks

## Phase 1: Remove Session System

### Task 1.1
**Goal**: Delete session type definitions
**Files**: Delete `internal/state/session.go`
**Impact**: Removes `Session`, `Phase`, `PhaseHistory`, `TestResult`, `ReviewReportEntry` types from state package
**Risk**: High - many files depend on these types
**Before**: Start here
**After**: Proceed to 1.2

### Task 1.2
**Goal**: Delete session HTTP handlers
**Files**: Delete `internal/api/session_handler.go`
**Impact**: Removes `POST /api/sessions`, `GET /api/sessions`, `POST /api/sessions/:id/*` routes
**Risk**: Medium - handlers referenced in main.go
**Before**: 1.1
**After**: Proceed to 1.3

### Task 1.3
**Goal**: Delete session API types
**Files**: Delete `internal/api/session_types.go`
**Impact**: Removes `SessionStore`, `SessionCreateRequest`, `SessionResponse`, etc.
**Risk**: Medium - `SessionStore` used in main.go and render handlers
**Before**: 1.2
**After**: Proceed to 1.4

### Task 1.4
**Goal**: Delete gate store
**Files**: Delete `internal/api/gate.go`
**Impact**: Removes `GateStore` type and its methods
**Risk**: Low - gate store used only in session handler and main.go
**Before**: 1.3
**After**: Proceed to 1.5

### Task 1.5
**Goal**: Simplify state store - remove session CRUD
**Files**: Edit `internal/state/store.go`
**Changes**:
- Remove `SaveSession`, `GetSession`, `ListSessions`, `GetCurrentSession`
- Remove `SavePhaseHistory`, `GetSessionHistory`
- Remove `SaveTestResult`, `GetTestResults`
- Remove `SaveReviewReport`, `GetReviewReports`
- Keep: `UpdatePhaseStatus`, `SetError`, `InitStateStore` (simplified)
- Remove `Session` map, keep only `currentPhase` and `status`
**Risk**: High - orchestrator depends on these methods
**Before**: 1.4
**After**: Proceed to 1.6

### Task 1.6
**Goal**: Update orchestrator to use simplified state
**Files**: Edit `internal/orchestrator/orchestrator.go`
**Changes**:
- Replace `state.Store` methods with direct field access on `Orchestrator.currentSession`
- Remove calls to `stateStore.SaveSession`, `stateStore.SavePhaseHistory`, etc.
- Keep phase transitions and human gate logic
**Risk**: High - core orchestrator logic
**Before**: 1.5
**After**: Proceed to 1.7

### Task 1.7
**Goal**: Update main.go - remove session store
**Files**: Edit `cmd/daemon/main.go`
**Changes**:
- Remove `sessionStore := api.NewSessionStore()`
- Remove `gateStore := api.NewGateStore()`
- Remove phase router initialization
- Remove all session-related route registrations
- Remove `sessionHandler` and `gateHandler` initialization
- Keep `projectStore` and `htmxRenderHandler`
**Risk**: High - entry point
**Before**: 1.6
**After**: Phase 1 complete → Proceed to Phase 2

---

## Phase 2: Remove Project System

### Task 2.1
**Goal**: Delete project handlers
**Files**: Delete `internal/api/handlers/project_handler.go`
**Impact**: Removes `POST /api/projects`, `GET /api/projects`, `GET /api/projects/:id/files`
**Risk**: Medium - project store used in render handler
**Before**: Start here
**After**: Proceed to 2.2

### Task 2.2
**Goal**: Delete project types
**Files**: Delete `internal/api/handlers/project_types.go`
**Impact**: Removes `ProjectStore`, `ProjectResponse`, `ProjectSummary`, etc.
**Risk**: Medium - types used in render handler
**Before**: 2.1
**After**: Proceed to 2.3

### Task 2.3
**Goal**: Delete project files handler
**Files**: Delete `internal/api/handlers/project_files.go`
**Impact**: Removes file content API endpoint
**Risk**: Low - file content now served via editor component
**Before**: 2.2
**After**: Proceed to 2.4

### Task 2.4
**Goal**: Update render handler - get project from config
**Files**: Edit `internal/api/handlers/htmx_render.go`
**Changes**:
- Remove `projectStore` field from `HTMXRenderHandler`
- Read project path from config or cwd directly
- File tree reads from filesystem directly
**Risk**: Medium - affects main page rendering
**Before**: 2.3
**After**: Proceed to 2.5

### Task 2.5
**Goal**: Update file tree handler - direct filesystem access
**Files**: Edit `internal/api/handlers/htmx_filetree.go`
**Changes**:
- Remove project store dependency
- Walk filesystem directly from configured project path
**Risk**: Medium - file tree is core UI feature
**Before**: 2.4
**After**: Phase 2 complete → Proceed to Phase 3

---

## Phase 3: Remove Skills System

### Task 3.1
**Goal**: Delete skills directory
**Files**: Delete `internal/agent/skills/` (entire directory)
**Files**: `library.go`, `registry.go`, `registry_test.go`, `skills.go`, `utils.go`, `utils_test.go`
**Impact**: Removes skills registry, library, and utilities
**Risk**: Medium - agents reference skills
**Before**: Start here
**After**: Proceed to 3.2

### Task 3.2
**Goal**: Delete skills API handler
**Files**: Delete `internal/api/skills_handler.go`
**Impact**: Removes `/api/skills/*` endpoints
**Risk**: Low - skills are config-only now
**Before**: 3.1
**After**: Proceed to 3.3

### Task 3.3
**Goal**: Delete skills API types
**Files**: Delete `internal/api/skills_types.go`
**Impact**: Removes `SkillResponse`, `SkillListResponse`, etc.
**Risk**: Low
**Before**: 3.2
**After**: Proceed to 3.4

### Task 3.4
**Goal**: Simplify config - remove SkillsConfig
**Files**: Edit `internal/config/config.go`
**Changes**:
- Remove `SkillsConfig` struct
- Remove `Skills` field from `Config`
- Remove `applyDefaults()` skills initialization
- Keep `AgentConfig.Skills` as simple `[]string`
**Risk**: Medium - config structure changes
**Before**: 3.3
**After**: Proceed to 3.5

### Task 3.5
**Goal**: Update agent registry - remove skills dependency
**Files**: Edit `internal/agent/registry.go`
**Changes**:
- Remove `skillsRegistry` parameter from `InitAgentRegistry`
- Agents receive skills as `[]string` from config
**Risk**: Medium - agent creation
**Before**: 3.4
**After**: Phase 3 complete → Proceed to Phase 4

---

## Phase 4: Simplify UI Components

### Task 4.1
**Goal**: Delete unused component templates
**Files**: Delete these files:
- `internal/api/templates/components/accessibility-results.html`
- `internal/api/templates/components/accessibility-testing.html`
- `internal/api/templates/components/agent-skill-badge.html`
- `internal/api/templates/components/agent-skills.html`
- `internal/api/templates/components/bulk-action-item.html`
- `internal/api/templates/components/bulk-actions.html`
- `internal/api/templates/components/dashboard.html`
- `internal/api/templates/components/header_settings.html`
- `internal/api/templates/components/mobile-sidebar.html`
- `internal/api/templates/components/project-form.html`
- `internal/api/templates/components/project-modal.html`
- `internal/api/templates/components/session-controls.html`
- `internal/api/templates/components/skill-card.html`
- `internal/api/templates/components/skill-field.html`
- `internal/api/templates/components/skill-form.html`
- `internal/api/templates/components/skills-library.html`
**Impact**: ~16 template files removed, ~100KB
**Risk**: Low - these components are unused
**Before**: Start here
**After**: Proceed to 4.2

### Task 4.2
**Goal**: Delete unused phase templates
**Files**: Delete:
- `internal/api/templates/phases/completed/` (entire directory)
- `internal/api/templates/phases/coding-content.html`
- `internal/api/templates/phases/coding-structure.html`
- `internal/api/templates/phases/testing-content.html`
- `internal/api/templates/phases/testing-structure.html`
- `internal/api/templates/phases/review-content.html`
- `internal/api/templates/phases/review-structure.html`
- `internal/api/templates/phases/human-review-content.html`
- `internal/api/templates/phases/human-review-structure.html`
**Impact**: ~10 template files removed, ~80KB
**Risk**: Medium - phase rendering depends on these
**Before**: 4.1
**After**: Proceed to 4.3

### Task 4.3
**Goal**: Delete unused editor templates
**Files**: Delete `internal/api/templates/editors/full-file-editor.html`
**Keep**: `internal/api/templates/editors/code-editor.html`
**Impact**: 1 file removed
**Risk**: Low - code-editor.html is the main editor
**Before**: 4.2
**After**: Proceed to 4.4

### Task 4.4
**Goal**: Delete unused CSS files
**Files**: Delete:
- `internal/api/static/css/accessibility.css`
- `internal/api/static/css/activity-log.css`
- `internal/api/static/css/dashboard.css`
- `internal/api/static/css/skills.css`
- `internal/api/static/css/responsive.css`
**Keep**: `main.css`, `editor.css`, `sidebar.css`, `phase-tracker.css`, `modal.css`, `toast.css`, `header.css`
**Impact**: 5 CSS files removed, ~25KB
**Risk**: Low
**Before**: 4.3
**After**: Proceed to 4.5

### Task 4.5
**Goal**: Simplify base template
**Files**: Edit `internal/api/templates/base.html`
**Changes**:
- Remove mobile sidebar toggle and overlay
- Simplify header block (remove settings)
- Keep: sidebar (file tree), main (editor + chat), footer (status bar)
- Remove global loading overlay (simplify)
- Remove toast notification container (keep but simplify)
**Risk**: High - affects all pages
**Before**: 4.4
**After**: Proceed to 4.6

### Task 4.6
**Goal**: Simplify IDE template
**Files**: Edit `internal/api/templates/ide.html`
**Changes**:
- Remove activity/phase info panel (right sidebar) - or simplify to just phase tracker
- Keep: file tree (left), editor (center)
- Add: chat input area (bottom or right panel)
- Simplify footer to show: connected status, current phase
**Risk**: High - main UI layout
**Before**: 4.5
**After**: Proceed to 4.7

### Task 4.7
**Goal**: Simplify template engine functions
**Files**: Edit `internal/api/handlers/htmx_templates.go`
**Changes**:
- Remove unused template functions:
  - `getStats`, `logTypeClass`, `logStatusClass`
  - `typeBadgeClass`, `phaseState`, `phaseClass`, `phaseLabelClass`
  - `phaseConnectorClass`, `phaseProgress`, `currentPhaseDotClass`
  - `priorityClass`, `agentIconClass`, `agentIcon`, `humanName`
  - `categoryBadgeClass`, `scoreColor`, `totalIssues`, `countBySeverity`
  - `filterBySeverity`, `isSkillAssigned`, `phaseBorderClass`
  - `phaseDotClass`, `phaseProgressClass`, `phaseTextClass`
  - `coverageColor`, `md5`
- Keep: `fileExtension`, `fileIcon`, `codeLines`, `lower`, `title`, `dict`
**Risk**: Medium - template rendering
**Before**: 4.6
**After**: Phase 4 complete → Proceed to Phase 5

---

## Phase 5: Simplify Orchestrator

### Task 5.1
**Goal**: Simplify orchestrator Run() method
**Files**: Edit `internal/orchestrator/orchestrator.go`
**Changes**:
- Replace infinite loop `Run()` with single-pass `RunOnce()`
- Flow: coding → testing → review → human_gate → return
- Remove auto-retry loops (let caller handle retries)
- Keep phase transition validation
**Risk**: High - core pipeline logic
**Before**: Start here
**After**: Proceed to 5.2

### Task 5.2
**Goal**: Remove complex phase router
**Files**: Delete `internal/orchestrator/phase_router.go` and `internal/orchestrator/phase_router_test.go`
**Impact**: Removes dynamic phase routing
**Risk**: Low - replaced by simple linear flow
**Before**: 5.1
**After**: Proceed to 5.3

### Task 5.3
**Goal**: Simplify history tracking
**Files**: Edit `internal/orchestrator/history.go`
**Changes**:
- Remove complex history graph
- Keep simple phase log: array of {phase, status, timestamp, output}
**Risk**: Medium - history used in UI
**Before**: 5.2
**After**: Proceed to 5.4

### Task 5.4
**Goal**: Simplify retry logic
**Files**: Edit `internal/orchestrator/retry.go`
**Changes**:
- Keep basic retry with exponential backoff
- Remove complex retry strategies
- Simplify `WithRetry` function
**Risk**: Low
**Before**: 5.3
**After**: Proceed to 5.5

### Task 5.5
**Goal**: Update human gate for simple approve/edit/refuse
**Files**: Edit `internal/orchestrator/human_gate.go`
**Changes**:
- Simplify `GateResponse` to include `Action` (approve/edit/refuse) and `Feedback`
- Remove timeout complexity
- Keep channel-based response mechanism
**Risk**: Medium - human interaction
**Before**: 5.4
**After**: Phase 5 complete → Proceed to Phase 6

---

## Phase 6: Add Chat API

### Task 6.1
**Goal**: Create chat types
**Files**: Create `internal/api/handlers/chat_types.go`
**Content**:
```go
type ChatRequest struct {
    Message string `json:"message"`
    FilePath string `json:"file_path,omitempty"`  // optional: file being edited
    LineNumber int `json:"line_number,omitempty"`  // optional: cursor position
}

type ChatResponse struct {
    Role string `json:"role"`  // "assistant" or "system"
    Content string `json:"content"`
    Phase string `json:"phase"`  // "coding", "testing", "review"
    Timestamp time.Time `json:"timestamp"`
}
```
**Risk**: Low - new types
**Before**: Start here
**After**: Proceed to 6.2

### Task 6.2
**Goal**: Create chat handler
**Files**: Create `internal/api/handlers/chat_handler.go`
**Endpoints**:
- `POST /api/chat/message` - Send message, trigger pipeline
- `GET /api/chat/history` - Get conversation history
**Logic**:
- Parse user request
- Call orchestrator with request + file context
- Stream responses back to client
- Store chat history in memory
**Risk**: Medium - new feature
**Before**: 6.1
**After**: Proceed to 6.3

### Task 6.3
**Goal**: Register chat routes in main.go
**Files**: Edit `cmd/daemon/main.go`
**Changes**:
- Add `chatHandler := handlers.NewChatHandler(...)`
- Register routes:
  ```go
  mux.HandleFunc("POST /api/chat/message", chatHandler.SendMessage)
  mux.HandleFunc("GET /api/chat/history", chatHandler.GetHistory)
  ```
**Risk**: Low - just registration
**Before**: 6.2
**After**: Phase 6 complete → Proceed to Phase 7

---

## Phase 7: Update UI for Chat

### Task 7.1
**Goal**: Add chat panel to IDE layout
**Files**: Edit `internal/api/templates/ide.html`
**Changes**:
- Add chat panel (right sidebar or bottom panel)
- Chat messages area (scrollable)
- Chat input with textarea and send button
- Phase indicator in chat header
**Risk**: Medium - UI layout change
**Before**: Start here
**After**: Proceed to 7.2

### Task 7.2
**Goal**: Create chat UI components
**Files**: Create `internal/api/templates/components/chat-message.html`
**Files**: Create `internal/api/templates/components/chat-input.html`
**Content**:
- `chat-message.html`: Render single message (user/assistant) with phase badge
- `chat-input.html`: Textarea + send button + file context indicator
**Risk**: Low - new components
**Before**: 7.1
**After**: Proceed to 7.3

### Task 7.3
**Goal**: Add chat JavaScript
**Files**: Create `internal/api/static/js/chat.js`
**Content**:
- Send message to `/api/chat/message`
- Receive and display responses
- Stream updates for multi-phase pipeline
- Handle accept/edit/refuse actions
**Risk**: Medium - client-side logic
**Before**: 7.2
**After**: Proceed to 7.4

### Task 7.4
**Goal**: Add review modal
**Files**: Create `internal/api/templates/components/review-modal.html`
**Content**:
- Shows generated code diff
- Shows test results
- Shows review report
- Buttons: Accept, Edit (with feedback), Refuse
**Risk**: Medium - new modal
**Before**: 7.3
**After**: Phase 7 complete → Proceed to Phase 8

---

## Phase 8: Final Cleanup & Testing

### Task 8.1
**Goal**: Update base template functions
**Files**: Edit `internal/api/templates/base.html`
**Changes**:
- Remove HTMX batcher (simplify)
- Remove debounce utilities
- Keep: HTMX config, Tailwind config
**Risk**: Low - JS utilities
**Before**: Start here
**After**: Proceed to 8.2

### Task 8.2
**Goal**: Remove test files for deleted code
**Files**: Delete test files for removed functionality:
- `internal/state/store_test.go` (rewrite for simplified store)
- `internal/agent/skills/*_test.go` (already deleted with skills)
- `internal/orchestrator/phase_router_test.go` (already deleted)
- `internal/orchestrator/retry_test.go` (rewrite for simplified retry)
- `internal/orchestrator/human_gate_test.go` (rewrite)
- `internal/orchestrator/errors_test.go` (review)
- `internal/orchestrator/history_test.go` (rewrite)
**Risk**: Low - test cleanup
**Before**: 8.1
**After**: Proceed to 8.3

### Task 8.3
**Goal**: Rewrite remaining tests
**Files**: Edit test files to match simplified code:
- `internal/state/store_test.go`
- `internal/orchestrator/retry_test.go`
- `internal/orchestrator/human_gate_test.go`
- `internal/orchestrator/history_test.go`
- `internal/agent/coder_test.go`
- `internal/agent/tester_test.go`
- `internal/agent/reviewer_test.go`
- `internal/agent/client_test.go`
- `internal/agent/orchestrator_test.go`
- `internal/tools/*_test.go`
**Risk**: Medium - tests must pass
**Before**: 8.2
**After**: Proceed to 8.4

### Task 8.4
**Goal**: Build and test
**Commands**:
```bash
cd /Users/marcoandreose/DEV/lab/mini-orca
go build ./...
go test ./...
```
**Risk**: High - final validation
**Before**: 8.3
**After**: Proceed to 8.5

### Task 8.5
**Goal**: Clean up build artifacts
**Files**: Delete:
- `mini-orca` (binary)
- `daemon` (binary)
- `build/coverage.html`
**Risk**: Low
**Before**: 8.4
**After**: Proceed to 8.6

### Task 8.6
**Goal**: Update documentation
**Files**: Update:
- `README.md` - Simplified architecture
- `API.md` - New endpoints only
- `CONFIG.md` - Simplified config
- `Makefile` - Remove unused targets
**Risk**: Low - documentation only
**Before**: 8.5
**After**: ✅ Cleanup complete!

---

## Quick Reference: Files by Phase

### Phase 1 (Sessions) - 7 tasks
- Delete: `session.go`, `session_handler.go`, `session_types.go`, `gate.go`
- Edit: `store.go`, `orchestrator.go`, `main.go`

### Phase 2 (Projects) - 5 tasks
- Delete: `project_handler.go`, `project_types.go`, `project_files.go`
- Edit: `htmx_render.go`, `htmx_filetree.go`

### Phase 3 (Skills) - 5 tasks
- Delete: `skills/` directory, `skills_handler.go`, `skills_types.go`
- Edit: `config.go`, `registry.go`

### Phase 4 (UI) - 7 tasks
- Delete: 16 components, 9 phase templates, 1 editor, 5 CSS files
- Edit: `base.html`, `ide.html`, `htmx_templates.go`

### Phase 5 (Orchestrator) - 5 tasks
- Edit: `orchestrator.go`, `human_gate.go`
- Delete: `phase_router.go`, `phase_router_test.go`
- Edit: `history.go`, `retry.go`

### Phase 6 (Chat API) - 3 tasks
- Create: `chat_types.go`, `chat_handler.go`, `chat.js`
- Edit: `main.go`

### Phase 7 (Chat UI) - 4 tasks
- Edit: `ide.html`
- Create: `chat-message.html`, `chat-input.html`, `review-modal.html`, `chat.js`

### Phase 8 (Final) - 6 tasks
- Edit: `base.html`, test files, `README.md`, `API.md`, `CONFIG.md`, `Makefile`
- Delete: binaries, coverage.html

**Total: 42 tasks across 8 phases**
