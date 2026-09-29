package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertIs
import kotlin.test.assertTrue

class ProjectSummaryEvidenceTest {
  private fun state(
      vararg pages: AnalysisResultPageState,
      verified: List<UnifiedFinding> = emptyList()
  ): DesktopState {
    val run = pages.first().run
    return DesktopState(
        projectState = ProjectWorkspaceState(project = pages.first().project),
        analysisRun =
            ProjectAnalysisRunState(
                run = run,
                sections = pages.associate { AnalysisResultKey(it.category) to it.section }),
        findings = FindingsState(findings = verified))
  }

  @Test
  fun previewsSortBeforeBoundingAndKeepCountsAndExactProducerIdentity() {
    val bugs = resultPageFixture("bugs")
    val performance = performancePageFixture()
    val security = securityPageFixture()
    val base = bugs.results!!
    val sample =
        UnifiedFinding(
            id = "same",
            projectId = "project",
            projectRevision = "revision",
            category = "bugs",
            confidence = "suggested",
            title = "Review",
            location = FindingLocation(path = "b.go", startLine = 2))
    val semantic =
        listOf("z.go", "b.go", "a.go").map {
          sample.copy(location = sample.location.copy(path = it))
        }
    val verified =
        listOf("v.go", "c.go").map {
          sample.copy(
              confidence = "tool_reported",
              source = "vet",
              location = sample.location.copy(path = it))
        }
    val bugPage =
        bugs.copy(
            section =
                bugs.section.copy(
                    results = base.copy(semantic = semantic, unclassified = emptyList())))
    val first = summaryFindingPreview(state(bugPage, performance, security, verified = verified))
    val shuffled =
        summaryFindingPreview(
            state(
                bugPage.copy(
                    section =
                        bugPage.section.copy(
                            results =
                                base.copy(
                                    semantic = semantic.reversed(), unclassified = emptyList()))),
                performance,
                security,
                verified = verified.reversed()))
    assertEquals(first.rows, shuffled.rows)
    assertEquals(8, first.loadedCount)
    assertEquals(5, first.rows.size)
    assertEquals(
        listOf("a.go:2", "b.go:2", "c.go:2", "v.go:2", "z.go:2"), first.rows.map { it.location })
    assertEquals(listOf(5, 1, 2), first.categories.map { it.loadedCount })
    assertEquals(5, first.rows.map { it.target.rowKey }.distinct().size)
    assertTrue(first.rows[2].target.producer is SummaryFindingProducer.Verified)
    assertEquals(bugs.run!!.identity, first.rows[2].target.run)
    assertEquals(bugs.run.identity, first.rows.first().target.run)
    assertEquals("project", first.rows.first().target.projectId)
  }

  @Test
  fun rejectsUnmatchedAndHistoricalDetailsWithoutReinterpretingReportedCounts() {
    val bugs = resultPageFixture("bugs")
    val performance = performancePageFixture()
    val security = securityPageFixture()
    val foreign =
        UnifiedFinding(
            id = "foreign",
            projectId = "elsewhere",
            projectRevision = "revision",
            category = "bugs",
            confidence = "tool_reported")
    val history = foreign.copy(projectId = "project", category = "", id = "history")
    val brokenBugs =
        bugs.copy(
            section =
                bugs.section.copy(
                    results =
                        bugs.results!!.copy(
                            unclassified = listOf(history), semantic = listOf(foreign))))
    val mismatched =
        performance.copy(
            section =
                performance.section.copy(
                    results =
                        performance.results!!.copy(
                            progress = performance.progress!!.copy(findingCount = 99))))
    val preview =
        summaryFindingPreview(state(brokenBugs, mismatched, security, verified = listOf(foreign)))
    assertEquals(2, preview.loadedCount)
    assertEquals(listOf(0, 0, 2), preview.categories.map { it.loadedCount })
    assertFalse(preview.rows.any { it.target.category == AnalysisResultType.Bugs })
    assertTrue(preview.categories[1].detail!!.contains("not loaded"))
  }

  @Test
  fun retainedFailuresAndOriginsDoNotBecomeMeasuredOrAssured() {
    val bugs = resultPageFixture("bugs")
    val performance = performancePageFixture()
    val security = securityPageFixture()
    val stale =
        bugs.copy(
            run = bugs.run!!.copy(status = "stale"),
            section = bugs.section.copy(error = "disk failed"))
    val failed = performance.copy(section = performance.section.copy(error = "read failed"))
    val securityDetails = requireNotNull(security.results)
    val source = securityDetails.security.first()
    val securityWithMissingOrigin =
        security.copy(
            section =
                security.section.copy(
                    results =
                        securityDetails.copy(
                            security = securityDetails.security + source.copy(source = "unknown"))))
    // A shared run can be partial; report-level stale/partial still qualifies retained evidence.
    val preview = summaryFindingPreview(state(bugs, performance, securityWithMissingOrigin))
    assertEquals(
        "Model suggestion · unmeasured recommendation",
        preview.rows.first { it.target.category == AnalysisResultType.Performance }.origin)
    assertTrue(preview.rows.any { it.origin == "Source rule" })
    assertTrue(preview.rows.any { it.origin == "Model hypothesis" })
    assertTrue(preview.rows.any { it.origin == "Evidence type unavailable" })
    assertTrue(
        preview.rows
            .filter { it.target.category == AnalysisResultType.Security }
            .all { "Partial" in it.materialState })
    val retained = summaryFindingPreview(state(stale, performance, security))
    assertEquals("Stale", retained.categories.first().status)
    assertTrue(retained.categories.first().detail!!.contains("disk failed"))
    assertTrue(
        retained.rows
            .first { it.target.category == AnalysisResultType.Bugs }
            .materialState
            .contains("Saved details unavailable"))
    val failedRun = bugs.run.copy(status = "failed")
    val failedPreview =
        summaryFindingPreview(state(bugs.copy(run = failedRun), performance, security))
    assertTrue(
        failedPreview.rows
            .first { it.target.category == AnalysisResultType.Bugs }
            .materialState
            .contains("Failed"))
    assertTrue(
        summaryFindingPreview(state(bugs, failed, security))
            .categories[1]
            .detail!!
            .contains("read failed"))
  }

  @Test
  fun verifiedBugsKeepTheirOwnMaterialStateWhenAnalysisRunOrDetailReadFails() {
    val bugs = resultPageFixture("bugs")
    val verified =
        UnifiedFinding(
            id = "tool-bug",
            projectId = "project",
            projectRevision = "revision",
            category = "bugs",
            confidence = "tool_reported",
            source = "vet",
            location = FindingLocation("tool.go"),
            freshness = "fresh",
            status = "open")
    for (runStatus in listOf("stale", "failed", "canceled")) {
      val page =
          bugs.copy(
              run = bugs.run!!.copy(status = runStatus),
              section = bugs.section.copy(error = "saved detail read failed"))
      val preview = summaryFindingPreview(state(page, verified = listOf(verified)))
      val toolRow = preview.rows.single { it.target.producer is SummaryFindingProducer.Verified }
      val semanticRow =
          preview.rows.single { it.target.producer is SummaryFindingProducer.Semantic }
      assertEquals(page.run!!.identity, toolRow.target.run)
      assertEquals("", toolRow.materialState, "verified finding with $runStatus run")
      assertTrue("Saved details unavailable" in semanticRow.materialState)
      assertTrue("Partial" in semanticRow.materialState)
      if (runStatus == "stale") assertTrue("Stale" in semanticRow.materialState)
      else assertTrue(analysisStatusLabel(runStatus) in semanticRow.materialState)
      assertTrue(preview.categories.first().detail!!.contains("saved detail read failed"))
    }
    val ownFailure =
        summaryFindingPreview(
            state(
                bugs.copy(run = bugs.run!!.copy(status = "stale")),
                verified = listOf(verified.copy(status = "failed", freshness = "stale"))))
    assertEquals(
        "Failed · Stale",
        ownFailure.rows
            .single { it.target.producer is SummaryFindingProducer.Verified }
            .materialState)
  }

  @Test
  fun semanticAndTypedRowsKeepReportAndRunIdentityAndCanceledQualifications() {
    val bugs = resultPageFixture("bugs")
    val performance = performancePageFixture()
    val security = securityPageFixture()
    val run =
        bugs.run!!.copy(
            status = "canceled", sections = bugs.run.sections.map { it.copy(status = "canceled") })
    val semantic =
        UnifiedFinding(
            id = "same",
            projectId = "project",
            projectRevision = "revision",
            category = "performance",
            title = "Check cost",
            location = FindingLocation("other.go"),
            freshness = "stale",
            confidence = "suggested")
    val perfDetails = requireNotNull(performance.results)
    val perf =
        performance.copy(
            section =
                performance.section.copy(
                    results =
                        perfDetails.copy(
                            progress = run.sections.first { it.category == "performance" },
                            semantic = listOf(semantic))))
    val secDetails = requireNotNull(security.results)
    val sec =
        security.copy(
            section =
                security.section.copy(
                    results =
                        secDetails.copy(
                            progress = run.sections.first { it.category == "security" })))
    val currentBugs =
        bugs.copy(
            run = run,
            section =
                bugs.section.copy(
                    results =
                        requireNotNull(bugs.results)
                            .copy(progress = run.sections.first { it.category == "bugs" })))
    val preview = summaryFindingPreview(state(currentBugs, perf, sec))
    assertEquals(5, preview.loadedCount)
    assertEquals("Canceled", preview.categories[1].status)
    val semanticRow =
        preview.rows.single {
          it.target.producer is SummaryFindingProducer.Semantic &&
              it.target.category == AnalysisResultType.Performance
        }
    assertEquals(run.identity, semanticRow.target.run)
    assertTrue("Stale" in semanticRow.materialState)
    assertTrue("Canceled" in semanticRow.materialState)
    val perfRow = preview.rows.single { it.target.producer is SummaryFindingProducer.Performance }
    assertEquals(
        SummaryFindingProducer.Performance("main.go", "base", "perf", "", "", ""),
        perfRow.target.producer)
    assertTrue("Canceled" in perfRow.materialState)
    val securityRows = preview.rows.filter { it.target.producer is SummaryFindingProducer.Security }
    assertEquals(
        setOf("ai", "deterministic"),
        securityRows.map { (it.target.producer as SummaryFindingProducer.Security).source }.toSet())
    assertTrue(securityRows.all { "Partial" in it.materialState && "Canceled" in it.materialState })
  }

  @Test
  fun verifiedFindingTargetIsBoundToTheOriginatingRunEvenWhenTheFindingPersists() {
    val bugs = resultPageFixture("bugs")
    val verified =
        UnifiedFinding(
            id = "tool-bug",
            projectId = "project",
            projectRevision = "revision",
            category = "bugs",
            confidence = "tool_reported",
            source = "vet",
            location = FindingLocation("tool.go"))
    val original = state(bugs, verified = listOf(verified))
    val target =
        summaryFindingPreview(original)
            .rows
            .single { it.target.producer is SummaryFindingProducer.Verified }
            .target
    assertEquals(bugs.run!!.identity, target.run)
    assertIs<ExplicitResultTarget.Resolved>(resolveSummaryTarget(target, original))
    val replaced =
        bugs.copy(run = bugs.run.copy(identity = bugs.run.identity.copy(generation = "new")))
    val newState = state(replaced, verified = listOf(verified))
    assertTrue(summaryFindingPreview(newState).rows.any { it.target.rowKey == target.rowKey })
    assertIs<ExplicitResultTarget.Unavailable>(resolveSummaryTarget(target, newState))
  }

  @Test
  fun resolutionRejectsChangedReportsAndAmbiguousKeysWithoutGuessing() {
    val bugs = resultPageFixture("bugs")
    val performance = performancePageFixture()
    val security = securityPageFixture()
    val original = state(bugs, performance, security)
    val target =
        summaryFindingPreview(original)
            .rows
            .single { it.target.producer is SummaryFindingProducer.Performance }
            .target
    assertIs<ExplicitResultTarget.Resolved>(resolveSummaryTarget(target, original))
    val details = requireNotNull(performance.results)
    val report = details.performance.single()
    val changed =
        performance.copy(
            section =
                performance.section.copy(
                    results = details.copy(performance = listOf(report.copy(model = "changed")))))
    assertIs<ExplicitResultTarget.Unavailable>(
        resolveSummaryTarget(target, state(bugs, changed, security)))
    val duplicated =
        performance.copy(
            section =
                performance.section.copy(
                    results = details.copy(performance = listOf(report, report))))
    val ambiguous = resolveSummaryTarget(target, state(bugs, duplicated, security))
    assertIs<ExplicitResultTarget.Unavailable>(ambiguous)
    assertTrue("Multiple" in ambiguous.reason)
    val removed =
        performance.copy(
            section = performance.section.copy(results = details.copy(performance = emptyList())))
    assertIs<ExplicitResultTarget.Unavailable>(
        resolveSummaryTarget(target, state(bugs, removed, security)))
    assertIs<ExplicitResultTarget.Unavailable>(
        resolveSummaryTarget(target.copy(projectRevision = "old"), original))
  }

  @Test
  fun typedResultOrderIsIndependentOfReportOrderAndAmbiguousKeysAreNotCollapsed() {
    val bugs = resultPageFixture("bugs")
    val performance = performancePageFixture()
    val security = securityPageFixture()
    val perfDetails = requireNotNull(performance.results)
    val report = perfDetails.performance.single()
    val extra = report.copy(path = "a.go", findings = report.findings.map { it.copy(id = "perf") })
    val perf =
        performance.copy(
            section =
                performance.section.copy(
                    results = perfDetails.copy(performance = listOf(report, extra))))
    val secDetails = requireNotNull(security.results)
    val first = summaryFindingPreview(state(bugs, perf, security))
    val reversed =
        summaryFindingPreview(
            state(
                bugs,
                perf.copy(
                    section =
                        perf.section.copy(
                            results = perfDetails.copy(performance = listOf(extra, report)))),
                security.copy(
                    section =
                        security.section.copy(
                            results = secDetails.copy(security = secDetails.security.reversed())))))
    assertEquals(first, reversed)
    assertEquals(5, first.loadedCount)
    assertEquals(5, first.rows.size)
    assertEquals(2, first.categories[1].loadedCount)
    assertEquals("a.go:4 · Run", first.rows[1].location)
    val duplicate =
        perf.copy(
            section =
                perf.section.copy(results = perfDetails.copy(performance = listOf(report, report))))
    val duplicated = summaryFindingPreview(state(bugs, duplicate, security))
    assertEquals(5, duplicated.loadedCount)
    val key = "performance:${report.path}:${report.findings.single().id}"
    assertEquals(2, duplicated.rows.count { it.target.rowKey == key })
    val changedReport = report.copy(model = "new-model")
    val changed =
        summaryFindingPreview(
            state(
                bugs,
                perf.copy(
                    section =
                        perf.section.copy(
                            results = perfDetails.copy(performance = listOf(changedReport)))),
                security))
    assertEquals(key, changed.rows.first { it.target.rowKey == key }.target.rowKey)
    assertTrue(
        first.rows.first { it.target.rowKey == key }.target.producer !=
            changed.rows.first { it.target.rowKey == key }.target.producer)
  }
}
