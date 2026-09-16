package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

class FindingsPresentationTest {
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
  fun fixPreparationUsesTheSelectedFindingAndItsExactTarget() {
    var prepared: UnifiedFinding? = null
    val actions =
        FindingActions(
            prepareFinding = { prepared = it },
            triageFinding = { _, _ -> },
        )
    val index =
        ProjectIndex(
            "project",
            "revision",
            files = listOf(IndexedFile("internal/main.go", "hash", "Go", false)))

    actions.prepareFix(highOpen)

    assertEquals(highOpen, prepared)
    assertEquals(
        EditorNavigationTarget("internal/main.go", "Run", 12),
        findingNavigationTarget(highOpen, index))
  }

  @Test
  fun routineProvenanceAndOpenFreshLabelsStayOutOfRows() {
    val current = semanticResultRow(highOpen)
    val stale = semanticResultRow(highOpen.copy(freshness = "stale", status = "partial"))

    assertEquals("", current.source)
    assertEquals("", current.state)
    assertEquals("Handle the returned error.", current.summary)
    assertEquals("Partial · Stale", stale.state)
    assertTrue(current.location.contains("internal/main.go:12"))
  }
}
