package io.miniorca.desktop

import java.io.File
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import kotlinx.coroutines.runInterruptible
import kotlinx.coroutines.withContext

data class WorkflowProjectIdentity(val id: String, val revision: String)

data class WorkflowFileIdentity(
    val project: WorkflowProjectIdentity,
    val path: String,
    val contentHash: String,
)

data class WorkflowTaskIdentity(
    val file: WorkflowFileIdentity,
    val mode: ChatEditMode,
    val symbol: String,
    val taskSpec: BugTaskSpec?,
)

data class WorkflowDraftIdentity(
    val file: WorkflowFileIdentity,
    val id: String,
    val revision: Long,
    val hash: String,
)

data class DesktopWorkflowSnapshot(
    val state: DesktopState = DesktopState(),
    val modelCatalog: ModelCatalog = ModelCatalog(),
    val providerConfirmations: ScopedConfirmationState = ScopedConfirmationState(),
    val contextManifest: ContextManifest? = null,
    val analysisInProgress: Boolean = false,
    val generating: Boolean = false,
    val draftValidationInProgress: Boolean = false,
) {
    fun model(scope: ModelScope): ScopedModel = modelCatalog.forScope(scope)
    fun providerConfirmed(scope: ModelScope): Boolean = providerConfirmations.confirmed(scope)
}

/**
 * Owns daemon interaction and every workflow coroutine for the desktop app.
 * Its state flow contains only immutable snapshots; visual-only Compose state
 * remains with the composables that render it.
 */
class DesktopWorkflowPresenter(
    private val api: ApiClient,
    private val lastProjectStore: LastProjectStore,
    parentScope: CoroutineScope,
    private val ioDispatcher: CoroutineDispatcher = Dispatchers.IO,
    private val pollingIntervalMillis: Long = 750,
) : AutoCloseable {
    private val lifetime = SupervisorJob(parentScope.coroutineContext[Job])
    private val scope = CoroutineScope(parentScope.coroutineContext + lifetime)
    private val controller = DesktopWorkflowController()
    private val analyzeAllPolling = AnalyzeAllPollingController()
    private val mutableSnapshot = MutableStateFlow(DesktopWorkflowSnapshot())

    private var connectionJob: Job? = null
    private var projectJob: Job? = null
    private var fileJob: Job? = null
    private var enrichmentJobs: List<Job> = emptyList()
    private var analysisJob: Job? = null
    private var chatJob: Job? = null
    private var draftValidationJob: Job? = null
    private var draftChecksJob: Job? = null
    private var analyzeAllPollJob: Job? = null
    private var scanPollJob: Job? = null
    private var analysisGeneration = 0L
    private var activeTask: WorkflowTaskIdentity? = null
    private var activeDraft: WorkflowDraftIdentity? = null

    val snapshot: StateFlow<DesktopWorkflowSnapshot> = mutableSnapshot.asStateFlow()

    fun start() {
        refreshConnection()
        lastProjectStore.load()?.let { loadProject(it, restore = true) }
    }

    fun dispatch(event: DesktopEvent) {
        controller.dispatch(event)
        publish()
    }

    fun setProviderConfirmation(scope: ModelScope, confirmed: Boolean) {
        mutableSnapshot.value = mutableSnapshot.value.copy(
            providerConfirmations = mutableSnapshot.value.providerConfirmations.withConfirmation(scope, confirmed),
        )
    }

    fun refreshConnection() {
        connectionJob?.cancel()
        connectionJob = scope.launch {
            val startedAt = System.nanoTime()
            try {
                val (status, catalog) = io { api.status() to api.modelCatalog() }
                val previous = mutableSnapshot.value
                val confirmations = if (previous.modelCatalog.identity() == catalog.identity()) previous.providerConfirmations else ScopedConfirmationState()
                mutableSnapshot.value = previous.copy(modelCatalog = catalog, providerConfirmations = confirmations)
                val function = catalog.forScope(ModelScope.Function)
                dispatch(DesktopEvent.ConnectionUpdated(ConnectionState("Daemon connected", "${function.profile} · ${function.model}", status.version, true, api.endpointLocality(), "${(System.nanoTime() - startedAt) / 1_000_000}ms")))
            } catch (_: CancellationException) {
                throw CancellationException()
            } catch (_: Exception) {
                dispatch(DesktopEvent.ConnectionUpdated(ConnectionState(label = "Daemon unavailable", locality = api.endpointLocality())))
            }
        }
    }

    fun loadProject(path: String, restore: Boolean) {
        projectJob?.cancel()
        cancelProjectScopedWork()
        val request = controller.beginProjectLoad()
        publish()
        dispatch(DesktopEvent.Status(if (restore) "Reopening ${File(path).name}…" else "Importing ${File(path).name}…"))
        projectJob = scope.launch {
            try {
                val (project, index) = io {
                    val loaded = if (restore) api.restoreProject(path) else api.importProject(path, snapshot.value.providerConfirmed(ModelScope.Analyze))
                    loaded to api.index()
                }
                if (!controller.projectLoaded(request, project, index)) return@launch
                publish()
                try {
                    lastProjectStore.save(project.path)
                } catch (_: Exception) {
                    // Project restore is optional; a persistence failure must not discard the loaded project.
                }
                if (restore) dispatch(DesktopEvent.Status("Reopened ${project.name}"))
                analyzeAllPolling.activate(project.projectRevision)
                refreshProjectWorkspace(project.identity())
            } catch (_: CancellationException) {
                throw CancellationException()
            } catch (error: Exception) {
                if (!controller.isCurrentProjectRequest(request)) return@launch
                if (restore) dispatch(DesktopEvent.Failed(error.message ?: "Could not reopen the last project"))
                else modelRequestFailed(error, ModelScope.Analyze, "Import failed")
            }
        }
    }

    fun reanalyze() {
        val project = snapshot.value.state.project ?: return
        cancelProjectScopedWork()
        dispatch(DesktopEvent.Loading)
        dispatch(DesktopEvent.Status("Refreshing deterministic project facts…"))
        projectJob = scope.launch {
            try {
                val index = io { api.reindex(project.projectRevision) }
                if (!matchesProject(project.identity())) return@launch
                dispatch(DesktopEvent.IndexRefreshed(index))
                analyzeAllPolling.activate(index.projectRevision)
                refreshProjectWorkspace(project.identity())
            } catch (_: CancellationException) {
                throw CancellationException()
            } catch (error: Exception) {
                if (matchesProject(project.identity())) dispatch(DesktopEvent.Failed(error.message ?: "Re-analysis failed"))
            }
        }
    }

    fun selectFile(
        path: String,
        editorTarget: EditorNavigationTarget? = null,
        preparedFixRequest: String? = null,
        preparedTaskSpec: BugTaskSpec? = null,
    ) {
        chatJob?.cancel()
        draftValidationJob?.cancel()
        draftChecksJob?.cancel()
        activeTask = null
        activeDraft = null
        val request = controller.beginFileLoad(path) ?: return
        publish()
        fileJob?.cancel()
        enrichmentJobs.forEach(Job::cancel)
        fileJob = scope.launch {
            try {
                val (file, symbols) = io { api.fileInfo(path) to api.symbols(path).symbols }
                if (!controller.fileLoaded(request, file, symbols)) return@launch
                publish()
                editorTarget?.let { target ->
                    val selection = resolveEditorNavigation(symbols, target)
                    dispatch(DesktopEvent.EditorContextSelected(selection.symbol, selection.focusLine))
                }
                preparedFixRequest?.let { requestText -> dispatch(DesktopEvent.SuggestionPrepared("fix", requestText, editorTarget?.let { resolveEditorNavigation(symbols, it).symbol }, preparedTaskSpec)) }
                val loaded = controller.currentFileRequest() ?: return@launch
                loadFileEnrichments(loaded)
            } catch (_: CancellationException) {
                controller.cancelFileLoad(request)
                publish()
            } catch (error: Exception) {
                if (controller.fileFailed(request, error.message ?: "File load failed")) publish()
            }
        }
    }

    fun openFileInEditor(
        path: String,
        editorTarget: EditorNavigationTarget? = null,
        preparedFixRequest: String? = null,
        preparedTaskSpec: BugTaskSpec? = null,
    ) {
        if (snapshot.value.state.index?.files?.any { it.path == path } != true) {
            dispatch(DesktopEvent.Failed("This file no longer points to an indexed file in the active project."))
            return
        }
        dispatch(DesktopEvent.WorkspaceSelected(fileInspectionWorkspace()))
        selectFile(path, editorTarget, preparedFixRequest, preparedTaskSpec)
    }

    fun openFinding(finding: UnifiedFinding) {
        val target = findingNavigationTarget(finding, snapshot.value.state.index)
        if (target == null) dispatch(DesktopEvent.Failed("This finding no longer points to a file in the active project."))
        else openFileInEditor(target.path, target)
    }

    fun prepareFinding(finding: UnifiedFinding) {
        if (!findingCanPrepareFix(finding)) {
            dispatch(DesktopEvent.Failed("Refresh this finding before preparing a fix."))
            return
        }
        val target = findingTaskNavigationTarget(finding)
        val requirement = findingTaskRequirement(finding)
        if (target == null || requirement == null || snapshot.value.state.index?.files?.any { it.path == target.path } != true) {
            dispatch(DesktopEvent.Failed("This finding no longer points to a file in the active project."))
            return
        }
        openFileInEditor(target.path, target, requirement, finding.taskSpec)
    }

    fun triageFinding(finding: UnifiedFinding, action: FindingLifecycleAction) {
        val project = snapshot.value.state.project ?: return
        if (finding.projectRevision.isNotBlank() && finding.projectRevision != project.projectRevision) {
            dispatch(DesktopEvent.Failed("Refresh findings before changing triage for out-of-date results."))
            return
        }
        scope.launch {
            try {
                io { api.updateFindingStatus(finding.id, project.projectRevision, action.status) }
                if (!matchesProject(project.identity())) return@launch
                dispatch(DesktopEvent.FindingStatusUpdated(finding.id, action.status))
                refreshFindings(project.identity())
            } catch (_: CancellationException) {
                throw CancellationException()
            } catch (error: Exception) {
                if (matchesProject(project.identity())) dispatch(DesktopEvent.Failed(error.message ?: "Unable to update finding triage"))
            }
        }
    }

    fun analyzeSelected(refresh: Boolean) {
        val state = snapshot.value.state
        val project = state.project ?: return
        val file = state.selectedFile ?: return
        if (snapshot.value.model(ModelScope.Bug).remoteProvider && !snapshot.value.providerConfirmed(ModelScope.Bug)) {
            dispatch(DesktopEvent.Failed("Confirm the Bugs model destination before analyzing this file."))
            return
        }
        analysisJob?.cancel()
        val generation = ++analysisGeneration
        val (request, fileRequest) = controller.beginAnalysis() ?: return
        dispatch(DesktopEvent.Loading)
        dispatch(DesktopEvent.Status("${if (refresh) "Refreshing" else "Analyzing"} ${file.path}…"))
        setOperation(analysis = true)
        analysisJob = scope.launch {
            try {
                val result = io { api.analyze(file.path, project.projectRevision, refresh, snapshot.value.providerConfirmed(ModelScope.Bug)) }
                if (generation == analysisGeneration && controller.analysisCompleted(request, fileRequest, result)) {
                    publish()
                    dispatch(DesktopEvent.Status("Summary ${result.status}"))
                }
            } catch (_: CancellationException) {
                dispatch(DesktopEvent.Status("Analysis canceled"))
                throw CancellationException()
            } catch (error: Exception) {
                if (generation == analysisGeneration && isCurrentFile(file.identity(project))) modelRequestFailed(error, ModelScope.Bug, "Analysis failed")
            } finally {
                if (analysisJob === coroutineContext[Job]) setOperation(analysis = false)
            }
        }
    }

    fun cancelAnalysis() {
        analysisGeneration++
        analysisJob?.cancel()
        setOperation(analysis = false)
    }

    fun cancelGeneration() {
        activeTask = null
        chatJob?.cancel()
        setOperation(generating = false)
    }

    fun cancelDraftValidation() {
        activeDraft = null
        draftValidationJob?.cancel()
        draftChecksJob?.cancel()
        setOperation(validating = false)
    }

    fun inspectContext(action: String) {
        val file = snapshot.value.state.selectedFile ?: return
        val identity = file.identity(snapshot.value.state.project ?: return)
        scope.launch {
            try {
                val manifest = io { api.context(file.path, action) }
                if (isCurrentFile(identity)) mutableSnapshot.value = mutableSnapshot.value.copy(contextManifest = manifest)
            } catch (_: CancellationException) {
                throw CancellationException()
            } catch (error: Exception) {
                if (isCurrentFile(identity)) dispatch(DesktopEvent.Failed(error.message ?: "Context preview failed"))
            }
        }
    }

    fun clearContextManifest() {
        mutableSnapshot.value = mutableSnapshot.value.copy(contextManifest = null)
    }

    fun sendChatMessage(mode: ChatEditMode, requestedSymbol: String, message: String, repair: Boolean = false) {
        val state = snapshot.value.state
        val project = state.project ?: return
        val file = state.selectedFile ?: return
        val target = validateChatTarget(file, state.symbols, state.selectedSymbol, mode, requestedSymbol)
        if (!target.valid) {
            dispatch(DesktopEvent.Failed(target.message))
            return
        }
        val content = message.trim()
        if (content.isBlank()) {
            dispatch(DesktopEvent.Failed("Write a message before sending."))
            return
        }
        if (snapshot.value.model(ModelScope.Function).remoteProvider && !snapshot.value.providerConfirmed(ModelScope.Function)) {
            dispatch(DesktopEvent.Failed("Confirm the Function edits model destination before sending context."))
            return
        }
        val (request, fileRequest) = controller.beginChatLoad() ?: return
        val chatTarget = target.target ?: return
        val taskSpec = state.preparedTaskSpec?.takeIf { it.targetPath == file.path && it.targetSymbol == chatTarget.symbol && chatTarget.mode == ChatEditMode.ReplaceSymbol }
        val identity = WorkflowTaskIdentity(file.identity(project), chatTarget.mode, chatTarget.symbol, taskSpec)
        activeTask = identity
        val matchingSession = state.chat.session?.takeIf { chatSessionMatches(it, file, project, chatTarget, taskSpec) }
        chatJob?.cancel()
        dispatch(DesktopEvent.Loading)
        dispatch(DesktopEvent.Status("Sending a request for ${chatTarget.symbol} in ${file.path}…"))
        setOperation(generating = true)
        chatJob = scope.launch {
            try {
                val session = matchingSession ?: io { api.openChatSession(project.projectId, project.projectRevision, file.contentHash, file.path, chatTarget.mode.wireValue, chatTarget.symbol, taskSpec) }
                val proposal = io { api.sendChatMessage(session.id, content, session.latestDraftId, snapshot.value.providerConfirmed(ModelScope.Function), repair) }
                if (activeTask == identity && controller.chatProposalLoaded(request, fileRequest, session.copy(repairCount = session.repairCount + if (repair) 1 else 0), content, proposal)) {
                    activeDraft = proposal.draft.identity(identity.file)
                    publish()
                    dispatch(DesktopEvent.Status("Draft is ready for review."))
                }
            } catch (_: CancellationException) {
                if (controller.cancelChatLoad(request, fileRequest)) publish()
                throw CancellationException()
            } catch (error: Exception) {
                if (activeTask == identity && controller.cancelChatLoad(request, fileRequest)) publish()
                if (activeTask == identity) modelRequestFailed(error, ModelScope.Function, "Chat request failed")
            } finally {
                if (chatJob === coroutineContext[Job]) setOperation(generating = false)
            }
        }
    }

    fun reviseWithCheckOutput(mode: ChatEditMode, requestedSymbol: String) {
        val state = snapshot.value.state
        val message = repairMessageForChecks(state.chat.session, state.review.draft, state.checks) ?: return
        sendChatMessage(mode, requestedSymbol, message, repair = true)
    }

    fun validateEditableDraft() {
        val state = snapshot.value.state
        val editor = state.review.editor ?: return
        val project = state.project ?: return
        val file = state.selectedFile ?: return
        if (!draftEditorMatchesOpenFile(editor, file, project)) {
            dispatch(DesktopEvent.DraftMarkedStale)
            dispatch(DesktopEvent.Failed("The draft no longer matches the open file."))
            return
        }
        val (request, fileRequest) = controller.beginDraftLoad() ?: return
        val identity = editor.serverDraft.identity(file.identity(project))
        activeDraft = identity
        draftValidationJob?.cancel()
        dispatch(DesktopEvent.DraftValidationStarted)
        dispatch(DesktopEvent.Status("Validating ${editor.serverDraft.targetSymbol}…"))
        setOperation(validating = true)
        draftValidationJob = scope.launch {
            try {
                val updated = io { api.updateDraft(editor.serverDraft.id, project.projectRevision, editor.serverDraft.revision, editor.declaration, editor.imports) }
                val validated = io { api.validateDraft(updated.id, project.projectRevision, updated.revision) }
                if (activeDraft == identity && controller.draftLoaded(request, fileRequest, validated)) {
                    activeDraft = validated.identity(identity.file)
                    publish()
                    dispatch(DesktopEvent.Status(if (validated.validation?.applicable == true) "Draft validation passed." else "Draft validation needs attention."))
                }
            } catch (_: CancellationException) {
                dispatch(DesktopEvent.Status("Draft validation canceled"))
                throw CancellationException()
            } catch (error: Exception) {
                if (activeDraft != identity) return@launch
                if (error is ApiException && error.status == 409) dispatch(DesktopEvent.DraftMarkedStale)
                dispatch(DesktopEvent.Failed(error.message ?: "Draft validation failed"))
            } finally {
                if (draftValidationJob === coroutineContext[Job]) setOperation(validating = false)
            }
        }
    }

    fun runDraftChecks() {
        val state = snapshot.value.state
        val draft = state.review.draft ?: return
        val editor = state.review.editor
        val project = state.project ?: return
        val file = state.selectedFile ?: return
        val eligibility = draftReviewEligibility(editor, draft, null, file, project)
        if (editor?.status != DraftEditorStatus.Valid || !draftEditorMatchesOpenFile(editor, file, project)) {
            dispatch(DesktopEvent.Failed(eligibility.reason))
            return
        }
        val (request, fileRequest) = controller.beginDraftLoad() ?: return
        val identity = draft.identity(file.identity(project))
        activeDraft = identity
        draftChecksJob?.cancel()
        dispatch(DesktopEvent.Loading)
        dispatch(DesktopEvent.Status("Running focused checks for ${draft.targetSymbol}…"))
        draftChecksJob = scope.launch {
            try {
                val checks = io { api.checkDraft(draft.id, project.projectRevision, draft.revision, draft.hash) }
                if (activeDraft == identity && controller.draftChecksLoaded(request, fileRequest, draft, checks)) {
                    publish()
                    dispatch(DesktopEvent.Status(if (checks.applicable) "Focused checks passed." else "Focused checks need attention."))
                }
            } catch (_: CancellationException) {
                throw CancellationException()
            } catch (error: Exception) {
                if (activeDraft != identity) return@launch
                if (error is ApiException && error.status == 409) dispatch(DesktopEvent.DraftMarkedStale)
                dispatch(DesktopEvent.Failed(error.message ?: "Focused checks failed"))
            }
        }
    }

    fun applyEditableDraft() {
        val state = snapshot.value.state
        val draft = state.review.draft ?: return
        val project = state.project ?: return
        val file = state.selectedFile ?: return
        val identity = draft.identity(file.identity(project))
        val eligibility = draftReviewEligibility(state.review.editor, draft, state.checks, file, project)
        if (!eligibility.eligible) {
            dispatch(DesktopEvent.Failed(eligibility.reason))
            return
        }
        dispatch(DesktopEvent.Loading)
        dispatch(DesktopEvent.Status("Applying reviewed declaration draft…"))
        scope.launch {
            try {
                val result = io { api.applyDraft(draft) }
                if (activeDraft != null && activeDraft != identity) return@launch
                dispatch(DesktopEvent.Applied(result))
                dispatch(DesktopEvent.Status("Applied ${draft.targetPath}; Undo is available."))
                reloadAfterMutation(identity.file, result.projectRevision)
            } catch (_: CancellationException) {
                throw CancellationException()
            } catch (error: Exception) {
                if (currentDraftIdentity() == identity) dispatch(DesktopEvent.Failed(error.message ?: "Apply failed"))
            }
        }
    }

    fun undoAppliedDraft() {
        val state = snapshot.value.state
        val project = state.project ?: return
        val result = state.review.applied ?: return
        val file = state.selectedFile ?: return
        val identity = file.identity(project)
        dispatch(DesktopEvent.Loading)
        dispatch(DesktopEvent.Status("Undoing the applied declaration draft…"))
        scope.launch {
            try {
                val undo = io { api.undo(project.projectId, result.projectRevision, result.postApplyHash) }
                if (!isCurrentFile(identity)) return@launch
                dispatch(DesktopEvent.Applied(undo))
                reloadAfterMutation(identity, undo.projectRevision)
            } catch (_: CancellationException) {
                throw CancellationException()
            } catch (error: Exception) {
                if (isCurrentFile(identity)) dispatch(DesktopEvent.Failed(error.message ?: "Undo failed"))
            }
        }
    }

    fun startAnalyzeAll(options: AnalyzeAllRunOptions) = runAnalyzeAllAction("Unable to start Analyze-all") { revision ->
        analyzeAllPolling.activate(revision)
        api.startAnalyzeAll(revision, options.maxFiles, options.maxRetries, options.confirmRemoteProvider)
    }

    fun pauseAnalyzeAll() = runAnalyzeAllAction("Unable to update Analyze-all") { api.pauseAnalyzeAll(it) }

    fun resumeAnalyzeAll(confirmRemoteProvider: Boolean) = runAnalyzeAllAction("Unable to update Analyze-all") { revision ->
        analyzeAllPolling.activate(revision)
        api.resumeAnalyzeAll(revision, confirmRemoteProvider)
    }

    fun cancelAnalyzeAll() = runAnalyzeAllAction("Unable to update Analyze-all") { api.cancelAnalyzeAll(it) }

    fun runVerifiedScan() {
        val project = snapshot.value.state.project ?: return
        val identity = project.identity()
        scope.launch {
            try {
                val report = io { api.startGoScan(identity.revision) }
                if (!matchesProject(identity)) return@launch
                dispatch(DesktopEvent.GoScanLoaded(report))
                pollVerifiedScan(identity)
            } catch (_: CancellationException) {
                throw CancellationException()
            } catch (error: Exception) {
                if (matchesProject(identity)) dispatch(DesktopEvent.Failed(error.message ?: "Unable to start verified scan"))
            }
        }
    }

    fun cancelVerifiedScan() {
        val project = snapshot.value.state.project ?: return
        val identity = project.identity()
        scope.launch {
            try {
                val report = io { api.cancelGoScan(identity.revision) }
                if (!matchesProject(identity)) return@launch
                dispatch(DesktopEvent.GoScanLoaded(report))
                scanPollJob?.cancel()
                refreshFindings(identity)
            } catch (_: CancellationException) {
                throw CancellationException()
            } catch (error: Exception) {
                if (matchesProject(identity)) dispatch(DesktopEvent.Failed(error.message ?: "Unable to cancel verified scan"))
            }
        }
    }

    fun discardDraft() {
        chatJob?.cancel()
        draftValidationJob?.cancel()
        draftChecksJob?.cancel()
        activeTask = null
        activeDraft = null
        dispatch(DesktopEvent.DraftDiscarded)
    }

    private fun cancelAll() {
        cancelAnalysis()
        cancelGeneration()
        cancelDraftValidation()
        analyzeAllPolling.stop()
        analyzeAllPollJob?.cancel()
        scanPollJob?.cancel()
    }

    override fun close() {
        cancelAll()
        connectionJob?.cancel()
        projectJob?.cancel()
        fileJob?.cancel()
        enrichmentJobs.forEach(Job::cancel)
        lifetime.cancel()
    }

    private fun loadFileEnrichments(request: RequestIdentity) {
        enrichmentJobs = listOf(
            scope.launch { optionalFileLoad(request, { api.analysis(request.path, request.projectRevision) }) { controller.analysisLoaded(request, it) } },
            scope.launch { optionalFileLoad(request, { api.impact(request.path) }) { controller.impactLoaded(request, it) } },
            scope.launch { optionalFileLoad(request, { api.gitStatus(request.path) }) { controller.gitStatusLoaded(request, it) } },
        )
    }

    private suspend fun <T> optionalFileLoad(request: RequestIdentity, requestValue: () -> T, accept: (T) -> Boolean) {
        try {
            val value = io(requestValue)
            if (accept(value)) publish()
        } catch (_: CancellationException) {
            throw CancellationException()
        } catch (_: Exception) {
            controller.optionalLoadFailed(request)
        }
    }

    private fun refreshProjectWorkspace(identity: WorkflowProjectIdentity) {
        scope.launch {
            try {
                val details = io { WorkflowProjectWorkspaceDetails(api.overview(identity.revision), api.findings(identity.revision), api.analyzeAllJob(identity.revision), api.goScan(identity.revision)) }
                if (!matchesProject(identity)) return@launch
                dispatch(DesktopEvent.OverviewLoaded(details.overview))
                dispatch(DesktopEvent.FindingsLoaded(details.findings.findings))
                dispatch(DesktopEvent.GoScanLoaded(details.scan))
                publishAnalyzeAll(identity, details.analyzeAll)
                if (shouldPollVerifiedScan(details.scan)) pollVerifiedScan(identity)
            } catch (_: CancellationException) {
                throw CancellationException()
            } catch (_: Exception) {
                if (matchesProject(identity)) dispatch(DesktopEvent.Status("Project facts are available; workspace details could not be refreshed."))
            }
        }
    }

    private fun refreshFindings(identity: WorkflowProjectIdentity) {
        scope.launch {
            try {
                val response = io { api.findings(identity.revision) }
                if (matchesProject(identity)) dispatch(DesktopEvent.FindingsLoaded(response.findings))
            } catch (_: CancellationException) {
                throw CancellationException()
            } catch (_: Exception) {
                // Findings refresh is enrichment; preserve visible deterministic state.
            }
        }
    }

    private fun refreshAnalysisFreshness(identity: WorkflowProjectIdentity) {
        scope.launch {
            try {
                val index = io { api.index() }
                if (matchesProject(identity) && index.projectRevision == identity.revision) dispatch(DesktopEvent.IndexRefreshed(index))
            } catch (_: CancellationException) {
                throw CancellationException()
            } catch (_: Exception) {
                // Cache freshness is optional enrichment.
            }
        }
    }

    private fun runAnalyzeAllAction(fallback: String, action: (String) -> AnalyzeAllJob) {
        val project = snapshot.value.state.project ?: return
        val identity = project.identity()
        scope.launch {
            try {
                val job = io { action(identity.revision) }
                if (matchesProject(identity)) publishAnalyzeAll(identity, job)
            } catch (_: CancellationException) {
                throw CancellationException()
            } catch (error: Exception) {
                if (matchesProject(identity)) modelRequestFailed(error, ModelScope.Bug, fallback)
            }
        }
    }

    private fun publishAnalyzeAll(identity: WorkflowProjectIdentity, job: AnalyzeAllJob?) {
        if (!matchesProject(identity)) return
        val accepted = analyzeAllPolling.receive(identity.revision, job)
        if (accepted == null) {
            if (job == null) dispatch(DesktopEvent.AnalyzeAllLoaded(null))
            return
        }
        dispatch(DesktopEvent.AnalyzeAllLoaded(accepted))
        refreshAnalysisFreshness(identity)
        if (analyzeAllPolling.shouldPoll(identity.revision) && analyzeAllPollJob?.isActive != true) pollAnalyzeAll(identity)
    }

    private fun pollAnalyzeAll(identity: WorkflowProjectIdentity) {
        analyzeAllPollJob?.cancel()
        analyzeAllPollJob = scope.launch {
            while (matchesProject(identity) && analyzeAllPolling.shouldPoll(identity.revision)) {
                try {
                    publishAnalyzeAll(identity, io { api.analyzeAllJob(identity.revision) })
                } catch (_: CancellationException) {
                    throw CancellationException()
                } catch (error: Exception) {
                    if (matchesProject(identity)) dispatch(DesktopEvent.Failed(error.message ?: "Analyze-all status failed"))
                    return@launch
                }
                if (analyzeAllPolling.shouldPoll(identity.revision)) delay(pollingIntervalMillis)
            }
        }
    }

    private fun pollVerifiedScan(identity: WorkflowProjectIdentity) {
        scanPollJob?.cancel()
        scanPollJob = scope.launch {
            while (matchesProject(identity)) {
                try {
                    val scan = io { api.goScan(identity.revision) }
                    if (!matchesProject(identity)) return@launch
                    dispatch(DesktopEvent.GoScanLoaded(scan))
                    if (!shouldPollVerifiedScan(scan)) {
                        refreshFindings(identity)
                        return@launch
                    }
                } catch (_: CancellationException) {
                    throw CancellationException()
                } catch (error: Exception) {
                    if (matchesProject(identity)) dispatch(DesktopEvent.Failed(error.message ?: "Verified scan status failed"))
                    return@launch
                }
                delay(pollingIntervalMillis)
            }
        }
    }

    private fun reloadAfterMutation(file: WorkflowFileIdentity, revision: String) {
        scope.launch {
            try {
                val (index, loaded, symbols) = io { Triple(api.index(), api.fileInfo(file.path), api.symbols(file.path).symbols) }
                if (!matchesProject(file.project)) return@launch
                dispatch(DesktopEvent.IndexRefreshed(index))
                dispatch(DesktopEvent.FileLoaded(loaded, symbols))
                dispatch(DesktopEvent.Status("Project refreshed."))
                refreshProjectWorkspace(WorkflowProjectIdentity(file.project.id, revision))
            } catch (_: CancellationException) {
                throw CancellationException()
            } catch (error: Exception) {
                if (matchesProject(file.project)) dispatch(DesktopEvent.Failed(error.message ?: "Project refresh failed"))
            }
        }
    }

    private fun cancelProjectScopedWork() {
        analyzeAllPolling.stop()
        analyzeAllPollJob?.cancel()
        scanPollJob?.cancel()
        fileJob?.cancel()
        enrichmentJobs.forEach(Job::cancel)
        cancelAll()
    }

    private fun modelRequestFailed(error: Exception, scope: ModelScope, fallback: String) {
        val staleConfirmation = staleRemoteConfirmationMessage(error, scope)
        if (staleConfirmation != null) setProviderConfirmation(scope, false)
        dispatch(DesktopEvent.Failed(staleConfirmation ?: error.message ?: fallback))
    }

    private fun publish() {
        mutableSnapshot.value = mutableSnapshot.value.copy(state = controller.state)
    }

    private fun setOperation(analysis: Boolean? = null, generating: Boolean? = null, validating: Boolean? = null) {
        val current = mutableSnapshot.value
        mutableSnapshot.value = current.copy(
            analysisInProgress = analysis ?: current.analysisInProgress,
            generating = generating ?: current.generating,
            draftValidationInProgress = validating ?: current.draftValidationInProgress,
        )
    }

    private fun matchesProject(identity: WorkflowProjectIdentity): Boolean = snapshot.value.state.project?.identity() == identity
    private fun isCurrentFile(identity: WorkflowFileIdentity): Boolean {
        val state = snapshot.value.state
        val project = state.project ?: return false
        return state.selectedFile?.identity(project) == identity
    }
    private fun currentDraftIdentity(): WorkflowDraftIdentity? {
        val state = snapshot.value.state
        val project = state.project ?: return null
        val file = state.selectedFile ?: return null
        val draft = state.review.draft ?: return null
        return draft.identity(file.identity(project))
    }

    private suspend fun <T> io(block: () -> T): T = withContext(ioDispatcher) { runInterruptible { block() } }
}

private fun ProjectAnalysis.identity(): WorkflowProjectIdentity = WorkflowProjectIdentity(projectId, projectRevision)
private fun ProjectFileInfo.identity(project: ProjectAnalysis): WorkflowFileIdentity = WorkflowFileIdentity(project.identity(), path, contentHash)
private fun DeclarationDraft.identity(file: WorkflowFileIdentity): WorkflowDraftIdentity = WorkflowDraftIdentity(file, id, revision, hash)

private data class WorkflowProjectWorkspaceDetails(
    val overview: ProjectOverview,
    val findings: FindingsResponse,
    val analyzeAll: AnalyzeAllJob?,
    val scan: GoScanReport?,
)
