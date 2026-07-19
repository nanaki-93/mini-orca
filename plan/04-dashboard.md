# Mini-Orca v2.0 — HTMX Frontend Dashboard

## 1. Technology Stack

| Technology | Purpose |
|------------|---------|
| **HTMX** | DOM manipulation via HTML attributes (no JS framework) |
| **Alpine.js** | Lightweight reactivity for interactive elements |
| **TailwindCSS** | Utility-first CSS |
| **Go Templates** | Server-side HTML rendering |

**Why HTMX?**
- No build step, no npm, no bundler
- Server-rendered HTML (fast initial load)
- Progressive enhancement
- Simple mental model
- Works with existing Go template system
- Small footprint (~15KB gzipped)

---

## 2. Dashboard Layout

```
┌─────────────────────────────────────────────────────────────────────────────┐
│  HEADER                                                                     │
│  ┌──────────────┐  ┌────────────────────────────────────────────────────┐   │
│  │  🐋 Mini-Orca │  │  Sessions: [3]  │  Status: ● Running  │ ⚙️ Config │   │
│  └──────────────┘  └────────────────────────────────────────────────────┘   │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌─────────────────────────┐  ┌─────────────────────────────────────────┐  │
│  │  SESSION INFO           │  │  MAIN CONTENT AREA                      │  │
│  │                         │  │                                         │  │
│  │  📁 Project: ./my-app   │  │  ┌───────────────────────────────────┐  │  │
│  │  🎯 Goal: Auth module   │  │  │  Phase Tracker (horizontal)       │  │  │
│  │  📊 State: CODING       │  │  │  [✓] [✓] [○] [○] [○]            │  │  │
│  │  ⏱️ Duration: 2m 34s    │  │  │  Plan Code Test Rev Human         │  │  │
│  │  📝 Units: 3/12         │  │  └───────────────────────────────────┘  │  │
│  │                         │  │                                         │  │
│  │  ─────────────────────  │  │  ┌───────────────────────────────────┐  │  │
│  │  [New Session]          │  │  │                                   │  │  │
│  │  [Pause] [Resume]       │  │  │  Dynamic Content (HTMX swap):     │  │  │
│  │  [Stop]                 │  │  │                                   │  │  │
│  │                         │  │  │  - Plan view (Phase A)            │  │  │
│  │  ─────────────────────  │  │  │  - Code editor (Phase B)          │  │  │
│  │  Session History        │  │  │  - Test results (Phase C)         │  │  │
│  │  ┌───────────────────┐  │  │  │  - Review report (Phase D)        │  │  │
│  │  │ Session 1         │  │  │  │  - Human review (Phase E)         │  │  │
│  │  │ Completed • 5m    │  │  │  │                                   │  │  │
│  │  └───────────────────┘  │  │  └───────────────────────────────────┘  │  │
│  │  ┌───────────────────┐  │  │                                         │  │
│  │  │ Session 2         │  │  │  ┌───────────────────────────────────┐  │  │
│  │  │ Running • 2m      │  │  │  │  Activity Log                     │  │  │
│  │  └───────────────────┘  │  │  │  [12:34] Planning started         │  │  │
│  │  ┌───────────────────┐  │  │  │  [12:35] Plan generated           │  │  │
│  │  │ Session 3         │  │  │  │  [12:36] Code review passed       │  │  │
│  │  │ Failed • -        │  │  │  │  [12:37] Tests passed             │  │  │
│  │  └───────────────────┘  │  │  │                                   │  │  │
│  │                         │  │  └───────────────────────────────────┘  │  │
│  └─────────────────────────┘  └─────────────────────────────────────────┘  │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 3. Component Structure (Go Templates)

```
internal/api/templates/
├── base.html              ← Base layout (header, sidebar, main)
├── dashboard.html         ← Main dashboard page
├── components/
│   ├── header.html        ← Header bar
│   ├── sidebar.html       ← Left sidebar (session info + history)
│   ├── phase-tracker.html ← Phase progress indicator
│   └── activity-log.html  ← Activity feed
├── phases/
│   ├── planning.html      ← Planning phase view
│   ├── planning-review.html ← Waiting for plan approval
│   ├── coding.html        ← Coding phase view
│   ├── testing.html       ← Testing phase view
│   ├── review.html        ← Code review view
│   └── human-review.html  ← Human review view
└── sessions/
    ├── list.html          ← Session list
    └── new.html           ← New session form
```

---

## 4. Template Examples

### 4.1 Base Layout

```html
<!-- internal/api/templates/base.html -->
<!DOCTYPE html>
<html lang="en" x-data="{ darkMode: true }">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Mini-Orca — Agent Orchestrator</title>
    
    <!-- TailwindCSS (CDN for dev, build for prod) -->
    <script src="https://cdn.tailwindcss.com"></script>
    
    <!-- HTMX -->
    <script src="https://unpkg.com/htmx.org@2.0.0"></script>
    
    <!-- Alpine.js -->
    <script defer src="https://cdn.jsdelivr.net/npm/alpinejs@3.x.x/dist/cdn.min.js"></script>
    
    <style>
        /* Custom scrollbar */
        ::-webkit-scrollbar { width: 8px; }
        ::-webkit-scrollbar-track { background: #1e293b; }
        ::-webkit-scrollbar-thumb { background: #475569; border-radius: 4px; }
    </style>
</head>
<body class="bg-slate-950 text-slate-200 font-sans antialiased">
    <div class="min-h-screen flex flex-col">
        {{ template "header" . }}
        
        <div class="flex flex-1">
            {{ template "sidebar" . }}
            
            <main class="flex-1 p-6 overflow-auto">
                {{ block "main" . }}{{ end }}
            </main>
        </div>
    </div>
</body>
</html>
```

### 4.2 Dashboard Page

```html
<!-- internal/api/templates/dashboard.html -->
{{ template "base.html" . }}

{{ define "main" }}
<div class="space-y-6">
    <!-- Phase Tracker -->
    <div class="bg-slate-900 border border-slate-800 rounded-lg p-4">
        {{ template "components/phase-tracker" . }}
    </div>
    
    <!-- Dynamic Phase Content -->
    <div id="phase-content"
         hx-get="/web/session/{{.SessionID}}/phase"
         hx-trigger="load, phaseChanged from:body"
         hx-swap="innerHTML">
        <div class="text-center py-12 text-slate-500 animate-pulse">
            Loading phase content...
        </div>
    </div>
    
    <!-- Activity Log -->
    <div class="bg-slate-900 border border-slate-800 rounded-lg p-4">
        {{ template "components/activity-log" . }}
    </div>
</div>
{{ end }}
```

### 4.3 Phase Tracker Component

```html
<!-- internal/api/templates/components/phase-tracker.html -->
<div class="flex items-center gap-2">
    {{ $phases := slice 
        (dict "key" "PLANNING" "label" "Plan")
        (dict "key" "CODING" "label" "Code")
        (dict "key" "TESTING" "label" "Test")
        (dict "key" "CODE_REVIEW" "label" "Review")
        (dict "key" "WAITING_FOR_HUMAN_APPROVAL" "label" "Human")
    }}
    
    {{ $currentPhase := .CurrentState }}
    
    {{ range $phases }}
        {{ $isActive := eq $currentPhase .key }}
        {{ $isCompleted := or 
            (eq $currentPhase "COMPLETED")
            (gt (index $.CompletedPhases .key) 0)
        }}
        
        <div class="flex items-center gap-2">
            <div class="flex items-center justify-center w-8 h-8 rounded-full 
                {{ if $isCompleted }}bg-emerald-500{{ else }}{{ if $isActive }}bg-blue-500 animate-pulse{{ else }}bg-slate-700{{ end }}
            ">
                {{ if $isCompleted }}
                    <svg class="w-4 h-4 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"/>
                    </svg>
                {{ else if $isActive }}
                    <svg class="w-4 h-4 text-white animate-spin" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"/>
                    </svg>
                {{ else }}
                    <div class="w-2 h-2 rounded-full bg-slate-400"></div>
                {{ end }}
            </div>
            <span class="text-xs {{ if $isActive }}text-blue-400 font-semibold{{ else }}text-slate-400{{ end }}">
                {{ .label }}
            </span>
        </div>
        
        {{ if lt (index $phases | len) 4 }}
            <div class="w-4 h-px bg-slate-700"></div>
        {{ end }}
    {{ end }}
</div>
```

### 4.4 Planning Phase View

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

### 4.5 Planning Review View (User Approval)

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
        <button hx-post="/api/sessions/{{.SessionID}}/approve"
                hx-vals='{"approved": false, "feedback": "Please regenerate with different approach"}'
                hx-include="#feedback-input"
                class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 rounded-lg transition cursor-pointer">
            Request Changes
        </button>
        <button hx-post="/api/sessions/{{.SessionID}}/approve"
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

### 4.6 Coding Phase View

```html
<!-- internal/api/templates/phases/coding.html -->
<div class="space-y-4">
    <div class="flex items-center justify-between">
        <div>
            <h2 class="text-lg font-semibold text-white">
                Generating: {{ .CodeOutput.UnitName }}
            </h2>
            <p class="text-sm text-slate-400">
                {{ .CodeOutput.UnitType }} in {{ .CodeOutput.TargetFile }} • {{ .CodeOutput.LinesAdded }} lines
            </p>
        </div>
        <span class="px-3 py-1 bg-blue-500/20 text-blue-400 rounded-full text-xs font-mono">
            PHASE B: CODING
        </span>
    </div>
    
    <!-- Code Display -->
    <div class="rounded-lg overflow-hidden border border-slate-700">
        <div class="flex items-center justify-between px-4 py-2 border-b border-slate-800 bg-slate-900">
            <span class="text-sm text-slate-400 font-mono">{{ .CodeOutput.TargetFile }}</span>
            <span class="text-xs text-slate-500">{{ .CodeOutput.UnitName }}()</span>
        </div>
        <pre class="p-4 text-sm text-slate-300 font-mono overflow-x-auto bg-slate-950 max-h-96 overflow-y-auto">
{{ .CodeOutput.Code }}</pre>
    </div>
    
    <!-- Auto-progress indicator -->
    <div class="flex items-center gap-2 text-sm text-slate-400">
        <div class="inline-block animate-spin rounded-full h-4 w-4 border-2 border-blue-500 border-t-transparent"></div>
        <span>Unit generated. Proceeding to testing...</span>
    </div>
</div>
```

### 4.7 Testing Phase View

```html
<!-- internal/api/templates/phases/testing.html -->
<div class="space-y-4">
    <div class="flex items-center justify-between">
        <h2 class="text-lg font-semibold text-white">Test Results</h2>
        <span class="px-3 py-1 rounded-full text-xs font-semibold
            {{ if .TestReport.Passed }}bg-emerald-500/20 text-emerald-400{{ else }}bg-red-500/20 text-red-400{{ end }}">
            {{ if .TestReport.Passed }}✓ PASSED{{ else }}✗ FAILED{{ end }}
        </span>
    </div>
    
    <!-- Summary Cards -->
    <div class="grid grid-cols-4 gap-4">
        <div class="bg-slate-900 rounded-lg p-4 border border-slate-800">
            <div class="text-2xl font-bold text-white">{{ len .TestReport.Tests }}</div>
            <div class="text-xs text-slate-400">Total Tests</div>
        </div>
        <div class="bg-slate-900 rounded-lg p-4 border border-slate-800">
            <div class="text-2xl font-bold text-emerald-400">
                {{ len (where .TestReport.Tests "Passed" true) }}
            </div>
            <div class="text-xs text-slate-400">Passed</div>
        </div>
        <div class="bg-slate-900 rounded-lg p-4 border border-slate-800">
            <div class="text-2xl font-bold text-red-400">
                {{ len (where .TestReport.Tests "Passed" false) }}
            </div>
            <div class="text-xs text-slate-400">Failed</div>
        </div>
        <div class="bg-slate-900 rounded-lg p-4 border border-slate-800">
            <div class="text-2xl font-bold text-blue-400">{{ printf "%.1f" .TestReport.Coverage }}%</div>
            <div class="text-xs text-slate-400">Coverage</div>
        </div>
    </div>
    
    <!-- Test Details Table -->
    <div class="bg-slate-900 rounded-lg border border-slate-800 overflow-hidden">
        <table class="w-full">
            <thead>
                <tr class="border-b border-slate-800">
                    <th class="text-left p-3 text-xs font-semibold text-slate-400">Test</th>
                    <th class="text-left p-3 text-xs font-semibold text-slate-400">Status</th>
                    <th class="text-left p-3 text-xs font-semibold text-slate-400">Duration</th>
                </tr>
            </thead>
            <tbody>
                {{ range .TestReport.Tests }}
                    <tr class="border-b border-slate-800/50">
                        <td class="p-3 text-sm text-slate-300 font-mono">{{ .Name }}</td>
                        <td class="p-3">
                            <span class="text-xs font-semibold px-2 py-0.5 rounded
                                {{ if .Passed }}bg-emerald-500/20 text-emerald-400{{ else }}bg-red-500/20 text-red-400{{ end }}">
                                {{ if .Passed }}PASS{{ else }}FAIL{{ end }}
                            </span>
                        </td>
                        <td class="p-3 text-xs text-slate-400">{{ .Duration }}</td>
                    </tr>
                {{ end }}
            </tbody>
        </table>
    </div>
    
    <!-- Errors (if any) -->
    {{ if gt (len .TestReport.Errors) 0 }}
        <div class="bg-red-950/20 border border-red-800/30 rounded-lg p-4">
            <h4 class="text-sm font-semibold text-red-400 mb-2">Errors</h4>
            <pre class="text-xs text-red-300 whitespace-pre-wrap font-mono">{{ .TestReport.Errors | join "\n" }}</pre>
        </div>
    {{ end }}
    
    <!-- Suggestions -->
    {{ if gt (len .TestReport.Suggestions) 0 }}
        <div class="bg-blue-950/20 border border-blue-800/30 rounded-lg p-4">
            <h4 class="text-sm font-semibold text-blue-400 mb-2">Suggestions</h4>
            <ul class="space-y-1">
                {{ range .TestReport.Suggestions }}
                    <li class="text-sm text-blue-300">• {{ . }}</li>
                {{ end }}
            </ul>
        </div>
    {{ end }}
</div>
```

### 4.8 Human Review View

```html
<!-- internal/api/templates/phases/human-review.html -->
<div class="space-y-6" hx-ext="json-enc">
    <!-- Warning Banner -->
    <div class="bg-amber-950/20 border border-amber-800/30 rounded-lg p-4 flex items-center gap-3">
        <span class="text-2xl">⚠️</span>
        <div>
            <h4 class="text-sm font-semibold text-amber-400">Final Review Required</h4>
            <p class="text-xs text-amber-300/70">
                Please review the {{ .CodeOutput.UnitType }} before committing.
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
    
    <!-- Feedback -->
    <div>
        <label class="text-sm font-medium text-slate-300 mb-2 block">
            Feedback (optional, for regeneration)
        </label>
        <textarea id="human-feedback"
                  name="feedback"
                  placeholder="Describe any changes you'd like..."
                  class="w-full h-24 bg-slate-900 border border-slate-800 rounded-lg p-3 text-sm text-slate-300 
                         resize-none focus:outline-none focus:border-blue-500"></textarea>
    </div>
    
    <!-- Action Buttons -->
    <div class="flex justify-end gap-3">
        <button hx-post="/api/sessions/{{.SessionID}}/approve"
                hx-vals='{"approved": false}'
                hx-include="#human-feedback"
                class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 rounded-lg transition cursor-pointer">
            Request Changes
        </button>
        <button hx-post="/api/sessions/{{.SessionID}}/approve"
                hx-vals='{"approved": true}'
                class="px-6 py-2 bg-emerald-600 hover:bg-emerald-500 text-white font-semibold rounded-lg transition cursor-pointer">
            ✓ Accept & Commit
        </button>
    </div>
</div>
```

---

## 5. Alpine.js Interactivity

Alpine.js is used for lightweight client-side state without a framework:

```html
<!-- Model Configuration Panel (Alpine.js) -->
<div x-data="{ 
        showConfig: false,
        currentProvider: 'lm-studio',
        models: {{ .AvailableModels }},
        selectedModel: '{{ .CurrentModel }}'
    }">
    <!-- Toggle Button -->
    <button @click="showConfig = !showConfig"
            class="p-2 hover:bg-slate-800 rounded-lg transition">
        ⚙️
    </button>
    
    <!-- Config Panel -->
    <div x-show="showConfig"
         x-transition
         class="absolute right-4 top-12 w-80 bg-slate-900 border border-slate-800 
                rounded-lg shadow-xl p-4 z-50">
        <h3 class="text-sm font-semibold text-white mb-3">Model Configuration</h3>
        
        <label class="text-xs text-slate-400 block mb-1">Provider</label>
        <select x-model="currentProvider" class="w-full bg-slate-800 border border-slate-700 
                 rounded px-3 py-2 text-sm text-slate-300 mb-3">
            <option value="lm-studio">LM Studio</option>
            <!-- Future: <option value="ollama">Ollama</option> -->
        </select>
        
        <label class="text-xs text-slate-400 block mb-1">Model</label>
        <select x-model="selectedModel" class="w-full bg-slate-800 border border-slate-700 
                 rounded px-3 py-2 text-sm text-slate-300">
            <template x-for="model in models" :key="model">
                <option :value="model" x-text="model"></option>
            </template>
        </select>
        
        <button @click="updateModel()" 
                class="w-full mt-3 px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white 
                       text-sm font-medium rounded-lg transition cursor-pointer">
            Apply
        </button>
    </div>
</div>
```

---

## 6. API Endpoints (HTMX-Optimized)

```go
// No authentication - local environment

// Session Management
GET  /api/sessions              → List sessions (JSON)
POST /api/sessions              → Create session (JSON)
GET  /api/sessions/:id          → Get session (JSON)
POST /api/sessions/:id/approve  → Approve/reject (JSON)

// UI Rendering (HTMX)
GET  /dashboard                 → Main dashboard (HTML)
GET  /web/session/:id/phase     → Phase content partial (HTML)
GET  /web/session/:id/status    → Status badge (HTML)
GET  /web/session/:id/activity  → Activity log (HTML)

// Configuration
GET  /api/config/models         → Get model config (JSON)
PUT  /api/config/models/:phase  → Update model config (JSON)
```

---

## 7. Styling Theme

```css
/* Already using TailwindCSS - dark theme by default */
/* Custom utilities can be added in base.html */
```

---

## 8. Responsive Design

The HTMX dashboard is responsive by design using TailwindCSS:

- **Desktop:** Full 3-column layout
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
