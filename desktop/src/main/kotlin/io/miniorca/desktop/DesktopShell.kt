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
import androidx.compose.material.CircularProgressIndicator
import androidx.compose.material.DrawerValue
import androidx.compose.material.ModalDrawer
import androidx.compose.material.Surface
import androidx.compose.material.Text
import androidx.compose.material.rememberDrawerState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
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

internal enum class DesktopShellMode { ProjectLanding, ProjectWorkspace }

internal fun desktopShellMode(appState: DesktopState): DesktopShellMode =
    if (appState.project == null) DesktopShellMode.ProjectLanding else DesktopShellMode.ProjectWorkspace

internal fun narrowDrawerLabel(drawer: NarrowDrawer): String = when (drawer) {
    NarrowDrawer.Files -> "Files"
    NarrowDrawer.Context -> "Context"
}

fun useNarrowLayout(widthDp: Float): Boolean = widthDp < 1000f

internal fun editorChromeVisible(workspace: Workspace): Boolean = workspace == Workspace.Editor

internal fun editorDrawerActionsVisible(workspace: Workspace, widthDp: Float): Boolean =
    useNarrowLayout(widthDp) && editorChromeVisible(workspace)

internal fun contextDrawerForSourceSelection(workspace: Workspace, widthDp: Float): NarrowDrawer? =
    NarrowDrawer.Context.takeIf { editorDrawerActionsVisible(workspace, widthDp) }

internal fun fileInspectionWorkspace(): Workspace = Workspace.Editor

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

/** Immutable shell state assembled from feature-specific desktop workflow state. */
internal data class DesktopShellState(
    val app: DesktopState,
    val paneWidths: PaneWidths,
    val editor: DesktopShellEditorState,
    val context: DesktopShellContextState,
    val palette: DesktopShellPaletteState,
)

internal data class DesktopShellEditorState(
    val progress: EditorProgressUiState,
    val contextualActions: EditorContextualActions,
    val analysisInProgress: Boolean,
    val generating: Boolean,
)

internal data class DesktopShellContextState(
    val visible: Boolean,
    val manifest: ContextManifest?,
    val bugModel: ScopedModel,
    val bugProviderConfirmed: Boolean,
)

internal data class DesktopShellPaletteState(
    val mode: PaletteMode,
    val query: String,
    val visible: Boolean,
)

internal data class DesktopShellLayoutActions(
    val updatePaneWidths: (PaneWidths) -> Unit,
    val savePaneWidths: () -> Unit,
)

internal data class DesktopShellProjectActions(
    val importProject: () -> Unit,
    val reanalyzeProject: () -> Unit,
    val reconnect: () -> Unit,
)

internal data class DesktopShellEditorActions(
    val selectWorkspace: (Workspace) -> Unit,
    val focusChat: () -> Unit,
    val focusDraft: () -> Unit,
    val cancelAnalysis: () -> Unit,
    val sourceLineSelected: (SourceLineSelection) -> Unit,
    val validateDraft: () -> Unit,
    val runDraftChecks: () -> Unit,
    val generate: () -> Unit,
    val cancelGeneration: () -> Unit,
    val dismissContext: () -> Unit,
)

internal data class DesktopShellAnalysisActions(
    val confirmBugProvider: (Boolean) -> Unit,
    val startAnalyzeAll: (AnalyzeAllRunOptions) -> Unit,
    val pauseAnalyzeAll: () -> Unit,
    val resumeAnalyzeAll: (Boolean) -> Unit,
    val cancelAnalyzeAll: () -> Unit,
    val startScan: () -> Unit,
    val cancelScan: () -> Unit,
)

internal data class DesktopShellFindingActions(
    val openFinding: (UnifiedFinding) -> Unit,
    val prepareFinding: (UnifiedFinding) -> Unit,
    val triageFinding: (UnifiedFinding, FindingLifecycleAction) -> Unit,
)

internal data class DesktopShellPaletteActions(
    val updateQuery: (String) -> Unit,
    val dismiss: () -> Unit,
    val open: (PaletteMode) -> Unit,
    val selectFile: (String) -> Unit,
    val selectSymbol: (SymbolInfo) -> Unit,
    val selectAction: (String) -> Unit,
)

internal data class DesktopShellPanes(
    val explorer: @Composable (Modifier, () -> Unit) -> Unit,
    val context: @Composable (Modifier) -> Unit,
)

@Composable
internal fun DesktopShell(
    state: DesktopShellState,
    layoutActions: DesktopShellLayoutActions,
    projectActions: DesktopShellProjectActions,
    editorActions: DesktopShellEditorActions,
    analysisActions: DesktopShellAnalysisActions,
    findingActions: DesktopShellFindingActions,
    paletteActions: DesktopShellPaletteActions,
    panes: DesktopShellPanes,
) {
    val appState = state.app
    val paneWidths = state.paneWidths
    val editor = state.editor
    val context = state.context
    val palette = state.palette
    val workspace = appState.workspace
    val shellMode = desktopShellMode(appState)
    val scope = rememberCoroutineScope()
    val drawerState = rememberDrawerState(DrawerValue.Closed)
    var narrowDrawer by remember { mutableStateOf(NarrowDrawer.Files) }
    val showsEditorChrome = shellMode == DesktopShellMode.ProjectWorkspace && editorChromeVisible(workspace)
    fun openDrawer(drawer: NarrowDrawer) {
        if (!showsEditorChrome) return
        narrowDrawer = drawer
        scope.launch { drawerState.open() }
    }
    fun selectWorkspace(nextWorkspace: Workspace) {
        if (!editorChromeVisible(nextWorkspace)) scope.launch { drawerState.close() }
        editorActions.selectWorkspace(nextWorkspace)
    }
    LaunchedEffect(showsEditorChrome) {
        if (!showsEditorChrome) drawerState.close()
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
            val shortcut = desktopShortcut(key, event.isMetaPressed || event.isCtrlPressed, event.isShiftPressed)
            if (!shortcutAvailable(shellMode, shortcut)) return@onPreviewKeyEvent false
            when (shortcut) {
                DesktopShortcut.OpenProject -> if (!appState.loading) projectActions.importProject()
                DesktopShortcut.OpenFile -> paletteActions.open(PaletteMode.Files)
                DesktopShortcut.OpenSymbol -> paletteActions.open(PaletteMode.Symbols)
                DesktopShortcut.OpenAction -> paletteActions.open(PaletteMode.Actions)
                DesktopShortcut.FocusChat -> if (editor.contextualActions.canFocusChat) editorActions.focusChat() else return@onPreviewKeyEvent false
                DesktopShortcut.FocusDraft -> if (editor.contextualActions.canFocusDraft) editorActions.focusDraft() else return@onPreviewKeyEvent false
                DesktopShortcut.FocusBugsFilters -> selectWorkspace(Workspace.Bugs)
                DesktopShortcut.ValidateDraft -> if (editor.contextualActions.canValidateDraft) editorActions.validateDraft() else return@onPreviewKeyEvent false
                DesktopShortcut.RunDraftChecks -> if (editor.contextualActions.canRunFocusedChecks) editorActions.runDraftChecks() else return@onPreviewKeyEvent false
                DesktopShortcut.SummaryWorkspace -> selectWorkspace(Workspace.Summary)
                DesktopShortcut.AnalysisWorkspace -> selectWorkspace(Workspace.Analysis)
                DesktopShortcut.BugsWorkspace -> selectWorkspace(Workspace.Bugs)
                DesktopShortcut.EditorWorkspace -> selectWorkspace(Workspace.Editor)
                DesktopShortcut.Generate -> if (editor.generating) editorActions.cancelGeneration() else if (editor.contextualActions.canGenerate) editorActions.generate() else return@onPreviewKeyEvent false
                DesktopShortcut.Cancel -> when {
                    palette.visible -> paletteActions.dismiss()
                    context.visible -> editorActions.dismissContext()
                    editor.generating -> editorActions.cancelGeneration()
                    editor.analysisInProgress -> editorActions.cancelAnalysis()
                    else -> return@onPreviewKeyEvent false
                }
                DesktopShortcut.NextTab -> selectWorkspace(nextWorkspace(workspace))
                null -> return@onPreviewKeyEvent false
            }
            true
        },
        color = AppBackground,
    ) {
        if (shellMode == DesktopShellMode.ProjectLanding) {
            ProjectLanding(appState, projectActions.importProject)
        } else {
            BoxWithConstraints {
                val widthDp = maxWidth.value
                val narrow = useNarrowLayout(widthDp)
                val showEditorDrawers = editorDrawerActionsVisible(workspace, widthDp)
                ModalDrawer(
                drawerState = drawerState,
                drawerContent = {
                    if (showsEditorChrome) {
                        if (narrowDrawer == NarrowDrawer.Files) {
                            panes.explorer(Modifier.fillMaxHeight().width(320.dp)) { scope.launch { drawerState.close() } }
                        } else {
                            panes.context(Modifier.fillMaxHeight().width(360.dp))
                        }
                    }
                },
                ) {
                    Column {
                    AppTopBar(appState.project, appState.loading, appState.connection, projectActions.importProject, projectActions.reanalyzeProject, projectActions.reconnect, { paletteActions.open(PaletteMode.Actions) }, showEditorDrawers, { openDrawer(NarrowDrawer.Files) }, { openDrawer(NarrowDrawer.Context) })
                    Row(modifier = Modifier.weight(1f).fillMaxWidth()) {
                        WorkspaceRail(workspace, ::selectWorkspace, Modifier.width(176.dp).fillMaxHeight())
                        if (!narrow && showsEditorChrome) {
                            panes.explorer(Modifier.width(paneWidths.explorer.dp).fillMaxHeight()) {}
                            ResizableDivider(onDelta = { layoutActions.updatePaneWidths(paneWidths.withExplorer(paneWidths.explorer + it)) }, onCommit = layoutActions.savePaneWidths)
                        }
                        ContentPane(
                            state = ContentPaneState(
                                project = appState.project,
                                overview = appState.overview,
                                selected = appState.selectedFile,
                                symbols = appState.symbols,
                                selectedSymbol = appState.selectedSymbol,
                                workspace = workspace,
                                editorProgress = editor.progress,
                                draft = appState.review.draft,
                                focusedLine = appState.selection.focusedLine,
                                analysis = AnalysisWorkspacePaneState(
                                    job = appState.findings.analyzeAll,
                                    coverage = appState.overview?.analysisCoverage,
                                    model = context.bugModel,
                                    remoteProviderConfirmed = context.bugProviderConfirmed,
                                ),
                                bugs = BugsWorkspacePaneState(appState.findings.findings, appState.findings.scan),
                            ),
                            navigation = ContentPaneNavigationActions(
                                selectWorkspace = ::selectWorkspace,
                                sourceLineSelected = { selection ->
                                    editorActions.sourceLineSelected(selection)
                                    contextDrawerForSourceSelection(workspace, widthDp)?.let(::openDrawer)
                                },
                            ),
                            analysisActions = analysisActions.toWorkspaceActions(),
                            bugsActions = BugsWorkspaceActions(
                                openFinding = findingActions.openFinding,
                                prepareFinding = findingActions.prepareFinding,
                                triageFinding = findingActions.triageFinding,
                                startScan = analysisActions.startScan,
                                cancelScan = analysisActions.cancelScan,
                            ),
                            modifier = Modifier.weight(1f).fillMaxHeight(),
                        )
                        if (!narrow && showsEditorChrome) {
                            ResizableDivider(onDelta = { layoutActions.updatePaneWidths(paneWidths.withAction(paneWidths.action - it)) }, onCommit = layoutActions.savePaneWidths)
                            panes.context(Modifier.width(paneWidths.action.dp).fillMaxHeight())
                        }
                    }
                        DesktopStatusBar(appState.status, appState.error, appState.loading)
                    }
                }
                if (palette.visible) {
                    CommandPaletteDialog(
                        palette.mode,
                        palette.query,
                        paletteActions.updateQuery,
                        appState.index?.files.orEmpty(),
                        appState.symbols,
                        appState.analysis,
                        paletteActions.selectFile,
                        { symbol ->
                            paletteActions.selectSymbol(symbol)
                            contextDrawerForSourceSelection(Workspace.Editor, widthDp)?.let(::openDrawer)
                        },
                        paletteActions.selectAction,
                        paletteActions.dismiss,
                    )
                }
            }
            if (context.visible) ContextInspectorDialog(context.manifest ?: ContextManifest(), editorActions.dismissContext)
        }
    }
}

@Composable
private fun ProjectLanding(appState: DesktopState, onOpenProject: () -> Unit) {
    Box(Modifier.fillMaxSize().padding(24.dp), contentAlignment = Alignment.Center) {
        Column(horizontalAlignment = Alignment.CenterHorizontally) {
            MiniOrcaMark()
            Spacer(Modifier.height(12.dp))
            Text("Mini-Orca", color = PrimaryText, fontSize = 22.sp, fontWeight = FontWeight.SemiBold)
            FocusFlowButton(
                onClick = onOpenProject,
                enabled = !appState.loading,
                tone = ActionTone.Primary,
                modifier = Modifier.padding(top = 20.dp),
            ) { Text("Open project") }
            when {
                appState.loading -> {
                    Row(Modifier.padding(top = 12.dp), verticalAlignment = Alignment.CenterVertically) {
                        CircularProgressIndicator(Modifier.size(16.dp), color = CyanAccent, strokeWidth = 2.dp)
                        Spacer(Modifier.width(8.dp))
                        Text("Opening project…", color = SecondaryText, fontSize = 12.sp)
                    }
                }
                appState.error != null -> Text("Could not open project. ${appState.error}", color = Error, fontSize = 12.sp, modifier = Modifier.padding(top = 12.dp))
            }
        }
    }
}

@Composable
private fun ContentPane(
    state: ContentPaneState,
    navigation: ContentPaneNavigationActions,
    analysisActions: AnalysisWorkspaceActions,
    bugsActions: BugsWorkspaceActions,
    modifier: Modifier,
) {
    Column(modifier.background(AppBackground)) {
        when (state.workspace) {
            Workspace.Summary -> ProjectSummaryPane(state.overview, state.project, navigation.selectWorkspace)
            Workspace.Editor -> EditorWorkspace(state.selected, canvas = {
                if (state.editorProgress.progress == EditorProgress.Review) {
                    ReviewDiffCanvas(state.draft)
                } else {
                    EditorPane(state.project, state.selected, state.symbols, state.selectedSymbol, state.focusedLine, navigation.sourceLineSelected)
                }
            })
            Workspace.Analysis -> AnalysisWorkspacePane(state.analysis, analysisActions)
            Workspace.Bugs -> BugsWorkspacePane(state.bugs, bugsActions)
        }
    }
}

private data class ContentPaneState(
    val project: ProjectAnalysis?,
    val overview: ProjectOverview?,
    val selected: ProjectFileInfo?,
    val symbols: List<SymbolInfo>,
    val selectedSymbol: SymbolInfo?,
    val workspace: Workspace,
    val editorProgress: EditorProgressUiState,
    val draft: DeclarationDraft?,
    val focusedLine: Int,
    val analysis: AnalysisWorkspacePaneState,
    val bugs: BugsWorkspacePaneState,
)

private data class ContentPaneNavigationActions(
    val selectWorkspace: (Workspace) -> Unit,
    val sourceLineSelected: (SourceLineSelection) -> Unit,
)

private fun DesktopShellAnalysisActions.toWorkspaceActions() = AnalysisWorkspaceActions(
    confirmRemoteProvider = confirmBugProvider,
    start = startAnalyzeAll,
    pause = pauseAnalyzeAll,
    resume = resumeAnalyzeAll,
    cancel = cancelAnalyzeAll,
)

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

internal fun desktopStatusBarVisible(loading: Boolean, error: String?): Boolean = loading || error != null

@Composable
private fun DesktopStatusBar(status: String, error: String?, loading: Boolean) {
    if (!desktopStatusBarVisible(loading, error)) return
    Row(Modifier.fillMaxWidth().height(30.dp).background(Panel).border(BorderStroke(1.dp, Border)).padding(horizontal = 12.dp), verticalAlignment = Alignment.CenterVertically) {
        Box(Modifier.size(7.dp).background(if (error == null) Warning else Error, RoundedCornerShape(50)))
        Spacer(Modifier.width(7.dp))
        Text(error ?: status, color = if (error == null) SecondaryText else Error, fontSize = 11.sp, maxLines = 1)
    }
}

internal fun modelDestinationLabel(scope: ModelScope, model: ScopedModel): String {
    val reasoningEffort = model.reasoningEffort.takeIf(String::isNotBlank)?.let { " · reasoning: $it" }.orEmpty()
    return if (model.remoteProvider) {
        "${scope.label}: ${model.profile} · ${model.model}$reasoningEffort · remote provider · confirmation required before sending project context"
    } else {
        "${scope.label}: ${model.profile} · ${model.model}$reasoningEffort · local provider · project context stays on this machine"
    }
}

internal fun contextManifestSummary(manifest: ContextManifest): String =
    "${manifest.included.size} included · ${manifest.excluded.size} excluded · ${manifest.estimatedTokens} estimated tokens${if (manifest.truncated) " · truncated" else ""}"

@Composable
private fun ContextInspectorDialog(manifest: ContextManifest, onDismiss: () -> Unit) {
    AlertDialog(onDismissRequest = onDismiss, title = { Text("Context inspector · read-only") }, text = {
        Column(Modifier.verticalScroll(rememberScrollState())) {
            FocusFlowPanel(Modifier.fillMaxWidth(), raised = true) {
                SectionLabel("DESTINATION")
                val scope = ModelScope.entries.firstOrNull { it.wireValue == manifest.scope }?.label ?: manifest.scope.ifBlank { "Function edits" }
                val provider = if (manifest.remoteProvider) "remote provider · confirmation required before sending project context" else "local provider · project context stays on this machine"
                Text("$scope: ${manifest.model.ifBlank { "configured model" }} · ${manifest.providerOrigin.ifBlank { "configured destination" }} · $provider", color = if (manifest.remoteProvider) Warning else SecondaryText, fontSize = 12.sp, modifier = Modifier.padding(top = 5.dp))
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
    }, confirmButton = { FocusFlowButton(onClick = onDismiss, tone = ActionTone.Neutral) { Text("Close") } })
}
