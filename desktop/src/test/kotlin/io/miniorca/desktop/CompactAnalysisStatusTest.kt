package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

class CompactAnalysisStatusTest {
  @Test
  fun previousRunRemainsAccessibleWithoutACurrentRun() {
    val previous = compactRun(listOf("main.go"), "failed")
    ComposeVisualFixture(800, 700) {
          AnalysisRunStrip(
              AnalysisWorkspacePaneState(
                  resultProjectFixture(), ProjectAnalysisRunState(previousRun = previous)),
              null,
              AnalysisRunStripScope.Analysis)
        }
        .use { fixture ->
          fixture.render()
          assertFalse(fixture.hasText(savedRunIdentityLabel(previous)))
          fixture.clickDescription("Expand Run details")
          fixture.render()
          fixture.clickDescription("Expand Previous observed run · Partial")
          fixture.render()
          assertTrue(fixture.hasText(savedRunIdentityLabel(previous)))
          assertFalse(fixture.hasText("Latest saved run"))
        }
  }

  @Test
  fun completedSummaryKeepsOneOutcomeAndDisclosesDiagnosticsLocally() {
    val reason = "Provider timed out while analyzing main.go."
    val run = compactRun(listOf("main.go"), "failed").copy(reason = reason)
    for ((width, scale) in listOf(800 to 1f, 430 to 1.5f)) {
      var requests = 0
      ComposeVisualFixture(width, 400, scale) {
            AnalysisRunStrip(
                AnalysisWorkspacePaneState(
                    resultProjectFixture(), ProjectAnalysisRunState(run = run)),
                AnalysisWorkspaceActions(
                    { _, _ -> requests++ },
                    { requests++ },
                    { requests++ },
                    { requests++ },
                    { requests++ },
                    refreshStatus = { requests++ }),
                AnalysisRunStripScope.Summary)
          }
          .use { fixture ->
            fixture.render("compact-summary-partial-$width-$scale")
            fixture.assertTextFits("Partial · 1 failure")
            fixture.assertColorVisible(Warning)
            assertEquals(0, fixture.tagCount("analysis-run-progress-track"))
            assertFalse(fixture.hasText(reason))
            assertFalse(
                fixture.hasText(
                    "Finished includes partial and failed outcomes; it does not mean successful."))
            assertTrue(fixture.requestDescriptionFocus("Expand Run diagnostic"))
            assertTrue(fixture.pressKey(androidx.compose.ui.input.key.Key.Spacebar))
            fixture.render("compact-summary-diagnostic-$width-$scale")
            fixture.assertTextFits(reason, maxLines = 4)
            assertEquals(0, requests)
          }
    }
  }

  @Test
  fun missingFileCountDeduplicatesStagesAndUnknownStatusKeepsSavedOutcomeQualified() {
    val missing = compactRun(listOf("a.go", "b.go", "c.go"), null)
    val presentation =
        projectRunPresentation(resultProjectFixture(), ProjectAnalysisRunState(run = missing))
    assertEquals("3 missing files", analysisRunOutcomeLabel(presentation))
    val unavailable = ProjectAnalysisRunState(run = missing, statusUnavailable = true)
    ComposeVisualFixture(430, 400, 1.5f) {
          AnalysisRunStrip(
              AnalysisWorkspacePaneState(resultProjectFixture(), unavailable),
              null,
              AnalysisRunStripScope.Summary)
        }
        .use { fixture ->
          fixture.render("compact-summary-status-unavailable")
          fixture.assertTextFits("Status unavailable · 3 missing files", maxLines = 3)
          assertTrue(fixture.hasText("Saved · Partial"))
          assertFalse(fixture.hasText("0 failures"))
          fixture.assertColorVisible(Warning)
        }
    val foreign = missing.copy(identity = missing.identity.copy(projectId = "another-project"))
    assertNull(
        analysisRunOutcomeLabel(
            projectRunPresentation(resultProjectFixture(), ProjectAnalysisRunState(run = foreign))))
    val excluded =
        compactRun(listOf("a.go"), "unavailable").let { run ->
          run.copy(
              plan =
                  run.plan.copy(
                      files =
                          run.plan.files.map { file ->
                            file.copy(stages = file.stages.map { it.copy(eligible = false) })
                          }))
        }
    assertNull(
        analysisRunOutcomeLabel(
            projectRunPresentation(
                resultProjectFixture(), ProjectAnalysisRunState(run = excluded))))
  }

  private fun compactRun(paths: List<String>, status: String?): AnalysisRun {
    val base = analysisRunFixture()
    val stages =
        if (status == null) listOf("semantic", "performance", "security") else listOf("semantic")
    return base.copy(
        status = "partial",
        plan =
            base.plan.copy(
                files =
                    paths.map { path ->
                      AnalysisPlannedFile(
                          path,
                          "base",
                          "Go",
                          20,
                          stages.map { AnalysisStagePlan(it, true, false, maxModelRequests = 0) })
                    }),
        files =
            paths.map { path ->
              AnalysisRunFile(
                  path,
                  "base",
                  "Go",
                  if (status == null) emptyList()
                  else stages.map { AnalysisStageProgress(it, status, 1, false) })
            })
  }
}
