package io.miniorca.desktop

import androidx.compose.foundation.background
import androidx.compose.foundation.focusable
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.input.key.KeyEventType
import androidx.compose.ui.input.key.key
import androidx.compose.ui.input.key.onPreviewKeyEvent
import androidx.compose.ui.input.key.type
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

internal data class EditorChromeUiState(
    val title: String,
    val path: String,
    val breadcrumbs: String,
    val accessibleDescription: String,
    val activeSurface: EditorSurface,
    val reviewAvailable: Boolean,
    val stageLabel: String,
)

internal fun editorChromeUiState(
    file: ProjectFileInfo?,
    selectedSymbol: SymbolInfo?,
    requestedSurface: EditorSurface,
    progress: EditorProgressUiState,
    draft: DeclarationDraft?,
): EditorChromeUiState {
  val path = file?.path ?: "No file selected"
  val reviewAvailable =
      progress.progress == EditorProgress.Review && draft?.validation?.applicable == true
  val activeSurface =
      if (reviewAvailable && requestedSurface == EditorSurface.Review) EditorSurface.Review
      else EditorSurface.Source
  val title = file?.name ?: "No file open"
  val stageLabel =
      when {
        reviewAvailable && activeSurface == EditorSurface.Source -> "REVIEW READY"
        activeSurface == EditorSurface.Review -> "CURRENT REVIEW"
        draft != null && progress.progress == EditorProgress.Edit -> "DRAFT EDITED"
        else -> "SOURCE"
      }
  val symbol = selectedSymbol?.name?.takeIf(String::isNotBlank)
  return EditorChromeUiState(
      title = title,
      path = path,
      breadcrumbs = editorBreadcrumbLabel(path, symbol),
      accessibleDescription =
          "$title. $path${symbol?.let { ". Selected declaration $it" }.orEmpty()}. Read-only ${activeSurface.name.lowercase()} surface. $stageLabel.",
      activeSurface = activeSurface,
      reviewAvailable = reviewAvailable,
      stageLabel = stageLabel,
  )
}

internal fun editorBreadcrumbLabel(path: String, symbol: String? = null): String {
  val segments = path.split('/').filter(String::isNotBlank)
  val compactPath =
      when {
        segments.isEmpty() -> path
        segments.size <= 3 -> segments.joinToString(" / ")
        else -> listOf(segments.first(), "…", segments.last()).joinToString(" / ")
      }
  return listOfNotNull(compactPath, symbol).joinToString(" / ")
}

@Composable
internal fun EditorWorkspace(
    chrome: EditorChromeUiState,
    onSelectSurface: (EditorSurface) -> Unit,
    canvas: @Composable () -> Unit,
    modifier: Modifier = Modifier,
) {
  Column(modifier.fillMaxSize().background(AppBackground)) {
    ActiveFileEditorChrome(chrome, onSelectSurface)
    Box(Modifier.fillMaxWidth().weight(1f)) { canvas() }
  }
}

@Composable
private fun ActiveFileEditorChrome(
    state: EditorChromeUiState,
    onSelectSurface: (EditorSurface) -> Unit,
) {
  val surfaces = buildList {
    add(EditorSurface.Source)
    if (state.reviewAvailable) add(EditorSurface.Review)
  }
  var focusedSurface by
      remember(state.activeSurface, state.reviewAvailable) { mutableStateOf(state.activeSurface) }
  var tabGroupHasFocus by remember { mutableStateOf(false) }
  Column(
      Modifier.fillMaxWidth()
          .background(Panel)
          .padding(horizontal = 12.dp, vertical = 7.dp)
          .onFocusChanged { tabGroupHasFocus = it.hasFocus }
          .focusable()
          .onPreviewKeyEvent { event ->
            if (event.type != KeyEventType.KeyDown) return@onPreviewKeyEvent false
            val interaction =
                tabGroupInteraction(surfaces, focusedSurface, tabGroupKey(event.key))
                    ?: return@onPreviewKeyEvent false
            focusedSurface = interaction.focused
            interaction.activate?.let(onSelectSurface)
            true
          }
          .semantics { contentDescription = state.accessibleDescription },
  ) {
    Row(verticalAlignment = Alignment.CenterVertically) {
      Text(
          state.title,
          color = PrimaryText,
          fontSize = 13.sp,
          fontWeight = FontWeight.SemiBold,
          maxLines = 1,
          overflow = TextOverflow.Ellipsis,
          modifier = Modifier.weight(1f))
      Text("READ-ONLY", color = SecondaryText, fontSize = 10.sp, fontWeight = FontWeight.SemiBold)
      Spacer(Modifier.width(8.dp))
      FocusFlowButton(
          onClick = { onSelectSurface(EditorSurface.Source) },
          tone = ActionTone.Navigation,
          density = ButtonDensity.Toolbar,
          selected = state.activeSurface == EditorSurface.Source,
          focusHighlight = tabGroupHasFocus && focusedSurface == EditorSurface.Source) {
            Text("Source", fontSize = 10.sp)
          }
      if (state.reviewAvailable) {
        Spacer(Modifier.width(4.dp))
        FocusFlowButton(
            onClick = { onSelectSurface(EditorSurface.Review) },
            tone = ActionTone.Navigation,
            density = ButtonDensity.Toolbar,
            selected = state.activeSurface == EditorSurface.Review,
            focusHighlight = tabGroupHasFocus && focusedSurface == EditorSurface.Review) {
              Text("Review candidate", fontSize = 10.sp)
            }
      }
    }
    Text(
        state.breadcrumbs,
        color = SecondaryText,
        fontSize = 11.sp,
        maxLines = 1,
        overflow = TextOverflow.Ellipsis,
        modifier = Modifier.padding(top = 4.dp))
    Text(
        state.stageLabel,
        color = if (state.reviewAvailable) Success else FaintText,
        fontSize = 10.sp,
        fontWeight = FontWeight.SemiBold,
        modifier = Modifier.padding(top = 3.dp))
  }
}
