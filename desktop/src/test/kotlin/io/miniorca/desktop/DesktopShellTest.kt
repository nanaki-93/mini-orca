package io.miniorca.desktop

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material.Text
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.mutableStateOf
import androidx.compose.ui.Modifier
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.platform.testTag
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
                layout = DesktopLayoutState(activeRightToolWindow = RightToolWindow.Review),
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
          fixture.resize(800, 650)
          fixture.render()
          assertEquals(selected, shell.value.app.selectedFile)
          assertTrue(fixture.hasDescription("Source file · another.go"))
          assertTrue(fixture.hasText("package selectedfile"))
          assertEquals(RightToolWindow.Review, shell.value.layout.activeRightToolWindow)
          assertTrue(fixture.hasDescription("Review tool window tab, selected"))
          assertTrue(fixture.taggedBounds("desktop-canvas-focus").width > 0)
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
                  leftDivider = {},
                  rightDivider = {})
            }
          }
          .use { fixture ->
            fixture.render()
            assertTrue(fixture.taggedBounds("canvas").width > 0)
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
  fun keyboardSplittersUseTheSameBoundedResizeStepAsPointerSplitters() {
    assertEquals(-KEYBOARD_SPLITTER_STEP, verticalSplitterKeyboardDelta(Key.DirectionLeft))
    assertEquals(KEYBOARD_SPLITTER_STEP, verticalSplitterKeyboardDelta(Key.DirectionRight))
    assertEquals(null, verticalSplitterKeyboardDelta(Key.DirectionUp))
    assertEquals(KEYBOARD_SPLITTER_STEP, horizontalSplitterKeyboardDelta(Key.DirectionUp))
    assertEquals(-KEYBOARD_SPLITTER_STEP, horizontalSplitterKeyboardDelta(Key.DirectionDown))
    assertEquals(null, horizontalSplitterKeyboardDelta(Key.DirectionLeft))
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
