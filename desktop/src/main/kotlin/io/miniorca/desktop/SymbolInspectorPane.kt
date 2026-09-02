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
    inspector: SymbolInspectorUiState?,
    remoteProvider: Boolean,
    remoteProviderConfirmed: Boolean,
    onRemoteProviderConfirmed: (Boolean) -> Unit,
    onAnalyze: () -> Unit,
    onRefresh: () -> Unit,
    onCancel: () -> Unit,
    onEditSelected: (SymbolInspectorSymbolState) -> Unit,
    modifier: Modifier = Modifier,
) {
    if (inspector == null) {
        SystemStateMessage("Editor context", "Open a file to inspect its declarations.", modifier = modifier)
        return
    }
    Column(modifier.verticalScroll(rememberScrollState()).padding(12.dp)) {
        when (inspector.mode) {
            SymbolInspectorMode.FileFallback -> FileInspectorCard(inspector)
            SymbolInspectorMode.SelectedSymbol -> SymbolInspectorCard(inspector, onEditSelected)
        }
        if (inspector.analysisAction == InspectorAnalysisAction.None && inspector.remoteProviderConfirmationRequired) {
            Spacer(Modifier.height(10.dp))
            FocusFlowPanel(Modifier.fillMaxWidth()) {
                Text("Confirm before using Refresh file analysis from Commands.", color = SecondaryText, fontSize = 11.sp)
                RemoteProviderConfirmation(remoteProvider, remoteProviderConfirmed, onRemoteProviderConfirmed)
            }
        }
        if (inspector.analysisAction != InspectorAnalysisAction.None) {
            val onAnalysisAction: () -> Unit = when (inspector.analysisAction) {
                InspectorAnalysisAction.AnalyzeFile -> onAnalyze
                InspectorAnalysisAction.RefreshAnalysis -> onRefresh
                InspectorAnalysisAction.CancelAnalysis, InspectorAnalysisAction.None -> onCancel
            }
            Spacer(Modifier.height(10.dp))
            FocusFlowPanel(Modifier.fillMaxWidth()) {
                Text(inspector.analysisStatus.label, color = SecondaryText, fontSize = 11.sp)
                if (inspector.remoteProviderConfirmationRequired) {
                    RemoteProviderConfirmation(remoteProvider, remoteProviderConfirmed, onRemoteProviderConfirmed)
                }
                FocusFlowButton(
                    onClick = onAnalysisAction,
                    enabled = !inspector.remoteProviderConfirmationRequired || remoteProviderConfirmed,
                    tone = when (inspector.analysisAction) {
                        InspectorAnalysisAction.CancelAnalysis -> ActionTone.Destructive
                        else -> ActionTone.Primary
                    },
                    modifier = Modifier.padding(top = 8.dp),
                ) { Text(inspector.analysisAction.label) }
            }
        }
    }
}

@Composable
private fun FileInspectorCard(inspector: SymbolInspectorUiState) {
    FocusFlowPanel(Modifier.fillMaxWidth(), raised = true) {
        SectionLabel("FILE")
        Text(inspector.file.path, color = PrimaryText, fontFamily = FontFamily.Monospace, fontSize = 12.sp, modifier = Modifier.padding(top = 6.dp))
        Text(
            "${inspector.file.language} · ${formatBytes(inspector.file.sizeBytes)} · ${inspector.file.lineCount} lines · ${inspector.analysisStatus.label}",
            color = SecondaryText,
            fontSize = 11.sp,
        )
        if (inspector.filePurpose.isNotBlank()) {
            Text(inspector.filePurpose, color = PrimaryText, fontSize = 12.sp, modifier = Modifier.padding(top = 10.dp))
        }
        Text(inspector.selectionPrompt, color = SecondaryText, fontSize = 12.sp, modifier = Modifier.padding(top = 10.dp))
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
        Text("${symbol.symbol.kind} · ${symbol.symbol.name}", color = PrimaryText, fontWeight = FontWeight.SemiBold, fontSize = 16.sp, modifier = Modifier.padding(top = 6.dp))
        if (symbol.signature.isNotBlank()) {
            Text(symbol.signature, color = PrimaryText, fontFamily = FontFamily.Monospace, fontSize = 11.sp, modifier = Modifier.padding(top = 8.dp))
        }
        Text("${symbol.rangeLabel} · ${symbol.confidenceLabel}", color = SecondaryText, fontSize = 11.sp, modifier = Modifier.padding(top = 5.dp))
        Text(symbol.explanation ?: "No cached explanation is available for this declaration.", color = PrimaryText, fontSize = 12.sp, modifier = Modifier.padding(top = 10.dp))
        if (symbol.editEligibility.eligible) {
            FocusFlowButton(
                onClick = { onEditSelected(symbol) },
                tone = ActionTone.Primary,
                modifier = Modifier.fillMaxWidth().padding(top = 10.dp),
            ) { Text("Edit ${symbol.symbol.name}") }
        } else {
            Text(symbol.editEligibility.blockedReason, color = Warning, fontSize = 11.sp, modifier = Modifier.padding(top = 10.dp))
        }
    }
}
