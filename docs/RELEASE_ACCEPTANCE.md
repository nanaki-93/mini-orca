# v4.3 focused AI IDE release acceptance

Run this checklist from a clean release worktree after applying the intended
implementation and documentation changes. The automated commands are recorded
in the v4.3 release notes; the manual checks below are reproducible evidence
for a release operator, not claims that a source file was edited directly.

## Fixture and variants

Use `internal/project/testdata/release-fixture`. Its normal Go module supports
the successful replace/create → Validate → Checks → Apply → Undo flow. It also
contains ignored placeholder secrets, an AI-suggestion fixture, and parser,
vet, and test failure inputs with `.fixture` extensions. Copy the fixture to
temporary directories before use:

1. Leave one copy outside Git to verify the non-Git fallback.
2. In a second copy, run `git init`, configure a local test identity, add the
   files, and create one initial commit to verify Git status behavior.
3. For each verified failure case, copy the relevant `.fixture` input over a
   temporary `.go` or `_test.go` file in a separate copied module, then run the
   explicit Go scan. Do not run scans against the imported source project.

The normal fixture has the `Run` function and `Worker` type for replacement;
`ReleaseNote` is an absent valid declaration name for creation. The `.env`,
`private.pem`, and `config.yaml` placeholders must remain excluded from model
context and findings.

## Automated evidence

| Product requirement | Reproducible evidence |
|---|---|
| Summary facts, structured report, analysis coverage, and source-separated counts | `go test ./internal/app -run ProjectOverview` |
| Explicit verified parser/vet/test scans and cancellation in an isolated copy | `go test ./internal/app -run 'ScanGoProject|GoScanLifecycle'` |
| AI suggestions retain `suggested` confidence; findings retain provenance, triage, freshness, and redaction | `go test ./internal/project -run Finding` |
| Replace/create composition changes only the target declaration and requested imports | `go test ./internal/project -run 'ComposeGoDeclaration|ReleaseFixture'` |
| File-bound chat, offline/remote confirmation, cancellation, stale sessions, and draft lineage | `go test ./internal/app ./internal/api/handlers -run 'ChatSession|Draft'` |
| Manual draft edit invalidates old validation/checks; invalid or stale drafts cannot Apply | `go test ./internal/app -run 'DraftLifecycle|DraftValidateCheckApply'` |
| One-file guarded Apply/Undo, comparison, export, and source-free audit | `go test ./internal/app -run 'Apply|Undo|Compare|Export'` |
| Empty landing/open-project transition, shortcut gate, compact semantic controls, concise workspace labels, Bugs filter disclosure, Analyze-all limits, responsive groups, Editor stage semantics, Editor-only explorer/context panes, read-only source/diff, and revision/file guards | `./desktop/gradlew -p desktop test` |
| Live routes, OpenAPI, API reference, version, and retired browser routes | `go test ./cmd/daemon ./internal/api/handlers` |
| All supported formatting, unit, race, vet, and desktop checks | `make check` |

## Manual desktop acceptance

Start the daemon and desktop client with the normal fixture. First start with no project
open, then complete this exact keyboard flow on a wide layout, exactly 1000dp, and a
window narrower than 1000dp:

1. With no project open, confirm only product identity, **Open project**, and concise
   opening/retry feedback are visible. Press `Cmd/Ctrl+O` to open the chooser, cancel
   it, and confirm no workspace action becomes available. Exercise a failed open when
   possible and confirm retry stays available. Then open the normal fixture and confirm
   the landing state is fully replaced by the workspace.
2. Use `Cmd/Ctrl+1` through `Cmd/Ctrl+4` to visit Summary, Analysis, Bugs, and
   Editor. Confirm labels, selected state, and freshness/confidence/severity
   text are understandable without color. Confirm the top bar has one text-labeled
   connection state and workspace labels have no numeric inventory counters or raw
   revision/hash identifiers. On wide windows, Summary, Analysis,
   and Bugs must have no Explorer or Context pane; only Editor has them.
3. On Summary, inspect deterministic facts and structured analysis. On Analysis,
   explicitly Start, Pause, Resume, and Cancel Analyze-all; it must not begin on
   import or reindex. Confirm project coverage and current/last-run totals are
   distinct, only failed/error-bearing files are listed with attempts and
   sanitized errors, the file/retry limits share a compact row, and related action
   buttons wrap rather than clip when narrow. Analysis has no file-opening action. Run an explicit
   verified scan and inspect its separate parser, vet, and test progress.
4. On Bugs, confirm only the compact search field is initially visible; use Filters to
   reveal Source, Severity, Freshness, and Lifecycle. Verify active filters are named
   in text without a numeric badge. Filter verified findings separately from AI suggestions, change a
   finding triage state, and use Prepare fix only to open the associated file
   and prefill chat. It must not generate or apply a change.
5. From a non-Editor workspace, use `Cmd/Ctrl+P` to open an indexed file and
   confirm it activates Editor with that exact path. In Editor, select `Run` to
   replace it, then repeat with an absent
   `ReleaseNote` type to create it. Confirm the source and composed diff remain
   selectable and read-only and the file/symbol brief remains visible above the
   source, including in the narrow drawer layout.
6. Open a file-scoped chat session, send one request with `Cmd/Ctrl+Enter`, and
   verify a different file cannot be targeted. Edit only the declaration/import
   draft, use `Cmd/Ctrl+Shift+V` to validate and `Cmd/Ctrl+Shift+C` for checks.
   A manual edit must make the old evidence unusable.
7. Confirm Apply names the expected file and symbol. Apply only after the fresh
   validation and checks, then use the displayed Undo action. Verify
   exactly that one file changes and stale sessions/drafts cannot apply.
8. Confirm compact controls remain focusable and text-labeled at supported text scaling.
   Verify action meaning is not color-only: primary, navigation, positive, attention,
   destructive, and neutral actions each retain readable labels and disabled/selected
   state. Check long labels, focused fields, pressed buttons, and blocked reasons.
9. At exactly 1000dp, confirm Editor retains its wide Explorer and Context panes.
   Below 1000dp, confirm Files and Context drawer actions appear only in Editor
   and leaving Editor closes an open drawer. In both Git and non-Git copies,
   inspect the advisory Git status. Exercise a
   remote-provider confirmation and an offline provider error; no prompt may be
   sent to a remote destination without confirmation.

Use `desktop/KEYBOARD_SMOKE_CHECKLIST.md` for the key-by-key version of the
same flow. Record the release operator, date, fixture variant, and any failure
next to the release candidate; do not record prompts, source, or credentials.

## Supported scope and release decision

Mini-Orca is Go-first for safe declaration editing. Kotlin, Java, TypeScript,
Python, Rust, and other languages can retain conservative analysis and symbol
information but do not have parser-backed exact declaration composition or
equivalent Apply validators in v4.3. This is intentionally deferred, not a
fallback to direct source editing.

The release is acceptable only when the automated table passes, the manual flow
is recorded without a safety-boundary exception, `git diff --check` passes, and
the worktree contains only the intended implementation and documentation
changes. Never release local `config.yaml` or provider credentials.
