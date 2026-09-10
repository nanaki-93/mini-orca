# Mini-Orca current plan and status

This is the sole execution/status ledger. The accepted product remains a local-first
assistant for one project, file and symbol, with explicit candidate review and Apply.
REL-01 and REL-02 are **Complete** for the documented limited release scope.
The current cleanup specification is [docs/tasks.md](docs/tasks.md); the
[execution guide](tasks/README.md) describes its coordinator/review boundaries.

## Cleanup execution — authorized 2026-09-10

The user authorized background implementation of the cleanup specification in [docs/tasks.md](docs/tasks.md). **Mini-Orca cleanup agents** (mini-orca-cleanup-agents) is now **Paused** after completing all twelve tasks; it ran every 20 minutes and processed at most one task per wake, sequentially, with one writer, fresh independent review, and coordinator-run verification. This is separate from the historical dispatcher and supersedes planning-only wording for CLN tasks only.

Use the current checkout on codex/autopilot and preserve unrelated edits. On 2026-09-10 the user explicitly authorized committing CLN-01 through CLN-03 together, then one local commit per accepted task. The coordinator alone commits after independent review and validation, including only the task changes and its ledger/checklist updates. This supersedes the initial no-commit policy; resets, stashes, pushes, releases, and live Mini-Orca evaluation calls remain outside scope. Historical insight qualification remains on standby and its budgets/evidence are unchanged. Pause the cleanup automation on an unresolved blocker or when the queue is complete.

| ID | Task | Status |
| --- | --- | --- |
| CLN-01 | Make the reachability gate enforce its result | Complete |
| CLN-02 | Remove confirmed unused desktop methods | Complete |
| CLN-03 | Reuse durable storage for evaluation metadata | Complete |
| CLN-04 | Handle Analyze-all persistence failures explicitly | Complete |
| CLN-05 | Handle Performance persistence failures explicitly | Complete |
| CLN-06 | Preserve asynchronous scan failure diagnostics | Complete |
| CLN-07 | Share file-analysis validation and evaluation diagnostics | Complete |
| CLN-08 | Move evaluation tooling out of package app | Complete |
| CLN-09 | Give benchmark operations one desktop owner | Complete |
| CLN-10 | Give security operations one desktop owner | Complete |
| CLN-11 | Separate current documentation from historical execution records | Complete |
| CLN-12 | Validate the cleanup as one release-preserving change | Complete |

**Current:** CLN-01 through CLN-12 Complete. **Active writer:** none. **Next task:** none. Cleanup automation **Paused** after final acceptance.

**Accepted CLN-12 — 2026-09-10:** Aggregate writer review of fe4cdad → 4c1c904 found no actionable code issue, removed test coverage, accidental external API, or source/privacy regression. Ownership moves are distinguished from removal: presenter 1,952 → 1,616 lines with 457 lines in two owners; evaluation/locks moved from app to insighteval; four unused methods and duplicate assessment/replaceable-metadata mechanics removed. Go Test functions 374 → 400 and desktop tests 327 → 351; all baseline evaluation tests remain. No production corrections or configuration/data migration.

Writer and coordinator exact supported validation passed all nine gates; writer diff check passed. Writer Go packages: seven executed/four cached; coordinator: all 11 cached. Race packages were all cached in both runs; daemon contracts were fresh for writer, cached for coordinator. Each Python run discovered 55 tests, executed 54 and skipped opt-in runtime conformance; the sealed-fixture Go skip remains. Desktop static/test tasks were up-to-date, with retained 351 passing tests. `createDistributable` passed with three executed/five up-to-date tasks.

Coordinator native smoke used the current macOS arm64 app bundle/JBR 25 with a synthetic loopback service and in-memory preferences. It observed compact 800 × 600 drawers and wide 1,336 × 768 docked panes, Ctrl keyboard navigation/validation, read-only source/diff labels, one-attempt Security consent with both reports retained, and benchmark GET-only listing followed by explicit trust/run requests and an unavailable synthetic result. Source hash was unchanged; no Apply/Undo, real provider, project execution or benchmark measurement occurred. Both owned processes stopped. Direct Java attachment and intermittent CUA window/capture operations failed; the normal bundle worked. Cmd mappings, exact 1000/999dp/scaling boundaries, editing resistance and spoken VoiceOver remain unverified. This does not establish production-daemon end-to-end behavior or expand release acceptance. Evidence: `.mini-orca/autopilot/cleanup/CLN-12/` (`writer/`, `coordinator-validation.json`, `native/smoke.json`, `native/requests.jsonl`). Fresh independent review approved the frozen candidate with no actionable findings; coordinator acceptance is complete.

The post-smoke validation rerun failed only formatting of the ignored synthetic Go fixture; all other gates passed. The stopped fixture was archived under a fixture extension without changing its source bytes/hash. No production correction or gate change was needed. Failed-run and scratch-repair evidence is retained. Both the repaired rerun and the final frozen-candidate run passed all nine gates. The final run used cached Go/race/daemon results and up-to-date desktop tasks; Python executed 54 of 55 discovered tests with one opt-in skip in 55.174 seconds. Diff and all 19 Markdown link checks passed. Candidate SHA-256: 70652874d9b6981aed0eac2c859f64974b3117f469f32968d5f937c75a38231b. Evidence: .mini-orca/autopilot/cleanup/CLN-12/acceptance.json.

The proposed combined per-file analysis contract remains a separate product decision. Cleanup does not reopen insight qualification, expand release claims, consume grants or expose sealed holdout material. All twelve cleanup tasks are accepted and the cleanup scheduler is Paused; the historical dispatcher and model evaluation remain on standby.

**Accepted 2026-09-10:** CLN-11 separates current plan/feature/agent guidance from inactive improvement and evaluation history, retaining complete prior instructions, recovered fe4cdad:docs/tasks.md provenance, accepted evidence, status rows, failed/unrun verdicts, budgets and holdout limits. Fresh review found no issues and independently verified all 103 anchor mappings. Coordinator passed exact Go documentation checks, 55 Python tests (one opt-in runtime conformance skip), diff checks, all 19 Markdown link checks and mechanical historical preservation. No repairs, configuration/data migration or live evaluation; release edits are navigation-only. Candidate SHA-256: ec33a5d8468e7776685eba02267f42269c686c153c50877abe5e2ca095478c50. Evidence: .mini-orca/autopilot/cleanup/CLN-11/acceptance.json. Full supported validation remains CLN-12.

**Accepted 2026-09-10:** CLN-10 moves security request ownership into DesktopSecurityWorkflow with live controller state, generation checks, per-attempt consent, report/source validation, and lifecycle cancellation. Replacement cancels the prior section while retaining evidence; presenter navigation and benchmark ownership remain intact. Fresh final review found no actionable issues; coordinator executed 68 focused and 351 full desktop tests with no failures/errors/skips, passed desktop quality (up-to-date after writer execution), and verified frozen hashes and unchanged outside-scope files. Two test corrections: structured error fixtures, then deterministic scheduling and endpoint fixture repair for an existing chat-consent test that failed the first coordinator full run. All consent assertions retained; failed-run evidence preserved. No configuration/migration or native/live execution claims; Go checks not run for desktop-only changes. Candidate SHA-256: 2fef2844a5546f776b7a794a105e4c069c8fcd171ec69772477f0e2e9f1b27b8. Evidence: .mini-orca/autopilot/cleanup/CLN-10/acceptance.json.

**Accepted 2026-09-10:** CLN-09 moves benchmark request jobs, generation/identity checks, consent, and lifecycle cancellation into DesktopBenchmarkWorkflow, using the existing authoritative controller state and event sink. Presenter entry points remain delegates. Fresh review found no actionable issues; coordinator executed 70 focused and 337 full desktop tests with no failures/errors/skips, passed desktop quality (up-to-date after writer execution), and verified the frozen candidate and unchanged outside-scope files. No repairs, configuration changes, or migrations. No live benchmark/provider execution or native visual acceptance; Go checks not run for desktop-only changes. Candidate SHA-256: fb4d5e68d11a0cdacb268aa4dbe250d41938d9311f340429839015241a2e585a. Evidence: .mini-orca/autopilot/cleanup/CLN-09/acceptance.json.

**Accepted 2026-09-10:** CLN-08 moves evaluation/campaign ownership and locks into internal/insighteval, retaining the narrow app adapter, CLI/wire identities, fixed grants, private artifacts, and fixture locations. Fresh review found no actionable issues and verified all 63 moved tests remain. Coordinator passed both exact commands, full Go/race tests, formatting, vet, native and Windows/Linux compile checks for both executables, and normalized production/dependency checks. No repairs or runtime platform claims; no live evaluation or qualification. Candidate SHA-256: e69a984329d26204ee629c89c4800949fb695b1e7d4f5706cd9a9612d4d51b3b. Evidence: .mini-orca/autopilot/cleanup/CLN-08/acceptance.json.

**Accepted 2026-09-10:** CLN-07 shares semantic validation and source-free diagnostics through a narrow evaluation adapter, removing duplicate runner decoding and optional validation. Synthetic parity tests preserve prompt/schema/version, provider classification, parent rejection, optional degradation/retention, and private diagnostic classifications. Fresh review found no actionable issues; coordinator passed exact focused tests, all Go tests, full race, formatting, vet, Go quality, and diff checks. One test-fixture correction; no migration or live evaluation calls. Candidate SHA-256: faf6967551c9887d18e27eecb9e2ea8b6c4d44d0fe65b6664b77a964f9625250. Evidence: .mini-orca/autopilot/cleanup/CLN-07/acceptance.json.

**Accepted 2026-09-10:** CLN-06 logs bounded, sanitized secondary failure-report persistence errors while retaining the original scan failure in memory and existing durable progress. Tests verify unchanged disk bytes, secret/path redaction, and replacement project/revision isolation. Fresh review found no actionable issues; coordinator passed focused GoScan race tests, all Go tests, full race, formatting, vet, Go quality, and diff checks. One test-fixture correction; no migration or live evaluation calls. Candidate SHA-256: 91b144d8d0512d240445c7b525d842041fa63d61e1432af408956f75381a6ae6. Evidence: .mini-orca/autopilot/cleanup/CLN-06/acceptance.json.

**Accepted 2026-09-10:** CLN-05 stops Performance work on save failures, exposes durable progress, preserves reports/accounting, and supports explicit recovery with project-scoped detached failures. Following user-authorized continuation, one repair resolved the concurrent Resume/result-write fault by retaining persistence authority through recovery and releasing it for worker join. Fresh review found no findings; coordinator passed focused and full Go tests, race tests, formatting, vet, Go quality, and diff checks. Historical blocked evidence and two earlier preparation corrections remain recorded; no migrations or live evaluation calls. Candidate SHA-256: d7f76368f095939c4dd049b0d92aaf6b1855b9963e2fc182d39b5b9296c290a8. Evidence: .mini-orca/autopilot/cleanup/CLN-05/acceptance.json.

**Accepted 2026-09-10:** CLN-04 stops Analyze-all dispatch on persistence failure, retains sanitized progress errors and completed reports, and requires durable explicit recovery while preserving attempt budgets and replacement identity. Fresh review found no actionable issues; coordinator passed exact Analyze-all race tests, all Go tests, race tests, formatting, vet, Go quality, and diff checks. Two focused preparation corrections; no migrations or live evaluation calls. Candidate SHA-256: 79efd26ba3059477fe28f5ce3e9c275bea26aa2cd6041a618b597af5c9d6ff7a. Evidence: .mini-orca/autopilot/cleanup/CLN-04/acceptance.json.

**Accepted 2026-09-10:** CLN-03 reused shared atomic storage for evaluation metadata, preserving sanitized errors and immutable evidence. Fresh review found no actionable issues; coordinator passed exact focused tests, all Go tests, race tests, formatting, vet, Go quality, and diff checks. No repairs, migrations, or live evaluation calls; accepted before the later batch-commit authorization. Candidate SHA-256: 6267407558d67a09c17753e26526274c205343bdbcd7fed52336e7958b6ae9a8. Evidence: .mini-orca/autopilot/cleanup/CLN-03/acceptance.json.

**Accepted 2026-09-10:** CLN-02 removed four unreferenced desktop methods and redundant nullable test operations. Fresh review found no actionable issues. Writer executed desktop tests (327 passing) and quality; coordinator reran both exact commands successfully (up-to-date) and verified test XML and diff checks. No repairs; accepted before the later batch-commit authorization. Candidate SHA-256: 1713a9ddcb438a3e21037d8d47b8e8b163740c4955156535ef09422c0acd5817. Evidence: .mini-orca/autopilot/cleanup/CLN-02/acceptance.json.

**Accepted 2026-09-10:** CLN-01 makes reachability findings and tool failures fail the gate while preserving later stages. Fresh review found no actionable issues; coordinator reran 4 isolated Python tests, all four Go quality stages, shell syntax, and diff checks successfully. No repairs; accepted before the later batch-commit authorization. Candidate SHA-256: 6c04a707f8c7c85ad4e75d86a3b72327c7ad4ed173474ff0389b8676d102e96d. Exact evidence: .mini-orca/autopilot/cleanup/CLN-01/acceptance.json.

## Current scope decision — 2026-09-09

The user placed engineering-insight improvement and qualification on **standby**.
QUAL-06, RCV-07 and RCV-08 are deferred, not passed; their historical failed or
unrun verdicts remain intact in the historical records. This decision supersedes earlier instructions
that make insight qualification a prerequisite for REL-01 or resume recovery.
Completed insight implementation remains available, with content quality explicitly
unqualified; this documentation change does not disable the feature.

The user subsequently requested closure of **REL-01 → REL-02**. REL-01 accepted
the documented limited release scope, preserving source safety, consent, provider integration,
native UI and distribution requirements. Insight usefulness/omission qualification
is excluded from this release acceptance scope; neither insight quality nor broader
AI factual reliability may be claimed validated by this deferral. REL-01 records
local acceptance separately; publishing remains outside this work.

The historical dispatcher remains **Paused**; the cleanup scheduler above is also **Paused** after completion. No additional model calls or recovery attempts
are authorized. Preserve cumulative consumption of **60 development / 24
qualification requests**, all receipts and grants, and the sealed, unused conditional
24-case holdout. Reopening insight work requires a new user decision and explicit
experiment scope/budget; do not replay the failed runs or weaken their gates.

## Task ledger

The delivered scope and standby tasks retain their accepted statuses below.
These rows are not an active dispatcher queue. Detailed completed task instructions,
original dependencies, dated verdicts and the [old-to-new anchor map](docs/history/improvement-plan-2026-09.md#old-to-new-anchor-map)
are in [improvement history](docs/history/improvement-plan-2026-09.md).
Cleanup statuses are maintained in the table above; checkboxes in docs/tasks.md
mirror acceptance only after independent review and coordinator verification.

| ID | Deliverable | Dependencies | Effort / risk | Status |
| --- | --- | --- | --- | --- |
| AUTO-00 | Bootstrap unattended plan execution | — | S / low | Complete |
| FND-01 | Reconcile baseline and preserve working evidence | AUTO-00 | S / low | Complete |
| FND-02 | Simplify Performance review/validation | FND-01 | S / medium | Complete |
| FND-03 | Simplify Performance job admission/validation | FND-02 | M / high | Complete |
| FND-04 | Simplify Analyze-all admission | FND-01 | S / high | Complete |
| FND-05 | Remove proven contract and code redundancy | FND-01 | M / medium | Complete |
| FND-06 | Isolate desktop job coordination | FND-03, FND-04 | M / high | Complete |
| AUTO-01 | Reproducible validation entry point and CI | FND-02, FND-03, FND-04, FND-05 | M / medium | Complete |
| UI-01 | Remove unsupported product previews | FND-01 | M / medium | Complete |
| UI-02 | Correct source-first layout against captures | UI-01 | M / medium | Complete |
| UI-03 | Make draft/review progression explicit | UI-02 | M / high | Complete |
| UI-04 | Close inherited native/accessibility checks | UI-03 | M / high | Complete |
| FLOW-01 | Short function-scoped change requests | UI-03 | M / medium | Complete |
| FLOW-02 | Read-only declaration explanation | FLOW-01, SEC-02, FND-06 | M / high | Complete |
| FLOW-03 | Reusable temporary behavioral proof | FLOW-01, SEC-03, SEC-04 | M / high | Complete |
| LEARN-01 | Concise, grounded engineering insights | FLOW-02 | S / medium | Complete |
| LEARN-02 | Evaluate usefulness and model reliability | LEARN-01, FLOW-03 | M / medium | Complete |
| SEC-01 | Harden daemon and container network boundary | FND-01 | M / high | Complete |
| SEC-02 | Bind provider delivery to consent and context | FND-01 | M / high | Complete |
| SEC-03 | Bound subprocess output and workspace resources | FND-01 | M / high | Complete |
| SEC-04 | Explicit project-execution trust and environment | SEC-03 | M / high | Complete |
| SEC-05 | Define strict Security report contracts | FND-05 | S / medium | Complete |
| SEC-06 | Add small deterministic Go security rules | SEC-05 | M / medium | Complete |
| SEC-07 | Add explicit AI Security review | SEC-05, SEC-02, FND-03 | M / high | Complete |
| SEC-08 | Deliver Security workspace and focused handoff | SEC-06, SEC-07, UI-03, FND-06 | M / high | Complete |
| PERF-01 | Measure and improve Mini-Orca hotspots | FND-03, FND-06, SEC-03 | M / medium | Complete |
| PERF-02 | Compare an explicitly selected Go benchmark | SEC-04, FLOW-03, PERF-01 | M / high | Complete |
| PERF-03 | Present measured evidence beside hypotheses | PERF-02, UI-03 | M / medium | Complete |
| AUTO-02 | Bounded agent dispatcher with review gate | AUTO-01 | M / high | Complete |
| QUAL-01 | Realistic development/qualification corpus and rubric | LEARN-02 | S / medium | Complete |
| QUAL-02 | Strict repeated-run receipts and qualification gates | QUAL-01 | M / high | Complete |
| QUAL-03 | Resumable provider-budgeted evaluation runner | QUAL-02 | M / high | Complete |
| QUAL-04 | Grounded insights and intentional omission | QUAL-01, QUAL-03 | S / medium | Complete |
| AUTO-03 | Repair dispatcher startup diagnostics and explicit recovery | AUTO-02 | M / high | Complete |
| AUTO-04 | Report tokens without blocking and escalate Terra repairs to Sol | AUTO-03 | M / high | Complete |
| AUTO-05 | Isolate validation scratch and preserve failure evidence | AUTO-04 | S / high | Complete |
| REC-01 | Preserve explicit zero-temperature provider requests | QUAL-04, AUTO-05 | S / medium | Complete |
| REC-02 | Account for one model-bound six-request recovery grant | REC-01, QUAL-03 | M / high | Complete |
| REC-03 | Prepare and verify the frozen local reasoning candidate | REC-02 | M / high | Complete |
| REC-04 | Establish standalone runtime compatibility | REC-03, QUAL-03 | S / high | Complete |
| REC-05 | Prepare bounded standalone runtime lifecycle | REC-04 | M / high | Complete |
| REC-06 | Verify reasoning and final-schema boundary offline | REC-05 | M / high | Complete |
| REC-07 | Authorize six runtime-bound development slots | REC-06 | M / high | Complete |
| QUAL-05 | Repeated six-request recovery pilot and candidate freeze | QUAL-04, REC-07 | S / high | Complete |
| QUAL-06 | Independent 24-request qualification verdict | QUAL-05 | M / high | Blocked (standby) |
| RCV-01 | Source-free optional-output rejection diagnostics | QUAL-05 | S / high | Complete |
| RCV-02 | Generation field bounds aligned with parser | RCV-01 | S / high | Complete |
| RCV-03 | General source-grounded explanation contract | RCV-02 | S / high | Complete |
| RCV-04 | Broader development and sealed fresh qualification | RCV-03 | M / high | Complete |
| RCV-05 | Fixed successor accounting and v2 gates | RCV-04 | M / high | Complete |
| RCV-06 | Offline runtime validation and candidate freeze | RCV-05 | M / high | Complete |
| RCV-07 | Twelve distinct development requests | RCV-06 | M / high | Blocked (standby) |
| RCV-08 | Conditional fresh 24-request qualification | RCV-07 | M / high | Blocked (standby) |
| REL-01 | End-to-end, native and distribution acceptance | AUTO-01, UI-04, LEARN-02, SEC-01, SEC-08, PERF-03 | M / high | Complete |
| REL-02 | Final code/doc retirement and handoff | REL-01, AUTO-02 | S / medium | Complete |

## Maintained documentation

- [Usage](README.md), [configuration](CONFIG.md), [API contract](docs/api-contract.md)
  and [desktop runtime](desktop/README.md) describe supported behavior.
- [UI guidelines](desktop/UI_DESIGN_GUIDELINES.md) and
  [insights and Performance](docs/insights-performance/README.md) describe current features.
- [Release acceptance](docs/RELEASE_ACCEPTANCE.md) owns release evidence and limits;
  macOS arm64 Desktop and the Linux arm64 container are the accepted platforms.
  Remote/mixed live compatibility, broader AI factual reliability, unsupported native
  combinations and spoken screen-reader output remain unclaimed.
- [Improvement history](docs/history/improvement-plan-2026-09.md) retains the prior
  plan and paused dispatcher instructions. [Insight history](docs/history/insight-evaluation-2026-09.md)
  retains runtime/campaign commands and **fe4cdad:docs/tasks.md** recovery provenance.

Cleanup requires no configuration or data migration. No publishing, release,
new provider requests or insight recovery is authorized by this documentation.
