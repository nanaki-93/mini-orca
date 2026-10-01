package io.miniorca.desktop

import androidx.compose.foundation.background
import androidx.compose.foundation.focusable
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.compositionLocalOf
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.input.key.KeyEventType
import androidx.compose.ui.input.key.key
import androidx.compose.ui.input.key.onPreviewKeyEvent
import androidx.compose.ui.input.key.type
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

@Composable
internal fun RightToolWindowContainer(
    activeToolWindow: RightToolWindow,
    onSelect: (RightToolWindow) -> Unit,
    content: @Composable (RightToolWindow, Modifier) -> Unit,
    badges: Map<RightToolWindow, RightToolWindowBadge> = emptyMap(),
    modifier: Modifier = Modifier,
) {
  var focusedToolWindow by remember(activeToolWindow) { mutableStateOf(activeToolWindow) }
  var tabGroupHasFocus by remember { mutableStateOf(false) }
  Column(modifier.fillMaxSize()) {
    Row(
        Modifier.fillMaxWidth()
            .onFocusChanged { tabGroupHasFocus = it.hasFocus }
            .focusable()
            .onPreviewKeyEvent { event ->
              if (event.type != KeyEventType.KeyDown) return@onPreviewKeyEvent false
              val interaction =
                  tabGroupInteraction(
                      RightToolWindow.entries.toList(), focusedToolWindow, tabGroupKey(event.key))
                      ?: return@onPreviewKeyEvent false
              focusedToolWindow = interaction.focused
              interaction.activate?.let(onSelect)
              true
            }
            .semantics {
              contentDescription =
                  "Right tool windows. ${rightToolWindowLabel(activeToolWindow)} selected."
            }) {
          RightToolWindow.entries.forEach { toolWindow ->
            val selected = toolWindow == activeToolWindow
            val badge = badges[toolWindow]
            ChromeTab(
                onClick = { onSelect(toolWindow) },
                selected = selected,
                focusHighlight = tabGroupHasFocus && toolWindow == focusedToolWindow,
                modifier =
                    Modifier.weight(1f).semantics {
                      contentDescription =
                          rightToolWindowTabDescription(
                              toolWindow,
                              selected,
                              badge,
                              tabGroupHasFocus && toolWindow == focusedToolWindow)
                      this.selected = selected
                    },
            ) {
              Column {
                Text(
                    rightToolWindowLabel(toolWindow),
                    fontSize = 12.sp,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis)
                badge?.let {
                  Text(
                      it.label,
                      color = SelectionText,
                      fontSize = 11.sp,
                      maxLines = 1,
                      overflow = TextOverflow.Ellipsis)
                }
              }
            }
          }
        }
    content(activeToolWindow, Modifier.fillMaxWidth().weight(1f))
  }
}

internal fun rightToolWindowTabDescription(
    toolWindow: RightToolWindow,
    selected: Boolean,
    badge: RightToolWindowBadge? = null,
    focused: Boolean = false,
): String =
    "${rightToolWindowLabel(toolWindow)} tool window tab${badge?.let { ", ${it.label}" }.orEmpty()}, ${if (selected) "selected" else "not selected"}${if (focused) ", focused" else ""}"

/** The Editor owns file creation while its docked Context pane inspects that same file. */
internal val LocalContextCreationActionVisible = compositionLocalOf { true }

@Composable
internal fun ContextToolWindow(
    state: ContextToolWindowState,
    actions: ContextToolWindowActions,
    modifier: Modifier = Modifier,
) {
  val inspector = state.inspector
  if (inspector == null) {
    SystemStateMessage(
        title = if (state.fileReadError != null) "Could not open file" else "No file selected",
        message =
            state.fileReadError?.let {
              "Reading local file data failed. ${it.ifBlank { "No details available." }} Select a file in Files to try again."
            } ?: "Select a file in Files to inspect its declarations.",
        accent = if (state.fileReadError != null) Error else SecondaryText,
        modifier = modifier,
        action =
            actions.openFile?.let { open ->
              { MiniOrcaButton(onClick = open) { Text("Select a file") } }
            })
    return
  }
  // Local details belong to the loaded file and selected declaration.
  var activeTab by
      rememberSaveable(
          state.project?.projectId,
          state.project?.projectRevision,
          inspector.file.path,
          inspector.file.contentHash,
          inspector.selectedSymbol?.symbol) {
            mutableStateOf(ContextTab.Actions)
          }
  Column(
      modifier.background(Panel).semantics {
        contentDescription = contextToolWindowDescription(inspector)
      }) {
        Column(Modifier.fillMaxWidth().padding(8.dp)) {
          Text(
              inspector.selectedSymbol?.symbol?.name ?: inspector.file.name,
              color = PrimaryText,
              style = IdeTypography.body.copy(fontWeight = FontWeight.SemiBold),
              maxLines = 2,
              overflow = TextOverflow.Ellipsis)
          Text(
              inspector.selectedSymbol?.rangeLabel ?: inspector.file.language,
              color = FaintText,
              style = IdeTypography.compactBody)
        }
        if (inspector.selectedSymbol == null) ContextTabs(activeTab, { activeTab = it })
        IdeHorizontalSeparator()
        androidx.compose.runtime.key(
            state.project?.projectId,
            state.project?.projectRevision,
            activeTab,
            inspector.file.path,
            inspector.file.contentHash,
            inspector.selectedSymbol?.symbol) {
              Column(
                  Modifier.fillMaxWidth()
                      .weight(1f)
                      .verticalScroll(rememberScrollState())
                      .testTag("context-content")
                      .padding(8.dp)) {
                    if (inspector.selectedSymbol != null) {
                      ContextDeclaration(state, actions)
                    } else
                        when (activeTab) {
                          ContextTab.Actions -> ContextActions(state, actions)
                          ContextTab.Details -> ContextDetails(state)
                        }
                  }
            }
      }
}

private enum class ContextTab(val label: String, val tint: Color) {
  Actions("Actions", SelectionText),
  Details("Details", SecondaryText),
}

@Composable
private fun ContextTabs(active: ContextTab, onSelect: (ContextTab) -> Unit) {
  val focusRequesters = remember { ContextTab.entries.associateWith { FocusRequester() } }
  Row(Modifier.fillMaxWidth()) {
    ContextTab.entries.forEach { tab ->
      Box(Modifier.weight(1f)) {
        ChromeTab(
            onClick = { onSelect(tab) },
            selected = active == tab,
            accent = tab.tint,
            accessibleName = "${tab.label} context tab",
            modifier =
                Modifier.fillMaxWidth()
                    .semantics { selected = active == tab }
                    .focusRequester(focusRequesters.getValue(tab))
                    .onPreviewKeyEvent { event ->
                      if (event.type != KeyEventType.KeyDown) return@onPreviewKeyEvent false
                      val key = tabGroupKey(event.key)
                      if (key != TabGroupKey.Previous && key != TabGroupKey.Next)
                          return@onPreviewKeyEvent false
                      val next =
                          tabGroupInteraction(ContextTab.entries, tab, key)
                              ?: return@onPreviewKeyEvent false
                      focusRequesters.getValue(next.focused).requestFocus()
                      true
                    }) {
              Text(tab.label, color = tab.tint, style = IdeTypography.compactBody)
            }
      }
    }
  }
}

@Composable
private fun ContextDeclaration(state: ContextToolWindowState, actions: ContextToolWindowActions) {
  val inspector = requireNotNull(state.inspector)
  val symbol = requireNotNull(inspector.selectedSymbol)
  val actionPresentation = declarationActionPresentation(explanationForSelection(state))
  var detailsExpanded by
      rememberSaveable(
          state.project?.projectId,
          state.project?.projectRevision,
          inspector.file.path,
          inspector.file.contentHash,
          symbol.symbol) {
            mutableStateOf(false)
          }
  SelectionContainer {
    Column(Modifier.fillMaxWidth()) {
      Text(inspector.file.path, color = PrimaryText, style = IdeTypography.compactBody)
      Text(
          "${symbol.rangeLabel} · ${inspector.file.language}",
          color = SecondaryText,
          style = IdeTypography.compactBody)
      Text(symbol.confidenceLabel, color = SecondaryText, style = IdeTypography.compactBody)
      Text(
          inspector.sourceIndexCorrespondence.label,
          color = SecondaryText,
          style = IdeTypography.compactBody)
    }
  }
  ContextSection(
      "Declaration details",
      DesktopIcon.Editor,
      detailsExpanded,
      { detailsExpanded = !detailsExpanded }) {
        SelectionContainer {
          Text(
              symbol.signature.ifBlank { "Signature unavailable" },
              color = PrimaryText,
              fontFamily = FontFamily.Monospace,
              style = IdeTypography.compactBody)
        }
      }
  Spacer(Modifier.height(8.dp))
  val visibleExplanation = explanationForSelection(state)
  DeclarationExplanationDetails(visibleExplanation)
  if (visibleExplanation.status != DeclarationExplanationStatus.Current &&
      state.fileAnalysis?.path == inspector.file.path &&
      state.fileAnalysis.symbolExplanations[symbol.symbol.name] == symbol.explanation &&
      !symbol.explanation.isNullOrBlank()) {
    Spacer(Modifier.height(8.dp))
    IdeLabelBadge("Saved file analysis · ${inspector.analysisStatus.label}", SecondaryText)
    Text(
        "${inspector.sourceIndexCorrespondence.label} · not an on-demand explanation",
        color = SecondaryText,
        style = IdeTypography.compactBody)
    ModelResultContent(symbol.explanation)
  }
  Spacer(Modifier.height(8.dp))
  ExplanationAction(state, actions, actionPresentation.explanationTone)
  Spacer(Modifier.height(8.dp))
  if (symbol.editEligibility.eligible) {
    MiniOrcaButton(
        onClick = { actions.editSelected(symbol) },
        tone = actionPresentation.refactorTone,
        modifier = Modifier.fillMaxWidth()) {
          DesktopLineIcon(
              DesktopIcon.Editor, "Refactor declaration", iconSize = 16.dp, tint = OnActionFill)
          Spacer(Modifier.width(8.dp))
          Text("Refactor", style = IdeTypography.action)
        }
  } else {
    Text(symbol.editEligibility.blockedReason, color = Warning, style = IdeTypography.compactBody)
  }
  contextStateBadge(inspector)?.let {
    Text(
        it,
        color = SelectionText,
        style = IdeTypography.compactBody,
        modifier = Modifier.padding(top = 8.dp))
  }
  Spacer(Modifier.height(8.dp))
  ContextReferences(state)
  ContextDetails(state, declarationSelected = true)
}

@Composable
private fun ContextReferences(state: ContextToolWindowState) {
  val inspector = requireNotNull(state.inspector)
  val selected = requireNotNull(inspector.selectedSymbol).symbol.name
  val impact = state.impact
  val matches =
      impact != null &&
          impact.targetPath == inspector.file.path &&
          (impact.targetSymbol.isBlank() || impact.targetSymbol == selected)
  val status =
      when {
        impact == null -> "Reference preview unavailable."
        !matches -> "Reference preview does not match the selected file and declaration."
        impact.references.isEmpty() ->
            "File-scoped reference preview has no indexed references; runtime callers are unknown."
        else ->
            "File-scoped reference preview · advisory indexed relationships, not runtime callers."
      }
  var expanded by remember { mutableStateOf(false) }
  SelectionContainer { Text(status, color = SecondaryText, style = IdeTypography.compactBody) }
  ContextSection("References", DesktopIcon.Branch, expanded, { expanded = !expanded }) {
    if (matches) {
      SelectionContainer {
        Column(Modifier.fillMaxWidth()) {
          impact.references.forEach { reference ->
            Text(reference.path, color = PrimaryText, style = IdeTypography.compactBody)
            if (reference.symbol.isNotBlank())
                Text(reference.symbol, color = PrimaryText, style = IdeTypography.compactBody)
            Text(
                "Confidence · ${reference.confidence}",
                color = SecondaryText,
                style = IdeTypography.compactBody)
            Text(reference.reason, color = SecondaryText, style = IdeTypography.compactBody)
            Spacer(Modifier.height(8.dp))
          }
        }
      }
    }
  }
}

@Composable
private fun ContextActions(state: ContextToolWindowState, actions: ContextToolWindowActions) {
  val inspector = requireNotNull(state.inspector)
  if (LocalContextCreationActionVisible.current) {
    ContextCreationAction(state, actions)
    Spacer(Modifier.height(12.dp))
  }
  IdePaneHeader(
      title = "File analysis",
      stateLabel = inspector.analysisStatus.label,
      stateTint =
          when (inspector.analysisStatus) {
            InspectorAnalysisStatus.Failed -> Error
            InspectorAnalysisStatus.Stale -> Warning
            InspectorAnalysisStatus.Running -> Information
            InspectorAnalysisStatus.Fresh -> Success
            InspectorAnalysisStatus.Missing -> SecondaryText
          })
  ContextProjectAnalysisActions(state, actions)
  Text(
      inspector.selectionPrompt,
      color = SecondaryText,
      style = IdeTypography.compactBody,
      modifier = Modifier.padding(top = 8.dp))
}

@Composable
internal fun ContextCreationAction(
    state: ContextToolWindowState,
    actions: ContextToolWindowActions
) {
  val file = state.inspector?.file ?: return
  actions.createDeclaration?.let { create ->
    val reason = declarationCreationBlockedReason(file, state.creationInProgress)
    NewFunctionButton(file.path, reason, create, Modifier.fillMaxWidth())
    reason?.let {
      Text(
          it,
          color = SecondaryText,
          style = IdeTypography.compactBody,
          modifier = Modifier.padding(top = 4.dp))
    }
  }
}

@Composable
private fun ExplanationAction(
    state: ContextToolWindowState,
    actions: ContextToolWindowActions,
    tone: ActionTone,
) {
  val symbol = requireNotNull(state.inspector?.selectedSymbol)
  if (!symbol.editEligibility.eligible) {
    return
  }
  val explanation = explanationForSelection(state)
  val loading = explanation.status == DeclarationExplanationStatus.Loading
  SelectionContainer {
    Text(
        modelDestinationLabel(ModelScope.Function, state.functionModel),
        color = if (state.functionModel.remoteProvider) Warning else SecondaryText,
        fontSize = 11.sp,
        modifier = Modifier.padding(top = MiniOrcaSpacing.standard))
  }
  if (state.functionModel.remoteProvider) {
    IdeCheckbox(
        checked = state.functionRemoteProviderConfirmed,
        onCheckedChange = actions.confirmFunctionRemoteProvider,
        accessibleName = "Confirm remote destination",
        modifier = Modifier.padding(top = MiniOrcaSpacing.compact),
        enabled = !loading,
        stateLabel = if (state.functionRemoteProviderConfirmed) "Confirmed" else "Not confirmed",
        label = "Confirm remote destination")
  }
  MiniOrcaButton(
      onClick = if (loading) actions.cancelExplanation else actions.explainSelected,
      enabled =
          loading || !state.functionModel.remoteProvider || state.functionRemoteProviderConfirmed,
      tone = tone,
      modifier = Modifier.fillMaxWidth()) {
        Text(explanationActionLabel(explanation), style = IdeTypography.action)
      }
}

@Composable
private fun ContextDetails(state: ContextToolWindowState, declarationSelected: Boolean = false) {
  val inspector = requireNotNull(state.inspector)
  var projectExpanded by remember { mutableStateOf(false) }
  var fileContextExpanded by remember { mutableStateOf(false) }
  if (declarationSelected) {
    var fileDetailsExpanded by remember { mutableStateOf(false) }
    ContextSection(
        "File details",
        DesktopIcon.Editor,
        fileDetailsExpanded,
        { fileDetailsExpanded = !fileDetailsExpanded }) {
          ContextFileDetails(inspector)
          state.fileAnalysis
              ?.takeIf { it.path == inspector.file.path }
              ?.engineeringInsight
              ?.let { insight ->
                val pieces = engineeringInsightPieces(insight)
                if (pieces.isNotEmpty()) {
                  Text(
                      engineeringInsightStateLabel(
                          "File", state.fileAnalysis.status.equals("stale", ignoreCase = true)),
                      color = SecondaryText,
                      style = IdeTypography.compactBody)
                  pieces.forEach { piece ->
                    Text(piece.label, color = ResultAccent, style = IdeTypography.resultLabel)
                    ModelResultContent(piece.content)
                  }
                }
              }
        }
  } else {
    ContextFileDetails(inspector)
    EngineeringInsightPanel(
        state.fileAnalysis?.engineeringInsight,
        stale = state.fileAnalysis?.status.equals("stale", ignoreCase = true),
        scopeLabel = "File")
  }
  Spacer(Modifier.height(8.dp))
  ContextSection(
      "Project context",
      DesktopIcon.Summary,
      projectExpanded,
      { projectExpanded = !projectExpanded }) {
        val project = projectSummaryPresentation(state.overview, state.project)
        Text(
            project.purpose ?: project.analysisMessage,
            color = PrimaryText,
            style = IdeTypography.compactBody)
        if (project.purpose != null)
            StatusBadge(project.analysisStatus, Modifier.padding(top = 4.dp))
        if (project.hasProject) {
          Text(
              listOf(project.projectType, project.languages, project.buildMetadata)
                  .filter(String::isNotBlank)
                  .joinToString(" · "),
              color = SecondaryText,
              style = IdeTypography.compactBody,
              modifier = Modifier.padding(top = 4.dp))
        }
      }
  if (state.impact != null || state.gitStatus != null) {
    ContextSection(
        "File context",
        DesktopIcon.Branch,
        fileContextExpanded,
        { fileContextExpanded = !fileContextExpanded }) {
          ContextReadOnlySummaries(state.impact, state.gitStatus)
        }
  }
}

@Composable
private fun ContextSection(
    title: String,
    icon: DesktopIcon,
    expanded: Boolean,
    onToggle: () -> Unit,
    content: @Composable () -> Unit,
) {
  Column(Modifier.fillMaxWidth()) {
    IdePaneHeader(
        title = title,
        icon = icon,
        expanded = expanded,
        onToggle = onToggle,
    )
    if (expanded) {
      Column(Modifier.fillMaxWidth().padding(horizontal = 8.dp, vertical = 4.dp)) { content() }
    }
    IdeHorizontalSeparator()
  }
}

/** Context content is read-only until an explicit file-analysis or edit intent is chosen. */
internal data class ContextToolWindowState(
    val inspector: SymbolInspectorUiState?,
    val bugModel: ScopedModel,
    val remoteProviderConfirmed: Boolean,
    val impact: ImpactPreview?,
    val gitStatus: GitStatus?,
    val fileAnalysis: FileAnalysis? = null,
    val project: ProjectAnalysis? = null,
    val overview: ProjectOverview? = null,
    val functionModel: ScopedModel = ScopedModel(scope = "function"),
    val functionRemoteProviderConfirmed: Boolean = false,
    val declarationExplanation: DeclarationExplanationState = DeclarationExplanationState(),
    val creationInProgress: Boolean = false,
    val analysisRun: ProjectAnalysisRunState = ProjectAnalysisRunState(),
    val fileReadError: String? = null,
)

/** File analysis and direct-edit intents available from Context. */
internal data class ContextToolWindowActions(
    val confirmRemoteProvider: (Boolean) -> Unit,
    val analyze: () -> Unit,
    val refresh: () -> Unit,
    val cancel: () -> Unit,
    val editSelected: (SymbolInspectorSymbolState) -> Unit,
    val confirmFunctionRemoteProvider: (Boolean) -> Unit = {},
    val explainSelected: () -> Unit = {},
    val cancelExplanation: () -> Unit = {},
    val createDeclaration: (() -> Unit)? = null,
    val viewResults: (() -> Unit)? = null,
    val openFile: (() -> Unit)? = null,
)

// Do not rely on the presenter's lifecycle alone: Context may render a selection while a
// previous request is being invalidated. Only the full request and response identity can be
// Current.
internal fun explanationForSelection(state: ContextToolWindowState): DeclarationExplanationState {
  val explanation = state.declarationExplanation
  if (explanation.status == DeclarationExplanationStatus.Unavailable) return explanation
  val file = state.inspector?.file
  val symbol = state.inspector?.selectedSymbol?.symbol
  val project = state.project
  val target = explanation.target
  val matches =
      project != null &&
          file != null &&
          symbol != null &&
          target != null &&
          target.file.project.id == project.projectId &&
          target.file.project.revision == project.projectRevision &&
          target.file.path == file.path &&
          target.file.contentHash == file.contentHash &&
          target.symbol == symbol.name &&
          target.signature == symbol.signature &&
          target.startLine == symbol.startLine &&
          target.endLine == symbol.endLine
  if (!matches)
      return DeclarationExplanationState(
          status = DeclarationExplanationStatus.Stale,
          message = "The selected declaration changed. Request a new explanation.")
  val result = explanation.result
  if (explanation.status == DeclarationExplanationStatus.Current &&
      (result == null ||
          result.projectId != project.projectId ||
          result.projectRevision != project.projectRevision ||
          result.baseFileHash != file.contentHash ||
          result.anchor.path != file.path ||
          result.anchor.symbol != symbol.name ||
          result.anchor.signature != symbol.signature ||
          result.anchor.startLine != symbol.startLine ||
          result.anchor.endLine != symbol.endLine))
      return DeclarationExplanationState(
          status = DeclarationExplanationStatus.Stale,
          message =
              "The explanation does not match the selected declaration. Request a new explanation.")
  return explanation
}

internal fun explanationActionLabel(state: DeclarationExplanationState): String =
    when (state.status) {
      DeclarationExplanationStatus.Loading -> "Cancel explanation"
      DeclarationExplanationStatus.Current -> "Refresh explanation"
      else -> "Explain declaration"
    }

/** Keeps one declaration action primary while retaining an explicit explanation lifecycle. */
internal data class DeclarationActionPresentation(
    val explanationTone: ActionTone,
    val refactorTone: ActionTone,
)

internal fun declarationActionPresentation(
    explanation: DeclarationExplanationState,
): DeclarationActionPresentation =
    when (explanation.status) {
      DeclarationExplanationStatus.Current ->
          DeclarationActionPresentation(ActionTone.Neutral, ActionTone.Primary)
      DeclarationExplanationStatus.Loading ->
          DeclarationActionPresentation(ActionTone.Destructive, ActionTone.Neutral)
      else -> DeclarationActionPresentation(ActionTone.Primary, ActionTone.Neutral)
    }

internal fun contextHeaderLabel(inspector: SymbolInspectorUiState): String =
    when (inspector.mode) {
      SymbolInspectorMode.FileFallback -> "File context"
      SymbolInspectorMode.SelectedSymbol ->
          "Declaration · ${requireNotNull(inspector.selectedSymbol).symbol.name}"
    }

internal fun contextToolWindowDescription(inspector: SymbolInspectorUiState): String =
    "Context tool window. ${contextHeaderLabel(inspector)}. ${inspector.file.path}. ${inspector.analysisStatus.label}."

internal fun contextStateBadge(inspector: SymbolInspectorUiState): String? =
    inspector.currentEditIdentity?.let { identity ->
      if (identity.hasDraft) "CURRENT DRAFT · ${identity.targetSymbol}"
      else "BOUND CONVERSATION · ${identity.targetSymbol}"
    }

@Composable
private fun ContextFileDetails(inspector: SymbolInspectorUiState) {
  Column(Modifier.fillMaxWidth()) {
    SelectionContainer {
      Text(
          inspector.file.path,
          color = PrimaryText,
          fontFamily = FontFamily.Monospace,
          fontSize = 12.sp,
          modifier = Modifier.padding(top = 6.dp))
    }
    Text(
        "${inspector.file.language} · ${formatBytes(inspector.file.sizeBytes)} · ${inspector.file.lineCount} lines · ${inspector.analysisStatus.label}",
        color = SecondaryText,
        fontSize = 11.sp,
    )
    Text(
        inspector.sourceIndexCorrespondence.label,
        color = SecondaryText,
        style = IdeTypography.compactBody)
    if (inspector.filePurpose.isNotBlank()) {
      Text(
          inspector.filePurpose,
          color = PrimaryText,
          fontSize = 12.sp,
          modifier = Modifier.padding(top = 10.dp))
    }
  }
}

@Composable
internal fun DeclarationExplanationDetails(state: DeclarationExplanationState) {
  Column(Modifier.fillMaxWidth()) {
    val status = explanationStatusStyle(state.status)
    IdeLabelBadge(status.label, status.color)
    val message = explanationStatusMessage(state)
    Text(
        message,
        color = if (state.status == DeclarationExplanationStatus.Failed) Error else SecondaryText,
        style = IdeTypography.compactBody,
        modifier = Modifier.padding(top = 4.dp))
    val recovery = explanationRecoveryAction(state.status)
    if (recovery.isNotEmpty() && !message.contains(recovery, ignoreCase = true)) {
      Text(recovery, color = SecondaryText, style = IdeTypography.compactBody)
    }
    state.result
        ?.takeIf { state.status == DeclarationExplanationStatus.Current }
        ?.let { result -> ModelResultContent(result.summary) }
  }
}

private fun explanationStatusMessage(state: DeclarationExplanationState): String {
  if (state.status == DeclarationExplanationStatus.Current)
      return "On-demand · matches the selected loaded source and declaration; not a disk check."
  return state.message.takeUnless {
    it.isBlank() ||
        (state.status != DeclarationExplanationStatus.Unavailable &&
            (it == "No on-demand explanation has been requested." ||
                it.startsWith("Current explanation ·")))
  } ?: explanationRecoveryMessage(state.status)
}

private fun explanationRecoveryAction(status: DeclarationExplanationStatus): String =
    when (status) {
      DeclarationExplanationStatus.Loading -> "Cancel to stop this request."
      DeclarationExplanationStatus.Stale,
      DeclarationExplanationStatus.Canceled,
      DeclarationExplanationStatus.Failed -> "Select Explain to retry."
      else -> ""
    }

private fun explanationRecoveryMessage(status: DeclarationExplanationStatus): String =
    when (status) {
      DeclarationExplanationStatus.Unavailable ->
          "No on-demand explanation yet. Select Explain to request one."
      DeclarationExplanationStatus.Loading ->
          "Explanation in progress. Cancel to stop this request."
      DeclarationExplanationStatus.Stale ->
          "Source or selection changed. Request a new explanation."
      DeclarationExplanationStatus.Canceled -> "Request canceled. Select Explain to retry."
      DeclarationExplanationStatus.Failed -> "Explanation failed. Select Explain to retry."
      DeclarationExplanationStatus.Current -> ""
    }

internal fun explanationStatusStyle(status: DeclarationExplanationStatus): StatusBadgeStyle =
    when (status) {
      DeclarationExplanationStatus.Unavailable ->
          StatusBadgeStyle("No explanation yet", SecondaryText)
      DeclarationExplanationStatus.Loading -> StatusBadgeStyle("Explaining…", ResultAccent)
      DeclarationExplanationStatus.Current -> StatusBadgeStyle("Current explanation", Success)
      DeclarationExplanationStatus.Stale -> StatusBadgeStyle("Explanation needs refresh", Warning)
      DeclarationExplanationStatus.Canceled ->
          StatusBadgeStyle("Explanation canceled", SecondaryText)
      DeclarationExplanationStatus.Failed -> StatusBadgeStyle("Explanation failed", Error)
    }

@Composable
private fun ContextReadOnlySummaries(impact: ImpactPreview?, gitStatus: GitStatus?) {
  Column(Modifier.fillMaxWidth()) {
    if (impact != null) {
      SectionLabel("IMPACT · READ-ONLY")
      Text(
          advisoryImpactLabel(impact),
          color = SecondaryText,
          fontSize = 11.sp,
          modifier = Modifier.padding(top = 4.dp))
    }
    if (gitStatus != null) {
      if (impact != null) Spacer(Modifier.height(8.dp))
      SectionLabel("GIT CONTEXT · READ-ONLY")
      Text(
          gitContextLabel(gitStatus),
          color = SecondaryText,
          fontSize = 11.sp,
          modifier = Modifier.padding(top = 4.dp))
    }
  }
}

/** Context starts whole-project admission; the adjacent results action only filters a page. */
@Composable
internal fun ContextProjectAnalysisActions(
    state: ContextToolWindowState,
    actions: ContextToolWindowActions
) {
  Column(Modifier.fillMaxWidth()) {
    MiniOrcaButton(
        onClick = actions.analyze,
        enabled = state.analysisRun.run?.isActive() != true && state.analysisRun.action.isBlank(),
        tone = ActionTone.Neutral,
        modifier = Modifier.fillMaxWidth()) {
          Text("Analyze project", style = IdeTypography.action)
        }
    Text(
        "Whole project · ${analysisStatusLabel(state.analysisRun.run?.status)}",
        color = SecondaryText,
        style = IdeTypography.compactBody,
        modifier = Modifier.padding(vertical = 4.dp))
    actions.viewResults?.let { view ->
      MiniOrcaButton(
          onClick = view, tone = ActionTone.Navigation, modifier = Modifier.fillMaxWidth()) {
            Text("View this file’s results", style = IdeTypography.action)
          }
    }
  }
}
