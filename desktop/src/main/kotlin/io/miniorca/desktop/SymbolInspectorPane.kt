package io.miniorca.desktop

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

@Composable
internal fun SymbolInspectorPane(
    state: SymbolInspectorPaneState,
    actions: SymbolInspectorActions,
    modifier: Modifier = Modifier,
) {
  val inspector = state.inspector
  if (inspector == null) {
    SystemStateMessage(
        "Editor context", "Open a file to inspect its declarations.", modifier = modifier)
    return
  }
  Column(modifier.verticalScroll(rememberScrollState()).padding(12.dp)) {
    when (inspector.mode) {
      SymbolInspectorMode.FileFallback -> FileInspectorCard(inspector)
      SymbolInspectorMode.SelectedSymbol -> SymbolInspectorCard(inspector, actions.editSelected)
    }
    if (inspector.analysisAction == InspectorAnalysisAction.None &&
        inspector.remoteProviderConfirmationRequired) {
      Spacer(Modifier.height(10.dp))
      FocusFlowPanel(Modifier.fillMaxWidth()) {
        Text(
            "Confirm before using Refresh file analysis from Commands.",
            color = SecondaryText,
            fontSize = 11.sp)
        RemoteProviderConfirmation(
            ModelScope.Bug,
            state.bugModel,
            state.remoteProviderConfirmed,
            actions.confirmRemoteProvider)
      }
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

/** Inspector data remains read-only until an explicit editor workflow intent is chosen. */
internal data class SymbolInspectorPaneState(
    val inspector: SymbolInspectorUiState?,
    val bugModel: ScopedModel,
    val remoteProviderConfirmed: Boolean,
)

/** File analysis and direct-edit intents available from the inspector. */
internal data class SymbolInspectorActions(
    val confirmRemoteProvider: (Boolean) -> Unit,
    val analyze: () -> Unit,
    val refresh: () -> Unit,
    val cancel: () -> Unit,
    val editSelected: (SymbolInspectorSymbolState) -> Unit,
)

@Composable
private fun FileInspectorCard(inspector: SymbolInspectorUiState) {
  FocusFlowPanel(Modifier.fillMaxWidth(), raised = true) {
    SectionLabel("FILE")
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
    Text(
        inspector.selectionPrompt,
        color = SecondaryText,
        fontSize = 12.sp,
        modifier = Modifier.padding(top = 10.dp))
  }
}

@Composable
private fun SymbolInspectorCard(
    inspector: SymbolInspectorUiState,
    onEditSelected: (SymbolInspectorSymbolState) -> Unit,
) {
  val symbol = requireNotNull(inspector.selectedSymbol)
  FocusFlowPanel(Modifier.fillMaxWidth(), raised = true) {
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
