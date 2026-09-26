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

internal sealed interface ExplicitResultTarget {
  val target: SummaryFindingTarget

  data class Resolved(override val target: SummaryFindingTarget) : ExplicitResultTarget

  data class Unavailable(override val target: SummaryFindingTarget, val reason: String) :
      ExplicitResultTarget
}

internal class ResultBrowserState internal constructor(val identity: ResultBrowserIdentity) {
  var filter: ResultBrowserFilter by mutableStateOf(ResultBrowserFilter.All)
  var query: String by mutableStateOf("")
  var selectedKey: String? by mutableStateOf(null)
  var explicitTarget: ExplicitResultTarget? by mutableStateOf(null)
    private set

  val listState = LazyListState()

  fun choose(key: String) {
    explicitTarget = null
    selectedKey = key
  }

  fun dismissTarget() {
    explicitTarget = null
  }

  internal fun target(result: ExplicitResultTarget) {
    explicitTarget = result
    selectedKey = (result as? ExplicitResultTarget.Resolved)?.target?.rowKey
  }
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

  /** Revalidate a click against the latest snapshot, not the bounded Summary rows. */
  fun activate(target: SummaryFindingTarget, state: DesktopState): ExplicitResultTarget? {
    val page = state.analysisResultPage(target.category.category)
    val project = page.project ?: return null
    val scope = ResultBrowserScope(project.projectId, project.projectRevision, page.run?.identity)
    // A queued click from an older project or run must not reset or mutate current browsers.
    if (scope != activeScope ||
        target.projectId != scope.projectId ||
        target.projectRevision != scope.projectRevision ||
        target.run != scope.run)
        return null
    val browser = stateFor(page)
    val existing = browser.explicitTarget
    if (existing is ExplicitResultTarget.Unavailable && existing.target == target) return existing
    val result = resolveSummaryTarget(target, state)
    if (result is ExplicitResultTarget.Resolved) {
      val row = summaryLoadedFindingRows(state).single { it.target == target }
      if (browser.query.isNotBlank() &&
          !row.title.contains(browser.query.trim(), ignoreCase = true) &&
          !row.location.contains(browser.query.trim(), ignoreCase = true))
          browser.query = ""
      if (browser.filter != ResultBrowserFilter.All &&
          (browser.filter as? ResultBrowserFilter.Value)?.value != resultFacetValue(row.severity))
          browser.filter = ResultBrowserFilter.All
    }
    browser.target(result)
    return result
  }

  /** A resolved target can disappear during a same-run refresh; never fall back to another row. */
  fun reconcileTarget(page: AnalysisResultPageState, state: DesktopState) {
    val identity = resultBrowserIdentity(page)
    val scope = ResultBrowserScope(identity.projectId, identity.projectRevision, identity.run)
    // Late refreshes must not reset the active store or reconcile against a newer snapshot.
    if (scope != activeScope ||
        resultBrowserIdentity(state.analysisResultPage(page.category)) != identity)
        return
    val browser = states[identity] ?: return
    val target = (browser.explicitTarget as? ExplicitResultTarget.Resolved)?.target ?: return
    browser.target(resolveSummaryTarget(target, state))
  }

  fun resetFor(project: ProjectAnalysis?, run: AnalysisRun?) =
      resetFor(ResultBrowserScope(project?.projectId, project?.projectRevision, run?.identity))

  private fun resetFor(scope: ResultBrowserScope) {
    if (scope == activeScope) return
    states.clear()
    activeScope = scope
  }
}

internal fun resolveSummaryTarget(
    target: SummaryFindingTarget,
    state: DesktopState,
): ExplicitResultTarget {
  val page = state.analysisResultPage(target.category.category)
  if (page.project?.projectId != target.projectId ||
      page.project.projectRevision != target.projectRevision ||
      page.run?.identity != target.run)
      return ExplicitResultTarget.Unavailable(
          target, "This finding belongs to a different project or run.")
  val rows = summaryLoadedFindingRows(state).filter { it.target.category == target.category }
  val matches = rows.filter { it.target == target }
  return when {
    matches.isEmpty() ->
        ExplicitResultTarget.Unavailable(target, "This finding is no longer in the loaded results.")
    matches.size != 1 || rows.count { it.target.rowKey == target.rowKey } != 1 ->
        ExplicitResultTarget.Unavailable(
            target,
            "Multiple results share this finding identity; select a result from the category list.")
    else -> ExplicitResultTarget.Resolved(target)
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
