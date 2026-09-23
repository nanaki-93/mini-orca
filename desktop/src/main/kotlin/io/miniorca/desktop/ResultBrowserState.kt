package io.miniorca.desktop

import androidx.compose.foundation.lazy.LazyListState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue

/** Local result navigation state; it deliberately has no daemon or disk ownership. */
internal data class ResultBrowserIdentity(
    val projectId: String?,
    val projectRevision: String?,
    val run: AnalysisRunIdentity?,
    val category: String,
)

private data class ResultBrowserScope(
    val projectId: String?,
    val projectRevision: String?,
    val run: AnalysisRunIdentity?,
)

internal sealed class ResultBrowserFilter {
  data object All : ResultBrowserFilter()

  data class Value(val value: String) : ResultBrowserFilter()
}

internal data class ResultBrowserFacet(val value: String, val label: String, val count: Int)

internal class ResultBrowserState internal constructor(val identity: ResultBrowserIdentity) {
  var filter: ResultBrowserFilter by mutableStateOf(ResultBrowserFilter.All)
  var query: String by mutableStateOf("")
  var selectedKey: String? by mutableStateOf(null)
  val listState = LazyListState()
}

/** The shell owns these states so local navigation survives page composition changes. */
internal class ResultBrowserStore {
  private val states = mutableMapOf<ResultBrowserIdentity, ResultBrowserState>()
  private var activeScope: ResultBrowserScope? = null

  fun stateFor(page: AnalysisResultPageState): ResultBrowserState {
    val identity = resultBrowserIdentity(page)
    resetFor(ResultBrowserScope(identity.projectId, identity.projectRevision, identity.run))
    return states.getOrPut(identity) { ResultBrowserState(identity) }
  }

  fun resetFor(project: ProjectAnalysis?, run: AnalysisRun?) =
      resetFor(ResultBrowserScope(project?.projectId, project?.projectRevision, run?.identity))

  private fun resetFor(scope: ResultBrowserScope) {
    if (scope == activeScope) return
    states.clear()
    activeScope = scope
  }
}

internal fun resultBrowserIdentity(page: AnalysisResultPageState) =
    ResultBrowserIdentity(
        projectId = page.project?.projectId,
        projectRevision = page.project?.projectRevision,
        run = page.run?.identity,
        category = page.category,
    )

internal fun newResultBrowserState(page: AnalysisResultPageState) =
    ResultBrowserState(resultBrowserIdentity(page))

internal fun resultFacetValue(value: String): String =
    value.trim().lowercase().ifBlank { "unknown" }

internal fun resultFacetLabel(value: String): String =
    when (resultFacetValue(value)) {
      "critical" -> "Critical"
      "high" -> "High"
      "medium" -> "Medium"
      "low" -> "Low"
      "unknown" -> "Unknown"
      else -> value.trim().ifBlank { "Unknown" }
    }

internal fun resultBrowserFacets(rows: List<ResultRowPresentation>): List<ResultBrowserFacet> {
  val counts = rows.groupingBy { resultFacetValue(it.severity) }.eachCount()
  val order = listOf("critical", "high", "medium", "low")
  return counts.keys
      .sortedWith(
          compareBy<String> { order.indexOf(it).let { index -> if (index < 0) 99 else index } }
              .thenBy { it })
      .map { value -> ResultBrowserFacet(value, resultFacetLabel(value), counts.getValue(value)) }
}

internal fun filteredResultRows(
    rows: List<ResultRowPresentation>,
    filter: ResultBrowserFilter,
    query: String,
): List<ResultRowPresentation> {
  val normalizedQuery = query.trim()
  return rows.filter { row ->
    (filter == ResultBrowserFilter.All ||
        (filter as? ResultBrowserFilter.Value)?.value == resultFacetValue(row.severity)) &&
        (normalizedQuery.isEmpty() ||
            row.title.contains(normalizedQuery, ignoreCase = true) ||
            row.location.contains(normalizedQuery, ignoreCase = true))
  }
}

internal fun resultBrowserSelection(
    current: String?,
    visibleRows: List<ResultRowPresentation>,
): String? =
    current?.takeIf { key -> visibleRows.any { it.key == key } } ?: visibleRows.firstOrNull()?.key

internal fun nextResultBrowserKey(
    rows: List<ResultRowPresentation>,
    current: String,
    delta: Int,
): String? {
  val index = rows.indexOfFirst { it.key == current }
  if (index < 0 || rows.isEmpty()) return null
  return rows[(index + delta).mod(rows.size)].key
}
