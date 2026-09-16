package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

class AnalysisWorkspaceStateTest {

  @Test
  fun retryButtonPreviewsOnlyStaleAndFailedFilesAtSupportedSizes() {
    for ((width, height, scale) in
        listOf(
            Triple(1440, 900, 1f),
            Triple(1000, 760, 1f),
            Triple(999, 760, 1f),
            Triple(800, 650, 1.5f),
            Triple(1280, 600, 1.5f))) {
      val starts = mutableListOf<Pair<AnalysisRunLimits, Boolean>>()
      ComposeVisualFixture(width, height, scale) {
            AnalysisWorkspacePane(
                AnalysisWorkspacePaneState(resultProjectFixture(), ProjectAnalysisRunState()),
                AnalysisWorkspaceActions(
                    { limits, retry -> starts.add(limits to retry) }, {}, {}, {}, {}))
          }
          .use { fixture ->
            fixture.render("analysis-retry-$width-$scale")
            fixture.assertTextFits("Start analysis")
            fixture.assertTextFits("Analyze stale & failed")
            assertFalse(fixture.hasText("Run limits"))
            fixture.assertTextFits("Last run · None")
            assertTrue(starts.isEmpty())
            fixture.clickText("Analyze stale & failed")
            assertEquals(listOf(AnalysisRunLimits(100, 900, 2) to true), starts)
            fixture.clickText("Start analysis")
            assertEquals(false, starts.last().second)
            assertEquals(AnalysisRunLimits(100, 900, 2), starts.last().first)
          }
    }
  }

  @Test
  fun headerUsesStoredProgressAndTimeWithoutInventingMissingFacts() {
    val run =
        analysisRunFixture()
            .copy(
                status = "running",
                windowElapsedSeconds = 42,
                windowFilesCompleted = 3,
                updatedAt = "2026-09-15T15:30:00Z",
                files =
                    listOf(
                        AnalysisRunFile(
                            "main.go",
                            "base",
                            "Go",
                            listOf(
                                AnalysisStageProgress("semantic", "completed", 1, false),
                                AnalysisStageProgress("performance", "pending", 0, false)))))
    assertEquals(
        "Current run · 3 files processed · 42s elapsed",
        projectRunPresentation(ProjectAnalysisRunState(run = run)).headline)
    assertEquals(
        "Last run · Paused · 1 of 2 stages · 2026-09-15T15:30:00Z",
        projectRunPresentation(ProjectAnalysisRunState(run = run.copy(status = "paused"))).headline)
    assertEquals(
        "Last run · 1 of 2 stages",
        projectRunPresentation(
                ProjectAnalysisRunState(run = run.copy(status = "completed", updatedAt = "")))
            .headline)
    assertEquals(
        "Current run",
        projectRunPresentation(
                ProjectAnalysisRunState(
                    run =
                        run.copy(
                            files = emptyList(),
                            windowFilesCompleted = 0,
                            windowElapsedSeconds = 0)))
            .headline)
  }

  @Test
  fun analysisOwnsBoundedRunAndFileStageFailuresWithoutDispatchOnOpen() {
    val failure = "scan\u0000 unavailable\n" + "context ".repeat(700)
    val run =
        analysisRunFixture()
            .copy(
                status = "failed",
                files =
                    listOf(
                        AnalysisRunFile(
                            "main.go",
                            "base",
                            "Go",
                            listOf(
                                AnalysisStageProgress(
                                    "semantic", "failed", 2, false, reason = failure)))))
    var calls = 0
    ComposeVisualFixture(800, 650, 1.5f) {
          AnalysisWorkspacePane(
              AnalysisWorkspacePaneState(
                  resultProjectFixture(),
                  ProjectAnalysisRunState(run = run, error = "Cannot resume\u0000 run")),
              AnalysisWorkspaceActions(
                  { _, _ -> calls++ }, { calls++ }, { calls++ }, { calls++ }, { calls++ }))
        }
        .use { fixture ->
          fixture.render("bottom-analysis-failures-800-1.5")
          assertTrue(fixture.hasText("Cannot resume  run"))
          assertEquals(0, calls)
        }
    ComposeVisualFixture(800, 650) {
          AnalysisFailureDetails(AnalysisStageFailure("main.go", "semantic", 2, failure))
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText(sanitizedOutputText(failure)))
          assertFalse(fixture.hasText(failure))
        }
    assertEquals(
        failure,
        projectRunPresentation(ProjectAnalysisRunState(run = run)).failures.single().reason)
  }

  @Test
  fun lifecycleCommandsFollowTheDaemonAndResumeRequiresFreshAdmission() {
    assertEquals(
        listOf(AnalysisRunCommand.Start, AnalysisRunCommand.RetryStaleFailed),
        projectRunPresentation(ProjectAnalysisRunState()).commands)
    val expected =
        mapOf(
            "running" to listOf(AnalysisRunCommand.Pause, AnalysisRunCommand.Cancel),
            "queued" to listOf(AnalysisRunCommand.Pause, AnalysisRunCommand.Cancel),
            "pausing" to listOf(AnalysisRunCommand.Cancel),
            "canceling" to emptyList(),
            "paused" to listOf(AnalysisRunCommand.Resume, AnalysisRunCommand.Cancel),
            "interrupted" to listOf(AnalysisRunCommand.Resume, AnalysisRunCommand.Cancel),
            "stale" to
                listOf(
                    AnalysisRunCommand.Start,
                    AnalysisRunCommand.RetryStaleFailed,
                    AnalysisRunCommand.Cancel))
    expected.forEach { (status, commands) ->
      assertEquals(
          commands,
          projectRunPresentation(
                  ProjectAnalysisRunState(run = analysisRunFixture().copy(status = status)))
              .commands)
    }
    listOf("completed", "completed_empty", "failed", "partial", "canceled", "unavailable").forEach {
      assertEquals(
          listOf(AnalysisRunCommand.Start, AnalysisRunCommand.RetryStaleFailed),
          projectRunPresentation(
                  ProjectAnalysisRunState(run = analysisRunFixture().copy(status = it)))
              .commands)
    }
  }

  @Test
  fun summaryProgressUsesOnlyTheCurrentRunAndItsExistingLifecycleStates() {
    val project = resultProjectFixture()
    val run = analysisRunFixture()

    listOf("queued", "running", "pausing", "canceling", "paused", "interrupted").forEach { status ->
      assertTrue(run.copy(status = status).showsProgressOnSummary())
    }
    listOf("stale", "failed", "partial", "canceled", "completed", "completed_empty").forEach {
        status ->
      assertFalse(run.copy(status = status).showsProgressOnSummary())
    }
    assertEquals(run, currentProjectRun(run, project))
    assertNull(
        currentProjectRun(run.copy(identity = run.identity.copy(projectId = "other")), project))
    assertNull(
        currentProjectRun(run.copy(identity = run.identity.copy(projectRevision = "old")), project))
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
    assertEquals(0f, result.fileProgress)
    assertEquals(listOf("main.go"), result.currentFiles)
    assertEquals(
        AnalysisStageFailure("main.go", "security_source", 2, "source scanner unavailable"),
        result.failures.single())
  }

  @Test
  fun fileProgressCountsOnlyFilesWhoseStagesFinishedWithoutCallingThemSuccessful() {
    val files =
        listOf(
                listOf("completed", "completed_empty"),
                listOf("completed", "failed"),
                listOf("partial", "skipped"),
                listOf("completed", "running"),
                listOf("pending"),
                emptyList())
            .mapIndexed { index, states ->
              AnalysisRunFile(
                  "file$index.go",
                  "base",
                  "Go",
                  states.mapIndexed { stage, status ->
                    AnalysisStageProgress("stage$stage", status, 1, false)
                  })
            }
    val result =
        projectRunPresentation(
            ProjectAnalysisRunState(run = analysisRunFixture().copy(files = files)))
    assertEquals(6, result.totalFiles)
    assertEquals(3, result.finishedFiles)
    assertEquals(.5f, result.fileProgress)
    assertNull(projectRunPresentation(ProjectAnalysisRunState()).fileProgress)
    assertNull(
        projectRunPresentation(
                ProjectAnalysisRunState(run = analysisRunFixture().copy(files = emptyList())))
            .fileProgress)
  }

  @Test
  fun stagesUnavailableByPlanDoNotAppearAsOperationalFailures() {
    val files =
        listOf(
                "README.md" to "Markdown",
                ".gitignore" to "Text",
                "config.yaml" to "YAML",
                "package.json" to "JSON",
                "settings.xml" to "XML",
                "App.kt" to "Kotlin")
            .map { (path, language) ->
              AnalysisPlannedFile(
                  path,
                  "base",
                  language,
                  20,
                  listOf(
                      AnalysisStagePlan(
                          "security_source",
                          eligible = false,
                          cached = false,
                          reason = "Passive security rules require a Go source file.",
                          maxModelRequests = 0)))
            }
    val run =
        analysisRunFixture()
            .copy(
                plan = analysisPreviewFixture().copy(files = files),
                files =
                    files.map { file ->
                      AnalysisRunFile(
                          file.path,
                          file.contentHash,
                          file.language,
                          file.stages.map {
                            AnalysisStageProgress(
                                it.stage, "unavailable", 0, false, reason = it.reason)
                          })
                    })

    val result = projectRunPresentation(ProjectAnalysisRunState(run = run))

    assertTrue(result.failures.isEmpty())
    assertEquals(files.size, result.totalSteps)
    assertEquals(files.size, result.finishedSteps)
    assertEquals(1f, result.fileProgress)
  }

  @Test
  fun operationalFailuresRemainVisibleIncludingUnavailableEligibleStagesWithoutAttempts() {
    val stages =
        listOf(
            AnalysisStageProgress("semantic", "failed", 2, false, reason = "Request failed."),
            AnalysisStageProgress(
                "performance", "unavailable", 0, false, reason = "Model not configured."),
            AnalysisStageProgress(
                "security_source", "failed", 0, false, reason = "Source scanner failed."),
            AnalysisStageProgress(
                "security_ai", "interrupted", 1, false, reason = "Request interrupted."))
    val run =
        analysisRunFixture()
            .copy(
                plan =
                    analysisPreviewFixture()
                        .copy(
                            files =
                                listOf(
                                    AnalysisPlannedFile(
                                        "main.go",
                                        "base",
                                        "Go",
                                        20,
                                        stages.map {
                                          AnalysisStagePlan(
                                              it.stage,
                                              eligible = true,
                                              cached = false,
                                              maxModelRequests = 0)
                                        }))),
                files = listOf(AnalysisRunFile("main.go", "base", "Go", stages)))
    val expected = stages.map { AnalysisStageFailure("main.go", it.stage, it.attempts, it.reason) }

    assertEquals(expected, projectRunPresentation(ProjectAnalysisRunState(run = run)).failures)
    assertEquals(
        expected,
        projectRunPresentation(
                ProjectAnalysisRunState(run = run.copy(plan = analysisPreviewFixture())))
            .failures)
  }

  @Test
  fun wholeProjectCoverageKeepsUnknownCountsDistinctFromZero() {
    val page = resultPageFixture("bugs")
    assertEquals(1, page.reportedCount)
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
      AnalysisResultType.fromCategory(category),
      resultProjectFixture(),
      run,
      AnalysisSectionState(results = analysisResultsFixture(run, category)))
}
