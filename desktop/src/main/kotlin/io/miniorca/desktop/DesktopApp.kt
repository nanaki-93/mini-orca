package io.miniorca.desktop

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.text.selection.SelectionContainer
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
import java.util.concurrent.CompletableFuture
import javax.swing.JFileChooser
import javax.swing.SwingUtilities
import javax.swing.Timer

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

internal sealed interface PendingDraftDiscard {
  val currentDraft: CurrentEditIdentity?
  val nextLabel: String

  data class Replace(
      val request: DirectEditRequest,
      override val currentDraft: CurrentEditIdentity,
  ) : PendingDraftDiscard {
    override val nextLabel: String = "edit ${request.target.symbol}"
  }

  enum class CreationIntent {
    Fresh,
    ChangeKind
  }

  data class Create(
      override val currentDraft: CurrentEditIdentity,
      val intent: CreationIntent,
      val kind: DeclarationCreationKind,
      val priorKind: DeclarationCreationKind,
      val priorMode: ChatEditMode,
      val name: String,
      val project: ProjectAnalysis?,
      val index: ProjectIndex?,
      val file: ProjectFileInfo,
      val review: DraftReviewState,
      val chat: ChatState,
      val message: TextFieldValue,
      val constraints: TextFieldValue,
  ) : PendingDraftDiscard {
    override val nextLabel: String = "create a ${kind.noun}"
  }

  data class FileNavigation(
      val intent: DesktopWorkflowPresenter.FileNavigationIntent,
      override val currentDraft: CurrentEditIdentity?,
      val chatMessage: TextFieldValue,
      val constraints: TextFieldValue,
  ) : PendingDraftDiscard {
    override val nextLabel: String = "open ${intent.path}"
  }

  data class PerformancePreparation(
      val intent: DesktopWorkflowPresenter.PerformancePreparationIntent,
      override val currentDraft: CurrentEditIdentity?,
      val chatMessage: TextFieldValue,
      val constraints: TextFieldValue,
  ) : PendingDraftDiscard {
    override val nextLabel: String = "prepare a fix for ${intent.result.finding.title}"
  }

  data class PerformanceSource(
      val intent: DesktopWorkflowPresenter.PerformanceSourceIntent,
      override val currentDraft: CurrentEditIdentity?,
      val chatMessage: TextFieldValue,
      val constraints: TextFieldValue,
  ) : PendingDraftDiscard {
    override val nextLabel: String = "open ${intent.target.path}"
  }

  data class SecuritySource(
      val intent: DesktopWorkflowPresenter.SecuritySourceIntent,
      override val currentDraft: CurrentEditIdentity?,
      val chatMessage: TextFieldValue,
      val constraints: TextFieldValue,
  ) : PendingDraftDiscard {
    override val nextLabel: String = "open ${intent.target.path}"
  }

  data class SecurityPreparation(
      val intent: DesktopWorkflowPresenter.SecurityPreparationIntent,
      override val currentDraft: CurrentEditIdentity?,
      val chatMessage: TextFieldValue,
      val constraints: TextFieldValue,
  ) : PendingDraftDiscard {
    override val nextLabel: String = "prepare a fix for ${intent.result.finding.title}"
  }

  data class Finding(
      val intent: DesktopWorkflowPresenter.FindingIntent,
      override val currentDraft: CurrentEditIdentity?,
      val chatMessage: TextFieldValue,
      val constraints: TextFieldValue,
  ) : PendingDraftDiscard {
    override val nextLabel: String =
        if (intent.prepare) "prepare a fix for ${intent.finding.title}"
        else "open ${intent.target.path}"
  }
}

internal data class SwitchProjectIdentity(val id: String, val revision: String, val path: String) {
  constructor(
      project: ProjectAnalysis
  ) : this(project.projectId, project.projectRevision, project.path)
}

/**
 * The editor buffer is part of the identity: editing without a new server draft revokes approval.
 */
internal data class SwitchDraftIdentity(
    val session: ChatSession?,
    val draft: DeclarationDraft?,
    val editor: EditableDraftState?,
) {
  val hasWork: Boolean
    get() = session != null || draft != null || editor != null
}

internal data class SwitchAnalyzeDestination(
    val scope: String,
    val profile: String,
    val model: String,
    val providerOrigin: String,
    val remote: Boolean,
) {
  constructor(
      model: ScopedModel
  ) : this(model.scope, model.profile, model.model, model.providerOrigin, model.remoteProvider)
}

internal data class ProjectSwitchContext(
    val project: SwitchProjectIdentity?,
    val draft: SwitchDraftIdentity,
    val destination: SwitchAnalyzeDestination,
    val analyzeConfirmed: Boolean,
) {
  constructor(
      workflow: DesktopWorkflowSnapshot
  ) : this(
      workflow.state.project?.let(::SwitchProjectIdentity),
      SwitchDraftIdentity(
          workflow.state.chat.session, workflow.state.review.draft, workflow.state.review.editor),
      SwitchAnalyzeDestination(workflow.model(ModelScope.Analyze)),
      workflow.providerConfirmed(ModelScope.Analyze))
}

internal enum class SwitchReviewStage {
  Draft,
  Provider,
  Review,
  Final,
  Committed,
}

internal data class SwitchCleanupFeedback(
    val error: String? = null,
    val outstanding: Boolean = false
)

internal class SwitchTerminalCleanup(
    val closeAll: () -> CompletableFuture<TerminalWorkspaceState>,
    val state: () -> TerminalWorkspaceState,
)

internal data class PendingProjectSwitch(
    val requestId: Long,
    val path: String,
    val context: ProjectSwitchContext,
    val stage: SwitchReviewStage,
)

/**
 * Admission records intent only. The caller owns the Analyze confirmation and performs cleanup
 * later.
 */
internal class ProjectSwitchAdmission {
  var pending: PendingProjectSwitch? = null
    private set

  private var nextRequestId = 0L
  var cleanupOutstanding: Boolean = false
    private set

  fun cleanupStarted(requestId: Long) {
    if (pending?.requestId == requestId && pending?.stage == SwitchReviewStage.Committed)
        cleanupOutstanding = true
  }

  fun cleanupSettled(requestId: Long) {
    if (pending?.requestId == requestId && pending?.stage == SwitchReviewStage.Committed)
        cleanupOutstanding = false
  }

  fun choose(path: String, context: ProjectSwitchContext): PendingProjectSwitch? {
    if (path.isBlank() || pending != null) return null
    pending = PendingProjectSwitch(++nextRequestId, path, context, firstStage(context))
    return pending
  }

  fun dismiss(requestId: Long) {
    if (pending?.requestId == requestId && pending?.stage != SwitchReviewStage.Committed)
        pending = null
  }

  fun finish(requestId: Long) {
    if (pending?.requestId == requestId &&
        pending?.stage == SwitchReviewStage.Committed &&
        !cleanupOutstanding)
        pending = null
  }

  fun approveDraft(requestId: Long, context: ProjectSwitchContext) {
    val current = review(requestId, context) ?: return
    if (current.stage == SwitchReviewStage.Draft) pending = current.copy(stage = nextStage(context))
  }

  fun approveProvider(requestId: Long, context: ProjectSwitchContext) {
    val current = review(requestId, context) ?: return
    if (current.stage == SwitchReviewStage.Provider &&
        (!context.destination.remote || context.analyzeConfirmed))
        pending = current.copy(stage = SwitchReviewStage.Final)
  }

  /** A changed identity without draft or provider steps still needs a fresh, explicit review. */
  fun approveReview(requestId: Long, context: ProjectSwitchContext) {
    val current = review(requestId, context) ?: return
    if (current.stage == SwitchReviewStage.Review)
        pending = current.copy(stage = SwitchReviewStage.Final)
  }

  /** Returns the committed request once; no side effects are performed by admission. */
  fun commit(requestId: Long, context: ProjectSwitchContext): PendingProjectSwitch? {
    val current = review(requestId, context) ?: return null
    if (current.stage != SwitchReviewStage.Final ||
        (context.destination.remote && !context.analyzeConfirmed))
        return null
    pending = current.copy(stage = SwitchReviewStage.Committed)
    return pending
  }

  private fun review(requestId: Long, context: ProjectSwitchContext): PendingProjectSwitch? {
    val current = pending ?: return null
    if (current.requestId != requestId || current.stage == SwitchReviewStage.Committed) return null
    if (current.context != context &&
        !(current.stage == SwitchReviewStage.Provider &&
            !current.context.analyzeConfirmed &&
            context.analyzeConfirmed &&
            current.context.copy(analyzeConfirmed = true) == context)) {
      val stage = firstStage(context)
      pending =
          current.copy(
              context = context,
              stage = if (stage == SwitchReviewStage.Final) SwitchReviewStage.Review else stage)
      return null
    }
    if (current.context != context) pending = current.copy(context = context)
    return pending
  }

  private fun firstStage(context: ProjectSwitchContext): SwitchReviewStage =
      if (context.draft.hasWork) SwitchReviewStage.Draft else nextStage(context)

  private fun nextStage(context: ProjectSwitchContext): SwitchReviewStage =
      if (context.destination.remote && !context.analyzeConfirmed) SwitchReviewStage.Provider
      else SwitchReviewStage.Final
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
  var paletteMode by remember { mutableStateOf(PaletteMode.Files) }
  var paletteQuery by remember { mutableStateOf("") }
  var showPalette by remember { mutableStateOf(false) }
  var paletteBlockedReason by remember { mutableStateOf<String?>(null) }
  var chatMode by remember { mutableStateOf(ChatEditMode.ReplaceSymbol) }
  var creationKind by remember { mutableStateOf(DeclarationCreationKind.Function) }
  var newChatSymbol by remember { mutableStateOf("") }
  var chatMessage by remember { mutableStateOf(TextFieldValue()) }
  var advancedConstraints by remember { mutableStateOf(TextFieldValue()) }
  var consumedPreparedRequestGeneration by remember { mutableStateOf(0L) }
  var draftFieldKey by remember { mutableStateOf<DraftFieldIdentity?>(null) }
  var draftFieldValue by remember { mutableStateOf(TextFieldValue()) }
  val switchAdmission = remember { ProjectSwitchAdmission() }
  var pendingSwitch by remember { mutableStateOf<PendingProjectSwitch?>(null) }
  var chooserOpen by remember { mutableStateOf(false) }
  var switchCleanupError by remember { mutableStateOf<String?>(null) }
  var composerRequested by remember { mutableStateOf(false) }
  var pendingComposerFocus by remember { mutableStateOf<ComposerFocusTarget?>(null) }
  var pendingDraftDiscard by remember { mutableStateOf<PendingDraftDiscard?>(null) }
  val chatFocusRequester = remember { FocusRequester() }
  val creationNameFocusRequester = remember { FocusRequester() }
  val draftFocusRequester = remember { FocusRequester() }
  val inspectContextFocusRequester = remember { FocusRequester() }
  var contextInspectOpener by remember { mutableStateOf<ContextInspectFocusOrigin?>(null) }
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
                  checksRunning =
                      appState.review.checkAttempt?.status == ValidationAttemptStatus.Running,
                  checkAttempt = appState.review.checkAttempt,
              ),
          decision =
              applyDecisionUiState(
                  project = appState.project,
                  selected = appState.selectedFile,
                  editor = appState.review.editor,
                  draft = appState.review.draft,
                  checks = appState.review.checks,
                  applied = appState.review.applied,
                  checkAttempt = appState.review.checkAttempt,
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
    if (appState.preparedTaskSpec != null) {
      presenter.contextCreationTargetChanged("", "")
      chatMode = ChatEditMode.ReplaceSymbol
    }
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
    presenter.contextCreationTargetChanged("", "")
    chatMode = ChatEditMode.ReplaceSymbol
    contextAction = "fix"
    newChatSymbol = ""
    clearComposerInput()
    focusAssistantControl(ComposerFocusTarget.Chat)
  }

  fun requestDirectEdit(symbol: SymbolInspectorSymbolState) {
    routeContextRefactor(presenter, symbol, ::startReplaceEdit) { pendingDraftDiscard = it }
  }

  fun startCreateDeclaration(kind: DeclarationCreationKind) {
    presenter.clearPreparedSuggestion()
    presenter.contextCreationTargetChanged("", kind.noun)
    chatMode = ChatEditMode.CreateSymbol
    creationKind = kind
    newChatSymbol = ""
    clearComposerInput()
    presenter.dispatch(DesktopEvent.WorkspaceSelected(Workspace.Editor))
    focusAssistantControl(ComposerFocusTarget.Name)
  }

  fun requestFileNavigation(path: String) {
    routeFileNavigationRequest(
        presenter,
        path,
        chatMessage,
        advancedConstraints,
        { chatMessage to advancedConstraints },
        ::clearComposerInput) {
          pendingDraftDiscard = it
        }
  }

  fun applyCreationKind(kind: DeclarationCreationKind) {
    presenter.contextCreationTargetChanged(newChatSymbol, kind.noun)
    creationKind = kind
  }

  fun changeCreationKind(kind: DeclarationCreationKind) {
    routeCreationKindChange(
        workflow,
        chatMode,
        creationKind,
        kind,
        newChatSymbol,
        chatMessage,
        advancedConstraints,
        ::applyCreationKind) {
          pendingDraftDiscard = it
        }
  }

  fun requestCreateDeclaration(kind: DeclarationCreationKind): String? {
    return routeCreationRequest(
        workflow,
        kind,
        ::startCreateDeclaration,
        { pendingDraftDiscard = it },
        chatMessage,
        advancedConstraints,
        creationKind,
        newChatSymbol,
        chatMode)
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
    val pending = pendingDraftDiscard
    pendingDraftDiscard = null
    continueAfterDraftDiscard(
        pending,
        presenter,
        chatMessage,
        advancedConstraints,
        ::clearComposerInput,
        { advancedConstraints = TextFieldValue() },
        { chatMessage to advancedConstraints },
        ::startReplaceEdit,
        ::startCreateDeclaration,
        ::applyCreationKind,
        { Triple(chatMode, creationKind, newChatSymbol) })
    if (pending is PendingDraftDiscard.Create) {
      focusAssistantControl(composerFocusAfterDiscard(chatMode))
    }
  }

  fun updatePendingSwitch() {
    pendingSwitch = switchAdmission.pending
  }

  fun dismissSwitch(requestId: Long) {
    switchAdmission.dismiss(requestId)
    updatePendingSwitch()
  }

  fun commitSwitch(requestId: Long) =
      commitProjectSwitch(
          requestId,
          switchAdmission,
          { ProjectSwitchContext(presenter.snapshot.value) },
          { projectOpenAvailable(presenter.snapshot.value.state.projectState.openingAttempt) },
          SwitchTerminalCleanup(terminal::closeAllSessions) { terminal.state.value },
          presenter::discardDraft,
          { presenter.loadProject(it, restore = false) },
          ::updatePendingSwitch,
          { switchCleanupError = it },
          { SwingUtilities.invokeLater(it) },
          ::scheduleSwitchCleanupTimeout)

  fun importProject() =
      admitProjectChooser(
          presenter.snapshot.value.state,
          projectSwitchPending(chooserOpen, switchAdmission.pending),
          setChooserOpen = { chooserOpen = it },
          choose = { switching -> chooseDirectory(switching) },
          current = { presenter.snapshot.value },
          admission = switchAdmission,
          updatePending = ::updatePendingSwitch)

  fun retryRestore() =
      dispatchProjectAction(
          !projectSwitchPending(chooserOpen, switchAdmission.pending),
          presenter::retryProjectRestore)

  fun reindexProject() {
    val current = presenter.snapshot.value.state
    dispatchProjectAction(
        projectActionAvailability(
                current.project,
                current.projectState.openingAttempt,
                current.projectState.indexingAttempt,
                projectSwitchPending(chooserOpen, switchAdmission.pending))
            .reindex,
        presenter::reindexProject)
  }

  fun openPalette(mode: PaletteMode) {
    paletteMode = mode
    paletteQuery = ""
    paletteBlockedReason = null
    showPalette = true
  }

  fun switchPaletteMode(mode: PaletteMode) {
    paletteMode = mode
    paletteBlockedReason = null
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
                projectAvailable = appState.project != null,
                readError = appState.selection.fileReadError,
                pendingFilePath = appState.selection.pendingFilePath,
                failedFilePath = appState.selection.failedFilePath,
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
                  requestFileNavigation(path)
                  onSelected()
                },
                openProject = ::importProject,
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
                        project = appState.project,
                        index = appState.index,
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
                fileReadError = appState.selection.fileReadError,
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
                openFile = { openPalette(PaletteMode.Files) },
            ),
        modifier = modifier,
    )
  }
  val assistantPane: @Composable (Modifier) -> Unit = { modifier ->
    if (composerRequested || editorProgress.progress == EditorProgress.Edit) {
      val targetValidation = validateChatTarget(appState.selection, chatMode, newChatSymbol)
      val target = targetValidation.target
      val draft = appState.review.draft
      val draftEditorVisible =
          draft != null &&
              appState.review.editor != null &&
              chatDraftMatchesSession(draft, appState.chat.session)
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
                  validating = workflow.draftValidationInProgress,
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
                  attempts = appState.chat.attempts,
                  taskSpec =
                      appState.preparedTaskSpec?.takeIf {
                        it.targetPath == appState.selectedFile?.path &&
                            it.targetSymbol == target?.symbol &&
                            chatMode == ChatEditMode.ReplaceSymbol
                      },
                  inspectContextFocus = inspectContextFocusRequester,
              ),
          conversationActions =
              AssistantConversationActions(
                  updateMessage = { chatMessage = TextFieldValue(it) },
                  updateNewSymbol = {
                    presenter.contextCreationTargetChanged(
                        it, if (chatMode == ChatEditMode.CreateSymbol) creationKind.noun else "")
                    newChatSymbol = it
                  },
                  confirmRemoteProvider = {
                    presenter.setProviderConfirmation(ModelScope.Function, it)
                  },
                  inspectContext = {
                    contextInspectOpener =
                        ContextInspectFocusOrigin(
                            appState.project?.projectId,
                            appState.workspace,
                            inspectContextFocusRequester)
                    presenter.inspectContext(
                        if (chatMode == ChatEditMode.CreateSymbol) "create" else contextAction,
                        chatMode,
                        newChatSymbol,
                        creationKind.noun)
                  },
                  send = ::sendComposerMessage,
                  cancel = presenter::cancelGeneration,
                  changeCreationKind = ::changeCreationKind,
                  updateMessageValue = { chatMessage = it },
                  preparePreset = { preset ->
                    presenter.clearPreparedSuggestion()
                    contextAction = "fix"
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
      ComposerFocusEffect(
          pendingComposerFocus,
          draftEditorVisible,
          showPalette || pendingDraftDiscard != null,
          creationNameFocusRequester,
          chatFocusRequester,
          draftFocusRequester) { focused ->
            if (pendingComposerFocus == focused) pendingComposerFocus = null
          }
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
          openSource = {
            routeFindingRequest(presenter, it, false, chatMessage, advancedConstraints) {
              pendingDraftDiscard = it
            }
          },
          prepareFinding = {
            routeFindingRequest(presenter, it, true, chatMessage, advancedConstraints) {
              pendingDraftDiscard = it
            }
          },
          triageFinding = presenter::triageFinding,
      )
  val terminalContent: @Composable (Modifier) -> Unit = { modifier ->
    if (appState.project != null) TerminalToolWindow(terminal, modifier)
  }
  val terminalState by terminal.state.collectAsState()
  fun selectPaletteAction(action: String): Boolean {
    val kind = creationKindForCommand(action)
    if (kind != null) {
      paletteBlockedReason = requestCreateDeclaration(kind)
      if (paletteBlockedReason == null) showPalette = false
    } else {
      showPalette = false
      commandActionWorkspace(action)?.let { presenter.dispatch(DesktopEvent.WorkspaceSelected(it)) }
      when (action) {
        "start_analysis" -> presenter.previewAnalysis()
        "fix",
        "refactor",
        "document" -> contextAction = action
      }
    }
    return kind != null && paletteBlockedReason == null
  }
  val contextualActions =
      editorContextualActions(
          appState,
          chatMode,
          newChatSymbol,
          chatMessage.text,
          sending = workflow.generating,
          validating = workflow.draftValidationInProgress,
          functionModel = functionModel,
          remoteProviderConfirmed = workflow.providerConfirmed(ModelScope.Function),
      )
  DesktopShell(
      terminal = terminal,
      contextInspectOpener = contextInspectOpener,
      contextInspectControlPresent =
          (composerRequested || editorProgress.progress == EditorProgress.Edit) &&
              !workflow.generating,
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
                      inspection = workflow.contextInspection,
                      bugModel = bugModel,
                      bugProviderConfirmed = workflow.providerConfirmed(ModelScope.Bug),
                      analyzeModel = analyzeModel,
                      analyzeProviderConfirmed = workflow.providerConfirmed(ModelScope.Analyze),
                      securityReviewRemoteConfirmed = workflow.securityReviewRemoteConfirmed,
                  ),
              palette =
                  DesktopShellPaletteState(
                      paletteMode, paletteQuery, showPalette, paletteBlockedReason),
              statusProviders =
                  DesktopShellStatusProviders(
                      analyze = analyzeModel,
                      bugs = bugModel,
                      functionEdits = functionModel,
                  ),
              switchPending = projectSwitchPending(chooserOpen, pendingSwitch),
          ),
      layoutActions =
          DesktopShellLayoutActions(
              updateLayout = { layout = it },
              saveLayout = layoutStore::save,
          ),
      projectActions =
          DesktopShellProjectActions(
              importProject = ::importProject,
              reindexProject = ::reindexProject,
              reconnect = presenter::refreshConnection,
              retryRestore = ::retryRestore,
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
              sourceLineSelected =
                  sourceLineSelectionAction(presenter) { composerRequested = false },
              validateDraft = presenter::validateEditableDraft,
              runDraftChecks = presenter::runDraftChecks,
              generate = ::sendComposerMessage,
              createDeclaration = { requestCreateDeclaration(DeclarationCreationKind.Function) },
              openFile = ::requestFileNavigation,
              cancelGeneration = presenter::cancelGeneration,
              dismissContext = presenter::closeContextInspection,
              retryContext = presenter::retryContextInspection,
              cancelContext = presenter::cancelContextInspection,
          ),
      analysisActions =
          DesktopShellAnalysisActions(
              refreshStatus = presenter::refreshAnalysis,
              refreshAnalysisSelection = presenter::refreshAnalysisSelection,
              saveAnalysisSelection = presenter::saveAnalysisSelection,
              retryResults = presenter::loadAnalysisResults,
              startAnalysis = { limits, retry ->
                presenter.previewAnalysis(limits = limits, retryStaleFailed = retry)
              },
              pauseAnalysis = presenter::pauseAnalysis,
              resumeAnalysis = presenter::resumeAnalysis,
              cancelAnalysis = presenter::cancelAnalysis,
              startScan = presenter::runVerifiedScan,
              cancelScan = presenter::cancelVerifiedScan,
              refreshScanStatus = presenter::refreshVerifiedScanStatus,
              openPerformanceSource = { result, selectionCurrent ->
                routePerformanceSourceRequest(
                    presenter,
                    result,
                    chatMessage,
                    advancedConstraints,
                    selectionCurrent,
                    { chatMessage to advancedConstraints },
                    ::clearComposerInput) {
                      pendingDraftDiscard = it
                    }
              },
              preparePerformanceFinding = { result, selectionCurrent ->
                routePerformancePreparationRequest(
                    presenter,
                    result,
                    chatMessage,
                    advancedConstraints,
                    selectionCurrent,
                    { chatMessage to advancedConstraints },
                    { advancedConstraints = TextFieldValue() }) {
                      pendingDraftDiscard = it
                    }
              },
              loadGoBenchmarks = presenter::loadGoBenchmarks,
              selectGoBenchmark = presenter::selectGoBenchmark,
              compareSelectedGoBenchmark = presenter::compareSelectedGoBenchmark,
              prepareSecurityFinding = { result, selectionCurrent ->
                routeSecurityPreparationRequest(
                    presenter,
                    result,
                    chatMessage,
                    advancedConstraints,
                    selectionCurrent,
                    { chatMessage to advancedConstraints },
                    ::clearComposerInput) {
                      pendingDraftDiscard = it
                    }
              },
              openSecuritySource = { result, selectionCurrent ->
                routeSecuritySourceRequest(
                    presenter,
                    result,
                    chatMessage,
                    advancedConstraints,
                    selectionCurrent,
                    { chatMessage to advancedConstraints },
                    ::clearComposerInput) {
                      pendingDraftDiscard = it
                    }
              },
          ),
      findingActions = findingActions,
      paletteActions =
          DesktopShellPaletteActions(
              updateQuery = {
                paletteQuery = it
                paletteBlockedReason = null
              },
              dismiss = {
                showPalette = false
                paletteBlockedReason = null
              },
              open = ::openPalette,
              switchMode = ::switchPaletteMode,
              selectFile = {
                showPalette = false
                requestFileNavigation(it)
              },
              selectSymbol = {
                showPalette = false
                inspectPaletteSymbol(presenter, it)
                composerRequested = false
              },
              selectAction = ::selectPaletteAction,
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
  pendingSwitch?.let { pending ->
    ProjectSwitchReviewDialog(
        pending,
        analyzeModel,
        workflow.providerConfirmed(ModelScope.Analyze),
        terminalState,
        SwitchCleanupFeedback(switchCleanupError, switchAdmission.cleanupOutstanding),
        onCancel = {
          if (pending.stage == SwitchReviewStage.Committed) {
            switchAdmission.finish(pending.requestId)
            updatePendingSwitch()
          } else dismissSwitch(pending.requestId)
        },
        onDraftApproved = {
          switchAdmission.approveDraft(
              pending.requestId, ProjectSwitchContext(presenter.snapshot.value))
          updatePendingSwitch()
        },
        onProviderConfirmed = { presenter.setProviderConfirmation(ModelScope.Analyze, it) },
        onProviderApproved = {
          switchAdmission.approveProvider(
              pending.requestId, ProjectSwitchContext(presenter.snapshot.value))
          updatePendingSwitch()
        },
        onReviewApproved = {
          switchAdmission.approveReview(
              pending.requestId, ProjectSwitchContext(presenter.snapshot.value))
          updatePendingSwitch()
        },
        onCommit = { commitSwitch(pending.requestId) },
    )
  }
  DesktopAnalysisAdmissionOverlay(appState.analysisRun, presenter)
  pendingDraftDiscard?.let { pending ->
    DraftDiscardDialog(pending.currentDraft, pending.nextLabel, ::discardDraftAndContinue) {
      pendingDraftDiscard = null
      if (pending is PendingDraftDiscard.Create) {
        focusAssistantControl(composerFocusAfterDiscard(chatMode))
      }
    }
  }
}

@Composable
private fun ComposerFocusEffect(
    target: ComposerFocusTarget?,
    draftVisible: Boolean,
    dialogOpen: Boolean,
    name: FocusRequester,
    chat: FocusRequester,
    draft: FocusRequester,
    onFocused: (ComposerFocusTarget) -> Unit,
) {
  LaunchedEffect(target, draftVisible, dialogOpen) {
    if (dialogOpen) return@LaunchedEffect
    when (target) {
      ComposerFocusTarget.Name -> name.requestFocus()
      ComposerFocusTarget.Chat -> chat.requestFocus()
      ComposerFocusTarget.Draft -> {
        if (!draftVisible) return@LaunchedEffect
        draft.requestFocus()
      }
      null -> return@LaunchedEffect
    }
    onFocused(target)
  }
}

private fun composerFocusAfterDiscard(mode: ChatEditMode): ComposerFocusTarget =
    if (mode == ChatEditMode.CreateSymbol) ComposerFocusTarget.Name else ComposerFocusTarget.Chat

private fun creationKindForCommand(action: String): DeclarationCreationKind? =
    when (action) {
      "create_function" -> DeclarationCreationKind.Function
      "create_type" -> DeclarationCreationKind.Type
      else -> null
    }

internal fun routeContextRefactor(
    presenter: DesktopWorkflowPresenter,
    symbol: SymbolInspectorSymbolState,
    prepare: (DirectEditRequest) -> Unit,
    confirmDiscard: (PendingDraftDiscard.Replace) -> Unit,
) {
  val state = presenter.snapshot.value.state
  val request =
      directEditRequest(
          state.selectedFile, state.symbols, symbol.symbol, currentEditIdentity(state)) ?: return
  val currentDraft = request.currentDraft
  if (request.requiresDraftDiscard && currentDraft != null)
      confirmDiscard(PendingDraftDiscard.Replace(request, currentDraft))
  else prepare(request)
}

private fun continueAfterDraftDiscard(
    pending: PendingDraftDiscard?,
    presenter: DesktopWorkflowPresenter,
    message: TextFieldValue,
    constraints: TextFieldValue,
    clearComposer: () -> Unit,
    clearConstraints: () -> Unit,
    currentInput: () -> Pair<TextFieldValue, TextFieldValue>,
    replace: (DirectEditRequest) -> Unit,
    create: (DeclarationCreationKind) -> Unit,
    changeKind: (DeclarationCreationKind) -> Unit,
    currentCreation: () -> Triple<ChatEditMode, DeclarationCreationKind, String>,
) {
  when (pending) {
    is PendingDraftDiscard.Replace -> {
      presenter.discardDraft()
      replace(pending.request)
    }
    is PendingDraftDiscard.Create ->
        confirmCreationDiscard(
            pending, presenter, currentInput, currentCreation, create, changeKind)
    is PendingDraftDiscard.FileNavigation ->
        confirmFileNavigationDiscard(presenter, pending, currentInput, clearComposer)
    is PendingDraftDiscard.PerformancePreparation ->
        confirmPerformancePreparationDiscard(
            presenter,
            pending,
            message,
            constraints,
            currentInput,
            clearConstraints = clearConstraints)
    is PendingDraftDiscard.PerformanceSource ->
        confirmPerformanceSourceDiscard(
            presenter, pending, message, constraints, currentInput, clearComposer)
    is PendingDraftDiscard.SecuritySource ->
        confirmSecuritySourceDiscard(
            presenter, pending, message, constraints, currentInput, clearComposer)
    is PendingDraftDiscard.SecurityPreparation ->
        confirmSecurityPreparationDiscard(
            presenter, pending, message, constraints, currentInput, clearComposer)
    is PendingDraftDiscard.Finding ->
        confirmFindingDiscard(presenter, pending, message, constraints, clearComposer)
    null -> Unit
  }
}

internal fun confirmCreationDiscard(
    pending: PendingDraftDiscard.Create,
    presenter: DesktopWorkflowPresenter,
    currentInput: () -> Pair<TextFieldValue, TextFieldValue>,
    currentCreation: () -> Triple<ChatEditMode, DeclarationCreationKind, String>,
    create: (DeclarationCreationKind) -> Unit,
    changeKind: (DeclarationCreationKind) -> Unit,
) {
  val workflow = presenter.snapshot.value
  val state = workflow.state
  if (state.project != pending.project ||
      state.index != pending.index ||
      state.selectedFile != pending.file ||
      state.review != pending.review ||
      state.chat != pending.chat ||
      currentEditIdentity(state) != pending.currentDraft ||
      state.review.draft?.let { it.id to it.revision } !=
          pending.review.draft?.let { it.id to it.revision } ||
      currentCreation() != Triple(pending.priorMode, pending.priorKind, pending.name) ||
      (pending.intent == PendingDraftDiscard.CreationIntent.ChangeKind &&
          pending.priorKind == pending.kind) ||
      currentInput() != (pending.message to pending.constraints) ||
      declarationCreationBlockedReason(state.selectedFile, workflow.creationInProgress) != null)
      return
  presenter.discardDraft()
  when (pending.intent) {
    PendingDraftDiscard.CreationIntent.Fresh -> create(pending.kind)
    PendingDraftDiscard.CreationIntent.ChangeKind -> changeKind(pending.kind)
  }
}

internal fun sourceLineSelectionAction(
    presenter: DesktopWorkflowPresenter,
    onInspected: () -> Unit,
): (SourceLineSelection) -> Unit = { selection ->
  presenter.dispatch(DesktopEvent.SourceLineSelected(selection))
  onInspected()
}

internal fun inspectPaletteSymbol(presenter: DesktopWorkflowPresenter, symbol: SymbolInfo) {
  presenter.dispatch(DesktopEvent.SymbolSelected(symbol))
  presenter.dispatch(DesktopEvent.WorkspaceSelected(Workspace.Editor))
}

internal fun routeFileNavigationRequest(
    presenter: DesktopWorkflowPresenter,
    path: String,
    message: TextFieldValue,
    constraints: TextFieldValue,
    currentInput: () -> Pair<TextFieldValue, TextFieldValue>,
    clearComposer: () -> Unit,
    pending: (PendingDraftDiscard.FileNavigation) -> Unit,
) {
  val intent =
      presenter.fileNavigationIntent(
          path,
          composerHasWork = message.text.isNotEmpty() || constraints.text.isNotEmpty(),
          composerCurrent = { currentInput() == (message to constraints) })
  if (intent == null) {
    presenter.dispatch(
        DesktopEvent.Failed("This file no longer points to an indexed file in the active project."))
    return
  }
  val navigation =
      PendingDraftDiscard.FileNavigation(
          intent, currentEditIdentity(presenter.snapshot.value.state), message, constraints)
  if (intent.requiresDiscard) pending(navigation)
  else confirmFileNavigationDiscard(presenter, navigation, currentInput, clearComposer)
}

internal fun confirmFileNavigationDiscard(
    presenter: DesktopWorkflowPresenter,
    pending: PendingDraftDiscard.FileNavigation,
    currentInput: () -> Pair<TextFieldValue, TextFieldValue>,
    clearComposer: () -> Unit,
) {
  presenter.confirmFileNavigationIntent(pending.intent) {
    if (currentInput() == (pending.chatMessage to pending.constraints)) clearComposer()
  }
}

internal fun routePerformancePreparationRequest(
    presenter: DesktopWorkflowPresenter,
    result: PerformanceResult,
    message: TextFieldValue,
    constraints: TextFieldValue,
    selectionCurrent: () -> Boolean,
    currentInput: () -> Pair<TextFieldValue, TextFieldValue>,
    clearConstraints: () -> Unit,
    pending: (PendingDraftDiscard.PerformancePreparation) -> Unit,
) {
  val intent = presenter.performancePreparationIntent(result, selectionCurrent)
  if (intent != null &&
      (intent.draft.hasWork || message.text.isNotEmpty() || constraints.text.isNotEmpty()))
      pending(
          PendingDraftDiscard.PerformancePreparation(
              intent, currentEditIdentity(presenter.snapshot.value.state), message, constraints))
  else
      presenter.preparePerformanceFinding(
          result,
          selectionCurrent,
          { currentInput() == (message to constraints) },
          clearConstraints)
}

internal fun confirmPerformancePreparationDiscard(
    presenter: DesktopWorkflowPresenter,
    pending: PendingDraftDiscard.PerformancePreparation,
    message: TextFieldValue,
    constraints: TextFieldValue,
    currentInput: () -> Pair<TextFieldValue, TextFieldValue>,
    clearConstraints: () -> Unit,
) {
  if (pending.chatMessage != message || pending.constraints != constraints) {
    presenter.dispatch(DesktopEvent.Failed("Assistant input changed. Choose Prepare fix again."))
    return
  }
  presenter.confirmPerformancePreparationIntent(
      pending.intent,
      { currentInput() == (pending.chatMessage to pending.constraints) },
      clearConstraints)
}

internal fun routePerformanceSourceRequest(
    presenter: DesktopWorkflowPresenter,
    result: PerformanceResult,
    message: TextFieldValue,
    constraints: TextFieldValue,
    selectionCurrent: () -> Boolean,
    currentInput: () -> Pair<TextFieldValue, TextFieldValue>,
    clearComposer: () -> Unit,
    pending: (PendingDraftDiscard.PerformanceSource) -> Unit,
) {
  val intent = presenter.performanceSourceIntent(result, selectionCurrent)
  if (intent != null &&
      intent.selectedFile?.path != intent.target.path &&
      (intent.draft.hasWork || message.text.isNotEmpty() || constraints.text.isNotEmpty()))
      pending(
          PendingDraftDiscard.PerformanceSource(
              intent, currentEditIdentity(presenter.snapshot.value.state), message, constraints))
  else
      presenter.openPerformanceFinding(result, selectionCurrent) {
        if (currentInput() == (message to constraints)) clearComposer()
      }
}

internal fun confirmPerformanceSourceDiscard(
    presenter: DesktopWorkflowPresenter,
    pending: PendingDraftDiscard.PerformanceSource,
    message: TextFieldValue,
    constraints: TextFieldValue,
    currentInput: () -> Pair<TextFieldValue, TextFieldValue>,
    clearComposer: () -> Unit,
) {
  if (pending.chatMessage != message || pending.constraints != constraints) {
    presenter.dispatch(DesktopEvent.Failed("Assistant input changed. Choose Open source again."))
    return
  }
  presenter.confirmPerformanceSourceIntent(pending.intent) {
    if (currentInput() == (pending.chatMessage to pending.constraints)) clearComposer()
  }
}

internal fun routeSecurityPreparationRequest(
    presenter: DesktopWorkflowPresenter,
    result: SecurityResult,
    message: TextFieldValue,
    constraints: TextFieldValue,
    selectionCurrent: () -> Boolean,
    currentInput: () -> Pair<TextFieldValue, TextFieldValue>,
    clearComposer: () -> Unit,
    pending: (PendingDraftDiscard.SecurityPreparation) -> Unit,
) {
  val intent = presenter.securityPreparationIntent(result, selectionCurrent)
  if (intent != null &&
      (intent.draft.hasWork || message.text.isNotEmpty() || constraints.text.isNotEmpty()))
      pending(
          PendingDraftDiscard.SecurityPreparation(
              intent, currentEditIdentity(presenter.snapshot.value.state), message, constraints))
  else
      presenter.prepareSecurityFinding(
          result, selectionCurrent, { currentInput() == (message to constraints) }) {
            if (currentInput() == (message to constraints)) clearComposer()
          }
}

internal fun confirmSecurityPreparationDiscard(
    presenter: DesktopWorkflowPresenter,
    pending: PendingDraftDiscard.SecurityPreparation,
    message: TextFieldValue,
    constraints: TextFieldValue,
    currentInput: () -> Pair<TextFieldValue, TextFieldValue>,
    clearComposer: () -> Unit,
) {
  if (pending.chatMessage != message || pending.constraints != constraints) {
    presenter.dispatch(DesktopEvent.Failed("Assistant input changed. Choose Prepare fix again."))
    return
  }
  presenter.confirmSecurityPreparationIntent(
      pending.intent, { currentInput() == (pending.chatMessage to pending.constraints) }) {
        if (currentInput() == (pending.chatMessage to pending.constraints)) clearComposer()
      }
}

internal fun routeSecuritySourceRequest(
    presenter: DesktopWorkflowPresenter,
    result: SecurityResult,
    message: TextFieldValue,
    constraints: TextFieldValue,
    selectionCurrent: () -> Boolean,
    currentInput: () -> Pair<TextFieldValue, TextFieldValue>,
    clearComposer: () -> Unit,
    pending: (PendingDraftDiscard.SecuritySource) -> Unit,
) {
  val intent = presenter.securitySourceIntent(result, selectionCurrent)
  if (intent != null &&
      intent.selectedFile?.path != intent.target.path &&
      (intent.draft.hasWork || message.text.isNotEmpty() || constraints.text.isNotEmpty()))
      pending(
          PendingDraftDiscard.SecuritySource(
              intent, currentEditIdentity(presenter.snapshot.value.state), message, constraints))
  else
      presenter.openSecurityFinding(
          result, selectionCurrent, { currentInput() == (message to constraints) }) {
            if (currentInput() == (message to constraints)) clearComposer()
          }
}

internal fun confirmSecuritySourceDiscard(
    presenter: DesktopWorkflowPresenter,
    pending: PendingDraftDiscard.SecuritySource,
    message: TextFieldValue,
    constraints: TextFieldValue,
    currentInput: () -> Pair<TextFieldValue, TextFieldValue>,
    clearComposer: () -> Unit,
) {
  if (pending.chatMessage != message || pending.constraints != constraints) {
    presenter.dispatch(DesktopEvent.Failed("Assistant input changed. Choose Open source again."))
    return
  }
  presenter.confirmSecuritySourceIntent(
      pending.intent, { currentInput() == (pending.chatMessage to pending.constraints) }) {
        if (currentInput() == (pending.chatMessage to pending.constraints)) clearComposer()
      }
}

private fun routeFindingRequest(
    presenter: DesktopWorkflowPresenter,
    finding: UnifiedFinding,
    prepare: Boolean,
    message: TextFieldValue,
    constraints: TextFieldValue,
    pending: (PendingDraftDiscard.Finding) -> Unit,
) {
  val intent = presenter.findingIntent(finding, prepare)
  if (intent != null && findingRequiresDiscard(intent, message.text, constraints.text))
      pending(
          PendingDraftDiscard.Finding(
              intent, currentEditIdentity(presenter.snapshot.value.state), message, constraints))
  else if (prepare) presenter.prepareFinding(finding) else presenter.openFinding(finding)
}

private fun confirmFindingDiscard(
    presenter: DesktopWorkflowPresenter,
    pending: PendingDraftDiscard.Finding,
    message: TextFieldValue,
    constraints: TextFieldValue,
    clearComposer: () -> Unit,
) {
  if (pending.chatMessage != message || pending.constraints != constraints) {
    presenter.dispatch(
        DesktopEvent.Failed("Assistant input changed. Choose the finding action again."))
    return
  }
  if (presenter.confirmFindingIntent(pending.intent) && !pending.intent.prepare) clearComposer()
}

internal fun findingRequiresDiscard(
    intent: DesktopWorkflowPresenter.FindingIntent,
    message: String,
    constraints: String,
): Boolean =
    (intent.prepare || intent.selectedFile?.path != intent.target.path) &&
        (intent.draft.hasWork || message.isNotEmpty() || constraints.isNotEmpty())

private fun projectSwitchPending(chooserOpen: Boolean, pending: PendingProjectSwitch?): Boolean =
    chooserOpen || pending != null

private fun dispatchProjectAction(available: Boolean, action: () -> Unit) {
  if (available) action()
}

private fun admitProjectChooser(
    state: DesktopState,
    pending: Boolean,
    setChooserOpen: (Boolean) -> Unit,
    choose: (Boolean) -> File?,
    current: () -> DesktopWorkflowSnapshot,
    admission: ProjectSwitchAdmission,
    updatePending: () -> Unit,
) {
  if (!projectActionAvailability(
          state.project,
          state.projectState.openingAttempt,
          state.projectState.indexingAttempt,
          pending)
      .open)
      return
  setChooserOpen(true)
  try {
    chooseProjectDirectory({ choose(state.project != null) }) { path ->
      val latest = current()
      if (projectOpenAvailable(latest.state.projectState.openingAttempt)) {
        admission.choose(path, ProjectSwitchContext(latest))
        updatePending()
      }
    }
  } finally {
    setChooserOpen(false)
  }
}

private fun scheduleSwitchCleanupTimeout(onTimeout: () -> Unit): () -> Unit {
  val timer = Timer(15_000) { onTimeout() }
  timer.isRepeats = false
  timer.start()
  return timer::stop
}

/** All callbacks and the timeout resolve on Swing's event thread; late cleanup cannot import. */
internal fun commitProjectSwitch(
    requestId: Long,
    admission: ProjectSwitchAdmission,
    currentContext: () -> ProjectSwitchContext,
    openingAvailable: () -> Boolean,
    terminal: SwitchTerminalCleanup,
    discardDraft: () -> Unit,
    importProject: (String) -> Unit,
    updatePending: () -> Unit,
    updateError: (String?) -> Unit,
    post: (() -> Unit) -> Unit,
    scheduleTimeout: (() -> Unit) -> (() -> Unit),
) {
  if (!openingAvailable()) return
  val committed = admission.commit(requestId, currentContext())
  updatePending()
  if (committed == null) return
  updateError(null)
  post {
    if (admission.pending != committed) return@post
    if (!openingAvailable() || currentContext() != committed.context) {
      updateError(
          "The project, draft or Analyze destination changed before shell cleanup. The project has not been switched. Review the current state before trying again.")
      return@post
    }
    var resolved = false
    val stopTimeout = scheduleTimeout {
      if (!resolved && admission.pending == committed) {
        resolved = true
        updateError(
            "Shell cleanup did not finish in 15 seconds. The project has not been switched. Some tabs may already be closed; check the terminal before trying again.")
      }
    }
    try {
      // Terminal ownership is on Swing; even exited tabs must be removed before importing.
      admission.cleanupStarted(requestId)
      terminal.closeAll().whenComplete { closed, error ->
        post completion@{
          if (admission.pending != committed) return@completion
          admission.cleanupSettled(requestId)
          stopTimeout()
          if (resolved) {
            updateError(
                if (error != null ||
                    closed == null ||
                    closed.cleanupPending ||
                    closed.tabs.isNotEmpty() ||
                    terminal.state().let {
                      it.cleanupPending || it.tabs.isNotEmpty() || it.closingAll
                    })
                    "Shell cleanup returned after the timeout but is incomplete. The project has not been switched. Some tabs may still be closing; check the terminal before trying again."
                else
                    "Shell cleanup has now finished after the timeout. The project has not been switched. Some tabs may already be closed; review the terminal before trying again.")
            return@completion
          }
          resolved = true
          val current = terminal.state()
          when {
            error != null ||
                closed == null ||
                closed.cleanupPending ||
                closed.tabs.isNotEmpty() ||
                current.cleanupPending ||
                current.tabs.isNotEmpty() ||
                current.closingAll ->
                updateError(
                    "Shell cleanup is incomplete. The project has not been switched. Some tabs may already be closed: " +
                        (error?.message?.takeIf(String::isNotBlank)
                            ?: "Check the terminal before trying again."))
            !openingAvailable() || currentContext() != committed.context ->
                updateError(
                    "The project, draft or Analyze destination changed during shell cleanup. The project has not been switched. Review the current state before trying again; closed tabs cannot be restored.")
            else -> {
              if (committed.context.draft.hasWork) discardDraft()
              admission.finish(requestId)
              updatePending()
              importProject(committed.path)
            }
          }
        }
      }
    } catch (error: Exception) {
      if (admission.pending == committed) {
        admission.cleanupSettled(requestId)
        resolved = true
        stopTimeout()
        updateError(
            "Shell cleanup could not start. The project has not been switched: " +
                (error.message?.takeIf(String::isNotBlank)
                    ?: "Check the terminal before trying again."))
      }
    }
  }
}

private val DesktopWorkflowSnapshot.creationInProgress: Boolean
  get() = generating || draftValidationInProgress

internal fun routeCreationKindChange(
    workflow: DesktopWorkflowSnapshot,
    mode: ChatEditMode,
    currentKind: DeclarationCreationKind,
    requestedKind: DeclarationCreationKind,
    name: String,
    message: TextFieldValue,
    constraints: TextFieldValue,
    change: (DeclarationCreationKind) -> Unit,
    confirmDiscard: (PendingDraftDiscard.Create) -> Unit,
) {
  if (mode != ChatEditMode.CreateSymbol || currentKind == requestedKind) return
  val state = workflow.state
  if (declarationCreationBlockedReason(
      state.selectedFile,
      workflow.creationInProgress || state.review.editor?.status == DraftEditorStatus.Validating) !=
      null)
      return
  val draft = currentEditIdentity(state)?.takeIf { it.hasDraft }
  if (draft == null) change(requestedKind)
  else
      confirmDiscard(
          creationDiscard(
              workflow,
              draft,
              PendingDraftDiscard.CreationIntent.ChangeKind,
              requestedKind,
              currentKind,
              mode,
              name,
              message,
              constraints))
}

internal fun routeCreationRequest(
    workflow: DesktopWorkflowSnapshot,
    kind: DeclarationCreationKind,
    start: (DeclarationCreationKind) -> Unit,
    confirmDiscard: (PendingDraftDiscard.Create) -> Unit,
    message: TextFieldValue = TextFieldValue(),
    constraints: TextFieldValue = TextFieldValue(),
    currentKind: DeclarationCreationKind = DeclarationCreationKind.Function,
    name: String = "",
    mode: ChatEditMode = ChatEditMode.CreateSymbol,
): String? {
  val state = workflow.state
  val blockedReason =
      declarationCreationBlockedReason(
          state.selectedFile,
          workflow.creationInProgress ||
              state.review.editor?.status == DraftEditorStatus.Validating)
  if (blockedReason != null) return blockedReason
  val currentDraft = currentEditIdentity(state)?.takeIf { it.hasDraft }
  if (currentDraft == null) start(kind)
  else
      confirmDiscard(
          creationDiscard(
              workflow,
              currentDraft,
              PendingDraftDiscard.CreationIntent.Fresh,
              kind,
              currentKind,
              mode,
              name,
              message,
              constraints))
  return null
}

private fun creationDiscard(
    workflow: DesktopWorkflowSnapshot,
    draft: CurrentEditIdentity,
    intent: PendingDraftDiscard.CreationIntent,
    kind: DeclarationCreationKind,
    priorKind: DeclarationCreationKind,
    priorMode: ChatEditMode,
    name: String,
    message: TextFieldValue,
    constraints: TextFieldValue,
): PendingDraftDiscard.Create {
  val state = workflow.state
  return PendingDraftDiscard.Create(
      draft,
      intent,
      kind,
      priorKind,
      priorMode,
      name,
      state.project,
      state.index,
      requireNotNull(state.selectedFile),
      state.review,
      state.chat,
      message,
      constraints)
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
  val state = workflow.state
  if (assistantComposerBlockedReason(
      mode,
      state.selectedFile,
      validateChatTarget(state.selection, mode, name),
      behavior,
      workflow.generating,
      workflow.draftValidationInProgress ||
          state.review.editor?.status == DraftEditorStatus.Validating,
      workflow.model(ModelScope.Function),
      workflow.providerConfirmed(ModelScope.Function)) != null)
      return
  presenter.sendChatMessage(
      mode,
      name,
      behavior,
      constraints,
      creationKind = kind.takeIf { mode == ChatEditMode.CreateSymbol })
}

internal fun reviewToolWindowState(state: DesktopState) =
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
        checksRunning = state.review.checkAttempt?.status == ValidationAttemptStatus.Running,
        checkAttempt = state.review.checkAttempt,
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
internal fun ProjectSwitchReviewDialog(
    pending: PendingProjectSwitch,
    model: ScopedModel,
    confirmed: Boolean,
    terminal: TerminalWorkspaceState,
    cleanupFeedback: SwitchCleanupFeedback,
    onCancel: () -> Unit,
    onDraftApproved: () -> Unit,
    onProviderConfirmed: (Boolean) -> Unit,
    onProviderApproved: () -> Unit,
    onReviewApproved: () -> Unit,
    onCommit: () -> Unit,
) {
  val committed = pending.stage == SwitchReviewStage.Committed
  val cleanupError = cleanupFeedback.error
  val cleanupOutstanding = cleanupFeedback.outstanding
  IdeDialog(
      onDismissRequest = {
        if (!committed || (cleanupError != null && !cleanupOutstanding)) onCancel()
      },
      title = {
        Text(
            when (pending.stage) {
              SwitchReviewStage.Draft -> "Review draft before switching"
              SwitchReviewStage.Provider -> "Confirm project analysis destination"
              SwitchReviewStage.Review -> "Review changed project switch"
              SwitchReviewStage.Final -> "Switch project?"
              SwitchReviewStage.Committed ->
                  if (cleanupOutstanding) "Closing project shells…" else "Project switch stopped"
            })
      },
      content = {
        ProjectSwitchReviewBody(
            pending, model, confirmed, terminal, cleanupFeedback, onProviderConfirmed)
      },
      actions = {
        ProjectSwitchReviewActions(
            pending,
            confirmed,
            terminal,
            cleanupFeedback,
            onCancel,
            onDraftApproved,
            onProviderApproved,
            onReviewApproved,
            onCommit)
      },
  )
}

@Composable
internal fun ProjectSwitchReviewBody(
    pending: PendingProjectSwitch,
    model: ScopedModel,
    confirmed: Boolean,
    terminal: TerminalWorkspaceState,
    cleanupFeedback: SwitchCleanupFeedback,
    onProviderConfirmed: (Boolean) -> Unit,
) {
  val committed = pending.stage == SwitchReviewStage.Committed
  val cleanupError = cleanupFeedback.error
  val cleanupOutstanding = cleanupFeedback.outstanding
  SelectionContainer {
    Column {
      Text("Current project: ${pending.context.project?.path ?: "None open"}")
      Text("Requested project: ${pending.path}")
      if (pending.context.draft.hasWork) {
        val target =
            pending.context.draft.draft?.targetPath?.takeIf(String::isNotBlank)
                ?: pending.context.draft.session?.openPath?.takeIf(String::isNotBlank)
        Text(
            "If you switch, the in-memory conversation, editable draft and focused checks${target?.let { " for $it" } ?: ""} will be discarded. Continuing this review does not discard them yet.")
      }
      if (terminal.tabs.isNotEmpty()) {
        Text(
            "Switching will close all ${terminal.tabs.size} project shell tabs and their child processes, including hidden and exited tabs.")
        terminal.tabs.forEach { Text(it.title) }
      }
      if (pending.stage == SwitchReviewStage.Provider) {
        Text("Import may send the selected project's analysis context to this provider.")
      }
      if (pending.stage == SwitchReviewStage.Final && !model.remoteProvider)
          Text("This Analyze destination is local; no remote confirmation is required.")
      if (committed) {
        Text(
            "Switch commitment has begun. Any closed tabs cannot be restored by dismissing this review.")
        cleanupError?.let { DiagnosticText(it, color = Error) }
        if (cleanupOutstanding)
            Text(
                "Waiting for shell cleanup to finish. Another switch cannot start while tabs may still close.")
      }
    }
  }
  if (pending.stage == SwitchReviewStage.Provider)
      RemoteProviderConfirmation(ModelScope.Analyze, model, confirmed, onProviderConfirmed)
}

@Composable
internal fun ProjectSwitchReviewActions(
    pending: PendingProjectSwitch,
    confirmed: Boolean,
    terminal: TerminalWorkspaceState,
    cleanupFeedback: SwitchCleanupFeedback,
    onCancel: () -> Unit,
    onDraftApproved: () -> Unit,
    onProviderApproved: () -> Unit,
    onReviewApproved: () -> Unit,
    onCommit: () -> Unit,
) {
  val committed = pending.stage == SwitchReviewStage.Committed
  val cleanupError = cleanupFeedback.error
  val cleanupOutstanding = cleanupFeedback.outstanding
  if (committed && cleanupError != null && !cleanupOutstanding)
      MiniOrcaButton(onClick = onCancel, tone = ActionTone.Neutral) { Text("Close review") }
  if (!committed) {
    MiniOrcaButton(onClick = onCancel, tone = ActionTone.Neutral) { Text("Cancel switch") }
    when (pending.stage) {
      SwitchReviewStage.Draft ->
          MiniOrcaButton(onClick = onDraftApproved, tone = ActionTone.Destructive) {
            Text("Approve draft discard for switch")
          }
      SwitchReviewStage.Provider ->
          MiniOrcaButton(onClick = onProviderApproved, enabled = confirmed) {
            Text("Continue with provider")
          }
      SwitchReviewStage.Review ->
          MiniOrcaButton(onClick = onReviewApproved) { Text("Continue to switch review") }
      SwitchReviewStage.Final ->
          MiniOrcaButton(onClick = onCommit, tone = ActionTone.Destructive) {
            Text(if (terminal.tabs.isEmpty()) "Switch project" else "Close shells and switch")
          }
      SwitchReviewStage.Committed -> Unit
    }
  }
}

@Composable
internal fun ProjectImportConfirmationDialog(
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
internal fun DraftDiscardDialog(
    currentDraft: CurrentEditIdentity?,
    nextLabel: String,
    onDiscard: () -> Unit,
    onCancel: () -> Unit,
) {
  IdeDialog(
      onDismissRequest = onCancel,
      title = {
        Text(if (currentDraft == null) "Discard current work?" else "Discard current draft?")
      },
      content = {
        Text(
            if (currentDraft == null)
                "Discard the current conversation or Assistant input before you $nextLabel?"
            else
                "Discard the draft for ${currentDraft.targetSymbol} in ${currentDraft.targetPath} before you $nextLabel? This clears the in-memory conversation, draft, and focused checks.")
      },
      actions = {
        MiniOrcaButton(onClick = onCancel, tone = ActionTone.Neutral) {
          Text(if (currentDraft == null) "Keep work" else "Keep draft")
        }
        MiniOrcaButton(onClick = onDiscard, tone = ActionTone.Destructive) {
          Text(if (currentDraft == null) "Discard work" else "Discard draft")
        }
      },
  )
}

internal fun chooseProjectDirectory(choose: () -> File?, onSelected: (String) -> Unit) {
  val directory = choose() ?: return
  onSelected(directory.absolutePath)
}

private fun chooseDirectory(switching: Boolean): File? {
  val chooser =
      JFileChooser().apply {
        dialogTitle = if (switching) "Switch project…" else "Open project…"
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
