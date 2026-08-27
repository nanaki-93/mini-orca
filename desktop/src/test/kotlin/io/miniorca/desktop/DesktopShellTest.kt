package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

class DesktopShellTest {
    @Test fun explorerGroupsRelativePathsAndKeepsOnlyMatchingBranches() {
        val files = listOf(
            IndexedFile("internal/project/index.go", "a", "Go", false, analysisStatus = "fresh"),
            IndexedFile("internal/project/path.go", "b", "Go", false, analysisStatus = "stale"),
            IndexedFile("README.md", "c", "Markdown", false),
        )

        val rows = explorerRows(files, "index")

        assertEquals(listOf("internal", "internal/project", "internal/project/index.go"), rows.map { it.path })
        assertTrue(rows.last().analysisStatus == "fresh")
    }

    @Test fun analysisBadgesUseTextualFreshnessLabels() {
        assertEquals("Fresh", statusBadgeStyle("fresh").label)
        assertEquals("Not analyzed", statusBadgeStyle("missing").label)
        assertEquals("Failed", statusBadgeStyle("failed").label)
    }

    @Test fun paneWidthsRemainWithinUsableBounds() {
        val widths = PaneWidths().withExplorer(1f).withAction(10_000f)
        assertEquals(180f, widths.explorer)
        assertEquals(560f, widths.action)
    }

    @Test fun narrowWindowsUseDrawersInsteadOfSqueezingThreePanes() {
        assertTrue(useNarrowLayout(999f))
        assertTrue(!useNarrowLayout(1_000f))
        assertEquals("Files", narrowDrawerLabel(NarrowDrawer.Files))
        assertEquals("Context", narrowDrawerLabel(NarrowDrawer.Context))
    }

    @Test fun largeExplorerKeepsAStableFilteredSelectionPath() {
        val files = (1..2_000).map { number ->
            IndexedFile("src/module$number/File$number.kt", "hash-$number", "Kotlin", false, analysisStatus = if (number % 2 == 0) "fresh" else "missing")
        }

        val rows = explorerRows(files, "File1999.kt")

        assertEquals(listOf("src", "src/module1999", "src/module1999/File1999.kt"), rows.map { it.path })
    }

    @Test fun explorerDescriptionsExposeRolesLanguageFreshnessAndExpansion() {
        val file = ExplorerRow("internal/main.go", "main.go", 1, false, "stale", "Go")
        val folder = ExplorerRow("internal", "internal", 0, true)

        assertEquals("Go file main.go, Stale, selected", explorerRowDescription(file, selected = true, expanded = false))
        assertEquals("Folder internal, collapsed", explorerRowDescription(folder, selected = false, expanded = false))
        assertEquals("DIR −", explorerRoleLabel(folder, expanded = true))
        assertEquals("12 indexed files · project revision revision", explorerProjectLabel(ProjectIndex("project", "revision", files = List(12) { IndexedFile("$it.go", "hash", "Go", false) })))
    }

    @Test fun collapsedFolderShowsItsChildrenAfterItIsExpanded() {
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

    @Test fun explorerPlacesDescendantsImmediatelyAfterTheirFolder() {
        val files = listOf(
            IndexedFile("cmd/root.go", "a", "Go", false),
            IndexedFile("cmd/sub/child.go", "b", "Go", false),
            IndexedFile("model/item.go", "c", "Go", false),
            IndexedFile("README.md", "d", "Markdown", false),
        )

        assertEquals(
            listOf("cmd", "cmd/sub", "cmd/sub/child.go", "cmd/root.go", "model", "model/item.go", "README.md"),
            explorerRows(files).map { it.path },
        )
    }

    @Test fun workspaceRailUsesLabelsAndTextCounts() {
        val counts = WorkspaceCounts(analyzedFiles = 4, verifiedFindings = 2, aiSuggestions = 3, drafts = 1)

        assertEquals("Summary", workspaceRailLabel(Workspace.Summary, counts))
        assertEquals("Analysis · 4 analyzed", workspaceRailLabel(Workspace.Analysis, counts))
        assertEquals("Bugs · 2 verified · 3 AI", workspaceRailLabel(Workspace.Bugs, counts))
        assertEquals("Editor · 1 drafts", workspaceRailLabel(Workspace.Editor, counts))
    }

    @Test fun topBarTextNamesProjectRevisionAndConnectionState() {
        val project = ProjectAnalysis("project", "revision-hash", "Long project name", "/tmp/project", "go", fileCount = 1, sourceFileCount = 1, totalLines = 1, analysisFile = "", summary = "", aiStatus = "fresh", analyzedAt = "")

        assertEquals("Long project name · revision revision-has", projectBreadcrumbLabel(project))
        assertTrue(connectionLabel(ConnectionState(connected = true, locality = "Local endpoint", model = "local model", latency = "12ms")).contains("Connected · Local endpoint"))
        assertTrue(connectionLabel(ConnectionState(label = "Daemon unavailable", locality = "Remote endpoint")).contains("Disconnected · Remote endpoint"))
    }

    @Test fun keyboardWorkspaceOrderCoversAllFourWorkspaces() {
        assertEquals(Workspace.Analysis, nextWorkspace(Workspace.Summary))
        assertEquals(Workspace.Bugs, nextWorkspace(Workspace.Analysis))
        assertEquals(Workspace.Editor, nextWorkspace(Workspace.Bugs))
        assertEquals(Workspace.Summary, nextWorkspace(Workspace.Editor))
    }

    @Test fun analysisPollingStopsForNoContentAndTerminalJobs() {
        assertTrue(!shouldPollAnalyzeAll(null))
        assertTrue(shouldPollAnalyzeAll(AnalyzeAllJob(status = "running")))
        assertTrue(!shouldPollAnalyzeAll(AnalyzeAllJob(status = "paused")))
        assertTrue(!shouldPollAnalyzeAll(AnalyzeAllJob(status = "completed")))
    }

    @Test fun onlyFreshLocatedFindingsCanPrepareFixes() {
        assertTrue(findingCanPrepareFix(UnifiedFinding(freshness = "fresh", location = FindingLocation(path = "main.go"))))
        assertTrue(!findingCanPrepareFix(UnifiedFinding(freshness = "stale", location = FindingLocation(path = "main.go"))))
        assertTrue(!findingCanPrepareFix(UnifiedFinding(freshness = "fresh")))
    }

    @Test fun providerDestinationAndContextManifestCountsAreExplicit() {
        assertTrue(contextDestinationLabel(remoteProvider = true).contains("remote provider"))
        assertTrue(contextDestinationLabel(remoteProvider = true).contains("confirmation required"))
        assertTrue(contextDestinationLabel(remoteProvider = false).contains("local provider"))
        assertEquals("1 included · 1 excluded · 12 estimated tokens · truncated", contextManifestSummary(ContextManifest(
            included = listOf(ContextFile("main.go", 12, "hash", 6)),
            excluded = listOf(ContextDecision("secret.env", false, "secret")),
            estimatedTokens = 12,
            truncated = true,
        )))
    }
}
