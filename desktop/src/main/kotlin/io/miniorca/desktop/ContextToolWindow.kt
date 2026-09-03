package io.miniorca.desktop

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
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
  Column(
      modifier.fillMaxSize().semantics {
        contentDescription =
            "Right tool windows. ${rightToolWindowLabel(activeToolWindow)} selected."
      }) {
        Row(Modifier.fillMaxWidth().padding(horizontal = 8.dp, vertical = 6.dp)) {
          RightToolWindow.entries.forEach { toolWindow ->
            val selected = toolWindow == activeToolWindow
            val badge = badges[toolWindow]
            FocusFlowButton(
                onClick = { onSelect(toolWindow) },
                tone = ActionTone.Navigation,
                density = ButtonDensity.Toolbar,
                selected = selected,
                modifier =
                    Modifier.weight(1f).semantics {
                      contentDescription =
                          rightToolWindowTabDescription(toolWindow, selected, badge)
                      this.selected = selected
                    },
            ) {
              Text(
                  rightToolWindowLabel(toolWindow) + badge?.let { " · ${it.label}" }.orEmpty(),
                  fontSize = 10.sp)
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
): String =
    "${rightToolWindowLabel(toolWindow)} tool window tab${badge?.let { ", ${it.label}" }.orEmpty()}, ${if (selected) "selected" else "not selected"}"

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
      modifier.verticalScroll(rememberScrollState()).padding(12.dp).semantics {
        contentDescription = contextToolWindowDescription(inspector)
      }) {
        Text("CONTEXT", color = SecondaryText, fontSize = 10.sp, fontWeight = FontWeight.Bold)
        Text(
            contextHeaderLabel(inspector),
            color = PrimaryText,
            fontSize = 14.sp,
            fontWeight = FontWeight.SemiBold,
            modifier = Modifier.padding(top = 4.dp))
        contextStateBadge(inspector)?.let { badge ->
          Text(
              badge,
              color = Warning,
              fontSize = 10.sp,
              fontWeight = FontWeight.SemiBold,
              modifier = Modifier.padding(top = 3.dp))
        }
        ContextFileDetails(inspector)
        if (inspector.mode == SymbolInspectorMode.SelectedSymbol)
            ContextDeclarationDetails(inspector, actions.editSelected)
        else
            Text(
                inspector.selectionPrompt,
                color = SecondaryText,
                fontSize = 12.sp,
                modifier = Modifier.padding(top = 10.dp))
        if (state.impact != null || state.gitStatus != null) {
          Spacer(Modifier.height(10.dp))
          ContextReadOnlySummaries(state.impact, state.gitStatus)
        }
        if (inspector.analysisAction != InspectorAnalysisAction.None) {
          val onAnalysisAction: () -> Unit =
              when (inspector.analysisAction) {
                InspectorAnalysisAction.AnalyzeFile -> actions.analyze
                InspectorAnalysisAction.RefreshAnalysis -> actions.refresh
                InspectorAnalysisAction.CancelAnalysis,
                InspectorAnalysisAction.None -> actions.cancel
              }
          Spacer(Modifier.height(10.dp))
          FocusFlowPanel(Modifier.fillMaxWidth()) {
            Text(inspector.analysisStatus.label, color = SecondaryText, fontSize = 11.sp)
            if (inspector.remoteProviderConfirmationRequired) {
              RemoteProviderConfirmation(
                  ModelScope.Bug,
                  state.bugModel,
                  state.remoteProviderConfirmed,
                  actions.confirmRemoteProvider)
            }
            FocusFlowButton(
                onClick = onAnalysisAction,
                enabled =
                    !inspector.remoteProviderConfirmationRequired || state.remoteProviderConfirmed,
                tone =
                    when (inspector.analysisAction) {
                      InspectorAnalysisAction.CancelAnalysis -> ActionTone.Destructive
                      else -> ActionTone.Primary
                    },
                modifier = Modifier.padding(top = 8.dp),
            ) {
              Text(inspector.analysisAction.label)
            }
          }
        }
      }
}

/** Context content is read-only until an explicit file-analysis or edit intent is chosen. */
internal data class ContextToolWindowState(
    val inspector: SymbolInspectorUiState?,
    val bugModel: ScopedModel,
    val remoteProviderConfirmed: Boolean,
    val impact: ImpactPreview?,
    val gitStatus: GitStatus?,
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
  FocusFlowPanel(Modifier.fillMaxWidth(), raised = true) {
    SectionLabel("FILE METADATA")
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
    onEditSelected: (SymbolInspectorSymbolState) -> Unit,
) {
  val symbol = requireNotNull(inspector.selectedSymbol)
  FocusFlowPanel(Modifier.fillMaxWidth().padding(top = 10.dp), raised = true) {
    SectionLabel("DECLARATION")
    Text(
        "${symbol.symbol.kind} · ${symbol.symbol.name}",
        color = PrimaryText,
        fontWeight = FontWeight.SemiBold,
        fontSize = 16.sp,
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
    if (symbol.editEligibility.eligible) {
      FocusFlowButton(
          onClick = { onEditSelected(symbol) },
          tone = ActionTone.Primary,
          modifier = Modifier.fillMaxWidth().padding(top = 10.dp),
      ) {
        Text("Edit ${symbol.symbol.name}")
      }
    } else {
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
  FocusFlowPanel(Modifier.fillMaxWidth()) {
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
