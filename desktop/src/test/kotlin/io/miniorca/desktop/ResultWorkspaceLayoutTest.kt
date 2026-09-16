package io.miniorca.desktop

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material.Text
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
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
  fun selectingAnotherFindingStartsItsEvidenceAtTheTop() {
    val rows = (1..2).map { row(it) }
    var selected by mutableStateOf<String?>(rows.first().key)
    ComposeVisualFixture(1000, 600) {
          ResultListDetail(
              rows, selected, { selected = it }, "No findings", Modifier.fillMaxSize()) {
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
    var selected by mutableStateOf<String?>(null)
    ComposeVisualFixture(800, 650) {
          ResultListDetail(
              rows, selected, { selected = it }, "No findings", Modifier.fillMaxSize()) {
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
  fun longRefreshErrorsLeaveRetainedFindingsAndDisabledFixReachable() {
    val original = performancePageFixture()
    val stale =
        original.copy(
            run = original.run!!.copy(status = "stale"),
            section =
                original.section.copy(
                    error = "The result refresh failed for a long project path. ".repeat(12)))
    var fixes = 0
    ComposeVisualFixture(800, 650, 1.5f) {
          PerformanceWorkspacePane(
              PerformanceWorkspacePaneState(stale, resultIndexFixture()),
              PerformanceWorkspaceActions({ _, _ -> fixes++ }, {}, FindingActions({}, { _, _ -> })))
        }
        .use { fixture ->
          fixture.render("rounded-results-long-error-800-150")
          assertTrue(fixture.taggedBounds("result-list").height > 250f)
          fixture.clickDescription("Inspect Avoid repeated allocation")
          fixture.render()
          fixture.revealText("Prepare fix", "result-detail")
          assertTrue(fixture.isDisabled("Prepare fix"))
          assertEquals(0, fixes)
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
}
