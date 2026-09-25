package io.miniorca.desktop

import androidx.compose.foundation.background
import androidx.compose.foundation.focusable
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.relocation.BringIntoViewRequester
import androidx.compose.foundation.relocation.bringIntoViewRequester
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.key
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusManager
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.input.key.KeyEvent
import androidx.compose.ui.input.key.KeyEventType
import androidx.compose.ui.input.key.isCtrlPressed
import androidx.compose.ui.input.key.isMetaPressed
import androidx.compose.ui.input.key.isShiftPressed
import androidx.compose.ui.input.key.key
import androidx.compose.ui.input.key.onPreviewKeyEvent
import androidx.compose.ui.input.key.type
import androidx.compose.ui.layout.Layout
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.LocalFocusManager
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.Constraints
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

internal fun projectOpenAvailable(attempt: ProjectOpeningAttempt?): Boolean =
    attempt?.outcome != ProjectOpeningOutcome.Opening

internal fun editorChromeVisible(workspace: Workspace): Boolean = workspace == Workspace.Editor

internal fun resizeExplorerFromDisplayed(
    preferred: DesktopLayoutState,
    displayed: Float,
    delta: Float,
): DesktopLayoutState = preferred.withExplorerWidth(displayed + delta)

internal fun resizeToolFromDisplayed(
    preferred: DesktopLayoutState,
    displayed: Float,
    delta: Float,
): DesktopLayoutState = preferred.withActionWidth(displayed - delta)

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
    val retryRestore: () -> Unit = {},
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
    val retryResults: (String, String) -> Unit,
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
    val commandsTrigger: FocusRequester,
    val modelsTrigger: FocusRequester,
    val statusDetailsTrigger: FocusRequester,
    val landing: FocusRequester,
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

internal enum class TransientOpener {
  Region,
  HeaderSearch,
  RailCommands,
  RailModels,
  FooterModels,
}

private data class TransientFocusOrigin(
    val region: DesktopFocusRegion,
    val projectId: String?,
    val opener: TransientOpener = TransientOpener.Region,
)

internal fun transientFocusOpener(
    opener: TransientOpener,
    sameProject: Boolean,
    region: DesktopFocusRegion?,
): TransientOpener =
    if (sameProject &&
        when (opener) {
          TransientOpener.HeaderSearch -> region == DesktopFocusRegion.Toolbar
          TransientOpener.RailCommands,
          TransientOpener.RailModels -> region == DesktopFocusRegion.LeftToolWindow
          TransientOpener.FooterModels -> region == DesktopFocusRegion.StatusBar
          TransientOpener.Region -> false
        })
        opener
    else TransientOpener.Region

internal fun transientFocusRegion(
    previous: DesktopFocusRegion,
    sameProject: Boolean,
    workspace: Workspace,
    rightToolWindowVisible: Boolean,
    statusBarVisible: Boolean,
): DesktopFocusRegion? {
  if (!sameProject) return if (statusBarVisible) DesktopFocusRegion.Toolbar else null
  return when (previous) {
    DesktopFocusRegion.StatusBar -> if (statusBarVisible) previous else DesktopFocusRegion.Toolbar
    else ->
        paletteFocusRestorationRegion(
            previous,
            rightToolWindowVisible = workspace == Workspace.Editor && rightToolWindowVisible,
            bottomToolWindowVisible = true)
  }
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
        commandsTrigger = FocusRequester(),
        modelsTrigger = FocusRequester(),
        statusDetailsTrigger = FocusRequester(),
        landing = FocusRequester(),
    )
  }
  var paletteOrigin by remember { mutableStateOf<TransientFocusOrigin?>(null) }
  var statusDetailsVisible by remember { mutableStateOf(false) }
  var statusOrigin by remember { mutableStateOf<TransientFocusOrigin?>(null) }
  var contextOrigin by remember { mutableStateOf<TransientFocusOrigin?>(null) }
  var pendingFocus by remember { mutableStateOf<TransientFocusOrigin?>(null) }
  var focusedSideSplitter by remember { mutableStateOf(false) }
  var sideResize by remember { mutableStateOf<Pair<Float?, Float?>>(null to null) }
  var editorMode by remember { mutableStateOf(DesktopLayoutMode.Wide) }
  RestoreFocusFromRemovedSplitter(
      editorMode,
      focusedSideSplitter,
      palette.visible,
      statusDetailsVisible,
      context.visible,
      terminal?.ownsFocus() == true,
      focusRequesters.editor) {
        focusedSideSplitter = false
      }
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
  RestoreLandingFocusOnProjectOpen(shellMode, focusManager, focusRequesters.paletteTrigger)
  LaunchedEffect(appState.project?.projectId, appState.project?.projectRevision) {
    if (statusDetailsVisible) pendingFocus = statusOrigin
    statusOrigin = null
    statusDetailsVisible = false
    if (paletteOrigin != null && paletteOrigin?.projectId != appState.project?.projectId) {
      pendingFocus = paletteOrigin
      paletteOrigin = null
      paletteActions.dismiss()
    }
    if (shellMode == DesktopShellMode.ProjectLanding && contextOrigin != null) {
      pendingFocus = contextOrigin
      contextOrigin = null
      editorActions.dismissContext()
    }
  }
  LaunchedEffect(context.visible, shellMode) {
    if (context.visible &&
        shellMode == DesktopShellMode.ProjectWorkspace &&
        contextOrigin == null) {
      contextOrigin = TransientFocusOrigin(layout.lastFocusedRegion, appState.project?.projectId)
    } else if (contextOrigin != null) {
      pendingFocus = contextOrigin
      contextOrigin = null
    }
  }
  fun openPalette(mode: PaletteMode, opener: TransientOpener = TransientOpener.Region) {
    if (palette.visible) return
    paletteOrigin =
        TransientFocusOrigin(
            when (opener) {
              TransientOpener.HeaderSearch -> DesktopFocusRegion.Toolbar
              TransientOpener.RailCommands -> DesktopFocusRegion.LeftToolWindow
              else -> layout.lastFocusedRegion
            },
            appState.project?.projectId,
            opener)
    paletteActions.open(mode)
  }
  fun dismissPaletteAndRestoreFocus() {
    pendingFocus = paletteOrigin
    paletteOrigin = null
    paletteActions.dismiss()
  }
  fun showStatusDetails(opener: TransientOpener) {
    if (statusDetailsVisible) return
    val region =
        when (opener) {
          TransientOpener.RailModels -> DesktopFocusRegion.LeftToolWindow
          TransientOpener.FooterModels -> DesktopFocusRegion.StatusBar
          else -> return
        }
    statusOrigin = TransientFocusOrigin(region, appState.project?.projectId, opener)
    layoutActions.updateLayout(layout.withFocus(region))
    statusDetailsVisible = true
  }
  fun dismissStatusDetailsAndRestoreFocus() {
    pendingFocus = statusOrigin
    statusOrigin = null
    statusDetailsVisible = false
  }
  fun dismissContextAndRestoreFocus() {
    pendingFocus = contextOrigin
    contextOrigin = null
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
  LaunchedEffect(
      pendingFocus,
      palette.visible,
      statusDetailsVisible,
      context.visible,
      appState.project?.projectId,
      workspace,
      layout.rightToolWindowVisible) {
        val origin = pendingFocus
        if (origin != null &&
            !palette.visible &&
            !statusDetailsVisible &&
            !(context.visible && shellMode == DesktopShellMode.ProjectWorkspace)) {
          val region =
              transientFocusRegion(
                  origin.region,
                  origin.projectId == appState.project?.projectId,
                  workspace,
                  showsEditorChrome && layout.rightToolWindowVisible,
                  desktopStatusBarVisible(appState.project))
          val requester =
              when {
                region == null ->
                    if (projectOpenAvailable(appState.projectState.openingAttempt))
                        focusRequesters.landing
                    else focusRequesters.fallback
                else ->
                    when (transientFocusOpener(
                        origin.opener, origin.projectId == appState.project?.projectId, region)) {
                      TransientOpener.HeaderSearch -> focusRequesters.paletteTrigger
                      TransientOpener.RailCommands -> focusRequesters.commandsTrigger
                      TransientOpener.RailModels -> focusRequesters.modelsTrigger
                      TransientOpener.FooterModels -> focusRequesters.statusDetailsTrigger
                      TransientOpener.Region -> focusRequesters.forRegion(region)
                    }
              }
          requester.requestFocus()
          pendingFocus = null
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
                    paletteActions = paletteActions.copy(open = { openPalette(it) }),
                    onDismissTransient = ::dismissTopmostTransient,
                    onWorkspaceSelected = ::selectWorkspace,
                )
              },
  ) {
    if (shellMode == DesktopShellMode.ProjectLanding) {
      ProjectLanding(appState, projectActions, focusRequesters.landing)
    } else {
      BoxWithConstraints {
        Column {
          MainToolbar(
              state =
                  ToolbarState(
                      project = appState.project,
                      busy = appState.loading,
                      operationStatus = appState.status,
                      connection = appState.connection,
                      gitStatus = appState.gitStatus,
                      analysisStatus = toolbarAnalysisStatus(appState),
                      openingAttempt = appState.projectState.openingAttempt,
                  ),
              actions =
                  ToolbarActions(
                      onImport = projectActions.importProject,
                      onReanalyze = projectActions.reanalyzeProject,
                      onReconnect = projectActions.reconnect,
                      onRetryRestore = projectActions.retryRestore,
                      onPalette = {
                        layoutActions.updateLayout(layout.withFocus(DesktopFocusRegion.Toolbar))
                        openPalette(PaletteMode.Files, TransientOpener.HeaderSearch)
                      },
                  ),
              modifier = Modifier.focusRequester(focusRequesters.toolbar).focusable(),
              paletteFocusRequester = focusRequesters.paletteTrigger,
          )
          WorkspaceFrame(
              rail = {
                ToolWindowBar(
                    leftToolWindowForWorkspace(workspace),
                    ::selectToolWindow,
                    Modifier.focusRequester(focusRequesters.leftToolWindow),
                    onOpenTerminal = ::openTerminal,
                    onOpenCommands = {
                      openPalette(PaletteMode.Actions, TransientOpener.RailCommands)
                    },
                    commandsFocusRequester = focusRequesters.commandsTrigger,
                    onOpenModels = { showStatusDetails(TransientOpener.RailModels) },
                    modelsFocusRequester = focusRequesters.modelsTrigger)
              },
              panes = {
                DesktopCanvas(
                    state,
                    resultBrowsers,
                    editorActions,
                    analysisActions,
                    findingActions,
                    ::selectWorkspace,
                    Modifier.weight(1f)
                        .fillMaxHeight()
                        .focusRequester(focusRequesters.editor)
                        .focusable()
                        .testTag("desktop-canvas-focus"))
              },
              editorPanes =
                  if (showsEditorChrome)
                      { width, height ->
                        val resolved =
                            resolveDesktopLayout(layout, width, LocalDensity.current.fontScale)
                        LaunchedEffect(resolved.mode) {
                          editorMode = resolved.mode
                          if (resolved.mode == DesktopLayoutMode.Compact) sideResize = null to null
                        }
                        EditorPaneArrangement(
                            resolved,
                            layout,
                            height,
                            left = { modifier ->
                              DockedToolWindow(
                                  "Files",
                                  { paneModifier -> panes.explorer(paneModifier) {} },
                                  modifier,
                                  showHeader = false)
                            },
                            canvas = { modifier ->
                              DesktopCanvas(
                                  state,
                                  resultBrowsers,
                                  editorActions,
                                  analysisActions,
                                  findingActions,
                                  ::selectWorkspace,
                                  modifier
                                      .focusRequester(focusRequesters.editor)
                                      .focusable()
                                      .testTag("desktop-canvas-focus"))
                            },
                            right = { modifier ->
                              DockedToolWindow(
                                  "Tool windows",
                                  content = { paneModifier ->
                                    CompositionLocalProvider(
                                        LocalContextCreationActionVisible provides false) {
                                          RightToolWindowContainer(
                                              layout.activeRightToolWindow,
                                              ::selectRightToolWindow,
                                              panes.rightToolWindows,
                                              panes.rightToolWindowBadges,
                                              paneModifier.focusRequester(
                                                  focusRequesters.rightToolWindow))
                                        }
                                  },
                                  modifier = modifier,
                                  showHeader = false)
                            },
                            leftDivider = {
                              ResizableDivider(
                                  onDelta = {
                                    val next =
                                        resizeExplorerFromDisplayed(
                                            layout, sideResize.first ?: resolved.explorerWidth, it)
                                    sideResize = next.explorerWidth to sideResize.second
                                    layoutActions.updateLayout(next)
                                  },
                                  onCommit = {
                                    sideResize = null to sideResize.second
                                    layoutActions.saveLayout(layout)
                                  },
                                  onFocusChanged = { focused ->
                                    if (focused) focusedSideSplitter = true
                                    else
                                        scope.launch {
                                          androidx.compose.runtime.withFrameNanos {}
                                          if (editorMode == DesktopLayoutMode.Wide)
                                              focusedSideSplitter = false
                                        }
                                  })
                            },
                            rightDivider = {
                              ResizableDivider(
                                  onDelta = {
                                    val next =
                                        resizeToolFromDisplayed(
                                            layout, sideResize.second ?: resolved.actionWidth, it)
                                    sideResize = sideResize.first to next.actionWidth
                                    layoutActions.updateLayout(next)
                                  },
                                  onCommit = {
                                    sideResize = sideResize.first to null
                                    layoutActions.saveLayout(layout)
                                  },
                                  onFocusChanged = { focused ->
                                    if (focused) focusedSideSplitter = true
                                    else
                                        scope.launch {
                                          androidx.compose.runtime.withFrameNanos {}
                                          if (editorMode == DesktopLayoutMode.Wide)
                                              focusedSideSplitter = false
                                        }
                                  })
                            })
                      }
                  else null,
              terminal = { workspaceHeight ->
                val effectiveHeight =
                    resolveTerminalDockHeight(
                        layout, workspaceHeight, LocalDensity.current.fontScale)
                TerminalDock(
                    layout = layout,
                    effectiveHeight = effectiveHeight,
                    state = panes.terminalState,
                    tabActions = panes.terminalTabActions,
                    onOpen = ::openTerminal,
                    onCollapse = ::collapseTerminal,
                    onHeightDelta = {
                      layoutActions.updateLayout(layout.withBottomHeight(effectiveHeight + it))
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
                onOpenDetails = { showStatusDetails(TransientOpener.FooterModels) },
                modifier = Modifier.focusRequester(focusRequesters.statusBar).focusable(),
                detailsFocusRequester = focusRequesters.statusDetailsTrigger,
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
                pendingFocus =
                    TransientFocusOrigin(DesktopFocusRegion.Editor, appState.project?.projectId)
                paletteOrigin = null
              },
              { symbol ->
                paletteActions.selectSymbol(symbol)
                pendingFocus =
                    TransientFocusOrigin(DesktopFocusRegion.Editor, appState.project?.projectId)
                paletteOrigin = null
              },
              { action ->
                paletteActions.selectAction(action)
                pendingFocus = paletteOrigin
                paletteOrigin = null
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

@Composable
private fun RestoreFocusFromRemovedSplitter(
    mode: DesktopLayoutMode,
    splitterFocused: Boolean,
    paletteVisible: Boolean,
    statusVisible: Boolean,
    contextVisible: Boolean,
    terminalFocused: Boolean,
    editorFocus: FocusRequester,
    clearSplitterFocus: () -> Unit,
) {
  LaunchedEffect(mode) {
    if (mode == DesktopLayoutMode.Compact && splitterFocused) {
      clearSplitterFocus()
      if (!paletteVisible && !statusVisible && !contextVisible && !terminalFocused)
          editorFocus.requestFocus()
    }
  }
}

// A single layout node changes placement without replacing the keyed pane compositions.
// Compact children receive finite heights even though the outer container scrolls vertically.
@Composable
internal fun EditorPaneArrangement(
    resolved: ResolvedDesktopLayout,
    preferred: DesktopLayoutState,
    viewportHeight: Float,
    left: @Composable (Modifier) -> Unit,
    canvas: @Composable (Modifier) -> Unit,
    right: @Composable (Modifier) -> Unit,
    leftDivider: @Composable () -> Unit,
    rightDivider: @Composable () -> Unit,
    modifier: Modifier = Modifier,
) {
  val compact = resolved.mode == DesktopLayoutMode.Compact
  val density = LocalDensity.current
  val scroll = rememberScrollState()
  val filesReveal = remember { BringIntoViewRequester() }
  val canvasReveal = remember { BringIntoViewRequester() }
  val toolReveal = remember { BringIntoViewRequester() }
  var focusedPane by remember { mutableStateOf<String?>(null) }
  LaunchedEffect(compact, focusedPane) {
    if (compact) {
      when (focusedPane) {
        "files" -> filesReveal.bringIntoView()
        "canvas" -> canvasReveal.bringIntoView()
        "tool" -> toolReveal.bringIntoView()
      }
    }
  }
  Layout(
      content = {
        if (preferred.leftToolWindowVisible) {
          key("files") {
            left(
                Modifier.bringIntoViewRequester(filesReveal).onFocusChanged {
                  if (it.hasFocus) focusedPane = "files"
                })
          }
          if (!compact) key("files-divider") { leftDivider() }
        }
        key("canvas") {
          canvas(
              Modifier.bringIntoViewRequester(canvasReveal).onFocusChanged {
                if (it.hasFocus) focusedPane = "canvas"
              })
        }
        if (preferred.rightToolWindowVisible) {
          if (!compact) key("tool-divider") { rightDivider() }
          key("tool") {
            right(
                Modifier.bringIntoViewRequester(toolReveal).onFocusChanged {
                  if (it.hasFocus) focusedPane = "tool"
                })
          }
        }
      },
      modifier =
          modifier.fillMaxSize().then(if (compact) Modifier.verticalScroll(scroll) else Modifier),
  ) { measurables, constraints ->
    val width = constraints.maxWidth
    val height =
        if (compact) with(density) { viewportHeight.dp.roundToPx() } else constraints.maxHeight
    val gap = with(density) { WORKSPACE_FRAME_INSET.dp.roundToPx() }
    val divider = with(density) { RESIZE_DIVIDER_WIDTH.dp.roundToPx() }
    // Compact panes scroll as a group; the Editor needs room for chrome and a source/diff viewport.
    val childHeight =
        if (compact)
            maxOf(
                height * 2 / 3,
                with(density) { (MIN_WORKSPACE_PANE_HEIGHT * fontScale).dp.roundToPx() })
        else height
    val positions = mutableListOf<Pair<Int, Int>>()
    val measured = mutableListOf<androidx.compose.ui.layout.Placeable>()
    var index = 0
    var x = 0
    var y = 0
    fun place(widthPx: Int, heightPx: Int) {
      val placeable =
          measurables[index++].measure(
              Constraints.fixed(widthPx.coerceAtLeast(0), heightPx.coerceAtLeast(0)))
      // Placeables are kept in source order; the mode changes coordinates, not identity.
      measured += placeable
      positions += if (compact) 0 to y else x to 0
      if (compact) y += placeable.height + gap else x += placeable.width
    }
    if (preferred.leftToolWindowVisible) {
      place(
          if (compact) width else with(density) { resolved.explorerWidth.dp.roundToPx() },
          if (compact) maxOf(height / 3, with(density) { 180.dp.roundToPx() }) else height)
      if (!compact) place(divider, height)
    }
    val rightWidth =
        if (preferred.rightToolWindowVisible && !compact)
            with(density) { resolved.actionWidth.dp.roundToPx() }
        else 0
    val canvasWidth =
        if (compact) width
        else
            (width - x - rightWidth - if (preferred.rightToolWindowVisible) divider else 0)
                .coerceAtLeast(0)
    place(canvasWidth, childHeight)
    if (preferred.rightToolWindowVisible) {
      if (!compact) place(divider, height)
      place(if (compact) width else rightWidth, childHeight)
    }
    layout(width, if (compact) (y - gap).coerceAtLeast(0) else height) {
      // Measure results are retained by the layout pass, not recomposed across mode changes.
      measured.forEachIndexed { i, child ->
        child.placeRelative(positions[i].first, positions[i].second)
      }
    }
  }
}

@Composable
private fun RestoreLandingFocusOnProjectOpen(
    mode: DesktopShellMode,
    focusManager: FocusManager,
    toolbarAction: FocusRequester,
) {
  var previous by remember { mutableStateOf(mode) }
  LaunchedEffect(mode) {
    if (previous == DesktopShellMode.ProjectLanding && mode == DesktopShellMode.ProjectWorkspace) {
      androidx.compose.runtime.withFrameNanos {}
      focusManager.clearFocus(force = true)
      toolbarAction.requestFocus()
    }
    previous = mode
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

internal fun handleDesktopShortcut(
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
      if (projectOpenAvailable(appState.projectState.openingAttempt)) {
        projectActions.importProject()
        true
      } else false
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
internal fun ProjectLanding(
    appState: DesktopState,
    actions: DesktopShellProjectActions,
    focusRequester: FocusRequester,
) {
  val projectState = appState.projectState
  val attempt = projectState.openingAttempt
  val opening = attempt?.outcome == ProjectOpeningOutcome.Opening
  val statusFocus = remember { FocusRequester() }
  var focusedRecovery by remember { mutableStateOf(false) }
  var focusedStatus by remember { mutableStateOf(false) }
  var focusedOpen by remember { mutableStateOf(false) }
  LaunchedEffect(opening, attempt?.requestId, attempt?.outcome) {
    if (focusedOpen && opening) {
      focusedOpen = false
      statusFocus.requestFocus()
    } else if (focusedRecovery &&
        (attempt?.outcome !is ProjectOpeningOutcome.Failed ||
            attempt.kind != ProjectOpeningKind.Restore)) {
      focusedRecovery = false
      if (projectOpenAvailable(appState.projectState.openingAttempt)) focusRequester.requestFocus()
      else statusFocus.requestFocus()
    } else if (focusedStatus && projectOpenAvailable(appState.projectState.openingAttempt)) {
      focusedStatus = false
      focusRequester.requestFocus()
    }
  }
  Box(Modifier.fillMaxSize().padding(16.dp), contentAlignment = Alignment.Center) {
    Column(
        Modifier.widthIn(max = 520.dp)
            .fillMaxWidth()
            .verticalScroll(rememberScrollState())
            .testTag("project-landing-scroll")) {
          MiniOrcaMark()
          Spacer(Modifier.height(12.dp))
          Text("Mini-Orca", color = PrimaryText, fontSize = 22.sp, fontWeight = FontWeight.SemiBold)
          Spacer(Modifier.height(12.dp))
          SystemStateMessage(
              title = "No project open",
              message =
                  "Open a project to inspect its files and analysis. Import may use the configured Analyze provider and require confirmation.",
              action = {
                MiniOrcaButton(
                    onClick = actions.importProject,
                    enabled = projectOpenAvailable(appState.projectState.openingAttempt),
                    tone = ActionTone.Primary,
                    modifier =
                        Modifier.focusRequester(focusRequester).onFocusChanged {
                          if (it.isFocused) {
                            focusedOpen = true
                            focusedRecovery = false
                          } else if (!opening) {
                            focusedOpen = false
                          }
                        }) {
                      Text("Open project")
                    }
              })
          Spacer(Modifier.height(12.dp))
          val remembered = projectState.rememberedPath
          if (remembered == null) {
            SystemStateMessage(
                "Last project",
                if (projectState.preferenceReadWarning != null)
                    "Last project unknown; local preferences could not be read."
                else "No project remembered on this device.")
          } else {
            SystemStateMessage(
                "Last project · ${projectPathLabel(remembered)}",
                "Remembered locally; not open yet.",
                action = { LandingPath("Remembered path", remembered) })
          }
          if (attempt != null) {
            Spacer(Modifier.height(12.dp))
            val restoring = attempt.kind == ProjectOpeningKind.Restore
            val failure = attempt.outcome as? ProjectOpeningOutcome.Failed
            SystemStateMessage(
                modifier =
                    Modifier.focusRequester(statusFocus)
                        .onFocusChanged {
                          focusedStatus = it.isFocused
                          if (it.isFocused) focusedOpen = false
                        }
                        .focusable()
                        .testTag("project-opening-focus"),
                title =
                    when {
                      failure != null ->
                          if (restoring) "Could not restore project" else "Could not import project"
                      opening -> if (restoring) "Restoring local project…" else "Importing project…"
                      else -> if (restoring) "Restore canceled" else "Import canceled"
                    },
                message =
                    if (failure != null) "The requested project did not open."
                    else if (opening && restoring)
                        "Reading saved local project data; no model request is made."
                    else if (opening) "Import may use the configured Analyze provider."
                    else "Open project to choose another folder.",
                accent = if (failure != null) Error else SecondaryText,
                action = {
                  LandingPath("Requested path", attempt.path)
                  if (failure != null) {
                    Spacer(Modifier.height(8.dp))
                    Text(
                        "Opening diagnostic",
                        color = SecondaryText,
                        style = IdeTypography.resultLabel)
                    DiagnosticText(failure.message, color = Error)
                    if (restoring) {
                      Spacer(Modifier.height(8.dp))
                      MiniOrcaButton(
                          onClick = actions.retryRestore,
                          tone = ActionTone.Neutral,
                          modifier =
                              Modifier.onFocusChanged {
                                if (it.isFocused) focusedRecovery = true
                              }) {
                            Text("Retry restore")
                          }
                    }
                  }
                })
          }
          if (connectionPresentation(appState.connection).canReconnect) {
            Spacer(Modifier.height(12.dp))
            SystemStateMessage(
                "Daemon disconnected",
                "Reconnect reads daemon status and model configuration. It does not contact a provider or run project code.",
                accent = Error,
                action = {
                  MiniOrcaButton(onClick = actions.reconnect, tone = ActionTone.Neutral) {
                    Text("Reconnect daemon")
                  }
                })
          }
          projectState.preferenceReadWarning?.let { warning ->
            Spacer(Modifier.height(12.dp))
            SystemStateMessage(
                "Could not read last project preference",
                "Open project is still available. Local preference storage could not be read.",
                accent = Warning,
                action = { DiagnosticText(warning, color = Warning) })
          }
          projectState.preferenceSaveWarning?.let { warning ->
            Spacer(Modifier.height(12.dp))
            SystemStateMessage(
                "Could not remember project",
                "The opened project could not be remembered for the next launch.",
                accent = Warning,
                action = { DiagnosticText(warning, color = Warning) })
          }
        }
  }
}

private fun projectPathLabel(path: String): String =
    path.trimEnd('/', '\\').replace('\\', '/').substringAfterLast('/').ifBlank { path }

@Composable
private fun LandingPath(label: String, path: String) {
  Text(label, color = SecondaryText, style = IdeTypography.resultLabel)
  SelectionContainer {
    Text(
        path,
        color = PrimaryText,
        fontFamily = FontFamily.Monospace,
        style = IdeTypography.resultCode)
  }
}

@Composable
private fun DesktopCanvas(
    state: DesktopShellState,
    resultBrowsers: ResultBrowserStore,
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
                    retryResults = { analysisActions.retryResults("bugs", "") },
                ),
            performanceActions =
                PerformanceWorkspaceActions(
                    openAnalysis = { onWorkspaceSelected(Workspace.Analysis) },
                    semanticActions = findingActions,
                    prepareOptimization = analysisActions.preparePerformanceFinding,
                    loadBenchmarks = analysisActions.loadGoBenchmarks,
                    selectBenchmark = analysisActions.selectGoBenchmark,
                    runBenchmark = analysisActions.compareSelectedGoBenchmark,
                    retryResults = { analysisActions.retryResults("performance", "") }),
            securityActions =
                SecurityWorkspaceActions(
                    openAnalysis = { onWorkspaceSelected(Workspace.Analysis) },
                    semanticActions = findingActions,
                    prepareFix = analysisActions.prepareSecurityFinding,
                    retryResults = { analysisActions.retryResults("security", "") }),
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
        Column {
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
