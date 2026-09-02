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
import androidx.compose.foundation.text.selection.SelectionContainer
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
    val language: String = "",
)

internal enum class NarrowDrawer { Files, Context }

internal fun narrowDrawerLabel(drawer: NarrowDrawer): String = when (drawer) {
    NarrowDrawer.Files -> "Files"
    NarrowDrawer.Context -> "Context"
}

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
                rows += ExplorerRow(path, path.substringAfterLast('/'), path.count { it == '/' }, directory, matching[path]?.analysisStatus ?: "missing", matching[path]?.language.orEmpty())
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
    editorFlow: EditorFlowUiState,
    onEditorStage: (EditorStage) -> Unit,
    onFocusChat: () -> Unit,
    onFocusDraft: () -> Unit,
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
    contextPane: @Composable (Modifier) -> Unit,
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
    onUndo: () -> Unit,
    onGenerate: () -> Unit,
    onCancelGeneration: () -> Unit,
    onCancelAll: () -> Unit,
) {
    val scope = rememberCoroutineScope()
    val drawerState = rememberDrawerState(DrawerValue.Closed)
    var narrowDrawer by remember { mutableStateOf(NarrowDrawer.Files) }
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
                DesktopShortcut.OpenAction -> onOpenPalette(PaletteMode.Actions)
                DesktopShortcut.FocusChat -> onFocusChat()
                DesktopShortcut.FocusDraft -> onFocusDraft()
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
                    if (narrowDrawer == NarrowDrawer.Files) {
                        explorer(Modifier.fillMaxHeight().width(320.dp)) { scope.launch { drawerState.close() } }
                    } else {
                        contextPane(Modifier.fillMaxHeight().width(360.dp))
                    }
                },
            ) {
                Column {
                    AppTopBar(appState.project, appState.loading, connection, onImport, onReanalyze, { onOpenPalette(PaletteMode.Actions) }, narrow, { openDrawer(NarrowDrawer.Files) }, { openDrawer(NarrowDrawer.Context) })
                    Row(modifier = Modifier.weight(1f).fillMaxWidth()) {
                        WorkspaceRail(workspace, workspaceCounts, onWorkspace, Modifier.width(176.dp).fillMaxHeight())
                        if (!narrow) {
                            explorer(Modifier.width(paneWidths.explorer.dp).fillMaxHeight()) {}
                            ResizableDivider(onDelta = { onPaneWidths(paneWidths.withExplorer(paneWidths.explorer + it)) }, onCommit = onSavePaneWidths)
                        }
                        ContentPane(
                            project = appState.project, overview = appState.overview, selected = appState.selectedFile, symbols = appState.symbols, analysis = appState.analysis, selectedSymbol = appState.selectedSymbol,
                            workspace = workspace, analysisInProgress = analysisInProgress, onAnalyze = onAnalyze, onRefreshAnalysis = onRefreshAnalysis, onCancelAnalysis = onCancelAnalysis, remoteProvider = remoteProvider, remoteProviderConfirmed = remoteProviderConfirmed, onRemoteProviderConfirmed = onRemoteProviderConfirmed,
                            editorFlow = editorFlow, onEditorStage = onEditorStage,
                            onSelectSymbol = onSelectSymbol, onPrepareSuggestion = onPrepareSuggestion, checks = appState.checks, draft = appState.review.draft, editor = appState.review.editor, applied = appState.review.applied, onApplyDraft = onApplyDraft, onUndo = onUndo,
                            findings = appState.findings.findings, scan = appState.findings.scan, analyzeAll = appState.findings.analyzeAll, coverage = appState.overview?.analysisCoverage, onOpenFinding = onOpenFinding, onPrepareFinding = onPrepareFinding, onTriageFinding = onTriageFinding,
                            onStartAnalyzeAll = onStartAnalyzeAll, onPauseAnalyzeAll = onPauseAnalyzeAll, onResumeAnalyzeAll = onResumeAnalyzeAll, onCancelAnalyzeAll = onCancelAnalyzeAll, onStartScan = onStartScan, onCancelScan = onCancelScan,
                            focusedLine = appState.selection.focusedLine, showCompactEditorBrief = narrow, onWorkspace = onWorkspace, modifier = Modifier.weight(1f).fillMaxHeight(),
                        )
                        if (!narrow) {
                            ResizableDivider(onDelta = { onPaneWidths(paneWidths.withAction(paneWidths.action - it)) }, onCommit = onSavePaneWidths)
                            contextPane(Modifier.width(paneWidths.action.dp).fillMaxHeight())
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
    editorFlow: EditorFlowUiState, onEditorStage: (EditorStage) -> Unit,
    onSelectSymbol: (SymbolInfo) -> Unit, onPrepareSuggestion: (Suggestion) -> Unit, checks: CandidateCheckReport?, draft: DeclarationDraft?, editor: EditableDraftState?, applied: ApplyResult?, onApplyDraft: () -> Unit, onUndo: () -> Unit,
    findings: List<UnifiedFinding>, scan: GoScanReport?, analyzeAll: AnalyzeAllJob?, coverage: AnalysisCoverage?, onOpenFinding: (UnifiedFinding) -> Unit, onPrepareFinding: (UnifiedFinding) -> Unit, onTriageFinding: (UnifiedFinding, FindingLifecycleAction) -> Unit,
    onStartAnalyzeAll: (AnalyzeAllRunOptions) -> Unit, onPauseAnalyzeAll: () -> Unit, onResumeAnalyzeAll: (Boolean) -> Unit, onCancelAnalyzeAll: () -> Unit, onStartScan: () -> Unit, onCancelScan: () -> Unit,
    focusedLine: Int, showCompactEditorBrief: Boolean, onWorkspace: (Workspace) -> Unit, modifier: Modifier,
) {
    Column(modifier.background(AppBackground)) {
        when (workspace) {
            Workspace.Summary -> ProjectSummaryPane(overview, project, onWorkspace)
            Workspace.Editor -> EditorWorkspace(editorFlow, onEditorStage, canvas = {
                when (editorFlow.activeStage) {
                    EditorStage.Verify -> VerifyDiffCanvas(draft)
                    EditorStage.Apply -> ApplyDiffCanvas(draft, applied)
                    else -> EditorPane(project, selected, selectedSymbol, focusedLine)
                }
            })
            Workspace.Analysis -> AnalysisWorkspacePane(analyzeAll, coverage, remoteProvider, remoteProviderConfirmed, onRemoteProviderConfirmed, onStartAnalyzeAll, onPauseAnalyzeAll, onResumeAnalyzeAll, onCancelAnalyzeAll)
            Workspace.Bugs -> BugsWorkspacePane(findings, scan, onOpenFinding, onPrepareFinding, onTriageFinding, onStartScan, onCancelScan)
        }
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
        Text(error ?: connectionLabel(connection), color = if (error == null) SecondaryText else Error, fontSize = 11.sp, maxLines = 1)
        Spacer(Modifier.width(8.dp))
        Text("· Preview-first mode · ${status.ifBlank { "Ready" }}", color = SecondaryText, fontSize = 10.sp, maxLines = 1)
        Spacer(Modifier.weight(1f))
        Button(onClick = onReconnect, modifier = Modifier.height(24.dp), contentPadding = androidx.compose.foundation.layout.PaddingValues(horizontal = 7.dp, vertical = 0.dp)) { Text("Reconnect", fontSize = 10.sp) }
        Spacer(Modifier.width(8.dp))
        Text(if (connection.version.isBlank()) "Mini-Orca" else "Mini-Orca v${connection.version}", color = SecondaryText, fontSize = 10.sp)
    }
}

internal fun contextDestinationLabel(remoteProvider: Boolean): String = if (remoteProvider) {
    "Destination: remote provider · confirmation required before sending project context"
} else {
    "Destination: local provider · project context stays on this machine"
}

internal fun contextManifestSummary(manifest: ContextManifest): String =
    "${manifest.included.size} included · ${manifest.excluded.size} excluded · ${manifest.estimatedTokens} estimated tokens${if (manifest.truncated) " · truncated" else ""}"

@Composable
private fun ContextInspectorDialog(manifest: ContextManifest, remoteProvider: Boolean, onDismiss: () -> Unit) {
    AlertDialog(onDismissRequest = onDismiss, title = { Text("Context inspector · read-only") }, text = {
        Column(Modifier.verticalScroll(rememberScrollState())) {
            FocusFlowPanel(Modifier.fillMaxWidth(), raised = true) {
                SectionLabel("DESTINATION")
                Text(contextDestinationLabel(remoteProvider), color = if (remoteProvider) Warning else SecondaryText, fontSize = 12.sp, modifier = Modifier.padding(top = 5.dp))
            }
            Text(contextManifestSummary(manifest), color = PrimaryText, fontSize = 12.sp, modifier = Modifier.padding(top = 10.dp))
            if (manifest.tokenLimit > 0) Text("Token budget: ${manifest.estimatedTokens} / ${manifest.tokenLimit}", color = SecondaryText, fontSize = 11.sp)
            if (manifest.byteLimit > 0) Text("Byte limit: ${formatBytes(manifest.byteLimit.toLong())}", color = SecondaryText, fontSize = 11.sp)
            SelectionContainer {
                Column {
                    Text("Included", color = PrimaryText, fontWeight = FontWeight.SemiBold, modifier = Modifier.padding(top = 10.dp))
                    if (manifest.included.isEmpty()) Text("No files included.", color = SecondaryText, fontSize = 11.sp)
                    manifest.included.forEach { Text("${it.path} · ${formatBytes(it.sizeBytes)} · ${it.estimatedTokens} tokens", color = SecondaryText, fontSize = 11.sp) }
                    Text("Excluded", color = PrimaryText, fontWeight = FontWeight.SemiBold, modifier = Modifier.padding(top = 10.dp))
                    if (manifest.excluded.isEmpty()) Text("No files excluded.", color = SecondaryText, fontSize = 11.sp)
                    manifest.excluded.forEach { Text("${it.path} · ${it.reason}", color = SecondaryText, fontSize = 11.sp) }
                }
            }
        }
    }, confirmButton = { Button(onClick = onDismiss) { Text("Close") } })
}
