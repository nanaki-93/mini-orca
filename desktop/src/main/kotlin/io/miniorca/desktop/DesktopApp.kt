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

internal data class ConnectionState(val label: String = "Connecting", val model: String = "", val version: String = "", val connected: Boolean = false, val locality: String = "", val latency: String = "")
private data class FileSelection(val file: ProjectFileInfo, val symbols: List<SymbolInfo>, val analysis: FileAnalysis, val impact: ImpactPreview, val gitStatus: GitStatus)
internal enum class PaletteMode { Files, Symbols, Actions }
internal enum class NarrowDrawer { Explorer, Action }

@Composable
internal fun MiniOrcaApp(api: ApiClient = remember { ApiClient() }) {
    val scope = rememberCoroutineScope()
    val widthStore = remember { PaneWidthStore() }
    var appState by remember { mutableStateOf(DesktopState()) }
    var paneWidths by remember { mutableStateOf(widthStore.load()) }
    var filter by remember { mutableStateOf("") }
    var collapsedDirectories by remember { mutableStateOf(emptySet<String>()) }
    var activeTab by remember { mutableStateOf(0) }
    var connection by remember { mutableStateOf(ConnectionState()) }
    var analysisJob by remember { mutableStateOf<Job?>(null) }
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
                    connection = ConnectionState("Daemon connected", "${model.profile} · ${model.model}", status.version, true, api.endpointLocality(), "${elapsed}ms")
                }
                .onFailure { connection = ConnectionState(label = "Daemon unavailable", model = "Retry from the status bar", locality = api.endpointLocality()) }
        }
    }

    fun openPalette(mode: PaletteMode) {
        paletteMode = mode
        paletteQuery = ""
        showPalette = true
    }
    fun importProject() {
        val directory = chooseDirectory() ?: return
        appState = appState.reduce(DesktopEvent.Loading).reduce(DesktopEvent.Status("Importing ${directory.name}…"))
        scope.launch {
            runCatching { withContext(Dispatchers.IO) { api.importProject(directory.absolutePath) to api.index() } }
                .onSuccess { (project, index) ->
                    appState = appState.reduce(DesktopEvent.ProjectLoaded(project, index))
                    collapsedDirectories = explorerDirectories(index.files)
                    activeTab = 0
                }
                .onFailure { appState = appState.reduce(DesktopEvent.Failed(it.message ?: "Import failed")) }
        }
    }
    fun reanalyze() {
        val project = appState.project ?: return
        appState = appState.reduce(DesktopEvent.Loading).reduce(DesktopEvent.Status("Refreshing deterministic project facts…"))
        scope.launch {
            runCatching { withContext(Dispatchers.IO) { api.reindex(project.projectRevision) } }
                .onSuccess { appState = appState.reduce(DesktopEvent.IndexRefreshed(it)) }
                .onFailure { appState = appState.reduce(DesktopEvent.Failed(it.message ?: "Re-analysis failed")) }
        }
    }
    fun selectFile(path: String) {
        val project = appState.project ?: return
        appState = appState.reduce(DesktopEvent.Loading).reduce(DesktopEvent.Status("Loading $path…"))
        scope.launch {
            runCatching {
                withContext(Dispatchers.IO) {
                    val file = api.fileInfo(path)
                    FileSelection(file, api.symbols(path).symbols, api.analysis(path, project.projectRevision), api.impact(path), api.gitStatus(path))
                }
            }.onSuccess { selection ->
                appState = appState.reduce(DesktopEvent.FileLoaded(selection.file, selection.symbols, selection.impact, selection.gitStatus))
                    .reduce(DesktopEvent.AnalysisLoaded(selection.analysis))
                activeTab = 0
            }
                .onFailure { appState = appState.reduce(DesktopEvent.Failed(it.message ?: "File load failed")) }
        }
    }
    fun analyzeSelected(refresh: Boolean) {
        val project = appState.project ?: return
        val file = appState.selectedFile ?: return
        analysisJob?.cancel()
        analysisRequestId += 1
        val requestId = analysisRequestId
        appState = appState.reduce(DesktopEvent.Loading).reduce(DesktopEvent.Status("${if (refresh) "Refreshing" else "Analyzing"} ${file.path}…"))
        analysisJob = scope.launch {
            try {
                val analysis = withContext(Dispatchers.IO) { runInterruptible { api.analyze(file.path, project.projectRevision, refresh) } }
                appState = appState.reduce(DesktopEvent.AnalysisLoaded(analysis)).reduce(DesktopEvent.Status("Summary ${analysis.status}"))
            } catch (_: CancellationException) {
                appState = appState.reduce(DesktopEvent.Status("Analysis canceled"))
            } catch (error: Throwable) {
                appState = appState.reduce(DesktopEvent.Failed(error.message ?: "Analysis failed"))
            } finally {
                if (analysisRequestId == requestId) analysisJob = null
            }
        }
    }

    fun prepareSuggestion(suggestion: Suggestion) {
        val target = appState.symbols.firstOrNull { it.name == suggestion.targetSymbol }
        appState = appState.reduce(DesktopEvent.SuggestionPrepared(suggestion.action, suggestion.summary, target))
    }
    fun prepareTemplate(id: String) {
        val template = templateFor(id)
        if (!templateAllowed(template, appState.selectedFile)) {
            appState = appState.reduce(DesktopEvent.Failed("Generate test requires a selected test file and test symbol."))
            return
        }
        templateID = template.id
        action = template.action
        scopeMode = template.scopeMode
        request = template.defaultRequest
        if (template.readOnly) {
            appState = appState.reduce(DesktopEvent.SuggestionPrepared(template.action, template.defaultRequest, appState.selectedSymbol))
            activeTab = 1
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
                .onFailure { appState = appState.reduce(DesktopEvent.Failed(it.message ?: "Context preview failed")) }
        }
    }

    fun refreshActivity() {
        scope.launch {
            runCatching { withContext(Dispatchers.IO) { api.activity() } }
                .onSuccess { activity = it }
                .onFailure { appState = appState.reduce(DesktopEvent.Failed(it.message ?: "Activity refresh failed")) }
        }
    }
    fun generatePreview(compareWithCurrent: Boolean = false) {
        val project = appState.project ?: return
        val file = appState.selectedFile ?: return
        val currentCandidate = appState.candidate
        if (compareWithCurrent && currentCandidate == null) {
            appState = appState.reduce(DesktopEvent.Failed("Generate one preview before requesting an alternate candidate."))
            return
        }
        val target = appState.selectedSymbol?.name ?: manualSymbol.trim()
        if (target.isBlank() || request.isBlank()) {
            appState = appState.reduce(DesktopEvent.Failed("Select or enter one target symbol and describe the requested change."))
            return
        }
        generationJob?.cancel()
        appState = appState.reduce(DesktopEvent.Loading).reduce(DesktopEvent.Status("Generating $target in ${file.path}…"))
        generationJob = scope.launch {
            try {
                val candidate = withContext(Dispatchers.IO) {
                    runInterruptible {
                        api.generate("$action: ${request.trim()}", file.path, target, project.projectId, project.projectRevision, file.contentHash, scopeMode, action, templateID)
                    }
                }
                appState = if (compareWithCurrent) appState.reduce(DesktopEvent.AlternateCandidateLoaded(currentCandidate!!, candidate)) else appState.reduce(DesktopEvent.CandidateLoaded(candidate))
                appState = appState.reduce(DesktopEvent.Status("Generated preview · ${candidate.scopeMode}"))
                activeTab = 2
                refreshActivity()
            } catch (_: CancellationException) {
                appState = appState.reduce(DesktopEvent.Status("Generation canceled"))
            } catch (error: Throwable) {
                appState = appState.reduce(DesktopEvent.Failed(error.message ?: "Generation failed"))
            } finally {
                generationJob = null
            }
        }
    }

    fun comparePreviews() {
        val base = appState.comparisonBase ?: return
        val candidate = appState.candidate ?: return
        appState = appState.reduce(DesktopEvent.Loading).reduce(DesktopEvent.Status("Comparing two preview-only candidates…"))
        scope.launch {
            runCatching {
                withContext(Dispatchers.IO) {
                    api.compareCandidates(base.generationId, candidate.generationId, candidate.projectRevision, comparisonBaseNote, comparisonCandidateNote)
                }
            }.onSuccess {
                appState = appState.reduce(DesktopEvent.ComparisonLoaded(it)).reduce(DesktopEvent.Status("Candidate comparison ready"))
            }.onFailure {
                appState = appState.reduce(DesktopEvent.Failed(it.message ?: "Candidate comparison failed"))
            }
        }
    }

    fun exportReview() {
        val candidate = appState.candidate ?: return
        appState = appState.reduce(DesktopEvent.Loading).reduce(DesktopEvent.Status("Preparing source-free review export…"))
        scope.launch {
            runCatching { withContext(Dispatchers.IO) { api.exportReview(candidate.generationId, candidate.projectRevision) } }
                .onSuccess { review ->
                    val destination = chooseExportFile(review.filename)
                    if (destination == null) {
                        appState = appState.reduce(DesktopEvent.Status("Review export canceled"))
                    } else {
                        runCatching { withContext(Dispatchers.IO) { destination.writeText(review.markdown) } }
                            .onSuccess { appState = appState.reduce(DesktopEvent.Status("Saved review export: ${destination.name}")) }
                            .onFailure { appState = appState.reduce(DesktopEvent.Failed(it.message ?: "Review export failed")) }
                    }
                }
                .onFailure { appState = appState.reduce(DesktopEvent.Failed(it.message ?: "Review export failed")) }
        }
    }

    fun runFocusedChecks() {
        val candidate = appState.candidate ?: return
        appState = appState.reduce(DesktopEvent.Loading).reduce(DesktopEvent.Status("Running focused checks…"))
        scope.launch {
            runCatching { withContext(Dispatchers.IO) { api.checks(candidate.generationId, candidate.projectRevision) } }
                .onSuccess { report ->
                    val outcome = if (report.applicable) "passed" else "need attention"
                    appState = appState.reduce(DesktopEvent.ChecksLoaded(report)).reduce(DesktopEvent.Status("Focused checks $outcome"))
                }
                .onFailure { appState = appState.reduce(DesktopEvent.Failed(it.message ?: "Focused checks failed")) }
        }
    }

    fun reloadAfterMutation(path: String, revision: String) {
        scope.launch {
            runCatching { withContext(Dispatchers.IO) { Triple(api.index(), api.fileInfo(path), api.symbols(path).symbols) } }
                .onSuccess { (index, file, symbols) ->
                    appState = appState.reduce(DesktopEvent.IndexRefreshed(index)).reduce(DesktopEvent.FileLoaded(file, symbols))
                    appState = appState.reduce(DesktopEvent.Status("Project revision $revision is active"))
                }
                .onFailure { appState = appState.reduce(DesktopEvent.Failed(it.message ?: "Project refresh failed")) }
        }
    }

    fun applyPreview() {
        val candidate = appState.candidate ?: return
        appState = appState.reduce(DesktopEvent.Loading).reduce(DesktopEvent.Status("Applying reviewed preview…"))
        scope.launch {
            runCatching { withContext(Dispatchers.IO) { api.apply(candidate.generationId, candidate.projectId, candidate.projectRevision, candidate.baseFileHash) } }
                .onSuccess { result ->
                    applied = result
                    appState = appState.reduce(DesktopEvent.CandidateDiscarded).reduce(DesktopEvent.Status("Applied ${candidate.targetPath}; Undo is available."))
                    reloadAfterMutation(candidate.targetPath, result.projectRevision)
                    refreshActivity()
                }
                .onFailure { appState = appState.reduce(DesktopEvent.Failed(it.message ?: "Apply failed")) }
        }
    }

    fun undoAppliedPreview() {
        val project = appState.project ?: return
        val result = applied ?: return
        val path = appState.selectedFile?.path ?: return
        appState = appState.reduce(DesktopEvent.Loading).reduce(DesktopEvent.Status("Undoing the last applied preview…"))
        scope.launch {
            runCatching { withContext(Dispatchers.IO) { api.undo(project.projectId, result.projectRevision, result.postApplyHash) } }
                .onSuccess { undo ->
                    applied = null
                    reloadAfterMutation(path, undo.projectRevision)
                    refreshActivity()
                }
                .onFailure { appState = appState.reduce(DesktopEvent.Failed(it.message ?: "Undo failed")) }
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
            onSelectSymbol = { appState = appState.reduce(DesktopEvent.SymbolSelected(it)) },
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
                    appState = appState.reduce(DesktopEvent.SuggestionPrepared("explain_symbol", "Show the cached explanation for ${symbol.name}.", symbol))
                    activeTab = 1
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
        connection = connection,
        activeTab = activeTab,
        onActiveTab = { activeTab = it },
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
            selectFile(it)
        },
        onSelectPaletteSymbol = {
            showPalette = false
            appState = appState.reduce(DesktopEvent.SymbolSelected(it))
        },
        onSelectPaletteAction = {
            showPalette = false
            action = it
        },
        explorer = explorer,
        focusedAction = focusedAction,
        onImport = ::importProject,
        onReanalyze = ::reanalyze,
        onReconnect = ::refreshConnection,
        onAnalyze = { analyzeSelected(false) },
        onRefreshAnalysis = { analyzeSelected(true) },
        onCancelAnalysis = { analysisJob?.cancel() },
        onSelectSymbol = { appState = appState.reduce(DesktopEvent.SymbolSelected(it)) },
        onPrepareSuggestion = ::prepareSuggestion,
        applied = applied,
        onDiscard = { appState = appState.reduce(DesktopEvent.CandidateDiscarded) },
        onAskForRevision = { appState = appState.reduce(DesktopEvent.Status("Revise the request in Focused Action, then generate a new preview.")) },
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
