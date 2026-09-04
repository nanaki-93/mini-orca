# v4.4 focused declaration-draft release acceptance

Run this checklist from a clean release worktree after applying the intended
implementation and documentation changes. The automated commands are recorded
in the v4.4 release notes; the manual checks below are reproducible evidence
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
| One-file guarded Apply/Undo and source-free audit | `go test ./internal/app -run 'Apply|Undo'` |
| Empty landing/open-project transition, shortcut gate, compact semantic controls, concise workspace labels, Bugs filters and priority grouping with per-card provenance, Analyze-all limits, responsive groups, direct source-symbol selection, active-file header in source/review, removed Editor/source helper copy, conditional imports, Editor-only explorer/context panes, read-only source/diff, and revision/file guards | `./desktop/gradlew -p desktop test` |
| Live routes, OpenAPI, API reference, version, and retired browser routes | `go test ./cmd/daemon ./internal/api/handlers` |
| Independent Analyze → Bug task → Function routing, temporary task-test checks, and no-model validation/check/Apply/Undo/scan/reindex path | `go test ./internal/app -run ScopedModelEndToEndTaskFlowKeepsNonPromptOperationsModelFree` |
| Legacy fallback, independent local/remote profile resolution, API-base validation, and scope-specific confirmation | `go test ./internal/config ./internal/app ./internal/api/handlers -run 'ModelProfile|ModelCatalog|RemoteProvider'` |
| All supported formatting, unit, race, vet, and desktop checks | `make check` |

## Manual desktop acceptance

Start the daemon and desktop client with the normal fixture. Use a clean desktop preference
profile to first start with no remembered project, then complete this exact keyboard flow
on a wide layout, exactly 1000dp, and a window narrower than 1000dp:

1. With no project open, confirm only product identity, **Open project**, and concise
   opening/retry feedback are visible. Press `Cmd/Ctrl+O` to open the chooser, cancel
   it, and confirm no workspace action becomes available. Exercise a failed open when
   possible and confirm retry stays available. Then open the normal fixture and confirm
   the landing state is fully replaced by the workspace. Restart the desktop client and
   confirm the fixture reopens automatically without model access or a project chooser.
2. Use `Cmd/Ctrl+1` through `Cmd/Ctrl+4` to visit Summary, Analysis, Bugs, and
   Editor. Confirm labels, selected state, confidence/severity text, and non-fresh
   analysis labels are understandable without color; confirm the fresh-file dot has
   an accessibility description. Confirm the top bar has one text-labeled
   connection state and workspace labels have no numeric inventory counters or raw
   revision/hash identifiers. Use `Cmd/Ctrl+K` to open **Performance** without starting
   a review; preview the run limits and verify its source-based/not-measured label before
   explicitly starting a run. On wide windows, Summary, Analysis,
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
   in text without a numeric badge. Confirm filtered findings are grouped in nonempty
   high, medium, low, then fallback priority sections, while every card still exposes
   verified/tool or AI provenance. Change a finding triage state, and use Prepare fix
   only to open the associated file and prefill chat. It must not generate or apply a
   change.
5. From a non-Editor workspace, use `Cmd/Ctrl+P` to open an indexed file and
   confirm it activates Editor with that exact path. Confirm the persistent header shows
   its basename and project-relative path, including in review and for duplicate
   basenames, without revealing an absolute path or retaining stale identity. Hover and
   click within `Run`'s declaration and confirm the selected declaration range and
   Context explanation update immediately. Confirm nested declarations select the
   narrowest containing declaration, and clicking between declarations clears stale
   symbol Context while retaining the focused line. Drag source text to confirm selection
   works without an edit or retarget. Repeat from the symbol palette and, below 1000dp,
   confirm both paths open the Context drawer. The source and composed diff must remain
   selectable and read-only.
6. Confirm exact atomic `Run` exposes one **Edit Run** action. Activate it and verify
   the bound Replace composer opens without a provider call or source write. Use
   Commands → **Create declaration** for absent `ReleaseNote`; the new-name field must
   appear only on that route. With a live draft, request another target and confirm the
   explicit draft-discard decision names both targets and does not silently clear the
   existing work.
7. Open a file-scoped chat session, send one request with `Cmd/Ctrl+Enter`, and
   verify a different file cannot be targeted. For a draft without imports, confirm
   `Required imports` is absent; for one with imports, confirm its values remain
   editable. Edit only the declaration/import draft, use `Cmd/Ctrl+Shift+V` to validate
   and `Cmd/Ctrl+Shift+C` for checks when each contextual action is available. A manual
   edit must make the old evidence unusable. Confirm no Editor progress explanation or
   source selection subtitle remains, and successful validation opens the read-only
   diff below the active-file header.
8. In Review, confirm scope identity, validation evidence, focused checks, collapsed
   diagnostic/command detail, and read-only impact/Git context appear together. Confirm
   Apply names the expected file and symbol and appears only after fresh checks. Apply
   only then, use the displayed Undo action, and verify exactly one file changes while
   stale sessions/drafts cannot apply.
9. Confirm compact controls remain focusable and text-labeled at supported text scaling.
   Verify action meaning is not color-only: primary, navigation, positive, attention,
   destructive, and neutral actions each retain readable labels and disabled/selected
   state. Check long labels, focused fields, pressed buttons, and blocked reasons.
10. At exactly 1000dp, confirm Editor retains its wide Explorer and Context panes.
   Below 1000dp, confirm Files and Context drawer actions appear only in Editor
   and leaving Editor closes an open drawer. In both Git and non-Git copies,
   inspect the advisory Git status. Exercise a
   remote-provider confirmation and an offline provider error; no prompt may be
   sent to a remote destination without confirmation.

Use `desktop/KEYBOARD_SMOKE_CHECKLIST.md` for the key-by-key version of the
same flow. Record the release operator, date, fixture variant, and any failure
next to the release candidate; do not record prompts, source, or credentials.

## Scoped-model manual acceptance

Use an ignored personal `config.yaml`; never put a real key in a screenshot,
fixture, log, or this repository. Restart the daemon after every scope-profile
change. Verify the following without relying on model listing success:

1. An all-local Ollama or LM Studio configuration for all three scopes.
2. One compatible online OpenAI, Claude, or Gemini profile, confirming only the
   scope that sends prompt content.
3. A mixed configuration with remote `analyze` and `bug` plus local `function`.
4. A fresh bug task with a deliberately failed temporary test, one explicit
   **Revise with check output**, review, and explicit Apply. Confirm the fourth
   repair is unavailable and that the generated test never appears in the
   imported project.

These checks are manual because they require a personal provider account or a
running local model and a Compose Desktop window. A release operator records
which profile type, date, and result were used, but not credentials, prompts, or
source content.

## Documentation and container acceptance

Confirm maintained documentation links resolve, `docs/openapi.yaml` remains the
machine-readable companion to the sole human API guide at
`docs/api-contract.md`, and the task ledger exposes only the active cleanup
record. Build the supported image without cleanup commands, start it with an
ignored local `config.yaml`, and confirm `GET /health` plus the image health
check succeed. The runtime image must contain the daemon binary and required
runtime packages only—never Go source, a local configuration file, generated
Desktop output, or credentials. If Docker is unavailable, record that as a
release blocker instead of claiming container acceptance.

## Current execution record

On 2026-09-02, the automated Desktop suite and `make check` passed for the focused
Desktop UX refinement, including active-file identity, removed Editor/source copy,
conditional imports, and priority-grouped Bugs. Manual desktop acceptance is **not
recorded as passed**: this non-interactive execution environment cannot inspect a
running Compose Desktop window at wide, exactly 1000dp, or below 1000dp, and no
release-fixture GUI session was available. A release operator must complete the manual
flow above before treating this release checklist as fully accepted.

On 2026-09-03, scoped-model automated coverage ran only against local `httptest`
providers and temporary repositories. It covers independent Analyze → Bug task →
Function routing, task pinning, temporary test failure/pass behavior, three explicit
repairs, and model-free validation/check/Apply/Undo/scan/reindex operations. The
all-local, online-compatible, mixed-provider, and GUI manual checks are **not run**
in this environment because no personal credentials, local model server, or interactive
Compose session is available. This is a recorded manual limitation, not a provider
compatibility claim.

On 2026-09-03, Task 117 ran the final automated acceptance matrix: `make
fmt-check`, `go test ./...`, `make test-race`, `make vet`, `go mod tidy -diff`,
`./desktop/gradlew -p desktop test`, `make check`, `make quality`, and `git diff
--check` all passed. The pinned staticcheck, dead-code, clone, gocyclo, Spotless,
and Detekt gates had no in-scope production finding; Go coverage was 75.6% and
the Desktop suite contained 135 tests. A loopback daemon started with
`config.example.yaml`; its `/health` and `/status` checks passed and it was shut
down. The fixture flow requiring a local/remote model and an interactive Compose
window remains **not run**, as does image build/health/size acceptance: Docker's
daemon and Compose plugin are unavailable in this environment. These are release
operator checks, not passed acceptance claims.

## Supported scope and release decision

Mini-Orca is Go-first for safe declaration editing. Kotlin, Java, TypeScript,
Python, Rust, and other languages can retain conservative analysis and symbol
information but do not have parser-backed exact declaration composition or
equivalent Apply validators in v4.4. This is intentionally deferred, not a
fallback to direct source editing.

The release is acceptable only when the automated table passes, the manual flow
is recorded without a safety-boundary exception, `git diff --check` passes, and
the worktree contains only the intended implementation and documentation
changes. Never release local `config.yaml` or provider credentials.
