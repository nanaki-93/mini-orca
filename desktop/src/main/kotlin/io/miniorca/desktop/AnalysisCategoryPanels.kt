package io.miniorca.desktop

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.IntrinsicSize
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

@Composable
internal fun AnalysisCategoryPanels(
    state: AnalysisWorkspacePaneState,
    openResults: (Workspace) -> Unit
) {
  val pages =
      AnalysisResultType.entries.map { type ->
        AnalysisResultPageState(
            type,
            state.project,
            state.analysis.run,
            state.analysis.sections[AnalysisResultKey(type.category)] ?: AnalysisSectionState())
      }
  val metrics = summaryIssueMetrics(state.project, state.analysis.run, state.analysis.sections)
  BoxWithConstraints(Modifier.fillMaxWidth()) {
    if (categoryPanelsStacked(maxWidth, LocalDensity.current.fontScale, 8.dp)) {
      Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
        pages.zip(metrics).forEach { (page, metric) ->
          AnalysisCategoryPanel(page, metric, openResults, Modifier.fillMaxWidth())
        }
      }
    } else {
      Row(
          Modifier.fillMaxWidth().height(IntrinsicSize.Max),
          horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            pages.zip(metrics).forEach { (page, metric) ->
              AnalysisCategoryPanel(page, metric, openResults, Modifier.weight(1f).fillMaxHeight())
            }
          }
    }
  }
}

@Composable
private fun AnalysisCategoryPanel(
    page: AnalysisResultPageState,
    metric: SummaryIssueMetric,
    openResults: (Workspace) -> Unit,
    modifier: Modifier = Modifier
) {
  val status = if (page.stale && page.run != null) "stale" else page.progress?.status
  val tint = analysisStatusTint(status)
  AnalysisCategoryBox(
      type = page.type,
      count = page.reportedCount,
      status = metric.statusCode,
      statusLabel =
          if (status == "completed_empty" && metric.statusCode == "completed") metric.status
          else null,
      tint = tint,
      onClick = { openResults(page.type.workspace) },
      modifier = modifier.testTag("analysis-category-${page.type.category}"),
      details = {
        if (status == "completed_empty" || page.section.loading || page.section.error != null)
            metric.detailStatus?.let {
              Text(it, color = SecondaryText, style = IdeTypography.compactBody)
            }
        page.coverageLabel?.let {
          Text(it, color = SecondaryText, style = IdeTypography.compactBody)
        }
      },
  )
}

internal fun analysisResultStatusLabel(status: String?): String? =
    when (status) {
      "completed" -> null
      "completed_empty" -> "No results"
      else -> analysisStatusLabel(status)
    }

private fun analysisCategoryStatusLabel(status: String?): String? =
    if (status == "completed") "Completed" else analysisResultStatusLabel(status)

@Composable
internal fun AnalysisCategoryBox(
    type: AnalysisResultType,
    count: Int?,
    status: String?,
    tint: Color,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    details: @Composable () -> Unit = {},
    statusLabel: String? = null,
) {
  val name = type.workspace.name
  IdeActionSurface(
      onClick = onClick,
      colors = analysisCategoryBoxColors(),
      accessibleName = "View $name results",
      tooltip = null,
      shape = MiniOrcaShapes.interactiveCard,
      minimumHeight = 148.dp,
      modifier = modifier.fillMaxWidth(),
      contentPadding = androidx.compose.foundation.layout.PaddingValues(0.dp),
  ) {
    Row(
        Modifier.fillMaxWidth().padding(16.dp).align(Alignment.Top),
        horizontalArrangement = Arrangement.spacedBy(12.dp),
        verticalAlignment = Alignment.Top,
    ) {
      AnalysisCategoryIcon(
          type,
          tint,
          description = "",
          modifier =
              Modifier.padding(top = 2.dp).testTag("analysis-category-icon-${type.category}"))
      Column(
          Modifier.weight(1f).testTag("analysis-category-content-${type.category}"),
          verticalArrangement = Arrangement.spacedBy(6.dp),
          horizontalAlignment = Alignment.Start,
      ) {
        Text(name, color = PrimaryText, style = IdeTypography.workspaceHeading)
        Text(
            count?.toString() ?: "—",
            color = PrimaryText,
            fontSize = 32.sp,
            lineHeight = 38.sp,
            fontWeight = FontWeight.SemiBold)
        Text(
            if (status == null) "Not analyzed"
            else statusLabel ?: analysisCategoryStatusLabel(status) ?: "Status unavailable",
            color = if (status == "completed_empty") SecondaryText else tint,
            style = IdeTypography.compactBody)
        details()
      }
    }
  }
}

@Composable
internal fun SummaryCategoryBox(
    type: AnalysisResultType,
    count: Int?,
    status: String?,
    tint: Color,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    details: @Composable () -> Unit = {},
) {
  val name = type.workspace.name
  IdeActionSurface(
      onClick = onClick,
      colors = analysisCategoryBoxColors(),
      accessibleName = "View $name results",
      tooltip = null,
      shape = MiniOrcaShapes.interactiveCard,
      minimumHeight = 132.dp,
      modifier = modifier.fillMaxWidth(),
      contentPadding = androidx.compose.foundation.layout.PaddingValues(0.dp),
  ) {
    Column(
        Modifier.fillMaxWidth().padding(16.dp).align(Alignment.Top),
        verticalArrangement = Arrangement.spacedBy(6.dp),
        horizontalAlignment = Alignment.Start,
    ) {
      Row(
          Modifier.fillMaxWidth(),
          horizontalArrangement = Arrangement.spacedBy(8.dp),
          verticalAlignment = Alignment.CenterVertically,
      ) {
        AnalysisCategoryIcon(
            type,
            tint,
            description = "",
            modifier =
                Modifier.testTag("summary-category-icon-${type.category}").clearAndSetSemantics {})
        Text(
            name,
            color = PrimaryText,
            style = IdeTypography.workspaceHeading,
            modifier = Modifier.testTag("summary-category-name-${type.category}"))
      }
      Text(
          count?.toString() ?: "—",
          color = if (count == null || count == 0) tint else PrimaryText,
          fontSize = 32.sp,
          lineHeight = 38.sp,
          fontWeight = FontWeight.SemiBold,
          modifier = Modifier.testTag("summary-category-count-${type.category}"))
      Text(
          if (status == null) "Not analyzed"
          else analysisCategoryStatusLabel(status) ?: "Status unavailable",
          color = if (status == "completed_empty") SecondaryText else tint,
          style = IdeTypography.compactBody,
          modifier = Modifier.testTag("summary-category-status-${type.category}"))
      details()
    }
  }
}

internal fun analysisCategoryBoxColors() =
    IdeActionColors(
        background = Panel,
        hoveredBackground = ControlHover,
        pressedBackground = SelectionSurface,
        selectedBackground = SelectionSurface,
        disabledBackground = Panel,
        content = PrimaryText,
        selectedContent = PrimaryText,
        disabledContent = FaintText,
        border = PaneSeparator,
    )
