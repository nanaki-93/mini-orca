package io.miniorca.desktop

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.IntrinsicSize
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp

@Composable
internal fun AnalysisCategoryPanels(
    state: AnalysisWorkspacePaneState,
    openResults: (Workspace) -> Unit
) {
  val pages =
      AnalysisResultType.entries.map { type ->
        AnalysisResultPageState(type, state.project, state.analysis.run)
      }
  BoxWithConstraints(Modifier.fillMaxWidth()) {
    if (maxWidth >= 640.dp)
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
  )
}

internal fun analysisCategoryBoxStatus(status: String?): String? =
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
    selected: Boolean = false,
    details: @Composable () -> Unit = {},
) {
  val name = type.workspace.name
  IdeActionSurface(
      onClick = onClick,
      colors = analysisCategoryBoxColors(tint),
      selected = selected,
      accessibleName = "View $name results",
      tooltip = name,
      shape = MiniOrcaShapes.interactiveCard,
      minimumHeight = 88.dp,
      modifier = modifier.fillMaxWidth().semantics { this.selected = selected },
      contentPadding = androidx.compose.foundation.layout.PaddingValues(8.dp),
  ) {
    Column(
        Modifier.fillMaxWidth(),
        verticalArrangement = Arrangement.spacedBy(4.dp),
        horizontalAlignment = Alignment.Start,
    ) {
      Row(
          Modifier.fillMaxWidth(),
          horizontalArrangement = Arrangement.spacedBy(8.dp),
          verticalAlignment = Alignment.CenterVertically,
      ) {
        AnalysisCategoryIcon(type, tint)
        Text(
            count?.toString() ?: "—",
            color = tint,
            style = IdeTypography.resultHeading,
        )
        Spacer(Modifier.weight(1f))
        if (selected) DesktopLineIcon(DesktopIcon.Check, "Selected", tint = SelectionAccent)
      }
      analysisCategoryBoxStatus(status)?.let {
        Text(
            it,
            color = if (status == "completed_empty") SecondaryText else tint,
            style = IdeTypography.compactBody)
      }
      details()
    }
  }
}

internal fun analysisCategoryBoxColors(tint: Color) =
    IdeActionColors(
        background = blendOver(tint.copy(alpha = 0.08f), Panel),
        hoveredBackground = blendOver(tint.copy(alpha = 0.14f), ControlHover),
        pressedBackground = SelectionSurface,
        selectedBackground = SelectionSurface,
        disabledBackground = Panel,
        content = PrimaryText,
        selectedContent = PrimaryText,
        disabledContent = FaintText,
        border = tint.copy(alpha = 0.75f),
    )
