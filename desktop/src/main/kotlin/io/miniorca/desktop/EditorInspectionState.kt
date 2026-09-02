package io.miniorca.desktop

/** A source location and the most specific indexed declaration containing it. */
data class SourceLineSelection(
    val line: Int,
    val symbol: SymbolInfo?,
)

fun sourceLineSelection(symbols: List<SymbolInfo>, line: Int): SourceLineSelection =
    SourceLineSelection(line, symbolAtLine(symbols, line))

/**
 * Resolves overlapping declarations predictably: shortest range first, then an exact
 * atomic target, then the order returned by the index.
 */
fun symbolAtLine(symbols: List<SymbolInfo>, line: Int): SymbolInfo? =
    if (line <= 0) null else symbols.withIndex()
        .filter { (_, symbol) -> symbol.hasValidRange() && line in symbol.startLine..symbol.endLine }
        .minWithOrNull(
            compareBy<IndexedValue<SymbolInfo>> { (_, symbol) -> symbol.rangeLength() }
                .thenBy { (_, symbol) -> if (symbol.isExactAtomicTarget()) 0 else 1 }
                .thenBy { it.index },
        )
        ?.value

private fun SymbolInfo.hasValidRange(): Boolean = startLine > 0 && endLine >= startLine

private fun SymbolInfo.rangeLength(): Long = endLine.toLong() - startLine.toLong() + 1

internal fun SymbolInfo.isExactAtomicTarget(): Boolean =
    confidence.equals("exact", ignoreCase = true) && atomicTarget

enum class InspectorAnalysisStatus(val label: String) {
    Missing("Not analyzed"),
    Stale("Stale"),
    Running("Analyzing"),
    Failed("Analysis failed"),
    Fresh("Fresh"),
}

enum class InspectorAnalysisAction(val label: String) {
    AnalyzeFile("Analyze file"),
    RefreshAnalysis("Refresh analysis"),
    CancelAnalysis("Cancel analysis"),
    None(""),
}

data class InspectorProviderState(
    val remoteProvider: Boolean,
    val remoteProviderConfirmed: Boolean,
)

data class SymbolEditEligibility(
    val eligible: Boolean,
    val blockedReason: String = "",
    val target: ChatTarget? = null,
)

data class CurrentEditIdentity(
    val mode: ChatEditMode,
    val targetPath: String,
    val targetSymbol: String,
    val hasDraft: Boolean,
) {
    fun matches(file: ProjectFileInfo?, symbol: SymbolInfo?): Boolean =
        file != null && symbol != null && targetPath == file.path && targetSymbol == symbol.name
}

data class SymbolInspectorSymbolState(
    val symbol: SymbolInfo,
    val signature: String,
    val rangeLabel: String,
    val confidenceLabel: String,
    val explanation: String?,
    val editEligibility: SymbolEditEligibility,
    val currentDraftBoundToAnotherTarget: Boolean,
)

enum class SymbolInspectorMode { FileFallback, SelectedSymbol }

/**
 * Presentation-only inspector data. It consumes existing edit identities and provider
 * confirmation state without adding another persisted target or guard.
 */
data class SymbolInspectorUiState(
    val mode: SymbolInspectorMode,
    val file: ProjectFileInfo,
    val filePurpose: String,
    val analysisStatus: InspectorAnalysisStatus,
    val analysisAction: InspectorAnalysisAction,
    val remoteProviderConfirmationRequired: Boolean,
    val selectionPrompt: String,
    val selectedSymbol: SymbolInspectorSymbolState? = null,
    val currentEditIdentity: CurrentEditIdentity? = null,
)

fun symbolInspectorUiState(
    selectedFile: ProjectFileInfo?,
    symbols: List<SymbolInfo>,
    selectedSymbol: SymbolInfo?,
    analysis: FileAnalysis?,
    analysisInProgress: Boolean,
    provider: InspectorProviderState,
    currentEditIdentity: CurrentEditIdentity?,
): SymbolInspectorUiState? {
    val file = selectedFile ?: return null
    val analysisStatus = inspectorAnalysisStatus(analysis, analysisInProgress)
    val action = analysisStatus.action()
    val symbolState = selectedSymbol?.let { symbol ->
        val editEligibility = symbolEditEligibility(file, symbols, symbol)
        SymbolInspectorSymbolState(
            symbol = symbol,
            signature = symbol.signature,
            rangeLabel = symbolRangeLabel(symbol),
            confidenceLabel = symbolConfidenceLabel(symbol, editEligibility),
            explanation = analysis?.symbolExplanations?.get(symbol.name)?.takeIf { it.isNotBlank() },
            editEligibility = editEligibility,
            currentDraftBoundToAnotherTarget = currentEditIdentity?.hasDraft == true &&
                !currentEditIdentity.matches(file, symbol),
        )
    }
    return SymbolInspectorUiState(
        mode = if (symbolState == null) SymbolInspectorMode.FileFallback else SymbolInspectorMode.SelectedSymbol,
        file = file,
        filePurpose = analysis?.purpose.orEmpty(),
        analysisStatus = analysisStatus,
        analysisAction = action,
        remoteProviderConfirmationRequired = provider.remoteProvider && !provider.remoteProviderConfirmed &&
            action != InspectorAnalysisAction.CancelAnalysis,
        selectionPrompt = "Select a declaration in the editor to inspect it.",
        selectedSymbol = symbolState,
        currentEditIdentity = currentEditIdentity,
    )
}

fun symbolEditEligibility(
    selectedFile: ProjectFileInfo?,
    symbols: List<SymbolInfo>,
    selectedSymbol: SymbolInfo?,
): SymbolEditEligibility {
    val validation = validateChatTarget(selectedFile, symbols, selectedSymbol, ChatEditMode.ReplaceSymbol, "")
    return SymbolEditEligibility(validation.valid, validation.message, validation.target)
}

fun currentEditIdentity(state: DesktopState): CurrentEditIdentity? =
    currentEditIdentity(state.chat.session, state.review.draft, state.selectedFile, state.project)

fun currentEditIdentity(
    session: ChatSession?,
    draft: DeclarationDraft?,
    selectedFile: ProjectFileInfo?,
    project: ProjectAnalysis?,
): CurrentEditIdentity? = currentDraftEditIdentity(draft, selectedFile, project)
    ?: currentSessionEditIdentity(session, selectedFile, project)

private fun currentDraftEditIdentity(
    draft: DeclarationDraft?,
    selectedFile: ProjectFileInfo?,
    project: ProjectAnalysis?,
): CurrentEditIdentity? {
    val file = selectedFile ?: return null
    val activeProject = project ?: return null
    val currentDraft = draft ?: return null
    val mode = currentDraft.mode.toChatEditMode() ?: return null
    if (
        currentDraft.targetPath != file.path || currentDraft.baseFileHash != file.contentHash ||
        currentDraft.projectId != activeProject.projectId || currentDraft.projectRevision != activeProject.projectRevision ||
        currentDraft.targetSymbol.isBlank()
    ) return null
    return CurrentEditIdentity(mode, currentDraft.targetPath, currentDraft.targetSymbol, hasDraft = true)
}

private fun currentSessionEditIdentity(
    session: ChatSession?,
    selectedFile: ProjectFileInfo?,
    project: ProjectAnalysis?,
): CurrentEditIdentity? {
    val currentSession = session ?: return null
    val mode = currentSession.mode.toChatEditMode() ?: return null
    val target = ChatTarget(mode, currentSession.targetSymbol)
    if (target.symbol.isBlank() || !chatSessionMatches(currentSession, selectedFile, project, target)) return null
    return CurrentEditIdentity(mode, currentSession.openPath, currentSession.targetSymbol, hasDraft = false)
}

private fun String.toChatEditMode(): ChatEditMode? =
    ChatEditMode.entries.firstOrNull { it.wireValue == this }

private fun inspectorAnalysisStatus(analysis: FileAnalysis?, analysisInProgress: Boolean): InspectorAnalysisStatus = when {
    analysisInProgress || analysis?.status.equals("running", ignoreCase = true) -> InspectorAnalysisStatus.Running
    analysis == null || analysis.status.equals("missing", ignoreCase = true) -> InspectorAnalysisStatus.Missing
    analysis.status.equals("stale", ignoreCase = true) -> InspectorAnalysisStatus.Stale
    analysis.status.equals("failed", ignoreCase = true) -> InspectorAnalysisStatus.Failed
    analysis.status.equals("fresh", ignoreCase = true) -> InspectorAnalysisStatus.Fresh
    else -> InspectorAnalysisStatus.Missing
}

private fun InspectorAnalysisStatus.action(): InspectorAnalysisAction = when (this) {
    InspectorAnalysisStatus.Missing, InspectorAnalysisStatus.Failed -> InspectorAnalysisAction.AnalyzeFile
    InspectorAnalysisStatus.Stale -> InspectorAnalysisAction.RefreshAnalysis
    InspectorAnalysisStatus.Running -> InspectorAnalysisAction.CancelAnalysis
    InspectorAnalysisStatus.Fresh -> InspectorAnalysisAction.None
}

private fun symbolRangeLabel(symbol: SymbolInfo): String =
    if (symbol.hasValidRange()) "Lines ${symbol.startLine}–${symbol.endLine}" else "Line range unavailable"

private fun symbolConfidenceLabel(symbol: SymbolInfo, eligibility: SymbolEditEligibility): String = when {
    eligibility.eligible -> "Exact atomic target · editable"
    symbol.confidence.equals("exact", ignoreCase = true) -> "Exact indexed declaration · read-only"
    else -> "Approximate indexed declaration · read-only"
}

enum class EditorProgress(val label: String) {
    Inspect("Inspect"),
    Edit("Edit"),
    Review("Review"),
    Receipt("Receipt"),
}

data class EditorProgressUiState(
    val progress: EditorProgress,
    val detail: String,
    val currentEditIdentity: CurrentEditIdentity? = null,
)

/** Derives contextual progress from existing selection, chat, draft, validation, check, and receipt truth. */
fun editorProgressUiState(state: DesktopState): EditorProgressUiState {
    val receipt = state.review.applied?.takeIf { it.undoAvailable }
    if (receipt != null) {
        return EditorProgressUiState(EditorProgress.Receipt, "Change applied. Guarded Undo is available.")
    }

    val identity = currentEditIdentity(state)
    if (identity == null || !identity.matches(state.selectedFile, state.selectedSymbol)) {
        return EditorProgressUiState(EditorProgress.Inspect, "Select a declaration to inspect it.", identity)
    }

    val draft = state.review.draft
    val editor = state.review.editor
    val validationCurrent = editor?.status == DraftEditorStatus.Valid && draft?.validation?.applicable == true &&
        draftEditorMatchesOpenFile(editor, state.selectedFile, state.project)
    return if (validationCurrent) {
        EditorProgressUiState(EditorProgress.Review, "Validation is current for ${identity.targetSymbol}.", identity)
    } else {
        EditorProgressUiState(EditorProgress.Edit, "Editing ${identity.targetSymbol}.", identity)
    }
}
