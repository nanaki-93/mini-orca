package io.miniorca.desktop

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.width
import androidx.compose.material.Text
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.TextRange
import androidx.compose.ui.text.input.TextFieldValue
import androidx.compose.ui.unit.dp
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

class EditorWorkspaceTest {
  @Test
  fun appSourceSelectionCallbackPreservesUnsentComposerAndDraftWhenInspectingAnotherSymbol() {
    val source = "package main\nfunc Run() {}\nfunc Other() {}\n"
    val other =
        SymbolInfo(
            "Other",
            "function",
            startLine = 3,
            endLine = 3,
            confidence = "exact",
            atomicTarget = true)
    FileNavigationUiFixture("draft", source, listOf(other)).use { navigation ->
      val before = navigation.presenter.snapshot.value.state
      navigation.message = TextFieldValue("Keep my request")
      navigation.constraints = TextFieldValue("Keep my constraints")
      val input = navigation.input()
      var composerRequested = true
      val onSourceLineSelected =
          sourceLineSelectionAction(navigation.presenter) { composerRequested = false }
      ComposeVisualFixture(800, 650) {
            SourceEditorPane(
                before.project,
                before.selectedFile,
                before.symbols,
                before.selectedSymbol,
                before.selection.focusedLine,
                emptyList(),
                onSourceLineSelected)
          }
          .use { fixture ->
            fixture.render()
            fixture.tapSourceText(3, 6)
            fixture.render()
            val after = navigation.presenter.snapshot.value.state
            assertEquals(other, after.selectedSymbol)
            assertEquals(3, after.selection.focusedLine)
            assertEquals(before.review, after.review)
            assertEquals(before.chat, after.chat)
            assertEquals(input, navigation.input())
            assertFalse(composerRequested)
            navigation.runPending()
            assertTrue(navigation.calls.isEmpty())
          }
    }
  }

  @Test
  fun contextSelectionTabsAndCachedInspectionArePassiveAndRefactorPreparesTheExactTarget() {
    val other =
        SymbolInfo(
            "Other", "function", "func Other()", 3, 3, confidence = "exact", atomicTarget = true)
    FileNavigationUiFixture(
            "composer", "package main\nfunc Run() {}\nfunc Other() {}\n", listOf(other))
        .use { navigation ->
          val presenter = navigation.presenter
          presenter.dispatch(
              DesktopEvent.AnalysisLoaded(
                  FileAnalysis(
                      "main.go", "fresh", symbolExplanations = mapOf("Run" to "Cached prose"))))
          var tool by mutableStateOf(RightToolWindow.Context)
          var prepared: DirectEditRequest? = null
          var pending: PendingDraftDiscard.Replace? = null
          var sends = 0
          val chatFocus = FocusRequester()
          val initialInput = navigation.input()
          ComposeVisualFixture(800, 650) {
                val workflow by presenter.snapshot.collectAsState()
                val state = workflow.state
                Row(Modifier.fillMaxSize()) {
                  SourceEditorPane(
                      state.project,
                      state.selectedFile,
                      state.symbols,
                      state.selectedSymbol,
                      state.selection.focusedLine,
                      emptyList(),
                      sourceLineSelectionAction(presenter) {})
                  RightToolWindowContainer(
                      tool,
                      { tool = it },
                      { active, modifier ->
                        when (active) {
                          RightToolWindow.Context ->
                              ContextToolWindow(
                                  ContextToolWindowState(
                                      symbolInspectorUiState(
                                          state.selectedFile,
                                          state.symbols,
                                          state.selectedSymbol,
                                          state.analysis,
                                          false,
                                          InspectorProviderState(false, false),
                                          currentEditIdentity(state)),
                                      ScopedModel(),
                                      false,
                                      state.impact,
                                      state.gitStatus,
                                      fileAnalysis = state.analysis,
                                      project = state.project),
                                  ContextToolWindowActions(
                                      {},
                                      {},
                                      {},
                                      {},
                                      editSelected = { symbol ->
                                        routeContextRefactor(
                                            presenter,
                                            symbol,
                                            {
                                              prepared = it
                                              tool = RightToolWindow.Assistant
                                            },
                                            { pending = it })
                                      }),
                                  modifier)
                          RightToolWindow.Assistant -> {
                            val validation =
                                validateChatTarget(
                                    state.selectedFile,
                                    state.symbols,
                                    state.selectedSymbol,
                                    ChatEditMode.ReplaceSymbol,
                                    "")
                            AssistantToolWindow(
                                AssistantToolWindowState(
                                    state.project,
                                    state.selectedFile,
                                    state.chat.session,
                                    state.review.draft,
                                    state.review.editor,
                                    validation.target,
                                    ChatEditMode.ReplaceSymbol,
                                    "",
                                    "",
                                    false,
                                    ScopedModel(),
                                    false,
                                    chatFocus,
                                    FocusRequester(),
                                    selectedSymbol = state.selectedSymbol,
                                    targetValidation = validation),
                                AssistantConversationActions({}, {}, {}, {}, { sends++ }, {}),
                                DraftEditorActions({}, {}, {}),
                                modifier)
                            if (prepared != null)
                                LaunchedEffect(prepared) { chatFocus.requestFocus() }
                          }
                          RightToolWindow.Review -> Text("Review")
                        }
                      },
                      modifier = Modifier.weight(1f))
                }
              }
              .use { fixture ->
                fixture.render()
                assertTrue(fixture.hasText("Cached prose"))
                fixture.clickText("Declaration details")
                fixture.render()
                fixture.tapSourceText(1, 2)
                fixture.render()
                fixture.clickText("Details")
                fixture.render()
                fixture.clickText("Actions")
                fixture.render()
                assertEquals(initialInput, navigation.input())
                fixture.clickText("Assistant")
                fixture.render()
                fixture.clickText("Context")
                fixture.render()
                fixture.tapSourceText(3, 6)
                fixture.render()
                assertEquals(other, presenter.snapshot.value.state.selectedSymbol)
                assertNull(prepared)
                assertNull(pending)
                navigation.runPending()
                assertEquals(emptyList(), navigation.calls)
                fixture.clickText("Refactor")
                fixture.render()
                assertEquals("Other", prepared?.target?.symbol)
                assertEquals(other, prepared?.selectedSymbol)
                assertTrue(fixture.hasText("Replace selected declaration · Other"))
                assertTrue(fixture.isDescriptionFocused("Intent"))
                assertEquals(initialInput, navigation.input())
                assertEquals(0, sends)
                assertEquals(emptyList(), navigation.calls, "Refactor preparation must not Send")
              }
        }
  }

  @Test
  fun decliningContextRefactorDiscardRetainsTheDraftAndItsEvidence() {
    val other =
        SymbolInfo(
            "Other", "function", "func Other()", 3, 3, confidence = "exact", atomicTarget = true)
    FileNavigationUiFixture(
            "draft", "package main\nfunc Run() {}\nfunc Other() {}\n", listOf(other))
        .use { navigation ->
          val presenter = navigation.presenter
          presenter.dispatch(DesktopEvent.SymbolSelected(other))
          val before = presenter.snapshot.value.state
          var pending by mutableStateOf<PendingDraftDiscard.Replace?>(null)
          var prepared = 0
          ComposeVisualFixture(800, 650) {
                val state by presenter.snapshot.collectAsState()
                ContextToolWindow(
                    ContextToolWindowState(
                        symbolInspectorUiState(
                            state.state.selectedFile,
                            state.state.symbols,
                            state.state.selectedSymbol,
                            state.state.analysis,
                            false,
                            InspectorProviderState(false, false),
                            currentEditIdentity(state.state)),
                        ScopedModel(),
                        false,
                        null,
                        null),
                    ContextToolWindowActions(
                        {},
                        {},
                        {},
                        {},
                        editSelected = { symbol ->
                          routeContextRefactor(presenter, symbol, { prepared++ }) { pending = it }
                        }))
                pending?.let { approval ->
                  DraftDiscardDialog(
                      approval.currentDraft,
                      approval.nextLabel,
                      onDiscard = { error("Cancel must not discard the draft") },
                      onCancel = { pending = null })
                }
              }
              .use { fixture ->
                fixture.render()
                fixture.clickText("Refactor")
                fixture.render()
                assertEquals("Other", pending?.request?.target?.symbol)
                assertEquals(0, prepared)
                assertEquals(before.review, presenter.snapshot.value.state.review)
                fixture.pressKey(Key.Escape)
                fixture.render()
                assertNull(pending)
                assertEquals(before.review, presenter.snapshot.value.state.review)
                assertEquals(before.chat, presenter.snapshot.value.state.chat)
                assertEquals(before.selection, presenter.snapshot.value.state.selection)
                navigation.runPending()
                assertEquals(emptyList(), navigation.calls)
              }
        }
  }

  @Test
  fun productionSourceTapInspectsButMultilineDragCopyAndMutationAttemptsRetainWork() {
    val source =
        "package worker\n\nfunc Run() {\n    work()\n}\n\nfunc Other() {\n    more()\n}\n\n// " +
            "longArgument".repeat(60)
    val file = testFile("internal/worker/main.go").copy(content = source)
    val run =
        SymbolInfo(
            "Run",
            "function",
            startLine = 3,
            endLine = 5,
            confidence = "exact",
            atomicTarget = true)
    val other = run.copy(name = "Other", startLine = 7, endLine = 9)
    val project = resultProjectFixture()
    for (scale in listOf(1f, 1.5f)) {
      val controller =
          DesktopWorkflowController(
              DesktopState(
                  projectState = ProjectWorkspaceState(project = project),
                  selection =
                      FileSelectionState(selectedFile = file, symbols = listOf(run, other))))
      val draft =
          validatedDraft()
              .copy(
                  projectId = project.projectId,
                  projectRevision = project.projectRevision,
                  baseFileHash = file.contentHash,
                  targetPath = file.path,
                  targetSymbol = other.name,
                  mode = ChatEditMode.ReplaceSymbol.wireValue,
                  declaration = "func Other() { candidate() }")
      var state by mutableStateOf(controller.dispatch(DesktopEvent.DraftLoaded(draft)))
      var draftInput by mutableStateOf(TextFieldValue(draft.declaration))
      val taps = mutableListOf<SourceLineSelection>()
      ComposeVisualFixture(1000, 650, scale) {
            Row(Modifier.fillMaxSize()) {
              Box(Modifier.weight(1f)) {
                EditorWorkspace(
                    editorChromeUiState(
                        state.selectedFile,
                        state.selectedSymbol,
                        EditorSurface.Source,
                        editorProgressUiState(state),
                        state.review.draft),
                    null,
                    {},
                    {},
                    canvas = {
                      SourceEditorPane(
                          project,
                          state.selectedFile,
                          state.symbols,
                          state.selectedSymbol,
                          state.selection.focusedLine,
                          listOf(
                              UnifiedFinding(
                                  id = "known",
                                  title = "Known finding",
                                  location = FindingLocation(file.path, startLine = 4)))) {
                              selection ->
                            taps += selection
                            state = controller.dispatch(DesktopEvent.SourceLineSelected(selection))
                          }
                    })
              }
              CompactMultilineField(
                  value = draftInput,
                  onValueChange = {
                    draftInput = it
                    state = controller.dispatch(DesktopEvent.DraftEdited(declaration = it.text))
                  },
                  label = "Declaration only",
                  minLines = 5,
                  modifier = Modifier.width(280.dp))
            }
          }
          .use { fixture ->
            fixture.render()
            // Establish that the fixture sends real typing/paste to an editable control first.
            fixture.focusDescribedEditor("Declaration only")
            fixture.typeCharacter(Key.X, 'x')
            assertTrue(draftInput.text != draft.declaration)
            fixture.setClipboardText("PASTED_CANDIDATE")
            fixture.pressKey(Key.Paste)
            fixture.render()
            assertTrue(draftInput.text.contains("PASTED_CANDIDATE"))
            val retainedInput = draftInput
            val retainedReview = state.review
            val retainedChat = state.chat
            fixture.tapSourceText(3, 6)
            assertEquals(listOf(SourceLineSelection(3, run)), taps)
            assertEquals(run, state.selectedSymbol)
            assertEquals(3, state.selection.focusedLine)
            assertEquals(retainedReview, state.review)
            assertEquals(retainedChat, state.chat)
            val inspected = state.selection
            val displayedLines = source.lines().map(::expandedEditorIndentation)
            fun assertMutationIsRejected() {
              assertFalse(fixture.hasTextMutationSemantics("source-viewport"))
              fixture.setClipboardText("MUST_NOT_REPLACE_SOURCE_OR_DRAFT")
              val attempts =
                  listOf<() -> Unit>(
                      { fixture.typeCharacter(Key.X, 'x') },
                      { fixture.pressKey(Key.Backspace) },
                      { fixture.pressKey(Key.Delete) },
                      { fixture.pressKey(Key.Paste) },
                      { fixture.pressKey(Key.Cut) })
              attempts.forEach { attempt ->
                attempt()
                fixture.render()
                assertEquals(file, state.selectedFile)
                assertEquals(inspected, state.selection)
                assertEquals(retainedInput, draftInput)
                assertEquals(retainedReview, state.review)
                assertEquals(retainedChat, state.chat)
                displayedLines.forEach { assertTrue(fixture.hasText(it)) }
                assertEquals(listOf(SourceLineSelection(3, run)), taps)
              }
            }
            assertMutationIsRejected()
            for (scroll in listOf(0f, 40f)) {
              if (scroll > 0f) {
                fixture.horizontalScrollWithin("source-viewport", scroll)
                fixture.awaitHorizontalScrollWithinValue("source-viewport", scroll)
              }
              val offset = if (scroll == 0f) 0 else 8
              val expected =
                  (if (scroll == 0f) "func Run" else "") +
                      "() {\n         work()\n}\n\nfunc Other()"
              for (reverse in listOf(false, true)) {
                fixture.setClipboardText("")
                if (reverse) fixture.dragSourceText(7, 12, 3, offset)
                else fixture.dragSourceText(3, offset, 7, 12)
                fixture.pressKey(Key.Copy)
                fixture.render()
                assertEquals(
                    expected,
                    fixture.clipboardText(),
                    "scale=$scale scroll=$scroll reverse=$reverse")
                assertEquals(listOf(SourceLineSelection(3, run)), taps, "Drag must not inspect")
                assertEquals(inspected, state.selection)
                assertEquals(retainedReview, state.review)
                assertEquals(retainedChat, state.chat)
                assertEquals(retainedInput, draftInput)
              }
              (1..source.lines().size).forEach { line ->
                val gutter = fixture.taggedBounds("source-gutter-$line")
                val code = fixture.taggedBounds("source-code-$line")
                assertEquals(gutter.top, code.top, 1f)
                assertEquals(gutter.height, code.height, 1f)
              }
            }
            assertMutationIsRejected()
          }
    }
  }

  @Test
  fun failedDestinationKeepsRetainedSourceIdentityAndLongPathsReachableAtLargerText() {
    val path = "internal/module-with-a-very-long-name/nested/another-module/failed_destination.go"
    val retained = testFile("cmd/worker/main.go").copy(content = "package retained")
    listOf(360 to 1.5f, 800 to 1f).forEach { (width, scale) ->
      val retries = mutableListOf<String>()
      var read by mutableStateOf<FileReadUiState>(FileReadUiState.Failed(path, "Permission denied"))
      ComposeVisualFixture(width, 650, scale) {
            EditorWorkspace(
                editorChromeUiState(
                    retained, null, EditorSurface.Source, progress(EditorProgress.Inspect), null),
                null,
                {},
                {},
                canvas = {
                  EditorPane(
                      resultProjectFixture(), retained, emptyList(), null, 0, emptyList(), {})
                },
                fileRead = read,
                onOpenFile = retries::add)
          }
          .use { fixture ->
            fixture.render("editor-read-failure-$width-$scale")
            fixture.assertTextFits("Retry opening file")
            assertTrue(fixture.hasDescription("Project-relative path: cmd/worker/main.go"))
            assertTrue(fixture.hasDescription("Failed destination: $path"))
            assertFalse(fixture.hasDescription("Project-relative path: $path"))
            assertTrue(fixture.hasText("package retained"))
            assertTrue(fixture.hasText("The current file remains open."))
            assertEquals(emptyList(), retries, "Rendering must not retry")
            if (width == 360) {
              fixture.horizontalScrollBy("file-read-path", 10000f)
              fixture.render()
              assertTrue(fixture.horizontalScrollValue("file-read-path") > 0f)
            }
            assertTrue(fixture.requestFocus("Retry opening file"))
            fixture.render()
            fixture.pressKey(Key.Enter)
            fixture.render()
            assertEquals(listOf(path), retries)
            read = FileReadUiState.Pending(path)
            fixture.render("editor-read-pending-$width-$scale")
            assertTrue(fixture.hasText("Opening file"))
            assertTrue(fixture.hasDescription("Pending destination: $path"))
            assertFalse(fixture.hasText("Retry opening file"))
            assertTrue(fixture.hasText("package retained"))
            assertEquals(listOf(path), retries)
          }
    }
  }

  @Test
  fun initialFailureBinaryPreviewAndValidEmptySourceRemainDistinct() {
    var file by mutableStateOf<ProjectFileInfo?>(null)
    var read by
        mutableStateOf<FileReadUiState?>(FileReadUiState.Failed("empty.go", "Read unavailable"))
    val retries = mutableListOf<String>()
    ComposeVisualFixture(360, 650, 1.5f) {
          EditorWorkspace(
              editorChromeUiState(
                  file, null, EditorSurface.Source, progress(EditorProgress.Inspect), null),
              null,
              {},
              {},
              canvas = {
                EditorPane(resultProjectFixture(), file, emptyList(), null, 0, emptyList(), {})
              },
              fileRead = read,
              onOpenFile = retries::add)
        }
        .use { fixture ->
          fixture.render("editor-initial-read-failure-360-1.5")
          assertTrue(fixture.hasText("Could not open file"))
          assertTrue(fixture.hasDescription("Project-relative path: No file selected"))
          assertFalse(fixture.hasText("The current file remains open."))
          assertEquals(emptyList(), retries)
          fixture.clickText("Retry opening file")
          assertEquals(listOf("empty.go"), retries)
          file = testFile("binary.go").copy(binary = true)
          read = null
          fixture.render("editor-binary-preview-360-1.5")
          assertTrue(fixture.hasText("Binary file: source preview is unavailable."))
          assertFalse(fixture.hasText("Could not open file"))
          assertFalse(fixture.hasText("Retry opening file"))
          file = testFile("empty.go").copy(content = "")
          fixture.render("editor-valid-empty-source-360-1.5")
          assertTrue(fixture.hasDescription("Project-relative path: empty.go"))
          assertTrue(fixture.hasText("Read-only"))
          assertFalse(fixture.hasText("Binary file: source preview is unavailable."))
          assertFalse(fixture.hasText("Could not open file"))
          assertFalse(fixture.hasText("Retry opening file"))
          assertEquals(listOf("empty.go"), retries)
        }
  }

  @Test
  fun retryUsesFreshComposerAndPresenterAdmissionRatherThanReusingFailedReadApproval() {
    for (work in listOf("composer", "session", "draft")) {
      FileNavigationUiFixture(work).use { navigation ->
        navigation.presenter.dispatch(DesktopEvent.FileLoadFailed("Permission denied", "other.go"))
        when (work) {
          "composer" -> navigation.message = TextFieldValue("New input after failure")
          "session" ->
              navigation.presenter.dispatch(
                  DesktopEvent.ChatLoaded(
                      requireNotNull(navigation.presenter.snapshot.value.state.chat.session)
                          .copy(id = "new-session-after-failure")))
          "draft" ->
              navigation.presenter.dispatch(
                  DesktopEvent.DraftEdited(
                      declaration = "func Run() { println(\"changed after failure\") }"))
        }
        val retained = navigation.presenter.snapshot.value.state
        val inputAtRetry = navigation.input()
        ComposeVisualFixture(800, 650) {
              EditorWorkspace(
                  editorChromeUiState(
                      retained.selectedFile,
                      retained.selectedSymbol,
                      EditorSurface.Source,
                      progress(EditorProgress.Inspect),
                      retained.review.draft),
                  null,
                  {},
                  {},
                  canvas = { Text(requireNotNull(retained.selectedFile).content) },
                  fileRead =
                      fileReadUiState(
                          retained.selection.pendingFilePath,
                          retained.selection.failedFilePath,
                          retained.selection.fileReadError),
                  onOpenFile = navigation::route)
              navigation.DiscardDialog()
            }
            .use { fixture ->
              fixture.render()
              navigation.runPending()
              assertEquals(emptyList(), navigation.calls)
              fixture.clickText("Retry opening file")
              fixture.render()
              assertTrue(navigation.pending != null)
              navigation.runPending()
              assertEquals(
                  emptyList(), navigation.calls, "Retry must wait for current discard admission")
              fixture.pressKey(Key.Escape)
              fixture.render()
              assertNull(navigation.pending)
              assertEquals(retained.selection, navigation.presenter.snapshot.value.state.selection)
              assertEquals(retained.chat, navigation.presenter.snapshot.value.state.chat)
              assertEquals(retained.review, navigation.presenter.snapshot.value.state.review)
              assertEquals(inputAtRetry, navigation.input())
              fixture.clickText("Retry opening file")
              fixture.render()
              val approval = requireNotNull(navigation.pending)
              navigation.message = TextFieldValue("Changed while retry confirmation was open")
              confirmFileNavigationDiscard(navigation.presenter, approval, navigation::input) {
                error("Stale retry must not clear input")
              }
              navigation.runPending()
              assertEquals(emptyList(), navigation.calls)
              assertEquals(retained.selection, navigation.presenter.snapshot.value.state.selection)
              navigation.pending = null
              fixture.render()
              fixture.clickText("Retry opening file")
              fixture.render()
              confirmFileNavigationDiscard(
                  navigation.presenter, requireNotNull(navigation.pending), navigation::input) {
                    navigation.clears++
                  }
              navigation.runPending()
              assertTrue(navigation.calls.any { it.contains("files/info?path=other.go") })
              assertEquals("other.go", navigation.presenter.snapshot.value.state.selectedFile?.path)
              assertEquals(1, navigation.clears)
            }
      }
    }
  }

  @Test
  fun creationAcceptsAGoFileWithoutDeclarationsAndExplainsUnsupportedOrBusyStates() {
    val file = testFile("empty.go").copy(content = "package demo\n")
    assertNull(declarationCreationBlockedReason(file))
    assertEquals(
        "Open a Go file to create a function or type.", declarationCreationBlockedReason(null))
    assertEquals(
        "Function and type creation requires a Go source file.",
        declarationCreationBlockedReason(file.copy(language = "Kotlin")))
    assertEquals(
        "Function and type creation requires a Go source file.",
        declarationCreationBlockedReason(file.copy(binary = true)))
    assertEquals(
        "Wait for the current generation or validation to finish.",
        declarationCreationBlockedReason(file, busy = true))
    assertNull(
        editorChromeUiState(
                file, null, EditorSurface.Source, progress(EditorProgress.Inspect), null)
            .creationBlockedReason)
  }

  @Test
  fun newFunctionIsVisibleAndKeyboardActivationDoesNotSelectAnEditorSurface() {
    listOf(360 to 1.5f, 800 to 1f).forEach { (width, scale) ->
      var creations = 0
      var surfaceSelections = 0
      val file = testFile("internal/empty.go").copy(content = "package demo\n")
      val chrome =
          editorChromeUiState(
              file, null, EditorSurface.Source, progress(EditorProgress.Inspect), null)
      ComposeVisualFixture(width, 650, scale) {
            EditorWorkspace(
                chrome,
                null,
                { surfaceSelections++ },
                { creations++ },
                canvas = { Text(file.content) })
          }
          .use { fixture ->
            fixture.render("editor-new-function-$width-$scale")
            fixture.assertTextFits("New function")
            assertTrue(fixture.hasDescription("New function in internal/empty.go"))
            assertFalse(fixture.isDisabled("New function"))
            assertEquals(0, creations)
            assertTrue(fixture.requestFocus("New function"))
            fixture.render()
            fixture.pressKey(Key.Enter)
            fixture.render()
            assertEquals(1, creations)
            assertEquals(0, surfaceSelections)
            assertEquals("package demo\n", file.content)
          }
    }
  }

  @Test
  fun newFunctionRoutesPackageOnlyPreparationButCancellationKeepsConflictingDraft() {
    FileNavigationUiFixture("composer").use { navigation ->
      val packageOnly =
          requireNotNull(navigation.presenter.snapshot.value.state.selectedFile)
              .copy(content = "package demo\n")
      navigation.presenter.dispatch(DesktopEvent.FileLoaded(packageOnly, emptyList()))
      var prepared: DeclarationCreationKind? = null
      var pending: PendingDraftDiscard.Create? = null
      val state = navigation.presenter.snapshot.value
      assertNull(declarationCreationBlockedReason(state.state.selectedFile))
      routeCreationRequest(
          state, DeclarationCreationKind.Function, { prepared = it }, { pending = it })
      assertEquals(DeclarationCreationKind.Function, prepared)
      assertNull(pending)
      assertEquals("package demo\n", packageOnly.content)
      assertEquals(emptyList(), navigation.calls, "Preparation must not send or write")
      for (blockedFile in
          listOf(null, packageOnly.copy(language = "Markdown"), packageOnly.copy(binary = true))) {
        prepared = null
        routeCreationRequest(
            state.copy(
                state =
                    state.state.copy(
                        selection = state.state.selection.copy(selectedFile = blockedFile))),
            DeclarationCreationKind.Function,
            { prepared = it },
            { pending = it })
        assertNull(prepared)
        assertNull(pending)
      }
      assertEquals(emptyList(), navigation.calls)
    }
    FileNavigationUiFixture("draft").use { navigation ->
      var prepared = 0
      var pending: PendingDraftDiscard.Create? by mutableStateOf(null)
      val before = navigation.presenter.snapshot.value.state
      ComposeVisualFixture(800, 650) {
            EditorWorkspace(
                editorChromeUiState(
                    before.selectedFile,
                    before.selectedSymbol,
                    EditorSurface.Source,
                    progress(EditorProgress.Inspect),
                    before.review.draft),
                null,
                {},
                {
                  routeCreationRequest(
                      navigation.presenter.snapshot.value,
                      DeclarationCreationKind.Function,
                      { prepared++ },
                      { pending = it })
                },
                canvas = { Text(requireNotNull(before.selectedFile).content) })
            pending?.let { approval ->
              DraftDiscardDialog(
                  approval.currentDraft,
                  approval.nextLabel,
                  onDiscard = { error("Cancellation must not discard") },
                  onCancel = { pending = null })
            }
          }
          .use { fixture ->
            fixture.render()
            fixture.clickText("New function")
            fixture.render()
            assertTrue(pending != null)
            assertEquals(0, prepared)
            assertTrue(navigation.calls.isEmpty())
            fixture.pressKey(Key.Escape)
            fixture.render()
            assertNull(pending)
            assertEquals(before.review, navigation.presenter.snapshot.value.state.review)
            assertEquals(before.chat, navigation.presenter.snapshot.value.state.chat)
            assertEquals(0, prepared)
            assertEquals(emptyList(), navigation.calls)
          }
    }
  }

  @Test
  fun creationDiscardApprovalCannotClearChangedComposerOrDraft() {
    FileNavigationUiFixture("draft").use { navigation ->
      val initial = navigation.presenter.snapshot.value
      val message = TextFieldValue("prepare a function")
      var prepared = 0
      var pending: PendingDraftDiscard.Create? = null
      routeCreationRequest(
          initial, DeclarationCreationKind.Function, { prepared++ }, { pending = it }, message)
      val approval = requireNotNull(pending)
      val before = initial.state.review
      confirmCreationDiscard(
          approval,
          navigation.presenter,
          { TextFieldValue("changed") to TextFieldValue() },
          { Triple(ChatEditMode.CreateSymbol, DeclarationCreationKind.Function, "") },
          { prepared++ },
          { error("Not a kind change") })
      assertEquals(before, navigation.presenter.snapshot.value.state.review)
      assertEquals(0, prepared)

      navigation.presenter.dispatch(
          DesktopEvent.DraftLoaded(requireNotNull(before.draft).copy(revision = 2)))
      val changedDraft = navigation.presenter.snapshot.value.state.review
      confirmCreationDiscard(
          approval,
          navigation.presenter,
          { message to TextFieldValue() },
          { Triple(ChatEditMode.CreateSymbol, DeclarationCreationKind.Function, "") },
          { prepared++ },
          { error("Not a kind change") })
      assertEquals(changedDraft, navigation.presenter.snapshot.value.state.review)
      assertEquals(0, prepared)
      assertEquals(emptyList(), navigation.calls)
    }
    FileNavigationUiFixture("draft").use { navigation ->
      val initial = navigation.presenter.snapshot.value
      var pending: PendingDraftDiscard.Create? = null
      var prepared = 0
      routeCreationRequest(
          initial, DeclarationCreationKind.Function, { prepared++ }, { pending = it })
      confirmCreationDiscard(
          requireNotNull(pending),
          navigation.presenter,
          { TextFieldValue() to TextFieldValue() },
          { Triple(ChatEditMode.CreateSymbol, DeclarationCreationKind.Function, "") },
          { prepared++ },
          { error("Not a kind change") })
      assertEquals(1, prepared)
      assertNull(navigation.presenter.snapshot.value.state.review.draft)
      assertEquals(emptyList(), navigation.calls)
    }
  }

  @Test
  fun kindChangeKeepDraftPreservesInputsAndEvidence() {
    FileNavigationUiFixture("draft").use { navigation ->
      val before = navigation.presenter.snapshot.value.state
      val message = TextFieldValue("Return value", TextRange(3, 6))
      val constraints = TextFieldValue("No imports", TextRange(2))
      var kind = DeclarationCreationKind.Function
      var pending: PendingDraftDiscard.Create? = null
      routeCreationKindChange(
          navigation.presenter.snapshot.value,
          ChatEditMode.CreateSymbol,
          kind,
          DeclarationCreationKind.Type,
          "Build",
          message,
          constraints,
          { kind = it },
          { pending = it })
      val approval = requireNotNull(pending)
      assertEquals(PendingDraftDiscard.CreationIntent.ChangeKind, approval.intent)
      assertEquals("Build", approval.name)
      assertEquals(message, approval.message)
      assertEquals(constraints, approval.constraints)
      pending = null // Keep draft / Escape does not run the continuation.
      assertNull(pending)
      assertEquals(DeclarationCreationKind.Function, kind)
      assertEquals(before.review, navigation.presenter.snapshot.value.state.review)
      assertEquals(before.chat, navigation.presenter.snapshot.value.state.chat)
      assertEquals(emptyList(), navigation.calls)
    }
  }

  @Test
  fun confirmedKindChangeDiscardsCandidateButPreservesComposer() {
    FileNavigationUiFixture("draft").use { navigation ->
      val message = TextFieldValue("Return value", TextRange(3, 6))
      val constraints = TextFieldValue("No imports", TextRange(2))
      var kind = DeclarationCreationKind.Function
      val name = "Build"
      var pending: PendingDraftDiscard.Create? = null
      routeCreationKindChange(
          navigation.presenter.snapshot.value,
          ChatEditMode.CreateSymbol,
          kind,
          DeclarationCreationKind.Type,
          name,
          message,
          constraints,
          { kind = it },
          { pending = it })
      confirmCreationDiscard(
          requireNotNull(pending),
          navigation.presenter,
          { message to constraints },
          { Triple(ChatEditMode.CreateSymbol, kind, name) },
          { error("Must not reset inputs") },
          { kind = it })
      assertEquals(DeclarationCreationKind.Type, kind)
      assertEquals("Build", name)
      assertEquals(TextFieldValue("Return value", TextRange(3, 6)), message)
      assertEquals(TextFieldValue("No imports", TextRange(2)), constraints)
      assertNull(navigation.presenter.snapshot.value.state.review.draft)
      assertNull(navigation.presenter.snapshot.value.state.chat.session)
      assertEquals(emptyList(), navigation.calls)
    }
  }

  @Test
  fun staleKindChangeApprovalCannotDiscardNewerWork() {
    FileNavigationUiFixture("draft").use { navigation ->
      val message = TextFieldValue("Return value", TextRange(3, 6))
      val constraints = TextFieldValue("No imports", TextRange(2))
      var kind = DeclarationCreationKind.Function
      var name = "Build"
      var pending: PendingDraftDiscard.Create? = null
      routeCreationKindChange(
          navigation.presenter.snapshot.value,
          ChatEditMode.CreateSymbol,
          kind,
          DeclarationCreationKind.Type,
          name,
          message,
          constraints,
          { kind = it },
          { pending = it })
      val approval = requireNotNull(pending)
      fun attempt() =
          confirmCreationDiscard(
              approval,
              navigation.presenter,
              { message to constraints },
              { Triple(ChatEditMode.CreateSymbol, kind, name) },
              { error("Stale fresh creation") },
              { error("Stale kind change") })
      name = "Changed"
      attempt()
      name = "Build"
      val original = requireNotNull(navigation.presenter.snapshot.value.state.review.draft)
      navigation.presenter.dispatch(
          DesktopEvent.DraftLoaded(original.copy(revision = original.revision + 1)))
      val newer = navigation.presenter.snapshot.value.state
      attempt()
      assertEquals(newer.review, navigation.presenter.snapshot.value.state.review)
      assertEquals(newer.chat, navigation.presenter.snapshot.value.state.chat)
      assertEquals(DeclarationCreationKind.Function, kind)
      assertEquals(emptyList(), navigation.calls)
    }
  }

  @Test
  fun fileCreationHasOneVisibleActionInDockedEditorAndContextAndRemainsAvailableInCompactContext() {
    val file = testFile("empty.go").copy(content = "package demo\n")
    val inspector =
        symbolInspectorUiState(
            file, emptyList(), null, null, false, InspectorProviderState(false, false), null)
    var creations = 0
    var analyses = 0
    val state = ContextToolWindowState(inspector, ScopedModel(), false, null, null)
    val actions =
        ContextToolWindowActions(
            {}, { analyses++ }, { analyses++ }, {}, {}, createDeclaration = { creations++ })
    ComposeVisualFixture(800, 320, 1.5f) {
          Row {
            EditorWorkspace(
                editorChromeUiState(
                    file, null, EditorSurface.Source, progress(EditorProgress.Inspect), null),
                null,
                {},
                { creations++ },
                canvas = { Text(file.content) },
                modifier = Modifier.weight(1f))
            CompositionLocalProvider(LocalContextCreationActionVisible provides false) {
              ContextToolWindow(state, actions, Modifier.weight(1f))
            }
          }
        }
        .use { fixture ->
          fixture.render("editor-context-one-new-function-800-1.5")
          assertEquals(1, fixture.textCount("New function"))
          fixture.clickText("New function")
          assertEquals(1, creations)
          assertEquals(0, analyses)
        }

    ComposeVisualFixture(320, 200, 1.5f) {
          CompositionLocalProvider(LocalContextCreationActionVisible provides true) {
            ContextToolWindow(state, actions, Modifier.fillMaxSize())
          }
        }
        .use { fixture ->
          fixture.render("context-new-function-compact-320-1.5")
          assertEquals(1, fixture.textCount("New function"))
          fixture.clickText("New function")
          assertEquals(2, creations)
          assertEquals(0, analyses)
        }
  }

  @Test
  fun sourceStaysActiveWhenAValidatedDraftMakesReviewAvailable() {
    val chrome =
        editorChromeUiState(
            file = testFile("internal/runner/run.go"),
            selectedSymbol = symbol(),
            requestedSurface = EditorSurface.Source,
            progress = progress(EditorProgress.Review),
            draft = validatedDraft(),
        )

    assertEquals(EditorSurface.Source, chrome.activeSurface)
    assertTrue(chrome.reviewAvailable)
    assertEquals("VALIDATED DRAFT", chrome.stageLabel)
    assertTrue(chrome.accessibleDescription.contains("Read-only source surface"))
  }

  @Test
  fun reviewIsOnlyShownAfterAnExplicitSelectionAndCurrentValidation() {
    val review =
        editorChromeUiState(
            file = testFile("internal/runner/run.go"),
            selectedSymbol = symbol(),
            requestedSurface = EditorSurface.Review,
            progress = progress(EditorProgress.Review),
            draft = validatedDraft(),
        )
    val stale =
        editorChromeUiState(
            file = testFile("internal/runner/run.go"),
            selectedSymbol = symbol(),
            requestedSurface = EditorSurface.Review,
            progress = progress(EditorProgress.Edit),
            draft = validatedDraft(),
        )

    assertEquals(EditorSurface.Review, review.activeSurface)
    assertEquals("REVIEW CANDIDATE", review.stageLabel)
    assertEquals(EditorSurface.Source, stale.activeSurface)
    assertTrue(!stale.reviewAvailable)
  }

  @Test
  fun breadcrumbsCollapseLongPathsButAccessibilityRetainsTheFullPath() {
    val path = "src/module/feature/runner/run.go"
    val chrome =
        editorChromeUiState(
            file = testFile(path),
            selectedSymbol = symbol(),
            requestedSurface = EditorSurface.Source,
            progress = progress(EditorProgress.Inspect),
            draft = null,
        )

    assertEquals(
        listOf(
            EditorBreadcrumbSegment("src", EditorBreadcrumbKind.Folder),
            EditorBreadcrumbSegment("…", EditorBreadcrumbKind.Collapsed),
            EditorBreadcrumbSegment("run.go", EditorBreadcrumbKind.File),
            EditorBreadcrumbSegment("Run", EditorBreadcrumbKind.Symbol),
        ),
        chrome.breadcrumbSegments,
    )
    assertTrue(chrome.accessibleDescription.contains(path))
    assertTrue(chrome.accessibleDescription.contains("Selected declaration Run"))
  }

  @Test
  fun duplicateBasenamesKeepTheirOwnProjectRelativeIdentity() {
    val first =
        editorChromeUiState(
            file = testFile("cmd/worker/main.go"),
            selectedSymbol = null,
            requestedSurface = EditorSurface.Source,
            progress = progress(EditorProgress.Inspect),
            draft = null,
        )
    val second =
        editorChromeUiState(
            file = testFile("cmd/server/main.go"),
            selectedSymbol = null,
            requestedSurface = EditorSurface.Source,
            progress = progress(EditorProgress.Inspect),
            draft = null,
        )

    assertEquals("main.go", first.title)
    assertEquals("main.go", second.title)
    assertEquals("cmd/worker/main.go", first.path)
    assertEquals("cmd/server/main.go", second.path)
    assertTrue(first.accessibleDescription.contains(first.path))
    assertTrue(second.accessibleDescription.contains(second.path))
  }

  @Test
  fun rootAndNestedFilesKeepTheirActualSegmentsWhenNoSymbolIsSelected() {
    assertEquals(
        listOf(EditorBreadcrumbSegment("main.go", EditorBreadcrumbKind.File)),
        editorBreadcrumbSegments("main.go"),
    )
    assertEquals(
        listOf(
            EditorBreadcrumbSegment("internal", EditorBreadcrumbKind.Folder),
            EditorBreadcrumbSegment("server", EditorBreadcrumbKind.Folder),
            EditorBreadcrumbSegment("main.go", EditorBreadcrumbKind.File),
        ),
        editorBreadcrumbSegments("internal/server/main.go"),
    )
  }

  @Test
  fun emptyFileStateIsNeutralAndReadOnly() {
    val chrome =
        editorChromeUiState(
            file = null,
            selectedSymbol = null,
            requestedSurface = EditorSurface.Review,
            progress = progress(EditorProgress.Inspect),
            draft = null,
        )

    assertEquals("No file open", chrome.title)
    assertEquals("No file selected", chrome.path)
    assertEquals(EditorSurface.Source, chrome.activeSurface)
    assertEquals(
        listOf(EditorBreadcrumbSegment("No file selected", EditorBreadcrumbKind.Placeholder)),
        chrome.breadcrumbSegments,
    )
    assertTrue(chrome.accessibleDescription.contains("Read-only source surface"))
  }

  @Test
  fun candidateTabAndEditDraftAreLocalNavigationAndReflectLiveEvidence() {
    var review by mutableStateOf(editorComparisonReviewFixture())
    var surface by mutableStateOf(EditorSurface.Source)
    var edits = 0
    var creations = 0
    ComposeVisualFixture(800, 600, 1.5f) {
          EditorWorkspace(
              editorChromeUiState(
                  review.selected,
                  review.selectedSymbol,
                  surface,
                  EditorProgressUiState(EditorProgress.Review, ""),
                  review.draft),
              review,
              { surface = it },
              { creations++ },
              canvas = {
                if (surface == EditorSurface.Review) ReviewDiffCanvas(review.draft)
                else Text("Source fixture")
              },
              onEditDraft = { edits++ })
        }
        .use { fixture ->
          fixture.render("editor-progression-ready-800-1.5")
          fixture.assertTextFits("Edit draft")
          fixture.awaitDescription("Focused checks: 1 checks (1 required) are current.", "Passed")
          fixture.clickText("Candidate diff")
          fixture.render()
          assertEquals(EditorSurface.Review, surface)
          assertEquals(0, edits)
          assertEquals(0, creations)
          assertFalse(fixture.hasEditableText())
          assertTrue(fixture.requestFocus("Edit draft"))
          fixture.render()
          fixture.pressKey(Key.Enter)
          assertEquals(1, edits)
          review = review.copy(checks = review.checks!!.copy(draftHash = "previous-draft"))
          fixture.render("editor-progression-stale-800-1.5")
          fixture.assertTextFits("Checks · Stale")
          fixture.assertTextFits("Review · Stale")
          review = review.copy(checks = null, checksRunning = true)
          fixture.render("editor-progression-running-800-1.5")
          fixture.assertTextFits("Checks · Running")
          assertEquals(1, edits)
          assertEquals(0, creations)
        }
  }

  @Test
  fun narrowChromeKeepsTabsActionsProgressAndCompletePathLocallyReachable() {
    val path = "src/module-with-a-very-long-name/feature/runner/run.go"
    listOf(1.25f, 1.5f).forEach { scale ->
      val review = editorComparisonReviewFixture()
      var surface by mutableStateOf(EditorSurface.Source)
      var selections = 0
      var creations = 0
      var edits = 0
      ComposeVisualFixture(220, 420, scale) {
            EditorWorkspace(
                editorChromeUiState(
                    testFile(path),
                    symbol(),
                    surface,
                    progress(EditorProgress.Review),
                    validatedDraft()),
                review,
                {
                  surface = it
                  selections++
                },
                { creations++ },
                canvas = {
                  Box(Modifier.fillMaxSize().testTag("editor-test-canvas")) {
                    Text("Source viewport")
                  }
                },
                onEditDraft = { edits++ })
          }
          .use { fixture ->
            fixture.render("editor-narrow-chrome-$scale")
            val viewport = fixture.taggedBounds("editor-test-canvas")
            assertTrue(viewport.height > 80f, "Progression must not consume the source viewport")
            assertTrue(fixture.hasDescription("Project-relative path: $path"))
            fixture.horizontalScrollBy("editor-path", 10000f)
            fixture.render()
            assertTrue(fixture.horizontalScrollValue("editor-path") > 0f)
            assertTrue(fixture.hasText(path))
            fixture.horizontalScrollBy("editor-progression", 10000f)
            fixture.render()
            assertTrue(fixture.horizontalScrollValue("editor-progression") > 0f)
            fixture.horizontalScrollBy("editor-tabs", 10000f)
            fixture.render()
            assertTrue(fixture.requestFocus("Candidate diff"))
            fixture.render()
            val tab = fixture.firstVisibleTextBounds("Candidate diff")
            assertTrue(tab.right <= 220f && tab.left >= 0f, "Candidate tab must fit: $tab")
            fixture.pressKey(Key.DirectionRight)
            fixture.pressKey(Key.Enter)
            fixture.render()
            assertEquals(EditorSurface.Review, surface)
            assertEquals(1, selections)
            fixture.horizontalScrollBy("editor-draft-actions", 10000f)
            fixture.render()
            assertTrue(fixture.requestFocus("Edit draft"))
            fixture.render()
            val edit = fixture.firstVisibleTextBounds("Edit draft")
            assertTrue(edit.right <= 220f && edit.left >= 0f, "Edit action must fit: $edit")
            fixture.pressKey(Key.Enter)
            fixture.render()
            assertEquals(1, edits)
            fixture.horizontalScrollBy("editor-draft-actions", -10000f)
            fixture.render()
            assertTrue(fixture.requestFocus("New function"))
            fixture.render()
            fixture.pressKey(Key.Enter)
            fixture.render()
            assertEquals(1, creations)
            assertEquals(1, selections)
          }
    }
  }

  @Test
  fun inspectionKeepsTheRetainedDraftExplicitWithoutRetargetingIt() {
    val path = "internal/module-with-a-very-long-name/nested/worker/main.go"
    val file =
        testFile(path)
            .copy(
                content =
                    "package worker\n\nfunc Run() {}\n\nfunc Other() {}\n\n// outside declarations")
    val run =
        SymbolInfo(
            "Run",
            "function",
            startLine = 3,
            endLine = 3,
            confidence = "exact",
            atomicTarget = true)
    val other = run.copy(name = "Other", startLine = 5, endLine = 5)
    val approximate = run.copy(name = "ApproximateRun", confidence = "approximate")
    val project = resultProjectFixture()
    val controller =
        DesktopWorkflowController(
            DesktopState(
                projectState = ProjectWorkspaceState(project = project),
                selection =
                    FileSelectionState(
                        selectedFile = file, symbols = listOf(run, other, approximate))))
    val draft =
        validatedDraft()
            .copy(
                projectId = project.projectId,
                projectRevision = project.projectRevision,
                baseFileHash = file.contentHash,
                targetPath = path,
                targetSymbol = other.name,
                mode = ChatEditMode.ReplaceSymbol.wireValue,
                declaration = "func Other() { work() }")
    controller.dispatch(DesktopEvent.DraftLoaded(draft))
    var state by mutableStateOf(controller.dispatch(DesktopEvent.SymbolSelected(run)))
    val retainedReview = state.review
    val retainedChat = state.chat
    ComposeVisualFixture(360, 650, 1.5f) {
          EditorWorkspace(
              editorChromeUiState(
                  state.selectedFile,
                  state.selectedSymbol,
                  EditorSurface.Source,
                  editorProgressUiState(state),
                  state.review.draft),
              null,
              {},
              {},
              canvas = {
                EditorPane(
                    project,
                    file,
                    state.symbols,
                    state.selectedSymbol,
                    state.selection.focusedLine,
                    emptyList(),
                    {})
              })
        }
        .use { fixture ->
          fixture.render("editor-inspection-retained-draft-360-1.5")
          val binding = "Retained draft: $path · Other (not the inspected declaration)"
          assertTrue(fixture.hasText("Inspecting declaration: Run · Lines 3–3"))
          assertTrue(fixture.hasText(binding))
          assertTrue(fixture.hasText("Read-only"))
          assertTrue(fixture.hasDescription("Project-relative path: $path"))
          fixture.horizontalScrollBy("editor-path", 10000f)
          fixture.horizontalScrollBy("editor-retained-draft", 10000f)
          fixture.render()
          assertTrue(fixture.horizontalScrollValue("editor-path") > 0f)
          assertTrue(fixture.horizontalScrollValue("editor-retained-draft") > 0f)
          assertTrue(fixture.hasText(path))
          state = controller.dispatch(DesktopEvent.SourceLineSelected(SourceLineSelection(7, null)))
          fixture.render("editor-inspection-file-retained-draft-360-1.5")
          assertTrue(fixture.hasText("Inspecting file"))
          assertTrue(fixture.hasText(binding))
          assertNull(state.selectedSymbol)
          assertEquals(retainedReview, state.review)
          assertEquals(retainedChat, state.chat)
          state = controller.dispatch(DesktopEvent.SymbolSelected(approximate))
          fixture.render("editor-inspection-approximate-retained-draft-360-1.5")
          assertTrue(
              fixture.hasText(
                  "Inspecting declaration: ApproximateRun · Lines 3–3 · Approximate · read-only"))
          assertFalse(symbolEditEligibility(file, state.symbols, state.selectedSymbol).eligible)
          fixture.horizontalScrollBy("editor-inspection", 10000f)
          fixture.render()
          assertTrue(fixture.horizontalScrollValue("editor-inspection") > 0f)
          assertEquals(retainedReview, state.review)
          assertEquals(retainedChat, state.chat)
          state = controller.dispatch(DesktopEvent.SymbolSelected(other))
          fixture.render()
          assertFalse(
              fixture.hasText(binding),
              "Matching inspection must not claim a different draft target")
          assertEquals(retainedReview, state.review)
        }
  }

  @Test
  fun emptyAndBinaryPreviewsCannotHighlightOrOfferAnIndexedDeclarationOnSyntheticRows() {
    val symbol =
        SymbolInfo(
            "Run",
            "function",
            startLine = 1,
            endLine = 999,
            confidence = "exact",
            atomicTarget = true)
    val empty = testFile("empty.go")
    for (file in listOf(empty, empty.copy(path = "binary.go", binary = true))) {
      ComposeVisualFixture(500, 300) {
            EditorWorkspace(
                editorChromeUiState(
                    file, symbol, EditorSurface.Source, progress(EditorProgress.Inspect), null),
                null,
                {},
                {},
                canvas = {
                  SourceEditorPane(
                      resultProjectFixture(), file, listOf(symbol), symbol, 1, emptyList(), {})
                })
          }
          .use { fixture ->
            fixture.render()
            assertTrue(fixture.hasText("Inspecting declaration: Run · Line range unavailable"))
            assertTrue(fixture.hasDescription("Line 1"))
            assertFalse(
                fixture.hasDescription(
                    "Line 1, focused location in selected declaration, selectable declaration Run"))
            assertFalse(fixture.hasDescription("Selected declaration Run marker at line 1"))
            assertEquals(0, fixture.tagCount("source-code-2"))
            assertFalse(fixture.hasEditableText(withinTag = "source-viewport"))
          }
    }
  }

  @Test
  fun duplicateBasenamesRenderDistinctCompletePathsWithoutHover() {
    var file by mutableStateOf(testFile("cmd/worker/main.go"))
    ComposeVisualFixture(480, 300) {
          EditorWorkspace(
              editorChromeUiState(
                  file, null, EditorSurface.Source, progress(EditorProgress.Inspect), null),
              null,
              {},
              {},
              canvas = {})
        }
        .use { fixture ->
          fixture.render()
          listOf("cmd", "worker", "main.go", "Read-only").forEach(fixture::assertTextFits)
          assertTrue(fixture.hasDescription("Project-relative path: cmd/worker/main.go"))
          file = testFile("cmd/server/main.go")
          fixture.render()
          listOf("cmd", "server", "main.go", "Read-only").forEach(fixture::assertTextFits)
          assertFalse(fixture.hasText("worker"))
          assertTrue(fixture.hasDescription("Project-relative path: cmd/server/main.go"))
        }
  }

  private fun progress(value: EditorProgress) = EditorProgressUiState(value, "")

  private fun symbol() = SymbolInfo("Run", "function", confidence = "exact", atomicTarget = true)

  private fun validatedDraft() =
      DeclarationDraft(
          id = "draft",
          targetPath = "internal/runner/run.go",
          revision = 3,
          validation =
              DeclarationValidation(
                  applicable = true,
                  scopeMode = "replace_symbol",
                  diff = UnifiedDiff("internal/runner/run.go", "internal/runner/run.go")),
      )

  private fun testFile(path: String) =
      ProjectFileInfo(
          path = path,
          contentHash = "hash",
          name = path.substringAfterLast('/'),
          language = "Go",
          sizeBytes = 0,
          lineCount = 0,
          modifiedAt = "",
          binary = false,
      )
}
