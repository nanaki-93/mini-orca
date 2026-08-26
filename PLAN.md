# Mini-Orca focused AI IDE roadmap

Date: 2026-08-26

Status: Approved product direction; implementation backlog pending

Task source: [`tasks/INDEX.md`](tasks/INDEX.md)

## 1. Product outcome

Mini-Orca will be a local-first, Go-first coding assistant organized around one
deliberate unit of work:

> One active project → one open file → one selected or new symbol → one editable
> AI draft → validation → focused checks → explicit Apply.

The source viewer remains read-only. The user may manually edit the AI-generated
declaration before accepting it, but Mini-Orca never becomes a general text
editor and never writes a draft automatically.

## 2. Confirmed product decisions

1. **Four workspaces:** Summary, Analysis, Bugs, and Editor.
2. **Read-only source:** normal source files are selectable and inspectable but
   cannot be typed into directly.
3. **Editable AI draft:** the generated function or type is editable before
   validation and Apply.
4. **Verified and AI findings:** tool-reported issues and model suggestions are
   shown separately and never presented with the same confidence.
5. **Go first:** safe changed candidates are supported for Go before adding
   parser-backed adapters for other languages.
6. **Existing or new symbol:** a draft may replace one exact selected function
   or type, or create one new top-level function or type in the open file.
7. **Preview first:** every changed draft must pass scope validation and required
   checks after its most recent manual edit.
8. **One-file boundary:** chat, draft composition, diff, checks, and Apply stay
   pinned to the file that was open when the session was created.

## 3. Current foundation

The repository already provides most safety primitives needed for this product:

- one active project with project ID, revision, and selected-file hash guards;
- a context policy, context manifest, and loopback-only daemon default;
- deterministic file indexing and exact Go symbol extraction;
- lazy semantic analysis for one file and sequential Analyze-all cache warming;
- one-file generation previews with Go scope validation;
- isolated candidate checks, diff review, explicit Apply, undo, and audit;
- a Compose Desktop file explorer, read-only source view, summary, and review UI.

The work should replace or extend these implementations rather than create a
second workflow, service, database, or orchestration layer.

## 4. Gaps to close

### 4.1 Project workspaces

The desktop currently has file-level Code, Summary, and Changes tabs. It lacks
dedicated project-level navigation and aggregated data for Summary, Analysis,
and Bugs.

The project AI analysis is a single Markdown string. It must become a structured,
versioned report so the desktop can render purpose, architecture, components,
entry points, flows, risks, and next steps without parsing presentation text.

### 4.2 File and symbol brief

File analysis and symbol explanations exist, but they are hidden behind the
Summary tab. The Editor must keep a compact deterministic brief visible beside
the read-only source, then enrich it with cached model interpretation when
available. Selecting a symbol must update its signature, range, explanation,
and advisory impact without leaving the Editor.

### 4.3 Bugs and findings

Current AI risks contain only severity and summary. There is no stable identity,
location, evidence, provenance, revision, freshness, lifecycle state, or project
aggregation. Verified project scans also do not exist as a first-class workflow.

The Bugs workspace needs a unified finding contract with two clear groups:

- **Verified/tool-reported:** parser diagnostics, `go vet`, and `go test` output
  from an explicit isolated scan.
- **AI suggestions:** risks from fresh cached project/file analyses, always
  labeled as model interpretation.

### 4.4 File-scoped chat and editable drafts

The current chat endpoint is a one-shot generation form. Its durable history is
source-free activity, not a conversation. Generation asks the model for a whole
file, only replaces an existing symbol, and stores only applicable candidates.

The target workflow requires a session pinned to project revision, open file,
base hash, and edit mode. The model should return one complete Go declaration
and required imports. The daemon—not the model or UI—composes the complete file.

Manual draft edits must create a new draft revision and immediately invalidate
old validation, checks, comparison, and Apply eligibility.

## 5. Target information architecture

### Summary

- deterministic project type, build metadata, languages, files, and line totals;
- structured architecture report with explicit freshness and model status;
- analysis coverage and verified/AI finding totals;
- links into Analysis and Bugs.

### Analysis

- fresh, stale, missing, failed, and running file-analysis counts;
- explicit Start, Pause, Resume, and Cancel controls for Analyze-all;
- per-file progress with retry limits and open-in-Editor navigation;
- no automatic Analyze-all on import or reindex.

### Bugs

- Verified issues and AI suggestions in separate sections;
- severity, provenance, evidence, file, line/symbol, revision, and freshness;
- source/severity/status filters and lifecycle actions;
- Prepare fix opens the associated file and prefills chat, but does not generate
  or apply automatically.

### Editor

- file explorer on the left;
- read-only, selectable source in the center;
- always-visible file/symbol brief above file-scoped chat on wide layouts;
- responsive drawers below 1000dp without losing the compact brief;
- editable declaration draft, read-only diff, validation, checks, and Apply
  review after generation.

## 6. Core domain contracts

### 6.1 Structured project analysis

The authoritative report is versioned structured data containing:

- project ID and revision;
- purpose and architecture summary;
- components and entry points;
- data/control flows;
- risks and suggested next steps;
- model/profile, prompt version, generated time, and status.

The existing Markdown analysis remains only as a human-readable projection of
the structured report, not a second parsing or storage implementation.

### 6.2 Unified finding

Each finding contains a stable ID, source, confidence, severity, title, message,
optional rule/tool, project revision, optional file hash, project-relative path,
optional line range and symbol, sanitized evidence, status, detected time, and
freshness. Finding IDs must be deterministic enough to preserve triage across
an unchanged rerun.

AI findings are `suggested`; parser, vet, and test results are `tool_reported`.
Neither label claims that a tool or model is infallible.

### 6.3 Go declaration draft

A draft uses one of two modes:

- `replace_symbol`: the exact selected symbol exists once before and after;
- `create_symbol`: the requested symbol is absent before and exists once after.

The draft stores only the declaration and requested imports plus immutable base
identity, edit mode, draft revision/hash, lineage, validation, and check state.
The daemon parses and formats the declaration, composes a full candidate in
memory, and proves every unrelated declaration is unchanged.

### 6.4 File chat session

A chat session is bound to project ID, project revision, open path, base file
hash, edit mode, and selected/new symbol. Messages cannot retarget the session.
Opening another file makes the previous session inactive or stale. Asking for a
revision creates a new draft linked to its predecessor.

## 7. Safety invariants

- No model or tool may mutate the imported project before explicit Apply.
- A project scan is explicit and runs in an isolated copy; tests are never run
  silently during import or file analysis.
- No draft may change more than the open file and one selected/new Go symbol,
  except validated required imports.
- A dirty, invalid, unchecked, stale, or hash-mismatched draft cannot apply.
- Every manual draft edit invalidates all earlier approval evidence.
- Source and diff views remain read-only.
- Apply and undo remain revision-guarded and auditable.
- AI interpretation is visually and structurally distinct from deterministic or
  tool-reported facts.
- Provider credentials, prompt bodies, source, and local configuration are not
  written into findings, activity, logs, or review exports.

## 8. Delivery phases

### Phase A — Restore a trustworthy baseline

Tasks 32–33 fix the hanging test fixtures, make validation green, stop tracking
local configuration, and remove version drift before feature implementation.

### Phase B — Project intelligence and Bugs backend

Tasks 34–37 add structured project analysis, the unified finding store, explicit
isolated Go scans, project overview data, and findings APIs.

### Phase C — Declaration drafts and file-scoped chat backend

Tasks 38–41 implement exact replace/create declaration composition, editable
draft revisions, real diff/check/apply invalidation, and conversation sessions
pinned to the open file.

### Phase D — Desktop application structure and workspaces

Tasks 42–49 decompose the desktop shell, add typed API/state support, introduce
the four workspaces, and implement Summary, Analysis, Bugs, and the always-visible
Editor brief.

### Phase E — Desktop chat and editable review

Tasks 50–52 deliver file-scoped conversation, editable declaration drafts, and
the complete Validate → Checks → Apply review flow.

### Phase F — Accessibility, verification, and release

Tasks 53–56 complete keyboard/responsive behavior, automated desktop integration
coverage, synchronized API/user documentation, and release acceptance.

## 9. Explicit non-goals

- Direct editing or saving in the source viewer.
- Arbitrary cursor/range insertion in the first Go release.
- Multiple open projects or simultaneous project mutations.
- Multi-file AI candidates or repository-wide autonomous refactors.
- Automatic commits, pushes, dependency installation, or background fixes.
- A database, plugin platform, second UI, or replacement framework.
- Parser-backed safe edits for Kotlin, Java, TypeScript, Python, or Rust in this
  milestone; those remain analysis-only until separate adapters are designed.

## 10. Definition of done

The milestone is complete when:

1. Summary, Analysis, Bugs, and Editor are distinct accessible workspaces.
2. Every open file shows a deterministic brief and optional fresh semantic brief.
3. Verified/tool-reported issues cannot be confused with AI suggestions.
4. Chat cannot target a file other than its bound open file.
5. A user can replace one selected Go function/type or create one new function/type.
6. The generated declaration is manually editable before acceptance.
7. Manual edits invalidate old validation and checks.
8. Only the most recently validated and checked draft can be applied.
9. Apply changes exactly one file; undo remains conflict-safe.
10. The complete keyboard workflow works below and above 1000dp.
11. Documentation and live API routes agree.
12. `make check`, desktop integration tests, and the release fixture checklist
    pass on a clean worktree.
