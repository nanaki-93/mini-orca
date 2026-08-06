# Task 3.11: Split internal/api/templates/components/file-tree.html

## Goal
Split 173-line file into 2 files: tree container + item rendering.

## Files to CREATE
- `internal/api/templates/components/file-tree.html` (~80 lines) — Tree container
- `internal/api/templates/components/file-tree-item.html` (~93 lines) — Item rendering

## Files to DELETE
- `internal/api/templates/components/file-tree.html` (original)

---

## File 1: file-tree.html (~80 lines)

**Keep:**
- Tree container div
- Root path display
- Search/filter input
- Loop over items: `{{ range .Items }}`

```html
{{ define "file-tree" }}
<div id="file-tree" class="file-tree">
    <div class="tree-header">
        <span class="tree-path">{{ .RootPath }}</span>
        <input type="text" placeholder="Search files..." />
    </div>
    <div class="tree-content">
        {{ range .Items }}
        {{ template "file-tree-item" . }}
        {{ end }}
    </div>
</div>
{{ end }}
```

---

## File 2: file-tree-item.html (~93 lines)

**Move here:**
- `{{ define "file-tree-item" }}` — Individual item
- Item content: icon, name, type (file/folder)
- Folder expand/collapse logic
- File click handler
- Item CSS

```html
{{ define "file-tree-item" }}
<div class="tree-item {{ if .IsFolder }}folder{{ else }}file{{ end }}"
     data-path="{{ .Path }}"
     {{ if .IsFolder }}data-expanded="{{ .Expanded }}"{{ end }}>
    <span class="tree-icon">{{ .Icon }}</span>
    <span class="tree-name">{{ .Name }}</span>
</div>
{{ end }}
```

---

## Implementation Steps
1. Create `file-tree-item.html` — extract item template
2. Rewrite `file-tree.html` — keep container + range loop
3. Delete original `file-tree.html`

## Verification
- `go build ./...` succeeds
- Both files exist
- No file exceeds 93 lines
- File tree renders correctly
