package io.miniorca.desktop

import androidx.compose.ui.unit.dp
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class ProjectSummaryPaneTest {
  @Test
  fun deterministicFactsRemainAvailableWhenModelInterpretationIsMissing() {
    val project =
        ProjectAnalysis(
            "project",
            "revision",
            "Mini",
            "/tmp/project",
            "go",
            buildFile = "go.mod",
            fileCount = 4,
            sourceFileCount = 3,
            totalLines = 120,
            languages = mapOf("Go" to 3),
            summary = "",
            aiStatus = "",
            analyzedAt = "")

    val summary = projectSummaryPresentation(null, project)

    assertTrue(summary.hasProject)
    assertEquals("Mini", summary.projectName)
    assertEquals("go · go.mod", "${summary.projectType} · ${summary.buildMetadata}")
    assertEquals("Go", summary.languages)
    assertEquals(listOf(4, 120, null, null), summary.projectMetrics.map { it.value })
    assertTrue(summary.coverageMetrics.all { it.value == null })
    assertFalse(summary.toString().contains("revision"))
    assertTrue(summary.analysisMessage.contains("Deterministic facts remain available"))
  }

  @Test
  fun interpretationDetailsAndFindingSourcesStayExplicit() {
    val overview =
        ProjectOverview(
            projectId = "project",
            projectRevision = "revision",
            metrics =
                ProjectMetrics(type = "go", fileCount = 2, sourceFileCount = 2, totalLines = 20),
            analysis =
                StructuredProjectAnalysis(
                    status = "stale",
                    purpose = "Coordinate requests through one handler.",
                    architecture = "Handlers call services.",
                    components = listOf("Handlers"),
                    risks = listOf(ProjectAnalysisRisk("medium", "Validate inputs."))),
            analysisCoverage = AnalysisCoverage(fresh = 1, stale = 1, failed = 1),
            findingCounts = FindingCounts(verified = 2, aiSuggestions = 3),
        )

    val summary = projectSummaryPresentation(overview, null)

    assertEquals("stale", summary.analysisStatus)
    assertEquals("Coordinate requests through one handler.", summary.purpose)
    assertTrue(summary.analysisMessage.contains("source may have changed"))
    assertEquals(listOf(2, 20, 2, 3), summary.projectMetrics.map { it.value })
    assertEquals(
        listOf("Fresh", "Stale", "Missing", "Running", "Failed"),
        summary.coverageMetrics.map { it.label },
    )
    assertEquals(listOf(1, 1, 0, 0, 1), summary.coverageMetrics.map { it.value })
    assertEquals(
        listOf("Architecture", "Components", "Risks · AI suggestions"),
        summary.details.map { it.title })
    assertEquals("MEDIUM · Validate inputs.", summary.details.last().values.single())
  }

  @Test
  fun unavailableCountsStayDistinctFromGenuineZeros() {
    val unavailable = projectSummaryPresentation(null, null)
    val zeroes =
        projectSummaryPresentation(
            ProjectOverview(
                metrics = ProjectMetrics(fileCount = 0, totalLines = 0),
                analysisCoverage = AnalysisCoverage(),
                findingCounts = FindingCounts()),
            null)

    assertFalse(unavailable.hasProject)
    assertTrue(unavailable.projectMetrics.all { it.value == null })
    assertTrue(unavailable.coverageMetrics.all { it.value == null })
    assertTrue(zeroes.hasProject)
    assertEquals(listOf(0, 0, 0, 0), zeroes.projectMetrics.map { it.value })
    assertEquals(listOf(0, 0, 0, 0, 0), zeroes.coverageMetrics.map { it.value })
  }

  @Test
  fun missingRunningAndFailedInterpretationStatesKeepFailureMeaning() {
    val running =
        projectSummaryPresentation(
            ProjectOverview(analysis = StructuredProjectAnalysis(status = "running")), null)
    val failed =
        projectSummaryPresentation(
            ProjectOverview(
                analysis = StructuredProjectAnalysis(status = "failed", failure = "Timed out.")),
            null)

    assertTrue(running.analysisMessage.contains("is running"))
    assertEquals("Timed out.", failed.analysisMessage)
    assertEquals(null, failed.purpose)
  }

  @Test
  fun summaryMetricsUseTheSharedWideAndNarrowGridPolicy() {
    assertEquals(5, summaryMetricColumnCount(1040.dp))
    assertEquals(4, summaryMetricColumnCount(780.dp))
    assertEquals(2, summaryMetricColumnCount(779.dp))
  }
}
