package io.miniorca.desktop

import kotlin.coroutines.CoroutineContext
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNotNull
import kotlin.test.assertNull
import kotlin.test.assertTrue
import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json

class DesktopAnalysisWorkflowTest {
  @Test
  fun staleFailedSelectionTravelsThroughPreviewStartAndResume() {
    Harness().use { h ->
      h.run = h.run.copy(plan = h.run.plan.copy(retryStaleFailed = true))
      h.workflow.preview(retryStaleFailed = true)
      h.drain()
      val preview = Json.decodeFromString<AnalysisPreviewRequest>(h.bodies.single())
      assertTrue(preview.retryStaleFailed)
      assertFalse(preview.refresh)
      assertTrue(h.state.analysisRun.admission!!.preview.retryStaleFailed)
      h.confirm()
      h.workflow.admit()
      h.drain()
      val start = Json.decodeFromString<AnalysisRunStartRequest>(h.bodies.last())
      assertTrue(start.retryStaleFailed)
      h.workflow.preview(resume = true)
      h.drain()
      val resume = Json.decodeFromString<AnalysisPreviewRequest>(h.bodies.last())
      assertTrue(resume.retryStaleFailed)
      assertEquals(h.run.identity, resume.resumeRun)
    }
  }

  @Test
  fun mismatchedRetryPreviewIsRejectedBeforeAdmission() {
    Harness().use { h ->
      h.wrongRetryPreview = true
      h.workflow.preview(retryStaleFailed = true)
      h.drain()
      assertNull(h.state.analysisRun.admission)
      assertTrue(h.state.analysisRun.error!!.contains("preview no longer matches"))
      assertFalse(h.calls.any { it.first == "POST" && it.second.endsWith("/run") })
    }
  }

  @Test
  fun admissionRequiresEveryDestinationAndSecurityIntentAndConsumesThemBeforeDispatch() {
    Harness().use { h ->
      h.workflow.preview()
      h.drain()
      assertEquals(listOf("POST" to "/api/projects/current/analysis/preview"), h.calls)
      h.workflow.admit()
      h.drain()
      assertEquals(1, h.calls.size)
      h.workflow.confirmProvider("bug-provider", true)
      h.workflow.admit()
      h.drain()
      assertEquals(1, h.calls.size)
      h.workflow.confirmProvider("analyze-provider", true)
      h.workflow.confirmSecurity(true)
      assertTrue(h.state.analysisRun.admission!!.isConfirmed())
      h.workflow.admit()
      assertNull(h.state.analysisRun.admission)
      h.drain()
      assertEquals(1, h.calls.count { it.first == "POST" && it.second.endsWith("/run") })
      assertEquals(h.run, h.state.analysisRun.run)
      val start =
          Json.decodeFromString<AnalysisRunStartRequest>(
              h.bodies.first { it.contains("confirmations") })
      assertEquals(
          setOf("bug-provider", "analyze-provider"), start.confirmations.providerIds.toSet())
      assertTrue(start.confirmations.securityReview)
      assertEquals(3, h.state.analysisRun.sections.size)
      h.workflow.admit()
      h.drain()
      assertEquals(1, h.calls.count { it.second.endsWith("/run") && it.first == "POST" })
    }
  }

  @Test
  fun uncertainAdmissionReadsDurableStateWithoutRetryingOrReusingConsent() {
    Harness().use { h ->
      h.workflow.preview()
      h.drain()
      h.confirm()
      h.failure = "/analysis/run"
      h.workflow.admit()
      h.drain()
      assertNull(h.state.analysisRun.admission)
      assertEquals(h.run, h.state.analysisRun.run)
      assertTrue(h.state.analysisRun.error!!.contains("unavailable"))
      h.workflow.admit()
      h.drain()
      assertEquals(1, h.calls.count { it == "POST" to "/api/projects/current/analysis/run" })
      assertTrue(h.calls.any { it.first == "GET" && it.second.contains("/analysis/run?") })
    }
  }

  @Test
  fun resumeUsesFreshPreviewAndExactPriorGenerationAndDoesNotResetRetainedEvidence() {
    Harness().use { h ->
      h.workflow.refresh()
      h.drain()
      val evidence = h.state.analysisRun.sections
      h.workflow.preview(resume = true)
      h.drain()
      assertFalse(h.state.analysisRun.admission!!.isConfirmed())
      assertEquals(evidence, h.state.analysisRun.sections)
      val preview = Json.decodeFromString<AnalysisPreviewRequest>(h.bodies.last())
      assertEquals(h.run.identity, preview.resumeRun)
      h.confirm()
      h.workflow.admit()
      h.drain()
      val control = Json.decodeFromString<AnalysisRunControlRequest>(h.bodies.last())
      assertEquals("resume", control.action)
      assertEquals(analysisRunFixture().identity, control.identity)
      assertEquals("new-generation", h.state.analysisRun.run?.identity?.generation)
      assertNull(h.state.analysisRun.admission)
    }
  }

  @Test
  fun previewReplacementProviderChangeDismissalAndRevisionChangeInvalidateConsent() {
    for (change in listOf("preview", "provider", "dismiss", "revision", "project", "close")) {
      Harness().use { h ->
        h.workflow.preview()
        h.drain()
        h.confirm()
        when (change) {
          "preview" -> h.workflow.preview(refresh = true)
          "provider" -> h.workflow.providerChanged()
          "dismiss" -> h.workflow.dismissAdmission()
          "revision" -> h.dispatch(DesktopEvent.IndexRefreshed(ProjectIndex("project", "new")))
          "project" ->
              h.dispatch(
                  DesktopEvent.ProjectLoaded(
                      analysisProjectFixture("other"), ProjectIndex("other", "revision")))
          "close" -> h.workflow.detach()
        }
        h.drain()
        assertTrue(h.state.analysisRun.admission?.isConfirmed() != true, change)
        assertFalse(h.calls.any { it.first == "POST" && it.second.endsWith("/run") }, change)
      }
    }
  }

  @Test
  fun latePreviewAndActionCannotPublishAfterProjectReplacementOrDetachment() {
    for (admit in listOf(false, true)) {
      for (close in listOf(false, true)) {
        Harness().use { h ->
          h.workflow.preview()
          if (admit) {
            h.drain()
            h.confirm()
            h.workflow.admit()
          }
          h.main.runPending()
          h.io.runPending()
          if (close) h.workflow.detach()
          else
              h.dispatch(
                  DesktopEvent.ProjectLoaded(
                      analysisProjectFixture("other"), ProjectIndex("other", "revision")))
          h.drain()
          assertNull(h.state.analysisRun.admission)
          assertNull(h.state.analysisRun.run)
        }
      }
    }
  }

  @Test
  fun fileAndPageSelectionDoNotCancelOrChangeProjectAdmission() {
    Harness().use { h ->
      h.workflow.preview()
      h.drain()
      h.confirm()
      val admission = h.state.analysisRun.admission
      h.dispatch(DesktopEvent.FileLoaded(analysisFileFixture("another.go"), emptyList()))
      Workspace.entries.forEach { h.dispatch(DesktopEvent.WorkspaceSelected(it)) }
      h.drain()
      assertEquals(admission, h.state.analysisRun.admission)
      assertEquals(1, h.calls.size)
      h.workflow.admit()
      h.drain()
      assertNotNull(h.state.analysisRun.run)
    }
  }

  @Test
  fun failedSectionAndFilteredReadsRetainOtherTypedEvidenceAndNeverDispatchModels() {
    Harness().use { h ->
      h.workflow.refresh()
      h.drain()
      val security = h.state.analysisRun.sections.getValue(AnalysisResultKey("security")).results
      assertEquals("ai", security!!.security.single().source)
      assertEquals("unverified", security.security.single().findings.single().verificationState)
      val performance =
          h.state.analysisRun.sections.getValue(AnalysisResultKey("performance")).results
      h.failure = "category=security"
      h.workflow.loadResults("security")
      h.drain()
      val failed = h.state.analysisRun.sections.getValue(AnalysisResultKey("security"))
      assertEquals(security, failed.results)
      assertTrue(failed.error!!.contains("unavailable"))
      assertEquals(
          performance,
          h.state.analysisRun.sections.getValue(AnalysisResultKey("performance")).results)
      h.workflow.loadResults("security", "main.go")
      h.drain()
      assertNotNull(
          h.state.analysisRun.sections.getValue(AnalysisResultKey("security", "main.go")).error)
      assertEquals(
          security, h.state.analysisRun.sections.getValue(AnalysisResultKey("security")).results)
      assertTrue(h.calls.all { it.first == "GET" })
    }
  }

  @Test
  fun resultScopeMismatchCannotEraseValidEvidenceAndOldGenerationCannotPublish() {
    Harness().use { h ->
      h.workflow.refresh()
      h.drain()
      val original = h.state.analysisRun.sections.getValue(AnalysisResultKey("security")).results
      h.wrongResult = true
      h.workflow.loadResults("security")
      h.drain()
      assertEquals(
          original, h.state.analysisRun.sections.getValue(AnalysisResultKey("security")).results)
      assertNotNull(h.state.analysisRun.sections.getValue(AnalysisResultKey("security")).error)
      h.wrongResult = false
      h.workflow.loadResults("security")
      h.main.runPending()
      h.io.runPending()
      h.run = h.run.copy(identity = h.run.identity.copy(generation = "replacement"))
      h.workflow.refresh()
      h.drain()
      assertTrue(h.state.analysisRun.sections.values.all { it.results?.identity == h.run.identity })
    }
  }

  @Test
  fun queuedOlderControlCannotReachTheDaemonAfterNewerControl() {
    Harness().use { h ->
      h.workflow.refresh()
      h.drain()
      h.workflow.control("pause")
      h.main.runPending()
      h.workflow.control("cancel")
      h.drain()
      val controls =
          h.bodies.mapNotNull {
            runCatching { Json.decodeFromString<AnalysisRunControlRequest>(it) }.getOrNull()
          }
      assertEquals(listOf("cancel"), controls.map { it.action })
      assertEquals("canceled", h.state.analysisRun.run?.status)
    }
  }

  @Test
  fun pausingAndCancelingKeepOnePollUntilTerminalAndSectionFailuresDoNotStopIt() {
    for (status in listOf("queued", "running", "pausing", "canceling")) {
      Harness(pollMillis = 1).use { h ->
        h.run = h.run.copy(status = status)
        h.failure = "category=performance"
        h.workflow.refresh()
        h.drain()
        assertEquals(status, h.state.analysisRun.run?.status)
        h.run = h.run.copy(status = "paused")
        h.await { h.state.analysisRun.run?.status == "paused" }
        val reads = h.calls.count { it.second.contains("/analysis/run?") }
        Thread.sleep(10)
        h.drain()
        assertEquals(reads, h.calls.count { it.second.contains("/analysis/run?") })
        assertNotNull(h.state.analysisRun.sections.getValue(AnalysisResultKey("performance")).error)
      }
    }
  }

  @Test
  fun readFailureCanRecoverRetainedOverviewAndReconnectNeverAdmitsWork() {
    Harness().use { h ->
      h.failure = "/analysis/run?"
      h.workflow.refresh()
      h.drain()
      assertEquals(h.run, h.state.analysisRun.run)
      assertNotNull(h.state.analysisRun.error)
      assertNull(h.state.analysisRun.admission)
      assertTrue(h.calls.all { it.first == "GET" })
      h.failure = ""
      h.run = h.run.copy(status = "interrupted")
      h.workflow.refresh()
      h.drain()
      assertEquals("interrupted", h.state.analysisRun.run?.status)
      assertNull(h.state.analysisRun.admission)
    }
  }

  @Test
  fun acceptedTriageUpdatesUnifiedEvidenceAndRejectsAReadStartedBeforeTheWrite() {
    Harness().use { h ->
      h.workflow.refresh()
      h.drain()
      h.workflow.loadResults("bugs")
      h.main.runPending()
      h.io.runPending()
      h.dispatch(DesktopEvent.FindingStatusUpdated("bug", "dismissed"))
      h.drain()
      val section = h.state.analysisRun.sections.getValue(AnalysisResultKey("bugs"))
      assertEquals("dismissed", section.results!!.semantic.single().status)
      assertFalse(section.loading)
    }
  }

  @Test
  fun finalProgressReloadsAReportReadThatWasStillInFlightAtCompletion() {
    Harness().use { h ->
      h.run = h.run.copy(status = "running")
      h.workflow.refresh()
      h.drain()
      h.workflow.loadResults("bugs")
      h.main.runPending()
      h.io.runPending()
      h.run = h.run.copy(status = "completed", updatedAt = "finished")
      h.dispatch(DesktopEvent.AnalysisRunUpdated(h.state.analysisRun.copy(run = h.run)))
      h.drain()
      val section = h.state.analysisRun.sections.getValue(AnalysisResultKey("bugs"))
      assertEquals("completed", section.results!!.semantic.single().message)
      assertFalse(section.loading)
    }
  }

  private class Harness(pollMillis: Long = 100_000) : AutoCloseable {
    val main = AnalysisQueuedDispatcher()
    val io = AnalysisQueuedDispatcher()
    private val scope = CoroutineScope(SupervisorJob() + main)
    var state =
        DesktopState()
            .reduce(
                DesktopEvent.ProjectLoaded(
                    analysisProjectFixture(), ProjectIndex("project", "revision")))
    val coordinator = DesktopJobCoordinator(scope, pollMillis)
    val calls = mutableListOf<Pair<String, String>>()
    val bodies = mutableListOf<String>()
    var run = analysisRunFixture()
    var failure = ""
    var wrongResult = false
    var wrongRetryPreview = false
    val workflow =
        DesktopAnalysisWorkflow(
            ApiClient(
                transport =
                    DaemonTransport { method, path, body ->
                      calls.add(method to path)
                      if (body != null) bodies.add(body)
                      if (failure.isNotEmpty() && path.contains(failure))
                          TransportResponse(500, """{"message":"unavailable"}""")
                      else
                          when {
                            path.endsWith("/preview") -> {
                              val request = Json.decodeFromString<AnalysisPreviewRequest>(body!!)
                              TransportResponse(
                                  200,
                                  Json.encodeToString(
                                      analysisPreviewFixture()
                                          .copy(
                                              refresh = request.refresh,
                                              limits = request.limits,
                                              retryStaleFailed =
                                                  request.retryStaleFailed && !wrongRetryPreview)))
                            }
                            path.endsWith("/control") -> {
                              val request = Json.decodeFromString<AnalysisRunControlRequest>(body!!)
                              run =
                                  when (request.action) {
                                    "resume" ->
                                        run.copy(
                                            identity =
                                                run.identity.copy(generation = "new-generation"))
                                    "cancel" -> run.copy(status = "canceled")
                                    else -> run.copy(status = "paused")
                                  }
                              TransportResponse(200, Json.encodeToString(run))
                            }
                            path.contains("/analysis/results?") -> {
                              val category = path.substringAfter("category=").substringBefore('&')
                              val filter = path.substringAfter("&path=", "")
                              var result = analysisResultsFixture(run, category, filter)
                              if (wrongResult) result = result.copy(path = "wrong.go")
                              TransportResponse(200, Json.encodeToString(result))
                            }
                            path.contains("/analysis/selection?") ->
                                TransportResponse(200, Json.encodeToString(selectionFixture()))
                            path.contains("/overview?") ->
                                TransportResponse(
                                    200, Json.encodeToString(ProjectOverview(analysisRun = run)))
                            path.contains("/analysis/run") ->
                                TransportResponse(200, Json.encodeToString(run))
                            else -> error("unexpected $method $path")
                          }
                    }),
            scope,
            io,
            coordinator,
            { state },
            ::dispatch)

    init {
      coordinator.projectOpened(WorkflowProjectIdentity("project", "revision"))
    }

    fun dispatch(event: DesktopEvent) {
      workflow.beforeEvent(event)
      state = state.reduce(event)
      if (event is DesktopEvent.ProjectLoaded)
          coordinator.projectOpened(
              WorkflowProjectIdentity(event.project.projectId, event.project.projectRevision))
    }

    fun confirm() {
      workflow.confirmProvider("bug-provider", true)
      workflow.confirmProvider("analyze-provider", true)
      workflow.confirmSecurity(true)
    }

    fun drain() {
      repeat(8) {
        main.runPending()
        io.runPending()
      }
      main.runPending()
    }

    fun await(predicate: () -> Boolean) {
      repeat(100) {
        drain()
        if (predicate()) return
        Thread.sleep(5)
      }
      assertTrue(predicate(), "Timed out waiting for analysis state")
    }

    override fun close() {
      workflow.detach()
      coordinator.close()
      scope.cancel()
      drain()
    }
  }
}

internal class AnalysisQueuedDispatcher : CoroutineDispatcher() {
  private val pending = java.util.concurrent.ConcurrentLinkedQueue<Runnable>()

  override fun dispatch(context: CoroutineContext, block: Runnable) {
    pending.add(block)
  }

  fun runPending() {
    while (true) (pending.poll() ?: return).run()
  }
}

internal fun analysisPreviewFixture() =
    AnalysisRunPreview(
        "1",
        "preview",
        AnalysisQueueIdentity("project", "revision", "policy", "providers", "queue"),
        "project",
        false,
        AnalysisRunLimits(100, 900, 2),
        files = listOf(AnalysisPlannedFile("main.go", "base", "Go", 20, emptyList())),
        providers =
            listOf(
                AnalysisProviderRequirement(
                    "bug-provider",
                    listOf("semantic"),
                    AnalysisEffectiveModel(
                        "bug",
                        "remote",
                        "bug-model",
                        providerOrigin = "https://bug.example",
                        remoteProvider = true,
                        timeout = "2m0s"),
                    true),
                AnalysisProviderRequirement(
                    "analyze-provider",
                    listOf("performance", "security_ai"),
                    AnalysisEffectiveModel(
                        "analyze",
                        "remote",
                        "review-model",
                        providerOrigin = "https://analyze.example",
                        remoteProvider = true,
                        timeout = "5m0s"),
                    true)),
        expectedModelRequests = 3,
        maxModelRequests = 6,
        securityReviewIntentRequired = true)

internal fun analysisRunFixture() =
    AnalysisRun(
        "1",
        AnalysisRunIdentity(
            "project", "revision", "policy", "providers", "queue", "run", "generation"),
        analysisPreviewFixture(),
        "paused",
        files = listOf(AnalysisRunFile("main.go", "base", "Go", emptyList())),
        sections =
            listOf("bugs", "performance", "security").map {
              AnalysisSectionProgress(
                  it, "paused", AnalysisRunCoverage(total = 1, succeeded = 1), 1)
            })

internal fun analysisResultsFixture(run: AnalysisRun, category: String, path: String = "") =
    AnalysisSectionResults(
        run.identity,
        run.sections.first { it.category == category },
        path,
        semantic =
            if (category == "bugs")
                listOf(
                    UnifiedFinding(
                        id = "bug",
                        category = "bugs",
                        message = run.status,
                        projectId = "project",
                        projectRevision = "revision",
                        fileHash = "base",
                        location = FindingLocation("main.go")))
            else emptyList(),
        performance =
            if (category == "performance")
                listOf(
                    PerformanceFileReport(
                        projectId = "project",
                        projectRevision = "revision",
                        path = "main.go",
                        contentHash = "base",
                        status = "completed_empty"))
            else emptyList(),
        security =
            if (category == "security")
                listOf(
                    SecurityFileReport(
                        projectId = "project",
                        projectRevision = "revision",
                        path = "main.go",
                        contentHash = "base",
                        status = "completed",
                        source = "ai",
                        findings =
                            listOf(
                                SecurityFinding(
                                    id = "security", verificationState = "unverified"))))
            else emptyList())

internal fun analysisProjectFixture(id: String = "project") =
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

internal fun analysisFileFixture(path: String = "main.go") =
    ProjectFileInfo(
        path,
        "base",
        path,
        language = "Go",
        sizeBytes = 20,
        lineCount = 1,
        modifiedAt = "",
        binary = false,
        content = "package main")
