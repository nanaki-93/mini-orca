package io.miniorca.desktop

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
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

internal data class EditorFileHeaderUiState(
    val title: String,
    val detail: String,
)

internal fun editorFileHeaderUiState(file: ProjectFileInfo?): EditorFileHeaderUiState = file?.let {
    EditorFileHeaderUiState(title = it.name, detail = it.path)
} ?: EditorFileHeaderUiState(title = "No file open", detail = "No file selected")

internal fun editorFileHeaderDescription(header: EditorFileHeaderUiState): String =
    "${header.title}. ${header.detail}"

@Composable
internal fun EditorWorkspace(
    selected: ProjectFileInfo?,
    canvas: @Composable () -> Unit,
    modifier: Modifier = Modifier,
) {
    Column(modifier.fillMaxSize().background(AppBackground)) {
        EditorFileHeader(selected)
        Box(Modifier.fillMaxWidth().weight(1f)) {
            canvas()
        }
    }
}

@Composable
internal fun EditorFileHeader(file: ProjectFileInfo?) {
    val header = editorFileHeaderUiState(file)
    Column(
        Modifier.fillMaxWidth().background(Panel).padding(horizontal = 14.dp, vertical = 10.dp)
            .semantics { contentDescription = editorFileHeaderDescription(header) },
    ) {
        Text(
            header.title,
            color = PrimaryText,
            fontSize = 17.sp,
            fontWeight = FontWeight.SemiBold,
        )
        Text(header.detail, color = SecondaryText, fontSize = 11.sp, modifier = Modifier.padding(top = 3.dp))
    }
}
