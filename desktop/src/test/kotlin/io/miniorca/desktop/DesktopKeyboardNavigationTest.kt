package io.miniorca.desktop

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.input.key.Key
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

class DesktopKeyboardNavigationTest {
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
          assertTrue(fixture.isFocusedControl("Open project"))
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
  ) {
    DesktopShell(
        state = state,
        layoutActions = DesktopShellLayoutActions({ onState(state.copy(layout = it)) }, {}),
        projectActions = DesktopShellProjectActions(onOperation, onOperation, onOperation),
        editorActions =
            DesktopShellEditorActions(
                selectWorkspace = {},
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
                terminalState = TerminalWorkspaceState(),
                terminalTabActions = TerminalTabActions({}, {}, {})))
  }

  @Test
  fun paletteShortcutsKeepFilesAndSymbolsDirectWhileCommandKFocusesChat() {
    assertEquals(DesktopShortcut.OpenFile, desktopShortcut("P", primaryModifier = true))
    assertEquals(
        DesktopShortcut.OpenSymbol, desktopShortcut("O", primaryModifier = true, shift = true))
    assertEquals(DesktopShortcut.FocusChat, desktopShortcut("K", primaryModifier = true))
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
                      Modifier.focusRequester(focus))
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
              Modifier.focusRequester(focus))
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
