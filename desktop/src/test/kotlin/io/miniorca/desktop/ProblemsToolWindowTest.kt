package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class ProblemsToolWindowTest {
  private val highOpen =
      UnifiedFinding(
          id = "high-open",
          source = "vet",
          confidence = "tool_reported",
          severity = "high",
          title = "Unchecked error",
          message = "Handle the returned error.",
          status = "open",
          freshness = "fresh",
          location = FindingLocation("internal/main.go", startLine = 12, symbol = "Run"),
          taskSpec =
              BugTaskSpec(
                  schemaVersion = "1",
                  targetPath = "internal/main.go",
                  targetSymbol = "Run",
                  targetSignature = "func Run() error",
                  acceptanceCriteria = listOf("Return the error.")),
      )

  @Test
  fun sharedPresentationKeepsBottomAndDetailedBugsOrderingAndFiltersAligned() {
    val mediumDismissed = highOpen.copy(id = "medium", severity = "medium", status = "dismissed")
    val lowOpen = highOpen.copy(id = "low", severity = "low")

    val presentation =
        findingsPresentation(
            listOf(lowOpen, mediumDismissed, highOpen), BugsFilters(source = "vet"))

    assertEquals(
        listOf(FindingPriority.High, FindingPriority.Medium, FindingPriority.Low),
        presentation.priorityGroups.map { it.priority })
    assertEquals(listOf(highOpen), presentation.priorityGroups.first().findings)
    assertEquals(listOf("Source: vet"), presentation.activeFilters)
    assertEquals(
        groupFindingsByPriority(
            filterFindings(listOf(lowOpen, mediumDismissed, highOpen), presentation.filters)),
        presentation.priorityGroups)
  }

  @Test
  fun collapsedSummaryUsesTotalAndHighestActionableSeverityWithoutColor() {
    assertEquals(
        ProblemsCollapsedSummary(total = 2, highestActionablePriority = FindingPriority.High),
        problemsCollapsedSummary(
            listOf(highOpen, highOpen.copy(id = "dismissed", status = "dismissed"))))
    assertEquals(
        "2 problems · highest actionable severity: High",
        problemsCollapsedSummary(listOf(highOpen, highOpen.copy(id = "low", severity = "low")))
            .text)
    assertEquals("No problems", problemsCollapsedSummary(emptyList()).text)
    assertEquals("Loading problems…", problemsCollapsedSummary(emptyList(), loading = true).text)
  }

  @Test
  fun selectingAProblemOnlyNavigatesAndNeverPreparesAFix() {
    var selected: UnifiedFinding? = null
    var prepared = false
    val actions =
        FindingActions(
            openFinding = { selected = it },
            prepareFinding = { prepared = true },
            triageFinding = { _, _ -> },
        )
    val index =
        ProjectIndex(
            "project",
            "revision",
            files = listOf(IndexedFile("internal/main.go", "hash", "Go", false)))

    actions.select(highOpen)

    assertEquals(highOpen, selected)
    assertFalse(prepared)
    assertEquals(
        EditorNavigationTarget("internal/main.go", "Run", 12),
        findingNavigationTarget(highOpen, index))
    assertTrue(compactProblemRowDescription(highOpen).contains("Open source navigates only."))
  }

  @Test
  fun collapsedTabTextAndSelectionRemainKeyboardDiscoverable() {
    val summary = problemsCollapsedSummary(listOf(highOpen)).text

    assertEquals("Problems", bottomToolWindowLabel(BottomToolWindow.Problems))
    assertEquals(
        "Problems tool window tab, $summary, selected",
        bottomToolWindowTabDescription(
            BottomToolWindow.Problems, selected = true, summary = summary))
    assertTrue(DesktopLayoutState().bottomToolWindowVisible)
    assertTrue(DesktopLayoutState().bottomCollapsed)
  }
}
