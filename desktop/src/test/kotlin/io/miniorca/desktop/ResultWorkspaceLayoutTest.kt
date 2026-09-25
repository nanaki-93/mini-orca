package io.miniorca.desktop

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material.Text
import androidx.compose.runtime.mutableStateOf
import androidx.compose.ui.Modifier
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.unit.dp
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class ResultWorkspaceLayoutTest {
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
          fixture.assertTextWrapsWithoutClipping(longTradeoff)
          fixture.scrollBy(100_000f, "result-detail")
          fixture.render()
          assertTrue(fixture.verticalScrollValue("result-detail") > 0f)
          fixture.revealText("Prepare fix", "result-detail")
          fixture.clickText("Prepare fix")
          assertEquals(1, fixes)
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
              SecurityWorkspaceActions({ fixes++ }, {}, FindingActions({}, { _, _ -> })))
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
          assertFalse(fixture.hasText("Preconditions / unknowns"))
          assertFalse(fixture.hasText("Safe verification idea"))
          fixture.revealText("Evidence and safe verification", "result-detail")
          fixture.clickText("Evidence and safe verification")
          fixture.render("security-detail-disclosure-800-400-150")
          assertTrue(fixture.hasText("Preconditions / unknowns"))
          assertTrue(fixture.hasText("Safe verification idea"))
          fixture.assertTextWrapsWithoutClipping(longPreconditions)
          fixture.revealText("Prepare fix", "result-detail")
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
              SecurityWorkspaceActions({ fixes++ }, {}, FindingActions({}, { _, _ -> })))
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
          fixture.scrollBy(100_000f, "result-empty")
          fixture.render()
          assertTrue(fixture.verticalScrollValue("result-empty") > 0f)
          assertTrue(fixture.hasText(refreshError))
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
    ComposeVisualFixture(800, 650, 1.5f) {
          BugsWorkspacePane(
              BugsWorkspacePaneState(page.semantic, null, false, page),
              BugsWorkspaceActions(FindingActions({}, { _, _ -> }), {}, {}))
        }
        .use { fixture ->
          fixture.render()
          fixture.clickDescription("Inspect $title")
          fixture.render("rounded-results-full-content-800-150")
          fixture.assertTextWrapsWithoutClipping(title)
          assertTrue(fixture.hasText(path))
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
