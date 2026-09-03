package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class EditorProgressStateTest {
    @Test fun progressComesFromCurrentSelectionDraftValidationAndReceiptTruth() {
        val ready = stateWithDraft()

        assertEquals(EditorProgress.Review, editorProgressUiState(ready).progress)
        assertEquals(EditorProgress.Edit, editorProgressUiState(ready.reduce(DesktopEvent.DraftEdited(declaration = "func Run() {}"))).progress)
        assertEquals(
            EditorProgress.Inspect,
            editorProgressUiState(ready.copy(selection = ready.selection.copy(selectedSymbol = SymbolInfo("Other", "function", confidence = "exact", atomicTarget = true)))).progress,
        )
        assertEquals(
            EditorProgress.Receipt,
            editorProgressUiState(ready.copy(review = ready.review.copy(applied = ApplyResult("next", "hash", true)))).progress,
        )
    }

    @Test fun contextualShortcutsRequireTheSameCurrentActionsAsTheEditor() {
        val ready = stateWithDraft()
        val localFunction = ScopedModel(scope = "function", profile = "function", model = "local")
        val review = editorContextualActions(ready, ChatEditMode.ReplaceSymbol, "", "", sending = false, functionModel = localFunction, remoteProviderConfirmed = false)
        val editable = editorContextualActions(ready.reduce(DesktopEvent.DraftEdited(declaration = "func Run() error { return nil }")), ChatEditMode.ReplaceSymbol, "", "Explain the change", sending = false, functionModel = localFunction, remoteProviderConfirmed = false)

        assertTrue(review.canFocusChat)
        assertTrue(review.canFocusDraft)
        assertFalse(review.canValidateDraft)
        assertTrue(review.canRunFocusedChecks)
        assertTrue(editable.canGenerate)
        assertTrue(editable.canValidateDraft)
        assertFalse(editable.canRunFocusedChecks)

        val hidden = editorContextualActions(ready.copy(workspace = Workspace.Summary), ChatEditMode.ReplaceSymbol, "", "Explain the change", sending = false, functionModel = localFunction, remoteProviderConfirmed = false)

        assertFalse(hidden.canGenerate)
        assertFalse(hidden.canValidateDraft)
        assertFalse(hidden.canRunFocusedChecks)
    }

    @Test fun createComposerCannotSendUntilTheNewNameIsValid() {
        val state = stateWithDraft().copy(chat = ChatState(), review = DraftReviewState())
        val localFunction = ScopedModel(scope = "function", profile = "function", model = "local")
        val invalid = editorContextualActions(state, ChatEditMode.CreateSymbol, "", "Create it", sending = false, functionModel = localFunction, remoteProviderConfirmed = false)
        val valid = editorContextualActions(state, ChatEditMode.CreateSymbol, "NewRun", "Create it", sending = false, functionModel = localFunction, remoteProviderConfirmed = false)

        assertFalse(invalid.canFocusChat)
        assertFalse(invalid.canGenerate)
        assertTrue(valid.canFocusChat)
        assertTrue(valid.canGenerate)
    }

    @Test fun generationUsesFunctionProviderLocalityOnly() {
        val state = stateWithDraft().copy(review = DraftReviewState(), chat = ChatState())
        val remoteFunction = ScopedModel(scope = "function", profile = "function", model = "remote-code", remoteProvider = true)
        val localFunction = remoteFunction.copy(model = "local-code", remoteProvider = false)

        assertFalse(editorContextualActions(state, ChatEditMode.ReplaceSymbol, "", "Explain", sending = false, functionModel = remoteFunction, remoteProviderConfirmed = false).canGenerate)
        assertTrue(editorContextualActions(state, ChatEditMode.ReplaceSymbol, "", "Explain", sending = false, functionModel = remoteFunction, remoteProviderConfirmed = true).canGenerate)
        assertTrue(editorContextualActions(state, ChatEditMode.ReplaceSymbol, "", "Explain", sending = false, functionModel = localFunction, remoteProviderConfirmed = false).canGenerate)
    }

    private fun stateWithDraft(): DesktopState {
        val draft = draft()
        return DesktopState(
            workspace = Workspace.Editor,
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

    private fun project() = ProjectAnalysis("project", "revision", "project", "/tmp/project", "go", fileCount = 1, sourceFileCount = 1, totalLines = 1, analysisFile = "", summary = "", aiStatus = "fresh", analyzedAt = "")
    private fun file() = ProjectFileInfo("main.go", "base", "main.go", language = "Go", sizeBytes = 1, lineCount = 1, modifiedAt = "", binary = false)
    private fun symbol() = SymbolInfo("Run", "function", "func Run()", 1, 3, "exact", true)
    private fun session() = ChatSession("session", "project", "revision", "base", "main.go", "replace_symbol", "Run", "active", "draft")
    private fun draft() = DeclarationDraft("draft", "project", "revision", "base", "main.go", "replace_symbol", "Run", "func Run() {}", revision = 2, hash = "draft-hash", validation = GenerationValidation(true, "replace_symbol", diff = UnifiedDiff("main.go", "main.go")))
}
