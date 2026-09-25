package io.miniorca.desktop

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.ui.Modifier
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNotNull
import kotlin.test.assertNull
import kotlin.test.assertTrue

class DesktopStatusBarTest {
  @Test
  fun bottomBarShowsOnlyModelCountsAcrossWidthsAndTextScales() {
    val state =
        DesktopState(
            workspace = Workspace.Editor,
            projectState = ProjectWorkspaceState(resultProjectFixture(), resultIndexFixture()),
            jobs = JobState(loading = true, status = "Indexing project", error = "Request failed"),
            connection = ConnectionState(label = "Daemon unavailable"),
            analysisRun = ProjectAnalysisRunState(run = analysisRunFixture()))
    listOf(false, true).forEach { remote ->
      val presentation = desktopStatusBarPresentation(state, providers(remote))
      val provider = assertNotNull(presentation.provider)
      listOf(
              Triple(1440, 900, 1f),
              Triple(1000, 760, 1f),
              Triple(999, 760, 1f),
              Triple(800, 650, 1.5f),
              Triple(1280, 600, 1.25f),
              Triple(1280, 600, 1.5f),
              Triple(500, 600, 1.5f))
          .forEach { (width, height, scale) ->
            var detailsOpened = false
            ComposeVisualFixture(width, height, scale) {
                  PersistentStatusBar(presentation, { detailsOpened = true })
                }
                .use { fixture ->
                  fixture.render("provider-bar-$remote-$width-$scale")
                  fixture.assertTextFits(presentation.modelsLabel)
                  assertFalse(fixture.hasDescription(provider.detail))
                  listOf(
                          "Function edits: local provider",
                          "Function edits: remote provider",
                          "Working: Indexing project",
                          "Error: Request failed",
                          "Project: ${state.project!!.name}",
                          "Index: ${state.index!!.files.size} files",
                          "Project analysis: Paused",
                          "Daemon: Disconnected")
                      .forEach { assertFalse(fixture.hasText(it)) }
                  fixture.clickText(presentation.modelsLabel)
                  assertTrue(detailsOpened)
                }
          }
    }
  }

  @Test
  fun providerDetailsKeepModelInformationSelectableAndCanBeDismissed() {
    val presentation =
        desktopStatusBarPresentation(DesktopState(workspace = Workspace.Editor), providers(true))
    val provider = assertNotNull(presentation.provider)
    assertTrue(provider.detail.contains("test-model"))
    assertTrue(provider.remoteProvider)
    var dismissed = false
    ComposeVisualFixture(800, 600, 1.5f) {
          Box(Modifier.fillMaxSize()) {
            DesktopStatusDetailsDialog(presentation) { dismissed = true }
          }
        }
        .use { fixture ->
          fixture.render("provider-details-800-1.5")
          assertTrue(fixture.hasText("Provider details"))
          assertTrue(fixture.hasText(provider.detail))
          assertTrue(fixture.hasText(presentation.modelsLabel))
          assertTrue(fixture.hasText(presentation.modelsDetail))
          assertTrue(fixture.scrollableContentCount() > 0)
          fixture.clickText("Close")
          assertTrue(dismissed)
        }
  }

  @Test
  fun analysisAndResultsKeepTheDisplayedRunsCapturedProviders() {
    val run = analysisRunFixture()
    val initial =
        DesktopState(
            projectState = ProjectWorkspaceState(resultProjectFixture()),
            analysisRun = ProjectAnalysisRunState(run = run))
    listOf(Workspace.Analysis, Workspace.Bugs, Workspace.Performance, Workspace.Security).forEach {
        workspace ->
      val state = initial.copy(workspace = workspace)
      val provider = assertNotNull(desktopStatusBarPresentation(state, providers()).provider)
      assertTrue(provider.detail.contains("bug-model"))
      assertTrue(provider.detail.contains("review-model"))
      assertFalse(provider.detail.contains("test-model"))
      assertTrue(provider.remoteProvider)
      assertEquals(
          provider,
          desktopStatusBarPresentation(
                  state.copy(
                      projectState =
                          ProjectWorkspaceState(
                              resultProjectFixture().copy(projectRevision = "next"))),
                  providers())
              .provider)
      assertNull(
          desktopStatusBarPresentation(
                  state.copy(
                      analysisRun =
                          state.analysisRun.copy(
                              run = run.copy(identity = run.identity.copy(projectId = "other")))),
                  providers())
              .provider)
      assertNull(
          desktopStatusBarPresentation(
                  state.copy(analysisRun = ProjectAnalysisRunState()), providers())
              .provider)
    }
  }

  @Test
  fun missingProviderConfigurationDoesNotInventProviderInformation() {
    val presentation =
        desktopStatusBarPresentation(
            DesktopState(workspace = Workspace.Editor),
            DesktopShellStatusProviders(ScopedModel(), ScopedModel(), ScopedModel()))
    assertNull(presentation.provider)
    assertEquals("Models: unavailable", presentation.modelsLabel)
    var opens = 0
    ComposeVisualFixture(800, 650) {
          PersistentStatusBar(presentation, onOpenDetails = { opens++ })
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasDescription("Configured model details"))
          fixture.clickDescription("Configured model details")
          assertEquals(1, opens)
        }
  }

  @Test
  fun modelCountsDeduplicateSharedDestinationsAndKeepCloudAndLocalDistinct() {
    val local = ScopedModel(model = "shared", providerOrigin = "http://localhost:11434")
    val cloud = local.copy(providerOrigin = "https://provider.example", remoteProvider = true)
    val state = DesktopState(workspace = Workspace.Editor)
    val mixed = DesktopShellStatusProviders(local, cloud, local.copy(scope = "function"))
    val presentation = desktopStatusBarPresentation(state, mixed)
    assertEquals("Models: 1 local · 1 cloud", presentation.modelsLabel)
    assertTrue(presentation.modelsDetail.contains("Analyze:"))
    assertTrue(presentation.modelsDetail.contains("Bugs:"))
    assertTrue(presentation.modelsDetail.contains("Function edits:"))
    assertEquals(
        "Models: 2 local · 1 cloud",
        desktopStatusBarPresentation(
                state,
                mixed.copy(functionEdits = local.copy(providerOrigin = "http://localhost:1234")))
            .modelsLabel)
    assertEquals(
        "Models: 1 local · 0 cloud", desktopStatusBarPresentation(state, providers()).modelsLabel)
    assertEquals(
        "Models: 0 local · 1 cloud",
        desktopStatusBarPresentation(state, providers(true)).modelsLabel)
    assertEquals(
        "Models: unavailable",
        desktopStatusBarPresentation(state, mixed.copy(analyze = ScopedModel(scope = "analyze")))
            .modelsLabel)
    assertEquals(
        presentation.modelsLabel,
        desktopStatusBarPresentation(state.copy(workspace = Workspace.Analysis), mixed).modelsLabel)
  }

  @Test
  fun statusBarIsPersistentForAProjectAndAbsentFromTheProjectLanding() {
    assertTrue(desktopStatusBarVisible(resultProjectFixture()))
    assertFalse(desktopStatusBarVisible(null))
  }

  private fun providers(remote: Boolean = false): DesktopShellStatusProviders {
    val model = ScopedModel(profile = "test", model = "test-model", remoteProvider = remote)
    return DesktopShellStatusProviders(
        model.copy(scope = "analyze"), model.copy(scope = "bug"), model.copy(scope = "function"))
  }
}
