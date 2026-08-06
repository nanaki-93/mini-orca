# Task 3.12: Split internal/api/templates/components/agent-skills.html

## Goal
Split 179-line file into 2 files: container + badge rendering.

## Files to CREATE
- `internal/api/templates/components/agent-skills.html` (~80 lines) — Container
- `internal/api/templates/components/agent-skill-badge.html` (~100 lines) — Badge

## Files to DELETE
- `internal/api/templates/components/agent-skills.html` (original)

---

## File 1: agent-skills.html (~80 lines)

**Keep:**
- Skills container div
- Agent name header
- Loop over skills: `{{ range .Skills }}`

```html
{{ define "agent-skills" }}
<div id="agent-skills" class="agent-skills">
    <h4>{{ .AgentName }} Skills</h4>
    <div class="skills-list">
        {{ range .Skills }}
        {{ template "agent-skill-badge" . }}
        {{ end }}
    </div>
</div>
{{ end }}
```

---

## File 2: agent-skill-badge.html (~100 lines)

**Move here:**
- `{{ define "agent-skill-badge" }}` — Individual badge
- Badge content: skill name, description tooltip, icon
- Badge CSS

```html
{{ define "agent-skill-badge" }}
<div class="skill-badge" title="{{ .Description }}">
    <span class="badge-icon">{{ .Icon }}</span>
    <span class="badge-name">{{ .Name }}</span>
</div>
{{ end }}
```

---

## Implementation Steps
1. Create `agent-skill-badge.html` — extract badge template
2. Rewrite `agent-skills.html` — keep container + range loop
3. Delete original `agent-skills.html`

## Verification
- `go build ./...` succeeds
- Both files exist
- No file exceeds 100 lines
- Agent skills render correctly
