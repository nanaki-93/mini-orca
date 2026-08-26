package io.miniorca.desktop

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.gestures.detectDragGestures
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.Button
import androidx.compose.material.ButtonDefaults
import androidx.compose.material.CircularProgressIndicator
import androidx.compose.material.AlertDialog
import androidx.compose.material.MaterialTheme
import androidx.compose.material.DrawerValue
import androidx.compose.material.ModalDrawer
import androidx.compose.material.OutlinedTextField
import androidx.compose.material.Surface
import androidx.compose.material.Tab
import androidx.compose.material.TabRow
import androidx.compose.material.Text
import androidx.compose.material.darkColors
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.material.rememberDrawerState
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.input.key.KeyEventType
import androidx.compose.ui.input.key.isCtrlPressed
import androidx.compose.ui.input.key.isMetaPressed
import androidx.compose.ui.input.key.isShiftPressed
import androidx.compose.ui.input.key.key
import androidx.compose.ui.input.key.onPreviewKeyEvent
import androidx.compose.ui.input.key.type
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.window.Window
import androidx.compose.ui.window.application
import java.io.File
import javax.swing.JFileChooser
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.launch
import kotlinx.coroutines.runInterruptible
import kotlinx.coroutines.withContext

private val AppBackground = Color(0xFF0D1117)
private val Panel = Color(0xFF161B22)
private val Card = Color(0xFF21262D)
private val Border = Color(0xFF30363D)
private val PrimaryText = Color(0xFFF0F6FC)
private val SecondaryText = Color(0xFF8B949E)
private val Accent = Color(0xFF2F81F7)
private val Success = Color(0xFF3FB950)
private val Warning = Color(0xFFD29922)
private val Error = Color(0xFFF85149)

private data class ConnectionState(val label: String = "Connecting", val model: String = "", val version: String = "", val connected: Boolean = false, val locality: String = "", val latency: String = "")
private data class FileSelection(val file: ProjectFileInfo, val symbols: List<SymbolInfo>, val analysis: FileAnalysis, val impact: ImpactPreview, val gitStatus: GitStatus)
private enum class PaletteMode { Files, Symbols, Actions }
private enum class NarrowDrawer { Explorer, Action }

fun main() = application {
    Window(onCloseRequest = ::exitApplication, title = "Mini-Orca", resizable = true) {
        MaterialTheme(colors = darkColors(primary = Accent, background = AppBackground, surface = Panel, onBackground = PrimaryText, onSurface = PrimaryText)) {
            MiniOrcaApp()
        }
    }
}

@Composable
private fun MiniOrcaApp(api: ApiClient = remember { ApiClient() }) {
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
    var narrowDrawer by remember { mutableStateOf(NarrowDrawer.Explorer) }
    val drawerState = rememberDrawerState(DrawerValue.Closed)

    LaunchedEffect(appState.preparedAction, appState.preparedRequest, appState.selectedSymbol) {
        if (appState.preparedAction.isNotBlank()) action = appState.preparedAction
        if (appState.preparedRequest.isNotBlank()) request = appState.preparedRequest
        appState.selectedSymbol?.let { manualSymbol = it.name }
    }

    fun refreshConnection() {
        scope.launch {
            val startedAt = System.nanoTime()
            runCatching { withContext(Dispatchers.IO) { api.status() to api.effectiveModel() } }
                .onSuccess {
                    val elapsed = (System.nanoTime() - startedAt) / 1_000_000
                    connection = ConnectionState("Daemon connected", "${it.second.profile} · ${it.second.model}", it.first.version, true, api.endpointLocality(), "${elapsed}ms")
                }
                .onFailure { connection = ConnectionState(label = "Daemon unavailable", model = "Retry from the status bar", locality = api.endpointLocality()) }
        }
    }

    fun openPalette(mode: PaletteMode) {
        paletteMode = mode
        paletteQuery = ""
        showPalette = true
    }

    fun openNarrowDrawer(drawer: NarrowDrawer) {
        narrowDrawer = drawer
        scope.launch { drawerState.open() }
    }

    LaunchedEffect(api) { refreshConnection() }

    fun importProject() {
        val directory = chooseDirectory() ?: return
        appState = appState.reduce(DesktopEvent.Loading).reduce(DesktopEvent.Status("Importing ${directory.name}…"))
        scope.launch {
            runCatching { withContext(Dispatchers.IO) { api.importProject(directory.absolutePath) to api.index() } }
                .onSuccess {
                    appState = appState.reduce(DesktopEvent.ProjectLoaded(it.first, it.second))
                    collapsedDirectories = explorerDirectories(it.second.files)
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
                    val symbols = api.symbols(path).symbols
                    FileSelection(file, symbols, api.analysis(path, project.projectRevision), api.impact(path), api.gitStatus(path))
                }
            }
                .onSuccess {
                    appState = appState.reduce(DesktopEvent.FileLoaded(it.file, it.symbols, it.impact, it.gitStatus)).reduce(DesktopEvent.AnalysisLoaded(it.analysis))
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
                .onSuccess { contextManifest = it; showContext = true }
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
                appState = if (compareWithCurrent) {
                    appState.reduce(DesktopEvent.AlternateCandidateLoaded(currentCandidate!!, candidate))
                } else {
                    appState.reduce(DesktopEvent.CandidateLoaded(candidate))
                }.reduce(DesktopEvent.Status("Generated preview · ${candidate.scopeMode}"))
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
            }
                .onSuccess { appState = appState.reduce(DesktopEvent.ComparisonLoaded(it)).reduce(DesktopEvent.Status("Candidate comparison ready")) }
                .onFailure { appState = appState.reduce(DesktopEvent.Failed(it.message ?: "Candidate comparison failed")) }
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
                .onSuccess { appState = appState.reduce(DesktopEvent.ChecksLoaded(it)).reduce(DesktopEvent.Status("Focused checks ${if (it.applicable) "passed" else "need attention"}")) }
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
                .onSuccess {
                    applied = it
                    appState = appState.reduce(DesktopEvent.CandidateDiscarded).reduce(DesktopEvent.Status("Applied ${candidate.targetPath}; Undo is available."))
                    reloadAfterMutation(candidate.targetPath, it.projectRevision)
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
                .onSuccess {
                    applied = null
                    reloadAfterMutation(path, it.projectRevision)
                    refreshActivity()
                }
                .onFailure { appState = appState.reduce(DesktopEvent.Failed(it.message ?: "Undo failed")) }
        }
    }

    LaunchedEffect(appState.project?.projectId, appState.project?.projectRevision) {
        if (appState.project != null) refreshActivity()
    }

    val explorerPane: @Composable (Modifier) -> Unit = { modifier ->
        Explorer(
                    index = appState.index,
                    selectedPath = appState.selectedFile?.path,
                    filter = filter,
                    collapsedDirectories = collapsedDirectories,
                    onFilter = { filter = it },
                    onToggleDirectory = { path ->
                        collapsedDirectories = if (path in collapsedDirectories) collapsedDirectories - path else collapsedDirectories + path
                    },
                    onSelect = {
                        selectFile(it)
                        scope.launch { drawerState.close() }
                    },
                    loading = appState.loading,
                    modifier = modifier,
                )
    }
    val actionPane: @Composable (Modifier) -> Unit = { modifier ->
        FocusedActionPane(
                    appState.selectedFile,
                    appState.symbols,
                    appState.selectedSymbol,
                    action,
                    request,
                    manualSymbol,
                    scopeMode,
                    generationJob != null,
                    { appState = appState.reduce(DesktopEvent.SymbolSelected(it)) },
                    { action = it },
                    { request = it },
                    { manualSymbol = it },
                    ::prepareTemplate,
                    { scopeMode = if (scopeMode == "strict_symbol") "symbol_plus_imports" else "strict_symbol" },
                    ::inspectContext,
                    { generatePreview() },
                    { generationJob?.cancel() },
                    {
                        appState.selectedSymbol?.let { symbol ->
                            appState = appState.reduce(
                                DesktopEvent.SuggestionPrepared(
                                    "explain_symbol",
                                    "Show the cached explanation for ${symbol.name}.",
                                    symbol,
                                ),
                            )
                            activeTab = 1
                        }
                    },
                    modifier,
                )
    }

    Surface(
        modifier = Modifier.fillMaxSize().onPreviewKeyEvent { event ->
            if (event.type != KeyEventType.KeyDown) return@onPreviewKeyEvent false
            val key = when (event.key) {
                Key.P -> "P"
                Key.O -> "O"
                Key.K -> "K"
                Key.Enter -> "Enter"
                Key.Escape -> "Escape"
                Key.Tab -> "Tab"
                else -> ""
            }
            when (desktopShortcut(key, event.isMetaPressed || event.isCtrlPressed, event.isShiftPressed)) {
                DesktopShortcut.OpenFile -> openPalette(PaletteMode.Files)
                DesktopShortcut.OpenSymbol -> openPalette(PaletteMode.Symbols)
                DesktopShortcut.OpenAction -> openPalette(PaletteMode.Actions)
                DesktopShortcut.Generate -> if (generationJob != null) generationJob?.cancel() else generatePreview()
                DesktopShortcut.Cancel -> {
                    showPalette = false
                    showContext = false
                    analysisJob?.cancel()
                    generationJob?.cancel()
                }
                DesktopShortcut.NextTab -> activeTab = (activeTab + 1) % 3
                null -> return@onPreviewKeyEvent false
            }
            true
        },
        color = AppBackground,
    ) {
        BoxWithConstraints {
            val narrow = useNarrowLayout(maxWidth.value)
            ModalDrawer(
                drawerState = drawerState,
                drawerContent = {
                    if (narrowDrawer == NarrowDrawer.Explorer) explorerPane(Modifier.fillMaxHeight().width(320.dp))
                    else actionPane(Modifier.fillMaxHeight().width(360.dp))
                },
            ) {
                Column {
                    Header(appState.project, appState.loading, connection, ::importProject, ::reanalyze, { openPalette(PaletteMode.Actions) }, narrow, { openNarrowDrawer(NarrowDrawer.Explorer) }, { openNarrowDrawer(NarrowDrawer.Action) })
                    Row(modifier = Modifier.weight(1f).fillMaxWidth()) {
                        if (!narrow) {
                            explorerPane(Modifier.width(paneWidths.explorer.dp).fillMaxHeight())
                            ResizableDivider(onDelta = { paneWidths = paneWidths.withExplorer(paneWidths.explorer + it) }) { widthStore.save(paneWidths) }
                        }
                        ContentPane(
                    project = appState.project, selected = appState.selectedFile, symbols = appState.symbols, analysis = appState.analysis,
                    selectedSymbol = appState.selectedSymbol, activeTab = activeTab, analysisInProgress = analysisJob != null,
                    onTab = { activeTab = it }, onAnalyze = { analyzeSelected(false) }, onRefreshAnalysis = { analyzeSelected(true) },
                    onCancelAnalysis = { analysisJob?.cancel() }, onSelectSymbol = { appState = appState.reduce(DesktopEvent.SymbolSelected(it)) },
                    onPrepareSuggestion = ::prepareSuggestion, candidate = appState.candidate, comparisonBase = appState.comparisonBase, comparison = appState.comparison, checks = appState.checks, applied = applied,
                    onDiscard = { appState = appState.reduce(DesktopEvent.CandidateDiscarded) },
                    onAskForRevision = { appState = appState.reduce(DesktopEvent.Status("Revise the request in Focused Action, then generate a new preview.")) },
                    onRunChecks = ::runFocusedChecks, onGenerateAlternate = { generatePreview(compareWithCurrent = true) }, onCompare = ::comparePreviews, onExport = ::exportReview,
                    comparisonBaseNote = comparisonBaseNote, comparisonCandidateNote = comparisonCandidateNote, onComparisonBaseNote = { comparisonBaseNote = it }, onComparisonCandidateNote = { comparisonCandidateNote = it },
                    onApply = ::applyPreview, onUndo = ::undoAppliedPreview,
                    activity = activity, showActivity = showActivity, onToggleActivity = { showActivity = !showActivity }, impact = appState.impact, gitStatus = appState.gitStatus, modifier = Modifier.weight(1f).fillMaxHeight(),
                )
                        if (!narrow) {
                            ResizableDivider(onDelta = { paneWidths = paneWidths.withAction(paneWidths.action - it) }) { widthStore.save(paneWidths) }
                            actionPane(Modifier.width(paneWidths.action.dp).fillMaxHeight())
                        }
                    }
                    StatusBar(appState.status, appState.error, connection, ::refreshConnection)
                }
            }
        }
        if (showContext) {
            ContextInspectorDialog(contextManifest ?: ContextManifest(), remoteProvider = !api.isLoopbackEndpoint(), onDismiss = { showContext = false })
        }
        if (showPalette) {
            CommandPaletteDialog(
                paletteMode, paletteQuery, { paletteQuery = it },
                appState.index?.files.orEmpty(), appState.symbols,
                onSelectFile = { showPalette = false; selectFile(it) },
                onSelectSymbol = { showPalette = false; appState = appState.reduce(DesktopEvent.SymbolSelected(it)) },
                onSelectAction = { showPalette = false; action = it },
                onDismiss = { showPalette = false },
            )
        }
    }
}

@Composable
private fun Header(
    project: ProjectAnalysis?, busy: Boolean, connection: ConnectionState, onImport: () -> Unit, onReanalyze: () -> Unit, onPalette: () -> Unit,
    narrow: Boolean, onOpenExplorer: () -> Unit, onOpenAction: () -> Unit,
) {
    Row(
        modifier = Modifier.fillMaxWidth().height(56.dp).background(Panel).border(BorderStroke(1.dp, Border)).padding(horizontal = 16.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Box(Modifier.size(26.dp).background(Accent, RoundedCornerShape(7.dp)), contentAlignment = Alignment.Center) { Text("⚡", fontSize = 14.sp) }
        Spacer(Modifier.width(10.dp))
        Text("Mini-Orca", fontWeight = FontWeight.SemiBold)
        project?.let { Spacer(Modifier.width(14.dp)); Text("/  ${it.name}", color = SecondaryText, fontSize = 13.sp) }
        Spacer(Modifier.weight(1f))
        Text(if (connection.connected) "● ${connection.model} · ${connection.locality} ${connection.latency}" else "○ ${connection.label} · ${connection.locality}", color = if (connection.connected) Success else Warning, fontSize = 11.sp)
        if (busy) { Spacer(Modifier.width(12.dp)); CircularProgressIndicator(Modifier.size(18.dp), color = Accent, strokeWidth = 2.dp) }
        Spacer(Modifier.width(12.dp))
        if (narrow) {
            Button(onClick = onOpenExplorer) { Text("Files") }
            Spacer(Modifier.width(8.dp))
            Button(onClick = onOpenAction) { Text("Action") }
            Spacer(Modifier.width(8.dp))
        }
        Button(onClick = onPalette) { Text("⌘K") }
        Spacer(Modifier.width(8.dp))
        Button(onClick = onReanalyze, enabled = project != null) { Text("Re-analyze") }
        Spacer(Modifier.width(8.dp))
        Button(onClick = onImport, colors = ButtonDefaults.buttonColors(backgroundColor = Accent, contentColor = Color.White)) { Text("Import") }
    }
}

@Composable
private fun CommandPaletteDialog(
    mode: PaletteMode, query: String, onQuery: (String) -> Unit, files: List<IndexedFile>, symbols: List<SymbolInfo>,
    onSelectFile: (String) -> Unit, onSelectSymbol: (SymbolInfo) -> Unit, onSelectAction: (String) -> Unit, onDismiss: () -> Unit,
) {
    val title = when (mode) {
        PaletteMode.Files -> "Go to file · ⌘P"
        PaletteMode.Symbols -> "Go to symbol · ⌘⇧O"
        PaletteMode.Actions -> "Focused action · ⌘K"
    }
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(title) },
        text = {
            Column {
                OutlinedTextField(value = query, onValueChange = onQuery, singleLine = true, label = { Text("Filter") }, modifier = Modifier.fillMaxWidth())
                Spacer(Modifier.height(8.dp))
                when (mode) {
                    PaletteMode.Files -> files.filter { it.path.contains(query, ignoreCase = true) }.take(12).forEach { file ->
                        Button(onClick = { onSelectFile(file.path) }, modifier = Modifier.fillMaxWidth().padding(top = 3.dp), colors = ButtonDefaults.buttonColors(backgroundColor = Card, contentColor = PrimaryText)) { Text(file.path, fontFamily = FontFamily.Monospace, fontSize = 11.sp) }
                    }
                    PaletteMode.Symbols -> symbols.filter { it.name.contains(query, ignoreCase = true) }.take(12).forEach { symbol ->
                        Button(onClick = { onSelectSymbol(symbol) }, modifier = Modifier.fillMaxWidth().padding(top = 3.dp), colors = ButtonDefaults.buttonColors(backgroundColor = Card, contentColor = PrimaryText)) { Text("${symbol.kind} · ${symbol.name}", fontFamily = FontFamily.Monospace, fontSize = 11.sp) }
                    }
                    PaletteMode.Actions -> listOf("fix", "refactor", "document").filter { it.contains(query, ignoreCase = true) }.forEach { action ->
                        Button(onClick = { onSelectAction(action) }, modifier = Modifier.fillMaxWidth().padding(top = 3.dp), colors = ButtonDefaults.buttonColors(backgroundColor = Card, contentColor = PrimaryText)) { Text(action.replaceFirstChar { it.uppercase() }) }
                    }
                }
            }
        },
        confirmButton = { Button(onClick = onDismiss) { Text("Close") } },
    )
}

@Composable
private fun Explorer(
    index: ProjectIndex?, selectedPath: String?, filter: String, collapsedDirectories: Set<String>, onFilter: (String) -> Unit,
    onToggleDirectory: (String) -> Unit, onSelect: (String) -> Unit, loading: Boolean, modifier: Modifier,
) {
    val rows = visibleExplorerRows(index?.files.orEmpty(), filter, collapsedDirectories)
    Column(modifier.background(Panel).border(BorderStroke(1.dp, Border)).padding(12.dp)) {
        Text("EXPLORER", color = SecondaryText, fontSize = 11.sp, fontWeight = FontWeight.Bold)
        Spacer(Modifier.height(10.dp))
        OutlinedTextField(value = filter, onValueChange = onFilter, placeholder = { Text("Filter files", color = SecondaryText, fontSize = 12.sp) }, singleLine = true, modifier = Modifier.fillMaxWidth())
        Spacer(Modifier.height(8.dp))
        when {
            index == null && loading -> LoadingRows("Loading indexed files")
            index == null -> Text("Import a project to browse its safe, indexed files.", color = SecondaryText, fontSize = 13.sp)
            rows.isEmpty() -> Text("No indexed files match this filter.", color = SecondaryText, fontSize = 13.sp)
            else -> LazyColumn {
                items(rows, key = { it.path }) { row ->
                    val active = !row.directory && row.path == selectedPath
                    Row(
                        modifier = Modifier.fillMaxWidth().semantics { contentDescription = if (row.directory) "Folder ${row.name}" else "${row.name}, ${analysisBadge(row.analysisStatus)}" }.background(if (active) Card else Color.Transparent, RoundedCornerShape(5.dp))
                            .clickable { if (row.directory) onToggleDirectory(row.path) else onSelect(row.path) }.padding(start = (8 + row.depth * 14).dp, end = 6.dp, top = 6.dp, bottom = 6.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Text(if (row.directory) if (row.path in collapsedDirectories) "▸" else "▾" else "▱", color = SecondaryText, fontSize = 12.sp)
                        Spacer(Modifier.width(6.dp))
                        Text(row.name, color = if (active) PrimaryText else SecondaryText, fontFamily = FontFamily.Monospace, fontSize = 12.sp, modifier = Modifier.weight(1f), maxLines = 1)
                        if (!row.directory) Text(analysisBadge(row.analysisStatus), color = badgeColor(row.analysisStatus), fontSize = 10.sp, maxLines = 1)
                    }
                }
            }
        }
        Spacer(Modifier.height(8.dp))
        Text("✓ fresh  ● stale  ○ not analyzed", color = SecondaryText, fontSize = 10.sp)
    }
}

@Composable
private fun LoadingRows(label: String) {
    Column {
        Text("$label…", color = SecondaryText, fontSize = 12.sp)
        repeat(3) { Box(Modifier.fillMaxWidth().height(20.dp).padding(top = 6.dp).background(Card, RoundedCornerShape(4.dp))) }
    }
}

@Composable
private fun ContentPane(
    project: ProjectAnalysis?, selected: ProjectFileInfo?, symbols: List<SymbolInfo>, analysis: FileAnalysis?, selectedSymbol: SymbolInfo?, activeTab: Int,
    analysisInProgress: Boolean, onTab: (Int) -> Unit, onAnalyze: () -> Unit, onRefreshAnalysis: () -> Unit, onCancelAnalysis: () -> Unit,
    onSelectSymbol: (SymbolInfo) -> Unit, onPrepareSuggestion: (Suggestion) -> Unit, candidate: GenerationResult?, comparisonBase: GenerationResult?, comparison: CandidateComparison?, checks: CandidateCheckReport?, applied: ApplyResult?,
    onDiscard: () -> Unit, onAskForRevision: () -> Unit, onRunChecks: () -> Unit, onGenerateAlternate: () -> Unit, onCompare: () -> Unit, onExport: () -> Unit,
    comparisonBaseNote: String, comparisonCandidateNote: String, onComparisonBaseNote: (String) -> Unit, onComparisonCandidateNote: (String) -> Unit, onApply: () -> Unit, onUndo: () -> Unit,
    activity: List<ActivityEntry>, showActivity: Boolean, onToggleActivity: () -> Unit, impact: ImpactPreview?, gitStatus: GitStatus?, modifier: Modifier,
) {
    Column(modifier.background(AppBackground)) {
        InfoStrip(project, selected)
        TabRow(selectedTabIndex = activeTab, backgroundColor = Panel, contentColor = Accent) {
            listOf("Code", "Summary", "Changes").forEachIndexed { index, title -> Tab(selected = activeTab == index, onClick = { onTab(index) }, text = { Text(title) }) }
        }
        when (activeTab) {
            0 -> CodeTab(project, selected)
            1 -> SummaryTab(selected, symbols, analysis, selectedSymbol, analysisInProgress, onAnalyze, onRefreshAnalysis, onCancelAnalysis, onSelectSymbol, onPrepareSuggestion)
            else -> ChangesTab(candidate, comparisonBase, comparison, checks, applied, selected, onDiscard, onAskForRevision, onRunChecks, onGenerateAlternate, onCompare, onExport, comparisonBaseNote, comparisonCandidateNote, onComparisonBaseNote, onComparisonCandidateNote, onApply, onUndo, activity, showActivity, onToggleActivity, impact, gitStatus)
        }
    }
}

@Composable
private fun SummaryTab(
    selected: ProjectFileInfo?, symbols: List<SymbolInfo>, analysis: FileAnalysis?, selectedSymbol: SymbolInfo?, analysisInProgress: Boolean,
    onAnalyze: () -> Unit, onRefresh: () -> Unit, onCancel: () -> Unit, onSelectSymbol: (SymbolInfo) -> Unit, onPrepareSuggestion: (Suggestion) -> Unit,
) {
    if (selected == null) {
        EmptyTab("Summary", "Select a file to view its deterministic facts and semantic summary.")
        return
    }
    val state = summaryState(analysis, symbols)
    Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(18.dp)) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Text("SUMMARY", fontWeight = FontWeight.SemiBold)
            Spacer(Modifier.width(10.dp))
            Text(analysisBadge(state.status), color = badgeColor(state.status), fontSize = 12.sp)
            Spacer(Modifier.weight(1f))
            if (analysisInProgress) Button(onClick = onCancel) { Text("Cancel") }
            else {
                Button(onClick = onAnalyze) { Text("Analyze") }
                Spacer(Modifier.width(8.dp))
                Button(onClick = onRefresh) { Text("Refresh") }
            }
        }
        Spacer(Modifier.height(18.dp))
        SummarySection("DETERMINISTIC FACTS") {
            Text("${selected.path} · ${selected.language} · ${selected.lineCount} lines · ${formatBytes(selected.sizeBytes)}", color = PrimaryText, fontFamily = FontFamily.Monospace, fontSize = 12.sp)
            if (state.approximateSymbols) Text("Some targets are approximate; confirm their scope before generation.", color = Warning, fontSize = 12.sp, modifier = Modifier.padding(top = 6.dp))
        }
        Spacer(Modifier.height(16.dp))
        SummarySection("SYMBOLS") {
            if (state.emptySymbols) Text("No symbols were extracted from this file.", color = SecondaryText, fontSize = 12.sp)
            symbols.forEach { symbol ->
                Column(
                    Modifier.fillMaxWidth().background(if (selectedSymbol?.name == symbol.name) Card else Color.Transparent, RoundedCornerShape(5.dp))
                        .clickable { onSelectSymbol(symbol) }.padding(8.dp),
                ) {
                    Text("${symbol.kind}  ${symbol.name}", fontFamily = FontFamily.Monospace, fontSize = 12.sp)
                    Text("${symbol.signature.ifBlank { "No signature" }} · lines ${symbol.startLine}–${symbol.endLine} · ${symbol.confidence} · ${if (symbol.atomicTarget) "atomic target" else "not atomic"}", color = SecondaryText, fontSize = 11.sp)
                }
            }
        }
        Spacer(Modifier.height(16.dp))
        SummarySection("MODEL-GENERATED INTERPRETATION") {
            when (state.status) {
                "fresh", "stale" -> {
                    Text(analysis?.purpose.orEmpty().ifBlank { "No purpose returned." }, color = PrimaryText, fontSize = 13.sp)
                    LabeledItems("Responsibilities", analysis?.responsibilities.orEmpty())
                    LabeledItems("Dependencies", analysis?.dependencies.orEmpty())
                    LabeledItems("Side effects", analysis?.sideEffects.orEmpty())
                    selectedSymbol?.let { symbol -> analysis?.symbolExplanations?.get(symbol.name)?.let { explanation -> LabeledItems("Explanation · ${symbol.name}", listOf(explanation)) } }
                    LabeledItems("Findings (model suggestions)", analysis?.risks.orEmpty().map { "${it.severity.uppercase()} · ${it.summary}" })
                    Text("Suggested atomic tasks (model suggestions)", color = SecondaryText, fontSize = 11.sp, modifier = Modifier.padding(top = 12.dp))
                    analysis?.suggestions.orEmpty().forEach { suggestion ->
                        Button(onClick = { onPrepareSuggestion(suggestion) }, modifier = Modifier.padding(top = 5.dp)) { Text(suggestion.title) }
                        Text(suggestion.summary, color = SecondaryText, fontSize = 11.sp)
                    }
                }
                "failed" -> Text(state.failure.ifBlank { "The model could not produce a usable summary. Retry the analysis." }, color = Error, fontSize = 13.sp)
                "running" -> Text("Analysis is running for this file.", color = SecondaryText, fontSize = 13.sp)
                else -> Text("No semantic summary exists yet. Analyze sends only this selected file and bounded project facts.", color = SecondaryText, fontSize = 13.sp)
            }
        }
    }
}

@Composable
private fun SummarySection(title: String, content: @Composable () -> Unit) {
    Text(title, color = SecondaryText, fontSize = 10.sp, fontWeight = FontWeight.Bold)
    Spacer(Modifier.height(6.dp))
    content()
}

@Composable
private fun LabeledItems(label: String, values: List<String>) {
    if (values.isEmpty()) return
    Text(label, color = SecondaryText, fontSize = 11.sp, modifier = Modifier.padding(top = 12.dp))
    values.forEach { Text("• $it", color = PrimaryText, fontSize = 12.sp, modifier = Modifier.padding(top = 3.dp)) }
}

@Composable
private fun CodeTab(project: ProjectAnalysis?, selected: ProjectFileInfo?) {
    val source = when {
        selected != null -> if (selected.binary) "Binary file: source preview is unavailable." else selected.content
        project != null -> project.summary
        else -> "Select Import to analyze a project. Mini-Orca indexes only policy-eligible project files."
    }
    SelectionContainer {
        Box(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(18.dp)) {
            Text(
                text = highlightedCode(source),
                color = PrimaryText, fontFamily = if (selected != null) FontFamily.Monospace else FontFamily.Default, fontSize = 13.sp, lineHeight = 20.sp,
            )
        }
    }
}

@Composable
private fun EmptyTab(title: String, message: String) {
    Box(Modifier.fillMaxSize().padding(24.dp), contentAlignment = Alignment.TopCenter) {
        Column(horizontalAlignment = Alignment.CenterHorizontally) {
            Text(title, color = PrimaryText, fontWeight = FontWeight.SemiBold)
            Spacer(Modifier.height(8.dp))
            Text(message, color = SecondaryText, fontSize = 13.sp)
        }
    }
}

@Composable
private fun FocusedActionPane(
    selected: ProjectFileInfo?, symbols: List<SymbolInfo>, selectedSymbol: SymbolInfo?, action: String, request: String, manualSymbol: String,
    scopeMode: String, generating: Boolean, onSelectSymbol: (SymbolInfo) -> Unit, onAction: (String) -> Unit, onRequest: (String) -> Unit,
    onManualSymbol: (String) -> Unit, onTemplate: (String) -> Unit, onToggleScope: () -> Unit, onInspectContext: () -> Unit, onGenerate: () -> Unit, onCancel: () -> Unit,
    onExplain: () -> Unit, modifier: Modifier,
) {
    Column(modifier.background(Panel).border(BorderStroke(1.dp, Border)).padding(16.dp).verticalScroll(rememberScrollState())) {
        Text("FOCUSED ACTION", fontWeight = FontWeight.SemiBold)
        Spacer(Modifier.height(4.dp))
        Text("One project · one file · one symbol", color = SecondaryText, fontSize = 12.sp)
        Spacer(Modifier.height(20.dp))
        Text("TARGET FILE", color = SecondaryText, fontSize = 10.sp, fontWeight = FontWeight.Bold)
        Text(selected?.path ?: "Select a source file", fontFamily = FontFamily.Monospace, fontSize = 12.sp)
        Spacer(Modifier.height(16.dp))
        Text("SYMBOLS", color = SecondaryText, fontSize = 10.sp, fontWeight = FontWeight.Bold)
        if (selected == null) Text("Choose a file to load atomic targets.", color = SecondaryText, fontSize = 12.sp)
        else if (symbols.isEmpty()) Text("No exact atomic targets are available for this file.", color = SecondaryText, fontSize = 12.sp)
        else symbols.take(8).forEach { symbol ->
            Text("${symbol.kind}  ${symbol.name}  L${symbol.startLine}–${symbol.endLine}", color = if (selectedSymbol?.name == symbol.name) Accent else PrimaryText, fontSize = 11.sp, fontFamily = FontFamily.Monospace, modifier = Modifier.clickable { onSelectSymbol(symbol) }.padding(top = 5.dp))
        }
        selectedSymbol?.let { symbol ->
            Spacer(Modifier.height(14.dp))
            Text("SELECTED SYMBOL", color = SecondaryText, fontSize = 10.sp, fontWeight = FontWeight.Bold)
            Text(symbol.name, fontFamily = FontFamily.Monospace, fontSize = 12.sp)
            Button(onClick = onExplain, modifier = Modifier.padding(top = 6.dp)) { Text("Explain symbol") }
        }
        Spacer(Modifier.height(16.dp))
        Text("MANUAL TARGET", color = SecondaryText, fontSize = 10.sp, fontWeight = FontWeight.Bold)
        OutlinedTextField(value = manualSymbol, onValueChange = onManualSymbol, enabled = !generating, placeholder = { Text("Function or class name") }, singleLine = true, modifier = Modifier.fillMaxWidth())
        if (selectedSymbol == null) Text("Manual targets are checked conservatively and may be rejected without an exact parser-backed symbol.", color = Warning, fontSize = 10.sp, modifier = Modifier.padding(top = 4.dp))
        Spacer(Modifier.height(14.dp))
        Text("TEMPLATE", color = SecondaryText, fontSize = 10.sp, fontWeight = FontWeight.Bold)
        promptTemplates.forEach { template ->
            Button(onClick = { onTemplate(template.id) }, enabled = selected != null && !generating, modifier = Modifier.padding(top = 4.dp), colors = ButtonDefaults.buttonColors(backgroundColor = Card, contentColor = PrimaryText)) { Text(template.label, fontSize = 10.sp) }
        }
        Spacer(Modifier.height(10.dp))
        Text("ACTION", color = SecondaryText, fontSize = 10.sp, fontWeight = FontWeight.Bold)
        Row(horizontalArrangement = Arrangement.spacedBy(4.dp), modifier = Modifier.fillMaxWidth()) {
            listOf("fix", "refactor", "document").forEach { option ->
                Button(onClick = { onAction(option) }, enabled = !generating, colors = ButtonDefaults.buttonColors(backgroundColor = if (action == option) Accent else Card, contentColor = PrimaryText)) { Text(option.replaceFirstChar { it.uppercase() }, fontSize = 10.sp) }
            }
        }
        Spacer(Modifier.height(10.dp))
        OutlinedTextField(value = request, onValueChange = onRequest, enabled = !generating, label = { Text("Request") }, placeholder = { Text("Describe one focused change") }, modifier = Modifier.fillMaxWidth(), minLines = 3)
        Spacer(Modifier.height(12.dp))
        Text("SCOPE", color = SecondaryText, fontSize = 10.sp, fontWeight = FontWeight.Bold)
        Button(onClick = onToggleScope, enabled = !generating, colors = ButtonDefaults.buttonColors(backgroundColor = Card, contentColor = PrimaryText)) {
            Text(if (scopeMode == "strict_symbol") "Strict symbol" else "Symbol + required imports", fontSize = 11.sp)
        }
        Spacer(Modifier.height(12.dp))
        Button(onClick = onInspectContext, enabled = selected != null && !generating, modifier = Modifier.fillMaxWidth(), colors = ButtonDefaults.buttonColors(backgroundColor = Card, contentColor = PrimaryText)) { Text("Inspect context") }
        Spacer(Modifier.height(8.dp))
        if (generating) Button(onClick = onCancel, modifier = Modifier.fillMaxWidth()) { Text("Cancel generation") }
        else Button(onClick = onGenerate, enabled = selected != null && action != "analyze_file" && action != "explain_symbol", modifier = Modifier.fillMaxWidth(), colors = ButtonDefaults.buttonColors(backgroundColor = Accent, contentColor = Color.White)) { Text("Generate preview") }
        if (action == "analyze_file" || action == "explain_symbol") Text("This template is read-only. Review its summary without creating a candidate.", color = SecondaryText, fontSize = 10.sp, modifier = Modifier.padding(top = 4.dp))
        Spacer(Modifier.height(12.dp))
        Text("Nothing is written automatically. Review the diff and required checks before Apply.", color = SecondaryText, fontSize = 11.sp)
    }
}

@Composable
private fun ChangesTab(
    candidate: GenerationResult?, comparisonBase: GenerationResult?, comparison: CandidateComparison?, checks: CandidateCheckReport?, applied: ApplyResult?, selected: ProjectFileInfo?, onDiscard: () -> Unit,
    onAskForRevision: () -> Unit, onRunChecks: () -> Unit, onGenerateAlternate: () -> Unit, onCompare: () -> Unit, onExport: () -> Unit,
    comparisonBaseNote: String, comparisonCandidateNote: String, onComparisonBaseNote: (String) -> Unit, onComparisonCandidateNote: (String) -> Unit, onApply: () -> Unit, onUndo: () -> Unit, activity: List<ActivityEntry>,
    showActivity: Boolean, onToggleActivity: () -> Unit, impact: ImpactPreview?, gitStatus: GitStatus?,
) {
    Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(18.dp)) {
        if (candidate == null) {
            if (applied?.undoAvailable == true) {
                Text("LAST APPLIED CHANGE", fontWeight = FontWeight.SemiBold)
                Text("Post-apply hash: ${applied.postApplyHash}", color = SecondaryText, fontFamily = FontFamily.Monospace, fontSize = 11.sp)
                Spacer(Modifier.height(10.dp))
                Button(onClick = onUndo) { Text("Undo") }
            } else {
                EmptyTab("Changes", "Generate a scoped preview to inspect its validation, diff, and checks.")
            }
        } else {
            Text("CHANGE PREVIEW · ${candidate.targetSymbol}", fontWeight = FontWeight.SemiBold)
            Text("${candidate.targetPath} → ${candidate.targetSymbol} · ${if (candidate.scopeMode == "strict_symbol") "Strict symbol" else "Symbol + required imports"}", color = SecondaryText, fontSize = 12.sp)
            Text("Base ${candidate.baseFileHash.takeLast(12)} · Candidate ${candidate.candidateHash.takeLast(12)}", color = SecondaryText, fontFamily = FontFamily.Monospace, fontSize = 11.sp, modifier = Modifier.padding(top = 4.dp))
            Spacer(Modifier.height(14.dp))
            Text(if (candidate.validation.applicable) "✓ Scope validation passed" else "! Scope validation failed", color = if (candidate.validation.applicable) Success else Error, fontSize = 12.sp)
            candidate.validation.diagnostics.forEach { finding -> Text("${finding.code}: ${finding.message}", color = Error, fontSize = 11.sp, modifier = Modifier.padding(top = 4.dp)) }
            Spacer(Modifier.height(12.dp))
            SelectionContainer {
                Column(Modifier.fillMaxWidth().background(Card, RoundedCornerShape(6.dp)).padding(10.dp)) {
                    candidate.validation.diff.lines.forEach { line ->
                        val prefix = when (line.kind) { "added" -> "+"; "removed" -> "-"; else -> " " }
                        Row {
                            Text("$prefix ${line.newLine.takeIf { it > 0 } ?: line.oldLine}  ", fontFamily = FontFamily.Monospace, fontSize = 11.sp)
                            Text(highlightedCode(line.text), color = when (line.kind) { "added" -> Success; "removed" -> Error; else -> PrimaryText }, fontFamily = FontFamily.Monospace, fontSize = 11.sp)
                        }
                    }
                }
            }
            Spacer(Modifier.height(12.dp))
            Text("FOCUSED CHECKS", color = SecondaryText, fontSize = 10.sp, fontWeight = FontWeight.Bold)
            if (checks == null) Text("Required checks have not run.", color = Warning, fontSize = 12.sp)
            else checks.checks.forEach { check -> Text("${if (check.state == "passed") "✓" else "!"} ${check.name} · ${check.state}", color = if (check.state == "passed" || check.state == "skipped") Success else Warning, fontSize = 12.sp) }
            Spacer(Modifier.height(12.dp))
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Button(onClick = onDiscard) { Text("Discard") }
                Button(onClick = onAskForRevision) { Text("Ask for revision") }
                Button(onClick = onRunChecks, enabled = candidate.validation.applicable) { Text("Run focused checks") }
            }
            Spacer(Modifier.height(8.dp))
            Button(onClick = onGenerateAlternate, enabled = candidate.validation.applicable, colors = ButtonDefaults.buttonColors(backgroundColor = Card, contentColor = PrimaryText)) { Text("Generate alternate") }
            Text("An alternate is another preview only. It cannot apply either candidate.", color = SecondaryText, fontSize = 10.sp, modifier = Modifier.padding(top = 4.dp))
            if (comparisonBase != null) {
                Spacer(Modifier.height(12.dp))
                Text("CANDIDATE COMPARISON", color = SecondaryText, fontSize = 10.sp, fontWeight = FontWeight.Bold)
                OutlinedTextField(value = comparisonBaseNote, onValueChange = onComparisonBaseNote, label = { Text("First candidate note") }, modifier = Modifier.fillMaxWidth(), minLines = 1)
                OutlinedTextField(value = comparisonCandidateNote, onValueChange = onComparisonCandidateNote, label = { Text("Alternate candidate note") }, modifier = Modifier.fillMaxWidth(), minLines = 1)
                Button(onClick = onCompare, colors = ButtonDefaults.buttonColors(backgroundColor = Card, contentColor = PrimaryText), modifier = Modifier.padding(top = 6.dp)) { Text("Compare candidates") }
                comparison?.let { result ->
                    Spacer(Modifier.height(6.dp))
                    Text("First · ${result.left.diffLines} diff lines · ${result.left.scopeMode} · checks ${result.left.checks} · ${result.left.model}", color = SecondaryText, fontSize = 11.sp)
                    Text("Alternate · ${result.right.diffLines} diff lines · ${result.right.scopeMode} · checks ${result.right.checks} · ${result.right.model}", color = SecondaryText, fontSize = 11.sp)
                }
            }
            Spacer(Modifier.height(8.dp))
            Button(onClick = onExport, colors = ButtonDefaults.buttonColors(backgroundColor = Card, contentColor = PrimaryText)) { Text("Export review Markdown") }
            Text("Exports source-free summary, findings, candidate metadata, checks, and audit reference only.", color = SecondaryText, fontSize = 10.sp, modifier = Modifier.padding(top = 4.dp))
            Spacer(Modifier.height(8.dp))
            val applyEnabled = candidate.validation.applicable && checks?.applicable == true && candidate.baseFileHash == selected?.contentHash
            Button(onClick = onApply, enabled = applyEnabled, colors = ButtonDefaults.buttonColors(backgroundColor = Accent, contentColor = Color.White)) { Text("Apply") }
            if (!applyEnabled) Text("Apply is enabled only when the selected file base hash, scope validation, and required checks all pass.", color = SecondaryText, fontSize = 10.sp, modifier = Modifier.padding(top = 4.dp))
        }
        Spacer(Modifier.height(18.dp))
        Text("ADVISORY IMPACT", color = SecondaryText, fontSize = 10.sp, fontWeight = FontWeight.Bold)
        if (impact?.references.isNullOrEmpty()) Text("No indexed dependents found. This never expands model context.", color = SecondaryText, fontSize = 11.sp)
        else impact!!.references.forEach { reference -> Text("${reference.confidence} · ${reference.path} · ${reference.reason}", color = SecondaryText, fontSize = 11.sp) }
        Spacer(Modifier.height(10.dp))
        Text("GIT (READ-ONLY)", color = SecondaryText, fontSize = 10.sp, fontWeight = FontWeight.Bold)
        if (gitStatus?.available == true) Text("${gitStatus.branch} · ${gitStatus.fileState.ifBlank { "clean" }} · ${gitStatus.diffState}", color = SecondaryText, fontSize = 11.sp) else Text("Git is unavailable for this project.", color = SecondaryText, fontSize = 11.sp)
        Spacer(Modifier.height(18.dp))
        Button(onClick = onToggleActivity, colors = ButtonDefaults.buttonColors(backgroundColor = Card, contentColor = PrimaryText)) { Text(if (showActivity) "Hide activity" else "Show activity") }
        if (showActivity) {
            if (activity.isEmpty()) Text("No project activity recorded yet.", color = SecondaryText, fontSize = 12.sp, modifier = Modifier.padding(top = 8.dp))
            activity.takeLast(12).reversed().forEach { entry -> Text("${entry.phase} · ${entry.content} · ${entry.targetFile} ${entry.targetSymbol}".trim(), color = SecondaryText, fontSize = 11.sp, modifier = Modifier.padding(top = 5.dp)) }
        }
    }
}

@Composable
private fun ContextInspectorDialog(manifest: ContextManifest, remoteProvider: Boolean, onDismiss: () -> Unit) {
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text("Context inspector") },
        text = {
            Column(Modifier.verticalScroll(rememberScrollState())) {
                Text("${manifest.estimatedTokens} / ${manifest.tokenLimit.takeIf { it > 0 } ?: "?"} estimated tokens${if (manifest.truncated) " · truncated" else ""}", color = SecondaryText, fontSize = 12.sp)
                if (manifest.byteLimit > 0) Text("${formatBytes(manifest.byteLimit.toLong())} byte limit", color = SecondaryText, fontSize = 11.sp)
                if (remoteProvider) Text("Warning: this provider is not loopback/local. Confirm the destination before sending project context.", color = Warning, fontSize = 12.sp, modifier = Modifier.padding(top = 8.dp))
                Text("Included", fontWeight = FontWeight.SemiBold, modifier = Modifier.padding(top = 10.dp))
                manifest.included.forEach { Text(it.path, fontFamily = FontFamily.Monospace, fontSize = 11.sp) }
                Text("Excluded", fontWeight = FontWeight.SemiBold, modifier = Modifier.padding(top = 10.dp))
                manifest.excluded.forEach { Text("${it.path} · ${it.reason}", color = SecondaryText, fontSize = 11.sp) }
            }
        },
        confirmButton = { Button(onClick = onDismiss) { Text("Close") } },
    )
}

@Composable
private fun InfoStrip(project: ProjectAnalysis?, file: ProjectFileInfo?) {
    Row(Modifier.fillMaxWidth().background(Panel).border(BorderStroke(1.dp, Border)).padding(10.dp), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
        if (project == null) Metric("PROJECT", "Not imported", Modifier.weight(1f))
        else {
            Metric("PROJECT", "${project.type} · ${project.fileCount} files", Modifier.weight(1f))
            Metric("SOURCE", "${project.sourceFileCount} files · ${project.totalLines} lines", Modifier.weight(1f))
            Metric("AI ANALYSIS", project.aiStatus, Modifier.weight(1f))
        }
        if (file != null) Metric("SELECTED FILE", "${file.language} · ${formatBytes(file.sizeBytes)} · ${file.lineCount} lines", Modifier.weight(1f))
    }
}

@Composable
private fun Metric(label: String, value: String, modifier: Modifier) {
    Column(modifier.background(Card, RoundedCornerShape(6.dp)).padding(9.dp)) {
        Text(label, color = SecondaryText, fontSize = 9.sp, fontWeight = FontWeight.Bold)
        Spacer(Modifier.height(3.dp))
        Text(value, color = PrimaryText, fontSize = 12.sp, maxLines = 1)
    }
}

@Composable
private fun ResizableDivider(onDelta: (Float) -> Unit, onCommit: () -> Unit) {
    val density = LocalDensity.current
    Box(
        Modifier.fillMaxHeight().width(6.dp).background(Border).pointerInput(Unit) {
            detectDragGestures(onDrag = { change, amount -> change.consume(); onDelta(with(density) { amount.x.toDp().value }) }, onDragEnd = onCommit)
        },
    )
}

@Composable
private fun StatusBar(status: String, error: String?, connection: ConnectionState, onReconnect: () -> Unit) {
    Row(Modifier.fillMaxWidth().height(30.dp).background(Panel).border(BorderStroke(1.dp, Border)).padding(horizontal = 12.dp), verticalAlignment = Alignment.CenterVertically) {
        Box(Modifier.size(7.dp).background(if (error == null && connection.connected) Success else Error, RoundedCornerShape(50)))
        Spacer(Modifier.width(7.dp))
        Text(error ?: status, color = if (error == null) SecondaryText else Error, fontSize = 11.sp, maxLines = 1)
        Spacer(Modifier.weight(1f))
        Button(onClick = onReconnect, modifier = Modifier.height(24.dp), contentPadding = androidx.compose.foundation.layout.PaddingValues(horizontal = 7.dp, vertical = 0.dp)) { Text("Reconnect", fontSize = 10.sp) }
        Spacer(Modifier.width(8.dp))
        Text(if (connection.version.isBlank()) "Mini-Orca" else "Mini-Orca v${connection.version}", color = SecondaryText, fontSize = 10.sp)
    }
}

private fun badgeColor(status: String) = when (status.lowercase()) {
    "fresh" -> Success
    "stale", "running" -> Warning
    "failed" -> Error
    else -> SecondaryText
}

private fun chooseDirectory(): File? {
    val chooser = JFileChooser().apply { dialogTitle = "Import project"; fileSelectionMode = JFileChooser.DIRECTORIES_ONLY; isAcceptAllFileFilterUsed = false }
    return if (chooser.showOpenDialog(null) == JFileChooser.APPROVE_OPTION) chooser.selectedFile else null
}

private fun chooseExportFile(filename: String): File? {
    val chooser = JFileChooser().apply {
        dialogTitle = "Export focused review"
        selectedFile = File(filename)
    }
    return if (chooser.showSaveDialog(null) == JFileChooser.APPROVE_OPTION) chooser.selectedFile else null
}

private fun formatBytes(bytes: Long): String = when {
    bytes < 1024 -> "$bytes B"
    bytes < 1024 * 1024 -> "${bytes / 1024} KB"
    else -> "${bytes / (1024 * 1024)} MB"
}
