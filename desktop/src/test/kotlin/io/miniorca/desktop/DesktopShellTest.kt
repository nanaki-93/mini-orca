package io.miniorca.desktop

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.width
import androidx.compose.material.Text
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.unit.dp
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

class DesktopShellTest {
  @Test
  fun assembledEditorReflowKeepsSelectedFileAndTabWithoutInvokingOperations() {
    val selected = analysisFileFixture("another.go").copy(content = "package selectedfile")
    val index =
        resultIndexFixture()
            .copy(
                files =
                    listOf(
                        IndexedFile("main.go", "base", "Go", false),
                        IndexedFile(selected.path, selected.contentHash, "Go", false)))
    val shell =
        mutableStateOf(
            DesktopShellState(
                app =
                    DesktopState(
                        workspace = Workspace.Editor,
                        projectState = ProjectWorkspaceState(resultProjectFixture(), index)),
                layout =
                    DesktopLayoutState(
                        activeRightToolWindow = RightToolWindow.Review,
                        explorerWidth = 520f,
                        actionWidth = 560f),
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
                    DesktopShellStatusProviders(ScopedModel(), ScopedModel(), ScopedModel())))
    val operations = mutableListOf<String>()
    var layoutSaves = 0
    var disposedPanes = 0
    ComposeVisualFixture(1600, 800) {
          val state = shell.value
          DesktopShell(
              state,
              layoutActions =
                  DesktopShellLayoutActions(
                      { shell.value = shell.value.copy(layout = it) }, { layoutSaves++ }),
              projectActions =
                  DesktopShellProjectActions(
                      { operations += "import" },
                      { operations += "analyze" },
                      { operations += "reconnect" }),
              editorActions =
                  DesktopShellEditorActions(
                      selectWorkspace = {
                        shell.value = shell.value.copy(app = state.app.copy(workspace = it))
                      },
                      selectEditorSurface = {
                        shell.value = shell.value.copy(layout = state.layout.withEditorSurface(it))
                      },
                      focusChat = { operations += "provider" },
                      focusDraft = { operations += "edit" },
                      cancelAnalysis = { operations += "cancel" },
                      sourceLineSelected = { operations += "source selection" },
                      validateDraft = { operations += "validate" },
                      runDraftChecks = { operations += "checks" },
                      generate = { operations += "provider" },
                      cancelGeneration = { operations += "cancel generation" },
                      dismissContext = { operations += "dismiss context" },
                      createDeclaration = { operations += "create" }),
              analysisActions =
                  DesktopShellAnalysisActions(
                      refreshAnalysisSelection = { operations += "refresh" },
                      saveAnalysisSelection = { operations += "save analysis" },
                      retryResults = { category, path -> operations += "retry $category $path" },
                      startAnalysis = { _, _ -> operations += "provider" },
                      pauseAnalysis = { operations += "pause" },
                      resumeAnalysis = { operations += "resume" },
                      cancelAnalysis = { operations += "cancel" },
                      startScan = { operations += "checks" },
                      cancelScan = { operations += "cancel scan" },
                      preparePerformanceFinding = { _, _ -> operations += "prepare" },
                      loadGoBenchmarks = { operations += "benchmarks" },
                      selectGoBenchmark = { operations += "benchmark selection" },
                      compareSelectedGoBenchmark = { operations += "checks" },
                      prepareSecurityFinding = { operations += "prepare" }),
              findingActions =
                  FindingActions({ operations += "finding" }, { _, _ -> operations += "finding" }),
              paletteActions =
                  DesktopShellPaletteActions(
                      updateQuery = { operations += "query" },
                      dismiss = { operations += "dismiss palette" },
                      open = { operations += "open palette" },
                      switchMode = { operations += "switch palette" },
                      selectFile = { operations += "palette file" },
                      selectSymbol = { operations += "palette symbol" },
                      selectAction = { operations += "palette action" }),
              panes =
                  DesktopShellPanes(
                      explorer = { modifier, _ ->
                        DisposableEffect(Unit) { onDispose { disposedPanes++ } }
                        ExplorerPane(
                            ExplorerPaneState(
                                state.app.index,
                                state.app.selectedFile?.path,
                                "",
                                emptySet(),
                                false),
                            ExplorerPaneActions(
                                { operations += "filter" },
                                { operations += "toggle" },
                                { operations += "collapse" },
                                { operations += "reveal" },
                                { path ->
                                  shell.value =
                                      shell.value.copy(
                                          app =
                                              state.app.copy(
                                                  selection =
                                                      FileSelectionState(selectedFile = selected)))
                                  assertEquals(selected.path, path)
                                }),
                            modifier)
                      },
                      rightToolWindows = { tab, modifier ->
                        DisposableEffect(Unit) { onDispose { disposedPanes++ } }
                        if (tab == RightToolWindow.Review)
                            ReviewToolWindow(
                                reviewToolWindowState(state.app),
                                ReviewToolWindowActions(
                                    { operations += "checks" },
                                    { operations += "provider" },
                                    { operations += "edit" }),
                                DraftApplicationActions(
                                    { operations += "source write" },
                                    { operations += "source write" }),
                                modifier)
                        else Box(modifier) { Text(tab.name) }
                      },
                      rightToolWindowBadges = emptyMap(),
                      terminalContent = { Box(it) },
                      terminalState = TerminalWorkspaceState(),
                      terminalTabActions =
                          TerminalTabActions(
                              { operations += "terminal select" },
                              { operations += "terminal open" },
                              { operations += "terminal close" })))
        }
        .use { fixture ->
          fixture.render()
          shell.value =
              shell.value.copy(
                  app =
                      shell.value.app.copy(
                          projectState =
                              shell.value.app.projectState.copy(
                                  preferenceSaveWarning = "Disk denied: next launch not saved")))
          fixture.render()
          assertEquals(Workspace.Editor, shell.value.app.workspace)
          assertTrue(fixture.hasText("Could not remember project"))
          assertTrue(
              fixture.hasText("The opened project could not be remembered for the next launch."))
          assertTrue(fixture.hasText("Disk denied: next launch not saved"))
          assertEquals(emptyList(), operations)
          shell.value =
              shell.value.copy(
                  app =
                      shell.value.app.copy(
                          projectState =
                              shell.value.app.projectState.copy(preferenceSaveWarning = null)))
          fixture.render()
          assertTrue(!fixture.hasText("Could not remember project"))
          assertTrue(fixture.hasText("another.go"), "Explorer should render the indexed file")
          fixture.clickText("another.go")
          fixture.render()
          assertEquals(selected, shell.value.app.selectedFile)
          assertTrue(fixture.hasDescription("Source file · another.go"))
          assertTrue(fixture.hasText("package selectedfile"))
          fixture.clickText("Assistant")
          fixture.render()
          assertEquals(RightToolWindow.Assistant, shell.value.layout.activeRightToolWindow)
          fixture.clickText("Review")
          fixture.render()
          assertTrue(fixture.hasDescription("Review tool window tab, selected"))
          assertTrue(fixture.taggedBounds("desktop-canvas-focus").width >= MIN_EDITOR_CANVAS_WIDTH)
          val toolTabs = "Right tool windows. Review selected."
          assertTrue(fixture.requestDescriptionFocus(toolTabs))
          fixture.render()
          assertTrue(fixture.isDescriptionFocused(toolTabs))
          fixture.resize(800, 650)
          fixture.render()
          assertTrue(fixture.isDescriptionFocused(toolTabs))
          assertTrue(fixture.descriptionBounds(toolTabs).bottom <= 650f)
          fixture.resize(1600, 800)
          fixture.render()
          assertTrue(fixture.isDescriptionFocused(toolTabs))
          assertTrue(
              fixture.requestDescriptionFocus("Resize adjacent panes. Use Left or Right Arrow."))
          fixture.render()
          assertTrue(
              fixture.isDescriptionFocused("Resize adjacent panes. Use Left or Right Arrow."))
          fixture.resize(800, 650)
          fixture.render()
          assertEquals(selected, shell.value.app.selectedFile)
          assertTrue(fixture.hasDescription("Source file · another.go"))
          assertTrue(fixture.hasText("package selectedfile"))
          assertEquals(RightToolWindow.Review, shell.value.layout.activeRightToolWindow)
          assertTrue(fixture.hasDescription("Review tool window tab, selected"))
          assertTrue(fixture.taggedBounds("desktop-canvas-focus").width > 0)
          assertTrue(fixture.isTaggedNodeFocused("desktop-canvas-focus"))
          fixture.scrollBy(100_000f)
          fixture.render()
          fixture.resize(1600, 800)
          fixture.render()
          assertEquals(selected, shell.value.app.selectedFile)
          assertTrue(
              fixture.hasDescription("Go file another.go at another.go, Not analyzed, selected"))
          assertTrue(fixture.hasDescription("Review tool window tab, selected"))
          assertTrue(fixture.taggedBounds("desktop-canvas-focus").width >= MIN_EDITOR_CANVAS_WIDTH)
          assertEquals(0, disposedPanes)
          assertEquals(emptyList(), operations)
          assertEquals(0, layoutSaves)
          for (type in AnalysisResultType.entries) {
            val page = resultPageFixture(type.category)
            shell.value =
                shell.value.copy(
                    app =
                        shell.value.app.copy(
                            workspace = type.workspace,
                            analysisRun =
                                ProjectAnalysisRunState(
                                    run = page.run,
                                    sections =
                                        mapOf(
                                            AnalysisResultKey(type.category) to
                                                page.section.copy(error = "read failed")))))
            fixture.render()
            fixture.assertTextFits("Retry loading results")
            fixture.clickText("Retry loading results")
            assertEquals("retry ${type.category} ", operations.last())
            fixture.clickText("View analysis")
            assertEquals(Workspace.Analysis, shell.value.app.workspace)
            assertEquals("retry ${type.category} ", operations.last())
          }
          assertEquals(3, operations.size)
        }
  }

  @Test
  fun editorArrangementOmitsHiddenPanesWithoutHidingTheCanvas() {
    for ((showFiles, showTool) in listOf(false to false, true to false, false to true)) {
      val preferred =
          DesktopLayoutState(leftToolWindowVisible = showFiles, rightToolWindowVisible = showTool)
      ComposeVisualFixture(800, 650, 1.5f) {
            androidx.compose.foundation.layout.BoxWithConstraints(Modifier.fillMaxSize()) {
              EditorPaneArrangement(
                  resolveDesktopLayout(preferred, maxWidth.value + 160f, 1.5f),
                  preferred,
                  maxHeight.value,
                  left = { modifier -> Box(modifier.testTag("files")) { Text("Files pane") } },
                  canvas = { modifier -> Box(modifier.testTag("canvas")) { Text("Source pane") } },
                  right = { modifier -> Box(modifier.testTag("tool")) { Text("Tool pane") } },
                  leftDivider = { ResizableDivider({}, {}) },
                  rightDivider = { ResizableDivider({}, {}) })
            }
          }
          .use { fixture ->
            fixture.render()
            assertTrue(fixture.taggedBounds("canvas").width > 0)
            assertEquals(
                false, fixture.hasDescription("Resize adjacent panes. Use Left or Right Arrow."))
            assertEquals(showFiles, fixture.hasText("Files pane"))
            assertEquals(showTool, fixture.hasText("Tool pane"))
          }
    }
  }

  @Test
  fun toolbarAnalysisStatusFollowsTheCurrentRunAndRetainsStaleAndErrorStates() {
    val run = analysisRunFixture().copy(status = "running")
    val initial =
        DesktopState(
            projectState = ProjectWorkspaceState(resultProjectFixture(), resultIndexFixture()),
            analysisRun = ProjectAnalysisRunState(run = run))
    val running = toolbarAnalysisStatus(initial)!!
    assertEquals("Analysis · Running", running.label)
    assertTrue(running.running)
    listOf("paused", "completed", "failed", "partial", "canceled", "interrupted").forEach { status
      ->
      val result =
          toolbarAnalysisStatus(
              initial.copy(
                  analysisRun = initial.analysisRun.copy(run = run.copy(status = status))))!!
      assertEquals("Analysis · ${status.replaceFirstChar { it.uppercase() }}", result.label)
      assertEquals(false, result.running)
      assertEquals(status in setOf("failed", "partial", "interrupted"), result.attention)
    }
    val stale =
        toolbarAnalysisStatus(
            initial.copy(
                projectState =
                    initial.projectState.copy(
                        project = initial.project!!.copy(projectRevision = "next"))))!!
    assertEquals("Analysis · Stale", stale.label)
    assertEquals(false, stale.running)
    assertTrue(stale.attention)
    val failedRead =
        toolbarAnalysisStatus(
            initial.copy(analysisRun = initial.analysisRun.copy(error = "Read failed")))!!
    assertTrue(failedRead.attention)
    assertTrue(failedRead.detail.contains("Read failed"))
    val pending =
        toolbarAnalysisStatus(
            initial.copy(analysisRun = ProjectAnalysisRunState(action = "starting")))!!
    assertEquals("Analysis · Starting", pending.label)
    assertTrue(pending.running)
    val foreign =
        initial.copy(
            analysisRun =
                initial.analysisRun.copy(
                    run = run.copy(identity = run.identity.copy(projectId = "other"))))
    assertEquals(null, toolbarAnalysisStatus(foreign))
    assertEquals(null, toolbarAnalysisStatus(initial.copy(analysisRun = ProjectAnalysisRunState())))
    assertEquals(null, toolbarAnalysisStatus(DesktopState()))
  }

  @Test
  fun wideEditorExposesSplittersAndCompactEditorDoesNot() {
    val preferred = DesktopLayoutState()
    var width by mutableStateOf(1600f)
    ComposeVisualFixture(1600, 650) {
          androidx.compose.foundation.layout.Box(Modifier.width(width.dp).height(650.dp)) {
            EditorPaneArrangement(
                resolveDesktopLayout(preferred, width + 112f, 1f),
                preferred,
                650f,
                left = { Box(it) },
                canvas = { Box(it) },
                right = { Box(it) },
                leftDivider = { ResizableDivider({}, {}) },
                rightDivider = { ResizableDivider({}, {}) })
          }
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasDescription("Resize adjacent panes. Use Left or Right Arrow."))
          width = 800f
          fixture.render()
          assertTrue(!fixture.hasDescription("Resize adjacent panes. Use Left or Right Arrow."))
        }
  }

  @Test
  fun sideSplitterPointerAndKeyboardCommitsUseDisplayedSizeOncePerGesture() {
    for (leftSide in listOf(true, false)) {
      val starting =
          DesktopLayoutState(
              leftToolWindowVisible = leftSide,
              rightToolWindowVisible = !leftSide,
              explorerWidth = 520f,
              actionWidth = 560f)
      var preferred by mutableStateOf(starting)
      val saved = mutableListOf<Float>()
      ComposeVisualFixture(800, 650) {
            EditorPaneArrangement(
                resolveDesktopLayout(preferred, 912f, 1f),
                preferred,
                650f,
                left = { Box(it) },
                canvas = { Box(it) },
                right = { Box(it) },
                leftDivider = {
                  ResizableDivider(
                      {
                        preferred =
                            resizeExplorerFromDisplayed(
                                preferred,
                                resolveDesktopLayout(preferred, 912f, 1f).explorerWidth,
                                it)
                      },
                      { saved += preferred.explorerWidth })
                },
                rightDivider = {
                  ResizableDivider(
                      {
                        preferred =
                            resizeToolFromDisplayed(
                                preferred,
                                resolveDesktopLayout(preferred, 912f, 1f).actionWidth,
                                it)
                      },
                      { saved += preferred.actionWidth })
                })
          }
          .use { fixture ->
            val displayed = resolveDesktopLayout(starting, 912f, 1f)
            val original = if (leftSide) displayed.explorerWidth else displayed.actionWidth
            assertTrue(original < if (leftSide) starting.explorerWidth else starting.actionWidth)
            fixture.render()
            val label = "Resize adjacent panes. Use Left or Right Arrow."
            fixture.dragDescription(label, Offset(if (leftSide) -12f else 12f, 0f))
            fixture.awaitResizeCommit(saved, 1)
            val afterPointer = if (leftSide) preferred.explorerWidth else preferred.actionWidth
            assertEquals(original - 12f, afterPointer, 1f)
            assertEquals(afterPointer, saved.single())
            assertTrue(fixture.requestDescriptionFocus(label))
            fixture.render()
            assertTrue(fixture.pressKey(if (leftSide) Key.DirectionLeft else Key.DirectionRight))
            fixture.render()
            assertEquals(2, saved.size)
            val afterKey = if (leftSide) preferred.explorerWidth else preferred.actionWidth
            assertEquals(afterPointer - KEYBOARD_SPLITTER_STEP, afterKey, 1f)
            assertEquals(afterKey, saved.last())
          }
    }
  }

  @Test
  fun constrainedSplitterDeltasStartAtDisplayedSizeAndClampExplicitPreferences() {
    val preferred = DesktopLayoutState(explorerWidth = 520f, actionWidth = 560f)
    val effective = resolveDesktopLayout(preferred, 1100f, 1f)
    assertEquals(DesktopLayoutMode.Wide, effective.mode)
    assertTrue(effective.explorerWidth < preferred.explorerWidth)
    assertTrue(effective.actionWidth < preferred.actionWidth)
    assertEquals(
        effective.explorerWidth + 12f,
        resizeExplorerFromDisplayed(preferred, effective.explorerWidth, 12f).explorerWidth)
    assertEquals(
        effective.actionWidth - 12f,
        resizeToolFromDisplayed(preferred, effective.actionWidth, 12f).actionWidth)
    assertEquals(
        DesktopLayoutState.MIN_EXPLORER_WIDTH,
        resizeExplorerFromDisplayed(preferred, effective.explorerWidth, -1000f).explorerWidth)
    assertEquals(
        DesktopLayoutState.MIN_ACTION_WIDTH,
        resizeToolFromDisplayed(preferred, effective.actionWidth, 1000f).actionWidth)
  }

  @Test
  fun keyboardSplittersUseTheSameBoundedResizeStepAsPointerSplitters() {
    assertEquals(-KEYBOARD_SPLITTER_STEP, verticalSplitterKeyboardDelta(Key.DirectionLeft))
    assertEquals(KEYBOARD_SPLITTER_STEP, verticalSplitterKeyboardDelta(Key.DirectionRight))
    assertEquals(null, verticalSplitterKeyboardDelta(Key.DirectionUp))
    assertEquals(KEYBOARD_SPLITTER_STEP, horizontalSplitterKeyboardDelta(Key.DirectionUp))
    assertEquals(-KEYBOARD_SPLITTER_STEP, horizontalSplitterKeyboardDelta(Key.DirectionDown))
    assertEquals(null, horizontalSplitterKeyboardDelta(Key.DirectionLeft))
  }

  @Test
  fun workspaceHeaderKeepsCurrentProjectWhileAnotherOpensOrFails() {
    var opens = 0
    var reconnects = 0
    var analyses = 0
    var retries = 0
    val project = resultProjectFixture()
    val target = "/projects/other-workspace"
    var state by
        mutableStateOf<ToolbarState>(
            ToolbarState(
                project = project,
                busy = true,
                operationStatus = "Analyzing",
                connection = ConnectionState(label = "Daemon unavailable"),
                gitStatus = null,
                analysisStatus =
                    ToolbarAnalysisStatus(
                        "Analysis · Running", "Whole-project analysis · Running", true, false)))
    ComposeVisualFixture(1024, 768, 1.5f) {
          MainToolbar(
              state,
              ToolbarActions({ opens++ }, { analyses++ }, { reconnects++ }, {}, { retries++ }))
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText(project.name))
          assertTrue(fixture.hasText("Analysis · Running"))
          assertTrue(!fixture.hasText("Restoring local project…"))
          state =
              state.copy(
                  openingAttempt = ProjectOpeningAttempt(1, target, ProjectOpeningKind.Restore))
          fixture.render()
          assertTrue(fixture.hasText("Current project"))
          assertTrue(fixture.hasText(project.name))
          assertTrue(!fixture.hasText("other-workspace"))
          assertTrue(fixture.hasText("Requested path"))
          assertTrue(fixture.hasText(target))
          assertTrue(fixture.hasText("Restoring local project…"))
          assertTrue(!fixture.hasText("Retry restore"))
          state =
              state.copy(
                  busy = false,
                  openingAttempt =
                      ProjectOpeningAttempt(
                          1,
                          target,
                          ProjectOpeningKind.Restore,
                          ProjectOpeningOutcome.Failed("Read denied")))
          fixture.render()
          assertTrue(fixture.hasText(project.name))
          assertTrue(fixture.hasText("Could not restore project"))
          assertTrue(fixture.hasText("Read denied"))
          assertTrue(fixture.hasText("Requested path"))
          assertTrue(fixture.hasText(target))
          assertTrue(fixture.hasText("Daemon disconnected"))
          assertTrue(fixture.hasText("Analysis · Running"))
          assertEquals(0, opens + reconnects + analyses + retries)
          assertTrue(fixture.requestFocus("Retry restore"))
          fixture.render()
          assertEquals(0, retries)
          fixture.clickText("Retry restore")
          assertEquals(1, retries)
          fixture.clickText("Switch project…")
          assertEquals(1, opens)
          state =
              state.copy(
                  openingAttempt =
                      ProjectOpeningAttempt(
                          2,
                          target,
                          ProjectOpeningKind.Import,
                          ProjectOpeningOutcome.Failed("Import denied")))
          fixture.render()
          assertTrue(fixture.hasText("Could not import project"))
          assertTrue(fixture.hasText("Import denied"))
          assertTrue(fixture.hasText(project.name))
          assertTrue(!fixture.hasText("Retry restore"))
          state =
              state.copy(
                  openingAttempt = ProjectOpeningAttempt(3, target, ProjectOpeningKind.Import))
          fixture.render()
          assertTrue(
              fixture.hasText(
                  "Import may use the configured Analyze provider and require confirmation. The current project remains open."))
          assertTrue(!fixture.hasText("Retry restore"))
          fixture.clickText(project.name)
          fixture.render()
          assertTrue(
              fixture.hasText(
                  "Daemon disconnected. Reconnect reads daemon status and model configuration; it does not contact a provider or run project code."))
          fixture.clickText("Reconnect")
          assertEquals(1, reconnects)
          assertEquals(0, analyses)
        }
  }

  @Test
  fun summaryHeaderShowsLocalPreferenceWarningWithoutAnOpeningAttempt() {
    val project = resultProjectFixture()
    ComposeVisualFixture(800, 650, 1.5f) {
          MainToolbar(
              ToolbarState(
                  project = project,
                  busy = false,
                  operationStatus = "Daemon ready",
                  connection = ConnectionState(),
                  gitStatus = null,
                  preferenceSaveWarning = "Disk denied"),
              ToolbarActions({}, {}, {}, {}))
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText(project.name))
          fixture.assertTextFits("Could not remember project")
          fixture.revealText("Disk denied", "project-preference-scroll")
          assertTrue(
              fixture.hasText("The opened project could not be remembered for the next launch."))
          assertTrue(!fixture.hasText("Could not restore project"))
        }
  }

  @Test
  fun loadedHeaderDisablesPointerOpenOnlyDuringProjectOpening() {
    var opens = 0
    val project = resultProjectFixture()
    var state by
        mutableStateOf(ToolbarState(project, true, "Analysis running", ConnectionState(), null))
    ComposeVisualFixture(1024, 768) { MainToolbar(state, ToolbarActions({ opens++ }, {}, {}, {})) }
        .use { fixture ->
          fixture.render()
          fixture.clickText(project.name)
          fixture.render()
          assertTrue(!fixture.isDisabled("Switch project…"))
          fixture.clickText("Switch project…")
          assertEquals(1, opens)
          state =
              state.copy(
                  openingAttempt = ProjectOpeningAttempt(1, "/next", ProjectOpeningKind.Import))
          fixture.render()
          fixture.clickText(project.name)
          fixture.render()
          assertTrue(fixture.isDisabled("Switch project…"))
          assertEquals(1, opens)
          state =
              state.copy(
                  openingAttempt =
                      state.openingAttempt!!.copy(
                          outcome = ProjectOpeningOutcome.Failed("Import failed")))
          fixture.render()
          assertTrue(!fixture.isDisabled("Switch project…"))
        }
  }

  @Test
  fun projectMenuActionsTrackOpeningSwitchAndIndexingWithoutDispatchOnFocusOrDismissal() {
    val project = resultProjectFixture()
    val indexAttempt =
        ProjectIndexingAttempt(1, project.projectId, project.projectRevision, project.path)
    var opens = 0
    var reindexes = 0
    var state by mutableStateOf(ToolbarState(project, false, "", ConnectionState(), null))
    ComposeVisualFixture(1024, 768) {
          MainToolbar(state, ToolbarActions({ opens++ }, { reindexes++ }, {}, {}))
        }
        .use { fixture ->
          fun menu() {
            fixture.clickText(project.name)
            fixture.render()
          }
          fixture.render()
          assertTrue(fixture.requestFocus(project.name))
          assertTrue(fixture.pressKey(androidx.compose.ui.input.key.Key.Enter))
          fixture.render()
          assertTrue(fixture.hasText("Switch project…"))
          assertTrue(!fixture.isDisabled("Switch project…"))
          assertTrue(!fixture.isDisabled("Re-index project"))
          assertEquals(0, opens + reindexes)
          fixture.pressKey(androidx.compose.ui.input.key.Key.Escape)
          fixture.render()
          assertTrue(fixture.isFocusedControl(project.name))
          assertEquals(0, opens + reindexes)

          state = state.copy(indexingAttempt = indexAttempt)
          fixture.render()
          menu()
          assertTrue(fixture.isDisabled("Re-index project"))
          assertTrue(!fixture.isDisabled("Switch project…"))
          assertEquals(0, opens + reindexes)
          state = state.copy(indexingAttempt = null, switchPending = true)
          fixture.render()
          assertTrue(fixture.isDisabled("Re-index project"))
          assertTrue(fixture.isDisabled("Switch project…"))
          state =
              state.copy(
                  switchPending = false,
                  openingAttempt = ProjectOpeningAttempt(1, "/next", ProjectOpeningKind.Import))
          fixture.render()
          assertTrue(fixture.isDisabled("Re-index project"))
          assertTrue(fixture.isDisabled("Switch project…"))
          state = state.copy(openingAttempt = null)
          fixture.render()
          assertTrue(!fixture.isDisabled("Re-index project"))
          fixture.clickText("Re-index project")
          assertEquals(1, reindexes)
          assertEquals(0, opens)
        }
  }

  @Test
  fun landingKeepsLocalOpenFailureAndDaemonRecoverySeparate() {
    var opens = 0
    var reconnects = 0
    var retries = 0
    val actions = DesktopShellProjectActions({ opens++ }, {}, { reconnects++ }, { retries++ })
    val failed =
        DesktopState(
            projectState =
                ProjectWorkspaceState(
                    rememberedPath = "/projects/remembered",
                    openingAttempt =
                        ProjectOpeningAttempt(
                            1,
                            "/projects/remembered",
                            ProjectOpeningKind.Restore,
                            ProjectOpeningOutcome.Failed("Permission denied"))),
            connection = ConnectionState(label = "Daemon unavailable"))
    ComposeVisualFixture(800, 650, 1.5f) {
          ProjectLanding(failed, actions, androidx.compose.ui.focus.FocusRequester())
        }
        .use { fixture ->
          fixture.render()
          fixture.assertTextFits("Could not restore project")
          assertTrue(fixture.hasText("Permission denied"))
          assertTrue(fixture.hasText("Remembered path"))
          assertTrue(fixture.hasText("Requested path"))
          fixture.assertTextFits("Retry restore")
          fixture.assertTextFits("Reconnect daemon")
          assertEquals(0, opens + reconnects + retries)
          assertTrue(fixture.requestFocus("Retry restore"))
          fixture.render()
          assertEquals(0, opens + reconnects + retries)
          fixture.clickText("Retry restore")
          assertEquals(1, retries)
          fixture.clickText("Reconnect daemon")
          assertEquals(1, reconnects)
          assertEquals(0, opens)
          fixture.clickText("Open project…")
          assertEquals(1, opens)
          assertEquals(1, reconnects)
        }
    ComposeVisualFixture(320, 500, 1.5f) {
          ProjectLanding(DesktopState(), actions, androidx.compose.ui.focus.FocusRequester())
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText("No project remembered on this device."))
          fixture.assertTextFits("Open project…")
          assertTrue(!fixture.hasText("Retry restore"))
          assertTrue(!fixture.hasText("Reconnect daemon"))
        }
  }

  @Test
  fun landingTreatsUnreadableLastProjectAsUnknown() {
    var opens = 0
    val state =
        DesktopState(projectState = ProjectWorkspaceState(preferenceReadWarning = "Storage denied"))
    ComposeVisualFixture(320, 500, 1.5f) {
          ProjectLanding(
              state,
              DesktopShellProjectActions({ opens++ }, {}, {}),
              androidx.compose.ui.focus.FocusRequester())
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText("Last project unknown; local preferences could not be read."))
          assertTrue(!fixture.hasText("No project remembered on this device."))
          assertTrue(fixture.hasText("Could not read last project preference"))
          assertTrue(fixture.hasText("Storage denied"))
          assertTrue(!fixture.isDisabled("Open project…"))
          assertEquals(0, opens)
          fixture.clickText("Open project…")
          assertEquals(1, opens)
        }
  }

  @Test
  fun landingOpeningStateIgnoresUnrelatedJobsAndNeverOffersImportAsRestoreRetry() {
    var opens = 0
    var retries = 0
    val actions = DesktopShellProjectActions({ opens++ }, {}, {}, { retries++ })
    var app by
        mutableStateOf(
            DesktopState(
                jobs = JobState(loading = true, status = "Analyzing", error = "Provider failed")))
    ComposeVisualFixture(800, 650, 1.5f) {
          ProjectLanding(app, actions, androidx.compose.ui.focus.FocusRequester())
        }
        .use { fixture ->
          fixture.render()
          assertTrue(!fixture.hasText("Restoring local project…"))
          assertTrue(!fixture.hasText("Provider failed"))
          assertTrue(!fixture.isDisabled("Open project…"))
          fixture.clickText("Open project…")
          assertEquals(1, opens)
          app =
              app.copy(
                  projectState =
                      ProjectWorkspaceState(
                          openingAttempt =
                              ProjectOpeningAttempt(
                                  3,
                                  "/other/project",
                                  ProjectOpeningKind.Import,
                                  ProjectOpeningOutcome.Failed("Import denied"))))
          fixture.render()
          assertTrue(fixture.hasText("Could not import project"))
          assertTrue(fixture.hasText("/other/project"))
          assertTrue(!fixture.hasText("Retry restore"))
          app =
              app.copy(
                  projectState =
                      app.projectState.copy(
                          openingAttempt =
                              ProjectOpeningAttempt(
                                  4, "/other/project", ProjectOpeningKind.Restore)))
          fixture.render()
          assertTrue(fixture.hasText("Restoring local project…"))
          assertTrue(fixture.isDisabled("Open project…"))
          assertTrue(!fixture.hasText("Retry restore"))
          assertEquals(0, retries)
          app =
              app.copy(
                  projectState =
                      ProjectWorkspaceState(preferenceReadWarning = "Preferences unavailable"))
          fixture.render()
          assertTrue(fixture.hasText("Could not read last project preference"))
          assertTrue(fixture.hasText("Preferences unavailable"))
          assertTrue(!fixture.isDisabled("Open project…"))
          assertTrue(!fixture.hasText("Retry restore"))
          assertEquals(1, opens)
        }
  }

  @Test
  fun fileAndProjectOpeningFeedbackDoesNotFollowUnrelatedGlobalErrors() {
    val state =
        DesktopState(
                projectState = ProjectWorkspaceState(openingError = "Project denied"),
                selection = FileSelectionState(fileReadError = "File denied"),
                jobs = JobState(status = "Loading main.go…", error = "File denied"))
            .reduce(DesktopEvent.Failed("Unrelated provider failure"))
    assertEquals("Project denied", state.projectState.openingError)
    assertEquals("File denied", state.selection.fileReadError)
    assertEquals("Unrelated provider failure", state.error)
  }

  @Test
  fun shellUsesADedicatedLandingBranchUntilAProjectExists() {
    assertEquals(DesktopShellMode.ProjectLanding, desktopShellMode(DesktopState()))
    val project =
        ProjectAnalysis(
            "project",
            "revision",
            "Mini",
            "/tmp/project",
            "go",
            fileCount = 1,
            sourceFileCount = 1,
            totalLines = 1,
            summary = "",
            aiStatus = "fresh",
            analyzedAt = "")

    assertEquals(
        DesktopShellMode.ProjectWorkspace,
        desktopShellMode(DesktopState(projectState = ProjectWorkspaceState(project))))
  }

  @Test
  fun explorerGroupsRelativePathsAndKeepsOnlyMatchingBranches() {
    val files =
        listOf(
            IndexedFile("internal/project/index.go", "a", "Go", false, analysisStatus = "fresh"),
            IndexedFile("internal/project/path.go", "b", "Go", false, analysisStatus = "stale"),
            IndexedFile("README.md", "c", "Markdown", false),
        )

    val rows = explorerRows(files, "index")

    assertEquals(
        listOf("internal", "internal/project", "internal/project/index.go"), rows.map { it.path })
    assertTrue(rows.last().analysisStatus == "fresh")
  }

  @Test
  fun analysisBadgesUseTextualFreshnessLabels() {
    assertEquals("Fresh", statusBadgeStyle("fresh").label)
    assertEquals("Not analyzed", statusBadgeStyle("missing").label)
    assertEquals("Failed", statusBadgeStyle("failed").label)
  }

  @Test
  fun editorChromeIsScopedToTheEditorWorkspace() {
    assertTrue(editorChromeVisible(Workspace.Editor))
    assertTrue(!editorChromeVisible(Workspace.Summary))
    assertTrue(!editorChromeVisible(Workspace.Analysis))
    assertTrue(!editorChromeVisible(Workspace.Bugs))
  }

  @Test
  fun largeExplorerKeepsAStableFilteredSelectionPath() {
    val files =
        (1..2_000).map { number ->
          IndexedFile(
              "src/module$number/File$number.kt",
              "hash-$number",
              "Kotlin",
              false,
              analysisStatus = if (number % 2 == 0) "fresh" else "missing")
        }

    val rows = explorerRows(files, "File1999.kt")

    assertEquals(
        listOf("src", "src/module1999", "src/module1999/File1999.kt"), rows.map { it.path })
  }

  @Test
  fun explorerDescriptionsExposeRolesLanguageFreshnessAndExpansion() {
    val file = ExplorerRow("internal/main.go", "main.go", 1, false, "stale", "Go")
    val folder = ExplorerRow("internal", "internal", 0, true)

    assertEquals(
        "Go file main.go at internal/main.go, Stale, selected",
        explorerRowDescription(file, selected = true, expanded = false))
    assertEquals(
        "Folder internal, collapsed",
        explorerRowDescription(folder, selected = false, expanded = false))
    assertEquals("DIR −", explorerRoleLabel(folder, expanded = true))
  }

  @Test
  fun collapsedFolderShowsItsChildrenAfterItIsExpanded() {
    val files = listOf(IndexedFile("internal/project/index.go", "hash", "Go", false))
    val collapsed = explorerDirectories(files)

    assertEquals(listOf("internal"), visibleExplorerRows(files, "", collapsed).map { it.path })
    assertEquals(
        listOf("internal", "internal/project"),
        visibleExplorerRows(files, "", collapsed - "internal").map { it.path },
    )
    assertEquals(
        listOf("internal", "internal/project", "internal/project/index.go"),
        visibleExplorerRows(files, "", collapsed - "internal" - "internal/project").map { it.path },
    )
  }

  @Test
  fun explorerPlacesDescendantsImmediatelyAfterTheirFolder() {
    val files =
        listOf(
            IndexedFile("cmd/root.go", "a", "Go", false),
            IndexedFile("cmd/sub/child.go", "b", "Go", false),
            IndexedFile("model/item.go", "c", "Go", false),
            IndexedFile("README.md", "d", "Markdown", false),
        )

    assertEquals(
        listOf(
            "cmd",
            "cmd/sub",
            "cmd/sub/child.go",
            "cmd/root.go",
            "model",
            "model/item.go",
            "README.md"),
        explorerRows(files).map { it.path },
    )
  }

  @Test
  fun toolWindowBarMapsTheSixNavigationDestinations() {
    assertEquals(
        listOf(
            LeftToolWindow.Summary,
            LeftToolWindow.Analysis,
            LeftToolWindow.Performance,
            LeftToolWindow.Problems,
            LeftToolWindow.Security,
            LeftToolWindow.Editor),
        LeftToolWindow.entries.toList())
    assertEquals("Summary", leftToolWindowLabel(LeftToolWindow.Summary))
    assertEquals("Analysis", leftToolWindowLabel(LeftToolWindow.Analysis))
    assertEquals("Bugs", leftToolWindowLabel(LeftToolWindow.Problems))
    assertEquals("Security", leftToolWindowLabel(LeftToolWindow.Security))
    assertEquals("Editor", leftToolWindowLabel(LeftToolWindow.Editor))
    assertEquals(Workspace.Bugs, workspaceForLeftToolWindow(LeftToolWindow.Problems))
    assertEquals(LeftToolWindow.Summary, leftToolWindowForWorkspace(Workspace.Summary))
    assertEquals(LeftToolWindow.Analysis, leftToolWindowForWorkspace(Workspace.Analysis))
    assertEquals(LeftToolWindow.Performance, leftToolWindowForWorkspace(Workspace.Performance))
    assertEquals(LeftToolWindow.Problems, leftToolWindowForWorkspace(Workspace.Bugs))
    assertEquals(LeftToolWindow.Security, leftToolWindowForWorkspace(Workspace.Security))
    assertEquals(LeftToolWindow.Editor, leftToolWindowForWorkspace(Workspace.Editor))
    assertEquals(DesktopIcon.Problems, leftToolWindowIcon(LeftToolWindow.Problems))
    assertEquals(DesktopIcon.Performance, leftToolWindowIcon(LeftToolWindow.Performance))
  }

  @Test
  fun topBarTextNamesTheProjectWithoutItsRevision() {
    val project =
        ProjectAnalysis(
            "project",
            "revision-hash",
            "Long project name",
            "/tmp/project",
            "go",
            fileCount = 1,
            sourceFileCount = 1,
            totalLines = 1,
            summary = "",
            aiStatus = "fresh",
            analyzedAt = "")

    assertEquals("Long project name", projectBreadcrumbLabel(project))
    assertEquals(
        "Daemon connected",
        connectionPresentation(
                ConnectionState(
                    connected = true,
                    locality = "Local endpoint",
                    model = "local model",
                    latency = "12ms"))
            .label)
    assertEquals(
        "Daemon disconnected",
        connectionPresentation(
                ConnectionState(label = "Daemon unavailable", locality = "Remote endpoint"))
            .label)
    assertEquals(
        ConnectionPresentation("Daemon connected", Success, false),
        connectionPresentation(ConnectionState(connected = true)))
    assertEquals(
        ConnectionPresentation("Daemon connecting", Warning, false),
        connectionPresentation(ConnectionState(label = "Connecting")))
    assertEquals(
        ConnectionPresentation("Daemon disconnected", Error, true),
        connectionPresentation(ConnectionState(label = "Daemon unavailable")))
  }

  @Test
  fun statusProviderTracksTheCurrentWorkspaceScope() {
    val providers =
        DesktopShellStatusProviders(
            analyze = ScopedModel(model = "analyze"),
            bugs = ScopedModel(model = "bugs"),
            functionEdits = ScopedModel(model = "function"),
        )

    assertEquals(ModelScope.Analyze, statusProviderForWorkspace(Workspace.Summary, providers).scope)
    assertEquals(ModelScope.Bug, statusProviderForWorkspace(Workspace.Analysis, providers).scope)
    assertEquals(ModelScope.Bug, statusProviderForWorkspace(Workspace.Bugs, providers).scope)
    assertEquals(ModelScope.Function, statusProviderForWorkspace(Workspace.Editor, providers).scope)
  }

  @Test
  fun branchContextUsesOnlyActualGitEvidence() {
    assertEquals(
        BranchPresentation("main", "Current Git branch: main"),
        branchPresentation(GitStatus(available = true, branch = "main")),
    )
    assertEquals(
        BranchPresentation("Unavailable", "Git branch is unavailable for the selected file."),
        branchPresentation(GitStatus(available = false, branch = "main")),
    )
    assertEquals("Unavailable", branchPresentation(null).label)
    assertEquals(
        BranchPresentation("Unavailable", "Git branch is unavailable for the selected file."),
        branchPresentation(GitStatus(available = true, branch = " \t\n ")))
    assertEquals(
        BranchPresentation("feature/long-name", "Current Git branch: feature/long-name"),
        branchPresentation(GitStatus(available = true, branch = " feature/long-name ")))
  }

  @Test
  fun paletteDismissalRestoresThePriorMeaningfulVisibleRegion() {
    assertEquals(
        DesktopFocusRegion.RightToolWindow,
        paletteFocusRestorationRegion(
            DesktopFocusRegion.RightToolWindow,
            rightToolWindowVisible = true,
            bottomToolWindowVisible = true,
        ),
    )
    assertEquals(
        DesktopFocusRegion.BottomToolWindow,
        paletteFocusRestorationRegion(
            DesktopFocusRegion.BottomToolWindow,
            rightToolWindowVisible = true,
            bottomToolWindowVisible = true,
        ),
    )
    assertEquals(
        DesktopFocusRegion.Editor,
        paletteFocusRestorationRegion(
            DesktopFocusRegion.RightToolWindow,
            rightToolWindowVisible = false,
            bottomToolWindowVisible = true,
        ),
    )
    assertEquals(
        DesktopFocusRegion.StatusBar,
        paletteFocusRestorationRegion(
            DesktopFocusRegion.StatusBar,
            rightToolWindowVisible = true,
            bottomToolWindowVisible = true,
        ),
    )
  }

  @Test
  fun transientOpenerIdentityOnlySurvivesWithItsProjectAndRegion() {
    assertEquals(
        TransientOpener.RailCommands,
        transientFocusOpener(TransientOpener.RailCommands, true, DesktopFocusRegion.LeftToolWindow))
    assertEquals(
        TransientOpener.HeaderSearch,
        transientFocusOpener(TransientOpener.HeaderSearch, true, DesktopFocusRegion.Toolbar))
    assertEquals(
        TransientOpener.FooterModels,
        transientFocusOpener(TransientOpener.FooterModels, true, DesktopFocusRegion.StatusBar))
    assertEquals(
        TransientOpener.RailModels,
        transientFocusOpener(TransientOpener.RailModels, true, DesktopFocusRegion.LeftToolWindow))
    assertEquals(
        TransientOpener.Region,
        transientFocusOpener(TransientOpener.RailModels, false, DesktopFocusRegion.Toolbar))
    assertEquals(
        TransientOpener.Region,
        transientFocusOpener(TransientOpener.RailModels, true, DesktopFocusRegion.StatusBar))
    assertEquals(
        TransientOpener.Region,
        transientFocusOpener(TransientOpener.RailCommands, false, DesktopFocusRegion.Toolbar))
    assertEquals(
        TransientOpener.Region,
        transientFocusOpener(TransientOpener.RailCommands, true, DesktopFocusRegion.Editor))
  }

  @Test
  fun transientDismissalChoosesOnlyAttachedOwnerRegions() {
    assertEquals(
        DesktopFocusRegion.StatusBar,
        transientFocusRegion(DesktopFocusRegion.StatusBar, true, Workspace.Editor, false, true))
    assertEquals(
        DesktopFocusRegion.Editor,
        transientFocusRegion(
            DesktopFocusRegion.RightToolWindow, true, Workspace.Summary, true, true))
    assertEquals(
        DesktopFocusRegion.Editor,
        transientFocusRegion(
            DesktopFocusRegion.RightToolWindow, true, Workspace.Editor, false, true))
    assertEquals(
        DesktopFocusRegion.Toolbar,
        transientFocusRegion(DesktopFocusRegion.StatusBar, false, Workspace.Editor, true, true))
    assertEquals(
        null,
        transientFocusRegion(DesktopFocusRegion.Toolbar, false, Workspace.Summary, false, false))
  }

  @Test
  fun keyboardWorkspaceOrderUsesExplicitStableMappings() {
    assertEquals(Workspace.Analysis, nextWorkspace(Workspace.Summary))
    assertEquals(Workspace.Performance, nextWorkspace(Workspace.Analysis))
    assertEquals(Workspace.Bugs, nextWorkspace(Workspace.Performance))
    assertEquals(Workspace.Security, nextWorkspace(Workspace.Bugs))
    assertEquals(Workspace.Editor, nextWorkspace(Workspace.Security))
    assertEquals(Workspace.Summary, nextWorkspace(Workspace.Editor))
  }

  @Test
  fun analysisPollingStopsForNoContentAndTerminalJobs() {
    assertTrue(analysisRunFixture().copy(status = "running").isActive())
    assertTrue(analysisRunFixture().copy(status = "pausing").isActive())
    assertTrue(!analysisRunFixture().copy(status = "paused").isActive())
    assertTrue(!analysisRunFixture().copy(status = "completed").isActive())
  }

  @Test
  fun onlyFreshLocatedFindingsCanPrepareFixes() {
    val task = BugTaskSpec("1", "main.go", "Run", "func Run()", listOf("Return errors."))
    assertTrue(
        findingCanPrepareFix(
            UnifiedFinding(
                freshness = "fresh",
                location = FindingLocation(path = "main.go", symbol = "Run"),
                taskSpec = task)))
    assertTrue(
        !findingCanPrepareFix(
            UnifiedFinding(
                freshness = "stale",
                location = FindingLocation(path = "main.go", symbol = "Run"),
                taskSpec = task)))
    assertTrue(!findingCanPrepareFix(UnifiedFinding(freshness = "fresh")))
  }

  @Test
  fun providerDestinationAndContextManifestCountsAreExplicit() {
    val remote =
        ScopedModel(
            scope = "function", profile = "function", model = "cloud-model", remoteProvider = true)
    val local =
        ScopedModel(
            scope = "bug", profile = "bug", model = "local-model", reasoningEffort = "medium")
    assertTrue(modelDestinationLabel(ModelScope.Function, remote).contains("remote provider"))
    assertTrue(modelDestinationLabel(ModelScope.Function, remote).contains("confirmation required"))
    assertTrue(modelDestinationLabel(ModelScope.Bug, local).contains("local provider"))
    assertTrue(modelDestinationLabel(ModelScope.Bug, local).contains("local-model"))
    assertTrue(modelDestinationLabel(ModelScope.Bug, local).contains("reasoning: medium"))
    assertEquals(
        "1 included · 1 excluded · 12 estimated tokens · truncated",
        contextManifestSummary(
            ContextManifest(
                included = listOf(ContextFile("main.go", 12, "hash", 6)),
                excluded = listOf(ContextDecision("secret.env", false, "secret")),
                estimatedTokens = 12,
                truncated = true,
            )))
  }
}
