package io.miniorca.desktop

import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.input.key.Key
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class SimplifiedWorkspaceTest {
  @Test
  fun summaryStartsWithActionsAndResultsAndRevealsDetailsOnlyOnRequest() {
    for ((width, height, scale) in
        listOf(
            Triple(1600, 1000, 1f),
            Triple(1440, 900, 1.25f),
            Triple(1024, 768, 1f),
            Triple(800, 650, 1.5f),
            Triple(1280, 600, 1.5f))) {
      var project by mutableStateOf(visualFixtureProject)
      var overview by mutableStateOf(visualFixtureOverview.copy(analysisRun = null))
      var requests = 0
      var diagramRenders = 0
      val destinations = mutableListOf<Workspace>()
      ComposeVisualFixture(width, height, scale) {
            ProjectSummaryPane(
                overview,
                project,
                destinations::add,
                analysisActions =
                    AnalysisWorkspaceActions(
                        { _, _ -> requests++ },
                        { requests++ },
                        { requests++ },
                        { requests++ },
                        destinations::add,
                        refreshStatus = { requests++ }),
                diagramRender = {
                  diagramRenders++
                  MermaidImage(ImageBitmap(8, 8), 8f, 8f)
                })
          }
          .use { fixture ->
            fixture.render("simplified-summary-$width-$height-$scale")
            assertTrue(fixture.hasText("Start analysis"))
            assertTrue(fixture.hasText("AI description"))
            assertEquals(0, fixture.tagCount("summary-lower-composition"))
            assertEquals(0, fixture.tagCount("summary-selected-findings"))
            assertEquals(0, fixture.tagCount("summary-change-lifecycle"))
            assertEquals(0, diagramRenders)

            for (title in listOf("File evidence", "Project details", "Editing guide")) {
              fixture.revealText(title, "summary-scroll")
              assertEquals("Collapsed", fixture.descriptionStateDescription("Expand $title"))
              assertTrue(fixture.requestDescriptionFocus("Expand $title"))
              assertTrue(fixture.pressKey(Key.Enter))
              fixture.render()
              assertEquals("Expanded", fixture.descriptionStateDescription("Collapse $title"))
              assertTrue(fixture.isFocusedControl("Collapse $title"))
              assertTrue(fixture.pressKey(Key.Spacebar))
              fixture.render()
              assertEquals("Collapsed", fixture.descriptionStateDescription("Expand $title"))
              assertTrue(fixture.isFocusedControl("Expand $title"))
            }

            fixture.expandSummarySection("Project details")
            fixture.revealText("Architecture", "summary-scroll")
            fixture.render("simplified-summary-details-$width-$height-$scale")
            assertEquals(1, fixture.tagCount("summary-architecture"))
            overview = overview.copy(findingCounts = FindingCounts(verified = 3))
            fixture.render()
            fixture.scrollBy(100_000f, "summary-scroll")
            fixture.render()
            fixture.revealText("Project details", "summary-scroll")
            assertEquals(
                "Expanded", fixture.descriptionStateDescription("Collapse Project details"))

            project = project.copy(projectRevision = "next-revision")
            overview = overview.copy(projectRevision = project.projectRevision)
            fixture.render()
            fixture.revealText("Project details", "summary-scroll")
            assertEquals("Collapsed", fixture.descriptionStateDescription("Expand Project details"))
            assertEquals(0, fixture.tagCount("summary-architecture"))
            assertEquals(0, requests)
            assertTrue(destinations.isEmpty())
          }
    }
  }

  @Test
  fun analysisCountHelpDoesNotHideResultsOrDispatchWork() {
    for ((width, height, scale) in listOf(Triple(1440, 900, 1f), Triple(800, 650, 1.5f))) {
      var requests = 0
      val help =
          "Saved findings stay visible during retries. Successful reviews replace each file’s previous findings; run coverage is shown separately."
      ComposeVisualFixture(width, height, scale) {
            AnalysisWorkspacePane(
                AnalysisWorkspacePaneState(visualFixtureProject, roundedAnalysisStateFixture()),
                AnalysisWorkspaceActions(
                    { _, _ -> requests++ },
                    { requests++ },
                    { requests++ },
                    { requests++ },
                    { requests++ },
                    refreshStatus = { requests++ }))
          }
          .use { fixture ->
            fixture.render("simplified-analysis-$width-$scale")
            fixture.revealText("Saved results", "analysis-page")
            assertFalse(fixture.hasText(help))
            assertTrue(fixture.requestDescriptionFocus("About result counts"))
            assertTrue(fixture.pressKey(Key.Enter))
            fixture.render()
            assertTrue(fixture.hasText(help))
            assertEquals("Expanded", fixture.descriptionStateDescription("About result counts"))
            assertTrue(fixture.pressKey(Key.Spacebar))
            fixture.render()
            assertFalse(fixture.hasText(help))
            for (type in AnalysisResultType.entries) {
              fixture.revealText(type.workspace.name, "analysis-page")
              assertEquals(
                  1, fixture.clickableDescriptionCount("View ${type.workspace.name} results"))
            }
            assertEquals(0, requests)
          }
    }
  }
}
