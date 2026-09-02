package io.miniorca.desktop

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.disabled
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

internal enum class EditorSurface(val label: String) {
    Source("Source / diff"),
    FileAnalysis("File analysis"),
}

@Composable
internal fun EditorWorkspace(
    flow: EditorFlowUiState,
    onStageSelected: (EditorStage) -> Unit,
    surface: EditorSurface,
    onSurfaceSelected: (EditorSurface) -> Unit,
    canvas: @Composable () -> Unit,
    analysis: @Composable () -> Unit,
    modifier: Modifier = Modifier,
) {
    Column(modifier.fillMaxSize().background(AppBackground)) {
        EditorStageBar(flow, onStageSelected)
        EditorSurfaceBar(surface, onSurfaceSelected)
        if (surface == EditorSurface.FileAnalysis) analysis() else canvas()
    }
}

@Composable
private fun EditorSurfaceBar(surface: EditorSurface, onSurfaceSelected: (EditorSurface) -> Unit) {
    Row(Modifier.fillMaxWidth().background(Panel).padding(horizontal = 14.dp, vertical = 7.dp)) {
        EditorSurface.entries.forEachIndexed { index, option ->
            if (index > 0) Spacer(Modifier.width(6.dp))
            val current = option == surface
            FocusFlowButton(
                onClick = { onSurfaceSelected(option) },
                tone = ActionTone.Navigation,
                selected = current,
                modifier = Modifier.weight(1f).semantics {
                    selected = current
                    contentDescription = editorSurfaceSemanticsLabel(option, current)
                },
            ) { Text(editorSurfaceLabel(option, current), fontSize = 11.sp, fontWeight = if (current) FontWeight.SemiBold else FontWeight.Normal) }
        }
    }
}

internal fun editorSurfaceLabel(surface: EditorSurface, current: Boolean): String =
    if (current) "${surface.label} · Current" else surface.label

internal fun editorSurfaceSemanticsLabel(surface: EditorSurface, current: Boolean): String =
    "${editorSurfaceLabel(surface, current)} view"

@Composable
internal fun EditorStageBar(flow: EditorFlowUiState, onStageSelected: (EditorStage) -> Unit) {
    Column(Modifier.fillMaxWidth().background(Panel).padding(horizontal = 14.dp, vertical = 10.dp)) {
        SectionLabel("EDITOR FLOW")
        Text("Current: ${flow.activeStage.label}", color = SecondaryText, fontSize = 11.sp, modifier = Modifier.padding(top = 4.dp))
        Row(Modifier.fillMaxWidth().padding(top = 6.dp)) {
            flow.stages.forEachIndexed { index, stage ->
                if (index > 0) Spacer(Modifier.width(6.dp))
                val current = stage.stage == flow.activeStage
                FocusFlowButton(
                    onClick = { onStageSelected(stage.stage) },
                    enabled = stage.unlocked,
                    tone = ActionTone.Navigation,
                    selected = current,
                    modifier = Modifier.weight(1f).semantics {
                        selected = current
                        contentDescription = editorStageSemanticsLabel(stage, current)
                        if (!stage.unlocked) disabled()
                    },
                ) { Text(editorStageLabel(stage, current), fontSize = 11.sp, fontWeight = if (current) FontWeight.SemiBold else FontWeight.Normal) }
            }
        }
    }
}

internal fun editorStageLabel(stage: EditorStageUiState, current: Boolean): String = stage.stage.label

internal fun editorStageStateLabel(stage: EditorStageUiState, current: Boolean): String = when {
    current -> "Current"
    stage.unlocked -> "Ready"
    else -> "Locked"
}

internal fun editorStageSemanticsLabel(stage: EditorStageUiState, current: Boolean): String =
    "${stage.stage.label}, ${editorStageStateLabel(stage, current)}. ${stage.reason}"
