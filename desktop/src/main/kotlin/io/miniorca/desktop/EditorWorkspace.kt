package io.miniorca.desktop

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.material.Button
import androidx.compose.material.ButtonDefaults
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

@Composable
internal fun EditorWorkspace(
    flow: EditorFlowUiState,
    onStageSelected: (EditorStage) -> Unit,
    canvas: @Composable () -> Unit,
    modifier: Modifier = Modifier,
) {
    Column(modifier.fillMaxSize().background(AppBackground)) {
        EditorStageBar(flow, onStageSelected)
        canvas()
    }
}

@Composable
internal fun EditorStageBar(flow: EditorFlowUiState, onStageSelected: (EditorStage) -> Unit) {
    Column(Modifier.fillMaxWidth().background(Panel).padding(horizontal = 14.dp, vertical = 10.dp)) {
        SectionLabel("EDITOR FLOW")
        Row(Modifier.fillMaxWidth().padding(top = 6.dp)) {
            flow.stages.forEachIndexed { index, stage ->
                if (index > 0) Spacer(Modifier.width(6.dp))
                val current = stage.stage == flow.activeStage
                Button(
                    onClick = { onStageSelected(stage.stage) },
                    enabled = stage.unlocked,
                    colors = ButtonDefaults.buttonColors(
                        backgroundColor = if (current) Accent else Card,
                        contentColor = if (current) OnAccent else PrimaryText,
                        disabledBackgroundColor = Panel,
                        disabledContentColor = FaintText,
                    ),
                    modifier = Modifier.weight(1f).semantics {
                        selected = current
                        contentDescription = editorStageSemanticsLabel(stage, current)
                        if (!stage.unlocked) disabled()
                    },
                ) { Text(editorStageLabel(stage, current), fontSize = 11.sp, fontWeight = if (current) FontWeight.SemiBold else FontWeight.Normal) }
            }
        }
        Text(flow.stage(flow.activeStage).reason, color = if (flow.stage(flow.activeStage).unlocked) SecondaryText else Warning, fontSize = 11.sp, modifier = Modifier.padding(top = 6.dp))
    }
}

internal fun editorStageLabel(stage: EditorStageUiState, current: Boolean): String = when {
    current -> "${stage.stage.label} · Current"
    stage.unlocked -> "${stage.stage.label} · Ready"
    else -> "${stage.stage.label} · Locked"
}

internal fun editorStageSemanticsLabel(stage: EditorStageUiState, current: Boolean): String =
    "${editorStageLabel(stage, current)}. ${stage.reason}"
