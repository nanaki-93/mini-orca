package io.miniorca.desktop

/** The deliberate presentation sequence for one file-scoped declaration change. */
enum class EditorStage(val label: String) {
    Target("Target"),
    Draft("Draft"),
    Verify("Verify"),
    Apply("Apply"),
}

data class EditorStageUiState(
    val stage: EditorStage,
    val unlocked: Boolean,
    val reason: String,
)

data class EditorFlowSummary(
    val label: String,
    val detail: String,
)

/**
 * Presentation-only projection of the existing chat, draft, validation, and
 * check guards. The selected stage is transient UI state; all workflow truth
 * remains in [DesktopState].
 */
data class EditorFlowUiState(
    val activeStage: EditorStage,
    val stages: List<EditorStageUiState>,
    val target: EditorFlowSummary,
    val draft: EditorFlowSummary,
    val validation: EditorFlowSummary,
    val checks: EditorFlowSummary,
    val apply: EditorFlowSummary,
) {
    val unlockedStages: Set<EditorStage> get() = stages.filter { it.unlocked }.mapTo(linkedSetOf()) { it.stage }

    fun stage(stage: EditorStage): EditorStageUiState = stages.first { it.stage == stage }
}

fun editorFlowUiState(
    state: DesktopState,
    mode: ChatEditMode,
    requestedSymbol: String,
    activeStage: EditorStage = EditorStage.Target,
): EditorFlowUiState {
    val targetValidation = validateChatTarget(state.selectedFile, state.symbols, state.selectedSymbol, mode, requestedSymbol)
    val target = targetValidation.target
    val session = state.chat.session
    val boundSession = target != null && chatSessionMatches(session, state.selectedFile, state.project, target)
    val draft = state.review.draft
    val editor = state.review.editor
    val currentDraft = boundSession && currentDraftMatchesSession(editor, draft, session)
    val validationCurrent = currentDraft && editor?.status == DraftEditorStatus.Valid && draft?.validation?.applicable == true &&
        draftEditorMatchesOpenFile(editor, state.selectedFile, state.project)
    val checksCurrent = validationCurrent && checksMatchDraft(state.review.checks, draft)
    val reviewEligibility = draftReviewEligibility(editor, draft, state.review.checks, state.selectedFile, state.project)
    val appliedReceipt = state.review.applied?.takeIf { it.undoAvailable }

    val draftUnlocked = targetValidation.valid
    val verifyUnlocked = draftUnlocked && validationCurrent
    val applyUnlocked = appliedReceipt != null || (verifyUnlocked && checksCurrent && reviewEligibility.eligible)
    val stages = listOf(
        EditorStageUiState(EditorStage.Target, true, targetStageReason(targetValidation)),
        EditorStageUiState(EditorStage.Draft, draftUnlocked, draftStageReason(targetValidation, boundSession, currentDraft, draft)),
        EditorStageUiState(EditorStage.Verify, verifyUnlocked, verifyStageReason(draftUnlocked, currentDraft, editor, validationCurrent)),
        EditorStageUiState(EditorStage.Apply, applyUnlocked, applyStageReason(appliedReceipt, currentDraft, checksCurrent, reviewEligibility)),
    )
    val unlockedStages = stages.filter { it.unlocked }.mapTo(linkedSetOf()) { it.stage }

    return EditorFlowUiState(
        activeStage = clampEditorStage(activeStage, unlockedStages),
        stages = stages,
        target = EditorFlowSummary("Target", targetStageReason(targetValidation)),
        draft = EditorFlowSummary("Draft", draftSummary(boundSession, currentDraft, draft)),
        validation = EditorFlowSummary("Validation", validationSummary(editor, validationCurrent)),
        checks = EditorFlowSummary("Focused checks", checksSummary(currentDraft, checksCurrent, state.review.checks, draft)),
        apply = EditorFlowSummary("Apply", applyStageReason(appliedReceipt, currentDraft, checksCurrent, reviewEligibility)),
    )
}

fun clampEditorStage(activeStage: EditorStage, unlockedStages: Set<EditorStage>): EditorStage =
    activeStage.takeIf { it in unlockedStages } ?: unlockedStages.maxByOrNull { it.ordinal } ?: EditorStage.Target

private fun currentDraftMatchesSession(editor: EditableDraftState?, draft: DeclarationDraft?, session: ChatSession?): Boolean =
    editor != null && draft != null && chatDraftMatchesSession(draft, session) &&
        editor.serverDraft.id == draft.id && editor.serverDraft.revision == draft.revision && editor.serverDraft.hash == draft.hash

internal fun checksMatchDraft(checks: CandidateCheckReport?, draft: DeclarationDraft?): Boolean =
    checks?.applicable == true && draft != null && checks.draftId == draft.id &&
        checks.draftRevision == draft.revision && checks.draftHash == draft.hash

private fun targetStageReason(validation: ChatTargetValidation): String =
    validation.target?.let { "${it.symbol} is ready for a file-scoped declaration conversation." }
        ?: validation.message

private fun draftStageReason(
    target: ChatTargetValidation,
    boundSession: Boolean,
    currentDraft: Boolean,
    draft: DeclarationDraft?,
): String = when {
    !target.valid -> "Choose a valid target before opening a draft."
    !boundSession -> "Draft is ready for the first message bound to this target."
    currentDraft -> "Latest editable draft: ${draft?.targetSymbol}."
    else -> "The current conversation has no loaded latest editable draft."
}

private fun verifyStageReason(
    draftUnlocked: Boolean,
    currentDraft: Boolean,
    editor: EditableDraftState?,
    validationCurrent: Boolean,
): String = when {
    !draftUnlocked -> "Start a file-scoped conversation before verification."
    !currentDraft -> "Load the latest editable draft for this conversation."
    validationCurrent -> "Validation is current for this draft."
    else -> validationSummary(editor, false)
}

private fun applyStageReason(
    appliedReceipt: ApplyResult?,
    currentDraft: Boolean,
    checksCurrent: Boolean,
    eligibility: ApplyEligibility,
): String = when {
    appliedReceipt != null -> "Change applied. Guarded Undo is available."
    !currentDraft -> "Load the latest editable draft before applying it."
    !checksCurrent -> "Run focused checks for the latest draft before applying it."
    else -> eligibility.reason
}

private fun draftSummary(boundSession: Boolean, currentDraft: Boolean, draft: DeclarationDraft?): String = when {
    !boundSession -> "No conversation is currently bound to this target."
    currentDraft -> "${draft?.targetPath} · ${draft?.targetSymbol}."
    else -> "A bound conversation is ready for its next draft."
}

internal fun validationSummary(editor: EditableDraftState?, validationCurrent: Boolean): String = when {
    validationCurrent -> "Validated for the latest draft."
    editor == null -> "No editable draft is loaded."
    editor.status == DraftEditorStatus.Dirty -> "Manual edits require validation and fresh checks."
    editor.status == DraftEditorStatus.Validating -> "Validation is running."
    editor.status == DraftEditorStatus.Invalid -> "Fix validation diagnostics before continuing."
    editor.status == DraftEditorStatus.Stale -> "The draft is stale; start a new file-scoped conversation."
    else -> "Validate the latest declaration draft before continuing."
}

private fun checksSummary(
    currentDraft: Boolean,
    checksCurrent: Boolean,
    checks: CandidateCheckReport?,
    draft: DeclarationDraft?,
): String = when {
    !currentDraft -> "No current draft is available for focused checks."
    checksCurrent -> "Focused checks are current."
    checks != null -> "Focused checks do not match the latest draft."
    else -> "Run focused checks after validating the latest draft."
}
