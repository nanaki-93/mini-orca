package io.miniorca.desktop

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp

@Composable
internal fun AnalysisResultsPane(
    page: AnalysisResultPageState,
    rows: List<ResultRowPresentation>,
    openAnalysis: () -> Unit,
    emptyMessage: String = "No findings yet.",
    tools: @Composable () -> Unit = {},
    detail: @Composable (String) -> Unit,
) {
  var selectedKey by
      remember(page.type, page.project?.projectId, page.run?.identity) {
        mutableStateOf<String?>(null)
      }
  Column(Modifier.fillMaxSize()) {
    ResultSectionHeader(page, openAnalysis)
    Column(Modifier.fillMaxWidth().padding(horizontal = 8.dp)) {
      when (page.type) {
        AnalysisResultType.Bugs -> Unit
        AnalysisResultType.Performance ->
            Text(
                "Source-based hypotheses · Not measured",
                color = Warning,
                style = IdeTypography.resultLabel)
        AnalysisResultType.Security ->
            Text(
                "Source rules and AI suspicions · Findings do not establish that the project is secure.",
                color = SecondaryText,
                style = IdeTypography.compactBody)
      }
      tools()
      PreviousAnalysisDetails(page.unclassified)
    }
    ResultListDetail(
        rows,
        selectedKey,
        { selectedKey = it },
        emptyMessage,
        Modifier.weight(1f).fillMaxWidth(),
        detail)
  }
}
