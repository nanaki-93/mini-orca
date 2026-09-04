package io.miniorca.desktop

import androidx.compose.foundation.background
import androidx.compose.foundation.focusable
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
import androidx.compose.material.Divider
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.onFocusChanged
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
  Column(
      modifier
          .fillMaxSize()
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
        Row(Modifier.fillMaxWidth()) {
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
  Column(
      modifier
          .background(Panel)
          .verticalScroll(rememberScrollState())
          .padding(horizontal = 18.dp)
          .semantics { contentDescription = contextToolWindowDescription(inspector) }) {
        ContextSection("Project summary", DesktopIcon.Summary, SelectionText) {
          val project = projectSummaryPresentation(state.overview, state.project)
          Text(
              project.purpose ?: project.analysisMessage,
              color = PrimaryText,
              fontSize = 13.sp,
              lineHeight = 20.sp)
          if (project.purpose != null) {
            Text(
                "${statusBadgeStyle(project.analysisStatus).label} · advisory interpretation",
                color = if (project.analysisStatus == "stale") Warning else SecondaryText,
                fontSize = 11.sp,
                modifier = Modifier.padding(top = 6.dp))
          }
          if (project.hasProject) {
            Text(
                listOf(project.projectType, project.languages, project.buildMetadata)
                    .filter(String::isNotBlank)
                    .joinToString(" · "),
                color = SecondaryText,
                fontSize = 12.sp,
                lineHeight = 18.sp,
                modifier = Modifier.padding(top = 10.dp))
          }
        }
        ContextSection("Focused analysis", DesktopIcon.Editor, CodeKeyword) {
          ContextFileDetails(inspector)
          if (inspector.mode == SymbolInspectorMode.SelectedSymbol)
              ContextDeclarationDetails(inspector)
          else
              Text(
                  inspector.selectionPrompt,
                  color = SecondaryText,
                  fontSize = 12.sp,
                  modifier = Modifier.padding(top = 12.dp))
          contextStateBadge(inspector)?.let {
            Text(it, color = Warning, fontSize = 11.sp, modifier = Modifier.padding(top = 12.dp))
          }
          EngineeringInsightPanel(
              state.fileAnalysis?.engineeringInsight,
              stale = state.fileAnalysis?.status.equals("stale", ignoreCase = true),
              scopeLabel = "File")
        }
        ContextSection("Quick actions", DesktopIcon.Run, FocusAccent) {
          if (inspector.analysisAction != InspectorAnalysisAction.None) {
            val onAnalysisAction =
                when (inspector.analysisAction) {
                  InspectorAnalysisAction.AnalyzeFile -> actions.analyze
                  InspectorAnalysisAction.RefreshAnalysis -> actions.refresh
                  InspectorAnalysisAction.CancelAnalysis,
                  InspectorAnalysisAction.None -> actions.cancel
                }
            if (inspector.remoteProviderConfirmationRequired) {
              RemoteProviderConfirmation(
                  ModelScope.Bug,
                  state.bugModel,
                  state.remoteProviderConfirmed,
                  actions.confirmRemoteProvider)
            }
            MiniOrcaButton(
                onClick = onAnalysisAction,
                enabled =
                    !inspector.remoteProviderConfirmationRequired || state.remoteProviderConfirmed,
                tone =
                    if (inspector.analysisAction == InspectorAnalysisAction.CancelAnalysis)
                        ActionTone.Destructive
                    else ActionTone.Neutral,
                modifier = Modifier.fillMaxWidth(),
            ) {
              DesktopLineIcon(DesktopIcon.Search, "File analysis", iconSize = 16.dp)
              Spacer(Modifier.width(8.dp))
              Text(inspector.analysisAction.label, fontSize = 12.sp, modifier = Modifier.weight(1f))
            }
          }
          inspector.selectedSymbol
              ?.takeIf { it.editEligibility.eligible }
              ?.let { symbol ->
                MiniOrcaButton(
                    onClick = { actions.editSelected(symbol) },
                    modifier = Modifier.fillMaxWidth().padding(top = 8.dp),
                ) {
                  DesktopLineIcon(DesktopIcon.Editor, "Refactor declaration", iconSize = 16.dp)
                  Spacer(Modifier.width(8.dp))
                  Text(
                      "Refactor ${symbol.symbol.name}",
                      fontSize = 12.sp,
                      maxLines = 1,
                      overflow = TextOverflow.Ellipsis,
                      modifier = Modifier.weight(1f))
                }
              }
          PreviewFeatureButton(
              PreviewFeature("Generate unit test", "Dedicated test generation is not available."),
              modifier = Modifier.fillMaxWidth().padding(top = 8.dp),
              density = ButtonDensity.Standard)
          Text(
              "Complexity — · Readability — · Preview",
              color = FaintText,
              fontSize = 11.sp,
              modifier = Modifier.padding(top = 12.dp))
        }
        if (state.impact != null || state.gitStatus != null) {
          ContextSection("File context", DesktopIcon.Branch, SecondaryText) {
            ContextReadOnlySummaries(state.impact, state.gitStatus)
          }
        }
      }
}

@Composable
private fun ContextSection(
    title: String,
    icon: DesktopIcon,
    tint: androidx.compose.ui.graphics.Color,
    content: @Composable () -> Unit,
) {
  Column(Modifier.fillMaxWidth().padding(vertical = 20.dp)) {
    Row(verticalAlignment = androidx.compose.ui.Alignment.CenterVertically) {
      DesktopLineIcon(icon, title, tint = tint, iconSize = 19.dp)
      Spacer(Modifier.width(10.dp))
      Text(title, color = PrimaryText, fontSize = 14.sp, fontWeight = FontWeight.Medium)
    }
    Spacer(Modifier.height(14.dp))
    content()
  }
  Divider(color = Border)
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
)

/** File analysis and direct-edit intents available from Context. */
internal data class ContextToolWindowActions(
    val confirmRemoteProvider: (Boolean) -> Unit,
    val analyze: () -> Unit,
    val refresh: () -> Unit,
    val cancel: () -> Unit,
    val editSelected: (SymbolInspectorSymbolState) -> Unit,
)

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
private fun ContextDeclarationDetails(
    inspector: SymbolInspectorUiState,
) {
  val symbol = requireNotNull(inspector.selectedSymbol)
  Column(Modifier.fillMaxWidth().padding(top = 16.dp)) {
    Text(
        "${symbol.symbol.kind} · ${symbol.symbol.name}",
        color = PrimaryText,
        fontWeight = FontWeight.SemiBold,
        fontSize = 14.sp,
        modifier = Modifier.padding(top = 6.dp))
    if (symbol.signature.isNotBlank()) {
      Text(
          symbol.signature,
          color = PrimaryText,
          fontFamily = FontFamily.Monospace,
          fontSize = 11.sp,
          modifier = Modifier.padding(top = 8.dp))
    }
    Text(
        "${symbol.rangeLabel} · ${symbol.confidenceLabel}",
        color = SecondaryText,
        fontSize = 11.sp,
        modifier = Modifier.padding(top = 5.dp))
    Text(
        symbol.explanation ?: "No cached explanation is available for this declaration.",
        color = PrimaryText,
        fontSize = 12.sp,
        modifier = Modifier.padding(top = 10.dp))
    if (!symbol.editEligibility.eligible) {
      Text(
          symbol.editEligibility.blockedReason,
          color = Warning,
          fontSize = 11.sp,
          modifier = Modifier.padding(top = 10.dp))
    }
  }
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
