package io.miniorca.desktop

enum class DraftEditorStatus { Generated, Dirty, Validating, Valid, Invalid, Stale }

data class EditableDraftState(
    val serverDraft: DeclarationDraft,
    val declaration: String = serverDraft.declaration,
    val imports: List<String> = serverDraft.imports,
    val status: DraftEditorStatus = draftEditorStatus(serverDraft),
) {
    val diagnostics: List<DeclarationFinding> get() = serverDraft.validation?.diagnostics.orEmpty()
}

fun draftEditorStatus(draft: DeclarationDraft): DraftEditorStatus = when {
    draft.validation?.applicable == true -> DraftEditorStatus.Valid
    draft.validation != null -> DraftEditorStatus.Invalid
    else -> DraftEditorStatus.Generated
}

fun editableDraft(draft: DeclarationDraft): EditableDraftState = EditableDraftState(draft)

fun editDraft(state: EditableDraftState, declaration: String = state.declaration, imports: List<String> = state.imports): EditableDraftState =
    state.copy(declaration = declaration, imports = imports, status = DraftEditorStatus.Dirty)

fun draftEditorMatchesOpenFile(editor: EditableDraftState?, file: ProjectFileInfo?, project: ProjectAnalysis?): Boolean =
    editor != null && file != null && project != null &&
        editor.serverDraft.projectId == project.projectId && editor.serverDraft.projectRevision == project.projectRevision &&
        editor.serverDraft.targetPath == file.path && editor.serverDraft.baseFileHash == file.contentHash &&
        editor.status != DraftEditorStatus.Stale
