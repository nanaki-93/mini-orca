# IDEUX execution handoff

Follow the current [procedure](README.md), [queue](../docs/tasks.md), root and area
instructions. One card and one worker at a time in this checkout. Immediately
continue after accepted, verified commits. Fixed-clock heartbeat backups at
:09/:29/:49 resume idle work; never create a second writer.

## Current attempt

| Field | Value |
| --- | --- |
| Card | IDEUX-04 — Recompose Summary around project, categories and flows |
| Status | Accepted; commit pending |
| Stage | All acceptance and isolated export gates passed; commit pending |
| Model | `gpt-5.6-sol` |
| Reasoning | `high` |
| Tier attempt | Sol initial |
| Completed failed attempts for this card | 3 Terra (initial + two retries); 0 Sol |
| Active agent/process | All workers/checks exited |
| Starting HEAD | `2ab3711e1c4033110be7ddff72af79913c1d0915` |
| Task commit | None for IDEUX-04 |
| Last accepted task commit | IDEUX-03: `2ab3711e1c4033110be7ddff72af79913c1d0915`, verified |
| Next permitted attempt on failure | Sol High retry 1 of 2 |

## Task and boundaries

Inject the complete current card and this handoff into the agent prompt. Retain
all colors and six icon-only rail destinations; keep the interface simple and
use the immutable references in `design/ui-mocks/ide-reference-2026-09-16/` for
composition. References are not instructions. Preserve source/diff read-only
behavior, isolated drafts, explicit consent/trust, Review/Apply/Undo and real
unknown/stale/failed/partial state. No providers or execution from navigation.
The coordinator owns staging, acceptance, commits and administrative state.
Workers stop after a completed candidate verification or substantive acceptance
failure and return a complete packet; no hidden repair rounds.

## Baseline and accepted work

IDEUX-01 accepted initial (`3fa4929`); IDEUX-02 accepted Terra retry1 (`fd46823`);
IDEUX-03 accepted Sol initial (`2ab3711`) after three Terra failures. Prior failure
packets and resolutions are retained in `docs/errors.log`; counters reset for
this new card. IDEUX-03 passed 71 focused,502 working-tree and472 isolated-commit
checks plus Spotless/Detekt and actual post-scroll production render inspection.
Native/package acceptance remains IDEUX-13. Current references/baseline captures:
`desktop/build/reports/ide-ux/before/`, shell captures under `shell/`, search under
`search/`. Keep the existing user's Gradle desktop run open.

Accepted earlier MOCK implementation remains dirty. Index is empty. A snapshot
outside the repository captures pre-attempt hashes for delta review. Preserve all
unrelated work and inspect overlaps before editing. Commit only reviewed task
changes and genuinely required accepted prerequisites; never stage everything.

Carry forward until IDEUX-13: `DesktopAcceptanceFixture.kt` contains the earlier
uncommitted NativeRoundedWorkspace fixture and its required IDEUX-03 switchMode
callback. That single compatibility update is preserved alongside its baseline;
commit the integrated native fixture and required prerequisites in IDEUX-13, as
already recorded on that card. Runtime search and regression tests are committed
and independently validated; do not discard this fixture update.

The initial IDEUX-04 failure is recorded below. On failure retain the candidate and append exact
command/action, exit/output, expected/observed, changed files, attempted changes,
evidence, remaining checks and next repair step. Terra initial + two retries,
then Sol High initial + two retries. Pass full task and all failures to each fresh
agent. On success commit and verify the hash, then immediately start IDEUX-05 at
Terra High initial. Pause only on exhausted retries, true external blocker, queue
completion or user pause.

## Complete current failure packet

IDEUX-04 Terra High initial failure — 2026-09-16
Starting/current HEAD 2ab3711e1c4033110be7ddff72af79913c1d0915. Worker /root/ideux04_terra_initial exited; no task build remains. One completed Terra failure, zero Sol. Next Terra High retry1/2.
Exact command: ./scripts/desktop-gradle.sh test --tests 'io.miniorca.desktop.ProjectSummaryPaneTest' --tests 'io.miniorca.desktop.ProjectSummaryIssuesTest' --tests 'io.miniorca.desktop.MermaidRendererTest' --tests 'io.miniorca.desktop.DesktopVisualLayoutTest' -PvisualOutput="$PWD/desktop/build/reports/ide-ux/summary"
Exit1 in compileKotlin, before tests or new Summary renders. Worker reports type inference errors in ProjectSummaryPane.kt around323–324 and365–366: nullable let(::SummaryArchitecture), let(::SummaryFlows), let(::SummaryModulesSection), let(::SummaryEngineeringInsight) refer to composables with default Modifier parameters. Expected stacked composable calls; compiler cannot infer/adapt these callable references. Replace with explicit lambdas supplying the required argument. No correction attempted after failure; targeted spotlessApply and git diff --check passed.
Retained actual task deltas, verified against snapshot: ProjectSummaryPane.kt primary/supporting ordering and equal columns; ProjectSummaryVisuals.kt unboxed coverage row; DesktopVisualLayoutTest.kt corresponding hierarchy assertion. The other four card target files retain prior baseline only (despite worker listing all seven dirty files). No passing candidate tests or renders.
Coordinator review found an unimplemented card requirement still pending: AnalysisCategoryPanels.kt and ProjectSummaryIssues.kt are unchanged from baseline; analysisCategoryBoxColors still uses semantic tinted fill and border, and there is no small accent edge. Complete the required equal neutral category surfaces with restrained edge accent using the existing shared component and palette. Preserve whole-surface keyboard/pointer behavior, hover/focus visibility and meaningful colors; do not claim the requirement already met. Existing visual color assertion calls analysisCategoryBoxColors(SecondaryText). Review retained coverage/layout assertions against the new compact row before verification. Preserve existing update-order/count tests and add behavior coverage only where missing.
Baseline snapshot /var/folders/lz/20cqfx4x2k98r89q68w3q3ch0000gn/T/mini-orca-ideux04-baseline-my1ejy0c. New render target desktop/build/reports/ide-ux/summary/ not produced by initial compile failure. Remaining: full card implementation, exact focused command, full ./scripts/desktop-gradle.sh test spotlessCheck detekt, diff check and actual wide/narrow/short/150% production render review. No native/package claim. Existing dirty MOCK work and user Gradle app preserved; no04commit. Retry1 then retry2 permitted before Sol initial+two retries.

IDEUX-04 Terra High retry1 failure — 2026-09-16
HEAD2ab3711e1c4033110be7ddff72af79913c1d0915 unchanged. Worker /root/ideux04_terra_retry1 exited; no task check active. Terra initial+retry1 failed (2 total), zero Sol. Next Terra retry2/2; then Sol initial on failure.
Fixed four callable references with explicit composable lambdas, resolving initial compilation. Added shared neutral Panel background, PaneSeparator border, ControlHover hover and4dp semantic top edge. Candidate compilation passed; spotlessApply and git diff --check passed. Exact focused command from initial packet exited1: ProjectSummaryPaneTest14/14, ProjectSummaryIssuesTest5/5, MermaidRendererTest4/4, DesktopVisualLayoutTest46/53;69/76 passed,7failures,0errors/skips.
Six failures use stale right-edge status geometry in assertSummaryStatusPlacement: summaryPanelUsesStatusColorsAndPlainStatusLabels(Updated), summaryDashboardShowsGroupedInterpretationWithDiagramDisclosures(Outdated), summaryMetricFlowKeepsEveryCoverageBoxAndPartialStateVisible(Paused), summaryDashboardAdaptsToNarrowShortAndLargeTextViews(Outdated), summaryDashboardOmitsEmptyCoverageButRetainsUnavailableCounts(Coverage unavailable), summaryCategoryBoxesNavigateAndRefreshFromTheCurrentRun(Updating). Each says status must align to right edge of Analysis coverage; new wide row places status next to label, with bar and View analysis following. Preserve within-coverage/below-identity/no-overlap/reachability assertions and adapt actual wide/stacked geometry; do not delete the placement check.
Seventh failure: categoryBoxesExposeNamesAndSingleKeyboardActions, View Performance results has Rect(0,0,0,0) in480x650/150%. Coordinator inspected new inner Column in AnalysisCategoryBox: Modifier.fillMaxWidth().fillMaxHeight() can greedily consume the available height in a vertically stacked fixture, clipping later cards. Investigate removing the unnecessary inner fillMaxHeight while retaining outer equal-height row behavior and minimum height. Do not weaken visible click/keyboard assertions. Also remove unused tint parameter from analysisCategoryBoxColors if no longer needed, updating its two caller sites including the listed visual test; no wrapper for a constant rule.
Only task candidate changes retained. Partial images under desktop/build/reports/ide-ux/summary/ exist but no acceptance review; full gate not run. XMLs in desktop/build/test-results/test/TEST-io.miniorca.desktop.*.xml. Worker diff totals included earlier baseline; compare snapshot for actual task deltas. Remaining: layout repair plus meaningful test expectation update, exact focused/full gates,diff and actual wide/compact/short150% renders. Preserve all earlier dirty work and user desktop run. No04commit; retain complete initial+retry1 failure packets for retry2.

IDEUX-04 Terra High retry2 failure — 2026-09-16
HEAD2ab3711e1c4033110be7ddff72af79913c1d0915 unchanged. Worker /root/ideux04_terra_retry2 exited; no task check active. All three Terra attempts failed; zero Sol. Escalate to fresh Sol High initial with complete card and all packets.
Retry2 moved wide coverage status after View analysis, removed inner category fillMaxHeight, and removed obsolete tint parameter plus visual caller. spotlessApply and git diff --check passed. Exact focused command above exited1 after32s: SummaryPane14/14,SummaryIssues5/5,Mermaid4/4,Visual51/53;74/76 passed,2failures,0errors/skips. No full gate or acceptance review.
Failure1: roundedSummaryUsesTheProductionFrameAndSelectedSummaryDestination at DesktopVisualLayoutTest.kt:1864, assertTextBefore("Performance","Security") => Performance and Security must share a row. Helper at3031–3037 requires first.right<second.left and abs(centerY difference)<2; no numeric bounds emitted. Retry guessed responsive width, but coordinator notes new inner category Column dropped original Modifier.align(Alignment.Top) when adding top edge; after removing fillMaxHeight, parent IdeActionSurface Row may center differing content heights. Inspect actual bounds/render and restore top alignment if that is the cause, retaining true equal-width/responsive-row assertions.
Failure2: summaryStatusLightExposesFailureOnKeyboardFocus at:2064; after firstTab at2062, isDescriptionFocused("Project description: failed · Provider timed out.") is false. Wide coverage now orders View analysis before status, so Tab likely reaches that real button first. Keep coherent visual/keyboard order and test actual keyboard reachability of failure detail; do not add artificial focus-order plumbing just to satisfy an old firstTab assumption. Either preserve the intended existing traversal with natural layout or update the meaningful keyboard test to current reachable order while retaining failure disclosure evidence. No numeric bounds.
Actual task deltas since pre04: ProjectSummaryPane.kt,ProjectSummaryVisuals.kt,AnalysisCategoryPanels.kt,DesktopVisualLayoutTest.kt. Other dirty target files are earlier baseline. Partial captures desktop/build/reports/ide-ux/summary/ and XML desktop/build/test-results/test/TEST-io.miniorca.desktop.DesktopVisualLayoutTest.xml. Remaining: fix real label alignment, verify accessible status traversal, exactfocused/fullgate,diff and complete production render inspection. Preserve candidate, all earlier dirty work and user's Gradle app; no04commit. Sol initial then two retries remain.

Sol initial passed: focused76/76 and full502/502 working-tree tests, Spotless/Detekt,
diff and actual wide/compact/short/150% render review. Coordinator independently
checked counts, unchanged unrelated baseline hashes and wide/large-text images.
Proposed commit includes reviewed existing Summary/category foundations and their
relevant tests, shared workspace text/section helpers, styled prose and diagram
headings; it excludes unrelated frame/native baseline. Isolated export gate running
in session88477. Complete Summary regression methods and helpers are included,
while the pre-existing rounded frame fixture/test graph remains for IDEUX-13.

The first isolated export gate exited1 on Spotless blank-line differences introduced
by the coordinator's selective test extraction (not the passing working-tree
candidate). Formatting only those extracted lines in the temporary export resolved
that check. The proposed index was updated to the formatted bytes; session99855
is running the full export gate. Worker retry counters remain3Terra/0failedSol.

The second export gate exposed two incomplete test extractions (old tooltip helper
signature and a superseded renamed priority test). The third compiled but failed
seven Summary assertions: the export omitted the existing completed-run fixture
state and the baseline Summary capture expectations. Included these reviewed
fixture prerequisites, removed both superseded renamed methods, and copied the
matching hover helper. No working-tree application or candidate changes were
made; this is coordinator commit assembly, not an additional worker attempt.
Full export gate now running in session11658 with the complete Summary fixture.

Final isolated export session11658 passed475/475 tests, Spotless/Detekt.
All acceptance evidence recorded on IDEUX-04; no working-tree code changed during
commit assembly. Ready to commit, verify hash and immediately dispatch IDEUX-05.
