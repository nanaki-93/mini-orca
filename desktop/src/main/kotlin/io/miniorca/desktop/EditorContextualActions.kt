package io.miniorca.desktop

/** Actions that are currently meaningful for the one-file editor workflow. */
internal data class EditorContextualActions(
    val canFocusChat: Boolean,
    val canFocusDraft: Boolean,
    val canGenerate: Boolean,
    val canValidateDraft: Boolean,
    val canRunFocusedChecks: Boolean,
)

internal fun editorContextualActions(
    state: DesktopState,
    mode: ChatEditMode,
    requestedSymbol: String,
    message: String,
    sending: Boolean,
    functionModel: ScopedModel,
    remoteProviderConfirmed: Boolean,
): EditorContextualActions {
    if (state.workspace != Workspace.Editor) {
        return EditorContextualActions(
            canFocusChat = false,
            canFocusDraft = false,
            canGenerate = false,
            canValidateDraft = false,
            canRunFocusedChecks = false,
        )
    }
    val target = validateChatTarget(state.selectedFile, state.symbols, state.selectedSymbol, mode, requestedSymbol)
    val editor = state.review.editor
    val draft = state.review.draft
    val draftCurrent = draftEditorMatchesOpenFile(editor, state.selectedFile, state.project)
    val canValidate = draftCurrent && editor?.status in setOf(
        DraftEditorStatus.Generated,
        DraftEditorStatus.Dirty,
        DraftEditorStatus.Invalid,
    )
    val canChecks = reviewEvidenceUiState(
        state.project,
        state.selectedFile,
        editor,
        draft,
        state.review.checks,
        checksRunning = state.loading,
    ).canRunChecks
    return EditorContextualActions(
        canFocusChat = target.valid,
        canFocusDraft = draftCurrent,
        canGenerate = target.valid && message.isNotBlank() && !sending && (!functionModel.remoteProvider || remoteProviderConfirmed),
        canValidateDraft = canValidate,
        canRunFocusedChecks = canChecks,
    )
}
