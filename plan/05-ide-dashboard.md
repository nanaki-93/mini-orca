# Mini-Orca v2.0 — IDE Dashboard

## 1. Overview

The frontend is an **IDE-like dashboard** built with HTMX, Alpine.js, and TailwindCSS. It provides:

- **File tree** — Navigate project structure
- **Code editor** — View/edit generated code (single function or full file)
- **Phase tracker** — Visual indicator of current phase
- **Activity log** — Scrollable log of agent actions
- **Standalone function writer** — Write functions outside the normal flow
- **Insertion tool** — Insert functions into any file at any position

**No build step, no npm, no bundler** — pure Go templates with HTMX for DOM updates.

---

## 2. Dashboard Layout

```
┌──────────────────────────────────────────────────────────────────────────────────────┐
│  HEADER                                                                              │
│  ┌────────────────────────────────────────────────────────────────────────────────┐  │
│  │  🐋 Mini-Orca  │  Project: ./my-app  │  Goal: Auth module  │  ⚙️ Config  │  🔄 Git │  │
│  └────────────────────────────────────────────────────────────────────────────────┘  │
├──────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                      │
│  ┌─────────────────────────┐  ┌──────────────────────────────────────────────────┐  │
│  │  FILE TREE              │  │  MAIN WORKSPACE                                   │  │
│  │                         │  │                                                   │  │
│  │  📁 src/                │  │  ┌──────────────────────────────────────────────┐ │  │
│  │  📁 auth/               │  │  │  Code Editor                                  │ │  │
│  │  📁 models/             │  │  │                                              │ │  │
│  │  📄 main.go             │  │  │  func ValidateToken(token string) bool {     │ │  │
│  │  📄 user.go             │  │  │      // generated code...                    │ │  │
│  │  📄 auth_handler.go     │  │  │  }                                           │ │  │
│  │  📄 business_logic.md   │  │  │                                              │ │  │
│  │                         │  │  │  ┌──────────────────────────────────────────┐ │ │  │
│  │  ┌───────────────────┐  │  │  │  │  Actions                                  │ │ │  │
│  │  │ 📝 Write Function │  │  │  │  │                                          │ │ │  │
│  │  └───────────────────┘  │  │  │  │  [Edit Function] [Edit Full File]        │ │ │  │
│  │  ┌───────────────────┐  │  │  │  │  [Submit]  [Cancel]                      │ │ │  │
│  │  │ 📎 Insert Function│  │  │  │  └──────────────────────────────────────────┘ │ │  │
│  │  └───────────────────┘  │  │  └──────────────────────────────────────────────┘ │  │
│  │                         │  │                                                   │  │
│  │  ─────────────────────  │  │  ┌──────────────────────────────────────────────┐ │  │
│  │  Session Info           │  │  │  Phase Tracker                                │ │  │
│  │  🎯 Goal: Auth module   │  │  │  [✓] Plan → [✓] Code → [○] Test → [○] Rev → │ │  │
│  │  📊 State: CODING       │  │  │  [✓] [✓] [○] [○] [○]                         │ │  │
│  │  ⏱️ Duration: 2m 34s    │  │  └──────────────────────────────────────────────┘ │  │
│  │  📝 Units: 3/12         │  │                                                   │  │
│  │  📁 Target: ./src/auth/ │  │  ┌──────────────────────────────────────────────┐ │  │
│  │                         │  │  │  Activity Log (scrollable)                    │ │  │
│  │  ─────────────────────  │  │  │  [12:34] Planning started                   │ │  │
│  │  [Pause] [Resume]       │  │  │  [12:35] Plan generated                     │ │  │
│  │  [Stop]                 │  │  │  [12:36] Code generated: ValidateToken      │ │  │
│  │                         │  │  │  [12:37] Tests passed                       │ │  │
│  └─────────────────────────┘  │  │  [12:38] Review passed                      │ │  │
│                                │  └──────────────────────────────────────────────┘ │  │
│                                └───────────────────────────────────────────────────┘  │
│                                                                                      │
└──────────────────────────────────────────────────────────────────────────────────────┘
```

---

## 3. Component Structure

```
internal/api/templates/
├── base.html                    ← Base layout (header, sidebar, main)
├── ide.html                     ← Main IDE page
├── components/
│   ├── header.html              ← Header bar (project info, model config, git status)
│   ├── sidebar.html             ← Left sidebar (file tree, session info)
│   ├── file-tree.html           ← File tree component
│   ├── phase-tracker.html       ← Phase progress indicator
│   └── activity-log.html        ← Activity feed
├── editors/
│   ├── code-editor.html         ← Code editor (function or full file)
│   ├── standalone-function.html ← Standalone function writer
│   └── insertion-picker.html    ← Insertion point picker
└── phases/
    ├── planning.html            ← Planning phase view
    ├── planning-review.html     ← Waiting for plan approval
    ├── coding.html              ← Coding phase view
    ├── testing.html             ← Testing phase view
    ├── review.html              ← Code review view
    └── human-review.html        ← Human review view
```

---

## 4. File Tree Component

```html
<!-- internal/api/templates/components/file-tree.html -->
<div class="bg-slate-900 border-r border-slate-800 p-4 h-full overflow-auto"
     hx-get="/api/project/files"
     hx-trigger="load, fileChanged from:body"
     hx-swap="innerHTML">
    <h3 class="text-xs font-semibold text-slate-400 uppercase mb-3">File Tree</h3>
    
    <div class="space-y-1">
        <!-- Business Logic File (special) -->
        <div class="flex items-center gap-2 px-2 py-1 rounded cursor-pointer hover:bg-slate-800 
                    text-sm text-amber-400"
             hx-get="/api/project/file"
             hx-vals='{"path": "business_logic.md"}'
             hx-target="#editor-content">
            <span>📄</span>
            <span>business_logic.md</span>
        </div>
        
        {{ range .Files }}
            <div class="flex items-center gap-2 px-2 py-1 rounded cursor-pointer hover:bg-slate-800 
                        text-sm text-slate-300"
                 hx-get="/api/project/file"
                 hx-vals='{"path": "{{ .Path }}"}'
                 hx-target="#editor-content">
                <span>📄</span>
                <span>{{ .Name }}</span>
            </div>
        {{ end }}
    </div>
    
    <!-- Action Buttons -->
    <div class="mt-4 pt-4 border-t border-slate-800 space-y-2">
        <button hx-get="/web/standalone-function"
                hx-target="#main-content"
                class="w-full px-3 py-2 bg-blue-600 hover:bg-blue-500 text-white text-sm rounded-lg transition cursor-pointer">
            📝 Write Function
        </button>
        
        <button hx-get="/web/insert-function"
                hx-target="#main-content"
                class="w-full px-3 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 text-sm rounded-lg transition cursor-pointer">
            📎 Insert Function
        </button>
    </div>
</div>
```

---

## 5. Code Editor Component

### 5.1 Function Editor (Edit Single Function)

```html
<!-- internal/api/templates/editors/code-editor.html -->
<div class="flex flex-col h-full">
    <!-- Editor Header -->
    <div class="flex items-center justify-between px-4 py-2 border-b border-slate-800 bg-slate-900">
        <div class="flex items-center gap-3">
            <span class="text-sm text-slate-400 font-mono">{{ .TargetFile }}</span>
            <span class="text-xs px-2 py-0.5 rounded bg-blue-500/20 text-blue-400">
                {{ .UnitType }}: {{ .UnitName }}
            </span>
        </div>
        
        <div class="flex items-center gap-2">
            <button hx-post="/api/project/edit/function"
                    hx-vals='{"path": "{{ .TargetFile }}", "unit_name": "{{ .UnitName }}"}'
                    hx-include="#function-editor"
                    class="px-4 py-2 bg-emerald-600 hover:bg-emerald-500 text-white text-sm font-medium rounded-lg transition cursor-pointer">
                Submit
            </button>
            <button hx-get="/web/phase/{{ .Phase }}"
                    hx-target="#main-content"
                    class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 text-sm rounded-lg transition cursor-pointer">
                Cancel
            </button>
        </div>
    </div>
    
    <!-- Code Editor -->
    <div class="flex-1 overflow-auto p-4">
        <textarea id="function-editor"
                  name="code"
                  class="w-full h-full bg-slate-950 border border-slate-800 rounded-lg p-4 
                         text-sm text-slate-300 font-mono resize-none focus:outline-none 
                         focus:border-blue-500"
                  spellcheck="false">{{ .Code }}</textarea>
    </div>
</div>
```

### 5.2 Full File Editor (Edit Entire File with New Unit)

```html
<!-- internal/api/templates/editors/full-file-editor.html -->
<div class="flex flex-col h-full">
    <!-- Editor Header -->
    <div class="flex items-center justify-between px-4 py-2 border-b border-slate-800 bg-slate-900">
        <div class="flex items-center gap-3">
            <span class="text-sm text-slate-400 font-mono">{{ .TargetFile }}</span>
            <span class="text-xs px-2 py-0.5 rounded bg-blue-500/20 text-blue-400">
                Full File ({{ .LinesAdded }} lines added)
            </span>
        </div>
        
        <div class="flex items-center gap-2">
            <button hx-post="/api/project/edit/file"
                    hx-vals='{"path": "{{ .TargetFile }}"}'
                    hx-include="#full-file-editor"
                    class="px-4 py-2 bg-emerald-600 hover:bg-emerald-500 text-white text-sm font-medium rounded-lg transition cursor-pointer">
                Submit
            </button>
            <button hx-get="/web/phase/{{ .Phase }}"
                    hx-target="#main-content"
                    class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 text-sm rounded-lg transition cursor-pointer">
                Cancel
            </button>
        </div>
    </div>
    
    <!-- Code Editor -->
    <div class="flex-1 overflow-auto p-4">
        <textarea id="full-file-editor"
                  name="code"
                  class="w-full h-full bg-slate-950 border border-slate-800 rounded-lg p-4 
                         text-sm text-slate-300 font-mono resize-none focus:outline-none 
                         focus:border-blue-500"
                  spellcheck="false">{{ .FullFileContent }}</textarea>
    </div>
</div>
```

---

## 6. Standalone Function Writer

```html
<!-- internal/api/templates/editors/standalone-function.html -->
<div class="space-y-4">
    <h2 class="text-lg font-semibold text-white">Write Standalone Function</h2>
    <p class="text-sm text-slate-400">
        Write a function outside the normal flow. After writing, click a file in the file tree
        to insert it at a specific position.
    </p>
    
    <div class="bg-slate-900 border border-slate-800 rounded-lg p-4">
        <label class="text-sm font-medium text-slate-300 block mb-2">Function Name</label>
        <input id="function-name"
               type="text"
               placeholder="e.g., ValidateToken"
               class="w-full bg-slate-950 border border-slate-800 rounded-lg p-3 text-sm 
                      text-slate-300 focus:outline-none focus:border-blue-500">
    </div>
    
    <div class="bg-slate-900 border border-slate-800 rounded-lg p-4">
        <label class="text-sm font-medium text-slate-300 block mb-2">Function Code</label>
        <textarea id="function-code"
                  name="code"
                  placeholder="Write your function here..."
                  class="w-full h-64 bg-slate-950 border border-slate-800 rounded-lg p-4 
                         text-sm text-slate-300 font-mono resize-none focus:outline-none 
                         focus:border-blue-500"
                  spellcheck="false"></textarea>
    </div>
    
    <div class="flex justify-end gap-3">
        <button hx-post="/api/project/standalone-function/save"
                hx-include="#function-name, #function-code"
                class="px-6 py-2 bg-blue-600 hover:bg-blue-500 text-white font-medium rounded-lg transition cursor-pointer">
            Save Function
        </button>
    </div>
    
    <!-- Instructions -->
    <div class="bg-blue-950/20 border border-blue-800/30 rounded-lg p-4">
        <h4 class="text-sm font-semibold text-blue-400 mb-2">Next Steps:</h4>
        <ol class="text-sm text-blue-300 space-y-1 list-decimal list-inside">
            <li>Save the function above</li>
            <li>Click a file in the file tree</li>
            <li>Choose where to insert the function</li>
            <li>Submit → Testing → Review → Human Review</li>
        </ol>
    </div>
</div>
```

---

## 7. Insertion Picker

```html
<!-- internal/api/templates/editors/insertion-picker.html -->
<div class="space-y-4">
    <h2 class="text-lg font-semibold text-white">Insert Function into File</h2>
    <p class="text-sm text-slate-400">
        Select the file and insertion point for your standalone function.
    </p>
    
    <!-- File Selection -->
    <div class="bg-slate-900 border border-slate-800 rounded-lg p-4">
        <label class="text-sm font-medium text-slate-300 block mb-2">Target File</label>
        <select id="target-file"
                hx-get="/api/project/files"
                hx-trigger="load"
                class="w-full bg-slate-950 border border-slate-800 rounded-lg p-3 text-sm 
                       text-slate-300 focus:outline-none focus:border-blue-500">
            {{ range .Files }}
                <option value="{{ .Path }}">{{ .Path }}</option>
            {{ end }}
        </select>
    </div>
    
    <!-- Insertion Point Selection -->
    <div class="bg-slate-900 border border-slate-800 rounded-lg p-4">
        <label class="text-sm font-medium text-slate-300 block mb-2">Insertion Point</label>
        <div class="space-y-2">
            <label class="flex items-center gap-3 p-3 rounded-lg bg-slate-950 border border-slate-800 cursor-pointer">
                <input type="radio" name="insertion_point" value="before_first" class="text-blue-500">
                <span class="text-sm text-slate-300">A. Insert before the first function</span>
            </label>
            
            <label class="flex items-center gap-3 p-3 rounded-lg bg-slate-950 border border-slate-800 cursor-pointer">
                <input type="radio" name="insertion_point" value="after_last" class="text-blue-500">
                <span class="text-sm text-slate-300">B. Insert after the last function</span>
            </label>
            
            <label class="flex items-center gap-3 p-3 rounded-lg bg-slate-950 border border-slate-800 cursor-pointer">
                <input type="radio" name="insertion_point" value="around" class="text-blue-500">
                <span class="text-sm text-slate-300">C. Insert before/after a specific function</span>
            </label>
        </div>
    </div>
    
    <!-- Specific Function Selection (shown when C is selected) -->
    <div id="specific-function-selector"
         x-show="selectedPoint === 'around'"
         class="bg-slate-900 border border-slate-800 rounded-lg p-4 hidden">
        <label class="text-sm font-medium text-slate-300 block mb-2">
            Select Reference Function
        </label>
        <select id="reference-function"
                class="w-full bg-slate-950 border border-slate-800 rounded-lg p-3 text-sm 
                       text-slate-300 focus:outline-none focus:border-blue-500">
            {{ range .Functions }}
                <option value="{{ .Name }}">{{ .Name }} (line {{ .Line }})</option>
            {{ end }}
        </select>
        
        <label class="text-sm font-medium text-slate-300 block mt-3 mb-2">
            Position Relative to Reference
        </label>
        <div class="flex gap-3">
            <button type="button"
                    class="flex-1 px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 text-sm rounded-lg transition cursor-pointer">
                Before
            </button>
            <button type="button"
                    class="flex-1 px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 text-sm rounded-lg transition cursor-pointer">
                After
            </button>
        </div>
    </div>
    
    <!-- Action Buttons -->
    <div class="flex justify-end gap-3">
        <button hx-get="/web/file-tree"
                hx-target="#sidebar"
                class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 rounded-lg transition cursor-pointer">
            Cancel
        </button>
        <button hx-post="/api/project/insert/function"
                hx-include="#target-file, #function-code, #insertion_point, #reference-function"
                class="px-6 py-2 bg-blue-600 hover:bg-blue-500 text-white font-medium rounded-lg transition cursor-pointer">
            Insert & Submit
        </button>
    </div>
</div>
```

---

## 8. Phase-Specific Views

### 8.1 Planning Phase

```html
<!-- internal/api/templates/phases/planning.html -->
<div class="space-y-6">
    <div class="flex items-center justify-between">
        <h2 class="text-lg font-semibold text-blue-400">Planning Phase</h2>
        <span class="px-3 py-1 bg-blue-500/20 text-blue-400 rounded-full text-xs font-mono">
            AGENT: PLANNER
        </span>
    </div>
    
    <div class="bg-slate-950 border border-blue-900/50 p-8 rounded-xl text-center">
        <div class="inline-block animate-spin rounded-full h-8 w-8 border-4 border-blue-500 border-t-transparent mb-4"></div>
        <h3 class="text-lg font-bold text-blue-400">Analyzing Your Project</h3>
        <p class="text-slate-400 text-sm mt-2">
            The planner agent is analyzing your specs and generating an implementation plan...
        </p>
    </div>
</div>
```

### 8.2 Planning Review (User Approval)

```html
<!-- internal/api/templates/phases/planning-review.html -->
<div class="space-y-6" hx-ext="json-enc">
    <div class="flex items-center justify-between">
        <h2 class="text-lg font-semibold text-amber-400">Review Implementation Plan</h2>
        <span class="px-3 py-1 bg-amber-500/20 text-amber-400 rounded-full text-xs font-mono font-semibold">
            ACTION REQUIRED
        </span>
    </div>
    
    <!-- Architecture Overview -->
    <div class="bg-slate-900 border border-slate-800 rounded-lg p-6">
        <h3 class="text-md font-semibold text-white mb-3">Architecture</h3>
        <div class="prose prose-invert prose-sm max-w-none">
            {{ .Plan.Architecture }}
        </div>
    </div>
    
    <!-- Atomic Units List -->
    <div class="bg-slate-900 border border-slate-800 rounded-lg p-6">
        <h3 class="text-md font-semibold text-white mb-3">
            Implementation Plan ({{ len .Plan.AtomicUnits }} units)
        </h3>
        <div class="space-y-2">
            {{ range $index, $unit := .Plan.AtomicUnits }}
                <div class="flex items-start gap-3 p-3 rounded-lg bg-slate-950 border border-slate-800">
                    <span class="flex-shrink-0 w-6 h-6 rounded-full bg-blue-500/20 text-blue-400 
                        flex items-center justify-center text-xs font-mono">
                        {{ add $index 1 }}
                    </span>
                    <div class="flex-1 min-w-0">
                        <div class="flex items-center gap-2">
                            <span class="text-xs font-semibold px-2 py-0.5 rounded 
                                {{ if eq $unit.Type "function" }}bg-purple-500/20 text-purple-400{{ else }}{{ if eq $unit.Type "struct" }}bg-green-500/20 text-green-400{{ else }}bg-orange-500/20 text-orange-400{{ end }}
                            ">
                                {{ $unit.Type }}
                            </span>
                            <span class="text-sm font-medium text-slate-200">{{ $unit.Name }}</span>
                        </div>
                        <p class="text-xs text-slate-400 mt-1">{{ $unit.Description }}</p>
                        <div class="flex gap-3 mt-1">
                            <span class="text-xs text-slate-500 font-mono">📄 {{ $unit.TargetFile }}</span>
                            <span class="text-xs text-slate-500">📐 {{ $unit.EstimatedSize }}</span>
                        </div>
                    </div>
                </div>
            {{ end }}
        </div>
    </div>
    
    <!-- Action Buttons -->
    <div class="flex justify-end gap-3">
        <button hx-post="/api/project/approve"
                hx-vals='{"approved": false, "feedback": "Please regenerate with different approach"}'
                hx-include="#feedback-input"
                class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 rounded-lg transition cursor-pointer">
            Request Changes
        </button>
        <button hx-post="/api/project/approve"
                hx-vals='{"approved": true}'
                class="px-6 py-2 bg-emerald-600 hover:bg-emerald-500 text-white font-semibold rounded-lg transition cursor-pointer">
            ✅ Approve & Start
        </button>
    </div>
    
    <!-- Optional Feedback -->
    <textarea id="feedback-input"
              name="feedback"
              placeholder="Feedback for regeneration (optional)..."
              class="w-full h-20 bg-slate-900 border border-slate-800 rounded-lg p-3 text-sm text-slate-300 
                     resize-none focus:outline-none focus:border-amber-500"></textarea>
</div>
```

### 8.3 Human Review (IDE)

```html
<!-- internal/api/templates/phases/human-review.html -->
<div class="space-y-6" hx-ext="json-enc">
    <!-- Warning Banner -->
    <div class="bg-amber-950/20 border border-amber-800/30 rounded-lg p-4 flex items-center gap-3">
        <span class="text-2xl">⚠️</span>
        <div>
            <h4 class="text-sm font-semibold text-amber-400">Final Review Required</h4>
            <p class="text-xs text-amber-300/70">
                Please review the {{ .CodeOutput.UnitType }} before accepting.
                Tests and automated review have passed.
            </p>
        </div>
    </div>
    
    <!-- Code Display -->
    <div class="bg-slate-900 rounded-lg border border-slate-800 overflow-hidden">
        <div class="flex items-center justify-between px-4 py-2 border-b border-slate-800">
            <span class="text-sm text-slate-400 font-mono">{{ .CodeOutput.TargetFile }}</span>
            <span class="text-xs text-slate-500">{{ .CodeOutput.UnitName }}()</span>
        </div>
        <pre class="p-4 text-sm text-slate-300 font-mono overflow-x-auto bg-slate-950 max-h-96 overflow-y-auto">
{{ .CodeOutput.Code }}</pre>
    </div>
    
    <!-- Review Summary -->
    <div class="grid grid-cols-2 gap-4">
        <div class="bg-slate-900 rounded-lg border border-slate-800 p-4">
            <h4 class="text-sm font-semibold text-white mb-2">Automated Review</h4>
            <div class="flex items-center gap-2 mb-2">
                <span class="text-lg font-bold text-emerald-400">{{ .ReviewReport.Score }}/10</span>
                <span class="text-xs text-slate-400">Score</span>
            </div>
            {{ range .ReviewReport.Comments }}
                <p class="text-xs text-slate-400 mb-1">• {{ . }}</p>
            {{ end }}
        </div>
        <div class="bg-slate-900 rounded-lg border border-slate-800 p-4">
            <h4 class="text-sm font-semibold text-white mb-2">Test Summary</h4>
            <p class="text-xs text-slate-400">
                {{ len (where .TestReport.Tests "Passed" true) }}/{{ len .TestReport.Tests }} tests passed
            </p>
            <p class="text-xs text-slate-400">Coverage: {{ printf "%.1f" .TestReport.Coverage }}%</p>
        </div>
    </div>
    
    <!-- Action Buttons -->
    <div class="flex justify-end gap-3">
        <button hx-post="/api/project/approve"
                hx-vals='{"approved": false}'
                class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 rounded-lg transition cursor-pointer">
            Reject (loop to coding)
        </button>
        <button hx-post="/api/project/approve"
                hx-vals='{"approved": true}'
                class="px-6 py-2 bg-emerald-600 hover:bg-emerald-500 text-white font-semibold rounded-lg transition cursor-pointer">
            ✓ Accept & Commit
        </button>
    </div>
</div>
```

---

## 9. API Endpoints (HTMX-Optimized)

```go
// No authentication - local environment

// Project Management
GET  /api/project                   → Get project info (JSON)
POST /api/project                   → Initialize project (JSON)
GET  /api/project/files             → List files (JSON)
GET  /api/project/file              → Get file content (JSON)
POST /api/project/approve           → Approve/reject plan or unit (JSON)

// Code Editing
POST /api/project/edit/function     → Edit single function (JSON)
POST /api/project/edit/file         → Edit full file (JSON)
POST /api/project/standalone-function/save → Save standalone function (JSON)
POST /api/project/insert/function   → Insert function into file (JSON)

// UI Rendering (HTMX)
GET  /web/ide                       → Main IDE page (HTML)
GET  /web/phase/:phase              → Phase content partial (HTML)
GET  /web/file-tree                 → File tree partial (HTML)
GET  /web/standalone-function       → Standalone function writer (HTML)
GET  /web/insert-function           → Insertion picker (HTML)
GET  /web/session/:id/activity      → Activity log partial (HTML)

// Configuration
GET  /api/config/models             → Get model config (JSON)
PUT  /api/config/models/:phase      → Update model config (JSON)

// Skills Management
GET    /api/config/skills             → List all skills (JSON)
POST   /api/config/skills             → Create new skill (JSON)
GET    /api/config/skills/:id         → Get single skill (JSON)
PUT    /api/config/skills/:id         → Update skill (JSON)
DELETE /api/config/skills/:id         → Delete skill (JSON)
GET    /api/config/skills/agents      → List agent-skill mappings (JSON)
PUT    /api/config/skills/agents      → Update agent-skill mappings (JSON)
POST   /api/config/skills/reset       → Reset skills to defaults (JSON)
POST   /api/config/skills/export      → Export skills config (JSON)
POST   /api/config/skills/import      → Import skills config (JSON)

// UI Rendering (HTMX)
GET  /web/ide                       → Main IDE page (HTML)
GET  /web/phase/:phase              → Phase content partial (HTML)
GET  /web/file-tree                 → File tree partial (HTML)
GET  /web/standalone-function       → Standalone function writer (HTML)
GET  /web/insert-function           → Insertion picker (HTML)
GET  /web/session/:id/activity      → Activity log partial (HTML)
GET  /web/config/skills             → Skills management page (HTML)
GET  /web/config/skills/new         → New skill form (HTML partial)
GET  /web/config/skills/:id/edit    → Edit skill form (HTML partial)
```

---

## 10. Styling Theme

```css
/* Already using TailwindCSS - dark theme by default */
/* Custom utilities can be added in base.html */
```

---

## 11. Skills Management UI

### 11.1 Overview

A dedicated section in the IDE for managing skills — adding, editing, removing, and associating them with agents. Accessible via the **⚙️ Config** button in the header.

### 11.2 Skills Management Layout

```
┌──────────────────────────────────────────────────────────────────────────────┐
│  CONFIGURATION PANEL                                                          │
│  ┌──────────────────────────────────────────────────────────────────────────┐│
│  │  🐋 Mini-Orca  │  🏠 Sessions  │  📝 Functions  │  ⚙️ Skills  │  🔧 Models ││
│  └──────────────────────────────────────────────────────────────────────────┘│
├──────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  ┌──────────────────────────────────────────────────────────────────────────┐│
│  │  SKILLS LIBRARY                                                          ││
│  │                                                                          ││
│  │  ┌──────────────────────────────────────────────────────────────────────┐││
│  │  │  🔍 Search skills...                           [+ Add New Skill]     │││
│  │  └──────────────────────────────────────────────────────────────────────┘││
│  │                                                                          ││
│  │  ┌──────────────────────────────────────────────────────────────────────┐││
│  │  │  📚 Knowledge Skills                                                 │││
│  │  │                                                                      │││
│  │  │  ┌────────────────────────────────────────────────────────────────┐ │││
│  │  │  │  🏛️  SOLID Principles              [✓] [✏️] [🗑️]              │ │││
│  │  │  │     Follow SOLID principles in all code...                     │ │││
│  │  │  └────────────────────────────────────────────────────────────────┘ │││
│  │  │  ┌────────────────────────────────────────────────────────────────┐ │││
│  │  │  │  🧹 Clean Code                   [✓] [✏️] [🗑️]              │ │││
│  │  │  │     Write clean, readable, maintainable code...                │ │││
│  │  │  └────────────────────────────────────────────────────────────────┘ │││
│  │  │  ┌────────────────────────────────────────────────────────────────┐ │││
│  │  │  │  🤏 KISS Principle             [✓] [✏️] [🗑️]              │ │││
│  │  │  │     Keep It Simple, Stupid...                                  │ │││
│  │  │  └────────────────────────────────────────────────────────────────┘ │││
│  │  │  ┌────────────────────────────────────────────────────────────────┐ │││
│  │  │  │  🚫 No Repetition (DRY)        [✓] [✏️] [🗑️]              │ │││
│  │  │  │     Avoid code duplication...                                  │ │││
│  │  │  └────────────────────────────────────────────────────────────────┘ │││
│  │  │  ┌────────────────────────────────────────────────────────────────┐ │││
│  │  │  │  📖 Business Logic Adherence   [✓] [✏️] [🗑️]              │ │││
│  │  │  │     Always follow business_logic.md...                         │ │││
│  │  │  └────────────────────────────────────────────────────────────────┘ │││
│  │  └──────────────────────────────────────────────────────────────────────┘││
│  │                                                                          ││
│  │  ┌──────────────────────────────────────────────────────────────────────┐││
│  │  │  🔧 Tool Skills                                                      │││
│  │  │                                                                      │││
│  │  │  ┌────────────────────────────────────────────────────────────────┐ │││
│  │  │  │  ⌨️  Shell Execute               [✓] [✏️] [🗑️]              │ │││
│  │  │  │     Execute shell commands                                     │ │││
│  │  │  └────────────────────────────────────────────────────────────────┘ │││
│  │  │  ┌────────────────────────────────────────────────────────────────┐ │││
│  │  │  │  📄 File Read                    [✓] [✏️] [🗑️]              │ │││
│  │  │  │     Read file contents                                         │ │││
│  │  │  └────────────────────────────────────────────────────────────────┘ │││
│  │  │  ┌────────────────────────────────────────────────────────────────┐ │││
│  │  │  │  ✍️  Atomic File Write         [✓] [✏️] [🗑️]              │ │││
│  │  │  │     Write ONE function/struct/class atomically                 │ │││
│  │  │  └────────────────────────────────────────────────────────────────┘ │││
│  │  │  ┌────────────────────────────────────────────────────────────────┐ │││
│  │  │  │  🧪 Run Tests                    [✓] [✏️] [🗑️]              │ │││
│  │  │  │     Execute project test suite                                 │ │││
│  │  │  └────────────────────────────────────────────────────────────────┘ │││
│  │  └──────────────────────────────────────────────────────────────────────┘││
│  └──────────────────────────────────────────────────────────────────────────┘│
│                                                                              │
│  ┌──────────────────────────────────────────────────────────────────────────┐│
│  │  AGENT SKILL ASSIGNMENTS                                                 ││
│  │                                                                          ││
│  │  ┌──────────────────────────────────────────────────────────────────────┐││
│  │  │  🤖 Planner Agent                                                    │││
│  │  │  ┌────────────────────────────────────────────────────────────────┐ │││
│  │  │  │  [✓] 🏛️  SOLID Principles         [✓] 🔧 Shell Execute       │ │││
│  │  │  │  [✓] 🧹 Clean Code                [✓] 🔧 File Read           │ │││
│  │  │  │  [✓] 📖 Business Logic Adherence  [✓] 📐 Architecture Design │ │││
│  │  │  │  [✓] 📐 Task Breakdown            [✓] 📐 Dependency Mapping  │ │││
│  │  │  └────────────────────────────────────────────────────────────────┘ │││
│  │  └──────────────────────────────────────────────────────────────────────┘││
│  │                                                                          ││
│  │  ┌──────────────────────────────────────────────────────────────────────┐││
│  │  │  🤖 Coder Agent                                                      │││
│  │  │  ┌────────────────────────────────────────────────────────────────┐ │││
│  │  │  │  [✓] 🏛️  SOLID Principles         [✓] 🔧 File Read           │ │││
│  │  │  │  [✓] 🧹 Clean Code                [✓] 🔧 Atomic File Write   │ │││
│  │  │  │  [✓] 🤏 KISS Principle            [✓] 🔧 Format Code         │ │││
│  │  │  │  [✓] 🚫 No Repetition (DRY)       [✓] 📝 Function Generation │ │││
│  │  │  │  [✓] 📐 Struct Design             [✓] 📐 Class Creation      │ │││
│  │  │  └────────────────────────────────────────────────────────────────┘ │││
│  │  └──────────────────────────────────────────────────────────────────────┘││
│  │                                                                          ││
│  │  ┌──────────────────────────────────────────────────────────────────────┐││
│  │  │  🤖 Tester Agent                                                     │││
│  │  │  ┌────────────────────────────────────────────────────────────────┐ │││
│  │  │  │  [✓] 🧹 Clean Code                [✓] 🔧 Run Tests           │ │││
│  │  │  │  [✓] 🔧 File Read                 [✓] 🔧 Shell Execute       │ │││
│  │  │  │  [✓] 🧪 Unit Testing              [✓] 🧪 Integration Testing │ │││
│  │  │  │  [✓] 📊 Coverage Analysis         [✓] 🧪 Test Generation     │ │││
│  │  │  └────────────────────────────────────────────────────────────────┘ │││
│  │  └──────────────────────────────────────────────────────────────────────┘││
│  │                                                                          ││
│  │  ┌──────────────────────────────────────────────────────────────────────┐││
│  │  │  🤖 Reviewer Agent                                                   │││
│  │  │  ┌────────────────────────────────────────────────────────────────┐ │││
│  │  │  │  [✓] 🏛️  SOLID Principles         [✓] 🔧 File Read           │ │││
│  │  │  │  [✓] 🧹 Clean Code                [✓] 📋 Style Check         │ │││
│  │  │  │  [✓] 📖 Business Logic Adherence  [✓] 🔍 Logic Review        │ │││
│  │  │  │  [✓] 🔒 Security Audit            [✓] 🔧 Shell Execute       │ │││
│  │  │  └────────────────────────────────────────────────────────────────┘ │││
│  │  └──────────────────────────────────────────────────────────────────────┘││
│  └──────────────────────────────────────────────────────────────────────────┘│
│                                                                              │
│  ┌──────────────────────────────────────────────────────────────────────────┐│
│  │  [💾 Save All]  [↩️ Reset to Defaults]  [📥 Export Config]  [📤 Import] ││
│  └──────────────────────────────────────────────────────────────────────────┘│
└──────────────────────────────────────────────────────────────────────────────┘
```

### 11.3 Add New Skill Dialog

```html
<!-- internal/api/templates/config/skill-editor.html -->
<div class="space-y-4">
    <h2 class="text-lg font-semibold text-white">{{ .IsNew ? "Add New Skill" : "Edit Skill" }}</h2>
    
    <!-- Skill Name -->
    <div class="bg-slate-900 border border-slate-800 rounded-lg p-4">
        <label class="text-sm font-medium text-slate-300 block mb-2">Skill Name</label>
        <input id="skill-name"
               type="text"
               value="{{ .Skill.Name }}"
               placeholder="e.g., DRY Principles"
               class="w-full bg-slate-950 border border-slate-800 rounded-lg p-3 text-sm 
                      text-slate-300 focus:outline-none focus:border-blue-500">
    </div>
    
    <!-- Skill Description -->
    <div class="bg-slate-900 border border-slate-800 rounded-lg p-4">
        <label class="text-sm font-medium text-slate-300 block mb-2">Description</label>
        <input id="skill-description"
               type="text"
               value="{{ .Skill.Description }}"
               placeholder="Brief description of what this skill does"
               class="w-full bg-slate-950 border border-slate-800 rounded-lg p-3 text-sm 
                      text-slate-300 focus:outline-none focus:border-blue-500">
    </div>
    
    <!-- Skill Type -->
    <div class="bg-slate-900 border border-slate-800 rounded-lg p-4">
        <label class="text-sm font-medium text-slate-300 block mb-2">Skill Type</label>
        <div class="flex gap-3">
            <label class="flex items-center gap-2 px-4 py-2 rounded-lg bg-slate-950 border cursor-pointer
                        {{ if eq .Skill.Type "knowledge" }}border-blue-500 bg-blue-950/20{{ else }}border-slate-800 hover:border-slate-700"">
                <input type="radio" name="skill-type" value="knowledge" 
                       {{ if eq .Skill.Type "knowledge" }}checked{{ end }}
                       class="text-blue-500">
                <span class="text-sm text-slate-300">📚 Knowledge (guidelines)</span>
            </label>
            <label class="flex items-center gap-2 px-4 py-2 rounded-lg bg-slate-950 border cursor-pointer
                        {{ if eq .Skill.Type "tool" }}border-blue-500 bg-blue-950/20{{ else }}border-slate-800 hover:border-slate-700"">
                <input type="radio" name="skill-type" value="tool" 
                       {{ if eq .Skill.Type "tool" }}checked{{ end }}
                       class="text-blue-500">
                <span class="text-sm text-slate-300">🔧 Tool (executable)</span>
            </label>
        </div>
    </div>
    
    <!-- Priority -->
    <div class="bg-slate-900 border border-slate-800 rounded-lg p-4">
        <label class="text-sm font-medium text-slate-300 block mb-2">Priority (0 = highest)</label>
        <input id="skill-priority"
               type="number"
               value="{{ .Skill.Priority }}"
               min="0"
               max="100"
               class="w-full bg-slate-950 border border-slate-800 rounded-lg p-3 text-sm 
                      text-slate-300 focus:outline-none focus:border-blue-500">
    </div>
    
    <!-- Prompt Template (Knowledge skills only) -->
    <div id="knowledge-fields" class="bg-slate-900 border border-slate-800 rounded-lg p-4
                {{ if eq .Skill.Type "tool" }}hidden{{ end }}">
        <label class="text-sm font-medium text-slate-300 block mb-2">Prompt Template</label>
        <textarea id="skill-prompt"
                  name="prompt_template"
                  placeholder="Instructions that will be included in the agent's prompt..."
                  class="w-full h-40 bg-slate-950 border border-slate-800 rounded-lg p-4 
                         text-sm text-slate-300 font-mono resize-none focus:outline-none 
                         focus:border-blue-500" spellcheck="false">{{ .Skill.PromptTemplate }}</textarea>
        <p class="text-xs text-slate-500 mt-2">This template will be inserted into the agent's system prompt.</p>
    </div>
    
    <!-- Tool Name (Tool skills only) -->
    <div id="tool-fields" class="bg-slate-900 border border-slate-800 rounded-lg p-4
                {{ if eq .Skill.Type "knowledge" }}hidden{{ end }}">
        <label class="text-sm font-medium text-slate-300 block mb-2">Tool Name</label>
        <input id="skill-tool-name"
               type="text"
               value="{{ .Skill.ToolName }}"
               placeholder="e.g., shell, file_read, test_runner"
               class="w-full bg-slate-950 border border-slate-800 rounded-lg p-3 text-sm 
                      text-slate-300 focus:outline-none focus:border-blue-500">
        <p class="text-xs text-slate-500 mt-2">The tool identifier that the executor will recognize.</p>
    </div>
    
    <!-- Associated Agents -->
    <div class="bg-slate-900 border border-slate-800 rounded-lg p-4">
        <label class="text-sm font-medium text-slate-300 block mb-2">Associate with Agents</label>
        <div class="space-y-2">
            {{ range .Agents }}
                <label class="flex items-center gap-3 p-3 rounded-lg bg-slate-950 border border-slate-800 cursor-pointer">
                    <input type="checkbox" name="agent-{{ .Name }}" 
                           value="{{ .Name }}"
                           {{ if in .Skill.AssociatedAgents .Name }}checked{{ end }}
                           class="text-blue-500">
                    <span class="text-sm text-slate-300">🤖 {{ .Name }}</span>
                </label>
            {{ end }}
        </div>
    </div>
    
    <!-- Action Buttons -->
    <div class="flex justify-end gap-3">
        <button hx-get="/web/config/skills"
                hx-target="#main-content"
                class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 rounded-lg transition cursor-pointer">
            Cancel
        </button>
        <button hx-post="/api/config/skills"
                hx-include="#skill-name, #skill-description, #skill-type, #skill-priority, #skill-prompt, #skill-tool-name"
                class="px-6 py-2 bg-blue-600 hover:bg-blue-500 text-white font-medium rounded-lg transition cursor-pointer">
            {{ if .IsNew }}Add Skill{{ else }}Save Changes{{ end }}
        </button>
    </div>
</div>
```

### 11.4 Skills Configuration API Endpoints

```go
// Skills Management Endpoints

// Skill list (for JSON API)
GET    /api/config/skills             → List all skills (JSON)
POST   /api/config/skills             → Create new skill (JSON)
GET    /api/config/skills/:id         → Get single skill (JSON)
PUT    /api/config/skills/:id         → Update skill (JSON)
DELETE /api/config/skills/:id         → Delete skill (JSON)

// Agent-Skill Associations
GET    /api/config/skills/agents      → List all agent-skill mappings (JSON)
PUT    /api/config/skills/agents      → Update agent-skill mappings (JSON)

// UI Rendering (HTMX)
GET    /web/config/skills             → Skills management page (HTML)
GET    /web/config/skills/new         → New skill form (HTML partial)
GET    /web/config/skills/:id/edit    → Edit skill form (HTML partial)

// Bulk Actions
POST   /api/config/skills/reset       → Reset skills to defaults (JSON)
POST   /api/config/skills/export      → Export skills config (JSON)
POST   /api/config/skills/import      → Import skills config (JSON)
```

### 11.5 Skills Management JavaScript (Alpine.js)

```html
<!-- internal/api/templates/config/skills.html -->
<div x-data="skillsManager()" class="space-y-6">
    <!-- Search and Add -->
    <div class="flex items-center gap-3">
        <input type="text" 
               x-model="searchQuery"
               placeholder="🔍 Search skills..."
               class="flex-1 bg-slate-900 border border-slate-800 rounded-lg p-3 text-sm 
                      text-slate-300 focus:outline-none focus:border-blue-500">
        <button @click="showAddDialog = true"
                class="px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white text-sm font-medium rounded-lg transition cursor-pointer">
            + Add New Skill
        </button>
    </div>
    
    <!-- Knowledge Skills List -->
    <div class="bg-slate-900 border border-slate-800 rounded-lg p-6">
        <h3 class="text-md font-semibold text-white mb-4">📚 Knowledge Skills</h3>
        <div class="space-y-2">
            <template x-for="skill in filteredKnowledgeSkills" :key="skill.id">
                <div class="flex items-center justify-between p-3 rounded-lg bg-slate-950 border border-slate-800">
                    <div>
                        <div class="flex items-center gap-2">
                            <span class="text-sm font-medium text-slate-200" x-text="skill.name"></span>
                            <span class="text-xs px-2 py-0.5 rounded bg-blue-500/20 text-blue-400">knowledge</span>
                        </div>
                        <p class="text-xs text-slate-400 mt-1" x-text="skill.description"></p>
                    </div>
                    <div class="flex items-center gap-2">
                        <input type="checkbox" 
                               x-model="skill.enabled"
                               @change="updateSkill(skill)"
                               class="text-blue-500">
                        <button @click="editSkill(skill)"
                                class="p-1.5 rounded hover:bg-slate-800 text-slate-400 hover:text-blue-400 transition cursor-pointer">
                            ✏️
                        </button>
                        <button @click="deleteSkill(skill.id)"
                                class="p-1.5 rounded hover:bg-slate-800 text-slate-400 hover:text-red-400 transition cursor-pointer">
                            🗑️
                        </button>
                    </div>
                </div>
            </template>
        </div>
    </div>
    
    <!-- Tool Skills List -->
    <div class="bg-slate-900 border border-slate-800 rounded-lg p-6">
        <h3 class="text-md font-semibold text-white mb-4">🔧 Tool Skills</h3>
        <div class="space-y-2">
            <template x-for="skill in filteredToolSkills" :key="skill.id">
                <div class="flex items-center justify-between p-3 rounded-lg bg-slate-950 border border-slate-800">
                    <div>
                        <div class="flex items-center gap-2">
                            <span class="text-sm font-medium text-slate-200" x-text="skill.name"></span>
                            <span class="text-xs px-2 py-0.5 rounded bg-emerald-500/20 text-emerald-400">tool</span>
                        </div>
                        <p class="text-xs text-slate-400 mt-1" x-text="skill.description"></p>
                    </div>
                    <div class="flex items-center gap-2">
                        <input type="checkbox" 
                               x-model="skill.enabled"
                               @change="updateSkill(skill)"
                               class="text-blue-500">
                        <button @click="editSkill(skill)"
                                class="p-1.5 rounded hover:bg-slate-800 text-slate-400 hover:text-blue-400 transition cursor-pointer">
                            ✏️
                        </button>
                        <button @click="deleteSkill(skill.id)"
                                class="p-1.5 rounded hover:bg-slate-800 text-slate-400 hover:text-red-400 transition cursor-pointer">
                            🗑️
                        </button>
                    </div>
                </div>
            </template>
        </div>
    </div>
    
    <!-- Agent Skill Assignments -->
    <div class="bg-slate-900 border border-slate-800 rounded-lg p-6">
        <h3 class="text-md font-semibold text-white mb-4">🤖 Agent Skill Assignments</h3>
        <template x-for="agent in agents" :key="agent.name">
            <div class="mb-4 last:mb-0">
                <h4 class="text-sm font-semibold text-blue-400 mb-2">🤖 <span x-text="agent.name"></span></h4>
                <div class="grid grid-cols-2 gap-2">
                    <template x-for="skill in agent.skills" :key="skill.id">
                        <label class="flex items-center gap-2 p-2 rounded-lg bg-slate-950 border border-slate-800 cursor-pointer">
                            <input type="checkbox" 
                                   :value="skill.id"
                                   :name="'agent-' + agent.name"
                                   x-model="agent.skillIds"
                                   @change="updateAgentSkills(agent)"
                                   class="text-blue-500">
                            <span class="text-xs text-slate-300" x-text="skill.name"></span>
                        </label>
                    </template>
                </div>
            </div>
        </template>
    </div>
    
    <!-- Bulk Actions -->
    <div class="flex justify-end gap-3">
        <button @click="resetDefaults()"
                class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 rounded-lg transition cursor-pointer">
            ↩️ Reset to Defaults
        </button>
        <button @click="exportConfig()"
                class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 rounded-lg transition cursor-pointer">
            📥 Export
        </button>
        <button @click="importConfig()"
                class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 rounded-lg transition cursor-pointer">
            📤 Import
        </button>
        <button @click="saveAll()"
                class="px-6 py-2 bg-emerald-600 hover:bg-emerald-500 text-white font-semibold rounded-lg transition cursor-pointer">
            💾 Save All
        </button>
    </div>
    
    <!-- Add/Edit Skill Dialog -->
    <div x-show="showAddDialog || editingSkill"
         x-transition
         class="fixed inset-0 z-50 bg-black/50 flex items-center justify-center"
         @click.self="closeDialog()">
        <div class="bg-slate-900 border border-slate-800 rounded-xl p-6 w-full max-w-2xl max-h-[90vh] overflow-y-auto">
            <template x-if="editingSkill">
                <div x-html="editSkillHtml"></div>
            </template>
            <template x-if="showAddDialog && !editingSkill">
                <div x-html="addSkillHtml"></div>
            </template>
        </div>
    </div>
</div>

<script>
function skillsManager() {
    return {
        searchQuery: '',
        skills: [],
        agents: [],
        showAddDialog: false,
        editingSkill: null,
        editSkillHtml: '',
        
        get filteredKnowledgeSkills() {
            return this.skills
                .filter(s => s.type === 'knowledge')
                .filter(s => !this.searchQuery || s.name.toLowerCase().includes(this.searchQuery.toLowerCase()))
                .sort((a, b) => a.priority - b.priority);
        },
        
        get filteredToolSkills() {
            return this.skills
                .filter(s => s.type === 'tool')
                .filter(s => !this.searchQuery || s.name.toLowerCase().includes(this.searchQuery.toLowerCase()))
                .sort((a, b) => a.priority - b.priority);
        },
        
        async init() {
            await this.loadSkills();
            await this.loadAgents();
        },
        
        async loadSkills() {
            const resp = await fetch('/api/config/skills');
            this.skills = await resp.json();
        },
        
        async loadAgents() {
            const resp = await fetch('/api/config/skills/agents');
            this.agents = await resp.json();
        },
        
        async updateSkill(skill) {
            await fetch(`/api/config/skills/${skill.id}`, {
                method: 'PUT',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify(skill)
            });
        },
        
        async deleteSkill(id) {
            if (confirm('Delete this skill?')) {
                await fetch(`/api/config/skills/${id}`, {method: 'DELETE'});
                await this.loadSkills();
            }
        },
        
        editSkill(skill) {
            this.editingSkill = skill;
            // Fetch the edit form HTML
            fetch(`/web/config/skills/${skill.id}/edit`)
                .then(r => r.text())
                .then(html => { this.editSkillHtml = html; });
        },
        
        closeDialog() {
            this.showAddDialog = false;
            this.editingSkill = null;
            this.editSkillHtml = '';
        },
        
        async saveAll() {
            await Promise.all(this.skills.map(s => this.updateSkill(s)));
            await Promise.all(this.agents.map(a => this.updateAgentSkills(a)));
            alert('All changes saved!');
        },
        
        async resetDefaults() {
            if (confirm('Reset all skills to defaults?')) {
                await fetch('/api/config/skills/reset', {method: 'POST'});
                await this.loadSkills();
                await this.loadAgents();
            }
        },
        
        async exportConfig() {
            const resp = await fetch('/api/config/skills/export');
            const blob = await resp.blob();
            const url = URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = url;
            a.download = 'mini-orca-skills.json';
            a.click();
        },
        
        async importConfig() {
            const input = document.createElement('input');
            input.type = 'file';
            input.accept = '.json';
            input.onchange = async (e) => {
                const file = e.target.files[0];
                const formData = new FormData();
                formData.append('file', file);
                await fetch('/api/config/skills/import', {
                    method: 'POST',
                    body: formData
                });
                await this.loadSkills();
                await this.loadAgents();
            };
            input.click();
        }
    }
}
</script>
```

---

## 12. Responsive Design

The HTMX dashboard is responsive by design using TailwindCSS:

- **Desktop:** Full 3-column layout (file tree + editor + activity log)
- **Tablet:** Sidebar collapses, main content expands
- **Mobile:** Single column, hamburger menu for sidebar

```html
<!-- Example responsive sidebar -->
<aside class="hidden lg:block w-80 bg-slate-950 border-r border-slate-800 p-4">
    <!-- Desktop sidebar -->
</aside>

<aside class="lg:hidden fixed inset-0 z-50 bg-slate-950 transform 
              transition-transform duration-300"
       x-show="sidebarOpen"
       x-transition>
    <!-- Mobile sidebar -->
</aside>
```
