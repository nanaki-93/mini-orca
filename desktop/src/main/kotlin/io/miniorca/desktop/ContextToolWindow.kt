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
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
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

@Composable
internal fun ContextToolWindow(
    state: ContextToolWindowState,
    actions: ContextToolWindowActions,
    modifier: Modifier = Modifier,
) {
  val inspector = state.inspector
  if (inspector == null) {
    SystemStateMessage("Context", "Open a file to inspect its declarations.", modifier = modifier)
    return
  }
  // File details reset on navigation; selected declarations have one compact view.
  var activeTab by
      rememberSaveable(inspector.file.path, inspector.selectedSymbol?.symbol) {
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
            activeTab, inspector.file.path, inspector.selectedSymbol?.symbol) {
              Column(
                  Modifier.fillMaxWidth()
                      .weight(1f)
                      .verticalScroll(rememberScrollState())
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
  val explanation = state.declarationExplanation
  if (explanation.status == DeclarationExplanationStatus.Unavailable &&
      symbol.explanation != null) {
    ModelResultContent(symbol.explanation)
    if (inspector.analysisStatus != InspectorAnalysisStatus.Fresh) {
      StatusBadge(inspector.analysisStatus.label, Modifier.padding(top = 4.dp))
    }
  } else {
    DeclarationExplanationDetails(explanation)
  }
  Spacer(Modifier.height(8.dp))
  ExplanationAction(state, actions)
  Spacer(Modifier.height(8.dp))
  if (symbol.editEligibility.eligible) {
    MiniOrcaButton(
        onClick = { actions.editSelected(symbol) },
        tone = ActionTone.Primary,
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
}

@Composable
private fun ContextActions(state: ContextToolWindowState, actions: ContextToolWindowActions) {
  val inspector = requireNotNull(state.inspector)
  ContextCreationAction(state, actions)
  Spacer(Modifier.height(12.dp))
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
private fun ExplanationAction(state: ContextToolWindowState, actions: ContextToolWindowActions) {
  val symbol = requireNotNull(state.inspector?.selectedSymbol)
  if (!symbol.editEligibility.eligible) {
    return
  }
  val loading = state.declarationExplanation.status == DeclarationExplanationStatus.Loading
  if (state.functionModel.remoteProvider && !state.functionRemoteProviderConfirmed && !loading) {
    RemoteProviderConfirmation(
        ModelScope.Function,
        state.functionModel,
        state.functionRemoteProviderConfirmed,
        actions.confirmFunctionRemoteProvider)
  }
  MiniOrcaButton(
      onClick = if (loading) actions.cancelExplanation else actions.explainSelected,
      enabled =
          loading || !state.functionModel.remoteProvider || state.functionRemoteProviderConfirmed,
      tone = if (loading) ActionTone.Destructive else ActionTone.Navigation,
      modifier = Modifier.fillMaxWidth()) {
        Text(explanationActionLabel(state.declarationExplanation), style = IdeTypography.action)
      }
}

@Composable
private fun ContextDetails(state: ContextToolWindowState) {
  val inspector = requireNotNull(state.inspector)
  var projectExpanded by rememberSaveable { mutableStateOf(false) }
  var fileContextExpanded by rememberSaveable { mutableStateOf(false) }
  ContextFileDetails(inspector)
  EngineeringInsightPanel(
      state.fileAnalysis?.engineeringInsight,
      stale = state.fileAnalysis?.status.equals("stale", ignoreCase = true),
      scopeLabel = "File")
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
)

internal fun explanationActionLabel(state: DeclarationExplanationState): String =
    when (state.status) {
      DeclarationExplanationStatus.Loading -> "Cancel explanation"
      DeclarationExplanationStatus.Current -> "Refresh explanation"
      else -> "Explain declaration"
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
    Text(
        inspector.file.path,
        color = PrimaryText,
        fontFamily = FontFamily.Monospace,
        fontSize = 12.sp,
        modifier = Modifier.padding(top = 6.dp))
    Text(
        "${inspector.file.language} · ${formatBytes(inspector.file.sizeBytes)} · ${inspector.file.lineCount} lines · ${inspector.analysisStatus.label}",
        color = SecondaryText,
        fontSize = 11.sp,
    )
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
    if (state.status != DeclarationExplanationStatus.Current) {
      val status = explanationStatusStyle(state.status)
      IdeLabelBadge(status.label, status.color)
    }
    if (state.status == DeclarationExplanationStatus.Failed) {
      Text(
          state.message,
          color = Error,
          style = IdeTypography.body,
          modifier = Modifier.padding(top = 4.dp))
    }
    state.result
        ?.takeIf { state.status == DeclarationExplanationStatus.Current }
        ?.let { result -> ModelResultContent(result.summary) }
  }
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
