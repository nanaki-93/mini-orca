package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class EditorFlowStateTest {
    @Test fun targetIsAlwaysReachableAndAValidBoundSessionUnlocksDraft() {
        val empty = editorFlowUiState(DesktopState(), ChatEditMode.ReplaceSymbol, "", EditorStage.Apply)

        assertEquals(EditorStage.Target, empty.activeStage)
        assertTrue(empty.stage(EditorStage.Target).unlocked)
        assertFalse(empty.stage(EditorStage.Draft).unlocked)

        val ready = editorFlowUiState(stateWithDraft(), ChatEditMode.ReplaceSymbol, "", EditorStage.Apply)

        assertEquals(EditorStage.Apply, ready.activeStage)
        assertEquals(setOf(EditorStage.Target, EditorStage.Draft, EditorStage.Verify, EditorStage.Apply), ready.unlockedStages)
        assertTrue(ready.apply.detail.contains("Ready to apply"))
    }

    @Test fun draftChangesAndValidationStatesRelockVerifyAndApplyAtDraft() {
        val ready = stateWithDraft()
        val dirty = ready.reduce(DesktopEvent.DraftEdited(declaration = "func Run() error { return nil }"))

        assertRelockedAtDraft(dirty)
        DraftEditorStatus.entries.filter { it in setOf(DraftEditorStatus.Generated, DraftEditorStatus.Validating, DraftEditorStatus.Invalid, DraftEditorStatus.Stale) }
            .forEach { status ->
                val state = ready.copy(review = ready.review.copy(editor = ready.review.editor!!.copy(status = status)))
                assertRelockedAtDraft(state)
            }
    }

    @Test fun staleProjectOrChecksClampForwardStagesToTheLatestUnlockedStage() {
        val ready = stateWithDraft()
        val staleProject = ready.copy(projectState = ready.projectState.copy(project = project("next")))
        val staleChecks = ready.copy(review = ready.review.copy(checks = ready.review.checks!!.copy(draftHash = "old-hash")))

        val staleProjectFlow = editorFlowUiState(staleProject, ChatEditMode.ReplaceSymbol, "", EditorStage.Apply)
        val staleChecksFlow = editorFlowUiState(staleChecks, ChatEditMode.ReplaceSymbol, "", EditorStage.Apply)

        assertEquals(EditorStage.Draft, staleProjectFlow.activeStage)
        assertTrue(staleProjectFlow.stage(EditorStage.Draft).unlocked)
        assertEquals(EditorStage.Verify, staleChecksFlow.activeStage)
        assertTrue(staleChecksFlow.stage(EditorStage.Verify).unlocked)
        assertFalse(staleChecksFlow.stage(EditorStage.Apply).unlocked)
        assertTrue(staleChecksFlow.checks.detail.contains("do not match"))
    }

    @Test fun appliedReceiptKeepsApplyVisibleWhileUndoneStateDoesNotBypassDraftGuards() {
        val applied = DesktopState(review = DraftReviewState(applied = ApplyResult("next", "applied-hash", true)))
        val undone = DesktopState(review = DraftReviewState(applied = ApplyResult("restored", "restored-hash", false)))

        val appliedFlow = editorFlowUiState(applied, ChatEditMode.ReplaceSymbol, "", EditorStage.Apply)
        val undoneFlow = editorFlowUiState(undone, ChatEditMode.ReplaceSymbol, "", EditorStage.Apply)

        assertEquals(EditorStage.Apply, appliedFlow.activeStage)
        assertTrue(appliedFlow.stage(EditorStage.Apply).unlocked)
        assertTrue(appliedFlow.apply.detail.contains("Undo is available"))
        assertEquals(EditorStage.Target, undoneFlow.activeStage)
        assertFalse(undoneFlow.stage(EditorStage.Apply).unlocked)
    }

    @Test fun validTargetOpensDraftBeforeTheFirstBoundMessage() {
        val state = stateWithDraft().copy(chat = ChatState(), review = DraftReviewState())

        val flow = editorFlowUiState(state, ChatEditMode.ReplaceSymbol, "", EditorStage.Draft)

        assertEquals(EditorStage.Draft, flow.activeStage)
        assertTrue(flow.stage(EditorStage.Draft).unlocked)
        assertFalse(flow.stage(EditorStage.Verify).unlocked)
        assertTrue(flow.draft.detail.contains("No conversation"))
    }

    private fun assertRelockedAtDraft(state: DesktopState) {
        val flow = editorFlowUiState(state, ChatEditMode.ReplaceSymbol, "", EditorStage.Apply)

        assertEquals(EditorStage.Draft, flow.activeStage)
        assertTrue(flow.stage(EditorStage.Draft).unlocked)
        assertFalse(flow.stage(EditorStage.Verify).unlocked)
        assertFalse(flow.stage(EditorStage.Apply).unlocked)
    }

    private fun stateWithDraft(): DesktopState {
        val draft = draft()
        return DesktopState(
            projectState = ProjectWorkspaceState(project(), ProjectIndex("project", "revision")),
            selection = FileSelectionState(selectedFile = file(), symbols = listOf(symbol()), selectedSymbol = symbol()),
            chat = ChatState(session()),
            review = DraftReviewState(
                draft = draft,
                editor = editableDraft(draft),
                checks = CandidateCheckReport("main.go", true, draftId = draft.id, draftRevision = draft.revision, draftHash = draft.hash),
            ),
        )
    }

    private fun project(revision: String = "revision") = ProjectAnalysis("project", revision, "project", "/tmp/project", "go", fileCount = 1, sourceFileCount = 1, totalLines = 1, analysisFile = "", summary = "", aiStatus = "fresh", analyzedAt = "")
    private fun file() = ProjectFileInfo("main.go", "base", "main.go", language = "Go", sizeBytes = 1, lineCount = 1, modifiedAt = "", binary = false)
    private fun symbol() = SymbolInfo("Run", "function", "func Run()", 1, 3, "exact", true)
    private fun session() = ChatSession("session", "project", "revision", "base", "main.go", "replace_symbol", "Run", "active", "draft")
    private fun draft() = DeclarationDraft("draft", "project", "revision", "base", "main.go", "replace_symbol", "Run", "func Run() {}", revision = 2, hash = "draft-hash", validation = GenerationValidation(true, "replace_symbol", diff = UnifiedDiff("main.go", "main.go")))
}
