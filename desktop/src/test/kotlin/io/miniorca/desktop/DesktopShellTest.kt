package io.miniorca.desktop

import androidx.compose.ui.input.key.Key
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

class DesktopShellTest {
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
  fun narrowWindowsUseDrawersInsteadOfSqueezingThreePanes() {
    assertTrue(useNarrowLayout(999f))
    assertTrue(!useNarrowLayout(1_000f))
    assertEquals("Files", narrowDrawerLabel(NarrowDrawer.Files))
    assertEquals("Context", narrowDrawerLabel(NarrowDrawer.Context))
  }

  @Test
  fun editorChromeAndDrawerActionsAreScopedToEditor() {
    assertTrue(editorChromeVisible(Workspace.Editor))
    assertTrue(!editorChromeVisible(Workspace.Summary))
    assertTrue(!editorChromeVisible(Workspace.Analysis))
    assertTrue(!editorChromeVisible(Workspace.Bugs))
    assertTrue(editorDrawerActionsVisible(Workspace.Editor, 999f))
    assertTrue(!editorDrawerActionsVisible(Workspace.Editor, 1_000f))
    assertTrue(!editorDrawerActionsVisible(Workspace.Analysis, 999f))
    assertEquals(NarrowDrawer.Context, contextDrawerForSourceSelection(Workspace.Editor, 999f))
    assertEquals(null, contextDrawerForSourceSelection(Workspace.Editor, 1_000f))
    assertEquals(null, contextDrawerForSourceSelection(Workspace.Analysis, 999f))
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
  fun toolWindowBarMapsTheFiveRetainedNavigationDestinations() {
    assertEquals(
        listOf(
            LeftToolWindow.Summary,
            LeftToolWindow.Analysis,
            LeftToolWindow.Performance,
            LeftToolWindow.Problems,
            LeftToolWindow.Editor),
        LeftToolWindow.entries.toList())
    assertEquals("Summary", leftToolWindowLabel(LeftToolWindow.Summary))
    assertEquals("Analysis", leftToolWindowLabel(LeftToolWindow.Analysis))
    assertEquals("Bugs & Problems", leftToolWindowLabel(LeftToolWindow.Problems))
    assertEquals("Editor", leftToolWindowLabel(LeftToolWindow.Editor))
    assertEquals(Workspace.Bugs, workspaceForLeftToolWindow(LeftToolWindow.Problems))
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
  fun toolbarKeepsTheBranchAndSearchDetailAtTheSupportedWidths() {
    assertEquals(ToolbarPresentation(true, true, true), toolbarPresentation(1_220f))
    assertEquals(ToolbarPresentation(true, false, true), toolbarPresentation(1_000f))
    assertEquals(ToolbarPresentation(false, false, false), toolbarPresentation(999f))
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
  }

  @Test
  fun bottomToolTabsUseLabeledIconsWithoutChangingTheirStableIdentity() {
    assertEquals("Bugs & Problems", bottomToolWindowLabel(BottomToolWindow.Problems))
    assertEquals(DesktopIcon.Problems, bottomToolWindowIcon(BottomToolWindow.Problems))
    assertEquals(DesktopIcon.Summary, bottomToolWindowIcon(BottomToolWindow.Checks))
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
  fun keyboardWorkspaceOrderUsesExplicitStableMappings() {
    assertEquals(Workspace.Analysis, nextWorkspace(Workspace.Summary))
    assertEquals(Workspace.Performance, nextWorkspace(Workspace.Analysis))
    assertEquals(Workspace.Bugs, nextWorkspace(Workspace.Performance))
    assertEquals(Workspace.Editor, nextWorkspace(Workspace.Bugs))
    assertEquals(Workspace.Summary, nextWorkspace(Workspace.Editor))
  }

  @Test
  fun analysisPollingStopsForNoContentAndTerminalJobs() {
    assertTrue(!shouldPollAnalyzeAll(null))
    assertTrue(shouldPollAnalyzeAll(AnalyzeAllJob(status = "running")))
    assertTrue(!shouldPollAnalyzeAll(AnalyzeAllJob(status = "paused")))
    assertTrue(!shouldPollAnalyzeAll(AnalyzeAllJob(status = "completed")))
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
