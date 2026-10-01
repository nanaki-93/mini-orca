package io.miniorca.desktop

import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.state.ToggleableState
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

class DesktopAccessibilityTest {
  @Test
  fun sourceRecoveryWithoutFilesAndCreationKeepNamedKeyboardActionsAcrossReflow() {
    val evidence = sourceNavigationReviewFixture()
    val file = requireNotNull(evidence.selected)
    val symbol = requireNotNull(evidence.selectedSymbol)
    val destination = "internal/" + "replacement/日本語/".repeat(8) + "user.go"
    val read = FileReadUiState.Failed(destination, "Local read unavailable.")
    val preferred = DesktopLayoutState(leftToolWindowVisible = false)
    var creations = 0
    val reopened = mutableListOf<String>()
    var privileged = 0
    ComposeVisualFixture(1280, 600, 1.5f) {
          AdaptiveProductionEditorFixture(
              preferred,
              false,
              evidence = evidence,
              fileRead = read,
              terminalCollapsed = true,
              onCreate = { creations++ },
              onOpenFile = { reopened += it },
              onRequest = { privileged++ },
              onWrite = { privileged++ },
              onSourceLine = { privileged++ })
        }
        .use { fixture ->
          fixture.render("f23-accessibility-hidden-files-before")
          assertEquals(0, fixture.tagCount("f04-files"))
          assertTrue(fixture.hasDescription("Project-relative path: ${file.path}"))
          assertTrue(fixture.hasDescription("Failed destination: $destination"))
          assertTrue(fixture.hasDescription("Selected declaration ${symbol.name} marker at line 5"))
          assertTrue(fixture.hasText("Read-only"))
          assertFalse(fixture.hasEditableText(withinTag = "source-viewport"))
          assertTrue(fixture.requestDescriptionFocus("Retry opening $destination"))
          fixture.awaitDescriptionFocus("Retry opening $destination")
          fixture.render("f23-accessibility-hidden-files-recovery-initial")
          for (width in listOf(800, 1280)) {
            fixture.resize(width, 600)
            fixture.render()
            fixture.awaitDescriptionFocus("Retry opening $destination")
            fixture.assertDescriptionFullyVisible("Retry opening $destination", "f04-canvas")
            fixture.render("f23-accessibility-hidden-files-recovery-$width")
            assertEquals(0, creations + reopened.size + privileged)
          }
          assertTrue(fixture.pressKey(Key.Enter))
          assertEquals(listOf(destination), reopened)
          assertTrue(fixture.requestDescriptionFocus("New function in ${file.path}"))
          fixture.render()
          for (width in listOf(800, 1280)) {
            fixture.resize(width, 600)
            fixture.render()
            fixture.awaitDescriptionFocus("New function in ${file.path}")
            fixture.assertDescriptionFullyVisible("New function in ${file.path}", "f04-canvas")
            fixture.render("f23-accessibility-hidden-files-creation-$width")
            assertEquals(0, creations + privileged)
          }
          assertTrue(fixture.pressKey(Key.Spacebar))
          assertEquals(1, creations)
          assertEquals(listOf(destination), reopened)
          assertEquals(0, privileged)
          assertEquals(file, evidence.selected)
          assertEquals(symbol, evidence.selectedSymbol)
        }
  }

  @Test
  fun compactContextExposesConsentBlockAndSelectableDetailsWithoutPassiveRequests() {
    val base = contextVisualState()
    val inspector = requireNotNull(base.inspector)
    val signature = "func Run(" + "context.Context, veryLongArgument string, ".repeat(7) + ") error"
    val longPath = "internal/" + "nested/".repeat(12) + "handler.go"
    val blocked =
        inspector.selectedSymbol!!.copy(
            signature = signature,
            editEligibility =
                SymbolEditEligibility(false, "Approximate grouped declaration cannot be edited."))
    var state by
        mutableStateOf(
            base.copy(
                inspector =
                    inspector.copy(
                        file = inspector.file.copy(path = longPath), selectedSymbol = blocked),
                functionModel =
                    ScopedModel(
                        scope = "function", model = "remote/editor", remoteProvider = true)))
    var privileged = 0
    ComposeVisualFixture(280, 400, 1.5f) {
          ContextToolWindow(
              state,
              ContextToolWindowActions(
                  { privileged++ },
                  { privileged++ },
                  { privileged++ },
                  { privileged++ },
                  { privileged++ },
                  confirmFunctionRemoteProvider = {
                    state = state.copy(functionRemoteProviderConfirmed = it)
                  },
                  explainSelected = { privileged++ }),
              androidx.compose.ui.Modifier.fillMaxSize())
        }
        .use { fixture ->
          fixture.render("f24-accessibility-blocked")
          assertTrue(fixture.hasText(blocked.editEligibility.blockedReason))
          fixture.assertEveryTextLineReachable(longPath, "context-content")
          fixture.revealText(longPath, "context-content")
          assertTrue(fixture.copyTextByDragging(longPath).isNotEmpty())
          fixture.revealText(blocked.editEligibility.blockedReason, "context-content")
          assertFalse(fixture.hasText("Explain declaration"))
          fixture.revealText("Declaration details", "context-content")
          assertTrue(fixture.requestDescriptionFocus("Expand Declaration details"))
          assertEquals("Collapsed", fixture.descriptionState("Expand Declaration details"))
          assertTrue(fixture.pressKey(Key.Enter))
          fixture.render()
          assertEquals("Expanded", fixture.descriptionState("Collapse Declaration details"))
          fixture.assertEveryTextLineReachable(signature, "context-content")
          assertEquals(0, privileged)
          state = state.copy(inspector = inspector)
          fixture.render("f24-accessibility-remote")
          fixture.revealText("Confirm remote destination", "context-content")
          assertFalse(fixture.isDescriptionDisabled("Confirm remote destination"))
          assertTrue(fixture.isDisabled("Explain declaration"))
          assertTrue(fixture.requestDescriptionFocus("Confirm remote destination"))
          fixture.render("f24-accessibility-consent-focus")
          fixture.assertColorVisible(FocusAccent)
          assertTrue(fixture.pressKey(Key.Spacebar))
          fixture.render()
          assertEquals(0, privileged, "Confirmation is not dispatch")
          assertFalse(fixture.isDisabled("Explain declaration"))
        }
  }

  @Test
  fun scanControlsExposeNamesDisabledReasonsAndDisclosureStates() {
    val cases = verifiedScanLayoutCases().filter { it.first != "long-output" }
    for ((name, state) in cases) {
      var starts = 0
      var cancels = 0
      var reads = 0
      ComposeVisualFixture(800, 650, 1.5f) {
            BugsWorkspacePane(
                state,
                BugsWorkspaceActions(
                    FindingActions({}, { _, _ -> }, {}),
                    { starts++ },
                    { cancels++ },
                    refreshScanStatus = { reads++ }))
          }
          .use { fixture ->
            fixture.render()
            val progress = verifiedScanProgress(state.project, state.scanState, state.scan)
            fixture.revealTextFullyWithin(progress.summary, "result-overview")
            assertTrue(fixture.hasText(progress.summary), name)
            val action =
                if (progress.action == VerifiedScanAction.Cancel) "Cancel checks"
                else "Trust project-code execution & run checks"
            fixture.revealTextFullyWithin(action, "result-overview")
            assertTrue(fixture.hasText(action), name)
            if (progress.action == VerifiedScanAction.Waiting) {
              assertTrue(fixture.isDisabled(action), name)
              assertFalse(
                  fixture.requestFocus(action), "Disabled execution must not accept focus: $name")
              assertEquals(0, starts)
              assertEquals(0, cancels)
            } else {
              assertFalse(fixture.isDisabled(action), name)
              assertTrue(fixture.requestFocus(action), name)
              fixture.render()
              assertTrue(fixture.isFocusedControl(action), name)
              assertTrue(fixture.pressKey(Key.Enter), name)
              assertEquals(if (progress.action == VerifiedScanAction.Start) 1 else 0, starts)
              assertEquals(if (progress.action == VerifiedScanAction.Cancel) 1 else 0, cancels)
            }
            assertEquals(0, reads)
            fixture.revealTextFullyWithin("Command and output", "result-overview")
            assertEquals("Collapsed", fixture.descriptionState("Expand Command and output"))
            assertTrue(fixture.requestDescriptionFocus("Expand Command and output"))
            assertTrue(fixture.pressKey(Key.Spacebar))
            fixture.render()
            assertEquals("Expanded", fixture.descriptionState("Collapse Command and output"))
            assertEquals(0, reads)
          }
    }
  }

  @Test
  fun bugsSelectionAndBlockedPreparationExposeDistinctAccessibleStates() {
    val page = resultPageFixture("bugs")
    val finding =
        page.semantic
            .first()
            .copy(
                title = "Inspect saved Bugs evidence",
                location = FindingLocation("main.go", startLine = 7, symbol = "Run"),
                freshness = "stale")
    val loaded =
        page.copy(
            section = page.section.copy(results = page.results!!.copy(semantic = listOf(finding))))
    var selected: UnifiedFinding? = null
    var prepared = 0
    ComposeVisualFixture(800, 650, 1.5f) {
          BugsWorkspacePane(
              BugsWorkspacePaneState(
                  listOf(finding), null, false, loaded, index = resultIndexFixture()),
              BugsWorkspaceActions(
                  FindingActions({ prepared++ }, { _, _ -> }, { selected = it }), {}, {}))
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.isDescriptionSelected("Inspect ${finding.title}"))
          assertTrue(fixture.requestDescriptionFocus("Inspect ${finding.title}"))
          assertTrue(fixture.pressKey(Key.Spacebar))
          fixture.render()
          assertTrue(fixture.isDescriptionSelected("Inspect ${finding.title}"))
          assertTrue(fixture.hasText("main.go:7 · Run"))
          assertTrue(fixture.hasText("Stale"))
          fixture.revealText("Prepare fix", "result-detail")
          assertTrue(fixture.isDisabled("Prepare fix"))
          assertTrue(fixture.hasText("Analyze again to prepare a fix from current source."))
          assertTrue(fixture.requestFocus("Open source"))
          assertTrue(fixture.pressKey(Key.Enter))
          assertEquals(finding, selected)
          assertEquals(0, prepared)
        }
  }

  @Test
  fun switchReviewsKeepSafeFocusAndEscapeOnlyDismissesTheActiveReview() {
    val draft = DeclarationDraft(id = "draft", targetPath = "cmd/main.go")
    val model = ScopedModel(scope = "analyze", remoteProvider = true, model = "remote-model")
    val context =
        ProjectSwitchContext(
            SwitchProjectIdentity(resultProjectFixture()),
            SwitchDraftIdentity(null, draft, editableDraft(draft)),
            SwitchAnalyzeDestination(model),
            false)
    val admission = ProjectSwitchAdmission()
    var pending by mutableStateOf(admission.choose("/next/a very long project path", context))
    var confirmed by mutableStateOf(false)
    var cancels = 0
    var imports = 0
    val shells =
        TerminalWorkspaceState(
            tabs = listOf(TerminalTabState(1, "Shell 1"), TerminalTabState(2, "Shell 2")))
    ComposeVisualFixture(800, 650, 1.5f) {
          pending?.let { current ->
            ProjectSwitchReviewDialog(
                current,
                model,
                confirmed,
                shells,
                SwitchCleanupFeedback(),
                onCancel = {
                  cancels++
                  admission.dismiss(current.requestId)
                  pending = admission.pending
                },
                onDraftApproved = {
                  admission.approveDraft(current.requestId, context)
                  pending = admission.pending
                },
                onProviderConfirmed = { confirmed = it },
                onProviderApproved = {
                  admission.approveProvider(
                      current.requestId, context.copy(analyzeConfirmed = confirmed))
                  pending = admission.pending
                },
                onReviewApproved = {},
                onCommit = { imports++ },
            )
          }
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.isFocusedControl("Cancel switch"))
          assertTrue(fixture.hasText("Requested project: /next/a very long project path"))
          assertTrue(fixture.hasText("Shell 2"))
          assertTrue(fixture.hasText("Approve draft discard for switch"))
          fixture.clickText("Approve draft discard for switch")
          fixture.render()
          assertEquals(SwitchReviewStage.Provider, pending?.stage)
          assertEquals(draft, pending?.context?.draft?.draft)
          assertTrue(fixture.isFocusedControl("Cancel switch"))
          assertTrue(fixture.hasText("Confirm remote destination"))
          assertFalse(fixture.isFocusedControl("Continue with provider"))
          assertTrue(fixture.pressKey(Key.Escape))
          fixture.render()
          assertNull(pending)
          assertEquals(1, cancels)
          assertEquals(0, imports)
          assertEquals(2, shells.tabs.size)

          pending = admission.choose("/next/a very long project path", context)
          fixture.render()
          fixture.clickText("Approve draft discard for switch")
          fixture.render()
          fixture.clickText("Confirm remote destination")
          fixture.render()
          assertTrue(confirmed)
          assertEquals(SwitchReviewStage.Provider, pending?.stage)
          assertEquals(0, imports)
          fixture.clickText("Continue with provider")
          fixture.render()
          assertEquals(SwitchReviewStage.Final, pending?.stage)
          assertEquals(draft, pending?.context?.draft?.draft)
          assertTrue(fixture.isFocusedControl("Cancel switch"))
          assertTrue(fixture.pressKey(Key.Escape))
          fixture.render()
          assertNull(pending)
          assertEquals(2, cancels)
          assertEquals(0, imports)
          assertEquals(2, shells.tabs.size)
        }
  }

  @Test
  fun localSwitchFinalReviewNeedsNoRemoteConsentAndEscapeKeepsTheProject() {
    val model = ScopedModel(scope = "analyze")
    val context =
        ProjectSwitchContext(
            SwitchProjectIdentity(resultProjectFixture()),
            SwitchDraftIdentity(null, null, null),
            SwitchAnalyzeDestination(model),
            false)
    val admission = ProjectSwitchAdmission()
    var pending by mutableStateOf(admission.choose("/next/local", context))
    var cancels = 0
    var confirmations = 0
    var imports = 0
    ComposeVisualFixture(800, 650, 1.5f) {
          pending?.let { current ->
            ProjectSwitchReviewDialog(
                current,
                model,
                false,
                TerminalWorkspaceState(),
                SwitchCleanupFeedback(),
                onCancel = {
                  cancels++
                  admission.dismiss(current.requestId)
                  pending = admission.pending
                },
                onDraftApproved = {},
                onProviderConfirmed = { confirmations++ },
                onProviderApproved = {},
                onReviewApproved = {},
                onCommit = { imports++ },
            )
          }
        }
        .use { fixture ->
          fixture.render()
          assertEquals(SwitchReviewStage.Final, pending?.stage)
          assertTrue(fixture.hasText("Current project: ${context.project?.path}"))
          assertTrue(fixture.hasText("Requested project: /next/local"))
          assertTrue(
              fixture.hasText(
                  "This Analyze destination is local; no remote confirmation is required."))
          assertFalse(fixture.hasText("Confirm remote destination"))
          assertTrue(fixture.isFocusedControl("Cancel switch"))
          assertTrue(fixture.pressKey(Key.Escape))
          fixture.render()
          assertNull(pending)
          assertEquals(1, cancels)
          assertEquals(0, confirmations)
          assertEquals(0, imports)
        }
  }

  @Test
  fun committedCleanupFailureOffersAnExplicitExitWithoutCallingSwitchAgain() {
    val model = ScopedModel(scope = "analyze")
    val context =
        ProjectSwitchContext(
            SwitchProjectIdentity(resultProjectFixture()),
            SwitchDraftIdentity(null, null, null),
            SwitchAnalyzeDestination(model),
            false)
    val admission = ProjectSwitchAdmission()
    val request = admission.choose("/next", context)!!.requestId
    admission.commit(request, context)
    var closes = 0
    var imports = 0
    var outstanding by mutableStateOf(true)
    admission.cleanupStarted(request)
    ComposeVisualFixture(800, 650, 1.5f) {
          ProjectSwitchReviewDialog(
              admission.pending!!,
              model,
              false,
              TerminalWorkspaceState(),
              SwitchCleanupFeedback(
                  "Shell cleanup did not finish in 15 seconds. The project has not been switched.",
                  outstanding),
              onCancel = {
                admission.finish(request)
                closes++
              },
              onDraftApproved = {},
              onProviderConfirmed = {},
              onProviderApproved = {},
              onReviewApproved = {},
              onCommit = { imports++ })
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText("Closing project shells…"))
          assertFalse(fixture.hasText("Close review"))
          assertFalse(fixture.hasText("Cancel switch"))
          fixture.pressKey(Key.Escape)
          assertEquals(0, closes)
          admission.cleanupSettled(request)
          outstanding = false
          fixture.render()
          assertTrue(fixture.hasText("Project switch stopped"))
          assertTrue(fixture.hasText("Close review"))
          fixture.clickText("Close review")
          assertEquals(1, closes)
          assertNull(admission.pending)
          assertEquals(0, imports)
        }
  }

  @Test
  fun savedReadFailureNamesRecoveryAndKeepsFilteredFailureAccessible() {
    AnalysisResultType.entries.forEach { type ->
      val page =
          resultPageFixture(type.category).let {
            it.copy(section = it.section.copy(error = "saved read failed"))
          }
      val browser = newResultBrowserState(page)
      browser.query = "no matching result"
      val rows =
          listOf(
              ResultRowPresentation(
                  "retained", "Retained result", "main.go:1", "Evidence", "high", "", ""))
      ComposeVisualFixture(800, 650, 1.5f) {
            AnalysisResultsPane(page, rows, browser, openAnalysis = {}, retryResults = {}) {
              androidx.compose.material.Text("Retained detail")
            }
          }
          .use { fixture ->
            fixture.render()
            assertTrue(fixture.hasText("No matching results."))
            assertTrue(fixture.hasText("Results could not be refreshed: saved read failed"))
            assertTrue(
                fixture.hasText(
                    "Reloads saved results for ${type.workspace.name}; does not start analysis."))
            assertTrue(fixture.hasText("Retry loading results"))
            assertTrue(fixture.hasDescription("Clear filters"))
          }
    }
  }

  @Test
  fun reviewAttemptRetainsFailureAndDisabledReasonWithHelpCollapsed() {
    val base = editorComparisonReviewFixture()
    val draft = requireNotNull(base.draft)
    val running =
        base.copy(
            checkAttempt = CheckAttempt(6, CheckCandidate(draft), ValidationAttemptStatus.Running))
    ComposeVisualFixture(800, 650, 1.5f) {
          ReviewToolWindow(
              running, ReviewToolWindowActions({}, {}, {}), DraftApplicationActions({}, {}))
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText("Checks running"))
          assertTrue(fixture.hasText("Previous check report (retained; not current approval)"))
          assertFalse(fixture.hasText("Ready to apply"))
          assertFalse(fixture.hasText("Apply change"))
        }
  }

  @Test
  fun resultPagesStartKeyboardNavigationAtViewAnalysis() {
    AnalysisResultType.entries.forEach { current ->
      ComposeVisualFixture(1_000, 760, 1.25f) { AcceptanceResultPane(current.category, "partial") }
          .use { fixture ->
            fixture.render()
            assertTrue(fixture.pressKey(Key.Tab))
            fixture.render()
            assertTrue(fixture.isFocused("View analysis"))
          }
    }
  }

  @Test
  fun summaryCategoriesExposeOneNamedActionAndDecorativeIcons() {
    ComposeVisualFixture(1_600, 1_000) {
          ProjectSummaryPane(visualFixtureOverview, visualFixtureProject, {})
        }
        .use { fixture ->
          fixture.render()
          AnalysisResultType.entries.forEach { type ->
            val action = "View ${type.workspace.name} results"
            assertEquals(
                1,
                fixture.clickableDescriptionCount(action),
                "${type.workspace.name} must expose exactly one navigation action")
            assertTrue(
                fixture.tagIsDecorative("summary-category-icon-${type.category}"),
                "${type.workspace.name} icon must not add a second announcement")
          }
        }
  }

  @Test
  fun summaryExposesNamedLocalDisclosuresAndVisibleNonColorInterpretationState() {
    val overview =
        visualFixtureOverview.copy(
            analysis =
                visualFixtureOverview.analysis.copy(
                    status = "stale",
                    failure = "The saved project description is from an older revision.",
                    engineeringInsight =
                        EngineeringInsight(
                            mechanism = "Validate requests before persistence.",
                            whyItMattersHere = "Invalid input stays outside the repository.",
                            tradeoffOrFailureMode = "Rules need one owner.")))
    ComposeVisualFixture(1_600, 1_000) { ProjectSummaryPane(overview, visualFixtureProject, {}) }
        .use { fixture ->
          fixture.render()
          assertEquals(1, fixture.textCount("Summary"))
          assertEquals(
              listOf(
                  "Summary",
                  "go-shop · fixture",
                  "Analysis coverage",
                  "File evidence",
                  "Architecture",
                  "Engineering insight",
                  "More insight",
                  "Selected findings"),
              fixture.semanticHeadingTexts(),
              "Visible Summary headings must follow the page's reading order")
          assertTrue(fixture.hasText("Project description: stale · source may have changed"))
          assertTrue(fixture.hasText("Outdated"))
          assertTrue(fixture.hasText("View analysis"))
          fixture.awaitDescription("Expand Architecture diagram", "Preview")
          fixture.revealText("Flows", "summary-scroll")
          fixture.awaitDescription("Expand Flow 1 diagram", "Preview")
          assertEquals("Collapsed", fixture.descriptionStateDescription("Expand More insight"))
          fixture.revealText("Change lifecycle", "summary-scroll")
          assertEquals(
              listOf("Flows", "Change lifecycle"), fixture.semanticHeadingTexts().takeLast(2))
        }
  }

  @Test
  fun soleTerminalControlAnnouncesItsStateAndActivatesFromTheKeyboard() {
    var opens = 0
    ComposeVisualFixture(800, 100, 1.5f) {
          TerminalBar(
              TerminalWorkspaceState(),
              collapsed = true,
              onToggle = { opens++ },
              tabActions = TerminalTabActions({}, {}, {}))
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasDescription("Open terminal · Ctrl+Shift+T"))
          assertEquals("Collapsed", fixture.stateDescription("Terminal"))
          assertEquals(0, opens)
          assertTrue(fixture.requestFocus("Terminal"))
          assertTrue(fixture.pressKey(Key.Enter))
          assertEquals(1, opens)
          listOf("Bugs & Problems", "Checks", "Output").forEach { assertFalse(fixture.hasText(it)) }
        }
  }

  @Test
  fun packageOnlyCreationHasAnExplicitNameAndKeepsSourceReadOnly() {
    val file =
        ProjectFileInfo(
            "empty.go",
            "base",
            "empty.go",
            language = "Go",
            sizeBytes = 13,
            lineCount = 1,
            modifiedAt = "",
            binary = false,
            content = "package main\n")
    var requests = 0
    ComposeVisualFixture(800, 100, 1.5f) {
          NewFunctionButton(file.path, declarationCreationBlockedReason(file), { requests++ })
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasDescription("New function in empty.go"))
          assertTrue(fixture.requestFocus("New function"))
          assertTrue(fixture.pressKey(Key.Enter))
          assertEquals(1, requests)
          assertEquals("package main\n", file.content)
        }
  }

  @Test
  fun disappearingPauseKeepsKeyboardFocusOnReadOnlyStatusRecovery() {
    val run = analysisRunFixture().copy(status = "running")
    var analysis by mutableStateOf(ProjectAnalysisRunState(run = run))
    var pauses = 0
    var refreshes = 0
    var otherActions = 0
    ComposeVisualFixture(800, 440, 1.5f) {
          AnalysisWorkspacePane(
              AnalysisWorkspacePaneState(resultProjectFixture(), analysis),
              AnalysisWorkspaceActions(
                  { _, _ -> otherActions++ },
                  { pauses++ },
                  { otherActions++ },
                  { otherActions++ },
                  { otherActions++ },
                  refreshStatus = { refreshes++ }))
        }
        .use { fixture ->
          fixture.render()
          fixture.revealText("Pause", "analysis-page")
          assertTrue(fixture.hasDescription("Pause analysis at the next stage boundary"))
          assertTrue(fixture.requestDescriptionFocus("Pause analysis at the next stage boundary"))
          assertTrue(fixture.pressKey(Key.Enter))
          assertEquals(1, pauses)
          analysis = analysis.copy(run = run.copy(status = "pausing"))
          fixture.render()
          assertFalse(fixture.hasDescription("Pause analysis at the next stage boundary"))
          assertTrue(fixture.isDescriptionFocused("Refresh analysis run status"))
          assertEquals(0, refreshes + otherActions)
          analysis =
              analysis.copy(
                  run = run.copy(status = "paused"),
                  error = "Status could not be read",
                  errorKind = AnalysisRunErrorKind.StatusRead,
                  statusUnavailable = true)
          fixture.render()
          assertTrue(fixture.isDescriptionFocused("Refresh analysis run status"))
          assertTrue(fixture.pressKey(Key.Spacebar))
          assertEquals(1, refreshes)
          assertEquals(0, otherActions)
        }
  }

  @Test
  fun statusRecoveryAndFileRefreshRemainSeparateKeyboardActionsAtReducedHeight() {
    val state =
        AnalysisWorkspacePaneState(
            resultProjectFixture(),
            ProjectAnalysisRunState(
                run = analysisRunFixture().copy(status = "paused"),
                error = "Status unavailable after cancel",
                errorKind = AnalysisRunErrorKind.StatusRead,
                statusUnavailable = true,
                fileSelection = AnalysisSelectionState(selectionFixture())))
    var statusReads = 0
    var fileReads = 0
    var privileged = 0
    ComposeVisualFixture(800, 440, 1.5f) {
          AnalysisWorkspacePane(
              state,
              AnalysisWorkspaceActions(
                  { _, _ -> privileged++ },
                  { privileged++ },
                  { privileged++ },
                  { privileged++ },
                  { privileged++ },
                  { fileReads++ },
                  refreshStatus = { statusReads++ }))
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText("Status unavailable after cancel"))
          assertTrue(fixture.requestDescriptionFocus("Refresh analysis run status"))
          assertTrue(fixture.pressKey(Key.Enter))
          assertEquals(1, statusReads)
          assertEquals(0, fileReads + privileged)
          fixture.revealText("Refresh files", "analysis-page")
          assertTrue(fixture.requestFocus("Refresh files"))
          assertTrue(fixture.pressKey(Key.Spacebar))
          assertEquals(1, fileReads)
          assertEquals(1, statusReads)
          assertEquals(0, privileged)
        }
  }

  @Test
  fun analysisFilesControlsExposeDisclosureFilterAndLockedSelectionStates() {
    val run =
        analysisRunFixture()
            .copy(
                status = "running",
                plan =
                    analysisPreviewFixture()
                        .copy(
                            files =
                                listOf(
                                    AnalysisPlannedFile(
                                        "helper.go",
                                        "helper",
                                        "Go",
                                        20,
                                        listOf(
                                            AnalysisStagePlan(
                                                "semantic", true, false, maxModelRequests = 0))))),
                files =
                    listOf(
                        AnalysisRunFile(
                            "helper.go",
                            "helper",
                            "Go",
                            listOf(AnalysisStageProgress("semantic", "running", 1, false)))),
                sections = analysisRunFixture().sections.map { it.copy(status = "running") },
            )
    ComposeVisualFixture(1_600, 1_000) {
          AnalysisWorkspacePane(
              AnalysisWorkspacePaneState(
                  resultProjectFixture(),
                  ProjectAnalysisRunState(
                      run = run, fileSelection = AnalysisSelectionState(selectionFixture()))),
              AnalysisWorkspaceActions(
                  { _, _ -> },
                  {},
                  {},
                  {},
                  {},
                  refreshStatus = { error("Unexpected status refresh") }),
          )
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasDescription("Collapse Files"))
          assertEquals("Expanded", fixture.stateDescription("Files"))
          assertTrue(fixture.hasDescription("Filter files"))
          assertTrue(fixture.hasText("Search file paths"))
          assertTrue(fixture.isDescriptionSelected("All"))
          assertTrue(fixture.hasDescription("Analyze helper.go"))
          assertTrue(fixture.isDescriptionDisabled("Analyze helper.go"))
          assertTrue(fixture.hasText("Up to date"))
          assertEquals(
              "Selected for analysis", fixture.descriptionStateDescription("Analyze helper.go"))
          assertEquals(ToggleableState.On, fixture.descriptionToggleableState("Analyze helper.go"))
          assertTrue(fixture.hasDescription("Files finished: 0 of 1"))
          assertTrue(
              fixture.hasText(
                  "Selection locked. Finish or cancel the current run to change files."))
        }
  }

  @Test
  fun lockedCollapsedFilesRetainRefreshAndErrorRecoveryWithoutMutations() {
    val selection = selectionFixture().copy(editable = false)
    var reads = 0
    var writes = 0
    var admissions = 0
    var state by mutableStateOf(AnalysisSelectionState(selection))
    ComposeVisualFixture(800, 650, 1.5f) {
          AnalysisFileSelector(
              ProjectAnalysisRunState(fileSelection = state),
              AnalysisWorkspaceActions(
                  { _, _ -> admissions++ },
                  { admissions++ },
                  { admissions++ },
                  { admissions++ },
                  { admissions++ },
                  { reads++ },
                  { writes++ },
                  refreshStatus = { error("Unexpected status refresh") }))
        }
        .use { fixture ->
          fixture.render()
          fixture.clickDescription("Collapse Files")
          fixture.render()
          assertTrue(fixture.hasText("Selection changes unavailable."))
          assertTrue(
              fixture.hasText(
                  "File selection is independent of the open Editor file and does not start analysis."))
          assertTrue(fixture.requestFocus("Refresh files"))
          assertTrue(fixture.pressKey(Key.Enter))
          state =
              state.copy(
                  error = "Saved selection uncertain", failure = AnalysisSelectionFailure.Save)
          fixture.render()
          assertTrue(fixture.hasText("Saved selection uncertain"))
          assertTrue(fixture.hasText("Could not save file selection"))
          assertTrue(fixture.requestFocus("Refresh files"))
          assertTrue(fixture.pressKey(Key.Enter))
          fixture.clickDescription("Expand Files")
          fixture.render()
          assertTrue(fixture.hasDescription("Filter files"))
          fixture.scrollBy(100_000f, "analysis-file-table")
          fixture.render()
          assertTrue(fixture.isDescriptionDisabled("Analyze main.go"))
          assertEquals(2, reads)
          assertEquals(0, writes)
          assertEquals(0, admissions)
        }
  }

  @Test
  fun analysisFileStatusMarkersAreDecorativeAtFullSize() {
    ComposeVisualFixture(1_600, 1_000, 1.5f) {
          AnalysisFileSelector(
              ProjectAnalysisRunState(fileSelection = AnalysisSelectionState(selectionFixture())),
              AnalysisWorkspaceActions(
                  { _, _ -> },
                  {},
                  {},
                  {},
                  {},
                  refreshStatus = { error("Unexpected status refresh") }))
        }
        .use { fixture ->
          fixture.render()
          listOf(".env", "helper.go", "main.go").forEach { path ->
            assertTrue(
                fixture.tagIsDecorative("analysis-file-status-marker-$path"),
                "$path status marker must not add accessible meaning")
          }
        }
  }

  @Test
  fun reflowedFileRowsKeepSelectableTextAndLocalDetailsSeparateFromSelection() {
    val path = "internal/services/identity/long-request-handler.go"
    val reason = "Saved analysis is outdated for the current source revision."
    val selection =
        selectionFixture()
            .copy(
                files =
                    listOf(
                        AnalysisSelectableFile(path, "", selectionStageFixture("stale", reason))))
    for ((width, scale) in listOf(1600 to 1f, 800 to 1.5f)) {
      var reads = 0
      var writes = 0
      var admissions = 0
      ComposeVisualFixture(width, 650, scale) {
            AnalysisFileSelector(
                ProjectAnalysisRunState(fileSelection = AnalysisSelectionState(selection)),
                AnalysisWorkspaceActions(
                    { _, _ -> admissions++ },
                    { admissions++ },
                    { admissions++ },
                    { admissions++ },
                    { admissions++ },
                    { reads++ },
                    { writes++ },
                    refreshStatus = { error("Unexpected status refresh") }))
          }
          .use { fixture ->
            fixture.render()
            assertFalse(fixture.hasEditableText(withinTag = "analysis-file-row-$path"))
            assertEquals(1, fixture.clickableDescriptionCount("Analyze $path"))
            assertEquals(
                "Selected for analysis", fixture.descriptionStateDescription("Analyze $path"))
            assertTrue(fixture.hasText("Outdated"))
            assertTrue(fixture.hasText(reason))
            assertTrue(fixture.copyTextByDragging(path).isNotEmpty())
            assertEquals(0, writes + reads + admissions)
            assertEquals(
                "Collapsed", fixture.descriptionStateDescription("Analysis details for $path"))
            fixture.clickDescription("Analysis details for $path")
            fixture.render()
            assertEquals(
                "Expanded", fixture.descriptionStateDescription("Analysis details for $path"))
            assertTrue(
                fixture.hasText(
                    listOf(
                            "Code analysis",
                            "Performance review",
                            "Security rules",
                            "AI Security review")
                        .joinToString("\n") { "$it: $reason" }))
            assertEquals(0, writes + reads + admissions)
          }
    }
  }

  @Test
  fun longPolicyReasonCanBeExpandedWithoutSelectingOrNavigating() {
    val path = "generated/long-identity.go"
    val reason = "Policy excluded: " + "generated/package/日本語/".repeat(210)
    val selection = selectionFixture().copy(files = listOf(AnalysisSelectableFile(path, reason)))
    var writes = 0
    var reads = 0
    ComposeVisualFixture(800, 650, 1.5f) {
          AnalysisFileSelector(
              ProjectAnalysisRunState(fileSelection = AnalysisSelectionState(selection)),
              AnalysisWorkspaceActions(
                  { _, _ -> },
                  {},
                  {},
                  {},
                  {},
                  { reads++ },
                  { writes++ },
                  refreshStatus = { error("Unexpected status refresh") }))
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText("Excluded"))
          assertTrue(fixture.isDescriptionDisabled("Analyze $path"))
          assertTrue(fixture.hasDescription("Analysis details for $path"))
          fixture.clickDescription("Analysis details for $path")
          fixture.render()
          assertTrue(fixture.hasText("… output truncated"))
          fixture.clickDescription("Expand available diagnostic output")
          fixture.render()
          assertTrue(fixture.hasText(reason))
          assertEquals(0, reads + writes)
        }
  }

  @Test
  fun analysisStageFailureRetainsItsRecoveryActionAndReadableDiagnostic() {
    val reason =
        "Semantic analysis failed because the local scanner is unavailable. Start analysis to retry."
    val run =
        analysisRunFixture()
            .copy(
                status = "failed",
                plan =
                    analysisPreviewFixture()
                        .copy(
                            files =
                                listOf(
                                    AnalysisPlannedFile(
                                        "cmd/miniorca/main.go",
                                        "base",
                                        "Go",
                                        20,
                                        listOf(
                                            AnalysisStagePlan(
                                                "semantic", true, false, maxModelRequests = 0))))),
                files =
                    listOf(
                        AnalysisRunFile(
                            "cmd/miniorca/main.go",
                            "base",
                            "Go",
                            listOf(
                                AnalysisStageProgress(
                                    "semantic", "failed", 1, false, reason = reason)))))
    var starts = 0
    ComposeVisualFixture(800, 650, 1.5f) {
          AnalysisWorkspacePane(
              AnalysisWorkspacePaneState(
                  resultProjectFixture(), ProjectAnalysisRunState(run = run)),
              AnalysisWorkspaceActions(
                  { _, _ -> starts++ },
                  {},
                  {},
                  {},
                  {},
                  refreshStatus = { error("Unexpected status refresh") }))
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText("Start analysis"))
          fixture.clickText("Start analysis")
          assertEquals(1, starts)
          assertTrue(fixture.hasText("Attention · 1 failed"))
          assertFalse(fixture.hasText(reason))
          fixture.revealText("Code analysis · 1/1 finished · 1 failed", "analysis-page")
          fixture.clickText("Code analysis · 1/1 finished · 1 failed")
          fixture.render()
          fixture.revealText(reason, "analysis-page")
          fixture.assertTextWrapsWithoutClipping(reason)
        }
  }

  @Test
  fun keyboardShortcutsCoverFocusedWorkflowWithoutMouse() {
    assertEquals(DesktopShortcut.OpenFile, desktopShortcut("P", primaryModifier = true))
    assertEquals(DesktopShortcut.OpenProject, desktopShortcut("O", primaryModifier = true))
    assertEquals(
        DesktopShortcut.OpenSymbol, desktopShortcut("O", primaryModifier = true, shift = true))
    assertEquals(DesktopShortcut.FocusChat, desktopShortcut("K", primaryModifier = true))
    assertEquals(DesktopShortcut.Generate, desktopShortcut("Enter", primaryModifier = true))
    assertEquals(DesktopShortcut.Cancel, desktopShortcut("Escape", primaryModifier = false))
    assertEquals(DesktopShortcut.NextTab, desktopShortcut("Tab", primaryModifier = true))
    assertNull(desktopShortcut("O", primaryModifier = false))
  }

  @Test
  fun keyboardRoutesCoverWorkspacesBugsDraftValidationAndChecks() {
    assertEquals(DesktopShortcut.SummaryWorkspace, desktopShortcut("1", primaryModifier = true))
    assertEquals(DesktopShortcut.AnalysisWorkspace, desktopShortcut("2", primaryModifier = true))
    assertEquals(DesktopShortcut.BugsWorkspace, desktopShortcut("3", primaryModifier = true))
    assertEquals(DesktopShortcut.EditorWorkspace, desktopShortcut("4", primaryModifier = true))
    assertNull(desktopShortcut("F", primaryModifier = true, shift = true))
    assertEquals(
        DesktopShortcut.FocusDraft, desktopShortcut("D", primaryModifier = true, shift = true))
    assertEquals(
        DesktopShortcut.ValidateDraft, desktopShortcut("V", primaryModifier = true, shift = true))
    assertEquals(
        DesktopShortcut.RunDraftChecks, desktopShortcut("C", primaryModifier = true, shift = true))
  }

  @Test
  fun toolWindowSemanticsKeepTextualSelectedStateWithoutCounters() {
    assertEquals(
        "Editor tool window, selected", toolWindowSemanticsLabel(LeftToolWindow.Editor, true))
    assertEquals(
        "Bugs tool window, not selected", toolWindowSemanticsLabel(LeftToolWindow.Problems, false))
  }

  @Test
  fun retainedNavigationStatesDescribeSelectionAndFocusSeparately() {
    assertEquals(
        "Editor tool window, selected, focused",
        toolWindowSemanticsLabel(LeftToolWindow.Editor, selected = true, focused = true),
    )
    assertEquals(
        "Editor tool window, selected",
        toolWindowSemanticsLabel(LeftToolWindow.Editor, selected = true),
    )
  }

  @Test
  fun editorChromeExposesTheFullSelectedFileIdentityAndReadOnlySurface() {
    val chrome =
        editorChromeUiState(
            file =
                ProjectFileInfo(
                    path = "cmd/miniorca/main.go",
                    contentHash = "hash",
                    name = "main.go",
                    language = "Go",
                    sizeBytes = 0,
                    lineCount = 0,
                    modifiedAt = "",
                    binary = false,
                ),
            selectedSymbol =
                SymbolInfo("Main", "function", confidence = "exact", atomicTarget = true),
            requestedSurface = EditorSurface.Source,
            progress = EditorProgressUiState(EditorProgress.Inspect, ""),
            draft = null,
        )

    assertTrue(chrome.accessibleDescription.contains("cmd/miniorca/main.go"))
    assertTrue(chrome.accessibleDescription.contains("Selected declaration Main"))
    assertTrue(chrome.accessibleDescription.contains("Read-only source surface"))
  }

  @Test
  fun technicalReviewIdentityHashesAreLabeledForTheDisclosure() {
    assertEquals(
        listOf(
            ReviewIdentityHashDetail("Candidate hash", "draft-hash"),
            ReviewIdentityHashDetail("Check identity hash", "check-hash"),
        ),
        reviewIdentityHashDetails("draft-hash", "check-hash"),
    )
    assertEquals(emptyList(), reviewIdentityHashDetails("", null))
  }

  @Test
  fun landingModeAcceptsOnlyTheOpenProjectShortcut() {
    DesktopShortcut.entries.forEach { shortcut ->
      assertEquals(
          shortcut == DesktopShortcut.OpenProject,
          shortcutAvailable(DesktopShellMode.ProjectLanding, shortcut))
      assertTrue(shortcutAvailable(DesktopShellMode.ProjectWorkspace, shortcut))
    }
    assertTrue(!shortcutAvailable(DesktopShellMode.ProjectLanding, null))
  }

  @Test
  fun restoreProgressAndFailureNeverUnlockProjectOnlyShortcuts() {
    val attempt = ProjectOpeningAttempt(1, "/remembered", ProjectOpeningKind.Restore)
    listOf(ProjectOpeningOutcome.Opening, ProjectOpeningOutcome.Failed("Missing")).forEach { outcome
      ->
      val state =
          DesktopState(
              projectState =
                  ProjectWorkspaceState(openingAttempt = attempt.copy(outcome = outcome)))
      val mode = desktopShellMode(state)
      assertEquals(DesktopShellMode.ProjectLanding, mode)
      assertTrue(shortcutAvailable(mode, DesktopShortcut.OpenProject))
      listOf(
              DesktopShortcut.OpenFile,
              DesktopShortcut.OpenSymbol,
              DesktopShortcut.SummaryWorkspace,
              DesktopShortcut.Generate,
              DesktopShortcut.FocusDraft)
          .forEach { assertFalse(shortcutAvailable(mode, it)) }
    }
  }

  @Test
  fun highlightingLeavesSourceIntactAndStylesRecognizedTokens() {
    val source = "package demo\n// note\nfun run() = \"ok\"\n"
    val highlighted = highlightedCode(source)

    assertEquals(source, highlighted.text)
    assertTrue(highlighted.spanStyles.isNotEmpty())
  }
}
