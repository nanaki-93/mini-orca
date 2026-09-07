package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNotNull
import kotlin.test.assertTrue
import kotlinx.serialization.json.Json

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
