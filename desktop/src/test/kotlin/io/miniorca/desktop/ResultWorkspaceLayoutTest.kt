package io.miniorca.desktop

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material.Text
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.unit.dp
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertSame
import kotlin.test.assertTrue

class ResultWorkspaceLayoutTest {
  @Test
  fun scanReadFailureRemainsVisibleBesideCancellationAndUnsupportedAvailability() {
    val base = verifiedScanLayoutCases().first { it.first == "running" }.second
    val message = "Live scan status could not be read: poll offline"
    val states =
        listOf(
            base.copy(
                scanState =
                    base.scanState.copy(operation = VerifiedScanOperation.CancellationRequested)),
            base.copy(
                scanState =
                    base.scanState.copy(
                        operation =
                            VerifiedScanOperation.CancellationUnconfirmed("Cancel timed out"))),
            base.copy(project = base.project!!.copy(type = "python")))
    for ((index, state) in states.withIndex()) {
      var calls = 0
      ComposeVisualFixture(800, 400, 1.5f) {
            BugsWorkspacePane(
                state.copy(
                    scanState =
                        state.scanState.copy(read = VerifiedScanRead.PollUnavailable(message))),
                BugsWorkspaceActions(
                    FindingActions({}, { _, _ -> }, {}),
                    { calls++ },
                    { calls++ },
                    refreshScanStatus = { calls++ }))
          }
          .use { fixture ->
            fixture.render()
            fixture.revealTextFullyWithin("Live status unavailable: $message", "result-overview")
            fixture.assertTextFits("Live status unavailable: $message", maxLines = 12)
            assertEquals("Collapsed", fixture.descriptionState("Expand Command and output"))
            if (state.scanState.operation != VerifiedScanOperation.CancellationRequested) {
              fixture.revealTextFullyWithin("Refresh scan status", "result-overview")
              fixture.assertTextFits("Refresh scan status")
            }
            fixture.render("f20-combined-read-error-$index-800-400-1.5")
            assertEquals(0, calls)
          }
    }
  }

  @Test
  fun scanDisclosureResetsOnProjectRevisionAndReportReplacementButNotReadFeedback() {
    var state by
        mutableStateOf(verifiedScanLayoutCases().first { it.first == "failed-phase" }.second)
    var calls = 0
    ComposeVisualFixture(800, 400, 1.5f) {
          BugsWorkspacePane(
              state,
              BugsWorkspaceActions(FindingActions({}, { _, _ -> }, {}), { calls++ }, { calls++ }))
        }
        .use { fixture ->
          fixture.render()
          fun expand() {
            fixture.revealTextFullyWithin("Command and output", "result-overview")
            fixture.clickDescription("Expand Command and output")
            fixture.render()
            assertEquals("Expanded", fixture.descriptionState("Collapse Command and output"))
            assertTrue(fixture.hasText("main.go:7: vet diagnostic"))
          }
          expand()
          state =
              state.copy(
                  scanState =
                      state.scanState.copy(
                          read = VerifiedScanRead.PollUnavailable("Status read failed")))
          fixture.render()
          assertTrue(fixture.hasDescription("Collapse Command and output"))
          for (replacement in listOf("report", "revision", "project")) {
            state =
                when (replacement) {
                  "report" -> state.copy(scan = state.scan!!.copy(completedAt = "replacement"))
                  "revision" ->
                      state.copy(project = state.project!!.copy(projectRevision = "next-revision"))
                  else -> state.copy(project = state.project!!.copy(projectId = "next-project"))
                }
            fixture.render()
            assertEquals("Collapsed", fixture.descriptionState("Expand Command and output"))
            assertFalse(fixture.hasText("main.go:7: vet diagnostic"))
            expand()
          }
          assertEquals(0, calls)
        }
  }

  @Test
  fun scanFullAvailableOutputAndLongCommandRemainScrollableAndSelectableAtReducedHeight() {
    for (scale in listOf(1f, 1.25f, 1.5f)) {
      for (kind in listOf("output", "command")) {
        val original = verifiedScanLayoutCases().first { it.first == "long-output" }.second
        val command =
            listOf("go", "test", "./" + "deeply/nested/日本語/".repeat(260) + "FINAL_PACKAGE")
        val phase = original.scan!!.phases.single()
        val state =
            if (kind == "output") original
            else
                original.copy(
                    scan =
                        original.scan.copy(
                            phases =
                                listOf(
                                    phase.copy(
                                        command = command, output = "Recorded command failure"))))
        val available = if (kind == "output") phase.output else "$ " + command.joinToString(" ")
        var calls = 0
        ComposeVisualFixture(800, 400, scale) {
              BugsWorkspacePane(
                  state,
                  BugsWorkspaceActions(
                      FindingActions({}, { _, _ -> }, {}), { calls++ }, { calls++ }))
            }
            .use { fixture ->
              fixture.render()
              fixture.revealTextFullyWithin("Command and output", "result-overview")
              fixture.clickDescription("Expand Command and output")
              fixture.render()
              fixture.scrollBy(100_000f, "result-overview")
              fixture.render()
              fixture.revealTextFullyWithin("Show full available output", "scan-diagnostics")
              fixture.clickDescription("Expand available diagnostic output")
              fixture.render()
              assertTrue(fixture.hasText(available))
              assertFalse(fixture.hasEditableText(withinTag = "scan-diagnostics"))
              fixture.scrollBy(100_000f, "scan-diagnostics")
              fixture.render()
              fixture.scrollTagged(
                  "diagnostic-output-scroll", horizontal = false, pixels = 100_000f)
              assertEquals(
                  fixture.scrollMaximum("diagnostic-output-scroll", false),
                  fixture.scrollPosition("diagnostic-output-scroll", false))
              fixture.render("f20-$kind-full-tail-800-400-$scale")
              fixture.revealTextFullyWithin("Show preview", "scan-diagnostics")
              fixture.clickDescription("Collapse available diagnostic output")
              fixture.render()
              assertFalse(fixture.hasText(available))
              // Exercise the DiagnosticText clipboard, not an editable field or phase heading.
              val selectable =
                  if (kind == "output") "$ go vet ./..." else "Recorded command failure"
              fixture.scrollBy(if (kind == "output") -100_000f else 100_000f, "scan-diagnostics")
              fixture.render()
              fixture.revealTextFullyWithin(selectable, "result-overview")
              fixture.revealTextFullyWithin(selectable, "scan-diagnostics")
              assertTrue(fixture.copyTextByDragging(selectable, selectable).isNotBlank())
              assertEquals(0, calls)
            }
      }
    }
  }

  @Test
  fun completedScanWithFailedPhaseShowsOutcomeAndProvenanceWithoutOpeningDetails() {
    val project = resultProjectFixture()
    val report =
        GoScanReport(
            project.projectId,
            project.projectRevision,
            "completed",
            phases = listOf(GoScanPhase("go vet", "failed", output = "vet error", exitCode = 0)))
    var executions = 0
    ComposeVisualFixture(800, 650, 1.5f) {
          BugsWorkspacePane(
              BugsWorkspacePaneState(
                  emptyList(),
                  report,
                  false,
                  project = project,
                  scanState = VerifiedScanState(read = VerifiedScanRead.Loaded)),
              BugsWorkspaceActions(
                  FindingActions({}, { _, _ -> }, {}), { executions++ }, { executions++ }))
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText("Completed"))
          assertTrue(
              fixture.hasText(
                  "Verified checks completed; Reported phases: go vet — Failed. Review command and output for details. Completion is not proof that every phase passed."))
          assertTrue(
              fixture.hasText(
                  "Local tool evidence is scoped to these checks, not a general safety assurance. Model suggestions are separate results below."))
          assertFalse(fixture.hasText("vet error"))
          fixture.clickText("Command and output")
          fixture.render()
          assertTrue(fixture.hasText("vet error"))
          assertFalse(fixture.hasText("Exit code: 0"))
          assertEquals(0, executions)
        }
  }

  @Test
  fun scanDiagnosticsRetainReportedOrderUnknownNamesAndOnlyMeaningfulExitCodes() {
    val phases =
        listOf(
            GoScanPhase("workspace", "failed", output = "copy failed", exitCode = 7),
            GoScanPhase("parse", "failed", exitCode = 3),
            GoScanPhase("go vet", "failed", listOf("go", "vet", "./..."), "vet failed", 2),
            GoScanPhase("go test", "skipped", listOf("go", "test", "./..."), exitCode = 9),
            GoScanPhase("future phase", "unavailable", output = "unknown tool state"),
            GoScanPhase("command failure", "failed", listOf("go", "test"), exitCode = -1),
            GoScanPhase("cancellation", "canceled", listOf("go", "test", "./..."), exitCode = 143),
            GoScanPhase("", ""))
    val report = GoScanReport(status = "completed", phases = phases)
    ComposeVisualFixture(800, 650, 1.5f) { VerifiedScanDiagnostics(report) }
        .use { fixture ->
          fixture.render()
          phases
              .filter { it.name.isNotBlank() }
              .forEach { phase -> fixture.assertTextFits(phase.name) }
          assertTrue(fixture.hasText("Unnamed scan phase"))
          assertTrue(fixture.hasText("State unavailable"))
          assertTrue(fixture.hasText("Failed"))
          assertTrue(fixture.hasText("Skipped"))
          assertTrue(fixture.hasText("Canceled"))
          assertTrue(fixture.hasText("Unavailable"))
          assertTrue(fixture.hasText("copy failed"))
          assertTrue(fixture.hasText("$ go vet ./..."))
          assertTrue(fixture.hasText("$ go test ./..."))
          assertTrue(fixture.hasText("Exit code: 2"))
          assertTrue(fixture.hasText("Exit code: -1"))
          assertTrue(fixture.hasText("Exit code: 143"))
          listOf(0, 3, 7, 9).forEach { assertFalse(fixture.hasText("Exit code: $it")) }
          assertTrue(fixture.hasText("No output reported for this phase."))
          assertTrue(fixture.hasText("No command reported for this phase."))
          assertTrue(fixture.hasText("unknown tool state"))
        }
    ComposeVisualFixture(800, 650) { VerifiedScanDiagnostics(report.copy(phases = emptyList())) }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText("No phases reported; no check outcome is available."))
          assertFalse(fixture.hasText("Passed"))
        }
  }

  @Test
  fun scanOutputDistinguishesDaemonLimitFromUiPreviewAndRetainsAvailableText() {
    val output = "recorded evidence\n".repeat(400) + "TAIL\n[output truncated]"
    val report =
        GoScanReport(
            status = "failed",
            phases =
                listOf(GoScanPhase("go test", "failed", listOf("go", "test", "./..."), output, 1)))
    ComposeVisualFixture(800, 650, 1.5f) { VerifiedScanDiagnostics(report) }
        .use { fixture ->
          fixture.render()
          assertTrue(
              fixture.hasText(
                  "Daemon output limit reached; text beyond the recorded output is unavailable."))
          assertTrue(
              fixture.hasText(
                  "UI previews are limited; Show full available output reveals all recorded text."))
          assertTrue(fixture.hasText("… output truncated"))
          assertFalse(fixture.hasText(output))
          fixture.clickDescription("Expand available diagnostic output")
          fixture.render()
          assertTrue(fixture.hasText(output))
          assertTrue(fixture.hasText("Exit code: 1"))
        }
    ComposeVisualFixture(800, 650) {
          VerifiedScanDiagnostics(
              report.copy(
                  phases =
                      listOf(
                          GoScanPhase(
                              "go vet",
                              "failed",
                              listOf("go", "vet", "./..."),
                              "short output\n[output truncated]",
                              1))))
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText("short output\n[output truncated]"))
          assertTrue(
              fixture.hasText(
                  "Daemon output limit reached; text beyond the recorded output is unavailable."))
          assertFalse(fixture.hasText("UI previews are limited"))
          assertFalse(fixture.hasText("Show full available output"))
          assertFalse(fixture.hasText("… output truncated"))
        }
    ComposeVisualFixture(800, 650) {
          VerifiedScanDiagnostics(
              report.copy(phases = listOf(GoScanPhase("parse", "passed", output = "   "))))
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText("No output reported for this phase."))
          assertFalse(fixture.hasText("Exit code: 0"))
          assertTrue(fixture.hasText("No command reported for this phase."))
          assertFalse(fixture.hasText("Daemon output limit reached"))
        }
    ComposeVisualFixture(800, 650) {
          VerifiedScanDiagnostics(
              report.copy(
                  phases =
                      listOf(
                          GoScanPhase(
                              "go vet",
                              "failed",
                              listOf("go", "vet", "./..."),
                              "recorded text\n".repeat(400),
                              1))))
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText("… output truncated"))
          assertFalse(fixture.hasText("Daemon output limit reached"))
          fixture.clickDescription("Expand available diagnostic output")
          fixture.render()
          assertTrue(fixture.hasText("recorded text\n".repeat(400).trim()))
        }
  }

  @Test
  fun allResultPagesKeepListAndDetailReachableAcrossViewports() {
    listOf(
            Triple(1440, 900, 1f),
            Triple(1000, 760, 1f),
            Triple(999, 760, 1f),
            Triple(800, 650, 1.5f),
            Triple(1280, 600, 1.5f))
        .forEach { (width, height, scale) ->
          AnalysisResultType.entries.forEach { type ->
            ComposeVisualFixture(width, height, scale) {
                  AcceptanceResultPane(type.category, "partial")
                }
                .use { fixture ->
                  fixture.render("rounded-${type.category}-$width-$height-$scale")
                  fixture.assertTextFits(type.workspace.name)
                  fixture.assertTextFits("View analysis")
                  val header = fixture.taggedBounds("result-header")
                  val action = fixture.firstVisibleTextBounds("View analysis")
                  val list = fixture.taggedBounds("result-list")
                  assertTrue(header.left > 0 && header.right < width)
                  assertTrue(action.top >= header.top && action.bottom <= header.bottom)
                  assertTrue(list.top > header.bottom && list.bottom < height)
                  assertTrue(list.height > 0)
                  val detail = fixture.taggedBounds("result-detail")
                  assertTrue(detail.height > 0 && detail.right < width && detail.bottom < height)
                  if (width == 800) {
                    assertEquals(list.left, detail.left)
                    assertTrue(list.bottom < detail.top)
                    assertEquals(list.right, detail.right)
                  } else {
                    assertTrue(list.right < detail.left)
                    assertEquals(list.top, detail.top)
                    assertEquals(list.bottom, detail.bottom)
                  }
                }
          }
        }
  }

  @Test
  fun listAndDetailCrossLocalTextScaledBreakpointWithoutLosingTheirScrollOwners() {
    listOf(1f to 628, 1.5f to 938).forEach { (scale, boundary) ->
      val rows = (1..320).map { row(it) }
      val browser = newResultBrowserState(resultPageFixture("bugs"))
      val widths = listOf(boundary + 1, boundary, boundary - 1, boundary + 1)
      ComposeVisualFixture(widths.first(), 650, scale) {
            ResultListDetail(
                rows,
                browser,
                { browser.selectedKey = it },
                "No findings",
                Modifier.fillMaxSize()) {
                  Column {
                    Text("Evidence for $it")
                    repeat(40) { line -> Text("Evidence paragraph $line") }
                  }
                }
          }
          .use { fixture ->
            fixture.render()
            fixture.revealText("Finding 20", "result-list")
            fixture.clickDescription("Inspect Finding 20")
            fixture.render()
            fixture.scrollBy(563f, "result-list")
            awaitResultListSettled(fixture, browser)
            val index = browser.listState.firstVisibleItemIndex
            val offset = browser.listState.firstVisibleItemScrollOffset
            assertTrue(index > 0 && offset > 0)
            fixture.scrollBy(260f, "result-detail")
            fixture.render()
            val detailOffset = fixture.verticalScrollValue("result-detail")
            assertTrue(detailOffset > 0f)
            widths.forEachIndexed { step, width ->
              fixture.resize(width, 650)
              fixture.render("result-reflow-$scale-$step")
              val list = fixture.taggedBounds("result-list")
              val detail = fixture.taggedBounds("result-detail")
              if (width < boundary) {
                assertTrue(list.bottom < detail.top)
                assertEquals(list.left, detail.left)
              } else {
                assertTrue(list.right < detail.left)
                assertEquals(list.top, detail.top)
              }
              assertTrue(list.height > 0 && detail.height > 0 && detail.bottom <= 650)
              assertEquals("finding-20", browser.selectedKey)
              assertTrue(fixture.hasText("Evidence for finding-20"))
              assertEquals(detailOffset, fixture.verticalScrollValue("result-detail"), 5f)
              assertEquals(index, browser.listState.firstVisibleItemIndex)
              assertEquals(offset, browser.listState.firstVisibleItemScrollOffset)
              browser.listState.requestScrollToItem(319)
              fixture.render()
              fixture.awaitVisibleDescription("Inspect Finding 320")
              fixture.revealText("Evidence paragraph 39", "result-detail")
              // Restore comparable scroll positions before measuring the next layout.
              browser.listState.requestScrollToItem(index, offset)
              fixture.scrollBy(-100_000f, "result-detail")
              fixture.render()
              fixture.scrollBy(detailOffset, "result-detail")
              fixture.render()
            }
          }
    }
  }

  @Test
  fun populatedFilteredAndEmptyResultsKeepTruthfulStateThroughReflow() {
    val page = resultPageFixture("bugs")
    val rows = (1..80).map { row(it) } + row(81).copy(severity = "", state = "Failed · Stale")
    val browser = newResultBrowserState(page)
    ComposeVisualFixture(1100, 650, 1.5f) {
          AnalysisResultsPane(page, rows, browser, openAnalysis = {}) { key ->
            Text("Evidence for $key")
          }
        }
        .use { fixture ->
          fixture.render()
          fixture.revealText("Finding 20", "result-list")
          fixture.clickDescription("Inspect Finding 20")
          fixture.render()
          assertEquals("finding-20", browser.selectedKey)
          browser.query = "Finding"
          browser.filter = ResultBrowserFilter.Value("high")
          fixture.render()
          fixture.scrollBy(480f, "result-list")
          awaitResultListSettled(fixture, browser)
          val index = browser.listState.firstVisibleItemIndex
          val offset = browser.listState.firstVisibleItemScrollOffset
          assertTrue(index > 0)
          listOf(800, 1100).forEach { width ->
            fixture.resize(width, 650)
            fixture.render()
            val list = fixture.taggedBounds("result-list")
            val detail = fixture.taggedBounds("result-detail")
            if (width == 800) assertTrue(list.bottom < detail.top)
            else assertTrue(list.right < detail.left)
            assertEquals(index, browser.listState.firstVisibleItemIndex)
            assertEquals(offset, browser.listState.firstVisibleItemScrollOffset)
            assertEquals("finding-20", browser.selectedKey)
            assertTrue(fixture.hasText("Evidence for finding-20"))
            assertEquals("Finding", browser.query)
            assertEquals(ResultBrowserFilter.Value("high"), browser.filter)
          }
          browser.filter = ResultBrowserFilter.Value("unknown")
          fixture.render()
          fixture.assertTextFits("Unknown 1")
          fixture.assertTextFits("Failed · Stale")
          assertEquals("finding-81", browser.selectedKey)
          browser.query = "no matches"
          fixture.resize(800, 650)
          fixture.render()
          fixture.assertTextFits("No matching results.")
          assertEquals(null, browser.selectedKey)
          browser.query = "Finding"
          fixture.render()
          assertEquals("finding-81", browser.selectedKey)
        }
  }

  @Test
  fun browsingAllLoadedResultsAndSwitchingCategoriesOnlyChangesLocalState() {
    val bugs = resultPageFixture("bugs")
    val security = bugs.copy(type = AnalysisResultType.Security)
    val store = ResultBrowserStore()
    val bugsBrowser = store.stateFor(bugs)
    val securityBrowser = store.stateFor(security)
    val bugsRows =
        (1..320).map { number ->
          row(number)
              .copy(
                  location = "internal/module$number/Handler.go:$number",
                  severity = if (number % 2 == 0) "HIGH" else "")
        }
    var category by mutableStateOf(AnalysisResultType.Bugs)
    var privilegedActions = 0
    ComposeVisualFixture(800, 650, 1.5f) {
          val page = if (category == AnalysisResultType.Bugs) bugs else security
          AnalysisResultsPane(
              page,
              if (category == AnalysisResultType.Bugs) bugsRows else listOf(row(1)),
              store.stateFor(page),
              openAnalysis = { privilegedActions++ },
              retryResults = { privilegedActions++ }) { key ->
                Text("Evidence for $key")
              }
        }
        .use { fixture ->
          fixture.render()
          bugsBrowser.listState.requestScrollToItem(319)
          fixture.render()
          fixture.awaitVisibleDescription("Inspect Finding 320")
          fixture.clickDescription("Inspect Finding 320")
          fixture.render()
          assertEquals("finding-320", bugsBrowser.selectedKey)
          assertTrue(fixture.hasText("Evidence for finding-320"))
          fixture.clickDescription("Filter results")
          fixture.setFocusedText("mOdUlE319/hAnDlEr.Go")
          fixture.render()
          assertEquals("finding-319", bugsBrowser.selectedKey)
          assertTrue(fixture.hasText("Evidence for finding-319"))
          fixture.clickDescription("Severity Unknown 160")
          fixture.render()
          assertTrue(fixture.hasText("Evidence for finding-319"))
          fixture.clickDescription("Clear filters")
          fixture.render()
          assertEquals(ResultBrowserFilter.All, bugsBrowser.filter)
          assertEquals("", bugsBrowser.query)
          bugsBrowser.listState.requestScrollToItem(18, 4)
          fixture.render()
          val index = bugsBrowser.listState.firstVisibleItemIndex
          val offset = bugsBrowser.listState.firstVisibleItemScrollOffset
          category = AnalysisResultType.Security
          fixture.render()
          assertTrue(fixture.hasText("Evidence for finding-1"))
          assertEquals("", securityBrowser.query)
          category = AnalysisResultType.Bugs
          fixture.render()
          assertSame(bugsBrowser, store.stateFor(bugs))
          assertEquals("finding-319", bugsBrowser.selectedKey)
          assertEquals(index, bugsBrowser.listState.firstVisibleItemIndex)
          assertEquals(offset, bugsBrowser.listState.firstVisibleItemScrollOffset)
          assertEquals(0, privilegedActions)
        }
  }

  @Test
  fun criticalUnknownAndNoMatchFiltersStayVisibleAtCompactLargeTextScale() {
    val page = resultPageFixture("bugs")
    val rows =
        listOf(
            row(1).copy(severity = "critical"),
            row(2).copy(severity = ""),
            row(3).copy(severity = "high"),
        )
    val browser = newResultBrowserState(page)
    var openedAnalysis = 0
    ComposeVisualFixture(800, 650, 1.5f) {
          AnalysisResultsPane(
              page = page,
              rows = rows,
              browser = browser,
              openAnalysis = { openedAnalysis++ },
          ) { key ->
            Text("Evidence for $key")
          }
        }
        .use { fixture ->
          fixture.render("results-filters-critical-unknown-800-150")
          fixture.assertTextFits("Critical 1")
          fixture.assertTextFits("Unknown 1")
          fixture.assertTextFits("High 1")
          listOf("All 3", "Critical 1", "Unknown 1", "High 1").forEach {
            fixture.assertTextFontFamily(it, FontFamily.SansSerif)
          }
          fixture.assertTextFits("Unknown")
          assertEquals(1, fixture.textCount("Unknown"))
          fixture.clickDescription("Filter results")
          fixture.setFocusedText("no loaded finding")
          fixture.render("results-filters-no-match-800-150")
          fixture.assertTextFits("No matching results.")
          fixture.assertTextFits("Clear filters to view loaded results.")
          fixture.assertTextFits("Clear filters")
          fixture.assertTextFontFamily("Clear filters", FontFamily.SansSerif)
          assertEquals(1, fixture.textCount("Clear filters"))
          fixture.clickDescription("Clear filters")
          fixture.render("results-filters-cleared-800-150")
          assertFalse(fixture.hasText("No matching results."))
          assertFalse(fixture.hasText("Clear filters to view loaded results."))
          assertEquals(0, openedAnalysis)
        }
  }

  @Test
  fun emptyStateUsesOneFullWidthSurfaceAndKeepsItsScopeDistinct() {
    fun page(status: String, findingCount: Int?): AnalysisResultPageState {
      val original = resultPageFixture("bugs")
      val progress =
          requireNotNull(original.progress).copy(status = status, findingCount = findingCount)
      val run =
          requireNotNull(original.run)
              .copy(
                  status = status,
                  sections =
                      original.run.sections.map { if (it.category == "bugs") progress else it })
      return original.copy(
          run = run,
          section =
              AnalysisSectionState(
                  results =
                      requireNotNull(original.results)
                          .copy(progress = progress, semantic = emptyList())))
    }

    listOf(
            Triple("running", null, "Analysis is in progress."),
            Triple("completed", 0, "No findings in the analyzed scope."),
            Triple("partial", 0, "Analysis completed partially."),
            Triple("failed", 0, "Analysis failed for this category."))
        .forEach { (status, findingCount, message) ->
          var openedAnalysis = 0
          ComposeVisualFixture(800, 650, 1.5f) {
                AnalysisResultsPane(
                    page = page(status, findingCount),
                    rows = emptyList(),
                    browser = newResultBrowserState(page(status, findingCount)),
                    openAnalysis = { openedAnalysis++ }) {
                      Text("unused")
                    }
              }
              .use { fixture ->
                fixture.render("results-empty-$status-800-150")
                fixture.assertTextFits(message)
                assertTrue(fixture.taggedBounds("result-empty").width > 700f)
                assertFalse(fixture.hasDescription("Filter results"))
                assertFalse(fixture.hasText("All 0"))
                assertFalse(fixture.hasText("Severity"))
                fixture.clickText("View analysis")
                assertEquals(1, openedAnalysis)
              }
        }

    val completed = page("completed", 0)
    val browser = newResultBrowserState(completed).also { it.query = "retained query" }
    ComposeVisualFixture(800, 650, 1.5f) {
          AnalysisResultsPane(
              page = completed, rows = emptyList(), browser = browser, openAnalysis = {}) {
                Text("unused")
              }
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasDescription("Filter results"))
          fixture.assertTextFits("Clear filters")
          fixture.clickDescription("Clear filters")
          fixture.render()
          assertFalse(fixture.hasDescription("Filter results"))
          fixture.assertTextFits("No findings in the analyzed scope.")
        }
  }

  @Test
  fun explicitTargetRevealsItsActualPositionAndMissingTargetHasLocalRecovery() {
    val page = resultPageFixture("bugs")
    var rows by mutableStateOf((1..320).map { row(it) })
    val browser = newResultBrowserState(page)
    val target =
        SummaryFindingTarget(
            page.project!!.projectId,
            page.project.projectRevision,
            page.run!!.identity,
            AnalysisResultType.Bugs,
            SummaryFindingProducer.Semantic("internal/handler.go", "finding-280", "file_analysis"),
            "finding-280")
    var navigations = 0
    ComposeVisualFixture(800, 650, 1.5f) {
          AnalysisResultsPane(page, rows, browser, openAnalysis = { navigations++ }) { key ->
            Text("Evidence for $key")
          }
        }
        .use { fixture ->
          fixture.render()
          browser.target(ExplicitResultTarget.Resolved(target))
          fixture.render("result-target-revealed-800-150")
          fixture.awaitVisibleDescription("Inspect Finding 280")
          assertTrue(browser.listState.firstVisibleItemIndex > 200)
          assertEquals("finding-280", browser.selectedKey)
          assertTrue(fixture.hasText("Evidence for finding-280"))
          assertFalse(fixture.hasText("Finding unavailable"))
          assertTrue(fixture.requestDescriptionFocus("Inspect Finding 280"))
          val revealedIndex = browser.listState.firstVisibleItemIndex
          rows = rows.map { if (it.key == "finding-1") it.copy(summary = "Refreshed") else it }
          fixture.render()
          assertEquals(revealedIndex, browser.listState.firstVisibleItemIndex)
          assertTrue(fixture.isDescriptionFocused("Inspect Finding 280"))

          rows = rows.filterNot { it.key == target.rowKey }
          fixture.render("result-target-unavailable-800-150")
          assertEquals(null, browser.selectedKey)
          assertTrue(
              fixture.hasText(
                  "Finding unavailable · This finding is no longer in the loaded results."))
          assertFalse(fixture.hasText("Evidence for finding-281"))
          assertFalse(fixture.hasText("Evidence for finding-1"))
          assertTrue(
              fixture.taggedBounds("result-target-unavailable").bottom <=
                  fixture.taggedBounds("result-list").top)
          assertTrue(fixture.hasText("View current Bugs results"))
          rows = (1..320).map { row(it) }
          fixture.render()
          assertTrue(browser.explicitTarget is ExplicitResultTarget.Unavailable)
          assertFalse(fixture.hasText("Evidence for finding-280"))
          fixture.clickText("View current Bugs results")
          fixture.render()
          assertEquals(null, browser.explicitTarget)
          assertTrue(fixture.hasText("Evidence for finding-1"))
          assertEquals(0, navigations)
          browser.listState.requestScrollToItem(19)
          fixture.render()
          fixture.awaitVisibleDescription("Inspect Finding 20")
          assertTrue(fixture.requestDescriptionFocus("Inspect Finding 20"))
          assertTrue(fixture.pressKey(Key.Spacebar))
          fixture.render()
          assertEquals("finding-20", browser.selectedKey)
          assertTrue(fixture.hasText("Evidence for finding-20"))
        }
  }

  @Test
  fun ambiguousTargetShowsNoSubstituteAndRowSelectionDismissesFeedback() {
    val page = resultPageFixture("bugs")
    val rows = listOf(row(1), row(1).copy(title = "Colliding result"), row(2))
    val browser = newResultBrowserState(page)
    val target =
        SummaryFindingTarget(
            page.project!!.projectId,
            page.project.projectRevision,
            page.run!!.identity,
            AnalysisResultType.Bugs,
            SummaryFindingProducer.Semantic("internal/handler.go", "finding-1", "file_analysis"),
            "finding-1")
    browser.target(ExplicitResultTarget.Resolved(target))
    ComposeVisualFixture(800, 650) {
          AnalysisResultsPane(page, rows, browser, openAnalysis = {}) { key ->
            Text("Evidence for $key")
          }
        }
        .use { fixture ->
          fixture.render()
          assertTrue(
              fixture.hasText(
                  "Finding unavailable · Multiple results share this finding identity; select a result from the category list."))
          assertFalse(fixture.hasText("Evidence for finding-1"))
          fixture.clickDescription("Inspect Finding 2")
          fixture.render()
          assertEquals(null, browser.explicitTarget)
          assertEquals("finding-2", browser.selectedKey)
          assertFalse(
              fixture.hasText(
                  "Finding unavailable · Multiple results share this finding identity; select a result from the category list."))
          assertTrue(fixture.hasText("Evidence for finding-2"))
        }
  }

  @Test
  fun selectingAnotherFindingStartsItsEvidenceAtTheTop() {
    val rows = (1..2).map { row(it) }
    val browser =
        newResultBrowserState(resultPageFixture("bugs")).also { it.selectedKey = rows.first().key }
    ComposeVisualFixture(1000, 600) {
          ResultListDetail(
              rows, browser, { browser.selectedKey = it }, "No findings", Modifier.fillMaxSize()) {
                Column(verticalArrangement = Arrangement.spacedBy(16.dp)) {
                  Text("Evidence for $it")
                  repeat(30) { line -> Text("Evidence paragraph $line for $it") }
                }
              }
        }
        .use { fixture ->
          fixture.render()
          fixture.scrollBy(800f, "result-detail")
          fixture.render()
          fixture.clickDescription("Inspect Finding 2")
          fixture.render()
          fixture.assertTextFits("Evidence for finding-2")
        }
  }

  @Test
  fun filteringClearsDetailAndArrowKeysKeepLongListsNavigable() {
    val page = resultPageFixture("bugs")
    val rows = (1..320).map { row(it) }
    val browser = newResultBrowserState(page)
    var openedAnalysis = 0
    ComposeVisualFixture(800, 650) {
          AnalysisResultsPane(
              page = page,
              rows = rows,
              browser = browser,
              openAnalysis = { openedAnalysis++ },
          ) { key ->
            Text("Evidence for $key")
          }
        }
        .use { fixture ->
          fixture.render()
          fixture.revealText("Finding 20", "result-list")
          fixture.clickDescription("Inspect Finding 20")
          fixture.render()
          assertTrue(fixture.hasText("Evidence for finding-20"))
          fixture.clickDescription("Filter results")
          fixture.setFocusedText("no loaded finding")
          fixture.render()
          assertFalse(fixture.hasText("Evidence for finding-20"))
          assertTrue(fixture.hasText("No matching results."))
          assertTrue(fixture.hasText("Clear filters to view loaded results."))
          fixture.clickDescription("Clear filters")
          fixture.render()
          assertFalse(fixture.hasText("No matching results."))
          assertFalse(fixture.hasText("Clear filters to view loaded results."))
          fixture.revealText("Finding 1", "result-list")
          assertTrue(fixture.requestDescriptionFocus("Inspect Finding 1"))
          assertTrue(fixture.pressKey(Key.DirectionUp))
          fixture.awaitVisibleDescription("Inspect Finding 320")
          assertTrue(fixture.isDescriptionFocused("Inspect Finding 320"))
          assertTrue(fixture.pressKey(Key.Spacebar))
          fixture.render()
          assertTrue(fixture.hasText("Evidence for finding-320"))
          assertEquals(0, openedAnalysis)
        }
  }

  @Test
  fun leavingAndReturningKeepsSelectedRowAndExactLazyListPosition() {
    val page = resultPageFixture("bugs")
    val rows = (1..320).map { row(it) }
    val browser =
        newResultBrowserState(page).also {
          it.filter = ResultBrowserFilter.Value("high")
          it.query = "Finding"
          it.selectedKey = "finding-20"
        }
    val showingResults = mutableStateOf(true)
    ComposeVisualFixture(1000, 650) {
          if (showingResults.value)
              ResultListDetail(
                  rows,
                  browser,
                  { browser.selectedKey = it },
                  "No findings",
                  Modifier.fillMaxSize()) {
                    Text("Evidence for $it")
                  }
          else Text("Other workspace")
        }
        .use { fixture ->
          fixture.render()
          fixture.scrollBy(563f, "result-list")
          awaitResultListSettled(fixture, browser)
          val retainedIndex = browser.listState.firstVisibleItemIndex
          val retainedOffset = browser.listState.firstVisibleItemScrollOffset
          assertTrue(retainedIndex > 0)
          assertTrue(retainedOffset > 0)
          showingResults.value = false
          fixture.render()
          showingResults.value = true
          fixture.render()
          assertEquals(ResultBrowserFilter.Value("high"), browser.filter)
          assertEquals("Finding", browser.query)
          assertEquals("finding-20", browser.selectedKey)
          assertEquals(retainedIndex, browser.listState.firstVisibleItemIndex)
          assertEquals(retainedOffset, browser.listState.firstVisibleItemScrollOffset)
        }
  }

  @Test
  fun savedResultReadFeedbackStaysAboveRetainedAndFilteredEvidenceForEveryCategory() {
    AnalysisResultType.entries.forEach { type ->
      val original = resultPageFixture(type.category)
      val rows = (1..80).map { row(it) }
      val browser = newResultBrowserState(original)
      var page by mutableStateOf(original)
      var navigations = 0
      var retries = 0
      ComposeVisualFixture(800, 650, 1.5f) {
            AnalysisResultsPane(
                page,
                rows,
                browser,
                openAnalysis = { navigations++ },
                retryResults = { retries++ }) { key ->
                  Text("Evidence for $key")
                }
          }
          .use { fixture ->
            fixture.render()
            browser.listState.requestScrollToItem(19)
            fixture.render()
            fixture.awaitVisibleDescription("Inspect Finding 20")
            fixture.clickDescription("Inspect Finding 20")
            browser.query = "Finding"
            browser.filter = ResultBrowserFilter.Value("high")
            fixture.render()
            fixture.scrollBy(480f, "result-list")
            awaitResultListSettled(fixture, browser)
            val index = browser.listState.firstVisibleItemIndex
            val offset = browser.listState.firstVisibleItemScrollOffset
            assertTrue(index > 0)
            page = original.copy(section = original.section.copy(loading = true))
            fixture.render()
            fixture.assertTextFits("Loading results…")
            assertFalse(fixture.hasText("Retry loading results"))
            assertTrue(
                fixture.hasText(
                    "Reading saved ${type.workspace.name} results. Previously loaded results remain available below."))
            assertTrue(fixture.hasText("Evidence for finding-20"))
            assertEquals(index, browser.listState.firstVisibleItemIndex)
            assertEquals(offset, browser.listState.firstVisibleItemScrollOffset)
            page = original.copy(section = original.section.copy(error = "refresh failed"))
            fixture.render()
            fixture.assertTextFits("${type.workspace.name} · saved result read failed")
            fixture.assertTextFits("Results could not be refreshed: refresh failed")
            fixture.assertTextFits("Retry loading results")
            assertTrue(
                fixture.hasText(
                    "Reloads saved results for ${type.workspace.name}; does not start analysis."))
            val feedback = fixture.taggedBounds("result-read-feedback")
            val list = fixture.taggedBounds("result-list")
            assertTrue(feedback.bottom <= list.top)
            assertEquals(index, browser.listState.firstVisibleItemIndex)
            assertEquals(offset, browser.listState.firstVisibleItemScrollOffset)
            browser.query = "no matching title"
            fixture.render()
            fixture.assertTextFits("No matching results.")
            fixture.assertTextFits("Results could not be refreshed: refresh failed")
            assertTrue(
                fixture.taggedBounds("result-read-feedback").bottom <=
                    fixture.taggedBounds("result-empty").top)
            fixture.clickText("Retry loading results")
            assertEquals(1, retries)
            assertEquals(0, navigations)
            fixture.clickDescription("Clear filters")
            fixture.render()
            assertFalse(fixture.hasText("No matching results."))
            assertEquals(index, browser.listState.firstVisibleItemIndex)
            assertEquals(offset, browser.listState.firstVisibleItemScrollOffset)
            browser.listState.requestScrollToItem(19)
            fixture.render()
            fixture.awaitVisibleDescription("Inspect Finding 20")
            assertTrue(fixture.hasDescription("Inspect Finding 20"))
            assertEquals(ResultBrowserFilter.All, browser.filter)
            assertEquals("", browser.query)
            assertEquals("finding-1", browser.selectedKey)
            assertEquals(0, navigations)
            assertEquals(1, retries)
            fixture.clickText("View analysis")
            assertEquals(1, navigations)
            assertEquals(1, retries)
          }
    }
  }

  @Test
  fun bugsBrowsingDisclosuresAndCategoryNavigationDoNotInvokeWorkIntents() {
    val page = resultPageFixture("bugs")
    val findings =
        (1..2).map { number ->
          UnifiedFinding(
              id = "result-$number",
              projectId = page.project!!.projectId,
              projectRevision = page.project.projectRevision,
              category = "bugs",
              title = "Result $number",
              severity = if (number == 1) "high" else "",
              location = FindingLocation("internal/handler$number.go", startLine = number),
              evidence = "Local evidence $number")
        }
    val store = ResultBrowserStore()
    val bugsBrowser = store.stateFor(page)
    val security = page.copy(type = AnalysisResultType.Security)
    var category by mutableStateOf(AnalysisResultType.Bugs)
    val requests = mutableListOf<String>()
    ComposeVisualFixture(800, 650) {
          if (category == AnalysisResultType.Bugs)
              BugsWorkspacePane(
                  BugsWorkspacePaneState(findings, null, false, page, store.stateFor(page)),
                  BugsWorkspaceActions(
                      FindingActions(
                          { requests += "prepare/provider" },
                          { _, _ -> requests += "triage/write" },
                          { requests += "source/read" }),
                      { requests += "scan/execution" },
                      { requests += "cancel scan" },
                      { requests += "analysis/provider" },
                      { requests += "retry/read" }))
          else
              AnalysisResultsPane(
                  security,
                  emptyList(),
                  store.stateFor(security),
                  openAnalysis = { requests += "analysis/provider" },
                  retryResults = { requests += "retry/read" }) {
                    Text("unused")
                  }
        }
        .use { fixture ->
          fixture.render()
          fixture.clickDescription("Inspect Result 2")
          fixture.render()
          assertTrue(fixture.hasText("Result 2"))
          fixture.clickText("Evidence and fix criteria")
          fixture.render()
          assertTrue(fixture.hasText("Local evidence 2"))
          fixture.clickDescription("Filter results")
          fixture.setFocusedText("HANDLER2.GO")
          fixture.render()
          assertTrue(fixture.hasText("Result 2"))
          category = AnalysisResultType.Security
          fixture.render()
          category = AnalysisResultType.Bugs
          fixture.render()
          assertEquals("HANDLER2.GO", bugsBrowser.query)
          assertEquals(semanticResultRow(findings[1]).key, bugsBrowser.selectedKey)
          assertTrue(fixture.hasText("Result 2"))
          assertTrue(
              requests.isEmpty(),
              "Local browsing must not dispatch read, provider, scan or write intents")
        }
  }

  @Test
  fun unknownCountsAndUnavailableReadsNeverLookLikeCompletedEmpty() {
    val original = resultPageFixture("bugs")
    val progress = original.progress!!.copy(status = "unavailable", findingCount = null)
    val run = original.run!!.copy(status = "unavailable", sections = listOf(progress))
    val page = original.copy(run = run, section = AnalysisSectionState())
    ComposeVisualFixture(800, 650) {
          AnalysisResultsPane(page, emptyList(), newResultBrowserState(page), openAnalysis = {}) {
            Text("unused")
          }
        }
        .use { fixture ->
          fixture.render()
          fixture.assertTextFits("— reported (count unavailable)")
          fixture.assertTextFits("Analysis is unavailable for this category.")
          assertFalse(fixture.hasText("No findings in the analyzed scope."))
          assertFalse(fixture.hasText("0 findings"))
        }
  }

  @Test
  fun emptySavedResultReadsKeepFailureSeparateFromSuccessfulEmptiness() {
    AnalysisResultType.entries.forEach { type ->
      val original = resultPageFixture(type.category)
      var page by mutableStateOf(original.copy(section = AnalysisSectionState(loading = true)))
      val browser = newResultBrowserState(original)
      var retries = 0
      ComposeVisualFixture(800, 650, 1.5f) {
            AnalysisResultsPane(
                page, emptyList(), browser, openAnalysis = {}, retryResults = { retries++ }) {
                  Text("unused")
                }
          }
          .use { fixture ->
            fixture.render()
            fixture.assertTextFits("Loading results…")
            assertTrue(
                fixture.hasText(
                    "Reading saved ${type.workspace.name} results; no new analysis is being started."))
            assertFalse(fixture.hasText("No findings in the analyzed scope."))
            page = original.copy(section = AnalysisSectionState(error = ""))
            fixture.render()
            fixture.assertTextFits("${type.workspace.name} · saved result read failed")
            assertTrue(
                fixture.hasText(
                    "Results could not be refreshed: The saved result read failed without a diagnostic."))
            fixture.assertTextFits("No result details loaded yet.")
            fixture.assertTextFits("Retry loading results")
            fixture.clickText("Retry loading results")
            assertEquals(1, retries)
            assertFalse(fixture.hasText("No findings in the analyzed scope."))
            assertTrue(
                fixture.taggedBounds("result-read-feedback").bottom <=
                    fixture.taggedBounds("result-empty").top)
          }
    }
  }

  @Test
  fun longRefreshErrorsLeaveRetainedFindingsAndDisabledFixReachable() {
    val original = performancePageFixture()
    val refreshError = "The result refresh failed for a long project path. ".repeat(12)
    val stale =
        original.copy(
            run = original.run!!.copy(status = "stale"),
            section = original.section.copy(error = refreshError))
    var fixes = 0
    ComposeVisualFixture(800, 650, 1.5f) {
          PerformanceWorkspacePane(
              PerformanceWorkspacePaneState(stale, resultIndexFixture()),
              PerformanceWorkspaceActions(
                  { fixes++ }, {}, FindingActions({}, { _, _ -> }, {}), openSource = {}))
        }
        .use { fixture ->
          fixture.render("rounded-results-long-error-800-150")
          assertTrue(fixture.taggedBounds("result-list").height > 0f)
          assertTrue(fixture.hasText("Results could not be refreshed: $refreshError"))
          fixture.clickDescription("Inspect Avoid repeated allocation")
          fixture.render()
          fixture.revealText("Prepare fix", "result-detail")
          assertTrue(fixture.isDisabled("Prepare fix"))
          assertEquals(0, fixes)
        }
  }

  @Test
  fun compactPerformanceDetailKeepsUnmeasuredRecommendationAndFixReachable() {
    val page = performancePageFixture()
    val results = requireNotNull(page.results)
    val longTradeoff =
        "Retained buffers can increase memory under bursty workloads; measure the representative request mix before accepting the change. "
            .repeat(5)
    val populated =
        page.copy(
            section =
                page.section.copy(
                    results =
                        results.copy(
                            performance =
                                results.performance.map { report ->
                                  report.copy(
                                      findings =
                                          report.findings.map {
                                            it.copy(
                                                workloadConditions =
                                                    "High sustained request volume across multiple payload sizes.",
                                                tradeoff = longTradeoff)
                                          })
                                })))
    var fixes = 0
    ComposeVisualFixture(800, 400, 1.5f) {
          PerformanceWorkspacePane(
              PerformanceWorkspacePaneState(populated, resultIndexFixture()),
              PerformanceWorkspaceActions(
                  { fixes++ }, {}, FindingActions({}, { _, _ -> }, {}), openSource = {}))
        }
        .use { fixture ->
          fixture.render("performance-detail-compact-800-400-150")
          assertTrue(fixture.hasText("Not measured · explicit local execution"))
          fixture.clickDescription("Inspect Avoid repeated allocation")
          fixture.render()
          fixture.revealText("Prepare fix", "result-detail")
          assertTrue(fixture.hasText("Model suggestion"))
          assertTrue(
              fixture.hasText(
                  "Unmeasured recommendation. Benchmark the affected workload before claiming an improvement."))
          assertFalse(fixture.hasText("Dismiss"))
          assertEquals(0, fixes)
          assertTrue(fixture.hasText("Workload conditions"))
          assertTrue(fixture.hasText("Potential impact · qualitative, not a measured gain"))
          assertTrue(fixture.hasText("Model confidence · not a measurement or speedup probability"))
          assertTrue(fixture.hasText("Trade-offs"))
          assertTrue(fixture.hasText("Verification plan"))
          fixture.assertTextWrapsWithoutClipping(longTradeoff)
          fixture.revealText("Report metadata", "result-detail")
          fixture.clickText("Report metadata")
          fixture.render()
          assertTrue(fixture.hasText("Provider origin"))
          fixture.scrollBy(100_000f, "result-detail")
          fixture.render()
          assertTrue(fixture.verticalScrollValue("result-detail") > 0f)
          fixture.revealText("Prepare fix", "result-detail")
          fixture.clickText("Prepare fix")
          assertEquals(1, fixes)
        }
  }

  @Test
  fun securityEmptyAndRetainedStatesKeepWarningsAndRecoveryReachable() {
    val original = securityPageFixture()
    val reports = requireNotNull(original.results)
    val source = reports.security.first()
    val ai = reports.security.last()
    val warning =
        "Security findings describe analyzed evidence, not proof of safety. No findings does not mean the project is secure; unavailable evidence and incomplete coverage remain unknown."
    val completedProgress =
        requireNotNull(original.progress).copy(status = "completed_empty", findingCount = 0)
    val completed =
        original.copy(
            run =
                requireNotNull(original.run)
                    .copy(
                        status = "completed",
                        sections =
                            original.run.sections.map {
                              if (it.category == "security") completedProgress else it
                            }),
            section =
                original.section.copy(
                    results =
                        reports.copy(
                            progress = completedProgress,
                            security =
                                listOf(
                                    source.copy(
                                        status = "completed_empty", findings = emptyList())))))
    val cases =
        listOf(
            Triple("completed-empty", completed, "No findings in the analyzed scope."),
            Triple(
                "unavailable",
                original.copy(
                    section =
                        original.section.copy(
                            results =
                                reports.copy(
                                    security =
                                        listOf(
                                            ai.copy(
                                                status = "unavailable",
                                                reason = "Provider not configured",
                                                findings = emptyList()))))),
                "ai · main.go · Unavailable: Provider not configured"),
            Triple(
                "partial",
                original,
                "deterministic · main.go · Partial: Some rules were unavailable."),
            Triple(
                "stale",
                original.copy(run = original.run.copy(status = "stale")),
                "Analyze again to prepare a fix from current source."),
            Triple(
                "retained-error",
                original.copy(
                    section = original.section.copy(error = "Saved results could not refresh")),
                "Results could not be refreshed: Saved results could not refresh"))
    for ((name, page, expected) in cases) {
      var requests = 0
      ComposeVisualFixture(800, 650, 1.5f) {
            SecurityWorkspacePane(
                SecurityWorkspacePaneState(page, resultIndexFixture()),
                SecurityWorkspaceActions(
                    { _, _ -> requests++ },
                    { _, _ -> requests++ },
                    { requests++ },
                    FindingActions({}, { _, _ -> }, {}),
                    { requests++ }))
          }
          .use { fixture ->
            fixture.render("f19-security-$name")
            fixture.revealText(warning, "result-overview")
            assertTrue(fixture.hasText(warning))
            if (name == "stale") {
              fixture.clickDescription("Inspect Credential-like assignment")
              fixture.render()
            }
            fixture.revealText(
                expected,
                if (name == "stale") "result-detail"
                else if (name == "retained-error") "result-read-feedback"
                else if (name == "completed-empty") "result-empty" else "result-overview")
            assertTrue(fixture.hasText(expected), name)
            if (name == "completed-empty" || name == "unavailable") {
              assertFalse(fixture.hasText("0 hypotheses"))
              assertFalse(fixture.hasText("Prepare fix"))
            } else {
              if (name != "stale") {
                fixture.clickDescription("Inspect Credential-like assignment")
                fixture.render()
              }
              revealDetailAction(fixture, "Prepare fix")
              if (name == "stale") assertTrue(fixture.isDisabled("Prepare fix"))
              if (name == "retained-error") {
                assertTrue(fixture.hasText("Credential-like assignment"))
                fixture.revealText("Retry loading results", "result-read-feedback")
                fixture.assertTextFits("Retry loading results")
              }
            }
            assertEquals(0, requests, name)
          }
    }
  }

  @Test
  fun securityReportAvailabilityStaysOutsideFindingSelectionAndOptionalDisclosure() {
    val original = securityPageFixture()
    val source = original.results!!.security.first()
    val ai =
        source.copy(
            source = "ai",
            status = "failed",
            findings = emptyList(),
            reason = "AI evidence could not be loaded")
    val page =
        original.copy(
            section =
                original.section.copy(
                    results = original.results!!.copy(security = listOf(source, ai))))
    val warning =
        "Security findings describe analyzed evidence, not proof of safety. No findings does not mean the project is secure; unavailable evidence and incomplete coverage remain unknown."
    ComposeVisualFixture(800, 650, 1.5f) {
          SecurityWorkspacePane(
              SecurityWorkspacePaneState(page, resultIndexFixture()),
              SecurityWorkspaceActions(
                  { _, _ -> }, { _, _ -> }, {}, FindingActions({}, { _, _ -> }, {})))
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText(warning))
          assertTrue(fixture.hasText("ai · main.go · Failed: AI evidence could not be loaded"))
          assertTrue(
              fixture.hasText("deterministic · main.go · Partial: Some rules were unavailable."))
          assertTrue(fixture.hasText("Source rule · Partial"))
          assertTrue(fixture.hasDescription("Inspect Credential-like assignment"))
          fixture.clickDescription("Inspect Credential-like assignment")
          fixture.render()
          assertTrue(fixture.hasText("Source rule"))
          assertTrue(fixture.hasText(warning))
        }

    val unknown =
        source.copy(
            source = "other",
            findings =
                listOf(source.findings.single().copy(id = "unknown", title = "Unknown provenance")))
    val provenance =
        original.copy(
            section =
                original.section.copy(
                    results =
                        original.results!!.copy(
                            security =
                                listOf(source, original.results!!.security.last(), unknown))))
    ComposeVisualFixture(800, 650) {
          SecurityWorkspacePane(
              SecurityWorkspacePaneState(provenance, resultIndexFixture()),
              SecurityWorkspaceActions(
                  { _, _ -> }, { _, _ -> }, {}, FindingActions({}, { _, _ -> }, {})))
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText("Source rule · Partial"))
          assertTrue(fixture.hasText("Model hypothesis · Partial"))
          assertTrue(fixture.hasText("Evidence type unavailable · Partial"))
        }

    listOf("failed", "partial", "unavailable").forEach { status ->
      val findingFree =
          original.copy(
              section =
                  original.section.copy(
                      results =
                          original.results!!.copy(
                              security =
                                  listOf(
                                      ai.copy(
                                          status = status,
                                          reason = "Reason for $status",
                                          findings = emptyList())))))
      ComposeVisualFixture(800, 650) {
            SecurityWorkspacePane(
                SecurityWorkspacePaneState(findingFree, resultIndexFixture()),
                SecurityWorkspaceActions(
                    { _, _ -> }, { _, _ -> }, {}, FindingActions({}, { _, _ -> }, {})))
          }
          .use { fixture ->
            fixture.render()
            assertTrue(fixture.hasText(warning))
            assertTrue(
                fixture.hasText(
                    "ai · main.go · ${status.replaceFirstChar(Char::uppercase)}: Reason for $status"))
            assertFalse(fixture.hasDescription("Inspect Credential-like assignment"))
          }
    }

    val empty =
        original.copy(
            run =
                original.run!!.copy(
                    status = "completed",
                    sections =
                        original.run.sections.map {
                          if (it.category == "security")
                              it.copy(status = "completed_empty", findingCount = 0)
                          else it
                        }),
            section =
                original.section.copy(
                    results =
                        original.results!!.copy(
                            progress =
                                original.progress!!.copy(
                                    status = "completed_empty", findingCount = 0),
                            security =
                                listOf(
                                    source.copy(
                                        status = "completed_empty",
                                        findings = emptyList(),
                                        reason = "")))))
    ComposeVisualFixture(800, 650, 1.5f) {
          SecurityWorkspacePane(
              SecurityWorkspacePaneState(empty, resultIndexFixture()),
              SecurityWorkspaceActions(
                  { _, _ -> }, { _, _ -> }, {}, FindingActions({}, { _, _ -> }, {})))
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText(warning))
          assertTrue(fixture.hasText("No findings in the analyzed scope."))
          assertFalse(fixture.hasText("0 hypotheses"))
        }

    val unavailable =
        original.copy(
            section =
                original.section.copy(
                    error = "Saved result read failed",
                    results =
                        original.results!!.copy(
                            security =
                                listOf(
                                    ai.copy(
                                        status = "unavailable", reason = "Model not configured")))))
    ComposeVisualFixture(800, 650, 1.5f) {
          SecurityWorkspacePane(
              SecurityWorkspacePaneState(unavailable, resultIndexFixture()),
              SecurityWorkspaceActions(
                  { _, _ -> }, { _, _ -> }, {}, FindingActions({}, { _, _ -> }, {})))
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText(warning))
          assertTrue(fixture.hasText("ai · main.go · Unavailable: Model not configured"))
          assertTrue(fixture.hasText("Results could not be refreshed: Saved result read failed"))
        }
  }

  @Test
  fun securityDetailExposesSuppliedEvidenceAndMetadataWithoutActivatingModelProse() {
    val original = securityPageFixture()
    val longPath = "nested/".repeat(18) + "boundary.go"
    val literal = "<img src='https://example.invalid/track'> [reference](https://example.invalid)"
    val source = original.results!!.security.first()
    val rule =
        source.copy(
            path = longPath,
            scope = "package boundary",
            ruleSetVersion = "rules-7",
            generatedAt = "2026-01-02T03:04:05Z",
            findings =
                listOf(
                    source.findings
                        .single()
                        .copy(
                            anchor = SecuritySourceAnchor(longPath, 4, 5, "Run"),
                            category = "secrets",
                            confidence = "medium",
                            cwe = "CWE-798",
                            reference = literal,
                            triage = "reviewing",
                            verificationState = "unverified",
                            preconditions = "Only if reachable",
                            verificationIdea = "Inspect test fixtures without execution",
                            engineeringInsight =
                                EngineeringInsight(
                                    mechanism = literal,
                                    whyItMattersHere = "User input crosses a boundary",
                                    tradeoffOrFailureMode = "Rotation may invalidate fixtures",
                                    transferableLesson = "Keep secrets out of source"))))
    val ai =
        source.copy(
            source = "ai",
            model = "review-model",
            configuredModel = "configured-model",
            profile = "advisory",
            providerOrigin = "local",
            reasoningEffort = "low",
            promptVersion = "prompt-3",
            contextPolicyVersion = "context-2",
            findings =
                listOf(
                    rule.findings
                        .single()
                        .copy(
                            id = "model-2",
                            title = "Hypothesis about the same boundary",
                            evidenceKind = "model_suspicion",
                            verificationIdea = "")))
    val page =
        original.copy(
            section =
                original.section.copy(
                    results = original.results!!.copy(security = listOf(rule, ai))))
    var privileged = 0
    var inspected: SecurityResult? = null
    var selectionCurrent: (() -> Boolean)? = null
    val browser = newResultBrowserState(page)
    ComposeVisualFixture(800, 650, 1.5f) {
          SecurityWorkspacePane(
              SecurityWorkspacePaneState(page, resultIndexFixture(), browser),
              SecurityWorkspaceActions(
                  { _, _ -> privileged++ },
                  { result, guard ->
                    inspected = result
                    selectionCurrent = guard
                    privileged++
                  },
                  { privileged++ },
                  FindingActions({}, { _, _ -> }, {})))
        }
        .use { fixture ->
          fixture.render()
          fixture.revealText("Credential-like assignment", "result-list")
          fixture.clickDescription("Inspect Credential-like assignment")
          fixture.render()
          assertTrue(fixture.hasText("Source rule"))
          assertTrue(
              fixture.hasText(
                  "A source rule match identifies a pattern; it does not confirm a vulnerability."))
          listOf(
                  "High",
                  "Reported range",
                  "4–5",
                  "Run",
                  "partial",
                  "secrets",
                  "medium",
                  "CWE-798",
                  "reviewing",
                  "unverified",
                  "Only if reachable",
                  "Inspect test fixtures without execution",
                  "Engineering insight",
                  literal,
                  "User input crosses a boundary",
                  "Rotation may invalidate fixtures",
                  "Keep secrets out of source")
              .forEach { assertTrue(fixture.hasText(it), "Missing detail: $it") }
          fixture.revealText(longPath + ":4", "result-detail")
          assertTrue(fixture.taggedTextCount("result-detail", longPath + ":4") > 0)
          assertFalse(fixture.hasText("Scope"))
          fixture.revealText("Report metadata", "result-detail")
          fixture.clickText("Report metadata")
          fixture.render()
          listOf("package boundary", "rules-7", "2026-01-02T03:04:05Z", "base").forEach {
            assertTrue(fixture.hasText(it), "Missing metadata: $it")
          }
          fixture.revealText("Report metadata", "result-detail")
          fixture.clickText("Report metadata")
          fixture.clickDescription("Severity High 2")
          fixture.render()
          fixture.revealText("Hypothesis about the same boundary", "result-list")
          assertTrue(fixture.hasDescription("Inspect Hypothesis about the same boundary"))
          fixture.clickDescription("Filter results")
          fixture.setFocusedText("Hypothesis about the same boundary")
          fixture.render()
          fixture.clickDescription("Inspect Hypothesis about the same boundary")
          fixture.render()
          assertTrue(fixture.hasText("Model hypothesis"))
          assertTrue(
              fixture.hasText(
                  "Unverified model hypothesis. Validate the preconditions and source evidence before remediation."))
          assertTrue(fixture.hasText("Not supplied."))
          assertFalse(fixture.hasText("package boundary"))
          fixture.revealText("Report metadata", "result-detail")
          fixture.clickText("Report metadata")
          fixture.render()
          listOf("review-model", "configured-model", "advisory", "local", "prompt-3", "context-2")
              .forEach { assertTrue(fixture.hasText(it), "Missing metadata: $it") }
          assertTrue(fixture.hasText("Generated at"))
          assertFalse(fixture.hasText("2026-01-02T03:04:05Z"))
          assertEquals(0, privileged)
          fixture.revealText("Open source", "result-detail")
          fixture.clickText("Open source")
          assertEquals("ai", inspected?.report?.source)
          assertTrue(
              requireNotNull(selectionCurrent)(),
              "selected=${browser.selectedKey}, expected=${inspected?.rowKey}, filter=${browser.filter}, query=${browser.query}")
          assertEquals(1, privileged)
          fixture.clickDescription("Filter results")
          fixture.setFocusedText("no matching result")
          fixture.render()
          assertFalse(requireNotNull(selectionCurrent)())
          assertEquals(1, privileged)
        }
  }

  @Test
  fun securityDetailKeepsEvidenceWarningsVisibleAndLongVerificationOnDemand() {
    val original = securityPageFixture()
    val results = requireNotNull(original.results)
    val source = results.security.single { it.source == "deterministic" }
    val longWarning =
        "Some deterministic rules were unavailable for this file; rerun the scoped analysis before relying on complete coverage. "
            .repeat(5)
    val longPreconditions =
        "The value may be a placeholder or test fixture, and external reachability is unknown until the owning request path is reviewed. "
            .repeat(5)
    val populated =
        original.copy(
            section =
                original.section.copy(
                    results =
                        results.copy(
                            security =
                                listOf(
                                    source.copy(
                                        reason = longWarning,
                                        findings =
                                            source.findings.map {
                                              it.copy(preconditions = longPreconditions)
                                            })))))
    var fixes = 0
    ComposeVisualFixture(800, 400, 1.5f) {
          SecurityWorkspacePane(
              SecurityWorkspacePaneState(populated, resultIndexFixture()),
              SecurityWorkspaceActions(
                  { _, _ -> fixes++ }, { _, _ -> }, {}, FindingActions({}, { _, _ -> }, {})))
        }
        .use { fixture ->
          fixture.render("security-detail-compact-800-400-150")
          fixture.clickDescription("Inspect Credential-like assignment")
          fixture.render()
          fixture.revealText("Prepare fix", "result-detail")
          assertTrue(fixture.hasText("Source rule"))
          assertTrue(
              fixture.hasText(
                  "A source rule match identifies a pattern; it does not confirm a vulnerability."))
          assertTrue(fixture.hasText(longWarning))
          assertTrue(fixture.hasText("Preconditions / unknowns"))
          assertTrue(fixture.hasText("Safe verification idea · not performed"))
          assertFalse(fixture.hasText("Scope"))
          fixture.revealText("Report metadata", "result-detail")
          fixture.clickText("Report metadata")
          fixture.render("security-detail-disclosure-800-400-150")
          assertTrue(fixture.hasText("Scope"))
          assertTrue(fixture.taggedTextCount("result-detail", longPreconditions) > 0)
          fixture.scrollBy(100_000f, "result-detail")
          fixture.render()
          assertTrue(fixture.verticalScrollValue("result-detail") > 0f)
          revealDetailAction(fixture, "Prepare fix")
          fixture.clickText("Prepare fix")
          assertEquals(1, fixes)
          fixture.scrollBy(100_000f, "result-detail")
          fixture.render()
          assertTrue(fixture.verticalScrollValue("result-detail") > 0f)
        }

    val stale = populated.copy(run = requireNotNull(populated.run).copy(status = "stale"))
    ComposeVisualFixture(800, 400, 1.5f) {
          SecurityWorkspacePane(
              SecurityWorkspacePaneState(stale, resultIndexFixture()),
              SecurityWorkspaceActions(
                  { _, _ -> fixes++ }, { _, _ -> }, {}, FindingActions({}, { _, _ -> }, {})))
        }
        .use { fixture ->
          fixture.render("security-detail-stale-800-400-150")
          fixture.clickDescription("Inspect Credential-like assignment")
          fixture.render()
          revealDetailAction(fixture, "Prepare fix")
          assertTrue(fixture.isDisabled("Prepare fix"))
          assertTrue(fixture.hasText("Analyze again to prepare a fix from current source."))
          assertEquals(1, fixes)
        }
  }

  @Test
  fun longRefreshErrorWithoutRowsRemainsReachableInShortLargeTextWindow() {
    val original = performancePageFixture()
    val refreshError =
        "The result refresh failed for a long project path. ".repeat(24) +
            "Final diagnostic detail remains available."
    val page =
        original.copy(
            section =
                original.section.copy(
                    error = refreshError,
                    results = requireNotNull(original.results).copy(semantic = emptyList())))
    var openedAnalysis = 0
    ComposeVisualFixture(800, 400, 1.5f) {
          AnalysisResultsPane(
              page = page,
              rows = emptyList(),
              browser = newResultBrowserState(page),
              facetLabel = "Impact",
              openAnalysis = { openedAnalysis++ }) {
                Text("unused")
              }
        }
        .use { fixture ->
          fixture.render("results-empty-long-error-800-400-150")
          fixture.assertTextWrapsAndTailIsReachable(
              "Results could not be refreshed: $refreshError", "result-read-feedback")
          assertTrue(fixture.verticalScrollValue("result-read-feedback") > 0f)
          assertTrue(fixture.taggedBounds("result-empty").height > 0f)
          fixture.clickText("View analysis")
          assertEquals(1, openedAnalysis)
        }
  }

  @Test
  fun detailKeepsTheCompleteTitlePathAndModelProse() {
    val title =
        "Reject credential-like assignments when a generated configuration value crosses the trusted input boundary"
    val path =
        "internal/platform/transport/generated/configuration/validation/credential_assignment_handler.go:128 · validateCredentialAssignment"
    val prose =
        "The complete model explanation remains selectable in the detail pane, including literal fallback text such as <review-result>."
    val original = resultPageFixture("bugs")
    val finding =
        UnifiedFinding(
            id = "long-content",
            category = original.category,
            projectId = requireNotNull(original.project).projectId,
            projectRevision = requireNotNull(original.run).identity.projectRevision,
            severity = "high",
            source = "file_analysis",
            confidence = "suggested",
            title = title,
            message = prose,
            location =
                FindingLocation(
                    path.substringBefore(":"),
                    startLine = 128,
                    symbol = "validateCredentialAssignment"),
            status = "open",
            freshness = "fresh")
    val page =
        original.copy(
            section =
                original.section.copy(
                    results = requireNotNull(original.results).copy(semantic = listOf(finding))))
    var opened: UnifiedFinding? = null
    var prepared = 0
    ComposeVisualFixture(800, 650, 1.5f) {
          BugsWorkspacePane(
              BugsWorkspacePaneState(
                  page.semantic,
                  null,
                  false,
                  page,
                  index =
                      ProjectIndex(
                          finding.projectId,
                          finding.projectRevision,
                          files = listOf(IndexedFile(finding.location.path, "hash", "Go", false)))),
              BugsWorkspaceActions(
                  FindingActions({ prepared++ }, { _, _ -> }, { opened = it }), {}, {}))
        }
        .use { fixture ->
          fixture.render()
          fixture.clickDescription("Inspect $title")
          fixture.render("rounded-results-full-content-800-150")
          fixture.assertTextWrapsWithoutClipping(title)
          assertTrue(fixture.hasText(path))
          fixture.assertTextWrapsWithoutClipping(prose)
          assertTrue(fixture.hasText("Model suggestion"))
          fixture.clickText("Open source")
          assertEquals(finding, opened)
          assertEquals(0, prepared)
          assertTrue(fixture.isDisabled("Prepare fix"))
        }
  }

  @Test
  fun bugsDetailRetainsLongEvidenceCriteriaAndReviewOnlyCandidateBehindDisclosure() {
    val original = resultPageFixture("bugs")
    val title = "An exact and very long finding title ".repeat(5).trim()
    val path = "internal/very/deeply/nested/path/to/a/source/file/with/a/long/name/handler.go"
    val message =
        "Full message with <script>ignored</script> and https://example.test/proof ".repeat(12)
    val evidence = "Evidence with ![remote image](https://example.test/image.png) ".repeat(12)
    val criteria = "Keep all acceptance criteria even when they are long. ".repeat(12)
    val nonGoal = "Do not modify unrelated declarations. ".repeat(12)
    val finding =
        UnifiedFinding(
            id = "long-evidence",
            projectId = original.project!!.projectId,
            projectRevision = original.run!!.identity.projectRevision,
            source = "file_analysis",
            confidence = "suggested",
            severity = "high",
            title = title,
            message = message,
            evidence = evidence,
            location = FindingLocation(path, startLine = 91, symbol = "Serve"),
            status = "partial",
            freshness = "stale",
            taskSpec =
                BugTaskSpec(
                    targetSymbol = "Serve",
                    targetSignature = "func Serve() error",
                    acceptanceCriteria = listOf(criteria),
                    nonGoals = listOf(nonGoal),
                    goTestCandidate =
                        GoTestCandidateSpec("TestServe", "Review only: assert error.")),
            engineeringInsight = EngineeringInsight(mechanism = "Insight retained for review."))
    val page =
        original.copy(
            section =
                original.section.copy(
                    results = original.results!!.copy(semantic = listOf(finding))))
    ComposeVisualFixture(800, 650, 1.5f) {
          BugsWorkspacePane(
              BugsWorkspacePaneState(listOf(finding), null, false, page),
              BugsWorkspaceActions(FindingActions({}, { _, _ -> }, {}), {}, {}))
        }
        .use { fixture ->
          fixture.render()
          fixture.clickDescription("Inspect $title")
          fixture.render()
          assertTrue(fixture.hasText(title))
          assertTrue(fixture.hasText("$path:91 · Serve"))
          assertTrue(fixture.hasText("Partial · Stale"))
          assertTrue(fixture.hasText("Model suggestion"))
          assertTrue(fixture.hasText(message))
          assertFalse(fixture.hasText(evidence))
          fixture.revealText("Evidence and fix criteria", "result-detail")
          fixture.clickText("Evidence and fix criteria")
          fixture.render()
          assertTrue(fixture.hasText(evidence))
          assertTrue(fixture.hasText(criteria))
          assertTrue(fixture.hasText(nonGoal))
          assertTrue(fixture.hasText("Test candidate · review only"))
          assertTrue(fixture.hasText("TestServe"))
          assertFalse(fixture.hasDescription("https://example.test/proof"))
          assertFalse(fixture.hasDescription("https://example.test/image.png"))
          fixture.revealText("Engineering insight", "result-detail")
          if (!fixture.hasText("Insight retained for review.")) {
            fixture.clickText("Engineering insight")
            fixture.render()
          }
          assertTrue(fixture.hasText("Insight retained for review."))
        }
  }

  @Test
  fun bugsChecksKeepTrustAndCommandsReachableInCompactLongDiagnosticViews() {
    val diagnostic = "tool diagnostic from a deeply nested project path ".repeat(20)
    var starts = 0
    ComposeVisualFixture(800, 400, 1.5f) {
          BugsWorkspacePane(
              BugsWorkspacePaneState(
                  emptyList(),
                  GoScanReport(
                      projectId = "project",
                      projectRevision = "revision",
                      status = "failed",
                      phases = listOf(GoScanPhase("go vet", "failed", output = diagnostic))),
                  false,
                  project = resultProjectFixture(),
                  scanState = VerifiedScanState(read = VerifiedScanRead.Loaded)),
              BugsWorkspaceActions(FindingActions({}, { _, _ -> }, {}), { starts++ }, {}))
        }
        .use { fixture ->
          fixture.render("bugs-checks-long-diagnostic-800-400-150")
          fixture.assertTextFits("Verified Go scan")
          fixture.assertTextFits("Trust project-code execution & run checks")
          fixture.clickText("Command and output")
          fixture.render()
          assertTrue(fixture.hasText(sanitizedOutputText(diagnostic)))
          assertEquals(0, starts)
        }
  }

  @Test
  fun scanProgressAndRecoveryRemainAvailableWithDetailsCollapsed() {
    val project = resultProjectFixture()
    val report = GoScanReport(project.projectId, project.projectRevision, "running")
    val page = resultPageFixture("bugs")
    var scanState by mutableStateOf(VerifiedScanState(read = VerifiedScanRead.Loaded))
    val requests = mutableListOf<String>()
    ComposeVisualFixture(800, 400, 1.5f) {
          BugsWorkspacePane(
              BugsWorkspacePaneState(
                  emptyList(), report, false, page, project = project, scanState = scanState),
              BugsWorkspaceActions(
                  FindingActions({}, { _, _ -> }, {}),
                  { requests += "start" },
                  { requests += "cancel" },
                  refreshScanStatus = { requests += "status" }))
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText("Running"))
          assertTrue(fixture.hasText("Cancel checks"))
          assertTrue(fixture.hasText("Refresh scan status"))
          assertTrue(fixture.hasText("Command and output"))
          scanState = scanState.copy(operation = VerifiedScanOperation.Starting)
          fixture.render()
          assertTrue(fixture.hasText("Starting"))
          assertFalse(fixture.hasText("Refresh scan status"))
          scanState =
              scanState.copy(
                  operation = VerifiedScanOperation.Idle, read = VerifiedScanRead.Reading)
          fixture.render()
          assertTrue(fixture.hasText("Reading status"))
          assertFalse(fixture.hasText("Refresh scan status"))
          scanState =
              scanState.copy(
                  read = VerifiedScanRead.Loaded,
                  operation = VerifiedScanOperation.CancellationRequested)
          fixture.render()
          assertTrue(fixture.hasText("Cancellation requested"))
          assertTrue(fixture.isDisabled("Trust project-code execution & run checks"))
          assertFalse(fixture.hasText("Refresh scan status"))
          scanState =
              scanState.copy(
                  operation =
                      VerifiedScanOperation.CancellationUnconfirmed(
                          "Cancel timed out; refresh scan status."))
          fixture.render()
          assertTrue(fixture.hasText("Cancel timed out; refresh scan status."))
          fixture.clickText("Refresh scan status")
          assertEquals(listOf("status"), requests)
          scanState =
              scanState.copy(
                  operation = VerifiedScanOperation.Idle,
                  read = VerifiedScanRead.PollUnavailable("Poll timed out"),
                  findingsRefresh = VerifiedScanFindingsRefresh.Unavailable("Findings read failed"))
          fixture.render()
          assertTrue(fixture.hasText("Poll timed out"))
          scanState = scanState.copy(read = VerifiedScanRead.Unavailable("Status read failed"))
          fixture.render()
          assertTrue(fixture.hasText("Status read failed"))
          assertTrue(
              fixture.hasText(
                  "Tool findings unavailable: Findings read failed. Previously loaded findings may be stale."))
          assertTrue(fixture.hasText("Refresh scan status"))
          assertEquals(listOf("status"), requests)
        }
  }

  private fun revealDetailAction(fixture: ComposeVisualFixture, label: String) {
    fixture.scrollBy(-100_000f, "result-detail")
    fixture.render()
    repeat(1000) {
      val action = runCatching { fixture.firstVisibleTextBounds(label) }.getOrNull()
      val detail = fixture.taggedBounds("result-detail")
      if (action != null && action.top >= detail.top && action.bottom <= detail.bottom) return
      fixture.scrollBy(24f, "result-detail")
      fixture.render()
    }
    error("$label must be reachable inside the detail viewport")
  }

  private fun row(number: Int) =
      ResultRowPresentation(
          "finding-$number",
          "Finding $number",
          "internal/handler.go:$number",
          "The full evidence remains available on inspection.",
          "high",
          "",
          "")

  private fun awaitResultListSettled(
      fixture: ComposeVisualFixture,
      browser: ResultBrowserState,
  ) {
    val deadline = System.nanoTime() + 2_000_000_000L
    fixture.render()
    while (browser.listState.isScrollInProgress && System.nanoTime() < deadline) {
      fixture.render()
      Thread.yield()
    }
    assertFalse(browser.listState.isScrollInProgress, "Timed out waiting for result list scroll")
    fixture.render()
  }
}
