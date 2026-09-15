package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNotNull
import kotlin.test.assertTrue
import kotlinx.serialization.json.Json

class PerformanceWorkspaceTest {
  @Test
  fun typedResultsKeepPathIdentityAndUnmeasuredState() {
    val page = performancePageFixture()
    val result = performanceResults(page).single()
    assertEquals("main.go", result.report.path)
    assertEquals("", result.row().state)
    assertEquals("", result.row().source)
    assertTrue(
        performanceResults(page.copy(project = page.project!!.copy(projectId = "other"))).isEmpty())
    assertTrue(
        performanceResults(page.copy(run = page.run!!.copy(status = "stale"))).single().stale)
    assertEquals(
        "Stale",
        performanceResults(page.copy(run = page.run.copy(status = "stale"))).single().row().state)
  }

  @Test
  fun prepareRequiresMatchingRevisionHashAndExactGoAnchor() {
    val result = performanceResults(performancePageFixture()).single()
    val index = resultIndexFixture()
    assertTrue(performanceCanPrepare(result, index))
    assertFalse(performanceCanPrepare(result.copy(stale = true), index))
    assertFalse(
        performanceCanPrepare(result.copy(report = result.report.copy(contentHash = "old")), index))
    assertFalse(performanceCanPrepare(result, index.copy(projectRevision = "next")))
    assertFalse(performanceCanPrepare(result, index.copy(projectId = "other")))
    assertFalse(
        performanceCanPrepare(result.copy(finding = result.finding.copy(startLine = 19)), index))
    assertFalse(
        performanceCanPrepare(
            result, index.copy(files = index.files.map { it.copy(language = "Kotlin") })))
    assertFalse(
        performanceCanPrepare(
            result,
            index.copy(
                files =
                    index.files.map {
                      it.copy(
                          symbols =
                              it.symbols.map { symbol -> symbol.copy(confidence = "heuristic") })
                    })))
  }

  @Test
  fun benchmarkPresentationParsesCompactComparableEvidenceAndKeepsMemoryTradeoffsVisible() {
    val comparison =
        Json.decodeFromString<GoBenchmarkComparison>(
            """{
              "draft_id":"draft-1", "draft_revision":2, "draft_hash":"candidate-hash",
              "project_id":"project", "project_revision":"source-revision",
              "base_file_hash":"base-hash", "target_path":"internal/work.go",
              "benchmark":"BenchmarkWork", "scope":"scope", "status":"completed",
              "command":["go","test","-run","^$","-bench","^BenchmarkWork$","-benchtime","100ms","-benchmem"],
              "base":{"samples":[
                {"iterations":1,"ns_per_op":100,"bytes_per_op":10,"allocs_per_op":1},
                {"iterations":1,"ns_per_op":100,"bytes_per_op":10,"allocs_per_op":1},
                {"iterations":1,"ns_per_op":100,"bytes_per_op":10,"allocs_per_op":1},
                {"iterations":1,"ns_per_op":100,"bytes_per_op":10,"allocs_per_op":1},
                {"iterations":1,"ns_per_op":100,"bytes_per_op":10,"allocs_per_op":1}]},
              "candidate":{"samples":[
                {"iterations":1,"ns_per_op":90,"bytes_per_op":12,"allocs_per_op":1},
                {"iterations":1,"ns_per_op":90,"bytes_per_op":12,"allocs_per_op":1},
                {"iterations":1,"ns_per_op":90,"bytes_per_op":12,"allocs_per_op":1},
                {"iterations":1,"ns_per_op":90,"bytes_per_op":12,"allocs_per_op":1},
                {"iterations":1,"ns_per_op":90,"bytes_per_op":12,"allocs_per_op":1}]}
            }"""
                .trimIndent())

    val presentation = performanceBenchmarkPresentation(comparison, comparison.identity())

    assertEquals("Measured · selected benchmark", presentation.stateLabel)
    assertTrue(presentation.isMeasured)
    assertFalse(presentation.inconclusive)
    assertTrue(presentation.conditions.contains("Target duration per sample: 100ms"))
    assertTrue(presentation.rows.contains("Workload" to "BenchmarkWork"))
    assertTrue(presentation.rows.any { it.first == "ns/op" && it.second.contains("-10.0%") })
    assertTrue(presentation.rows.any { it.first == "B/op" && it.second.contains("+20.0%") })
    assertTrue(presentation.summary.contains("higher memory use"))
    assertTrue(presentation.insights.single().contains("trade-off"))
  }

  @Test
  fun benchmarkPresentationMarksNoisyAndIncompleteEvidenceAsInconclusive() {
    val noisy = comparison(candidateNanoseconds = listOf(80.0, 80.0, 80.0, 80.0, 120.0))
    val noisyPresentation = performanceBenchmarkPresentation(noisy, noisy.identity())

    assertEquals("Inconclusive · noisy samples", noisyPresentation.stateLabel)
    assertTrue(noisyPresentation.inconclusive)
    assertFalse(noisyPresentation.isMeasured)
    assertTrue(noisyPresentation.summary.contains("too variable"))
    assertTrue(noisyPresentation.rows.any { it == ("Variability" to "high: ns/op") })

    val incomplete =
        noisy.copy(candidate = GoBenchmarkMeasurement(noisy.candidate!!.samples.take(4)))
    val incompletePresentation = performanceBenchmarkPresentation(incomplete, incomplete.identity())

    assertEquals(
        "Inconclusive · incomplete measurement evidence", incompletePresentation.stateLabel)
    assertTrue(incompletePresentation.summary.contains("complete comparable"))

    val zeroAllocation =
        noisy.copy(
            candidate =
                GoBenchmarkMeasurement(
                    List(5) { benchmarkSample(90.0, bytes = 0, allocations = 0) }),
            base =
                GoBenchmarkMeasurement(
                    List(5) { benchmarkSample(100.0, bytes = 0, allocations = 0) }),
        )
    assertFalse(
        performanceBenchmarkPresentation(zeroAllocation, zeroAllocation.identity()).inconclusive)
  }

  @Test
  fun benchmarkPresentationRejectsPartialMemoryRegressionAsInconclusive() {
    val memoryRegressionWithMissingSample =
        comparison()
            .copy(
                candidate =
                    GoBenchmarkMeasurement(
                        List(4) { benchmarkSample(90.0, bytes = 12, allocations = 1) } +
                            benchmarkSample(90.0, bytes = 12, allocations = 1)
                                .copy(bytesPerOperation = null)),
            )

    val presentation =
        performanceBenchmarkPresentation(
            memoryRegressionWithMissingSample, memoryRegressionWithMissingSample.identity())

    assertEquals("Inconclusive · incomplete memory evidence", presentation.stateLabel)
    assertTrue(presentation.inconclusive)
    assertFalse(presentation.isMeasured)
    assertTrue(presentation.rows.any { it == ("B/op" to "incomplete: 5 base · 4 candidate") })
    assertTrue(presentation.summary.contains("B/op"))
    assertTrue(presentation.insights.single().contains("cannot establish a performance win"))
  }

  @Test
  fun benchmarkPresentationTreatsWhollyMissingBenchmemMetricsAsIncomplete() {
    val missingMemoryEvidence =
        comparison()
            .copy(
                base =
                    GoBenchmarkMeasurement(
                        List(5) { benchmarkSample(100.0, 10, 1).withoutMemoryMetrics() }),
                candidate =
                    GoBenchmarkMeasurement(
                        List(5) { benchmarkSample(90.0, 10, 1).withoutMemoryMetrics() }),
            )

    val presentation =
        performanceBenchmarkPresentation(missingMemoryEvidence, missingMemoryEvidence.identity())

    assertEquals("Inconclusive · incomplete memory evidence", presentation.stateLabel)
    assertFalse(presentation.isMeasured)
    assertTrue(presentation.inconclusive)
    assertTrue(presentation.rows.any { it == ("B/op" to "incomplete: 0 base · 0 candidate") })
    assertTrue(presentation.rows.any { it == ("allocs/op" to "incomplete: 0 base · 0 candidate") })
    assertFalse(presentation.summary.contains("CPU is lower"))
  }

  @Test
  fun benchmarkPresentationTreatsOpposingCompleteMemorySignalsAsInconclusive() {
    val opposingMemoryEvidence =
        comparison()
            .copy(
                base = GoBenchmarkMeasurement(List(5) { benchmarkSample(100.0, 10, 2) }),
                candidate = GoBenchmarkMeasurement(List(5) { benchmarkSample(90.0, 12, 1) }),
            )

    val presentation =
        performanceBenchmarkPresentation(opposingMemoryEvidence, opposingMemoryEvidence.identity())

    assertEquals("Inconclusive · opposing memory signals", presentation.stateLabel)
    assertFalse(presentation.isMeasured)
    assertTrue(presentation.inconclusive)
    assertTrue(presentation.rows.any { it.first == "B/op" && it.second.contains("+20.0%") })
    assertTrue(presentation.rows.any { it.first == "allocs/op" && it.second.contains("-50.0%") })
    assertTrue(presentation.summary.contains("B/op increased while allocs/op decreased"))
    assertTrue(presentation.insights.single().contains("cannot establish a performance win"))
  }

  @Test
  fun benchmarkPresentationFormatsZeroMemoryBaselinesWithoutNonFinitePercentages() {
    val zeroToZero =
        comparison()
            .copy(
                base = GoBenchmarkMeasurement(List(5) { benchmarkSample(100.0, 0, 0) }),
                candidate = GoBenchmarkMeasurement(List(5) { benchmarkSample(90.0, 0, 0) }),
            )
    val zeroToPositive =
        zeroToZero.copy(
            candidate = GoBenchmarkMeasurement(List(5) { benchmarkSample(90.0, 4, 0) }),
        )

    val unchanged = performanceBenchmarkPresentation(zeroToZero, zeroToZero.identity())
    val increased = performanceBenchmarkPresentation(zeroToPositive, zeroToPositive.identity())

    assertEquals("0 → 0 · no change from zero", unchanged.rows.single { it.first == "B/op" }.second)
    assertEquals("0 → 4 · from zero to 4", increased.rows.single { it.first == "B/op" }.second)
    assertTrue(
        (unchanged.rows + increased.rows).none { "NaN" in it.second || "Infinity" in it.second })
  }

  @Test
  fun benchmarkPresentationRejectsStaleCandidateIdentityAndNamesTerminalStates() {
    val comparison = comparison()
    val staleIdentity = comparison.identity().copy(draftHash = "new-candidate-hash")

    val stale = performanceBenchmarkPresentation(comparison, staleIdentity)
    assertEquals("Stale · candidate identity changed", stale.stateLabel)
    assertTrue(stale.isStale)
    assertFalse(stale.isMeasured)

    val noCurrentCandidate = performanceBenchmarkPresentation(comparison)
    assertEquals("Stale · no current candidate", noCurrentCandidate.stateLabel)
    assertTrue(noCurrentCandidate.isStale)
    assertFalse(noCurrentCandidate.isMeasured)

    val staleBenchmark =
        performanceBenchmarkPresentation(
            comparison,
            comparison.identity(),
            GoBenchmarkChoice(name = "BenchmarkOther", scope = "other-scope"))
    assertEquals("Stale · selected benchmark changed", staleBenchmark.stateLabel)
    assertTrue(staleBenchmark.isStale)

    val unavailable =
        performanceBenchmarkPresentation(
            comparison.copy(status = "unavailable", reason = "No compatible benchmark"),
            comparison.identity())
    assertEquals("Not measured · unavailable", unavailable.stateLabel)
    assertEquals("No compatible benchmark", unavailable.summary)
    assertFalse(unavailable.isMeasured)
  }

  @Test
  fun benchmarkEvidenceIdentityRequiresAnApplicableValidatedDraft() {
    val applicable =
        comparison()
            .identityDraft(
                DeclarationValidation(
                    applicable = true,
                    scopeMode = "strict_symbol",
                    diff = UnifiedDiff("internal/work.go", "internal/work.go")))
    val unavailable = applicable.copy(validation = applicable.validation?.copy(applicable = false))

    assertNotNull(
        benchmarkEvidenceIdentity(
            DraftReviewState(draft = applicable, editor = editableDraft(applicable))))
    assertEquals(
        null,
        benchmarkEvidenceIdentity(
            DraftReviewState(draft = unavailable, editor = editableDraft(unavailable))))
  }

  private fun GoBenchmarkComparison.identity(): GoBenchmarkComparisonIdentity =
      assertNotNull(goBenchmarkComparisonIdentity(identityDraft()))

  private fun GoBenchmarkComparison.identityDraft(
      validation: DeclarationValidation? = null,
  ): DeclarationDraft =
      DeclarationDraft(
          id = draftId,
          revision = draftRevision,
          hash = draftHash,
          projectId = projectId,
          projectRevision = projectRevision,
          baseFileHash = baseFileHash,
          targetPath = targetPath,
          validation = validation,
      )

  private fun comparison(candidateNanoseconds: List<Double> = List(5) { 90.0 }) =
      GoBenchmarkComparison(
          draftId = "draft-1",
          draftRevision = 2,
          draftHash = "candidate-hash",
          projectId = "project",
          projectRevision = "source-revision",
          baseFileHash = "base-hash",
          targetPath = "internal/work.go",
          benchmark = "BenchmarkWork",
          status = "completed",
          command = listOf("go", "test", "-benchtime", "100ms", "-benchmem"),
          base = GoBenchmarkMeasurement(List(5) { benchmarkSample(100.0, 10, 1) }),
          candidate =
              GoBenchmarkMeasurement(candidateNanoseconds.map { benchmarkSample(it, 10, 1) }),
      )

  private fun benchmarkSample(nanoseconds: Double, bytes: Long, allocations: Long) =
      GoBenchmarkSample(
          iterations = 1,
          nanosecondsPerOperation = nanoseconds,
          bytesPerOperation = bytes,
          allocationsPerOperation = allocations,
      )

  private fun GoBenchmarkSample.withoutMemoryMetrics() =
      copy(bytesPerOperation = null, allocationsPerOperation = null)
}

internal fun performancePageFixture(): AnalysisResultPageState {
  val page = resultPageFixture("performance")
  val report =
      page.results!!
          .performance
          .single()
          .copy(
              status = "completed",
              findings =
                  listOf(
                      PerformanceFinding(
                          id = "perf",
                          title = "Avoid repeated allocation",
                          category = "allocation",
                          potentialImpact = "high",
                          confidence = "medium",
                          observedPattern = "A buffer is allocated on every request.",
                          recommendation = "Reuse a bounded buffer.",
                          workloadConditions = "High request volume.",
                          tradeoff = "Retained buffers increase memory use.",
                          verificationPlan = "Measure representative traffic.",
                          startLine = 4,
                          symbol = "Run")))
  return page.copy(
      section = page.section.copy(results = page.results!!.copy(performance = listOf(report))))
}
