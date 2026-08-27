package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

class DraftEditorStateTest {
    @Test fun manualDeclarationOrImportEditMakesTheDraftDirtyAndClearsEvidence() {
        val original = draft(validation = GenerationValidation(true, "replace_symbol", diff = UnifiedDiff("main.go", "main.go")))
        val state = DesktopState(
            review = DraftReviewState(
                draft = original,
                editor = editableDraft(original),
                checks = CandidateCheckReport("main.go", true, draftId = original.id, draftRevision = original.revision, draftHash = original.hash),
            ),
        ).reduce(DesktopEvent.DraftEdited(declaration = "func Run() error { return nil }", imports = listOf("fmt")))

        assertEquals(DraftEditorStatus.Dirty, state.review.editor?.status)
        assertEquals("func Run() error { return nil }", state.review.editor?.declaration)
        assertEquals(listOf("fmt"), state.review.editor?.imports)
        assertNull(state.review.draft?.validation)
        assertNull(state.review.checks)
        assertFalse(draftApplyEligibility(state.review.draft, state.review.checks, file()).eligible)
    }

    @Test fun validationResponseReplacesLocalTextWithDaemonNormalizedDraft() {
        val generated = draft()
        val normalized = generated.copy(
            declaration = "func Run() error {\n\treturn nil\n}",
            imports = listOf("fmt"),
            revision = 3,
            hash = "normalized",
            validation = GenerationValidation(true, "replace_symbol", diff = UnifiedDiff("main.go", "main.go")),
        )
        val state = DesktopState(review = DraftReviewState(draft = generated, editor = editDraft(editableDraft(generated), declaration = "func Run() error{return nil}")))
            .reduce(DesktopEvent.DraftLoaded(normalized))

        assertEquals(DraftEditorStatus.Valid, state.review.editor?.status)
        assertEquals(normalized.declaration, state.review.editor?.declaration)
        assertEquals(normalized.imports, state.review.editor?.imports)
        assertEquals(3, state.review.draft?.revision)
    }

    @Test fun invalidValidationResultStaysEditableAndCannotAuthorizeChecksOrApply() {
        val invalid = draft(validation = GenerationValidation(false, "replace_symbol", diagnostics = listOf(GenerationFinding("syntax", "missing brace")), diff = UnifiedDiff("main.go", "main.go")))
        val state = DesktopState().reduce(DesktopEvent.DraftLoaded(invalid))

        assertEquals(DraftEditorStatus.Invalid, state.review.editor?.status)
        assertEquals("missing brace", state.review.editor?.diagnostics?.single()?.message)
        assertFalse(draftApplyEligibility(state.review.draft, null, file()).eligible)
        assertEquals(DraftEditorStatus.Dirty, state.reduce(DesktopEvent.DraftEdited(declaration = "func Run() {} ")).review.editor?.status)
    }

    @Test fun optimisticConflictMarksTheDraftStaleAndPreservesNoPriorApproval() {
        val valid = draft(validation = GenerationValidation(true, "replace_symbol", diff = UnifiedDiff("main.go", "main.go")))
        val state = DesktopState(review = DraftReviewState(draft = valid, editor = editableDraft(valid), checks = CandidateCheckReport("main.go", true, draftId = valid.id, draftRevision = valid.revision, draftHash = valid.hash)))
            .reduce(DesktopEvent.DraftMarkedStale)

        assertEquals(DraftEditorStatus.Stale, state.review.editor?.status)
        assertNull(state.review.draft?.validation)
        assertNull(state.review.checks)
        assertFalse(draftApplyEligibility(state.review.draft, state.review.checks, file()).eligible)
    }

    @Test fun staleBaseIdentityCannotBeValidated() {
        val editor = editableDraft(draft())

        assertTrue(draftEditorMatchesOpenFile(editor, file(), project()))
        assertFalse(draftEditorMatchesOpenFile(editor, file(hash = "changed"), project()))
        assertFalse(draftEditorMatchesOpenFile(editor, file(), project(revision = "next")))
    }

    private fun draft(validation: GenerationValidation? = null) = DeclarationDraft("draft", "project", "revision", "base", "main.go", "replace_symbol", "Run", "func Run() {}", revision = 2, hash = "hash", validation = validation)
    private fun file(hash: String = "base") = ProjectFileInfo("main.go", hash, "main.go", language = "Go", sizeBytes = 1, lineCount = 1, modifiedAt = "", binary = false)
    private fun project(revision: String = "revision") = ProjectAnalysis("project", revision, "project", "/tmp/project", "go", fileCount = 1, sourceFileCount = 1, totalLines = 1, analysisFile = "", summary = "", aiStatus = "fresh", analyzedAt = "")
}
