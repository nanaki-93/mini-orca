package io.miniorca.desktop

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.unit.dp

@Composable
internal fun AnalysisResultsPane(
    page: AnalysisResultPageState,
    rows: List<ResultRowPresentation>,
    browser: ResultBrowserState,
    facetLabel: String = "Severity",
    openAnalysis: () -> Unit,
    emptyMessage: String = "No findings yet.",
    tools: @Composable () -> Unit = {},
    detail: @Composable (String) -> Unit,
) {
  BoxWithConstraints(Modifier.fillMaxSize().background(EditorCanvas)) {
    val headerLimit = maxHeight * 0.45f
    val contentWidth = maxWidth - workspacePageHorizontalGutter(maxWidth) * 2
    val wide = resultListDetailUsesTwoPanes(contentWidth, LocalDensity.current.fontScale)
    val visibleRows = filteredResultRows(rows, browser.filter, browser.query)
    LaunchedEffect(browser.identity, visibleRows, wide) {
      browser.selectedKey = resultBrowserSelection(browser.selectedKey, visibleRows, wide)
    }
    Column(
        Modifier.fillMaxSize().padding(workspacePagePadding(maxWidth, vertical = 16.dp)),
        verticalArrangement = Arrangement.spacedBy(12.dp)) {
          Column(
              Modifier.fillMaxWidth()
                  .heightIn(max = headerLimit)
                  .verticalScroll(rememberScrollState())
                  .testTag("result-overview"),
              verticalArrangement = Arrangement.spacedBy(8.dp)) {
                ResultSectionHeader(page, rows.size, openAnalysis)
                ResultBrowserFilters(
                    rows = rows,
                    browser = browser,
                    facetLabel = facetLabel,
                )
                tools()
                PreviousAnalysisDetails(page.unclassified)
              }
          ResultListDetail(
              visibleRows,
              browser,
              { browser.selectedKey = it },
              if (rows.isNotEmpty() && visibleRows.isEmpty())
                  "No matching results. Clear filters to view loaded results."
              else emptyMessage,
              Modifier.weight(1f).fillMaxWidth(),
              wide,
              detail)
        }
  }
}

@Composable
private fun ResultBrowserFilters(
    rows: List<ResultRowPresentation>,
    browser: ResultBrowserState,
    facetLabel: String,
) {
  val facets = resultBrowserFacets(rows)
  androidx.compose.foundation.layout.FlowRow(
      horizontalArrangement = Arrangement.spacedBy(8.dp),
      verticalArrangement = Arrangement.spacedBy(8.dp)) {
        CompactSingleLineField(
            value = browser.query,
            onValueChange = { browser.query = it },
            label = "Filter results",
            modifier = Modifier.width(220.dp),
            placeholder = "Title or path",
        )
        androidx.compose.material.Text(facetLabel, style = IdeTypography.workspaceMetadata)
        ChromeTab(
            selected = browser.filter == ResultBrowserFilter.All,
            onClick = { browser.filter = ResultBrowserFilter.All },
            accessibleName = "All ${rows.size}") {
              androidx.compose.material.Text(
                  "All ${rows.size}", style = IdeTypography.workspaceMetadata)
            }
        facets.forEach { facet ->
          ChromeTab(
              selected = browser.filter == ResultBrowserFilter.Value(facet.value),
              onClick = { browser.filter = ResultBrowserFilter.Value(facet.value) },
              accessibleName = "$facetLabel ${facet.label} ${facet.count}") {
                androidx.compose.material.Text(
                    "${facet.label} ${facet.count}", style = IdeTypography.workspaceMetadata)
              }
        }
        if (browser.filter != ResultBrowserFilter.All || browser.query.isNotBlank())
            ChromeButton(
                onClick = {
                  browser.filter = ResultBrowserFilter.All
                  browser.query = ""
                },
                accessibleName = "Clear filters") {
                  androidx.compose.material.Text(
                      "Clear filters", style = IdeTypography.workspaceMetadata)
                }
      }
}
