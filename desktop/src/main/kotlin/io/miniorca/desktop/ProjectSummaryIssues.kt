package io.miniorca.desktop

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.stateDescription
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

internal data class SummaryBugPriorities(
    val high: Int,
    val medium: Int,
    val low: Int,
    val other: Int
) {
  val score: Int?
    get() = if (other == 0) high * 3 + medium * 2 + low else null
}

internal data class SummaryIssueMetric(
    val label: String,
    val value: Int?,
    val score: Int?,
    val status: String,
    val type: AnalysisResultType,
    val priorities: SummaryBugPriorities? = null,
)

internal fun summaryIssueMetrics(
    project: ProjectAnalysis?,
    run: AnalysisRun?,
    bugSection: AnalysisSectionState,
): List<SummaryIssueMetric> =
    AnalysisResultType.entries.map { type ->
      val page = AnalysisResultPageState(type, project, run, bugSection)
      val priorities = if (type == AnalysisResultType.Bugs) summaryBugPriorities(page) else null
      SummaryIssueMetric(
          label =
              when (type) {
                AnalysisResultType.Bugs -> "Bugs"
                AnalysisResultType.Performance -> "Performance Issues"
                AnalysisResultType.Security -> "Security Issues"
              },
          value = page.reportedCount,
          score = if (type == AnalysisResultType.Bugs) priorities?.score else page.reportedCount,
          status = page.statusLabel,
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

internal fun summaryIssueTint(score: Int?) =
    when {
      score == null -> FaintText
      score < 5 -> Success
      score < 10 -> Warning
      else -> Error
    }

@Composable
internal fun SummaryIssue(metric: SummaryIssueMetric, modifier: Modifier = Modifier) {
  val tint = summaryIssueTint(metric.score)
  Column(
      modifier
          .testTag("summary-metric-${metric.label}")
          .semantics { stateDescription = metric.status }
          .background(blendOver(tint.copy(alpha = 0.08f), Panel))) {
        Box(Modifier.fillMaxWidth().height(4.dp).background(tint))
        Column(Modifier.padding(6.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
          Row(
              verticalAlignment = Alignment.CenterVertically,
              horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                SummaryIssueIcon(metric, tint)
                Text(
                    metric.value?.toString() ?: "—",
                    color = tint,
                    fontSize = 18.sp,
                    lineHeight = 22.sp,
                    fontWeight = FontWeight.SemiBold)
              }
          if (metric.type == AnalysisResultType.Bugs) SummaryBugBreakdown(metric)
          if (metric.status != "Partial")
              Text(metric.status, color = SecondaryText, style = IdeTypography.compactBody)
        }
      }
}

@Composable
internal fun SummaryBugBreakdown(metric: SummaryIssueMetric, modifier: Modifier = Modifier) {
  val priorities = metric.priorities
  Column(modifier) {
    listOf("High" to priorities?.high, "Medium" to priorities?.medium, "Low" to priorities?.low)
        .forEach { (label, count) ->
          Text("$label: ${count ?: "—"}", color = PrimaryText, style = IdeTypography.compactBody)
        }
    if (priorities != null && priorities.other > 0)
        Text("Other: ${priorities.other}", color = SecondaryText, style = IdeTypography.compactBody)
  }
}
