# Task 3.6: Split internal/api/templates/components/skills-library.html

## Goal
Split 234-line file into 2 files: library grid + skill card.

## Files to CREATE
- `internal/api/templates/components/skills-library.html` (~120 lines) — Library grid
- `internal/api/templates/components/skill-card.html` (~110 lines) — Individual card

## Files to DELETE
- `internal/api/templates/components/skills-library.html` (original)

---

## File 1: skills-library.html (~120 lines)

**Keep:**
- Library container
- Search/filter bar
- Grid layout wrapper
- "Add Skill" button
- Loop over skills: `{{ range .Skills }}`

```html
{{ define "skills-library" }}
<div id="skills-library">
    <div class="library-header">
        <h3>Skills Library</h3>
        <input type="text" placeholder="Search skills..." />
        <button>Add Skill</button>
    </div>
    <div class="skills-grid">
        {{ range .Skills }}
        {{ template "skill-card" . }}
        {{ end }}
    </div>
</div>
{{ end }}
```

---

## File 2: skill-card.html (~110 lines)

**Move here:**
- `{{ define "skill-card" }}` — Individual skill card
- Card content: name, description, tags, actions
- Card CSS

```html
{{ define "skill-card" }}
<div class="skill-card">
    <div class="skill-card-header">
        <h4>{{ .Name }}</h4>
        <span class="skill-badge">{{ .Type }}</span>
    </div>
    <p>{{ .Description }}</p>
    <div class="skill-tags">
        {{ range .Tags }}<span>{{ . }}</span>{{ end }}
    </div>
    <div class="skill-actions">
        <button>Edit</button>
        <button>Delete</button>
    </div>
</div>
{{ end }}
```

---

## Implementation Steps
1. Create `skill-card.html` — extract card template
2. Rewrite `skills-library.html` — keep grid + range loop
3. Delete original `skills-library.html`

## Verification
- `go build ./...` succeeds
- Both files exist
- No file exceeds 120 lines
- Skills library renders correctly
