package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class CommandPaletteTest {
  @Test
  fun freshFileAnalysisCanBeRefreshedFromCommandsWithoutAddingASecondInspectorAction() {
    val freshActions = availableCommandActions(FileAnalysis("main.go", "fresh"))

    assertTrue("refresh_file_analysis" in freshActions)
    assertTrue("create_declaration" in freshActions)
    assertFalse(
        "refresh_file_analysis" in availableCommandActions(FileAnalysis("main.go", "stale")))
    assertEquals("Refresh file analysis", commandActionLabel("refresh_file_analysis"))
    assertEquals("Create declaration", commandActionLabel("create_declaration"))
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

    val fileResults = commandSearchResults(PaletteMode.Files, "", files, symbols, null, true)
    val symbolResults = commandSearchResults(PaletteMode.Symbols, "", files, symbols, null, true)

    assertEquals(listOf("internal/alpha.go", "README.md", "zeta.go"), fileResults.map { it.path })
    assertEquals(listOf("Alpha", "Zed"), symbolResults.map { it.label })
    assertTrue(fileResults.all { it.type == CommandSearchResultType.File })
    assertTrue(symbolResults.all { it.type == CommandSearchResultType.Symbol })
  }

  @Test
  fun actionsKeepFileCommandsScopedAndExposePerformanceNavigation() {
    assertEquals(
        listOf("open_performance"),
        availableCommandActions(FileAnalysis("main.go", "fresh"), hasActiveFile = false))
    val freshActions =
        commandSearchResults(
            PaletteMode.Actions,
            "",
            emptyList(),
            emptyList(),
            FileAnalysis("main.go", "fresh"),
            hasActiveFile = true)

    assertEquals(
        listOf(
            "Create declaration",
            "Document",
            "Fix",
            "Open Performance workspace",
            "Refactor",
            "Refresh file analysis"),
        freshActions.map { it.label })
    assertTrue(freshActions.any { it.label == "Open Performance workspace" })
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
                null,
                hasActiveFile = false)
            .single()
    val actionResult =
        commandSearchResults(
                PaletteMode.Actions, "create", emptyList(), emptyList(), null, hasActiveFile = true)
            .single()

    assertEquals(
        CommandSearchActivation.File("internal/main.go"), commandSearchActivation(fileResult))
    assertEquals(
        CommandSearchActivation.Action("create_declaration"), commandSearchActivation(actionResult))
  }
}
