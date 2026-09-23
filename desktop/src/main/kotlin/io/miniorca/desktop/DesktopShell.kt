package io.miniorca.desktop

import androidx.compose.foundation.background
import androidx.compose.foundation.focusable
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
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusManager
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.input.key.KeyEvent
import androidx.compose.ui.input.key.KeyEventType
import androidx.compose.ui.input.key.isCtrlPressed
import androidx.compose.ui.input.key.isMetaPressed
import androidx.compose.ui.input.key.isShiftPressed
import androidx.compose.ui.input.key.key
import androidx.compose.ui.input.key.onPreviewKeyEvent
import androidx.compose.ui.input.key.type
import androidx.compose.ui.platform.LocalFocusManager
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.launch

data class ExplorerRow(
    val path: String,
    val name: String,
    val depth: Int,
    val directory: Boolean,
    val analysisStatus: String = "missing",
    val language: String = "",
)

internal enum class DesktopShellMode {
  ProjectLanding,
  ProjectWorkspace
}

internal fun desktopShellMode(appState: DesktopState): DesktopShellMode =
    if (appState.project == null) DesktopShellMode.ProjectLanding
    else DesktopShellMode.ProjectWorkspace

fun useNarrowLayout(widthDp: Float): Boolean = widthDp < 1000f

internal fun editorChromeVisible(workspace: Workspace): Boolean = workspace == Workspace.Editor

internal fun fileInspectionWorkspace(): Workspace = Workspace.Editor

/** Builds a stable project-relative explorer without exposing filesystem paths. */
fun explorerRows(files: List<IndexedFile>, filter: String = ""): List<ExplorerRow> {
  val matching =
      files
          .filter { filter.isBlank() || it.path.contains(filter, ignoreCase = true) }
          .associateBy { it.path }
  if (matching.isEmpty()) return emptyList()

  val directories = sortedSetOf<String>()
  matching.keys.forEach { path ->
    path
        .substringBeforeLast('/', "")
        .takeIf { it.isNotBlank() }
        ?.let { parent ->
          parent.split('/').indices.forEach { index ->
            directories += parent.split('/').take(index + 1).joinToString("/")
          }
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
    children[parent]
        .orEmpty()
        .sortedWith(
            compareBy<String> { it !in directories }
                .thenBy { it.substringAfterLast('/').lowercase() })
        .forEach { path ->
          val directory = path in directories
          rows +=
              ExplorerRow(
                  path,
                  path.substringAfterLast('/'),
                  path.count { it == '/' },
                  directory,
                  matching[path]?.analysisStatus ?: "missing",
                  matching[path]?.language.orEmpty())
          if (directory) appendChildren(path)
        }
  }
  appendChildren("")
  return rows
}

fun explorerDirectories(files: List<IndexedFile>): Set<String> =
    explorerRows(files).filter { it.directory }.mapTo(linkedSetOf()) { it.path }

fun visibleExplorerRows(
    files: List<IndexedFile>,
    filter: String,
    collapsedDirectories: Set<String>
): List<ExplorerRow> {
  val rows = explorerRows(files, filter)
  if (filter.isNotBlank()) return rows
  return rows.filter { row -> collapsedDirectories.none { row.path.startsWith("$it/") } }
}

/** Immutable shell state assembled from feature-specific desktop workflow state. */
internal data class DesktopShellState(
    val app: DesktopState,
    val layout: DesktopLayoutState,
    val editor: DesktopShellEditorState,
    val context: DesktopShellContextState,
    val palette: DesktopShellPaletteState,
    val statusProviders: DesktopShellStatusProviders,
)

internal data class DesktopShellStatusProviders(
    val analyze: ScopedModel,
    val bugs: ScopedModel,
    val functionEdits: ScopedModel,
)

internal fun statusProviderForWorkspace(
    workspace: Workspace,
    providers: DesktopShellStatusProviders,
): DesktopStatusProvider =
    when (workspace) {
      Workspace.Summary -> DesktopStatusProvider(ModelScope.Analyze, providers.analyze)
      Workspace.Performance -> DesktopStatusProvider(ModelScope.Analyze, providers.analyze)
      Workspace.Analysis,
      Workspace.Bugs -> DesktopStatusProvider(ModelScope.Bug, providers.bugs)
      Workspace.Security -> DesktopStatusProvider(ModelScope.Analyze, providers.analyze)
      Workspace.Editor -> DesktopStatusProvider(ModelScope.Function, providers.functionEdits)
    }

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
    val analyzeModel: ScopedModel,
    val analyzeProviderConfirmed: Boolean,
    val securityReviewRemoteConfirmed: Boolean,
)

internal data class DesktopShellPaletteState(
    val mode: PaletteMode,
    val query: String,
    val visible: Boolean,
)

internal data class DesktopShellLayoutActions(
    val updateLayout: (DesktopLayoutState) -> Unit,
    val saveLayout: (DesktopLayoutState) -> Unit,
)

internal data class DesktopShellProjectActions(
    val importProject: () -> Unit,
    val reanalyzeProject: () -> Unit,
    val reconnect: () -> Unit,
)

internal data class DesktopShellEditorActions(
    val selectWorkspace: (Workspace) -> Unit,
    val selectEditorSurface: (EditorSurface) -> Unit,
    val focusChat: () -> Unit,
    val focusDraft: () -> Unit,
    val cancelAnalysis: () -> Unit,
    val sourceLineSelected: (SourceLineSelection) -> Unit,
    val validateDraft: () -> Unit,
    val runDraftChecks: () -> Unit,
    val generate: () -> Unit,
    val cancelGeneration: () -> Unit,
    val dismissContext: () -> Unit,
    val createDeclaration: () -> Unit,
)

internal data class DesktopShellAnalysisActions(
    val refreshAnalysisSelection: () -> Unit,
    val saveAnalysisSelection: (List<String>) -> Unit,
    val startAnalysis: (AnalysisRunLimits, Boolean) -> Unit,
    val pauseAnalysis: () -> Unit,
    val resumeAnalysis: () -> Unit,
    val cancelAnalysis: () -> Unit,
    val startScan: () -> Unit,
    val cancelScan: () -> Unit,
    val preparePerformanceFinding: (String, PerformanceFinding) -> Unit,
    val loadGoBenchmarks: () -> Unit,
    val selectGoBenchmark: (GoBenchmarkChoice) -> Unit,
    val compareSelectedGoBenchmark: () -> Unit,
    val prepareSecurityFinding: (SecurityFinding) -> Unit,
)

internal data class DesktopShellPaletteActions(
    val updateQuery: (String) -> Unit,
    val dismiss: () -> Unit,
    val open: (PaletteMode) -> Unit,
    val switchMode: (PaletteMode) -> Unit,
    val selectFile: (String) -> Unit,
    val selectSymbol: (SymbolInfo) -> Unit,
    val selectAction: (String) -> Unit,
)

internal data class DesktopShellPanes(
    val explorer: @Composable (Modifier, () -> Unit) -> Unit,
    val rightToolWindows: @Composable (RightToolWindow, Modifier) -> Unit,
    val rightToolWindowBadges: Map<RightToolWindow, RightToolWindowBadge>,
    val terminalContent: @Composable (Modifier) -> Unit,
    val terminalState: TerminalWorkspaceState,
    val terminalTabActions: TerminalTabActions,
)

private data class ShellFocusRequesters(
    val fallback: FocusRequester,
    val toolbar: FocusRequester,
    val leftToolWindow: FocusRequester,
    val editor: FocusRequester,
    val rightToolWindow: FocusRequester,
    val bottomToolWindow: FocusRequester,
    val statusBar: FocusRequester,
    val paletteTrigger: FocusRequester,
)

internal fun paletteFocusRestorationRegion(
    previous: DesktopFocusRegion,
    rightToolWindowVisible: Boolean,
    bottomToolWindowVisible: Boolean,
): DesktopFocusRegion =
    when (previous) {
      DesktopFocusRegion.RightToolWindow ->
          if (rightToolWindowVisible) previous else DesktopFocusRegion.Editor
      DesktopFocusRegion.BottomToolWindow ->
          if (bottomToolWindowVisible) previous else DesktopFocusRegion.Editor
      else -> previous
    }

private fun ShellFocusRequesters.forRegion(region: DesktopFocusRegion): FocusRequester =
    when (region) {
      DesktopFocusRegion.Toolbar -> toolbar
      DesktopFocusRegion.LeftToolWindow -> leftToolWindow
      DesktopFocusRegion.Editor -> editor
      DesktopFocusRegion.RightToolWindow -> rightToolWindow
      DesktopFocusRegion.BottomToolWindow -> bottomToolWindow
      DesktopFocusRegion.StatusBar -> statusBar
    }

private fun ShellFocusRequesters.paletteRestorationRequester(
    openedFromToolbar: Boolean,
    region: DesktopFocusRegion,
): FocusRequester =
    if (openedFromToolbar && region == DesktopFocusRegion.Toolbar) paletteTrigger
    else forRegion(region)

@Composable
internal fun DesktopShell(
    state: DesktopShellState,
    layoutActions: DesktopShellLayoutActions,
    projectActions: DesktopShellProjectActions,
    editorActions: DesktopShellEditorActions,
    analysisActions: DesktopShellAnalysisActions,
    findingActions: FindingActions,
    paletteActions: DesktopShellPaletteActions,
    panes: DesktopShellPanes,
    terminal: DesktopTerminalWorkspace? = null,
) {
  val appState = state.app
  val layout = state.layout
  val editor = state.editor
  val context = state.context
  val palette = state.palette
  val statusPresentation = desktopStatusBarPresentation(appState, state.statusProviders)
  val workspace = appState.workspace
  val shellMode = desktopShellMode(appState)
  val scope = rememberCoroutineScope()
  val resultBrowsers = remember { ResultBrowserStore() }
  resultBrowsers.resetFor(appState.project, appState.analysisRun.run)
  val focusManager = LocalFocusManager.current
  val focusRequesters = remember {
    ShellFocusRequesters(
        fallback = FocusRequester(),
        toolbar = FocusRequester(),
        leftToolWindow = FocusRequester(),
        editor = FocusRequester(),
        rightToolWindow = FocusRequester(),
        bottomToolWindow = FocusRequester(),
        statusBar = FocusRequester(),
        paletteTrigger = FocusRequester(),
    )
  }
  var paletteFocusRestoreTarget by remember { mutableStateOf<DesktopFocusRegion?>(null) }
  var paletteOpenedFromToolbar by remember { mutableStateOf(false) }
  var statusDetailsVisible by remember { mutableStateOf(false) }
  var statusDetailsFocusRestoreTarget by remember { mutableStateOf<DesktopFocusRegion?>(null) }
  var contextFocusRestoreTarget by remember { mutableStateOf<DesktopFocusRegion?>(null) }
  val showsEditorChrome =
      shellMode == DesktopShellMode.ProjectWorkspace && editorChromeVisible(workspace)
  fun selectWorkspace(nextWorkspace: Workspace) {
    layoutActions.updateLayout(
        layout
            .openLeft(leftToolWindowForWorkspace(nextWorkspace))
            .withFocus(DesktopFocusRegion.Editor))
    editorActions.selectWorkspace(nextWorkspace)
  }
  fun selectToolWindow(toolWindow: LeftToolWindow) {
    layoutActions.updateLayout(
        layout.openLeft(toolWindow).withFocus(DesktopFocusRegion.LeftToolWindow))
    val nextWorkspace = workspaceForLeftToolWindow(toolWindow)
    editorActions.selectWorkspace(nextWorkspace)
  }
  fun selectRightToolWindow(toolWindow: RightToolWindow) {
    layoutActions.updateLayout(
        layout.openRight(toolWindow).withFocus(DesktopFocusRegion.RightToolWindow))
  }
  fun openTerminal() {
    appState.project?.path?.let { terminal?.activate(it) }
    val updated = layout.openTerminal().withFocus(DesktopFocusRegion.BottomToolWindow)
    layoutActions.updateLayout(updated)
    layoutActions.saveLayout(updated)
  }
  fun collapseTerminal() {
    val updated = layout.withBottomCollapsed(true).withFocus(DesktopFocusRegion.BottomToolWindow)
    layoutActions.updateLayout(updated)
    layoutActions.saveLayout(updated)
  }
  TerminalFocusReturnEffect(terminal) {
    editorActions.selectWorkspace(Workspace.Editor)
    scope.launch { restoreTerminalEditorFocus(focusManager, focusRequesters.editor) }
  }
  LaunchedEffect(Unit) { focusRequesters.fallback.requestFocus() }
  LaunchedEffect(appState.project?.projectId, appState.project?.projectRevision) {
    statusDetailsVisible = false
  }
  fun dismissPaletteAndRestoreFocus() {
    paletteFocusRestoreTarget = layout.lastFocusedRegion
    paletteActions.dismiss()
  }
  fun showStatusDetails() {
    layoutActions.updateLayout(layout.withFocus(DesktopFocusRegion.StatusBar))
    statusDetailsFocusRestoreTarget = DesktopFocusRegion.StatusBar
    statusDetailsVisible = true
  }
  fun dismissStatusDetailsAndRestoreFocus() {
    statusDetailsFocusRestoreTarget = DesktopFocusRegion.StatusBar
    statusDetailsVisible = false
  }
  fun dismissContextAndRestoreFocus() {
    contextFocusRestoreTarget = layout.lastFocusedRegion
    editorActions.dismissContext()
  }
  fun dismissTopmostTransient(): Boolean =
      when (topmostTransientSurface(
          contextVisible = context.visible,
          paletteVisible = palette.visible,
          statusDetailsVisible = statusDetailsVisible,
      )) {
        TransientSurface.Context -> {
          dismissContextAndRestoreFocus()
          true
        }
        TransientSurface.Palette -> {
          dismissPaletteAndRestoreFocus()
          true
        }
        TransientSurface.StatusDetails -> {
          dismissStatusDetailsAndRestoreFocus()
          true
        }
        null -> false
      }
  LaunchedEffect(statusDetailsVisible, statusDetailsFocusRestoreTarget) {
    if (!statusDetailsVisible && statusDetailsFocusRestoreTarget != null) {
      focusRequesters.forRegion(statusDetailsFocusRestoreTarget!!).requestFocus()
      statusDetailsFocusRestoreTarget = null
    }
  }
  LaunchedEffect(context.visible, contextFocusRestoreTarget) {
    if (!context.visible && contextFocusRestoreTarget != null) {
      focusRequesters.forRegion(contextFocusRestoreTarget!!).requestFocus()
      contextFocusRestoreTarget = null
    }
  }
  Box(
      modifier =
          Modifier.fillMaxSize()
              .background(AppBackground)
              .focusRequester(focusRequesters.fallback)
              .focusable()
              .onPreviewKeyEvent { event ->
                handleDesktopShortcut(
                    event = event,
                    terminal = terminal,
                    onTerminalSelected = ::openTerminal,
                    shellMode = shellMode,
                    appState = appState,
                    editor = editor,
                    projectActions = projectActions,
                    editorActions = editorActions,
                    paletteActions = paletteActions,
                    onDismissTransient = ::dismissTopmostTransient,
                    onWorkspaceSelected = ::selectWorkspace,
                )
              },
  ) {
    if (shellMode == DesktopShellMode.ProjectLanding) {
      ProjectLanding(appState, projectActions.importProject)
    } else {
      BoxWithConstraints {
        val widthDp = maxWidth.value
        val restoredFocusRegion =
            paletteFocusRestorationRegion(
                previous = paletteFocusRestoreTarget ?: layout.lastFocusedRegion,
                rightToolWindowVisible = showsEditorChrome && layout.rightToolWindowVisible,
                bottomToolWindowVisible = true,
            )
        LaunchedEffect(palette.visible, paletteFocusRestoreTarget, restoredFocusRegion) {
          if (!palette.visible && paletteFocusRestoreTarget != null) {
            focusRequesters
                .paletteRestorationRequester(paletteOpenedFromToolbar, restoredFocusRegion)
                .requestFocus()
            paletteFocusRestoreTarget = null
            paletteOpenedFromToolbar = false
          }
        }
        Column {
          MainToolbar(
              state =
                  ToolbarState(
                      widthDp = widthDp,
                      project = appState.project,
                      busy = appState.loading,
                      operationStatus = appState.status,
                      connection = appState.connection,
                      gitStatus = appState.gitStatus,
                      analysisStatus = toolbarAnalysisStatus(appState),
                  ),
              actions =
                  ToolbarActions(
                      onImport = projectActions.importProject,
                      onReanalyze = projectActions.reanalyzeProject,
                      onReconnect = projectActions.reconnect,
                      onPalette = {
                        layoutActions.updateLayout(layout.withFocus(DesktopFocusRegion.Toolbar))
                        paletteOpenedFromToolbar = true
                        paletteActions.open(PaletteMode.Files)
                      },
                  ),
              modifier = Modifier.focusRequester(focusRequesters.toolbar).focusable(),
              paletteFocusRequester = focusRequesters.paletteTrigger,
          )
          val dockedWidths = dockedPaneWidths(widthDp, layout.explorerWidth, layout.actionWidth)
          WorkspaceFrame(
              rail = {
                ToolWindowBar(
                    leftToolWindowForWorkspace(workspace),
                    ::selectToolWindow,
                    Modifier.focusRequester(focusRequesters.leftToolWindow))
              },
              panes = {
                if (showsEditorChrome && layout.leftToolWindowVisible) {
                  DockedToolWindow(
                      "Files",
                      content = { modifier -> panes.explorer(modifier) {} },
                      modifier = Modifier.width(dockedWidths.explorer.dp).fillMaxHeight(),
                      // Explorer owns its Files heading and actions in a docked layout.
                      showHeader = false)
                  ResizableDivider(
                      onDelta = {
                        layoutActions.updateLayout(
                            layout.withExplorerWidth(layout.explorerWidth + it))
                      },
                      onCommit = { layoutActions.saveLayout(layout) })
                }
                DesktopCanvas(
                    state = state,
                    resultBrowsers = resultBrowsers,
                    widthDp = widthDp,
                    editorActions = editorActions,
                    analysisActions = analysisActions,
                    findingActions = findingActions,
                    onWorkspaceSelected = ::selectWorkspace,
                    modifier =
                        Modifier.weight(1f)
                            .fillMaxHeight()
                            .focusRequester(focusRequesters.editor)
                            .focusable(),
                )
                if (showsEditorChrome && layout.rightToolWindowVisible) {
                  ResizableDivider(
                      onDelta = {
                        layoutActions.updateLayout(layout.withActionWidth(layout.actionWidth - it))
                      },
                      onCommit = { layoutActions.saveLayout(layout) })
                  DockedToolWindow(
                      "Tool windows",
                      content = { modifier ->
                        CompositionLocalProvider(LocalContextCreationActionVisible provides false) {
                          RightToolWindowContainer(
                              layout.activeRightToolWindow,
                              ::selectRightToolWindow,
                              panes.rightToolWindows,
                              panes.rightToolWindowBadges,
                              modifier.focusRequester(focusRequesters.rightToolWindow))
                        }
                      },
                      modifier = Modifier.width(dockedWidths.action.dp).fillMaxHeight(),
                      // The right-window tabs identify their own active content.
                      showHeader = false)
                }
              },
              terminal = {
                TerminalDock(
                    layout = layout,
                    state = panes.terminalState,
                    tabActions = panes.terminalTabActions,
                    onOpen = ::openTerminal,
                    onCollapse = ::collapseTerminal,
                    onHeightDelta = {
                      layoutActions.updateLayout(layout.withBottomHeight(layout.bottomHeight + it))
                    },
                    onHeightCommit = { layoutActions.saveLayout(layout) },
                    content = panes.terminalContent,
                    controlModifier = Modifier.focusRequester(focusRequesters.bottomToolWindow),
                )
              },
              modifier = Modifier.weight(1f),
          )
          if (desktopStatusBarVisible(appState.project)) {
            PersistentStatusBar(
                presentation = statusPresentation,
                onOpenDetails = ::showStatusDetails,
                modifier = Modifier.focusRequester(focusRequesters.statusBar).focusable(),
            )
          }
        }
        if (palette.visible) {
          CommandPaletteDialog(
              palette.mode,
              palette.query,
              paletteActions.updateQuery,
              paletteActions.switchMode,
              appState.index?.files.orEmpty(),
              appState.symbols,
              appState.selectedFile != null,
              { path ->
                paletteActions.selectFile(path)
                paletteFocusRestoreTarget = DesktopFocusRegion.Editor
              },
              { symbol ->
                paletteActions.selectSymbol(symbol)
                paletteFocusRestoreTarget = DesktopFocusRegion.Editor
              },
              { action ->
                paletteActions.selectAction(action)
                paletteFocusRestoreTarget = layout.lastFocusedRegion
              },
              ::dismissPaletteAndRestoreFocus,
          )
        }
        if (statusDetailsVisible) {
          DesktopStatusDetailsDialog(statusPresentation, ::dismissStatusDetailsAndRestoreFocus)
        }
      }
      if (context.visible)
          ContextInspectorDialog(
              context.manifest ?: ContextManifest(), ::dismissContextAndRestoreFocus)
    }
  }
}

private suspend fun restoreTerminalEditorFocus(
    focusManager: FocusManager,
    editor: FocusRequester,
) {
  kotlinx.coroutines.yield()
  // Swing can own native focus while Compose still considers the editor focused.
  focusManager.clearFocus(force = true)
  editor.requestFocus()
}

private fun handleDesktopShortcut(
    event: KeyEvent,
    shellMode: DesktopShellMode,
    appState: DesktopState,
    editor: DesktopShellEditorState,
    projectActions: DesktopShellProjectActions,
    editorActions: DesktopShellEditorActions,
    paletteActions: DesktopShellPaletteActions,
    onDismissTransient: () -> Boolean,
    onWorkspaceSelected: (Workspace) -> Unit,
    terminal: DesktopTerminalWorkspace?,
    onTerminalSelected: () -> Unit,
): Boolean {
  if (event.type != KeyEventType.KeyDown || !appShortcutAllowed(terminal?.ownsFocus() == true))
      return false
  if (event.key == Key.T &&
      event.isCtrlPressed &&
      event.isShiftPressed &&
      appState.project != null) {
    onTerminalSelected()
    return true
  }
  val key =
      when (event.key) {
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
  val shortcut =
      desktopShortcut(key, event.isMetaPressed || event.isCtrlPressed, event.isShiftPressed)
  if (!shortcutAvailable(shellMode, shortcut)) return false
  return when (shortcut) {
    DesktopShortcut.OpenProject -> {
      if (!appState.loading) projectActions.importProject()
      true
    }
    DesktopShortcut.OpenFile -> {
      paletteActions.open(PaletteMode.Files)
      true
    }
    DesktopShortcut.OpenSymbol -> {
      paletteActions.open(PaletteMode.Symbols)
      true
    }
    DesktopShortcut.FocusChat ->
        editor.contextualActions.canFocusChat.also { if (it) editorActions.focusChat() }
    DesktopShortcut.FocusDraft ->
        editor.contextualActions.canFocusDraft.also { if (it) editorActions.focusDraft() }
    DesktopShortcut.ValidateDraft ->
        editor.contextualActions.canValidateDraft.also { if (it) editorActions.validateDraft() }
    DesktopShortcut.RunDraftChecks ->
        editor.contextualActions.canRunFocusedChecks.also { if (it) editorActions.runDraftChecks() }
    DesktopShortcut.SummaryWorkspace -> {
      onWorkspaceSelected(Workspace.Summary)
      true
    }
    DesktopShortcut.AnalysisWorkspace -> {
      onWorkspaceSelected(Workspace.Analysis)
      true
    }
    DesktopShortcut.BugsWorkspace -> {
      onWorkspaceSelected(Workspace.Bugs)
      true
    }
    DesktopShortcut.EditorWorkspace -> {
      onWorkspaceSelected(Workspace.Editor)
      true
    }
    DesktopShortcut.Generate ->
        when {
          editor.generating -> {
            editorActions.cancelGeneration()
            true
          }
          editor.contextualActions.canGenerate -> {
            editorActions.generate()
            true
          }
          else -> false
        }
    DesktopShortcut.Cancel ->
        when {
          onDismissTransient() -> true
          editor.generating -> {
            editorActions.cancelGeneration()
            true
          }
          editor.analysisInProgress -> {
            editorActions.cancelAnalysis()
            true
          }
          else -> false
        }
    DesktopShortcut.NextTab -> {
      onWorkspaceSelected(nextWorkspace(appState.workspace))
      true
    }
    null -> false
  }
}

@Composable
private fun ProjectLanding(appState: DesktopState, onOpenProject: () -> Unit) {
  Box(Modifier.fillMaxSize().padding(24.dp), contentAlignment = Alignment.Center) {
    Column(horizontalAlignment = Alignment.CenterHorizontally) {
      MiniOrcaMark()
      Spacer(Modifier.height(12.dp))
      Text("Mini-Orca", color = PrimaryText, fontSize = 22.sp, fontWeight = FontWeight.SemiBold)
      MiniOrcaButton(
          onClick = onOpenProject,
          enabled = !appState.loading,
          tone = ActionTone.Primary,
          modifier = Modifier.padding(top = 20.dp),
      ) {
        Text("Open project")
      }
      when {
        appState.loading -> {
          Row(Modifier.padding(top = 12.dp), verticalAlignment = Alignment.CenterVertically) {
            IdeBusyIndicator(Modifier.size(16.dp), color = FocusAccent, strokeWidth = 2.dp)
            Spacer(Modifier.width(8.dp))
            Text("Opening project…", color = SecondaryText, fontSize = 12.sp)
          }
        }
        appState.error != null ->
            Text(
                "Could not open project. ${appState.error}",
                color = Error,
                fontSize = 12.sp,
                modifier = Modifier.padding(top = 12.dp))
      }
    }
  }
}

@Composable
private fun DesktopCanvas(
    state: DesktopShellState,
    resultBrowsers: ResultBrowserStore,
    widthDp: Float,
    editorActions: DesktopShellEditorActions,
    analysisActions: DesktopShellAnalysisActions,
    findingActions: FindingActions,
    onWorkspaceSelected: (Workspace) -> Unit,
    modifier: Modifier,
) {
  val appState = state.app
  val layout = state.layout
  val editor = state.editor
  val context = state.context
  val workspace = appState.workspace
  EditorArea(
      content = {
        ContentPane(
            state =
                ContentPaneState(
                    project = appState.project,
                    overview = appState.overview,
                    selected = appState.selectedFile,
                    symbols = appState.symbols,
                    selectedSymbol = appState.selectedSymbol,
                    workspace = workspace,
                    editorChrome =
                        editorChromeUiState(
                            file = appState.selectedFile,
                            selectedSymbol = appState.selectedSymbol,
                            requestedSurface = layout.editorSurface,
                            progress = editor.progress,
                            draft = appState.review.draft,
                            creationInProgress =
                                editor.generating ||
                                    appState.review.editor?.status == DraftEditorStatus.Validating,
                        ),
                    draft = appState.review.draft,
                    review = reviewToolWindowState(appState),
                    focusedLine = appState.selection.focusedLine,
                    findings = appState.findings.findings,
                    analysis = AnalysisWorkspacePaneState(appState.project, appState.analysisRun),
                    bugs =
                        BugsWorkspacePaneState(
                            appState.projectBugFindings(),
                            appState.findings.scan,
                            appState.loading,
                            appState.analysisResultPage("bugs"),
                            resultBrowsers.stateFor(appState.analysisResultPage("bugs"))),
                    performance =
                        PerformanceWorkspacePaneState(
                            page = appState.analysisResultPage("performance"),
                            index = appState.index,
                            benchmarkComparison = appState.review.benchmark.comparison,
                            expectedBenchmarkIdentity = benchmarkEvidenceIdentity(appState.review),
                            benchmarkCatalog = appState.review.benchmark.catalog,
                            selectedBenchmark = appState.review.benchmark.selected,
                            benchmarkRunning = appState.review.benchmark.running,
                            browser =
                                resultBrowsers.stateFor(
                                    appState.analysisResultPage("performance"))),
                    security =
                        SecurityWorkspacePaneState(
                            appState.analysisResultPage("security"),
                            appState.index,
                            resultBrowsers.stateFor(appState.analysisResultPage("security"))),
                ),
            navigation =
                ContentPaneNavigationActions(
                    selectWorkspace = onWorkspaceSelected,
                    selectEditorSurface = editorActions.selectEditorSurface,
                    createDeclaration = editorActions.createDeclaration,
                    editDraft = editorActions.focusDraft,
                    sourceLineSelected = { selection ->
                      editorActions.sourceLineSelected(selection)
                    },
                ),
            analysisActions = analysisActions.toWorkspaceActions(onWorkspaceSelected),
            bugsActions =
                BugsWorkspaceActions(
                    findingActions = findingActions,
                    startScan = analysisActions.startScan,
                    cancelScan = analysisActions.cancelScan,
                    openAnalysis = { onWorkspaceSelected(Workspace.Analysis) },
                ),
            performanceActions =
                PerformanceWorkspaceActions(
                    openAnalysis = { onWorkspaceSelected(Workspace.Analysis) },
                    semanticActions = findingActions,
                    prepareOptimization = analysisActions.preparePerformanceFinding,
                    loadBenchmarks = analysisActions.loadGoBenchmarks,
                    selectBenchmark = analysisActions.selectGoBenchmark,
                    runBenchmark = analysisActions.compareSelectedGoBenchmark),
            securityActions =
                SecurityWorkspaceActions(
                    openAnalysis = { onWorkspaceSelected(Workspace.Analysis) },
                    semanticActions = findingActions,
                    prepareFix = analysisActions.prepareSecurityFinding),
            modifier = Modifier.fillMaxSize(),
        )
      },
      modifier = modifier,
  )
}

@Composable
private fun ContentPane(
    state: ContentPaneState,
    navigation: ContentPaneNavigationActions,
    analysisActions: AnalysisWorkspaceActions,
    bugsActions: BugsWorkspaceActions,
    performanceActions: PerformanceWorkspaceActions,
    securityActions: SecurityWorkspaceActions,
    modifier: Modifier,
) {
  Column(modifier.background(EditorCanvas)) {
    when (state.workspace) {
      Workspace.Summary ->
          ProjectSummaryPane(
              state.overview,
              state.project,
              openResults = navigation.selectWorkspace,
              run = state.analysis.analysis.run,
              sections = state.analysis.analysis.sections,
              fileSelection = state.analysis.analysis.fileSelection.selection,
              analysisState = state.analysis.analysis,
              analysisActions = analysisActions)
      Workspace.Editor ->
          EditorWorkspace(
              chrome = state.editorChrome,
              review = state.review,
              onEditDraft = navigation.editDraft,
              onSelectSurface = navigation.selectEditorSurface,
              onCreateDeclaration = navigation.createDeclaration,
              canvas = {
                if (state.editorChrome.activeSurface == EditorSurface.Review) {
                  ReviewDiffCanvas(state.draft)
                } else {
                  EditorPane(
                      state.project,
                      state.selected,
                      state.symbols,
                      state.selectedSymbol,
                      state.focusedLine,
                      state.findings,
                      navigation.sourceLineSelected)
                }
              })
      Workspace.Analysis -> AnalysisWorkspacePane(state.analysis, analysisActions)
      Workspace.Performance -> PerformanceWorkspacePane(state.performance, performanceActions)
      Workspace.Bugs -> BugsWorkspacePane(state.bugs, bugsActions)
      Workspace.Security -> SecurityWorkspacePane(state.security, securityActions)
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
    val editorChrome: EditorChromeUiState,
    val review: ReviewToolWindowState,
    val draft: DeclarationDraft?,
    val focusedLine: Int,
    val findings: List<UnifiedFinding>,
    val analysis: AnalysisWorkspacePaneState,
    val bugs: BugsWorkspacePaneState,
    val performance: PerformanceWorkspacePaneState,
    val security: SecurityWorkspacePaneState,
)

private data class ContentPaneNavigationActions(
    val editDraft: () -> Unit,
    val selectWorkspace: (Workspace) -> Unit,
    val selectEditorSurface: (EditorSurface) -> Unit,
    val sourceLineSelected: (SourceLineSelection) -> Unit,
    val createDeclaration: () -> Unit,
)

private fun DesktopShellAnalysisActions.toWorkspaceActions(openResults: (Workspace) -> Unit) =
    AnalysisWorkspaceActions(
        startAnalysis,
        pauseAnalysis,
        resumeAnalysis,
        cancelAnalysis,
        openResults,
        refreshAnalysisSelection,
        saveAnalysisSelection)

internal fun modelDestinationLabel(scope: ModelScope, model: ScopedModel): String {
  val reasoningEffort =
      model.reasoningEffort.takeIf(String::isNotBlank)?.let { " · reasoning: $it" }.orEmpty()
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
  IdeDialog(
      onDismissRequest = onDismiss,
      title = { Text("Context inspector · read-only") },
      content = {
        Column(Modifier.verticalScroll(rememberScrollState())) {
          Column(Modifier.fillMaxWidth()) {
            IdePaneHeader("Destination")
            val scope =
                ModelScope.entries.firstOrNull { it.wireValue == manifest.scope }?.label
                    ?: manifest.scope.ifBlank { "Function edits" }
            val provider =
                if (manifest.remoteProvider)
                    "remote provider · confirmation required before sending project context"
                else "local provider · project context stays on this machine"
            Text(
                "$scope: ${manifest.model.ifBlank { "configured model" }} · ${manifest.providerOrigin.ifBlank { "configured destination" }} · $provider",
                color = if (manifest.remoteProvider) Warning else SecondaryText,
                fontSize = 12.sp,
                modifier = Modifier.padding(top = 5.dp))
          }
          IdeHorizontalSeparator(Modifier.padding(top = 10.dp))
          Text(
              contextManifestSummary(manifest),
              color = PrimaryText,
              fontSize = 12.sp,
              modifier = Modifier.padding(top = 10.dp))
          if (manifest.tokenLimit > 0)
              Text(
                  "Token budget: ${manifest.estimatedTokens} / ${manifest.tokenLimit}",
                  color = SecondaryText,
                  fontSize = 11.sp)
          if (manifest.byteLimit > 0)
              Text(
                  "Byte limit: ${formatBytes(manifest.byteLimit.toLong())}",
                  color = SecondaryText,
                  fontSize = 11.sp)
          SelectionContainer {
            Column {
              Text(
                  "Included",
                  color = PrimaryText,
                  fontWeight = FontWeight.SemiBold,
                  modifier = Modifier.padding(top = 10.dp))
              if (manifest.included.isEmpty())
                  Text("No files included.", color = SecondaryText, fontSize = 11.sp)
              manifest.included.forEach {
                Text(
                    "${it.path} · ${formatBytes(it.sizeBytes)} · ${it.estimatedTokens} tokens",
                    color = SecondaryText,
                    fontSize = 11.sp)
              }
              Text(
                  "Excluded",
                  color = PrimaryText,
                  fontWeight = FontWeight.SemiBold,
                  modifier = Modifier.padding(top = 10.dp))
              if (manifest.excluded.isEmpty())
                  Text("No files excluded.", color = SecondaryText, fontSize = 11.sp)
              manifest.excluded.forEach {
                Text("${it.path} · ${it.reason}", color = SecondaryText, fontSize = 11.sp)
              }
            }
          }
        }
      },
      actions = {
        MiniOrcaButton(onClick = onDismiss, tone = ActionTone.Neutral) { Text("Close") }
      })
}
