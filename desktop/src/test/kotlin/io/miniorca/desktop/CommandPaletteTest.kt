package io.miniorca.desktop

import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.text.TextRange
import androidx.compose.ui.text.input.TextFieldValue
import kotlin.coroutines.CoroutineContext
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue
import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json

class CommandPaletteTest {
  @Test
  fun filePaletteRoutesProtectedWorkThroughConfirmationAndKeepsItOnCancelOrEscape() {
    for (work in listOf("draft", "session", "composer")) {
      for (action in listOf("cancel", "escape", "confirm")) {
        FileNavigationUiFixture(work).use { navigation ->
          var visible by mutableStateOf(true)
          var query by mutableStateOf("main")
          val previous = navigation.presenter.snapshot.value.state
          val originalInput = navigation.input()
          ComposeVisualFixture(800, 650) {
                if (visible)
                    CommandPaletteDialog(
                        mode = PaletteMode.Files,
                        query = query,
                        onQuery = {},
                        onMode = {},
                        files = previous.index!!.files,
                        symbols = previous.symbols,
                        hasActiveFile = true,
                        onSelectFile = {
                          visible = false
                          navigation.route(it)
                        },
                        onSelectSymbol = {},
                        onSelectAction = {},
                        onDismiss = { visible = false })
                navigation.DiscardDialog()
              }
              .use { fixture ->
                fixture.render()
                assertTrue(fixture.pressKey(Key.Enter))
                fixture.render()
                assertFalse(visible)
                assertEquals(null, navigation.pending)
                navigation.runPending()
                assertTrue(navigation.calls.isEmpty())
                assertEquals(originalInput, navigation.input())
                assertEquals(0, navigation.clears)
                query = "other"
                visible = true
                fixture.render()
                assertTrue(fixture.pressKey(Key.Enter))
                fixture.render()
                assertFalse(visible)
                assertTrue(navigation.pending != null)
                navigation.runPending()
                assertTrue(navigation.calls.isEmpty())
                assertEquals(
                    previous.selection, navigation.presenter.snapshot.value.state.selection)
                assertEquals(previous.chat, navigation.presenter.snapshot.value.state.chat)
                assertEquals(previous.review, navigation.presenter.snapshot.value.state.review)
                assertEquals(originalInput, navigation.input())
                when (action) {
                  "cancel" -> fixture.clickText(if (work == "draft") "Keep draft" else "Keep work")
                  "escape" -> assertTrue(fixture.pressKey(Key.Escape))
                  "confirm" ->
                      fixture.clickText(if (work == "draft") "Discard draft" else "Discard work")
                }
                fixture.render()
                navigation.runPending()
                assertEquals(null, navigation.pending)
                if (action == "confirm") {
                  assertEquals(
                      "other.go", navigation.presenter.snapshot.value.state.selectedFile?.path)
                  assertEquals(1, navigation.clears)
                  assertEquals(TextFieldValue() to TextFieldValue(), navigation.input())
                  assertEquals(1, navigation.calls.count { it.contains("files/info?") })
                } else {
                  assertEquals(
                      previous.selection, navigation.presenter.snapshot.value.state.selection)
                  assertEquals(previous.chat, navigation.presenter.snapshot.value.state.chat)
                  assertEquals(previous.review, navigation.presenter.snapshot.value.state.review)
                  assertEquals(originalInput, navigation.input())
                  assertEquals(0, navigation.clears)
                  assertTrue(navigation.calls.isEmpty())
                }
              }
        }
      }
    }
  }

  @Test
  fun paletteFocusStaysOnInputUntilUserTabsAndEscapeOnlyDismisses() {
    var query by mutableStateOf("")
    var visible by mutableStateOf(true)
    var dismissals = 0
    var selections = 0
    ComposeVisualFixture(800, 650) {
          if (visible)
              CommandPaletteDialog(
                  mode = PaletteMode.Actions,
                  query = query,
                  onQuery = { query = it },
                  onMode = {},
                  files = emptyList(),
                  symbols = emptyList(),
                  hasActiveFile = false,
                  onSelectFile = {},
                  onSelectSymbol = {},
                  onSelectAction = { selections++ },
                  onDismiss = {
                    dismissals++
                    visible = false
                  })
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.isFocusedControl("Filter commands"))
          assertFalse(fixture.isFocusedControl("Close"))
          fixture.setFocusedText("analysis")
          fixture.render()
          assertEquals("analysis", query)
          assertEquals(0, selections)
          assertTrue(fixture.pressKey(Key.Escape))
          fixture.render()
          assertEquals(1, dismissals)
          assertEquals(0, selections)
        }
  }

  @Test
  fun openingActionsAndClosingWithoutSelectionDispatchesNothing() {
    var selections = 0
    var visible by mutableStateOf(false)
    ComposeVisualFixture(800, 650) {
          if (visible)
              CommandPaletteDialog(
                  mode = PaletteMode.Actions,
                  query = "",
                  onQuery = {},
                  onMode = {},
                  files = emptyList(),
                  symbols = emptyList(),
                  hasActiveFile = true,
                  onSelectFile = {},
                  onSelectSymbol = {},
                  onSelectAction = { selections++ },
                  onDismiss = { visible = false })
        }
        .use { fixture ->
          fixture.render()
          visible = true
          fixture.render()
          assertTrue(fixture.isFocusedControl("Filter commands"))
          assertEquals(0, selections)
          fixture.clickText("Close")
          fixture.render()
          assertEquals(0, selections)
        }
  }

  @Test
  fun projectActionScopeRemainsReadableAndKeyboardActivationIsExplicit() {
    listOf(800 to 650, 1280 to 600).forEach { (width, height) ->
      val selected = mutableListOf<String>()
      ComposeVisualFixture(width, height, 1.5f) {
            CommandPaletteDialog(
                mode = PaletteMode.Actions,
                query = "analysis",
                onQuery = {},
                onMode = {},
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
  fun modeControlsKeepTheScopedLabelsAndOnlyShowWiredShortcuts() {
    assertEquals("Files · ⌘P", commandSearchTitle(PaletteMode.Files))
    assertEquals("Symbols · active file · ⌘⇧O", commandSearchTitle(PaletteMode.Symbols))
    assertEquals("Commands", commandSearchTitle(PaletteMode.Actions))
    assertEquals("Filter files", commandSearchFieldLabel(PaletteMode.Files))
    assertEquals("Filter active-file symbols", commandSearchFieldLabel(PaletteMode.Symbols))
    assertEquals("Filter commands", commandSearchFieldLabel(PaletteMode.Actions))
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

/** Uses the same composer-owned route and discard surface as the application callbacks. */
internal class FileNavigationUiFixture(work: String) : AutoCloseable {
  private val dispatcher =
      object : CoroutineDispatcher() {
        val pending = ArrayDeque<Runnable>()

        override fun dispatch(context: CoroutineContext, block: Runnable) {
          pending += block
        }
      }
  private val scope = CoroutineScope(SupervisorJob() + dispatcher)
  val calls = mutableListOf<String>()
  private val file =
      ProjectFileInfo(
          "main.go",
          "base",
          "main.go",
          language = "Go",
          sizeBytes = 12,
          lineCount = 1,
          modifiedAt = "",
          binary = false,
          content = "package main")
  val presenter =
      DesktopWorkflowPresenter(
          ApiClient(
              transport =
                  DaemonTransport { method, path, _ ->
                    calls += "$method $path"
                    val body =
                        when {
                          path.contains("files/info?") ->
                              Json.encodeToString(file.copy(path = "other.go", name = "other.go"))
                          path.contains("files/symbols?") ->
                              Json.encodeToString(
                                  SymbolsResponse("project", "revision", "other.go"))
                          else -> "{}"
                        }
                    TransportResponse(200, body)
                  }),
          LastProjectStore(InMemoryPreferences()),
          scope,
          dispatcher)
  var message =
      if (work == "composer") TextFieldValue("keep message", TextRange(1, 4)) else TextFieldValue()
  var constraints =
      if (work == "composer") TextFieldValue("keep constraints", composition = TextRange(0, 4))
      else TextFieldValue()
  var pending by mutableStateOf<PendingDraftDiscard.FileNavigation?>(null)
  var clears = 0

  init {
    presenter.dispatch(
        DesktopEvent.ProjectLoaded(
            resultProjectFixture(),
            resultIndexFixture()
                .copy(
                    files =
                        listOf("main.go", "other.go").map {
                          IndexedFile(it, "base", "Go", false)
                        })))
    val symbol = SymbolInfo("Run", "function", confidence = "exact", atomicTarget = true)
    presenter.dispatch(DesktopEvent.FileLoaded(file, listOf(symbol)))
    presenter.dispatch(DesktopEvent.SymbolSelected(symbol))
    if (work != "composer")
        presenter.dispatch(
            DesktopEvent.ChatLoaded(
                ChatSession("session", "project", "revision", "base", "main.go")))
    if (work == "draft")
        presenter.dispatch(
            DesktopEvent.DraftLoaded(
                DeclarationDraft(
                    "draft",
                    "project",
                    "revision",
                    "base",
                    "main.go",
                    "replace_symbol",
                    "Run",
                    "func Run() {}",
                    revision = 1,
                    hash = "draft-hash",
                    validation =
                        DeclarationValidation(
                            true, "strict_symbol", diff = UnifiedDiff("main.go", "main.go")))))
  }

  fun input(): Pair<TextFieldValue, TextFieldValue> = message to constraints

  fun route(path: String) {
    routeFileNavigationRequest(presenter, path, message, constraints, ::input, ::clear) {
      pending = it
    }
  }

  private fun clear() {
    clears++
    message = TextFieldValue()
    constraints = TextFieldValue()
  }

  @Composable
  fun DiscardDialog() {
    pending?.let { approval ->
      DraftDiscardDialog(
          approval.currentDraft,
          approval.nextLabel,
          onDiscard = {
            val current = pending
            pending = null
            current?.let { confirmFileNavigationDiscard(presenter, it, ::input, ::clear) }
          },
          onCancel = { pending = null })
    }
  }

  fun runPending() {
    while (dispatcher.pending.isNotEmpty()) dispatcher.pending.removeFirst().run()
  }

  override fun close() {
    presenter.close()
    scope.cancel()
  }
}
