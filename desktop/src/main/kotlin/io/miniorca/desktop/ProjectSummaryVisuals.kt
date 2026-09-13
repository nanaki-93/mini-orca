package io.miniorca.desktop

import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.TooltipArea
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.focusable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.IntOffset
import androidx.compose.ui.unit.IntRect
import androidx.compose.ui.unit.IntSize
import androidx.compose.ui.unit.LayoutDirection
import androidx.compose.ui.unit.dp
import androidx.compose.ui.window.Popup
import androidx.compose.ui.window.PopupPositionProvider

@Composable
internal fun SummaryUnderstandingHeader(presentation: ProjectSummaryPresentation) {
  Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
    Text(
        "Project understanding",
        color = ResultAccent,
        style = IdeTypography.resultHeading,
        modifier = Modifier.weight(1f).semantics { heading() })
    SummaryStatusLight(presentation)
  }
}

@Composable
@OptIn(ExperimentalFoundationApi::class)
private fun SummaryStatusLight(presentation: ProjectSummaryPresentation) {
  val tint =
      when (presentation.analysisStatus) {
        "fresh" -> Success
        "failed" -> Error
        else -> Warning
      }
  var focused by remember { mutableStateOf(false) }
  val description = presentation.analysisMessage
  val tooltip: @Composable () -> Unit = {
    Text(
        description,
        color = PrimaryText,
        style = IdeTypography.compactBody,
        modifier =
            Modifier.widthIn(max = 360.dp)
                .background(OverlaySurface)
                .border(1.dp, PaneSeparator)
                .padding(8.dp))
  }
  TooltipArea(tooltip = tooltip) {
    Box(
        Modifier.size(28.dp)
            .then(if (focused) Modifier.border(1.dp, FocusAccent) else Modifier)
            .semantics { contentDescription = description }
            .onFocusChanged { focused = it.isFocused }
            .focusable(),
        contentAlignment = Alignment.Center) {
          Box(Modifier.size(10.dp).background(tint, CircleShape))
          if (focused)
              Popup(popupPositionProvider = SummaryStatusTooltipPosition, content = tooltip)
        }
  }
}

private object SummaryStatusTooltipPosition : PopupPositionProvider {
  override fun calculatePosition(
      anchorBounds: IntRect,
      windowSize: IntSize,
      layoutDirection: LayoutDirection,
      popupContentSize: IntSize,
  ): IntOffset =
      IntOffset(
          (anchorBounds.right - popupContentSize.width).coerceIn(
              0, (windowSize.width - popupContentSize.width).coerceAtLeast(0)),
          anchorBounds.bottom.coerceIn(
              0, (windowSize.height - popupContentSize.height).coerceAtLeast(0)),
      )
}

internal data class SummaryModule(val name: String, val path: String?, val description: String)

internal fun summaryModule(value: String): SummaryModule {
  val match =
      Regex("^([^()]+)\\(([^()]+)\\)(.*)$", RegexOption.DOT_MATCHES_ALL).matchEntire(value.trim())
          ?: return SummaryModule(value, null, "")
  return SummaryModule(
      match.groupValues[2].trim(),
      match.groupValues[1].trim().trim('`'),
      match.groupValues[3].trim().trimStart(':', '-', '–', '—').trim())
}

@Composable
internal fun SummaryModules(values: List<String>) {
  Column(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(12.dp)) {
    values.forEachIndexed { index, value ->
      if (index > 0) IdeHorizontalSeparator()
      val module = remember(value) { summaryModule(value) }
      Column(
          Modifier.fillMaxWidth().padding(horizontal = 8.dp, vertical = 4.dp),
          verticalArrangement = Arrangement.spacedBy(4.dp)) {
            Text(module.name, color = PrimaryText, style = IdeTypography.resultHeading)
            module.path?.let {
              androidx.compose.foundation.text.selection.SelectionContainer {
                Text(it, color = SecondaryText, style = IdeTypography.resultCode)
              }
            }
            if (module.description.isNotBlank())
                ModelResultContent(module.description, preview = false)
          }
    }
  }
}
