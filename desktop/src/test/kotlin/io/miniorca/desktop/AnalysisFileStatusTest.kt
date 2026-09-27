package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class AnalysisFileStatusTest {
  @Test
  fun missingStagesNeverLookUpdatedAndEveryCauseIsVisible() {
    val file =
        AnalysisSelectableFile(
            "main.go",
            "",
            listOf(
                AnalysisFileStageStatus("semantic", "fresh", "Current"),
                AnalysisFileStageStatus("performance", "failed", "The attempt limit was reached."),
                AnalysisFileStageStatus("security_rules", "skipped", "Not eligible for rules."),
                AnalysisFileStageStatus(
                    "security_ai", "unavailable", "The model is not configured.")))
    val result = analysisFileStatus(file)
    assertEquals(AnalysisFileSyncStatus.Failed, result.status)
    assertTrue(result.explanation.contains("The attempt limit was reached."))
    assertTrue(result.explanation.contains("The model is not configured."))
    assertEquals(
        AnalysisFileSyncStatus.Unknown,
        analysisFileStatus(AnalysisSelectableFile("unknown.go", "")).status)
    val current =
        analysisFileStatus(
            file.copy(
                stages =
                    file.stages.map {
                      if (it.status == "skipped") it else it.copy(status = "fresh")
                    }))
    assertFalse(current.needsAttention)
  }

  @Test
  fun userAndPolicyExclusionsStayOutsideFreshnessCountsAndCanBeRestored() {
    val current =
        AnalysisSelectableFile(
            "current.go", "", listOf(AnalysisFileStageStatus("semantic", "fresh", "Current")))
    val old =
        AnalysisSelectableFile(
            "old.go", "", listOf(AnalysisFileStageStatus("semantic", "stale", "Source changed")))
    val updated = analysisFileStatus(current)
    val outdated = analysisFileStatus(old)
    val ignored = analysisFileStatus(old, excluded = true)
    val policyExcluded = analysisFileStatus(AnalysisSelectableFile(".env", "Secret file"))
    val rows = listOf(updated, ignored, policyExcluded)
    assertEquals(AnalysisFileSyncStatus.Excluded, ignored.status)
    assertEquals("Excluded by you.", ignored.explanation)
    assertFalse(ignored.needsAttention)
    assertFalse(policyExcluded.needsAttention)
    assertEquals(
        listOf(ignored, policyExcluded),
        filteredAnalysisFiles(rows, "", AnalysisFileFilter.Excluded))
    assertEquals(listOf(updated), filteredAnalysisFiles(rows, "", AnalysisFileFilter.Updated))
    assertTrue(filteredAnalysisFiles(rows, "", AnalysisFileFilter.Attention).isEmpty())
    assertEquals(listOf(ignored), filteredAnalysisFiles(rows, "OLD", AnalysisFileFilter.All))
    assertEquals(
        AnalysisFileSyncStatus.Excluded, analysisFileStatus(current, excluded = true).status)
    assertEquals(outdated, analysisFileStatus(ignored.file, excluded = false))
    assertEquals(
        listOf(outdated),
        filteredAnalysisFiles(
            listOf(updated, outdated, policyExcluded), "", AnalysisFileFilter.Attention))
  }

  @Test
  fun fileStatesKeepTheirDistinctReasonsAndSemanticTints() {
    assertEquals(Success, AnalysisFileSyncStatus.Updated.tint)
    assertEquals(Warning, AnalysisFileSyncStatus.Stale.tint)
    assertEquals(Warning, AnalysisFileSyncStatus.Pending.tint)
    assertEquals(Error, AnalysisFileSyncStatus.Failed.tint)
    assertEquals(Information, AnalysisFileSyncStatus.Running.tint)
    mapOf(
            "missing" to AnalysisFileSyncStatus.Missing,
            "paused" to AnalysisFileSyncStatus.Paused,
            "pending" to AnalysisFileSyncStatus.Pending,
            "canceled" to AnalysisFileSyncStatus.Canceled,
            "interrupted" to AnalysisFileSyncStatus.Interrupted,
            "partial" to AnalysisFileSyncStatus.Partial,
            "running" to AnalysisFileSyncStatus.Running)
        .forEach { (status, expected) ->
          val row =
              analysisFileStatus(
                  AnalysisSelectableFile(
                      "main.go",
                      "",
                      listOf(AnalysisFileStageStatus("semantic", status, "Reason for $status"))))
          assertEquals(expected, row.status)
          assertTrue(row.explanation.contains("Reason for $status"))
        }
  }

  @Test
  fun savedCoverageCountsMatchInspectableRowsAndExcludeEveryKindOfExclusion() {
    val states =
        listOf(
            "fresh" to AnalysisCoverageBucket.UpToDate,
            "stale" to AnalysisCoverageBucket.Outdated,
            "missing" to AnalysisCoverageBucket.NotAnalyzed,
            "running" to AnalysisCoverageBucket.Running,
            "pending" to AnalysisCoverageBucket.Running,
            "failed" to AnalysisCoverageBucket.Failed,
            "partial" to AnalysisCoverageBucket.Incomplete,
            "paused" to AnalysisCoverageBucket.Incomplete,
            "interrupted" to AnalysisCoverageBucket.Incomplete,
            "canceled" to AnalysisCoverageBucket.Incomplete,
            "unavailable" to AnalysisCoverageBucket.Unavailable,
            "future_status" to AnalysisCoverageBucket.Unavailable)
    val files =
        states.mapIndexed { index, (status, _) ->
          AnalysisSelectableFile(
              "src/$index.go",
              "",
              listOf(AnalysisFileStageStatus("semantic", status, "Cause: $status")))
        } +
            listOf(
                AnalysisSelectableFile("no-stages.go", ""),
                AnalysisSelectableFile(
                    "policy.go", "Excluded by policy", selectionStageFixture("fresh", "Current")),
                AnalysisSelectableFile(
                    "skipped.go", "", selectionStageFixture("skipped", "Not applicable")),
                AnalysisSelectableFile("user.go", "", selectionStageFixture("fresh", "Current")))
    val selection =
        selectionFixture()
            .copy(files = files, excludedPaths = listOf("user.go", "absent-from-inventory.go"))
    val rows = analysisSelectionCoverageRows(selection)
    assertEquals(
        states.map { it.second } + AnalysisCoverageBucket.Unavailable, rows.map { it.bucket })
    assertEquals(files.size - 3, rows.size)
    states.forEachIndexed { index, (status, _) ->
      assertEquals("src/$index.go", rows[index].saved.file.path)
      if (status !in setOf("fresh", "future_status"))
          assertTrue(rows[index].saved.explanation.contains("Cause: $status"))
    }
    assertEquals(AnalysisFileSyncStatus.Unknown, rows[11].saved.status)
    assertEquals(AnalysisFileSyncStatus.Unknown, rows.last().saved.status)
    assertTrue(rows.last().saved.explanation.contains("No analysis status is available"))
    assertEquals(
        AnalysisCoverage(
            total = 13,
            fresh = 1,
            stale = 1,
            missing = 1,
            running = 2,
            failed = 1,
            partial = 4,
            unavailable = 3),
        analysisSelectionCoverage(selection))
    val restored = selection.copy(excludedPaths = listOf("absent-from-inventory.go"))
    assertEquals(
        AnalysisFileSyncStatus.Updated, analysisSelectionCoverageRows(restored).last().saved.status)
    assertEquals(2, analysisSelectionCoverage(restored).fresh)
    assertEquals(14, analysisSelectionCoverage(restored).total)
  }

  @Test
  fun fullyExcludedSelectionHasNoSavedCoverageRows() {
    val selection =
        selectionFixture()
            .copy(
                excludedPaths = listOf("helper.go", "main.go", "absent.go"),
                files =
                    selectionFixture().files +
                        AnalysisSelectableFile(
                            "skipped.go", "", selectionStageFixture("skipped", "Not applicable")))
    assertTrue(analysisSelectionCoverageRows(selection).isEmpty())
    assertEquals(AnalysisCoverage(), analysisSelectionCoverage(selection))
  }

  @Test
  fun runProgressCannotUpgradeSavedCoverageOrInspectableReasons() {
    val selection = selectionFixture()
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
                            listOf(AnalysisStageProgress("semantic", "pending", 0, false)))))
    val savedRows = analysisSelectionCoverageRows(selection)
    assertEquals(AnalysisFileSyncStatus.Pending, analysisFileStatuses(selection, run).last().status)
    assertEquals(AnalysisFileSyncStatus.Missing, savedRows.last().saved.status)
    assertTrue(savedRows.last().saved.explanation.contains("No saved analysis exists"))
    val finished =
        run.copy(
            files =
                run.files.map {
                  it.copy(stages = it.stages.map { stage -> stage.copy(status = "completed") })
                })
    assertEquals(
        AnalysisFileSyncStatus.Finished, analysisFileStatuses(selection, finished).last().status)
    assertEquals(savedRows, analysisSelectionCoverageRows(selection))
    assertEquals(
        AnalysisCoverage(total = 2, fresh = 1, missing = 1), analysisSelectionCoverage(selection))
  }

  @Test
  fun liveRunRowsKeepSavedFreshnessAndRequireMatchingAdmission() {
    val selection = selectionFixture()
    val initial =
        analysisRunFixture()
            .copy(
                status = "running",
                files =
                    listOf(
                        AnalysisRunFile(
                            "main.go",
                            "base",
                            "Go",
                            listOf(AnalysisStageProgress("semantic", "running", 1, false)))))
    val running = analysisFileStatuses(selection, initial).last()
    assertEquals(AnalysisFileSyncStatus.Running, running.status)
    assertEquals(AnalysisFileSyncStatus.Missing, running.savedStatus)
    assertTrue(running.explanation.contains("Running"))
    listOf(
            initial.copy(identity = initial.identity.copy(projectId = "other")),
            initial.copy(identity = initial.identity.copy(projectRevision = "other")),
            initial.copy(
                plan = initial.plan.copy(identity = initial.plan.identity.copy(queueId = "other"))),
            initial.copy(files = initial.files.map { it.copy(contentHash = "other") }),
            initial.copy(plan = initial.plan.copy(files = emptyList())),
            initial.copy(status = "completed"),
            initial.copy(status = "canceled"),
            initial.copy(status = "stale"))
        .forEach { run ->
          assertEquals(
              analysisFileStatus(selection.files.last()),
              analysisFileStatuses(selection, run).last())
        }
    assertEquals(
        AnalysisFileSyncStatus.Excluded,
        analysisFileStatuses(selection.copy(excludedPaths = listOf("main.go")), initial)
            .last()
            .status)
    val completed =
        initial.copy(
            files =
                initial.files.map {
                  it.copy(stages = it.stages.map { it.copy(status = "completed") })
                })
    assertEquals(
        AnalysisFileSyncStatus.Finished, analysisFileStatuses(selection, completed).last().status)
    val fresh =
        selection.copy(
            files =
                selection.files.map { it.copy(stages = selectionStageFixture("fresh", "Current")) })
    assertEquals(
        AnalysisFileSyncStatus.Updated, analysisFileStatuses(fresh, completed).last().status)
    val pending =
        initial.copy(
            files =
                initial.files.map {
                  it.copy(stages = it.stages.map { it.copy(status = "pending") })
                })
    assertEquals(
        AnalysisFileSyncStatus.Pending, analysisFileStatuses(selection, pending).last().status)
    assertEquals(
        AnalysisFileSyncStatus.Paused,
        analysisFileStatuses(selection, pending.copy(status = "paused")).last().status)
    assertEquals(
        AnalysisFileSyncStatus.Interrupted,
        analysisFileStatuses(selection, pending.copy(status = "interrupted")).last().status)
    assertEquals(
        AnalysisFileSyncStatus.Running, analysisFileStatuses(selection, initial).last().status)
  }

  @Test
  fun ineligibleRunStagesDoNotTurnCurrentSavedFilesIntoFailures() {
    val selected =
        selectionFixture().copy(files = listOf(selectionFixture().files[1].copy(path = "main.go")))
    val original = analysisRunFixture()
    val run =
        original.copy(
            status = "running",
            plan =
                original.plan.copy(
                    files =
                        listOf(
                            AnalysisPlannedFile(
                                "main.go",
                                "base",
                                "Go",
                                20,
                                listOf(
                                    AnalysisStagePlan(
                                        "semantic", true, false, maxModelRequests = 1),
                                    AnalysisStagePlan(
                                        "security_source",
                                        false,
                                        false,
                                        reason = "Not applicable",
                                        maxModelRequests = 0))))),
            files =
                listOf(
                    AnalysisRunFile(
                        "main.go",
                        "base",
                        "Go",
                        listOf(
                            AnalysisStageProgress("semantic", "completed", 1, false),
                            AnalysisStageProgress(
                                "security_source",
                                "unavailable",
                                0,
                                false,
                                reason = "Not applicable")))))
    assertEquals(
        AnalysisFileSyncStatus.Updated, analysisFileStatuses(selected, run).single().status)
  }

  @Test
  fun runFailuresUnknownStagesAndSavedSourceChangesRemainDistinct() {
    val selection =
        selectionFixture()
            .copy(
                files =
                    listOf(
                        selectionFixture()
                            .files
                            .last()
                            .copy(stages = selectionStageFixture("stale", "Source changed."))))
    val statuses =
        mapOf(
            "failed" to AnalysisFileSyncStatus.Failed,
            "partial" to AnalysisFileSyncStatus.Partial,
            "unavailable" to AnalysisFileSyncStatus.Unavailable,
            "future_status" to AnalysisFileSyncStatus.Unknown)
    statuses.forEach { (status, expected) ->
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
                                  AnalysisStageProgress(
                                      "semantic", status, 1, false, reason = "Specific cause"),
                                  AnalysisStageProgress("performance", "pending", 0, false)))))
      val row = analysisFileStatuses(selection, run).single()
      assertEquals(expected, row.status)
      assertEquals(AnalysisFileSyncStatus.Stale, row.savedStatus)
      assertTrue(row.explanation.contains("Specific cause"))
      assertTrue(row.needsAttention)
    }
  }

  @Test
  fun mixedSavedStagesAndAdmittedProgressNeverCertifyMissingEvidence() {
    val savedStates =
        listOf(
            "fresh" to AnalysisFileSyncStatus.Updated,
            "stale" to AnalysisFileSyncStatus.Stale,
            "failed" to AnalysisFileSyncStatus.Failed,
            "partial" to AnalysisFileSyncStatus.Partial,
            "unavailable" to AnalysisFileSyncStatus.Unavailable,
            "future_status" to AnalysisFileSyncStatus.Unknown)
    val files =
        savedStates.map { (stage, _) ->
          AnalysisSelectableFile(
              "$stage.go",
              "",
              listOf(
                  AnalysisFileStageStatus("semantic", "fresh", "Current"),
                  AnalysisFileStageStatus("performance", stage, "Saved $stage"),
                  AnalysisFileStageStatus("security_rules", "skipped", "Not applicable")))
        } +
            listOf(
                AnalysisSelectableFile("policy.go", "Policy excludes this file"),
                AnalysisSelectableFile("user.go", "", selectionStageFixture("stale", "Old source")),
                AnalysisSelectableFile("absent.go", ""))
    val selection =
        selectionFixture().copy(files = files, excludedPaths = listOf("user.go", "missing.go"))
    val planFiles = files.map { AnalysisPlannedFile(it.path, "base", "Go", 20, emptyList()) }
    val base = analysisRunFixture()
    val run =
        base.copy(
            status = "running",
            plan = base.plan.copy(files = planFiles),
            files =
                files.map {
                  AnalysisRunFile(
                      it.path,
                      "base",
                      "Go",
                      listOf(AnalysisStageProgress("semantic", "pending", 0, false)))
                })
    val saved = analysisFileStatuses(selection, null)
    assertEquals(savedStates.map { it.second }, saved.take(6).map { it.status })
    assertEquals(
        listOf(AnalysisFileSyncStatus.Excluded, AnalysisFileSyncStatus.Excluded),
        saved.drop(6).take(2).map { it.status })
    assertEquals(AnalysisFileSyncStatus.Unknown, saved.last().status)
    assertTrue(saved[5].explanation.contains("Saved future_status"))
    val coverage = analysisSelectionCoverage(selection)
    assertEquals(7, coverage.total)
    assertEquals(1, coverage.fresh)
    assertEquals(3, coverage.unavailable) // unavailable, unknown stage and no stage data
    val pending = analysisFileStatuses(selection, run)
    saved.take(6).forEachIndexed { index, row ->
      assertEquals(AnalysisFileSyncStatus.Pending, pending[index].status)
      assertEquals(row.status, pending[index].savedStatus)
      assertEquals("Queued", pending[index].summary)
    }
    assertEquals(saved.drop(6).take(2), pending.drop(6).take(2))
    val finished =
        analysisFileStatuses(
            selection,
            run.copy(
                files =
                    run.files.map { file ->
                      file.copy(stages = file.stages.map { it.copy(status = "completed") })
                    }))
    assertEquals(AnalysisFileSyncStatus.Updated, finished.first().status)
    finished.drop(1).take(5).forEachIndexed { index, row ->
      assertEquals(AnalysisFileSyncStatus.Finished, row.status)
      assertEquals(saved[index + 1].status, row.savedStatus)
      assertEquals("All run stages finished.", row.summary)
    }
    assertEquals(coverage, analysisSelectionCoverage(selection))
    val reselected = selection.copy(excludedPaths = listOf("missing.go"))
    assertEquals(AnalysisFileSyncStatus.Stale, analysisFileStatuses(reselected, null)[7].status)
    assertEquals(AnalysisFileSyncStatus.Pending, analysisFileStatuses(reselected, run)[7].status)
    assertEquals(AnalysisFileSyncStatus.Stale, analysisFileStatuses(reselected, run)[7].savedStatus)
    assertEquals(coverage.total + 1, analysisSelectionCoverage(reselected).total)
  }

  @Test
  fun mismatchedOrTerminalRunsCannotOverlayMixedSavedRows() {
    val selection =
        selectionFixture()
            .copy(
                files =
                    listOf(
                        AnalysisSelectableFile(
                            "main.go", "", selectionStageFixture("stale", "Old"))))
    val base =
        analysisRunFixture()
            .copy(
                status = "running",
                files =
                    listOf(
                        AnalysisRunFile(
                            "main.go",
                            "base",
                            "Go",
                            listOf(AnalysisStageProgress("semantic", "pending", 0, false)))))
    assertEquals(
        AnalysisFileSyncStatus.Pending, analysisFileStatuses(selection, base).single().status)
    val mismatches =
        listOf(
            base.copy(identity = base.identity.copy(projectId = "other")),
            base.copy(identity = base.identity.copy(projectRevision = "other")),
            base.copy(plan = base.plan.copy(identity = base.plan.identity.copy(queueId = "other"))),
            base.copy(
                plan =
                    base.plan.copy(files = base.plan.files.map { it.copy(contentHash = "other") })),
            base.copy(files = base.files.map { it.copy(contentHash = "other") }),
            base.copy(status = "completed"),
            base.copy(status = "canceled"))
    mismatches.forEach { mismatch ->
      assertEquals(analysisFileStatuses(selection, null), analysisFileStatuses(selection, mismatch))
    }
  }

  @Test
  fun categoryColorsFollowAnalysisOutcomeRegardlessOfFindingCount() {
    val navigations = mutableListOf<Workspace>()
    val outcomes =
        listOf(
            Triple("completed", 12, Success),
            Triple("completed_empty", 0, Success),
            Triple("partial", 0, Warning),
            Triple("failed", null, Error),
            Triple("running", 12, Information),
            Triple("stale", 12, Warning),
            Triple("unavailable", null, SecondaryText))
    outcomes.forEach { (status, findings, tint) ->
      val run =
          analysisRunFixture()
              .copy(
                  status = status,
                  sections =
                      AnalysisResultType.entries.map {
                        AnalysisSectionProgress(
                            it.category, status, AnalysisRunCoverage(total = 1), findings)
                      })
      listOf(1280 to 1f, 1000 to 1.25f, 999 to 1.5f, 800 to 1.5f, 640 to 1.5f, 639 to 1.5f)
          .forEach { (width, scale) ->
            ComposeVisualFixture(width, 600, scale) {
                  AnalysisCategoryPanels(
                      AnalysisWorkspacePaneState(
                          analysisProjectFixture(), ProjectAnalysisRunState(run = run)),
                      navigations::add)
                }
                .use { fixture ->
                  fixture.render("analysis-category-$status-$width-$scale")
                  fixture.assertCategoryBoxesFit()
                  fixture.assertColorVisible(tint)
                  val count = if (status == "stale") "—" else findings?.toString() ?: "—"
                  assertEquals(3, fixture.textCount(count), "$status count at $width")
                  val label =
                      when (status) {
                        "completed" -> "Completed"
                        "completed_empty" -> "Completed · details not confirmed"
                        else -> analysisStatusLabel(status)
                      }
                  assertEquals(3, fixture.textCount(label))
                  fixture.assertTextFits(
                      label, maxLines = if (status == "completed_empty") 3 else 1)
                  if (status == "completed_empty")
                      assertEquals(3, fixture.textCount("0 reported · details not confirmed"))
                  if (status != "stale") assertEquals(3, fixture.textCount("0/1 stages covered"))
                  fixture.clickVisibleDescription("View Security results")
                  assertEquals(Workspace.Security, navigations.last())
                }
          }
    }
  }

  @Test
  fun analysisCategoryShowsSavedReadFailureWithoutReplacingRunStatus() {
    val run =
        analysisRunFixture().let { fixture ->
          fixture.copy(
              status = "interrupted",
              sections = fixture.sections.map { it.copy(status = "interrupted", findingCount = 2) })
        }
    val project = analysisProjectFixture()
    ComposeVisualFixture(800, 650, 1.5f) {
          AnalysisCategoryPanels(
              AnalysisWorkspacePaneState(
                  project,
                  ProjectAnalysisRunState(
                      run = run,
                      sections =
                          mapOf(
                              AnalysisResultKey("bugs") to
                                  AnalysisSectionState(error = "Saved read failed")))),
              {})
        }
        .use { fixture ->
          fixture.render()
          assertEquals(
              1,
              fixture.taggedTextCount(
                  "analysis-category-content-bugs", "Saved details unavailable · 2 reported"))
          assertEquals(1, fixture.taggedTextCount("analysis-category-content-bugs", "Interrupted"))
          assertEquals(0, fixture.textCount("No results"))
        }
  }

  @Test
  fun analysisCategoryRequiresMatchingSavedDetailsBeforeConfirmingEmpty() {
    val run =
        analysisRunFixture()
            .copy(
                status = "completed_empty",
                sections =
                    AnalysisResultType.entries.map {
                      AnalysisSectionProgress(
                          it.category, "completed_empty", AnalysisRunCoverage(total = 1), 0)
                    })
    val project = analysisProjectFixture()
    AnalysisResultType.entries.forEach { type ->
      val progress = run.sections.first { it.category == type.category }
      val saved = AnalysisSectionResults(run.identity, progress)
      val key = AnalysisResultKey(type.category)
      val cases =
          listOf(
              AnalysisSectionState() to "0 reported · details not confirmed",
              AnalysisSectionState(error = "Saved read failed") to
                  "Saved details unavailable · 0 reported",
              AnalysisSectionState(results = saved, error = "Saved read failed") to
                  "Saved details unavailable · 0 reported",
              AnalysisSectionState(results = saved.copy(path = "main.go")) to
                  "0 reported · details not confirmed",
              AnalysisSectionState(results = saved) to "0 reported")
      cases.forEach { (section, detail) ->
        ComposeVisualFixture(800, 650, 1.5f) {
              AnalysisCategoryPanels(
                  AnalysisWorkspacePaneState(
                      project,
                      ProjectAnalysisRunState(run = run, sections = mapOf(key to section))),
                  {})
            }
            .use { fixture ->
              fixture.render()
              val tag = "analysis-category-content-${type.category}"
              assertEquals(1, fixture.taggedTextCount(tag, detail), "${type.category}: $section")
              assertEquals(
                  1,
                  fixture.taggedTextCount(
                      tag,
                      if (section.results == saved && section.error == null) "No results"
                      else "Completed · details not confirmed"))
              assertEquals(
                  if (section.results == saved && section.error == null) 1 else 0,
                  fixture.taggedTextCount(tag, "No results"))
            }
      }
    }
  }
}
