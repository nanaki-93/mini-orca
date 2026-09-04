package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals

class AssistantToolWindowTest {
  @Test
  fun compactDraftStatusCopyRetainsReviewAndStaleRecoveryGuidance() {
    assertEquals("Validate before review.", draftEditorStatusMessage(DraftEditorStatus.Generated))
    assertEquals(
        "Edits need validation and focused checks.",
        draftEditorStatusMessage(DraftEditorStatus.Dirty))
    assertEquals("Validating.", draftEditorStatusMessage(DraftEditorStatus.Validating))
    assertEquals(
        "Validated. Review evidence and checks are current.",
        draftEditorStatusMessage(DraftEditorStatus.Valid))
    assertEquals(
        "Fix validation diagnostics before continuing.",
        draftEditorStatusMessage(DraftEditorStatus.Invalid))
    assertEquals(
        "Draft is stale. Start a new conversation.",
        draftEditorStatusMessage(DraftEditorStatus.Stale))
  }
}
