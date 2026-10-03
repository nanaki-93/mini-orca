package io.miniorca.desktop

import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.input.key.Key
import java.net.http.HttpTimeoutException
import java.util.concurrent.ConcurrentLinkedQueue
import java.util.concurrent.CountDownLatch
import java.util.concurrent.Executors
import java.util.concurrent.TimeUnit
import kotlin.coroutines.CoroutineContext
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Job
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
                fixture.hasText(performanceBenchmarkStatusPresentation(null, null, null).summary))
            assertTrue(fixture.hasText("Refresh clears the benchmark selection"))
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
  fun renderedCatalogPreservesDaemonOrderAndLocalSelectionWithoutPrivilegedRequests() {
    Harness().use { harness ->
      val choices =
          listOf(choice.copy(name = "BenchmarkZ"), choice, choice.copy(name = "BenchmarkA"))
      harness.response =
          TransportResponse(200, Json.encodeToString(catalog.copy(benchmarks = choices)))
      harness.workflow.loadGoBenchmarks()
      harness.completeRequest()
      var snapshot by mutableStateOf(harness.state)
      val page = performancePageFixture()
      val browser = newResultBrowserState(page)
      ComposeVisualFixture(1600, 1000, frameDurationNanos = 16_000_000) {
            val evidence = snapshot.review.benchmark
            PerformanceWorkspacePane(
                PerformanceWorkspacePaneState(
                    page,
                    null,
                    browser = browser,
                    benchmarkCatalog = evidence.catalog,
                    selectedBenchmark = evidence.selected,
                    benchmarkDiscovery = evidence.discovery,
                    benchmarkAdmission = evidence.admission,
                    benchmarkEligibility = benchmarkEligibility(snapshot)),
                PerformanceWorkspaceActions(
                    {},
                    {},
                    FindingActions({}, { _, _ -> }, {}),
                    openSource = {},
                    loadBenchmarks = harness.workflow::loadGoBenchmarks,
                    selectBenchmark = {
                      harness.workflow.selectGoBenchmark(it)
                      snapshot = harness.state
                    },
                    runBenchmark = harness.workflow::compareSelectedGoBenchmark))
          }
          .use { fixture ->
            fixture.render()
            fixture.clickDescription("Expand Explore benchmark evidence")
            fixture.render()
            assertTrue(fixture.hasText("Select one listed benchmark before comparing."))
            assertNull(snapshot.review.benchmark.selected)
            val positions =
                choices.map { fixture.firstVisibleTextBounds("Select · ${it.name}").top }
            assertEquals(
                positions.sorted(),
                positions,
                "Choices retain daemon order rather than sorting by name")
            assertTrue(fixture.requestFocus("Refresh compatible benchmarks"))
            tabToBenchmarkControl(fixture, "benchmark-choice-0")
            assertNull(snapshot.review.benchmark.selected)
            tabToBenchmarkControl(fixture, "benchmark-choice-1")
            assertNull(snapshot.review.benchmark.selected)
            assertTrue(fixture.pressKey(Key.Spacebar))
            assertEquals(choice, snapshot.review.benchmark.selected)
            fixture.render()
            assertTrue(fixture.hasText("Selected · ${choice.name}"))
            assertTrue(fixture.hasText("Select · BenchmarkZ"))
            assertTrue(fixture.hasText("Select · BenchmarkA"))
            assertEquals(choice, snapshot.review.benchmark.selected)
            for (text in
                listOf(
                    performanceBenchmarkArgv(choice.command).lines().first(),
                    performanceBenchmarkArgv(choice.command).lines().last(),
                    "Opaque scope guard (identity metadata): ${choice.scope}")) {
              fixture.revealTextFullyWithin(text, "result-overview")
              assertTrue(fixture.copyTextByDragging(text, expectedText = text).isNotEmpty())
            }
            fixture.resize(800, 650)
            fixture.render()
            assertTrue(fixture.requestDescriptionFocus("Selected benchmark argv"))
            fixture.render()
            tabToBenchmarkControl(fixture, "benchmark-run")
            assertTrue(fixture.requestDescriptionFocus("Collapse Explore benchmark evidence"))
            assertTrue(fixture.pressKey(Key.Enter))
            fixture.render()
            harness.completeRequest()
            assertEquals(
                listOf("GET"),
                harness.methods,
                "Disclosure, copying and local selection cannot trust, compare, contact providers or write source")
          }
    }
  }

  @Test
  fun keyboardRecoveryRetriesOnlyReadOnlyDiscoveryAndNeverAdmitsExecution() {
    for (activation in listOf(Key.Enter, Key.Spacebar)) {
      Harness().use { harness ->
        harness.transportFailure = HttpTimeoutException("Lookup timed out; retry explicitly.")
        harness.workflow.loadGoBenchmarks()
        harness.completeRequest()
        harness.transportFailure = null
        var snapshot by mutableStateOf(harness.state)
        var privileged = 0
        val page = performancePageFixture()
        val browser = newResultBrowserState(page)
        browser.choose(performanceResults(page).single().row().key)
        ComposeVisualFixture(800, 650, 1.5f, frameDurationNanos = 16_000_000) {
              val evidence = snapshot.review.benchmark
              PerformanceWorkspacePane(
                  PerformanceWorkspacePaneState(
                      page,
                      null,
                      browser = browser,
                      benchmarkDiscovery = evidence.discovery,
                      benchmarkEligibility = benchmarkEligibility(snapshot)),
                  PerformanceWorkspaceActions(
                      { privileged++ },
                      {},
                      FindingActions({ privileged++ }, { _, _ -> privileged++ }, {}),
                      {},
                      loadBenchmarks = {
                        harness.workflow.loadGoBenchmarks()
                        snapshot = harness.state
                      },
                      selectBenchmark = harness.workflow::selectGoBenchmark,
                      runBenchmark = harness.workflow::compareSelectedGoBenchmark))
            }
            .use { fixture ->
              fixture.render()
              assertTrue(fixture.requestDescriptionFocus("Expand Explore benchmark evidence"))
              assertTrue(fixture.pressKey(activation))
              fixture.render()
              assertTrue(fixture.hasText("Lookup timed out; retry explicitly."))
              tabToBenchmarkControl(fixture, "benchmark-discovery")
              assertEquals(listOf("GET"), harness.methods, "Focus is not discovery")
              fixture.render("f21-keyboard-recovery-$activation")
              assertTrue(fixture.pressKey(activation))
              assertEquals(BenchmarkDiscoveryOutcome.Loading, snapshot.review.benchmark.discovery)
              fixture.render("f21-keyboard-recovery-loading-$activation")
              assertTrue(fixture.hasText("Listing · read-only discovery"))
              assertTrue(fixture.isDisabled("Refresh compatible benchmarks"))
              harness.completeRequest()
              assertEquals(listOf("GET", "GET"), harness.methods)
              assertTrue(harness.requests.all { it.second.contains("/benchmarks?") })
              assertNull(harness.state.review.benchmark.selected)
              assertEquals(0, privileged)
              assertTrue(harness.events.none { it == DesktopEvent.GoBenchmarkComparisonStarted })
            }
      }
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
  fun duplicateLookupIsSuppressedEvenWhenTheResponseIsReadyToPublish() {
    Harness().use { harness ->
      harness.workflow.loadGoBenchmarks()
      assertEquals(BenchmarkDiscoveryOutcome.Loading, harness.state.review.benchmark.discovery)
      assertFalse(benchmarkEligibility(harness.state).canDiscover)
      assertTrue(harness.methods.isEmpty(), "Loading must precede scheduling the GET")
      harness.workflow.loadGoBenchmarks()
      harness.main.runPending()
      harness.io.runPending()
      harness.response = TransportResponse(200, Json.encodeToString(catalog.copy(reason = "newer")))
      harness.workflow.loadGoBenchmarks()
      harness.completeRequest()
      assertEquals(catalog, harness.state.review.benchmark.catalog)
      assertEquals(listOf("GET"), harness.methods)
      assertEquals(1, harness.events.count { it == DesktopEvent.GoBenchmarkDiscoveryStarted })
      assertEquals(1, harness.events.filterIsInstance<DesktopEvent.GoBenchmarkCatalogLoaded>().size)
    }
  }

  @Test
  fun refreshImmediatelyRevokesSelectionAndAdmissionButRetainsMeasurements() {
    Harness().use { harness ->
      harness.selectBenchmark()
      harness.dispatch(DesktopEvent.GoBenchmarkComparisonLoaded(comparison))
      harness.workflow.compareSelectedGoBenchmark()
      harness.workflow.loadGoBenchmarks()
      assertEquals(BenchmarkDiscoveryOutcome.Loading, harness.state.review.benchmark.discovery)
      assertEquals(BenchmarkAdmissionOutcome.Stopped, harness.state.review.benchmark.admission)
      assertNull(harness.state.review.benchmark.catalog)
      assertNull(harness.state.review.benchmark.selected)
      assertEquals(comparison, harness.state.review.benchmark.comparison)
      assertFalse(benchmarkEligibility(harness.state).canCompare)
      harness.workflow.compareSelectedGoBenchmark()
      harness.workflow.loadGoBenchmarks()
      harness.completeRequest()
      assertEquals(listOf("GET"), harness.methods)
      assertEquals(catalog, harness.state.review.benchmark.catalog)
      assertNull(harness.state.review.benchmark.selected)
      assertEquals(comparison, harness.state.review.benchmark.comparison)
      assertEquals(BenchmarkAdmissionOutcome.Idle, harness.state.review.benchmark.admission)
    }
  }

  @Test
  fun invalidationAllowsReplacementLookupForTheSameCandidateAndRejectsLatePublication() {
    Harness().use { harness ->
      harness.workflow.loadGoBenchmarks()
      harness.main.runPending()
      harness.io.runPending()
      harness.workflow.invalidate()
      assertEquals(BenchmarkDiscoveryOutcome.Invalidated, harness.state.review.benchmark.discovery)
      assertTrue(benchmarkEligibility(harness.state).canDiscover)
      val replacement = catalog.copy(benchmarks = listOf(choice.copy(name = "BenchmarkNew")))
      harness.response = TransportResponse(200, Json.encodeToString(replacement))
      harness.workflow.loadGoBenchmarks()
      harness.completeRequest()
      assertEquals(replacement, harness.state.review.benchmark.catalog)
      assertEquals(listOf("GET", "GET"), harness.methods)
      assertEquals(1, harness.events.filterIsInstance<DesktopEvent.GoBenchmarkCatalogLoaded>().size)
      assertNull(harness.state.review.benchmark.selected)
    }
  }

  @Test
  fun successfulCatalogPreservesDaemonOrderWithoutSelectingAndEmptySuccessCanRefresh() {
    Harness().use { harness ->
      val choices =
          listOf(choice.copy(name = "BenchmarkZ"), choice, choice.copy(name = "BenchmarkA"))
      harness.response =
          TransportResponse(200, Json.encodeToString(catalog.copy(benchmarks = choices)))
      harness.workflow.loadGoBenchmarks()
      harness.completeRequest()
      assertEquals(choices, harness.state.review.benchmark.catalog!!.benchmarks)
      assertNull(harness.state.review.benchmark.selected)
      harness.response =
          TransportResponse(200, Json.encodeToString(catalog.copy(benchmarks = emptyList())))
      harness.workflow.loadGoBenchmarks()
      harness.completeRequest()
      assertEquals(BenchmarkDiscoveryOutcome.Loaded, harness.state.review.benchmark.discovery)
      assertTrue(harness.state.review.benchmark.catalog!!.benchmarks.isEmpty())
      assertTrue(benchmarkEligibility(harness.state).canDiscover)
      assertFalse(benchmarkEligibility(harness.state).canCompare)
      harness.workflow.compareSelectedGoBenchmark()
      harness.completeRequest()
      assertEquals(listOf("GET", "GET"), harness.methods)
      harness.response = TransportResponse(200, Json.encodeToString(catalog))
      harness.workflow.loadGoBenchmarks()
      harness.completeRequest()
      assertEquals(catalog, harness.state.review.benchmark.catalog)
      assertNull(harness.state.review.benchmark.selected)
    }
  }

  @Test
  fun sparseUnavailableCatalogPublishesReasonButNeverExecutableChoices() {
    Harness().use { harness ->
      harness.dispatch(DesktopEvent.GoBenchmarkComparisonLoaded(comparison))
      harness.response =
          TransportResponse(
              200,
              Json.encodeToString(
                  GoBenchmarkCatalog(reason = "Candidate is invalid", benchmarks = listOf(choice))))
      harness.workflow.loadGoBenchmarks()
      harness.completeRequest()
      assertEquals(
          BenchmarkDiscoveryOutcome.Unavailable("Candidate is invalid"),
          harness.state.review.benchmark.discovery)
      assertEquals(
          GoBenchmarkCatalog(reason = "Candidate is invalid"),
          harness.state.review.benchmark.catalog)
      assertEquals(comparison, harness.state.review.benchmark.comparison)
      assertTrue(benchmarkEligibility(harness.state).canDiscover)
      assertFalse(benchmarkEligibility(harness.state).canCompare)
      harness.workflow.selectGoBenchmark(choice)
      harness.workflow.compareSelectedGoBenchmark()
      harness.completeRequest()
      assertNull(harness.state.review.benchmark.selected)
      assertEquals(listOf("GET"), harness.methods)
    }
  }

  @Test
  fun lateSparseUnavailabilityCannotOverwriteReplacementDiscovery() {
    Harness().use { harness ->
      harness.response =
          TransportResponse(
              200, Json.encodeToString(GoBenchmarkCatalog(reason = "Old candidate is invalid")))
      harness.workflow.loadGoBenchmarks()
      harness.main.runPending()
      harness.io.runPending()
      harness.workflow.invalidate()
      harness.response = TransportResponse(200, Json.encodeToString(catalog))
      harness.workflow.loadGoBenchmarks()
      harness.completeRequest()
      assertEquals(BenchmarkDiscoveryOutcome.Loaded, harness.state.review.benchmark.discovery)
      assertEquals(catalog, harness.state.review.benchmark.catalog)
      assertEquals(1, harness.events.filterIsInstance<DesktopEvent.GoBenchmarkCatalogLoaded>().size)
    }
  }

  @Test
  fun everyAvailableCatalogIdentityMismatchEndsLoadingWithRetryableLocalFailure() {
    listOf(
            catalog.copy(draftId = "other"),
            catalog.copy(draftRevision = 2),
            catalog.copy(draftHash = "other"),
            catalog.copy(projectId = "other"),
            catalog.copy(projectRevision = "other"),
            catalog.copy(baseFileHash = "other"),
            catalog.copy(targetPath = "other.go"),
            GoBenchmarkCatalog(available = true, benchmarks = listOf(choice)),
        )
        .forEach { foreign ->
          Harness().use { harness ->
            harness.response = TransportResponse(200, Json.encodeToString(foreign))
            harness.workflow.loadGoBenchmarks()
            harness.completeRequest()
            val outcome = harness.state.review.benchmark.discovery
            assertTrue(outcome is BenchmarkDiscoveryOutcome.Failed, foreign.toString())
            assertTrue(outcome.message.contains("does not match the current candidate"))
            assertTrue(benchmarkEligibility(harness.state).canDiscover)
            assertFalse(benchmarkEligibility(harness.state).canCompare)
            assertNull(harness.state.review.benchmark.catalog)
            assertNull(harness.state.review.benchmark.selected)
            assertNull(harness.state.jobs.error, "Lookup failures stay benchmark-local")
            harness.response = TransportResponse(200, Json.encodeToString(catalog))
            harness.workflow.loadGoBenchmarks()
            harness.completeRequest()
            assertEquals(BenchmarkDiscoveryOutcome.Loaded, harness.state.review.benchmark.discovery)
            assertEquals(listOf("GET", "GET"), harness.methods)
          }
        }
  }

  @Test
  fun lookupTimeoutIsLocalRetainsPriorMeasurementsAndRequiresExplicitRetry() {
    Harness().use { harness ->
      harness.selectBenchmark()
      harness.dispatch(DesktopEvent.GoBenchmarkComparisonLoaded(comparison))
      harness.transportFailure = HttpTimeoutException("Benchmark lookup timed out")
      harness.workflow.loadGoBenchmarks()
      harness.completeRequest()
      assertEquals(
          BenchmarkDiscoveryOutcome.Failed("Benchmark lookup timed out"),
          harness.state.review.benchmark.discovery)
      assertEquals(comparison, harness.state.review.benchmark.comparison)
      assertNull(harness.state.review.benchmark.catalog)
      assertNull(harness.state.review.benchmark.selected)
      assertNull(harness.state.jobs.error)
      assertTrue(benchmarkEligibility(harness.state).canDiscover)
      harness.completeRequest()
      assertEquals(listOf("GET"), harness.methods, "Timeout must not automatically retry")
      harness.transportFailure = null
      harness.workflow.loadGoBenchmarks()
      harness.completeRequest()
      assertEquals(BenchmarkDiscoveryOutcome.Loaded, harness.state.review.benchmark.discovery)
      assertEquals(listOf("GET", "GET"), harness.methods)
    }
  }

  @Test
  fun lookupConflictMarksDraftStaleAndKeepsALocalExplanationWithoutCatalogAuthority() {
    Harness().use { harness ->
      harness.selectBenchmark()
      harness.response = TransportResponse(409, """{"message":"Candidate changed"}""")
      harness.workflow.loadGoBenchmarks()
      harness.completeRequest()
      assertEquals(DraftEditorStatus.Stale, harness.state.review.editor!!.status)
      assertEquals(
          BenchmarkDiscoveryOutcome.Failed("Candidate changed"),
          harness.state.review.benchmark.discovery)
      assertNull(harness.state.review.benchmark.catalog)
      assertNull(harness.state.review.benchmark.selected)
      assertFalse(benchmarkEligibility(harness.state).canDiscover)
      assertTrue(harness.events.contains(DesktopEvent.DraftMarkedStale))
      assertEquals(listOf("GET"), harness.methods)
    }
  }

  @Test
  fun lookupCancellationPropagatesWithoutFabricatingFailureAndAllowsExplicitRetry() {
    Harness().use { harness ->
      val cancellation = CancellationException("Lookup cancelled")
      harness.transportFailure = cancellation
      harness.workflow.loadGoBenchmarks()
      harness.main.runPending()
      val request = harness.scope.coroutineContext[Job]!!.children.single()
      var completion: Throwable? = null
      request.invokeOnCompletion { completion = it }
      harness.io.runPending()
      harness.main.runPending()
      assertTrue(request.isCancelled)
      assertTrue(completion is CancellationException)
      assertEquals(cancellation.message, completion?.message)
      assertEquals(BenchmarkDiscoveryOutcome.Invalidated, harness.state.review.benchmark.discovery)
      assertNull(harness.state.review.benchmark.catalog)
      assertNull(harness.state.jobs.error)
      assertTrue(harness.events.none { it is DesktopEvent.GoBenchmarkDiscoveryFailed })
      assertTrue(benchmarkEligibility(harness.state).canDiscover)
      harness.transportFailure = null
      harness.workflow.loadGoBenchmarks()
      harness.completeRequest()
      assertEquals(BenchmarkDiscoveryOutcome.Loaded, harness.state.review.benchmark.discovery)
    }
  }

  @Test
  fun discoverySendsOnlyTheGuardedGetWithoutTrustOrExecution() {
    Harness().use { harness ->
      harness.workflow.loadGoBenchmarks()
      harness.completeRequest()
      assertEquals(
          listOf(
              Triple<String, String, String?>(
                  "GET",
                  "/api/projects/current/drafts/draft/benchmarks?project_revision=revision&expected_revision=1&expected_hash=draft-hash",
                  null)),
          harness.requests)
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
      assertTrue(harness.events.none { it is DesktopEvent.GoBenchmarkCatalogLoaded })
      assertEquals(BenchmarkDiscoveryOutcome.Invalidated, harness.state.review.benchmark.discovery)
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
      assertTrue(harness.admissionFailure().contains("trust scope changed"))
      assertNull(harness.state.jobs.error)
      assertNull(harness.state.review.benchmark.catalog)
      assertNull(harness.state.review.benchmark.selected)
      assertEquals(BenchmarkDiscoveryOutcome.Invalidated, harness.state.review.benchmark.discovery)
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
      assertEquals(
          BenchmarkComparisonOutcome(comparison), harness.state.review.benchmark.latestOutcome)
      harness.response = TransportResponse(500, """{"message":"Comparison failed"}""")
      harness.workflow.compareSelectedGoBenchmark()
      harness.completeRequest()
      assertEquals(
          BenchmarkAdmissionOutcome.Failed(
              "Comparison failed Execution may have started; no new measurements were confirmed."),
          harness.state.review.benchmark.admission)
      assertFalse(harness.state.review.benchmark.running)
      assertEquals(comparison, harness.state.review.benchmark.comparison)
      assertNull(
          harness.state.review.benchmark.latestOutcome, "Transport failure is not a daemon outcome")
      harness.completeRequest()
      assertEquals(listOf("POST", "POST"), harness.methods)
    }
  }

  @Test
  fun daemonTerminalOutcomesRemainDistinctAfterTrustedAndStagedAdmission() {
    val statuses =
        listOf(
            "completed" to BenchmarkComparisonStatus.Completed,
            "canceled" to BenchmarkComparisonStatus.Canceled,
            "failed" to BenchmarkComparisonStatus.Failed,
            "unavailable" to BenchmarkComparisonStatus.Unavailable,
            "future-status" to BenchmarkComparisonStatus.Unsupported)
    for (trusted in listOf(false, true)) {
      for ((status, expected) in statuses) {
        for (reason in listOf("", "Recorded daemon reason for $status")) {
          Harness().use { harness ->
            harness.selectBenchmark(trusted)
            harness.dispatch(DesktopEvent.GoBenchmarkComparisonLoaded(comparison))
            val terminal =
                comparison.copy(status = status, reason = reason, base = null, candidate = null)
            if (!trusted) {
              harness.enqueue(trust.copy(trusted = false))
              harness.enqueue(trust)
            }
            harness.enqueue(terminal)
            harness.workflow.compareSelectedGoBenchmark()
            assertNull(harness.state.review.benchmark.latestOutcome)
            harness.completeRequest()
            val evidence = harness.state.review.benchmark
            assertEquals(expected, evidence.latestOutcome?.status)
            assertEquals(terminal, evidence.latestOutcome?.response)
            assertEquals(
                comparison, evidence.comparison, "No measurements must retain prior evidence")
            assertEquals(BenchmarkAdmissionOutcome.Idle, evidence.admission)
            assertFalse(evidence.running)
            assertFalse(harness.state.loading)
            assertNull(harness.state.jobs.error)
            assertTrue(harness.events.none { it is DesktopEvent.GoBenchmarkComparisonFailed })
            val message = harness.events.filterIsInstance<DesktopEvent.Status>().last().message
            if (status != "completed" && reason.isNotBlank()) assertTrue(message.contains(reason))
            if (expected == BenchmarkComparisonStatus.Unsupported) {
              assertTrue(message.contains("Unsupported benchmark status future-status"))
              assertTrue(message.contains("no successful comparison is confirmed"))
            }
            if (expected == BenchmarkComparisonStatus.Unavailable) {
              assertNull(evidence.catalog)
              assertNull(evidence.selected)
              assertFalse(benchmarkEligibility(harness.state).canCompare)
              assertTrue(message.contains("Refresh compatible benchmarks and select again."))
            } else {
              assertEquals(choice, evidence.selected)
            }
            harness.completeRequest()
            assertEquals(
                if (trusted) listOf("POST") else listOf("GET", "POST", "POST"), harness.methods)
          }
        }
      }
    }
  }

  @Test
  fun sparseUnavailableExplainsCurrentAttemptWithoutFillingIdentityOrReplacingMeasurements() {
    val responses =
        listOf(
            GoBenchmarkComparison(status = "unavailable", reason = "Candidate is invalid"),
            comparison.copy(
                status = "unavailable",
                reason = "Workspace scope changed",
                projectId = "",
                baseFileHash = "",
                scope = "recomputed-scope",
                base = null,
                candidate = null))
    for (prior in listOf(null, comparison)) {
      for (terminal in responses) {
        Harness().use { harness ->
          harness.selectBenchmark()
          if (prior != null) harness.dispatch(DesktopEvent.GoBenchmarkComparisonLoaded(prior))
          harness.enqueue(terminal)
          harness.workflow.compareSelectedGoBenchmark()
          harness.completeRequest()
          val evidence = harness.state.review.benchmark
          assertEquals(BenchmarkComparisonStatus.Unavailable, evidence.latestOutcome?.status)
          assertEquals(terminal, evidence.latestOutcome?.response, "Recorded metadata stays sparse")
          assertEquals(prior, evidence.comparison)
          assertEquals(BenchmarkAdmissionOutcome.Idle, evidence.admission)
          assertEquals(BenchmarkDiscoveryOutcome.Invalidated, evidence.discovery)
          assertNull(evidence.catalog)
          assertNull(evidence.selected)
          assertFalse(benchmarkEligibility(harness.state).canCompare)
          assertTrue(harness.events.none { it is DesktopEvent.GoBenchmarkComparisonFailed })
          assertTrue(
              harness.events
                  .filterIsInstance<DesktopEvent.Status>()
                  .last()
                  .message
                  .contains(terminal.reason))
          harness.workflow.compareSelectedGoBenchmark()
          harness.completeRequest()
          assertEquals(
              listOf("POST"), harness.methods, "Recovery cannot renew trust or retry implicitly")
          assertEquals(terminal, harness.state.review.benchmark.latestOutcome?.response)
        }
      }
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

  @Test
  fun missingInvalidAndMismatchedCandidatesBlockExplicitActionsWithLocalReasons() {
    val valid = candidateState()
    val cases = mutableListOf<Pair<String, DesktopState>>()
    cases += "project" to valid.copy(projectState = ProjectWorkspaceState())
    cases += "file" to valid.copy(selection = FileSelectionState())
    cases += "draft" to valid.copy(review = DraftReviewState())
    cases += "editor" to valid.copy(review = valid.review.copy(editor = null))
    DraftEditorStatus.entries
        .filter { it != DraftEditorStatus.Valid }
        .forEach { status ->
          cases +=
              status.name to
                  valid.copy(
                      review =
                          valid.review.copy(editor = valid.review.editor!!.copy(status = status)))
        }
    val draftChanges =
        listOf(
            draft().copy(id = ""),
            draft().copy(revision = 0),
            draft().copy(hash = " "),
            draft().copy(projectId = ""),
            draft().copy(projectRevision = ""),
            draft().copy(targetPath = ""),
            draft().copy(baseFileHash = ""),
            draft().copy(projectId = "other"),
            draft().copy(projectRevision = "other"),
            draft().copy(targetPath = "other.go"),
            draft().copy(baseFileHash = "other"),
            draft().copy(validation = null),
            draft().copy(validation = draft().validation!!.copy(applicable = false)))
    draftChanges.forEachIndexed { index, changed ->
      cases +=
          "draft prerequisite $index" to
              valid.copy(
                  review = valid.review.copy(draft = changed, editor = editableDraft(changed)))
    }
    listOf(
            draft().copy(id = "other"),
            draft().copy(revision = 2),
            draft().copy(hash = "other"),
            draft().copy(projectId = "other"),
            draft().copy(projectRevision = "other"),
            draft().copy(targetPath = "other.go"),
            draft().copy(baseFileHash = "other"))
        .forEachIndexed { index, changed ->
          cases +=
              "editor identity $index" to
                  valid.copy(review = valid.review.copy(editor = editableDraft(changed)))
        }
    cases +=
        "unreported declaration edit" to
            valid.copy(
                review =
                    valid.review.copy(
                        editor = valid.review.editor!!.copy(declaration = "func Other() {}")))
    cases +=
        "unreported import edit" to
            valid.copy(
                review =
                    valid.review.copy(editor = valid.review.editor!!.copy(imports = listOf("fmt"))))
    cases.forEach { (label, snapshot) ->
      Harness(snapshot).use { harness ->
        val decision = benchmarkEligibility(harness.state)
        assertFalse(decision.canDiscover, label)
        assertFalse(decision.canCompare, label)
        assertTrue(harness.events.isEmpty(), "Eligibility must be side-effect-free: $label")
        harness.workflow.loadGoBenchmarks()
        assertEquals(
            BenchmarkDiscoveryOutcome.Failed(decision.discoveryBlockedReason!!),
            harness.state.review.benchmark.discovery,
            label)
        harness.workflow.compareSelectedGoBenchmark()
        assertEquals(
            BenchmarkAdmissionOutcome.Failed(decision.comparisonBlockedReason!!),
            harness.state.review.benchmark.admission,
            label)
        harness.completeRequest()
        assertTrue(harness.methods.isEmpty(), label)
      }
    }
  }

  @Test
  fun comparisonRequiresLoadedAvailableExactCatalogIdentityAndExecutableSelection() {
    val valid = candidateState()
    val evidence =
        BenchmarkEvidenceState(
            catalog = catalog, selected = choice, discovery = BenchmarkDiscoveryOutcome.Loaded)
    val foreignCatalogs =
        listOf(
            catalog.copy(draftId = "other"),
            catalog.copy(draftRevision = 2),
            catalog.copy(draftHash = "other"),
            catalog.copy(projectId = "other"),
            catalog.copy(projectRevision = "other"),
            catalog.copy(baseFileHash = "other"),
            catalog.copy(targetPath = "other.go"))
    val cases = mutableListOf<Pair<String, BenchmarkEvidenceState>>()
    foreignCatalogs.forEachIndexed { index, foreign ->
      cases += "catalog identity $index" to evidence.copy(catalog = foreign)
    }
    listOf(
            BenchmarkDiscoveryOutcome.NotRequested,
            BenchmarkDiscoveryOutcome.Loading,
            BenchmarkDiscoveryOutcome.Invalidated,
            BenchmarkDiscoveryOutcome.Failed("lookup"),
            BenchmarkDiscoveryOutcome.Unavailable("unavailable"))
        .forEach { outcome -> cases += "discovery $outcome" to evidence.copy(discovery = outcome) }
    cases += "missing catalog" to evidence.copy(catalog = null)
    cases += "unavailable catalog" to evidence.copy(catalog = catalog.copy(available = false))
    cases += "empty catalog" to evidence.copy(catalog = catalog.copy(benchmarks = emptyList()))
    cases += "no explicit selection" to evidence.copy(selected = null)
    cases += "same name different scope" to evidence.copy(selected = choice.copy(scope = "other"))
    cases +=
        "same name different argv" to
            evidence.copy(selected = choice.copy(command = listOf("other")))
    listOf(choice.copy(name = " "), choice.copy(scope = " "), choice.copy(command = emptyList()))
        .forEachIndexed { index, incomplete ->
          cases +=
              "incomplete executable $index" to
                  evidence.copy(
                      catalog = catalog.copy(benchmarks = listOf(incomplete)),
                      selected = incomplete)
        }
    cases.forEach { (label, benchmark) ->
      Harness(valid.copy(review = valid.review.copy(benchmark = benchmark))).use { harness ->
        val decision = benchmarkEligibility(harness.state)
        assertEquals(
            benchmark.discovery != BenchmarkDiscoveryOutcome.Loading, decision.canDiscover, label)
        assertFalse(decision.canCompare, label)
        assertTrue(decision.comparisonBlockedReason!!.isNotBlank(), label)
        harness.workflow.compareSelectedGoBenchmark()
        harness.completeRequest()
        assertTrue(harness.methods.isEmpty(), label)
        assertEquals(
            BenchmarkAdmissionOutcome.Failed(decision.comparisonBlockedReason),
            harness.state.review.benchmark.admission,
            label)
      }
    }
  }

  @Test
  fun exactChoiceAuthorizesComparisonWithoutChangingMeasurementIdentity() {
    Harness().use { harness ->
      harness.selectBenchmark()
      val decision = benchmarkEligibility(harness.state)
      assertTrue(decision.canDiscover)
      assertTrue(decision.canCompare)
      assertEquals(
          goBenchmarkComparisonIdentity(draft()),
          (decision.candidate as? BenchmarkCandidateDecision.Ready)
              ?.draft
              ?.let(::goBenchmarkComparisonIdentity))
      harness.response = TransportResponse(200, Json.encodeToString(comparison))
      harness.workflow.compareSelectedGoBenchmark()
      harness.completeRequest()
      assertEquals(listOf("POST"), harness.methods)
      assertEquals(comparison, harness.state.review.benchmark.comparison)
    }
  }

  @Test
  fun changedOpenFileRejectsCatalogAndComparisonPublicationWithoutLifecycleCancellation() {
    Harness().use { harness ->
      harness.workflow.loadGoBenchmarks()
      harness.main.runPending()
      harness.io.runPending()
      harness.controller.dispatch(
          DesktopEvent.FileLoaded(file().copy(contentHash = "other"), emptyList()))
      harness.main.runPending()
      assertNull(harness.state.review.benchmark.catalog)
    }
    Harness().use { harness ->
      harness.selectBenchmark()
      harness.response = TransportResponse(200, Json.encodeToString(comparison))
      harness.workflow.compareSelectedGoBenchmark()
      harness.main.runPending()
      harness.io.runPending()
      harness.controller.dispatch(
          DesktopEvent.FileLoaded(file().copy(path = "other.go"), emptyList()))
      harness.main.runPending()
      assertNull(harness.state.review.benchmark.comparison)
    }
  }

  @Test
  fun untrustedAdmissionAwaitsEachValidatedStageAndSuppressesDuplicateActivation() {
    Harness().use { harness ->
      harness.selectBenchmark(trusted = false)
      harness.enqueueAdmission()
      harness.dispatch(DesktopEvent.GoBenchmarkComparisonLoaded(comparison))
      harness.workflow.compareSelectedGoBenchmark()
      assertEquals(BenchmarkAdmissionOutcome.Admitting, harness.state.review.benchmark.admission)
      assertEquals(comparison, harness.state.review.benchmark.comparison)
      assertTrue(harness.requests.isEmpty())
      harness.workflow.compareSelectedGoBenchmark()
      harness.runStage()
      assertEquals(listOf("GET"), harness.methods)
      assertEquals(BenchmarkAdmissionOutcome.Admitting, harness.state.review.benchmark.admission)
      harness.workflow.compareSelectedGoBenchmark()
      harness.runStage()
      assertEquals(listOf("GET", "POST"), harness.methods)
      assertEquals(BenchmarkAdmissionOutcome.Running, harness.state.review.benchmark.admission)
      harness.workflow.compareSelectedGoBenchmark()
      harness.runStage()
      assertEquals(listOf("GET", "POST", "POST"), harness.methods)
      assertEquals(
          listOf(
              "/api/projects/current/execution-trust?project_revision=revision",
              "/api/projects/current/execution-trust",
              "/api/projects/current/drafts/draft/benchmarks"),
          harness.requests.map { it.second })
      assertEquals("""{"project_revision":"revision","confirm":true}""", harness.requests[1].third)
      assertEquals(BenchmarkAdmissionOutcome.Idle, harness.state.review.benchmark.admission)
      assertEquals(comparison, harness.state.review.benchmark.comparison)
      assertEquals(1, harness.events.count { it == DesktopEvent.GoBenchmarkAdmissionStarted })
      harness.completeRequest()
      assertEquals(3, harness.requests.size)
    }
  }

  @Test
  fun everyInvalidTrustResponseRevokesAuthorityWithoutReachingComparison() {
    val mismatches =
        listOf(
            trust.copy(projectId = ""),
            trust.copy(projectId = " "),
            trust.copy(projectId = "other"),
            trust.copy(projectRevision = ""),
            trust.copy(projectRevision = "other"),
            trust.copy(commands = emptyList()),
            trust.copy(commands = listOf(listOf("go", "test", "other"))),
            trust.copy(commands = trust.commands + listOf(listOf("go", "test", "."))))
    for (stage in 0..1) {
      val responses = if (stage == 1) mismatches + trust.copy(trusted = false) else mismatches
      for (invalid in responses) {
        Harness().use { harness ->
          harness.selectBenchmark(trusted = false)
          harness.dispatch(DesktopEvent.GoBenchmarkComparisonLoaded(comparison))
          if (stage == 1) harness.enqueue(trust.copy(trusted = false))
          harness.enqueue(invalid)
          harness.workflow.compareSelectedGoBenchmark()
          harness.completeRequest()
          assertEquals(stage + 1, harness.requests.size, "$stage: $invalid")
          assertTrue(harness.admissionFailure().contains("Refresh compatible benchmarks"))
          assertFalse(harness.state.review.benchmark.running)
          assertNull(harness.state.review.benchmark.catalog)
          assertNull(harness.state.review.benchmark.selected)
          assertEquals(comparison, harness.state.review.benchmark.comparison)
          assertNull(
              harness.state.review.benchmark.latestOutcome, "Trust failure is not a daemon outcome")
          assertNull(harness.state.jobs.error)
          harness.workflow.compareSelectedGoBenchmark()
          harness.completeRequest()
          assertEquals(stage + 1, harness.requests.size, "Rediscovery must be explicit")
        }
      }
    }
  }

  @Test
  fun untrustedCatalogStillValidatesAPostAcknowledgmentWhenTrustGetAlreadyReportsTrusted() {
    Harness().use { harness ->
      harness.selectBenchmark(trusted = false)
      harness.enqueue(trust)
      harness.enqueue(trust)
      harness.enqueue(comparison)
      harness.workflow.compareSelectedGoBenchmark()
      harness.completeRequest()
      assertEquals(listOf("GET", "POST", "POST"), harness.methods)
      assertEquals(comparison, harness.state.review.benchmark.comparison)
    }
  }

  @Test
  fun trustedCatalogRunsDirectlyOnceAndExpiredTrustNeverRenewsImplicitly() {
    Harness().use { harness ->
      harness.selectBenchmark()
      val unavailable =
          comparison.copy(
              status = "unavailable",
              reason = "Execution trust is required",
              base = null,
              candidate = null)
      harness.enqueue(unavailable)
      harness.workflow.compareSelectedGoBenchmark()
      harness.workflow.compareSelectedGoBenchmark()
      harness.completeRequest()
      assertEquals(listOf("POST"), harness.methods)
      assertTrue(harness.requests.single().second.endsWith("/drafts/draft/benchmarks"))
      assertEquals(
          BenchmarkComparisonStatus.Unavailable,
          harness.state.review.benchmark.latestOutcome?.status)
      assertEquals(unavailable, harness.state.review.benchmark.latestOutcome?.response)
      assertEquals(BenchmarkAdmissionOutcome.Idle, harness.state.review.benchmark.admission)
      assertTrue(
          harness.events
              .filterIsInstance<DesktopEvent.Status>()
              .last()
              .message
              .contains("Refresh compatible benchmarks"))
      assertNull(harness.state.review.benchmark.catalog)
      assertNull(harness.state.review.benchmark.selected)
      assertNull(harness.state.review.benchmark.comparison)
      harness.workflow.compareSelectedGoBenchmark()
      harness.completeRequest()
      assertEquals(listOf("POST"), harness.methods)
    }
  }

  @Test
  fun comparisonAssociationMismatchesRetainOnlyPriorEvidenceAndRequireRediscovery() {
    val mismatches =
        listOf(
            comparison.copy(draftId = "other"),
            comparison.copy(draftRevision = 2),
            comparison.copy(draftHash = "other"),
            comparison.copy(projectId = "other"),
            comparison.copy(projectRevision = "other"),
            comparison.copy(baseFileHash = "other"),
            comparison.copy(targetPath = "other.go"),
            comparison.copy(benchmark = "other"),
            comparison.copy(scope = "new-scope"),
            comparison.copy(command = emptyList()),
            comparison.copy(command = choice.command.reversed()),
            comparison.copy(command = choice.command + "-benchmem"))
    for (status in listOf("completed", "failed", "canceled", "unavailable")) {
      // Even empty measurement objects require strict association, not the sparse exception.
      for (sides in listOf("both", "base", "candidate", "empty")) {
        mismatches.forEach { mismatch ->
          val rejected =
              mismatch.copy(
                  status = status,
                  base =
                      when (sides) {
                        "candidate" -> null
                        "empty" -> GoBenchmarkMeasurement(emptyList())
                        else -> mismatch.base
                      },
                  candidate =
                      if (sides == "both" || sides == "candidate") mismatch.candidate else null)

          Harness().use { harness ->
            harness.selectBenchmark()
            harness.dispatch(DesktopEvent.GoBenchmarkComparisonLoaded(comparison))
            harness.enqueue(rejected)
            harness.workflow.compareSelectedGoBenchmark()
            harness.completeRequest()
            assertTrue(harness.admissionFailure().contains("Refresh compatible benchmarks"))
            assertTrue(harness.admissionFailure().contains("Benchmark response"))
            if (mismatch.command != choice.command)
                assertTrue(harness.admissionFailure().contains("argv"))
            assertNull(harness.state.jobs.error, "Association failures stay benchmark-local")
            assertNull(harness.state.review.benchmark.latestOutcome)
            assertEquals(comparison, harness.state.review.benchmark.comparison)
            assertNull(harness.state.review.benchmark.catalog)
            assertNull(harness.state.review.benchmark.selected)
            assertFalse(harness.state.review.benchmark.running)
            assertEquals(listOf("POST"), harness.methods)
            assertTrue(
                harness.events.none {
                  it is DesktopEvent.GoBenchmarkComparisonLoaded && it.comparison == rejected
                })
            harness.workflow.compareSelectedGoBenchmark()
            harness.completeRequest()
            assertEquals(listOf("POST"), harness.methods, "Rediscovery/selection must be explicit")
          }
        }
      }
    }
  }

  @Test
  fun completedStagesRecheckCandidateCatalogAndSelectionWithoutRelyingOnCancellation() {
    val changes = publicationInvalidations()
    for (stage in 0..2) {
      for (change in changes) {
        Harness().use { harness ->
          harness.selectBenchmark(trusted = false)
          harness.enqueueAdmission()
          harness.workflow.compareSelectedGoBenchmark()
          repeat(stage) { harness.runStage() }
          harness.main.runPending()
          harness.io.runPending() // Completed transport, publication still queued.
          harness.controller.dispatch(change) // Bypass workflow cancellation deliberately.
          if (change is DesktopEvent.GoBenchmarkCatalogLoaded)
              harness.controller.dispatch(DesktopEvent.GoBenchmarkSelected(choice))
          harness.completeRequest()
          assertEquals(stage + 1, harness.requests.size, "$stage: $change")
          assertTrue(harness.events.none { it is DesktopEvent.GoBenchmarkComparisonLoaded })
          assertTrue(harness.events.none { it is DesktopEvent.GoBenchmarkComparisonFailed })
          assertFalse(harness.state.review.benchmark.running, "$stage: $change")
        }
      }
    }
  }

  @Test
  fun nonCooperativeTerminalResponsesCannotPublishAfterCandidateOrAuthorityChanges() {
    val executor = Executors.newSingleThreadExecutor()
    try {
      val terminals =
          listOf("completed", "failed", "canceled", "unavailable").map {
            comparison.copy(status = it, reason = "Late $it")
          } + GoBenchmarkComparison(status = "unavailable", reason = "Late sparse unavailable")
      for (terminal in terminals) {
        for (change in publicationInvalidations()) {
          for (cancelThroughLifecycle in listOf(false, true)) {
            Harness().use { harness ->
              harness.selectBenchmark()
              harness.dispatch(DesktopEvent.GoBenchmarkComparisonLoaded(comparison))
              val entered = CountDownLatch(1)
              val interrupted = CountDownLatch(1)
              val release = CountDownLatch(1)
              harness.responses.addLast {
                entered.countDown()
                // Deliberately return a terminal response even after transport interruption.
                while (true) {
                  try {
                    release.await()
                    break
                  } catch (_: InterruptedException) {
                    interrupted.countDown()
                  }
                }
                TransportResponse(200, Json.encodeToString(terminal))
              }
              harness.workflow.compareSelectedGoBenchmark()
              harness.main.runPending()
              val job = harness.scope.coroutineContext[Job]!!.children.single()
              val transport = executor.submit { harness.io.runPending() }
              try {
                assertTrue(entered.await(5, TimeUnit.SECONDS), "Transport must start")
                if (cancelThroughLifecycle) harness.dispatch(change)
                else harness.controller.dispatch(change) // Exercise guards without cancellation.
                if (change is DesktopEvent.GoBenchmarkCatalogLoaded) {
                  if (cancelThroughLifecycle) harness.workflow.selectGoBenchmark(choice)
                  else harness.controller.dispatch(DesktopEvent.GoBenchmarkSelected(choice))
                }
                if (cancelThroughLifecycle) {
                  assertTrue(job.isCancelled, change.toString())
                  assertTrue(
                      interrupted.await(5, TimeUnit.SECONDS), "Transport ignores interruption")
                } else assertFalse(job.isCancelled)
                val replacement = harness.state
                val eventCount = harness.events.size
                release.countDown()
                transport.get(5, TimeUnit.SECONDS)
                harness.completeRequest()
                assertEquals(replacement, harness.state, "${terminal.status}: $change")
                assertEquals(eventCount, harness.events.size, "No obsolete outcome or diagnostic")
                assertEquals(listOf("POST"), harness.methods, "No trust grant, switch or retry")
                assertTrue(job.isCompleted)
              } finally {
                release.countDown()
                transport.get(5, TimeUnit.SECONDS)
              }
            }
          }
        }
      }
    } finally {
      executor.shutdownNow()
    }
  }

  @Test
  fun everyLateTerminalIsRejectedWhenCatalogAndChoiceReturnToTheCapturedValues() {
    for (status in listOf("completed", "failed", "canceled", "unavailable")) {
      Harness().use { harness ->
        harness.selectBenchmark()
        harness.enqueue(comparison.copy(status = status))
        harness.workflow.compareSelectedGoBenchmark()
        harness.main.runPending()
        harness.io.runPending()
        harness.dispatch(DesktopEvent.GoBenchmarkCatalogLoaded(catalog))
        harness.workflow.selectGoBenchmark(choice)
        val replacement = comparison.copy(reason = "Replacement evidence")
        harness.dispatch(DesktopEvent.GoBenchmarkComparisonLoaded(replacement))
        val snapshot = harness.state
        val eventCount = harness.events.size
        harness.completeRequest()
        assertEquals(snapshot, harness.state, status)
        assertEquals(eventCount, harness.events.size, status)
        assertEquals(listOf("POST"), harness.methods)
      }
    }
  }

  @Test
  fun lifecycleEventsCancelEveryAdmissionStageAndRevokeAuthorityImmediately() {
    val changes =
        listOf<DesktopEvent>(
            DesktopEvent.DraftValidationStarted(1),
            DesktopEvent.DraftEdited(declaration = "func Run() { println(1) }"),
            DesktopEvent.DraftEdited(imports = listOf("fmt")),
            DesktopEvent.DraftLoaded(draft().copy(validation = null)),
            DesktopEvent.DraftLoaded(draft()),
            DesktopEvent.DraftDiscarded,
            DesktopEvent.DraftMarkedStale,
            DesktopEvent.FileLoaded(file().copy(path = "other.go"), emptyList()),
            DesktopEvent.FileLoaded(file().copy(contentHash = "other"), emptyList()),
            DesktopEvent.SelectedFileRefreshed(file().copy(contentHash = "other"), emptyList()),
            DesktopEvent.SelectedFileUnavailable("File was removed"),
            DesktopEvent.ProjectLoaded(project("other"), ProjectIndex("other", "revision")),
            DesktopEvent.ProjectLoaded(project(revision = "next"), ProjectIndex("project", "next")),
            DesktopEvent.IndexRefreshed(ProjectIndex("project", "next")),
            DesktopEvent.Applied(ApplyResult("next", "after", true)))
    for (stage in 0..2) {
      for (completed in listOf(false, true)) {
        for (change in changes) {
          Harness().use { harness ->
            harness.selectBenchmark(trusted = false)
            harness.enqueueAdmission()
            harness.workflow.compareSelectedGoBenchmark()
            repeat(stage) { harness.runStage() }
            harness.main.runPending()
            val job = harness.scope.coroutineContext[Job]!!.children.single()
            if (completed) harness.io.runPending()
            harness.dispatch(change)
            assertTrue(job.isCancelled, "$stage/$completed: $change")
            assertFalse(harness.state.review.benchmark.running, "$stage/$completed: $change")
            assertNull(harness.state.review.benchmark.catalog)
            assertNull(harness.state.review.benchmark.selected)
            harness.completeRequest()
            assertEquals(stage + if (completed) 1 else 0, harness.requests.size)
            assertNull(harness.state.review.benchmark.comparison)
            assertTrue(harness.events.none { it is DesktopEvent.GoBenchmarkComparisonLoaded })
            assertTrue(harness.events.none { it is DesktopEvent.GoBenchmarkComparisonFailed })
          }
        }
      }
    }
  }

  @Test
  fun catalogReplacementCancelsPendingDiscoveryAndRejectsItsLateResponse() {
    for (completed in listOf(false, true)) {
      Harness().use { harness ->
        harness.workflow.loadGoBenchmarks()
        harness.main.runPending()
        val job = harness.scope.coroutineContext[Job]!!.children.single()
        if (completed) harness.io.runPending()
        val replacement = catalog.copy(reason = "Replacement catalog")
        harness.dispatch(DesktopEvent.GoBenchmarkCatalogLoaded(replacement))
        assertTrue(job.isCancelled)
        harness.completeRequest()
        assertEquals(replacement, harness.state.review.benchmark.catalog)
        assertEquals(BenchmarkDiscoveryOutcome.Loaded, harness.state.review.benchmark.discovery)
        assertEquals(
            1, harness.events.filterIsInstance<DesktopEvent.GoBenchmarkCatalogLoaded>().size)
        assertEquals(if (completed) 1 else 0, harness.requests.size)
        assertNull(harness.state.review.benchmark.selected)
      }
    }
  }

  @Test
  fun acceptedIndexingCompletionCancelsEachAdmissionStageButIgnoredCompletionsDoNot() {
    val attempt = ProjectIndexingAttempt(1, "project", "revision", "/tmp/project")
    for (stage in 0..2) {
      for (accepted in listOf(false, true)) {
        Harness().use { harness ->
          harness.selectBenchmark(trusted = false)
          harness.dispatch(DesktopEvent.ProjectIndexingStarted(attempt))
          harness.enqueueAdmission()
          harness.workflow.compareSelectedGoBenchmark()
          repeat(stage) { harness.runStage() }
          harness.main.runPending()
          harness.io.runPending()
          val job = harness.scope.coroutineContext[Job]!!.children.single()
          harness.dispatch(
              DesktopEvent.ProjectIndexingCompleted(
                  if (accepted) attempt else attempt.copy(generation = 99),
                  ProjectIndex("project", "next")))
          assertEquals(accepted, job.isCancelled)
          assertEquals(!accepted, harness.state.review.benchmark.running)
          harness.completeRequest()
          assertEquals(if (accepted) stage + 1 else 3, harness.requests.size)
          assertEquals(
              if (accepted) null else comparison, harness.state.review.benchmark.comparison)
          if (accepted) {
            assertEquals(DraftEditorStatus.Stale, harness.state.review.editor?.status)
            assertNull(harness.state.review.benchmark.catalog)
            assertNull(harness.state.review.benchmark.selected)
          }
        }
      }
    }
  }

  @Test
  fun catalogReplacementCancelsAdmissionEvenWhenTheExactCatalogAndChoiceAreRestored() {
    for (stage in 0..2) {
      Harness().use { harness ->
        harness.selectBenchmark(trusted = false)
        harness.enqueueAdmission()
        harness.workflow.compareSelectedGoBenchmark()
        repeat(stage) { harness.runStage() }
        harness.main.runPending()
        harness.io.runPending()
        val job = harness.scope.coroutineContext[Job]!!.children.single()
        harness.dispatch(DesktopEvent.GoBenchmarkCatalogLoaded(catalog.copy(trusted = false)))
        assertTrue(job.isCancelled)
        assertNull(harness.state.review.benchmark.selected)
        harness.workflow.selectGoBenchmark(choice)
        harness.completeRequest()
        assertEquals(stage + 1, harness.requests.size)
        assertEquals(choice, harness.state.review.benchmark.selected)
        assertFalse(harness.state.review.benchmark.running)
        assertNull(harness.state.review.benchmark.comparison)
      }
    }
  }

  @Test
  fun sameChoiceIsANoOpButChangingChoiceStopsEachStageWithoutRunningTheReplacement() {
    val other = choice.copy(name = "BenchmarkOther", scope = "other-scope")
    for (stage in 0..2) {
      for (same in listOf(false, true)) {
        Harness().use { harness ->
          harness.dispatch(
              DesktopEvent.GoBenchmarkCatalogLoaded(
                  catalog.copy(trusted = false, benchmarks = listOf(choice, other))))
          harness.workflow.selectGoBenchmark(choice)
          harness.enqueueAdmission()
          harness.workflow.compareSelectedGoBenchmark()
          repeat(stage) { harness.runStage() }
          harness.main.runPending()
          harness.io.runPending()
          val job = harness.scope.coroutineContext[Job]!!.children.single()
          val before = harness.state.review.benchmark
          val eventCount = harness.events.size
          harness.workflow.selectGoBenchmark(if (same) choice else other)
          assertEquals(!same, job.isCancelled)
          if (same) {
            assertEquals(before, harness.state.review.benchmark)
            assertEquals(eventCount, harness.events.size)
          } else {
            assertEquals(other, harness.state.review.benchmark.selected)
            assertEquals(
                BenchmarkAdmissionOutcome.Stopped, harness.state.review.benchmark.admission)
          }
          harness.completeRequest()
          assertEquals(if (same) 3 else stage + 1, harness.requests.size)
          assertEquals(if (same) comparison else null, harness.state.review.benchmark.comparison)
          assertFalse(harness.state.review.benchmark.running)
        }
      }
    }
  }

  @Test
  fun ignoredRefreshAndObsoleteValidationEventsPreserveCurrentAdmission() {
    val ignored =
        listOf<DesktopEvent>(
            DesktopEvent.SelectedFileRefreshed(file(), emptyList()),
            DesktopEvent.SelectedFileRefreshed(file().copy(path = "other.go"), emptyList()),
            DesktopEvent.IndexRefreshed(ProjectIndex("project", "revision")),
            DesktopEvent.IndexRefreshed(ProjectIndex("other", "next")),
            DesktopEvent.DraftValidationUpdated(99, draft().copy(validation = null)),
            DesktopEvent.DraftValidationStopped(99, ValidationAttemptStatus.Failed, "Old failure"))
    Harness().use { harness ->
      harness.selectBenchmark(trusted = false)
      harness.enqueueAdmission()
      harness.workflow.compareSelectedGoBenchmark()
      harness.main.runPending()
      harness.io.runPending()
      val before = harness.state.review.benchmark
      val job = harness.scope.coroutineContext[Job]!!.children.single()
      ignored.forEach { harness.dispatch(it) }
      assertEquals(before, harness.state.review.benchmark)
      assertFalse(job.isCancelled)
      harness.completeRequest()
      assertEquals(3, harness.requests.size)
      assertEquals(comparison, harness.state.review.benchmark.comparison)
    }
  }

  @Test
  fun generationInvalidationStopsEachStageAndRejectsAlreadyCompletedResponses() {
    for (stage in 0..2) {
      Harness().use { harness ->
        harness.selectBenchmark(trusted = false)
        harness.enqueueAdmission()
        harness.workflow.compareSelectedGoBenchmark()
        repeat(stage) { harness.runStage() }
        harness.main.runPending()
        harness.io.runPending()
        harness.workflow.cancel()
        harness.completeRequest()
        assertEquals(stage + 1, harness.requests.size)
        assertEquals(BenchmarkAdmissionOutcome.Stopped, harness.state.review.benchmark.admission)
        assertNull(harness.state.review.benchmark.comparison)
        assertTrue(harness.events.none { it is DesktopEvent.GoBenchmarkComparisonLoaded })
        assertTrue(harness.events.none { it is DesktopEvent.GoBenchmarkComparisonFailed })
      }
    }
  }

  @Test
  fun queuedStagesRecheckOpenFileBeforeAnyConsequentialRequest() {
    for (stage in 0..2) {
      Harness().use { harness ->
        harness.selectBenchmark(trusted = false)
        harness.enqueueAdmission()
        harness.workflow.compareSelectedGoBenchmark()
        repeat(stage) { harness.runStage() }
        harness.main.runPending()
        harness.controller.dispatch(
            DesktopEvent.FileLoaded(file().copy(path = "other.go"), emptyList()))
        harness.completeRequest()
        assertEquals(stage, harness.requests.size)
        assertNull(harness.state.review.benchmark.comparison)
      }
    }
  }

  @Test
  fun stageTimeoutsEndActiveStateLocallyWithoutRetriesOrInventedMeasurements() {
    for (stage in 0..2) {
      Harness().use { harness ->
        harness.selectBenchmark(trusted = false)
        harness.dispatch(DesktopEvent.GoBenchmarkComparisonLoaded(comparison))
        repeat(stage) { harness.enqueue(trust) }
        harness.responses.addLast { throw HttpTimeoutException("Stage timed out") }
        harness.workflow.compareSelectedGoBenchmark()
        harness.completeRequest()
        val failure = harness.admissionFailure()
        assertTrue(failure.contains("Stage timed out"))
        assertEquals(stage == 2, failure.contains("Execution may have started"))
        assertEquals(stage == 2, failure.contains("no new measurements were confirmed"))
        assertNull(harness.state.review.benchmark.latestOutcome, "Timeout is not a daemon outcome")
        assertEquals(comparison, harness.state.review.benchmark.comparison)
        assertFalse(harness.state.review.benchmark.running)
        assertNull(harness.state.jobs.error)
        harness.completeRequest()
        assertEquals(stage + 1, harness.requests.size)
      }
    }
  }

  @Test
  fun stageConflictsMarkTheDraftStaleAndRevokeSelectionWithLocalRecovery() {
    for (stage in 0..2) {
      Harness().use { harness ->
        harness.selectBenchmark(trusted = false)
        repeat(stage) { harness.enqueue(trust) }
        harness.responses.addLast { TransportResponse(409, """{"message":"Candidate changed"}""") }
        harness.workflow.compareSelectedGoBenchmark()
        harness.completeRequest()
        assertEquals(DraftEditorStatus.Stale, harness.state.review.editor!!.status)
        assertEquals("Candidate changed", harness.admissionFailure())
        assertNull(harness.state.review.benchmark.selected)
        assertNull(harness.state.review.benchmark.catalog)
        assertNull(harness.state.jobs.error)
        assertFalse(harness.state.review.benchmark.running)
        assertEquals(stage + 1, harness.requests.size)
      }
    }
  }

  @Test
  fun stageCancellationsPropagateTheOriginalCauseAndNeverFabricateComparisonResults() {
    for (stage in 0..2) {
      Harness().use { harness ->
        harness.selectBenchmark(trusted = false)
        repeat(stage) { harness.enqueue(trust) }
        val cancellation = CancellationException("Stage cancelled")
        harness.responses.addLast { throw cancellation }
        harness.workflow.compareSelectedGoBenchmark()
        harness.main.runPending()
        val job = harness.scope.coroutineContext[Job]!!.children.single()
        var completion: Throwable? = null
        job.invokeOnCompletion { completion = it }
        harness.completeRequest()
        assertTrue(job.isCancelled)
        assertEquals(cancellation.message, completion?.message)
        assertEquals(BenchmarkAdmissionOutcome.Stopped, harness.state.review.benchmark.admission)
        assertNull(harness.state.review.benchmark.comparison)
        assertNull(
            harness.state.review.benchmark.latestOutcome,
            "Local cancellation is not a daemon outcome")
        assertTrue(harness.events.none { it is DesktopEvent.GoBenchmarkComparisonFailed })
        assertEquals(stage + 1, harness.requests.size)
      }
    }
  }

  private fun publicationInvalidations(): List<DesktopEvent> =
      listOf(
          DesktopEvent.DraftLoaded(draft().copy(id = "other")),
          DesktopEvent.DraftLoaded(draft().copy(revision = 2)),
          DesktopEvent.DraftLoaded(draft().copy(hash = "other")),
          DesktopEvent.DraftLoaded(draft().copy(projectId = "other")),
          DesktopEvent.DraftLoaded(draft().copy(projectRevision = "other")),
          DesktopEvent.DraftLoaded(draft().copy(targetPath = "other.go")),
          DesktopEvent.DraftLoaded(draft().copy(baseFileHash = "other")),
          DesktopEvent.DraftLoaded(draft().copy(validation = null)),
          DesktopEvent.DraftEdited(declaration = "func Run() { println(1) }"),
          DesktopEvent.DraftEdited(imports = listOf("fmt")),
          DesktopEvent.DraftValidationStarted(1),
          DesktopEvent.DraftMarkedStale,
          DesktopEvent.DraftDiscarded,
          DesktopEvent.SelectedFileRefreshed(file().copy(contentHash = "other"), emptyList()),
          DesktopEvent.SelectedFileUnavailable("File was removed"),
          DesktopEvent.IndexRefreshed(ProjectIndex("project", "next")),
          DesktopEvent.Applied(ApplyResult("next", "after", true)),
          DesktopEvent.FileLoaded(file().copy(contentHash = "other"), emptyList()),
          DesktopEvent.FileLoaded(file().copy(path = "other.go"), emptyList()),
          DesktopEvent.ProjectLoaded(project(revision = "next"), ProjectIndex("project", "next")),
          DesktopEvent.ProjectLoaded(project("other"), ProjectIndex("other", "revision")),
          DesktopEvent.GoBenchmarkCatalogLoaded(catalog.copy(reason = "replacement")),
          DesktopEvent.GoBenchmarkSelected(choice.copy(name = "BenchmarkOther")),
          DesktopEvent.GoBenchmarkSelected(choice.copy(command = listOf("other"))),
          DesktopEvent.GoBenchmarkSelected(choice.copy(scope = "other")))

  private fun candidateState(): DesktopState {
    val controller = DesktopWorkflowController()
    controller.dispatch(DesktopEvent.ProjectLoaded(project(), ProjectIndex("project", "revision")))
    controller.dispatch(DesktopEvent.FileLoaded(file(), emptyList()))
    controller.dispatch(DesktopEvent.DraftLoaded(draft()))
    return controller.state
  }

  private inner class Harness(initial: DesktopState = candidateState()) : AutoCloseable {
    val main = QueuedDispatcher()
    val io = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + main)
    val controller = DesktopWorkflowController(initial)
    val state
      get() = controller.state

    val requests = mutableListOf<Triple<String, String, String?>>()
    val methods
      get() = requests.map { it.first }

    val events = mutableListOf<DesktopEvent>()
    var response = TransportResponse(200, Json.encodeToString(catalog))
    var transportFailure: Exception? = null
    val responses = ArrayDeque<() -> TransportResponse>()
    val workflow =
        DesktopBenchmarkWorkflow(
            ApiClient(
                transport =
                    DaemonTransport { method, path, body ->
                      requests.add(Triple(method, path, body))
                      transportFailure?.let { throw it }
                      if (responses.isEmpty()) response else responses.removeFirst().invoke()
                    }),
            scope,
            io,
            { controller.state },
            ::dispatch)

    fun dispatch(event: DesktopEvent) {
      workflow.beforeEvent(event)
      controller.dispatch(event)
      events.add(event)
    }

    fun selectBenchmark(trusted: Boolean = true) {
      dispatch(DesktopEvent.GoBenchmarkCatalogLoaded(catalog.copy(trusted = trusted)))
      workflow.selectGoBenchmark(choice)
    }

    fun enqueue(value: ExecutionTrust) {
      responses.addLast { TransportResponse(200, Json.encodeToString(value)) }
    }

    fun enqueue(value: GoBenchmarkComparison) {
      responses.addLast { TransportResponse(200, Json.encodeToString(value)) }
    }

    fun enqueueAdmission() {
      enqueue(trust.copy(trusted = false))
      enqueue(trust)
      enqueue(comparison)
    }

    fun admissionFailure(): String =
        (state.review.benchmark.admission as BenchmarkAdmissionOutcome.Failed).message

    fun runStage() {
      main.runPending()
      io.runPending()
      main.runPending()
    }

    fun completeRequest() {
      do {
        runStage()
      } while (main.hasPending || io.hasPending)
    }

    override fun close() {
      workflow.cancel()
      scope.cancel()
      completeRequest()
    }
  }

  private class QueuedDispatcher : CoroutineDispatcher() {
    private val pending = ConcurrentLinkedQueue<Runnable>()
    val hasPending: Boolean
      get() = pending.isNotEmpty()

    override fun dispatch(context: CoroutineContext, block: Runnable) {
      pending.add(block)
    }

    fun runPending() {
      while (true) (pending.poll() ?: return).run()
    }
  }

  private val trust =
      ExecutionTrust("project", "revision", true, listOf(listOf("go", "test", "./...")))
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
          status = "completed",
          command = choice.command,
          base = GoBenchmarkMeasurement(listOf(GoBenchmarkSample(10, 100.0))),
          candidate = GoBenchmarkMeasurement(listOf(GoBenchmarkSample(10, 80.0))))

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
