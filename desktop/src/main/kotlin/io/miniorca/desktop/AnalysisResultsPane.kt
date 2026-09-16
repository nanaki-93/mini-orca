package io.miniorca.desktop

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.testTag
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
  BoxWithConstraints(Modifier.fillMaxSize().background(EditorCanvas)) {
    val headerLimit = maxHeight * 0.45f
    Column(
        Modifier.fillMaxSize().padding(workspacePagePadding(maxWidth, vertical = 16.dp)),
        verticalArrangement = Arrangement.spacedBy(12.dp)) {
          Column(
              Modifier.fillMaxWidth()
                  .heightIn(max = headerLimit)
                  .verticalScroll(rememberScrollState())
                  .testTag("result-overview"),
              verticalArrangement = Arrangement.spacedBy(8.dp)) {
                ResultSectionHeader(page, openAnalysis)
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
}
