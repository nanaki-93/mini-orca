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

class DesktopBenchmarkWorkflowTest {
  @Test
  fun exploringPerformanceEvidenceWithoutCandidateDoesNotDispatchTransportOrActions() {
    Harness().use { harness ->
      var sourceActions = 0
      val page = performancePageFixture()
      val browser = newResultBrowserState(page)
      browser.choose(performanceResults(page).single().row().key)
      ComposeVisualFixture(800, 650) {
            PerformanceWorkspacePane(
                PerformanceWorkspacePaneState(page, null, browser = browser),
                PerformanceWorkspaceActions(
                    prepareOptimization = { sourceActions++ },
                    openAnalysis = { sourceActions++ },
                    semanticActions =
                        FindingActions(
                            { sourceActions++ }, { _, _ -> sourceActions++ }, { sourceActions++ }),
                    openSource = { sourceActions++ },
                    loadBenchmarks = harness.workflow::loadGoBenchmarks,
                    selectBenchmark = harness.workflow::selectGoBenchmark,
                    runBenchmark = harness.workflow::compareSelectedGoBenchmark))
          }
          .use { fixture ->
            fixture.render()
            fixture.clickDescription("Expand Explore benchmark evidence")
            fixture.render()
            assertTrue(
                fixture.hasText(
                    performanceBenchmarkStatusPresentation(null, null, null, false).summary))
            assertTrue(
                fixture.hasText(
                    "Listing compatible benchmarks is read-only and does not execute project code."))
            assertTrue(fixture.isDisabled("List compatible benchmarks"))
            harness.completeRequest()
            assertTrue(
                harness.methods.isEmpty(),
                "Disclosure must not call even read-only catalog transport")
            assertTrue(harness.events.isEmpty(), "Disclosure must not select, trust or run")
            assertEquals(0, sourceActions, "Disclosure must not prepare, navigate or write source")
          }
    }
  }

  @Test
  fun catalogAndSelectionNeverAuthorizeExecutionImplicitly() {
    Harness().use { harness ->
      harness.workflow.loadGoBenchmarks()
      harness.completeRequest()
      assertEquals(listOf("GET"), harness.methods)
      assertEquals(BenchmarkDiscoveryOutcome.Loaded, harness.state.review.benchmark.discovery)
      assertEquals(BenchmarkAdmissionOutcome.Idle, harness.state.review.benchmark.admission)
      assertNull(harness.state.review.benchmark.selected)
      harness.workflow.compareSelectedGoBenchmark()
      harness.completeRequest()
      assertEquals(listOf("GET"), harness.methods)
      harness.workflow.selectGoBenchmark(choice.copy(scope = "unlisted"))
      assertNull(harness.state.review.benchmark.selected)
      harness.workflow.selectGoBenchmark(choice)
      harness.completeRequest()
      assertEquals(listOf("GET"), harness.methods)
      assertEquals(choice, harness.state.review.benchmark.selected)
    }
  }

  @Test
  fun invalidationCancelsQueuedCatalogWithoutIssuingARequest() {
    Harness().use { harness ->
      harness.workflow.loadGoBenchmarks()
      harness.main.runPending()
      harness.workflow.invalidate()
      harness.completeRequest()
      assertTrue(harness.methods.isEmpty())
      assertNull(harness.state.review.benchmark.catalog)
    }
  }

  @Test
  fun cancelStopsComparisonAndCancelsItsQueuedRequest() {
    Harness().use { harness ->
      harness.selectBenchmark()
      harness.workflow.compareSelectedGoBenchmark()
      harness.main.runPending()
      assertTrue(harness.state.review.benchmark.running)
      harness.workflow.cancel()
      assertFalse(harness.state.review.benchmark.running)
      assertEquals(BenchmarkAdmissionOutcome.Stopped, harness.state.review.benchmark.admission)
      harness.completeRequest()
      assertTrue(harness.methods.isEmpty())
      assertNull(harness.state.review.benchmark.comparison)
    }
  }

  @Test
  fun newerCatalogReplacesPendingPublicationForTheSameDraft() {
    Harness().use { harness ->
      harness.workflow.loadGoBenchmarks()
      harness.main.runPending()
      harness.io.runPending()
      harness.response = TransportResponse(200, Json.encodeToString(catalog.copy(reason = "newer")))
      harness.workflow.loadGoBenchmarks()
      harness.completeRequest()
      assertEquals("newer", harness.state.review.benchmark.catalog?.reason)
      assertEquals(1, harness.events.filterIsInstance<DesktopEvent.GoBenchmarkCatalogLoaded>().size)
    }
  }

  @Test
  fun catalogUsesCurrentStateWhenItsResponseIsReadyToPublish() {
    Harness().use { harness ->
      harness.workflow.loadGoBenchmarks()
      harness.main.runPending()
      harness.io.runPending()
      // Change the authoritative store without lifecycle cancellation to exercise the identity
      // guard.
      harness.controller.dispatch(DesktopEvent.DraftLoaded(draft().copy(hash = "new-hash")))
      harness.main.runPending()
      assertNull(harness.state.review.benchmark.catalog)
      assertTrue(harness.events.isEmpty())
    }
  }

  @Test
  fun comparisonUsesCurrentSelectionWhenItsResponseIsReadyToPublish() {
    Harness().use { harness ->
      harness.selectBenchmark()
      harness.response = TransportResponse(200, Json.encodeToString(comparison))
      harness.workflow.compareSelectedGoBenchmark()
      harness.main.runPending()
      harness.io.runPending()
      harness.controller.dispatch(DesktopEvent.GoBenchmarkSelected(choice.copy(name = "other")))
      harness.main.runPending()
      assertNull(harness.state.review.benchmark.comparison)
      assertFalse(harness.state.review.benchmark.running)
    }
  }

  @Test
  fun selectingAnotherListedBenchmarkCancelsPendingComparison() {
    Harness().use { harness ->
      val other = choice.copy(name = "BenchmarkOther", scope = "other-scope")
      harness.dispatch(
          DesktopEvent.GoBenchmarkCatalogLoaded(catalog.copy(benchmarks = listOf(choice, other))))
      harness.workflow.selectGoBenchmark(choice)
      harness.workflow.compareSelectedGoBenchmark()
      harness.main.runPending()
      harness.workflow.selectGoBenchmark(other)
      harness.completeRequest()
      assertTrue(harness.methods.isEmpty())
      assertEquals(other, harness.state.review.benchmark.selected)
      assertFalse(harness.state.review.benchmark.running)
    }
  }

  @Test
  fun changedExecutionTrustScopeCannotAuthorizeOrCompare() {
    Harness().use { harness ->
      harness.dispatch(DesktopEvent.GoBenchmarkCatalogLoaded(catalog.copy(trusted = false)))
      harness.workflow.selectGoBenchmark(choice)
      harness.response =
          TransportResponse(
              200,
              """{"project_id":"project","project_revision":"revision","commands":[["go","test","other"]]}""")
      harness.workflow.compareSelectedGoBenchmark()
      harness.completeRequest()
      assertEquals(listOf("GET"), harness.methods)
      assertFalse(harness.state.review.benchmark.running)
      assertTrue(harness.state.jobs.error.orEmpty().contains("trust scope changed"))
      assertEquals(
          BenchmarkAdmissionOutcome.Failed(harness.state.jobs.error!!),
          harness.state.review.benchmark.admission)
    }
  }

  @Test
  fun catalogUnavailabilityAndLookupFailureRemainDistinctWithPriorEvidence() {
    Harness().use { harness ->
      harness.dispatch(DesktopEvent.GoBenchmarkComparisonLoaded(comparison))
      harness.response =
          TransportResponse(
              200,
              Json.encodeToString(
                  catalog.copy(available = false, reason = "No compatible benchmark")))
      harness.workflow.loadGoBenchmarks()
      harness.completeRequest()
      assertEquals(
          BenchmarkDiscoveryOutcome.Unavailable("No compatible benchmark"),
          harness.state.review.benchmark.discovery)
      assertEquals(comparison, harness.state.review.benchmark.comparison)
      harness.response = TransportResponse(500, """{"message":"Lookup failed"}""")
      harness.workflow.loadGoBenchmarks()
      harness.completeRequest()
      assertEquals(
          BenchmarkDiscoveryOutcome.Failed("Lookup failed"),
          harness.state.review.benchmark.discovery)
      assertNull(harness.state.review.benchmark.catalog)
      assertNull(harness.state.review.benchmark.selected)
      assertEquals(comparison, harness.state.review.benchmark.comparison)
      assertEquals(listOf("GET", "GET"), harness.methods)
    }
  }

  @Test
  fun comparisonCompletionAndFailureEndActiveStateWithoutInventingMeasurements() {
    Harness().use { harness ->
      harness.selectBenchmark()
      harness.response = TransportResponse(200, Json.encodeToString(comparison))
      harness.workflow.compareSelectedGoBenchmark()
      assertEquals(BenchmarkAdmissionOutcome.Running, harness.state.review.benchmark.admission)
      harness.completeRequest()
      assertEquals(BenchmarkAdmissionOutcome.Idle, harness.state.review.benchmark.admission)
      assertEquals(comparison, harness.state.review.benchmark.comparison)
      harness.response = TransportResponse(500, """{"message":"Comparison failed"}""")
      harness.workflow.compareSelectedGoBenchmark()
      harness.completeRequest()
      assertEquals(
          BenchmarkAdmissionOutcome.Failed("Comparison failed"),
          harness.state.review.benchmark.admission)
      assertFalse(harness.state.review.benchmark.running)
      assertEquals(comparison, harness.state.review.benchmark.comparison)
      assertEquals(listOf("POST", "POST"), harness.methods)
    }
  }

  @Test
  fun projectEventCancelsPendingBenchmarkWork() {
    Harness().use { harness ->
      harness.workflow.loadGoBenchmarks()
      harness.main.runPending()
      harness.dispatch(
          DesktopEvent.ProjectLoaded(project("other", "new"), ProjectIndex("other", "new")))
      harness.completeRequest()
      assertTrue(harness.methods.isEmpty())
      assertNull(harness.state.review.benchmark.catalog)
    }
  }

  private inner class Harness : AutoCloseable {
    val main = QueuedDispatcher()
    val io = QueuedDispatcher()
    private val scope = CoroutineScope(SupervisorJob() + main)
    val controller = DesktopWorkflowController()
    val state
      get() = controller.state

    val methods = mutableListOf<String>()
    val events = mutableListOf<DesktopEvent>()
    var response = TransportResponse(200, Json.encodeToString(catalog))
    val workflow =
        DesktopBenchmarkWorkflow(
            ApiClient(
                transport =
                    DaemonTransport { method, _, _ ->
                      methods.add(method)
                      response
                    }),
            scope,
            io,
            { controller.state },
            ::dispatch)

    init {
      controller.dispatch(
          DesktopEvent.ProjectLoaded(project(), ProjectIndex("project", "revision")))
      controller.dispatch(DesktopEvent.FileLoaded(file(), emptyList()))
      controller.dispatch(DesktopEvent.DraftLoaded(draft()))
    }

    fun dispatch(event: DesktopEvent) {
      workflow.beforeEvent(event)
      controller.dispatch(event)
      events.add(event)
    }

    fun selectBenchmark() {
      dispatch(DesktopEvent.GoBenchmarkCatalogLoaded(catalog))
      workflow.selectGoBenchmark(choice)
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

  private val choice =
      GoBenchmarkChoice("BenchmarkRun", listOf("go", "test", "-bench", "^BenchmarkRun$"), "scope")
  private val catalog =
      GoBenchmarkCatalog(
          draftId = "draft",
          draftRevision = 1,
          draftHash = "draft-hash",
          projectId = "project",
          projectRevision = "revision",
          baseFileHash = "base",
          targetPath = "main.go",
          available = true,
          trusted = true,
          benchmarks = listOf(choice))
  private val comparison =
      GoBenchmarkComparison(
          draftId = "draft",
          draftRevision = 1,
          draftHash = "draft-hash",
          projectId = "project",
          projectRevision = "revision",
          baseFileHash = "base",
          targetPath = "main.go",
          benchmark = choice.name,
          scope = choice.scope,
          status = "completed")

  private fun project(id: String = "project", revision: String = "revision") =
      ProjectAnalysis(
          id,
          revision,
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

  private fun draft() =
      DeclarationDraft(
          id = "draft",
          projectId = "project",
          projectRevision = "revision",
          baseFileHash = "base",
          targetPath = "main.go",
          mode = "replace_symbol",
          targetSymbol = "Run",
          declaration = "func Run() {}",
          revision = 1,
          hash = "draft-hash",
          validation =
              DeclarationValidation(
                  true, "strict_symbol", diff = UnifiedDiff("main.go", "main.go")),
      )
}
