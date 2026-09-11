package io.miniorca.desktop

import androidx.compose.ui.input.key.Key
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class CommandPaletteTest {
  @Test
  fun projectActionScopeRemainsReadableAndKeyboardActivationIsExplicit() {
    listOf(800 to 650, 1280 to 600).forEach { (width, height) ->
      val selected = mutableListOf<String>()
      ComposeVisualFixture(width, height, 1.5f) {
            CommandPaletteDialog(
                mode = PaletteMode.Actions,
                query = "analysis",
                onQuery = {},
                files = emptyList(),
                symbols = emptyList(),
                hasActiveFile = false,
                onSelectFile = {},
                onSelectSymbol = {},
                onSelectAction = { selected += it },
                onDismiss = {})
          }
          .use { fixture ->
            fixture.render("palette-analysis-$width-1.5")
            fixture.assertTextFits("Start analysis")
            fixture.assertTextFits("View analysis progress")
            assertTrue(fixture.hasText(commandActionDetail("start_analysis")))
            assertTrue(selected.isEmpty())
            assertTrue(fixture.pressKey(Key.DirectionDown))
            fixture.render()
            assertTrue(selected.isEmpty())
            assertTrue(fixture.pressKey(Key.Enter))
            assertEquals(listOf("open_analysis"), selected)
          }
    }
  }

  @Test
  fun projectAnalysisAndResultsRemainAvailableWithoutASelectedFile() {
    val projectActions = availableCommandActions(hasActiveFile = false)
    assertEquals(
        listOf("start_analysis", "open_analysis", "open_bugs", "open_performance", "open_security"),
        projectActions)
    assertFalse("create_function" in projectActions)
    assertTrue("create_function" in availableCommandActions())
    assertTrue("create_type" in availableCommandActions())
    assertFalse("refresh_file_analysis" in availableCommandActions())
    assertEquals("Start analysis", commandActionLabel("start_analysis"))
    assertTrue(commandActionDetail("start_analysis").startsWith("Whole project"))
    assertTrue(commandActionDetail("open_bugs").contains("navigation only"))
    assertEquals(Workspace.Analysis, commandActionWorkspace("start_analysis"))
    assertEquals(Workspace.Analysis, commandActionWorkspace("open_analysis"))
    assertEquals(Workspace.Editor, commandActionWorkspace("create_function"))
    assertEquals(null, commandActionWorkspace("unknown"))
  }

  @Test
  fun searchOrdersIndexedResultsWithinTheirModeAndNeverAddsGlobalFiles() {
    val files =
        listOf(
            IndexedFile("zeta.go", "z", "Go", false),
            IndexedFile("internal/alpha.go", "a", "Go", false),
            IndexedFile("README.md", "r", "Markdown", false))
    val symbols =
        listOf(
            SymbolInfo("Zed", "function", startLine = 4, confidence = "exact", atomicTarget = true),
            SymbolInfo(
                "Alpha", "function", startLine = 10, confidence = "exact", atomicTarget = true))

    val fileResults = commandSearchResults(PaletteMode.Files, "", files, symbols, true)
    val symbolResults = commandSearchResults(PaletteMode.Symbols, "", files, symbols, true)

    assertEquals(listOf("internal/alpha.go", "README.md", "zeta.go"), fileResults.map { it.path })
    assertEquals(listOf("Alpha", "Zed"), symbolResults.map { it.label })
    assertTrue(fileResults.all { it.type == CommandSearchResultType.File })
    assertTrue(symbolResults.all { it.type == CommandSearchResultType.Symbol })
  }

  @Test
  fun actionsKeepFileCommandsScopedAndExposeWorkspaceNavigation() {
    assertEquals(
        listOf("start_analysis", "open_analysis", "open_bugs", "open_performance", "open_security"),
        availableCommandActions(hasActiveFile = false))
    val freshActions =
        commandSearchResults(
            PaletteMode.Actions, "", emptyList(), emptyList(), hasActiveFile = true)

    assertEquals(
        listOf(
            "Document",
            "Fix",
            "New Go function",
            "New Go type",
            "Refactor",
            "Start analysis",
            "View Bugs results",
            "View Performance results",
            "View Security results",
            "View analysis progress"),
        freshActions.map { it.label })
    assertTrue(freshActions.any { it.label == "View Performance results" })
    assertFalse(
        freshActions.any {
          it.label in
              setOf(
                  "Terminal",
                  "Run",
                  "Debug",
                  "New file",
                  "Branch actions",
                  "Settings & Help",
                  "Generate unit test")
        })
  }

  @Test
  fun keyboardSelectionWrapsAndHintsDescribeTheSharedInteraction() {
    assertEquals(1, nextCommandSearchSelection(0, 3, 1))
    assertEquals(2, nextCommandSearchSelection(0, 3, -1))
    assertEquals(0, nextCommandSearchSelection(2, 3, 1))
    assertEquals(-1, nextCommandSearchSelection(0, 0, 1))
    assertEquals("↑↓ select · Enter activate · Esc close", commandSearchHint(PaletteMode.Files))
    assertEquals("No focused action is available", commandSearchEmptyTitle(PaletteMode.Actions))
  }

  @Test
  fun activationUsesOnlyTheTypedResultReturnedByTheScopedSearch() {
    val fileResult =
        commandSearchResults(
                PaletteMode.Files,
                "main",
                listOf(IndexedFile("internal/main.go", "hash", "Go", false)),
                emptyList(),
                hasActiveFile = false)
            .single()
    val actionResult =
        commandSearchResults(
                PaletteMode.Actions, "function", emptyList(), emptyList(), hasActiveFile = true)
            .single()

    assertEquals(
        CommandSearchActivation.File("internal/main.go"), commandSearchActivation(fileResult))
    assertEquals(
        CommandSearchActivation.Action("create_function"), commandSearchActivation(actionResult))
  }
}
