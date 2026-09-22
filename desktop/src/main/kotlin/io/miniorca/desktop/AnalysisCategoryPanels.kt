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
        AnalysisResultPageState(type, state.project, state.analysis.run)
      }
  val fontScale = LocalDensity.current.fontScale
  BoxWithConstraints(Modifier.fillMaxWidth()) {
    if (maxWidth / fontScale >= 640.dp)
        Row(
            Modifier.height(IntrinsicSize.Max),
            horizontalArrangement = Arrangement.spacedBy(8.dp)) {
              pages.forEach { page ->
                AnalysisCategoryPanel(page, openResults, Modifier.weight(1f).fillMaxHeight())
              }
            }
    else
        Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
          pages.forEach { page -> AnalysisCategoryPanel(page, openResults) }
        }
  }
}

@Composable
private fun AnalysisCategoryPanel(
    page: AnalysisResultPageState,
    openResults: (Workspace) -> Unit,
    modifier: Modifier = Modifier
) {
  val status = if (page.stale && page.run != null) "stale" else page.progress?.status
  val tint = analysisStatusTint(status)
  AnalysisCategoryBox(
      type = page.type,
      count = page.reportedCount,
      status = status,
      tint = tint,
      onClick = { openResults(page.type.workspace) },
      modifier = modifier.testTag("analysis-category-${page.type.category}"),
      details = {
        page.progress
            ?.coverage
            ?.takeIf { !page.stale && it.total > 0 }
            ?.let { coverage ->
              val facts = buildList {
                add("${coverage.succeeded} covered")
                if (coverage.pending + coverage.running > 0)
                    add("${coverage.pending + coverage.running} remaining")
                if (coverage.partial > 0) add("${coverage.partial} partial")
                if (coverage.failed > 0) add("${coverage.failed} failed")
                if (coverage.skipped > 0) add("${coverage.skipped} skipped")
                if (coverage.unavailable > 0) add("${coverage.unavailable} unavailable")
              }
              Text(
                  facts.joinToString(" · "),
                  color = SecondaryText,
                  style = IdeTypography.compactBody)
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

@Composable
internal fun AnalysisCategoryBox(
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
      minimumHeight = 120.dp,
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
        AnalysisCategoryIcon(type, tint)
        Text(name, color = PrimaryText, style = IdeTypography.workspaceHeading)
      }
      Text(
          count?.toString() ?: "—",
          color = PrimaryText,
          fontSize = 32.sp,
          lineHeight = 38.sp,
          fontWeight = FontWeight.SemiBold)
      (if (status == null) "Not analyzed" else analysisResultStatusLabel(status))?.let {
        Text(
            it,
            color = if (status == "completed_empty") SecondaryText else tint,
            style = IdeTypography.compactBody)
      }
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
