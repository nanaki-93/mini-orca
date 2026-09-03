package io.miniorca.desktop

import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.material.AlertDialog
import androidx.compose.material.Text
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.launch
import kotlinx.coroutines.runInterruptible
import kotlinx.coroutines.withContext
import java.io.File
import javax.swing.JFileChooser

internal enum class PaletteMode { Files, Symbols, Actions }

private enum class ComposerFocusTarget { Chat, Draft }

private data class ProjectWorkspaceDetails(
    val overview: ProjectOverview,
    val findings: FindingsResponse,
    val analyzeAll: AnalyzeAllJob?,
    val scan: GoScanReport?,
)

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
    val widthStore = remember { PaneWidthStore() }
    val workflow = remember { DesktopWorkflowController() }
    var appState by remember { mutableStateOf(DesktopState()) }
    var paneWidths by remember { mutableStateOf(widthStore.load()) }
    var filter by remember { mutableStateOf("") }
    var collapsedDirectories by remember { mutableStateOf(emptySet<String>()) }
    var analysisJob by remember { mutableStateOf<Job?>(null) }
    var analyzeAllPollJob by remember { mutableStateOf<Job?>(null) }
    var scanPollJob by remember { mutableStateOf<Job?>(null) }
    val analyzeAllPolling = remember { AnalyzeAllPollingController() }
    var analysisRequestId by remember { mutableStateOf(0) }
    var chatJob by remember { mutableStateOf<Job?>(null) }
    var draftValidationJob by remember { mutableStateOf<Job?>(null) }
    var contextAction by remember { mutableStateOf("fix") }
    var contextManifest by remember { mutableStateOf<ContextManifest?>(null) }
    var showContext by remember { mutableStateOf(false) }
    var paletteMode by remember { mutableStateOf(PaletteMode.Files) }
    var paletteQuery by remember { mutableStateOf("") }
    var showPalette by remember { mutableStateOf(false) }
    var chatMode by remember { mutableStateOf(ChatEditMode.ReplaceSymbol) }
    var newChatSymbol by remember { mutableStateOf("") }
    var chatMessage by remember { mutableStateOf("") }
    var modelCatalog by remember { mutableStateOf(ModelCatalog()) }
    var providerConfirmations by remember { mutableStateOf(ScopedConfirmationState()) }
    var pendingImportPath by remember { mutableStateOf<String?>(null) }
    var composerRequested by remember { mutableStateOf(false) }
    var pendingComposerFocus by remember { mutableStateOf<ComposerFocusTarget?>(null) }
    var pendingDraftDiscard by remember { mutableStateOf<PendingDraftDiscard?>(null) }
    val chatFocusRequester = remember { FocusRequester() }
    val draftFocusRequester = remember { FocusRequester() }
    val analyzeModel = modelCatalog.forScope(ModelScope.Analyze)
    val bugModel = modelCatalog.forScope(ModelScope.Bug)
    val functionModel = modelCatalog.forScope(ModelScope.Function)

    val editorProgress = editorProgressUiState(appState)
    LaunchedEffect(editorProgress.progress) {
        if (editorProgress.progress in setOf(EditorProgress.Review, EditorProgress.Receipt)) composerRequested = false
    }

    fun update(event: DesktopEvent) {
        appState = workflow.dispatch(event)
    }

    fun modelRequestFailed(error: Throwable, scope: ModelScope, fallback: String) {
        val staleConfirmation = staleRemoteConfirmationMessage(error, scope)
        if (staleConfirmation != null) providerConfirmations = providerConfirmations.withConfirmation(scope, false)
        update(DesktopEvent.Failed(staleConfirmation ?: error.message ?: fallback))
    }

    fun focusComposerControl(target: ComposerFocusTarget) {
        composerRequested = true
        pendingComposerFocus = target
    }

    fun selectSourceLine(selection: SourceLineSelection) {
        update(DesktopEvent.SourceLineSelected(selection))
        composerRequested = false
    }

    fun startReplaceEdit(request: DirectEditRequest) {
        if (appState.selectedSymbol != request.selectedSymbol) update(DesktopEvent.SymbolSelected(request.selectedSymbol))
        chatMode = ChatEditMode.ReplaceSymbol
        newChatSymbol = ""
        focusComposerControl(ComposerFocusTarget.Chat)
    }

    fun requestDirectEdit(symbol: SymbolInspectorSymbolState) {
        val request = directEditRequest(appState.selectedFile, appState.symbols, symbol.symbol, currentEditIdentity(appState)) ?: return
        val currentDraft = request.currentDraft
        if (request.requiresDraftDiscard && currentDraft != null) {
            pendingDraftDiscard = PendingDraftDiscard.Replace(request, currentDraft)
        } else {
            startReplaceEdit(request)
        }
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
                chatJob?.cancel()
                draftValidationJob?.cancel()
                update(DesktopEvent.DraftDiscarded)
                chatMessage = ""
                pendingDraftDiscard = null
                startReplaceEdit(pending.request)
            }
            is PendingDraftDiscard.Create -> {
                chatJob?.cancel()
                draftValidationJob?.cancel()
                update(DesktopEvent.DraftDiscarded)
                chatMessage = ""
                pendingDraftDiscard = null
                startCreateDeclaration()
            }
            null -> Unit
        }
    }

    fun selectPaletteSymbol(symbol: SymbolInfo) {
        update(DesktopEvent.SymbolSelected(symbol))
        update(DesktopEvent.WorkspaceSelected(Workspace.Editor))
        composerRequested = false
    }

    DisposableEffect(Unit) {
        onDispose {
            analyzeAllPolling.dispose()
            analyzeAllPollJob?.cancel()
            scanPollJob?.cancel()
        }
    }

    LaunchedEffect(appState.preparedAction, appState.preparedRequest, appState.selectedSymbol) {
        if (appState.preparedAction.isNotBlank()) contextAction = appState.preparedAction
        if (appState.preparedRequest.isNotBlank()) {
            if (appState.preparedTaskSpec != null) chatMode = ChatEditMode.ReplaceSymbol
            chatMessage = appState.preparedRequest
            composerRequested = true
        }
    }
    fun refreshConnection() {
        scope.launch {
            val startedAt = System.nanoTime()
            runCatching { withContext(Dispatchers.IO) { api.status() to api.modelCatalog() } }
                .onSuccess { (status, catalog) ->
                    val elapsed = (System.nanoTime() - startedAt) / 1_000_000
                    if (modelCatalog.identity() != catalog.identity()) providerConfirmations = ScopedConfirmationState()
                    modelCatalog = catalog
                    val function = catalog.forScope(ModelScope.Function)
                    update(DesktopEvent.ConnectionUpdated(ConnectionState("Daemon connected", "${function.profile} · ${function.model}", status.version, true, api.endpointLocality(), "${elapsed}ms")))
                }
                .onFailure { update(DesktopEvent.ConnectionUpdated(ConnectionState(label = "Daemon unavailable", locality = api.endpointLocality()))) }
        }
    }

    fun openPalette(mode: PaletteMode) {
        paletteMode = mode
        paletteQuery = ""
        showPalette = true
    }
    fun refreshAnalysisFreshness(revision: String) {
        scope.launch {
            runCatching { withContext(Dispatchers.IO) { api.index() } }
                .onSuccess { index ->
                    if (appState.project?.projectRevision == revision && index.projectRevision == revision) {
                        update(DesktopEvent.IndexRefreshed(index))
                    }
                }
        }
    }
    fun refreshFindings(revision: String) {
        scope.launch {
            runCatching { withContext(Dispatchers.IO) { api.findings(revision) } }
                .onSuccess { response ->
                    if (appState.project?.projectRevision == revision) update(DesktopEvent.FindingsLoaded(response.findings))
                }
        }
    }
    fun pollVerifiedScan(revision: String) {
        scanPollJob?.cancel()
        scanPollJob = scope.launch {
            while (appState.project?.projectRevision == revision) {
                val response = runCatching { withContext(Dispatchers.IO) { api.goScan(revision) } }
                if (response.isFailure) {
                    update(DesktopEvent.Failed(response.exceptionOrNull()?.message ?: "Verified scan status failed"))
                    return@launch
                }
                val scan = response.getOrNull()
                update(DesktopEvent.GoScanLoaded(scan))
                if (!shouldPollVerifiedScan(scan)) {
                    refreshFindings(revision)
                    return@launch
                }
                kotlinx.coroutines.delay(750)
            }
        }
    }
    lateinit var publishAnalyzeAll: (String, AnalyzeAllJob?) -> Unit
    fun pollAnalyzeAll(revision: String) {
        analyzeAllPollJob?.cancel()
        analyzeAllPollJob = scope.launch {
            while (analyzeAllPolling.shouldPoll(revision)) {
                val response = runCatching { withContext(Dispatchers.IO) { api.analyzeAllJob(revision) } }
                if (response.isFailure) {
                    if (appState.project?.projectRevision == revision) update(DesktopEvent.Failed(response.exceptionOrNull()?.message ?: "Analyze-all status failed"))
                    return@launch
                }
                val job = response.getOrNull()
                if (appState.project?.projectRevision != revision) return@launch
                publishAnalyzeAll(revision, job)
                if (!analyzeAllPolling.shouldPoll(revision)) break
                kotlinx.coroutines.delay(750)
            }
        }
    }
    publishAnalyzeAll = { revision, job ->
        val accepted = analyzeAllPolling.receive(revision, job)
        if (accepted == null) {
            if (job == null && appState.project?.projectRevision == revision) update(DesktopEvent.AnalyzeAllLoaded(null))
        } else if (appState.project?.projectRevision == revision) {
            update(DesktopEvent.AnalyzeAllLoaded(accepted))
            refreshAnalysisFreshness(revision)
            if (analyzeAllPolling.shouldPoll(revision) && analyzeAllPollJob?.isActive != true) pollAnalyzeAll(revision)
        }
    }
    fun refreshProjectWorkspace(revision: String) {
        scope.launch {
            runCatching {
                withContext(Dispatchers.IO) {
                    ProjectWorkspaceDetails(api.overview(revision), api.findings(revision), api.analyzeAllJob(revision), api.goScan(revision))
                }
            }.onSuccess { details ->
                    if (appState.project?.projectRevision == revision) {
                        update(DesktopEvent.OverviewLoaded(details.overview))
                        update(DesktopEvent.FindingsLoaded(details.findings.findings))
                        update(DesktopEvent.GoScanLoaded(details.scan))
                        publishAnalyzeAll(revision, details.analyzeAll)
                        if (shouldPollVerifiedScan(details.scan)) pollVerifiedScan(revision)
                    }
                }
                .onFailure { update(DesktopEvent.Status("Project facts are available; workspace details could not be refreshed.")) }
        }
    }
    fun startAnalyzeAll(options: AnalyzeAllRunOptions) {
        val project = appState.project ?: return
        analyzeAllPolling.activate(project.projectRevision)
        scope.launch { runCatching { withContext(Dispatchers.IO) { api.startAnalyzeAll(project.projectRevision, options.maxFiles, options.maxRetries, options.confirmRemoteProvider) } }
            .onSuccess { publishAnalyzeAll(project.projectRevision, it) }
            .onFailure { modelRequestFailed(it, ModelScope.Bug, "Unable to start Analyze-all") } }
    }
    fun pauseAnalyzeAll() {
        val project = appState.project ?: return
        scope.launch { runCatching { withContext(Dispatchers.IO) { api.pauseAnalyzeAll(project.projectRevision) } }
            .onSuccess { publishAnalyzeAll(project.projectRevision, it) }
            .onFailure { update(DesktopEvent.Failed(it.message ?: "Unable to update Analyze-all")) } }
    }
    fun resumeAnalyzeAll(confirmRemoteProvider: Boolean) {
        val project = appState.project ?: return
        analyzeAllPolling.activate(project.projectRevision)
        scope.launch { runCatching { withContext(Dispatchers.IO) { api.resumeAnalyzeAll(project.projectRevision, confirmRemoteProvider) } }
            .onSuccess { publishAnalyzeAll(project.projectRevision, it) }
            .onFailure { modelRequestFailed(it, ModelScope.Bug, "Unable to update Analyze-all") } }
    }
    fun cancelAnalyzeAll() {
        val project = appState.project ?: return
        scope.launch { runCatching { withContext(Dispatchers.IO) { api.cancelAnalyzeAll(project.projectRevision) } }
            .onSuccess { publishAnalyzeAll(project.projectRevision, it) }
            .onFailure { update(DesktopEvent.Failed(it.message ?: "Unable to update Analyze-all")) } }
    }
    fun runVerifiedScan() {
        val project = appState.project ?: return
        scope.launch { runCatching { withContext(Dispatchers.IO) { api.startGoScan(project.projectRevision) } }
            .onSuccess { update(DesktopEvent.GoScanLoaded(it)); pollVerifiedScan(project.projectRevision) }
            .onFailure { update(DesktopEvent.Failed(it.message ?: "Unable to start verified scan")) } }
    }
    fun cancelVerifiedScan() {
        val project = appState.project ?: return
        scope.launch { runCatching { withContext(Dispatchers.IO) { api.cancelGoScan(project.projectRevision) } }
            .onSuccess { update(DesktopEvent.GoScanLoaded(it)); scanPollJob?.cancel(); refreshFindings(project.projectRevision) }
            .onFailure { update(DesktopEvent.Failed(it.message ?: "Unable to cancel verified scan")) } }
    }
    fun loadProject(path: String, restore: Boolean) {
        if (appState.loading) return
        analyzeAllPolling.stop()
        analyzeAllPollJob?.cancel()
        scanPollJob?.cancel()
        val requestId = workflow.beginProjectLoad()
        appState = workflow.state
        val projectName = File(path).name
        update(DesktopEvent.Status(if (restore) "Reopening $projectName…" else "Importing $projectName…"))
        scope.launch {
            runCatching {
                withContext(Dispatchers.IO) {
                    val project = if (restore) api.restoreProject(path) else api.importProject(path, providerConfirmations.confirmed(ModelScope.Analyze))
                    project to api.index()
                }
            }
                .onSuccess { (project, index) ->
                    if (workflow.projectLoaded(requestId, project, index)) {
                        appState = workflow.state
                        runCatching { lastProjectStore.save(project.path) }
                        if (restore) update(DesktopEvent.Status("Reopened ${project.name}"))
                        analyzeAllPolling.activate(project.projectRevision)
                        collapsedDirectories = explorerDirectories(index.files)
                        refreshProjectWorkspace(project.projectRevision)
                    }
                }
                .onFailure {
                    if (workflow.isCurrentProjectRequest(requestId)) {
                        if (restore) update(DesktopEvent.Failed(it.message ?: "Could not reopen the last project"))
                        else modelRequestFailed(it, ModelScope.Analyze, "Import failed")
                    }
                }
        }
    }
    fun importProject() {
        val directory = chooseDirectory() ?: return
        if (analyzeModel.remoteProvider && !providerConfirmations.confirmed(ModelScope.Analyze)) {
            pendingImportPath = directory.absolutePath
        } else {
            loadProject(directory.absolutePath, restore = false)
        }
    }
    fun reanalyze() {
        val project = appState.project ?: return
        analyzeAllPolling.stop()
        analyzeAllPollJob?.cancel()
        scanPollJob?.cancel()
        update(DesktopEvent.Loading)
        update(DesktopEvent.Status("Refreshing deterministic project facts…"))
        scope.launch {
            runCatching { withContext(Dispatchers.IO) { api.reindex(project.projectRevision) } }
                .onSuccess {
                    update(DesktopEvent.IndexRefreshed(it))
                    analyzeAllPolling.activate(it.projectRevision)
                    refreshProjectWorkspace(it.projectRevision)
                }
                .onFailure { update(DesktopEvent.Failed(it.message ?: "Re-analysis failed")) }
        }
    }
    fun selectFile(
        path: String,
        editorTarget: EditorNavigationTarget? = null,
        preparedFixRequest: String? = null,
        preparedTaskSpec: BugTaskSpec? = null,
    ) {
        composerRequested = false
        chatJob?.cancel()
        draftValidationJob?.cancel()
        workflow.synchronize(appState)
        val request = workflow.beginFileLoad(path) ?: return
        appState = workflow.state
        scope.launch {
            runCatching { withContext(Dispatchers.IO) { api.fileInfo(path) to api.symbols(path).symbols } }
                .onSuccess { (file, symbols) ->
                    if (!workflow.fileLoaded(request, file, symbols)) return@onSuccess
                    appState = workflow.state
                    val navigation = editorTarget?.let { target ->
                        resolveEditorNavigation(symbols, target).also { selection ->
                            update(DesktopEvent.EditorContextSelected(selection.symbol, selection.focusLine))
                        }
                    }
                    preparedFixRequest?.let { request ->
                        update(DesktopEvent.SuggestionPrepared("fix", request, navigation?.symbol, preparedTaskSpec))
                    }
                    val loadedRequest = workflow.currentFileRequest() ?: return@onSuccess
                    scope.launch {
                        runCatching { withContext(Dispatchers.IO) { api.analysis(path, loadedRequest.projectRevision) } }
                            .onSuccess { if (workflow.analysisLoaded(loadedRequest, it)) appState = workflow.state }
                            .onFailure { workflow.optionalLoadFailed(loadedRequest) }
                    }
                    scope.launch {
                        runCatching { withContext(Dispatchers.IO) { api.impact(path) } }
                            .onSuccess { if (workflow.impactLoaded(loadedRequest, it)) appState = workflow.state }
                            .onFailure { workflow.optionalLoadFailed(loadedRequest) }
                    }
                    scope.launch {
                        runCatching { withContext(Dispatchers.IO) { api.gitStatus(path) } }
                            .onSuccess { if (workflow.gitStatusLoaded(loadedRequest, it)) appState = workflow.state }
                            .onFailure { workflow.optionalLoadFailed(loadedRequest) }
                    }
                }
            .onFailure { if (workflow.fileFailed(request, it.message ?: "File load failed")) appState = workflow.state }
        }
    }
    fun openFileInEditor(
        path: String,
        editorTarget: EditorNavigationTarget? = null,
        preparedFixRequest: String? = null,
        preparedTaskSpec: BugTaskSpec? = null,
    ) {
        if (appState.index?.files?.any { it.path == path } != true) {
            update(DesktopEvent.Failed("This file no longer points to an indexed file in the active project."))
            return
        }
        update(DesktopEvent.WorkspaceSelected(fileInspectionWorkspace()))
        selectFile(path, editorTarget, preparedFixRequest, preparedTaskSpec)
    }
    fun openFinding(finding: UnifiedFinding) {
        val target = findingNavigationTarget(finding, appState.index)
        if (target == null) {
            update(DesktopEvent.Failed("This finding no longer points to a file in the active project."))
            return
        }
        openFileInEditor(target.path, target)
    }
    fun prepareFinding(finding: UnifiedFinding) {
        if (!findingCanPrepareFix(finding)) { update(DesktopEvent.Failed("Refresh this finding before preparing a fix.")); return }
        val target = findingTaskNavigationTarget(finding)
        val requirement = findingTaskRequirement(finding)
        if (target == null || requirement == null || appState.index?.files?.any { it.path == target.path } != true) {
            update(DesktopEvent.Failed("This finding no longer points to a file in the active project."))
            return
        }
        openFileInEditor(target.path, target, requirement, finding.taskSpec)
    }
    fun triageFinding(finding: UnifiedFinding, action: FindingLifecycleAction) {
        val project = appState.project ?: return
        if (finding.projectRevision.isNotBlank() && finding.projectRevision != project.projectRevision) {
            update(DesktopEvent.Failed("Refresh findings before changing triage for out-of-date results."))
            return
        }
        scope.launch {
            runCatching { withContext(Dispatchers.IO) { api.updateFindingStatus(finding.id, project.projectRevision, action.status) } }
                .onSuccess {
                    update(DesktopEvent.FindingStatusUpdated(finding.id, action.status))
                    refreshFindings(project.projectRevision)
                }
                .onFailure { update(DesktopEvent.Failed(it.message ?: "Unable to update finding triage")) }
        }
    }
    fun analyzeSelected(refresh: Boolean) {
        val project = appState.project ?: return
        val file = appState.selectedFile ?: return
        if (bugModel.remoteProvider && !providerConfirmations.confirmed(ModelScope.Bug)) {
            update(DesktopEvent.Failed("Confirm the Bugs model destination before analyzing this file."))
            return
        }
        analysisJob?.cancel()
        val (requestId, requestIdentity) = workflow.beginAnalysis() ?: return
        analysisRequestId = requestId.toInt()
        update(DesktopEvent.Loading)
        update(DesktopEvent.Status("${if (refresh) "Refreshing" else "Analyzing"} ${file.path}…"))
        analysisJob = scope.launch {
            try {
                val analysis = withContext(Dispatchers.IO) { runInterruptible { api.analyze(file.path, project.projectRevision, refresh, providerConfirmations.confirmed(ModelScope.Bug)) } }
                if (workflow.analysisCompleted(requestId, requestIdentity, analysis)) {
                    appState = workflow.state
                    update(DesktopEvent.Status("Summary ${analysis.status}"))
                }
            } catch (_: CancellationException) {
                update(DesktopEvent.Status("Analysis canceled"))
            } catch (error: Throwable) {
                modelRequestFailed(error, ModelScope.Bug, "Analysis failed")
            } finally {
                if (analysisRequestId == requestId.toInt()) analysisJob = null
            }
        }
    }

    fun inspectContext() {
        val file = appState.selectedFile ?: return
        scope.launch {
            runCatching { withContext(Dispatchers.IO) { runInterruptible { api.context(file.path, contextAction) } } }
                .onSuccess {
                    contextManifest = it
                    showContext = true
                }
                .onFailure { update(DesktopEvent.Failed(it.message ?: "Context preview failed")) }
        }
    }

    fun sendChatMessage(repair: Boolean = false, repairMessage: String? = null) {
        val project = appState.project ?: return
        val file = appState.selectedFile ?: return
        val target = validateChatTarget(file, appState.symbols, appState.selectedSymbol, chatMode, newChatSymbol)
        if (!target.valid) {
            update(DesktopEvent.Failed(target.message))
            return
        }
        val message = (repairMessage ?: chatMessage).trim()
        if (message.isBlank()) {
            update(DesktopEvent.Failed("Write a message before sending."))
            return
        }
        if (functionModel.remoteProvider && !providerConfirmations.confirmed(ModelScope.Function)) {
            update(DesktopEvent.Failed("Confirm the Function edits model destination before sending context."))
            return
        }
        val (requestId, requestIdentity) = workflow.beginChatLoad() ?: return
        val targetIdentity = target.target!!
        val taskSpec = appState.preparedTaskSpec?.takeIf {
            it.targetPath == file.path && it.targetSymbol == targetIdentity.symbol && targetIdentity.mode == ChatEditMode.ReplaceSymbol
        }
        val matchingSession = appState.chat.session?.takeIf { chatSessionMatches(it, file, project, targetIdentity, taskSpec) }
        update(DesktopEvent.Loading)
        update(DesktopEvent.Status("Sending a request for ${targetIdentity.symbol} in ${file.path}…"))
        chatJob = scope.launch {
            try {
                val session = matchingSession ?: withContext(Dispatchers.IO) {
                    runInterruptible {
                        api.openChatSession(project.projectId, project.projectRevision, file.contentHash, file.path, targetIdentity.mode.wireValue, targetIdentity.symbol, taskSpec)
                    }
                }
                val proposal = withContext(Dispatchers.IO) {
                    runInterruptible {
                        api.sendChatMessage(session.id, message, session.latestDraftId, providerConfirmations.confirmed(ModelScope.Function), repair)
                    }
                }
                val updatedSession = session.copy(repairCount = session.repairCount + if (repair) 1 else 0)
                if (workflow.chatProposalLoaded(requestId, requestIdentity, updatedSession, message, proposal)) {
                    appState = workflow.state
                    chatMessage = ""
                    update(DesktopEvent.Status("Draft is ready for review."))
                }
            } catch (_: CancellationException) {
                if (workflow.cancelChatLoad(requestId, requestIdentity)) appState = workflow.state
            } catch (error: Throwable) {
                if (workflow.cancelChatLoad(requestId, requestIdentity)) appState = workflow.state
                modelRequestFailed(error, ModelScope.Function, "Chat request failed")
            } finally {
                chatJob = null
            }
        }
    }

    fun reviseWithCheckOutput() {
        val message = repairMessageForChecks(appState.chat.session, appState.review.draft, appState.checks) ?: return
        sendChatMessage(repair = true, repairMessage = message)
    }

    fun validateEditableDraft() {
        val editor = appState.review.editor ?: return
        val project = appState.project ?: return
        val file = appState.selectedFile ?: return
        if (!draftEditorMatchesOpenFile(editor, file, project)) {
            update(DesktopEvent.DraftMarkedStale)
            update(DesktopEvent.Failed("The draft no longer matches the open file."))
            return
        }
        val (requestId, requestIdentity) = workflow.beginDraftLoad() ?: return
        update(DesktopEvent.DraftValidationStarted)
        update(DesktopEvent.Status("Validating ${editor.serverDraft.targetSymbol}…"))
        draftValidationJob = scope.launch {
            try {
                val updated = withContext(Dispatchers.IO) {
                    runInterruptible {
                        api.updateDraft(editor.serverDraft.id, project.projectRevision, editor.serverDraft.revision, editor.declaration, editor.imports)
                    }
                }
                val validated = withContext(Dispatchers.IO) {
                    runInterruptible { api.validateDraft(updated.id, project.projectRevision, updated.revision) }
                }
                if (workflow.draftLoaded(requestId, requestIdentity, validated)) {
                    appState = workflow.state
                    update(DesktopEvent.Status(if (validated.validation?.applicable == true) "Draft validation passed." else "Draft validation needs attention."))
                }
            } catch (_: CancellationException) {
                update(DesktopEvent.Status("Draft validation canceled"))
            } catch (error: Throwable) {
                if (error is ApiException && error.status == 409) update(DesktopEvent.DraftMarkedStale)
                update(DesktopEvent.Failed(error.message ?: "Draft validation failed"))
            } finally {
                draftValidationJob = null
            }
        }
    }

    fun runDraftChecks() {
        val draft = appState.review.draft ?: return
        val editor = appState.review.editor
        val project = appState.project ?: return
        val file = appState.selectedFile ?: return
        val eligibility = draftReviewEligibility(editor, draft, null, file, project)
        if (editor?.status != DraftEditorStatus.Valid || !draftEditorMatchesOpenFile(editor, file, project)) {
            update(DesktopEvent.Failed(eligibility.reason))
            return
        }
        val (requestId, requestIdentity) = workflow.beginDraftLoad() ?: return
        update(DesktopEvent.Loading)
        update(DesktopEvent.Status("Running focused checks for ${draft.targetSymbol}…"))
        scope.launch {
            runCatching { withContext(Dispatchers.IO) { api.checkDraft(draft.id, project.projectRevision, draft.revision, draft.hash) } }
                .onSuccess { checks ->
                    if (workflow.draftChecksLoaded(requestId, requestIdentity, draft, checks)) {
                        appState = workflow.state
                        update(DesktopEvent.Status(if (checks.applicable) "Focused checks passed." else "Focused checks need attention."))
                    }
                }
                .onFailure { error ->
                    if (error is ApiException && error.status == 409) update(DesktopEvent.DraftMarkedStale)
                    update(DesktopEvent.Failed(error.message ?: "Focused checks failed"))
                }
        }
    }

    fun reloadAfterMutation(path: String, revision: String) {
        scope.launch {
            runCatching { withContext(Dispatchers.IO) { Triple(api.index(), api.fileInfo(path), api.symbols(path).symbols) } }
                .onSuccess { (index, file, symbols) ->
                    update(DesktopEvent.IndexRefreshed(index))
                    update(DesktopEvent.FileLoaded(file, symbols))
                    update(DesktopEvent.Status("Project refreshed."))
                    refreshProjectWorkspace(revision)
                }
                .onFailure { update(DesktopEvent.Failed(it.message ?: "Project refresh failed")) }
        }
    }

    fun applyEditableDraft() {
        val draft = appState.review.draft ?: return
        val eligibility = draftReviewEligibility(appState.review.editor, draft, appState.checks, appState.selectedFile, appState.project)
        if (!eligibility.eligible) {
            update(DesktopEvent.Failed(eligibility.reason))
            return
        }
        update(DesktopEvent.Loading)
        update(DesktopEvent.Status("Applying reviewed declaration draft…"))
        scope.launch {
            runCatching { withContext(Dispatchers.IO) { api.applyDraft(draft) } }
                .onSuccess { result ->
                    update(DesktopEvent.Applied(result))
                    update(DesktopEvent.Status("Applied ${draft.targetPath}; Undo is available."))
                    reloadAfterMutation(draft.targetPath, result.projectRevision)
                }
                .onFailure { update(DesktopEvent.Failed(it.message ?: "Apply failed")) }
        }
    }

    fun undoAppliedDraft() {
        val project = appState.project ?: return
        val result = appState.review.applied ?: return
        val path = appState.selectedFile?.path ?: return
        update(DesktopEvent.Loading)
        update(DesktopEvent.Status("Undoing the applied declaration draft…"))
        scope.launch {
            runCatching { withContext(Dispatchers.IO) { api.undo(project.projectId, result.projectRevision, result.postApplyHash) } }
                .onSuccess { undo ->
                    update(DesktopEvent.Applied(undo))
                    reloadAfterMutation(path, undo.projectRevision)
                }
                .onFailure { update(DesktopEvent.Failed(it.message ?: "Undo failed")) }
        }
    }
    LaunchedEffect(api, lastProjectStore) {
        refreshConnection()
        lastProjectStore.load()?.let { loadProject(it, restore = true) }
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
                openFileInEditor(path)
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
                ::runDraftChecks, ::reviseWithCheckOutput, { composerRequested = true }, ::applyEditableDraft, ::undoAppliedDraft, modifier,
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
            DraftContextPane(appState.project, appState.selectedFile, appState.chat.session, appState.review.draft, appState.review.editor, target, chatMode, newChatSymbol, chatMessage, chatJob != null, functionModel, providerConfirmations.confirmed(ModelScope.Function), chatFocusRequester, draftFocusRequester, { chatMessage = it }, { newChatSymbol = it }, { providerConfirmations = providerConfirmations.withConfirmation(ModelScope.Function, it) }, ::inspectContext, { update(DesktopEvent.DraftEdited(declaration = it)) }, { update(DesktopEvent.DraftEdited(imports = it)) }, ::validateEditableDraft, ::sendChatMessage, { chatJob?.cancel() }, modifier)
        } else if (editorProgress.progress == EditorProgress.Review) {
            ReviewContextPane(
                appState.project, appState.selectedFile, appState.chat.session, appState.review.editor, appState.review.draft, appState.checks,
                appState.impact, appState.gitStatus, appState.review.applied, appState.loading,
                ::runDraftChecks, ::reviseWithCheckOutput, { composerRequested = true }, ::applyEditableDraft, ::undoAppliedDraft, modifier,
            )
        } else {
            SymbolInspectorPane(
                inspector = symbolInspectorUiState(
                    selectedFile = appState.selectedFile,
                    symbols = appState.symbols,
                    selectedSymbol = appState.selectedSymbol,
                    analysis = appState.analysis,
                    analysisInProgress = analysisJob != null,
                    provider = InspectorProviderState(bugModel.remoteProvider, providerConfirmations.confirmed(ModelScope.Bug)),
                    currentEditIdentity = currentEditIdentity(appState),
                ),
                bugModel = bugModel,
                remoteProviderConfirmed = providerConfirmations.confirmed(ModelScope.Bug),
                onRemoteProviderConfirmed = { providerConfirmations = providerConfirmations.withConfirmation(ModelScope.Bug, it) },
                onAnalyze = { analyzeSelected(false) },
                onRefresh = { analyzeSelected(true) },
                onCancel = { analysisJob?.cancel() },
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
        sending = chatJob != null,
        functionModel = functionModel,
        remoteProviderConfirmed = providerConfirmations.confirmed(ModelScope.Function),
    )
    DesktopShell(
        appState = appState,
        paneWidths = paneWidths,
        onPaneWidths = { paneWidths = it },
        onSavePaneWidths = { widthStore.save(paneWidths) },
        connection = appState.connection,
        workspace = appState.workspace,
        onWorkspace = { update(DesktopEvent.WorkspaceSelected(it)) },
        editorProgress = editorProgress,
        onFocusChat = { focusComposerControl(ComposerFocusTarget.Chat) },
        onFocusDraft = { focusComposerControl(ComposerFocusTarget.Draft) },
        canFocusChat = contextualActions.canFocusChat,
        canFocusDraft = contextualActions.canFocusDraft,
        canGenerate = contextualActions.canGenerate,
        canValidateDraft = contextualActions.canValidateDraft,
        canRunDraftChecks = contextualActions.canRunFocusedChecks,
        analysisInProgress = analysisJob != null,
        generating = chatJob != null,
        showContext = showContext,
        contextManifest = contextManifest,
        bugModel = bugModel,
        bugProviderConfirmed = providerConfirmations.confirmed(ModelScope.Bug),
        onBugProviderConfirmed = { providerConfirmations = providerConfirmations.withConfirmation(ModelScope.Bug, it) },
        onDismissContext = { showContext = false },
        paletteMode = paletteMode,
        paletteQuery = paletteQuery,
        showPalette = showPalette,
        onPaletteQuery = { paletteQuery = it },
        onDismissPalette = { showPalette = false },
        onOpenPalette = ::openPalette,
        onSelectPaletteFile = {
            showPalette = false
            openFileInEditor(it)
        },
        onSelectPaletteSymbol = {
            showPalette = false
            selectPaletteSymbol(it)
        },
        onSelectPaletteAction = {
            showPalette = false
            update(DesktopEvent.WorkspaceSelected(Workspace.Editor))
            when (it) {
                "refresh_file_analysis" -> analyzeSelected(true)
                "create_declaration" -> requestCreateDeclaration()
                else -> contextAction = it
            }
        },
        onOpenFinding = ::openFinding,
        onPrepareFinding = ::prepareFinding,
        onTriageFinding = ::triageFinding,
        onStartAnalyzeAll = ::startAnalyzeAll,
        onPauseAnalyzeAll = ::pauseAnalyzeAll,
        onResumeAnalyzeAll = ::resumeAnalyzeAll,
        onCancelAnalyzeAll = ::cancelAnalyzeAll,
        onStartScan = ::runVerifiedScan,
        onCancelScan = ::cancelVerifiedScan,
        explorer = explorer,
        contextPane = contextPane,
        onImport = ::importProject,
        onReanalyze = ::reanalyze,
        onReconnect = ::refreshConnection,
        onCancelAnalysis = { analysisJob?.cancel() },
        onSourceLineSelected = ::selectSourceLine,
        onValidateDraft = ::validateEditableDraft,
        onRunDraftChecks = ::runDraftChecks,
        onGenerate = ::sendChatMessage,
        onCancelGeneration = { chatJob?.cancel() },
        onCancelAll = {
            showPalette = false
            showContext = false
            analysisJob?.cancel()
            chatJob?.cancel()
            draftValidationJob?.cancel()
            analyzeAllPolling.stop()
            analyzeAllPollJob?.cancel()
            scanPollJob?.cancel()
        },
    )
    pendingDraftDiscard?.let { pending ->
        DraftDiscardDialog(
            pending = pending,
            onDiscard = ::discardDraftAndContinue,
            onCancel = { pendingDraftDiscard = null },
        )
    }
    pendingImportPath?.let { path ->
        ProjectImportConfirmationDialog(
            model = analyzeModel,
            confirmed = providerConfirmations.confirmed(ModelScope.Analyze),
            onConfirmed = { providerConfirmations = providerConfirmations.withConfirmation(ModelScope.Analyze, it) },
            onImport = {
                pendingImportPath = null
                loadProject(path, restore = false)
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
        confirmButton = {
            FocusFlowButton(onClick = onImport, enabled = confirmed, tone = ActionTone.Primary) { Text("Import project") }
        },
        dismissButton = {
            FocusFlowButton(onClick = onCancel, tone = ActionTone.Neutral) { Text("Cancel") }
        },
    )
}

@Composable
private fun DraftDiscardDialog(
    pending: PendingDraftDiscard,
    onDiscard: () -> Unit,
    onCancel: () -> Unit,
) {
    AlertDialog(
        onDismissRequest = onCancel,
        title = { Text("Discard current draft?") },
        text = {
            Text("Discard the draft for ${pending.currentDraft.targetSymbol} and ${pending.nextLabel}? This only clears the in-memory conversation, draft, and focused checks.")
        },
        confirmButton = {
            FocusFlowButton(onClick = onDiscard, tone = ActionTone.Destructive) { Text("Discard draft") }
        },
        dismissButton = {
            FocusFlowButton(onClick = onCancel, tone = ActionTone.Neutral) { Text("Keep draft") }
        },
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
