package io.miniorca.desktop

import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.input.key.Key
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

class ExplorerPaneTest {
  @Test
  fun treeFocusSurvivesExpansionAndReconcilesCollapsedFilteredAndRemovedPaths() {
    val files =
        listOf("src/a.go", "src/nested/b.go", "z.go").map { IndexedFile(it, "hash", "Go", false) }
    var state by
        mutableStateOf(
            ExplorerPaneState(
                ProjectIndex("project", "revision", files = files), "z.go", "", emptySet(), false))
    val opened = mutableListOf<String>()
    val toggled = mutableListOf<String>()
    val actions =
        ExplorerPaneActions(
            { state = state.copy(filter = it) },
            { path ->
              toggled += path
              state =
                  state.copy(
                      collapsedDirectories =
                          if (path in state.collapsedDirectories) state.collapsedDirectories - path
                          else state.collapsedDirectories + path)
            },
            { state = state.copy(collapsedDirectories = explorerDirectories(state.index!!.files)) },
            {
              state =
                  state.copy(
                      filter = "",
                      collapsedDirectories =
                          revealExplorerPath(
                              state.index!!.files, state.collapsedDirectories, state.selectedPath))
            },
            opened::add)
    ComposeVisualFixture(360, 400) { ExplorerPane(state, actions, Modifier.fillMaxSize()) }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.requestDescriptionFocus("Indexed file tree"))
          fixture.render()
          fun assertFocused(path: String, expanded: Boolean = true) {
            val row = explorerRows(state.index!!.files).single { it.path == path }
            val description = explorerRowDescription(row, path == state.selectedPath, expanded)
            fixture.awaitVisibleDescription(description)
            assertTrue(fixture.descriptionState(description)!!.contains("Keyboard focused"))
          }
          assertFocused("z.go")
          repeat(4) {
            assertTrue(fixture.pressKey(Key.DirectionUp))
            fixture.render()
          }
          assertFocused("src")
          assertTrue(fixture.pressKey(Key.DirectionLeft))
          fixture.render()
          assertFocused("src", expanded = false)
          assertEquals(setOf("src"), state.collapsedDirectories)
          assertTrue(fixture.pressKey(Key.DirectionRight))
          fixture.render()
          assertFocused("src")
          assertTrue(fixture.pressKey(Key.DirectionRight))
          fixture.render()
          assertFocused("src/nested")
          for (key in listOf(Key.Spacebar, Key.Enter)) {
            assertTrue(fixture.pressKey(key))
            fixture.render()
            assertFocused("src/nested", expanded = key == Key.Enter)
          }
          assertEquals(listOf("src", "src", "src/nested", "src/nested"), toggled)
          assertTrue(fixture.pressKey(Key.DirectionRight))
          fixture.render()
          assertFocused("src/nested/b.go")
          state = state.copy(collapsedDirectories = setOf("src/nested"))
          fixture.render()
          assertFocused("src/nested", expanded = false)
          // Filtering reveals required ancestors even when they were locally collapsed.
          state = state.copy(filter = "b.go")
          fixture.render()
          assertFocused("src/nested")
          fixture.pressKey(Key.DirectionDown)
          fixture.render()
          assertFocused("src/nested/b.go")
          state = state.copy(filter = "a.go")
          fixture.render()
          assertFocused("src")
          state = state.copy(filter = "", collapsedDirectories = emptySet())
          fixture.render()
          assertFocused("src")
          fixture.pressKey(Key.DirectionDown)
          fixture.render()
          assertFocused("src/nested")
          state = state.copy(index = state.index!!.copy(projectRevision = "new-revision"))
          fixture.render()
          assertFocused("src/nested", expanded = true)
          state = state.copy(index = state.index!!.copy(files = files.filter { it.path == "z.go" }))
          fixture.render()
          assertFocused("z.go")
          assertTrue(opened.isEmpty(), "Reconciliation never activates the newly focused row")
        }
  }

  @Test
  fun pointerActivationCollapseAllAndRevealKeepOneKeyboardTreeStop() {
    val files = listOf(IndexedFile("src/nested/active.go", "hash", "Go", false))
    var state by
        mutableStateOf(
            ExplorerPaneState(
                ProjectIndex("project", "revision", files = files),
                files.single().path,
                "",
                emptySet(),
                false))
    val opened = mutableListOf<String>()
    ComposeVisualFixture(360, 400) {
          ExplorerPane(
              state,
              ExplorerPaneActions(
                  {},
                  { path -> state = state.copy(collapsedDirectories = setOf(path)) },
                  { state = state.copy(collapsedDirectories = explorerDirectories(files)) },
                  {
                    state =
                        state.copy(
                            collapsedDirectories =
                                revealExplorerPath(
                                    files, state.collapsedDirectories, state.selectedPath))
                  },
                  opened::add),
              Modifier.fillMaxSize())
        }
        .use { fixture ->
          fixture.render()
          fixture.clickText("nested")
          fixture.render()
          assertEquals(setOf("src/nested"), state.collapsedDirectories)
          assertEquals(
              "Collapsed, Keyboard focused",
              fixture.descriptionState("Folder src/nested, collapsed"))
          fixture.pressKey(Key.DirectionLeft)
          fixture.render()
          assertEquals(
              "Expanded, Keyboard focused", fixture.descriptionState("Folder src, expanded"))
          fixture.pressKey(Key.Tab, shift = true)
          fixture.render()
          assertTrue(fixture.isFocusedControl("Filter indexed relative file paths"))
          fixture.clickDescription("Collapse all folders")
          fixture.render()
          assertEquals(setOf("src", "src/nested"), state.collapsedDirectories)
          assertFalse(fixture.hasText("active.go"))
          fixture.clickDescription("Reveal active file")
          fixture.render()
          assertEquals(emptySet(), state.collapsedDirectories)
          assertTrue(fixture.hasText("active.go"))
          assertTrue(opened.isEmpty())
          fixture.clickText("active.go")
          fixture.render()
          assertEquals(listOf("src/nested/active.go"), opened)
          assertTrue(fixture.requestDescriptionFocus("Indexed file tree"))
          fixture.render()
          fixture.pressKey(Key.Spacebar)
          fixture.render()
          assertEquals(List(2) { "src/nested/active.go" }, opened)
        }
  }

  @Test
  fun replacementFeedbackKeepsTheOldRowSelectedAndRetryEmitsOnlyTheFailedDestination() {
    val destination = "internal/module-with-a-very-long-name/nested/package/failed_destination.go"
    val files = listOf("main.go", destination).map { IndexedFile(it, "hash", "Go", false) }
    val selected = mutableListOf<String>()
    var state by
        mutableStateOf(
            ExplorerPaneState(
                ProjectIndex("project", "revision", files = files),
                "main.go",
                "",
                emptySet(),
                false,
                readError = "Permission denied",
                failedFilePath = destination))
    ComposeVisualFixture(360, 650, 1.5f) {
          ExplorerPane(
              state, ExplorerPaneActions({}, {}, {}, {}, selected::add), Modifier.fillMaxSize())
        }
        .use { fixture ->
          fixture.render("explorer-failed-destination-360-1.5")
          fixture.assertTextFits("Retry opening file")
          assertTrue(fixture.hasDescription("Go file main.go at main.go, Not analyzed, selected"))
          assertTrue(fixture.hasDescription("Failed destination: $destination"))
          assertTrue(fixture.hasDescription("Retry opening $destination"))
          assertEquals(emptyList(), selected)
          fixture.horizontalScrollBy("file-read-path", 10000f)
          fixture.render()
          assertTrue(fixture.horizontalScrollValue("file-read-path") > 0f)
          assertTrue(fixture.requestFocus("Retry opening file"))
          fixture.render()
          fixture.pressKey(Key.Enter)
          fixture.render()
          assertEquals(
              listOf(destination), selected, "Keyboard retry must not activate the retained row")
          state = state.copy(pendingFilePath = destination, failedFilePath = null, readError = null)
          fixture.render("explorer-pending-destination-360-1.5")
          assertTrue(fixture.hasDescription("Pending destination: $destination"))
          assertTrue(fixture.hasDescription("Go file main.go at main.go, Not analyzed, selected"))
          assertFalse(fixture.hasText("Retry opening file"))
          assertEquals(listOf(destination), selected)
        }
  }

  @Test
  fun initialFileReadFailureCanRetryEvenWithoutAnIndexAndDoesNotPretendAFileIsLoaded() {
    val selected = mutableListOf<String>()
    ComposeVisualFixture(360, 650, 1.5f) {
          ExplorerPane(
              ExplorerPaneState(
                  null,
                  null,
                  "",
                  emptySet(),
                  false,
                  projectAvailable = true,
                  readError = "Symbols unavailable",
                  failedFilePath = "main.go"),
              ExplorerPaneActions({}, {}, {}, {}, selected::add),
              Modifier.fillMaxSize())
        }
        .use { fixture ->
          fixture.render("explorer-initial-read-failure-360-1.5")
          fixture.assertTextFits("Retry opening file")
          assertTrue(fixture.hasDescription("Failed destination: main.go"))
          assertFalse(fixture.hasText("The current file remains open."))
          assertEquals(emptyList(), selected)
          fixture.clickText("Retry opening file")
          assertEquals(listOf("main.go"), selected)
        }
  }

  @Test
  fun localReadFailureStaysAboveIndexedRowsAndEmptyGuidanceDoesNotReplaceIt() {
    val files = listOf(IndexedFile("main.go", "hash", "Go", false))
    var opens = 0
    var selected = 0
    val actions = ExplorerPaneActions({}, {}, {}, {}, { selected++ }, { opens++ })
    for (index in listOf(null, ProjectIndex("project", "revision", files = files))) {
      ComposeVisualFixture(360, 500, 1.5f) {
            ExplorerPane(
                ExplorerPaneState(
                    index,
                    null,
                    if (index != null) "no match" else "",
                    emptySet(),
                    false,
                    projectAvailable = index != null,
                    readError = "Permission denied"),
                actions,
                Modifier.fillMaxSize())
          }
          .use { fixture ->
            fixture.render()
            assertTrue(fixture.hasText("Reading local file data failed. Permission denied"))
            assertTrue(!fixture.hasText("No project open"))
            if (index == null) {
              fixture.clickText("Open project")
              assertEquals(1, opens)
            } else {
              assertTrue(fixture.hasText("No matching files"))
            }
          }
    }
    assertEquals(0, selected)
  }

  @Test
  fun noProjectGuidanceOffersTheExistingOpenAction() {
    var opens = 0
    ComposeVisualFixture(360, 500) {
          ExplorerPane(
              ExplorerPaneState(null, null, "", emptySet(), false),
              ExplorerPaneActions({}, {}, {}, {}, {}, { opens++ }),
              Modifier.fillMaxSize())
        }
        .use { fixture ->
          fixture.render()
          fixture.clickText("Open project")
          assertEquals(1, opens)
        }
  }

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
