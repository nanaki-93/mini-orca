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
  fun fileStatesKeepTheirDistinctReasons() {
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
                  assertFalse(fixture.hasText("Completed"))
                  if (status != "completed") {
                    val label =
                        if (status == "completed_empty") "No results"
                        else analysisStatusLabel(status)
                    assertEquals(3, fixture.textCount(label))
                    fixture.assertTextFits(label)
                  }
                  fixture.clickVisibleDescription("View Security results")
                  assertEquals(Workspace.Security, navigations.last())
                }
          }
    }
  }
}
