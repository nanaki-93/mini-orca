package io.miniorca.desktop

import androidx.compose.ui.unit.dp
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

class AnalysisWorkspaceStateTest {
  @Test
  fun workspaceWidthRulesUseSharedGuttersAndStackOnlyWhenTheRunWouldBeTooNarrow() {
    assertEquals(16.dp, workspacePageHorizontalGutter(899.dp))
    assertEquals(24.dp, workspacePageHorizontalGutter(900.dp))
    assertEquals(360.dp, analysisRunControlLayout(1_440.dp).controlsWidth)
    assertFalse(analysisRunControlLayout(720.dp).stacked)
    assertTrue(analysisRunControlLayout(719.dp).stacked)
    assertEquals(320.dp, analysisRunControlLayout(1_000.dp, fontScale = 1.3f).controlsWidth)
    assertFalse(analysisRunControlLayout(1_000.dp, fontScale = 1.3f).stacked)
  }

  @Test
  fun metricColumnsReflowBeforeNarrowLabelsCanCrowd() {
    assertEquals(6, analysisMetricColumnCount(780.dp))
    assertEquals(3, analysisMetricColumnCount(779.dp))
    assertEquals(2, analysisMetricColumnCount(519.dp))
  }

  @Test
  fun progressCountsFailuresAsProcessedAndKeepsEmptyQueuesAtZero() {
    val empty = analyzeAllPresentation(null, null).run
    assertEquals(0f, analysisRunProgress(empty))
    val run =
        analyzeAllPresentation(
                job(
                    "running",
                    files =
                        listOf(
                            file("one.go", "completed"),
                            file("two.go", "failed"),
                            file("three.go", "running"),
                            file("four.go", "pending"))),
                null)
            .run
    assertEquals(0.5f, analysisRunProgress(run))
    assertEquals(1f, analysisRunProgress(run.copy(completed = 8)))
  }

  @Test
  fun pollingLifecycleStopsForPauseCancelAndNoContent() {
    val controller = AnalyzeAllPollingController()
    controller.activate("revision")

    assertNull(controller.receive("revision", null))
    assertFalse(controller.shouldPoll("revision"))

    controller.receive("revision", job("running"))
    assertTrue(controller.shouldPoll("revision"))

    controller.receive("revision", job("paused"))
    assertFalse(controller.shouldPoll("revision"))

    controller.receive("revision", job("running"))
    assertTrue(controller.shouldPoll("revision"))

    controller.receive("revision", job("canceled"))
    assertFalse(controller.shouldPoll("revision"))
  }

  @Test
  fun resumeAndCancelRemainBoundToTheActiveRevision() {
    val controller = AnalyzeAllPollingController()
    controller.activate("old")
    controller.receive("old", job("paused", "old"))
    controller.activate("new")

    assertNull(controller.receive("old", job("running", "old")))
    assertFalse(controller.shouldPoll("old"))
    assertFalse(controller.shouldPoll("new"))

    controller.receive("new", job("running", "new"))
    assertTrue(controller.shouldPoll("new"))
    controller.dispose()
    assertFalse(controller.shouldPoll("new"))
  }

  @Test
  fun runOptionsKeepFileAndRetryLimitsWithinDaemonBounds() {
    assertEquals(
        AnalyzeAllRunOptions(maxFiles = 1, maxRetries = 0),
        AnalyzeAllRunOptions(maxFiles = -1, maxRetries = -1).bounded())
    assertEquals(
        AnalyzeAllRunOptions(maxFiles = 500, maxRetries = 3),
        AnalyzeAllRunOptions(maxFiles = 501, maxRetries = 4).bounded())
    assertTrue(AnalyzeAllRunOptions(confirmRemoteProvider = true).bounded().confirmRemoteProvider)
  }

  @Test
  fun analysisPresentationSeparatesCoverageRunCountsAndFailureRows() {
    val presentation =
        analyzeAllPresentation(
            AnalyzeAllJob(
                projectRevision = "revision",
                status = "running",
                maxFiles = 20,
                maxRetries = 2,
                files =
                    listOf(
                        file("complete.go", "completed"),
                        file("failed.go", "failed", attempts = 2, error = "sanitized failure"),
                        file("active.go", "running"),
                        file("queued.go", "pending"),
                        file("unknown.go", "unexpected", error = "sanitized unknown failure"),
                        file("missing-message.go", "failed", attempts = 3),
                    ),
            ),
            AnalysisCoverage(
                total = 42, fresh = 8, stale = 5, missing = 20, running = 6, failed = 3),
        )

    assertEquals(42, presentation.coverage.total)
    assertEquals(8, presentation.coverage.fresh)
    assertEquals(6, presentation.coverage.running)
    assertEquals(3, presentation.coverage.failed)
    assertEquals(20, presentation.run.maxFiles)
    assertEquals(2, presentation.run.maxRetries)
    assertEquals(6, presentation.run.candidates)
    assertEquals(1, presentation.run.completed)
    assertEquals(3, presentation.run.failed)
    assertEquals(1, presentation.run.running)
    assertEquals(1, presentation.run.remaining)
    assertEquals(
        listOf("failed.go", "unknown.go", "missing-message.go"),
        presentation.failures.map { it.path },
    )
    assertEquals("sanitized failure", presentation.failures[0].error)
    assertEquals(2, presentation.failures[0].attempts)
    assertEquals("Analysis failed", presentation.failures[2].error)
    assertTrue(presentation.statusDetail.contains("8 fresh"))
    assertTrue(presentation.run.statusDetail.contains("Run counts and analysis errors"))
    assertFalse(presentation.run.statusDetail.contains("revision"))
  }

  @Test
  fun analysisPresentationUsesStableLifecycleTextForEveryJobState() {
    val cases =
        listOf(
            null to Triple("Not started", "Start", "never start one automatically"),
            "running" to Triple("Running", "Pause or cancel", "processing candidates"),
            "paused" to Triple("Paused", "Resume or cancel", "Resume explicitly"),
            "completed" to Triple("Completed", "Start a new run", "refresh coverage"),
            "canceled" to Triple("Canceled", "Start a new run", "recorded analysis errors"),
            "stale" to Triple("Stale", "Start a new run", "cannot resume"),
            "failed" to Triple("Failed", "Retry", "Inspect the analysis errors"),
        )

    cases.forEach { (status, expected) ->
      val presentation = analyzeAllPresentation(status?.let(::job), null)

      assertEquals(expected.first, presentation.run.statusLabel)
      assertEquals(expected.second, presentation.run.controls)
      assertTrue(presentation.run.statusDetail.contains(expected.third))
    }
  }

  @Test
  fun errorBearingUnknownStatesAreFailuresWhileSuccessfulAndActiveFilesAreNot() {
    val presentation =
        analyzeAllPresentation(
            job(
                "running",
                files =
                    listOf(
                        file("completed.go", "completed"),
                        file("running.go", "running"),
                        file("pending.go", "pending"),
                        file("unknown.go", "unknown", error = "sanitized failure"),
                    ),
            ),
            null,
        )

    assertEquals(1, presentation.run.failed)
    assertEquals(listOf("unknown.go"), presentation.failures.map { it.path })
    assertEquals("sanitized failure", presentation.failures.single().error)
  }

  @Test
  fun emptyFailureListUsesAnExplicitNoErrorsMessage() {
    val presentation =
        analyzeAllPresentation(job("completed", files = listOf(file("main.go", "completed"))), null)

    assertTrue(presentation.failures.isEmpty())
    assertEquals("No analysis errors in this run.", presentation.noErrorsMessage)
  }

  @Test
  fun missingAnalysisJobStatesThatImportAndReindexNeverStartIt() {
    val presentation = analyzeAllPresentation(null, null)

    assertEquals("Not started", presentation.statusLabel)
    assertTrue(presentation.statusDetail.contains("never start one automatically"))
    assertEquals("Start", presentation.controls)
  }

  private fun job(
      status: String,
      revision: String = "revision",
      files: List<AnalyzeAllFileJob> = emptyList()
  ) =
      AnalyzeAllJob(
          projectId = "project",
          projectRevision = revision,
          status = status,
          files = files,
      )

  private fun file(path: String, status: String, attempts: Int = 0, error: String = "") =
      AnalyzeAllFileJob(path = path, status = status, attempts = attempts, error = error)
}
