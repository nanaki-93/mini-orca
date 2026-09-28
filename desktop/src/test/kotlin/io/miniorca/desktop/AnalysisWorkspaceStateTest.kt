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
            fixture.assertTextFits("Ready to analyze", maxLines = if (width == 800) 2 else 1)
            assertFalse(fixture.hasText("Last run · None"))
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
                elapsedSeconds = 125,
                windowElapsedSeconds = 42,
                windowFilesCompleted = 3,
                createdAt = "2026-09-15T14:00:00Z",
                plan = plannedRunFiles("main.go" to listOf("semantic", "performance")),
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
        "Current run · 3 files processed in current window · Reported run time · 125s · Current window · 42s · Created · 2026-09-15T14:00:00Z · Updated · 2026-09-15T15:30:00Z",
        projectRunPresentation(resultProjectFixture(), ProjectAnalysisRunState(run = run)).headline)
    assertEquals(
        "Last run · Paused · 1 of 2 stages · Reported run time · 125s · Current window · 42s · Created · 2026-09-15T14:00:00Z · Updated · 2026-09-15T15:30:00Z",
        projectRunPresentation(
                resultProjectFixture(), ProjectAnalysisRunState(run = run.copy(status = "paused")))
            .headline)
    assertEquals(
        "Last run · 1 of 2 stages · Reported run time · 125s · Current window · 42s · Created · 2026-09-15T14:00:00Z",
        projectRunPresentation(
                resultProjectFixture(),
                ProjectAnalysisRunState(run = run.copy(status = "completed", updatedAt = "")))
            .headline)
    assertEquals(
        "Current run · Reported run time · unavailable",
        projectRunPresentation(
                resultProjectFixture(),
                ProjectAnalysisRunState(
                    run =
                        run.copy(
                            files = emptyList(),
                            elapsedSeconds = 0,
                            windowFilesCompleted = 0,
                            windowElapsedSeconds = 0,
                            createdAt = "",
                            updatedAt = "")))
            .headline)
    listOf("interrupted", "failed", "partial", "completed_empty", "canceled").forEach { status ->
      val metadata = analysisRunTimeMetadata(run.copy(status = status, windowElapsedSeconds = 0))
      assertEquals("Reported run time · 125s", metadata.first())
      assertFalse(metadata.any { it.startsWith("Current window") })
      assertFalse(metadata.any { it.startsWith("Completed ·") })
    }
  }

  @Test
  fun analysisOwnsBoundedRunAndFileStageFailuresWithoutDispatchOnOpen() {
    val failure = "scan\u0000 unavailable\n" + "context ".repeat(700)
    val run =
        analysisRunFixture()
            .copy(
                status = "failed",
                plan = plannedRunFiles("main.go" to listOf("semantic")),
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
          AnalysisWorkspacePane(
              AnalysisWorkspacePaneState(
                  resultProjectFixture(), ProjectAnalysisRunState(run = run)),
              AnalysisWorkspaceActions(
                  { _, _ -> calls++ }, { calls++ }, { calls++ }, { calls++ }, { calls++ }))
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText("Attention · 1 failed"))
          assertFalse(fixture.hasText(failure))
          fixture.revealText("Code analysis · 1/1 finished · 1 failed", "analysis-page")
          fixture.clickDescription("Expand Code analysis · 1/1 finished · 1 failed")
          fixture.render()
          assertTrue(fixture.hasText(failure.replace('\u0000', ' ').take(4_096)))
          assertTrue(fixture.hasText("… output truncated"))
          assertFalse(fixture.hasText(sanitizedOutputText(failure)))
          assertFalse(fixture.hasText(failure))
        }
    assertEquals(
        failure,
        projectRunPresentation(resultProjectFixture(), ProjectAnalysisRunState(run = run))
            .failures
            .single()
            .reason)
  }

  @Test
  fun lifecycleCommandsFollowTheDaemonAndResumeRequiresFreshAdmission() {
    assertEquals(
        listOf(AnalysisRunCommand.Start, AnalysisRunCommand.RetryStaleFailed),
        projectRunPresentation(resultProjectFixture(), ProjectAnalysisRunState()).commands)
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
                  resultProjectFixture(),
                  ProjectAnalysisRunState(run = analysisRunFixture().copy(status = status)))
              .commands)
    }
    listOf("completed", "completed_empty", "failed", "partial", "canceled", "unavailable").forEach {
      assertEquals(
          listOf(AnalysisRunCommand.Start, AnalysisRunCommand.RetryStaleFailed),
          projectRunPresentation(
                  resultProjectFixture(),
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
                plan =
                    plannedRunFiles(
                        "main.go" to
                            listOf("semantic", "performance", "security_source", "security_ai")),
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
    val result = projectRunPresentation(resultProjectFixture(), ProjectAnalysisRunState(run = run))
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
            resultProjectFixture(),
            ProjectAnalysisRunState(
                run =
                    analysisRunFixture()
                        .copy(
                            plan =
                                plannedRunFiles(
                                    *files
                                        .map { it.path to it.stages.map { stage -> stage.stage } }
                                        .toTypedArray()),
                            files = files)))
    assertEquals(6, result.totalFiles)
    assertEquals(3, result.finishedFiles)
    assertEquals(.5f, result.fileProgress)
    assertNull(
        projectRunPresentation(resultProjectFixture(), ProjectAnalysisRunState()).fileProgress)
    val missing =
        projectRunPresentation(
            resultProjectFixture(),
            ProjectAnalysisRunState(
                run =
                    analysisRunFixture()
                        .copy(
                            plan = plannedRunFiles("main.go" to listOf("semantic")),
                            files = emptyList())))
    assertEquals(RunProgressAvailability.Incomplete, missing.progressAvailability)
    assertNull(missing.fileProgress)
    val empty =
        projectRunPresentation(
            resultProjectFixture(),
            ProjectAnalysisRunState(
                run = analysisRunFixture().copy(plan = plannedRunFiles(), files = emptyList())))
    assertEquals(RunProgressAvailability.EmptyScope, empty.progressAvailability)
    assertNull(empty.fileProgress)
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

    val result = projectRunPresentation(resultProjectFixture(), ProjectAnalysisRunState(run = run))

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

    val presentation =
        projectRunPresentation(resultProjectFixture(), ProjectAnalysisRunState(run = run))
    assertEquals(expected, presentation.failures)
    assertEquals("unavailable", presentation.stages[1].files.single().status)
    assertEquals(1, presentation.stages[1].attention)
    assertEquals("interrupted", presentation.stages.last().files.single().status)
    assertEquals(0, presentation.stages.last().finished)
    assertEquals(1, presentation.stages.last().attention)
    assertTrue(
        projectRunPresentation(
                resultProjectFixture(),
                ProjectAnalysisRunState(run = run.copy(plan = analysisPreviewFixture())))
            .failures
            .isEmpty())
  }

  @Test
  fun invalidCapturedEvidenceCannotInflateProgressOrExposeActivePaths() {
    val base =
        analysisRunFixture()
            .copy(
                status = "running",
                plan = plannedRunFiles("main.go" to listOf("semantic", "performance")),
                files =
                    listOf(
                        AnalysisRunFile(
                            "main.go",
                            "base",
                            "Go",
                            listOf(
                                AnalysisStageProgress("semantic", "completed", 1, false),
                                AnalysisStageProgress("performance", "running", 1, false)))))
    val valid = projectRunPresentation(resultProjectFixture(), ProjectAnalysisRunState(run = base))
    assertEquals(RunProgressAvailability.Available, valid.progressAvailability)
    assertEquals(1, valid.finishedSteps)
    assertEquals(listOf("main.go"), valid.currentFiles)
    val invalid =
        listOf(
            base.copy(identity = base.identity.copy(projectId = "other")),
            base.copy(identity = base.identity.copy(projectRevision = "other")),
            base.copy(plan = base.plan.copy(identity = base.plan.identity.copy(queueId = "other"))),
            base.copy(files = base.files.map { it.copy(path = "other.go") }),
            base.copy(files = base.files.map { it.copy(contentHash = "other") }),
            base.copy(files = base.files + base.files.first()),
            base.copy(files = base.files.map { it.copy(stages = it.stages + it.stages.first()) }),
            base.copy(
                files =
                    base.files.map {
                      it.copy(stages = listOf(AnalysisStageProgress("other", "running", 1, false)))
                    }))
    invalid.forEach { run ->
      val result =
          projectRunPresentation(resultProjectFixture(), ProjectAnalysisRunState(run = run))
      assertNull(result.fileProgress)
      assertEquals(0, result.finishedFiles)
      assertTrue(result.currentFiles.isEmpty())
      assertTrue(result.failures.isEmpty())
    }
    val partial =
        projectRunPresentation(
            resultProjectFixture(),
            ProjectAnalysisRunState(
                run = base.copy(files = base.files.map { it.copy(stages = it.stages.take(1)) })))
    assertEquals(RunProgressAvailability.Incomplete, partial.progressAvailability)
    assertEquals(0, partial.finishedFiles)
    assertNull(partial.fileProgress)
    val missingFile =
        projectRunPresentation(
            resultProjectFixture(),
            ProjectAnalysisRunState(
                run =
                    base.copy(
                        plan =
                            plannedRunFiles(
                                "main.go" to listOf("semantic", "performance"),
                                "missing.go" to listOf("semantic")),
                        files =
                            listOf(
                                base.files
                                    .single()
                                    .copy(
                                        stages =
                                            listOf(
                                                AnalysisStageProgress(
                                                    "semantic", "completed", 1, false),
                                                AnalysisStageProgress(
                                                    "performance", "failed", 1, false)))))))
    assertEquals(2, missingFile.totalFiles)
    assertEquals(1, missingFile.finishedFiles)
    assertEquals(3, missingFile.totalSteps)
    assertEquals(2, missingFile.finishedSteps)
    assertEquals(RunProgressAvailability.Incomplete, missingFile.progressAvailability)
    assertNull(missingFile.fileProgress)
    val duplicatePlan =
        projectRunPresentation(
            resultProjectFixture(),
            ProjectAnalysisRunState(
                run =
                    base.copy(
                        plan = base.plan.copy(files = base.plan.files + base.plan.files.first()))))
    assertEquals(RunProgressAvailability.Unavailable, duplicatePlan.progressAvailability)
    assertTrue(duplicatePlan.currentFiles.isEmpty())
  }

  @Test
  fun stageSummariesFollowCapturedPlanAndKeepMixedOutcomesAndDetails() {
    val plan =
        plannedRunFiles(
            "a.go" to listOf("semantic", "performance"),
            "b.go" to listOf("semantic", "performance"),
            "c.go" to listOf("semantic", "performance"),
            "d.go" to listOf("semantic", "performance"))
    val run =
        analysisRunFixture()
            .copy(
                plan = plan,
                files =
                    listOf(
                        AnalysisRunFile(
                            "a.go",
                            "base",
                            "Go",
                            listOf(
                                AnalysisStageProgress("semantic", "completed", 0, true),
                                AnalysisStageProgress("performance", "running", 2, false))),
                        AnalysisRunFile(
                            "b.go",
                            "base",
                            "Go",
                            listOf(
                                AnalysisStageProgress(
                                    "semantic", "partial", 1, false, reason = "Partial evidence"),
                                AnalysisStageProgress("performance", "pending", 0, false))),
                        AnalysisRunFile(
                            "c.go",
                            "base",
                            "Go",
                            listOf(
                                AnalysisStageProgress("semantic", "failed", 3, false),
                                AnalysisStageProgress("performance", "skipped", 0, false))),
                        AnalysisRunFile(
                            "d.go",
                            "base",
                            "Go",
                            listOf(
                                AnalysisStageProgress("semantic", "completed_empty", 0, false),
                                AnalysisStageProgress(
                                    "performance",
                                    "unexpected_state",
                                    1,
                                    false,
                                    reason = "New state")))))
    val result = projectRunPresentation(resultProjectFixture(), ProjectAnalysisRunState(run = run))
    assertEquals(listOf("semantic", "performance"), result.stages.map { it.stage })
    val semantic = result.stages.first()
    assertEquals("4/4 finished · 1 partial · 1 failed", analysisStageBreakdown(semantic))
    assertEquals(4, semantic.total)
    assertEquals(4, semantic.finished)
    assertEquals(2, semantic.attention)
    assertEquals(
        mapOf("completed" to 1, "partial" to 1, "failed" to 1, "completed_empty" to 1),
        semantic.statuses)
    assertEquals(
        AnalysisStageDetail(
            "a.go", "completed", 0, true, true, "No reason was supplied for this stage."),
        semantic.files.first())
    assertEquals("Partial evidence", semantic.files[1].reason)
    assertEquals("No diagnostic was supplied for this stage.", semantic.files[2].reason)
    val performance = result.stages.last()
    assertEquals(
        "1/4 finished · 1 running · 1 pending · 1 skipped · 1 unexpected state",
        analysisStageBreakdown(performance))
    assertEquals(1, performance.running)
    assertEquals(1, performance.pending)
    assertEquals(1, performance.finished)
    assertEquals(2, performance.attention)
    assertEquals("unexpected_state", performance.files.last().status)
    assertEquals("New state", performance.files.last().reason)
    assertEquals("No diagnostic was supplied for this stage.", result.failures.single().reason)
    assertEquals(
        0,
        result.stages.sumOf {
          it.files.count { detail -> detail.reused == true && detail.path != "a.go" }
        })
  }

  @Test
  fun stageSummariesRetainPlanIneligibilityAndMissingEvidenceWithoutInventingReuse() {
    val plan =
        plannedRunFiles("main.go" to listOf("semantic", "security_rules")).let { preview ->
          preview.copy(
              files =
                  preview.files.map { file ->
                    file.copy(
                        stages =
                            file.stages.map { stage ->
                              if (stage.stage == "security_rules")
                                  stage.copy(
                                      eligible = false,
                                      cached = true,
                                      reason = "Requires Go rules.")
                              else stage.copy(cached = true)
                            })
                  })
        }
    val run =
        analysisRunFixture()
            .copy(
                plan = plan,
                files =
                    listOf(
                        AnalysisRunFile(
                            "main.go",
                            "base",
                            "Go",
                            listOf(
                                AnalysisStageProgress("semantic", "pending", 0, false),
                                AnalysisStageProgress("security_rules", "unavailable", 0, false)))))
    val result = projectRunPresentation(resultProjectFixture(), ProjectAnalysisRunState(run = run))
    assertEquals(listOf("semantic", "security_rules"), result.stages.map { it.stage })
    assertEquals(1, result.stages.first().pending)
    assertEquals(false, result.stages.first().files.single().reused)
    val ineligible = result.stages.last()
    assertEquals(1, ineligible.finished)
    assertEquals(0, ineligible.attention)
    assertEquals("1/1 finished · 1 not applicable", analysisStageBreakdown(ineligible))
    assertEquals("Requires Go rules.", ineligible.files.single().reason)
    assertEquals(false, ineligible.files.single().eligible)
    assertTrue(result.failures.isEmpty())

    val missing =
        projectRunPresentation(
            resultProjectFixture(),
            ProjectAnalysisRunState(
                run = run.copy(files = listOf(run.files.single().copy(stages = emptyList())))))
    assertEquals(RunProgressAvailability.Incomplete, missing.progressAvailability)
    assertEquals("0/1 finished · 1 unreported", analysisStageBreakdown(missing.stages.first()))
    assertEquals(1, missing.stages.first().missing)
    assertEquals(0, missing.stages.first().pending)
    assertNull(missing.stages.first().files.single().attempts)
    assertNull(missing.stages.first().files.single().reused)
    assertEquals(
        "Stage progress has not been reported.", missing.stages.first().files.single().reason)
    assertEquals("Requires Go rules.", missing.stages.last().files.single().reason)
  }

  @Test
  fun stageSummariesExcludeInvalidEvidenceAndDoNotInventAbsentStages() {
    val base =
        analysisRunFixture()
            .copy(
                plan = plannedRunFiles("main.go" to listOf("semantic")),
                files =
                    listOf(
                        AnalysisRunFile(
                            "main.go",
                            "base",
                            "Go",
                            listOf(AnalysisStageProgress("semantic", "running", 1, true)))))
    val invalid =
        listOf(
            base.copy(identity = base.identity.copy(projectRevision = "other")),
            base.copy(plan = base.plan.copy(identity = base.plan.identity.copy(queueId = "other"))),
            base.copy(files = base.files.map { it.copy(contentHash = "other") }),
            base.copy(files = base.files + base.files.first()),
            base.copy(files = base.files.map { it.copy(stages = it.stages + it.stages.first()) }),
            base.copy(
                files =
                    base.files.map {
                      it.copy(
                          stages =
                              it.stages + AnalysisStageProgress("security_ai", "running", 1, false))
                    }))
    invalid.forEach { candidate ->
      val result =
          projectRunPresentation(resultProjectFixture(), ProjectAnalysisRunState(run = candidate))
      assertTrue(result.stages.flatMap { it.files }.none { it.status == "running" })
      assertTrue(result.stages.flatMap { it.files }.none { it.reused == true })
    }
    val limited =
        projectRunPresentation(resultProjectFixture(), ProjectAnalysisRunState(run = base))
    assertEquals(listOf("semantic"), limited.stages.map { it.stage })
    assertEquals(1, limited.stages.single().running)
    val unknownStage =
        base.copy(
            plan = plannedRunFiles("main.go" to listOf("future_stage")),
            files =
                listOf(
                    AnalysisRunFile(
                        "main.go",
                        "base",
                        "Go",
                        listOf(AnalysisStageProgress("future_stage", "new_status", 1, false)))))
    val unknown =
        projectRunPresentation(resultProjectFixture(), ProjectAnalysisRunState(run = unknownStage))
    assertEquals("future_stage", unknown.stages.single().stage)
    assertEquals(mapOf("new_status" to 1), unknown.stages.single().statuses)
    assertEquals(1, unknown.stages.single().attention)
    assertEquals(0, unknown.stages.single().finished)
    assertTrue(
        projectRunPresentation(resultProjectFixture(), ProjectAnalysisRunState()).stages.isEmpty())
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
    assertEquals(
        "1/1 stages covered", analysisCoverageLabel(AnalysisRunCoverage(total = 1, succeeded = 1)))
    assertNull(analysisCoverageLabel(null))
    assertNull(analysisCoverageLabel(AnalysisRunCoverage()))
  }

  @Test
  fun emptyResultsKeepLifecycleAndPendingDetailsDistinct() {
    fun projected(status: String, reportedCount: Int? = null): AnalysisResultPageState {
      val original = resultPageFixture("bugs")
      val progress =
          requireNotNull(original.progress).copy(status = status, findingCount = reportedCount)
      val run =
          requireNotNull(original.run)
              .copy(
                  status = status,
                  sections =
                      original.run.sections.map { if (it.category == "bugs") progress else it })
      return original.copy(
          run = run,
          section =
              AnalysisSectionState(
                  results =
                      requireNotNull(original.results)
                          .copy(progress = progress, semantic = emptyList())))
    }

    assertEquals(
        AnalysisResultAvailability.NoProject,
        resultPageFixture("bugs")
            .copy(project = null, run = null)
            .emptyPresentation(0)
            .availability)
    assertEquals(
        AnalysisResultAvailability.NotStarted,
        resultPageFixture("bugs").copy(run = null).emptyPresentation(0).availability)
    assertEquals(
        AnalysisResultAvailability.Loading,
        projected("running")
            .copy(section = AnalysisSectionState(loading = true))
            .emptyPresentation(0)
            .availability)
    assertEquals(
        AnalysisResultAvailability.Running, projected("running").emptyPresentation(0).availability)
    assertEquals("Analysis is in progress.", projected("running").emptyPresentation(0).message)
    assertTrue(projected("running").emptyPresentation(0).detail.contains("stages covered"))
    assertEquals(
        "0 findings reported so far; results are not final. · 1/1 stages covered",
        projected("running", reportedCount = 0).emptyPresentation(0).detail)
    assertEquals(
        AnalysisResultAvailability.PendingDetails,
        projected("running", reportedCount = 2).emptyPresentation(0).availability)
    assertEquals(
        "No results loaded yet.",
        projected("running", reportedCount = 2).emptyPresentation(0).message)
    assertEquals(
        AnalysisResultAvailability.Paused,
        projected("paused", reportedCount = 2).emptyPresentation(0).availability)
    assertEquals(
        AnalysisResultAvailability.Interrupted,
        projected("interrupted", reportedCount = 2).emptyPresentation(0).availability)
    assertEquals(
        AnalysisResultAvailability.Partial,
        projected("partial", reportedCount = 2).emptyPresentation(0).availability)
    assertEquals(
        AnalysisResultAvailability.Failed,
        projected("failed", reportedCount = 2).emptyPresentation(0).availability)
    assertEquals(
        AnalysisResultAvailability.Canceled,
        projected("canceled", reportedCount = 2).emptyPresentation(0).availability)
    assertEquals(
        AnalysisResultAvailability.Unavailable,
        projected("unavailable", reportedCount = 2).emptyPresentation(0).availability)
    listOf("paused", "interrupted", "partial", "failed", "canceled", "cancelled", "unavailable")
        .forEach { status ->
          val incomplete = projected(status, reportedCount = 0)
          assertFalse(
              incomplete.emptyPresentation(0).availability ==
                  AnalysisResultAvailability.CompletedEmpty)
          assertEquals("0 reported", incomplete.countLabel(0))
        }
  }

  @Test
  fun completedEmptyRequiresMatchingCurrentZeroDetailAndTimeStaysProjectScoped() {
    fun completedPage(reportedCount: Int?): AnalysisResultPageState {
      val original = resultPageFixture("bugs")
      val progress =
          requireNotNull(original.progress).copy(status = "completed", findingCount = reportedCount)
      val run =
          requireNotNull(original.run)
              .copy(
                  status = "completed",
                  elapsedSeconds = 42,
                  sections =
                      original.run.sections.map { if (it.category == "bugs") progress else it })
      return original.copy(
          run = run,
          section =
              AnalysisSectionState(
                  results =
                      requireNotNull(original.results)
                          .copy(progress = progress, semantic = emptyList())))
    }

    val completed = completedPage(0)
    assertEquals(
        AnalysisResultAvailability.CompletedEmpty, completed.emptyPresentation(0).availability)
    assertEquals("Run time · 42s", completed.runTimeLabel)
    assertEquals(
        AnalysisResultAvailability.PendingDetails,
        completed.copy(section = AnalysisSectionState()).emptyPresentation(0).availability)
    assertEquals(
        AnalysisResultAvailability.PendingDetails,
        completedPage(null).emptyPresentation(0).availability)
    assertEquals(
        AnalysisResultAvailability.PendingDetails,
        completedPage(2).emptyPresentation(0).availability)
    assertEquals("0 findings", completed.countLabel(0))
    assertEquals("— reported (count unavailable)", completedPage(null).countLabel(0))
    assertEquals("2 reported", completedPage(2).countLabel(0))
    assertEquals("0 reported", completed.copy(section = AnalysisSectionState()).countLabel(0))
    assertNull(
        completed
            .copy(
                run =
                    completed.run!!.copy(
                        identity = completed.run.identity.copy(projectId = "other")))
            .runTimeLabel)

    val stale = completed.copy(project = completed.project!!.copy(projectRevision = "next"))
    assertEquals(AnalysisResultAvailability.Stale, stale.emptyPresentation(0).availability)
    assertEquals("Run time · 42s", stale.runTimeLabel)
    assertEquals(
        AnalysisResultAvailability.Error,
        completed
            .copy(section = AnalysisSectionState(error = "refresh failed"))
            .emptyPresentation(0)
            .availability)
  }

  @Test
  fun completedEmptyNeedsMatchingSuccessfulDetailsNotJustAReportedZero() {
    val original = resultPageFixture("bugs")
    val progress = original.progress!!.copy(status = "completed_empty", findingCount = 0)
    val run =
        original.run!!.copy(
            status = "completed_empty",
            sections = original.run.sections.map { if (it.category == "bugs") progress else it })
    val results = original.results!!.copy(progress = progress, semantic = emptyList())
    val page = original.copy(run = run, section = AnalysisSectionState(results = results))
    assertEquals(AnalysisResultAvailability.CompletedEmpty, page.emptyPresentation(0).availability)
    assertEquals("0 findings", page.countLabel(0))
    val missingOrMismatched =
        listOf(
            page.copy(section = AnalysisSectionState()),
            page.copy(
                section =
                    page.section.copy(
                        results = results.copy(progress = progress.copy(status = "partial")))),
            page.copy(
                section =
                    page.section.copy(
                        results = results.copy(progress = progress.copy(findingCount = null)))),
            page.copy(section = page.section.copy(results = results.copy(path = "main.go"))),
            page.copy(
                section =
                    page.section.copy(
                        results = results.copy(identity = results.identity.copy(id = "other")))),
            page.copy(
                section =
                    page.section.copy(
                        results =
                            results.copy(semantic = requireNotNull(original.results).semantic))),
            page.copy(section = page.section.copy(loading = true)))
    missingOrMismatched.forEach { candidate ->
      assertFalse(
          candidate.emptyPresentation(0).availability == AnalysisResultAvailability.CompletedEmpty)
      assertEquals("0 reported", candidate.countLabel(0))
    }
    val error = page.copy(section = AnalysisSectionState(error = ""))
    assertEquals(AnalysisResultAvailability.Error, error.emptyPresentation(0).availability)
    assertEquals(
        "The saved result read failed without a diagnostic.", error.emptyPresentation(0).detail)
    assertEquals("0 reported", error.countLabel(0))
    assertEquals(
        "The saved result read failed without a diagnostic.",
        page
            .copy(section = AnalysisSectionState(error = ""))
            .copy(
                run =
                    run.copy(
                        sections =
                            run.sections.map {
                              if (it.category == "bugs") it.copy(findingCount = null) else it
                            }))
            .emptyPresentation(0)
            .detail)

    listOf("performance", "security").forEach { category ->
      val initial = resultPageFixture(category)
      val zero = initial.progress!!.copy(status = "completed_empty", findingCount = 0)
      val categoryRun =
          initial.run!!.copy(
              status = "completed_empty",
              sections = initial.run.sections.map { if (it.category == category) zero else it })
      val initialDetails = requireNotNull(initial.results)
      val details =
          initialDetails.copy(
              progress = zero,
              performance = initialDetails.performance.map { it.copy(status = "completed_empty") },
              security =
                  initialDetails.security.map {
                    it.copy(status = "completed_empty", findings = emptyList())
                  })
      val verified =
          initial.copy(run = categoryRun, section = AnalysisSectionState(results = details))
      assertEquals(
          AnalysisResultAvailability.CompletedEmpty, verified.emptyPresentation(0).availability)
      val incomplete =
          details.copy(
              performance = details.performance.map { it.copy(status = "failed") },
              security = details.security.map { it.copy(status = "partial") })
      assertEquals(
          AnalysisResultAvailability.PendingDetails,
          verified
              .copy(section = AnalysisSectionState(results = incomplete))
              .emptyPresentation(0)
              .availability)
    }
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

private fun plannedRunFiles(vararg files: Pair<String, List<String>>): AnalysisRunPreview =
    analysisPreviewFixture()
        .copy(
            files =
                files.map { (path, stages) ->
                  AnalysisPlannedFile(
                      path,
                      "base",
                      "Go",
                      20,
                      stages.map {
                        AnalysisStagePlan(it, eligible = true, cached = false, maxModelRequests = 0)
                      })
                })

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
