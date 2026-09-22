package io.miniorca.desktop

import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.stateDescription

internal data class SummaryBugPriorities(
    val high: Int,
    val medium: Int,
    val low: Int,
    val other: Int
)

internal data class SummaryIssueMetric(
    val label: String,
    val value: Int?,
    val status: String,
    val statusCode: String?,
    val detailStatus: String?,
    val type: AnalysisResultType,
    val priorities: SummaryBugPriorities? = null,
)

internal fun summaryIssueMetrics(
    project: ProjectAnalysis?,
    run: AnalysisRun?,
    sections: Map<AnalysisResultKey, AnalysisSectionState>,
): List<SummaryIssueMetric> =
    AnalysisResultType.entries.map { type ->
      val section = sections[AnalysisResultKey(type.category)] ?: AnalysisSectionState()
      val page = AnalysisResultPageState(type, project, run, section)
      val priorities = if (type == AnalysisResultType.Bugs) summaryBugPriorities(page) else null
      val statusCode = if (page.stale && page.run != null) "stale" else page.progress?.status
      SummaryIssueMetric(
          label = type.workspace.name,
          value = page.reportedCount,
          status = page.statusLabel,
          statusCode = statusCode,
          detailStatus =
              when {
                section.loading -> "Loading details"
                section.error != null -> "Details unavailable"
                else -> null
              },
          type = type,
          priorities = priorities)
    }

private fun summaryBugPriorities(page: AnalysisResultPageState): SummaryBugPriorities? {
  val count = page.reportedCount ?: return null
  if (count == 0) return SummaryBugPriorities(0, 0, 0, 0)
  val results = page.results ?: return null
  // Progress can arrive before the corresponding findings while a run is updating.
  if (page.section.error != null || results.progress != page.progress) return null
  val findings = page.semantic.filter { it.freshness == "fresh" }
  if (findings.size != count) return null
  val counts = findings.groupingBy(::findingPriority).eachCount()
  return SummaryBugPriorities(
      counts[FindingPriority.High] ?: 0,
      counts[FindingPriority.Medium] ?: 0,
      counts[FindingPriority.Low] ?: 0,
      counts[FindingPriority.Other] ?: 0)
}

internal fun summaryIssueTint(metric: SummaryIssueMetric) =
    if (metric.value == null || metric.value == 0) FaintText
    else
        when (metric.type) {
          AnalysisResultType.Bugs -> Error
          AnalysisResultType.Performance -> Information
          AnalysisResultType.Security -> Warning
        }

@Composable
internal fun SummaryIssue(
    metric: SummaryIssueMetric,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
  val tint = summaryIssueTint(metric)
  SummaryCategoryBox(
      type = metric.type,
      count = metric.value,
      status = metric.statusCode,
      tint = tint,
      onClick = onClick,
      modifier =
          modifier.testTag("summary-metric-${metric.label}").semantics {
            stateDescription = metric.status
          },
      details = {
        metric.detailStatus?.let {
          Text(
              it,
              color = SecondaryText,
              style = IdeTypography.compactBody,
              modifier = Modifier.testTag("summary-category-detail-${metric.type.category}"))
        }
        if (metric.type == AnalysisResultType.Bugs)
            SummaryBugBreakdown(
                metric, Modifier.testTag("summary-category-priorities-${metric.type.category}"))
      })
}

@Composable
internal fun SummaryBugBreakdown(metric: SummaryIssueMetric, modifier: Modifier = Modifier) {
  val priorities = metric.priorities ?: return
  if (metric.value == 0) return
  val counts =
      listOf("High" to priorities.high, "Medium" to priorities.medium, "Low" to priorities.low) +
          if (priorities.other > 0) listOf("Other" to priorities.other) else emptyList()
  Text(
      counts.joinToString(" · ") { (label, count) -> "$label: $count" },
      color = SecondaryText,
      style = IdeTypography.compactBody,
      modifier = modifier)
}
