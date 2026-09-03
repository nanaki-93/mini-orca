package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class ReviewEvidencePaneTest {
    @Test fun validatedButUncheckedDraftStaysInReviewWithAnExplicitCheckAction() {
        val state = reviewEvidenceUiState(project(), file(), editableDraft(draft()), draft(), checks = null)

        assertEquals(ReviewEvidenceStatus.Passed, state.validation.status)
        assertEquals(ReviewEvidenceStatus.Missing, state.checks.status)
        assertTrue(state.canRunChecks)
    }

    @Test fun staleOrFailedCheckEvidenceCannotEnableApply() {
        val current = draft()
        val stale = CandidateCheckReport("main.go", true, draftId = current.id, draftRevision = current.revision, draftHash = "old-hash")
        val failed = CandidateCheckReport("main.go", true, checks = listOf(CandidateCheck("go test", required = true, state = "failed")), draftId = current.id, draftRevision = current.revision, draftHash = current.hash)

        val staleEvidence = reviewEvidenceUiState(project(), file(), editableDraft(current), current, stale)
        val failedEvidence = reviewEvidenceUiState(project(), file(), editableDraft(current), current, failed)

        assertEquals(ReviewEvidenceStatus.Stale, staleEvidence.checks.status)
        assertEquals(ReviewEvidenceStatus.Failed, failedEvidence.checks.status)
    }

    @Test fun verificationAndReceiptsKeepIdentityGuardsInternal() {
        val current = draft()
        val checks = CandidateCheckReport("main.go", true, draftId = current.id, draftRevision = current.revision, draftHash = current.hash)
        val evidence = reviewEvidenceUiState(project(), file(), editableDraft(current), current, checks)
        val receipt = applyDecisionUiState(project(), file(), editableDraft(current), current, checks, ApplyResult("revision", "post-hash", true, AuditEntry("apply", "main.go", "applied", "")))

        assertFalse(evidence.identity.detail.contains("revision"))
        assertFalse(evidence.identity.detail.contains("hash"))
        assertFalse(evidence.checks.detail.contains("revision"))
        assertFalse(evidence.checks.detail.contains("hash"))
        assertEquals("main.go was updated.", receipt.receiptDetail)
    }

    @Test fun aManualEditInvalidatesCurrentCheckEvidenceAndReturnsTheUserToDraftRecovery() {
        val current = draft()
        val matchingChecks = CandidateCheckReport("main.go", true, draftId = current.id, draftRevision = current.revision, draftHash = current.hash)
        val edited = editDraft(editableDraft(current), declaration = "func Run() error { return nil }")
        val evidence = reviewEvidenceUiState(project(), file(), edited, current.copy(validation = null), matchingChecks)

        assertEquals(ReviewEvidenceStatus.Missing, evidence.validation.status)
        assertFalse(evidence.canRunChecks)
        assertTrue(reviewValidationSummary(edited, validationCurrent = false).contains("Manual edits"))
    }

    @Test fun diagnosticsAndReadOnlyImpactAndGitContextRemainExplicit() {
        val invalid = draft().copy(validation = GenerationValidation(false, "replace_symbol", diagnostics = listOf(GenerationFinding("scope", "Only one declaration may change.")), diff = UnifiedDiff("main.go", "main.go")))
        val evidence = reviewEvidenceUiState(project(), file(), editableDraft(invalid), invalid, checks = null)
        val impact = ImpactPreview("main.go", "Run", listOf(ImpactReference("main_test.go", "Run", "high", "calls Run")))

        assertEquals(ReviewEvidenceStatus.Failed, evidence.validation.status)
        assertEquals("Only one declaration may change.", invalid.validation!!.diagnostics.single().message)
        assertTrue(advisoryImpactLabel(impact).contains("read-only"))
        assertTrue(gitContextLabel(GitStatus(true, "main", "modified", "changed")).contains("read-only"))
        assertTrue(gitContextLabel(null).contains("read-only"))
    }

    @Test fun taskBoundFailureCreatesABoundedExplicitRepairMessageUntilTheLimit() {
        val task = BugTaskSpec("1", "main.go", "Run", "func Run()", listOf("Return an error."))
        val current = draft().copy(taskSpec = task)
        val session = ChatSession(id = "session", taskSpec = task, repairCount = 2)
        val checks = CandidateCheckReport("main.go", false, listOf(CandidateCheck("task test candidate", required = true, state = "failed", output = "<workspace>/main_test.go: assertion failed")), current.id, current.revision, current.hash)

        val repair = repairMessageForChecks(session, current, checks)

        assertTrue(repair?.contains("sanitized focused check evidence") == true)
        assertTrue(repair?.contains("assertion failed") == true)
        assertTrue(repair!!.length < 4096)
        assertEquals(null, repairMessageForChecks(session.copy(repairCount = 3), current, checks))
        assertFalse(checksMatchDraft(CandidateCheckReport("main.go", false, draftId = current.id, draftRevision = current.revision, draftHash = "stale"), current))
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
