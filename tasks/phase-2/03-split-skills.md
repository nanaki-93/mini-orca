# Task 2.3: Split internal/api/skills.go

## Goal
Split 604-line file into 2 files: types + handler.

## Files to CREATE
- `internal/api/skills_types.go`
- `internal/api/skills_handler.go`

## Files to DELETE
- `internal/api/skills.go`

---

## File 1: skills_types.go (~150 lines)

**Move here:**
- All data struct definitions (SkillResponse, SkillListResponse, etc.)
- `SkillStore` struct
- `NewSkillStore()`
- All Store methods (CreateSkill, GetSkill, UpdateSkill, DeleteSkill, ListSkills, etc.)
- Helper functions for skills

```go
package api

type SkillResponse struct { ... }
type SkillListResponse struct { ... }

type SkillStore struct { ... }
func NewSkillStore() *SkillStore { ... }
func (s *SkillStore) CreateSkill(...) (*Skill, error) { ... }
func (s *SkillStore) GetSkill(id string) (*Skill, error) { ... }
func (s *SkillStore) UpdateSkill(id string, ...) error { ... }
func (s *SkillStore) DeleteSkill(id string) error { ... }
func (s *SkillStore) ListSkills() ([]*Skill, error) { ... }
```

---

## File 2: skills_handler.go (~450 lines)

**Move here:**
- `SkillHandler` struct
- `NewSkillHandler()`
- All HTTP handler methods:
  - `CreateSkill()`
  - `GetSkill()`
  - `UpdateSkill()`
  - `DeleteSkill()`
  - `ListSkills()`
  - `GetSkillKnowledge()`
  - `UpdateSkillKnowledge()`
  - `ListSkillTools()`
  - `UpdateSkillTool()`

```go
package api

type SkillHandler struct { ... }
func NewSkillHandler(...) *SkillHandler { ... }
func (h *SkillHandler) CreateSkill(w http.ResponseWriter, r *http.Request) { ... }
func (h *SkillHandler) GetSkill(w http.ResponseWriter, r *http.Request) { ... }
func (h *SkillHandler) UpdateSkill(w http.ResponseWriter, r *http.Request) { ... }
func (h *SkillHandler) DeleteSkill(w http.ResponseWriter, r *http.Request) { ... }
func (h *SkillHandler) ListSkills(w http.ResponseWriter, r *http.Request) { ... }
// ... all other handler methods
```

## Verification
- `go build ./internal/api/...` succeeds
- `go test ./internal/api/...` passes
- No file exceeds 450 lines
