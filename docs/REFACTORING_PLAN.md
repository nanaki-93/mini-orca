# Frontend & Architecture Simplification Plan

## Overview

This plan details the refactoring steps to:
1. **Remove all model/provider configuration** — configuration flows entirely from `config.yaml`
2. **Split large files** into smaller, single-responsibility components (no file > 300 lines)

---

## Phase 1: Remove Model/Provider Layer (Backend)

### Task 1.1: Simplify `internal/config/config.go`
**File:** `internal/config/config.go`
**Goal:** Remove `ModelsConfig`, `ProviderConfig`, `PhaseModelConfig` — replace with flat config

**Changes:**
- Remove `ModelsConfig`, `ProviderConfig`, `PhaseModelConfig` structs
- Add new flat config fields to `Config`:
  ```go
  type Config struct {
      LLM     LLMConfig     `json:"llm" yaml:"llm"`
      Agents  AgentsConfig  `json:"agents" yaml:"agents"`
      Skills  SkillsConfig  `json:"skills" yaml:"skills"`
      Retry   RetryConfig   `json:"retry" yaml:"retry"`
      Logging LoggingConfig `json:"logging" yaml:"logging"`
  }

  type LLMConfig struct {
      BaseURL     string  `json:"base_url" yaml:"base_url"`
      APIKey      string  `json:"api_key,omitempty" yaml:"api_key,omitempty"`
      Model       string  `json:"model" yaml:"model"`
      Temperature float32 `json:"temperature,omitempty" yaml:"temperature,omitempty"`
      MaxTokens   int     `json:"max_tokens,omitempty" yaml:"max_tokens,omitempty"`
  }
  ```
- Update `applyDefaults()` to set `LLM.BaseURL = DefaultProviderURL`
- Update `Validate()` to only check `LLM.BaseURL`

**Dependencies to update:** `internal/config/default.go`, all files importing `config.ModelsConfig`

---

### Task 1.2: Delete `internal/model/` package entirely
**Files to DELETE:**
- `internal/model/provider.go`
- `internal/model/types.go`
- `internal/model/router.go`
- `internal/model/lm_studio.go`
- `internal/model/router_test.go`
- `internal/model/lm_studio_test.go`
- `internal/model/full_flow_test.go`

---

### Task 1.3: Create `internal/llm/client.go` (replacement)
**File:** `internal/llm/client.go` (NEW)
**Goal:** Simple, single-responsibility LLM client — no router, no provider abstraction

```go
package llm

type Client struct {
    baseURL     string
    apiKey      string
    model       string
    temperature float32
    maxTokens   int
    httpClient  *http.Client
}

func NewClient(baseURL, apiKey, model string, temperature float32, maxTokens int) *Client
func (c *Client) Chat(ctx context.Context, messages []ChatMessage) (*ChatResponse, error)
func (c *Client) ListModels(ctx context.Context) ([]Model, error)
```

**Lines target:** ~150 lines

---

### Task 1.4: Update `internal/agent/client.go`
**File:** `internal/agent/client.go`
**Goal:** Replace `*model.Router` with `*llm.Client`

**Changes:**
- Change `router *model.Router` → `llmClient *llm.Client`
- Update `NewClient()` signature
- Update `Execute()` to call `c.llmClient.Chat()` directly
- Remove `ListModels()` or simplify to just call `c.llmClient.ListModels()`

---

### Task 1.5: Update `internal/agent/registry.go`
**File:** `internal/agent/registry.go`
**Goal:** Update to use `*llm.Client` instead of `*model.Router`

---

### Task 1.6: Update `internal/agent/coder.go`, `tester.go`, `reviewer.go`
**Files:** `internal/agent/coder.go`, `internal/agent/tester.go`, `internal/agent/reviewer.go`
**Goal:** Update constructor signatures from `*model.Router` to `*llm.Client`

---

### Task 1.7: Update `internal/agent/orchestrator.go`
**File:** `internal/agent/orchestrator.go`
**Goal:** Update to use `*llm.Client`

---

### Task 1.8: Update `cmd/daemon/main.go`
**File:** `cmd/daemon/main.go`
**Goal:** Remove model router initialization, use simple LLM client

**Changes:**
- Remove `model.InitRouter(cfg)` call
- Add `llm.NewClient()` call with config values
- Remove `router.ListProviders()`, `router.ListModels()` logging
- Update agent creation to pass `*llm.Client`

---

### Task 1.9: Update `internal/api/handlers/config.go`
**File:** `internal/api/handlers/config.go`
**Goal:** Remove model/provider-specific endpoints and types

**Changes:**
- Remove `ModelListResponse`, `PhaseConfigResponse` types
- Remove `ListModels()` and `GetPhaseConfigs()` handler methods
- Remove `ConfigUpdateRequest.Models` field
- Simplify `ConfigResponse` to not include models
- Remove `updateModels()` and `copyConfig()` model-related code

---

### Task 1.10: Update `internal/api/handlers/htmx_render.go`
**File:** `internal/api/handlers/htmx_render.go`
**Goal:** Remove model/provider references from `RenderMainPage`

**Changes:**
- Remove `data["ModelName"] = "gpt-4o"` and `data["ProviderName"] = "OpenAI"`
- These values should come from config if needed, or be removed entirely

---

### Task 1.11: Update `internal/api/handlers/system.go`
**File:** `internal/api/handlers/system.go`
**Goal:** Remove model/provider info from system info endpoint

---

### Task 1.12: Update frontend HTML templates
**Files:**
- `internal/api/templates/components/header.html`
- `internal/api/templates/components/model-config-panel.html` (DELETE — no more model config UI)
- `internal/api/templates/ide.html` (remove model references in footer)

**Changes:**
- Remove all model/provider display from header
- Delete `model-config-panel.html` entirely
- Remove settings UI for model configuration
- Remove `ModelName`, `ProviderName`, `Temperature`, `MaxTokens` template variables

---

### Task 1.13: Update `config.yaml` and `config.example.yaml`
**Files:** `config.yaml`, `config.example.yaml`
**Goal:** Simplify config structure

**Before:**
```yaml
models:
  active_provider: "lm-studio"
  providers:
    lm-studio:
      base_url: "http://localhost:1234"
  phases:
    coding: { provider, model, temperature, max_tokens }
```

**After:**
```yaml
llm:
  base_url: "http://localhost:1234"
  api_key: ""
  model: ""
  temperature: 0.7
  max_tokens: 8192
```

---

## Phase 2: Split Large Go Files

### Task 2.1: Split `htmx_render.go` (1493 lines → multiple files)
**File:** `internal/api/handlers/htmx_render.go` (currently 1493 lines)

**Target structure:**
```
internal/api/handlers/
├── htmx_render.go          (~100 lines) — TemplateEngine + HTMXRenderHandler struct + New functions
├── htmx_phase.go           (~200 lines) — Phase rendering: RenderPhase, PhaseRenderData, phase helpers
├── htmx_filetree.go        (~250 lines) — File tree: RenderFileTree, ExpandFolder, ViewFile
├── htmx_dashboard.go       (~250 lines) — Dashboard: RenderDashboard, RenderPhaseTracker, RenderActivityLog
├── htmx_templates.go       (~200 lines) — funcMap, template helpers, fileIcon, etc.
└── htmx_types.go           (~150 lines) — All render data structs (PhaseRenderData, etc.)
```

**Specific splits:**

#### `htmx_types.go` — All data structs
Move here:
- `PhaseRenderData`, `CodingPhaseData`, `TestingPhaseData`, `ReviewPhaseData`, `HumanReviewPhaseData`
- `FileTreeRenderData`, `FileSystemItem`
- `ActivityLogRenderData`, `ActivityEntry`
- `PhaseTrackerRenderData`, `PhaseTrackerItem`
- `DashboardRenderData`
- `FileOp`, `NextUnit`, `TestResult`, `ReviewIssue`

#### `htmx_templates.go` — Template engine + funcMap
Move here:
- `TemplateEngine` struct
- `NewTemplateEngine()`
- `loadTemplates()`
- `funcMap()` — entire funcMap function
- `RenderPhasePartial()`, `RenderComponentPartial()`, `RenderMain()`

#### `htmx_render.go` — Handler struct + main page
Keep here:
- `HTMXRenderHandler` struct
- `NewHTMXRenderHandler()`
- `RenderMainPage()`

#### `htmx_phase.go` — Phase rendering
Move here:
- `RenderPhase()` handler
- `extractPhaseFromPath()`
- `getPhaseStep()`
- `getCurrentPhaseInfo()`, `getCurrentPhaseName()`
- Phase rendering logic

#### `htmx_filetree.go` — File tree operations
Move here:
- `RenderFileTree()`
- `ExpandFolder()`
- `ViewFile()`

#### `htmx_dashboard.go` — Dashboard + tracker + activity log
Move here:
- `RenderDashboard()`
- `RenderActivityLog()`
- `RenderPhaseTracker()`
- `getPhaseProgress()`

---

### Task 2.2: Split `session.go` (426 lines → 2 files)
**File:** `internal/api/session.go`

**Target structure:**
```
internal/api/
├── session_types.go    (~150 lines) — Request/Response structs, SessionStore
└── session_handler.go  (~280 lines) — SessionHandler + all HTTP handlers
```

---

### Task 2.3: Split `skills.go` (604 lines → 2 files)
**File:** `internal/api/skills.go`

**Target structure:**
```
internal/api/
├── skills_types.go     (~150 lines) — Skill-related structs, SkillStore
└── skills_handler.go   (~450 lines) — SkillHandler + HTTP handlers
```

---

### Task 2.4: Split `project.go` (679 lines → 2-3 files)
**File:** `internal/api/handlers/project.go`

**Target structure:**
```
internal/api/handlers/
├── project_types.go    (~150 lines) — Project, FileEntry structs, ProjectStore
├── project_handler.go  (~350 lines) — ProjectHandler + HTTP handlers
└── project_files.go    (~180 lines) — File operations: ListFiles, GetFileContent
```

---

### Task 2.5: Split `tools_test.go` (1452 lines → multiple files)
**File:** `internal/tools/tools_test.go`

**Target structure:**
```
internal/tools/
├── tools_test.go           (~100 lines) — Common test helpers
├── executor_test.go        (~200 lines) — Executor tests
├── file_ops_test.go        (~200 lines) — File operations tests
├── formatter_test.go       (~150 lines) — Formatter tests
├── go_executor_test.go     (existing, keep separate)
├── python_executor_test.go (~150 lines) — Python executor tests
├── typescript_executor_test.go (~150 lines) — TypeScript executor tests
└── ... other executor tests
```

---

## Phase 3: Split Frontend HTML Components

### Task 3.1: Delete `model-config-panel.html`
**File to DELETE:** `internal/api/templates/components/model-config-panel.html`

---

### Task 3.2: Split `base.html` (1072 lines → 2 files)
**File:** `internal/api/templates/base.html`

**Target structure:**
```
internal/api/templates/
├── base.html             (~150 lines) — HTML structure, block definitions, {{ block }} calls
├── base_styles.html      (~200 lines) — CSS variables, global styles, scrollbar
├── base_scripts.html     (~400 lines) — HTMX config, global JS, loading, toasts, error handling
└── base_modals.html      (~320 lines) — Project modal, other modals
```

**Split details:**
- `base.html`: Keep only the HTML skeleton, block definitions, and `{{ template }}` includes
- `base_styles.html`: Extract all `<style>` content
- `base_scripts.html`: Extract all `<script>` content (HTMX config, ScreenReader, FocusManager, Toast, etc.)
- `base_modals.html`: Extract `{{ template "project-modal" . }}` and related modal code

---

### Task 3.3: Split `ide.html` (77 lines → keep as is, but clean up)
**File:** `internal/api/templates/ide.html`
**Goal:** This file is already small. Just remove any model/provider references.

---

### Task 3.4: Split `header.html` (348 lines → 2 files)
**File:** `internal/api/templates/components/header.html`

**Target structure:**
```
internal/api/templates/components/
├── header.html           (~150 lines) — Header bar structure, project info, session controls
└── header_settings.html  (~200 lines) — Settings panel, model config (REMOVE model config)
```

---

### Task 3.5: Split `accessibility-testing.html` (786 lines → 2-3 files)
**File:** `internal/api/templates/components/accessibility-testing.html`

**Target structure:**
```
internal/api/templates/components/
├── accessibility-testing.html   (~300 lines) — Main accessibility panel + tests
└── accessibility-results.html   (~480 lines) — Results display, reports
```

---

### Task 3.6: Split `skills-library.html` (234 lines → 2 files)
**File:** `internal/api/templates/components/skills-library.html`

**Target structure:**
```
internal/api/templates/components/
├── skills-library.html    (~120 lines) — Library grid, search
└── skill-card.html        (~110 lines) — Individual skill card component
```

---

### Task 3.7: Split `project-modal.html` (364 lines → 2 files)
**File:** `internal/api/templates/components/project-modal.html`

**Target structure:**
```
internal/api/templates/components/
├── project-modal.html     (~150 lines) — Modal overlay, form structure
└── project-form.html      (~210 lines) — Form fields, validation, submit logic
```

---

### Task 3.8: Split phase templates (all ~250-400 lines each → 2 files each)

**For each phase template:**
```
internal/api/templates/phases/
├── coding.html            (~150 lines) — Phase structure, controls
├── coding-content.html    (~150 lines) — Code display, file operations
├── testing.html           (~200 lines) — Phase structure, controls
├── testing-content.html   (~200 lines) — Test results, coverage
├── review.html            (~170 lines) — Phase structure, controls
├── review-content.html    (~180 lines) — Issues list, scoring
└── human-review.html      (~320 lines) — Phase structure, controls
```

**Split details:**
- Each phase splits into: structure/controls + content display
- Remove any model/provider references

---

### Task 3.9: Split `activity-log.html` (378 lines → 2 files)
**File:** `internal/api/templates/components/activity-log.html`

**Target structure:**
```
internal/api/templates/components/
├── activity-log.html      (~150 lines) — Log container, filtering
└── activity-entry.html    (~230 lines) — Individual entry rendering
```

---

### Task 3.10: Split `phase-tracker.html` (275 lines → 2 files)
**File:** `internal/api/templates/components/phase-tracker.html`

**Target structure:**
```
internal/api/templates/components/
├── phase-tracker.html     (~120 lines) — Tracker container, navigation
└── phase-node.html        (~155 lines) — Individual phase node rendering
```

---

### Task 3.11: Split `file-tree.html` (173 lines → 2 files)
**File:** `internal/api/templates/components/file-tree.html`

**Target structure:**
```
internal/api/templates/components/
├── file-tree.html         (~80 lines) — Tree container, root
└── file-tree-item.html    (~93 lines) — Individual item (file/folder)
```

---

### Task 3.12: Split `agent-skills.html` (179 lines → 2 files)
**File:** `internal/api/templates/components/agent-skills.html`

**Target structure:**
```
internal/api/templates/components/
├── agent-skills.html      (~80 lines) — Agent skills container
└── agent-skill-badge.html (~100 lines) — Individual skill badge
```

---

### Task 3.13: Split `bulk-actions.html` (337 lines → 2 files)
**File:** `internal/api/templates/components/bulk-actions.html`

**Target structure:**
```
internal/api/templates/components/
├── bulk-actions.html      (~130 lines) — Bulk action container
└── bulk-action-item.html  (~207 lines) — Individual action item
```

---

### Task 3.14: Split `skill-form.html` (195 lines → 2 files)
**File:** `internal/api/templates/components/skill-form.html`

**Target structure:**
```
internal/api/templates/components/
├── skill-form.html        (~90 lines) — Form container
└── skill-field.html       (~105 lines) — Individual field rendering
```

---

### Task 3.15: Clean up `session-controls.html` and `mobile-sidebar.html`
**Files:** `internal/api/templates/components/session-controls.html`, `mobile-sidebar.html`
**Goal:** Remove model/provider references, keep as is (already small)

---

### Task 3.16: Remove model references from all phase templates
**Files:**
- `internal/api/templates/phases/coding.html`
- `internal/api/templates/phases/testing.html`
- `internal/api/templates/phases/review.html`
- `internal/api/templates/phases/human-review.html`

**Changes:**
- Remove `{{ .ModelName }}`, `{{ .ProviderName }}`, `{{ .Temperature }}`, `{{ .MaxTokens }}`
- Remove model config UI from phase controls

---

## Phase 4: CSS Cleanup

### Task 4.1: Extract component-specific CSS
**File:** `internal/api/static/css/main.css` (2742 lines)

**Target structure:**
```
internal/api/static/css/
├── main.css              (~500 lines) — Base styles, utilities, variables
├── header.css            (~200 lines) — Header styles
├── sidebar.css           (~300 lines) — Sidebar, file tree styles
├── editor.css            (~400 lines) — Editor, code display styles
├── dashboard.css         (~300 lines) — Dashboard, phase tracker styles
├── activity-log.css      (~200 lines) — Activity log styles
├── phase-tracker.css     (~250 lines) — Phase tracker styles
├── modal.css             (~200 lines) — Modal styles
├── toast.css             (~150 lines) — Toast/notification styles
├── accessibility.css     (~200 lines) — Accessibility panel styles
├── skills.css            (~150 lines) — Skills library styles
└── responsive.css        (~900 lines) — Responsive breakpoints
```

---

## Implementation Order

Execute in this order for minimal breakage:

```
1. Phase 1.1  → Simplify config/config.go
2. Phase 1.13 → Update config.yaml / config.example.yaml
3. Phase 1.2  → Delete internal/model/ (after verifying no imports)
4. Phase 1.3  → Create internal/llm/client.go
5. Phase 1.4  → Update internal/agent/client.go
6. Phase 1.5  → Update internal/agent/registry.go
7. Phase 1.6  → Update coder.go, tester.go, reviewer.go
8. Phase 1.7  → Update orchestrator.go
9. Phase 1.8  → Update cmd/daemon/main.go
10. Phase 1.9 → Update handlers/config.go
11. Phase 1.10→ Update handlers/htmx_render.go
12. Phase 1.11→ Update handlers/system.go
13. Phase 1.12→ Update HTML templates (remove model UI)
14. Phase 1.14→ Delete model-config-panel.html
15. Phase 2.1 → Split htmx_render.go
16. Phase 2.2 → Split session.go
17. Phase 2.3 → Split skills.go
18. Phase 2.4 → Split project.go
19. Phase 2.5 → Split tools_test.go
20. Phase 3.1 → Delete model-config-panel.html (if not done in 1.14)
21. Phase 3.2 → Split base.html
22. Phase 3.3 → Clean ide.html
23. Phase 3.4 → Split header.html
24. Phase 3.5 → Split accessibility-testing.html
25. Phase 3.6 → Split skills-library.html
26. Phase 3.7 → Split project-modal.html
27. Phase 3.8 → Split phase templates
28. Phase 3.9 → Split activity-log.html
29. Phase 3.10→ Split phase-tracker.html
30. Phase 3.11→ Split file-tree.html
31. Phase 3.12→ Split agent-skills.html
32. Phase 3.13→ Split bulk-actions.html
33. Phase 3.14→ Split skill-form.html
34. Phase 3.15→ Clean session-controls.html, mobile-sidebar.html
35. Phase 3.16→ Remove model refs from phase templates
36. Phase 4.1 → Split main.css
```

---

## File Size Targets

| File | Current | Target |
|------|---------|--------|
| `htmx_render.go` | 1493 | 100 (main) + 4 split files |
| `session.go` | 426 | 150 + 280 |
| `skills.go` | 604 | 150 + 450 |
| `project.go` | 679 | 150 + 350 + 180 |
| `tools_test.go` | 1452 | ~200 avg across 8 files |
| `base.html` | 1072 | 150 + 200 + 400 + 320 |
| `header.html` | 348 | 150 + 200 |
| `main.css` | 2742 | ~200 avg across 12 files |
| Phase templates | 250-400 each | 150 + 150-250 each |

---

## Verification Checklist

After each phase:
- [ ] `go build ./...` succeeds
- [ ] `go test ./...` passes
- [ ] No file exceeds 300 lines (except tests allowed up to 500)
- [ ] No model/provider references in config or templates
- [ ] `config.yaml` has flat `llm:` structure
- [ ] No dead imports remain
