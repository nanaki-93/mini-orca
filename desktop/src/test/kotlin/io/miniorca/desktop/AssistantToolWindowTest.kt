package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals

class AssistantToolWindowTest {
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
