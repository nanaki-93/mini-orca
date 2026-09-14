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
      listOf(1280 to 1f, 1000 to 1.25f, 999 to 1.5f, 800 to 1.5f).forEach { (width, scale) ->
        ComposeVisualFixture(width, 600, scale) {
              AnalysisCategoryPanels(
                  AnalysisWorkspacePaneState(
                      analysisProjectFixture(), ProjectAnalysisRunState(run = run)),
                  navigations::add)
            }
            .use { fixture ->
              fixture.render("analysis-category-$status-$width-$scale")
              listOf("Bugs", "Performance", "Security").forEach(fixture::assertTextFits)
              fixture.assertColorVisible(tint)
              assertTrue(
                  fixture.hasText(
                      if (status == "completed_empty") "Completed"
                      else analysisStatusLabel(status)))
              fixture.clickDescription("View Security results")
              assertEquals(Workspace.Security, navigations.last())
            }
      }
    }
  }
}
