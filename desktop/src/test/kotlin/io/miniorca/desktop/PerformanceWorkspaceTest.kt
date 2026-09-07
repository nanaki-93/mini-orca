package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
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
  fun coveragePresentationTreatsStaleReportCoverageAsPartial() {
    val job =
        PerformanceJob(
            projectId = "project",
            projectRevision = "revision",
            queueId = "performance:queue",
            status = "completed",
        )
    val presentation =
        performanceCoveragePresentation(
            job,
            PerformanceReport(
                projectId = job.projectId,
                projectRevision = job.projectRevision,
                queueId = job.queueId,
                status = "stale",
                counts = mapOf("stale" to 1),
            ),
        )

    assertEquals("Partial · source or policy changed", presentation.stateLabel)
    assertEquals(
        listOf(
            "Reviewed" to "0 completed · 0 cached",
            "Stale" to "1",
            "Skipped" to "0",
            "Failed" to "0",
            "Remaining" to "0 pending · 0 running",
        ),
        presentation.rows,
    )
    assertEquals("Stale · source or policy changed", performanceStatusLabel(job, staleReport(job)))
    assertEquals(
        "Completed · source-based queue",
        performanceStatusLabel(job, staleReport(job).copy(queueId = "performance:old")),
    )
  }

  @Test
  fun mismatchedReportCannotRenderFindingsOrPaths() {
    val job =
        PerformanceJob(
            projectId = "project",
            projectRevision = "revision",
            queueId = "performance:current",
        )
    val finding = PerformanceFinding(id = "finding-1", title = "Current finding")
    val report =
        PerformanceReport(
            projectId = job.projectId,
            projectRevision = job.projectRevision,
            queueId = job.queueId,
            paths = mapOf(finding.id to "internal/current.go"),
            findings = listOf(finding),
        )

    val matching = performanceReviewPresentation(job, report)
    assertEquals(listOf(finding), matching.findings("", "", ""))
    assertEquals("internal/current.go", matching.pathFor(finding))
    assertFalse(matching.isStale)

    val stale = performanceReviewPresentation(job, staleReport(job))
    assertTrue(stale.isStale)

    val mismatched = performanceReviewPresentation(job, report.copy(queueId = "performance:old"))
    assertFalse(mismatched.hasReport)
    assertFalse(mismatched.isStale)
    assertEquals(emptyList(), mismatched.findings("", "", ""))
    assertEquals("", mismatched.pathFor(finding))

    val missingIdentity = performanceReviewPresentation(job.copy(queueId = ""), report)
    assertFalse(missingIdentity.hasReport)
    assertEquals(emptyList(), missingIdentity.findings("", "", ""))
    assertEquals("", missingIdentity.pathFor(finding))
  }

  private fun staleReport(job: PerformanceJob) =
      PerformanceReport(
          projectId = job.projectId,
          projectRevision = job.projectRevision,
          queueId = job.queueId,
          status = "stale",
          counts = mapOf("stale" to 1),
      )

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
