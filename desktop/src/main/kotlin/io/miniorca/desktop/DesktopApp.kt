package io.miniorca.desktop

import androidx.compose.foundation.layout.Column
import androidx.compose.material.AlertDialog
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import java.io.File
import javax.swing.JFileChooser

internal enum class PaletteMode {
  Files,
  Symbols,
  Actions
}

private enum class ComposerFocusTarget {
  Chat,
  Draft
}

private sealed interface PendingDraftDiscard {
  val currentDraft: CurrentEditIdentity
  val nextLabel: String

  data class Replace(
      val request: DirectEditRequest,
      override val currentDraft: CurrentEditIdentity,
  ) : PendingDraftDiscard {
    override val nextLabel: String = "edit ${request.target.symbol}"
  }

  data class Create(
      override val currentDraft: CurrentEditIdentity,
  ) : PendingDraftDiscard {
    override val nextLabel: String = "create a declaration"
  }
}

@Composable
internal fun MiniOrcaApp(
    api: ApiClient = remember { ApiClient() },
    lastProjectStore: LastProjectStore = remember { LastProjectStore() },
) {
  val scope = rememberCoroutineScope()
  val presenter =
      remember(api, lastProjectStore, scope) {
        DesktopWorkflowPresenter(api, lastProjectStore, scope)
      }
  val workflow by presenter.snapshot.collectAsState()
  val appState = workflow.state
  val layoutStore = remember { DesktopLayoutStore() }
  var layout by remember { mutableStateOf(layoutStore.load()) }
  var filter by remember { mutableStateOf("") }
  var collapsedDirectories by remember { mutableStateOf(emptySet<String>()) }
  var contextAction by remember { mutableStateOf("fix") }
  var showContext by remember { mutableStateOf(false) }
  var paletteMode by remember { mutableStateOf(PaletteMode.Files) }
  var paletteQuery by remember { mutableStateOf("") }
  var showPalette by remember { mutableStateOf(false) }
  var chatMode by remember { mutableStateOf(ChatEditMode.ReplaceSymbol) }
  var newChatSymbol by remember { mutableStateOf("") }
  var chatMessage by remember { mutableStateOf("") }
  var pendingImportPath by remember { mutableStateOf<String?>(null) }
  var composerRequested by remember { mutableStateOf(false) }
  var pendingComposerFocus by remember { mutableStateOf<ComposerFocusTarget?>(null) }
  var pendingDraftDiscard by remember { mutableStateOf<PendingDraftDiscard?>(null) }
  val chatFocusRequester = remember { FocusRequester() }
  val draftFocusRequester = remember { FocusRequester() }
  val analyzeModel = workflow.model(ModelScope.Analyze)
  val bugModel = workflow.model(ModelScope.Bug)
  val functionModel = workflow.model(ModelScope.Function)

  val editorProgress = editorProgressUiState(appState)
  LaunchedEffect(editorProgress.progress) {
    if (editorProgress.progress in setOf(EditorProgress.Review, EditorProgress.Receipt))
        composerRequested = false
  }
  LaunchedEffect(appState.preparedAction, appState.preparedRequest, appState.selectedSymbol) {
    if (appState.preparedAction.isNotBlank()) contextAction = appState.preparedAction
    if (appState.preparedRequest.isNotBlank()) {
      if (appState.preparedTaskSpec != null) chatMode = ChatEditMode.ReplaceSymbol
      chatMessage = appState.preparedRequest
      composerRequested = true
    }
  }
  LaunchedEffect(appState.project?.projectId, appState.project?.projectRevision) {
    appState.index?.let { collapsedDirectories = explorerDirectories(it.files) }
  }
  LaunchedEffect(appState.selectedFile?.path, appState.index?.projectRevision) {
    val activePath = appState.selectedFile?.path ?: return@LaunchedEffect
    val index = appState.index ?: return@LaunchedEffect
    filter = ""
    collapsedDirectories = revealExplorerPath(index.files, collapsedDirectories, activePath)
  }
  LaunchedEffect(appState.review.draft?.id, appState.review.draft?.revision) {
    if (appState.review.draft != null) {
      chatMessage = ""
      layout = layout.withEditorSurface(EditorSurface.Source)
    }
  }
  LaunchedEffect(workflow.contextManifest) {
    if (workflow.contextManifest != null) showContext = true
  }
  LaunchedEffect(presenter) { presenter.start() }
  DisposableEffect(presenter) { onDispose { presenter.close() } }

  fun focusComposerControl(target: ComposerFocusTarget) {
    composerRequested = true
    pendingComposerFocus = target
  }

  fun focusAssistantControl(target: ComposerFocusTarget) {
    layout =
        layout.openRight(RightToolWindow.Assistant).withFocus(DesktopFocusRegion.RightToolWindow)
    focusComposerControl(target)
  }

  fun startReplaceEdit(request: DirectEditRequest) {
    if (appState.selectedSymbol != request.selectedSymbol)
        presenter.dispatch(DesktopEvent.SymbolSelected(request.selectedSymbol))
    chatMode = ChatEditMode.ReplaceSymbol
    newChatSymbol = ""
    focusComposerControl(ComposerFocusTarget.Chat)
  }

  fun requestDirectEdit(symbol: SymbolInspectorSymbolState) {
    val request =
        directEditRequest(
            appState.selectedFile, appState.symbols, symbol.symbol, currentEditIdentity(appState))
            ?: return
    val currentDraft = request.currentDraft
    if (request.requiresDraftDiscard && currentDraft != null)
        pendingDraftDiscard = PendingDraftDiscard.Replace(request, currentDraft)
    else startReplaceEdit(request)
  }

  fun startCreateDeclaration() {
    chatMode = ChatEditMode.CreateSymbol
    newChatSymbol = ""
    focusComposerControl(ComposerFocusTarget.Chat)
  }

  fun requestCreateDeclaration() {
    val currentDraft = currentEditIdentity(appState)?.takeIf { it.hasDraft }
    if (currentDraft == null) startCreateDeclaration()
    else pendingDraftDiscard = PendingDraftDiscard.Create(currentDraft)
  }

  fun discardDraftAndContinue() {
    when (val pending = pendingDraftDiscard) {
      is PendingDraftDiscard.Replace -> {
        presenter.discardDraft()
        chatMessage = ""
        pendingDraftDiscard = null
        startReplaceEdit(pending.request)
      }
      is PendingDraftDiscard.Create -> {
        presenter.discardDraft()
        chatMessage = ""
        pendingDraftDiscard = null
        startCreateDeclaration()
      }
      null -> Unit
    }
  }

  fun importProject() {
    val directory = chooseDirectory() ?: return
    if (analyzeModel.remoteProvider && !workflow.providerConfirmed(ModelScope.Analyze))
        pendingImportPath = directory.absolutePath
    else presenter.loadProject(directory.absolutePath, restore = false)
  }

  fun openPalette(mode: PaletteMode) {
    paletteMode = mode
    paletteQuery = ""
    showPalette = true
  }

  val explorer: @Composable (Modifier, () -> Unit) -> Unit = { modifier, onSelected ->
    ExplorerPane(
        state =
            ExplorerPaneState(
                index = appState.index,
                selectedPath = appState.selectedFile?.path,
                filter = filter,
                collapsedDirectories = collapsedDirectories,
                loading = appState.loading,
            ),
        actions =
            ExplorerPaneActions(
                updateFilter = { filter = it },
                toggleDirectory = { path ->
                  collapsedDirectories =
                      if (path in collapsedDirectories) collapsedDirectories - path
                      else collapsedDirectories + path
                },
                collapseAll = {
                  collapsedDirectories = explorerDirectories(appState.index?.files.orEmpty())
                },
                revealActiveFile = {
                  appState.selectedFile?.path?.let { activePath ->
                    filter = ""
                    collapsedDirectories =
                        revealExplorerPath(
                            appState.index?.files.orEmpty(), collapsedDirectories, activePath)
                  }
                },
                selectFile = { path ->
                  presenter.openFileInEditor(path)
                  onSelected()
                },
            ),
        modifier = modifier,
    )
  }
  val contextPane: @Composable (Modifier) -> Unit = { modifier ->
    ContextToolWindow(
        state =
            ContextToolWindowState(
                inspector =
                    symbolInspectorUiState(
                        selectedFile = appState.selectedFile,
                        symbols = appState.symbols,
                        selectedSymbol = appState.selectedSymbol,
                        analysis = appState.analysis,
                        analysisInProgress = workflow.analysisInProgress,
                        provider =
                            InspectorProviderState(
                                bugModel.remoteProvider,
                                workflow.providerConfirmed(ModelScope.Bug)),
                        currentEditIdentity = currentEditIdentity(appState),
                    ),
                bugModel = bugModel,
                remoteProviderConfirmed = workflow.providerConfirmed(ModelScope.Bug),
                impact = appState.impact,
                gitStatus = appState.gitStatus,
            ),
        actions =
            ContextToolWindowActions(
                confirmRemoteProvider = { presenter.setProviderConfirmation(ModelScope.Bug, it) },
                analyze = { presenter.analyzeSelected(false) },
                refresh = { presenter.analyzeSelected(true) },
                cancel = presenter::cancelAnalysis,
                editSelected = ::requestDirectEdit,
            ),
        modifier = modifier,
    )
  }
  val assistantPane: @Composable (Modifier) -> Unit = { modifier ->
    if (composerRequested || editorProgress.progress == EditorProgress.Edit) {
      val target =
          validateChatTarget(
                  appState.selectedFile,
                  appState.symbols,
                  appState.selectedSymbol,
                  chatMode,
                  newChatSymbol)
              .target
      val draft = appState.review.draft
      val draftEditorVisible =
          draft != null &&
              appState.review.editor != null &&
              chatDraftMatchesSession(draft, appState.chat.session)
      val focusTarget = pendingComposerFocus
      LaunchedEffect(focusTarget, draftEditorVisible) {
        when (focusTarget) {
          ComposerFocusTarget.Chat -> chatFocusRequester.requestFocus()
          ComposerFocusTarget.Draft -> if (draftEditorVisible) draftFocusRequester.requestFocus()
          null -> Unit
        }
        if (pendingComposerFocus == focusTarget) pendingComposerFocus = null
      }
      DraftContextPane(
          state =
              DraftContextPaneState(
                  project = appState.project,
                  selected = appState.selectedFile,
                  session = appState.chat.session,
                  draft = appState.review.draft,
                  editor = appState.review.editor,
                  target = target,
                  mode = chatMode,
                  newSymbol = newChatSymbol,
                  message = chatMessage,
                  sending = workflow.generating,
                  functionModel = functionModel,
                  remoteConfirmed = workflow.providerConfirmed(ModelScope.Function),
                  chatFocus = chatFocusRequester,
                  draftFocus = draftFocusRequester,
              ),
          conversationActions =
              DraftConversationActions(
                  updateMessage = { chatMessage = it },
                  updateNewSymbol = { newChatSymbol = it },
                  confirmRemoteProvider = {
                    presenter.setProviderConfirmation(ModelScope.Function, it)
                  },
                  inspectContext = { presenter.inspectContext(contextAction) },
                  send = { presenter.sendChatMessage(chatMode, newChatSymbol, chatMessage) },
                  cancel = presenter::cancelGeneration,
              ),
          editorActions =
              DraftEditorActions(
                  updateDeclaration = {
                    presenter.dispatch(DesktopEvent.DraftEdited(declaration = it))
                  },
                  updateImports = { presenter.dispatch(DesktopEvent.DraftEdited(imports = it)) },
                  validate = presenter::validateEditableDraft,
              ),
          modifier = modifier,
      )
    } else {
      SystemStateMessage(
          "Assistant",
          "Start one declaration edit to open a bound conversation.",
          modifier = modifier)
    }
  }
  val reviewPane: @Composable (Modifier) -> Unit = { modifier ->
    if (editorProgress.progress in setOf(EditorProgress.Review, EditorProgress.Receipt)) {
      ReviewContextPane(
          reviewContextPaneState(appState),
          reviewEvidenceActions(presenter, chatMode, newChatSymbol) { composerRequested = true },
          draftApplicationActions(presenter),
          modifier)
    } else {
      SystemStateMessage(
          "Review",
          "Validate the current candidate to inspect evidence and guarded Apply.",
          modifier = modifier)
    }
  }
  val rightToolWindows: @Composable (RightToolWindow, Modifier) -> Unit = { toolWindow, modifier ->
    when (toolWindow) {
      RightToolWindow.Context -> contextPane(modifier)
      RightToolWindow.Assistant -> assistantPane(modifier)
      RightToolWindow.Review -> reviewPane(modifier)
    }
  }
  val contextualActions =
      editorContextualActions(
          appState,
          chatMode,
          newChatSymbol,
          chatMessage,
          sending = workflow.generating,
          functionModel = functionModel,
          remoteProviderConfirmed = workflow.providerConfirmed(ModelScope.Function),
      )
  DesktopShell(
      state =
          DesktopShellState(
              app = appState,
              layout = layout,
              editor =
                  DesktopShellEditorState(
                      progress = editorProgress,
                      contextualActions = contextualActions,
                      analysisInProgress = workflow.analysisInProgress,
                      generating = workflow.generating,
                  ),
              context =
                  DesktopShellContextState(
                      visible = showContext,
                      manifest = workflow.contextManifest,
                      bugModel = bugModel,
                      bugProviderConfirmed = workflow.providerConfirmed(ModelScope.Bug),
                  ),
              palette = DesktopShellPaletteState(paletteMode, paletteQuery, showPalette),
          ),
      layoutActions =
          DesktopShellLayoutActions(
              updateLayout = { layout = it },
              saveLayout = { layoutStore.save(layout) },
          ),
      projectActions =
          DesktopShellProjectActions(
              importProject = ::importProject,
              reanalyzeProject = presenter::reanalyze,
              reconnect = presenter::refreshConnection,
          ),
      editorActions =
          DesktopShellEditorActions(
              selectWorkspace = { presenter.dispatch(DesktopEvent.WorkspaceSelected(it)) },
              selectEditorSurface = { surface ->
                layout = layout.withEditorSurface(surface).withFocus(DesktopFocusRegion.Editor)
              },
              focusChat = { focusAssistantControl(ComposerFocusTarget.Chat) },
              focusDraft = { focusAssistantControl(ComposerFocusTarget.Draft) },
              cancelAnalysis = presenter::cancelAnalysis,
              sourceLineSelected = { selection ->
                presenter.dispatch(DesktopEvent.SourceLineSelected(selection))
                composerRequested = false
              },
              validateDraft = presenter::validateEditableDraft,
              runDraftChecks = presenter::runDraftChecks,
              generate = { presenter.sendChatMessage(chatMode, newChatSymbol, chatMessage) },
              cancelGeneration = presenter::cancelGeneration,
              dismissContext = {
                showContext = false
                presenter.clearContextManifest()
              },
          ),
      analysisActions =
          DesktopShellAnalysisActions(
              confirmBugProvider = { presenter.setProviderConfirmation(ModelScope.Bug, it) },
              startAnalyzeAll = presenter::startAnalyzeAll,
              pauseAnalyzeAll = presenter::pauseAnalyzeAll,
              resumeAnalyzeAll = presenter::resumeAnalyzeAll,
              cancelAnalyzeAll = presenter::cancelAnalyzeAll,
              startScan = presenter::runVerifiedScan,
              cancelScan = presenter::cancelVerifiedScan,
          ),
      findingActions =
          DesktopShellFindingActions(
              openFinding = presenter::openFinding,
              prepareFinding = presenter::prepareFinding,
              triageFinding = presenter::triageFinding,
          ),
      paletteActions =
          DesktopShellPaletteActions(
              updateQuery = { paletteQuery = it },
              dismiss = { showPalette = false },
              open = ::openPalette,
              selectFile = {
                showPalette = false
                presenter.openFileInEditor(it)
              },
              selectSymbol = {
                showPalette = false
                presenter.dispatch(DesktopEvent.SymbolSelected(it))
                presenter.dispatch(DesktopEvent.WorkspaceSelected(Workspace.Editor))
                composerRequested = false
              },
              selectAction = {
                showPalette = false
                presenter.dispatch(DesktopEvent.WorkspaceSelected(Workspace.Editor))
                when (it) {
                  "refresh_file_analysis" -> presenter.analyzeSelected(true)
                  "create_declaration" -> requestCreateDeclaration()
                  else -> contextAction = it
                }
              },
          ),
      panes = DesktopShellPanes(explorer, rightToolWindows),
  )
  pendingDraftDiscard?.let { pending ->
    DraftDiscardDialog(pending, ::discardDraftAndContinue) { pendingDraftDiscard = null }
  }
  pendingImportPath?.let { path ->
    ProjectImportConfirmationDialog(
        model = analyzeModel,
        confirmed = workflow.providerConfirmed(ModelScope.Analyze),
        onConfirmed = { presenter.setProviderConfirmation(ModelScope.Analyze, it) },
        onImport = {
          pendingImportPath = null
          presenter.loadProject(path, restore = false)
        },
        onCancel = { pendingImportPath = null },
    )
  }
}

private fun reviewContextPaneState(state: DesktopState) =
    ReviewContextPaneState(
        project = state.project,
        selected = state.selectedFile,
        session = state.chat.session,
        editor = state.review.editor,
        draft = state.review.draft,
        checks = state.checks,
        impact = state.impact,
        gitStatus = state.gitStatus,
        applied = state.review.applied,
        checksRunning = state.loading,
    )

private fun reviewEvidenceActions(
    presenter: DesktopWorkflowPresenter,
    chatMode: ChatEditMode,
    newChatSymbol: String,
    editDraft: () -> Unit,
) =
    ReviewEvidenceActions(
        runChecks = presenter::runDraftChecks,
        reviseWithCheckOutput = { presenter.reviseWithCheckOutput(chatMode, newChatSymbol) },
        editDraft = editDraft,
    )

private fun draftApplicationActions(presenter: DesktopWorkflowPresenter) =
    DraftApplicationActions(
        apply = presenter::applyEditableDraft,
        undo = presenter::undoAppliedDraft,
    )

@Composable
private fun ProjectImportConfirmationDialog(
    model: ScopedModel,
    confirmed: Boolean,
    onConfirmed: (Boolean) -> Unit,
    onImport: () -> Unit,
    onCancel: () -> Unit,
) {
  AlertDialog(
      onDismissRequest = onCancel,
      title = { Text("Confirm project analysis destination") },
      text = {
        Column {
          Text("Import sends the selected project's analysis context to this provider.")
          RemoteProviderConfirmation(ModelScope.Analyze, model, confirmed, onConfirmed)
        }
      },
      confirmButton = {
        FocusFlowButton(onClick = onImport, enabled = confirmed, tone = ActionTone.Primary) {
          Text("Import project")
        }
      },
      dismissButton = {
        FocusFlowButton(onClick = onCancel, tone = ActionTone.Neutral) { Text("Cancel") }
      },
  )
}

@Composable
private fun DraftDiscardDialog(
    pending: PendingDraftDiscard,
    onDiscard: () -> Unit,
    onCancel: () -> Unit
) {
  AlertDialog(
      onDismissRequest = onCancel,
      title = { Text("Discard current draft?") },
      text = {
        Text(
            "Discard the draft for ${pending.currentDraft.targetSymbol} and ${pending.nextLabel}? This only clears the in-memory conversation, draft, and focused checks.")
      },
      confirmButton = {
        FocusFlowButton(onClick = onDiscard, tone = ActionTone.Destructive) {
          Text("Discard draft")
        }
      },
      dismissButton = {
        FocusFlowButton(onClick = onCancel, tone = ActionTone.Neutral) { Text("Keep draft") }
      },
  )
}

private fun chooseDirectory(): File? {
  val chooser =
      JFileChooser().apply {
        dialogTitle = "Import project"
        fileSelectionMode = JFileChooser.DIRECTORIES_ONLY
        isAcceptAllFileFilterUsed = false
      }
  return if (chooser.showOpenDialog(null) == JFileChooser.APPROVE_OPTION) chooser.selectedFile
  else null
}

internal fun staleRemoteConfirmationMessage(error: Throwable, scope: ModelScope): String? =
    (error as? ApiException)
        ?.takeIf { it.message?.contains("confirmation", ignoreCase = true) == true }
        ?.let {
          "The ${scope.label.lowercase()} model destination changed. Confirm it again before retrying."
        }
