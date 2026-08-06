# Task 2.1: Split internal/api/handlers/htmx_render.go

## Goal
Split 1493-line file into 5 focused files, each under 300 lines.

## Files to CREATE
- `internal/api/handlers/htmx_types.go`
- `internal/api/handlers/htmx_templates.go`
- `internal/api/handlers/htmx_render.go` (reduced)
- `internal/api/handlers/htmx_phase.go`
- `internal/api/handlers/htmx_filetree.go`
- `internal/api/handlers/htmx_dashboard.go`

---

## File 1: htmx_types.go (~150 lines)

**Move ALL data structs here:**

```go
package handlers

// PhaseRenderData holds data for rendering phase partials.
type PhaseRenderData struct { ... }

// CodingPhaseData holds data for the coding phase partial.
type CodingPhaseData struct { ... }

// FileOp represents a file operation in the coding phase.
type FileOp struct { ... }

// NextUnit represents a unit waiting in the queue.
type NextUnit struct { ... }

// TestingPhaseData holds data for the testing phase partial.
type TestingPhaseData struct { ... }

// TestResult represents a single test result.
type TestResult struct { ... }

// ReviewPhaseData holds data for the review phase partial.
type ReviewPhaseData struct { ... }

// ReviewIssue represents an issue found during review.
type ReviewIssue struct { ... }

// HumanReviewPhaseData holds data for the human review phase partial.
type HumanReviewPhaseData struct { ... }

// FileTreeRenderData holds data for rendering the file tree partial.
type FileTreeRenderData struct { ... }

// FileSystemItem represents a file or directory in the tree.
type FileSystemItem struct { ... }

// ActivityLogRenderData holds data for rendering the activity log partial.
type ActivityLogRenderData struct { ... }

// ActivityEntry represents a single activity log entry.
type ActivityEntry struct { ... }

// PhaseTrackerRenderData holds data for rendering the phase tracker partial.
type PhaseTrackerRenderData struct { ... }

// PhaseTrackerItem represents a phase in the tracker.
type PhaseTrackerItem struct { ... }

// DashboardRenderData holds data for the combined dashboard partial.
type DashboardRenderData struct { ... }

// RenderErrorResponse represents an error response for render endpoints.
type RenderErrorResponse struct { ... }

// RenderStatusResponse represents the status response for render endpoints.
type RenderStatusResponse struct { ... }
```

---

## File 2: htmx_templates.go (~200 lines)

**Move here:**
- `TemplateEngine` struct
- `NewTemplateEngine()`
- `loadTemplates()`
- `funcMap()` — entire function
- `RenderPhasePartial()`
- `RenderComponentPartial()`
- `RenderMain()`

```go
package handlers

import (
    "crypto/md5"
    "encoding/hex"
    "html/template"
    "os"
    "path/filepath"
    "strings"
    "sync"
    "time"
    "unicode"
    ...
)

type TemplateEngine struct { ... }

func NewTemplateEngine(basePath string) (*TemplateEngine, error) { ... }
func (te *TemplateEngine) loadTemplates() error { ... }
func (te *TemplateEngine) funcMap() template.FuncMap { ... }
func (te *TemplateEngine) RenderPhasePartial(phase string, data PhaseRenderData) (string, error) { ... }
func (te *TemplateEngine) RenderComponentPartial(name string, data interface{}) (string, error) { ... }
func (te *TemplateEngine) RenderMain(w http.ResponseWriter, contentTemplate string, data map[string]interface{}) { ... }
```

---

## File 3: htmx_render.go (~100 lines) — REDUCED

**Keep only:**
- `HTMXRenderHandler` struct
- `NewHTMXRenderHandler()`
- `RenderMainPage()`

```go
package handlers

type HTMXRenderHandler struct { ... }

func NewHTMXRenderHandler(...) *HTMXRenderHandler { ... }

func (h *HTMXRenderHandler) RenderMainPage(w http.ResponseWriter, r *http.Request) { ... }
```

**Delete from this file:**
- All type definitions (moved to htmx_types.go)
- TemplateEngine code (moved to htmx_templates.go)
- RenderPhase (moved to htmx_phase.go)
- RenderFileTree, ExpandFolder, ViewFile (moved to htmx_filetree.go)
- RenderActivityLog, RenderPhaseTracker, RenderDashboard (moved to htmx_dashboard.go)
- Helper functions like extractPhaseFromPath, getCurrentPhaseInfo, etc. (moved to appropriate files)
- fileExtension, fileIcon helper functions (move to htmx_templates.go)

---

## File 4: htmx_phase.go (~200 lines)

**Move here:**
- `RenderPhase()` handler
- `extractPhaseFromPath()`
- `getPhaseStep()`
- `getCurrentPhaseInfo()`
- `getCurrentPhaseName()`
- `getPhaseProgress()`

```go
package handlers

func (h *HTMXRenderHandler) RenderPhase(w http.ResponseWriter, r *http.Request) { ... }

func extractPhaseFromPath(path string) string { ... }
func getPhaseStep(phase string) int { ... }
func getCurrentPhaseInfo(session *state.Session) (string, string) { ... }
func getCurrentPhaseName(session *state.Session) string { ... }
func getPhaseProgress(status state.SessionStatus) int { ... }
```

---

## File 5: htmx_filetree.go (~250 lines)

**Move here:**
- `RenderFileTree()`
- `ExpandFolder()`
- `ViewFile()`

```go
package handlers

func (h *HTMXRenderHandler) RenderFileTree(w http.ResponseWriter, r *http.Request) { ... }
func (h *HTMXRenderHandler) ExpandFolder(w http.ResponseWriter, r *http.Request) { ... }
func (h *HTMXRenderHandler) ViewFile(w http.ResponseWriter, r *http.Request) { ... }
```

**Helper functions to include:**
- `fileExtension()`
- `fileIcon()`

---

## File 6: htmx_dashboard.go (~250 lines)

**Move here:**
- `RenderDashboard()`
- `RenderActivityLog()`
- `RenderPhaseTracker()`

```go
package handlers

func (h *HTMXRenderHandler) RenderDashboard(w http.ResponseWriter, r *http.Request) { ... }
func (h *HTMXRenderHandler) RenderActivityLog(w http.ResponseWriter, r *http.Request) { ... }
func (h *HTMXRenderHandler) RenderPhaseTracker(w http.ResponseWriter, r *http.Request) { ... }
```

---

## Implementation Order
1. Create `htmx_types.go` — copy all struct definitions
2. Create `htmx_templates.go` — copy TemplateEngine + funcMap
3. Create `htmx_phase.go` — copy phase-related handlers + helpers
4. Create `htmx_filetree.go` — copy file tree handlers + helpers
5. Create `htmx_dashboard.go` — copy dashboard/agent handlers
6. Rewrite `htmx_render.go` — keep only HTMXRenderHandler struct + New + RenderMainPage
7. Delete the original `htmx_render.go` content

## Verification
- `go build ./internal/api/handlers/...` succeeds
- All 6 files exist
- No file exceeds 300 lines
- All tests pass: `go test ./internal/api/handlers/...`
