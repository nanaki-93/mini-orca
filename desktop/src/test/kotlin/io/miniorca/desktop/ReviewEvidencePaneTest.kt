package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class ReviewEvidencePaneTest {
    @Test fun validatedButUncheckedDraftStaysInVerifyWithAnExplicitReason() {
        val state = verifyEvidenceUiState(project(), file(), editableDraft(draft()), draft(), checks = null)

        assertEquals(VerifyEvidenceStatus.Passed, state.validation.status)
        assertEquals(VerifyEvidenceStatus.Missing, state.checks.status)
        assertTrue(state.canRunChecks)
        assertFalse(state.canContinueToApply)
        assertTrue(state.continueReason.contains("Run checks"))
    }

    @Test fun staleOrFailedCheckEvidenceCannotEnableApply() {
        val current = draft()
        val stale = CandidateCheckReport("main.go", true, draftId = current.id, draftRevision = current.revision, draftHash = "old-hash")
        val failed = CandidateCheckReport("main.go", true, checks = listOf(CandidateCheck("go test", required = true, state = "failed")), draftId = current.id, draftRevision = current.revision, draftHash = current.hash)

        val staleEvidence = verifyEvidenceUiState(project(), file(), editableDraft(current), current, stale)
        val failedEvidence = verifyEvidenceUiState(project(), file(), editableDraft(current), current, failed)

        assertEquals(VerifyEvidenceStatus.Stale, staleEvidence.checks.status)
        assertFalse(staleEvidence.canContinueToApply)
        assertEquals(VerifyEvidenceStatus.Failed, failedEvidence.checks.status)
        assertFalse(failedEvidence.canContinueToApply)
    }

    @Test fun aManualEditInvalidatesCurrentCheckEvidenceAndReturnsTheUserToDraftRecovery() {
        val current = draft()
        val matchingChecks = CandidateCheckReport("main.go", true, draftId = current.id, draftRevision = current.revision, draftHash = current.hash)
        val edited = editDraft(editableDraft(current), declaration = "func Run() error { return nil }")
        val evidence = verifyEvidenceUiState(project(), file(), edited, current.copy(validation = null), matchingChecks)

        assertEquals(VerifyEvidenceStatus.Missing, evidence.validation.status)
        assertFalse(evidence.canRunChecks)
        assertFalse(evidence.canContinueToApply)
        assertTrue(evidence.continueReason.contains("Manual edits"))
    }

    @Test fun diagnosticsAndReadOnlyImpactAndGitContextRemainExplicit() {
        val invalid = draft().copy(validation = GenerationValidation(false, "replace_symbol", diagnostics = listOf(GenerationFinding("scope", "Only one declaration may change.")), diff = UnifiedDiff("main.go", "main.go")))
        val evidence = verifyEvidenceUiState(project(), file(), editableDraft(invalid), invalid, checks = null)
        val impact = ImpactPreview("main.go", "Run", listOf(ImpactReference("main_test.go", "Run", "high", "calls Run")))

        assertEquals(VerifyEvidenceStatus.Failed, evidence.validation.status)
        assertEquals("Only one declaration may change.", invalid.validation!!.diagnostics.single().message)
        assertTrue(advisoryImpactLabel(impact).contains("read-only"))
        assertTrue(gitContextLabel(GitStatus(true, "main", "modified", "changed")).contains("read-only"))
        assertTrue(gitContextLabel(null).contains("read-only"))
    }

    private fun draft() = DeclarationDraft(
        "draft", "project", "revision", "base", "main.go", "replace_symbol", "Run", "func Run() {}",
        revision = 2,
        hash = "draft-hash",
        validation = GenerationValidation(true, "replace_symbol", diff = UnifiedDiff("main.go", "main.go")),
    )

    private fun project() = ProjectAnalysis("project", "revision", "project", "/tmp/project", "go", fileCount = 1, sourceFileCount = 1, totalLines = 1, analysisFile = "", summary = "", aiStatus = "fresh", analyzedAt = "")
    private fun file() = ProjectFileInfo("main.go", "base", "main.go", language = "Go", sizeBytes = 1, lineCount = 1, modifiedAt = "", binary = false)
}
