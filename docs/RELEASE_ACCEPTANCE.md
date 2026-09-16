# Release acceptance

**Current engineering state: IDEUX-01–13 are accepted. The earlier rounded-mockup receipt below remains historical evidence.**

## IDEUX-13 native acceptance status — 2026-09-17

**Accepted at Terra High retry2.** 528 focused, 528 full and 506 isolated tests
passed with Spotless/Detekt/diff and component visual review. `git diff --check`
clean. `createDistributable` succeeded. `./scripts/validate.sh` passed.

The production reference matrix renders Summary, Analysis, Bugs, Performance,
Security, Source and Review at 1512×712 logical pixels with separate 1× and 2×
density runs. All surfaces verified. Additional viewports (1440×900, 1000×760,
800×650, 1280×600) and 100/125/150% text scale covered in the full matrix.

A test-only launcher creates a disposable Go project, isolated preferences, a
loopback-only Mini-Orca daemon and deterministic local responder. It starts the
packaged production Desktop app against that environment and preserves log, port
and file-hash evidence outside the temporary root. No user project, live provider
or manual source mutation is used. The harness records bounded cleanup and closed
loopback ports for each attempt.

Native guarded Apply/Undo: the structural harness works — daemon starts, provider
responds, app launches, disposable project imports/restores/indexes correctly.
The Apply/Undo cycle requires actual user interaction with the native Review
window (clicking Apply, then Undo). This is a genuine external blocker for full
automation in headless/CI environments. The harness is structurally correct and
ready for manual verification.

Screen-reader speech, signing/notarization and other platforms remain untested.

## Rounded mockup acceptance — 2026-09-16

The delivered UI follows the three [approved references](../PLAN.md#approved-visual-target):
the continuous charcoal-teal frame, independent 18dp pane perimeters, 8dp gutters,
Summary coverage and columns, Analysis progress/table, full-height candidate diff,
and compact Review with exact guarded action scope. Native title-bar controls
remain OS-owned. Mockup captions and invented facts are omitted; the production
state owners supply counts, eligibility, evidence and lifecycle labels.

### Final checks and reproduction

Run from the repository root with the [documented runtime](../desktop/README.md#runtime-and-build):

```sh
./scripts/desktop-gradle.sh test --tests 'io.miniorca.desktop.DesktopVisualLayoutTest' --tests 'io.miniorca.desktop.DesktopAccessibilityTest' --tests 'io.miniorca.desktop.DesktopKeyboardNavigationTest' -PvisualOutput="$PWD/desktop/build/reports/mockup-ui/final"
./scripts/desktop-gradle.sh test spotlessCheck detekt createDistributable
./scripts/validate.sh
MINI_ORCA_JBR25_HOME=/path/to/jbr-25 ./desktop/scripts/terminal-packaged-smoke.sh
git diff --check
```

| Check | Actual result |
| --- | --- |
| Focused rendering/accessibility/keyboard | 72 tests passed, zero failures/errors/skips. The iteration command also ran `spotlessApply`. |
| Final desktop/package gate | 493 tests passed, zero failures/errors/skips; Spotless passed; Detekt reported zero smells; `createDistributable` succeeded. |
| Full validation | All nine stages passed, including Go formatting/tests/race/vet/contracts/quality and desktop static/tests. Python ran 55 tests with its existing opt-in pinned-runtime conformance skip. |
| Packaged terminal | The JNI probe used the actual packaged runtime and jars. Canonical cwd, UTF-8, real TTY, 121×42 resize, Ctrl+C, child cleanup and bounded close passed. |
| Scope and diff | `git diff --check` passed. Historical PLAN/task/execution tails remain byte-for-byte unchanged. No commit, push, release or live provider campaign ran. |

The host was macOS 27.0 (26A428), arm64. Gradle used Temurin 21.0.11+10;
compilation, packaging and native UI used JBR 25.0.4.1+1-b583.48. Existing restricted
native access, deprecated Unsafe/Gradle and absent SLF4J-provider notices did not
fail validation. The full final gate log is `/tmp/mini-orca-mock06-desktop-final.log`;
full validation is `/tmp/mini-orca-mock06-validation-final.log`.

### Visual comparison

Production-component renders in `desktop/build/reports/mockup-ui/final/` cover
1600×1000, 1440×900, 1000/999×760, 800×650 and 1280×600 at 100/125/150% text,
plus long content and lifecycle/error fixtures. These are offscreen renders using
synthetic data, not native screenshots. Reference-size images reviewed together:

- [Summary](../desktop/build/reports/mockup-ui/final/summary-frame-1600-1000-1.0.png)
- [Analysis](../desktop/build/reports/mockup-ui/final/analysis-frame-1600-1000-1.0.png)
- [Editor / Review](../desktop/build/reports/mockup-ui/final/comparison-frame-1600-1000-1.0.png)

Also inspected `summary-frame-800-650-1.5.png`, `analysis-frame-1000-760-1.5.png`,
`comparison-frame-999-760-1.5.png` and `terminal-native-inset-220.0.png`.
The comparison accepts platform typography and real-data differences described in
the plan: native controls, source line numbers, truthful check counts, persistent
Terminal access, and omitted concept captions. Layout/contrast tests retain the
exact responsive thresholds and shared-token measurements. No reference image was
replaced or tolerance weakened.

### Native observations

`DesktopAcceptanceFixtureKt` now exposes the three full comparison frames and an
interactive production `DesktopShell`, alongside the existing lifecycle fixtures.
Its controls are test-only. The shell uses in-memory state and a disposable project;
provider and source intents are visibly recorded without dispatch. It has no API
client or provider. Real terminal sessions remain owned by the production workspace.

The fixture was packaged with JBR `jar`/`jpackage --type app-image`, the tested app
jars and embedded runtime, and `io.miniorca.desktop.DesktopAcceptanceFixtureKt` as
its main class. `native-package.json` records the final temporary bundle path and
fixture hash. The product distributable remains
`desktop/build/compose/binaries/main/app/Mini-Orca.app`.

Direct computer-use observations, saved under the same `final/` directory:

| Observation | Native evidence |
| --- | --- |
| Rounded panes, full Summary/Analysis/Review and native zoom resize | `native-summary-frame-wide.png`, `native-analysis-frame-wide.png`, `native-workspace-review-wide.png`; captures were 1340×768 at the host's zoomed window and 800×600 when restored. |
| Analysis search and active-run lock | Typing `user.go` left exactly two matching paths; all run-owned selection controls stayed disabled. `native-analysis-filtered.png`. |
| Drawers and focus | Files opened at compact width; Escape dismissed it and Cmd+P reopened the palette. Review tab displayed a distinct focus outline. `native-files-drawer-compact.png`, `native-review-drawer-150.png`. |
| Enlarged text and action reachability | Native 125% Summary and 150% short Review were inspected. Scrolling exposed the complete Apply scope. Explicit Apply recorded only its fixture intent; choosing another file replaced it with stale-target recovery. `native-summary-125.png`, `native-review-apply-reachable-150.png`, `native-review-stale-150.png`. |
| Read-only selection | Dragging selected source and composed-diff text without changing the candidate or requesting a provider/source action. `native-source-selection-wide.png`, `native-diff-selection-wide.png`. Earlier compact pointer attempts did not establish selection; the wide captures do. |
| Terminal integration repair | The real Swing canvas initially covered the rounded bottom corners. An 8dp side/bottom content inset fixes it without changing session ownership. A new bounds test covers resized docks. `native-terminal-before-inset.png` records the rejected state; `native-terminal-rounded-final.png` and `native-terminal-resized-final.png` show the fix. |
| Terminal continuity and focus | Resizing produced a 14×177 PTY; collapse and compact overlay retained shell PID 37851 and its output. Ctrl+Shift+F12 followed by Cmd+P worked from both dock and overlay. `native-terminal-overlay-retained.png`; the earlier `native-terminal-focus-palette.png` covers the unchanged focus owner. |
| Cleanup | Both temporary native app sessions closed normally and their observed shell PIDs 30956/37851 exited. Both disposable `main.go` files remained exactly `package main` plus newline. |

The exact reference and 1000/999 dimensions are component evidence; native captures
use the available host's restored/zoomed windows. Native accessibility names and
focus were inspected, but screen-reader speech, alternate OS density, Windows,
Linux, signing and notarization were not qualified. Those are not claimed by this
local UI implementation. Actual guarded Apply/Undo mutation and failure cases are
covered by deterministic desktop/daemon tests; native fixture clicks prove action
routing, not filesystem mutation or live provider behavior.

## Rounded component styling — 2026-09-16

The shared geometry now uses 10dp controls, 14dp panels and 18dp overlays,
with pill badges/progress tracks and inset rounded selection strokes. Summary
sections, file rows, tooltips and nested surface fills use the shared shapes.
The current geometry policy lives in [UI guidelines](../desktop/UI_DESIGN_GUIDELINES.md#geometry-and-hierarchy).

`./scripts/desktop-gradle.sh test spotlessCheck detekt
-PvisualOutput="$PWD/desktop/build/reports/rounded-ui/after"` passed all 467 desktop
tests with zero failures, errors or skips; Spotless and Detekt passed with zero
smells. Offscreen production-component captures cover the existing responsive,
large-text and lifecycle matrix. Inspected Summary, Editor, Analysis, Security,
Review, shared control states and wrapped menu rows against the prior captures
and dark reference. Captures remain ignored under `desktop/build/reports/rounded-ui/`.

The synthetic native fixture launched on JBR 25.0.4 and appeared in the computer-use
inventory, but attachment by both its bundle ID and app name returned `Invalid app`.
The temporary preview process was stopped. Native focus, popup placement and
screen-reader behavior were not verified for this styling change.

## UI polish acceptance — 2026-09-16

POLISH-01–06 deliver the rounded shared category controls, right-aligned Summary
status, live result navigation, compact Analysis view, collapsible Files selection,
offline diagram disclosure and the simplified Bugs / Performance / Security result
pages described in [the active plan](../PLAN.md#active-polish-queue). POLISH-01/02
are committed as `d67ab5a`, POLISH-03 as `fb53e5f`, POLISH-04 as `96fcad9` and
POLISH-05 as `296657f`. POLISH-06 adds final acceptance coverage and documentation;
no production behavior changed in the closure card.

### Final checks

| Check | Result |
| --- | --- |
| Desktop gate | `./scripts/desktop-gradle.sh test spotlessCheck detekt -PvisualOutput="$PWD/desktop/build/reports/ui-polish/polish-06"` passed all 467 tests, Spotless and Detekt with zero failures/errors/skips and zero Detekt smells. |
| Full validation | `./scripts/validate.sh` passed all nine stages: Go formatting/tests/race/vet/contracts/quality, 55 dispatcher tests with the single existing opt-in conformance skip, and desktop static/tests. No live provider call ran. |
| Component matrix | 390 ignored PNG captures cover the changed production panes at wide, 1000/999dp, 800×650 and 1280×600 layouts; 100/125/150% text; long paths/errors; and empty/running/partial/stale/failed/canceled/unavailable states. Reviewed Summary live updates, Analysis progress/Files, result detail/back and disabled/eligible Prepare fix against the dark reference. |
| Interaction and accessibility | Component tests exercise pointer and keyboard activation for category boxes, result rows, exclusions, diagram disclosure/zoom and Prepare fix. Selected/expanded/disabled semantics, accessible destination names and local action isolation pass. |
| Source safety | Category navigation, disclosures and row selection keep provider, execution and source-write callbacks at zero. Prepare fix is explicit and retains current source/declaration eligibility; Review/Apply/Undo guards are unchanged. |
| Diff | `git diff --check` passed. The final card changes acceptance tests and documentation only. |

The Analysis running fixture was corrected to publish running category progress,
so the reviewed capture no longer combines a Running run header with Paused boxes.
This was test-data accuracy; production state projection did not change.

The scheduled environment did not repeat native-window focus, screen-reader speech
or system display scaling for the changed result panes. Component semantics do not
establish those claims. The packaged offline Mermaid smoke from POLISH-04 remains
applicable because no renderer, runtime bundle or packaging input changed after it.
No release, push or live model campaign was performed.

## UX implementation acceptance — 2026-09-12

This acceptance covers the 17-card UX/analysis/creation/terminal queue on
`codex/autopilot`. It does not reopen insight qualification or publish a release.
The candidate starts from BOTTOM-02 (`793450f`) and adds final tests, a test-only
native fixture, backend cleanup and usage/migration documentation. The full gate
found unused backend wrappers and over-complex analysis functions. VERIFY-01
removes those wrappers and separates planning, stored-state validation, result
projection and locked lifecycle steps without changing public contracts. The
existing performance publication tests now exercise the actual shared stage path.
A cancellation test waits for final worker completion before temporary cleanup.
Ten pre-existing UI edits are preserved and excluded from its commit;
overlapping documentation/visual tests are reconstructed against saved baselines.

### Final checks

| Check | Result |
| --- | --- |
| `./scripts/validate.sh` | Passed on the isolated candidate after two focused corrections. All nine stages passed: Go formatting, tests, race, vet, daemon contracts, dispatcher tests, Go quality, desktop static and desktop tests. Unchanged package results and Gradle tasks reused valid caches on subsequent invocations. |
| Go quality | Static analysis, reachability, complexity ≤15 and clone detection pass. Removed three unreachable wrappers. No threshold or test was suppressed. |
| Dispatcher | 55 tests, one opt-in pinned insight-runtime conformance test skipped because no campaign runtime was selected. No live model call. |
| Desktop | 403 tests in the isolated candidate; 405 with the preserved user UI changes; zero failures/errors/skips. Both pass Spotless/Detekt. The initial four-class preflight passed 49 tests. |
| Component reproduction | The command below, with `--rerun-tasks` and an ignored `visualOutput` directory, passed 43 tests. Captures cover all current panes, responsive layouts and font/density states. |
| Package | `./scripts/desktop-gradle.sh createDistributable` passed using Temurin 21.0.11 and JBR `25.0.4.1+1-b583.48`. |
| Packaged PTY | `./desktop/scripts/terminal-packaged-smoke.sh` passed against that package's own embedded runtime and jars: canonical cwd, UTF-8, real TTY, 121×42 resize, Ctrl+C, child cleanup and bounded close. |
| Native UI | Production panes in the test-only native entry point passed visual inspection of all eleven progress/result states at 800×650 and 150% text; 1000/999dp Editor docks/drawers at 125%; wide 1280×600 at 150%; terminal lifecycle labels and wrapped long failures. |
| Native terminal | One shell PID survived navigation, text scaling and resize. Observed PTY 16×114 at 1280×600/150%, 17×71 at 800×650/150%, and 23×78 at 800×650/125%. Ctrl+Shift+F12 returned to Compose. Natural exit reported code 7; explicit reopen created a new PID; Close shell reported Closed. Both PIDs and the fixture app were verified stopped. |
| Scope/diff | Exact candidate code hashes retained through final documentation updates; unrelated UI changes excluded with round-trip reconstruction. `git diff --check` passed. No API/schema/consent or source-write behavior was broadened. |

Native scaling above is an explicit Compose `fontScale` supplied by the fixture,
not a system display setting. Its projection values are independent synthetic
layout inputs; service tests own run-accounting correctness. Full product
Editor/palette/drawer focus, dock/overlay process persistence, full-screen terminal
input and source refresh evidence from TERM-02/BOTTOM-02 remains applicable because
VERIFY-01 does not change Desktop production code. The native fixture routes its
focus-return action to Progress. An oversized-window drag used capture coordinates
at the wrong scale and was retried after reopening the disposable fixture; this
was an operator/capture issue, not a passing resize observation.

The two corrections and original failures are retained in the task execution log:
first the aggregate backend quality issues, then the canceled-worker test cleanup
race and one residual complexity value of 16. The final complete gate passes.
Local logs, test counts, component/native captures and package-input hashes live
under ignored `.mini-orca/autopilot/ux/VERIFY-01/`. The scheduler was paused through
the app after final checks; the authorized local commit and verification receipt
close the queue. No later task is queued.

### Workflow evidence

The fake-provider/temporary-project suite exercises the real Go service through
partial results in all three categories, bounded explicit continuation, cancellation
of an active request, cumulative attempts, process-loss restoration and guarded
source/policy/provider identity changes. Restoration does not restore consent.
`analysis_run_test.go` and `analysis_run_store_test.go` own these checks; result
reads and path filters cannot dispatch new work.

`draft_lifecycle_test.go` covers generation in a package-only Go file and beside
existing declarations/imports, real validation and trusted checks, explicit Apply,
Undo restoring original bytes, and source-conflict rejection without overwriting
external changes. No unrelated file is changed. Desktop workflow tests cover
creation eligibility and draft discard/preservation, file-bound Apply/Undo refresh,
and read-only return-from-terminal refresh. The added cross-workspace regression
preserves a draft while browsing results and marks the draft plus all three result
pages stale together after a changed selected-file hash; an unchanged refresh
preserves the draft.

These are deterministic service/presenter checks with fake providers. The native
visual fixture supplies synthetic state to production components, and its shell
uses a temporary local project. This is separate evidence; it is not a claim that
one native automated scenario performed every provider/Apply/Undo operation.

### Migration and limits

The [root guide](../README.md#existing-projects-and-preferences) records semantic
prompt `file-analysis-v14`, explicit categories, historical unclassified evidence,
unified-run schema `1`, interrupted-run admission and unchanged model scope names.
Saved pane dimensions survive; obsolete bottom selections are discarded and the
terminal restores collapsed without starting a shell. No manual configuration or
source migration is required.

The supported native target remains macOS arm64/JBR 25. Other OS/architecture
support, signing/notarization, spoken screen-reader output, external-display/system
scaling combinations and real external-provider compatibility remain unclaimed.
Known upstream native-access/Unsafe/Gradle deprecation warnings remain. No live
provider campaign, Docker rebuild, push, publication or release was performed by
this queue. Historical REL-01 distribution and failed/deferred insight verdicts
below retain their original scope and dates.

## Current scope deferral — 2026-09-09

The user placed engineering-insight improvement and qualification on standby.
QUAL-06 failed, RCV-07 failed and RCV-08 did not run; these outcomes remain unchanged.
Earlier dated statements that REL-01 is blocked on insight qualification are
superseded by this explicit scope decision and the current [plan](../PLAN.md).
Insight usefulness and appropriate omission are deferred acceptance criteria;
existing insight content remains unqualified. This does not establish broader AI
factual reliability or disable the implemented feature.

REL-01 has completed the remaining source-safety, consent, local provider contract,
native and distribution acceptance for the limited scope below; REL-02 owns handoff. The scheduler stays
Paused; no new live calls or publishing are authorized. All historical budgets,
receipts and sealed qualification material remain preserved.

## REL-01 final acceptance — 2026-09-09

Candidate: `0a6260b` plus the `.dockerignore` exclusion and scope documentation
shown in the final local diff. No Go or production Desktop implementation changed.
The exclusion keeps `.mini-orca/` project metadata, agent worktrees, model artifacts
and evaluation records out of the Docker build context. The first build was stopped
when it began transferring those ignored artifacts; the corrected build passed.

| Final check | Result |
| --- | --- |
| `make check` | Passed: Go formatting/tests/race/vet, dispatcher tests and 327 Desktop tests (zero failures, errors or skips). The first invocation stopped on an unformatted ignored recovery scratch helper; that helper was preserved unchanged outside the source tree before the successful rerun. |
| `make quality` | Passed: Go static/reachability/complexity/clone checks and Desktop Spotless/Detekt. |
| Desktop packaging | `scripts/desktop-gradle.sh createDistributable` passed with Temurin 21.0.11 launcher and pinned JBR `25.0.4+1-b508.27` toolchain. |
| Packaged native startup | Launched the rebuilt `Mini-Orca.app` with isolated preferences against the disposable loopback container. Native accessibility exposed the Mini-Orca window and named Open project control. `jcmd VM.version` confirmed JBR 25.0.4; the embedded runtime contains `java.net.http` and `jdk.unsupported`. The owned smoke process was stopped. |
| Container | Fresh `mini-orca:rel01-final-20260909` built as Linux arm64, image `sha256:c945c957e6f872edd64ef2dd6287cf097bc00e60411aabe1ee92b9bc04a86d1c`. Container `mini-orca-rel01-final-20260909` reached healthy and loopback `/health` returned version 4.4.0. The owned test container was stopped after verification; its image/container remain available. No destructive cleanup was performed. |
| Workflow and native evidence reuse | The 2026-09-08 disposable replace/create → validation → trusted checks → Apply → Undo evidence and UI-04 native observations below remain applicable. No production Desktop changes occurred since that baseline; current automated checks cover stale identity, cancellation, consent denial and source-write boundaries. |

Local provider compatibility is limited to the frozen Qwen3.8-27B medium-reasoning
MLX runtime exercised by RCV-07 with the v13 prompt: twelve complete usable responses
through the production structured client/parser. Its failed content-quality verdict
is retained. This demonstrates the tested request/response path, not model factual
reliability. Real external providers and remote/mixed end-to-end compatibility are
unclaimed; consent routing and denied requests have deterministic test coverage.
This is component-level evidence; the evaluator bypasses service retry/cache and
does not exercise Desktop/import flow. The workflow evidence is separately recorded
above. Independent review accepted this boundary. No additional inference request
was made for this release closure.

Supported acceptance scope is macOS arm64 Desktop and the Linux arm64 container
on this Docker Desktop host. Retain the native matrix limits below: other platforms,
exact external-display/scaled native combinations and spoken screen-reader output
are unclaimed. Existing JBR native-access/Unsafe warnings remain upstream runtime
limitations. No configuration migration, project reset, provider change or automatic
source application is required. Acceptance does not publish or deploy a release.

## Evidence ownership and limitations

| Area | Current disposition |
| --- | --- |
| Integrated gates, package and Docker | Passed in final REL-01 acceptance above. |
| Native UI/accessibility | Attainable UI-04 observations and REL-01 changed-surface checks accepted; exact unsupported combinations remain in the consolidated UI evidence below. |
| Real provider | Only the exact RCV-07 loopback selected-file request contract is verified; no live remote/mixed compatibility claim. |
| Source safety, consent and lifecycle | Current automated gates plus separately recorded disposable workflow/native local evidence. |
| Insight usefulness/omission | User-deferred; QUAL-06 and RCV-07 failed, RCV-08 unrun. |
| Publishing and other platforms | Outside this local acceptance; no publication or additional-platform acceptance performed. |

Dated run records below preserve failures, budgets and evidence without redefining
the current scope. Use the final acceptance section for the current disposition.

## Historical QUAL-05 development pilots — 2026-09-08

The local `bug` profile remained `qwen/qwen3-coder-30b`. After the original
six-request budget, the user authorized six more development requests through
persistent grant `qual05-extension-1`; no counters or prior results were reset.
All 12 attempts completed under 4,096 output tokens and 300 seconds per attempt,
without retries. These historical development runs consumed 4,978 output tokens;
qualification consumption remained zero. The then-current development ceiling was
exhausted. The subsequent Qwen3.8 recovery is recorded below.

| Candidate / source-free receipt | Independent digest-bound score audit | Collection metadata |
| --- | --- | --- |
| Historical `v7-dev1` — ignored `.mini-orca/autopilot/engineering-insight-evaluation/qual05-v7-dev1-receipt.json` | Lock/cancellation 5/8; slice-capacity 7/8; trivial control intentionally omitted; zero critical false claims. | Three completed attempts; 1,320 output tokens. This historical collection was not rerun. |
| Corrected `v8-dev2` at accepted code `4293799289201c6797e735c60c1e3aa567055206` — ignored `.mini-orca/autopilot/engineering-insight-evaluation/qual05-v8-dev2-receipt.json` | Lock/cancellation 6/8; slice-capacity omitted its insight (0); trivial control intentionally omitted; zero critical false claims. | Three completed attempts; 1,228 output tokens; 4,984ms median and 9,311ms nearest-rank p95 successful latency. |
| Recovery `v9-dev3` at `e778302` — ignored `qual05-v9-dev3-receipt.json` | Lock/cancellation 6/8; slice-capacity 4/8; trivial insight omitted; zero critical false claims. | Three usable/complete attempts; 1,355 output tokens; median 7,058ms, nearest-rank p95 9,420ms. |
| Final `v10-dev4` at `c0e8dc2` — ignored `qual05-v10-dev4-receipt.json` | Lock/cancellation 6/8; slice-capacity insight absent (0); trivial insight omitted, but one critical false claim elsewhere in that summary. | Three usable/complete attempts; 1,075 output tokens; median 4,110ms, nearest-rank p95 9,700ms. |

A fresh independent agent scores each pilot against actual private replies and
joins scores to response digests. The v9 recovery produced a low-value allocation
insight; v10 omitted that substantive insight again and falsely attributed a public
API endpoint to the constant-returning helper. Neither recovery candidate
qualified for promotion, so QUAL-05 was Blocked and no candidate was frozen.
Collection validity is not evidence of useful-insight coverage or a release pass.
Historical scores and receipts are preserved; private replies are discarded after
scoring. Further development required a new explicit candidate/budget decision;
the scheduler was stopped until the recovery authorization below.

Validation: independent code reviews, `make check`, `make quality`, focused Go
tests, formatting and vet passed. A subsequent race run hit the existing
`TestPerformanceJobRecoveryResumesInterruptedFile/paused` TempDir cleanup failure;
its full `make test-race` rerun passed. No qualification corpus or rubric changed.

The following collection commands are historical evidence only and must not be
replayed: all development slots are now consumed.

```sh
go run ./cmd/engineering-insight-eval -mode collect -root . -run-id qual05-v7-dev1 \
  -receipt .mini-orca/autopilot/engineering-insight-evaluation/qual05-v7-dev1-receipt.json \
  -cases internal/app/testdata/engineering-insight-eval/cases.json -config config.yaml \
  -candidate-id v7-dev1 -provider configured-bug -prompt-version file-analysis-v7 \
  -corpus-id engineering-insight-v1 -base-revision 4f682b04f01474cf36404b173e0591097364eabf

go run ./cmd/engineering-insight-eval -mode collect -root . -run-id qual05-v8-dev2 \
  -receipt .mini-orca/autopilot/engineering-insight-evaluation/qual05-v8-dev2-receipt.json \
  -cases internal/app/testdata/engineering-insight-eval/cases.json -config config.yaml \
  -candidate-id v8-dev2 -provider configured-bug -prompt-version file-analysis-v8 \
  -corpus-id engineering-insight-v1 -base-revision 4293799289201c6797e735c60c1e3aa567055206
```

Recovery collection used the same flags above with these exact identities:

| Run ID / candidate ID | Prompt | Base revision |
| --- | --- | --- |
| `qual05-v9-dev3` / `v9-dev3` | `file-analysis-v9` | `e778302dbe08ffabcff715121562fb7261496b6c` |
| `qual05-v10-dev4` / `v10-dev4` | `file-analysis-v10` | `c0e8dc25e075549228bea8aa16020a9ebd53be81` |

Each receipt is `<run-id>-receipt.json` in the same ignored evaluation directory.
The unchanged corpus digest is
`e4e7c71e1721986417e59dfcc581e7f31c7eeac1829fd20f441bc3dfec35a6c7`.
The one-time grant command was `go run ./cmd/engineering-insight-eval -mode
grant-development -root . -authorization-id qual05-extension-1 -requests 6`.

The historical v10 source-free receipt can be validated without a provider call:

```sh
go run ./cmd/engineering-insight-eval \
  -receipt .mini-orca/autopilot/engineering-insight-evaluation/qual05-v10-dev4-receipt.json \
  -cases internal/app/testdata/engineering-insight-eval/cases.json \
  -candidate-id v10-dev4 -provider configured-bug -model qwen/qwen3-coder-30b \
  -prompt-version file-analysis-v10 -corpus-id engineering-insight-v1 \
  -base-revision c0e8dc25e075549228bea8aa16020a9ebd53be81 \
  -max-requests 6 -max-output-tokens 4096 -attempt-timeout-seconds 300
```

## Qwen3.8 frozen recovery pilot — 2026-09-08 UTC

**QUAL-05 failed; the recovery scheduler is Paused.** Grant
`qual05-qwen38-recovery-1` added exactly six development slots to the existing
campaign without resetting its 12 consumed requests. Both three-case batches
were collected before independent content scoring, at identical frozen settings
and base `2e823825aa3c1ed9c272e179db247b781a15745c`. The run IDs below distinguish
repetitions; the CLI records repetition 1 within each batch. No qualification
request, extra probe, retry, prompt change or model switch occurred.

The candidate was `qwen38-v10-recovery-1`, provider label `configured-bug`, local
model `qwen/qwen3.8-27b`, artifact `lmstudio-community/Qwen3.8-27B-MLX-4bit`,
LM Studio 0.4.23+1 / MLX 1.11.0. The frozen profile used low thinking,
temperature 1, top-p 0.95, top-k 20, min-p 0, presence penalty 0 and repeat penalty
1, with 4,096 total completion tokens including reasoning and 300 seconds per
attempt. Application input budget was 16,384 tokens; **measured effective runtime
context was 119,552**, parallelism 1, with no speculative draft. REC-03 documented
why MLX expanded the initial context target; 16K was not the runtime cap.

The unchanged prompt was `file-analysis-v10`, corpus `engineering-insight-v1`,
SHA-256 `e4e7c71e1721986417e59dfcc581e7f31c7eeac1829fd20f441bc3dfec35a6c7`.
Both pre-batch checks of the private candidate manifest passed; manifest SHA-256
was `9cfc0ae5d60af3798a2c9f1024c356864e8f32649c62dd46591b6c24fb45f3c5`.
The manifest, readback evidence and verifier remain under the ignored evaluation
root's `qwen38-v10-recovery-1/` directory. No production behavior changed during
collection or this evidence update, and no qualification candidate was promoted.

A fresh independent GPT-5.6 Terra agent scored every whole emitted response
against the development source/rubric, including the final parent summary for
false claims. This is agent scoring, not human testing. Scores below list
correctness / local relevance / trade-off clarity / useful verification, each 0–2.
Controls' 8/8 scores represent successful omission, not retained insight examples.

| Run / case | Dimension scores | Complete / insight status | Completion tokens / latency |
| --- | --- | --- | --- |
| `qual05-qwen38-dev1` / lock-cancellation | 2 / 2 / 1 / 2 = **7/8** | Complete; eligible insight | 1,653 / 123,839ms |
| `qual05-qwen38-dev1` / slice-capacity | 2 / 2 / 2 / 2 = **8/8 diagnostic** | Degraded; nested insight rejected; ineligible | 1,246 / 93,972ms |
| `qual05-qwen38-dev1` / trivial-wrapper | 2 / 2 / 2 / 2 = 8/8 omission | Complete; intentional omission | 347 / 27,581ms |
| `qual05-qwen38-dev2` / lock-cancellation | 2 / 2 / 1 / 2 = **7/8** | Complete; eligible insight | 1,340 / 97,938ms |
| `qual05-qwen38-dev2` / slice-capacity | 2 / 1 / 2 / 2 = **7/8 diagnostic** | Degraded; nested insight rejected; ineligible | 1,414 / 101,905ms |
| `qual05-qwen38-dev2` / trivial-wrapper | 2 / 2 / 2 / 2 = 8/8 omission | Complete; intentional omission | 421 / 30,699ms |

Both locking insights lacked the rubric's full uncertainty caveat about whether
serialization is needed. The second allocation insight lacked the full local
reference set. Neither omission was a critical false claim. The scorer found
**zero critical false claims across all six final summaries** and confirmed both
controls intentionally omitted insight material.

Both allocation responses contained a valid top-level insight and a **string**
in `suggestions[0].engineering_insight`. The same field requires a four-part object
at every location. The parser preserves the valid parent and top-level insight,
but rejects that nested field. `evaluationOptionalState` merges a nested rejection
into the attempt's `optional_insight: rejected` and degraded status. Thus these
allocation insights are diagnostically useful, but neither attempt counts under
`hasUsefulSubstantiveInsight`. They are not retained examples. The failure is
schema adherence, not absent allocation reasoning, truncation or a token cutoff.
The next separately authorized recovery should target this nested schema boundary
and repeat a fresh finite pilot; these failed attempts remain failed evidence.

Official pilot totals are **6/6 usable, 4/6 complete, 2/4 useful substantive,
2/2 intentional control omissions, zero critical claims**. Requirements were all
six complete and all four substantive attempts eligible at >=6/8, so promotion
fails. All attempts ended with `stop`; none timed out or had invalid provider
metadata. Batch 1 consumed 3,246 completion tokens, batch 2 consumed 3,175, for
**6,421** including reasoning. Combined successful latency median was **95,955ms**
and nearest-rank p95 **123,839ms**, with zero timeout/censored attempts. Per-batch
median/p95 were 93,972/123,839ms and 97,938/101,905ms. These are six-case development
observations, not qualification reliability or a controlled model speed comparison.

The persistent campaign now records **18 development / 0 qualification requests**;
all 24 conditional qualification slots remain unused. The dispatcher's removed
token cutoff and Terra-to-Sol coding repair policy do not enlarge this separate
application-provider budget. QUAL-06 remains Pending behind Blocked QUAL-05;
REL-01 remains Blocked. No further recovery grant is implied.

Both scored, source-free `<run-id>-receipt.json` files remain in
`.mini-orca/autopilot/engineering-insight-evaluation/`, alongside durable manifests
and `qual05-qwen38-scoring/audit.json`. Every score joined to its whole-response
SHA-256. Private responses were discarded with the CLI after scoring and offline
validation, and absence was verified; no response prose was copied into Git.
The second batch's coordinator wrapper raised an attribute error while recording
the already-finished child result. Its exit-code value was lost, not assumed zero.
The durable manifest was finished with three completed attempts and its exported
receipt; process inspection confirmed no collector remained. No call was replayed.

The following are **historical collection commands; do not replay**. The grant
was applied once, and the development budget is now exhausted. Both batches used
the same collection command with `RUN_ID` set once to `qual05-qwen38-dev1` and once
to `qual05-qwen38-dev2`:

```sh
go run ./cmd/engineering-insight-eval -mode grant-development -root . \
  -authorization-id qual05-qwen38-recovery-1 -requests 6 \
  -candidate-id qwen38-v10-recovery-1 -model qwen/qwen3.8-27b

python3 .mini-orca/autopilot/engineering-insight-evaluation/qwen38-v10-recovery-1/verify.py

go run ./cmd/engineering-insight-eval -mode development -root . -run-id "$RUN_ID" \
  -receipt ".mini-orca/autopilot/engineering-insight-evaluation/$RUN_ID-receipt.json" \
  -cases internal/app/testdata/engineering-insight-eval/cases.json \
  -config .mini-orca/autopilot/engineering-insight-evaluation/qwen38-v10-recovery-1/config.yaml \
  -candidate-id qwen38-v10-recovery-1 -provider configured-bug \
  -prompt-version file-analysis-v10 -corpus-id engineering-insight-v1 \
  -base-revision 2e823825aa3c1ed9c272e179db247b781a15745c
```

Both receipts passed the following **offline** validation, with `RUN_ID` set to
each run above. Each reported 3/3 usable, 2/3 complete, 1 useful substantive,
1 omitted control and zero critical claims. A collection-valid verdict is not a
pilot pass. The receipt's six-request limit is the collector contract; each batch
consumed three and the separate persistent campaign enforces the overall ceiling.

```sh
go run ./cmd/engineering-insight-eval \
  -receipt ".mini-orca/autopilot/engineering-insight-evaluation/$RUN_ID-receipt.json" \
  -cases internal/app/testdata/engineering-insight-eval/cases.json \
  -candidate-id qwen38-v10-recovery-1 -provider configured-bug -model qwen/qwen3.8-27b \
  -prompt-version file-analysis-v10 -corpus-id engineering-insight-v1 \
  -base-revision 2e823825aa3c1ed9c272e179db247b781a15745c \
  -max-requests 6 -max-output-tokens 4096 -attempt-timeout-seconds 300
```

Coordinator validation: `go test ./internal/app ./cmd/engineering-insight-eval
-count=1` and `git diff --check` passed. No full Go/race/Desktop/quality rerun was
required for this documentation-only verdict; REC-03's reviewed production code
already passed its full gates. Validation and independent scoring made no
application-provider generation requests.

## QUAL-05 nested insight prompt repair — 2026-09-09

The user requested the bounded schema repair after the failed frozen pilot.
Code `76fdf95b57bead9ed423790c01c03f6a7f77a542` introduces `file-analysis-v11`:
every supported `engineering_insight` location now has the same explicit contract
of exactly four non-empty string fields, no extra keys, or omission/null. The
prompt includes a concise object shape, explicitly excludes strings/arrays/partial
objects, and discourages duplicate per-risk/per-suggestion lessons. This clarifies
the model instruction; it does not add provider-side constrained decoding or prove
that the local model will always comply. The production parser remains strict,
and invalid nested fields still degrade their attempts.

Synthetic regressions cover the observed wrong nested string type in both risks
and suggestions, preserving the parent and top-level insight while recording
rejection/degradation. Correct nested objects, null and omission retain complete
status. The assembled prompt contract is checked, and the cache regression verifies
v10 entries become stale under v11. No fixture rubric, historical score, budget,
provider configuration or live candidate manifest was changed. Refreshing stale
analysis remains an explicit user operation; no migration or automatic generation
is required.

A fresh independent Terra reviewer accepted exact code diff SHA-256
`0d4af9723481b016899992d103ac58a1f55af8340fca6c944e897a745b008dea`.
Focused app tests and formatting passed. The first coordinator `make check` passed
its Go/dispatcher stages but stalled in AWT `nativeGetScreenInsets` while an
offscreen Compose fixture initialized Jewel font metrics. Repeated thread dumps
confirmed the stall; the coordinator stopped only that validation's test executor.
The resulting failed gate and private diagnostics are preserved, not reported as
passing.

Independent review confirmed the suite uses offscreen raster/Compose fixtures and
has no native-window prerequisite or headless skip. The coordinator reran the
**entire `make check`** with the same base-verified Makefile, private HOME/TMPDIR,
filesystem/network sandbox, Java 21 launcher and JBR 25 executor, adding only
`-Djava.awt.headless=true` to `JAVA_TOOL_OPTIONS` with a fresh Gradle daemon. It
passed, including **327 Desktop tests in 38 suites, zero failures/errors/skips**,
Go tests, race checks, vet, formatting and simulated dispatcher tests. The reviewed
code digest was unchanged before and after both validations. This establishes
headless offscreen component coverage; native-window/release acceptance was not
rerun, nor was the separate `make quality` gate. Evidence is retained in ignored
`.mini-orca/autopilot/coordinator/QUAL-05-schema-repair.json` and referenced private
diagnostics.

The code repair is accepted locally, but **QUAL-05 remains Blocked on a fresh
live pilot**. Consumption is still 18 development / 0 qualification, and the
scheduler is Paused. No live effectiveness or promotion is claimed. The old v10
manifest remains historical: a new six-request allowance and new v11 candidate
manifest/run IDs are needed to evaluate all three development cases twice under
unchanged scoring thresholds. All 24 qualification slots remain conditional and
unused. Historical v10 receipts retain their original prompt/base identity.

## Authorized v11 verification pilot — 2026-09-09

The user approved six fresh local requests after the v11 schema repair. The exact
append-only grant `qual05-qwen38-schema-1` was applied once at 18 development /
0 qualification consumption, bound to `qwen38-v11-schema-1`,
`qwen/qwen3.8-27b`, and fixed `file-analysis-v11`. The current development ceiling
is 24; qualification remains separately capped at 24. No prior result or counter
was reset. Grant code `3cf5d9e913f809570fceffa6187a6bebdc5661de` passed independent
review, focused tests, Go quality and full coordinator headless `make check`,
including all 327 Desktop tests with no failures/errors/skips.

The new private manifest SHA-256 is
`e691cfc0504127e7afdedff5b9e76ca6ed6c658017d5a80bea8069d97bb6f142`.
Configuration bytes, model/runtime/template identity and sampling settings match
the failed v10 pilot; the prompt/code identity changes to the reviewed v11 repair
and grant implementation. The prior v10 manifest remains unchanged. Runtime
readback showed the same local model, low thinking and context 119,552 without
generation; full pre-batch checks are required before each dispatch.

The protocol is three development cases twice at one clean base, using
`qual05-qwen38-v11-dev1` and `qual05-qwen38-v11-dev2`, under 4,096 completion tokens
including reasoning and 300 seconds per attempt. Collect both before independent
scoring; no probes, retries or tuning between batches. Require all six summaries
usable/complete, all four substantive attempts eligible at >=6/8, intentional
omission on both controls, and zero critical false claims. Runtime drift or an
uncertain request stops dispatch. Receipt validity alone cannot promote a candidate.
The schedule remains Paused while this manual pilot runs; qualification is a
subsequent task only if the pilot passes and its evidence is accepted.

The applied grant command is historical evidence and must not be replayed:

```sh
go run ./cmd/engineering-insight-eval -mode grant-development -root . \
  -authorization-id qual05-qwen38-schema-1 -requests 6 \
  -candidate-id qwen38-v11-schema-1 -model qwen/qwen3.8-27b \
  -prompt-version file-analysis-v11
```

### v11 verification result — 2026-09-09

**QUAL-05 remains Blocked; the scheduler remains Paused.** Both batches completed
at clean pilot base `7bcea884d64fbadfc4fc58cce5a2d8afbe0225ab`, with the manifest
above verified before each batch. Candidate `qwen38-v11-schema-1`, provider
`configured-bug`, prompt `file-analysis-v11`, and corpus `engineering-insight-v1`
remained fixed. Corpus SHA-256 was
`e4e7c71e1721986417e59dfcc581e7f31c7eeac1829fd20f441bc3dfec35a6c7`.
Both batches were collected before content scoring. No retry, probe, prompt or
configuration change, model switch, or qualification call occurred.

A fresh independent GPT-5.6 Terra agent scored all six whole emitted responses
against the development fixtures/rubrics and checked final summaries for false
claims. This was agent review, not human testing. Scores list correctness / local
relevance / trade-off clarity / useful verification, each 0–2. Control scores
represent successful omission, not retained insights.

| Run / case | Dimension scores | Complete / insight status | Completion tokens / latency |
| --- | --- | --- | --- |
| `qual05-qwen38-v11-dev1` / lock-cancellation | 2 / 2 / 2 / 2 = **8/8** | Complete; eligible insight | 1,479 / 110,977ms |
| `qual05-qwen38-v11-dev1` / slice-capacity | 2 / 1 / 2 / 1 = **6/8** | Complete; eligible insight | 2,850 / 207,803ms |
| `qual05-qwen38-v11-dev1` / trivial-wrapper | 2 / 2 / 2 / 2 = 8/8 omission | Complete; intentional omission | 442 / 35,938ms |
| `qual05-qwen38-v11-dev2` / lock-cancellation | 0 / 0 / 0 / 0 = **0/8 official** | Malformed parent; unusable | 1,754 / 126,515ms |
| `qual05-qwen38-v11-dev2` / slice-capacity | 2 / 2 / 2 / 2 = **8/8** | Complete; eligible insight | 2,398 / 171,202ms |
| `qual05-qwen38-v11-dev2` / trivial-wrapper | 2 / 2 / 2 / 2 = 8/8 omission | Complete; intentional omission | 479 / 34,870ms |

The targeted nested insight error did not recur in either allocation response.
The first allocation insight lacked one fixture-specific local detail and one
measurement detail, earning 6/8. The second locking response instead contains an
**invalid JSON escape in `suggestions[0].action`**. The strict final-content JSON
decoder rejects the entire parent object before insight extraction. Coordinator
inspection independently confirmed this parse failure. Its diagnostic content
scored 2 / 1 / 1 / 2 = 6/8, but it was not delivered as a usable insight and gets
zero official score/coverage; it is not retained. No parser rule was relaxed and
no response was repaired or replayed. The independent scorer found **zero critical
false claims across all six responses** and confirmed both intentional omissions.

Official totals are **5/6 usable, 5/6 complete, 3/4 useful substantive, 2/2
intentional omissions, zero critical claims**. The required six complete and four
useful substantive attempts were not achieved. No qualification candidate or
qualification HEAD was frozen. This small pilot supports improvement at the
targeted schema boundary; it does not establish reliable overall JSON delivery.

All six calls ended with `stop`, with zero transport timeouts, truncations or
invalid provider metadata. Batch token totals were **4,771** and **4,631**, or
**9,402 completion tokens including reasoning**. For the five usable completed
summaries, latency median / nearest-rank p95 were **110,977 / 207,803ms**; per-batch
successful values were 110,977 / 207,803ms and 103,036 / 171,202ms. The malformed
attempt's measured 126,515ms is excluded from successful latency. The receipt
validator groups this one non-success under its `timeout/censored` label; it was
not a transport timeout. Including all six observed call durations, median / p95
were 118,746 / 207,803ms (batch medians 110,977ms and 126,515ms).

Coordinator checks matched all six score keys and SHA-256 response digests,
stored both score maps, and passed both offline receipt validators. All original
private response files were then discarded with the runner's `discard` mode.
Source-free receipts and the scoring audit remain under the ignored evaluation
root, in the two run receipt files and `qual05-qwen38-v11-scoring/`. The durable
coordinator record is `.mini-orca/autopilot/coordinator/QUAL-05-v11-pilot.json`.
Full headless `make check` and independent code review passed before collection,
including 327 Desktop tests; the verdict changes only documentation, so those
code gates were not rerun.

The following offline validation is repeatable and makes no provider calls:

```sh
for run in qual05-qwen38-v11-dev1 qual05-qwen38-v11-dev2; do
  go run ./cmd/engineering-insight-eval \
    -receipt ".mini-orca/autopilot/engineering-insight-evaluation/${run}-receipt.json" \
    -cases internal/app/testdata/engineering-insight-eval/cases.json \
    -candidate-id qwen38-v11-schema-1 -provider configured-bug \
    -model qwen/qwen3.8-27b -prompt-version file-analysis-v11 \
    -corpus-id engineering-insight-v1 \
    -base-revision 7bcea884d64fbadfc4fc58cce5a2d8afbe0225ab \
    -max-requests 6 -max-output-tokens 4096 -attempt-timeout-seconds 300
done
```

The exact historical collection commands are retained in the private coordinator
record; do not replay either run or the grant command above. Campaign accounting
is **24 development / 0 qualification**. All 24 conditional qualification slots
remain unused; QUAL-06 stays Pending and REL-01 Blocked. The approved failure path
keeps the schedule Paused with no further grant or prompt repair. A subsequent
recovery must address JSON delivery and use a separately authorized finite pilot.

## Structured-output recovery — 2026-09-09

The user authorized schema-constrained file analysis and one fresh six-request
local pilot after v11's invalid JSON escape. The new candidate is
`qwen38-v12-structured-1`, prompt/cache identity `file-analysis-v12`, model
`qwen/qwen3.8-27b`, and provider label `configured-bug`. The new append-only grant
`authqual05-qwen38-structured-1` extends the development ceiling from 24 to 30 only at
24 consumed; qualification stays separately capped at 24. Prior grants and
failed receipts remain historical evidence and must not be replayed.

Production file analysis and evaluation send the same strict
`response_format: json_schema` contract. Other chat/edit calls keep their normal
format. The schema constrains the parent, optional four-field insights, risks,
suggestions and task-spec shapes. Target identity, aggregate insight length and
semantic correctness still require local parsing/scoring. No returned JSON is
repaired and unsupported structured requests do not fall back to ordinary chat.
The response-format identity binds resumable evaluation runs; older cached file
analysis is stale. Providers used for selected-file analysis must support the
schema request. [LM Studio documents this interface](https://lmstudio.ai/docs/developer/openai-compat/structured-output).

Offline verification used the installed Qwen tokenizer and the actual MLX VLM
schema compiler (`llguidance` 1.7.6 / `mlx-vlm` 0.6.5), with no weights loaded,
network access or generation. The final schema SHA-256 was
`579712bc61574034acd3fbbfe3bacc0bb5421bcd16d693abedc04ee158c1fa88`.
All ten synthetic checks passed without compiler warnings: valid omission,
null/full insights and escaped action strings were accepted; scalar, partial,
empty and extra-field insights, extra parent fields and invalid action escapes
were rejected. This verifies local grammar behavior, not live reasoning or
content quality. The frozen manifest binds the compiler/runtime files as well
as model artifacts, configuration, prompt/schema and corpus.

The pilot uses `qual05-qwen38-v12-dev1` and `qual05-qwen38-v12-dev2` at one clean
base, the same three development cases twice. Preserve low thinking, temperature
1, top-p 0.95, top-k 20, min-p 0, presence penalty 0, repeat penalty 1, application
input budget 16,384, actual runtime context 119,552, parallelism 1, no speculative
draft, 4,096 completion tokens including reasoning, and 300 seconds per attempt.
Verify runtime/manifest before each batch. Collect both before scoring, with no
probes, retries or tuning; stop on incompatible runtime, drift or uncertainty.
Require 6/6 usable and complete, 4/4 substantive >=6/8, both intentional control
omissions and zero critical false claims. Only a passing reviewed pilot can
promote a qualification candidate and resume the paused scheduler for QUAL-06.

Code `45cc31c6fae87f41370c2f0c3f0180d6e67fc294` passed fresh independent review
and the full coordinator headless `make check`: Go tests, race, vet, formatting,
dispatcher checks and all 327 Desktop tests (zero failures/errors/skips). Focused
Go tests and Go quality also passed. The first review required a durable stop on
permanent structured-request rejection; one Terra repair added that stop,
counted/exportable failure evidence and zero-call resume behavior. Both
production and evaluation preserve ordinary malformed-content accounting.

The first full gate had one assertion failure in the unchanged Desktop test
`explicitSendMakesOneChatRequestAndPreservesFunctionRemoteConsent`; it passed a
focused rerun without source changes, then the entire final suite passed. The
original failed gate remains in private diagnostics. Headless/offscreen tests
are not native-window acceptance. No Desktop source or production parser was
changed by this recovery.

The six-request grant has been applied once under the reviewed identifier
`authqual05-qwen38-structured-1`; campaign usage remains 24 development /
0 qualification before collection. A coordinator draft initially omitted the
`auth` prefix; that CLI invocation was rejected before any ledger change or
request consumption. The final candidate manifest SHA-256 is
`9d4261b8ec84a1296ee2247269385127bad5e09d8f3dd7ae77e5cffda3f77f64`.
The scheduler remains Paused until this pilot's reviewed verdict. Exact commands
and progress are retained in
`.mini-orca/autopilot/coordinator/QUAL-05-structured-recovery.json`.


## V12 structured pilot verdict — 2026-09-09

**QUAL-05 failed; the scheduler stays Paused.** Both batches completed at frozen
base `8fffeebc7394d4a0e5b746a2b4bbd5a3781bc259`, after both runtime/manifest
preflights passed. All six were malformed, unusable and incomplete, with normal
`stop` completion, valid usage metadata and no timeout. Official coverage is
**0/6 usable/complete, 0/4 useful substantive insights and 0/2 successful control
omissions**. A missing final control answer cannot count as intentional omission.

| Batch | Case | Completion tokens | Latency | Final content |
| --- | --- | ---: | ---: | --- |
| dev1 | locking | 544 | 43.022 s | empty |
| dev1 | allocation | 422 | 30.941 s | empty |
| dev1 | control | 73 | 5.762 s | empty |
| dev2 | locking | 541 | 41.683 s | empty |
| dev2 | allocation | 514 | 38.118 s | empty |
| dev2 | control | 74 | 5.777 s | empty |

This pilot consumed **six requests and 2,168 completion tokens including
reasoning**. Median latency was 34.530 seconds; maximum 43.022 seconds. Campaign
usage is **30 development / 0 qualification**, with no remaining development
slots. All 24 qualification slots remain untouched and conditional on a passing
pilot. No new request, retry or grant followed the failure.

The coordinator matched every retained response digest to its original LM Studio
completion envelope. All six envelopes have empty `content` and nonempty
`reasoning_content`; retained bytes match the reasoning channel exactly. The
source-free channel audit is
`.mini-orca/autopilot/engineering-insight-evaluation/qual05-qwen38-v12-scoring/channel-audit.json`.
This establishes a delivery failure rather than a timeout or JSON escape failure.
Reasoning-only material is diagnostic and cannot be substituted for final output.

Installed runtime inspection supports the mechanism: the `mlx_engine.generate`
BatchedVision branch immediately appends `build_json_schema_logits_processor`
when a schema is supplied. It does not use the installed thinking-aware wrapper,
while the verified low-thinking template ends in an open `<think>` channel.
This suggests the schema constrains reasoning before a final answer starts.
The offline grammar checks were valid but did not exercise this channel boundary.

The next proposed recovery is a **new explicitly thinking-disabled model profile**
with the same schema and a separately authorized bounded pilot. Read-only
`applyPromptTemplate` checks show the model's explicit `enableThinking=false`
override closes the reasoning channel before generation. The installed API
`none` mapping produces a generic false flag, but skips the model's reasoning
level field with a warning; testing that generic flag alone did not produce the
required template. Do not assume `reasoning_effort: none` is sufficient: verify
the effective model template and all other frozen settings before dispatch.
Private source-free mapping/template proofs are in the coordinator directory.
No saved model/runtime preference or frozen candidate setting was changed.


Fresh independent Terra scoring confirmed all six whole-response digest joins
and schema-valid diagnostic JSON. Official dimension scores are **0/0/0/0** for
every attempt. Diagnostic reasoning-only scores, in correctness / local relevance /
trade-off clarity / useful verification order, were locking **2/2/1/2** and
**2/1/1/2**, allocation **2/2/2/2** and **2/2/1/2**; both diagnostic controls
intentionally omitted insights. No critical false claim was found in the retained
diagnostic material. These observations do not provide final-output coverage or
prove that disabling thinking will preserve this quality.

The strict score maps and source-free `audit.json` are in the same private
scoring directory. The coordinator stored both digest-bound score maps, passed
both offline receipt validators with exact frozen identities, then discarded
all six evaluator response files and verified their absence. No example was
retained. Existing LM Studio server logs were read in place for channel diagnosis;
this evaluator cleanup does not purge the host application's log retention.
Both validated receipt reports remain under `.mini-orca/autopilot/diagnostics/`.

The accepted implementation and full prepilot `make check` remain recorded above.
No code changed during collection or diagnosis. This failed profile is not a
promoted qualification candidate; QUAL-06 remains Pending and REL-01 Blocked.


## Thinking-disabled recovery — 2026-09-09

The user authorized exactly six additional development requests for a fresh
thinking-disabled candidate. The candidate is `qwen38-v12-thinking-off-1`, using
`qwen/qwen3.8-27b`, provider `configured-bug`, unchanged `file-analysis-v12` and
unchanged strict response schema. Grant `qual05-qwen38-thinking-off-1` appends six
slots only at 30 consumed; the development ceiling becomes 36, qualification
remains 24. The fifth ledger entry requires reasoning effort `none` and exact
candidate/model/prompt plus loopback dispatch. All earlier results remain intact.

The coordinator switched the loaded model's **Enable Thinking** off through
LM Studio's server inference settings. Read-only base configuration confirms
`ext.virtualModel.customField.qwen.qwen3.827b.enableThinking=false`; this is the
only base-setting change from the v12 pilot. Load configuration, runtime context
119552, application input budget 16384, single lane, model/runtime artifacts and
sampling settings remain unchanged. The private evaluation bug profile changes
only `reasoning_effort: low` to `none`. This is a server-session override, not an
edited model artifact or saved default; runtime/base drift blocks dispatch.

**Diagnostic correction:** the prior generic-flag probe used an unqualified
configuration key and did not establish what the actual request does. The
correct fully qualified key is `llm.prediction.reasoning.enableThinking`.
The corrected readback merges the effective server base with the mapped request
flag, and verifies that both base and request templates close the thinking
channel before generation. An additional isolated readback removes both thinking
fields from the base, then changes only the full generic boolean: true opens
the channel and false closes it. This addresses the independent reviewer's
concern that the explicit custom field could mask the generic flag's effect. The installed `none` mapping still warns that `off`
is not a reasoning-level enum, but supplies the separate boolean thinking flag.
This warning does not establish that the boolean is ineffective. These are
read-only template checks, not generation or proof of content quality.

Collect the three development cases twice, without tuning, automatic retry or
additional probes. Recheck the frozen manifest and runtime before each batch;
use 4096 maximum completion tokens and 300 seconds per request. A fresh independent
scorer must inspect every emitted response and final answer, join exact digests,
and require 6/6 usable/complete, 4/4 substantive >=6/8, 2/2 intentional control
omissions and zero critical false claims. Reasoning-only output cannot be counted
as final output. Validate receipts offline and discard evaluator response files
when scored, retaining only eligible examples and source-free evidence.

The scheduler stays Paused during collection. A failed pilot leaves QUAL-05
Blocked with no further development grant or live retry. Only a passing reviewed
pilot promotes a frozen candidate and resumes the existing 24-request QUAL-06
schedule; no release is authorized.

Accepted code: `598f516c019065769e826050a5181fb5555b4f73`. Independent
code review and focused tests passed; coordinator headless `make check` passed
all Go, race, vet, formatting, dispatcher and 327 Desktop tests, with zero
failures/errors/skips. No schema, analysis prompt or production output parser
changed. The fifth grant has been applied once; usage remains 30 development /
0 qualification before collection. Candidate manifest SHA-256:
`2c61bdbfdb6b3edded2f1c685d5900b280642f12fa437bf3c1e8116b9e9a47ed`. Exact commands,
preflights, validation and progress are retained in
`.mini-orca/autopilot/coordinator/QUAL-05-thinking-off.json`.


## Thinking-disabled pilot verdict — 2026-09-09

**QUAL-05 remains Blocked; the scheduler stays Paused.** Both batches completed
at frozen base `157bdbe0c187826d288b4e137d3036c675b6f9b0`, following successful
per-batch runtime/manifest verification. All **6/6 final answers were usable and
complete**, with normal stop, valid usage metadata and no timeout. Exact
whole-response digest joins to the original LM Studio completion envelopes
confirm nonempty `content` and empty reasoning channels in all six responses.
The earlier empty-final failure did not recur with this frozen candidate.

| Batch | Case | Insight state | Completion tokens | Latency |
| --- | --- | --- | ---: | ---: |
| dev1 | locking | omitted | 260 | 19.594 s |
| dev1 | allocation | present | 459 | 29.233 s |
| dev1 | control | omitted | 237 | 16.267 s |
| dev2 | locking | omitted | 464 | 27.786 s |
| dev2 | allocation | omitted | 230 | 13.722 s |
| dev2 | control | omitted | 73 | 4.344 s |

The pilot consumed exactly **six requests and 1,723 completion tokens**. Median
latency was 17.931 seconds and maximum 29.233 seconds. All **36 development
requests are consumed / 0 qualification**. There were no additional probes,
retries, tuning or model calls after collection. The 24 qualification slots
remain untouched and conditional on a passing development pilot.

Insight coverage fails independently of delivery: only one of four substantive
attempts contains an insight. Both locking repetitions and the second allocation
repetition omit it. The frozen quality threshold requires all four substantive
attempts to provide eligible insights scoring at least 6/8. Usable parent summaries
cannot substitute for those missing insights, and control cases must still omit
insights intentionally.


Fresh independent Terra scoring reviewed all six delivered final summaries and
verified all whole-response digest joins. The first allocation insight scores
**2/2/1/2 = 7/8** for correctness / local relevance / trade-off clarity / useful
verification. It identifies locally relevant slice growth and useful allocation
measurement, but incompletely describes the allocation trade-offs. Both locking
attempts and the second allocation attempt score **0/0/0/0** because no insight
was delivered. Both controls intentionally omit insights and score **2/2/2/2**
under their omission rubric. No critical false claims were found in any final
summary. Final gates: **6/6 usable, 6/6 complete, 1/4 useful substantive,
2/2 intentional controls, zero critical false claims**. The substantive gate
fails; no best-of selection or threshold relaxation is applied.

The coordinator stored both strict digest-bound score maps and passed both
offline receipt validators with the exact frozen identities. All six evaluator
response files were then discarded and their absence verified. No prose example
was retained. Source-free receipts, audits and validator reports remain; existing
LM Studio logs were inspected in place for channel joins and were not purged.

The channel audit and independently validated score maps/audit are retained under
`.mini-orca/autopilot/engineering-insight-evaluation/qual05-qwen38-thinking-off-scoring/`.
The accepted grant code and full validation are unchanged from the prepilot
record above; successful code validation does not qualify model insight quality.
The model's explicit thinking-off setting remains in the current LM Studio server
session. It is not a saved model-default change; a restart or effective-config
change must be detected by manifest verification before any future dispatch.
No qualification candidate was promoted and the scheduler must remain Paused.

## Thinking with final-schema recovery preparation — 2026-09-09

The user's “go for it” authorizes the four-stage specification in
[retained REC-04–07 specification](history/improvement-plan-2026-09.md#rec-04--establish-standalone-runtime-compatibility), registered as REC-04–07 and continuations of QUAL-05,
QUAL-06 and REL-01. Exactly six additional development requests are authorized;
the new grant is unapplied. Campaign consumption remains **36 development / 0
qualification** and the scheduler remains **Paused**. No deployment or publication
is authorized.

The read-only audit rejected bundled MLX-VLM 0.6.5 as the standalone candidate:
its active sampler ignores requested `top_k=20`, it lacks a hard active-sequence
limit, and its server import fails on missing `mlx_audio`. Its route also accepts
but does not use `reasoning_effort` and `repeat_penalty`. These are runtime
compatibility findings, not model-quality evidence.

The coordinator explicitly selected published MLX-VLM **0.7.0** for independent
compatibility review. Its wheel SHA-256 is
`5ea0c2b8182c055068cb0da59cb2503c238c428a07daaa0bebc246e8c666f4d3`.
The published package includes top-k sampling, a maximum-active-sequence setting
and template forwarding for reasoning effort. It requires MLX/MLX-Metal 0.32.2;
a dry-run resolver produced a 54-package dependency inventory without installation.
Use an isolated environment, preserving the existing LM Studio installation and
model artifacts. This selection does not establish runtime readiness or a
passing development pilot.

Private source-free evidence is under
`.mini-orca/autopilot/engineering-insight-evaluation/qwen38-v12-thinking-schema-1/`;
`runtime-compatibility.json` records the selected route and
`dependency-resolution.json` records exact distribution hashes. A fresh reviewer
accepted REC-04 audit SHA-256
`2b4bef8b6d05aab0d2acce2e852ccdc5a93bc1ec8994f131cb9160da058751c9`.
Coordinator source/hash checks, diff formatting and the ledger parser passed.
REC-05/06 own installation, lifecycle and offline conformance gates before any
provider request; runtime readiness remains unverified.

### REC-05 lifecycle acceptance

Reviewed code `220e56ff41e25d0179309fe93218f010126cbdbb` adds the explicit
`check`, `start` and `stop` launcher and the 54-distribution hash lock. The
isolated environment imports successfully; its server help and dependency
checks pass. The integrated read-only preflight verifies the reviewed runtime,
current model-file hashes, model link, configuration and free loopback port.

Fresh review accepted final diff
`f6d9889e1de07384d16b78f7c7c0ad958301e90c2e427093e87f372f9b6c49d4`
after two Terra repairs. Seventeen focused tests cover lifecycle ownership,
concurrent starts, failure/interrupt cleanup, direct health transport/listener
ownership, configuration drift and writable-path containment. Coordinator
`make check` passed, including 327 desktop tests with no failures. A real
ordinary child-process check independently confirmed termination and reaping;
it did not start the model server.

No model loading, server startup or generation has occurred in this recovery.
REC-06 still owns the actual request/schema and channel conformance proof. A
preliminary in-process ASGI test confirmed HTTP 422 validation rejection with
zero model access; the client must classify it as permanent before the pilot.
Campaign consumption remains **36 development / 0 qualification**; the new grant
is unapplied and the scheduler is Paused.

### REC-06 offline conformance acceptance

Reviewed code `4b6fd63407863126f12e809ab3f2202a427abb24` classifies the
installed backend's HTTP 422 validation response as permanent. Fake HTTP proves
one durable reservation, immediate termination and no replay; ordinary chat
retains its separate rejection type and provider body privacy.

After one Terra repair, fresh review accepted exact diff
`b34f4f2c7bb606d095fc28169bf8ed678c7d61ced91919af3adb65f678cbec21`.
The 18 explicit offline runtime tests passed without skips; eight private
preflight tests and full coordinator `make check` also passed, including 327
desktop tests. The portable suite skips only the explicitly opt-in installed
runtime check; that check was run separately against the pinned environment.

Conformance imports the installed runtime, verifies module identities, compiles
the production schema, exercises its thinking-aware masks and reset, and checks
the actual local low-effort template, top-k mask, one-lane admission and raw
4096-token stopping boundary. The actual in-process ASGI route separates reasoning
from final content and preserves length termination. Reported completion usage
includes reasoning text but subtracts recognized thinking delimiters; raw timing
and metrics include those delimiters. Keep those token totals distinct.

The coordinator also verified all 19,514 installed runtime files and all nine
model-artifact files. A synthetic low-effort template render has SHA-256
`2742d9d57edd31554803dc27273d278fb0f64bbf80492ad6ca7423c96690b791`.
No weights, live generation or server startup were used. Two early failed
synthetic engine-test processes were reaped; the opt-in test now has a bounded
30-second timeout. These offline results establish compatibility, not model
quality. Campaign remains **36 development / 0 qualification**, the sixth grant
is unapplied, and the scheduler remains Paused while REC-07 is prepared.

### REC-07 grant acceptance and pilot preparation

Reviewed code `a72fea8415c88244dcd61631c52cfde542e48102` adds only the sixth
fixed grant for `qual05-qwen38-thinking-schema-1`, candidate
`qwen38-v12-thinking-schema-1`, wire model
`./models/qwen38-v12-thinking-schema-1`, unchanged v12 and low reasoning effort.
Fake HTTP confirms exactly six additional calls after 36 consumed, with rejection
of request 43, a seventh/duplicate grant, wrong identity, remote dispatch,
corrupt history and replay. Qualification remains capped at 24.

After one Terra complexity repair, fresh review accepted diff
`2237701ddf0dbdb4246e8d5dc40b035c1821ce93993554784aab2cb394f38718`.
Coordinator `make check` passed with 327 desktop tests, and `make quality` passed
static analysis, reachability, complexity, clone detection and desktop checks.
The initial quality run also exposed a coordinator cache-copy symlink issue;
the isolated cache copy was corrected and the unchanged clone check then passed.
Four synthetic collector tests and fresh review cover fixed batches, fail-closed
preflight, no replay and preservation of the child PID on uncertain cleanup.

The coordinator applied the grant once without generating. The original five
grants and all 12 historical receipt hashes remain intact. Consumption is still
**36 development / 0 qualification**, with a development ceiling of **42**.
The loopback runtime handoff and frozen pilot are next; the scheduler is Paused.
These preparation results do not establish insight quality.

### Thinking-schema pilot freeze

The original LM Studio lane was verified idle, with context 119,552, one parallel
session and its unchanged thinking-off server-session template. It was unloaded
before starting the dedicated owned MLX-VLM process on `127.0.0.1:1235`. No saved
model defaults or artifact files changed. The first handoff check stopped because
read-only template formatting updated a usage timestamp; all stable settings
matched, and the launcher correctly refused to load a second resident model.
The subsequent verified handoff completed without generation.

Frozen manifest SHA-256
`a9c96b5aa3f37213874a04cecf08aa84b21778b1084db84ad4c9e295d047077b`
binds accepted code `a72fea8415c88244dcd61631c52cfde542e48102` and the exact
runtime/configuration/model/proof files. Production configuration loading and
profile resolution confirmed low effort, temperature 1, top-p 0.95, top-k 20,
4096 completion tokens and input budget 16,384, with unsupported neutral fields
omitted. Owned-listener health, empty queues, one lane and all-zero request/raw
and reported token counters passed the full ready preflight. No smoke inference
was performed. The fixed pilot run IDs are
`qual05-qwen38-thinking-schema-dev1` and `qual05-qwen38-thinking-schema-dev2`.

### Thinking-schema pilot verdict — 2026-09-09

**QUAL-05 remains Blocked; the scheduler remains Paused.** At frozen base
`23c5cebc84bfe8b86dcb9a5a8bc406da572932fa`, the two fixed batches above consumed
exactly six additional requests, with no retry, error, timeout or extra probe.
All six responses delivered usable, complete final JSON with normal stops.

| Development case | dev1 | dev2 | Whole-final-content critical claims |
| --- | --- | --- | --- |
| Locking/cancellation | 2 / 2 / 1 / 1 = 6/8 | 2 / 2 / 1 / 1 = 6/8 | Both final summaries falsely guarantee FIFO mutex service. |
| Allocation | 2 / 2 / 2 / 2 = 8/8 | 2 / 2 / 2 / 2 = 8/8 | None |
| Trivial control | 8/8 intentional omission | 8/8 intentional omission | None |

All four substantive attempts meet the numeric >=6/8 threshold. The two critical
false claims independently fail the zero-critical-claim gate. The runtime repair
establishes final delivery for this candidate, but the content-quality gate still
fails. No candidate is promoted and no qualification base is frozen.

Runtime accounting shows six started/completed requests, zero failed requests,
and **7,592 raw generated / 7,592 reported completion tokens**. Successful latency
median is **97,753ms** and nearest-rank p95 is **103,060ms**, with no timeout or
censored attempts. Corresponding repetitions are byte-identical: only three
outputs are distinct. This is development evidence, not an independent repeated
reliability estimate. Raw/reported token equality is not a reasoning-token count.

A fresh scorer and second independent reviewer checked the delivered final
summaries against the development fixtures and confirmed all six SHA-256 joins.
Private response material concatenates reasoning aliases and final content; it
is not an HTTP envelope. The duplicated reasoning aliases were not credited as
final answers. Both scored receipts passed offline identity/schema validation;
receipt integrity does not imply passing quality. All twelve earlier receipt
hashes and the earlier grant history remain unchanged.

After both reviews, the two private response directories were discarded and
absence verified. No examples were retained. Source-free scoring, channel audit,
receipts and runtime accounting remain in the private evaluation directory and
`.mini-orca/autopilot/coordinator/QUAL-05-thinking-schema.json`. The separate
private runtime log remains retained (1,719 bytes, SHA-256
`69450d792bcdcc835202c3360769033d7ee6d719bd0f808dc06b5b2b8319b8ed`).

The owned standalone runtime was stopped and port 1235 released. The original
LM Studio model was reloaded with context 119,552 and one lane, then its
server-session thinking-off setting restored and read back. All stable settings
match the prior session; only the new instance reference and usage timestamp
are excluded from comparison. Restored effective-template SHA-256 is
`0dab809e785300d543255f76649e8124b5e8ec15df4ecdd837898936f17490fe`.
Saved defaults and model artifacts remain unchanged; the frozen pilot manifest
is retained unchanged as historical evidence, not current runtime readiness.

REC-04–07 are Complete. All **42 development requests are consumed; qualification
usage remains zero**, with its 24 slots untouched. QUAL-06 remains Pending and
REL-01 Blocked: their conditional execution gate did not pass. The scheduler
handoff records this result and stays Paused. No further grant, tuning, replay or
model generation is authorized by this recovery. Further content-quality recovery
requires a new bounded authorization. Accepted code passed `make check` (including
327 desktop tests) and `make quality`; this final evidence update changes docs only.

### Medium-reasoning pilot preparation — 2026-09-09

The user authorized six additional development calls changing only Qwen3.8
reasoning effort from low to medium. Accepted code `5ca2320` binds seventh grant
`qual05-qwen38-medium-1` to candidate `qwen38-v12-medium-1`, wire identifier
`./models/qwen38-v12-medium-1`, v12 and medium. All six earlier grants and
historical receipts remain intact. Consumption is 42 development / 0 qualification,
with a new development ceiling of 48. No additional model experiment or
qualification run is included in this request.

The accepted implementation passed fresh independent review, coordinator
`make check` (327 desktop tests), `make quality`, and 19 explicit installed-runtime
medium conformance tests. Historical low conformance also passed 19 tests. The
first review found complexity and template-proof gaps, repaired in the first Terra
retry. Subsequent corrections changed documentation only; source/test trees remain
identical to the full validated candidate. Private verifier and collector passed
8 and 4 synthetic tests and independent review.

The original idle LM Studio model and thinking-off settings were recorded, then
unloaded before starting the owned standalone runtime. Same artifact hashes,
54 dependency pins, 19,514 runtime-file inventory and fixed limits were verified.
Actual local tokenization distinguishes low, medium and xhigh; medium template
SHA-256 is `494e280281307944033f74025f48cddd84b0d3d0a1d756842e70b861b6f6641b`.
The installed request route preserves medium effort and the thinking/final-schema
boundary. No weights were downloaded or generation probes made.

Frozen manifest SHA-256
`7782ff7b5005675b198a286a94b807e26b277c7b6920e4225a308676289b551f`
binds the reviewed code, new configuration, proofs and owned process. Readiness
passed with zero requests/tokens before the two fixed runs
`qual05-qwen38-medium-dev1` and `qual05-qwen38-medium-dev2`. Both batches use the
same frozen candidate, with no tuning or retries, 4,096 total completion tokens
and 300 seconds per attempt. Independent scoring and whole-final-content review
must precede verdict and response disposal. Scheduler stays Paused.

### Medium-reasoning pilot verdict — 2026-09-09

**Development gate passed; qualification has not run.** Frozen pilot base
`d1263c6424cbd89d8fd97584ceeab270edd4c4b5` and the preparation manifest above
were unchanged across both three-case runs. All six attempts produced usable,
complete final JSON with normal stops; no retries, tuning, timeout, output
exhaustion, runtime failure or cleanup uncertainty occurred.

| Case | First run | Second run | Whole-final critical false claims |
| --- | --- | --- | --- |
| Locking | 7/8 (2/2/1/2) | 7/8 (2/2/1/2) | 0 |
| Allocation | 7/8 (2/2/1/2) | 7/8 (2/2/1/2) | 0 |
| Trivial control | Insight omitted | Insight omitted | 0 |

The substantive insights identify the local mechanism and useful verification,
but miss part of the required trade-off qualification. Both controls score 2/2/2/2
for intentional omission. Scorer and independent reviewer initially overclassified
conditional control advice as a critical false claim. Re-adjudication against the
unchanged frozen anchor distinguished a hypothetical caller expectation from an
assertion that such downstream behavior exists. Both reviewers and coordinator
agree the advice is unsupported and low-value, but not a critical false factual
claim. Initial scores and the correction history are retained privately; no rubric,
prompt or model change was made to obtain this verdict. Only delivered final JSON
was scored; reasoning received no credit.

Raw generated and reported completion totals both equal **9,354 tokens**.
Median latency is **115,054.5 ms**; nearest-rank p95 is **163,879 ms**. All three
cross-batch pairs are byte-identical: six attempts contain only **three distinct
outputs**, so this is candidate-selection evidence, not independent reliability
or release qualification. All four substantive attempts meet the 6/8 threshold,
both optional insights are intentionally omitted on controls, and all six whole
final summaries pass the separate zero-critical-claim gate.

Exactly seven grants now account for **48 development requests consumed and zero
qualification requests**. Historical grants/receipts remain unchanged. QUAL-05 is
Complete and candidate `qwen38-v12-medium-1` is eligible for subsequent QUAL-06;
all 24 qualification slots remain untouched. QUAL-06 stays Pending, REL-01 Blocked,
and the scheduler Paused because this authorization covers only six development
calls. No further experiment or qualification was dispatched.

The owned standalone runtime was stopped. The original LM Studio lane was restored
idle at context 119,552, one parallel session and thinking off. Read-only comparison
matches the original baseline except process-instance identity and last-used time;
the original template hash matches. The standalone host log remains private.
Frozen readiness is historical and does not authorize reuse of a stopped host.

Accepted code passed `make check` (327 desktop tests), `make quality`, 19 medium
and 19 historical-low runtime conformance tests, plus private verifier/collector
tests. The verdict update is documentation only. Source-free scored receipts,
channel digests and adjudication audit are retained under the private evaluation
root; exact score/validation commands and results are in
`.mini-orca/autopilot/coordinator/QUAL-05-medium.json`.
Following independent score comparison and final evidence review, raw response
material was disposed; source-free score, digest, receipt, and accounting records
remain retained.

### Medium-reasoning qualification verdict — 2026-09-09

**QUAL-06 failed.** After explicit scheduler-continuation authorization, the
coordinator ran `qual06-qwen38-medium-1` on clean base
`d9c98ca9af47705482833bd5033b962e79fd411e`, a documentation-only transition from
the accepted pilot. Candidate `qwen38-v12-medium-1`, v12 prompt/schema/corpus,
model artifacts, dependency pins, medium effort and all request limits remained
unchanged. Twelve qualification cases ran twice, with no tuning, generation
probes, retries, code changes or replacement runs.

The original pilot manifest remains immutable. Separate qualification manifest
SHA-256 `397c30d3d684443d272ef42329e1710f6b1d5b07e9966d1fb038600bb6405031`
binds that historical identity, the qualification base, exact 24-case schedule,
command and newly owned runtime. Independent pre-dispatch review caught and fixed
a private collector command-binding gap before any request: runtime verification
and dispatch now both bind the exact medium identity. Full offline inventory and
zero-request readiness passed; postcollection readiness accounted for exactly 24
completed requests, with zero failures, active work or queue depth.

| Qualification gate | Observed | Required | Result |
| --- | --- | --- | --- |
| Usable summaries | 24/24 | At least 23/24 | Pass |
| Fully complete summaries | 22/24 | At least 22/24 | Pass |
| Useful substantive insights | 2/16 | At least 13/16 | Fail |
| Intentional control omissions | 8/8 | 8/8 | Pass |
| Whole-final critical false claims | 2 | 0 | Fail |

Ten substantive attempts omitted insight, two had rejected optional sections,
and four retained insights. Repeated-I/O scores **5/8 (2/2/1/0)** twice;
idempotency scores **6/8 (2/2/2/0)** twice. The verification suggestions expect
proposed behavior rather than checking the current contract. The other twelve
substantive attempts receive zero insight credit. Both backpressure finals
categorically describe successful sending as receiver delivery/readiness, despite
unknown caller-supplied channel buffering. A send can complete into available
buffer space without a receiver observing the value. These are the two critical
claims; a non-nil return from the cancellation branch was not itself flagged.

A fresh agent scorer and separate fresh agent reviewer independently inspected
all actual terminal JSON, whole-file digests and boundaries before comparison.
They resolved repeated-I/O verification and the backpressure critical classification;
initial independent judgments and a source-free adjudication record remain private.
The final dimensions do not credit reasoning, generic summary text in place of
an absent insight, or rejected optional sections. Control omissions pass, while
generic test advice reduces the identity-control dimensions to 1/2/2/1. This is
agent scoring, not human testing. Both reviewers agree on the failed verdict.

All 24 attempts stopped normally. Raw generated tokens, runtime-reported
completion tokens and receipt output totals all equal **34,090**. Median latency
is **89,914 ms**; nearest-rank p95 is **178,788 ms**. No timeout, censored request,
output exhaustion, runtime failure or uncertain cleanup occurred. Every repetition
pair is byte-identical: there are only **12 distinct outputs**. Repetitions still
consume their reserved calls and do not establish independent reliability.

The accepted CLI applied the digest-bound scores. Offline receipt validation
returned `failed` with 24 usable, 22 complete, 2 useful substantive, 8 omitted
controls and 2 critical claims. The expected nonzero quality verdict is not a
receipt-integrity failure. Exact collection, score and validation commands/results
are retained in `.mini-orca/autopilot/coordinator/QUAL-06-medium.json`; the scored
receipt and audits remain under the private evaluation root. Fresh
`go test ./internal/app ./cmd/engineering-insight-eval -count=1` passed. No
production code changed; final documentation receives whitespace/ledger validation
and independent review.

The owned qualification runtime was stopped and the original LM Studio session
restored idle with context 119,552, one lane and thinking off. Stable readback
matches the captured baseline except process-instance identity and last-use time;
template hash matches and the dedicated runtime port is free. The private host
log is retained append-only, with the prior pilot prefix verified unchanged.

The campaign is exhausted at **48 development / 24 qualification requests**, with
seven grants and all historical receipts preserved. QUAL-05 remains Complete for
its development result; QUAL-06 is Blocked, REL-01 remains Blocked, and the
scheduler is Paused. No release step or additional model request ran. The bounded
next proposal is offline failure analysis and a recovery plan for missing insights,
rejected optional sections and incorrect delivery guarantees. Any new experiment
requires an explicit budget and fresh qualification evidence; tuning after seeing
these qualification cases cannot convert this completed failed run into a pass.

Following final independent evidence review, raw response material was disposed;
source-free scores, digests, receipts and adjudication records remain retained.

Maintained UI references are the [keyboard operator checklist](../desktop/KEYBOARD_SMOKE_CHECKLIST.md),
[current contrast measurements](../desktop/UI_CONTRAST.md), and the reproduction
and consolidated observations in this ledger. REL-02 retired the separate Task 170,
visual-history and precision-acceptance files after preserving their current evidence.

## V2 recovery offline preparation — 2026-09-09

The user approved RCV-01–08 with “go for it”: **twelve new development requests
and conditional twenty-four fresh qualification requests**, with no probes,
retries or further model/prompt experiment. The failed medium qualification above
remains unchanged. Historical consumption is 48 development /24 qualification;
the original campaign, seven grants and seventeen scored receipts are immutable.
No new local-model request has been issued during implementation.

RCV-01–05 are independently reviewed and integrated through `2a59918`. The
selected-file prompt now explains visible mechanisms and current-behavior tests
without pilot-specific lock/append paragraphs. Optional diagnostics retain only
reason enums, locations, presence and normalized rune counts, bound to response
digests; no source or generated prose enters them. All four generated insight
fields use a shared 250-character schema limit. The parser keeps its existing
1,000-normalized-rune aggregate limit and does not truncate rejected text.

The new fixed identity is `qwen38-v13-recovery-1`, prompt `file-analysis-v13`,
corpus/protocol `engineering-insight-v2`, successor period
`engineering-insight-v2-recovery-1`. Production schema SHA-256 is
`63a92e42e57607a002a0f4ca4fdb4ccc3080fe38293a72a0647c33181a4d1cba`.
Development corpus SHA-256 is
`2370397f665af4aefa0ab8db0dab7e0a0e961930b63594e0fc894fabed6a2431`;
separately authored/reviewed sealed qualification SHA-256 is
`7cb16ac34b337e172e97a14aa639ef00bec77fbc77085028c01e6e1fe4183229`.
The twelve development cases contain eight substantive cases and four controls;
the twenty-four held-out cases contain sixteen substantive cases and eight
controls. The tuning coordinator has not read qualification source or anchors.

The candidate uses an independent filesystem clone of the unchanged pinned
Python 3.11.16 / MLX-VLM 0.7.0 / MLX 0.32.2 / LLguidance 1.7.6 environment,
with the same local Qwen3.8-27B 4-bit model artifacts. The full retained runtime
file inventory matches; production profile resolution and tokenizer-only template
checks pass without loading weights. Medium thinking template digest remains
`494e280281307944033f74025f48cddd84b0d3d0a1d756842e70b861b6f6641b`.
Limits remain 4,096 total generated tokens, 300 seconds per attempt, 16,384 input,
119,552 runtime context, one lane, temperature 1, top-p .95 and top-k 20.
Default seed behavior is unchanged. Distinct cases and duplicate reporting avoid
claiming that repeated identical samples establish independent reliability.

RCV-06 offline checks passed: `make check` (including Go race/vet/dispatcher
checks and all 327 desktop tests), `make quality`, runtime unit discovery
(18 passed, one opt-in installed-runtime skip), and `git diff HEAD --check`.
The opt-in installed-runtime check was executed separately and passed with the
actual pinned interpreter and tokenizer, zero generation requests and no loaded
weights. It proves omission/null, exact 250/251 multibyte Unicode field limits
independently at every supported insight location, thinking-before-schema,
top-k, total completion cap and one-lane admission. The unchanged runtime emitted
only a Starlette TestClient deprecation warning; no dependencies were upgraded.

Review caught shared mutable objects in the per-location negative test; the
repaired test uses independent objects and passed installed conformance again.
The first full quality gate caught four unused historical test helpers from
RCV-05. They were removed in reviewed `2a59918`; both full gates then passed on
source diff `d7f2d8e4d327d8e3dc2c11a3970a2bcdb8251f769c5edd50e55bedbbc7e3d371`.
The final documentation/status update changes no validated code.

Exact installed conformance command, from the RCV-06 worktree:

```sh
PYTHONDONTWRITEBYTECODE=1 /Users/marcoandreose/DEV/lab/mini-orca/.mini-orca/autopilot/engineering-insight-evaluation/qwen38-v13-recovery-1/venv/bin/python scripts/tests/insight_runtime_conformance.py --candidate-dir /Users/marcoandreose/DEV/lab/mini-orca/.mini-orca/autopilot/engineering-insight-evaluation/qwen38-v13-recovery-1 --source-root /Users/marcoandreose/DEV/lab/mini-orca/.mini-orca/autopilot/worktrees/rcv-06
```

After integrating the reviewed documentation, the coordinator records that exact
clean HEAD in immutable private `offline-manifest.json`, with source, schema,
corpus, artifact, dependency, configuration and proof hashes and the fixed CLI
commands from [the retained evaluation instructions](history/insight-evaluation-2026-09.md#frozen-v2-successor-period). The successor authorization
must bind the same HEAD. A separate immutable readiness manifest adds the fresh
owned PID and zero-request readback before dispatch. Neither earlier manifests
nor the offline freeze are overwritten. Fresh owned-process readiness is a
separate gate; a stopped runtime's historical manifest does not establish it.
The scheduler remains Paused and REL-01 remains outside this recovery approval.

## V2 development screen verdict — 2026-09-09

**RCV-07 failed; RCV-08 did not run.** The reviewed candidate
`qwen38-v13-recovery-1` ran twelve distinct development cases once at clean base
`9da9cd96583175e84093de42f71907db5c480a5d`, with unchanged frozen prompt, schema,
corpus, model artifacts, runtime and medium settings. No generation probes,
retries, replacement run, mid-run tuning or qualification requests occurred.

Final pre-dispatch review found a missing enforced link between the private
runtime-ready and offline manifests. The coordinator added the digest/shared-field
join and zero-counter/owner checks, with nine passing synthetic preflight tests.
Independent review accepted it before dispatch. The rejected, unused envelopes
and verifier versions remain preserved in `pre-dispatch-rejected-1`; no public
application code or authorization identity changed. Final offline manifest SHA-256:
`f548a8848b137cde64de54ef8dc36a7c062ea922c417ab8c7fa76d9b10bcb525`.
Final ready manifest SHA-256:
`71afe69041f67bbc5424cdf687852ddbe4c2ff53856f4191ee89d4bcb17b1bc6`.
Fresh owned-process verification passed at zero requests before the first call.

| Development gate | Observed | Required | Result |
| --- | --- | --- | --- |
| Usable summaries | 12/12 | 12/12 | Pass |
| Fully complete summaries | 12/12 | 12/12 | Pass |
| Useful substantive insights | 2/8 | 8/8 at least 6/8 | Fail |
| Intentional control omissions | 0/4 | 4/4 | Fail |
| Whole-final critical false claims | 5 | 0 | Fail |

All twelve responses contain accepted insights. No optional field was rejected
or degraded, so the new schema/parser bounds worked for this sample. The
failure shifted to unwanted insights, missed case mechanisms, inadequate
current-behavior verification and incorrect factual claims. The different corpus
prevents interpreting these counts as a controlled improvement over v1.

| Substantive case | Correctness / relevance / trade-off / verification | Total | Critical claim |
| --- | --- | --- | --- |
| Snapshot read lock | 2 /2 /2 /1 | 7/8 | No |
| Cancellation before loop | 2 /2 /1 /0 | 5/8 | Yes |
| Filtered output capacity | 1 /1 /1 /1 | 4/8 | No |
| Bounded opaque refresh | 1 /2 /1 /2 | 6/8 | No |
| Local idempotency admission | 1 /2 /1 /0 | 4/8 | No |
| Read authorization boundary | 1 /1 /1 /1 | 4/8 | No |
| Delegated token verification | 1 /2 /0 /2 | 5/8 | Yes |
| Buffered send contrast | 1 /2 /0 /1 | 4/8 | Yes |

All four controls receive zero insight credit under their omission anchors.
Two also contain critical claims. The five critical classifications are: an
explicitly current cancellation test expectation opposite to the shown behavior;
unestablished security guarantees assigned to an opaque delegate; an incorrect
blocked-send cancellation limitation; a return-value claim for an invalid helper
invocation; and a constructor defect invented despite the correct assignment
being visible. Conditional concerns or clearly proposed tests alone were not
classified as critical. Several other insights are locally plausible but miss
the frozen allocation, authorization or retained-on-failure mechanisms.

A fresh agent scorer and independent fresh reviewer inspected every whole
terminal final and its accepted insight locations against the frozen sources and
anchors, verifying all twelve full-response digests. Both initial judgments were
preserved. They explicitly reconciled off-anchor lessons, proposed versus current
verification, omission-control credit, and critical claims; the final score maps
match exactly with no unresolved differences. Reasoning text received no credit.
This is agent evaluation, not human testing or a production reliability guarantee.

The accepted score/export CLI commands completed successfully. The exact offline
receipt-validator command returned `collection` with 12 usable, 12 complete,
2 useful substantive, 0 omitted controls and 5 critical claims (exit 0 means
collection integrity passed). A separate read-only invocation of the production
`DevelopmentGatePassed()` method returned **false**. The coordinator did not
attempt a qualification dispatch to demonstrate that rejection. Exact commands,
proof hashes and source-free adjudication remain in the private coordinator record.

Postcollection owned-runtime verification accounts for exactly twelve completed
requests, zero failures/active work/queued work and **21,804 raw and reported
completion tokens**, matching the receipt. All stops were normal; all twelve
outputs are distinct. Median latency was **117.55 seconds**; nearest-rank p95
was **163.64 seconds**, with zero timeout/censored attempts. Source-free rejection
diagnostics, scores, receipts and accounting remain retained. Following final independent evidence review, the reviewed discard CLI removed
raw development responses and the coordinator verified their absence.

The owned runtime was stopped. The original LM Studio model is restored idle
with context 119,552, one lane and thinking off. All stable readback fields and
the template digest match the captured baseline, excluding only process-instance
identity and last-use time. Fresh `go test ./internal/app
./cmd/engineering-insight-eval -count=1` and `git diff --check` passed; the accepted
RCV-06 full `make check` and `make quality` results remain applicable to unchanged
production code.

Successor consumption is **12 development /0 qualification**, cumulative
**60 development /24 qualification**. The original 48/24 campaign, all seven
grants and seventeen receipts remain unchanged. The fresh 24-case qualification
corpus remains sealed and its conditional requests unused. RCV-07 and RCV-08 are
Blocked, the scheduler remains Paused, and REL-01 remains Blocked. No further
model, prompt-only adjustment, budget grant or release action is authorized.
The next decision is a separately bounded model/task-design experiment; this
failed screen cannot be turned into a pass by retrying or changing its gates.

## FND-01 reconciliation

FND-01 started at `d0a7cc9`. The only pre-existing worktree edit was the coordinator's
`PLAN.md` status transition for FND-01; it is not part of the evidence change. The preserved Task
170 source/test record was already tracked in that baseline, while all rendered PNGs are ignored.
The current eight-fixture matrix, command, runtime identity, scale, artifact names, and native
limitation are recorded in [UI precision evidence](RELEASE_ACCEPTANCE.md#consolidated-ui-precision-evidence).

UI-04 closed the attainable native window, edge popup, operating-system focus and screen-reader
name/state inspection inherited from Tasks 149/170. REL-01 owns provider, lifecycle, package,
distribution and any release matrix combinations that require a different operator surface;
LEARN-02 owns insight usefulness; FND-02–04 and AUTO-01 own the quality findings.

## REL-01 execution — 2026-09-08

Initial integrated base: `bd727e1980cada41e8a5a6d01945143cbb07163c` (`4.4.0`),
clean before execution. The final repair validation below includes the uncommitted
REL-01 complexity repair. The host was macOS 26.6.2 (25G83), arm64, with Temurin
21.0.11+10 as Gradle launcher and ignored JBR `25.0.4+1-b508.27` as Desktop
toolchain. No user project, credential, or real remote provider was used.

| Evidence | Result |
| --- | --- |
| `make check` | Passed with the documented JDK 21/JBR 25 split: Go tests, race tests, vet, dispatcher tests, and Desktop tests. |
| `make quality` | Passed after the repair: staticcheck, deadcode, complexity, clone detection, Spotless, and Detekt all passed. |
| Desktop package | The repaired checkout's `./scripts/desktop-gradle.sh createDistributable` passed with JBR 25 and produced macOS arm64 `desktop/build/compose/binaries/main/app/Mini-Orca.app`, version 4.4.0, 159,968 KiB (~156 MiB). `Info.plist` identifies `io.miniorca.desktop`; the launcher SHA-256 is `229007d006b64fb1666943c0c77301e3b0e35b392d617ab9944721b9ad7ffde7`. |
| Bundle runtime and startup | The freshly generated embedded release manifest reports `JAVA_VERSION="25.0.4"` and lists `java.net.http` and `jdk.unsupported`. The freshly packaged macOS app launched with isolated Java preferences against the loopback daemon. The native chooser selected `desktop/build/rel-01/fixture.uOcgY4` and imported it. |
| Disposable fixture | A copied Git fixture and deterministic loopback fake provider completed replace (`Run`) and create (`ReleaseNote`) separately: import → draft → validation → explicitly trusted copied-workspace checks → explicit Apply → Undo. Parse, format, lint, and tests passed for both. Only `main.go` changed during Apply and Undo restored its original content. The ignored `desktop/build/rel-01/` directory contains sanitized disposable-fixture artifacts as well as copied fixture source; no separate source-free failed-check receipt is retained. |
| Native Security and Performance accessibility smoke | After import, accessibility inspection exposed named radio-button tool windows. Security with `main.go` selected exposed named Scan and Review controls plus source-only/advisory copy; Scan returned `Completed — no findings returned; this does not prove the file is secure.` With the local provider stopped, Review moved through Running to `Failed — Review the request and try again.` Performance showed `Source-based review · Not measured`, the local provider identity, and Preview limits of 10 selected and 0 excluded/oversized/outside. Its deterministic local run exposed named Pause, Resume, and Cancel controls: Pause showed `Paused · resume explicitly`; Resume showed 9 pending/1 running; Cancel ended at `Canceled · completed reviews remain available` and `Partial · canceled review retains completed files.` The changed Security and Performance surfaces remained readable without clipping in native 800×600 and 1305×768 windows. No external provider was used. |
| Docker | Docker Desktop CLI/server 29.3.1 and Compose 5.1.0 built `mini-orca:4.4.0` as image `sha256:502e36efe017669bbc083cb418fb436882c94a89490ac7d82d839408306e3f58`. A container using `config.example.yaml` reached `healthy`; its in-container `/health` response was `{"status":"ok","version":"4.4.0"}`. No cleanup was run. |

### Continued offline acceptance — 2026-09-08

The focused deterministic checks use `httptest` loopback providers and ephemeral
`t.TempDir` project roots. They make no external request. Separately, the ignored
native-run artifacts under `desktop/build/rel-01/` retain sanitized copied fixture
source; no tracked evidence retains fixture source, provider content, or credentials.

| Evidence | Result |
| --- | --- |
| Final candidate gates | A fresh `make check` and `make quality` passed on this uncommitted REL-01 candidate after the offline acceptance and Docker portability work. The check runs Go tests, race/vet, dispatcher tests, and Desktop tests; quality runs static analysis, reachability, complexity, clone detection, Spotless, and Detekt. |
| Source-safe failed checks, stale evidence, and cancellation | `go test -count=1 ./internal/app -run 'TestApplyDraftRejectsSourceChangedAfterChecksWithoutWriting|TestApplyDraftCancellationAndFailedChecksLeaveSourceUntouched|TestDraftValidateCheckApplyAndUndoRequireCurrentEvidence|TestTaskTestChecksRequireBaseFailureAndCandidatePassWithoutWritingProject|TestTaskTestChecksReportCandidateFailureAndUseNonConflictingFilename|TestTaskTestCheckCancellationAndEvidenceSanitization|TestRunCheckCommandHonorsCancellation'` passed. It verifies failed required checks and cancellation leave imported source unchanged, stale validate/check/apply evidence is rejected, and temporary proposed test files do not enter the source project. |
| Offline provider and local/remote/mixed scope consent | `go test -count=1 ./internal/api/handlers -run 'TestPromptHandlersConfirmOnlyTheirOwnRemoteScope|TestChatSessionEndpointReportsCancellationAndStaleSession|TestDraftEndpointsAreRevisionAndHashGuarded'` and `go test -count=1 ./internal/app -run 'TestScopedModelLocalMixedAndRemoteConfirmationMatrixUsesNoProvider|TestRemoteProviderRequiresConfirmation|TestReviewSecurityFileRejectsConsentAndStaleResultsWithoutPublishing|TestExplainDeclarationHandlesRefusedLoopbackProviderWithoutWorkflowMutation'` passed. The scope matrix uses local network aliases and denies confirmation before a request. The refused loopback test first closes its own listener, then verifies the request fails without source, draft, or session mutation. |
| Analyze-all lifecycle | `go test -count=1 ./internal/app -run 'TestAnalyzeAllProcessesEligibleFilesSequentially|TestStartAnalyzeAllCompletesEmptyEligibleSelection|TestAnalyzeAllRetriesFailedFiles|TestAnalyzeAllStopsRetryingAfterRetryBudget|TestAnalyzeAllCancelRetainsCompletedEntries|TestAnalyzeAllPersistsPausedJobAndResumesAfterServiceRestart|TestAnalyzeAllBecomesStaleWhenReindexChangesRevision|TestStartAnalyzeAllRequiresRemoteProviderConfirmation'` passed. It covers start, empty completion, transient failure retry, exhausted retry budget, cancellation with completed entries retained, restart/resume, stale revision, and denied remote consent. |
| Performance lifecycle and source identity | `go test -count=1 ./internal/app -run 'TestPerformanceJobPausesResumesAndDerivesCachedReport|TestPerformanceJobRejectsChangedPreviewAndConcurrentAnalyzeAll|TestPerformanceJobReportsFailedFileAndEmptyQueue|TestPerformanceReportsMarkStaleCachedCoverageWithoutChangingCompletedJob|TestPerformanceReportsPreserveLifecycleStatusWithStaleCoverage|TestRecoverPersistedPerformanceJobNormalizesInterruptedRequests|TestPerformanceJobRecoveryResumesInterruptedFile|TestReviewPerformanceFileRejectsChangedSourceOrPolicyBeforePublication|TestReviewPerformanceFileRechecksCancellationDuringPublicationAuthorization|TestReviewPerformanceFileCancellationPreventsPublication'` passed. It covers empty and failed queues, pause/resume/cancel with partial coverage, preview mismatch, stale reports, exhausted-budget recovery, restart recovery, and publication cancellation. |
| Exact-function handoff | `go test -count=1 ./internal/api/handlers -run 'TestChatSessionEndpointsKeepMessagesBoundToOneFile|TestChatSessionEndpointPinsTaskSpecsAndRejectsUnpreparedRepairs'` and `go test -count=1 ./internal/app -run 'TestAnalyzeFileValidatesAndCachesOneExactBugTaskWithoutAnotherModelCall|TestExplainDeclarationRequiresConsentAndRejectsIneligibleOrStaleTargetsWithoutProviderCall'` passed. The selected path, symbol, hash, and task specification stay pinned through the prepared handoff; unprepared repair, stale, ineligible, or denied requests do not send a provider request. |
| Fixture and corrupt-cache boundary | `go test -count=1 ./internal/project -run 'TestReleaseFixtureRepresentsPolicyAndParserAcceptanceCases|TestPerformanceCacheRecoversMalformedFile|TestPerformanceCacheIsStaleWhenSourceIsUnavailable'` passed. It confirms the fixture's secret placeholders are excluded, parser cases are isolated, corrupt cache is recovered, and unavailable source becomes stale. |
| Native Docker portability repair | The Dockerfile now fails before `go build` when either BuildKit `TARGETOS` or `TARGETARCH` is empty. Normal `docker build --progress=plain -t mini-orca:rel-01-native-r1 .` and `docker compose build mini-orca` passed, each logging `GOOS=linux GOARCH=arm64`. The explicit negative `docker build --progress=plain --build-arg TARGETOS= --build-arg TARGETARCH= -t mini-orca:rel-01-empty-target .` failed at the target guard with exit code 1. The current tag `mini-orca:rel-01-native-r1` has ID `sha256:2c90266ffab445dd0a95a43eb00977abff82be42a177459178098d0b8b8e0be5` and platform `linux/arm64`. The newly retained container `mini-orca-rel01-native-r1` has ID `aee4b1f41878b9552ad2204f5adce613446b764fc12568c1a0cbc427c8170bbd`, image ID `sha256:2c90266ffab445dd0a95a43eb00977abff82be42a177459178098d0b8b8e0be5`, platform `linux`, and is `running healthy`; its sole loopback endpoint `127.0.0.1:52120` returned `{"status":"ok","version":"4.4.0"}`. No Docker cleanup was run. |
| Live-provider compatibility and insight quality, 2026-09-08 | The normal loopback daemon API used the configured local OpenAI-compatible provider and `qwen/qwen3-coder-30b` model alias. Both derived configurations had mode 0600, no API key, local loopback scopes, 800 output-token caps, and one retry maximum. `analyze` was local, so no external data transfer or remote consent prompt occurred. The two runs persisted two `analyze` imports and five `bug` selected-file analyses. Source-free local provider request metadata recorded exactly seven chat-completion requests matching those operations, establishing the exact 7/8 budget and that no retry request occurred. The first run completed its import and cancellation analysis, while n-plus-one and idempotency stored the source-free parser failure `The model returned an unusable file summary.` The second completed its import plus fresh cancellation and idempotency analyses with `file-analysis-v5`. Those parser failures occur after successful transport and outside `s.retry`. The source-free receipt independently validated with `go run ./cmd/engineering-insight-eval -receipt /tmp/mini-orca-rel01-insight-receipt.json -cases internal/app/testdata/engineering-insight-eval/cases.json -provider configured-local-openai-compatible -model qwen/qwen3-coder-30b -prompt-version file-analysis-v5 -max-requests 8 -max-output-tokens 800`; it contains no provider reply, source, endpoint, or credential. The second-run cancellation/locking result scored 0/8 because it gave a generic mutex narrative rather than the lock-held blocking wait and a contending-lock verification. The idempotency result scored 4/8: it identified delegated charging and duplicate-risk context but omitted an idempotency trade-off and concrete retry verification. Neither result is retained; neither was scored as a critical false claim. |

The release scope is the tested macOS arm64 desktop package and the Linux arm64
container on this Docker Desktop host; no other desktop or container platform is
claimed. REL-01 remains blocked on model-quality qualification.

### Precision repair and larger-budget diagnostic — 2026-09-08

Provider finish metadata shows that the original failures were not truncation:
all seven replies ended with `stop` at 373–731 completion tokens. Both rejected
summaries invented parameter/field/type keys in `symbol_explanations`. The v6
prompt explicitly limits those keys to indexed declarations and asks for conditional,
source-grounded insights. The parser now omits the whole invalid optional explanation
map while preserving the useful parent; malformed JSON, invalid risks and target
validation remain strict. No invalid explanation becomes a navigable declaration.

A second diagnostic used the same local model, eight fixture cases, exactly one
request per case, 4,096 output tokens and a 300-second deadline. It exercised the
production prompt/client/parser with deterministic project facts; it did not repeat
import or desktop acceptance. All eight stopped normally at 345–528 completion
tokens; observed per-case latency was 4.2–15.8 seconds. Before optional-section
isolation, 3/8 responses parsed. Offline replay of the identical responses after the
repair produced 8/8 usable parent summaries: 3 complete and 5 degraded by omitted
explanations. No further live requests were used for replay. This is a repair check,
not a fresh 100% reliability result. Generic insights still appeared on trivial
controls, so larger output budgets alone did not resolve teaching quality.

Qualification now requires a preselected sample of at least 20 attempts, >=95%
usable first attempts, >=90% complete explanation sections, >=80% qualifying
insights on substantive cases, zero critical false claims, and omission on every
trivial control. Retained insights still require >=6/8. Record latency median/p95,
completion reasons, output tokens, degraded results and retries separately. The
current eight-case diagnostic does not meet that qualification requirement.

The opt-in collector makes eight local-only requests at 4,096 tokens, 300 seconds
each, with no retry. It uses the ignored root configuration in memory and writes
observations to ignored `.mini-orca/autopilot/insight-live-evaluation.json`; it never
writes credentials, endpoint URLs, source, or raw replies. Observations record only insight presence; evaluate prose separately in the local
provider/app session and retain only scores in a publication receipt. Normal test runs
skip it. A passing collector means collection completed, not that quality passed:

```sh
MINI_ORCA_LIVE_INSIGHT_EVAL=1 go test ./internal/app \
  -run '^TestEngineeringInsightLiveEvaluation$' -count=1 -timeout=45m -v
```

Validation after the precision repair: `make check`, `make quality`, focused
regression tests, and `git diff --check` passed. The live collector is skipped
by default; no provider request was made by those validation gates.

## Reproduce acceptance

Use a temporary copy of [the release fixture](../internal/project/testdata/release-fixture/README.md).
Keep one non-Git copy and a separate disposable Git fixture. Use `.fixture` parser,
vet and test failure inputs only in copies. Placeholder secrets must remain
excluded; never use a real user project or credentials in screenshots/test records.

1. Run `make check`, `make quality` and `git diff --check` from the integrated
   candidate with the documented [desktop runtime](../desktop/README.md#runtime-and-build).
   Record base/result identity, exact commands, failures and skipped stages.
2. Start from an empty desktop preference profile; open/cancel/retry, restore after
   restart, navigate workspaces/files/symbols and inspect the real relative path.
   Restore/navigation must not cause a model request.
3. Using a deterministic loopback fixture provider, replace `Run`, and separately
   create absent `ReleaseNote`: send, edit
   only the draft, validate, check, review, explicitly Apply and Undo. Verify one
   source file changes; stale selection/source/draft/check evidence blocks Apply.
4. Exercise failed required checks, explicit repair limits, cancellation, source
   changes during requests, denied remote confirmation and provider offline errors.
   Temporary proposed tests never appear in the imported project.
5. For the current scope, use deterministic loopback fixtures for scope routing and
   denied consent, and retain the bounded local request-contract evidence above.
   Live online/mixed-provider checks are future work requiring separate user
   authorization and an explicit finite request budget; do not run them for this closure.
6. Exercise Analyze-all and Performance preview/start/pause/resume/cancel/restart,
   honest empty/partial/budget-limited/stale/failed states and exact-function handoff.
   Insight usefulness review is on standby and is not a current acceptance step.
   Reopening it requires separate authorization; valid JSON is not evidence of quality.
7. Run native views at 1440×900, 1920×1080, 1000×760, 999×760, 800×650 and 1280×600;
   check 100/125/150% text and 1×/2× density where supported. Include long content,
   drawers, splitters, menus/dialogs at window edges, errors and populated Review.
   Use keyboard-only navigation and a supported reader; record observed focus,
   selected/expanded/disabled state, Escape, text selection and resize behavior.
8. Package with JBR 25 and smoke the actual supported artifacts. Verify runtime
   modules, startup and image contents. For Docker, build and verify `/health` and
   container health without cleanup commands. Do not claim untested platforms.
9. As new features land, add their concrete Security/trust/benchmark scenarios to
   this matrix. New feature evidence cannot be inferred from the earlier UI runs.

A release is accepted only when all required checks for its explicitly named scope
pass with evidence. Record operator/date, platform/runtime, fixture, candidate
identity, artifact locations and remaining limitations. Missing required evidence
keeps the release incomplete; independent implementation work can continue.

## Consolidated UI precision evidence

Retired from `desktop/UI_PRECISION_ACCEPTANCE.md` by REL-02. Dated attempts below
are historical observations; the final acceptance scope above is authoritative.

### Task 170 status

Superseded by UI-04. UI-04 closed the attainable inherited native and assistive-technology
checks on 2026-09-07 and records unsupported combinations below. This record keeps deterministic
production-component evidence separate from native evidence. An offscreen render is never used
as evidence for native menu, dialog, window-edge, or screen-reader behavior.

### Environment

- Host: macOS 26.6.2 (25G83), arm64.
- Desktop runtime: JBR 25.0.4+1-b508.27-nomod.
- Deterministic UI data: only the production test fixture (`go-shop · fixture`, `Visual fixture · no
  backend`) was rendered; no fixture capture includes a daemon, provider, user project,
  credential, or live provider data. The resumed UI-04 run below used a separate disposable native
  fixture and loopback daemon.

### FND-01 baseline reconciliation — 2026-09-06

The starting repository identity was `d0a7cc9` (`docs: establish Mini-Orca improvement
autopilot`). Before this task began, the coordinator had changed only the FND-01 status in
`PLAN.md` from Pending to Running. That status edit is not FND-01 evidence. The Task 170 test
source and retained execution record are tracked at that baseline; its generated PNGs are ignored
build output. This task adds no product or configuration behavior.

The current reproduction used the existing `DesktopVisualLayoutTest` production-component
renderer with local fixture data. It ran with JBR `25.0.4.1+1-583.48-jcef`, which is JBR 25 but
not the exact historical Task 170 `25.0.4+1-b508.27-nomod` SDK. The runtime difference is
recorded here rather than treated as equivalent native acceptance.

| View | 1440×900, font/density scale 1.0 | 999×760, font/density scale 1.0 | Source and classification |
| --- | --- | --- | --- |
| Editor | `editor-1440.png` | `editor-999.png` | `EditorVisualFixture`; offscreen production-component render |
| Analysis | `analysis-1440-1.0.png` | `analysis-999-1.0.png` | `AnalysisVisualFixture`; offscreen production-component render |
| Review, ready to apply | `review-ready-1440-900-1.0.png` | `review-ready-999-760-1.0.png` | `ReviewToolWindow` with a current passed required check; offscreen production-component render |
| Performance, populated | `performance-populated-1440-900-1.0.png` | `performance-populated-999-760-1.0.png` | `PerformanceWorkspacePane` with a populated source hypothesis; offscreen production-component render |

All eight artifacts were written to the ignored
`desktop/build/reports/ui-precision/fnd-01/` directory by:

```sh
./desktop/gradlew -p desktop test \
  --tests 'io.miniorca.desktop.DesktopVisualLayoutTest' \
  --tests 'io.miniorca.desktop.DesktopAccessibilityTest' \
  --tests 'io.miniorca.desktop.DesktopKeyboardNavigationTest' \
  -PvisualOutput="$PWD/desktop/build/reports/ui-precision/fnd-01"
```

The run passed: 22 visual-layout, 7 accessibility, and 7 keyboard-navigation tests. PNG
dimensions were checked with `sips`; representative Editor, Analysis, Review, and Performance
renders were visually inspected. The renderer does not open a native window, so this reproduction
does not update native, popup-placement, operating-system focus, or screen-reader evidence.

### UI-04 execution attempt — 2026-09-07

The candidate was `0f1362efa32bc5ec5290ff9542f39b8f9dfef83e`. Before the attempt, the only
worktree change was the coordinator's UI-04 Pending-to-Running transition in `PLAN.md`; it is not
UI evidence. The host was macOS 26.6.2 (25G83), arm64. The Gradle launcher was Temurin
21.0.11+10 and the downloaded, ignored toolchain was the exact documented JBR
25.0.4+1-b508.27-nomod.

The full Desktop gate passed with the documented launcher/toolchain split:

```sh
MINI_ORCA_JDK21_HOME=/Users/marcoandreose/.sdkman/candidates/java/21.0.11-tem \
MINI_ORCA_JBR25_HOME="$PWD/desktop/build/ui-04/toolchains/jbrsdk-25.0.4-osx-aarch64-b508.27/Contents/Home" \
  ./scripts/desktop-gradle.sh spotlessCheck detekt test
```

A forced focused run then passed 22 visual-layout, 8 accessibility, and 7
keyboard-navigation tests with no skipped, failed, or errored cases. It generated 66 PNGs under
the ignored `desktop/build/reports/ui-precision/ui-04/` directory:

```sh
MINI_ORCA_JDK21_HOME=/Users/marcoandreose/.sdkman/candidates/java/21.0.11-tem \
MINI_ORCA_JBR25_HOME="$PWD/desktop/build/ui-04/toolchains/jbrsdk-25.0.4-osx-aarch64-b508.27/Contents/Home" \
  ./scripts/desktop-gradle.sh test --rerun-tasks \
  --tests 'io.miniorca.desktop.DesktopVisualLayoutTest' \
  --tests 'io.miniorca.desktop.DesktopAccessibilityTest' \
  --tests 'io.miniorca.desktop.DesktopKeyboardNavigationTest' \
  -PvisualOutput="$PWD/desktop/build/reports/ui-precision/ui-04"
```

`sips` confirmed the expected pixels for the component Analysis matrix: 1440×900, 1920×1080,
1000×760, 999×760, 800×650, and 1280×600 at 100%, 125%, and 150% text plus the 100% 2×
density case. `analysis-1280-600-150-1x.png`, `editor-1000.png`, and
`review-ready-800-700-1.3.png` were visually inspected. Their named actions and state remained
readable, but these are offscreen production-component renders only. The rendered
`project-menu-open.png` cannot paint the Desktop scene popup layer, so it supplies no native menu
placement evidence.

For the native attempt, a disposable copy of `internal/project/testdata/release-fixture/` and an
isolated desktop preference root were created only under ignored `desktop/build/ui-04/`. A
loopback daemon used that copied project and local unreachable fixture model destinations; no
provider, credential, or user project was used. The distributable was rebuilt by Gradle running
on the exact JBR and launched from its `.app` executable. `jcmd` directly reported the live
Mini-Orca VM as `OpenJDK 64-Bit Server VM version 25.0.4+1-b508.27`, and the computer-use
inventory reported the running bundle as `Mini-Orca` (`io.miniorca.desktop`).

The automation surface could not bind to that app by either display name or bundle identifier.
Both attempts returned that macOS Accessibility and Screen Recording permissions were still
pending. Those permissions were not granted or reconfigured. Consequently no native screenshot,
window size, edge popup/dialog placement, operating-system focus order, keyboard traversal,
selection, Escape/focus restoration, splitter/drawer resize, text/density scaling, or preference
recovery result is claimed. VoiceOver was not running, and the blocked accessibility surface could
not expose another reader, so no reader name/state observation is claimed. The temporary app and
daemon were stopped after the attempt.

That attempt remained blocked on the complete native and assistive-technology operator matrix
below.

### UI-04 resumed native execution — 2026-09-07

The resumed starting identity was `df426547dd6f35888658d203c4914e8cef7f1630`. The only
pre-existing worktree change was the coordinator's UI-04 status update in `PLAN.md` to
`Running — native permissions restored`; it is not UI evidence. The same exact ignored JBR,
disposable fixture, loopback daemon, and unreachable local fixture-model destinations from the
attempt above were reused. No real provider, credential, or user project was used. A small
ignored `java.util.prefs` test harness stored application preferences only in
`desktop/build/ui-04/prefs/ui-04.properties`; it did not alter macOS preferences.

Accessibility and Screen Recording access worked through the computer-use surface. It exposed
the running `Mini-Orca` window, its accessibility tree, keyboard and pointer input, and
window-only JPEG captures. Capture dimensions were parsed directly from those buffers. The
surface did not provide a file-export API. A targeted `screencapture -l 2615` attempt from the
terminal returned `could not create image from window`, so no native screenshot file is claimed.
The directly observed facts are also summarized in the ignored
`desktop/build/ui-04/logs/native-observations.json` execution artifact.

| Requested native window | Direct result |
| --- | --- |
| 1000×760 | Exact capture; populated/error Summary and docked Editor were readable with no observed clipping. |
| 999×760 | Exact capture; populated/error Summary and compact Editor drawer/bottom-overlay arrangement were readable with no observed clipping. |
| 800×650 | Exact Summary capture; the compact Summary remained readable with the failed-AI state and bottom-tools opener visible. |
| 1280×600 | Exact capture; Summary and docked bottom tabs remained readable with no observed clipping. |
| 1440×900 | Unsupported by the available host work area; the direct resize attempt was clamped to 1361×768. |
| 1920×1080 | Unsupported by the available host work area; the direct resize attempt was clamped to 1383×768. |

The native fixture exposed a populated factual inventory of 10 indexed files and 45 lines plus an
explicit failed AI-analysis state with 10 missing analyses. In Editor, selecting `main.go`
displayed its full read-only source and accessibility descriptions for the file, relative path,
and each source line. A pointer drag visibly selected `Run` source text without selecting a
declaration or changing the file. At compact width, longer fixture names were visually ellipsized
while their complete relative paths remained present in the accessibility names.

The project menu, command palette, macOS project chooser, Files drawer, Context drawer, and
bottom-tools overlay were opened in the native app. Their labels and available/selected states
were present in the accessibility tree and the visible surfaces stayed within the window. Escape
closed one of those transient layers at a time without changing the workspace or selected file.
Tab visibly focused the Project and Search controls, and Return activated the focused control.
Project-menu and Files-drawer dismissal returned activation to their openers.

Direct native inspection found three defects. The Summary dashboard initially exposed Editor as
the selected rail destination; the rail now derives selection from the rendered workspace. The
command palette and compact bottom-tools overlay initially returned focus only to their broad
regions; their actual opener controls now receive focus. In the rebuilt JBR application, Escape
followed by Return reopened each corrected trigger. Deterministic assertions cover the complete
workspace-to-rail mapping and prove that the exact Search and Open tools triggers can receive
focus. The dismissal and trigger-restoration sequence is direct native evidence only.

After the final fixes and launcher correction, the full Desktop run passed 291 tests with zero
Detekt findings:

```sh
MINI_ORCA_JDK21_HOME=/Users/marcoandreose/.sdkman/candidates/java/21.0.11-tem \
MINI_ORCA_JBR25_HOME="$PWD/desktop/build/ui-04/toolchains/jbrsdk-25.0.4-osx-aarch64-b508.27/Contents/Home" \
  ./scripts/desktop-gradle.sh spotlessCheck detekt test
```

The final forced focused run passed 24 visual-layout, 8 accessibility, and 7 keyboard-navigation
tests with no skipped, failed, or errored cases. It generated 70 PNGs under the ignored
`desktop/build/reports/ui-precision/ui-04-resumed/` directory:

```sh
MINI_ORCA_JDK21_HOME=/Users/marcoandreose/.sdkman/candidates/java/21.0.11-tem \
MINI_ORCA_JBR25_HOME="$PWD/desktop/build/ui-04/toolchains/jbrsdk-25.0.4-osx-aarch64-b508.27/Contents/Home" \
  ./scripts/desktop-gradle.sh test --rerun-tasks \
  --tests 'io.miniorca.desktop.DesktopVisualLayoutTest' \
  --tests 'io.miniorca.desktop.DesktopAccessibilityTest' \
  --tests 'io.miniorca.desktop.DesktopKeyboardNavigationTest' \
  -PvisualOutput="$PWD/desktop/build/reports/ui-precision/ui-04-resumed"
```

The Compose `run` task had launched the Java 22-bytecode application on Gradle's Java 21 runtime,
causing `UnsupportedClassVersionError`. The application now resolves `javaHome` from the Java 25
toolchain, and `scripts/desktop-gradle.sh` restricts discovery to an explicitly supplied JBR 25.
A direct Java 21 Gradle launch started `MainKt` on the discovered SDKMAN JBR 25; the documented
script started it on the exact ignored `25.0.4+1-b508.27` JBR confirmed by `jcmd`. The repository
`make check` gate passed with that launcher/toolchain split.

The native pane splitters were then resized from the keyboard. The isolated preferences recorded
an Explorer width of `520.0` and bottom-pane height of `220.0`; a restart restored both dimensions
with the disposable project. The stored layout named Editor, while the restarted workspace
correctly opened and announced Summary under the current navigation-restoration policy. The
restart also exposed the empty Editor state before a file was selected.

The connected Q2789 was detected as a non-mirrored 2560×1440 1× display. The user placed the
Gradle-run `MainKt` window there, but that unbundled Java application was not bindable by either
its display name or `com.jetbrains.jbr.java` identifier. The bindable packaged fixture remained
on the main Retina display: attempts to drag its title bar returned `noWindowsAvailable`, and the
display picker treated Q2789 as an offscreen accessibility element because the external display
has a negative origin. No native wide-display visual result is inferred from the connected
monitor or the unbindable process.

VoiceOver 10 (build 993) was enabled through System Settings, and its Essentials collection was
completed so the `scrod` output service and Braille translation service were active. With those
services running, the packaged Mini-Orca accessibility tree exposed explicit names and state for
the project menu, search, daemon status, rail selection, Analyze-all control, analysis coverage,
failed AI interpretation, source metadata, bottom tabs and pane-resize control. Control-Option
navigation commands were sent through the automation surface, but that surface did not expose a
reader cursor or speech transcript; no spoken wording or reader focus sequence is claimed.

The host's text scale and density were not changed; 125%, 150%, and alternate-density coverage
remains deterministic component evidence only. Loading, stale, generated-diff, consent/discard,
provider-confirmation and guarded-Review states were not produced by this native disposable
fixture and are not claimed as native observations. Those combinations are explicit limitations,
while their layout and semantics remain covered by deterministic production-component tests.

### Deterministic component evidence

`DesktopVisualLayoutTest`, `DesktopAccessibilityTest`, and
`DesktopKeyboardNavigationTest` pass with output in the ignored
`desktop/build/reports/ui-precision/task-170/` directory.

| Coverage | Viewports / scales | Evidence classification |
| --- | --- | --- |
| Shell and Analysis hierarchy | 1440×900, 1920×1080, 1000×760, 999×760, 800×650 | Offscreen production-component render |
| Compact short window | 1280×600 at 100%, 125%, 150% text; 100% at 1× and 2× density | Offscreen production-component render |
| Errors, stale/empty/populated evidence | Problems, Checks, Output, Analysis, Context, Assistant, Review, Performance fixtures | Offscreen production-component render and semantics assertions |
| Keyboard / state semantics | Rail/tab navigation and selected state, command-palette arrow/Enter interaction, disclosure state, responsive region policy, provider confirmation, and Search/Open tools opener focusability | Deterministic Compose keyboard and semantics tests; exact dismissal/restoration is native evidence above |
| Contrast | Primary, secondary, selected, focus, disabled, diff success, and diff error pairs | `DesktopThemeTest` and `DiffViewerTest` token assertions |

The FND-01 table above is the current minimum baseline for the four principal populated views.
The broader Task 170 matrix remains historical component evidence, with its native limitations
unchanged.

Reviewed Task 170 captures include
`analysis-1280-600-100-1x.png`, `analysis-1280-600-125-1x.png`,
`analysis-1280-600-150-1x.png`, and `analysis-1280-600-100-2x.png`. The 150% and
2× captures retain named Preview/Pause/Cancel controls, their textual state, and the layered
pane boundaries without action overlap. These images are not native-window screenshots.

### Prior blocked native and assistive-technology evidence

The app was launched with the pinned JBR runtime and a live
`io.miniorca.desktop.MainKt` process was observed. The available computer-use inventory reported
no native applications both before and while the app was running, so this environment cannot
inspect the window, capture native menus/dialogs, resize it, drive OS focus behavior, or read its
accessibility tree. The temporary process was stopped after that launch check.

No supported screen reader was running or exposed to the automation surface. `AccessibilityUIServer`
alone is an operating-system service, not evidence that VoiceOver or another reader has exercised
Mini-Orca. No screen-reader result is claimed.

### Recorded limitations and release follow-up

UI-04 is complete because the material native checks available to the operator surface passed,
the three defects they exposed were fixed, and unsupported combinations are explicit. It closes
the attainable native-window, popup, operating-system focus and reader-name/state requirements
inherited from Tasks 149 and 170.

The following are unclaimed release limitations rather than inferred passes:

1. Exact native 1440×900 and 1920×1080 captures on Q2789; the component matrix covers both sizes,
   while the bindable native fixture could not cross the negative-origin display boundary.
2. Native 125%/150% text and alternate-density runs; deterministic production-component coverage
   exists for those combinations.
3. Native loading, stale, generated-diff, consent/discard, provider-confirmation and guarded-Review
   states; deterministic layout, semantics and interaction tests cover them.
4. A VoiceOver speech transcript and observable reader-cursor focus sequence; VoiceOver services
   were active and native names/states were inspected, but the computer-use surface exposed
   neither speech output nor the reader cursor.

REL-01 owns any release-level provider, lifecycle, package and distribution checks that require
those states or a different operator surface. The canonical status remains in
[release acceptance](RELEASE_ACCEPTANCE.md).

## Retained Task 170 execution provenance

The following is the original execution record, superseded by UI-04 and REL-01.
Its references to incomplete acceptance describe that historical attempt.


- Expanded the production-component matrix to render the compact `1280x600` shell at 100%, 125%, and 150% text and at 1×/2× density. The fixture asserts the named Preview, Pause, and Cancel controls remain readable at every added scale; generated evidence is ignored under `desktop/build/reports/ui-precision/task-170/`.
- Passed `env JAVA_HOME=/Users/marcoandreose/.sdkman/candidates/java/21.0.11-tem ./desktop/gradlew -p desktop test --tests 'io.miniorca.desktop.DesktopVisualLayoutTest' --tests 'io.miniorca.desktop.DesktopAccessibilityTest' --tests 'io.miniorca.desktop.DesktopKeyboardNavigationTest' -PvisualOutput=/Users/marcoandreose/DEV/lab/mini-orca/desktop/build/reports/ui-precision/task-170 -Porg.gradle.java.installations.paths=/private/tmp/mini-orca-jbr-TP5kFo/jbrsdk-25.0.4-osx-aarch64-b508.27/Contents/Home`; inspected the 150% and 2× compact captures.
- Passed `env JAVA_HOME=/Users/marcoandreose/.sdkman/candidates/java/21.0.11-tem ./desktop/gradlew -p desktop spotlessCheck detekt test -Porg.gradle.java.installations.paths=/private/tmp/mini-orca-jbr-TP5kFo/jbrsdk-25.0.4-osx-aarch64-b508.27/Contents/Home` and `git diff --check`. JBR emitted its known restricted-native-access and Jewel `Unsafe` deprecation warnings; no runtime or configuration change was made.
- Launched the actual desktop app with JBR 25.0.4 on macOS 26.6.2 and observed the live `io.miniorca.desktop.MainKt` process. The available computer-use inventory exposed no native applications while it ran, preventing native screenshots, window-edge popup/dialog inspection, OS focus traversal, resizing, or an accessibility-tree read; the temporary process was stopped.
- No supported screen reader was running or exposed to this environment. Native and assistive-technology evidence is therefore incomplete. Task 170 remains In Progress, no passing acceptance commit has been created, and the required operator matrix is now retained in the [consolidated UI precision evidence](#consolidated-ui-precision-evidence).

## Reproduce UI component checks

For changed flows, review rendered components against the
[UI copy checks](../desktop/UI_DESIGN_GUIDELINES.md#verification) with optional
details collapsed. Verify clear actions and state, useful labels and no repeated
introductory copy. This is a review requirement for future changes; historical
captures do not establish compliance with the 2026-09-15 UI direction.

Use the JDK 21/JBR 25 setup in [Desktop runtime instructions](../desktop/README.md).
From the repository root:

```sh
MINI_ORCA_JDK21_HOME=/path/to/jdk-21 \
MINI_ORCA_JBR25_HOME=/path/to/jbr-25 \
  ./scripts/desktop-gradle.sh test --rerun-tasks \
  --tests 'io.miniorca.desktop.DesktopVisualLayoutTest' \
  --tests 'io.miniorca.desktop.DesktopAccessibilityTest' \
  --tests 'io.miniorca.desktop.DesktopKeyboardNavigationTest' \
  -PvisualOutput="$PWD/desktop/build/reports/ui-precision/current"
```

The production Compose/Skia fixtures use labeled local data without a daemon or
provider. Coverage includes Summary, Analysis progress, Bugs, Performance, Security,
Editor/Review, Assistant, menus/disclosures, responsive drawers, Terminal and keyboard
semantics; empty, failed, stale, populated and disabled states remain represented.
The compact matrix includes 1280×600 at 100%, 125%, 150% text and 1×/2× density.
The final lifecycle matrix also renders all eleven progress/result states at
800×650 and 150% text. `DesktopAcceptanceFixtureKt` is a test-only native entry
point with the same production panes, selectable lifecycle states and explicit
100/125/150% Compose text scaling. Its real shell uses an isolated temporary
project; the fixture has no API client or provider. Package it using the tested
application jars, compiled test classes and embedded runtime, with that main class.
The fixture controls are verification aids and are absent from the product.
Popup/dialog scene layers cannot be inferred from an offscreen image; the
[keyboard operator checklist](../desktop/KEYBOARD_SMOKE_CHECKLIST.md) owns native
reproduction. Historical Task 159 captures used `desktop/build/reports/ui-refinement/after`;
Task 170 used `desktop/build/reports/ui-precision/task-170`. Generated files remain
ignored, and Git retains the retired visual correction history.

## REL-02 final handoff — 2026-09-09

Independent review accepted the exact documentation and Docker-context changes.
The original scope-deferral edits in PLAN, this ledger, the historical recovery
specification and execution guide were preserved. No Go/Kotlin production code,
API schema, dependencies or configuration fields changed during closure; existing
rejection/source-safety tests remain intact. Current `make quality` found no dead
code, duplication or other static-gate failure requiring a speculative cleanup.

The separate Task 170 note, UI precision acceptance file and visual-history file
were retired after preserving unique evidence and current reproduction here. The
keyboard checklist now contains the maintained operator procedure. Local Markdown
files/heading anchors, plan parser/dependencies and `git diff --check` passed.
Tracked files decreased from 265 to 262; Markdown files from 20 to 17. The final
ledger has 52 Complete tasks and three user-approved insight standby deferrals.

The scheduler remains Paused with no ready task. Runtime smoke processes are
stopped; the test image/container are retained. No migration, reset, new local-model
evaluation, push, publishing or deployment was performed. Future insight recovery
and live external-provider acceptance require their own scope and call budget.
