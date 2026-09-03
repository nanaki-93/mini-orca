package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class ReviewContextPaneTest {
    @Test fun eligibleDecisionNamesExactlyOneSymbolAndFile() {
        val current = draft()
        val checks = DraftCheckReport("main.go", true, draftId = current.id, draftRevision = current.revision, draftHash = current.hash)

        val decision = applyDecisionUiState(project(), file(), editableDraft(current), current, checks, applied = null)

        assertTrue(decision.eligible)
        assertEquals("Apply Run to main.go", decision.actionLabel)
        assertTrue(decision.reason.contains("Ready"))
    }

    @Test fun dirtyInvalidOrUncheckedDraftRemainsBlockedWithARecoveryReason() {
        val current = draft()
        val dirty = editDraft(editableDraft(current), declaration = "func Run() error { return nil }")
        val noChecks = applyDecisionUiState(project(), file(), dirty, current.copy(validation = null), checks = null, applied = null)
        val invalid = current.copy(validation = DeclarationValidation(false, "replace_symbol", diff = UnifiedDiff("main.go", "main.go")))
        val failedChecks = DraftCheckReport("main.go", true, checks = listOf(DraftCheck("go test", required = true, state = "failed")), draftId = current.id, draftRevision = current.revision, draftHash = current.hash)
        val failed = applyDecisionUiState(project(), file(), editableDraft(invalid), invalid, failedChecks, applied = null)

        assertFalse(noChecks.eligible)
        assertTrue(noChecks.reason.contains("Manual edits"))
        assertFalse(failed.eligible)
        assertTrue(failed.reason.contains("diagnostics"))
    }

    @Test fun receiptUsesHumanReadableTargetAndOnlyOffersReturnedUndo() {
        val applied = applyDecisionUiState(project(), file(), null, null, null, ApplyResult("next", "after", true))
        val undone = applyDecisionUiState(project(), file(), null, null, null, ApplyResult("restored", "base", false, audit = AuditEntry("undo", "main.go", "applied", "now")))

        assertEquals("Change applied", applied.receiptTitle)
        assertEquals("Selected file was updated.", applied.receiptDetail)
        assertEquals("Undo this change", applied.undoLabel)
        assertEquals("Change undone", undone.receiptTitle)
        assertEquals("Undo is no longer available", undone.undoLabel)
    }

    private fun draft() = DeclarationDraft(
        "draft", "project", "revision", "base", "main.go", "replace_symbol", "Run", "func Run() {}",
        revision = 2,
        hash = "draft-hash",
        validation = DeclarationValidation(true, "replace_symbol", diff = UnifiedDiff("main.go", "main.go")),
    )

    private fun project() = ProjectAnalysis("project", "revision", "project", "/tmp/project", "go", fileCount = 1, sourceFileCount = 1, totalLines = 1, analysisFile = "", summary = "", aiStatus = "fresh", analyzedAt = "")
    private fun file() = ProjectFileInfo("main.go", "base", "main.go", language = "Go", sizeBytes = 1, lineCount = 1, modifiedAt = "", binary = false)
}
