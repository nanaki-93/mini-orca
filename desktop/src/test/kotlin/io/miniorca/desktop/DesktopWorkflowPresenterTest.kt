package io.miniorca.desktop

import java.util.Collections
import java.util.concurrent.CountDownLatch
import java.util.concurrent.Executors
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicInteger
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.asCoroutineDispatcher
import kotlinx.coroutines.cancel
import kotlinx.coroutines.launch

class DesktopWorkflowPresenterTest {
  @Test
  fun benchmarkComparisonUsesAnExplicitReadOnlyCatalogThenRetainsExactEvidence() {
    val catalogCalls = AtomicInteger()
    val comparisonCalls = AtomicInteger()
    val presenter = presenter { method, path, body ->
      when (method to path) {
        "GET" to
            "/api/projects/current/drafts/draft/benchmarks?project_revision=revision&expected_revision=1&expected_hash=draft-hash" -> {
          catalogCalls.incrementAndGet()
          response(benchmarkCatalogJson())
        }
        "POST" to "/api/projects/current/drafts/draft/benchmarks" -> {
          comparisonCalls.incrementAndGet()
          assertTrue(body.orEmpty().contains("\"benchmark\":\"BenchmarkRun\""))
          assertTrue(body.orEmpty().contains("\"expected_scope\":\"scope\""))
          response(benchmarkComparisonJson())
        }
        else -> error("unexpected request $method $path")
      }
    }
    try {
      loadFile(presenter)
      presenter.dispatch(DesktopEvent.DraftLoaded(draft()))

      assertEquals(0, catalogCalls.get())
      assertEquals(0, comparisonCalls.get())
      presenter.loadGoBenchmarks()
      eventually { presenter.snapshot.value.state.review.benchmark.catalog != null }

      assertEquals(1, catalogCalls.get())
      assertNull(presenter.snapshot.value.state.review.benchmark.selected)
      presenter.compareSelectedGoBenchmark()
      assertEquals(0, comparisonCalls.get())
      assertFalse(presenter.snapshot.value.state.review.benchmark.running)

      presenter.selectGoBenchmark(
          presenter.snapshot.value.state.review.benchmark.catalog!!.benchmarks.single())
      presenter.compareSelectedGoBenchmark()
      eventually {
        presenter.snapshot.value.state.review.benchmark.comparison?.status == "completed"
      }

      assertEquals(1, comparisonCalls.get())
      assertEquals(
          "BenchmarkRun", presenter.snapshot.value.state.review.benchmark.comparison?.benchmark)
    } finally {
      presenter.close()
    }
  }

  @Test
  fun benchmarkCatalogRequestIsCanceledWhenTheDraftChanges() {
    val started = CountDownLatch(1)
    val interrupted = CountDownLatch(1)
    val release = CountDownLatch(1)
    val presenter = presenter { method, path, _ ->
      when (method to path) {
        "GET" to
            "/api/projects/current/drafts/draft/benchmarks?project_revision=revision&expected_revision=1&expected_hash=draft-hash" -> {
          started.countDown()
          try {
            release.await(2, TimeUnit.SECONDS)
          } catch (error: InterruptedException) {
            interrupted.countDown()
            throw error
          }
          response(benchmarkCatalogJson())
        }
        else -> error("unexpected request $method $path")
      }
    }
    try {
      loadFile(presenter)
      presenter.dispatch(DesktopEvent.DraftLoaded(draft()))
      presenter.loadGoBenchmarks()
      assertTrue(started.await(1, TimeUnit.SECONDS))

      presenter.dispatch(DesktopEvent.DraftEdited(declaration = "func Run() int { return 2 }"))

      assertTrue(interrupted.await(1, TimeUnit.SECONDS))
      assertNull(presenter.snapshot.value.state.review.benchmark.catalog)
      assertFalse(presenter.snapshot.value.state.loading)
    } finally {
      release.countDown()
      presenter.close()
    }
  }

  @Test
  fun benchmarkCatalogRequestIsCanceledImmediatelyWhenAFileIsSelected() {
    val started = CountDownLatch(1)
    val interrupted = CountDownLatch(1)
    val release = CountDownLatch(1)
    val presenter = presenter { _, path, _ ->
      when {
        path.startsWith("/api/projects/current/drafts/draft/benchmarks") -> {
          started.countDown()
          try {
            release.await(2, TimeUnit.SECONDS)
          } catch (error: InterruptedException) {
            interrupted.countDown()
            throw error
          }
          response(benchmarkCatalogJson())
        }
        path.contains("files/info?path=other.go") -> response(fileJson("other.go", "other"))
        path.contains("files/symbols?path=other.go") -> response(symbolsJson("other.go"))
        path.contains("files/analysis?path=other.go") ->
            response("""{"path":"other.go","status":"missing"}""")
        path.contains("/impact?path=other.go") -> response("""{"target_path":"other.go"}""")
        path.contains("/git?path=other.go") -> response("""{"available":false}""")
        else -> error("unexpected request $path")
      }
    }
    try {
      loadFile(presenter)
      presenter.dispatch(DesktopEvent.DraftLoaded(draft()))
      presenter.loadGoBenchmarks()
      assertTrue(started.await(1, TimeUnit.SECONDS))

      presenter.selectFile("other.go")

      assertTrue(interrupted.await(1, TimeUnit.SECONDS))
      eventually { presenter.snapshot.value.state.selectedFile?.path == "other.go" }
      assertNull(presenter.snapshot.value.state.review.benchmark.catalog)
    } finally {
      release.countDown()
      presenter.close()
    }
  }

  @Test
  fun projectSwitchAndCloseCancelBothBenchmarkRequestKinds() {
    for (comparing in listOf(false, true)) {
      for (closing in listOf(false, true)) {
        val started = CountDownLatch(1)
        val interrupted = CountDownLatch(1)
        val release = CountDownLatch(1)
        val presenter = presenter { method, path, _ ->
          check(path.contains("/benchmarks"))
          if (comparing && method == "GET") {
            response(benchmarkCatalogJson(trusted = true))
          } else {
            started.countDown()
            try {
              release.await(2, TimeUnit.SECONDS)
            } catch (error: InterruptedException) {
              interrupted.countDown()
              throw error
            }
            response(if (comparing) benchmarkComparisonJson() else benchmarkCatalogJson())
          }
        }
        try {
          loadFile(presenter)
          presenter.dispatch(DesktopEvent.DraftLoaded(draft()))
          presenter.loadGoBenchmarks()
          if (comparing) {
            eventually { presenter.snapshot.value.state.review.benchmark.catalog != null }
            presenter.selectGoBenchmark(
                presenter.snapshot.value.state.review.benchmark.catalog!!.benchmarks.single())
            presenter.compareSelectedGoBenchmark()
          }
          assertTrue(started.await(1, TimeUnit.SECONDS))
          if (closing) presenter.close()
          else
              presenter.dispatch(
                  DesktopEvent.ProjectLoaded(project("other", "new"), ProjectIndex("other", "new")))
          assertTrue(interrupted.await(1, TimeUnit.SECONDS))
          assertFalse(presenter.snapshot.value.state.review.benchmark.running)
          assertNull(presenter.snapshot.value.state.review.benchmark.comparison)
        } finally {
          release.countDown()
          presenter.close()
        }
      }
    }
  }

  @Test
  fun failedReanalysisDoesNotLeaveBenchmarkComparisonRunning() {
    val presenter = presenter { method, path, _ ->
      if (method == "POST" && path == "/api/projects/current/reindex")
          throw IllegalStateException("reindex failed")
      else error("unexpected request $method $path")
    }
    try {
      loadFile(presenter)
      presenter.dispatch(DesktopEvent.DraftLoaded(draft()))
      presenter.dispatch(DesktopEvent.GoBenchmarkComparisonStarted)
      assertTrue(presenter.snapshot.value.state.review.benchmark.running)

      presenter.reanalyze()

      eventually { presenter.snapshot.value.state.error == "reindex failed" }
      assertFalse(presenter.snapshot.value.state.review.benchmark.running)
      assertFalse(presenter.snapshot.value.state.loading)
    } finally {
      presenter.close()
    }
  }

  @Test
  fun benchmarkComparisonResponseCannotPublishAfterTheDraftChanges() {
    val started = CountDownLatch(1)
    val release = CountDownLatch(1)
    val presenter = presenter { method, path, _ ->
      when (method to path) {
        "GET" to
            "/api/projects/current/drafts/draft/benchmarks?project_revision=revision&expected_revision=1&expected_hash=draft-hash" ->
            response(benchmarkCatalogJson(trusted = true))
        "POST" to "/api/projects/current/drafts/draft/benchmarks" -> {
          started.countDown()
          release.await(2, TimeUnit.SECONDS)
          response(benchmarkComparisonJson())
        }
        else -> error("unexpected request $method $path")
      }
    }
    try {
      loadFile(presenter)
      presenter.dispatch(DesktopEvent.DraftLoaded(draft()))
      presenter.loadGoBenchmarks()
      eventually { presenter.snapshot.value.state.review.benchmark.catalog != null }
      presenter.selectGoBenchmark(
          presenter.snapshot.value.state.review.benchmark.catalog!!.benchmarks.single())
      presenter.compareSelectedGoBenchmark()
      assertTrue(started.await(1, TimeUnit.SECONDS))
      presenter.dispatch(DesktopEvent.DraftEdited(declaration = "func Run() int { return 1 }"))
      assertFalse(presenter.snapshot.value.state.review.benchmark.running)
      assertFalse(presenter.snapshot.value.state.loading)
      release.countDown()
      eventually { !presenter.snapshot.value.state.review.benchmark.running }

      assertNull(presenter.snapshot.value.state.review.benchmark.comparison)
      assertNull(presenter.snapshot.value.state.review.benchmark.catalog)
    } finally {
      release.countDown()
      presenter.close()
    }
  }

  @Test
  fun unavailableBenchmarkResponseWithARecomputedScopeStopsAndPublishesStaleEvidence() {
    val presenter = presenter { method, path, _ ->
      when (method to path) {
        "GET" to
            "/api/projects/current/drafts/draft/benchmarks?project_revision=revision&expected_revision=1&expected_hash=draft-hash" ->
            response(benchmarkCatalogJson(trusted = true))
        "POST" to "/api/projects/current/drafts/draft/benchmarks" ->
            response(unavailableBenchmarkComparisonJson(scope = "recomputed-scope"))
        else -> error("unexpected request $method $path")
      }
    }
    try {
      loadFile(presenter)
      presenter.dispatch(DesktopEvent.DraftLoaded(draft()))
      presenter.loadGoBenchmarks()
      eventually { presenter.snapshot.value.state.review.benchmark.catalog != null }
      presenter.selectGoBenchmark(
          presenter.snapshot.value.state.review.benchmark.catalog!!.benchmarks.single())
      presenter.compareSelectedGoBenchmark()

      eventually {
        !presenter.snapshot.value.state.review.benchmark.running &&
            presenter.snapshot.value.state.review.benchmark.comparison != null
      }

      val comparison = presenter.snapshot.value.state.review.benchmark.comparison!!
      assertEquals("unavailable", comparison.status)
      assertEquals("recomputed-scope", comparison.scope)
      assertEquals(
          "Stale · selected benchmark changed",
          performanceBenchmarkPresentation(
                  comparison,
                  goBenchmarkComparisonIdentity(presenter.snapshot.value.state.review.draft),
                  presenter.snapshot.value.state.review.benchmark.selected)
              .stateLabel)
    } finally {
      presenter.close()
    }
  }

  @Test
  fun explanationPublishesOnlyForItsExactSelectionAndDoesNotAlterDraftState() {
    val calls = AtomicInteger()
    val presenter = presenter { method, path, _ ->
      if (method == "POST" && path == "/api/projects/current/files/explanation") {
        calls.incrementAndGet()
        response(explanationJson("Run"))
      } else error("unexpected request $method $path")
    }
    try {
      loadFile(presenter)
      presenter.explainSelectedDeclaration()
      eventually {
        presenter.snapshot.value.declarationExplanation.status ==
            DeclarationExplanationStatus.Current
      }

      assertEquals(1, calls.get())
      assertEquals("Explains Run.", presenter.snapshot.value.declarationExplanation.result?.summary)
      assertEquals(null, presenter.snapshot.value.state.chat.session)
      assertEquals(null, presenter.snapshot.value.state.review.draft)
      assertEquals(null, presenter.snapshot.value.state.review.checks)
    } finally {
      presenter.close()
    }
  }

  @Test
  fun selectionChangeCancelsAndRejectsLateDeclarationExplanation() {
    val started = CountDownLatch(1)
    val release = CountDownLatch(1)
    val presenter = presenter { method, path, _ ->
      if (method == "POST" && path == "/api/projects/current/files/explanation") {
        started.countDown()
        release.await(2, TimeUnit.SECONDS)
        response(explanationJson("Run"))
      } else error("unexpected request $method $path")
    }
    try {
      loadProject(presenter)
      val run = SymbolInfo("Run", "function", confidence = "exact", atomicTarget = true)
      val other = SymbolInfo("Other", "function", confidence = "exact", atomicTarget = true)
      presenter.dispatch(DesktopEvent.FileLoaded(file(), listOf(run, other)))
      presenter.dispatch(DesktopEvent.SymbolSelected(run))
      presenter.explainSelectedDeclaration()
      assertTrue(started.await(1, TimeUnit.SECONDS))
      presenter.dispatch(DesktopEvent.SymbolSelected(other))
      release.countDown()
      eventually {
        presenter.snapshot.value.declarationExplanation.status == DeclarationExplanationStatus.Stale
      }

      assertEquals(null, presenter.snapshot.value.declarationExplanation.result)
      assertTrue(
          presenter.snapshot.value.declarationExplanation.message.contains("Selection changed"))
    } finally {
      release.countDown()
      presenter.close()
    }
  }

  @Test
  fun cachedSymbolExplanationRemainsAvailableWithoutAProviderRequest() {
    val calls = AtomicInteger()
    val presenter = presenter { _, _, _ -> calls.incrementAndGet().let { response("{}") } }
    try {
      loadFile(presenter)
      presenter.dispatch(
          DesktopEvent.AnalysisLoaded(
              FileAnalysis(
                  path = "main.go",
                  status = "fresh",
                  symbolExplanations = mapOf("Run" to "Cached explanation."))))

      val inspector =
          symbolInspectorUiState(
              presenter.snapshot.value.state.selectedFile,
              presenter.snapshot.value.state.symbols,
              presenter.snapshot.value.state.selectedSymbol,
              presenter.snapshot.value.state.analysis,
              false,
              InspectorProviderState(false, false),
              null)!!
      assertEquals("Cached explanation.", inspector.selectedSymbol?.explanation)
      assertEquals(0, calls.get())
    } finally {
      presenter.close()
    }
  }

  @Test
  fun remoteDeclarationExplanationRequiresCurrentFunctionConsentBeforeAnyCall() {
    val explanationCalls = AtomicInteger()
    val presenter = presenter { method, path, _ ->
      when (method to path) {
        "GET" to "/status" -> response("{\"status\":\"ok\",\"version\":\"v1\"}")
        "GET" to "/api/models/current" ->
            response(
                """{"scopes":{"function":{"scope":"function","profile":"function","model":"remote","remote_provider":true}}}""")
        "POST" to "/api/projects/current/files/explanation" -> {
          explanationCalls.incrementAndGet()
          response(explanationJson("Run"))
        }
        else -> error("unexpected request $method $path")
      }
    }
    try {
      presenter.refreshConnection()
      eventually { presenter.snapshot.value.model(ModelScope.Function).remoteProvider }
      loadFile(presenter)
      presenter.explainSelectedDeclaration()

      assertEquals(0, explanationCalls.get())
      assertEquals(
          DeclarationExplanationStatus.Failed,
          presenter.snapshot.value.declarationExplanation.status)
      assertTrue(presenter.snapshot.value.declarationExplanation.message.contains("Confirm"))
    } finally {
      presenter.close()
    }
  }

  @Test
  fun explanationConflictPublishesStaleWhileOtherFailuresRemainFailed() {
    val conflictPresenter = presenter { method, path, _ ->
      if (method == "POST" && path == "/api/projects/current/files/explanation")
          TransportResponse(
              409, """{"type":"conflict","message":"stale","user_message":"Refresh."}""")
      else error("unexpected request $method $path")
    }
    val failedPresenter = presenter { method, path, _ ->
      if (method == "POST" && path == "/api/projects/current/files/explanation")
          TransportResponse(
              502,
              """{"type":"internal","message":"provider failed","user_message":"Try again."}""")
      else error("unexpected request $method $path")
    }
    try {
      loadFile(conflictPresenter)
      conflictPresenter.explainSelectedDeclaration()
      eventually {
        conflictPresenter.snapshot.value.declarationExplanation.status ==
            DeclarationExplanationStatus.Stale
      }
      assertEquals(null, conflictPresenter.snapshot.value.declarationExplanation.result)

      loadFile(failedPresenter)
      failedPresenter.explainSelectedDeclaration()
      eventually {
        failedPresenter.snapshot.value.declarationExplanation.status ==
            DeclarationExplanationStatus.Failed
      }
      assertEquals(null, failedPresenter.snapshot.value.declarationExplanation.result)
    } finally {
      conflictPresenter.close()
      failedPresenter.close()
    }
  }

  @Test
  fun lateFileResponseCannotReplaceTheLatestSelection() {
    val firstStarted = CountDownLatch(1)
    val releaseFirst = CountDownLatch(1)
    val presenter = presenter { _, path, _ ->
      when {
        path.contains("files/info?path=first.go") -> {
          firstStarted.countDown()
          releaseFirst.await(2, TimeUnit.SECONDS)
          response(fileJson("first.go", "first"))
        }
        path.contains("files/info?path=second.go") -> response(fileJson("second.go", "second"))
        path.contains("files/symbols?path=second.go") -> response(symbolsJson("second.go"))
        path.contains("files/analysis") ->
            response("{\"path\":\"second.go\",\"status\":\"missing\"}")
        path.contains("/impact") -> response("{\"target_path\":\"second.go\"}")
        path.contains("/git") -> response("{\"available\":false}")
        else -> error("unexpected request $path")
      }
    }
    try {
      loadProject(presenter)
      presenter.selectFile("first.go")
      assertTrue(firstStarted.await(1, TimeUnit.SECONDS))
      presenter.selectFile("second.go")
      eventually { presenter.snapshot.value.state.selectedFile?.path == "second.go" }
      releaseFirst.countDown()

      assertEquals("second.go", presenter.snapshot.value.state.selectedFile?.path)
    } finally {
      releaseFirst.countDown()
      presenter.close()
    }
  }

  @Test
  fun cancelingAnalysisRetainsTheCurrentFileAndClearsOperationState() {
    val analysisStarted = CountDownLatch(1)
    val releaseAnalysis = CountDownLatch(1)
    val presenter = presenter { _, path, _ ->
      when {
        path.contains("files/info?path=main.go") -> response(fileJson("main.go", "base"))
        path.contains("files/symbols?path=main.go") -> response(symbolsJson("main.go", "Run"))
        path == "/api/projects/current/files/analysis" -> {
          analysisStarted.countDown()
          releaseAnalysis.await(2, TimeUnit.SECONDS)
          response("{\"path\":\"main.go\",\"status\":\"fresh\"}")
        }
        path.contains("files/analysis") -> response("{\"path\":\"main.go\",\"status\":\"missing\"}")
        path.contains("/impact") -> response("{\"target_path\":\"main.go\"}")
        path.contains("/git") -> response("{\"available\":false}")
        else -> error("unexpected request $path")
      }
    }
    try {
      loadProject(presenter)
      presenter.selectFile("main.go")
      eventually { presenter.snapshot.value.state.selectedFile?.path == "main.go" }
      presenter.analyzeSelected(refresh = false)
      assertTrue(analysisStarted.await(1, TimeUnit.SECONDS))
      presenter.cancelAnalysis()
      eventually { !presenter.snapshot.value.analysisInProgress }

      assertEquals("main.go", presenter.snapshot.value.state.selectedFile?.path)
      assertFalse(presenter.snapshot.value.analysisInProgress)
    } finally {
      releaseAnalysis.countDown()
      presenter.close()
    }
  }

  @Test
  fun reconnectPublishesDaemonFailuresAndThenTheRecoveredConnection() {
    val attempts = AtomicInteger()
    val presenter = presenter { _, path, _ ->
      when (path) {
        "/status" ->
            if (attempts.incrementAndGet() == 1) error("daemon down")
            else response("{\"status\":\"ok\",\"version\":\"v1\"}")
        "/api/models/current" ->
            response(
                "{\"scopes\":{\"function\":{\"scope\":\"function\",\"profile\":\"local\",\"model\":\"fixture\"}}}")
        else -> error("unexpected request $path")
      }
    }
    try {
      presenter.refreshConnection()
      eventually { attempts.get() == 1 && !presenter.snapshot.value.state.connection.connected }
      presenter.refreshConnection()
      eventually { presenter.snapshot.value.state.connection.connected }

      assertEquals("Daemon connected", presenter.snapshot.value.state.connection.label)
      assertEquals("fixture", presenter.snapshot.value.model(ModelScope.Function).model)
    } finally {
      presenter.close()
    }
  }

  @Test
  fun lateAnalyzeAllPollingCannotOverwriteAReplacementProject() {
    val pollStarted = CountDownLatch(1)
    val releasePoll = CountDownLatch(1)
    val presenter = presenter { _, path, _ ->
      when (path) {
        "/api/projects/current/analysis-job" ->
            response(
                "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"running\",\"files\":[]}")
        "/api/projects/current/analysis-job?project_revision=revision" -> {
          pollStarted.countDown()
          releasePoll.await(2, TimeUnit.SECONDS)
          response(
              "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"completed\",\"files\":[]}")
        }
        else -> error("unexpected request $path")
      }
    }
    try {
      loadProject(presenter)
      presenter.startAnalyzeAll(AnalyzeAllRunOptions())
      assertTrue(pollStarted.await(1, TimeUnit.SECONDS))
      presenter.dispatch(
          DesktopEvent.ProjectLoaded(
              project("next", "next-revision"), ProjectIndex("next", "next-revision")))
      releasePoll.countDown()
      eventually { presenter.snapshot.value.state.project?.projectId == "next" }

      assertEquals("next", presenter.snapshot.value.state.project?.projectId)
      assertEquals(null, presenter.snapshot.value.state.findings.analyzeAll)
    } finally {
      releasePoll.countDown()
      presenter.close()
    }
  }

  @Test
  fun analyzeAllPollsOnceAndStopsAfterItsTerminalResponse() {
    val polls = AtomicInteger()
    val presenter = presenter { method, path, _ ->
      when (method to path) {
        "POST" to "/api/projects/current/analysis-job" ->
            response(
                "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"running\"}")
        "GET" to "/api/projects/current/analysis-job?project_revision=revision" -> {
          polls.incrementAndGet()
          response(
              "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"completed\"}")
        }
        else -> response("{}")
      }
    }
    try {
      loadProject(presenter)
      presenter.startAnalyzeAll(AnalyzeAllRunOptions())
      eventually { presenter.snapshot.value.state.findings.analyzeAll?.status == "completed" }
      Thread.sleep(25)

      assertEquals(1, polls.get())
    } finally {
      presenter.close()
    }
  }

  @Test
  fun performancePollsOnlyWhileRunningAndStopsWhenPaused() {
    val polls = AtomicInteger()
    val presenter = presenter { method, path, _ ->
      when (method to path) {
        "POST" to "/api/projects/current/performance-job" ->
            response(
                "{\"id\":\"job\",\"project_id\":\"project\",\"project_revision\":\"revision\",\"queue_id\":\"queue\",\"status\":\"running\"}")
        "GET" to "/api/projects/current/performance-job?project_revision=revision" -> {
          polls.incrementAndGet()
          response(
              "{\"id\":\"job\",\"project_id\":\"project\",\"project_revision\":\"revision\",\"queue_id\":\"queue\",\"status\":\"paused\"}")
        }
        "GET" to "/api/projects/current/performance?project_revision=revision" ->
            TransportResponse(204, "")
        else -> response("{}")
      }
    }
    try {
      loadProject(presenter)
      presenter.startPerformance(
          PerformanceQueuePreview(
              projectId = "project", projectRevision = "revision", queueId = "queue", maxFiles = 1),
          confirmRemoteProvider = false)
      eventually { presenter.snapshot.value.state.findings.performanceJob?.status == "paused" }
      Thread.sleep(25)

      assertEquals(1, polls.get())
    } finally {
      presenter.close()
    }
  }

  @Test
  fun closeCancelsAnInFlightAnalyzeAllPollBeforeItCanPublish() {
    val pollStarted = CountDownLatch(1)
    val releasePoll = CountDownLatch(1)
    val presenter = presenter { _, path, _ ->
      when (path) {
        "/api/projects/current/analysis-job" ->
            response(
                "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"running\",\"files\":[]}")
        "/api/projects/current/analysis-job?project_revision=revision" -> {
          pollStarted.countDown()
          releasePoll.await(2, TimeUnit.SECONDS)
          response(
              "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"completed\",\"files\":[]}")
        }
        else -> response("{}")
      }
    }
    try {
      loadProject(presenter)
      presenter.startAnalyzeAll(AnalyzeAllRunOptions())
      assertTrue(pollStarted.await(1, TimeUnit.SECONDS))
      presenter.close()
      releasePoll.countDown()
      Thread.sleep(25)

      assertEquals("running", presenter.snapshot.value.state.findings.analyzeAll?.status)
    } finally {
      releasePoll.countDown()
      presenter.close()
    }
  }

  @Test
  fun lateAnalyzeAllActionCannotReplaceTheLatestActionForTheSameProject() {
    val firstStarted = CountDownLatch(1)
    val releaseFirst = CountDownLatch(1)
    val starts = AtomicInteger()
    val presenter = presenter { method, path, _ ->
      when (method to path) {
        "POST" to "/api/projects/current/analysis-job" ->
            if (starts.incrementAndGet() == 1) {
              firstStarted.countDown()
              releaseFirst.await(2, TimeUnit.SECONDS)
              response(
                  "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"paused\"}")
            } else
                response(
                    "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"running\"}")
        "GET" to "/api/projects/current/analysis-job?project_revision=revision" ->
            response(
                "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"completed\"}")
        else -> response("{}")
      }
    }
    try {
      loadProject(presenter)
      presenter.startAnalyzeAll(AnalyzeAllRunOptions())
      assertTrue(firstStarted.await(1, TimeUnit.SECONDS))
      presenter.startAnalyzeAll(AnalyzeAllRunOptions())
      eventually { presenter.snapshot.value.state.findings.analyzeAll?.status == "completed" }
      releaseFirst.countDown()
      Thread.sleep(25)

      assertEquals("completed", presenter.snapshot.value.state.findings.analyzeAll?.status)
    } finally {
      releaseFirst.countDown()
      presenter.close()
    }
  }

  @Test
  fun delayedVerifiedScanStartCannotReplaceACancelResponse() {
    val startRequested = CountDownLatch(1)
    val releaseStart = CountDownLatch(1)
    val findingsRefreshes = AtomicInteger()
    val presenter = presenter { method, path, _ ->
      when (method to path) {
        "POST" to "/api/projects/current/scan" -> {
          startRequested.countDown()
          releaseStart.await(2, TimeUnit.SECONDS)
          response(
              "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"running\"}")
        }
        "DELETE" to "/api/projects/current/scan?project_revision=revision" ->
            response(
                "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"canceled\"}")
        "GET" to "/api/projects/current/findings?project_revision=revision" -> {
          findingsRefreshes.incrementAndGet()
          response("{}")
        }
        else -> response("{}")
      }
    }
    try {
      loadProject(presenter)
      presenter.runVerifiedScan()
      assertTrue(startRequested.await(1, TimeUnit.SECONDS))
      presenter.cancelVerifiedScan()
      eventually { presenter.snapshot.value.state.findings.scan?.status == "canceled" }
      eventually { findingsRefreshes.get() == 1 }
      releaseStart.countDown()
      Thread.sleep(25)

      assertEquals("canceled", presenter.snapshot.value.state.findings.scan?.status)
      assertEquals(1, findingsRefreshes.get())
    } finally {
      releaseStart.countDown()
      presenter.close()
    }
  }

  @Test
  fun delayedOlderVerifiedScanStartCannotReplaceANewerStart() {
    val firstStartRequested = CountDownLatch(1)
    val releaseFirstStart = CountDownLatch(1)
    val starts = AtomicInteger()
    val presenter = presenter { method, path, _ ->
      when (method to path) {
        "POST" to "/api/projects/current/scan" ->
            if (starts.incrementAndGet() == 1) {
              firstStartRequested.countDown()
              releaseFirstStart.await(2, TimeUnit.SECONDS)
              response(
                  "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"canceled\"}")
            } else
                response(
                    "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"running\"}")
        "GET" to "/api/projects/current/scan?project_revision=revision" ->
            response(
                "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"completed\"}")
        else -> response("{}")
      }
    }
    try {
      loadProject(presenter)
      presenter.runVerifiedScan()
      assertTrue(firstStartRequested.await(1, TimeUnit.SECONDS))
      presenter.runVerifiedScan()
      eventually { presenter.snapshot.value.state.findings.scan?.status == "completed" }
      releaseFirstStart.countDown()
      Thread.sleep(25)

      assertEquals("completed", presenter.snapshot.value.state.findings.scan?.status)
    } finally {
      releaseFirstStart.countDown()
      presenter.close()
    }
  }

  @Test
  fun cancelingVerifiedScanPollsToTerminalStateAndRefreshesFindingsOnce() {
    val polls = AtomicInteger()
    val findingsRefreshes = AtomicInteger()
    val presenter = presenter { method, path, _ ->
      when (method to path) {
        "DELETE" to "/api/projects/current/scan?project_revision=revision" ->
            response(
                "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"canceling\"}")
        "GET" to "/api/projects/current/scan?project_revision=revision" -> {
          polls.incrementAndGet()
          response(
              "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"canceled\"}")
        }
        "GET" to "/api/projects/current/findings?project_revision=revision" -> {
          findingsRefreshes.incrementAndGet()
          response("{}")
        }
        else -> response("{}")
      }
    }
    try {
      loadProject(presenter)
      presenter.dispatch(
          DesktopEvent.GoScanLoaded(
              GoScanReport(
                  projectId = "project", projectRevision = "revision", status = "running")))
      presenter.cancelVerifiedScan()
      eventually { presenter.snapshot.value.state.findings.scan?.status == "canceled" }
      eventually { findingsRefreshes.get() == 1 }

      assertEquals(1, polls.get())
      assertEquals(1, findingsRefreshes.get())
    } finally {
      presenter.close()
    }
  }

  @Test
  fun terminalPollThatOverlapsDelayedCancelCannotPublishOrRefreshTwice() {
    val pollStarted = CountDownLatch(1)
    val releasePoll = CountDownLatch(1)
    val cancelStarted = CountDownLatch(1)
    val releaseCancel = CountDownLatch(1)
    val findingsRefreshes = AtomicInteger()
    val presenter = presenter { method, path, _ ->
      when (method to path) {
        "POST" to "/api/projects/current/scan" ->
            response(
                "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"running\"}")
        "GET" to "/api/projects/current/scan?project_revision=revision" -> {
          pollStarted.countDown()
          releasePoll.await(2, TimeUnit.SECONDS)
          response(
              "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"canceled\"}")
        }
        "DELETE" to "/api/projects/current/scan?project_revision=revision" -> {
          cancelStarted.countDown()
          releaseCancel.await(2, TimeUnit.SECONDS)
          response(
              "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"canceled\"}")
        }
        "GET" to "/api/projects/current/findings?project_revision=revision" -> {
          findingsRefreshes.incrementAndGet()
          response("{}")
        }
        else -> response("{}")
      }
    }
    try {
      loadProject(presenter)
      presenter.runVerifiedScan()
      assertTrue(pollStarted.await(1, TimeUnit.SECONDS))
      presenter.cancelVerifiedScan()
      assertTrue(cancelStarted.await(1, TimeUnit.SECONDS))
      releasePoll.countDown()
      Thread.sleep(25)

      assertEquals("running", presenter.snapshot.value.state.findings.scan?.status)
      assertEquals(0, findingsRefreshes.get())

      releaseCancel.countDown()
      eventually { presenter.snapshot.value.state.findings.scan?.status == "canceled" }
      eventually { findingsRefreshes.get() == 1 }

      assertEquals(1, findingsRefreshes.get())
    } finally {
      releasePoll.countDown()
      releaseCancel.countDown()
      presenter.close()
    }
  }

  @Test
  fun failedAnalyzeAllActionRestoresOnePollForTheActiveJob() {
    val initialPollStarted = CountDownLatch(1)
    val releaseInitialPoll = CountDownLatch(1)
    val polls = AtomicInteger()
    val presenter = presenter { method, path, _ ->
      when (method to path) {
        "POST" to "/api/projects/current/analysis-job" ->
            response(
                "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"running\"}")
        "POST" to "/api/projects/current/analysis-job/pause?project_revision=revision" ->
            TransportResponse(500, "pause failed")
        "GET" to "/api/projects/current/analysis-job?project_revision=revision" ->
            if (polls.incrementAndGet() == 1) {
              initialPollStarted.countDown()
              releaseInitialPoll.await(2, TimeUnit.SECONDS)
              response(
                  "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"running\"}")
            } else
                response(
                    "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"completed\"}")
        else -> response("{}")
      }
    }
    try {
      loadProject(presenter)
      presenter.startAnalyzeAll(AnalyzeAllRunOptions())
      assertTrue(initialPollStarted.await(1, TimeUnit.SECONDS))
      presenter.pauseAnalyzeAll()
      eventually { presenter.snapshot.value.state.findings.analyzeAll?.status == "completed" }
      Thread.sleep(25)

      assertEquals(2, polls.get())
      assertTrue(presenter.snapshot.value.state.jobs.error != null)
    } finally {
      releaseInitialPoll.countDown()
      presenter.close()
    }
  }

  @Test
  fun failedPerformanceActionRestoresOnePollForTheActiveJob() {
    val initialPollStarted = CountDownLatch(1)
    val releaseInitialPoll = CountDownLatch(1)
    val polls = AtomicInteger()
    val presenter = presenter { method, path, _ ->
      when (method to path) {
        "POST" to "/api/projects/current/performance-job" ->
            response(
                "{\"id\":\"job\",\"project_id\":\"project\",\"project_revision\":\"revision\",\"queue_id\":\"queue\",\"status\":\"running\"}")
        "POST" to
            "/api/projects/current/performance-job/pause?project_revision=revision&expected_job_id=job" ->
            TransportResponse(500, "pause failed")
        "GET" to "/api/projects/current/performance-job?project_revision=revision" ->
            if (polls.incrementAndGet() == 1) {
              initialPollStarted.countDown()
              releaseInitialPoll.await(2, TimeUnit.SECONDS)
              response(
                  "{\"id\":\"job\",\"project_id\":\"project\",\"project_revision\":\"revision\",\"queue_id\":\"queue\",\"status\":\"running\"}")
            } else
                response(
                    "{\"id\":\"job\",\"project_id\":\"project\",\"project_revision\":\"revision\",\"queue_id\":\"queue\",\"status\":\"paused\"}")
        "GET" to "/api/projects/current/performance?project_revision=revision" ->
            TransportResponse(204, "")
        else -> response("{}")
      }
    }
    try {
      loadProject(presenter)
      presenter.startPerformance(
          PerformanceQueuePreview(
              projectId = "project", projectRevision = "revision", queueId = "queue", maxFiles = 1),
          confirmRemoteProvider = false)
      assertTrue(initialPollStarted.await(1, TimeUnit.SECONDS))
      presenter.pausePerformance()
      eventually { presenter.snapshot.value.state.findings.performanceJob?.status == "paused" }
      Thread.sleep(25)

      assertEquals(2, polls.get())
      assertTrue(presenter.snapshot.value.state.jobs.error != null)
    } finally {
      releaseInitialPoll.countDown()
      presenter.close()
    }
  }

  @Test
  fun failedVerifiedScanStartRestoresOnePollForARunningScan() {
    val initialPollStarted = CountDownLatch(1)
    val releaseInitialPoll = CountDownLatch(1)
    val starts = AtomicInteger()
    val polls = AtomicInteger()
    val presenter = presenter { method, path, _ ->
      when (method to path) {
        "POST" to "/api/projects/current/scan" ->
            if (starts.incrementAndGet() == 1)
                response(
                    "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"running\"}")
            else TransportResponse(500, "start failed")
        "GET" to "/api/projects/current/scan?project_revision=revision" ->
            if (polls.incrementAndGet() == 1) {
              initialPollStarted.countDown()
              releaseInitialPoll.await(2, TimeUnit.SECONDS)
              response(
                  "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"running\"}")
            } else
                response(
                    "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"canceled\"}")
        else -> response("{}")
      }
    }
    try {
      loadProject(presenter)
      presenter.runVerifiedScan()
      assertTrue(initialPollStarted.await(1, TimeUnit.SECONDS))
      presenter.runVerifiedScan()
      eventually { presenter.snapshot.value.state.findings.scan?.status == "canceled" }
      Thread.sleep(25)

      assertEquals(2, polls.get())
      assertTrue(presenter.snapshot.value.state.jobs.error != null)
    } finally {
      releaseInitialPoll.countDown()
      presenter.close()
    }
  }

  @Test
  fun failedVerifiedScanCancelRestoresOnePollForACancelingScan() {
    val initialPollStarted = CountDownLatch(1)
    val releaseInitialPoll = CountDownLatch(1)
    val polls = AtomicInteger()
    val presenter = presenter { method, path, _ ->
      when (method to path) {
        "POST" to "/api/projects/current/scan" ->
            response(
                "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"canceling\"}")
        "DELETE" to "/api/projects/current/scan?project_revision=revision" ->
            TransportResponse(500, "cancel failed")
        "GET" to "/api/projects/current/scan?project_revision=revision" ->
            if (polls.incrementAndGet() == 1) {
              initialPollStarted.countDown()
              releaseInitialPoll.await(2, TimeUnit.SECONDS)
              response(
                  "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"canceling\"}")
            } else
                response(
                    "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"canceled\"}")
        else -> response("{}")
      }
    }
    try {
      loadProject(presenter)
      presenter.runVerifiedScan()
      assertTrue(initialPollStarted.await(1, TimeUnit.SECONDS))
      presenter.cancelVerifiedScan()
      eventually { presenter.snapshot.value.state.findings.scan?.status == "canceled" }
      Thread.sleep(25)

      assertEquals(2, polls.get())
      assertTrue(presenter.snapshot.value.state.jobs.error != null)
    } finally {
      releaseInitialPoll.countDown()
      presenter.close()
    }
  }

  @Test
  fun queuedOlderAnalyzeAllPauseCannotReachTheDaemonAfterANewerStart() {
    val dispatcher = Executors.newSingleThreadExecutor().asCoroutineDispatcher()
    val scope = CoroutineScope(SupervisorJob() + dispatcher)
    val blockerStarted = CountDownLatch(1)
    val releaseBlocker = CountDownLatch(1)
    val pauses = AtomicInteger()
    val starts = AtomicInteger()
    scope.launch {
      blockerStarted.countDown()
      releaseBlocker.await(2, TimeUnit.SECONDS)
    }
    val presenter =
        presenter(
            responder = { method, path, _ ->
              when (method to path) {
                "POST" to "/api/projects/current/analysis-job/pause?project_revision=revision" -> {
                  pauses.incrementAndGet()
                  response(
                      "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"paused\"}")
                }
                "POST" to "/api/projects/current/analysis-job" -> {
                  starts.incrementAndGet()
                  response(
                      "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"completed\"}")
                }
                else -> error("unexpected request $method $path")
              }
            },
            parentScope = scope)
    try {
      assertTrue(blockerStarted.await(1, TimeUnit.SECONDS))
      loadProject(presenter)
      presenter.dispatch(
          DesktopEvent.AnalyzeAllLoaded(
              AnalyzeAllJob(
                  projectId = "project", projectRevision = "revision", status = "running")))

      presenter.pauseAnalyzeAll()
      presenter.startAnalyzeAll(AnalyzeAllRunOptions())
      releaseBlocker.countDown()

      eventually { starts.get() == 1 }
      assertEquals(0, pauses.get())
      assertEquals("completed", presenter.snapshot.value.state.findings.analyzeAll?.status)
    } finally {
      releaseBlocker.countDown()
      presenter.close()
      scope.cancel()
      dispatcher.close()
    }
  }

  @Test
  fun queuedOlderPerformancePauseCannotTargetAReplacementJob() {
    val dispatcher = Executors.newSingleThreadExecutor().asCoroutineDispatcher()
    val scope = CoroutineScope(SupervisorJob() + dispatcher)
    val blockerStarted = CountDownLatch(1)
    val releaseBlocker = CountDownLatch(1)
    val pauses = AtomicInteger()
    val starts = AtomicInteger()
    scope.launch {
      blockerStarted.countDown()
      releaseBlocker.await(2, TimeUnit.SECONDS)
    }
    val presenter =
        presenter(
            responder = { method, path, _ ->
              when (method to path) {
                "POST" to
                    "/api/projects/current/performance-job/pause?project_revision=revision&expected_job_id=old" -> {
                  pauses.incrementAndGet()
                  response(
                      "{\"id\":\"old\",\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"paused\"}")
                }
                "POST" to "/api/projects/current/performance-job" -> {
                  starts.incrementAndGet()
                  response(
                      "{\"id\":\"replacement\",\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"paused\"}")
                }
                "GET" to "/api/projects/current/performance?project_revision=revision" ->
                    TransportResponse(204, "")
                else -> error("unexpected request $method $path")
              }
            },
            parentScope = scope)
    try {
      assertTrue(blockerStarted.await(1, TimeUnit.SECONDS))
      loadProject(presenter)
      presenter.dispatch(
          DesktopEvent.PerformanceLoaded(
              PerformanceJob(
                  id = "old",
                  projectId = "project",
                  projectRevision = "revision",
                  status = "running"),
              null))

      presenter.pausePerformance()
      presenter.startPerformance(
          PerformanceQueuePreview(
              projectId = "project", projectRevision = "revision", queueId = "new", maxFiles = 1),
          confirmRemoteProvider = false)
      releaseBlocker.countDown()

      eventually { starts.get() == 1 }
      eventually {
        presenter.snapshot.value.state.findings.performanceJob?.let { job ->
          job.id == "replacement" && job.status == "paused"
        } == true
      }
      assertEquals(0, pauses.get())
      assertEquals("replacement", presenter.snapshot.value.state.findings.performanceJob?.id)
      assertEquals("paused", presenter.snapshot.value.state.findings.performanceJob?.status)
    } finally {
      releaseBlocker.countDown()
      presenter.close()
      scope.cancel()
      dispatcher.close()
    }
  }

  @Test
  fun queuedOlderVerifiedScanStartCannotReachTheDaemonAfterCancel() {
    val dispatcher = Executors.newSingleThreadExecutor().asCoroutineDispatcher()
    val scope = CoroutineScope(SupervisorJob() + dispatcher)
    val blockerStarted = CountDownLatch(1)
    val releaseBlocker = CountDownLatch(1)
    val starts = AtomicInteger()
    val cancels = AtomicInteger()
    scope.launch {
      blockerStarted.countDown()
      releaseBlocker.await(2, TimeUnit.SECONDS)
    }
    val presenter =
        presenter(
            responder = { method, path, _ ->
              when (method to path) {
                "POST" to "/api/projects/current/scan" -> {
                  starts.incrementAndGet()
                  response(
                      "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"running\"}")
                }
                "DELETE" to "/api/projects/current/scan?project_revision=revision" -> {
                  cancels.incrementAndGet()
                  response(
                      "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"canceled\"}")
                }
                "GET" to "/api/projects/current/findings?project_revision=revision" ->
                    response("{}")
                else -> error("unexpected request $method $path")
              }
            },
            parentScope = scope)
    try {
      assertTrue(blockerStarted.await(1, TimeUnit.SECONDS))
      loadProject(presenter)
      presenter.dispatch(
          DesktopEvent.GoScanLoaded(
              GoScanReport(
                  projectId = "project", projectRevision = "revision", status = "running")))

      presenter.runVerifiedScan()
      presenter.cancelVerifiedScan()
      releaseBlocker.countDown()

      eventually { cancels.get() == 1 }
      eventually { presenter.snapshot.value.state.findings.scan?.status == "canceled" }
      assertEquals(0, starts.get())
      assertEquals("canceled", presenter.snapshot.value.state.findings.scan?.status)
    } finally {
      releaseBlocker.countDown()
      presenter.close()
      scope.cancel()
      dispatcher.close()
    }
  }

  @Test
  fun delayedWorkspaceRefreshCannotReplaceANewerAnalyzeAllAction() {
    val dispatcher = Executors.newSingleThreadExecutor().asCoroutineDispatcher()
    val scope = CoroutineScope(SupervisorJob() + dispatcher)
    val overviewStarted = CountDownLatch(1)
    val releaseOverview = CountDownLatch(1)
    val staleJobFetched = CountDownLatch(1)
    val jobRequests = AtomicInteger()
    val presenter =
        presenter(parentScope = scope) { method, path, _ ->
          when (method to path) {
            "POST" to "/api/projects/import" -> response(projectJson())
            "GET" to "/api/projects/current/index" -> response(indexJson())
            "GET" to "/api/projects/current/overview?project_revision=revision" -> {
              overviewStarted.countDown()
              releaseOverview.await(2, TimeUnit.SECONDS)
              response("{}")
            }
            "GET" to "/api/projects/current/findings?project_revision=revision" -> response("{}")
            "GET" to "/api/projects/current/analysis-job?project_revision=revision" ->
                if (jobRequests.incrementAndGet() == 1)
                    response(
                        "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"completed\"}")
                else {
                  staleJobFetched.countDown()
                  response(
                      "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"paused\"}")
                }
            "GET" to "/api/projects/current/scan?project_revision=revision" ->
                TransportResponse(204, "")
            "GET" to "/api/projects/current/performance-job?project_revision=revision",
            "GET" to "/api/projects/current/performance?project_revision=revision" ->
                TransportResponse(204, "")
            "POST" to "/api/projects/current/analysis-job" ->
                response(
                    "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"running\"}")
            else -> error("unexpected request $method $path")
          }
        }
    try {
      presenter.loadProject("/tmp/project", restore = false)
      assertTrue(overviewStarted.await(1, TimeUnit.SECONDS))
      presenter.startAnalyzeAll(AnalyzeAllRunOptions())
      eventually { presenter.snapshot.value.state.findings.analyzeAll?.status == "completed" }
      releaseOverview.countDown()
      assertTrue(staleJobFetched.await(1, TimeUnit.SECONDS))
      Thread.sleep(25)

      assertEquals("completed", presenter.snapshot.value.state.findings.analyzeAll?.status)
    } finally {
      releaseOverview.countDown()
      presenter.close()
      scope.cancel()
      dispatcher.close()
    }
  }

  @Test
  fun delayedWorkspaceRefreshCannotReplaceANewerPerformanceAction() {
    val overviewStarted = CountDownLatch(1)
    val releaseOverview = CountDownLatch(1)
    val staleJobFetched = CountDownLatch(1)
    val jobRequests = AtomicInteger()
    val presenter = presenter { method, path, _ ->
      when (method to path) {
        "POST" to "/api/projects/import" -> response(projectJson())
        "GET" to "/api/projects/current/index" -> response(indexJson())
        "GET" to "/api/projects/current/overview?project_revision=revision" -> {
          overviewStarted.countDown()
          releaseOverview.await(2, TimeUnit.SECONDS)
          response("{}")
        }
        "GET" to "/api/projects/current/findings?project_revision=revision" -> response("{}")
        "GET" to "/api/projects/current/analysis-job?project_revision=revision",
        "GET" to "/api/projects/current/scan?project_revision=revision" ->
            TransportResponse(204, "")
        "GET" to "/api/projects/current/performance-job?project_revision=revision" ->
            if (jobRequests.incrementAndGet() == 1)
                response(
                    "{\"id\":\"job\",\"project_id\":\"project\",\"project_revision\":\"revision\",\"queue_id\":\"queue\",\"status\":\"paused\"}")
            else {
              staleJobFetched.countDown()
              response(
                  "{\"id\":\"stale\",\"project_id\":\"project\",\"project_revision\":\"revision\",\"queue_id\":\"old\",\"status\":\"canceled\"}")
            }
        "GET" to "/api/projects/current/performance?project_revision=revision" ->
            TransportResponse(204, "")
        "POST" to "/api/projects/current/performance-job" ->
            response(
                "{\"id\":\"job\",\"project_id\":\"project\",\"project_revision\":\"revision\",\"queue_id\":\"queue\",\"status\":\"running\"}")
        else -> error("unexpected request $method $path")
      }
    }
    try {
      presenter.loadProject("/tmp/project", restore = false)
      assertTrue(overviewStarted.await(1, TimeUnit.SECONDS))
      presenter.startPerformance(
          PerformanceQueuePreview(
              projectId = "project", projectRevision = "revision", queueId = "queue", maxFiles = 1),
          confirmRemoteProvider = false)
      eventually { presenter.snapshot.value.state.findings.performanceJob?.status == "paused" }
      releaseOverview.countDown()
      assertTrue(staleJobFetched.await(1, TimeUnit.SECONDS))
      Thread.sleep(25)

      assertEquals("paused", presenter.snapshot.value.state.findings.performanceJob?.status)
      assertEquals("job", presenter.snapshot.value.state.findings.performanceJob?.id)
    } finally {
      releaseOverview.countDown()
      presenter.close()
    }
  }

  @Test
  fun performanceReportFailureAfterAStartedJobKeepsItsPollActive() {
    val pollStarted = CountDownLatch(1)
    val releasePoll = CountDownLatch(1)
    val polls = AtomicInteger()
    val reports = AtomicInteger()
    val presenter = presenter { method, path, _ ->
      when (method to path) {
        "POST" to "/api/projects/current/performance-job" ->
            response(
                "{\"id\":\"job\",\"project_id\":\"project\",\"project_revision\":\"revision\",\"queue_id\":\"queue\",\"status\":\"running\"}")
        "GET" to "/api/projects/current/performance?project_revision=revision" ->
            if (reports.incrementAndGet() == 1) TransportResponse(500, "report failed")
            else TransportResponse(204, "")
        "GET" to "/api/projects/current/performance-job?project_revision=revision" -> {
          polls.incrementAndGet()
          pollStarted.countDown()
          releasePoll.await(2, TimeUnit.SECONDS)
          response(
              "{\"id\":\"job\",\"project_id\":\"project\",\"project_revision\":\"revision\",\"queue_id\":\"queue\",\"status\":\"paused\"}")
        }
        else -> response("{}")
      }
    }
    try {
      loadProject(presenter)
      presenter.startPerformance(
          PerformanceQueuePreview(
              projectId = "project", projectRevision = "revision", queueId = "queue", maxFiles = 1),
          confirmRemoteProvider = false)
      assertTrue(pollStarted.await(1, TimeUnit.SECONDS))
      eventually { presenter.snapshot.value.state.findings.performanceJob?.status == "running" }
      eventually { presenter.snapshot.value.state.jobs.error != null }
      releasePoll.countDown()
      eventually { presenter.snapshot.value.state.findings.performanceJob?.status == "paused" }
      Thread.sleep(25)

      assertEquals(1, polls.get())
    } finally {
      releasePoll.countDown()
      presenter.close()
    }
  }

  @Test
  fun delayedInitialPerformanceReportCannotOverwriteANewerPollReport() {
    val initialReportStarted = CountDownLatch(1)
    val releaseInitialReport = CountDownLatch(1)
    val pollJobStarted = CountDownLatch(1)
    val releasePollJob = CountDownLatch(1)
    val reports = AtomicInteger()
    val presenter = presenter { method, path, _ ->
      when (method to path) {
        "POST" to "/api/projects/current/performance-job" ->
            response(
                "{\"id\":\"job\",\"generation\":\"one\",\"project_id\":\"project\",\"project_revision\":\"revision\",\"queue_id\":\"queue\",\"status\":\"running\"}")
        "GET" to "/api/projects/current/performance-job?project_revision=revision" -> {
          pollJobStarted.countDown()
          releasePollJob.await(2, TimeUnit.SECONDS)
          response(
              "{\"id\":\"job\",\"generation\":\"one\",\"project_id\":\"project\",\"project_revision\":\"revision\",\"queue_id\":\"queue\",\"status\":\"paused\"}")
        }
        "GET" to "/api/projects/current/performance?project_revision=revision" ->
            if (reports.incrementAndGet() == 1) {
              initialReportStarted.countDown()
              releaseInitialReport.await(2, TimeUnit.SECONDS)
              response("{\"status\":\"old\"}")
            } else response("{\"status\":\"new\"}")
        else -> error("unexpected request $method $path")
      }
    }
    try {
      loadProject(presenter)
      presenter.startPerformance(
          PerformanceQueuePreview(
              projectId = "project", projectRevision = "revision", queueId = "queue", maxFiles = 1),
          confirmRemoteProvider = false)
      assertTrue(initialReportStarted.await(1, TimeUnit.SECONDS))
      assertTrue(pollJobStarted.await(1, TimeUnit.SECONDS))
      releasePollJob.countDown()
      eventually { presenter.snapshot.value.state.findings.performanceReport?.status == "new" }
      releaseInitialReport.countDown()
      Thread.sleep(25)

      assertEquals("new", presenter.snapshot.value.state.findings.performanceReport?.status)
      assertEquals("paused", presenter.snapshot.value.state.findings.performanceJob?.status)
    } finally {
      releasePollJob.countDown()
      releaseInitialReport.countDown()
      presenter.close()
    }
  }

  @Test
  fun delayedTerminalScanFindingsCannotReplaceFindingsFromANewerScan() {
    val firstRefreshStarted = CountDownLatch(1)
    val releaseFirstRefresh = CountDownLatch(1)
    val starts = AtomicInteger()
    val findingsRefreshes = AtomicInteger()
    val presenter = presenter { method, path, _ ->
      when (method to path) {
        "POST" to "/api/projects/current/scan" ->
            if (starts.incrementAndGet() == 1)
                response(
                    "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"canceled\"}")
            else
                response(
                    "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"running\"}")
        "GET" to "/api/projects/current/scan?project_revision=revision" ->
            response(
                "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"completed\"}")
        "GET" to "/api/projects/current/findings?project_revision=revision" ->
            if (findingsRefreshes.incrementAndGet() == 1) {
              firstRefreshStarted.countDown()
              releaseFirstRefresh.await(2, TimeUnit.SECONDS)
              response("{\"findings\":[{\"id\":\"old\"}]}")
            } else response("{\"findings\":[{\"id\":\"new\"}]}")
        else -> error("unexpected request $method $path")
      }
    }
    try {
      loadProject(presenter)
      presenter.runVerifiedScan()
      assertTrue(firstRefreshStarted.await(1, TimeUnit.SECONDS))
      presenter.runVerifiedScan()
      eventually {
        presenter.snapshot.value.state.findings.scan?.status == "completed" &&
            presenter.snapshot.value.state.findings.findings.singleOrNull()?.id == "new"
      }
      releaseFirstRefresh.countDown()
      Thread.sleep(25)

      assertEquals("new", presenter.snapshot.value.state.findings.findings.singleOrNull()?.id)
    } finally {
      releaseFirstRefresh.countDown()
      presenter.close()
    }
  }

  @Test
  fun delayedWorkspaceFindingsCannotReplaceANewerTerminalScanRefresh() {
    val workspaceFindingsStarted = CountDownLatch(1)
    val releaseWorkspaceFindings = CountDownLatch(1)
    val findingsRequests = AtomicInteger()
    val presenter = presenter { method, path, _ ->
      when (method to path) {
        "POST" to "/api/projects/import" -> response(projectJson())
        "GET" to "/api/projects/current/index" -> response(indexJson())
        "GET" to "/api/projects/current/overview?project_revision=revision" -> response("{}")
        "GET" to "/api/projects/current/findings?project_revision=revision" ->
            if (findingsRequests.incrementAndGet() == 1) {
              workspaceFindingsStarted.countDown()
              releaseWorkspaceFindings.await(2, TimeUnit.SECONDS)
              response("{\"findings\":[{\"id\":\"stale\"}]}")
            } else response("{\"findings\":[{\"id\":\"fresh\"}]}")
        "POST" to "/api/projects/current/scan" ->
            response(
                "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"canceled\"}")
        "GET" to "/api/projects/current/analysis-job?project_revision=revision",
        "GET" to "/api/projects/current/scan?project_revision=revision",
        "GET" to "/api/projects/current/performance-job?project_revision=revision",
        "GET" to "/api/projects/current/performance?project_revision=revision" ->
            TransportResponse(204, "")
        else -> error("unexpected request $method $path")
      }
    }
    try {
      presenter.loadProject("/tmp/project", restore = false)
      assertTrue(workspaceFindingsStarted.await(1, TimeUnit.SECONDS))
      presenter.runVerifiedScan()
      eventually { presenter.snapshot.value.state.findings.findings.singleOrNull()?.id == "fresh" }
      releaseWorkspaceFindings.countDown()
      Thread.sleep(25)

      assertEquals("fresh", presenter.snapshot.value.state.findings.findings.singleOrNull()?.id)
    } finally {
      releaseWorkspaceFindings.countDown()
      presenter.close()
    }
  }

  @Test
  fun chatProposalReplacesOnlyTheActiveTaskDraft() {
    val presenter = presenter { _, path, _ ->
      when {
        path.contains("files/info?path=main.go") -> response(fileJson("main.go", "base"))
        path.contains("files/symbols?path=main.go") -> response(symbolsJson("main.go", "Run"))
        path.contains("files/analysis") -> response("{\"path\":\"main.go\",\"status\":\"missing\"}")
        path.contains("/impact") -> response("{\"target_path\":\"main.go\"}")
        path.contains("/git") -> response("{\"available\":false}")
        path == "/api/projects/current/chat/sessions" ->
            response(
                "{\"id\":\"session\",\"project_id\":\"project\",\"project_revision\":\"revision\",\"base_file_hash\":\"base\",\"open_path\":\"main.go\",\"mode\":\"replace_symbol\",\"target_symbol\":\"Run\",\"state\":\"active\",\"messages\":[]}")
        path == "/api/projects/current/chat/sessions/session/messages" ->
            response(
                "{\"session_id\":\"session\",\"draft\":{\"id\":\"draft\",\"project_id\":\"project\",\"project_revision\":\"revision\",\"base_file_hash\":\"base\",\"target_path\":\"main.go\",\"mode\":\"replace_symbol\",\"target_symbol\":\"Run\",\"declaration\":\"func Run() {}\",\"revision\":1,\"hash\":\"draft-hash\",\"state\":\"generated\"},\"assistant_message\":{\"role\":\"assistant\",\"content\":\"Ready\"}}")
        else -> error("unexpected request $path")
      }
    }
    try {
      loadProject(presenter)
      presenter.selectFile("main.go")
      eventually { presenter.snapshot.value.state.selectedFile?.path == "main.go" }
      presenter.dispatch(
          DesktopEvent.SymbolSelected(
              SymbolInfo("Run", "function", confidence = "exact", atomicTarget = true)))
      presenter.sendChatMessage(ChatEditMode.ReplaceSymbol, "", "Improve Run")
      eventually { presenter.snapshot.value.state.review.draft?.id == "draft" }

      assertEquals("Run", presenter.snapshot.value.state.review.draft?.targetSymbol)
      assertEquals("session", presenter.snapshot.value.state.chat.session?.id)
    } finally {
      presenter.close()
    }
  }

  @Test
  fun explicitSendMakesOneChatRequestAndPreservesFunctionRemoteConsent() {
    val sessionRequests = AtomicInteger()
    val messageRequests = AtomicInteger()
    var messageBody = ""
    val presenter = presenter { method, path, body ->
      when (method to path) {
        "GET" to "/status" -> response("{\"status\":\"ok\",\"version\":\"v1\"}")
        "GET" to "/api/models/current" ->
            response(
                """{"scopes":{"function":{"scope":"function","profile":"remote","model":"provider/editor","remote_provider":true}}}""")
        "GET" to "/api/projects/current/files/info?path=main.go" ->
            response(fileJson("main.go", "base"))
        "GET" to "/api/projects/current/files/symbols?path=main.go" ->
            response(symbolsJson("main.go", "Run"))
        "GET" to "/api/projects/current/files/analysis?path=main.go&refresh=false" ->
            response("{\"path\":\"main.go\",\"status\":\"missing\"}")
        "GET" to "/api/projects/current/impact?path=main.go" ->
            response("{\"target_path\":\"main.go\"}")
        "GET" to "/api/projects/current/git?path=main.go" -> response("{\"available\":false}")
        "POST" to "/api/projects/current/chat/sessions" -> {
          sessionRequests.incrementAndGet()
          response(
              "{\"id\":\"session\",\"project_id\":\"project\",\"project_revision\":\"revision\",\"base_file_hash\":\"base\",\"open_path\":\"main.go\",\"mode\":\"replace_symbol\",\"target_symbol\":\"Run\",\"state\":\"active\",\"messages\":[]}")
        }
        "POST" to "/api/projects/current/chat/sessions/session/messages" -> {
          messageRequests.incrementAndGet()
          messageBody = body.orEmpty()
          response(
              "{\"session_id\":\"session\",\"draft\":{\"id\":\"draft\",\"project_id\":\"project\",\"project_revision\":\"revision\",\"base_file_hash\":\"base\",\"target_path\":\"main.go\",\"mode\":\"replace_symbol\",\"target_symbol\":\"Run\",\"declaration\":\"func Run() {}\",\"revision\":1,\"hash\":\"draft-hash\",\"state\":\"generated\"},\"assistant_message\":{\"role\":\"assistant\",\"content\":\"Ready\"}}")
        }
        else -> error("unexpected request $method $path")
      }
    }
    try {
      presenter.refreshConnection()
      eventually { presenter.snapshot.value.model(ModelScope.Function).remoteProvider }
      loadProject(presenter)
      presenter.selectFile("main.go")
      eventually { presenter.snapshot.value.state.selectedFile?.path == "main.go" }
      presenter.dispatch(
          DesktopEvent.SymbolSelected(
              SymbolInfo("Run", "function", confidence = "exact", atomicTarget = true)))

      presenter.sendChatMessage(
          ChatEditMode.ReplaceSymbol, "", FunctionChangePreset.BugFix.preparedMessage())
      assertEquals(0, sessionRequests.get())
      assertEquals(0, messageRequests.get())
      assertEquals(
          "Add a concise intent after the selected preset before sending.",
          presenter.snapshot.value.state.error)

      presenter.sendChatMessage(
          ChatEditMode.ReplaceSymbol, "", "Fix a bug: return a typed error for a missing user")
      assertEquals(0, sessionRequests.get())
      assertEquals(0, messageRequests.get())
      assertEquals(
          "Confirm the Function edits model destination before sending context.",
          presenter.snapshot.value.state.error)

      presenter.setProviderConfirmation(ModelScope.Function, true)
      presenter.sendChatMessage(
          ChatEditMode.ReplaceSymbol, "", "Fix a bug: return a typed error for a missing user")
      eventually { presenter.snapshot.value.state.review.draft?.id == "draft" }

      assertEquals(1, sessionRequests.get())
      assertEquals(1, messageRequests.get())
      assertTrue(messageBody.contains("\"confirm_remote_provider\":true"))
    } finally {
      presenter.close()
    }
  }

  @Test
  fun newLocalChangeDoesNotAttachAnOldPreparedBugTask() {
    val sessionBodies = Collections.synchronizedList(mutableListOf<String>())
    val messageRequests = AtomicInteger()
    val presenter = presenter { method, path, body ->
      when (method to path) {
        "GET" to "/api/projects/current/files/info?path=main.go" ->
            response(fileJson("main.go", "base"))
        "GET" to "/api/projects/current/files/symbols?path=main.go" ->
            response(symbolsJson("main.go", "Run"))
        "GET" to "/api/projects/current/files/analysis?path=main.go&refresh=false" ->
            response("{\"path\":\"main.go\",\"status\":\"missing\"}")
        "GET" to "/api/projects/current/impact?path=main.go" ->
            response("{\"target_path\":\"main.go\"}")
        "GET" to "/api/projects/current/git?path=main.go" -> response("{\"available\":false}")
        "POST" to "/api/projects/current/chat/sessions" -> {
          sessionBodies += body.orEmpty()
          response(
              "{\"id\":\"session\",\"project_id\":\"project\",\"project_revision\":\"revision\",\"base_file_hash\":\"base\",\"open_path\":\"main.go\",\"mode\":\"replace_symbol\",\"target_symbol\":\"Run\",\"state\":\"active\",\"messages\":[]}")
        }
        "POST" to "/api/projects/current/chat/sessions/session/messages" -> {
          messageRequests.incrementAndGet()
          error("stop after observing the request")
        }
        else -> error("unexpected request $method $path")
      }
    }
    try {
      loadProject(presenter)
      presenter.selectFile("main.go")
      eventually { presenter.snapshot.value.state.selectedFile?.path == "main.go" }
      val symbol = SymbolInfo("Run", "function", confidence = "exact", atomicTarget = true)
      presenter.dispatch(DesktopEvent.SymbolSelected(symbol))
      val task =
          BugTaskSpec(
              "1", "main.go", "Run", "func Run()", listOf("Return an error for empty input."))

      presenter.dispatch(
          DesktopEvent.SuggestionPrepared(
              "fix", "Fix a bug: return an error for empty input", symbol, task))
      presenter.sendChatMessage(
          ChatEditMode.ReplaceSymbol, "", "Fix a bug: return an error for empty input")
      eventually { messageRequests.get() == 1 && !presenter.snapshot.value.generating }
      assertTrue(sessionBodies.single().contains("\"task_spec\""))

      presenter.dispatch(
          DesktopEvent.SuggestionPrepared(
              "fix", "Fix a bug: return an error for empty input", symbol, task))
      presenter.clearPreparedSuggestion()
      presenter.sendChatMessage(
          ChatEditMode.ReplaceSymbol, "", "Change behavior: preserve insertion order")
      eventually { messageRequests.get() == 2 && !presenter.snapshot.value.generating }

      assertEquals(2, sessionBodies.size)
      assertFalse(sessionBodies.last().contains("\"task_spec\""))
    } finally {
      presenter.close()
    }
  }

  @Test
  fun applyAndUndoReloadOnlyTheBoundDraftFile() {
    val presenter = presenter { method, path, _ ->
      when (method to path) {
        "POST" to "/api/projects/current/apply" ->
            response(
                "{\"project_revision\":\"next\",\"post_apply_hash\":\"after\",\"undo_available\":true}")
        "POST" to "/api/projects/current/undo" ->
            response(
                "{\"project_revision\":\"restored\",\"post_apply_hash\":\"before\",\"undo_available\":true}")
        else -> response("{}")
      }
    }
    try {
      loadFile(presenter)
      val draft = draft()
      presenter.dispatch(DesktopEvent.DraftLoaded(draft))
      presenter.dispatch(
          DesktopEvent.ChecksLoaded(
              DraftCheckReport(
                  "main.go",
                  true,
                  draftId = draft.id,
                  draftRevision = draft.revision,
                  draftHash = draft.hash)))
      presenter.applyEditableDraft()
      eventually { presenter.snapshot.value.state.review.applied?.postApplyHash == "after" }
      presenter.undoAppliedDraft()
      eventually { presenter.snapshot.value.state.review.applied?.postApplyHash == "before" }

      assertEquals("before", presenter.snapshot.value.state.review.applied?.postApplyHash)
    } finally {
      presenter.close()
    }
  }

  @Test
  fun verifiedScanTrustsBeforeStartingAndNavigationSendsNoTrustRequest() {
    val requests = mutableListOf<String>()
    val presenter =
        presenter(interceptTrust = false) { method, path, _ ->
          requests += "$method $path"
          when (method to path) {
            "GET" to "/api/projects/current/execution-trust?project_revision=revision" ->
                response(
                    """{"project_id":"project","project_revision":"revision","trusted":false,"commands":[["go","test","./..."]]}""")
            "POST" to "/api/projects/current/scan" ->
                response(
                    """{"project_id":"project","project_revision":"revision","status":"canceled"}""")
            "GET" to "/api/projects/current/findings?project_revision=revision" -> response("{}")
            else -> response("{}")
          }
        }
    try {
      loadProject(presenter)
      assertTrue(requests.isEmpty())
      presenter.runVerifiedScan()
      eventually { requests.any { it == "POST /api/projects/current/scan" } }
      assertEquals(
          listOf(
              "GET /api/projects/current/execution-trust?project_revision=revision",
              "POST /api/projects/current/execution-trust",
              "POST /api/projects/current/scan"),
          requests.take(3))
    } finally {
      presenter.close()
    }
  }

  @Test
  fun remoteSecurityReviewUsesItsOwnOneTimeConfirmationAndNeverStartsFromSelection() {
    val reviewCalls = AtomicInteger()
    val presenter = presenter { method, path, _ ->
      when (method to path) {
        "GET" to "/status" -> response("{\"status\":\"ok\",\"version\":\"v1\"}")
        "GET" to "/api/models/current" ->
            response(
                """{"scopes":{"analyze":{"scope":"analyze","profile":"analyze","model":"remote","remote_provider":true}}}""")
        "POST" to "/api/projects/current/security-review" -> {
          reviewCalls.incrementAndGet()
          response(securityReportJson("ai"))
        }
        else -> response("{}")
      }
    }
    try {
      presenter.refreshConnection()
      eventually { presenter.snapshot.value.model(ModelScope.Analyze).remoteProvider }
      loadFile(presenter)
      presenter.dispatch(DesktopEvent.WorkspaceSelected(Workspace.Security))

      assertEquals(0, reviewCalls.get())
      presenter.setProviderConfirmation(ModelScope.Analyze, true)
      presenter.reviewSecurity()
      assertEquals(0, reviewCalls.get())
      assertTrue(presenter.snapshot.value.state.security.error.orEmpty().contains("Confirm"))

      presenter.setSecurityReviewRemoteConfirmation(true)
      presenter.reviewSecurity()
      eventually { reviewCalls.get() == 1 }
      assertFalse(presenter.snapshot.value.securityReviewRemoteConfirmed)

      presenter.reviewSecurity()
      assertEquals(1, reviewCalls.get())
    } finally {
      presenter.close()
    }
  }

  @Test
  fun securityActionsRejectReportsOwnedByTheOtherSource() {
    val presenter = presenter { method, path, _ ->
      when (method to path) {
        "POST" to "/api/projects/current/files/security-scan" -> response(securityReportJson("ai"))
        "POST" to "/api/projects/current/security-review" ->
            response(securityReportJson("deterministic"))
        else -> response("{}")
      }
    }
    try {
      loadFile(presenter)

      presenter.scanSecurity()
      eventually {
        presenter.snapshot.value.state.security.sourceOperation.status ==
            SecuritySectionOperationStatus.Failed
      }
      assertNull(presenter.snapshot.value.state.security.sourceReport)
      assertNull(presenter.snapshot.value.state.security.aiReport)

      presenter.reviewSecurity()
      eventually {
        presenter.snapshot.value.state.security.aiOperation.status ==
            SecuritySectionOperationStatus.Failed
      }
      assertNull(presenter.snapshot.value.state.security.sourceReport)
      assertNull(presenter.snapshot.value.state.security.aiReport)
    } finally {
      presenter.close()
    }
  }

  @Test
  fun reanalyzeCancelsTheVisibleSecurityAction() {
    val scanStarted = CountDownLatch(1)
    val releaseScan = CountDownLatch(1)
    val presenter = presenter { method, path, _ ->
      when (method to path) {
        "POST" to "/api/projects/current/files/security-scan" -> {
          scanStarted.countDown()
          releaseScan.await(2, TimeUnit.SECONDS)
          response(securityReportJson("deterministic"))
        }
        "POST" to "/api/projects/current/reindex" -> response(indexJson())
        else -> response("{}")
      }
    }
    try {
      loadFile(presenter)
      presenter.scanSecurity()
      assertTrue(scanStarted.await(1, TimeUnit.SECONDS))

      presenter.reanalyze()

      assertTrue(presenter.snapshot.value.state.security.action.isBlank())
      assertEquals(
          SecuritySectionOperationStatus.Canceled,
          presenter.snapshot.value.state.security.sourceOperation.status)
    } finally {
      releaseScan.countDown()
      presenter.close()
    }
  }

  @Test
  fun preparingASecurityFixOnlyOpensTheComposerPathWithoutASend() {
    val chatPosts = AtomicInteger()
    val presenter = presenter { method, path, _ ->
      when {
        method == "POST" && path.contains("/chat/") -> {
          chatPosts.incrementAndGet()
          response("{}")
        }
        path.contains("files/info?path=main.go") -> response(fileJson("main.go", "base"))
        path.contains("files/symbols?path=main.go") -> response(symbolsJson("main.go", "Run"))
        path.contains("files/analysis") -> response("{\"path\":\"main.go\",\"status\":\"missing\"}")
        path.contains("/impact") -> response("{\"target_path\":\"main.go\"}")
        path.contains("/git") -> response("{\"available\":false}")
        else -> response("{}")
      }
    }
    try {
      presenter.dispatch(
          DesktopEvent.ProjectLoaded(
              project(),
              ProjectIndex(
                  "project",
                  "revision",
                  files =
                      listOf(
                          IndexedFile(
                              "main.go",
                              "base",
                              "Go",
                              false,
                              lineCount = 8,
                              symbols =
                                  listOf(
                                      SymbolInfo(
                                          "Run",
                                          "function",
                                          startLine = 3,
                                          endLine = 6,
                                          confidence = "exact",
                                          atomicTarget = true)))))))
      presenter.dispatch(DesktopEvent.FileLoaded(file(), emptyList()))
      val finding =
          SecurityFinding(
              id = "security-1",
              anchor = SecuritySourceAnchor("main.go", 4, 4, "Run"),
              observedCondition = "Unchecked input reaches a sink.",
              remediation = "Validate the input.")
      presenter.dispatch(
          DesktopEvent.SecurityReportLoaded(
              SecurityFileReport(
                  "1",
                  "project",
                  "revision",
                  "main.go",
                  "base",
                  "completed",
                  "deterministic",
                  findings = listOf(finding))))
      presenter.prepareSecurityFinding(finding)
      eventually { presenter.snapshot.value.state.preparedRequest.contains("Address the reviewed") }

      assertEquals(0, chatPosts.get())
      assertEquals("fix", presenter.snapshot.value.state.preparedAction)
    } finally {
      presenter.close()
    }
  }

  @Test
  fun securityReviewAcceptsEligibleNonGoTextWithoutAnExactSymbol() {
    val reviewCalls = AtomicInteger()
    val presenter = presenter { method, path, body ->
      if (method == "POST" && path == "/api/projects/current/security-review") {
        reviewCalls.incrementAndGet()
        assertFalse(body.orEmpty().contains("Run"))
        response(securityReportJson("ai"))
      } else response("{}")
    }
    try {
      loadProject(presenter)
      presenter.dispatch(DesktopEvent.FileLoaded(file().copy(language = "Markdown"), emptyList()))

      presenter.reviewSecurity()
      eventually { reviewCalls.get() == 1 }
      eventually { presenter.snapshot.value.state.security.aiReport?.source == "ai" }
    } finally {
      presenter.close()
    }
  }

  @Test
  fun staleSecurityFindingCannotPrepareAComposerRequest() {
    val sourceOpenCalls = AtomicInteger()
    val presenter = presenter { _, path, _ ->
      if (path.contains("/files/info")) sourceOpenCalls.incrementAndGet()
      response("{}")
    }
    try {
      loadProject(presenter)
      presenter.dispatch(
          DesktopEvent.FileLoaded(
              file(),
              listOf(
                  SymbolInfo(
                      "Run",
                      "function",
                      startLine = 2,
                      endLine = 4,
                      confidence = "exact",
                      atomicTarget = true))))
      val stale =
          SecurityFinding(
              id = "stale",
              anchor = SecuritySourceAnchor("main.go", 5, 5, "Run"),
              observedCondition = "Old evidence.",
              remediation = "Refresh.")
      presenter.openSecurityFinding(stale)
      presenter.prepareSecurityFinding(stale)

      assertTrue(presenter.snapshot.value.state.jobs.error.orEmpty().contains("Refresh Security"))
      assertTrue(presenter.snapshot.value.state.preparedRequest.isBlank())
      assertEquals(0, sourceOpenCalls.get())
    } finally {
      presenter.close()
    }
  }

  private fun presenter(
      parentScope: CoroutineScope = CoroutineScope(SupervisorJob() + Dispatchers.Default),
      interceptTrust: Boolean = true,
      responder: (String, String, String?) -> TransportResponse,
  ): DesktopWorkflowPresenter {
    return DesktopWorkflowPresenter(
        ApiClient(
            transport =
                DaemonTransport { method, path, body ->
                  if (interceptTrust &&
                      method == "GET" &&
                      path == "/api/projects/current/execution-trust?project_revision=revision")
                      response(
                          """{"project_id":"project","project_revision":"revision","trusted":false,"commands":[["go","test","./..."]]}""")
                  else if (interceptTrust &&
                      method == "POST" &&
                      path == "/api/projects/current/execution-trust")
                      response(
                          """{"project_id":"project","project_revision":"revision","trusted":true,"commands":[["go","test","./..."]]}""")
                  else responder(method, path, body)
                }),
        LastProjectStore(),
        parentScope,
        Dispatchers.Default,
        5)
  }

  private fun loadProject(presenter: DesktopWorkflowPresenter) {
    presenter.dispatch(DesktopEvent.ProjectLoaded(project(), ProjectIndex("project", "revision")))
  }

  private fun loadFile(presenter: DesktopWorkflowPresenter) {
    loadProject(presenter)
    presenter.dispatch(
        DesktopEvent.FileLoaded(
            file(),
            listOf(SymbolInfo("Run", "function", confidence = "exact", atomicTarget = true))))
    presenter.dispatch(
        DesktopEvent.SymbolSelected(
            SymbolInfo("Run", "function", confidence = "exact", atomicTarget = true)))
  }

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

  private fun benchmarkCatalogJson(trusted: Boolean = false) =
      """{"draft_id":"draft","draft_revision":1,"draft_hash":"draft-hash","project_id":"project","project_revision":"revision","base_file_hash":"base","target_path":"main.go","available":true,"trusted":$trusted,"benchmarks":[{"name":"BenchmarkRun","command":["go","test","-run","^$","-bench","^BenchmarkRun$","-benchtime","100ms","-benchmem"],"scope":"scope"}]}"""

  private fun benchmarkComparisonJson() =
      """{"draft_id":"draft","draft_revision":1,"draft_hash":"draft-hash","project_id":"project","project_revision":"revision","base_file_hash":"base","target_path":"main.go","benchmark":"BenchmarkRun","scope":"scope","status":"completed","command":["go","test","-benchtime","100ms","-benchmem"],"base":{"samples":[{"iterations":1,"ns_per_op":100,"bytes_per_op":10,"allocs_per_op":1},{"iterations":1,"ns_per_op":100,"bytes_per_op":10,"allocs_per_op":1},{"iterations":1,"ns_per_op":100,"bytes_per_op":10,"allocs_per_op":1},{"iterations":1,"ns_per_op":100,"bytes_per_op":10,"allocs_per_op":1},{"iterations":1,"ns_per_op":100,"bytes_per_op":10,"allocs_per_op":1}]},"candidate":{"samples":[{"iterations":1,"ns_per_op":90,"bytes_per_op":10,"allocs_per_op":1},{"iterations":1,"ns_per_op":90,"bytes_per_op":10,"allocs_per_op":1},{"iterations":1,"ns_per_op":90,"bytes_per_op":10,"allocs_per_op":1},{"iterations":1,"ns_per_op":90,"bytes_per_op":10,"allocs_per_op":1},{"iterations":1,"ns_per_op":90,"bytes_per_op":10,"allocs_per_op":1}]}}"""

  private fun unavailableBenchmarkComparisonJson(scope: String) =
      """{"draft_id":"draft","draft_revision":1,"draft_hash":"draft-hash","project_id":"project","project_revision":"revision","base_file_hash":"base","target_path":"main.go","benchmark":"BenchmarkRun","scope":"$scope","status":"unavailable","reason":"displayed benchmark scope changed","command":["go","test","-benchtime","100ms","-benchmem"]}"""

  private fun response(body: String) = TransportResponse(200, body)

  private fun explanationJson(symbol: String) =
      """{"version":"v1","project_id":"project","project_revision":"revision","base_file_hash":"base","anchor":{"path":"main.go","symbol":"$symbol","signature":"","start_line":0,"end_line":0},"summary":"Explains $symbol.","behavior":[],"inputs":[],"outputs":[],"side_effects":[],"error_behavior":[],"context_manifest":{"scope":"function"}}"""

  private fun securityReportJson(source: String) =
      """{"schema_version":"1","project_id":"project","project_revision":"revision","path":"main.go","content_hash":"base","status":"completed_empty","source":"$source","findings":[],"context_policy_version":"policy","generated_at":"2026-09-08T00:00:00Z"}"""

  private fun projectJson() =
      "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"name\":\"project\",\"path\":\"/tmp/project\",\"type\":\"go\",\"file_count\":0,\"source_file_count\":0,\"total_lines\":0,\"summary\":\"\",\"ai_status\":\"missing\",\"analyzed_at\":\"\"}"

  private fun indexJson() = "{\"project_id\":\"project\",\"project_revision\":\"revision\"}"

  private fun fileJson(path: String, hash: String) =
      "{\"path\":\"$path\",\"content_hash\":\"$hash\",\"name\":\"$path\",\"language\":\"Go\",\"size_bytes\":1,\"line_count\":1,\"modified_at\":\"\",\"binary\":false,\"content\":\"package main\"}"

  private fun symbolsJson(path: String, symbol: String = "") =
      if (symbol.isBlank())
          """{"project_id":"project","project_revision":"revision","path":"$path","symbols":[]}"""
      else
          """{"project_id":"project","project_revision":"revision","path":"$path","symbols":[{"name":"$symbol","kind":"function","confidence":"exact","atomic_target":true}]}"""

  private fun eventually(condition: () -> Boolean) {
    val deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(2)
    while (System.nanoTime() < deadline) {
      if (condition()) return
      Thread.sleep(10)
    }
    assertTrue(condition(), "condition did not become true")
  }
}
