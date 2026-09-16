package io.miniorca.desktop

import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.TooltipArea
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.focusable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.widthIn
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import androidx.compose.ui.window.Popup

internal fun summaryAnalysisTint(status: String): Color =
    when (status) {
      "failed",
      "unavailable" -> Error
      "running" -> Information
      "excluded",
      "unknown",
      "missing" -> SecondaryText
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
        "unknown" -> "Coverage unavailable"
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
          Text(label, color = tint, style = IdeTypography.workspaceMetadata)
          if (focused) Popup(popupPositionProvider = IdeTooltipPosition, content = tooltip)
        }
  }
}

internal fun summaryCoverageFractions(
    metrics: List<ProjectSummaryMetric>
): List<Pair<ProjectSummaryMetric, Float>> {
  val known = metrics.filter { (it.value ?: 0) > 0 }
  val total = known.sumOf { requireNotNull(it.value).toLong() }
  return known.map { it to (requireNotNull(it.value).toDouble() / total).toFloat() }
}

@Composable
internal fun SummaryCoverage(presentation: ProjectSummaryPresentation, openAnalysis: () -> Unit) {
  val fontScale = LocalDensity.current.fontScale
  BoxWithConstraints(Modifier.fillMaxWidth().testTag("analysis-summary")) {
    if (maxWidth / fontScale >= 640.dp) {
      Row(
          Modifier.fillMaxWidth(),
          horizontalArrangement = Arrangement.spacedBy(16.dp),
          verticalAlignment = Alignment.CenterVertically) {
            Text(
                "Analysis coverage",
                color = ResultAccent,
                style = IdeTypography.workspaceHeading,
                modifier = Modifier.semantics { heading() })
            SummaryCoverageBar(presentation, Modifier.weight(1f))
            MiniOrcaButton(onClick = openAnalysis, tone = ActionTone.Navigation) {
              Text("View analysis", style = IdeTypography.workspaceMetadata)
            }
            SummaryAnalysisStatus(presentation)
          }
    } else {
      Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
        Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
          Text(
              "Analysis coverage",
              color = ResultAccent,
              style = IdeTypography.workspaceHeading,
              modifier = Modifier.weight(1f).semantics { heading() })
          SummaryAnalysisStatus(presentation)
        }
        SummaryCoverageBar(presentation)
        MiniOrcaButton(onClick = openAnalysis, tone = ActionTone.Navigation) {
          Text("View analysis", style = IdeTypography.workspaceMetadata)
        }
      }
    }
  }
}

@Composable
@OptIn(ExperimentalLayoutApi::class)
private fun SummaryCoverageBar(
    presentation: ProjectSummaryPresentation,
    modifier: Modifier = Modifier
) {
  val segments = summaryCoverageFractions(presentation.coverageMetrics)
  val description =
      if (segments.isEmpty()) {
        if (presentation.summaryStatus == "excluded") "No files selected"
        else "Coverage unavailable"
      } else
          segments.joinToString(" · ") { (metric, _) ->
            "${metric.value} ${metric.label.lowercase()}"
          }
  Column(modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(12.dp)) {
    Row(
        Modifier.fillMaxWidth()
            .height(12.dp)
            .clip(MiniOrcaShapes.pill)
            .background(StrongSurface)
            .semantics { contentDescription = "Analysis coverage: $description" },
        horizontalArrangement = Arrangement.spacedBy(2.dp)) {
          segments.forEach { (metric, fraction) ->
            Box(Modifier.weight(fraction).height(12.dp).background(summaryMetricTint(metric.tone)))
          }
        }
    if (segments.isEmpty()) {
      Text(
          if (presentation.summaryStatus == "excluded") "0 selected files"
          else "File counts unavailable",
          color = SecondaryText,
          style = IdeTypography.workspaceMetadata)
    } else {
      FlowRow(
          horizontalArrangement = Arrangement.spacedBy(16.dp),
          verticalArrangement = Arrangement.spacedBy(8.dp)) {
            segments.forEach { (metric, _) ->
              Row(
                  verticalAlignment = Alignment.CenterVertically,
                  horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                    Box(
                        Modifier.size(10.dp)
                            .background(summaryMetricTint(metric.tone), MiniOrcaShapes.pill))
                    Text(
                        "${metric.value} ${metric.label.lowercase()}",
                        color = SecondaryText,
                        style = IdeTypography.workspaceMetadata)
                  }
            }
          }
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
  val fontScale = LocalDensity.current.fontScale
  Column(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(12.dp)) {
    values.forEachIndexed { index, value ->
      if (index > 0) IdeHorizontalSeparator()
      val module = remember(value) { summaryModule(value) }
      BoxWithConstraints(Modifier.fillMaxWidth().padding(horizontal = 8.dp, vertical = 4.dp)) {
        val identity: @Composable (Modifier) -> Unit = { modifier ->
          Column(modifier, verticalArrangement = Arrangement.spacedBy(4.dp)) {
            Text(module.name, color = PrimaryText, style = IdeTypography.workspaceHeading)
            module.path?.let {
              androidx.compose.foundation.text.selection.SelectionContainer {
                Text(it, color = SecondaryText, style = IdeTypography.resultCode)
              }
            }
          }
        }
        if (module.description.isNotBlank() && maxWidth / fontScale >= 500.dp) {
          Row(
              horizontalArrangement = Arrangement.spacedBy(20.dp),
              verticalAlignment = Alignment.CenterVertically) {
                identity(Modifier.weight(0.58f))
                ModelResultContent(
                    module.description,
                    preview = false,
                    style = IdeTypography.workspaceBody,
                    modifier = Modifier.weight(0.42f))
              }
        } else {
          Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
            identity(Modifier)
            if (module.description.isNotBlank())
                ModelResultContent(
                    module.description, preview = false, style = IdeTypography.workspaceBody)
          }
        }
      }
    }
  }
}
