package io.miniorca.desktop

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

internal data class EditorProgressStep(
    val label: String,
    val status: String,
)

internal fun editorProgressSteps(progress: EditorProgress): List<EditorProgressStep> = when (progress) {
    EditorProgress.Inspect -> listOf(
        EditorProgressStep("Inspect", "Current"),
        EditorProgressStep("Edit", "Next"),
        EditorProgressStep("Review", "Next"),
    )
    EditorProgress.Edit -> listOf(
        EditorProgressStep("Inspect", "Complete"),
        EditorProgressStep("Edit", "Current"),
        EditorProgressStep("Review", "Next"),
    )
    EditorProgress.Review -> listOf(
        EditorProgressStep("Inspect", "Complete"),
        EditorProgressStep("Edit", "Complete"),
        EditorProgressStep("Review", "Current"),
    )
    EditorProgress.Receipt -> listOf(
        EditorProgressStep("Inspect", "Complete"),
        EditorProgressStep("Edit", "Complete"),
        EditorProgressStep("Review", "Complete"),
    )
}

internal fun editorProgressSemanticsLabel(progress: EditorProgressUiState): String =
    "Editor progress. Current: ${progress.progress.label}. ${progress.detail} " +
        editorProgressSteps(progress.progress).joinToString(". ") { "${it.label}: ${it.status}" }

@Composable
internal fun EditorWorkspace(
    progress: EditorProgressUiState,
    canvas: @Composable () -> Unit,
    modifier: Modifier = Modifier,
) {
    Column(modifier.fillMaxSize().background(AppBackground)) {
        EditorProgressBar(progress)
        canvas()
    }
}

@Composable
internal fun EditorProgressBar(progress: EditorProgressUiState) {
    val steps = editorProgressSteps(progress.progress)
    Column(
        Modifier.fillMaxWidth().background(Panel).padding(horizontal = 14.dp, vertical = 10.dp)
            .semantics { contentDescription = editorProgressSemanticsLabel(progress) },
    ) {
        SectionLabel("EDITOR PROGRESS")
        Text(
            "Current: ${progress.progress.label} · ${progress.detail}",
            color = SecondaryText,
            fontSize = 11.sp,
            modifier = Modifier.padding(top = 4.dp),
        )
        Row(Modifier.fillMaxWidth().padding(top = 6.dp)) {
            steps.forEachIndexed { index, step ->
                if (index > 0) Text(" → ", color = SecondaryText, fontSize = 11.sp)
                Text(
                    "${step.label} · ${step.status}",
                    color = if (step.status == "Current") PrimaryText else SecondaryText,
                    fontSize = 11.sp,
                    fontWeight = if (step.status == "Current") FontWeight.SemiBold else FontWeight.Normal,
                )
            }
        }
    }
}
