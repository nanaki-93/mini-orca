# IDEUX execution handoff

Follow [procedure](README.md), [queue](../docs/tasks.md), root and area instructions.
One worker/card at a time; immediately continue after accepted verified commits.

| Field | Value |
| --- | --- |
| Card | IDEUX-07 — Add local filters and stable result navigation |
| Status | Accepted; commit pending |
| Stage | Coordinator selective commit |
| Model | `gpt-5.6-sol` |
| Reasoning | `high` |
| Tier attempt | Sol retry2/2 |
| Completed failed attempts for this card | 3 Terra; 2 Sol |
| Active agent/process | None; final worker/checks exited |
| Starting HEAD | `f06d5db4061c738f9f2b92114fb297ab088bf871` |
| Task commit | Pending local commit; 77/512/491 tests and quality passed |
| Last accepted task commit | IDEUX-06: `f06d5db4061c738f9f2b92114fb297ab088bf871`, verified |
| Next permitted attempt on failure | Pause; retries exhausted |

## Boundaries and baseline

Preserve current colors, six48dp icon-onlyrail, simple productionIDE hierarchy,
readonlysource/diff,isolateddraft,consent/trust,Review/Apply/Undo. References under
`design/ui-mocks/ide-reference-2026-09-16/` inform composition only. Coordinator owns
admin/acceptance/staging/commits; worker stops after first completed failed candidate
verification or substantive acceptance gap; no hidden repair rounds.

01–06accepted/committed:01 `3fa4929`,02 `fd46823`,03 `2ab3711`,04 `5011783`,
05 `6db69ec`,06 `f06d5db`. User-authorized Astra low repair resolved06 after its
sixTerra/Sol failures. All70focused/506working-tree/485isolatedcommit tests and
Spotless/Detekt/diff/productionrenderreviewpassed. Full failure history retained in
`docs/errors.log` and06receipt. Prior counters do not transfer to this new card;
TerraHighinitial+2retries thenSolHighinitial+2retries is unchanged.

Snapshot `/var/folders/lz/20cqfx4x2k98r89q68w3q3ch0000gn/T/mini-orca-ideux07-baseline-pjji9_ej`
holds37dirtybaselinefilecopies/hashes and startingHEAD. Compare actual delta to it;
useHEADfororiginallycleanfiles. ExistingacceptedMOCKwork remainsdirty,indexempty.
Preserveunrelatedbytes; never stageeverything. Necessaryreviewedprerequisitesmay be
selectivelycommitted with attribution and isolated-commit verification. Keepuserapp
PID62979open and residentGradledaemonsuntouched.

Remainingnative/fullframefixturecarry until13: DesktopAcceptanceFixture graph and
IDEUX03modecallback; DesktopVisualLayoutTest nativeframegraph and05roundedAnalysis
headingassertion. The06gutterregression/wait/draghelper and independent resultlayout
regressions are nowcommitted; don'tcarry orduplicate those again.

## Current task notes

Fullcardin docs/tasks.md. Added DesktopVisualLayoutTest to07targets because prior
result-page tests explicitly prohibit a filter field and expect manual firstentry;
update those assertions to the accepted07contract without weakening interaction,
readability or zeroexternalactionproof. Counts are loaded-rowcounts, not server
reported totals; unknown/criticalvaluesstayhonest. State survives workspacechanges,
resets onproject/revision/runreplacement; localnavigationneverprepares/applies.

Two07Terra failures recorded below. Onfailure retaincandidate,recordexactcommands/outputs,expected vs
observed,delta/diagnosis/attemptedfix/evidence/remainingchecks; inject fullcard plus
all failures intofreshretryafterpreviousworkerexits. Onacceptancecommit,verifyhash
thenimmediatelydispatch08atTerraHighinitial. Pauseonlyonretryexhaustion,external
blocker,userpauseorqueuecompletion.

Heartbeat `mini-orca-ux-implementation` confirmed ACTIVE through the app after06
commit verification; saved clock schedule and prompt preserved.

## Complete failure packet

IDEUX-07 Terra High initial failure — 2026-09-16
Starting/current HEAD f06d5db4061c738f9f2b92114fb297ab088bf871; worker /root/ideux07_terra_initial and check exited. One Terra failure, zero Sol; next Terra retry1/2.
Exact command: ./scripts/desktop-gradle.sh test --tests 'io.miniorca.desktop.ResultBrowserStateTest' --tests 'io.miniorca.desktop.ResultWorkspaceLayoutTest' --tests 'io.miniorca.desktop.DesktopKeyboardNavigationTest' --tests 'io.miniorca.desktop.DesktopVisualLayoutTest' -PvisualOutput="$PWD/desktop/build/reports/ide-ux/filters"
Exit1 after compilation:75tests,74passed,1failure,0errors/skips. ResultBrowserState3/3,ResultWorkspaceLayout6/6,Keyboard11/11,Visual54/55. Failure resultToolsAndToolWindowHeadersKeepInteractionLocalAtNarrowScale at DesktopVisualLayoutTest.kt2607: expected no editable controls after expanding Bugs verified checks, but the new local Filter results field is intentionally editable. Replace this obsolete blanket assertion with scoped filter identity/editability and zero external-action proof; do not weaken interaction checks.
Ten actual candidate targets retained: ResultBrowserState.kt(new),AnalysisResultsPane.kt,FindingsPresentation.kt,DesktopShell.kt,WorkspacePanes.kt,PerformanceWorkspace.kt,SecurityWorkspace.kt,ResultBrowserStateTest.kt(new),ResultWorkspaceLayoutTest.kt,DesktopVisualLayoutTest.kt. Typed store scopes project/revision/full run identity/category and clears old scopes; shell resets unconditionally before conditional content. Loaded facet counts, local title/path query/Clear filters, visible Impact, unknown/critical values and reported-vs-loaded counts added. Responsive selection and list state/keyboard wiring retained. All other baseline hashes unchanged.
Planned preliminary diagnostics before card verification: subset initially compile-failed missing KeyEvent.key/type extension imports, corrected; next subset found incorrect shared-category test expectation and offscreen starting focus after filter clearing; corrected to category-local state and reveal initial row as setup. Subset then passed. No edits after the completed exact candidate failure.
Coordinator review concerns to finish in retry: focusedKey initializes to selectedKey on page return and unconditionally animateScrollToItem; onClick/onFocusChanged also drive it, potentially overwriting saved nonzero index/offset on inspection or return. Separate explicit keyboard reveal from restored selection/focus; scroll only for navigation as needed. Add real leave/re-enter composition test with selected row and exact saved index plus nonzero offset. Prove arrow navigation itself reveals an initially offscreen target, then Enter/Space inspect, without fixture reveal after the key. Verify shared padded-width split near threshold and global scope reset even while landing/project modes bypass result panes.
git diff --check passed. Full desktop/Spotless/Detekt gate and actual render acceptance not run after failure. Candidate renders at desktop/build/reports/ide-ux/filters unaccepted. No07commit. Baseline snapshot /var/folders/lz/20cqfx4x2k98r89q68w3q3ch0000gn/T/mini-orca-ideux07-baseline-pjji9_ej. Preserve original appPID62979; next retry receives full card plus this packet.

IDEUX-07 Terra High retry1 failure — 2026-09-16
HEAD f06d5db4061c738f9f2b92114fb297ab088bf871 unchanged; worker and exact focused command exited1 after6m09:74completed,1failed,1skipped. Two Terra failures,zeroSol; next Terra retry2/2.
Command: ./scripts/desktop-gradle.sh test --tests 'io.miniorca.desktop.ResultBrowserStateTest' --tests 'io.miniorca.desktop.ResultWorkspaceLayoutTest' --tests 'io.miniorca.desktop.DesktopKeyboardNavigationTest' --tests 'io.miniorca.desktop.DesktopVisualLayoutTest' -PvisualOutput="$PWD/desktop/build/reports/ide-ux/filters"
Independent assertion failure leavingAndReturningKeepsSelectedRowAndExactLazyListPosition at ResultWorkspaceLayoutTest.kt161: expected saved offset9, observed16; selectedfinding20/index matched. Later filteringClearsCompactDetailAndArrowKeysKeepLongListsNavigable stalled. jcmd52918 Thread.print -l reported Found1deadlock: AWT-EventQueue-0 @coroutine#60534 blocked FlushCoroutineDispatcher.performRun:143 on ComposeUI SynchronizedObject0x15a1fcf00, holding runtime SynchronizedObject0x15a1fe620 through SnapshotFlow unregisterApplyObserver/Snapshot apply notifications. Only tail160 retained; counterpart missing, no dump file. Task executor52918 terminated SIGTERM143; userapp62979 preserved. Do not overclaim root cause.
Candidate separated keyboardFocusKey from restored selection, scoped Filterresults editability assertion, added retention/offscreen tests. No post-verdict edits. Diffcheck passed; full desktop quality and post-change visual acceptance unrun. Earlier three production renders predate edits and are not final acceptance.
Next repair: exact offset persistence without weakening saved-index/offset contract; diagnose deadlock and keyboard focus/scroll cancellation; include browser identity in AnalysisResultsPane selection LaunchedEffect so equal-row new scopes initialize; assert offscreen keyboard target actually visible without post-key fixture reveal; exercise actual no-match query and Clear filters recovery with zero external actions. Retain all initial failure context and baseline. No07commit/indexempty.

IDEUX-07 Terra High retry2 failure — 2026-09-16
HEADf06d5db unchanged; completed3Terra/0Sol failures, nextSolHighinitial. Exactfocused PASS76tests (Browser3/Workspace7/Keyboard11/Visual55),0fail/errors/skips, daemon Build4b9cb8df BUILD SUCCESSFUL34s at21:54:53. No deadlock. Preliminary diagnostic10tests initially9pass/1offsetfailure; observing isScrollInProgress established old offset9 was interim and16 settled. Same scoped LazyListState now retained instead of duplicated index/offset; bounded observable settle before exactoffset capture, testpasses. Keyboard scrollToItem replaces cancellation-prone animate; no-match/Clear/offscreenvisibleSpace and zeroopenAnalysis assertionspass. Browseridentity added to selection effect, revision/null-page resets covered. Direct composed equal-row scope swap nottested; coordinator accepted code+unit coverage for this narrow effect key correction.
Full ./scripts/desktop-gradle.sh test spotlessCheck detekt exit1 after23s atspotlessKotlinCheck: unformatted AnalysisResultsPane,FindingsPresentation,ResultBrowserState,ResultWorkspaceLayoutTest,DesktopVisualLayoutTest; testscompleted beforefailure; Detektunrun. No post-verdict edits/diffcheck. Apply scopedformatting BEFOREnextcandidateverification, then rerunexactfocused/fullgate/diff. Do not format unrelated dirty baseline.
Reviewed production images desktop/build/reports/ide-ux/filters/results-bugs-1440-1.0.png,results-bugs-800-1.0.png,results-performance-1440-1.0.png,rounded-security-1280-600-1.5.png: localfilters,compactlist,Impact,2loaded·1reported legible. Remaining required unknown/critical/no-match/short150% evidence needs review, no completeacceptance yet. Preserve existing app62979/baseline; no07commit. All prior packets remain applicable. Coordinator selectivecommit assembly scratch notaccepted/tested; reset/rebuild aftercandidateaccepted.

IDEUX-07 Sol High initial failure — 2026-09-16
HEADf06d5db unchanged. ThreeTerra/oneSol failures; nextSolretry1/2. Scopedformat only five violationtargets, unrelatedKotlinhashesunchanged. Exactfocusedexit0 BUILD SUCCESSFUL36s76/76passed. FulltestspotlessCheckdetekt exit1 after23s:511/511tests(52suites)pass,Spotlesspass,Detekt oneLongParameterList DesktopShell.kt921 DesktopCanvas13parameters vs12threshold (63files). No post-verdictcorrection. Suggested boundedfix useexistingDesktopShellState instead separateappState/layout/editor/context andderiveinside, retain shell-ownedResultBrowserStore; no suppression/thresholdchange. Diffcheckpass; no finalrenderacceptance/stage/commit. AppPID62979absent onworkerfinalread-onlycheck, workerdenieskillingit; residentdaemonsuntouched. Do not assume applicationrunning now. Preserve baseline. Requirednext exactfocused/fullgate/remainingactualrenderacceptance; preserveallpriorfailurepackets.

IDEUX-07 Sol High retry1 visual failure — 2026-09-16
HEADf06d5db unchanged;3Terra/2Sol failures, nextFINALSolretry2/2. DesktopCanvas uses existingDesktopShellState andderivesapp/layout/editor/context, preserves scopedbrowserstore. Addedproduction800x650@150%Critical/Unknown/no-match/Clearfixture. Focusedexit0 33s77/77;fulltestspotlessCheckdetekt exit0 21s512/512,Spotlesspass,Detekt63files0smells;diffpass. Actualimage desktop/build/reports/ide-ux/filters/results-filters-critical-unknown-800-150.png: Unknownchip correctlyUnknown1 but Finding2row rawblankseverity yields emptybadge. This violates honestunknownrepresentation. Stopatvisualverdict;nofurtheredits/commit. Fixexistingrowbadgelabel viaexistingfacet/presentationnormalization,assertvisibleUnknownrowbadge,scopedformatbeforeexact/fullgate/diff/renderreview. OthernomatchClear/Impact/countlag/widecompactshort150reviewpassed. Preserveallpriorcandidate/baseline. NextfailureexhaustsSol,pauseautomation andreport,no08dispatch.
