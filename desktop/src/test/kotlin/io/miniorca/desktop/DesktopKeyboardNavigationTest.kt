package io.miniorca.desktop

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.InternalComposeUiApi
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.input.key.KeyEvent
import androidx.compose.ui.input.key.KeyEventType
import java.nio.file.Files
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicInteger
import javax.swing.SwingUtilities
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

class DesktopKeyboardNavigationTest {
  @Test
  fun canceledDirectoryChooserLeavesTheExistingFailureAndSideEffectsUntouched() {
    val failure =
        ProjectOpeningAttempt(
            8,
            "/remembered",
            ProjectOpeningKind.Restore,
            ProjectOpeningOutcome.Failed("Missing saved analysis"))
    var state = DesktopState(projectState = ProjectWorkspaceState(openingAttempt = failure))
    var selections = 0
    var saves = 0
    var terminalCloses = 0
    var draftDiscards = 0
    chooseProjectDirectory({ null }) {
      selections++
      state = DesktopState()
      saves++
      terminalCloses++
      draftDiscards++
    }
    assertEquals(0, selections + saves + terminalCloses + draftDiscards)
    assertEquals(failure, state.projectState.openingAttempt)
    chooseProjectDirectory({ java.io.File("/chosen") }) {
      assertEquals("/chosen", it)
      selections++
    }
    assertEquals(1, selections)
  }

  @OptIn(InternalComposeUiApi::class)
  @Test
  fun openShortcutUsesTheSameAttemptAvailabilityAsThePointer() {
    var opens = 0
    val actions = DesktopShellProjectActions({ opens++ }, {}, {})
    val editor =
        DesktopShellEditorState(
            EditorProgressUiState(EditorProgress.Inspect, ""),
            EditorContextualActions(false, false, false, false, false),
            false,
            false)
    fun shortcut(state: DesktopState, switchPending: Boolean = false): Boolean =
        handleDesktopShortcut(
            KeyEvent(Key.O, KeyEventType.KeyDown, isCtrlPressed = true),
            desktopShellMode(state),
            state,
            editor,
            actions,
            switchPending,
            DesktopShellEditorActions({}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}),
            DesktopShellPaletteActions({}, {}, {}, {}, {}, {}, {}),
            onDismissTransient = { false },
            onWorkspaceSelected = {},
            terminal = null,
            onTerminalSelected = {})
    val unrelatedJob = DesktopState(jobs = JobState(loading = true))
    assertTrue(shortcut(unrelatedJob))
    assertEquals(1, opens)
    val opening =
        unrelatedJob.copy(
            projectState =
                ProjectWorkspaceState(
                    openingAttempt =
                        ProjectOpeningAttempt(1, "/remembered", ProjectOpeningKind.Restore)))
    assertFalse(projectOpenAvailable(opening.projectState.openingAttempt))
    assertFalse(shortcut(opening))
    assertEquals(1, opens)
    val failed =
        opening.copy(
            projectState =
                opening.projectState.copy(
                    openingAttempt =
                        ProjectOpeningAttempt(
                            1,
                            "/remembered",
                            ProjectOpeningKind.Restore,
                            ProjectOpeningOutcome.Failed("Missing"))))
    assertTrue(shortcut(failed))
    assertEquals(2, opens)
    assertTrue(
        shortcut(
            failed.copy(projectState = failed.projectState.copy(project = resultProjectFixture()))))
    assertEquals(3, opens)
    assertFalse(shortcut(failed, switchPending = true))
    assertEquals(3, opens)
    val project = resultProjectFixture()
    val indexing =
        failed.copy(
            projectState =
                failed.projectState.copy(
                    project = project,
                    indexingAttempt =
                        ProjectIndexingAttempt(
                            1, project.projectId, project.projectRevision, project.path)))
    assertTrue(shortcut(indexing), "Indexing does not start a switch; opening may invalidate it")
    assertEquals(4, opens)
    assertFalse(
        shortcut(
            opening.copy(
                projectState =
                    opening.projectState.copy(
                        openingAttempt =
                            opening.projectState.openingAttempt!!.copy(
                                kind = ProjectOpeningKind.Import)))))
    assertEquals(4, opens)
  }

  @Test
  fun disablingFocusedOpenMovesFocusToOpeningStatusWithoutDispatchingWork() {
    var opens = 0
    var retries = 0
    var state by mutableStateOf(DesktopState())
    ComposeVisualFixture(800, 650) {
          ProjectLanding(
              state,
              DesktopShellProjectActions({ opens++ }, {}, {}, { retries++ }),
              FocusRequester())
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.requestFocus("Open project…"))
          fixture.render()
          assertTrue(fixture.isFocusedControl("Open project…"))
          state =
              state.copy(
                  projectState =
                      ProjectWorkspaceState(
                          openingAttempt =
                              ProjectOpeningAttempt(1, "/remembered", ProjectOpeningKind.Restore)))
          fixture.render()
          assertTrue(fixture.isDisabled("Open project…"))
          assertTrue(fixture.isTaggedNodeFocused("project-opening-focus"))
          assertEquals(0, opens + retries)
          state =
              state.copy(
                  projectState =
                      state.projectState.copy(
                          openingAttempt =
                              state.projectState.openingAttempt!!.copy(
                                  outcome = ProjectOpeningOutcome.Failed("Missing"))))
          fixture.render()
          assertTrue(fixture.isFocusedControl("Open project…"))
          assertEquals(0, opens + retries)
        }
  }

  @Test
  fun cancelingChooserOrSwitchReviewRestoresLandingOpenFocusWithoutOpeningProject() {
    var pending by mutableStateOf(false)
    var reviewing by mutableStateOf(false)
    var requests = 0
    var imports = 0
    val review =
        PendingProjectSwitch(
            1,
            "/next",
            ProjectSwitchContext(
                null,
                SwitchDraftIdentity(null, null, null),
                SwitchAnalyzeDestination(ScopedModel(scope = ModelScope.Analyze.wireValue)),
                false),
            SwitchReviewStage.Final)
    ComposeVisualFixture(800, 650) {
          Box {
            ProjectLanding(
                DesktopState(),
                DesktopShellProjectActions(
                    importProject = {
                      requests++
                      pending = true
                    },
                    reindexProject = {},
                    reconnect = {}),
                FocusRequester(),
                switchPending = pending)
            if (reviewing) {
              ProjectSwitchReviewDialog(
                  review,
                  ScopedModel(scope = ModelScope.Analyze.wireValue),
                  false,
                  TerminalWorkspaceState(),
                  SwitchCleanupFeedback(),
                  onCancel = {
                    reviewing = false
                    pending = false
                  },
                  onDraftApproved = {},
                  onProviderConfirmed = {},
                  onProviderApproved = {},
                  onReviewApproved = {},
                  onCommit = { imports++ })
            }
          }
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.requestFocus("Open project…"))
          fixture.render()
          assertTrue(fixture.pressKey(Key.Enter))
          fixture.render()
          assertTrue(fixture.isDisabled("Open project…"))
          pending = false // Native chooser canceled without creating an opening attempt.
          fixture.render()
          assertTrue(fixture.isFocusedControl("Open project…"))
          assertEquals(1, requests)
          assertEquals(0, imports)

          assertTrue(fixture.pressKey(Key.Enter))
          reviewing = true
          fixture.render()
          assertTrue(fixture.isDisabled("Open project…"))
          assertTrue(fixture.isFocusedControl("Cancel switch"))
          assertTrue(fixture.pressKey(Key.Escape))
          fixture.render()
          assertTrue(fixture.isFocusedControl("Open project…"))
          assertEquals(2, requests)
          assertEquals(0, imports)
        }
  }

  @Test
  fun disappearingRetryMovesFocusWithoutDispatchingAnotherOperation() {
    var retries = 0
    var opens = 0
    val failed =
        ProjectOpeningAttempt(
            1, "/remembered", ProjectOpeningKind.Restore, ProjectOpeningOutcome.Failed("Missing"))
    var state by
        mutableStateOf(DesktopState(projectState = ProjectWorkspaceState(openingAttempt = failed)))
    ComposeVisualFixture(800, 650) {
          ProjectLanding(
              state,
              DesktopShellProjectActions({ opens++ }, {}, {}, { retries++ }),
              FocusRequester())
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.requestFocus("Retry restore"))
          fixture.render()
          assertEquals(0, retries + opens)
          assertTrue(fixture.pressKey(Key.Enter))
          assertEquals(1, retries)
          state =
              state.copy(
                  projectState =
                      state.projectState.copy(
                          openingAttempt =
                              failed.copy(requestId = 2, outcome = ProjectOpeningOutcome.Opening)))
          fixture.render()
          assertTrue(fixture.isTaggedNodeFocused("project-opening-focus"))
          assertEquals(1, retries + opens)
          state =
              state.copy(
                  projectState =
                      state.projectState.copy(
                          openingAttempt =
                              failed.copy(
                                  requestId = 2,
                                  outcome = ProjectOpeningOutcome.Failed("Still missing"))))
          fixture.render()
          assertTrue(fixture.isFocusedControl("Open project…"))
          assertEquals(1, retries + opens)
        }
  }

  @Test
  fun discardPromptFocusesKeepDraftAndEscapeCannotDiscard() {
    var visible by mutableStateOf(true)
    var cancels = 0
    var discards = 0
    ComposeVisualFixture(800, 650) {
          if (visible)
              DraftDiscardDialog(
                  CurrentEditIdentity(ChatEditMode.ReplaceSymbol, "main.go", "Run", true),
                  "edit Run",
                  onDiscard = { discards++ },
                  onCancel = {
                    cancels++
                    visible = false
                  })
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.isFocusedControl("Keep draft"))
          assertFalse(fixture.isFocusedControl("Discard draft"))
          assertTrue(fixture.pressKey(Key.Escape))
          fixture.render()
          assertEquals(1, cancels)
          assertEquals(0, discards)
        }
  }

  @Test
  fun importConsentPromptDismissalCannotConfirmOrImport() {
    var visible by mutableStateOf(true)
    var confirmations = 0
    var imports = 0
    var cancels = 0
    ComposeVisualFixture(800, 650) {
          if (visible)
              ProjectImportConfirmationDialog(
                  model = ScopedModel(scope = ModelScope.Analyze.wireValue, remoteProvider = true),
                  confirmed = false,
                  onConfirmed = { confirmations++ },
                  onImport = { imports++ },
                  onCancel = {
                    cancels++
                    visible = false
                  })
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.isFocusedControl("Cancel"))
          assertFalse(fixture.isFocusedControl("Import project"))
          assertTrue(fixture.pressKey(Key.Escape))
          fixture.render()
          assertEquals(1, cancels)
          assertEquals(0, confirmations)
          assertEquals(0, imports)
        }
  }

  @Test
  fun wrappedToolbarKeepsProjectMenuAndSearchKeyboardReachable() {
    val name = "Long project identity for keyboard traversal at scaled widths"
    for ((width, height) in listOf(800 to 650, 1280 to 600)) {
      for (scale in listOf(1.25f, 1.5f)) {
        var imports = 0
        var reindexes = 0
        var reconnects = 0
        var searches = 0
        ComposeVisualFixture(width, height, scale) {
              MainToolbar(
                  ToolbarState(
                      resultProjectFixture().copy(name = name),
                      false,
                      "",
                      ConnectionState(label = "Disconnected"),
                      GitStatus(available = true, branch = "feature/a-long-branch-name")),
                  ToolbarActions({ imports++ }, { reindexes++ }, { reconnects++ }, { searches++ }))
            }
            .use { fixture ->
              fixture.render()
              fixture.awaitVisibleDescription(name)
              fixture.awaitVisibleDescription("Search files, symbols, commands")
              assertTrue(fixture.requestDescriptionFocus(name))
              fixture.render()
              assertTrue(fixture.isFocusedControl(name))
              assertTrue(fixture.pressKey(Key.Tab))
              fixture.render("header-search-focus-$width-$height-$scale")
              assertTrue(fixture.isFocusedControl("Search files, symbols, commands"))
              fixture.assertColorVisible(FocusAccent)
              assertTrue(fixture.pressKey(Key.Enter))
              assertEquals(1, searches)
              assertTrue(fixture.requestDescriptionFocus(name))
              fixture.render()
              assertTrue(fixture.pressKey(Key.Enter))
              fixture.render()
              assertTrue(fixture.hasText("Switch project…"))
              assertTrue(fixture.hasText("Re-index project"))
              assertTrue(fixture.hasText("Reconnect"))
              fixture.clickText("Re-index project")
              fixture.render()
              assertEquals(1, reindexes)
              assertEquals(0, imports)
              assertEquals(0, reconnects)
            }
      }
    }
  }

  @Test
  fun resizingWithPaletteOpenDoesNotStealFocusOrActivateItsOpener() {
    var operations = 0
    val initial = shellFocusState(resultProjectFixture())
    var state by mutableStateOf(initial.copy(app = initial.app.copy(workspace = Workspace.Editor)))
    ComposeVisualFixture(1600, 800) {
          FocusTestShell(state, onState = { state = it }, onOperation = { operations++ })
        }
        .use { fixture ->
          fixture.render()
          fixture.clickText("Search files, symbols, commands")
          fixture.render()
          assertTrue(fixture.isFocusedControl("Filter files"))
          fixture.resize(800, 650)
          fixture.render()
          assertTrue(fixture.isFocusedControl("Filter files"))
          assertTrue(fixture.pressKey(Key.Escape))
          fixture.render()
          assertTrue(fixture.isFocusedControl("Search files, symbols, commands"))
          assertEquals(0, operations)
        }
  }

  @Test
  fun successfulOpenMovesLandingFocusToSurvivingToolbarActionWithoutWork() {
    var operations = 0
    var state by mutableStateOf(shellFocusState(resultProjectFixture()).copy(app = DesktopState()))
    ComposeVisualFixture(1280, 800) {
          FocusTestShell(state, onState = { state = it }, onOperation = { operations++ })
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.requestFocus("Open project…"))
          fixture.render()
          assertEquals(0, operations)
          state =
              state.copy(
                  app = DesktopState(projectState = ProjectWorkspaceState(resultProjectFixture())))
          fixture.render()
          fixture.render()
          assertTrue(
              fixture.isFocusedControl("Search files, symbols, commands"),
              "Toolbar search should own focus after landing is removed")
          assertEquals(0, operations)
        }
  }

  @Test
  fun shellRestoresStatusOpenerOnCloseAndEscapeWithoutDispatchingWork() {
    var operations = 0
    val project = resultProjectFixture()
    var state by mutableStateOf(shellFocusState(project))
    ComposeVisualFixture(1_280, 800) {
          FocusTestShell(state, onState = { state = it }, onOperation = { operations++ })
        }
        .use { fixture ->
          fixture.render()
          repeat(2) { escape ->
            assertTrue(fixture.requestDescriptionFocus("Configured model details"))
            fixture.render()
            assertTrue(fixture.isFocusedControl("Configured model details"))
            assertTrue(fixture.pressKey(Key.Enter))
            fixture.render()
            assertTrue(fixture.hasText("Provider details"))
            assertTrue(fixture.isFocusedControl("Close"))
            if (escape == 0) {
              fixture.clickText("Close")
            } else {
              assertTrue(fixture.pressKey(Key.Escape))
            }
            fixture.render()
            assertTrue(fixture.isFocusedControl("Configured model details"))
          }
          assertEquals(0, operations)
        }
  }

  @Test
  fun railAndFooterModelsShareUnavailableDetailsAndRestoreTheirOwnFocus() {
    var operations = 0
    var state by
        mutableStateOf(
            shellFocusState(resultProjectFixture()).let {
              it.copy(editor = it.editor.copy(analysisInProgress = true, generating = true))
            })
    ComposeVisualFixture(1_280, 800) {
          FocusTestShell(state, onState = { state = it }, onOperation = { operations++ })
        }
        .use { fixture ->
          fixture.render()
          listOf("Models · Configured model details", "Configured model details").forEach { opener
            ->
            repeat(2) { attempt ->
              assertTrue(fixture.requestDescriptionFocus(opener))
              fixture.render()
              assertTrue(fixture.pressKey(if (attempt == 0) Key.Enter else Key.Spacebar))
              fixture.render()
              assertTrue(fixture.hasText("Provider details"))
              assertTrue(fixture.hasText("Models: unavailable"))
              assertTrue(
                  fixture.hasText(
                      "Model counts are unavailable until all configured model scopes have been loaded."))
              assertEquals(Workspace.Summary, state.app.workspace)
              assertTrue(fixture.isFocusedControl("Close"))
              if (attempt == 0) fixture.clickText("Close")
              else assertTrue(fixture.pressKey(Key.Escape))
              fixture.render()
              assertFalse(fixture.hasText("Provider details"))
              assertTrue(fixture.isFocusedControl(opener))
            }
          }
          assertEquals(0, operations)
        }
  }

  @Test
  fun railAndFooterModelsShowTheSameConfiguredDetailsWithoutChangingWorkspace() {
    var operations = 0
    val local = ScopedModel(profile = "local", model = "shared", providerOrigin = "loopback")
    val cloud =
        ScopedModel(
            profile = "cloud",
            model = "cloud-model",
            providerOrigin = "remote",
            remoteProvider = true)
    var state by
        mutableStateOf(
            shellFocusState(resultProjectFixture())
                .copy(statusProviders = DesktopShellStatusProviders(local, cloud, local)))
    ComposeVisualFixture(1_280, 800) {
          FocusTestShell(state, onState = { state = it }, onOperation = { operations++ })
        }
        .use { fixture ->
          fixture.render()
          listOf("Models · Configured model details", "Configured model details").forEach { opener
            ->
            fixture.clickDescription(opener)
            fixture.render()
            assertTrue(fixture.hasText("Provider details"))
            assertTrue(fixture.hasText("Models: 1 local · 1 cloud"))
            assertTrue(
                fixture.hasText(
                    "Distinct configured models by destination; shared models are counted once." +
                        "\nAnalyze: local · shared · local provider · project context stays on this machine" +
                        "\nBugs: cloud · cloud-model · remote provider · confirmation required before sending project context" +
                        "\nFunction edits: local · shared · local provider · project context stays on this machine"))
            assertEquals(Workspace.Summary, state.app.workspace)
            fixture.clickDescription(opener)
            fixture.render()
            assertTrue(fixture.hasText("Provider details"))
            fixture.clickText("Close")
            fixture.render()
            assertFalse(fixture.hasText("Provider details"))
            assertTrue(fixture.isFocusedControl(opener))
          }
          assertEquals(0, operations)
        }
  }

  @Test
  fun railModelsProjectReplacementFallsBackToTheNewProjectOrLanding() {
    var operations = 0
    var state by mutableStateOf(shellFocusState(resultProjectFixture()))
    ComposeVisualFixture(1_280, 800) {
          FocusTestShell(state, onState = { state = it }, onOperation = { operations++ })
        }
        .use { fixture ->
          fixture.render()
          fixture.clickDescription("Models · Configured model details")
          fixture.render()
          assertTrue(fixture.hasText("Provider details"))
          state =
              state.copy(
                  app =
                      state.app.copy(
                          projectState =
                              ProjectWorkspaceState(
                                  resultProjectFixture().copy(projectId = "replacement"))))
          fixture.render()
          assertFalse(fixture.hasText("Provider details"))
          assertTrue(fixture.isFocusedControl("Search files, symbols, commands"))
          assertFalse(fixture.isFocusedControl("Models · Configured model details"))
          fixture.clickDescription("Models · Configured model details")
          fixture.render()
          state = state.copy(app = DesktopState())
          fixture.render()
          assertFalse(fixture.hasText("Provider details"))
          assertTrue(fixture.isFocusedControl("Open project…"))
          assertEquals(0, operations)
        }
  }

  @Test
  fun shellUsesLiveOwnerAfterProjectChangesAndDoesNotRefocusOldOpener() {
    var operations = 0
    var state by mutableStateOf(shellFocusState(resultProjectFixture()))
    ComposeVisualFixture(1_280, 800) {
          FocusTestShell(state, onState = { state = it }, onOperation = { operations++ })
        }
        .use { fixture ->
          fixture.render()
          fixture.clickText("Models: unavailable")
          fixture.render()
          assertTrue(fixture.hasText("Provider details"))
          state =
              state.copy(
                  app =
                      state.app.copy(
                          projectState =
                              ProjectWorkspaceState(
                                  resultProjectFixture().copy(projectId = "new"))))
          fixture.render()
          assertFalse(fixture.hasText("Provider details"))
          assertTrue(fixture.isFocusedControl("Search files, symbols, commands"))
          fixture.clickText("Models: unavailable")
          fixture.render()
          state = state.copy(app = DesktopState())
          fixture.render()
          assertFalse(fixture.hasText("Provider details"))
          assertTrue(fixture.isFocusedControl("Open project…"))
          assertEquals(0, operations)
        }
  }

  @Test
  fun paletteClosesOnProjectSwitchAndRestoresTheNewOwnerRegion() {
    var operations = 0
    var state by mutableStateOf(shellFocusState(resultProjectFixture()))
    ComposeVisualFixture(1_280, 800) {
          FocusTestShell(state, onState = { state = it }, onOperation = { operations++ })
        }
        .use { fixture ->
          fixture.render()
          fixture.clickText("Search files, symbols, commands")
          fixture.render()
          assertTrue(fixture.isFocusedControl("Filter files"))
          state =
              state.copy(
                  app =
                      state.app.copy(
                          projectState =
                              ProjectWorkspaceState(
                                  resultProjectFixture().copy(projectId = "other"))))
          fixture.render()
          assertFalse(fixture.hasText("Filter files"))
          assertTrue(fixture.isFocusedControl("Search files, symbols, commands"))
          assertEquals(0, operations)
        }
  }

  @Test
  fun contextDismissalFallsBackToEditorWhenWorkspaceHidesItsOwner() {
    var operations = 0
    var state by
        mutableStateOf(
            shellFocusState(resultProjectFixture()).let {
              it.copy(
                  app = it.app.copy(workspace = Workspace.Editor),
                  layout = it.layout.copy(lastFocusedRegion = DesktopFocusRegion.RightToolWindow),
                  context = it.context.copy(manifest = ContextManifest()))
            })
    ComposeVisualFixture(1_280, 800) {
          FocusTestShell(state, onState = { state = it }, onOperation = { operations++ })
        }
        .use { fixture ->
          fixture.render()
          repeat(2) { escape ->
            // Focus a live non-canvas control so fallback restoration cannot pass vacuously.
            assertTrue(fixture.requestDescriptionFocus("Search files, symbols, commands"))
            fixture.render()
            assertFalse(fixture.isTaggedNodeFocused("desktop-canvas-focus"))
            state = state.copy(context = state.context.copy(visible = true))
            fixture.render()
            assertTrue(fixture.hasText("Context inspector · read-only"))
            state = state.copy(app = state.app.copy(workspace = Workspace.Summary))
            fixture.render()
            assertFalse(fixture.isTaggedNodeFocused("desktop-canvas-focus"))
            if (escape == 0) fixture.clickText("Close")
            else assertTrue(fixture.pressKey(Key.Escape))
            fixture.render()
            assertFalse(fixture.hasText("Context inspector · read-only"))
            assertTrue(
                fixture.isTaggedNodeFocused("desktop-canvas-focus"),
                "Summary canvas must own focus after dismissal")
            assertEquals(0, operations)
            if (escape == 0) {
              state = state.copy(app = state.app.copy(workspace = Workspace.Editor))
              fixture.render()
            }
          }
        }
  }

  @Test
  fun shellPaletteCloseAndEscapeReturnToToolbarTriggerWithoutActivatingIt() {
    var operations = 0
    var state by mutableStateOf(shellFocusState(resultProjectFixture()))
    ComposeVisualFixture(1_280, 800) {
          FocusTestShell(state, onState = { state = it }, onOperation = { operations++ })
        }
        .use { fixture ->
          fixture.render()
          repeat(2) { escape ->
            fixture.clickText("Search files, symbols, commands")
            fixture.render()
            assertTrue(fixture.isFocusedControl("Filter files"))
            if (escape == 0) fixture.clickText("Close")
            else assertTrue(fixture.pressKey(Key.Escape))
            fixture.render()
            assertTrue(fixture.isFocusedControl("Search files, symbols, commands"))
          }
          assertEquals(0, operations)
        }
  }

  @Test
  fun commandsRailOpensActionsAndRestoresItsOwnFocusOnCloseAndEscape() {
    var operations = 0
    var opens = 0
    var state by
        mutableStateOf(
            shellFocusState(resultProjectFixture()).let {
              it.copy(editor = it.editor.copy(analysisInProgress = true, generating = true))
            })
    ComposeVisualFixture(1_280, 800) {
          FocusTestShell(
              state,
              onState = { state = it },
              onOperation = { operations++ },
              onPaletteOpen = { opens++ })
        }
        .use { fixture ->
          fixture.render()
          repeat(3) { gesture ->
            assertTrue(fixture.requestDescriptionFocus("Commands · Open actions"))
            fixture.render()
            when (gesture) {
              0 -> fixture.clickDescription("Commands · Open actions")
              1 -> assertTrue(fixture.pressKey(Key.Enter))
              else -> assertTrue(fixture.pressKey(Key.Spacebar))
            }
            fixture.render()
            assertEquals(gesture + 1, opens)
            assertEquals(PaletteMode.Actions, state.palette.mode)
            assertTrue(fixture.isFocusedControl("Filter commands"))
            assertEquals(Workspace.Summary, state.app.workspace)
            if (gesture == 1) assertTrue(fixture.pressKey(Key.Escape))
            else fixture.clickText("Close")
            fixture.render()
            assertTrue(fixture.isFocusedControl("Commands · Open actions"))
          }
          assertEquals(0, operations)
        }
  }

  @Test
  fun commandsPaletteProjectReplacementFallsBackWithoutFocusingDetachedOpener() {
    var operations = 0
    var state by mutableStateOf(shellFocusState(resultProjectFixture()))
    ComposeVisualFixture(1_280, 800) {
          FocusTestShell(state, onState = { state = it }, onOperation = { operations++ })
        }
        .use { fixture ->
          fixture.render()
          fixture.clickDescription("Commands · Open actions")
          fixture.render()
          assertTrue(fixture.isFocusedControl("Filter commands"))
          state =
              state.copy(
                  app =
                      state.app.copy(
                          projectState =
                              ProjectWorkspaceState(
                                  resultProjectFixture().copy(projectId = "other"))))
          fixture.render()
          assertFalse(fixture.hasText("Filter commands"))
          assertTrue(fixture.isFocusedControl("Search files, symbols, commands"))
          assertFalse(fixture.isFocusedControl("Commands · Open actions"))
          fixture.clickDescription("Commands · Open actions")
          fixture.render()
          state = state.copy(app = DesktopState())
          fixture.render()
          assertFalse(fixture.hasText("Filter commands"))
          assertTrue(fixture.isFocusedControl("Open project…"))
          assertEquals(0, operations)
        }
  }

  @Test
  fun shellNavigationAndReadOnlyUtilitiesDoNotInvokeWorkflowCallbacks() {
    var state by mutableStateOf(shellFocusState(resultProjectFixture()))
    var operations = 0
    var paletteOpens = 0
    ComposeVisualFixture(1_280, 800) {
          FocusTestShell(
              state,
              onState = { state = it },
              onOperation = { operations++ },
              onPaletteOpen = { paletteOpens++ })
        }
        .use { fixture ->
          fixture.render()
          listOf(
                  Workspace.Summary,
                  Workspace.Analysis,
                  Workspace.Bugs,
                  Workspace.Performance,
                  Workspace.Security,
                  Workspace.Editor)
              .forEach { workspace ->
                val label = leftToolWindowLabel(leftToolWindowForWorkspace(workspace))
                fixture.clickDescription(
                    "$label tool window, ${if (state.app.workspace == workspace) "selected" else "not selected"}")
                fixture.render()
                assertEquals(workspace, state.app.workspace)
                assertEquals(0, operations)
              }
          fixture.clickDescription("Models · Configured model details")
          fixture.render()
          assertTrue(fixture.hasText("Provider details"))
          fixture.clickText("Close")
          fixture.render()
          fixture.clickDescription("Commands · Open actions")
          fixture.render()
          assertEquals(PaletteMode.Actions, state.palette.mode)
          assertEquals(1, paletteOpens)
          fixture.clickText("Close")
          fixture.render()
          assertEquals(Workspace.Editor, state.app.workspace)
          assertEquals(
              0,
              operations,
              "Passive navigation/inspection cannot scan, check, Apply/Undo or write source")
        }
  }

  @Test
  fun terminalRailOpensExistingDockWithoutStartingOnRenderFocusOrWorkspaceSwitch() {
    val directory = Files.createTempDirectory("mini-orca-rail-terminal-")
    val starts = AtomicInteger()
    val terminal = DesktopTerminalWorkspace { path ->
      DesktopTerminalSession(
          path,
          shell = "/bin/sh",
          environment = emptyMap(),
          factory =
              TerminalProcessFactory {
                starts.incrementAndGet()
                throw IllegalStateException("Synthetic launch failure")
              })
    }
    var state by
        mutableStateOf(shellFocusState(resultProjectFixture().copy(path = directory.toString())))
    var selections = 0
    try {
      ComposeVisualFixture(1_280, 800) {
            FocusTestShell(
                state,
                onState = { state = it },
                onOperation = { selections++ },
                terminal = terminal)
          }
          .use { fixture ->
            fixture.render()
            assertEquals(0, starts.get())
            assertTrue(
                fixture.requestDescriptionFocus(
                    "Terminal · Open or focus; may start a local shell"))
            fixture.render()
            assertEquals(0, starts.get())
            fixture.clickDescription("Analysis tool window, not selected")
            fixture.render()
            assertEquals(0, starts.get())
            assertEquals(Workspace.Analysis, state.app.workspace)
            assertTrue(state.layout.bottomCollapsed)
            SwingUtilities.invokeAndWait {
              fixture.clickDescription("Terminal · Open or focus; may start a local shell")
            }
            fixture.render()
            assertFalse(state.layout.bottomCollapsed)
            assertEquals(Workspace.Analysis, state.app.workspace)
            assertEquals(1, terminal.state.value.tabs.size)
            val deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(5)
            while (starts.get() == 0 && System.nanoTime() < deadline) Thread.sleep(5)
            assertEquals(1, starts.get())
            val tab = terminal.state.value.activeTabId
            SwingUtilities.invokeAndWait {
              fixture.clickDescription("Terminal · Open or focus; may start a local shell")
            }
            fixture.render()
            assertEquals(tab, terminal.state.value.activeTabId)
            assertEquals(1, terminal.state.value.tabs.size)
            assertEquals(1, starts.get())
            assertEquals(Workspace.Analysis, state.app.workspace)
            assertEquals(0, selections)
          }
    } finally {
      SwingUtilities.invokeAndWait { terminal.close() }
      Files.deleteIfExists(directory)
    }
  }

  private fun shellFocusState(project: ProjectAnalysis): DesktopShellState =
      DesktopShellState(
          app = DesktopState(projectState = ProjectWorkspaceState(project)),
          layout = DesktopLayoutState(),
          editor =
              DesktopShellEditorState(
                  EditorProgressUiState(EditorProgress.Inspect, ""),
                  EditorContextualActions(false, false, false, false, false),
                  analysisInProgress = false,
                  generating = false),
          context =
              DesktopShellContextState(
                  false, null, ScopedModel(), false, ScopedModel(), false, false),
          palette = DesktopShellPaletteState(PaletteMode.Files, "", false),
          statusProviders =
              DesktopShellStatusProviders(ScopedModel(), ScopedModel(), ScopedModel()))

  @Composable
  private fun FocusTestShell(
      state: DesktopShellState,
      onState: (DesktopShellState) -> Unit,
      onOperation: () -> Unit,
      terminal: DesktopTerminalWorkspace? = null,
      onPaletteOpen: () -> Unit = {},
  ) {
    DesktopShell(
        state = state,
        layoutActions = DesktopShellLayoutActions({ onState(state.copy(layout = it)) }, {}),
        projectActions = DesktopShellProjectActions(onOperation, onOperation, onOperation),
        editorActions =
            DesktopShellEditorActions(
                selectWorkspace = { onState(state.copy(app = state.app.copy(workspace = it))) },
                selectEditorSurface = {},
                focusChat = {},
                focusDraft = {},
                cancelAnalysis = onOperation,
                sourceLineSelected = {},
                validateDraft = onOperation,
                runDraftChecks = onOperation,
                generate = onOperation,
                cancelGeneration = onOperation,
                dismissContext = {
                  onState(state.copy(context = state.context.copy(visible = false)))
                },
                createDeclaration = onOperation),
        analysisActions =
            DesktopShellAnalysisActions(
                refreshAnalysisSelection = onOperation,
                saveAnalysisSelection = {},
                retryResults = { _, _ -> onOperation() },
                startAnalysis = { _, _ -> onOperation() },
                pauseAnalysis = onOperation,
                resumeAnalysis = onOperation,
                cancelAnalysis = onOperation,
                startScan = onOperation,
                cancelScan = onOperation,
                preparePerformanceFinding = { _, _ -> onOperation() },
                loadGoBenchmarks = onOperation,
                selectGoBenchmark = {},
                compareSelectedGoBenchmark = onOperation,
                prepareSecurityFinding = { onOperation() }),
        findingActions = FindingActions({ onOperation() }, { _, _ -> onOperation() }),
        paletteActions =
            DesktopShellPaletteActions(
                updateQuery = { onState(state.copy(palette = state.palette.copy(query = it))) },
                dismiss = { onState(state.copy(palette = state.palette.copy(visible = false))) },
                open = {
                  onPaletteOpen()
                  onState(state.copy(palette = state.palette.copy(visible = true, mode = it)))
                },
                switchMode = { onState(state.copy(palette = state.palette.copy(mode = it))) },
                selectFile = {},
                selectSymbol = {},
                selectAction = {}),
        panes =
            DesktopShellPanes(
                explorer = { _, _ -> },
                rightToolWindows = { _, _ -> },
                rightToolWindowBadges = emptyMap(),
                terminalContent = {},
                terminalState = terminal?.state?.value ?: TerminalWorkspaceState(),
                terminalTabActions = TerminalTabActions({}, {}, {})),
        terminal = terminal)
  }

  @Test
  fun paletteShortcutsKeepFilesAndSymbolsDirectWhileCommandKFocusesChat() {
    assertEquals(DesktopShortcut.OpenFile, desktopShortcut("P", primaryModifier = true))
    assertEquals(
        DesktopShortcut.OpenSymbol, desktopShortcut("O", primaryModifier = true, shift = true))
    assertEquals(DesktopShortcut.FocusChat, desktopShortcut("K", primaryModifier = true))
  }

  @Test
  fun openingAndReconnectRecoveryRequireExplicitKeyboardActivation() {
    var opens = 0
    var reconnects = 0
    ComposeVisualFixture(800, 650, 1.5f) {
          ProjectLanding(
              DesktopState(
                  projectState = ProjectWorkspaceState(openingError = "Local read failed"),
                  connection = ConnectionState(label = "Disconnected")),
              DesktopShellProjectActions({ opens++ }, {}, { reconnects++ }),
              androidx.compose.ui.focus.FocusRequester())
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.requestFocus("Reconnect daemon"))
          fixture.render()
          assertEquals(0, opens + reconnects)
          assertTrue(fixture.pressKey(Key.Enter))
          assertEquals(1, reconnects)
          assertTrue(fixture.requestFocus("Open project…"))
          assertTrue(fixture.pressKey(Key.Spacebar))
          assertEquals(1, opens)
        }
  }

  @Test
  fun savedResultRecoveryRequiresKeyboardActivationAndDoesNotStartAnalysis() {
    AnalysisResultType.entries.forEach { type ->
      val page =
          resultPageFixture(type.category).let {
            it.copy(section = it.section.copy(error = "saved read failed"))
          }
      var retries = 0
      var navigations = 0
      val browser = newResultBrowserState(page)
      browser.query = "no matching result"
      ComposeVisualFixture(800, 650, 1.5f) {
            AnalysisResultsPane(
                page,
                listOf(
                    ResultRowPresentation(
                        "retained", "Retained result", "main.go:1", "Evidence", "high", "", "")),
                browser,
                openAnalysis = { navigations++ },
                retryResults = { retries++ }) {
                  androidx.compose.material.Text("Retained detail")
                }
          }
          .use { fixture ->
            fixture.render()
            fixture.revealText("Retry loading results", "result-read-feedback")
            assertTrue(fixture.requestFocus("Retry loading results"))
            fixture.render()
            assertEquals(0, retries + navigations)
            assertTrue(fixture.pressKey(Key.Enter))
            assertEquals(1, retries)
            fixture.revealText("Clear filters", "result-overview")
            assertTrue(fixture.requestDescriptionFocus("Clear filters"))
            assertTrue(fixture.pressKey(Key.Spacebar))
            fixture.render()
            assertEquals("", browser.query)
            assertEquals(1, retries)
            assertEquals(0, navigations)
          }
    }
  }

  @Test
  fun resultNavigationAndFixPreparationStaySeparateFromKeyboardInspection() {
    var destination: Workspace? = null
    var preparations = 0
    val page = performancePageFixture()
    ComposeVisualFixture(1_000, 760, 1.25f) {
          PerformanceWorkspacePane(
              PerformanceWorkspacePaneState(page, resultIndexFixture()),
              PerformanceWorkspaceActions(
                  prepareOptimization = { _, _ -> preparations++ },
                  openAnalysis = { destination = Workspace.Analysis },
                  semanticActions = FindingActions({}, { _, _ -> })))
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.requestFocus("View analysis"))
          assertTrue(fixture.pressKey(Key.Enter))
          assertEquals(Workspace.Analysis, destination)
          assertEquals(0, preparations)

          assertTrue(fixture.requestDescriptionFocus("Inspect Avoid repeated allocation"))
          assertTrue(fixture.pressKey(Key.Enter))
          fixture.render()
          assertTrue(fixture.hasText("Prepare fix"))
          assertEquals(0, preparations)

          assertTrue(fixture.requestFocus("Prepare fix"))
          assertTrue(fixture.pressKey(Key.Enter))
          assertEquals(1, preparations)
        }
  }

  @Test
  fun analysisKeyboardControlsNavigateOrUpdateOnlyLocalPresentationState() {
    val navigations = mutableListOf<Workspace>()
    var starts = 0
    var pauses = 0
    var resumes = 0
    var cancels = 0
    var refreshes = 0
    var saves = 0
    val run =
        analysisRunFixture()
            .copy(
                status = "running",
                files =
                    listOf(
                        AnalysisRunFile(
                            "helper.go",
                            "helper",
                            "Go",
                            listOf(AnalysisStageProgress("semantic", "running", 1, false))),
                        AnalysisRunFile(
                            "main.go",
                            "main",
                            "Go",
                            listOf(AnalysisStageProgress("semantic", "running", 1, false))),
                    ),
                sections = analysisRunFixture().sections.map { it.copy(status = "running") },
            )
    val actions =
        AnalysisWorkspaceActions(
            start = { _, _ -> starts++ },
            pause = { pauses++ },
            resume = { resumes++ },
            cancel = { cancels++ },
            openResults = { navigations += it },
            refreshSelection = { refreshes++ },
            saveSelection = { saves++ },
        )
    ComposeVisualFixture(1_600, 1_000) {
          AnalysisWorkspacePane(
              AnalysisWorkspacePaneState(
                  resultProjectFixture(),
                  ProjectAnalysisRunState(
                      run = run, fileSelection = AnalysisSelectionState(selectionFixture()))),
              actions,
          )
        }
        .use { fixture ->
          fixture.render()
          AnalysisResultType.entries.forEach { type ->
            assertTrue(fixture.requestDescriptionFocus("View ${type.workspace.name} results"))
            assertTrue(fixture.pressKey(Key.Enter))
            assertTrue(fixture.pressKey(Key.Spacebar))
            fixture.render()
          }
          assertEquals(
              AnalysisResultType.entries.flatMap { listOf(it.workspace, it.workspace) },
              navigations)

          assertTrue(fixture.requestDescriptionFocus("Show active files"))
          assertTrue(fixture.pressKey(Key.Enter))
          fixture.render()
          assertTrue(fixture.hasDescription("Hide active files"))

          assertTrue(fixture.requestDescriptionFocus("Collapse Files"))
          assertTrue(fixture.pressKey(Key.Spacebar))
          fixture.render()
          assertEquals("Collapsed", fixture.stateDescription("Files"))
          assertTrue(fixture.requestDescriptionFocus("Expand Files"))
          assertTrue(fixture.pressKey(Key.Enter))
          fixture.render()
          assertEquals("Expanded", fixture.stateDescription("Files"))

          assertTrue(fixture.requestDescriptionFocus("Filter files"))
          fixture.setFocusedText("helper.go")
          fixture.render()
          assertTrue(fixture.hasText("helper.go"))
          assertTrue(fixture.requestDescriptionFocus("Needs attention"))
          assertTrue(fixture.pressKey(Key.Spacebar))
          fixture.render()
          assertTrue(fixture.isDescriptionSelected("Needs attention"))

          assertEquals(0, starts)
          assertEquals(0, pauses)
          assertEquals(0, resumes)
          assertEquals(0, cancels)
          assertEquals(0, refreshes)
          assertEquals(0, saves)
        }
  }

  @Test
  fun analysisKeyboardRunSelectionAndDetailsControlsRemainIndependent() {
    var pauses = 0
    var resumes = 0
    var cancels = 0
    val runActions =
        AnalysisWorkspaceActions(
            start = { _, _ -> },
            pause = { pauses++ },
            resume = { resumes++ },
            cancel = { cancels++ },
            openResults = {},
        )
    ComposeVisualFixture(1_600, 1_000) {
          AnalysisWorkspacePane(
              AnalysisWorkspacePaneState(
                  resultProjectFixture(),
                  ProjectAnalysisRunState(run = analysisRunFixture().copy(status = "running"))),
              runActions,
          )
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.requestFocus("Pause"))
          assertTrue(fixture.pressKey(Key.Enter))
          assertTrue(fixture.requestFocus("Cancel"))
          assertTrue(fixture.pressKey(Key.Spacebar))
          assertEquals(1, pauses)
          assertEquals(1, cancels)
        }

    ComposeVisualFixture(1_600, 1_000) {
          AnalysisWorkspacePane(
              AnalysisWorkspacePaneState(
                  resultProjectFixture(),
                  ProjectAnalysisRunState(run = analysisRunFixture().copy(status = "paused"))),
              runActions,
          )
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.requestFocus("Resume"))
          assertTrue(fixture.pressKey(Key.Enter))
          assertEquals(1, resumes)
        }

    val selectionWithDetails =
        selectionFixture()
            .copy(
                files =
                    selectionFixture().files.map { file ->
                      if (file.path == "main.go")
                          file.copy(
                              stages =
                                  listOf(
                                      AnalysisFileStageStatus(
                                          "semantic", "missing", "Semantic results are missing."),
                                      AnalysisFileStageStatus(
                                          "performance",
                                          "fresh",
                                          "Performance results are current."),
                                  ))
                      else file
                    })
    var saves = 0
    ComposeVisualFixture(1_440, 900) {
          AnalysisFileSelector(
              ProjectAnalysisRunState(fileSelection = AnalysisSelectionState(selectionWithDetails)),
              AnalysisWorkspaceActions(
                  start = { _, _ -> },
                  pause = {},
                  resume = {},
                  cancel = {},
                  openResults = {},
                  saveSelection = { saves++ },
              ),
          )
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.requestDescriptionFocus("Analyze main.go"))
          assertTrue(fixture.pressKey(Key.Spacebar))
          assertEquals(1, saves)
          assertTrue(fixture.requestDescriptionFocus("Analysis details for main.go"))
          assertTrue(fixture.pressKey(Key.Enter))
          fixture.render()
          assertEquals(
              "Expanded", fixture.descriptionStateDescription("Analysis details for main.go"))
          assertEquals(1, saves, "Opening details must not update selection")
        }

    listOf(
            ProjectAnalysisRunState(
                run = analysisRunFixture().copy(status = "running"),
                fileSelection = AnalysisSelectionState(selectionWithDetails)),
            ProjectAnalysisRunState(
                run = analysisRunFixture().copy(status = "paused"),
                fileSelection = AnalysisSelectionState(selectionWithDetails)),
            ProjectAnalysisRunState(
                run = analysisRunFixture().copy(status = "interrupted"),
                fileSelection = AnalysisSelectionState(selectionWithDetails)),
            ProjectAnalysisRunState(
                fileSelection = AnalysisSelectionState(selectionWithDetails, saving = true)),
            ProjectAnalysisRunState(
                action = "pausing", fileSelection = AnalysisSelectionState(selectionWithDetails)),
        )
        .forEach { analysis ->
          ComposeVisualFixture(1_440, 900) {
                AnalysisFileSelector(
                    analysis,
                    AnalysisWorkspaceActions(
                        start = { _, _ -> },
                        pause = {},
                        resume = {},
                        cancel = {},
                        openResults = {},
                        saveSelection = { saves++ },
                    ),
                )
              }
              .use { fixture ->
                fixture.render()
                assertTrue(fixture.isDescriptionDisabled("Analyze main.go"))
              }
        }
  }

  @Test
  fun summaryKeyboardControlsKeepDisclosuresLocalAndNavigateOnlyToTheirWorkspace() {
    val destinations = mutableListOf<Workspace>()
    val overview =
        visualFixtureOverview.copy(
            analysis =
                visualFixtureOverview.analysis.copy(
                    engineeringInsight =
                        EngineeringInsight(
                            mechanism = "Validate requests before persistence.",
                            whyItMattersHere = "Invalid input stays outside the repository.",
                            tradeoffOrFailureMode = "Rules need one owner.")))
    ComposeVisualFixture(1_600, 1_000) {
          ProjectSummaryPane(overview, visualFixtureProject, destinations::add)
        }
        .use { fixture ->
          fixture.render()
          repeat(2) {
            assertTrue(fixture.pressKey(Key.Tab), "Tab must advance through the coverage status")
            fixture.render()
          }
          val traversal =
              listOf(
                  "View analysis",
                  "View Bugs results",
                  "View Performance results",
                  "View Security results")
          traversal.forEachIndexed { index, control ->
            assertTrue(fixture.pressKey(Key.Tab), "Tab must reach $control")
            fixture.render()
            assertTrue(
                fixture.isFocusedControl(control),
                "Tab position $index must focus $control in Summary visual order")
            when (control) {
              "View analysis" -> assertTrue(fixture.pressKey(Key.Enter))
              "View Bugs results",
              "View Performance results",
              "View Security results" -> assertTrue(fixture.pressKey(Key.Spacebar))
            }
            fixture.render()
          }
          assertEquals(
              listOf(Workspace.Analysis, Workspace.Bugs, Workspace.Performance, Workspace.Security),
              destinations)
          fixture.scrollBy(100_000f)
          fixture.render()
          assertTrue(fixture.tryClick("Expand More insight"))
          fixture.render()
          assertEquals("Expanded", fixture.descriptionStateDescription("Collapse More insight"))
          fixture.awaitDescription("Show Architecture diagram", "Collapsed")
          assertTrue(fixture.tryClick("Show Architecture diagram"))
          fixture.awaitDescription("Hide Architecture diagram", "Expanded")
          assertEquals(4, destinations.size, "Summary disclosures must not navigate or start work")
        }
  }

  @Test
  fun terminalInputOwnsInterruptAndOrdinaryAppChordsUntilExplicitFocusReturn() {
    assertFalse(appShortcutAllowed(terminalFocused = true))
    assertTrue(appShortcutAllowed(terminalFocused = false))
    assertFalse(terminalReturnShortcut(java.awt.event.KeyEvent.VK_C, control = true, shift = false))
    assertFalse(
        terminalReturnShortcut(java.awt.event.KeyEvent.VK_F12, control = false, shift = true))
    assertFalse(
        terminalReturnShortcut(java.awt.event.KeyEvent.VK_F12, control = true, shift = false))
    assertTrue(terminalReturnShortcut(java.awt.event.KeyEvent.VK_F12, control = true, shift = true))
  }

  @Test
  fun labeledRailKeepsVisibleNamesAndKeyboardReachabilityAtEverySupportedSize() {
    listOf(
            Triple(1440, 900, 1f),
            Triple(1000, 760, 1f),
            Triple(999, 760, 1f),
            Triple(800, 650, 1f),
            Triple(1280, 600, 1.25f),
            Triple(1280, 600, 1.5f))
        .forEach { (width, height, scale) ->
          var active by mutableStateOf(LeftToolWindow.Summary)
          var selections = 0
          val focus = FocusRequester()
          ComposeVisualFixture(width, height, scale) {
                Row(Modifier.fillMaxSize()) {
                  ToolWindowBar(
                      active,
                      {
                        active = it
                        selections++
                      },
                      Modifier.focusRequester(focus),
                      onOpenTerminal = {})
                  Box(Modifier.weight(1f))
                }
              }
              .use { fixture ->
                fixture.render("navigation-labels-$width-$scale")
                assertFalse(fixture.hasText("Perf."))
                workspaceRailOrder.forEach {
                  assertTrue(fixture.hasText(leftToolWindowLabel(it)))
                  assertTrue(fixture.hasDescription(toolWindowSemanticsLabel(it, it == active)))
                }
                workspaceRailOrder.forEach { fixture.assertRailLabelFits(leftToolWindowLabel(it)) }
                listOf(
                        "Project",
                        "Results",
                        "Editing",
                        "Run & progress",
                        "findings",
                        "Running",
                        "Completed")
                    .forEach { assertFalse(fixture.hasText(it)) }
                assertEquals(null, fixture.stateDescription("Performance"))
                focus.requestFocus()
                fixture.render()
                repeat(5) {
                  fixture.pressKey(Key.DirectionDown)
                  fixture.render()
                }
                fixture.render("navigation-editor-focused-$width-$scale")
                assertEquals(0, selections)
                assertEquals(LeftToolWindow.Summary, active)
                assertTrue(fixture.hasDescription("Editor tool window, not selected, focused"))
                assertTrue(fixture.hasText("Editor"))
                assertTrue(fixture.pressKey(Key.Enter))
                fixture.render()
                assertEquals(LeftToolWindow.Editor, active)
                assertEquals(1, selections)
                assertTrue(fixture.hasDescription("Editor tool window, selected, focused"))
                fixture.assertColorVisible(FocusAccent)
                fixture.assertColorVisible(SelectionAccent)
                assertTrue(fixture.pressKey(Key.DirectionUp))
                fixture.render()
                assertEquals(1, selections)
                assertTrue(fixture.hasDescription("Security tool window, not selected, focused"))
                assertTrue(fixture.hasDescription("Editor tool window, selected"))
                assertTrue(fixture.pressKey(Key.Spacebar))
                fixture.render()
                assertEquals(LeftToolWindow.Security, active)
                assertEquals(2, selections)
                assertTrue(fixture.hasDescription("Security tool window, selected, focused"))
              }
        }
  }

  @Test
  fun railScrollRevealsFocusedDestinationWithoutSelectingOrStartingWork() {
    var active by mutableStateOf(LeftToolWindow.Summary)
    val selected = mutableListOf<LeftToolWindow>()
    val focus = FocusRequester()
    ComposeVisualFixture(120, 220, 1.5f) {
          ToolWindowBar(
              active,
              {
                active = it
                selected += it
              },
              Modifier.focusRequester(focus),
              onOpenTerminal = {})
        }
        .use { fixture ->
          fixture.render()
          focus.requestFocus()
          fixture.render()
          repeat(5) {
            assertTrue(fixture.pressKey(Key.DirectionDown))
            fixture.render()
          }
          assertTrue(fixture.hasDescription("Editor tool window, not selected, focused"))
          val editor = fixture.firstVisibleTextBounds("Editor")
          assertTrue(editor.top >= 0 && editor.bottom <= 220, "Focus must reveal the last label")
          assertEquals(LeftToolWindow.Summary, active)
          assertTrue(selected.isEmpty())
          fixture.clickDescription("Editor tool window, not selected, focused")
          fixture.render()
          assertEquals(listOf(LeftToolWindow.Editor), selected)
        }
  }

  @Test
  fun allNineRailActionsCanBeRevealedByKeyboardWithoutDispatchingWork() {
    var selected by mutableStateOf(LeftToolWindow.Summary)
    var selections = 0
    var utilityActions = 0
    val railFocus = FocusRequester()
    ComposeVisualFixture(180, 220, 1.5f) {
          ToolWindowBar(
              selected,
              {
                selected = it
                selections++
              },
              Modifier.focusRequester(railFocus),
              onOpenTerminal = { utilityActions++ },
              onOpenCommands = { utilityActions++ },
              onOpenModels = { utilityActions++ })
        }
        .use { fixture ->
          fixture.render()
          railFocus.requestFocus()
          fixture.render()
          workspaceRailOrder.forEachIndexed { index, entry ->
            if (index > 0) {
              assertTrue(fixture.pressKey(Key.DirectionDown))
              fixture.render()
            }
            val label = leftToolWindowLabel(entry)
            fixture.awaitVisibleDescription(
                "$label tool window, ${if (index == 0) "selected" else "not selected"}, focused")
          }
          listOf(
                  "Terminal" to "Terminal · Open or focus; may start a local shell",
                  "Commands" to "Commands · Open actions",
                  "Models" to "Models · Configured model details")
              .forEach { (label, description) ->
                assertTrue(fixture.requestDescriptionFocus(description))
                fixture.awaitVisibleDescription(description)
                assertTrue(fixture.isFocusedControl(description), "$label must retain focus")
              }
          assertEquals(0, selections)
          assertEquals(0, utilityActions)
          assertEquals(LeftToolWindow.Summary, selected)
        }
  }

  @Test
  fun terminalRailActionIsSeparateFromWorkspaceSelectionAndKeyPreview() {
    var active by mutableStateOf(LeftToolWindow.Summary)
    var selections = 0
    var opens = 0
    ComposeVisualFixture(180, 340) {
          ToolWindowBar(
              active,
              {
                active = it
                selections++
              },
              onOpenTerminal = { opens++ })
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText("Terminal"))
          assertTrue(fixture.hasDescription("Terminal · Open or focus; may start a local shell"))
          assertEquals(0, opens)
          assertTrue(
              fixture.requestDescriptionFocus("Terminal · Open or focus; may start a local shell"))
          fixture.render()
          val terminalLabel = fixture.firstVisibleTextBounds("Terminal")
          assertTrue(terminalLabel.top >= 0 && terminalLabel.bottom <= 340)
          assertEquals(0, opens)
          assertTrue(fixture.pressKey(Key.Enter))
          fixture.render()
          assertEquals(1, opens)
          assertTrue(fixture.pressKey(Key.Spacebar))
          fixture.render()
          assertEquals(2, opens)
          fixture.clickDescription("Terminal · Open or focus; may start a local shell")
          fixture.render()
          assertEquals(3, opens)
          assertEquals(0, selections)
          assertEquals(LeftToolWindow.Summary, active)
        }
  }

  @Test
  fun commandsRailActionIsIndependentOfWorkspaceKeyPreview() {
    var active by mutableStateOf(LeftToolWindow.Summary)
    var selections = 0
    var opens = 0
    ComposeVisualFixture(180, 340) {
          ToolWindowBar(
              active,
              {
                active = it
                selections++
              },
              onOpenTerminal = {},
              onOpenCommands = { opens++ })
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText("Commands"))
          assertEquals(0, opens)
          assertTrue(fixture.requestDescriptionFocus("Commands · Open actions"))
          fixture.render()
          val label = fixture.firstVisibleTextBounds("Commands")
          assertTrue(label.top >= 0 && label.bottom <= 340)
          assertTrue(fixture.pressKey(Key.Enter))
          fixture.render()
          assertEquals(1, opens)
          assertTrue(fixture.pressKey(Key.Spacebar))
          fixture.render()
          assertEquals(2, opens)
          fixture.clickDescription("Commands · Open actions")
          fixture.render()
          assertEquals(3, opens)
          assertEquals(0, selections)
          assertEquals(LeftToolWindow.Summary, active)
        }
  }

  @Test
  fun modelsRailActionIsIndependentOfWorkspaceKeyPreview() {
    var active by mutableStateOf(LeftToolWindow.Summary)
    var selections = 0
    var opens = 0
    ComposeVisualFixture(180, 340) {
          ToolWindowBar(
              active,
              {
                active = it
                selections++
              },
              onOpenTerminal = {},
              onOpenModels = { opens++ })
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText("Models"))
          assertTrue(fixture.requestDescriptionFocus("Models · Configured model details"))
          fixture.render()
          val label = fixture.firstVisibleTextBounds("Models")
          assertTrue(label.top >= 0 && label.bottom <= 340)
          assertTrue(fixture.pressKey(Key.Enter))
          fixture.render()
          assertEquals(1, opens)
          assertTrue(fixture.pressKey(Key.Spacebar))
          fixture.render()
          assertEquals(2, opens)
          fixture.clickDescription("Models · Configured model details")
          fixture.render()
          assertEquals(3, opens)
          assertEquals(0, selections)
          assertEquals(LeftToolWindow.Summary, active)
        }
  }

  @Test
  fun tabGroupArrowsMoveFocusWithoutActivatingTheDestination() {
    val entries = listOf("Source", "Review", "History")

    assertEquals(
        TabGroupInteraction("History"),
        tabGroupInteraction(entries, "Source", TabGroupKey.Previous),
    )
    assertEquals(
        TabGroupInteraction("Review"),
        tabGroupInteraction(entries, "Source", TabGroupKey.Next),
    )
    assertEquals(
        TabGroupInteraction("Source", "Source"),
        tabGroupInteraction(entries, "Source", TabGroupKey.Activate),
    )
  }

  @Test
  fun activityRailCyclesTheSixRetainedWorkspaces() {
    val entries = workspaceRailOrder

    assertEquals(
        LeftToolWindow.Analysis,
        tabGroupInteraction(entries, LeftToolWindow.Summary, TabGroupKey.Next)?.focused)
    assertEquals(
        LeftToolWindow.Editor,
        tabGroupInteraction(entries, LeftToolWindow.Summary, TabGroupKey.Previous)?.focused)
    assertEquals(
        LeftToolWindow.Problems,
        tabGroupInteraction(entries, LeftToolWindow.Analysis, TabGroupKey.Next)?.focused)
    assertEquals(
        LeftToolWindow.Performance,
        tabGroupInteraction(entries, LeftToolWindow.Problems, TabGroupKey.Next)?.focused)
    assertEquals(
        TabGroupInteraction(LeftToolWindow.Editor, LeftToolWindow.Editor),
        tabGroupInteraction(entries, LeftToolWindow.Editor, TabGroupKey.Activate),
    )
  }

  @Test
  fun escapeSelectsOnlyTheTopmostTransientSurface() {
    assertEquals(
        TransientSurface.Context,
        topmostTransientSurface(
            contextVisible = true,
            paletteVisible = true,
            statusDetailsVisible = true,
        ),
    )
    assertEquals(
        TransientSurface.StatusDetails,
        topmostTransientSurface(
            contextVisible = false,
            paletteVisible = false,
            statusDetailsVisible = true,
        ),
    )
    assertNull(
        topmostTransientSurface(
            contextVisible = false,
            paletteVisible = false,
            statusDetailsVisible = false,
        ),
    )
  }

  @Test
  fun focusedTabsKeepTextualAccessibilityStateAtCompactWidths() {
    assertEquals(
        "Editor tool window, selected, focused",
        toolWindowSemanticsLabel(LeftToolWindow.Editor, selected = true, focused = true),
    )
    assertEquals(
        "Context tool window tab, not selected, focused",
        rightToolWindowTabDescription(
            RightToolWindow.Context,
            selected = false,
            focused = true,
        ),
    )
  }

  @Test
  fun editorBreadcrumbsKeepLongTextBounded() {
    assertEquals(
        listOf("very", "…", "main.go", "Run"),
        editorBreadcrumbSegments("very/long/project/path/main.go", "Run").map { it.label },
    )
  }
}
