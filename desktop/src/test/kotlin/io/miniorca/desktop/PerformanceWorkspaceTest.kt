package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

class PerformanceWorkspaceTest {
  @Test
  fun compactStatusLabelsPreserveBudgetStalenessAndSourceQualification() {
    assertEquals("No review yet", performanceStatusLabel(null))
    assertEquals(
        "Running · 12s budget used",
        performanceStatusLabel(PerformanceJob(status = "running", elapsed = 12_000_000_000)),
    )
    assertEquals(
        "Paused · resume explicitly", performanceStatusLabel(PerformanceJob(status = "paused")))
    assertTrue(
        performanceStatusLabel(PerformanceJob(status = "canceled"))
            .contains("reviews remain available"))
    assertTrue(
        performanceStatusLabel(PerformanceJob(status = "stale"))
            .contains("source or policy changed"))
    assertTrue(
        performanceStatusLabel(PerformanceJob(status = "completed")).contains("source-based"))
  }
}
