package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull

class ProjectSummaryIssuesTest {
  @Test
  fun issueCountsUseInclusiveTrafficLightBoundaries() {
    listOf(0 to Success, 4 to Success, 5 to Warning, 9 to Warning, 10 to Error, 25 to Error)
        .forEach { (count, tint) ->
          val run =
              analysisRunFixture().let { run ->
                run.copy(sections = run.sections.map { it.copy(findingCount = count) })
              }
          val metrics = summaryIssueMetrics(resultProjectFixture(), run, AnalysisSectionState())
          metrics.drop(1).forEach {
            assertEquals(count, it.score)
            assertEquals(tint, summaryIssueTint(it.score))
          }
        }
    assertEquals(FaintText, summaryIssueTint(null))
  }

  @Test
  fun bugsShowPriorityCountsAndUseWeightedPointsInsteadOfIssueCount() {
    listOf(
            listOf("high", "low") to (4 to Success),
            listOf("high", "medium") to (5 to Warning),
            listOf("high", "high", "high") to (9 to Warning),
            listOf("high", "high", "high", "low") to (10 to Error))
        .forEach { (severities, expected) ->
          val (run, section) = summaryBugFixture(severities)
          val metric = summaryIssueMetrics(resultProjectFixture(), run, section).first()
          assertEquals(severities.size, metric.value)
          assertEquals(severities.count { it == "high" }, metric.priorities?.high)
          assertEquals(severities.count { it == "medium" }, metric.priorities?.medium)
          assertEquals(severities.count { it == "low" }, metric.priorities?.low)
          assertEquals(expected.first, metric.score)
          assertEquals(expected.second, summaryIssueTint(metric.score))
        }
  }

  @Test
  fun unknownOrOutdatedPriorityEvidenceCannotProduceAGreenScore() {
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
          val metric = summaryIssueMetrics(resultProjectFixture(), run, incomplete).first()
          assertNull(metric.priorities)
          assertNull(metric.score)
          assertEquals(FaintText, summaryIssueTint(metric.score))
        }
    val (otherRun, otherSection) = summaryBugFixture(listOf("critical"))
    val other = summaryIssueMetrics(resultProjectFixture(), otherRun, otherSection).first()
    assertEquals(1, other.priorities?.other)
    assertNull(other.score)
    val stale =
        summaryIssueMetrics(resultProjectFixture(), run.copy(status = "stale"), section).first()
    assertNull(stale.score)
    assertEquals("Stale", stale.status)
  }

  @Test
  fun completedEmptyBugsHaveZeroPrioritiesAndZeroScore() {
    val (run, _) = summaryBugFixture(emptyList())
    val metric = summaryIssueMetrics(resultProjectFixture(), run, AnalysisSectionState()).first()
    assertEquals(SummaryBugPriorities(0, 0, 0, 0), metric.priorities)
    assertEquals(0, metric.score)
    assertEquals(Success, summaryIssueTint(metric.score))
  }
}

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
