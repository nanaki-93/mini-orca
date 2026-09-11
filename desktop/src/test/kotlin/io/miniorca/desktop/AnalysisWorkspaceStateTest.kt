package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

class AnalysisWorkspaceStateTest {
  @Test
  fun lifecycleCommandsFollowTheDaemonAndResumeRequiresFreshAdmission() {
    assertEquals(
        listOf(AnalysisRunCommand.Start),
        projectRunPresentation(ProjectAnalysisRunState()).commands)
    val expected =
        mapOf(
            "running" to listOf(AnalysisRunCommand.Pause, AnalysisRunCommand.Cancel),
            "queued" to listOf(AnalysisRunCommand.Pause, AnalysisRunCommand.Cancel),
            "pausing" to listOf(AnalysisRunCommand.Cancel),
            "canceling" to emptyList(),
            "paused" to listOf(AnalysisRunCommand.Resume, AnalysisRunCommand.Cancel),
            "interrupted" to listOf(AnalysisRunCommand.Resume, AnalysisRunCommand.Cancel),
            "stale" to listOf(AnalysisRunCommand.Start, AnalysisRunCommand.Cancel))
    expected.forEach { (status, commands) ->
      assertEquals(
          commands,
          projectRunPresentation(
                  ProjectAnalysisRunState(run = analysisRunFixture().copy(status = status)))
              .commands)
    }
    listOf("completed", "completed_empty", "failed", "partial", "canceled", "unavailable").forEach {
      assertEquals(
          listOf(AnalysisRunCommand.Start),
          projectRunPresentation(
                  ProjectAnalysisRunState(run = analysisRunFixture().copy(status = it)))
              .commands)
    }
  }

  @Test
  fun overallProgressCountsUniqueStagesAndRetainsFailuresAndCurrentPaths() {
    val run =
        analysisRunFixture()
            .copy(
                status = "running",
                files =
                    listOf(
                        AnalysisRunFile(
                            "main.go",
                            "base",
                            "Go",
                            listOf(
                                AnalysisStageProgress("semantic", "completed", 1, false),
                                AnalysisStageProgress("performance", "running", 1, false),
                                AnalysisStageProgress(
                                    "security_source",
                                    "failed",
                                    2,
                                    false,
                                    reason = "source scanner unavailable"),
                                AnalysisStageProgress("security_ai", "pending", 0, false)))))
    val result = projectRunPresentation(ProjectAnalysisRunState(run = run))
    assertEquals(4, result.totalSteps)
    assertEquals(2, result.finishedSteps)
    assertEquals(0.5f, result.progress)
    assertEquals(listOf("main.go"), result.currentFiles)
    assertEquals(
        AnalysisStageFailure("main.go", "security_source", 2, "source scanner unavailable"),
        result.failures.single())
  }

  @Test
  fun localFiltersNeverChangeCoverageAndUnknownCountsAreNotZero() {
    val page = resultPageFixture("bugs")
    assertEquals(1, page.reportedCount)
    assertTrue(page.copy(path = "missing").semantic.isEmpty())
    assertEquals(page.progress, page.copy(path = "missing").progress)
    assertEquals(page.reportedCount, page.copy(path = "missing").reportedCount)
    assertNull(
        page
            .copy(
                run =
                    page.run!!.copy(
                        sections = page.run.sections.map { it.copy(findingCount = null) }))
            .reportedCount)
    assertNull(page.copy(run = null).reportedCount)
    assertTrue(analysisCoverageLabel(null).contains("not available"))
  }

  @Test
  fun staleAndForeignEvidenceCannotAppearAsCurrentResults() {
    val page = resultPageFixture("bugs")
    val stale = page.copy(project = page.project!!.copy(projectRevision = "next"))
    assertEquals("Stale", stale.statusLabel)
    assertNull(stale.reportedCount)
    assertEquals("stale", stale.semantic.single().freshness)
    val foreign =
        page.copy(run = page.run!!.copy(identity = page.run.identity.copy(projectId = "other")))
    assertNull(foreign.results)
    assertNull(foreign.progress)
    assertTrue(foreign.semantic.isEmpty())
    assertNull(
        page
            .copy(section = page.section.copy(results = page.results!!.copy(path = "main.go")))
            .results)
    val wrongCategory = page.results!!.semantic.single().copy(category = "security")
    assertTrue(
        page
            .copy(
                section =
                    page.section.copy(
                        results = page.results!!.copy(semantic = listOf(wrongCategory))))
            .semantic
            .isEmpty())
    assertFalse(page.stale)
  }
}

internal fun resultProjectFixture() =
    ProjectAnalysis(
        "project",
        "revision",
        "Mini-Orca fixture",
        "/tmp/fixture",
        "Go",
        fileCount = 2,
        sourceFileCount = 2,
        totalLines = 20,
        summary = "",
        aiStatus = "",
        analyzedAt = "")

internal fun resultIndexFixture() =
    ProjectIndex(
        "project",
        "revision",
        files =
            listOf(
                IndexedFile(
                    "main.go",
                    "base",
                    "Go",
                    false,
                    lineCount = 20,
                    symbols =
                        listOf(SymbolInfo("Run", "function", "func Run()", 2, 10, "exact", true)))))

internal fun resultPageFixture(category: String): AnalysisResultPageState {
  val run =
      analysisRunFixture()
          .copy(
              status = "partial",
              sections = analysisRunFixture().sections.map { it.copy(status = "partial") })
  return AnalysisResultPageState(
      category,
      resultProjectFixture(),
      run,
      AnalysisSectionState(results = analysisResultsFixture(run, category)))
}
