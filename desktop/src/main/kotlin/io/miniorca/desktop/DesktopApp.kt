package io.miniorca.desktop

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.material.AlertDialog
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import java.io.File
import javax.swing.JFileChooser

internal enum class PaletteMode { Files, Symbols, Actions }

private enum class ComposerFocusTarget { Chat, Draft }

private sealed interface PendingDraftDiscard {
    val currentDraft: CurrentEditIdentity
    val nextLabel: String

    data class Replace(
        val request: DirectEditRequest,
        override val currentDraft: CurrentEditIdentity,
    ) : PendingDraftDiscard {
        override val nextLabel: String = "edit ${request.target.symbol}"
    }

    data class Create(
        override val currentDraft: CurrentEditIdentity,
    ) : PendingDraftDiscard {
        override val nextLabel: String = "create a declaration"
    }
}

@Composable
internal fun MiniOrcaApp(
    api: ApiClient = remember { ApiClient() },
    lastProjectStore: LastProjectStore = remember { LastProjectStore() },
) {
    val scope = rememberCoroutineScope()
    val presenter = remember(api, lastProjectStore, scope) { DesktopWorkflowPresenter(api, lastProjectStore, scope) }
    val workflow by presenter.snapshot.collectAsState()
    val appState = workflow.state
    val widthStore = remember { PaneWidthStore() }
    var paneWidths by remember { mutableStateOf(widthStore.load()) }
    var filter by remember { mutableStateOf("") }
    var collapsedDirectories by remember { mutableStateOf(emptySet<String>()) }
    var contextAction by remember { mutableStateOf("fix") }
    var showContext by remember { mutableStateOf(false) }
    var paletteMode by remember { mutableStateOf(PaletteMode.Files) }
    var paletteQuery by remember { mutableStateOf("") }
    var showPalette by remember { mutableStateOf(false) }
    var chatMode by remember { mutableStateOf(ChatEditMode.ReplaceSymbol) }
    var newChatSymbol by remember { mutableStateOf("") }
    var chatMessage by remember { mutableStateOf("") }
    var pendingImportPath by remember { mutableStateOf<String?>(null) }
    var composerRequested by remember { mutableStateOf(false) }
    var pendingComposerFocus by remember { mutableStateOf<ComposerFocusTarget?>(null) }
    var pendingDraftDiscard by remember { mutableStateOf<PendingDraftDiscard?>(null) }
    val chatFocusRequester = remember { FocusRequester() }
    val draftFocusRequester = remember { FocusRequester() }
    val analyzeModel = workflow.model(ModelScope.Analyze)
    val bugModel = workflow.model(ModelScope.Bug)
    val functionModel = workflow.model(ModelScope.Function)

    val editorProgress = editorProgressUiState(appState)
    LaunchedEffect(editorProgress.progress) {
        if (editorProgress.progress in setOf(EditorProgress.Review, EditorProgress.Receipt)) composerRequested = false
    }
    LaunchedEffect(appState.preparedAction, appState.preparedRequest, appState.selectedSymbol) {
        if (appState.preparedAction.isNotBlank()) contextAction = appState.preparedAction
        if (appState.preparedRequest.isNotBlank()) {
            if (appState.preparedTaskSpec != null) chatMode = ChatEditMode.ReplaceSymbol
            chatMessage = appState.preparedRequest
            composerRequested = true
        }
    }
    LaunchedEffect(appState.project?.projectId, appState.project?.projectRevision) {
        appState.index?.let { collapsedDirectories = explorerDirectories(it.files) }
    }
    LaunchedEffect(appState.review.draft?.id) {
        if (appState.review.draft != null) chatMessage = ""
    }
    LaunchedEffect(workflow.contextManifest) {
        if (workflow.contextManifest != null) showContext = true
    }
    LaunchedEffect(presenter) { presenter.start() }
    DisposableEffect(presenter) { onDispose { presenter.close() } }

    fun focusComposerControl(target: ComposerFocusTarget) {
        composerRequested = true
        pendingComposerFocus = target
    }

    fun startReplaceEdit(request: DirectEditRequest) {
        if (appState.selectedSymbol != request.selectedSymbol) presenter.dispatch(DesktopEvent.SymbolSelected(request.selectedSymbol))
        chatMode = ChatEditMode.ReplaceSymbol
        newChatSymbol = ""
        focusComposerControl(ComposerFocusTarget.Chat)
    }

    fun requestDirectEdit(symbol: SymbolInspectorSymbolState) {
        val request = directEditRequest(appState.selectedFile, appState.symbols, symbol.symbol, currentEditIdentity(appState)) ?: return
        val currentDraft = request.currentDraft
        if (request.requiresDraftDiscard && currentDraft != null) pendingDraftDiscard = PendingDraftDiscard.Replace(request, currentDraft)
        else startReplaceEdit(request)
    }

    fun startCreateDeclaration() {
        chatMode = ChatEditMode.CreateSymbol
        newChatSymbol = ""
        focusComposerControl(ComposerFocusTarget.Chat)
    }

    fun requestCreateDeclaration() {
        val currentDraft = currentEditIdentity(appState)?.takeIf { it.hasDraft }
        if (currentDraft == null) startCreateDeclaration() else pendingDraftDiscard = PendingDraftDiscard.Create(currentDraft)
    }

    fun discardDraftAndContinue() {
        when (val pending = pendingDraftDiscard) {
            is PendingDraftDiscard.Replace -> {
                presenter.discardDraft()
                chatMessage = ""
                pendingDraftDiscard = null
                startReplaceEdit(pending.request)
            }
            is PendingDraftDiscard.Create -> {
                presenter.discardDraft()
                chatMessage = ""
                pendingDraftDiscard = null
                startCreateDeclaration()
            }
            null -> Unit
        }
    }

    fun importProject() {
        val directory = chooseDirectory() ?: return
        if (analyzeModel.remoteProvider && !workflow.providerConfirmed(ModelScope.Analyze)) pendingImportPath = directory.absolutePath
        else presenter.loadProject(directory.absolutePath, restore = false)
    }

    fun openPalette(mode: PaletteMode) {
        paletteMode = mode
        paletteQuery = ""
        showPalette = true
    }

    val explorer: @Composable (Modifier, () -> Unit) -> Unit = { modifier, onSelected ->
        ExplorerPane(
            index = appState.index,
            selectedPath = appState.selectedFile?.path,
            filter = filter,
            collapsedDirectories = collapsedDirectories,
            onFilter = { filter = it },
            onToggleDirectory = { path -> collapsedDirectories = if (path in collapsedDirectories) collapsedDirectories - path else collapsedDirectories + path },
            onSelect = { path ->
                presenter.openFileInEditor(path)
                onSelected()
            },
            loading = appState.loading,
            modifier = modifier,
        )
    }
    val contextPane: @Composable (Modifier) -> Unit = { modifier ->
        if (appState.workspace != Workspace.Editor) {
            SystemStateMessage("Editor context", "Open the Editor workspace to inspect one declaration.", modifier = modifier)
        } else if (editorProgress.progress == EditorProgress.Receipt) {
            ReviewContextPane(
                appState.project, appState.selectedFile, appState.chat.session, appState.review.editor, appState.review.draft, appState.checks,
                appState.impact, appState.gitStatus, appState.review.applied, appState.loading,
                presenter::runDraftChecks, { presenter.reviseWithCheckOutput(chatMode, newChatSymbol) }, { composerRequested = true }, presenter::applyEditableDraft, presenter::undoAppliedDraft, modifier,
            )
        } else if (composerRequested || editorProgress.progress == EditorProgress.Edit) {
            val target = validateChatTarget(appState.selectedFile, appState.symbols, appState.selectedSymbol, chatMode, newChatSymbol).target
            val draft = appState.review.draft
            val draftEditorVisible = draft != null && appState.review.editor != null && chatDraftMatchesSession(draft, appState.chat.session)
            val focusTarget = pendingComposerFocus
            LaunchedEffect(focusTarget, draftEditorVisible) {
                when (focusTarget) {
                    ComposerFocusTarget.Chat -> chatFocusRequester.requestFocus()
                    ComposerFocusTarget.Draft -> if (draftEditorVisible) draftFocusRequester.requestFocus()
                    null -> Unit
                }
                if (pendingComposerFocus == focusTarget) pendingComposerFocus = null
            }
            DraftContextPane(
                appState.project, appState.selectedFile, appState.chat.session, appState.review.draft, appState.review.editor, target, chatMode, newChatSymbol, chatMessage,
                workflow.generating, functionModel, workflow.providerConfirmed(ModelScope.Function), chatFocusRequester, draftFocusRequester,
                { chatMessage = it }, { newChatSymbol = it }, { presenter.setProviderConfirmation(ModelScope.Function, it) },
                { presenter.inspectContext(contextAction) }, { presenter.dispatch(DesktopEvent.DraftEdited(declaration = it)) }, { presenter.dispatch(DesktopEvent.DraftEdited(imports = it)) },
                presenter::validateEditableDraft, { presenter.sendChatMessage(chatMode, newChatSymbol, chatMessage) }, presenter::cancelGeneration, modifier,
            )
        } else if (editorProgress.progress == EditorProgress.Review) {
            ReviewContextPane(
                appState.project, appState.selectedFile, appState.chat.session, appState.review.editor, appState.review.draft, appState.checks,
                appState.impact, appState.gitStatus, appState.review.applied, appState.loading,
                presenter::runDraftChecks, { presenter.reviseWithCheckOutput(chatMode, newChatSymbol) }, { composerRequested = true }, presenter::applyEditableDraft, presenter::undoAppliedDraft, modifier,
            )
        } else {
            SymbolInspectorPane(
                inspector = symbolInspectorUiState(
                    selectedFile = appState.selectedFile,
                    symbols = appState.symbols,
                    selectedSymbol = appState.selectedSymbol,
                    analysis = appState.analysis,
                    analysisInProgress = workflow.analysisInProgress,
                    provider = InspectorProviderState(bugModel.remoteProvider, workflow.providerConfirmed(ModelScope.Bug)),
                    currentEditIdentity = currentEditIdentity(appState),
                ),
                bugModel = bugModel,
                remoteProviderConfirmed = workflow.providerConfirmed(ModelScope.Bug),
                onRemoteProviderConfirmed = { presenter.setProviderConfirmation(ModelScope.Bug, it) },
                onAnalyze = { presenter.analyzeSelected(false) },
                onRefresh = { presenter.analyzeSelected(true) },
                onCancel = presenter::cancelAnalysis,
                onEditSelected = ::requestDirectEdit,
                modifier = modifier,
            )
        }
    }
    val contextualActions = editorContextualActions(
        appState,
        chatMode,
        newChatSymbol,
        chatMessage,
        sending = workflow.generating,
        functionModel = functionModel,
        remoteProviderConfirmed = workflow.providerConfirmed(ModelScope.Function),
    )
    DesktopShell(
        appState = appState,
        paneWidths = paneWidths,
        onPaneWidths = { paneWidths = it },
        onSavePaneWidths = { widthStore.save(paneWidths) },
        connection = appState.connection,
        workspace = appState.workspace,
        onWorkspace = { presenter.dispatch(DesktopEvent.WorkspaceSelected(it)) },
        editorProgress = editorProgress,
        onFocusChat = { focusComposerControl(ComposerFocusTarget.Chat) },
        onFocusDraft = { focusComposerControl(ComposerFocusTarget.Draft) },
        canFocusChat = contextualActions.canFocusChat,
        canFocusDraft = contextualActions.canFocusDraft,
        canGenerate = contextualActions.canGenerate,
        canValidateDraft = contextualActions.canValidateDraft,
        canRunDraftChecks = contextualActions.canRunFocusedChecks,
        analysisInProgress = workflow.analysisInProgress,
        generating = workflow.generating,
        showContext = showContext,
        contextManifest = workflow.contextManifest,
        bugModel = bugModel,
        bugProviderConfirmed = workflow.providerConfirmed(ModelScope.Bug),
        onBugProviderConfirmed = { presenter.setProviderConfirmation(ModelScope.Bug, it) },
        onDismissContext = { showContext = false; presenter.clearContextManifest() },
        paletteMode = paletteMode,
        paletteQuery = paletteQuery,
        showPalette = showPalette,
        onPaletteQuery = { paletteQuery = it },
        onDismissPalette = { showPalette = false },
        onOpenPalette = ::openPalette,
        onSelectPaletteFile = {
            showPalette = false
            presenter.openFileInEditor(it)
        },
        onSelectPaletteSymbol = {
            showPalette = false
            presenter.dispatch(DesktopEvent.SymbolSelected(it))
            presenter.dispatch(DesktopEvent.WorkspaceSelected(Workspace.Editor))
            composerRequested = false
        },
        onSelectPaletteAction = {
            showPalette = false
            presenter.dispatch(DesktopEvent.WorkspaceSelected(Workspace.Editor))
            when (it) {
                "refresh_file_analysis" -> presenter.analyzeSelected(true)
                "create_declaration" -> requestCreateDeclaration()
                else -> contextAction = it
            }
        },
        onOpenFinding = presenter::openFinding,
        onPrepareFinding = presenter::prepareFinding,
        onTriageFinding = presenter::triageFinding,
        onStartAnalyzeAll = presenter::startAnalyzeAll,
        onPauseAnalyzeAll = presenter::pauseAnalyzeAll,
        onResumeAnalyzeAll = presenter::resumeAnalyzeAll,
        onCancelAnalyzeAll = presenter::cancelAnalyzeAll,
        onStartScan = presenter::runVerifiedScan,
        onCancelScan = presenter::cancelVerifiedScan,
        explorer = explorer,
        contextPane = contextPane,
        onImport = ::importProject,
        onReanalyze = presenter::reanalyze,
        onReconnect = presenter::refreshConnection,
        onCancelAnalysis = presenter::cancelAnalysis,
        onSourceLineSelected = { selection -> presenter.dispatch(DesktopEvent.SourceLineSelected(selection)); composerRequested = false },
        onValidateDraft = presenter::validateEditableDraft,
        onRunDraftChecks = presenter::runDraftChecks,
        onGenerate = { presenter.sendChatMessage(chatMode, newChatSymbol, chatMessage) },
        onCancelGeneration = presenter::cancelGeneration,
        onCancelAll = {
            showPalette = false
            showContext = false
            presenter.clearContextManifest()
            presenter.cancelAll()
        },
    )
    pendingDraftDiscard?.let { pending ->
        DraftDiscardDialog(pending, ::discardDraftAndContinue) { pendingDraftDiscard = null }
    }
    pendingImportPath?.let { path ->
        ProjectImportConfirmationDialog(
            model = analyzeModel,
            confirmed = workflow.providerConfirmed(ModelScope.Analyze),
            onConfirmed = { presenter.setProviderConfirmation(ModelScope.Analyze, it) },
            onImport = {
                pendingImportPath = null
                presenter.loadProject(path, restore = false)
            },
            onCancel = { pendingImportPath = null },
        )
    }
}

@Composable
private fun ProjectImportConfirmationDialog(
    model: ScopedModel,
    confirmed: Boolean,
    onConfirmed: (Boolean) -> Unit,
    onImport: () -> Unit,
    onCancel: () -> Unit,
) {
    AlertDialog(
        onDismissRequest = onCancel,
        title = { Text("Confirm project analysis destination") },
        text = {
            Column {
                Text("Import sends the selected project's analysis context to this provider.")
                RemoteProviderConfirmation(ModelScope.Analyze, model, confirmed, onConfirmed)
            }
        },
        confirmButton = { FocusFlowButton(onClick = onImport, enabled = confirmed, tone = ActionTone.Primary) { Text("Import project") } },
        dismissButton = { FocusFlowButton(onClick = onCancel, tone = ActionTone.Neutral) { Text("Cancel") } },
    )
}

@Composable
private fun DraftDiscardDialog(pending: PendingDraftDiscard, onDiscard: () -> Unit, onCancel: () -> Unit) {
    AlertDialog(
        onDismissRequest = onCancel,
        title = { Text("Discard current draft?") },
        text = { Text("Discard the draft for ${pending.currentDraft.targetSymbol} and ${pending.nextLabel}? This only clears the in-memory conversation, draft, and focused checks.") },
        confirmButton = { FocusFlowButton(onClick = onDiscard, tone = ActionTone.Destructive) { Text("Discard draft") } },
        dismissButton = { FocusFlowButton(onClick = onCancel, tone = ActionTone.Neutral) { Text("Keep draft") } },
    )
}

private fun chooseDirectory(): File? {
    val chooser = JFileChooser().apply {
        dialogTitle = "Import project"
        fileSelectionMode = JFileChooser.DIRECTORIES_ONLY
        isAcceptAllFileFilterUsed = false
    }
    return if (chooser.showOpenDialog(null) == JFileChooser.APPROVE_OPTION) chooser.selectedFile else null
}

internal fun staleRemoteConfirmationMessage(error: Throwable, scope: ModelScope): String? =
    (error as? ApiException)
        ?.takeIf { it.message?.contains("confirmation", ignoreCase = true) == true }
        ?.let { "The ${scope.label.lowercase()} model destination changed. Confirm it again before retrying." }
