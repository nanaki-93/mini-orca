package io.miniorca.desktop

import kotlin.coroutines.CoroutineContext
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue
import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json

class DesktopSecurityWorkflowTest {
  @Test
  fun failedDeterministicScanRetainsBothExistingEvidenceSources() {
    Harness().use { harness ->
      harness.dispatch(DesktopEvent.SecurityReportLoaded(report("deterministic")))
      harness.dispatch(DesktopEvent.SecurityReportLoaded(report("ai")))
      harness.response =
          TransportResponse(500, Json.encodeToString(ApiError(message = "scan failed")))
      harness.workflow.scanSecurity()
      harness.completeRequest()
      assertEquals(report("deterministic"), harness.state.security.sourceReport)
      assertEquals(report("ai"), harness.state.security.aiReport)
      assertEquals(
          SecuritySectionOperationStatus.Failed, harness.state.security.sourceOperation.status)
    }
  }

  @Test
  fun scanUsesTheDeterministicEndpointWithoutConsumingReviewConsent() {
    Harness().use { harness ->
      harness.confirmed = true
      harness.workflow.scanSecurity()
      harness.completeRequest()
      assertEquals(listOf("/api/projects/current/files/security-scan"), harness.paths)
      assertEquals(report("deterministic"), harness.state.security.sourceReport)
      assertNull(harness.state.security.aiReport)
      assertTrue(harness.confirmed)
    }
  }

  @Test
  fun ineligibleFilesFailWithoutStartingRequests() {
    Harness().use { harness ->
      harness.dispatch(DesktopEvent.FileLoaded(file().copy(language = "Markdown"), emptyList()))
      harness.workflow.scanSecurity()
      assertEquals(
          SecuritySectionOperationStatus.Failed, harness.state.security.sourceOperation.status)
      harness.completeRequest()
      assertTrue(harness.paths.isEmpty())
    }
  }

  @Test
  fun newerRequestReplacesPendingPublicationForTheSameFile() {
    Harness().use { harness ->
      harness.workflow.scanSecurity()
      harness.main.runPending()
      harness.io.runPending()
      harness.workflow.scanSecurity()
      harness.response =
          TransportResponse(
              200, Json.encodeToString(report("deterministic").copy(reason = "newer")))
      harness.completeRequest()
      assertEquals("newer", harness.state.security.sourceReport?.reason)
      assertEquals(1, harness.events.filterIsInstance<DesktopEvent.SecurityReportLoaded>().size)
    }
  }

  @Test
  fun responseAndFailureGuardsReadTheCurrentAuthoritativeFileIdentity() {
    for (failure in listOf(false, true)) {
      for (change in listOf("project", "revision", "path", "hash")) {
        Harness().use { harness ->
          if (failure)
              harness.response =
                  TransportResponse(500, Json.encodeToString(ApiError(message = "old failure")))
          harness.workflow.scanSecurity()
          harness.main.runPending()
          harness.io.runPending()
          // Bypass lifecycle cancellation to exercise the live-state response guard itself.
          when (change) {
            "project" ->
                harness.controller.dispatch(
                    DesktopEvent.ProjectLoaded(project("other"), ProjectIndex("other", "revision")))
            "revision" ->
                harness.controller.dispatch(
                    DesktopEvent.IndexRefreshed(ProjectIndex("project", "new")))
            "path" ->
                harness.controller.dispatch(
                    DesktopEvent.FileLoaded(file().copy(path = "other.go"), emptyList()))
            "hash" ->
                harness.controller.dispatch(
                    DesktopEvent.FileLoaded(file().copy(contentHash = "new"), emptyList()))
          }
          harness.events.clear()
          harness.main.runPending()
          assertTrue(harness.events.isEmpty(), "$change, failure=$failure")
          assertNull(harness.state.security.sourceReport)
        }
      }
    }
  }

  @Test
  fun mismatchedReportCannotReplaceRetainedEvidence() {
    val original = report("deterministic")
    for (mismatch in
        listOf(
            original.copy(projectId = "other"),
            original.copy(projectRevision = "new"),
            original.copy(path = "other.go"),
            original.copy(contentHash = "new"),
            original.copy(source = "ai"))) {
      Harness().use { harness ->
        harness.dispatch(DesktopEvent.SecurityReportLoaded(original))
        harness.response = TransportResponse(200, Json.encodeToString(mismatch))
        harness.workflow.scanSecurity()
        harness.completeRequest()
        assertEquals(original, harness.state.security.sourceReport)
        assertEquals(
            SecuritySectionOperationStatus.Failed, harness.state.security.sourceOperation.status)
        assertTrue(harness.state.security.sourceOperation.message.contains("no longer matches"))
        assertNull(harness.state.security.aiReport)
      }
    }
  }

  @Test
  fun lifecycleEventsCancelQueuedWorkAndClearConfirmation() {
    val events =
        listOf(
            DesktopEvent.ProjectLoaded(project("other"), ProjectIndex("other", "revision")),
            DesktopEvent.FileLoaded(file().copy(path = "other.go"), emptyList()),
            DesktopEvent.IndexRefreshed(ProjectIndex("project", "new")))
    for (event in events) {
      Harness().use { harness ->
        harness.workflow.scanSecurity()
        harness.main.runPending()
        harness.confirmed = true
        harness.dispatch(event)
        harness.completeRequest()
        assertTrue(harness.paths.isEmpty())
        assertTrue(harness.state.security.action.isBlank())
        assertFalse(harness.confirmed)
      }
    }
  }

  @Test
  fun unchangedIndexDoesNotCancelTheCurrentRequest() {
    Harness().use { harness ->
      harness.workflow.scanSecurity()
      harness.main.runPending()
      harness.dispatch(DesktopEvent.IndexRefreshed(ProjectIndex("project", "revision")))
      harness.completeRequest()
      assertEquals(report("deterministic"), harness.state.security.sourceReport)
    }
  }

  @Test
  fun cancelRejectsPendingScanPublicationAndRetainsPriorReports() {
    Harness().use { harness ->
      harness.dispatch(DesktopEvent.SecurityReportLoaded(report("deterministic")))
      harness.response =
          TransportResponse(200, Json.encodeToString(report("deterministic").copy(reason = "late")))
      harness.workflow.scanSecurity()
      harness.main.runPending()
      harness.io.runPending()
      harness.workflow.cancel()
      harness.main.runPending()
      assertEquals(report("deterministic"), harness.state.security.sourceReport)
      assertEquals(
          SecuritySectionOperationStatus.Canceled, harness.state.security.sourceOperation.status)
    }
  }

  private inner class Harness : AutoCloseable {
    val main = QueuedDispatcher()
    val io = QueuedDispatcher()
    private val scope = CoroutineScope(SupervisorJob() + main)
    val controller = DesktopWorkflowController()
    val state
      get() = controller.state

    val paths = mutableListOf<String>()
    val bodies = mutableListOf<String>()
    val events = mutableListOf<DesktopEvent>()
    var confirmed = false
    var response = TransportResponse(200, Json.encodeToString(report("deterministic")))
    val workflow =
        DesktopSecurityWorkflow(
            ApiClient(
                transport =
                    DaemonTransport { method, path, body ->
                      assertEquals("POST", method)
                      paths.add(path)
                      bodies.add(body.orEmpty())
                      response
                    }),
            scope,
            io,
            { controller.state },
            ::dispatch,
            { confirmed = false })

    init {
      controller.dispatch(
          DesktopEvent.ProjectLoaded(project(), ProjectIndex("project", "revision")))
      controller.dispatch(DesktopEvent.FileLoaded(file(), emptyList()))
    }

    fun dispatch(event: DesktopEvent) {
      workflow.beforeEvent(event)
      controller.dispatch(event)
      events.add(event)
    }

    fun completeRequest() {
      main.runPending()
      io.runPending()
      main.runPending()
    }

    override fun close() {
      workflow.cancel()
      scope.cancel()
      completeRequest()
    }
  }

  private class QueuedDispatcher : CoroutineDispatcher() {
    private val pending = ArrayDeque<Runnable>()

    override fun dispatch(context: CoroutineContext, block: Runnable) {
      pending.addLast(block)
    }

    fun runPending() {
      while (pending.isNotEmpty()) pending.removeFirst().run()
    }
  }

  private fun report(source: String) =
      SecurityFileReport(
          projectId = "project",
          projectRevision = "revision",
          path = "main.go",
          contentHash = "base",
          source = source,
          status = "completed_empty")

  private fun project(id: String = "project") =
      ProjectAnalysis(
          id,
          "revision",
          id,
          "/tmp/$id",
          "go",
          fileCount = 1,
          sourceFileCount = 1,
          totalLines = 1,
          summary = "",
          aiStatus = "missing",
          analyzedAt = "")

  private fun file() =
      ProjectFileInfo(
          "main.go",
          "base",
          "main.go",
          language = "Go",
          sizeBytes = 1,
          lineCount = 1,
          modifiedAt = "",
          binary = false,
          content = "package main")
}
