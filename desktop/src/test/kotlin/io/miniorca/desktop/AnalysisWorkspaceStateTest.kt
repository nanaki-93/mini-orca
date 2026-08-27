package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

class AnalysisWorkspaceStateTest {
    @Test fun pollingLifecycleStopsForPauseCancelAndNoContent() {
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

    @Test fun resumeAndCancelRemainBoundToTheActiveRevision() {
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

    @Test fun runOptionsKeepFileAndRetryLimitsWithinDaemonBounds() {
        assertEquals(AnalyzeAllRunOptions(maxFiles = 1, maxRetries = 0), AnalyzeAllRunOptions(maxFiles = -1, maxRetries = -1).bounded())
        assertEquals(AnalyzeAllRunOptions(maxFiles = 500, maxRetries = 3), AnalyzeAllRunOptions(maxFiles = 501, maxRetries = 4).bounded())
        assertTrue(AnalyzeAllRunOptions(confirmRemoteProvider = true).bounded().confirmRemoteProvider)
    }

    @Test fun analysisPresentationKeepsPartialResultsAndExplicitTerminalActions() {
        val running = analyzeAllPresentation(AnalyzeAllJob(projectRevision = "revision", status = "running", files = listOf(AnalyzeAllFileJob("main.go", "completed"), AnalyzeAllFileJob("other.go", "running"))), AnalysisCoverage(fresh = 1, running = 1))
        val failed = analyzeAllPresentation(AnalyzeAllJob(projectRevision = "revision", status = "failed", files = listOf(AnalyzeAllFileJob("main.go", "completed"))), AnalysisCoverage(fresh = 1, failed = 1))

        assertEquals("Running", running.statusLabel)
        assertTrue(running.statusDetail.contains("Completed results remain available"))
        assertTrue(running.statusDetail.contains("1 fresh"))
        assertEquals("Retry", failed.controls)
        assertTrue(failed.statusDetail.contains("Completed results remain available"))
    }

    @Test fun missingAnalysisJobStatesThatImportAndReindexNeverStartIt() {
        val presentation = analyzeAllPresentation(null, null)

        assertEquals("Not started", presentation.statusLabel)
        assertTrue(presentation.statusDetail.contains("never start one automatically"))
        assertEquals("Start", presentation.controls)
    }

    private fun job(status: String, revision: String = "revision") = AnalyzeAllJob(
        projectId = "project",
        projectRevision = revision,
        status = status,
    )
}
