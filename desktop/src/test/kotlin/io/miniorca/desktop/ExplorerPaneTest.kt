package io.miniorca.desktop

import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.ui.Modifier
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull
import kotlin.test.assertTrue

class ExplorerPaneTest {
  @Test
  fun fileTreeKeepsStatusDescriptionsWithCompactDots() {
    val files =
        listOf("fresh", "stale", "failed", "ignored", "missing").map {
          IndexedFile("$it.go", it, "Go", false, analysisStatus = it)
        }
    ComposeVisualFixture(360, 500, 1.5f) {
          ExplorerPane(
              ExplorerPaneState(
                  ProjectIndex("project", "revision", files = files), null, "", emptySet(), false),
              ExplorerPaneActions({}, {}, {}, {}, {}),
              Modifier.fillMaxSize())
        }
        .use { fixture ->
          fixture.render("analysis-tree-status-dots")
          for (row in explorerRows(files)) {
            assertTrue(fixture.hasDescription(explorerRowDescription(row, false, true)))
          }
        }
  }

  @Test
  fun ignoredAndUnanalysedFilesHaveNoDotWhileAnalysisStatesKeepTheirMeaning() {
    for (status in listOf("ignored", "excluded", "skipped", "missing", "")) {
      assertNull(explorerStatusDotColor(status))
    }
    assertEquals(Success, explorerStatusDotColor("fresh"))
    assertEquals(Warning, explorerStatusDotColor("stale"))
    assertEquals(Error, explorerStatusDotColor("failed"))
    assertEquals(Information, explorerStatusDotColor("running"))
    assertTrue(
        explorerRowDescription(
                ExplorerRow("ignored.go", "ignored.go", 0, false, "ignored", "Go"), false, false)
            .contains("Ignored"))
  }

  private val files =
      listOf(
          IndexedFile("src/Main.kt", "main", "Kotlin", false),
          IndexedFile("src/nested/Worker.kt", "worker", "Kotlin", false),
          IndexedFile("README.md", "readme", "Markdown", false),
      )

  @Test
  fun keyboardTraversalMovesAcrossVisibleRowsAndActivatesOnlyIndexedFiles() {
    val rows = visibleExplorerRows(files, "", emptySet())

    assertEquals(
        "src/Main.kt",
        explorerTreeInteraction(rows, "README.md", emptySet(), ExplorerTreeKey.Previous)
            ?.focusedPath)
    assertEquals(
        "src/nested",
        explorerTreeInteraction(rows, "src", emptySet(), ExplorerTreeKey.Next)?.focusedPath)
    assertEquals(
        "src/Main.kt",
        explorerTreeInteraction(rows, "src/Main.kt", emptySet(), ExplorerTreeKey.Activate)
            ?.selectFile)
    assertNull(
        explorerTreeInteraction(rows, "src", emptySet(), ExplorerTreeKey.Activate)?.selectFile)
  }

  @Test
  fun keyboardExpansionAndCollapseRetainTheFocusedTreeNode() {
    val collapsed = setOf("src")
    val rows = visibleExplorerRows(files, "", collapsed)

    assertEquals(
        ExplorerTreeInteraction("src", toggleDirectory = "src"),
        explorerTreeInteraction(rows, "src", collapsed, ExplorerTreeKey.Expand))
    assertEquals(
        ExplorerTreeInteraction("src", toggleDirectory = "src"),
        explorerTreeInteraction(
            visibleExplorerRows(files, "", emptySet()),
            "src",
            emptySet(),
            ExplorerTreeKey.Collapse))
  }

  @Test
  fun revealActiveFileClearsOnlyItsAncestorCollapsesAndRejectsUnknownPaths() {
    assertEquals(
        emptySet(), revealExplorerPath(files, setOf("src", "src/nested"), "src/nested/Worker.kt"))
    assertEquals(setOf("src"), revealExplorerPath(files, setOf("src"), "missing/External.kt"))
  }

  @Test
  fun filteringKeepsDirectoriesNeededToDisambiguateDuplicateBasenames() {
    val duplicates = files + IndexedFile("examples/Main.kt", "example-main", "Kotlin", false)

    val rows = visibleExplorerRows(duplicates, "Main.kt", setOf("src", "examples"))

    assertEquals(listOf("examples", "examples/Main.kt", "src", "src/Main.kt"), rows.map { it.path })
    assertTrue(rows.filterNot { it.directory }.all { it.path.endsWith("Main.kt") })
  }

  @Test
  fun explorerUsesCompactTypeIconsWithoutChangingItsIndexedIdentity() {
    assertEquals(DesktopIcon.Code, explorerFileIcon("Go"))
    assertEquals(DesktopIcon.Document, explorerFileIcon("Markdown"))
    assertEquals(DesktopIcon.File, explorerFileIcon(""))
  }

  @Test
  fun fileRowsKeepDuplicateBasenamesVisibleAndExposeTheirProjectRelativePaths() {
    val duplicates =
        listOf(
            IndexedFile("cmd/worker/main.go", "worker", "Go", false),
            IndexedFile("cmd/server/main.go", "server", "Go", false),
        )
    ComposeVisualFixture(360, 300, 1.5f) {
          ExplorerPane(
              ExplorerPaneState(
                  ProjectIndex("project", "revision", files = duplicates),
                  "cmd/worker/main.go",
                  "",
                  emptySet(),
                  false),
              ExplorerPaneActions({}, {}, {}, {}, {}),
              Modifier.fillMaxSize())
        }
        .use { fixture ->
          fixture.render("explorer-project-relative-duplicates")
          assertEquals(2, fixture.textCount("main.go"))
          fixture.assertTextFits("worker")
          fixture.assertTextFits("server")
          assertTrue(
              fixture.hasDescription(
                  "Go file main.go at cmd/worker/main.go, Not analyzed, selected"))
          assertTrue(
              fixture.hasDescription(
                  "Go file main.go at cmd/server/main.go, Not analyzed, not selected"))
        }
  }
}
