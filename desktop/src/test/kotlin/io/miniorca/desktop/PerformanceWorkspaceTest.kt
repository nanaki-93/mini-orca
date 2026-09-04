package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

class PerformanceWorkspaceTest {
  @Test
  fun coveragePresentationLabelsUnknownPartialAndBudgetLimitedReviews() {
    assertEquals("Unknown", performanceCoveragePresentation(null, null).stateLabel)
    assertEquals(
        "Partial",
        performanceCoveragePresentation(PerformanceJob(status = "running"), null).stateLabel,
    )

    val presentation =
        performanceCoveragePresentation(
            PerformanceJob(
                status = "canceled",
                elapsed = 30_000_000_000,
                runBudget = 30_000_000_000,
                files =
                    listOf(
                        PerformanceJobFile(status = "completed"),
                        PerformanceJobFile(status = "skipped"),
                        PerformanceJobFile(status = "failed"),
                        PerformanceJobFile(status = "pending"),
                    ),
            ),
            null,
        )

    assertEquals("Partial · canceled review retains completed files", presentation.stateLabel)
    assertEquals(
        listOf(
            "Reviewed" to "1 completed · 0 cached",
            "Skipped" to "1",
            "Failed" to "1",
            "Remaining" to "1 pending · 0 running",
            "Budget" to "30s of 30s · budget-limited",
        ),
        presentation.rows,
    )
  }

  @Test
  fun headerActionsKeepPreviewAndJobLifecycleGuardsIntact() {
    val localModel = ScopedModel(scope = ModelScope.Analyze.wireValue)
    val remoteModel = localModel.copy(remoteProvider = true)

    assertEquals(
        listOf(
            PerformanceToolbarActionPresentation(PerformanceToolbarAction.Preview, true),
            PerformanceToolbarActionPresentation(PerformanceToolbarAction.Start, false),
        ),
        performanceToolbarActions(null, false, localModel, remoteProviderConfirmed = false),
    )
    assertEquals(
        listOf(
            PerformanceToolbarActionPresentation(PerformanceToolbarAction.Preview, true),
            PerformanceToolbarActionPresentation(PerformanceToolbarAction.Start, true),
        ),
        performanceToolbarActions(null, true, localModel, remoteProviderConfirmed = false),
    )
    assertEquals(
        listOf(
            PerformanceToolbarActionPresentation(PerformanceToolbarAction.Preview, true),
            PerformanceToolbarActionPresentation(PerformanceToolbarAction.Start, false),
        ),
        performanceToolbarActions(null, true, remoteModel, remoteProviderConfirmed = false),
    )
    assertEquals(
        listOf(
            PerformanceToolbarActionPresentation(PerformanceToolbarAction.Pause, true),
            PerformanceToolbarActionPresentation(PerformanceToolbarAction.Cancel, true),
        ),
        performanceToolbarActions(
            PerformanceJob(status = "running"), true, remoteModel, remoteProviderConfirmed = false),
    )
    assertEquals(
        listOf(
            PerformanceToolbarActionPresentation(PerformanceToolbarAction.Resume, false),
            PerformanceToolbarActionPresentation(PerformanceToolbarAction.Cancel, true),
        ),
        performanceToolbarActions(
            PerformanceJob(status = "paused"), true, remoteModel, remoteProviderConfirmed = false),
    )
  }

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
