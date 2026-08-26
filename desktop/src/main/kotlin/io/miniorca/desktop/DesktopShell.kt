package io.miniorca.desktop

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.gestures.detectDragGestures
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
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.AlertDialog
import androidx.compose.material.Button
import androidx.compose.material.DrawerValue
import androidx.compose.material.ModalDrawer
import androidx.compose.material.Surface
import androidx.compose.material.Text
import androidx.compose.material.rememberDrawerState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.input.key.KeyEventType
import androidx.compose.ui.input.key.isCtrlPressed
import androidx.compose.ui.input.key.isMetaPressed
import androidx.compose.ui.input.key.isShiftPressed
import androidx.compose.ui.input.key.key
import androidx.compose.ui.input.key.onPreviewKeyEvent
import androidx.compose.ui.input.key.type
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.launch
import java.util.prefs.Preferences

data class ExplorerRow(
    val path: String,
    val name: String,
    val depth: Int,
    val directory: Boolean,
    val analysisStatus: String = "missing",
)

fun useNarrowLayout(widthDp: Float): Boolean = widthDp < 1000f

/** Builds a stable project-relative explorer without exposing filesystem paths. */
fun explorerRows(files: List<IndexedFile>, filter: String = ""): List<ExplorerRow> {
    val matching = files
        .filter { filter.isBlank() || it.path.contains(filter, ignoreCase = true) }
        .associateBy { it.path }
    if (matching.isEmpty()) return emptyList()

    val directories = sortedSetOf<String>()
    matching.keys.forEach { path ->
        path.substringBeforeLast('/', "").takeIf { it.isNotBlank() }?.let { parent ->
            parent.split('/').indices.forEach { index -> directories += parent.split('/').take(index + 1).joinToString("/") }
        }
    }
    val children = mutableMapOf<String, MutableList<String>>()
    directories.forEach { directory ->
        children.getOrPut(directory.substringBeforeLast('/', "")) { mutableListOf() } += directory
    }
    matching.keys.forEach { path ->
        children.getOrPut(path.substringBeforeLast('/', "")) { mutableListOf() } += path
    }

    val rows = mutableListOf<ExplorerRow>()
    fun appendChildren(parent: String) {
        children[parent].orEmpty()
            .sortedWith(compareBy<String> { it !in directories }.thenBy { it.substringAfterLast('/').lowercase() })
            .forEach { path ->
                val directory = path in directories
                rows += ExplorerRow(path, path.substringAfterLast('/'), path.count { it == '/' }, directory, matching[path]?.analysisStatus ?: "missing")
                if (directory) appendChildren(path)
            }
    }
    appendChildren("")
    return rows
}

fun explorerDirectories(files: List<IndexedFile>): Set<String> = explorerRows(files)
    .filter { it.directory }
    .mapTo(linkedSetOf()) { it.path }

fun visibleExplorerRows(files: List<IndexedFile>, filter: String, collapsedDirectories: Set<String>): List<ExplorerRow> {
    val rows = explorerRows(files, filter)
    if (filter.isNotBlank()) return rows
    return rows.filter { row -> collapsedDirectories.none { row.path.startsWith("$it/") } }
}

fun analysisBadge(status: String): String = when (status.lowercase()) {
    "fresh" -> "✓ Fresh"
    "stale" -> "● Stale"
    "failed" -> "! Failed"
    "running" -> "… Analyzing"
    else -> "○ Not analyzed"
}

data class PaneWidths(val explorer: Float = 270f, val action: Float = 390f) {
    fun withExplorer(value: Float) = copy(explorer = value.coerceIn(180f, 520f))
    fun withAction(value: Float) = copy(action = value.coerceIn(280f, 560f))
}

class PaneWidthStore(private val preferences: Preferences = Preferences.userNodeForPackage(PaneWidthStore::class.java)) {
    fun load(): PaneWidths = PaneWidths(
        explorer = preferences.getFloat("explorer-width", 270f).coerceIn(180f, 520f),
        action = preferences.getFloat("action-width", 390f).coerceIn(280f, 560f),
    )

    fun save(widths: PaneWidths) {
        preferences.putFloat("explorer-width", widths.explorer)
        preferences.putFloat("action-width", widths.action)
    }
}

@Composable
internal fun DesktopShell(
    appState: DesktopState,
    paneWidths: PaneWidths,
    onPaneWidths: (PaneWidths) -> Unit,
    onSavePaneWidths: () -> Unit,
    connection: ConnectionState,
    workspace: Workspace,
    onWorkspace: (Workspace) -> Unit,
    workspaceCounts: WorkspaceCounts,
    analysisInProgress: Boolean,
    generating: Boolean,
    showContext: Boolean,
    contextManifest: ContextManifest?,
    remoteProvider: Boolean,
    remoteProviderConfirmed: Boolean,
    onRemoteProviderConfirmed: (Boolean) -> Unit,
    onDismissContext: () -> Unit,
    paletteMode: PaletteMode,
    paletteQuery: String,
    showPalette: Boolean,
    onPaletteQuery: (String) -> Unit,
    onDismissPalette: () -> Unit,
    onOpenPalette: (PaletteMode) -> Unit,
    onSelectPaletteFile: (String) -> Unit,
    onSelectPaletteSymbol: (SymbolInfo) -> Unit,
    onSelectPaletteAction: (String) -> Unit,
    onOpenFinding: (UnifiedFinding) -> Unit,
    onPrepareFinding: (UnifiedFinding) -> Unit,
    onTriageFinding: (UnifiedFinding, FindingLifecycleAction) -> Unit,
    onStartAnalyzeAll: (AnalyzeAllRunOptions) -> Unit,
    onPauseAnalyzeAll: () -> Unit,
    onResumeAnalyzeAll: (Boolean) -> Unit,
    onCancelAnalyzeAll: () -> Unit,
    onStartScan: () -> Unit,
    onCancelScan: () -> Unit,
    explorer: @Composable (Modifier, () -> Unit) -> Unit,
    focusedAction: @Composable (Modifier) -> Unit,
    onImport: () -> Unit,
    onReanalyze: () -> Unit,
    onReconnect: () -> Unit,
    onAnalyze: () -> Unit,
    onRefreshAnalysis: () -> Unit,
    onCancelAnalysis: () -> Unit,
    onSelectSymbol: (SymbolInfo) -> Unit,
    onPrepareSuggestion: (Suggestion) -> Unit,
    onValidateDraft: () -> Unit,
    onRunDraftChecks: () -> Unit,
    onApplyDraft: () -> Unit,
    applied: ApplyResult?,
    onDiscard: () -> Unit,
    onAskForRevision: () -> Unit,
    onRunChecks: () -> Unit,
    onGenerateAlternate: () -> Unit,
    onCompare: () -> Unit,
    onExport: () -> Unit,
    comparisonBaseNote: String,
    comparisonCandidateNote: String,
    onComparisonBaseNote: (String) -> Unit,
    onComparisonCandidateNote: (String) -> Unit,
    onApply: () -> Unit,
    onUndo: () -> Unit,
    activity: List<ActivityEntry>,
    showActivity: Boolean,
    onToggleActivity: () -> Unit,
    onGenerate: () -> Unit,
    onCancelGeneration: () -> Unit,
    onCancelAll: () -> Unit,
) {
    val scope = rememberCoroutineScope()
    val drawerState = rememberDrawerState(DrawerValue.Closed)
    var narrowDrawer by remember { mutableStateOf(NarrowDrawer.Explorer) }
    fun openDrawer(drawer: NarrowDrawer) {
        narrowDrawer = drawer
        scope.launch { drawerState.open() }
    }
    Surface(
        modifier = Modifier.fillMaxSize().onPreviewKeyEvent { event ->
            if (event.type != KeyEventType.KeyDown) return@onPreviewKeyEvent false
            val key = when (event.key) {
                Key.P -> "P"
                Key.O -> "O"
                Key.K -> "K"
                Key.One -> "1"
                Key.Two -> "2"
                Key.Three -> "3"
                Key.Four -> "4"
                Key.F -> "F"
                Key.D -> "D"
                Key.V -> "V"
                Key.C -> "C"
                Key.Enter -> "Enter"
                Key.Escape -> "Escape"
                Key.Tab -> "Tab"
                else -> ""
            }
            when (desktopShortcut(key, event.isMetaPressed || event.isCtrlPressed, event.isShiftPressed)) {
                DesktopShortcut.OpenFile -> onOpenPalette(PaletteMode.Files)
                DesktopShortcut.OpenSymbol -> onOpenPalette(PaletteMode.Symbols)
                DesktopShortcut.OpenAction, DesktopShortcut.FocusChat, DesktopShortcut.FocusDraft -> onOpenPalette(PaletteMode.Actions)
                DesktopShortcut.FocusBugsFilters -> onWorkspace(Workspace.Bugs)
                DesktopShortcut.ValidateDraft -> onValidateDraft()
                DesktopShortcut.RunDraftChecks -> onRunDraftChecks()
                DesktopShortcut.SummaryWorkspace -> onWorkspace(Workspace.Summary)
                DesktopShortcut.AnalysisWorkspace -> onWorkspace(Workspace.Analysis)
                DesktopShortcut.BugsWorkspace -> onWorkspace(Workspace.Bugs)
                DesktopShortcut.EditorWorkspace -> onWorkspace(Workspace.Editor)
                DesktopShortcut.Generate -> if (generating) onCancelGeneration() else onGenerate()
                DesktopShortcut.Cancel -> when {
                    showPalette -> onDismissPalette()
                    showContext -> onDismissContext()
                    generating -> onCancelGeneration()
                    analysisInProgress -> onCancelAnalysis()
                    else -> return@onPreviewKeyEvent false
                }
                DesktopShortcut.NextTab -> onWorkspace(nextWorkspace(workspace))
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
                    if (narrowDrawer == NarrowDrawer.Explorer) {
                        explorer(Modifier.fillMaxHeight().width(320.dp)) { scope.launch { drawerState.close() } }
                    } else {
                        focusedAction(Modifier.fillMaxHeight().width(360.dp))
                    }
                },
            ) {
                Column {
                    DesktopHeader(appState.project, appState.loading, connection, onImport, onReanalyze, { onOpenPalette(PaletteMode.Actions) }, narrow, { openDrawer(NarrowDrawer.Explorer) }, { openDrawer(NarrowDrawer.Action) })
                    WorkspaceNavigation(workspace, workspaceCounts, onWorkspace)
                    Row(modifier = Modifier.weight(1f).fillMaxWidth()) {
                        if (!narrow) {
                            explorer(Modifier.width(paneWidths.explorer.dp).fillMaxHeight()) {}
                            ResizableDivider(onDelta = { onPaneWidths(paneWidths.withExplorer(paneWidths.explorer + it)) }, onCommit = onSavePaneWidths)
                        }
                        ContentPane(
                            project = appState.project, overview = appState.overview, selected = appState.selectedFile, symbols = appState.symbols, analysis = appState.analysis, selectedSymbol = appState.selectedSymbol,
                            workspace = workspace, analysisInProgress = analysisInProgress, onAnalyze = onAnalyze, onRefreshAnalysis = onRefreshAnalysis, onCancelAnalysis = onCancelAnalysis, remoteProvider = remoteProvider, remoteProviderConfirmed = remoteProviderConfirmed, onRemoteProviderConfirmed = onRemoteProviderConfirmed,
                            onSelectSymbol = onSelectSymbol, onPrepareSuggestion = onPrepareSuggestion, candidate = appState.candidate, comparisonBase = appState.comparisonBase, comparison = appState.comparison,
                            checks = appState.checks, draft = appState.review.draft, editor = appState.review.editor, applied = applied, onRunDraftChecks = onRunDraftChecks, onApplyDraft = onApplyDraft, onDiscard = onDiscard, onAskForRevision = onAskForRevision, onRunChecks = onRunChecks, onGenerateAlternate = onGenerateAlternate,
                            onCompare = onCompare, onExport = onExport, comparisonBaseNote = comparisonBaseNote, comparisonCandidateNote = comparisonCandidateNote, onComparisonBaseNote = onComparisonBaseNote,
                            onComparisonCandidateNote = onComparisonCandidateNote, onApply = onApply, onUndo = onUndo, activity = activity, showActivity = showActivity, onToggleActivity = onToggleActivity,
                            impact = appState.impact, gitStatus = appState.gitStatus, findings = appState.findings.findings, scan = appState.findings.scan, analyzeAll = appState.findings.analyzeAll, coverage = appState.overview?.analysisCoverage, onOpenFinding = onOpenFinding, onPrepareFinding = onPrepareFinding, onTriageFinding = onTriageFinding,
                            onStartAnalyzeAll = onStartAnalyzeAll, onPauseAnalyzeAll = onPauseAnalyzeAll, onResumeAnalyzeAll = onResumeAnalyzeAll, onCancelAnalyzeAll = onCancelAnalyzeAll, onStartScan = onStartScan, onCancelScan = onCancelScan,
                            focusedLine = appState.selection.focusedLine, showCompactEditorBrief = narrow, onWorkspace = onWorkspace, modifier = Modifier.weight(1f).fillMaxHeight(),
                        )
                        if (!narrow) {
                            ResizableDivider(onDelta = { onPaneWidths(paneWidths.withAction(paneWidths.action - it)) }, onCommit = onSavePaneWidths)
                            focusedAction(Modifier.width(paneWidths.action.dp).fillMaxHeight())
                        }
                    }
                    DesktopStatusBar(appState.status, appState.error, connection, onReconnect)
                }
            }
        }
        if (showContext) ContextInspectorDialog(contextManifest ?: ContextManifest(), remoteProvider, onDismissContext)
        if (showPalette) CommandPaletteDialog(paletteMode, paletteQuery, onPaletteQuery, appState.index?.files.orEmpty(), appState.symbols, onSelectPaletteFile, onSelectPaletteSymbol, onSelectPaletteAction, onDismissPalette)
    }
}

@Composable
private fun ContentPane(
    project: ProjectAnalysis?, overview: ProjectOverview?, selected: ProjectFileInfo?, symbols: List<SymbolInfo>, analysis: FileAnalysis?, selectedSymbol: SymbolInfo?, workspace: Workspace,
    analysisInProgress: Boolean, onAnalyze: () -> Unit, onRefreshAnalysis: () -> Unit, onCancelAnalysis: () -> Unit, remoteProvider: Boolean, remoteProviderConfirmed: Boolean, onRemoteProviderConfirmed: (Boolean) -> Unit,
    onSelectSymbol: (SymbolInfo) -> Unit, onPrepareSuggestion: (Suggestion) -> Unit, candidate: GenerationResult?, comparisonBase: GenerationResult?, comparison: CandidateComparison?, checks: CandidateCheckReport?, draft: DeclarationDraft?, editor: EditableDraftState?, applied: ApplyResult?, onRunDraftChecks: () -> Unit, onApplyDraft: () -> Unit,
    onDiscard: () -> Unit, onAskForRevision: () -> Unit, onRunChecks: () -> Unit, onGenerateAlternate: () -> Unit, onCompare: () -> Unit, onExport: () -> Unit,
    comparisonBaseNote: String, comparisonCandidateNote: String, onComparisonBaseNote: (String) -> Unit, onComparisonCandidateNote: (String) -> Unit, onApply: () -> Unit, onUndo: () -> Unit,
    activity: List<ActivityEntry>, showActivity: Boolean, onToggleActivity: () -> Unit, impact: ImpactPreview?, gitStatus: GitStatus?, findings: List<UnifiedFinding>, scan: GoScanReport?, analyzeAll: AnalyzeAllJob?, coverage: AnalysisCoverage?, onOpenFinding: (UnifiedFinding) -> Unit, onPrepareFinding: (UnifiedFinding) -> Unit, onTriageFinding: (UnifiedFinding, FindingLifecycleAction) -> Unit,
    onStartAnalyzeAll: (AnalyzeAllRunOptions) -> Unit, onPauseAnalyzeAll: () -> Unit, onResumeAnalyzeAll: (Boolean) -> Unit, onCancelAnalyzeAll: () -> Unit, onStartScan: () -> Unit, onCancelScan: () -> Unit,
    focusedLine: Int, showCompactEditorBrief: Boolean, onWorkspace: (Workspace) -> Unit, modifier: Modifier,
) {
    Column(modifier.background(AppBackground)) {
        InfoStrip(project, selected)
        when (workspace) {
            Workspace.Summary -> ProjectSummaryPane(overview, project, onWorkspace)
            Workspace.Editor -> when {
                draft != null -> DraftReviewPane(project, selected, editor, draft, checks, impact, gitStatus, applied, onRunDraftChecks, onApplyDraft, onUndo)
                candidate == null -> EditorPane(project, selected, symbols, analysis, selectedSymbol, focusedLine, showCompactEditorBrief, remoteProvider, remoteProviderConfirmed, onRemoteProviderConfirmed, onSelectSymbol, onAnalyze, onRefreshAnalysis)
                else -> ReviewPane(candidate, comparisonBase, comparison, checks, applied, selected, onDiscard, onAskForRevision, onRunChecks, onGenerateAlternate, onCompare, onExport, comparisonBaseNote, comparisonCandidateNote, onComparisonBaseNote, onComparisonCandidateNote, onApply, onUndo, activity, showActivity, onToggleActivity, impact, gitStatus)
            }
            Workspace.Analysis -> AnalysisWorkspacePane(analyzeAll, coverage, remoteProvider, remoteProviderConfirmed, onRemoteProviderConfirmed, onStartAnalyzeAll, onPauseAnalyzeAll, onResumeAnalyzeAll, onCancelAnalyzeAll) { path -> onOpenFinding(UnifiedFinding(location = FindingLocation(path = path))) }
            Workspace.Bugs -> BugsWorkspacePane(findings, scan, onOpenFinding, onPrepareFinding, onTriageFinding, onStartScan, onCancelScan)
        }
    }
}

@Composable
private fun WorkspacePlaceholder(title: String, detail: String) {
    Column(Modifier.fillMaxSize().padding(18.dp)) {
        Text(title, color = PrimaryText, fontSize = 18.sp, fontWeight = FontWeight.SemiBold)
        Spacer(Modifier.height(8.dp))
        Text(detail, color = SecondaryText, fontSize = 13.sp)
    }
}

@Composable
private fun InfoStrip(project: ProjectAnalysis?, file: ProjectFileInfo?) {
    Row(Modifier.fillMaxWidth().background(Panel).border(BorderStroke(1.dp, Border)).padding(10.dp), horizontalArrangement = androidx.compose.foundation.layout.Arrangement.spacedBy(8.dp)) {
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
            detectDragGestures(
                onDrag = { change, amount ->
                    change.consume()
                    onDelta(with(density) { amount.x.toDp().value })
                },
                onDragEnd = onCommit,
            )
        },
    )
}

@Composable
private fun DesktopStatusBar(status: String, error: String?, connection: ConnectionState, onReconnect: () -> Unit) {
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

@Composable
private fun ContextInspectorDialog(manifest: ContextManifest, remoteProvider: Boolean, onDismiss: () -> Unit) {
    AlertDialog(onDismissRequest = onDismiss, title = { Text("Context inspector") }, text = {
        Column(Modifier.verticalScroll(rememberScrollState())) {
            Text("${manifest.estimatedTokens} / ${manifest.tokenLimit.takeIf { it > 0 } ?: "?"} estimated tokens${if (manifest.truncated) " · truncated" else ""}", color = SecondaryText, fontSize = 12.sp)
            if (manifest.byteLimit > 0) Text("${formatBytes(manifest.byteLimit.toLong())} byte limit", color = SecondaryText, fontSize = 11.sp)
            if (remoteProvider) Text("Warning: this provider is not loopback/local. Confirm the destination before sending project context.", color = Warning, fontSize = 12.sp, modifier = Modifier.padding(top = 8.dp))
            Text("Included", fontWeight = FontWeight.SemiBold, modifier = Modifier.padding(top = 10.dp))
            manifest.included.forEach { Text(it.path, fontSize = 11.sp) }
            Text("Excluded", fontWeight = FontWeight.SemiBold, modifier = Modifier.padding(top = 10.dp))
            manifest.excluded.forEach { Text("${it.path} · ${it.reason}", color = SecondaryText, fontSize = 11.sp) }
        }
    }, confirmButton = { Button(onClick = onDismiss) { Text("Close") } })
}
