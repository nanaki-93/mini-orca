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
import androidx.compose.ui.Modifier
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.launch
import kotlinx.coroutines.runInterruptible
import kotlinx.coroutines.withContext
import java.io.File
import javax.swing.JFileChooser

internal enum class PaletteMode { Files, Symbols, Actions }
internal enum class NarrowDrawer { Explorer, Action }

private data class ProjectWorkspaceDetails(
    val overview: ProjectOverview,
    val findings: FindingsResponse,
    val analyzeAll: AnalyzeAllJob?,
    val scan: GoScanReport?,
)

@Composable
internal fun MiniOrcaApp(api: ApiClient = remember { ApiClient() }) {
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
    var generationJob by remember { mutableStateOf<Job?>(null) }
    var chatJob by remember { mutableStateOf<Job?>(null) }
    var draftValidationJob by remember { mutableStateOf<Job?>(null) }
    var manualSymbol by remember { mutableStateOf("") }
    var action by remember { mutableStateOf("fix") }
    var request by remember { mutableStateOf("") }
    var scopeMode by remember { mutableStateOf("strict_symbol") }
    var templateID by remember { mutableStateOf("custom") }
    var contextManifest by remember { mutableStateOf<ContextManifest?>(null) }
    var showContext by remember { mutableStateOf(false) }
    var activity by remember { mutableStateOf<List<ActivityEntry>>(emptyList()) }
    var showActivity by remember { mutableStateOf(false) }
    var applied by remember { mutableStateOf<ApplyResult?>(null) }
    var comparisonBaseNote by remember { mutableStateOf("") }
    var comparisonCandidateNote by remember { mutableStateOf("") }
    var paletteMode by remember { mutableStateOf(PaletteMode.Files) }
    var paletteQuery by remember { mutableStateOf("") }
    var showPalette by remember { mutableStateOf(false) }
    var chatMode by remember { mutableStateOf(ChatEditMode.ReplaceSymbol) }
    var newChatSymbol by remember { mutableStateOf("") }
    var chatMessage by remember { mutableStateOf("") }
    var remoteProviderConfirmed by remember { mutableStateOf(false) }

    fun update(event: DesktopEvent) {
        appState = workflow.dispatch(event)
    }

    DisposableEffect(Unit) {
        onDispose {
            analyzeAllPolling.dispose()
            analyzeAllPollJob?.cancel()
            scanPollJob?.cancel()
        }
    }

    LaunchedEffect(appState.preparedAction, appState.preparedRequest, appState.selectedSymbol) {
        if (appState.preparedAction.isNotBlank()) action = appState.preparedAction
        if (appState.preparedRequest.isNotBlank()) {
            request = appState.preparedRequest
            chatMessage = appState.preparedRequest
        }
        appState.selectedSymbol?.let { manualSymbol = it.name }
    }
    fun refreshConnection() {
        scope.launch {
            val startedAt = System.nanoTime()
            runCatching { withContext(Dispatchers.IO) { api.status() to api.effectiveModel() } }
                .onSuccess { (status, model) ->
                    val elapsed = (System.nanoTime() - startedAt) / 1_000_000
                    update(DesktopEvent.ConnectionUpdated(ConnectionState("Daemon connected", "${model.profile} · ${model.model}", status.version, true, api.endpointLocality(), "${elapsed}ms")))
                }
                .onFailure { update(DesktopEvent.ConnectionUpdated(ConnectionState(label = "Daemon unavailable", model = "Retry from the status bar", locality = api.endpointLocality()))) }
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
            .onFailure { update(DesktopEvent.Failed(it.message ?: "Unable to start Analyze-all")) } }
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
            .onFailure { update(DesktopEvent.Failed(it.message ?: "Unable to update Analyze-all")) } }
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
    fun importProject() {
        val directory = chooseDirectory() ?: return
        analyzeAllPolling.stop()
        analyzeAllPollJob?.cancel()
        scanPollJob?.cancel()
        val requestId = workflow.beginProjectLoad()
        appState = workflow.state
        update(DesktopEvent.Status("Importing ${directory.name}…"))
        scope.launch {
            runCatching { withContext(Dispatchers.IO) { api.importProject(directory.absolutePath) to api.index() } }
                .onSuccess { (project, index) ->
                    if (workflow.projectLoaded(requestId, project, index)) {
                        appState = workflow.state
                        analyzeAllPolling.activate(project.projectRevision)
                        collapsedDirectories = explorerDirectories(index.files)
                        refreshProjectWorkspace(project.projectRevision)
                    }
                }
                .onFailure { if (workflow.isCurrentProjectRequest(requestId)) update(DesktopEvent.Failed(it.message ?: "Import failed")) }
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
    fun selectFile(path: String, editorTarget: EditorNavigationTarget? = null, preparedFixRequest: String? = null) {
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
                    editorTarget?.let { target ->
                        update(DesktopEvent.EditorContextSelected(symbolForNavigation(symbols, target), target.line))
                    }
                    preparedFixRequest?.let { request ->
                        update(DesktopEvent.SuggestionPrepared("fix", request, null))
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
    fun openFinding(finding: UnifiedFinding) {
        val target = findingNavigationTarget(finding, appState.index)
        if (target == null) {
            update(DesktopEvent.Failed("This finding no longer points to a file in the active project."))
            return
        }
        update(DesktopEvent.WorkspaceSelected(Workspace.Editor))
        selectFile(target.path, target)
    }
    fun prepareFinding(finding: UnifiedFinding) {
        if (!findingCanPrepareFix(finding)) { update(DesktopEvent.Failed("Refresh this finding before preparing a fix.")); return }
        val target = findingNavigationTarget(finding, appState.index)
        if (target == null) {
            update(DesktopEvent.Failed("This finding no longer points to a file in the active project."))
            return
        }
        update(DesktopEvent.WorkspaceSelected(Workspace.Editor))
        selectFile(target.path, target, "Address ${finding.title}: ${finding.message}")
    }
    fun triageFinding(finding: UnifiedFinding, action: FindingLifecycleAction) {
        val project = appState.project ?: return
        if (finding.projectRevision.isNotBlank() && finding.projectRevision != project.projectRevision) {
            update(DesktopEvent.Failed("Refresh findings before changing triage on an older revision."))
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
        analysisJob?.cancel()
        val (requestId, requestIdentity) = workflow.beginAnalysis() ?: return
        analysisRequestId = requestId.toInt()
        update(DesktopEvent.Loading)
        update(DesktopEvent.Status("${if (refresh) "Refreshing" else "Analyzing"} ${file.path}…"))
        analysisJob = scope.launch {
            try {
                val analysis = withContext(Dispatchers.IO) { runInterruptible { api.analyze(file.path, project.projectRevision, refresh) } }
                if (workflow.analysisCompleted(requestId, requestIdentity, analysis)) {
                    appState = workflow.state
                    update(DesktopEvent.Status("Summary ${analysis.status}"))
                }
            } catch (_: CancellationException) {
                update(DesktopEvent.Status("Analysis canceled"))
            } catch (error: Throwable) {
                update(DesktopEvent.Failed(error.message ?: "Analysis failed"))
            } finally {
                if (analysisRequestId == requestId.toInt()) analysisJob = null
            }
        }
    }

    fun prepareSuggestion(suggestion: Suggestion) {
        val target = appState.symbols.firstOrNull { it.name == suggestion.targetSymbol }
        update(DesktopEvent.SuggestionPrepared(suggestion.action, suggestion.summary, target))
    }
    fun prepareTemplate(id: String) {
        val template = templateFor(id)
        if (!templateAllowed(template, appState.selectedFile)) {
            update(DesktopEvent.Failed("Generate test requires a selected test file and test symbol."))
            return
        }
        templateID = template.id
        action = template.action
        scopeMode = template.scopeMode
        request = template.defaultRequest
        if (template.readOnly) {
            update(DesktopEvent.SuggestionPrepared(template.action, template.defaultRequest, appState.selectedSymbol))
            update(DesktopEvent.WorkspaceSelected(Workspace.Summary))
        }
    }
    fun inspectContext() {
        val file = appState.selectedFile ?: return
        scope.launch {
            runCatching { withContext(Dispatchers.IO) { runInterruptible { api.context(file.path, action) } } }
                .onSuccess {
                    contextManifest = it
                    showContext = true
                }
                .onFailure { update(DesktopEvent.Failed(it.message ?: "Context preview failed")) }
        }
    }

    fun sendChatMessage() {
        val project = appState.project ?: return
        val file = appState.selectedFile ?: return
        val target = validateChatTarget(file, appState.symbols, appState.selectedSymbol, chatMode, newChatSymbol)
        if (!target.valid) {
            update(DesktopEvent.Failed(target.message))
            return
        }
        if (chatMessage.isBlank()) {
            update(DesktopEvent.Failed("Write a message before sending."))
            return
        }
        if (!api.isLoopbackEndpoint() && !remoteProviderConfirmed) {
            update(DesktopEvent.Failed("Confirm the remote provider before sending context."))
            return
        }
        val (requestId, requestIdentity) = workflow.beginChatLoad() ?: return
        val message = chatMessage.trim()
        val targetIdentity = target.target!!
        val matchingSession = appState.chat.session?.takeIf { chatSessionMatches(it, file, project, targetIdentity) }
        update(DesktopEvent.Loading)
        update(DesktopEvent.Status("Sending a request for ${targetIdentity.symbol} in ${file.path}…"))
        chatJob = scope.launch {
            try {
                val session = matchingSession ?: withContext(Dispatchers.IO) {
                    runInterruptible {
                        api.openChatSession(project.projectId, project.projectRevision, file.contentHash, file.path, targetIdentity.mode.wireValue, targetIdentity.symbol)
                    }
                }
                val proposal = withContext(Dispatchers.IO) {
                    runInterruptible {
                        api.sendChatMessage(session.id, message, session.latestDraftId, remoteProviderConfirmed)
                    }
                }
                if (workflow.chatProposalLoaded(requestId, requestIdentity, session, message, proposal)) {
                    appState = workflow.state
                    chatMessage = ""
                    update(DesktopEvent.Status("Draft ${proposal.draft.revision} is ready for review."))
                }
            } catch (_: CancellationException) {
                if (workflow.cancelChatLoad(requestId, requestIdentity)) appState = workflow.state
            } catch (error: Throwable) {
                if (workflow.cancelChatLoad(requestId, requestIdentity)) appState = workflow.state
                update(DesktopEvent.Failed(error.message ?: "Chat request failed"))
            } finally {
                chatJob = null
            }
        }
    }

    fun validateEditableDraft() {
        val editor = appState.review.editor ?: return
        val project = appState.project ?: return
        val file = appState.selectedFile ?: return
        if (!draftEditorMatchesOpenFile(editor, file, project)) {
            update(DesktopEvent.DraftMarkedStale)
            update(DesktopEvent.Failed("The draft no longer matches the open file and project revision."))
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

    fun refreshActivity() {
        scope.launch {
            runCatching { withContext(Dispatchers.IO) { api.activity() } }
                .onSuccess { activity = it }
                .onFailure { update(DesktopEvent.Failed(it.message ?: "Activity refresh failed")) }
        }
    }
    fun generatePreview(compareWithCurrent: Boolean = false) {
        val project = appState.project ?: return
        val file = appState.selectedFile ?: return
        val fileRequest = workflow.currentFileRequest() ?: return
        val currentCandidate = appState.candidate
        if (compareWithCurrent && currentCandidate == null) {
            update(DesktopEvent.Failed("Generate one preview before requesting an alternate candidate."))
            return
        }
        val target = appState.selectedSymbol?.name ?: manualSymbol.trim()
        if (target.isBlank() || request.isBlank()) {
            update(DesktopEvent.Failed("Select or enter one target symbol and describe the requested change."))
            return
        }
        generationJob?.cancel()
        update(DesktopEvent.Loading)
        update(DesktopEvent.Status("Generating $target in ${file.path}…"))
        generationJob = scope.launch {
            try {
                val candidate = withContext(Dispatchers.IO) {
                    runInterruptible {
                        api.generate("$action: ${request.trim()}", file.path, target, project.projectId, project.projectRevision, file.contentHash, scopeMode, action, templateID)
                    }
                }
                if (!workflow.candidateLoaded(fileRequest, candidate, if (compareWithCurrent) currentCandidate else null)) return@launch
                appState = workflow.state
                update(DesktopEvent.Status("Generated preview · ${candidate.scopeMode}"))
                update(DesktopEvent.WorkspaceSelected(Workspace.Editor))
                refreshActivity()
            } catch (_: CancellationException) {
                update(DesktopEvent.Status("Generation canceled"))
            } catch (error: Throwable) {
                update(DesktopEvent.Failed(error.message ?: "Generation failed"))
            } finally {
                generationJob = null
            }
        }
    }

    fun comparePreviews() {
        val base = appState.comparisonBase ?: return
        val candidate = appState.candidate ?: return
        update(DesktopEvent.Loading)
        update(DesktopEvent.Status("Comparing two preview-only candidates…"))
        scope.launch {
            runCatching {
                withContext(Dispatchers.IO) {
                    api.compareCandidates(base.generationId, candidate.generationId, candidate.projectRevision, comparisonBaseNote, comparisonCandidateNote)
                }
            }.onSuccess {
                update(DesktopEvent.ComparisonLoaded(it))
                update(DesktopEvent.Status("Candidate comparison ready"))
            }.onFailure {
                update(DesktopEvent.Failed(it.message ?: "Candidate comparison failed"))
            }
        }
    }

    fun exportReview() {
        val candidate = appState.candidate ?: return
        update(DesktopEvent.Loading)
        update(DesktopEvent.Status("Preparing source-free review export…"))
        scope.launch {
            runCatching { withContext(Dispatchers.IO) { api.exportReview(candidate.generationId, candidate.projectRevision) } }
                .onSuccess { review ->
                    val destination = chooseExportFile(review.filename)
                    if (destination == null) {
                        update(DesktopEvent.Status("Review export canceled"))
                    } else {
                        runCatching { withContext(Dispatchers.IO) { destination.writeText(review.markdown) } }
                            .onSuccess { update(DesktopEvent.Status("Saved review export: ${destination.name}")) }
                            .onFailure { update(DesktopEvent.Failed(it.message ?: "Review export failed")) }
                    }
                }
                .onFailure { update(DesktopEvent.Failed(it.message ?: "Review export failed")) }
        }
    }

    fun runFocusedChecks() {
        val candidate = appState.candidate ?: return
        update(DesktopEvent.Loading)
        update(DesktopEvent.Status("Running focused checks…"))
        scope.launch {
            runCatching { withContext(Dispatchers.IO) { api.checks(candidate.generationId, candidate.projectRevision) } }
                .onSuccess { report ->
                    val outcome = if (report.applicable) "passed" else "need attention"
                    update(DesktopEvent.ChecksLoaded(report))
                    update(DesktopEvent.Status("Focused checks $outcome"))
                }
                .onFailure { update(DesktopEvent.Failed(it.message ?: "Focused checks failed")) }
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
                    update(DesktopEvent.Status("Project revision $revision is active"))
                    refreshProjectWorkspace(revision)
                }
                .onFailure { update(DesktopEvent.Failed(it.message ?: "Project refresh failed")) }
        }
    }

    fun applyPreview() {
        val candidate = appState.candidate ?: return
        update(DesktopEvent.Loading)
        update(DesktopEvent.Status("Applying reviewed preview…"))
        scope.launch {
            runCatching { withContext(Dispatchers.IO) { api.apply(candidate.generationId, candidate.projectId, candidate.projectRevision, candidate.baseFileHash) } }
                .onSuccess { result ->
                    applied = result
                    update(DesktopEvent.CandidateDiscarded)
                    update(DesktopEvent.Status("Applied ${candidate.targetPath}; Undo is available."))
                    reloadAfterMutation(candidate.targetPath, result.projectRevision)
                    refreshActivity()
                }
                .onFailure { update(DesktopEvent.Failed(it.message ?: "Apply failed")) }
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
                    applied = result
                    update(DesktopEvent.CandidateDiscarded)
                    update(DesktopEvent.Status("Applied ${draft.targetPath}; Undo is available."))
                    reloadAfterMutation(draft.targetPath, result.projectRevision)
                    refreshActivity()
                }
                .onFailure { update(DesktopEvent.Failed(it.message ?: "Apply failed")) }
        }
    }

    fun undoAppliedPreview() {
        val project = appState.project ?: return
        val result = applied ?: return
        val path = appState.selectedFile?.path ?: return
        update(DesktopEvent.Loading)
        update(DesktopEvent.Status("Undoing the last applied preview…"))
        scope.launch {
            runCatching { withContext(Dispatchers.IO) { api.undo(project.projectId, result.projectRevision, result.postApplyHash) } }
                .onSuccess { undo ->
                    applied = null
                    reloadAfterMutation(path, undo.projectRevision)
                    refreshActivity()
                }
                .onFailure { update(DesktopEvent.Failed(it.message ?: "Undo failed")) }
        }
    }
    LaunchedEffect(api) { refreshConnection() }
    LaunchedEffect(appState.project?.projectId, appState.project?.projectRevision) {
        if (appState.project != null) refreshActivity()
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
                selectFile(path)
                onSelected()
            },
            loading = appState.loading,
            modifier = modifier,
        )
    }
    val focusedAction: @Composable (Modifier) -> Unit = { modifier ->
        val actionPane: @Composable (Modifier) -> Unit = { actionModifier -> FocusedActionPane(
            project = appState.project,
            selected = appState.selectedFile,
            symbols = appState.symbols,
            selectedSymbol = appState.selectedSymbol,
            session = appState.chat.session,
            draft = appState.review.draft,
            editor = appState.review.editor,
            mode = chatMode,
            newSymbol = newChatSymbol,
            message = chatMessage,
            sending = chatJob != null,
            remoteProvider = !api.isLoopbackEndpoint(),
            remoteConfirmed = remoteProviderConfirmed,
            onSelectSymbol = { update(DesktopEvent.SymbolSelected(it)) },
            onMode = { chatMode = it },
            onNewSymbol = { newChatSymbol = it },
            onMessage = { chatMessage = it },
            onRemoteConfirmed = { remoteProviderConfirmed = it },
            onInspectContext = ::inspectContext,
            onDraftDeclaration = { update(DesktopEvent.DraftEdited(declaration = it)) },
            onDraftImports = { update(DesktopEvent.DraftEdited(imports = it)) },
            onValidateDraft = ::validateEditableDraft,
            onSend = ::sendChatMessage,
            onCancel = { chatJob?.cancel() },
            modifier = actionModifier,
        ) }
        if (appState.workspace == Workspace.Editor && appState.candidate == null) {
            Column(modifier) {
                EditorBriefPane(appState.selectedFile, appState.analysis, appState.selectedSymbol, appState.symbols, { update(DesktopEvent.SymbolSelected(it)) }, { analyzeSelected(false) }, { analyzeSelected(true) })
                actionPane(Modifier.weight(1f).fillMaxWidth())
            }
        } else {
            actionPane(modifier)
        }
    }
    DesktopShell(
        appState = appState,
        paneWidths = paneWidths,
        onPaneWidths = { paneWidths = it },
        onSavePaneWidths = { widthStore.save(paneWidths) },
        connection = appState.connection,
        workspace = appState.workspace,
        onWorkspace = { update(DesktopEvent.WorkspaceSelected(it)) },
        workspaceCounts = workspaceCounts(appState),
        analysisInProgress = analysisJob != null,
        generating = chatJob != null,
        showContext = showContext,
        contextManifest = contextManifest,
        remoteProvider = !api.isLoopbackEndpoint(),
        onDismissContext = { showContext = false },
        paletteMode = paletteMode,
        paletteQuery = paletteQuery,
        showPalette = showPalette,
        onPaletteQuery = { paletteQuery = it },
        onDismissPalette = { showPalette = false },
        onOpenPalette = ::openPalette,
        onSelectPaletteFile = {
            showPalette = false
            update(DesktopEvent.WorkspaceSelected(Workspace.Editor))
            selectFile(it)
        },
        onSelectPaletteSymbol = {
            showPalette = false
            update(DesktopEvent.SymbolSelected(it))
            update(DesktopEvent.WorkspaceSelected(Workspace.Editor))
        },
        onSelectPaletteAction = {
            showPalette = false
            action = it
            update(DesktopEvent.WorkspaceSelected(Workspace.Editor))
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
        focusedAction = focusedAction,
        onImport = ::importProject,
        onReanalyze = ::reanalyze,
        onReconnect = ::refreshConnection,
        onAnalyze = { analyzeSelected(false) },
        onRefreshAnalysis = { analyzeSelected(true) },
        onCancelAnalysis = { analysisJob?.cancel() },
        onSelectSymbol = { update(DesktopEvent.SymbolSelected(it)) },
        onPrepareSuggestion = ::prepareSuggestion,
        onValidateDraft = ::validateEditableDraft,
        onRunDraftChecks = ::runDraftChecks,
        onApplyDraft = ::applyEditableDraft,
        applied = applied,
        onDiscard = { update(DesktopEvent.CandidateDiscarded) },
        onAskForRevision = { update(DesktopEvent.Status("Revise the request in Focused Action, then generate a new preview.")) },
        onRunChecks = ::runFocusedChecks,
        onGenerateAlternate = { generatePreview(true) },
        onCompare = ::comparePreviews,
        onExport = ::exportReview,
        comparisonBaseNote = comparisonBaseNote,
        comparisonCandidateNote = comparisonCandidateNote,
        onComparisonBaseNote = { comparisonBaseNote = it },
        onComparisonCandidateNote = { comparisonCandidateNote = it },
        onApply = ::applyPreview,
        onUndo = ::undoAppliedPreview,
        activity = activity,
        showActivity = showActivity,
        onToggleActivity = { showActivity = !showActivity },
        onGenerate = ::sendChatMessage,
        onCancelGeneration = { chatJob?.cancel() },
        onCancelAll = {
            showPalette = false
            showContext = false
            analysisJob?.cancel()
            generationJob?.cancel()
            chatJob?.cancel()
            draftValidationJob?.cancel()
            analyzeAllPolling.stop()
            analyzeAllPollJob?.cancel()
            scanPollJob?.cancel()
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

private fun chooseExportFile(filename: String): File? {
    val chooser = JFileChooser().apply {
        dialogTitle = "Export focused review"
        selectedFile = File(filename)
    }
    return if (chooser.showSaveDialog(null) == JFileChooser.APPROVE_OPTION) chooser.selectedFile else null
}
