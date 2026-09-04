package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull
import kotlin.test.assertTrue

class ExplorerPaneTest {
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
  fun explorerUsesTheSharedFileIconWithoutChangingItsIndexedIdentity() {
    assertEquals(DesktopIcon.File, explorerFileIcon("Go"))
    assertEquals(DesktopIcon.File, explorerFileIcon(""))
  }
}
