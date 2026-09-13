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
    assertEquals("go · go.mod", "${summary.projectType} · ${summary.buildMetadata}")
    assertEquals("Go", summary.languages)
    assertEquals(listOf(4, 120), summary.projectMetrics.map { it.value })
    assertTrue(summary.findingMetrics.all { it.value == null })
    assertTrue(summary.coverageMetrics.all { it.value == null })
    assertFalse(summary.toString().contains("revision"))
    assertEquals("Project analysis: unavailable", summary.analysisMessage)
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
                    entryPoints = listOf("cmd/main.go"),
                    nextSteps = listOf("Add validation."),
                    risks = listOf(ProjectAnalysisRisk("medium", "Validate inputs."))),
            analysisCoverage = AnalysisCoverage(fresh = 1, stale = 1, failed = 1),
            findingCounts = FindingCounts(verified = 2, aiSuggestions = 3),
        )

    val summary = projectSummaryPresentation(overview, null)

    assertEquals("stale", summary.analysisStatus)
    assertEquals("Coordinate requests through one handler.", summary.purpose)
    assertTrue(summary.analysisMessage.contains("source may have changed"))
    assertEquals(listOf(2, 20), summary.projectMetrics.map { it.value })
    assertEquals(listOf(2, 3), summary.findingMetrics.map { it.value })
    assertEquals("Tool-reported issues", summary.findingMetrics.first().label)
    assertEquals(
        listOf("Ready", "Stale", "Failed"),
        summary.coverageMetrics.map { it.label },
    )
    assertEquals(listOf(1, 1, 1), summary.coverageMetrics.map { it.value })
    assertEquals(
        listOf("Architecture", "Packages / modules", "Risks · AI suggestions"),
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
    assertTrue(unavailable.findingMetrics.all { it.value == null })
    assertTrue(unavailable.coverageMetrics.all { it.value == null })
    assertTrue(zeroes.hasProject)
    assertEquals(listOf(0, 0), zeroes.projectMetrics.map { it.value })
    assertEquals(listOf(0, 0), zeroes.findingMetrics.map { it.value })
    assertTrue(zeroes.coverageMetrics.isEmpty())
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

    assertTrue(running.analysisMessage.contains(": running"))
    assertEquals("Project analysis: failed · Timed out.", failed.analysisMessage)
    assertEquals(null, failed.purpose)
  }

  @Test
  fun diagramInputsPreserveMermaidAndLegacyProse() {
    val mermaid = "flowchart TD\n A[API] --> B[Service]"
    assertEquals(mermaid, summaryDiagramInput(mermaid).source)
    assertEquals(
        SummaryDiagramInput(mermaid, "Overview"),
        summaryDiagramInput("Overview\n```mermaid\n$mermaid\n```"))
    assertTrue(
        summaryDiagramInput("API → Service").source!!.contains("n0[\"API\"] --> n1[\"Service\"]"))
    listOf("Handlers call services.", "API ->", "`API -> Service`", "API -> Service\nwith details")
        .forEach { assertEquals(SummaryDiagramInput(null, it), summaryDiagramInput(it)) }
  }

  @Test
  fun moduleNamesComeBeforeTheirPathsWithoutLosingDescriptions() {
    assertEquals(
        SummaryModule("Workflow orchestration", "internal/app", "Coordinates changes."),
        summaryModule("`internal/app` (Workflow orchestration): Coordinates changes."))
    assertEquals(SummaryModule("API", "internal/api", ""), summaryModule("internal/api (API)"))
    assertEquals(SummaryModule("Legacy component", null, ""), summaryModule("Legacy component"))
    assertEquals(SummaryModule("path (unfinished", null, ""), summaryModule("path (unfinished"))
  }

  @Test
  fun coverageMetricsKeepFiveColumnsWhenWideAndWrapAtNarrowWidths() {
    assertEquals(5, summaryMetricColumnCount(1040.dp))
    assertEquals(5, summaryMetricColumnCount(480.dp))
    assertEquals(3, summaryMetricColumnCount(479.dp))
    assertEquals(3, summaryMetricColumnCount(320.dp))
    assertEquals(2, summaryMetricColumnCount(319.dp))
  }
}
