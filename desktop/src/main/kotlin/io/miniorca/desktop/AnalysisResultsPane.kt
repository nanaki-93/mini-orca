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
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.unit.dp

@Composable
internal fun AnalysisResultsPane(
    page: AnalysisResultPageState,
    rows: List<ResultRowPresentation>,
    browser: ResultBrowserState,
    facetLabel: String = "Severity",
    openAnalysis: () -> Unit,
    retryResults: (() -> Unit)? = null,
    tools: @Composable () -> Unit = {},
    detail: @Composable (String) -> Unit,
) {
  BoxWithConstraints(Modifier.fillMaxSize().background(EditorCanvas)) {
    val headerLimit = maxHeight * 0.45f
    val readFeedbackLimit = maxHeight * 0.25f
    val visibleRows = filteredResultRows(rows, browser.filter, browser.query)
    val hasActiveFilter = browser.filter != ResultBrowserFilter.All || browser.query.isNotBlank()
    val explicitTarget = browser.explicitTarget
    LaunchedEffect(browser.identity, rows, visibleRows, explicitTarget) {
      if (explicitTarget is ExplicitResultTarget.Resolved) {
        val matches = rows.count { it.key == explicitTarget.target.rowKey }
        if (matches != 1) {
          browser.target(
              ExplicitResultTarget.Unavailable(
                  explicitTarget.target,
                  if (matches == 0) "This finding is no longer in the loaded results."
                  else
                      "Multiple results share this finding identity; select a result from the category list."))
          return@LaunchedEffect
        }
      }
      browser.selectedKey = resultBrowserSelection(browser.selectedKey, visibleRows, explicitTarget)
    }
    Column(
        Modifier.fillMaxSize().padding(workspacePagePadding(vertical = 16.dp)),
        verticalArrangement = Arrangement.spacedBy(12.dp)) {
          Column(
              Modifier.fillMaxWidth()
                  .heightIn(max = headerLimit)
                  .verticalScroll(rememberScrollState())
                  .testTag("result-overview"),
              verticalArrangement = Arrangement.spacedBy(8.dp)) {
                ResultSectionHeader(page, rows.size, openAnalysis)
                if (rows.isNotEmpty() || hasActiveFilter)
                    ResultBrowserFilters(
                        rows = rows,
                        browser = browser,
                        facetLabel = facetLabel,
                    )
                tools()
                PreviousAnalysisDetails(page.unclassified)
              }
          ResultReadFeedback(
              page,
              rows.isNotEmpty(),
              Modifier.fillMaxWidth().heightIn(max = readFeedbackLimit),
              retryResults)
          (browser.explicitTarget as? ExplicitResultTarget.Unavailable)?.let { unavailable ->
            WorkspaceSection(
                modifier =
                    Modifier.fillMaxWidth()
                        .heightIn(max = readFeedbackLimit)
                        .verticalScroll(rememberScrollState())
                        .testTag("result-target-unavailable")) {
                  androidx.compose.material.Text(
                      "Finding unavailable · ${unavailable.reason}",
                      color = Error,
                      style = IdeTypography.workspaceBody)
                  MiniOrcaButton(
                      onClick = { browser.dismissTarget() }, tone = ActionTone.Navigation) {
                        androidx.compose.material.Text(
                            "View current ${page.type.workspace.name} results",
                            style = IdeTypography.action)
                      }
                }
          }
          if (visibleRows.isEmpty()) {
            val empty =
                if (rows.isNotEmpty())
                    AnalysisResultEmptyPresentation(
                        AnalysisResultAvailability.FilterNoMatch,
                        "No matching results.",
                        "Clear filters to view loaded results.")
                else if (page.project != null &&
                    !page.stale &&
                    (page.section.loading || page.section.error != null))
                    AnalysisResultEmptyPresentation(
                        AnalysisResultAvailability.PendingDetails,
                        "No result details loaded yet.",
                        "Saved-result read status appears above.")
                else page.emptyPresentation(loadedRows = 0)
            ResultEmptyState(empty, Modifier.weight(1f).fillMaxWidth())
          } else
              ResultListDetail(
                  visibleRows,
                  browser,
                  { key -> key?.let(browser::choose) },
                  "",
                  Modifier.weight(1f).fillMaxWidth(),
                  detail = detail)
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
              androidx.compose.material.Text("All ${rows.size}", style = IdeTypography.action)
            }
        facets.forEach { facet ->
          ChromeTab(
              selected = browser.filter == ResultBrowserFilter.Value(facet.value),
              onClick = { browser.filter = ResultBrowserFilter.Value(facet.value) },
              accessibleName = "$facetLabel ${facet.label} ${facet.count}") {
                androidx.compose.material.Text(
                    "${facet.label} ${facet.count}", style = IdeTypography.action)
              }
        }
        if (browser.filter != ResultBrowserFilter.All || browser.query.isNotBlank())
            ChromeButton(
                onClick = {
                  browser.filter = ResultBrowserFilter.All
                  browser.query = ""
                },
                accessibleName = "Clear filters") {
                  androidx.compose.material.Text("Clear filters", style = IdeTypography.action)
                }
      }
}
