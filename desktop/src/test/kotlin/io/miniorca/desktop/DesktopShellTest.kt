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

    @Test fun analysisBadgesUseWordsAsWellAsSymbols() {
        assertEquals("✓ Fresh", analysisBadge("fresh"))
        assertEquals("○ Not analyzed", analysisBadge("missing"))
        assertEquals("! Failed", analysisBadge("failed"))
    }

    @Test fun paneWidthsRemainWithinUsableBounds() {
        val widths = PaneWidths().withExplorer(1f).withAction(10_000f)
        assertEquals(180f, widths.explorer)
        assertEquals(560f, widths.action)
    }

    @Test fun narrowWindowsUseDrawersInsteadOfSqueezingThreePanes() {
        assertTrue(useNarrowLayout(999f))
        assertTrue(!useNarrowLayout(1_000f))
    }

    @Test fun largeExplorerKeepsAStableFilteredSelectionPath() {
        val files = (1..2_000).map { number ->
            IndexedFile("src/module$number/File$number.kt", "hash-$number", "Kotlin", false, analysisStatus = if (number % 2 == 0) "fresh" else "missing")
        }

        val rows = explorerRows(files, "File1999.kt")

        assertEquals(listOf("src", "src/module1999", "src/module1999/File1999.kt"), rows.map { it.path })
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

    @Test fun workspaceNavigationUsesLabelsAndTextCounts() {
        val counts = WorkspaceCounts(analyzedFiles = 4, verifiedFindings = 2, aiSuggestions = 3, drafts = 1)

        assertEquals("Summary", workspaceNavigationLabel(Workspace.Summary, counts))
        assertEquals("Analysis · 4 analyzed", workspaceNavigationLabel(Workspace.Analysis, counts))
        assertEquals("Bugs · 2 verified · 3 AI", workspaceNavigationLabel(Workspace.Bugs, counts))
        assertEquals("Editor · 1 drafts", workspaceNavigationLabel(Workspace.Editor, counts))
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
}
