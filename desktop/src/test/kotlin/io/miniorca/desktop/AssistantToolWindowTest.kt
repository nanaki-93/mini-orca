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
    val prepared = preparedFunctionChangeMessage(FunctionChangePreset.Behavior)

    assertEquals("Change behavior: ", prepared.text)
    assertEquals(prepared.text.length, prepared.selection.start)
    assertEquals(prepared.text.length, prepared.selection.end)
  }
}
