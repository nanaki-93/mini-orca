package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

class WorkflowToolWindowsTest {
  @Test
  fun workflowBadgesSummarizeDraftReviewAndReceiptStateWithoutChangingEligibility() {
    val checksFailed = evidence(ReviewEvidenceStatus.Failed)
    val blocked = ApplyDecisionUiState(false, "Apply Run to main.go", "Run checks first.")
    val ready = ApplyDecisionUiState(true, "Apply Run to main.go", "Ready to apply.")
    val applied = ready.copy(eligible = false, receiptTitle = "Change applied")

    assertNull(workflowToolWindowBadges(null, checksFailed, blocked)[RightToolWindow.Assistant])
    assertEquals(
        "Draft",
        workflowToolWindowBadges(editableDraft(draft()), checksFailed, blocked)
            .getValue(RightToolWindow.Assistant)
            .label)
    assertEquals(
        "Invalid",
        workflowToolWindowBadges(invalidEditor(), checksFailed, blocked)
            .getValue(RightToolWindow.Assistant)
            .label)
    assertEquals(
        "Checks failed",
        workflowToolWindowBadges(editableDraft(draft()), checksFailed, blocked)
            .getValue(RightToolWindow.Review)
            .label)
    assertEquals(
        "Ready to apply",
        workflowToolWindowBadges(
                editableDraft(draft()), evidence(ReviewEvidenceStatus.Passed), ready)
            .getValue(RightToolWindow.Review)
            .label)
    assertEquals(
        "Applied",
        workflowToolWindowBadges(
                editableDraft(draft()), evidence(ReviewEvidenceStatus.Passed), applied)
            .getValue(RightToolWindow.Review)
            .label)
  }

  @Test
  fun assistantAndReviewScopesPinTheCurrentProjectRelativeFileAndTarget() {
    val file = file()
    val draft = draft()

    assertEquals(
        ToolWindowScope("internal/main.go", "Create new function/type · Build"),
        assistantToolWindowScope(file, ChatTarget(ChatEditMode.CreateSymbol, "Build"), null, ""))
    assertEquals(
        ToolWindowScope("internal/main.go", "Run"),
        reviewToolWindowScope(file, null, draft, applied = null))
  }

  @Test
  fun tabBadgesRemainTextualAndSelectingATabDoesNotAuthorizeWorkflowActions() {
    val badge = RightToolWindowBadge("Ready to apply")
    val layout = DesktopLayoutState(rightToolWindowVisible = false)

    assertEquals(
        "Review tool window tab, Ready to apply, not selected",
        rightToolWindowTabDescription(RightToolWindow.Review, selected = false, badge = badge))
    assertFalse(layout.rightToolWindowVisible)
    assertTrue(layout.openRight(RightToolWindow.Review).rightToolWindowVisible)
    assertFalse(ApplyDecisionUiState(false, "Apply draft", "Locked").eligible)
  }

  private fun evidence(checkStatus: ReviewEvidenceStatus) =
      ReviewEvidenceUiState(
          validation = ReviewEvidenceRow("Validation", "", ReviewEvidenceStatus.Passed),
          checks = ReviewEvidenceRow("Focused checks", "", checkStatus),
          identity = ReviewEvidenceRow("Scope identity", "", ReviewEvidenceStatus.Passed),
          canRunChecks = false,
          runChecksLabel = "Run focused checks",
      )

  private fun invalidEditor() = EditableDraftState(draft(), status = DraftEditorStatus.Invalid)

  private fun draft() =
      DeclarationDraft(
          id = "draft",
          projectId = "project",
          projectRevision = "revision",
          baseFileHash = "hash",
          targetPath = "internal/main.go",
          mode = ChatEditMode.ReplaceSymbol.wireValue,
          targetSymbol = "Run",
          declaration = "func Run() {}",
      )

  private fun file() =
      ProjectFileInfo(
          path = "internal/main.go",
          contentHash = "hash",
          name = "main.go",
          language = "Go",
          sizeBytes = 1,
          lineCount = 1,
          modifiedAt = "",
          binary = false,
      )
}
