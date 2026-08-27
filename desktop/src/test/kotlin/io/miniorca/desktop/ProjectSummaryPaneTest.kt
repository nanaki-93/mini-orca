package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

class ProjectSummaryPaneTest {
    @Test fun deterministicFactsRemainAvailableWhenModelInterpretationIsMissing() {
        val project = ProjectAnalysis("project", "revision", "Mini", "/tmp/project", "go", buildFile = "go.mod", fileCount = 4, sourceFileCount = 3, totalLines = 120, languages = mapOf("Go" to 3), analysisFile = "", summary = "", aiStatus = "", analyzedAt = "")

        val summary = projectSummaryPresentation(null, project)

        assertTrue(summary.hasProject)
        assertEquals("go · go.mod", "${summary.projectType} · ${summary.buildMetadata}")
        assertEquals("4 files · 3 source files · 120 lines", summary.inventory)
        assertTrue(summary.analysisMessage.contains("Deterministic facts remain available"))
    }

    @Test fun modelInterpretationAndFindingSourcesStayExplicit() {
        val overview = ProjectOverview(
            projectId = "project",
            projectRevision = "revision",
            metrics = ProjectMetrics(type = "go", fileCount = 2, sourceFileCount = 2, totalLines = 20),
            analysis = StructuredProjectAnalysis(status = "stale", failure = ""),
            analysisCoverage = AnalysisCoverage(fresh = 1, stale = 1, failed = 1),
            findingCounts = FindingCounts(verified = 2, aiSuggestions = 3),
        )

        val summary = projectSummaryPresentation(overview, null)

        assertEquals("stale", summary.analysisStatus)
        assertTrue(summary.analysisMessage.contains("stale"))
        assertEquals("2 verified · 3 AI suggestions", summary.findings)
        assertTrue(summary.coverage.contains("1 failed"))
    }
}
