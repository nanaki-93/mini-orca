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
  fun requestFailureAppearsBesideItsTargetWithoutExecutingAnotherRequest() {
    val target = ChatTarget(ChatEditMode.CreateSymbol, "Build")
    listOf(target, target.copy(symbol = "Other")).forEach { selected ->
      var calls = 0
      ComposeVisualFixture(360, 900, 1.5f) {
            AssistantToolWindow(
                AssistantToolWindowState(
                    null,
                    creationFile(),
                    null,
                    null,
                    null,
                    selected,
                    ChatEditMode.CreateSymbol,
                    selected.symbol,
                    "Return a reusable result",
                    false,
                    ScopedModel(),
                    false,
                    FocusRequester(),
                    FocusRequester(),
                    requestFailure = ChatRequestFailure(target, "provider\u0000 unavailable")),
                AssistantConversationActions({}, {}, {}, {}, { calls++ }, {}),
                DraftEditorActions({}, {}, {}),
                Modifier.fillMaxSize())
          }
          .use { fixture ->
            fixture.render("bottom-request-failure-${selected.symbol}-360-1.5")
            assertEquals(selected == target, fixture.hasText("Request failed"))
            assertEquals(selected == target, fixture.hasText("provider  unavailable"))
            assertEquals(0, calls)
          }
    }
  }

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
