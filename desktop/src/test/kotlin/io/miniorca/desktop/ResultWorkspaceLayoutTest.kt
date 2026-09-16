package io.miniorca.desktop

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material.Text
import androidx.compose.runtime.mutableStateOf
import androidx.compose.ui.Modifier
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.unit.dp
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class ResultWorkspaceLayoutTest {
  @Test
  fun allResultPagesKeepHeadersAndSeparatePanesInsideTheViewport() {
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
                  val list = fixture.taggedBounds("result-list")
                  assertTrue(header.left > 0 && header.right < width)
                  assertTrue(list.top > header.bottom && list.bottom < height)
                  assertTrue(list.height > height / 3f)
                  if (resultListDetailUsesTwoPanes(width.dp, scale)) {
                    val detail = fixture.taggedBounds("result-detail")
                    assertTrue(list.right < detail.left && detail.right < width)
                    assertEquals(list.top, detail.top)
                    assertEquals(list.bottom, detail.bottom)
                  }
                }
          }
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
          fixture.assertTextFits("Unknown")
          assertEquals(1, fixture.textCount("Unknown"))
          fixture.clickDescription("Filter results")
          fixture.setFocusedText("no loaded finding")
          fixture.render("results-filters-no-match-800-150")
          fixture.assertTextFits("No matching results.")
          fixture.assertTextFits("Clear filters to view loaded results.")
          fixture.assertTextFits("Clear filters")
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
            Triple("running", null, "No findings yet."),
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
  fun backToResultsRetainsThePositionOfTheInspectedFinding() {
    val rows = (1..30).map { row(it) }
    val browser = newResultBrowserState(resultPageFixture("bugs"))
    ComposeVisualFixture(800, 650) {
          ResultListDetail(
              rows, browser, { browser.selectedKey = it }, "No findings", Modifier.fillMaxSize()) {
                Text("Evidence for $it")
              }
        }
        .use { fixture ->
          fixture.render()
          fixture.revealText("Finding 20", "result-list")
          fixture.clickDescription("Inspect Finding 20")
          fixture.render()
          fixture.clickText("Back to results")
          fixture.render()
          fixture.assertTextFits("Finding 20")
          assertTrue(fixture.isDescriptionFocused("Inspect Finding 20"))
        }
  }

  @Test
  fun filteringClearsCompactDetailAndArrowKeysKeepLongListsNavigable() {
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
              PerformanceWorkspaceActions({ _, _ -> fixes++ }, {}, FindingActions({}, { _, _ -> })))
        }
        .use { fixture ->
          fixture.render("rounded-results-long-error-800-150")
          assertTrue(fixture.taggedBounds("result-list").height > 250f)
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
              PerformanceWorkspaceActions({ _, _ -> fixes++ }, {}, FindingActions({}, { _, _ -> })))
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
          fixture.revealText("Workload, trade-offs and verification", "result-detail")
          fixture.clickText("Workload, trade-offs and verification")
          fixture.render()
          assertEquals(0, fixes)
          assertTrue(fixture.hasText("When it matters"))
          fixture.assertTextWrapsAndTailIsReachable(longTradeoff, "result-detail")
          fixture.revealText("Prepare fix", "result-detail")
          fixture.clickText("Prepare fix")
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
          fixture.scrollBy(100_000f, "result-empty")
          fixture.render()
          assertTrue(fixture.verticalScrollValue("result-empty") > 0f)
          assertTrue(fixture.hasText(refreshError))
          fixture.clickText("View analysis")
          assertEquals(1, openedAnalysis)
        }
  }

  @Test
  fun compactDetailKeepsTheCompleteTitlePathAndModelProse() {
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
    ComposeVisualFixture(800, 650, 1.5f) {
          BugsWorkspacePane(
              BugsWorkspacePaneState(page.semantic, null, false, page),
              BugsWorkspaceActions(FindingActions({}, { _, _ -> }), {}, {}))
        }
        .use { fixture ->
          fixture.render()
          assertFalse(fixture.hasText(prose), "Rows must not repeat the explanation preview")
          fixture.clickDescription("Inspect $title")
          fixture.render("rounded-results-full-content-800-150")
          fixture.assertTextWrapsWithoutClipping(title)
          fixture.assertTextWrapsWithoutClipping(path)
          fixture.assertTextWrapsWithoutClipping(prose)
          assertTrue(fixture.hasText("Model suggestion"))
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
                      status = "failed",
                      phases = listOf(GoScanPhase("go vet", "failed", output = diagnostic))),
                  false),
              BugsWorkspaceActions(FindingActions({}, { _, _ -> }), { starts++ }, {}))
        }
        .use { fixture ->
          fixture.render("bugs-checks-long-diagnostic-800-400-150")
          fixture.assertTextFits("Verified checks")
          fixture.assertTextFits("Trust project-code execution & run checks")
          fixture.clickText("Command and output")
          fixture.render()
          assertTrue(fixture.hasText(sanitizedOutputText(diagnostic)))
          assertEquals(0, starts)
        }
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
