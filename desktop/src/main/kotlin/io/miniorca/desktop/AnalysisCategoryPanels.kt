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
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.font.FontWeight
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
  AccentPanel(
      page.type.workspace.name, tint, modifier.testTag("analysis-category-${page.type.category}")) {
        Column(
            Modifier.padding(start = 8.dp, end = 8.dp, bottom = 8.dp),
            verticalArrangement = Arrangement.spacedBy(4.dp)) {
              Text(
                  if (status == "completed_empty") "Completed" else analysisStatusLabel(status),
                  color = tint,
                  style = IdeTypography.resultHeading,
                  fontWeight = FontWeight.SemiBold)
              Text(
                  page.reportedCount?.let { "$it ${if (it == 1) "finding" else "findings"}" }
                      ?: "Findings unavailable",
                  color = SecondaryText,
                  style = IdeTypography.compactBody)
              ChromeButton(
                  onClick = { openResults(page.type.workspace) },
                  accessibleName = "View ${page.type.workspace.name} results",
                  modifier = Modifier.fillMaxWidth()) {
                    Text("Open results →", color = tint, style = IdeTypography.resultLabel)
                  }
            }
      }
}
