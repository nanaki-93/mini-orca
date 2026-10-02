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
  fun missingDestinationCannotCountAsAKnownLocalModel() {
    val complete = providers()
    val presentation =
        desktopStatusBarPresentation(
            DesktopState(workspace = Workspace.Editor),
            complete.copy(functionEdits = complete.functionEdits.copy(providerOrigin = "")))
    assertEquals("Models: unavailable", presentation.modelsLabel)
    assertTrue(presentation.provider!!.detail.contains("Configuration unavailable"))
    assertFalse(presentation.provider.detail.contains("local provider"))
    ComposeVisualFixture(800, 650, 1.5f) { DesktopStatusDetailsDialog(presentation) {} }
        .use { fixture ->
          fixture.render("f36-incomplete-destination")
          assertTrue(fixture.hasText("Destination: Unavailable"))
          assertTrue(fixture.hasText("Configuration unavailable"))
          assertFalse(fixture.hasText("Models: 0 local · 0 cloud"))
        }
  }

  @Test
  fun currentScopeCardsAndCapturedRunRemainDistinctAtAllAcceptanceSizes() {
    val destination = "https://provider.example/" + "long-destination-segment/".repeat(7)
    val configured =
        providers(true).let {
          it.copy(functionEdits = it.functionEdits.copy(providerOrigin = destination))
        }
    val state =
        DesktopState(
            workspace = Workspace.Analysis,
            projectState = ProjectWorkspaceState(resultProjectFixture()),
            analysisRun = ProjectAnalysisRunState(run = analysisRunFixture()))
    val presentation = desktopStatusBarPresentation(state, configured)
    assertTrue(presentation.provider!!.capturedRun)
    for ((width, height) in
        listOf(1600 to 1000, 1440 to 900, 1024 to 768, 800 to 650, 1280 to 600)) {
      for (scale in listOf(1f, 1.25f, 1.5f)) {
        ComposeVisualFixture(width, height, scale) { DesktopStatusDetailsDialog(presentation) {} }
            .use { fixture ->
              fixture.render("f36-scopes-$width-$height-$scale")
              assertTrue(fixture.hasText("Current configured scopes"))
              for (scope in ModelScope.entries) assertTrue(fixture.hasText(scope.label))
              assertTrue(fixture.hasText("Destination: $destination"))
              assertTrue(fixture.hasText("Displayed run configuration"))
              assertTrue(fixture.hasText(presentation.provider.detail))
              fixture.assertTextFits("Close")
              if (width == 800 && scale == 1.5f) {
                val label = "Destination: $destination"
                fixture.revealTextFullyWithin(label, "ide-dialog-body")
                fixture.assertTextWrapsWithoutClipping(label)
                fixture.render("f36-long-destination")
              }
            }
      }
    }
  }

  @Test
  fun configuredScopeRowSupportsCopyingItsLongDestination() {
    val model =
        providers(true)
            .functionEdits
            .copy(providerOrigin = "https://provider.example/" + "long-destination/".repeat(8))
    ComposeVisualFixture(800, 600, 1.5f) {
          ConfiguredProviderRow(DesktopStatusProvider(ModelScope.Function, model))
        }
        .use { fixture ->
          fixture.render("f36-selectable-destination")
          val label = "Destination: ${model.providerOrigin}"
          assertTrue(label.contains(fixture.copyTextByDragging(label, label)))
          fixture.render("f36-selected-destination")
        }
  }

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
  fun footerKeepsUnavailableAndLongModelLabelsReadableAndActionable() {
    val unavailable =
        desktopStatusBarPresentation(
            DesktopState(workspace = Workspace.Editor),
            providers().copy(functionEdits = ScopedModel(scope = "function")))
    val longLabel =
        unavailable.copy(
            modelsLabel = "Models: " + "123456789 local · 987654321 cloud · ".repeat(4))
    listOf(unavailable, longLabel).forEach { presentation ->
      listOf(800 to 1.5f, 320 to 1.5f).forEach { (width, scale) ->
        var opens = 0
        ComposeVisualFixture(width, 260, scale) {
              PersistentStatusBar(presentation, onOpenDetails = { opens++ })
            }
            .use { fixture ->
              fixture.render("footer-${presentation === longLabel}-$width-$scale")
              fixture.assertTextFits(presentation.modelsLabel, maxLines = 20)
              val footer = fixture.taggedBounds("model-count-footer")
              val action = fixture.taggedBounds("model-count-action")
              val label = fixture.firstVisibleTextBounds(presentation.modelsLabel)
              assertEquals(width.toFloat(), footer.right, 1f)
              assertEquals(width - 16f, action.right, 1f)
              assertTrue(action.left >= footer.left && label.left >= action.left)
              assertTrue(label.right <= action.right && action.bottom <= footer.bottom)
              fixture.clickDescription("Configured model details")
              assertEquals(1, opens)
            }
      }
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
    assertEquals(
        listOf(ModelScope.Analyze, ModelScope.Bug, ModelScope.Function),
        presentation.configuredScopes.map { it.scope })
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
    val model =
        ScopedModel(
            profile = "test",
            model = "test-model",
            providerOrigin = if (remote) "https://provider.example" else "http://localhost:11434",
            remoteProvider = remote)
    return DesktopShellStatusProviders(
        model.copy(scope = "analyze"), model.copy(scope = "bug"), model.copy(scope = "function"))
  }
}
