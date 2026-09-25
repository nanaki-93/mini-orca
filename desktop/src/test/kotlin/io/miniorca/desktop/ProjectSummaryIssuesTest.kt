package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull

class ProjectSummaryIssuesTest {
  @Test
  fun categoryTintsIdentifyResultsWithoutClaimingSafetyFromZeroCounts() {
    listOf(0, 1, 4, 5, 10, 25).forEach { count ->
      val run =
          analysisRunFixture().let {
            it.copy(sections = it.sections.map { section -> section.copy(findingCount = count) })
          }
      val metrics = summaryIssueMetrics(resultProjectFixture(), run, emptyMap())
      assertEquals(listOf(count, count, count), metrics.map { it.value })
      assertEquals(
          if (count == 0) listOf(FaintText, FaintText, FaintText)
          else listOf(Error, Information, Warning),
          metrics.map(::summaryIssueTint))
    }
    assertEquals(
        listOf(FaintText, FaintText, FaintText),
        summaryIssueMetrics(resultProjectFixture(), null, emptyMap()).map(::summaryIssueTint))
  }

  @Test
  fun bugPriorityCountsRemainAvailableWithoutAnInventedHealthScore() {
    listOf(listOf("high", "low"), listOf("high", "medium"), listOf("high", "high", "high", "low"))
        .forEach { severities ->
          val (run, section) = summaryBugFixture(severities)
          val metric =
              summaryIssueMetrics(resultProjectFixture(), run, bugSections(section)).first()
          assertEquals(severities.size, metric.value)
          assertEquals(severities.count { it == "high" }, metric.priorities?.high)
          assertEquals(severities.count { it == "medium" }, metric.priorities?.medium)
          assertEquals(severities.count { it == "low" }, metric.priorities?.low)
          assertEquals(Error, summaryIssueTint(metric))
        }
  }

  @Test
  fun unknownOrOutdatedPriorityEvidenceCannotInventABreakdown() {
    val (run, section) = summaryBugFixture(listOf("high"))
    val results = section.results!!
    listOf(
            AnalysisSectionState(),
            section.copy(error = "Could not refresh findings"),
            section.copy(results = results.copy(identity = run.identity.copy(generation = "old"))),
            section.copy(
                results = results.copy(progress = results.progress.copy(status = "running"))),
            section.copy(results = results.copy(path = "main.go")),
            section.copy(results = results.copy(semantic = emptyList())),
            section.copy(
                results =
                    results.copy(semantic = results.semantic.map { it.copy(freshness = "stale") })),
            section.copy(
                results =
                    results.copy(
                        semantic = results.semantic.map { it.copy(category = "security") })))
        .forEach { incomplete ->
          val metric =
              summaryIssueMetrics(resultProjectFixture(), run, bugSections(incomplete)).first()
          assertNull(metric.priorities)
          assertEquals(Error, summaryIssueTint(metric))
        }
    val (otherRun, otherSection) = summaryBugFixture(listOf("critical"))
    val other =
        summaryIssueMetrics(resultProjectFixture(), otherRun, bugSections(otherSection)).first()
    assertEquals(1, other.priorities?.other)
    val stale =
        summaryIssueMetrics(
                resultProjectFixture(), run.copy(status = "stale"), bugSections(section))
            .first()
    assertNull(stale.value)
    assertEquals(FaintText, summaryIssueTint(stale))
    assertEquals("Stale", stale.status)
  }

  @Test
  fun zeroReportedBugsRequireMatchingCompletedDetailsBeforeClaimingEmpty() {
    val (fixtureRun, _) = summaryBugFixture(emptyList())
    val run =
        fixtureRun.copy(
            status = "completed",
            sections =
                fixtureRun.sections.map {
                  if (it.category == "bugs") it.copy(status = "completed_empty") else it
                })
    val details =
        AnalysisSectionState(
            results =
                AnalysisSectionResults(run.identity, run.sections.first { it.category == "bugs" }))
    val pending = summaryIssueMetrics(resultProjectFixture(), run, emptyMap()).first()
    assertEquals(0, pending.value)
    assertNull(pending.priorities)
    assertEquals("Completed · details not confirmed", pending.status)
    assertEquals("0 reported · details not confirmed", pending.detailStatus)
    assertEquals("completed", pending.statusCode)
    assertEquals(FaintText, summaryIssueTint(pending))

    val completed = summaryIssueMetrics(resultProjectFixture(), run, bugSections(details)).first()
    assertEquals(SummaryBugPriorities(0, 0, 0, 0), completed.priorities)
    assertEquals("completed_empty", completed.statusCode)
    assertEquals("Completed · no findings", completed.status)
    assertEquals("0 reported", completed.detailStatus)

    val result = requireNotNull(details.results)
    listOf(
            details.copy(loading = true),
            details.copy(error = "Read failed"),
            details.copy(results = result.copy(identity = run.identity.copy(generation = "old"))),
            details.copy(
                results = result.copy(progress = result.progress.copy(status = "partial"))))
        .forEach { incomplete ->
          val metric =
              summaryIssueMetrics(resultProjectFixture(), run, bugSections(incomplete)).first()
          assertEquals(0, metric.value)
          assertNull(metric.priorities)
          assertEquals("completed", metric.statusCode)
          assertEquals("Completed · details not confirmed", metric.status)
        }
  }

  @Test
  fun eachCategoryKeepsRunCountsWhileItsOwnReportDetailsLoadOrFail() {
    val (baseRun, bugs) = summaryBugFixture(listOf("high"))
    val run =
        baseRun.copy(
            sections =
                baseRun.sections.map {
                  when (it.category) {
                    "bugs" -> it.copy(findingCount = 7)
                    "performance" -> it.copy(findingCount = 8)
                    "security" -> it.copy(findingCount = 9)
                    else -> it
                  }
                })
    val sections =
        mapOf(
            AnalysisResultKey("bugs") to bugs,
            AnalysisResultKey("performance") to AnalysisSectionState(loading = true),
            AnalysisResultKey("security") to AnalysisSectionState(error = "Result read failed"))

    val metrics = summaryIssueMetrics(resultProjectFixture(), run, sections)

    assertEquals(listOf(7, 8, 9), metrics.map { it.value })
    assertNull(metrics.first().priorities, "Mismatched evidence cannot classify a newer count")
    assertEquals(
        listOf(
            "7 reported",
            "Loading saved details · 8 reported",
            "Saved details unavailable · 9 reported"),
        metrics.map { it.detailStatus })
  }
}

internal fun bugSections(section: AnalysisSectionState) =
    mapOf(AnalysisResultKey("bugs") to section)

internal fun summaryBugFixture(severities: List<String>): Pair<AnalysisRun, AnalysisSectionState> {
  val run =
      analysisRunFixture().let { run ->
        run.copy(
            sections =
                run.sections.map {
                  if (it.category == "bugs") it.copy(findingCount = severities.size) else it
                })
      }
  val results =
      AnalysisSectionResults(
          identity = run.identity,
          progress = run.sections.first { it.category == "bugs" },
          semantic =
              severities.mapIndexed { index, severity ->
                UnifiedFinding(
                    id = "bug-$index",
                    category = "bugs",
                    severity = severity,
                    projectId = run.identity.projectId,
                    projectRevision = run.identity.projectRevision,
                    freshness = "fresh")
              })
  return run to AnalysisSectionState(results = results)
}
