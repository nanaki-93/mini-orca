package io.miniorca.desktop

import androidx.compose.foundation.layout.Column
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
import androidx.compose.ui.text.input.TextFieldValue
import java.io.File
import javax.swing.JFileChooser

internal enum class PaletteMode {
  Files,
  Symbols,
  Actions
}

private enum class ComposerFocusTarget {
  Name,
  Chat,
  Draft
}

internal fun layoutForPreparedRequest(layout: DesktopLayoutState): DesktopLayoutState =
    layout.openRight(RightToolWindow.Assistant).withFocus(DesktopFocusRegion.RightToolWindow)

private data class DraftFieldIdentity(
    val id: String,
    val revision: Long,
    val hash: String,
)

private fun draftFieldIdentity(editor: EditableDraftState?): DraftFieldIdentity? =
    editor?.serverDraft?.let { DraftFieldIdentity(it.id, it.revision, it.hash) }

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
      val kind: DeclarationCreationKind,
  ) : PendingDraftDiscard {
    override val nextLabel: String = "create a ${kind.noun}"
  }
}

@Composable
internal fun MiniOrcaApp(
    terminal: DesktopTerminalWorkspace = remember { DesktopTerminalWorkspace() },
    api: ApiClient = remember { ApiClient() },
    lastProjectStore: LastProjectStore = remember { LastProjectStore() },
    layoutStore: DesktopLayoutStore = remember { DesktopLayoutStore() },
) {
  val scope = rememberCoroutineScope()
  val presenter =
      remember(api, lastProjectStore, scope) {
        DesktopWorkflowPresenter(api, lastProjectStore, scope)
      }
  val workflow by presenter.snapshot.collectAsState()
  val appState = workflow.state
  var layout by remember { mutableStateOf(layoutStore.load()) }
  var filter by remember { mutableStateOf("") }
  var collapsedDirectories by remember { mutableStateOf(emptySet<String>()) }
  var contextAction by remember { mutableStateOf("fix") }
  var showContext by remember { mutableStateOf(false) }
  var paletteMode by remember { mutableStateOf(PaletteMode.Files) }
  var paletteQuery by remember { mutableStateOf("") }
  var showPalette by remember { mutableStateOf(false) }
  var chatMode by remember { mutableStateOf(ChatEditMode.ReplaceSymbol) }
  var creationKind by remember { mutableStateOf(DeclarationCreationKind.Function) }
  var newChatSymbol by remember { mutableStateOf("") }
  var chatMessage by remember { mutableStateOf(TextFieldValue()) }
  var advancedConstraints by remember { mutableStateOf(TextFieldValue()) }
  var consumedPreparedRequestGeneration by remember { mutableStateOf(0L) }
  var draftFieldKey by remember { mutableStateOf<DraftFieldIdentity?>(null) }
  var draftFieldValue by remember { mutableStateOf(TextFieldValue()) }
  var pendingTerminalSwitch by remember { mutableStateOf<String?>(null) }
  var pendingImportPath by remember { mutableStateOf<String?>(null) }
  var composerRequested by remember { mutableStateOf(false) }
  var pendingComposerFocus by remember { mutableStateOf<ComposerFocusTarget?>(null) }
  var pendingDraftDiscard by remember { mutableStateOf<PendingDraftDiscard?>(null) }
  val chatFocusRequester = remember { FocusRequester() }
  val creationNameFocusRequester = remember { FocusRequester() }
  val draftFocusRequester = remember { FocusRequester() }
  TerminalSourceRefreshEffect(appState, layout, terminal, presenter)
  val analyzeModel = workflow.model(ModelScope.Analyze)
  val bugModel = workflow.model(ModelScope.Bug)
  val functionModel = workflow.model(ModelScope.Function)

  val editorProgress = editorProgressUiState(appState)
  val rightToolWindowBadges =
      workflowToolWindowBadges(
          editor = appState.review.editor,
          evidence =
              reviewEvidenceUiState(
                  project = appState.project,
                  selected = appState.selectedFile,
                  editor = appState.review.editor,
                  draft = appState.review.draft,
                  checks = appState.review.checks,
                  checksRunning = appState.loading,
              ),
          decision =
              applyDecisionUiState(
                  project = appState.project,
                  selected = appState.selectedFile,
                  editor = appState.review.editor,
                  draft = appState.review.draft,
                  checks = appState.review.checks,
                  applied = appState.review.applied,
              ),
      )
  LaunchedEffect(editorProgress.progress) {
    if (editorProgress.progress in setOf(EditorProgress.Review, EditorProgress.Receipt))
        composerRequested = false
  }
  LaunchedEffect(appState.preparedRequestGeneration) {
    val preparedRequest =
        unconsumedPreparedRequest(appState, consumedPreparedRequestGeneration)
            ?: return@LaunchedEffect
    consumedPreparedRequestGeneration = appState.preparedRequestGeneration
    if (appState.preparedAction.isNotBlank()) contextAction = appState.preparedAction
    if (appState.preparedTaskSpec != null) chatMode = ChatEditMode.ReplaceSymbol
    chatMessage = TextFieldValue(preparedRequest)
    layout = layoutForPreparedRequest(layout)
    composerRequested = true
    pendingComposerFocus = ComposerFocusTarget.Chat
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
      chatMessage = TextFieldValue()
      advancedConstraints = TextFieldValue()
      layout = layout.withEditorSurface(EditorSurface.Source)
    }
  }
  val activeDraftFieldIdentity = draftFieldIdentity(appState.review.editor)
  LaunchedEffect(activeDraftFieldIdentity) {
    draftFieldKey = activeDraftFieldIdentity
    draftFieldValue = TextFieldValue(appState.review.editor?.declaration.orEmpty())
  }
  val activeDraftFieldValue =
      if (draftFieldKey == activeDraftFieldIdentity) draftFieldValue
      else TextFieldValue(appState.review.editor?.declaration.orEmpty())
  LaunchedEffect(workflow.contextManifest) {
    if (workflow.contextManifest != null) showContext = true
  }
  LaunchedEffect(presenter) { presenter.start() }
  DisposableEffect(presenter, terminal) {
    terminal.onFocusLeft = { presenter.refreshSelectedFile() }
    onDispose {
      terminal.onFocusLeft = {}
      presenter.close()
    }
  }

  fun focusComposerControl(target: ComposerFocusTarget) {
    composerRequested = true
    pendingComposerFocus = target
  }

  fun focusAssistantControl(target: ComposerFocusTarget) {
    layout =
        layout.openRight(RightToolWindow.Assistant).withFocus(DesktopFocusRegion.RightToolWindow)
    focusComposerControl(target)
  }

  fun clearComposerInput() {
    chatMessage = TextFieldValue()
    advancedConstraints = TextFieldValue()
  }

  fun startReplaceEdit(request: DirectEditRequest) {
    presenter.clearPreparedSuggestion()
    if (appState.selectedSymbol != request.selectedSymbol)
        presenter.dispatch(DesktopEvent.SymbolSelected(request.selectedSymbol))
    chatMode = ChatEditMode.ReplaceSymbol
    newChatSymbol = ""
    clearComposerInput()
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

  fun startCreateDeclaration(kind: DeclarationCreationKind) {
    presenter.clearPreparedSuggestion()
    chatMode = ChatEditMode.CreateSymbol
    creationKind = kind
    newChatSymbol = ""
    clearComposerInput()
    presenter.dispatch(DesktopEvent.WorkspaceSelected(Workspace.Editor))
    focusAssistantControl(ComposerFocusTarget.Name)
  }

  fun requestCreateDeclaration(kind: DeclarationCreationKind) {
    routeCreationRequest(workflow, kind, ::startCreateDeclaration) { pendingDraftDiscard = it }
  }

  fun sendComposerMessage() {
    submitComposerMessage(
        presenter,
        chatMode,
        creationKind,
        newChatSymbol,
        chatMessage.text,
        advancedConstraints.text)
  }

  fun discardDraftAndContinue() {
    when (val pending = pendingDraftDiscard) {
      is PendingDraftDiscard.Replace -> {
        presenter.discardDraft()
        pendingDraftDiscard = null
        startReplaceEdit(pending.request)
      }
      is PendingDraftDiscard.Create -> {
        presenter.discardDraft()
        pendingDraftDiscard = null
        startCreateDeclaration(pending.kind)
      }
      null -> Unit
    }
  }

  fun loadChosenProject(path: String) {
    if (analyzeModel.remoteProvider && !workflow.providerConfirmed(ModelScope.Analyze))
        pendingImportPath = path
    else presenter.loadProject(path, restore = false)
  }

  fun importProject() {
    val directory = chooseDirectory() ?: return
    if (terminal.state.value.requiresClose) pendingTerminalSwitch = directory.absolutePath
    else loadChosenProject(directory.absolutePath)
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
                  clearComposerInput()
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
                fileAnalysis = appState.analysis,
                analysisRun = appState.analysisRun,
                project = appState.project,
                overview = appState.overview,
                functionModel = workflow.model(ModelScope.Function),
                functionRemoteProviderConfirmed = workflow.providerConfirmed(ModelScope.Function),
                declarationExplanation = workflow.declarationExplanation,
                creationInProgress = workflow.creationInProgress,
            ),
        actions =
            ContextToolWindowActions(
                confirmRemoteProvider = { presenter.setProviderConfirmation(ModelScope.Bug, it) },
                analyze = { presenter.previewAnalysis() },
                viewResults = {
                  presenter.viewAnalysisResults("bugs", appState.selectedFile?.path.orEmpty())
                },
                refresh = { presenter.analyzeSelected(true) },
                cancel = presenter::cancelAnalysis,
                editSelected = ::requestDirectEdit,
                confirmFunctionRemoteProvider = {
                  presenter.setProviderConfirmation(ModelScope.Function, it)
                },
                explainSelected = presenter::explainSelectedDeclaration,
                cancelExplanation = presenter::cancelDeclarationExplanation,
                createDeclaration = { requestCreateDeclaration(DeclarationCreationKind.Function) },
            ),
        modifier = modifier,
    )
  }
  val assistantPane: @Composable (Modifier) -> Unit = { modifier ->
    if (composerRequested || editorProgress.progress == EditorProgress.Edit) {
      val targetValidation =
          validateChatTarget(
              appState.selectedFile,
              appState.symbols,
              appState.selectedSymbol,
              chatMode,
              newChatSymbol)
      val target = targetValidation.target
      val draft = appState.review.draft
      val draftEditorVisible =
          draft != null &&
              appState.review.editor != null &&
              chatDraftMatchesSession(draft, appState.chat.session)
      val focusTarget = pendingComposerFocus
      LaunchedEffect(focusTarget, draftEditorVisible) {
        when (focusTarget) {
          ComposerFocusTarget.Name -> creationNameFocusRequester.requestFocus()
          ComposerFocusTarget.Chat -> chatFocusRequester.requestFocus()
          ComposerFocusTarget.Draft -> if (draftEditorVisible) draftFocusRequester.requestFocus()
          null -> Unit
        }
        if (pendingComposerFocus == focusTarget) pendingComposerFocus = null
      }
      AssistantToolWindow(
          state =
              AssistantToolWindowState(
                  project = appState.project,
                  selected = appState.selectedFile,
                  session = appState.chat.session,
                  draft = appState.review.draft,
                  editor = appState.review.editor,
                  target = target,
                  mode = chatMode,
                  newSymbol = newChatSymbol,
                  message = chatMessage.text,
                  sending = workflow.generating,
                  functionModel = functionModel,
                  remoteConfirmed = workflow.providerConfirmed(ModelScope.Function),
                  chatFocus = chatFocusRequester,
                  draftFocus = draftFocusRequester,
                  messageInput = chatMessage,
                  draftInput = activeDraftFieldValue,
                  selectedSymbol = appState.selectedSymbol,
                  targetValidation = targetValidation,
                  advancedConstraintsInput = advancedConstraints,
                  creationKind = creationKind,
                  creationNameFocus = creationNameFocusRequester,
                  requestFailure = appState.chat.failure,
              ),
          conversationActions =
              AssistantConversationActions(
                  updateMessage = { chatMessage = TextFieldValue(it) },
                  updateNewSymbol = { newChatSymbol = it },
                  confirmRemoteProvider = {
                    presenter.setProviderConfirmation(ModelScope.Function, it)
                  },
                  inspectContext = { presenter.inspectContext(contextAction) },
                  send = ::sendComposerMessage,
                  cancel = presenter::cancelGeneration,
                  updateMessageValue = { chatMessage = it },
                  preparePreset = { preset ->
                    presenter.clearPreparedSuggestion()
                    chatMessage = preparedFunctionChangeMessage(preset)
                    focusComposerControl(ComposerFocusTarget.Chat)
                  },
                  updateAdvancedConstraintsValue = { advancedConstraints = it },
              ),
          editorActions =
              DraftEditorActions(
                  updateDeclaration = {
                    presenter.dispatch(DesktopEvent.DraftEdited(declaration = it))
                  },
                  updateImports = { presenter.dispatch(DesktopEvent.DraftEdited(imports = it)) },
                  validate = presenter::validateEditableDraft,
                  updateDeclarationValue = {
                    draftFieldValue = it
                    presenter.dispatch(DesktopEvent.DraftEdited(declaration = it.text))
                  },
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
      ReviewToolWindow(
          reviewToolWindowState(appState),
          reviewToolWindowActions(presenter, chatMode, newChatSymbol) { composerRequested = true },
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
  val findingActions =
      FindingActions(
          openFinding = {
            clearComposerInput()
            presenter.openFinding(it)
          },
          prepareFinding = {
            clearComposerInput()
            presenter.prepareFinding(it)
          },
          triageFinding = presenter::triageFinding,
      )
  val terminalContent: @Composable (Modifier) -> Unit = { modifier ->
    if (appState.project != null) TerminalToolWindow(terminal, modifier)
  }
  val terminalState by terminal.state.collectAsState()
  val contextualActions =
      editorContextualActions(
          appState,
          chatMode,
          newChatSymbol,
          chatMessage.text,
          sending = workflow.generating,
          functionModel = functionModel,
          remoteProviderConfirmed = workflow.providerConfirmed(ModelScope.Function),
      )
  DesktopShell(
      terminal = terminal,
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
                      analyzeModel = analyzeModel,
                      analyzeProviderConfirmed = workflow.providerConfirmed(ModelScope.Analyze),
                      securityReviewRemoteConfirmed = workflow.securityReviewRemoteConfirmed,
                  ),
              palette = DesktopShellPaletteState(paletteMode, paletteQuery, showPalette),
              statusProviders =
                  DesktopShellStatusProviders(
                      analyze = analyzeModel,
                      bugs = bugModel,
                      functionEdits = functionModel,
                  ),
          ),
      layoutActions =
          DesktopShellLayoutActions(
              updateLayout = { layout = it },
              saveLayout = layoutStore::save,
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
                if (selection.symbol != appState.selectedSymbol) clearComposerInput()
                composerRequested = false
              },
              validateDraft = presenter::validateEditableDraft,
              runDraftChecks = presenter::runDraftChecks,
              generate = ::sendComposerMessage,
              createDeclaration = { requestCreateDeclaration(DeclarationCreationKind.Function) },
              cancelGeneration = presenter::cancelGeneration,
              dismissContext = {
                showContext = false
                presenter.clearContextManifest()
              },
          ),
      analysisActions =
          DesktopShellAnalysisActions(
              startAnalysis = { limits, retry ->
                presenter.previewAnalysis(limits = limits, retryStaleFailed = retry)
              },
              pauseAnalysis = presenter::pauseAnalysis,
              resumeAnalysis = presenter::resumeAnalysis,
              cancelAnalysis = presenter::cancelAnalysis,
              startScan = presenter::runVerifiedScan,
              cancelScan = presenter::cancelVerifiedScan,
              openPerformanceFinding = { path, finding ->
                clearComposerInput()
                presenter.openFileInEditor(
                    path, EditorNavigationTarget(path, finding.symbol, finding.startLine))
              },
              preparePerformanceFinding = { path, finding ->
                clearComposerInput()
                presenter.preparePerformanceFinding(path, finding)
              },
              loadGoBenchmarks = presenter::loadGoBenchmarks,
              selectGoBenchmark = presenter::selectGoBenchmark,
              compareSelectedGoBenchmark = presenter::compareSelectedGoBenchmark,
              openSecurityFinding = presenter::openSecurityFinding,
              prepareSecurityFinding = { finding ->
                clearComposerInput()
                presenter.prepareSecurityFinding(finding)
              },
          ),
      findingActions = findingActions,
      paletteActions =
          DesktopShellPaletteActions(
              updateQuery = { paletteQuery = it },
              dismiss = { showPalette = false },
              open = ::openPalette,
              selectFile = {
                showPalette = false
                clearComposerInput()
                presenter.openFileInEditor(it)
              },
              selectSymbol = {
                showPalette = false
                presenter.dispatch(DesktopEvent.SymbolSelected(it))
                presenter.dispatch(DesktopEvent.WorkspaceSelected(Workspace.Editor))
                if (it != appState.selectedSymbol) clearComposerInput()
                composerRequested = false
              },
              selectAction = { action ->
                showPalette = false
                commandActionWorkspace(action)?.let {
                  presenter.dispatch(DesktopEvent.WorkspaceSelected(it))
                }
                when (action) {
                  "start_analysis" -> presenter.previewAnalysis()
                  "create_function" -> requestCreateDeclaration(DeclarationCreationKind.Function)
                  "create_type" -> requestCreateDeclaration(DeclarationCreationKind.Type)
                  "fix",
                  "refactor",
                  "document" -> contextAction = action
                }
              },
          ),
      panes =
          DesktopShellPanes(
              explorer,
              rightToolWindows,
              rightToolWindowBadges,
              terminalContent,
              terminalState,
              TerminalTabActions(
                  terminal::selectShell,
                  { appState.project?.path?.let { terminal.createShell(it) } },
                  { terminal.closeSession(it) })),
  )
  TerminalProjectSwitchDialog(pendingTerminalSwitch, terminal, { pendingTerminalSwitch = null }) {
      path ->
    if (pendingTerminalSwitch == path) {
      pendingTerminalSwitch = null
      loadChosenProject(path)
    }
  }
  DesktopAnalysisAdmissionOverlay(appState.analysisRun, presenter)
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

private val DesktopWorkflowSnapshot.creationInProgress: Boolean
  get() = generating || draftValidationInProgress

private fun routeCreationRequest(
    workflow: DesktopWorkflowSnapshot,
    kind: DeclarationCreationKind,
    start: (DeclarationCreationKind) -> Unit,
    confirmDiscard: (PendingDraftDiscard.Create) -> Unit,
) {
  if (declarationCreationBlockedReason(workflow.state.selectedFile, workflow.creationInProgress) !=
      null)
      return
  val currentDraft = currentEditIdentity(workflow.state)?.takeIf { it.hasDraft }
  if (currentDraft == null) start(kind)
  else confirmDiscard(PendingDraftDiscard.Create(currentDraft, kind))
}

private fun submitComposerMessage(
    presenter: DesktopWorkflowPresenter,
    mode: ChatEditMode,
    kind: DeclarationCreationKind,
    name: String,
    behavior: String,
    constraints: String,
) {
  val workflow = presenter.snapshot.value
  if (mode == ChatEditMode.CreateSymbol &&
      (declarationCreationBlockedReason(workflow.state.selectedFile, workflow.creationInProgress) !=
          null || !hasFunctionChangeIntent(behavior)))
      return
  val request = functionChangeRequest(behavior, constraints)
  presenter.sendChatMessage(
      mode,
      name,
      if (mode == ChatEditMode.CreateSymbol) creationMessage(kind, name, request) else request)
}

private fun reviewToolWindowState(state: DesktopState) =
    ReviewToolWindowState(
        project = state.project,
        selected = state.selectedFile,
        selectedSymbol = state.selectedSymbol,
        session = state.chat.session,
        editor = state.review.editor,
        draft = state.review.draft,
        checks = state.checks,
        impact = state.impact,
        gitStatus = state.gitStatus,
        applied = state.review.applied,
        checksRunning = state.loading,
    )

private fun reviewToolWindowActions(
    presenter: DesktopWorkflowPresenter,
    chatMode: ChatEditMode,
    newChatSymbol: String,
    editDraft: () -> Unit,
) =
    ReviewToolWindowActions(
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
  IdeDialog(
      onDismissRequest = onCancel,
      title = { Text("Confirm project analysis destination") },
      content = {
        Column {
          Text("Import sends the selected project's analysis context to this provider.")
          RemoteProviderConfirmation(ModelScope.Analyze, model, confirmed, onConfirmed)
        }
      },
      actions = {
        MiniOrcaButton(onClick = onCancel, tone = ActionTone.Neutral) { Text("Cancel") }
        MiniOrcaButton(onClick = onImport, enabled = confirmed, tone = ActionTone.Primary) {
          Text("Import project")
        }
      },
  )
}

@Composable
private fun DraftDiscardDialog(
    pending: PendingDraftDiscard,
    onDiscard: () -> Unit,
    onCancel: () -> Unit
) {
  IdeDialog(
      onDismissRequest = onCancel,
      title = { Text("Discard current draft?") },
      content = {
        Text(
            "Discard the draft for ${pending.currentDraft.targetSymbol} in ${pending.currentDraft.targetPath} before you ${pending.nextLabel} in ${pending.currentDraft.targetPath}? This only clears the in-memory conversation, draft, and focused checks.")
      },
      actions = {
        MiniOrcaButton(onClick = onCancel, tone = ActionTone.Neutral) { Text("Keep draft") }
        MiniOrcaButton(onClick = onDiscard, tone = ActionTone.Destructive) { Text("Discard draft") }
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
