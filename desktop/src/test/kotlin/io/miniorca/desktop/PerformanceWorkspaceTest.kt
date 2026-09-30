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
    assertEquals("Model suggestion", result.row().source)
    assertTrue(
        performanceResults(page.copy(project = page.project!!.copy(projectId = "other"))).isEmpty())
    assertTrue(
        performanceResults(page.copy(run = page.run!!.copy(status = "stale"))).single().stale)
    assertEquals(
        "Stale",
        performanceResults(page.copy(run = page.run.copy(status = "stale"))).single().row().state)
  }

  @Test
  fun typedLocationsAndReportStatesDoNotInventSourceLinesOrMeasurements() {
    val result = performanceResults(performancePageFixture()).single()
    assertEquals("main.go:4 · Run", result.row().location)
    assertEquals("high", result.row().severity)
    val missing =
        result.copy(
            report = result.report.copy(path = "", status = "partial"),
            finding = result.finding.copy(startLine = 0, symbol = "", potentialImpact = ""))
    assertEquals(
        "Path not supplied · Source line not supplied · Symbol not supplied",
        missing.row().location)
    assertEquals(
        "main.go · Source line not supplied · Run",
        result.copy(finding = result.finding.copy(startLine = -1)).row().location)
    assertFalse(missing.row().location.contains(":0"))
    assertEquals("Unknown impact", missing.row().severity)
    assertEquals("Partial", missing.row().state)
    assertEquals("Partial · Stale", missing.copy(stale = true).row().state)
    assertEquals(
        "Stale",
        missing.copy(stale = true, report = missing.report.copy(status = "stale")).row().state)
  }

  @Test
  fun summaryAndPerformanceResultKeepReportedCountWhenSavedDetailsFail() {
    val page = performancePageFixture()
    val run =
        page.run!!.copy(
            status = "partial",
            sections =
                page.run.sections.map {
                  if (it.category == "performance") it.copy(status = "partial", findingCount = 3)
                  else it
                })
    val section =
        page.section.copy(
            results =
                page.section.results!!.copy(
                    progress = run.sections.first { it.category == "performance" }),
            error = "Saved result read failed")
    val metric =
        summaryIssueMetrics(page.project, run, mapOf(AnalysisResultKey("performance") to section))[
            1]
    val resultPage = page.copy(run = run, section = section)

    assertEquals(3, metric.value)
    assertEquals("Partial", metric.status)
    assertEquals("Saved details unavailable · 3 reported", metric.detailStatus)
    assertEquals(
        "1 loaded · 3 reported", resultPage.countLabel(performanceResults(resultPage).size))
    assertEquals("Model suggestion", performanceResults(resultPage).single().row().source)
  }

  @Test
  fun latestComparisonOutcomesRemainPrimaryOverRetainedMeasurements() {
    val prior = comparison()
    val choice = GoBenchmarkChoice(prior.benchmark, prior.command, prior.scope)
    val initial = DesktopState().reduce(DesktopEvent.GoBenchmarkComparisonLoaded(prior))
    for ((status, label) in
        listOf(
            "completed" to "Completed · no new measurements",
            "canceled" to "Comparison canceled · daemon",
            "failed" to "Comparison failed · daemon",
            "unavailable" to "Comparison unavailable · daemon",
            "future-status" to "Comparison unsupported · daemon status")) {
      val response = GoBenchmarkComparison(status = status, reason = "Recorded $status reason")
      val terminal =
          initial
              .reduce(DesktopEvent.GoBenchmarkComparisonStarted)
              .reduce(DesktopEvent.GoBenchmarkComparisonLoaded(response))
              .review
              .benchmark
      val presentation =
          performanceBenchmarkStatusPresentation(
              terminal.comparison,
              prior.identity(),
              choice,
              discovery = BenchmarkDiscoveryOutcome.Invalidated,
              admission = terminal.admission,
              latestOutcome = terminal.latestOutcome)
      assertEquals(label, presentation.stateLabel)
      assertEquals(response.reason, presentation.summary)
      assertTrue(presentation.priorEvidence)
      val withoutPrior =
          performanceBenchmarkStatusPresentation(
              null, null, null, latestOutcome = terminal.latestOutcome)
      assertEquals(label, withoutPrior.stateLabel)
      assertEquals(response.reason, withoutPrior.summary)
      assertFalse(withoutPrior.priorEvidence)
      for ((admission, activeLabel) in
          listOf(
              BenchmarkAdmissionOutcome.Admitting to "Admitting · execution trust",
              BenchmarkAdmissionOutcome.Running to "Running · explicit local execution",
              BenchmarkAdmissionOutcome.Stopped to "Benchmark admission stopped",
              BenchmarkAdmissionOutcome.Failed("Local transport uncertainty") to
                  "Benchmark admission failed")) {
        val active =
            performanceBenchmarkStatusPresentation(
                prior,
                prior.identity(),
                choice,
                admission = admission,
                latestOutcome = terminal.latestOutcome)
        assertEquals(activeLabel, active.stateLabel)
        assertTrue(active.priorEvidence)
      }
    }
    val unsupported =
        performanceBenchmarkStatusPresentation(
            prior,
            prior.identity(),
            choice,
            latestOutcome = BenchmarkComparisonOutcome(GoBenchmarkComparison(status = "future")))
    assertTrue(unsupported.summary.contains("unsupported status future"))
    val completed =
        performanceBenchmarkStatusPresentation(
            prior, prior.identity(), choice, latestOutcome = initial.review.benchmark.latestOutcome)
    assertEquals("Measured · selected benchmark", completed.stateLabel)
    assertFalse(completed.priorEvidence)
    val partialResponse =
        prior.copy(status = "failed", candidate = null, reason = "Candidate failed")
    val partialOutcome =
        initial.reduce(DesktopEvent.GoBenchmarkComparisonLoaded(partialResponse)).review.benchmark
    val partial =
        performanceBenchmarkStatusPresentation(
            partialOutcome.comparison,
            prior.identity(),
            choice,
            latestOutcome = partialOutcome.latestOutcome)
    assertEquals("Comparison failed · daemon", partial.stateLabel)
    assertFalse(
        partial.priorEvidence, "Measurements returned by this attempt are not prior evidence")
    for (admission in
        listOf(BenchmarkAdmissionOutcome.Admitting, BenchmarkAdmissionOutcome.Running)) {
      val active =
          performanceBenchmarkStatusPresentation(
              prior,
              prior.identity(),
              choice,
              discovery = BenchmarkDiscoveryOutcome.Loading,
              admission = admission)
      assertTrue(
          active.stateLabel.startsWith(
              if (admission == BenchmarkAdmissionOutcome.Running) "Running" else "Admitting"))
    }
  }

  @Test
  fun priorMeasurementDetailsQualifyTheirAssessmentAndKeepRecordedValues() {
    val comparison = comparison()
    val choice = GoBenchmarkChoice(comparison.benchmark, comparison.command, comparison.scope)
    val current = performanceBenchmarkPresentation(comparison, comparison.identity(), choice)
    val prior =
        performanceBenchmarkPresentation(
            comparison, comparison.identity(), choice, priorEvidence = true)
    assertEquals("Prior evidence · Measured · selected benchmark", prior.stateLabel)
    assertTrue(
        prior.summary.startsWith("Prior comparison only; it does not confirm the latest attempt."))
    assertTrue(prior.insights.all { it.startsWith("Prior observation:") })
    assertEquals(current.rows, prior.rows)
    assertFalse(prior.isMeasured)
  }

  @Test
  fun recommendationAndBenchmarkStatusKeepPotentialAndMeasuredEvidenceSeparate() {
    val comparison = comparison()
    val selectedChoice =
        GoBenchmarkChoice(comparison.benchmark, comparison.command, comparison.scope)

    assertEquals(
        "Not measured · explicit local execution",
        performanceBenchmarkStatusPresentation(null, null, null).stateLabel)
    assertEquals(
        "Running · explicit local execution",
        performanceBenchmarkStatusPresentation(
                null, comparison.identity(), null, admission = BenchmarkAdmissionOutcome.Running)
            .stateLabel)
    assertEquals(
        "Measured · selected benchmark",
        performanceBenchmarkStatusPresentation(comparison, comparison.identity(), selectedChoice)
            .stateLabel)
    assertTrue(
        performanceBenchmarkStatusPresentation(comparison, comparison.identity(), selectedChoice)
            .summary
            .contains("does not measure this model suggestion"))
    assertEquals(
        "Stale · candidate identity changed",
        performanceBenchmarkStatusPresentation(
                comparison, comparison.identity().copy(draftHash = "new-candidate"), null)
            .stateLabel)
    assertEquals(
        "Not measured · unavailable",
        performanceBenchmarkStatusPresentation(
                comparison.copy(status = "unavailable", reason = "No compatible benchmark"),
                comparison.identity(),
                null)
            .stateLabel)
    for ((status, label) in
        listOf("failed" to "Not measured · failed", "canceled" to "Not measured · canceled")) {
      val presentation =
          performanceBenchmarkStatusPresentation(
              comparison.copy(status = status, reason = "Execution $status"),
              comparison.identity(),
              selectedChoice)
      assertEquals(label, presentation.stateLabel)
      assertTrue(presentation.summary.contains("does not measure this model suggestion"))
    }
    assertEquals(
        "Inconclusive · incomplete measurement evidence",
        performanceBenchmarkStatusPresentation(
                comparison.copy(
                    candidate = GoBenchmarkMeasurement(comparison.candidate!!.samples.take(4))),
                comparison.identity(),
                null)
            .stateLabel)
    assertEquals(
        "Inconclusive · incomplete memory evidence",
        performanceBenchmarkStatusPresentation(
                comparison.copy(
                    candidate =
                        GoBenchmarkMeasurement(
                            comparison.candidate!!.samples.mapIndexed { index, sample ->
                              if (index == 0) sample.withoutMemoryMetrics() else sample
                            })),
                comparison.identity(),
                null)
            .stateLabel)
  }

  @Test
  fun preparationCapturesExactIndexedDeclarationWithoutRequiringReportSignature() {
    val result = performanceResults(performancePageFixture()).single()
    val index = resultIndexFixture()
    val decision = performancePreparationDecision(result, index)
    assertEquals(
        PerformancePreparationDecision.Eligible(
            "project", "revision", "main.go", "base", index.files.single().symbols.single()),
        decision)
    val partialReport = result.report.copy(status = "partial")
    val partialPage = result.page.results!!.copy(performance = listOf(partialReport))
    val partial =
        result.copy(
            report = partialReport,
            page = result.page.copy(section = result.page.section.copy(results = partialPage)))
    assertTrue(
        performancePreparationDecision(partial, index) is PerformancePreparationDecision.Eligible)
    val pendingReport = partialReport.copy(status = "running")
    val pendingPage = partial.page.results!!.copy(performance = listOf(pendingReport))
    val pending =
        partial.copy(
            report = pendingReport,
            page = partial.page.copy(section = partial.page.section.copy(results = pendingPage)))
    assertTrue(
        (performancePreparationDecision(pending, index) as PerformancePreparationDecision.Blocked)
            .reason
            .contains("completed or partial"))
  }

  @Test
  fun loadedPreparationRejectsChangedSourceAndInexactOrAmbiguousDeclarations() {
    val result = performanceResults(performancePageFixture()).single()
    val index = resultIndexFixture()
    val target =
        performancePreparationDecision(result, index) as PerformancePreparationDecision.Eligible
    val symbol = target.declaration
    val file =
        ProjectFileInfo(
            "main.go",
            "base",
            "main.go",
            language = "Go",
            sizeBytes = 1,
            lineCount = 20,
            modifiedAt = "",
            binary = false)
    fun reason(source: ProjectFileInfo = file, symbols: List<SymbolInfo> = listOf(symbol)): String =
        (loadedPerformancePreparationDecision(target, result.finding, source, symbols)
                as PerformancePreparationDecision.Blocked)
            .reason
    assertEquals(
        target, loadedPerformancePreparationDecision(target, result.finding, file, listOf(symbol)))
    assertTrue(reason(file.copy(path = "other.go")).contains("indexed file"))
    assertTrue(reason(file.copy(contentHash = "changed")).contains("hash"))
    assertTrue(reason(symbols = emptyList()).contains("exact indexed"))
    assertTrue(reason(symbols = listOf(symbol, symbol)).contains("exact indexed"))
    assertTrue(
        reason(symbols = listOf(symbol.copy(signature = "func Run(int)")))
            .contains("exact indexed"))
    assertTrue(reason(symbols = listOf(symbol.copy(startLine = 5))).contains("exact indexed"))
    assertTrue(reason(source = file.copy(binary = true)).contains("Binary"))
    assertTrue(reason(source = file.copy(language = "Kotlin")).contains("Go"))
    assertTrue(
        performancePreparationRequest(result.finding)
            .contains("Workload conditions: High request volume."))
    assertTrue(
        performancePreparationRequest(result.finding)
            .contains("Verification plan: Measure representative traffic."))
    assertFalse(
        performancePreparationRequest(result.finding.copy(tradeoff = "")).contains("Trade-offs:"))
  }

  @Test
  fun preparationExplainsMissingStaleAndAmbiguousEvidence() {
    val result = performanceResults(performancePageFixture()).single()
    val index = resultIndexFixture()
    fun reason(candidate: PerformanceResult = result, source: ProjectIndex? = index): String =
        (performancePreparationDecision(candidate, source)
                as PerformancePreparationDecision.Blocked)
            .reason

    assertTrue(reason(source = null).contains("project index"))
    assertTrue(reason(source = index.copy(projectId = "other")).contains("project index"))
    assertTrue(reason(source = index.copy(projectRevision = "next")).contains("project index"))
    assertTrue(reason(result.copy(stale = true)).contains("Analyze again"))
    assertTrue(
        reason(result.copy(page = result.page.copy(project = null))).contains("project index"))
    assertTrue(reason(result.copy(page = result.page.copy(run = null))).contains("project index"))
    assertTrue(
        reason(result.copy(report = result.report.copy(contentHash = "old")))
            .contains("Performance results"))
    assertTrue(
        reason(result.copy(finding = result.finding.copy(startLine = 19)))
            .contains("Performance results"))
    assertTrue(reason(source = index.copy(files = emptyList())).contains("target file"))
    assertTrue(
        reason(source = index.copy(files = index.files + index.files.single()))
            .contains("target file"))
    assertTrue(
        reason(
                result.copy(
                    page = result.page.copy(section = result.page.section.copy(results = null))))
            .contains("Performance results"))
    val duplicateReport =
        result.page.results!!.copy(performance = listOf(result.report, result.report))
    assertTrue(
        reason(
                result.copy(
                    page =
                        result.page.copy(
                            section = result.page.section.copy(results = duplicateReport))))
            .contains("ambiguous"))
    val duplicateFinding = result.report.copy(findings = listOf(result.finding, result.finding))
    val duplicated = result.page.results!!.copy(performance = listOf(duplicateFinding))
    assertTrue(
        reason(
                result.copy(
                    report = duplicateFinding,
                    page =
                        result.page.copy(section = result.page.section.copy(results = duplicated))))
            .contains("ambiguous"))
  }

  @Test
  fun preparationExplainsHashAnchorAndDeclarationEligibilityFailures() {
    val result = performanceResults(performancePageFixture()).single()
    val index = resultIndexFixture()
    fun reason(source: ProjectIndex): String =
        (performancePreparationDecision(result, source) as PerformancePreparationDecision.Blocked)
            .reason
    fun changedFile(change: (IndexedFile) -> IndexedFile): ProjectIndex =
        index.copy(files = listOf(change(index.files.single())))

    assertTrue(reason(changedFile { it.copy(contentHash = "") }).contains("hash"))
    assertTrue(reason(changedFile { it.copy(contentHash = "new") }).contains("hash"))
    assertTrue(reason(changedFile { it.copy(binary = true) }).contains("Binary"))
    assertTrue(reason(changedFile { it.copy(language = "Kotlin") }).contains("Go"))
    assertTrue(reason(changedFile { it.copy(symbols = emptyList()) }).contains("declaration"))
    assertTrue(
        reason(changedFile { it.copy(symbols = it.symbols + it.symbols.single()) })
            .contains("unambiguous"))
    assertTrue(
        reason(changedFile { it.copy(symbols = it.symbols.map { s -> s.copy(startLine = 5) }) })
            .contains("source line"))
    assertTrue(
        reason(
                changedFile {
                  it.copy(symbols = it.symbols.map { s -> s.copy(confidence = "heuristic") })
                })
            .contains("exact"))
    assertTrue(
        reason(
                changedFile {
                  it.copy(symbols = it.symbols.map { s -> s.copy(atomicTarget = false) })
                })
            .contains("declaration"))
    assertTrue(
        reason(changedFile { it.copy(symbols = it.symbols.map { s -> s.copy(kind = "package") }) })
            .contains("Go"))
    val blankPathReport = result.report.copy(path = "")
    val blankPathPage = result.page.results!!.copy(performance = listOf(blankPathReport))
    val blankPathResult =
        result.copy(
            report = blankPathReport,
            page = result.page.copy(section = result.page.section.copy(results = blankPathPage)))
    assertTrue(
        (performancePreparationDecision(blankPathResult, index)
                as PerformancePreparationDecision.Blocked)
            .reason
            .contains("path"))
    val blankSymbol = result.finding.copy(symbol = "")
    val symbolReport = result.report.copy(findings = listOf(blankSymbol))
    val symbolPage = result.page.results!!.copy(performance = listOf(symbolReport))
    val symbolResult =
        result.copy(
            report = symbolReport,
            finding = blankSymbol,
            page = result.page.copy(section = result.page.section.copy(results = symbolPage)))
    assertTrue(
        (performancePreparationDecision(symbolResult, index)
                as PerformancePreparationDecision.Blocked)
            .reason
            .contains("declaration"))
    val blankHashReport = result.report.copy(contentHash = "")
    val blankHashPage = result.page.results!!.copy(performance = listOf(blankHashReport))
    val blankResult =
        result.copy(
            report = blankHashReport,
            page = result.page.copy(section = result.page.section.copy(results = blankHashPage)))
    assertTrue(
        (performancePreparationDecision(blankResult, index)
                as PerformancePreparationDecision.Blocked)
            .reason
            .contains("hash"))
    val missingAnchor = result.finding.copy(startLine = 0)
    val anchorReport = result.report.copy(findings = listOf(missingAnchor))
    val anchorPage = result.page.results!!.copy(performance = listOf(anchorReport))
    val anchorResult =
        result.copy(
            report = anchorReport,
            finding = missingAnchor,
            page = result.page.copy(section = result.page.section.copy(results = anchorPage)))
    assertTrue(
        (performancePreparationDecision(anchorResult, index)
                as PerformancePreparationDecision.Blocked)
            .reason
            .contains("source line"))
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

  @Test
  fun controlsUseCandidateEligibilityRatherThanRetainedMeasurementIdentity() {
    val current = benchmarkCandidateFixture()
    val cases =
        listOf(
            current.copy(projectState = ProjectWorkspaceState()),
            current.copy(selection = FileSelectionState()),
            current.copy(review = DraftReviewState()),
            current.copy(
                selection =
                    current.selection.copy(
                        selectedFile = current.selectedFile!!.copy(contentHash = "changed"))),
            current.copy(
                review =
                    current.review.copy(
                        editor = current.review.editor!!.copy(status = DraftEditorStatus.Dirty))),
            current.copy(
                review =
                    current.review.copy(
                        editor =
                            current.review.editor!!.copy(status = DraftEditorStatus.Validating))),
            current.copy(
                review =
                    current.review.copy(
                        editor = current.review.editor!!.copy(status = DraftEditorStatus.Invalid))),
            current.copy(
                review =
                    current.review.copy(
                        editor = current.review.editor!!.copy(status = DraftEditorStatus.Stale)))) +
            current
    cases.forEach { snapshot ->
      val eligibility = benchmarkEligibility(snapshot)
      var requests = 0
      ComposeVisualFixture(1600, 1000) {
            PerformanceWorkspacePane(
                PerformanceWorkspacePaneState(
                    performancePageFixture(),
                    null,
                    expectedBenchmarkIdentity = goBenchmarkComparisonIdentity(current.review.draft),
                    benchmarkEligibility = eligibility),
                benchmarkActions { requests++ })
          }
          .use { fixture ->
            fixture.render()
            fixture.clickDescription("Expand Explore benchmark evidence")
            fixture.render()
            eligibility.discoveryBlockedReason?.let {
              assertTrue(fixture.hasText(it))
              assertEquals(it, fixture.stateDescription("List compatible benchmarks"))
            }
            assertEquals(!eligibility.canDiscover, fixture.isDisabled("List compatible benchmarks"))
            assertEquals(0, requests)
            if (eligibility.canDiscover) {
              fixture.clickText("List compatible benchmarks")
              assertEquals(1, requests)
            }
          }
    }
  }

  @Test
  fun controlsBlockStaleOrIncompleteExactChoicesAndEnableOnlyAuthoritativeReadiness() {
    val current = benchmarkCandidateFixture()
    val draft = current.review.draft!!
    val choice = GoBenchmarkChoice("BenchmarkWork", listOf("go", "test", "."), "scope")
    val catalog =
        GoBenchmarkCatalog(
            draftId = draft.id,
            draftRevision = draft.revision,
            draftHash = draft.hash,
            projectId = draft.projectId,
            projectRevision = draft.projectRevision,
            baseFileHash = draft.baseFileHash,
            targetPath = draft.targetPath,
            available = true,
            trusted = true,
            benchmarks = listOf(choice))
    val ready =
        BenchmarkEvidenceState(
            catalog = catalog, selected = choice, discovery = BenchmarkDiscoveryOutcome.Loaded)
    val cases =
        listOf(
            ready.copy(selected = choice.copy(command = listOf("other"))),
            ready.copy(selected = choice.copy(scope = "other")),
            ready.copy(catalog = catalog.copy(projectRevision = "other")),
            ready.copy(discovery = BenchmarkDiscoveryOutcome.Invalidated)) +
            listOf(
                    choice.copy(name = " "),
                    choice.copy(scope = " "),
                    choice.copy(command = emptyList()))
                .map {
                  ready.copy(catalog = catalog.copy(benchmarks = listOf(it)), selected = it)
                } +
            ready
    cases.forEach { evidence ->
      val eligibility =
          benchmarkEligibility(current.copy(review = current.review.copy(benchmark = evidence)))
      var requests = 0
      ComposeVisualFixture(1600, 1000) {
            PerformanceWorkspacePane(
                PerformanceWorkspacePaneState(
                    performancePageFixture(),
                    null,
                    expectedBenchmarkIdentity = goBenchmarkComparisonIdentity(draft),
                    benchmarkCatalog = evidence.catalog,
                    selectedBenchmark = evidence.selected,
                    benchmarkDiscovery = evidence.discovery,
                    benchmarkAdmission = evidence.admission,
                    benchmarkEligibility = eligibility),
                benchmarkActions { requests++ })
          }
          .use { fixture ->
            fixture.render()
            fixture.clickDescription("Expand Explore benchmark evidence")
            fixture.render()
            if (evidence.discovery == BenchmarkDiscoveryOutcome.Loaded) {
              assertEquals(!eligibility.canCompare, fixture.isDisabled("Run selected benchmark"))
              eligibility.comparisonBlockedReason?.let {
                assertTrue(fixture.hasText(it), "Blocked reasons remain outside optional details")
                assertEquals(it, fixture.stateDescription("Run selected benchmark"))
              }
              evidence.catalog!!.benchmarks.forEach { choice ->
                val label = "Select benchmark ${choice.name.ifBlank { "Unnamed benchmark" }}"
                assertTrue(fixture.hasDescription(label))
                assertEquals(choice == evidence.selected, fixture.isDescriptionSelected(label))
                assertEquals(
                    if (choice == evidence.selected) "Selected" else "Not selected",
                    fixture.descriptionState(label))
              }
              assertTrue(fixture.hasDescription("Selected benchmark argv"))
              assertEquals("Read-only", fixture.descriptionState("Selected benchmark argv"))
              assertFalse(fixture.hasText("Finding ID"), "Optional report metadata is collapsed")
              assertTrue(fixture.hasText("Running benchmarks executes imported project code."))
              assertTrue(fixture.hasText("This is broader than benchmark-only permission."))
            } else {
              assertFalse(fixture.hasText("Run selected benchmark"))
              assertTrue(fixture.hasText("Discovery invalidated"))
              assertFalse(fixture.isDisabled("Refresh compatible benchmarks"))
            }
            assertEquals(0, requests, "Rendering and disclosure do not activate an action")
            if (eligibility.canCompare) {
              fixture.revealTextFullyWithin("Run selected benchmark", "result-overview")
              fixture.clickText("Run selected benchmark")
              assertEquals(1, requests)
            }
          }
    }
  }

  @Test
  fun admissionIdentifiesTheValidatedCandidateAndDerivesOnlyTheWorkingDirectory() {
    val current = benchmarkCandidateFixture()
    val candidate = benchmarkEligibility(current).candidate as BenchmarkCandidateDecision.Ready
    val choice =
        GoBenchmarkChoice("BenchmarkWork", listOf("go", "test", "."), "opaque:not/a/directory")
    for ((path, directory) in
        listOf(
            "work.go" to ".", "internal/work.go" to "internal", "pkg/日本語/work.go" to "pkg/日本語")) {
      val rows =
          performanceBenchmarkAdmissionRows(
                  choice, candidate.copy(draft = candidate.draft.copy(targetPath = path)))
              .toMap()
      assertEquals(choice.name, rows["Selected benchmark"])
      assertEquals(candidate.draft.projectId, rows["Project ID"])
      assertEquals(candidate.draft.projectRevision, rows["Project revision"])
      assertEquals(path, rows["Target path"])
      assertEquals(candidate.draft.revision.toString(), rows["Validated draft revision"])
      assertEquals(directory, rows["Package working directory"])
      assertEquals(choice.scope, rows["Opaque scope guard (identity metadata)"])
    }
    assertFalse(
        performanceBenchmarkAdmissionRows(
                choice, BenchmarkCandidateDecision.Blocked("Validate again"))
            .any { it.first == "Validated draft revision" },
        "Blocked candidates must not be described as validated")
  }

  @Test
  fun argvDisclosurePreservesEveryArgumentBoundaryWithoutShellReconstruction() {
    val command =
        listOf(
            "go",
            "test",
            ".",
            "-run",
            "^$",
            "-bench",
            "^BenchmarkWork$",
            "-count",
            "5",
            "-benchtime",
            "100ms",
            "-benchmem",
            "-timeout",
            "15s",
            "",
            "one argument with spaces",
            "quote\" and \\ slash",
            "line\nfeed\tand tab",
            "日本語",
            "$(not-a-shell); *")
    val rendered = performanceBenchmarkArgv(command).lines()
    assertEquals(command.size, rendered.size)
    assertEquals(
        command,
        rendered.mapIndexed { index, line ->
          assertTrue(line.startsWith("argv[$index] = "))
          Json.decodeFromString<String>(line.substringAfter(" = "))
        })
    assertEquals("", performanceBenchmarkArgv(emptyList()))
  }

  @Test
  fun currentDiscoveryAndAdmissionOutcomesTakePrecedenceOverRetainedSuccess() {
    val comparison = comparison()
    val choice = GoBenchmarkChoice(comparison.benchmark, comparison.command, comparison.scope)
    val cases =
        listOf(
            Triple(
                BenchmarkDiscoveryOutcome.Loading,
                BenchmarkAdmissionOutcome.Idle,
                "Listing · read-only discovery"),
            Triple(
                BenchmarkDiscoveryOutcome.Unavailable("No matching benchmark"),
                BenchmarkAdmissionOutcome.Idle,
                "Discovery unavailable"),
            Triple(
                BenchmarkDiscoveryOutcome.Failed("Lookup timed out; retry discovery."),
                BenchmarkAdmissionOutcome.Idle,
                "Benchmark lookup failed"),
            Triple(
                BenchmarkDiscoveryOutcome.Invalidated,
                BenchmarkAdmissionOutcome.Idle,
                "Discovery invalidated"),
            Triple(
                BenchmarkDiscoveryOutcome.Loaded,
                BenchmarkAdmissionOutcome.Admitting,
                "Admitting · execution trust"),
            Triple(
                BenchmarkDiscoveryOutcome.Loaded,
                BenchmarkAdmissionOutcome.Running,
                "Running · explicit local execution"),
            Triple(
                BenchmarkDiscoveryOutcome.Loaded,
                BenchmarkAdmissionOutcome.Failed("Comparison timed out; execution may have begun."),
                "Benchmark admission failed"),
            Triple(
                BenchmarkDiscoveryOutcome.Loaded,
                BenchmarkAdmissionOutcome.Stopped,
                "Benchmark admission stopped"))
    cases.forEach { (discovery, admission, label) ->
      val status =
          performanceBenchmarkStatusPresentation(
              comparison, comparison.identity(), choice, discovery, admission)
      assertEquals(label, status.stateLabel)
      assertTrue(status.priorEvidence)
      assertFalse(status.summary.contains("does not measure this model suggestion"))
      if (discovery is BenchmarkDiscoveryOutcome.Failed)
          assertEquals(discovery.message, status.summary)
      if (admission is BenchmarkAdmissionOutcome.Failed)
          assertEquals(admission.message, status.summary)
    }
  }

  @Test
  fun discoveryOutcomesKeepExplicitReadOnlyRecoveryAvailableWithoutSelectingOrRunning() {
    val current = benchmarkCandidateFixture()
    val cases =
        listOf(
            BenchmarkEvidenceState(discovery = BenchmarkDiscoveryOutcome.Loading) to
                "Listing · read-only discovery",
            BenchmarkEvidenceState(
                catalog = benchmarkCatalogFixture(current),
                discovery = BenchmarkDiscoveryOutcome.Loaded) to "No compatible benchmarks",
            BenchmarkEvidenceState(
                discovery =
                    BenchmarkDiscoveryOutcome.Unavailable(
                        "Daemon cannot discover this candidate.")) to
                "Daemon cannot discover this candidate.",
            BenchmarkEvidenceState(
                discovery =
                    BenchmarkDiscoveryOutcome.Failed("Lookup timeout; retry explicitly.")) to
                "Lookup timeout; retry explicitly.",
            BenchmarkEvidenceState(discovery = BenchmarkDiscoveryOutcome.Invalidated) to
                "Discovery invalidated")
    cases.forEach { (evidence, text) ->
      var lookups = 0
      var selections = 0
      var runs = 0
      val eligibility =
          benchmarkEligibility(current.copy(review = current.review.copy(benchmark = evidence)))
      ComposeVisualFixture(1600, 1000) {
            PerformanceWorkspacePane(
                PerformanceWorkspacePaneState(
                    performancePageFixture(),
                    null,
                    benchmarkCatalog = evidence.catalog,
                    benchmarkDiscovery = evidence.discovery,
                    benchmarkAdmission = evidence.admission,
                    benchmarkEligibility = eligibility),
                benchmarkActions {}
                    .copy(
                        loadBenchmarks = { lookups++ },
                        selectBenchmark = { selections++ },
                        runBenchmark = { runs++ }))
          }
          .use { fixture ->
            fixture.render()
            fixture.clickDescription("Expand Explore benchmark evidence")
            fixture.render()
            assertTrue(fixture.hasText(text))
            assertFalse(fixture.hasText("Run selected benchmark"))
            assertFalse(fixture.hasText("Trust and run selected benchmark"))
            assertEquals(0, lookups)
            assertEquals(0, selections)
            assertEquals(0, runs)
            assertEquals(
                !eligibility.canDiscover, fixture.isDisabled("Refresh compatible benchmarks"))
            if (eligibility.canDiscover) {
              fixture.clickText("Refresh compatible benchmarks")
              assertEquals(1, lookups)
            }
            assertEquals(0, selections)
            assertEquals(0, runs)
          }
    }
  }

  private fun benchmarkCatalogFixture(current: DesktopState) =
      current.review.draft!!.let { draft ->
        GoBenchmarkCatalog(
            draftId = draft.id,
            draftRevision = draft.revision,
            draftHash = draft.hash,
            projectId = draft.projectId,
            projectRevision = draft.projectRevision,
            baseFileHash = draft.baseFileHash,
            targetPath = draft.targetPath,
            available = true,
            trusted = true)
      }

  private fun benchmarkActions(action: () -> Unit) =
      PerformanceWorkspaceActions(
          prepareOptimization = {},
          openAnalysis = {},
          semanticActions = FindingActions({}, { _, _ -> }, {}),
          openSource = {},
          loadBenchmarks = action,
          selectBenchmark = { action() },
          runBenchmark = action)

  private fun benchmarkCandidateFixture(): DesktopState {
    val draft =
        comparison()
            .identityDraft(
                DeclarationValidation(
                    true,
                    "strict_symbol",
                    diff = UnifiedDiff("internal/work.go", "internal/work.go")))
    return DesktopState(
        projectState =
            ProjectWorkspaceState(
                project =
                    performancePageFixture()
                        .project!!
                        .copy(
                            projectId = draft.projectId, projectRevision = draft.projectRevision)),
        selection =
            FileSelectionState(
                selectedFile =
                    ProjectFileInfo(
                        draft.targetPath,
                        draft.baseFileHash,
                        "work.go",
                        language = "Go",
                        sizeBytes = 1,
                        lineCount = 1,
                        modifiedAt = "",
                        binary = false)),
        review = DraftReviewState(draft = draft, editor = editableDraft(draft)))
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
