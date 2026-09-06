# Release acceptance

**Current state: incomplete.** The 2026-09-06 planning/docs change does not establish
native, provider or distribution acceptance. [PLAN.md](../PLAN.md) is the task/status
source; this document owns release evidence and open validation obligations.

## Evidence and open work

| Evidence | Result / boundary | Plan owner |
| --- | --- | --- |
| Fresh `go test ./...` on 2026-09-06 | Passed, including daemon route/version contracts; some packages cached | Foundation |
| Fresh desktop `test` command | Successful; all tasks up to date, not a new native run | UI-04 |
| Post-cleanup daemon/config tests (`-count=1`) | Passed, including maintained documentation/version contracts | Foundation |
| Fresh `make quality` | Staticcheck/deadcode passed; gocyclo failed on five functions; later clone/Desktop static stages not run | FND-02–04, AUTO-01 |
| Earlier deterministic UI and workflow tests | Recorded passes in completed work; not rerun as a full matrix by this docs change | UI-04, REL-01 |
| Task 170 production render additions | Historical component/test evidence is tracked; generated images remain ignored build output | UI-04 |
| FND-01 production fixture baseline | Current 1440×900 and 999×760 Editor, Analysis, populated Review, and populated Performance component captures; JBR 25.0.4.1 rerun, not native acceptance | UI-04 |
| Saved fixture inspection during planning | Editor at 1000dp, Analysis at 150% text and ready Review inspected; concrete UI corrections recorded in PLAN | UI-01–03 |
| Native window/edge popup/OS focus/reader | Missing; observing an app process did not exercise the UI | UI-04 |
| Real local/remote/mixed model scopes | Manual compatibility and end-to-end checks not recorded as passed | REL-01 |
| Insight usefulness and Performance lifecycle UI | Manual content-quality/consent follow-ups outstanding | LEARN-02, REL-01 |
| Final packages / supported Docker image | Earlier macOS startup smoke is not final acceptance; Docker release evidence outstanding | REL-01 |

The exact quality diagnostics are in the plan. An old quality deferral is not a
current release waiver. Tasks 149/170/171 are superseded by their plan owners,
not marked passed. No future Security/benchmark feature is implemented yet.

Retained working evidence, including pre-existing user edits:

- [Task 170 execution record](../tasks/170_ui_precision_accessibility.md)
- [UI precision evidence](../desktop/UI_PRECISION_ACCEPTANCE.md)
- [Visual fixture reproduction](../desktop/VISUAL_REVIEW.md)
- [Keyboard/native checklist](../desktop/KEYBOARD_SMOKE_CHECKLIST.md)
- [Current contrast measurements](../desktop/UI_CONTRAST.md)

These files are temporarily retained to preserve ongoing work. Their dated
baseline sections describe history. REL-02 consolidates current reproduction and
remaining checks here before retiring redundant history.

## FND-01 reconciliation

FND-01 started at `d0a7cc9`. The only pre-existing worktree edit was the coordinator's
`PLAN.md` status transition for FND-01; it is not part of the evidence change. The preserved Task
170 source/test record was already tracked in that baseline, while all rendered PNGs are ignored.
The current eight-fixture matrix, command, runtime identity, scale, artifact names, and native
limitation are recorded in [UI precision evidence](../desktop/UI_PRECISION_ACCEPTANCE.md).

The inherited release items remain assigned: UI-04 owns native window, edge popup, operating-system
focus, and screen-reader inspection; REL-01 owns provider, lifecycle, package, and distribution
acceptance; LEARN-02 owns insight usefulness; FND-02–04 and AUTO-01 own the quality findings.

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
3. Replace fixture `Run`, and separately create absent `ReleaseNote`: send, edit
   only the draft, validate, check, review, explicitly Apply and Undo. Verify one
   source file changes; stale selection/source/draft/check evidence blocks Apply.
4. Exercise failed required checks, explicit repair limits, cancellation, source
   changes during requests, denied remote confirmation and provider offline errors.
   Temporary proposed tests never appear in the imported project.
5. Test all-local, one online-compatible and mixed `analyze`/`bug`/`function` scope
   configurations with explicit request consent. Record provider type/model/version
   and results, not keys/prompts. Use an agreed finite request budget.
6. Exercise Analyze-all and Performance preview/start/pause/resume/cancel/restart,
   honest empty/partial/budget-limited/stale/failed states and exact-function handoff.
   Inspect real insights for local relevance, trade-offs and useful verification;
   simple valid JSON alone does not pass content-quality review.
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
