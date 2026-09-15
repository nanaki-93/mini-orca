package io.miniorca.desktop

import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.TooltipArea
import androidx.compose.foundation.border
import androidx.compose.foundation.focusable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.widthIn
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import androidx.compose.ui.window.Popup

internal fun summaryAnalysisTint(status: String): Color =
    when (status) {
      "failed",
      "unavailable" -> Error
      "running" -> Information
      "excluded" -> SecondaryText
      "fresh" -> Success
      else -> Warning
    }

@Composable
@OptIn(ExperimentalFoundationApi::class)
internal fun SummaryAnalysisStatus(presentation: ProjectSummaryPresentation) {
  val label =
      when (presentation.summaryStatus) {
        "stale" -> "Outdated"
        "missing" -> "Not analyzed"
        "failed" -> "Failed"
        "running" -> "Updating"
        "fresh" -> "Updated"
        "excluded" -> "No files selected"
        else -> analysisStatusLabel(presentation.summaryStatus)
      }
  val tint = summaryAnalysisTint(presentation.summaryStatus)
  var focused by remember { mutableStateOf(false) }
  val description = presentation.analysisMessage
  val tooltip: @Composable () -> Unit = {
    IdeControlTooltip(description, Modifier.widthIn(max = 360.dp))
  }
  TooltipArea(tooltip = tooltip) {
    Box(
        Modifier.testTag("summary-analysis-status")
            .padding(end = 6.dp)
            .then(
                if (focused) Modifier.border(1.dp, FocusAccent, MiniOrcaShapes.control)
                else Modifier)
            .semantics { contentDescription = description }
            .onFocusChanged { focused = it.isFocused }
            .focusable(),
        contentAlignment = Alignment.Center) {
          Text(label, color = tint, style = IdeTypography.resultLabel)
          if (focused) Popup(popupPositionProvider = IdeTooltipPosition, content = tooltip)
        }
  }
}

@Composable
internal fun AnalysisCategoryIcon(
    type: AnalysisResultType,
    tint: Color,
    description: String = type.workspace.name,
) {
  val icon =
      when (type) {
        AnalysisResultType.Bugs -> DesktopIcon.Problems
        AnalysisResultType.Performance -> DesktopIcon.Performance
        AnalysisResultType.Security -> DesktopIcon.Security
      }
  DesktopLineIcon(icon, description, tint = tint)
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
