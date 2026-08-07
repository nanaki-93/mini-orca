# Mini-Orca Cleanup Plan

## Goal
Simplify the project to a focused IDE-app with:
- File tree sidebar
- config.yaml for all models
- 3 agents: coder, tester, reviewer
- Simple workflow: open file → select position → chat for function → coder writes → tester tests → reviewer reviews → user approves/edits/refuses

---

## Phase 1: Remove Session System

### Files to DELETE:
```
internal/state/session.go                    # Session type definitions
internal/api/session_handler.go              # Session HTTP handlers
internal/api/session_types.go                # Session API types
internal/api/gate.go                         # Gate store (session-based)
```

### Files to MODIFY:
```
internal/state/store.go                      # Remove session-related methods, keep only phase tracking
internal/api/handlers/htmx_render.go         # Remove session references
internal/api/handlers/htmx_phase.go          # Simplify phase rendering
cmd/daemon/main.go                           # Remove session store initialization
```

### Changes:
- Replace multi-session management with a single "active session" concept
- Remove endpoints: `POST /api/sessions`, `GET /api/sessions`, `POST /api/sessions/:id/start|pause|resume|stop`
- Keep: phase tracking (coding → testing → review → human_review)

---

## Phase 2: Remove Project System

### Files to DELETE:
```
internal/api/handlers/project_handler.go     # Project HTTP handlers
internal/api/handlers/project_types.go       # Project API types
internal/api/handlers/project_files.go       # File listing via API
```

### Files to MODIFY:
```
cmd/daemon/main.go                           # Remove project store, use config path directly
internal/api/handlers/htmx_render.go         # Get project from config, not store
internal/api/handlers/htmx_filetree.go       # Direct file tree from filesystem
```

### Changes:
- Project path comes from config.yaml (or cwd)
- Remove endpoints: `GET /api/projects`, `POST /api/projects`, `GET /api/projects/:id/files`
- File tree reads directly from the project directory on disk

---

## Phase 3: Remove Skills System

### Files to DELETE:
```
internal/agent/skills/                      # Entire skills directory
  - library.go
  - registry.go
  - registry_test.go
  - skills.go
  - utils.go
  - utils_test.go
internal/api/skills_handler.go
internal/api/skills_types.go
```

### Files to MODIFY:
```
internal/config/config.go                    # Remove SkillsConfig
cmd/daemon/main.go                           # Remove skills registry
internal/agent/registry.go                   # Remove skills from agent creation
```

### Changes:
- Skills become simple config strings in config.yaml (already supported)
- Agents read skills directly from config, no registry needed

---

## Phase 4: Simplify UI Components

### Files to DELETE (components):
```
internal/api/templates/components/accessibility-results.html
internal/api/templates/components/accessibility-testing.html
internal/api/templates/components/agent-skill-badge.html
internal/api/templates/components/agent-skills.html
internal/api/templates/components/bulk-action-item.html
internal/api/templates/components/bulk-actions.html
internal/api/templates/components/dashboard.html
internal/api/templates/components/header_settings.html
internal/api/templates/components/mobile-sidebar.html
internal/api/templates/components/project-form.html
internal/api/templates/components/project-modal.html
internal/api/templates/components/session-controls.html
internal/api/templates/components/skill-card.html
internal/api/templates/components/skill-field.html
internal/api/templates/components/skill-form.html
internal/api/templates/components/skills-library.html
```

### Files to DELETE (CSS):
```
internal/api/static/css/accessibility.css
internal/api/static/css/activity-log.css
internal/api/static/css/dashboard.css
internal/api/static/css/skills.css
internal/api/static/css/responsive.css
```

### Files to KEEP (components):
```
internal/api/templates/components/file-tree.html
internal/api/templates/components/file-tree-item.html
internal/api/templates/components/file-tree-search.html
internal/api/templates/components/phase-tracker.html
internal/api/templates/components/phase-node.html
internal/api/templates/components/activity-entry.html
internal/api/templates/components/activity-log.html
internal/api/templates/components/feedback-input.html
internal/api/templates/components/header.html
```

### Files to KEEP (editors):
```
internal/api/templates/editors/code-editor.html
```

### Files to DELETE (phases - reduce to minimal):
```
internal/api/templates/phases/completed/       # Delete entire directory
internal/api/templates/phases/coding-content.html
internal/api/templates/phases/coding-structure.html
internal/api/templates/phases/testing-content.html
internal/api/templates/phases/testing-structure.html
internal/api/templates/phases/review-content.html
internal/api/templates/phases/review-structure.html
internal/api/templates/phases/human-review-content.html
internal/api/templates/phases/human-review-structure.html
```

### Files to MODIFY:
```
internal/api/templates/base.html               # Simplify header/sidebar/main/footer
internal/api/templates/ide.html                # Simplify layout
internal/api/handlers/htmx_templates.go        # Remove unused template funcs
internal/api/static/css/main.css               # Keep only essential styles
internal/api/static/css/editor.css             # Keep code editor styles
internal/api/static/css/sidebar.css            # Keep file tree styles
internal/api/static/css/phase-tracker.css      # Keep phase tracker styles
internal/api/static/css/modal.css              # Keep for review modal
internal/api/static/css/toast.css              # Keep for notifications
internal/api/static/css/header.css             # Keep header styles
```

---

## Phase 5: Simplify Orchestrator

### Files to MODIFY:
```
internal/orchestrator/orchestrator.go          # Simplify Run() to single request flow
internal/orchestrator/phase_router.go          # Remove complex routing
internal/orchestrator/history.go               # Simplify history tracking
internal/orchestrator/retry.go                 # Simplify or remove
```

### New Flow:
```
User opens file + selects position + types request
    ↓
Coder agent generates function
    ↓
Tester agent generates + runs tests
    ↓
Reviewer agent reviews code
    ↓
Show result to user (edit/accept/refuse)
```

---

## Phase 6: New Chat API

### New Endpoints to ADD:
```
POST /api/chat/message       - Send user request, get streaming response
GET  /api/chat/history       - Get chat history for current session
```

### New Files to CREATE:
```
internal/api/handlers/chat_handler.go
internal/api/handlers/chat_types.go
```

---

## Summary of What Stays

### Core Go Files:
```
cmd/daemon/main.go                    # Entry point (simplified)
internal/config/config.go             # Config loading
internal/llm/client.go                # LLM client
internal/agent/agent.go               # Base agent
internal/agent/coder.go               # Coder agent
internal/agent/tester.go              # Tester agent
internal/agent/reviewer.go            # Reviewer agent
internal/agent/client.go              # Agent LLM client
internal/agent/prompts/               # Agent prompts
internal/agent/orchestrator.go        # Agent orchestrator (simplified)
internal/orchestrator/orchestrator.go # Pipeline orchestrator (simplified)
internal/orchestrator/human_gate.go   # Human approval gate
internal/state/store.go               # State management (simplified)
internal/tools/                       # File ops, executors
internal/logging/logging.go           # Logging
internal/errors/errors.go             # Error types
```

### UI Files:
```
internal/api/templates/base.html
internal/api/templates/ide.html
internal/api/templates/components/file-tree*.html
internal/api/templates/components/phase-tracker.html
internal/api/templates/components/phase-node.html
internal/api/templates/components/activity*.html
internal/api/templates/components/feedback-input.html
internal/api/templates/components/header.html
internal/api/templates/editors/code-editor.html
internal/api/static/css/main.css
internal/api/static/css/editor.css
internal/api/static/css/sidebar.css
internal/api/static/css/phase-tracker.css
internal/api/static/css/modal.css
internal/api/static/css/toast.css
internal/api/static/css/header.css
```

### Config:
```
config.yaml                           # All configuration
```

---

## Estimated Impact

| Category | Before | After | Reduction |
|----------|--------|-------|-----------|
| Go files | ~80 | ~35 | -56% |
| HTML templates | ~60 | ~20 | -67% |
| CSS files | 12 | 6 | -50% |
| API endpoints | ~25 | ~8 | -68% |
| Lines of code | ~15,000 | ~5,000 | -67% |

---

## Execution Order

1. **Phase 1**: Remove session system (safest, first step)
2. **Phase 2**: Remove project system
3. **Phase 3**: Remove skills system
4. **Phase 4**: Clean up UI components (visual confirmation)
5. **Phase 5**: Simplify orchestrator
6. **Phase 6**: Add new chat API
7. **Final**: Test build, verify everything works
