package io.miniorca.desktop

import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
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
    var analysisRequestId by remember { mutableStateOf(0) }
    var generationJob by remember { mutableStateOf<Job?>(null) }
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

    fun update(event: DesktopEvent) {
        appState = workflow.dispatch(event)
    }

    LaunchedEffect(appState.preparedAction, appState.preparedRequest, appState.selectedSymbol) {
        if (appState.preparedAction.isNotBlank()) action = appState.preparedAction
        if (appState.preparedRequest.isNotBlank()) request = appState.preparedRequest
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
    fun refreshProjectWorkspace(revision: String) {
        scope.launch {
            runCatching { withContext(Dispatchers.IO) { Triple(api.overview(revision), api.findings(revision), api.analyzeAllJob(revision)) } }
                .onSuccess { (overview, findings, job) ->
                    if (appState.project?.projectRevision == revision) {
                        update(DesktopEvent.OverviewLoaded(overview))
                        update(DesktopEvent.FindingsLoaded(findings.findings))
                        update(DesktopEvent.AnalyzeAllLoaded(job))
                    }
                }
                .onFailure { update(DesktopEvent.Status("Project facts are available; workspace details could not be refreshed.")) }
        }
    }
    fun pollAnalyzeAll(revision: String) {
        analyzeAllPollJob?.cancel()
        analyzeAllPollJob = scope.launch {
            while (true) {
                val response = runCatching { withContext(Dispatchers.IO) { api.analyzeAllJob(revision) } }
                if (response.isFailure) {
                    update(DesktopEvent.Failed(response.exceptionOrNull()?.message ?: "Analyze-all status failed"))
                    return@launch
                }
                val job = response.getOrNull()
                if (appState.project?.projectRevision != revision) break
                update(DesktopEvent.AnalyzeAllLoaded(job))
                if (!shouldPollAnalyzeAll(job)) break
                kotlinx.coroutines.delay(750)
            }
        }
    }
    fun startAnalyzeAll() {
        val project = appState.project ?: return
        scope.launch { runCatching { withContext(Dispatchers.IO) { api.startAnalyzeAll(project.projectRevision) } }
            .onSuccess { update(DesktopEvent.AnalyzeAllLoaded(it)); pollAnalyzeAll(project.projectRevision) }
            .onFailure { update(DesktopEvent.Failed(it.message ?: "Unable to start Analyze-all")) } }
    }
    fun changeAnalyzeAll(action: (String) -> AnalyzeAllJob) {
        val project = appState.project ?: return
        scope.launch { runCatching { withContext(Dispatchers.IO) { action(project.projectRevision) } }
            .onSuccess { update(DesktopEvent.AnalyzeAllLoaded(it)); if (shouldPollAnalyzeAll(it)) pollAnalyzeAll(project.projectRevision) }
            .onFailure { update(DesktopEvent.Failed(it.message ?: "Unable to update Analyze-all")) } }
    }
    fun runVerifiedScan() {
        val project = appState.project ?: return
        scope.launch { runCatching { withContext(Dispatchers.IO) { api.startGoScan(project.projectRevision) } }
            .onSuccess { update(DesktopEvent.GoScanLoaded(it)); refreshProjectWorkspace(project.projectRevision) }
            .onFailure { update(DesktopEvent.Failed(it.message ?: "Unable to start verified scan")) } }
    }
    fun cancelVerifiedScan() {
        val project = appState.project ?: return
        scope.launch { runCatching { withContext(Dispatchers.IO) { api.cancelGoScan(project.projectRevision) } }
            .onSuccess { update(DesktopEvent.GoScanLoaded(it)) }
            .onFailure { update(DesktopEvent.Failed(it.message ?: "Unable to cancel verified scan")) } }
    }
    fun importProject() {
        val directory = chooseDirectory() ?: return
        val requestId = workflow.beginProjectLoad()
        appState = workflow.state
        update(DesktopEvent.Status("Importing ${directory.name}…"))
        scope.launch {
            runCatching { withContext(Dispatchers.IO) { api.importProject(directory.absolutePath) to api.index() } }
                .onSuccess { (project, index) ->
                    if (workflow.projectLoaded(requestId, project, index)) {
                        appState = workflow.state
                        collapsedDirectories = explorerDirectories(index.files)
                        refreshProjectWorkspace(project.projectRevision)
                    }
                }
                .onFailure { if (workflow.isCurrentProjectRequest(requestId)) update(DesktopEvent.Failed(it.message ?: "Import failed")) }
        }
    }
    fun reanalyze() {
        val project = appState.project ?: return
        update(DesktopEvent.Loading)
        update(DesktopEvent.Status("Refreshing deterministic project facts…"))
        scope.launch {
            runCatching { withContext(Dispatchers.IO) { api.reindex(project.projectRevision) } }
                .onSuccess {
                    update(DesktopEvent.IndexRefreshed(it))
                    refreshProjectWorkspace(it.projectRevision)
                }
                .onFailure { update(DesktopEvent.Failed(it.message ?: "Re-analysis failed")) }
        }
    }
    fun selectFile(path: String, editorTarget: EditorNavigationTarget? = null) {
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
        openFinding(finding)
        update(DesktopEvent.SuggestionPrepared("fix", "Address ${finding.title}: ${finding.message}", null))
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

    fun reloadAfterMutation(path: String, revision: String) {
        scope.launch {
            runCatching { withContext(Dispatchers.IO) { Triple(api.index(), api.fileInfo(path), api.symbols(path).symbols) } }
                .onSuccess { (index, file, symbols) ->
                    update(DesktopEvent.IndexRefreshed(index))
                    update(DesktopEvent.FileLoaded(file, symbols))
                    update(DesktopEvent.Status("Project revision $revision is active"))
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
        FocusedActionPane(
            selected = appState.selectedFile,
            symbols = appState.symbols,
            selectedSymbol = appState.selectedSymbol,
            action = action,
            request = request,
            manualSymbol = manualSymbol,
            scopeMode = scopeMode,
            generating = generationJob != null,
            onSelectSymbol = { update(DesktopEvent.SymbolSelected(it)) },
            onAction = { action = it },
            onRequest = { request = it },
            onManualSymbol = { manualSymbol = it },
            onTemplate = ::prepareTemplate,
            onToggleScope = { scopeMode = if (scopeMode == "strict_symbol") "symbol_plus_imports" else "strict_symbol" },
            onInspectContext = ::inspectContext,
            onGenerate = { generatePreview() },
            onCancel = { generationJob?.cancel() },
            onExplain = {
                appState.selectedSymbol?.let { symbol ->
                    update(DesktopEvent.SuggestionPrepared("explain_symbol", "Show the cached explanation for ${symbol.name}.", symbol))
                    update(DesktopEvent.WorkspaceSelected(Workspace.Summary))
                }
            },
            modifier = modifier,
        )
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
        generating = generationJob != null,
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
        onStartAnalyzeAll = ::startAnalyzeAll,
        onPauseAnalyzeAll = { changeAnalyzeAll(api::pauseAnalyzeAll) },
        onResumeAnalyzeAll = { changeAnalyzeAll { api.resumeAnalyzeAll(it) } },
        onCancelAnalyzeAll = { changeAnalyzeAll(api::cancelAnalyzeAll) },
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
        onGenerate = { generatePreview() },
        onCancelGeneration = { generationJob?.cancel() },
        onCancelAll = {
            showPalette = false
            showContext = false
            analysisJob?.cancel()
            generationJob?.cancel()
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
