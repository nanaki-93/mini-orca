package io.miniorca.desktop

import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.input.key.Key
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertIs
import kotlin.test.assertTrue

class AssistantToolWindowTest {

  @Test
  fun findingDiscardRoutingProtectsDraftAndComposerButNotSameSourceInspection() {
    val finding = UnifiedFinding(title = "Lost update")
    val project = SwitchProjectIdentity("project", "revision", "/tmp/project")
    val draft = SwitchDraftIdentity(null, null, null)
    val source =
        DesktopWorkflowPresenter.FindingIntent(
            finding, false, project, draft, null, EditorNavigationTarget("other.go"))
    val prepare = source.copy(prepare = true)
    assertFalse(findingRequiresDiscard(source, "", ""))
    assertTrue(findingRequiresDiscard(source, "typed request", "keep constraints"))
    assertTrue(findingRequiresDiscard(prepare, "typed request", ""))
    val withDraft = source.copy(draft = draft.copy(editor = editableDraftForFindingTest()))
    assertTrue(findingRequiresDiscard(withDraft, "", ""))
    assertTrue(findingRequiresDiscard(withDraft.copy(prepare = true), "", ""))
    assertFalse(
        findingRequiresDiscard(
            withDraft.copy(selectedFile = creationFile().copy(path = "other.go")), "", ""))
  }

  @Test
  fun findingDiscardDialogEscapeAndKeepLeaveApprovalUnused() {
    var dismissed = 0
    var confirmed = 0
    ComposeVisualFixture(480, 360) {
          DraftDiscardDialog(
              CurrentEditIdentity(ChatEditMode.ReplaceSymbol, "main.go", "Run", true),
              "open other.go",
              { confirmed++ },
              { dismissed++ })
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText("Discard current draft?"))
          assertTrue(fixture.pressKey(Key.Escape))
          assertEquals(1, dismissed)
          assertEquals(0, confirmed)
          fixture.clickText("Keep draft")
          assertEquals(2, dismissed)
          assertEquals(0, confirmed)
        }
  }

  private fun editableDraftForFindingTest(): EditableDraftState =
      editableDraft(
          DeclarationDraft(
              id = "draft",
              projectId = "project",
              projectRevision = "revision",
              baseFileHash = "hash",
              targetPath = "main.go",
              mode = "replace_symbol",
              targetSymbol = "Run",
              declaration = "func Run() {}",
              revision = 1,
              hash = "draft-hash"))

  @Test
  fun composerShowsExactScopeAndKeepsActionsOutsideLongHistoryAndConstraints() {
    val symbol = SymbolInfo("Run", "function", "func Run()", 1, 3, "exact", true)
    val target = ChatTarget(ChatEditMode.ReplaceSymbol, "Run")
    val path = "internal/project/nested/run.go"
    val file = creationFile().copy(path = path)
    val scope = ChatRequestScope("project", "revision", path, "hash", target, declaration = symbol)
    var sends = 0
    var inspections = 0
    var prepared: FunctionChangePreset? = null
    var confirmations = 0
    val state =
        historyState(
                target,
                null,
                (1..30).map { index ->
                  ChatRequestAttempt(
                      index.toLong(),
                      scope,
                      ScopedModel(),
                      false,
                      "previous $index",
                      0,
                      ChatRequestOutcome.Failed("retry $index"))
                })
            .copy(
                selected = file,
                mode = ChatEditMode.ReplaceSymbol,
                selectedSymbol = symbol,
                targetValidation = ChatTargetValidation(target),
                message = "Preserve the public behavior.",
                messageInput =
                    androidx.compose.ui.text.input.TextFieldValue("Preserve the public behavior."),
                functionModel =
                    ScopedModel(
                        profile = "function-profile",
                        model = "edit-model",
                        providerOrigin = "http://localhost:11434"))
    ComposeVisualFixture(420, 650) {
          AssistantToolWindow(
              state,
              AssistantConversationActions(
                  {},
                  {},
                  { confirmations++ },
                  { inspections++ },
                  { sends++ },
                  {},
                  preparePreset = { prepared = it }),
              DraftEditorActions({}, {}, {}),
              Modifier.fillMaxSize())
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText(path))
          assertTrue(fixture.hasText("Replace selected declaration · Run"))
          assertTrue(
              fixture.hasText(
                  "Replace one declaration; source changes only after Review and Apply"))
          assertTrue(fixture.hasText("Intent"))
          assertTrue(
              fixture.hasText(
                  "Function edits: function-profile · edit-model · local provider · project context stays on this machine"))
          assertTrue(fixture.hasText("Function provider origin: http://localhost:11434"))
          assertTrue(
              fixture.hasText("File-scoped preview only · no Function request or confirmation."))
          assertEquals(30, assistantHistoryEntries(state).size)
          assertTrue(fixture.hasText("previous 1"))
          fixture.clickText("Advanced constraints")
          fixture.render()
          assertTrue(fixture.hasText("Constraints"))
          fixture.clickText("Refactor")
          fixture.clickText("Inspect context")
          assertEquals(FunctionChangePreset.Refactor, prepared)
          assertEquals(1, inspections)
          assertEquals(0, confirmations)
          assertEquals(0, sends)
          assertFalse(fixture.isDisabled("Send message"))
          fixture.clickText("Send message")
          assertEquals(1, sends)
        }
  }

  @Test
  fun shortComposerScrollReachesConsentStatusAndSendOrCancelIndependentlyOfHistory() {
    val target = ChatTarget(ChatEditMode.ReplaceSymbol, "Run")
    val symbol = SymbolInfo("Run", "function", "func Run()", 1, 3, "exact", true)
    val file = creationFile().copy(path = "internal/" + "nested/".repeat(12) + "run.go")
    val scope =
        ChatRequestScope("project", "revision", file.path, "hash", target, declaration = symbol)
    val attempts =
        (1..30).map { index ->
          ChatRequestAttempt(
              index.toLong(),
              scope,
              ScopedModel(),
              false,
              "previous $index",
              0,
              ChatRequestOutcome.Failed("retry $index"))
        }
    val base =
        historyState(target, null, attempts)
            .copy(
                selected = file,
                mode = ChatEditMode.ReplaceSymbol,
                selectedSymbol = symbol,
                targetValidation = ChatTargetValidation(target),
                message = "Preserve the behavior.",
                functionModel =
                    ScopedModel(
                        profile = "remote-profile",
                        model = "edit-model",
                        providerOrigin = "https://provider.example.invalid/" + "long/".repeat(50),
                        remoteProvider = true))
    listOf(false, true).forEach { running ->
      var activated = 0
      ComposeVisualFixture(360, 360, 1.5f) {
            AssistantToolWindow(
                base.copy(sending = running, remoteConfirmed = true),
                AssistantConversationActions({}, {}, {}, {}, { activated++ }, { activated++ }),
                DraftEditorActions({}, {}, {}),
                Modifier.fillMaxSize())
          }
          .use { fixture ->
            fixture.render()
            fixture.clickText("Advanced constraints")
            fixture.render()
            assertTrue(fixture.hasText("Constraints"))
            assertTrue(fixture.scrollMaximum("assistant-composer-scroll", false) > 0f)
            fixture.scrollBy(100_000f, "assistant-composer-scroll")
            fixture.render()
            val action = fixture.taggedBounds("assistant-request-action")
            val composer = fixture.taggedBounds("assistant-composer-scroll")
            assertTrue(
                action.top >= composer.top && action.bottom <= composer.bottom,
                "$action outside $composer")
            assertTrue(
                fixture.hasText(
                    "Function provider origin: ${sanitizedOutputText(base.functionModel.providerOrigin, 256)}"))
            assertTrue(
                fixture
                    .hasText("Request running · Cancel keeps the existing draft and conversation.")
                    .equals(running))
            fixture.clickText(if (running) "Cancel request" else "Send message")
            assertEquals(1, activated)
          }
    }
  }

  @Test
  fun incompleteFunctionMetadataDoesNotAssertLocalityOrInventProfileAndModel() {
    val model =
        ScopedModel(profile = "known-profile", providerOrigin = "https://provider.example.invalid")
    assertEquals(
        "Function edits: known-profile · model unavailable · provider locality unavailable",
        functionDestinationLabel(model))
    assertFalse(functionDestinationLabel(model).contains("local provider"))
    assertTrue(functionDestinationLabel(ScopedModel()).contains("profile unavailable"))
    val target = ChatTarget(ChatEditMode.ReplaceSymbol, "Run")
    ComposeVisualFixture(420, 650) {
          AssistantToolWindow(
              historyState(target, null, emptyList()).copy(functionModel = model),
              AssistantConversationActions({}, {}, {}, {}, {}, {}),
              DraftEditorActions({}, {}, {}),
              Modifier.fillMaxSize())
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText(functionDestinationLabel(model)))
          assertFalse(fixture.hasText("local provider"))
          assertTrue(fixture.hasText("Function provider origin: https://provider.example.invalid"))
        }
    val remote = model.copy(remoteProvider = true)
    var confirmations = 0
    ComposeVisualFixture(420, 650) {
          AssistantToolWindow(
              historyState(target, null, emptyList()).copy(functionModel = remote),
              AssistantConversationActions({}, {}, { confirmations++ }, {}, {}, {}),
              DraftEditorActions({}, {}, {}),
              Modifier.fillMaxSize())
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText("Confirm remote destination"))
          assertTrue(fixture.hasText("Confirm the Function remote destination before sending."))
          fixture.clickText("Confirm remote destination")
          assertEquals(1, confirmations)
        }
  }

  @Test
  fun composerExplainsInvalidAndNonPresetTargetsAndRemoteConsent() {
    val symbol = SymbolInfo("Config", "type", "type Config struct{}", 1, 1, "exact", true)
    val target = ChatTarget(ChatEditMode.ReplaceSymbol, "Config")
    val base =
        historyState(target, null, emptyList())
            .copy(
                mode = ChatEditMode.ReplaceSymbol,
                selectedSymbol = symbol,
                targetValidation = ChatTargetValidation(target),
                message = "Improve documentation.",
                functionModel =
                    ScopedModel(
                        profile = "remote-profile",
                        model = "edit-model",
                        providerOrigin = "https://provider.example.invalid",
                        remoteProvider = true))
    assertEquals(
        "Confirm the Function remote destination before sending.",
        assistantComposerBlockedReason(base))
    assertEquals(null, assistantComposerBlockedReason(base.copy(remoteConfirmed = true)))
    assertEquals(
        "Wait for the current draft validation to finish.",
        assistantComposerBlockedReason(base.copy(remoteConfirmed = true, validating = true)))
    val invalid =
        base.copy(
            target = null,
            targetValidation =
                ChatTargetValidation(
                    message =
                        "Select one declaration. Multi-function or grouped declaration changes are not supported."))
    assertEquals(invalid.targetValidation.message, assistantComposerBlockedReason(invalid))
    var confirmations = 0
    var sends = 0
    ComposeVisualFixture(420, 650) {
          AssistantToolWindow(
              base,
              AssistantConversationActions({}, {}, { confirmations++ }, {}, { sends++ }, {}),
              DraftEditorActions({}, {}, {}),
              Modifier.fillMaxSize())
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText("Confirm the Function remote destination before sending."))
          assertTrue(fixture.isDisabled("Send message"))
          fixture.clickText("Confirm remote destination")
          assertEquals(1, confirmations)
          assertEquals(0, sends)
        }
    ComposeVisualFixture(420, 650) {
          AssistantToolWindow(
              invalid,
              AssistantConversationActions({}, {}, {}, {}, {}, {}),
              DraftEditorActions({}, {}, {}),
              Modifier.fillMaxSize())
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText(invalid.targetValidation.message))
          assertTrue(fixture.isDisabled("Send message"))
        }
    ComposeVisualFixture(420, 650) {
          AssistantToolWindow(
              base.copy(remoteConfirmed = true),
              AssistantConversationActions({}, {}, {}, {}, {}, {}),
              DraftEditorActions({}, {}, {}),
              Modifier.fillMaxSize())
        }
        .use { fixture ->
          fixture.render()
          assertTrue(
              fixture.hasText(
                  "Quick changes require one Go function or method. Select one to use a preset."))
          assertTrue(fixture.hasText("Function provider origin: https://provider.example.invalid"))
          assertFalse(fixture.hasText("Refactor"))
          assertFalse(fixture.isDisabled("Send message"))
        }
  }

  @Test
  fun composerKeepsCancelReachableWhileRunningAndExplainsBlockedIntent() {
    val target = ChatTarget(ChatEditMode.ReplaceSymbol, "Run")
    val symbol = SymbolInfo("Run", "function", "func Run()", 1, 3, "exact", true)
    val base =
        historyState(target, null, emptyList())
            .copy(
                mode = ChatEditMode.ReplaceSymbol,
                selectedSymbol = symbol,
                targetValidation = ChatTargetValidation(target),
                message = "   ",
                messageInput = androidx.compose.ui.text.input.TextFieldValue("   "))
    assertEquals("Enter a specific intent before sending.", assistantComposerBlockedReason(base))
    val state = base.copy(sending = true)
    var canceled = 0
    var sent = 0
    ComposeVisualFixture(420, 650) {
          AssistantToolWindow(
              state,
              AssistantConversationActions({}, {}, {}, {}, { sent++ }, { canceled++ }),
              DraftEditorActions({}, {}, {}),
              Modifier.fillMaxSize())
        }
        .use { fixture ->
          fixture.render()
          assertTrue(
              fixture.hasText(
                  "Request running · Cancel keeps the existing draft and conversation."))
          assertFalse(fixture.hasText("Send message"))
          fixture.clickText("Cancel request")
          assertEquals(1, canceled)
          assertEquals(0, sent)
        }
  }

  @Test
  fun firstSessionFailureRendersSubmittedTextAndLiteralDiagnosticWithoutDispatch() {
    val target = ChatTarget(ChatEditMode.CreateSymbol, "Build")
    val scope = ChatRequestScope("project", "revision", "empty.go", "hash", target)
    var sends = 0
    ComposeVisualFixture(480, 900) {
          AssistantToolWindow(
              historyState(
                  target,
                  null,
                  listOf(
                      ChatRequestAttempt(
                          1,
                          scope,
                          ScopedModel(),
                          false,
                          "<b>literal request</b>",
                          0,
                          ChatRequestOutcome.Failed("provider\u0000 unavailable")))),
              AssistantConversationActions({}, {}, {}, {}, { sends++ }, {}),
              DraftEditorActions({}, {}, {}),
              Modifier.fillMaxSize())
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText("Request failed"))
          assertTrue(fixture.hasText("<b>literal request</b>"))
          assertTrue(fixture.hasText("provider  unavailable"))
          assertEquals(0, sends)
        }
  }

  @Test
  fun successfulTurnsAndFailedRetryAreInterleavedWithoutDuplicatingDaemonRequests() {
    val target = ChatTarget(ChatEditMode.CreateSymbol, "Build")
    val scope = ChatRequestScope("project", "revision", "empty.go", "hash", target)
    val attempts =
        listOf(
            ChatRequestAttempt(
                1,
                scope,
                ScopedModel(),
                false,
                "same request",
                0,
                ChatRequestOutcome.Succeeded("session", "draft-1")),
            ChatRequestAttempt(
                2,
                scope,
                ScopedModel(),
                false,
                "failed request",
                0,
                ChatRequestOutcome.Failed("timeout; send again")),
            ChatRequestAttempt(
                3, scope, ScopedModel(), false, "same request", 0, ChatRequestOutcome.Canceled),
            ChatRequestAttempt(
                4,
                scope,
                ScopedModel(),
                false,
                "same request",
                0,
                ChatRequestOutcome.Succeeded("session", "draft-2")))
    val session =
        ChatSession(
            "session",
            "project",
            "revision",
            "hash",
            "empty.go",
            "create_symbol",
            "Build",
            "active",
            messages =
                listOf(
                    ChatSessionMessage("user", "same request"),
                    ChatSessionMessage(
                        "assistant", "<img src='https://example.invalid/a'>", "draft-1"),
                    ChatSessionMessage("user", "same request"),
                    ChatSessionMessage("assistant", "", "draft-2")))
    val state = historyState(target, session, attempts)
    val entries = assistantHistoryEntries(state)
    assertEquals(6, entries.size)
    assertEquals(
        listOf(
            "same request",
            "<img src='https://example.invalid/a'>",
            "failed request",
            "same request",
            "same request",
            ""),
        entries.map {
          when (it) {
            is AssistantHistoryEntry.Turn -> it.message.content
            is AssistantHistoryEntry.Attempt -> it.attempt.requestText
          }
        })
    assertIs<AssistantHistoryEntry.Attempt>(entries[2])
    assertIs<AssistantHistoryEntry.Attempt>(entries[3])
    ComposeVisualFixture(480, 1000) {
          AssistantToolWindow(
              state,
              AssistantConversationActions({}, {}, {}, {}, {}, {}),
              DraftEditorActions({}, {}, {}),
              Modifier.fillMaxSize())
        }
        .use { fixture ->
          fixture.render()
          assertEquals(3, fixture.textCount("same request"))
          assertTrue(fixture.hasText("Request failed"))
          assertTrue(fixture.hasText("Request canceled"))
          assertTrue(fixture.hasText("timeout; send again"))
          assertTrue(fixture.hasText("<img src='https://example.invalid/a'>"))
          assertEquals(2, fixture.textCount("Model response"))
        }
  }

  @Test
  fun runningRequestRendersSubmittedTextBeforeAnySessionExists() {
    val target = ChatTarget(ChatEditMode.CreateSymbol, "Build")
    val scope = ChatRequestScope("project", "revision", "empty.go", "hash", target)
    val attempt = ChatRequestAttempt(1, scope, ScopedModel(), false, "pending request", 0)
    ComposeVisualFixture(480, 900) {
          AssistantToolWindow(
              historyState(target, null, listOf(attempt)).copy(sending = true),
              AssistantConversationActions({}, {}, {}, {}, {}, {}),
              DraftEditorActions({}, {}, {}),
              Modifier.fillMaxSize())
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText("Request running"))
          assertTrue(fixture.hasText("pending request"))
          assertTrue(fixture.hasText("Cancel request"))
          assertFalse(fixture.hasText("Request failed"))
        }
  }

  @Test
  fun failureAfterLoadedDaemonTurnsKeepsThoseTurnsAheadOfTheFailure() {
    val target = ChatTarget(ChatEditMode.CreateSymbol, "Build")
    val scope = ChatRequestScope("project", "revision", "empty.go", "hash", target)
    val session =
        ChatSession(
            "session",
            "project",
            "revision",
            "hash",
            "empty.go",
            "create_symbol",
            "Build",
            "active",
            messages =
                listOf(
                    ChatSessionMessage("user", "earlier request"),
                    ChatSessionMessage("assistant", "", "earlier-draft")))
    val attempt =
        ChatRequestAttempt(
            1,
            scope,
            ScopedModel(),
            false,
            "later failed request",
            0,
            ChatRequestOutcome.Failed("timeout"))
    val entries = assistantHistoryEntries(historyState(target, session, listOf(attempt)))
    assertEquals(3, entries.size)
    assertEquals(
        "earlier request", assertIs<AssistantHistoryEntry.Turn>(entries[0]).message.content)
    assertEquals("", assertIs<AssistantHistoryEntry.Turn>(entries[1]).message.content)
    assertEquals(
        "later failed request",
        assertIs<AssistantHistoryEntry.Attempt>(entries[2]).attempt.requestText)
  }

  @Test
  fun retainedSameFileHistoryLabelsOriginalScopeAndNeverLeaksAcrossFilesOrProjects() {
    val target = ChatTarget(ChatEditMode.CreateSymbol, "Build")
    val oldScope = ChatRequestScope("project", "revision", "empty.go", "hash", target)
    val attempt =
        ChatRequestAttempt(
            1,
            oldScope,
            ScopedModel(),
            false,
            "old request",
            0,
            ChatRequestOutcome.Failed("old failure"))
    val session =
        ChatSession(
            "session",
            "project",
            "revision",
            "hash",
            "empty.go",
            "create_symbol",
            "Build",
            "active",
            messages = listOf(ChatSessionMessage("assistant", "old prose")))
    val state = historyState(target.copy(symbol = "Other"), session, listOf(attempt))
    val entries = assistantHistoryEntries(state)
    assertEquals(2, entries.size)
    assertTrue(
        entries.all { entry ->
          when (entry) {
            is AssistantHistoryEntry.Turn -> entry.scopeLabel
            is AssistantHistoryEntry.Attempt -> entry.scopeLabel
          }?.contains("empty.go · Build") == true
        })
    assertTrue(
        assistantHistoryEntries(state.copy(selected = creationFile().copy(path = "other.go")))
            .isEmpty())
    assertTrue(
        assistantHistoryEntries(
                state.copy(project = testHistoryProject().copy(projectId = "other")))
            .isEmpty())
    val changedRevision =
        assistantHistoryEntries(
            historyState(target, session, listOf(attempt))
                .copy(
                    project = testHistoryProject().copy(projectRevision = "new-revision"),
                    selected = creationFile().copy(contentHash = "new-hash")))
    assertTrue(
        changedRevision
            .filterIsInstance<AssistantHistoryEntry.Attempt>()
            .single()
            .scopeLabel
            ?.contains("revision revision, file hash hash") == true)
  }

  private fun testHistoryProject() =
      ProjectAnalysis(
          "project",
          "revision",
          "project",
          "/tmp/project",
          "go",
          fileCount = 1,
          sourceFileCount = 1,
          totalLines = 1,
          summary = "",
          aiStatus = "fresh",
          analyzedAt = "")

  private fun historyState(
      target: ChatTarget,
      session: ChatSession?,
      attempts: List<ChatRequestAttempt>
  ) =
      AssistantToolWindowState(
          testHistoryProject(),
          creationFile(),
          session,
          null,
          null,
          target,
          ChatEditMode.CreateSymbol,
          target.symbol,
          "unsent intent",
          false,
          ScopedModel(),
          false,
          FocusRequester(),
          FocusRequester(),
          attempts = attempts)

  @Test
  fun inspectContextCanBeActivatedWithoutSelectionAndDoesNotSendOrConfirm() {
    var inspections = 0
    var sends = 0
    var confirmations = 0
    ComposeVisualFixture(360, 900) {
          AssistantToolWindow(
              AssistantToolWindowState(
                  null,
                  null,
                  null,
                  null,
                  null,
                  null,
                  ChatEditMode.ReplaceSymbol,
                  "",
                  "",
                  false,
                  ScopedModel(),
                  false,
                  FocusRequester(),
                  FocusRequester()),
              AssistantConversationActions(
                  {}, {}, { confirmations++ }, { inspections++ }, { sends++ }, {}),
              DraftEditorActions({}, {}, {}),
              Modifier.fillMaxSize())
        }
        .use { fixture ->
          fixture.render()
          assertFalse(fixture.isDisabled("Inspect context"))
          fixture.clickText("Inspect context")
          assertEquals(1, inspections)
          assertEquals(0, sends)
          assertEquals(0, confirmations)
        }
  }

  @Test
  fun creationFormFocusesTheNameThenBehaviorAndDoesNotGenerateUntilRequested() {
    DeclarationCreationKind.entries.forEach { kind ->
      var name by mutableStateOf("")
      var behavior by mutableStateOf("")
      var requests = 0
      var toolSelections = 0
      val nameFocus = FocusRequester()
      val chatFocus = FocusRequester()
      val file = creationFile()
      val nameLabel = "${kind.noun.replaceFirstChar { it.uppercase() }} name"
      ComposeVisualFixture(360, 900, 1.5f) {
            val validation =
                validateChatTarget(file, emptyList(), null, ChatEditMode.CreateSymbol, name)
            RightToolWindowContainer(
                RightToolWindow.Assistant,
                { toolSelections++ },
                content = { _, contentModifier ->
                  AssistantToolWindow(
                      AssistantToolWindowState(
                          null,
                          file,
                          null,
                          null,
                          null,
                          validation.target,
                          ChatEditMode.CreateSymbol,
                          name,
                          behavior,
                          false,
                          ScopedModel(),
                          false,
                          chatFocus,
                          FocusRequester(),
                          targetValidation = validation,
                          creationKind = kind,
                          creationNameFocus = nameFocus),
                      AssistantConversationActions(
                          { behavior = it }, { name = it }, {}, {}, { requests++ }, {}),
                      DraftEditorActions({}, {}, {}),
                      contentModifier)
                },
                modifier = Modifier.fillMaxSize())
            LaunchedEffect(Unit) { nameFocus.requestFocus() }
          }
          .use { fixture ->
            fixture.render("create-${kind.noun}-empty-360-1.5")
            assertTrue(fixture.hasText("New ${kind.noun}"))
            assertTrue(fixture.hasText("empty.go"))
            assertTrue(fixture.isDescriptionFocused(nameLabel))
            assertTrue(fixture.isDisabled("Generate ${kind.noun}"))
            assertEquals(0, requests)
            fixture.setFocusedText("Build")
            fixture.render()
            assertEquals("Build", name)
            fixture.pressKey(Key.Tab)
            fixture.render()
            assertTrue(fixture.isDescriptionFocused("Behavior"))
            fixture.setFocusedText("Return a reusable result.")
            fixture.pressKey(Key.DirectionLeft)
            fixture.pressKey(Key.Enter)
            fixture.render("create-${kind.noun}-ready-360-1.5")
            assertEquals(0, toolSelections)
            assertTrue(fixture.isDescriptionFocused("Behavior"))
            assertFalse(fixture.isDisabled("Generate ${kind.noun}"))
            fixture.clickText("Generate ${kind.noun}")
            assertEquals(1, requests)
            assertEquals("package demo\n", file.content)
          }
    }
  }

  @Test
  fun creationMessagesPreserveIntentAndIdentifyFunctionVersusType() {
    val intent = functionChangeRequest("Preserve order.", "Keep the public signature.")
    assertEquals(
        "Create a Go function named Build.\n\n$intent",
        creationMessage(DeclarationCreationKind.Function, " Build ", intent))
    assertEquals(
        "Create a Go type named Config.\n\nKeep fields exported.",
        creationMessage(DeclarationCreationKind.Type, "Config", "Keep fields exported."))
  }

  @Test
  fun unsupportedCreationExplainsTheFileBoundaryAndCannotGenerate() {
    val file = creationFile().copy(language = "Kotlin")
    val validation = validateChatTarget(file, emptyList(), null, ChatEditMode.CreateSymbol, "Build")
    ComposeVisualFixture(360, 900, 1.5f) {
          AssistantToolWindow(
              AssistantToolWindowState(
                  null,
                  file,
                  null,
                  null,
                  null,
                  validation.target,
                  ChatEditMode.CreateSymbol,
                  "Build",
                  "Return a result.",
                  false,
                  ScopedModel(),
                  false,
                  FocusRequester(),
                  FocusRequester(),
                  targetValidation = validation),
              AssistantConversationActions({}, {}, {}, {}, {}, {}),
              DraftEditorActions({}, {}, {}),
              Modifier.fillMaxSize())
        }
        .use { fixture ->
          fixture.render("create-unsupported-360-1.5")
          assertTrue(fixture.hasText("Function and type creation requires a Go source file."))
          assertTrue(fixture.isDisabled("Generate function"))
        }
  }

  private fun creationFile() =
      ProjectFileInfo(
          "empty.go",
          "hash",
          "empty.go",
          language = "Go",
          sizeBytes = 13,
          lineCount = 1,
          modifiedAt = "",
          binary = false,
          content = "package demo\n")

  @Test
  fun conversationLabelsDistinguishRequestsResponsesAndUnknownRoles() {
    assertEquals("Your request", assistantMessageLabel("user"))
    assertEquals("Model response", assistantMessageLabel("assistant"))
    assertEquals("Model response", assistantMessageLabel("ASSISTANT"))
    assertEquals("System context", assistantMessageLabel("system"))
    assertEquals("Tool", assistantMessageLabel("tool"))
    assertEquals("Message", assistantMessageLabel(""))
  }

  @Test
  fun compactDraftStatusCopyRetainsReviewAndStaleRecoveryGuidance() {
    assertEquals("Validate before review.", draftEditorStatusMessage(DraftEditorStatus.Generated))
    assertEquals(
        "Edits need validation and focused checks.",
        draftEditorStatusMessage(DraftEditorStatus.Dirty))
    assertEquals("Validating.", draftEditorStatusMessage(DraftEditorStatus.Validating))
    assertEquals(
        "Validated. Run focused checks before review.",
        draftEditorStatusMessage(DraftEditorStatus.Valid))
    assertEquals(
        "Fix validation diagnostics before continuing.",
        draftEditorStatusMessage(DraftEditorStatus.Invalid))
    assertEquals(
        "Draft is stale. Start a new conversation.",
        draftEditorStatusMessage(DraftEditorStatus.Stale))
  }

  @Test
  fun preparedPresetTextPlacesTheCaretAfterTheRequestLead() {
    val prepared = preparedFunctionChangeMessage(FunctionChangePreset.Refactor)

    assertEquals("Refactor without changing behavior: ", prepared.text)
    assertEquals(prepared.text.length, prepared.selection.start)
    assertEquals(prepared.text.length, prepared.selection.end)
  }
}
