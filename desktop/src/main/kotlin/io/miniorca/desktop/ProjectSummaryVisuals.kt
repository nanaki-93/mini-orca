package io.miniorca.desktop

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.TooltipArea
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.focusable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.interaction.collectIsFocusedAsState
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.selection.selectable
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.MutableState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.stateDescription
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.window.Popup

internal fun summaryAnalysisTint(status: String): Color =
    when (status) {
      "failed",
      "unavailable" -> Error
      "running" -> Information
      "excluded",
      "unknown",
      "missing" -> SecondaryText
      "fresh" -> Success
      else -> Warning
    }

@Composable
@OptIn(ExperimentalFoundationApi::class)
internal fun SummaryAnalysisStatus(presentation: ProjectSummaryPresentation) {
  val label =
      when (presentation.summaryStatus) {
        "stale" -> "Outdated"
        "missing" -> "Not analyzed"
        "failed" -> "Failed"
        "running" -> "Updating"
        "fresh" -> "Updated"
        "excluded" -> "No files selected"
        "unknown" -> "Coverage unavailable"
        else -> analysisStatusLabel(presentation.summaryStatus)
      }
  val tint = summaryAnalysisTint(presentation.summaryStatus)
  var focused by remember { mutableStateOf(false) }
  val description = "$label · ${presentation.analysisMessage}"
  val tooltip: @Composable () -> Unit = {
    IdeControlTooltip(description, Modifier.widthIn(max = 360.dp))
  }
  TooltipArea(tooltip = tooltip) {
    Box(
        Modifier.testTag("summary-analysis-status")
            .padding(end = 6.dp)
            .then(
                if (focused) Modifier.border(1.dp, FocusAccent, MiniOrcaShapes.control)
                else Modifier)
            .semantics { contentDescription = description }
            .onFocusChanged { focused = it.isFocused }
            .focusable(),
        contentAlignment = Alignment.Center) {
          Text(label, color = tint, style = IdeTypography.workspaceMetadata)
          if (focused) Popup(popupPositionProvider = IdeTooltipPosition, content = tooltip)
        }
  }
}

internal data class SummaryCoverageArc(
    val bucket: AnalysisCoverageBucket,
    val start: Double,
    val sweep: Double,
)

/** Consecutive boundaries avoid per-bucket gaps, rounding drift, and tiny-category inflation. */
internal fun summaryCoverageArcs(coverage: SummaryCoverageProjection): List<SummaryCoverageArc> {
  if (coverage !is SummaryCoverageProjection.Known || coverage.total <= 0) return emptyList()
  val total = coverage.total.toLong()
  if (coverage.buckets.any { it.count <= 0 } ||
      coverage.buckets.sumOf { it.count.toLong() } != total)
      return emptyList()
  var used = 0L
  return coverage.buckets.map { bucket ->
    val start = 360.0 * used.toDouble() / total.toDouble()
    used += bucket.count.toLong()
    val end = 360.0 * used.toDouble() / total.toDouble()
    SummaryCoverageArc(bucket.id, start, end - start)
  }
}

internal fun summaryCoveragePercent(current: Int, total: Int): String {
  require(total > 0 && current in 0..total)
  if (current == total) return "100%"
  // Whole percentages are readable, but an incomplete selection must never round to 100%.
  val rounded = (current.toLong() * 100L + total / 2L) / total
  return "${rounded.coerceAtMost(99)}%"
}

@Composable
internal fun SummaryCoverage(
    presentation: ProjectSummaryPresentation,
    inspectedBucket: MutableState<AnalysisCoverageBucket?>,
    focusedLegend: MutableState<AnalysisCoverageBucket?>,
    openAnalysis: () -> Unit,
) {
  val analysisFocus = remember { FocusRequester() }
  WorkspaceSection(modifier = Modifier.testTag("analysis-summary")) {
    Column(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(12.dp)) {
      BoxWithConstraints(Modifier.fillMaxWidth()) {
        val title: @Composable (Modifier) -> Unit = { modifier ->
          Text(
              "Analysis coverage",
              color = ResultAccent,
              style = IdeTypography.workspaceHeading,
              modifier = modifier.semantics { heading() })
        }
        if (coverageHeadingStacked(maxWidth, LocalDensity.current.fontScale)) {
          Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
            title(Modifier.fillMaxWidth())
            SummaryAnalysisStatus(presentation)
          }
        } else {
          Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
            title(Modifier.weight(1f))
            SummaryAnalysisStatus(presentation)
          }
        }
      }
      SummaryCoverageDial(presentation, inspectedBucket, focusedLegend, analysisFocus)
      MiniOrcaButton(
          onClick = openAnalysis,
          modifier = Modifier.testTag("summary-view-analysis").focusRequester(analysisFocus),
          tone = ActionTone.Navigation) {
            Text("View analysis", style = IdeTypography.action)
          }
    }
  }
}

internal fun coverageHeadingStacked(width: Dp, fontScale: Float): Boolean =
    width < 300.dp * fontScale

internal fun coverageDialStacked(width: Dp, fontScale: Float): Boolean = width < 320.dp * fontScale

@Composable
@OptIn(ExperimentalLayoutApi::class)
private fun SummaryCoverageDial(
    presentation: ProjectSummaryPresentation,
    inspectedBucket: MutableState<AnalysisCoverageBucket?>,
    focusedLegend: MutableState<AnalysisCoverageBucket?>,
    analysisFocus: FocusRequester,
) {
  val coverage = presentation.coverage
  val arcs = summaryCoverageArcs(coverage)
  val readout =
      when (coverage) {
        is SummaryCoverageProjection.Known ->
            "${coverage.saved.fresh} of ${coverage.total} selected files are up to date"
        is SummaryCoverageProjection.Empty -> "0 selected files"
        SummaryCoverageProjection.Unavailable -> "File counts unavailable"
      }
  val percent =
      (coverage as? SummaryCoverageProjection.Known)?.let {
        summaryCoveragePercent(it.saved.fresh, it.total)
      }
  val description =
      if (coverage is SummaryCoverageProjection.Known)
          "Analysis coverage: $readout · $percent · " +
              presentation.coverageMetrics.joinToString(" · ") {
                "${it.value} ${it.label.lowercase()}"
              }
      else "Analysis coverage: $readout"
  val ids = (coverage as? SummaryCoverageProjection.Known)?.buckets?.map { it.id }.orEmpty()
  val currentIds by rememberUpdatedState(ids)
  val focusRequesters =
      remember((coverage as? SummaryCoverageProjection.Known)?.owner) {
        AnalysisCoverageBucket.entries.associateWith { FocusRequester() }
      }
  LaunchedEffect(ids) {
    val removed = focusedLegend.value?.takeIf { it !in ids }
    if (removed != null) {
      val target = ids.minByOrNull { kotlin.math.abs(it.ordinal - removed.ordinal) }
      if (target != null) focusRequesters.getValue(target).requestFocus()
      else analysisFocus.requestFocus()
      focusedLegend.value = target
    }
  }
  Column(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(12.dp)) {
    BoxWithConstraints(Modifier.fillMaxWidth()) {
      val dial: @Composable () -> Unit = {
        Box(
            Modifier.size(112.dp).testTag("summary-coverage-dial").semantics {
              contentDescription = description
            },
            contentAlignment = Alignment.Center) {
              Canvas(Modifier.fillMaxSize().padding(8.dp)) {
                val stroke = 10.dp.toPx()
                val diameter = size.minDimension
                drawArc(
                    StrongSurface,
                    startAngle = -90f,
                    sweepAngle = 360f,
                    useCenter = false,
                    topLeft = androidx.compose.ui.geometry.Offset(stroke / 2, stroke / 2),
                    size = androidx.compose.ui.geometry.Size(diameter - stroke, diameter - stroke),
                    style = Stroke(stroke, cap = StrokeCap.Butt))
                arcs.zip(presentation.coverageMetrics).forEach { (arc, metric) ->
                  drawArc(
                      summaryMetricTint(metric.tone),
                      startAngle = (-90.0 + arc.start).toFloat(),
                      sweepAngle = arc.sweep.toFloat(),
                      useCenter = false,
                      topLeft = androidx.compose.ui.geometry.Offset(stroke / 2, stroke / 2),
                      size =
                          androidx.compose.ui.geometry.Size(diameter - stroke, diameter - stroke),
                      style = Stroke(stroke, cap = StrokeCap.Butt))
                }
              }
              Text(percent ?: "—", color = PrimaryText, style = IdeTypography.workspaceHeading)
            }
      }
      val caption: @Composable (Modifier) -> Unit = { modifier ->
        Column(modifier, verticalArrangement = Arrangement.spacedBy(4.dp)) {
          Text(readout, color = PrimaryText, style = IdeTypography.workspaceBody)
          Text(
              "Saved coverage · Coverage, not a health score.",
              color = SecondaryText,
              style = IdeTypography.workspaceMetadata)
          presentation.runMessage?.let {
            Text(
                it,
                color = SecondaryText,
                style = IdeTypography.workspaceMetadata,
                modifier = Modifier.testTag("summary-coverage-run-status"))
          }
        }
      }
      if (coverageDialStacked(maxWidth, LocalDensity.current.fontScale)) {
        Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
          dial()
          caption(Modifier.fillMaxWidth())
        }
      } else {
        Row(
            Modifier.fillMaxWidth(),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(16.dp)) {
              dial()
              caption(Modifier.weight(1f))
            }
      }
    }
    presentation.selectionNotice?.let { notice ->
      Column(
          Modifier.fillMaxWidth().testTag("summary-selection-notice"),
          verticalArrangement = Arrangement.spacedBy(4.dp)) {
            Text(
                if (presentation.selectionError) "File selection needs attention"
                else "File selection in progress",
                color = if (presentation.selectionError) Error else SecondaryText,
                style = IdeTypography.workspaceMetadata)
            DiagnosticText(
                notice, color = if (presentation.selectionError) Error else SecondaryText)
          }
    }
    if (coverage is SummaryCoverageProjection.Known && arcs.isNotEmpty()) {
      FlowRow(
          Modifier.testTag("summary-coverage-legend"),
          horizontalArrangement = Arrangement.spacedBy(16.dp),
          verticalArrangement = Arrangement.spacedBy(8.dp)) {
            coverage.buckets.zip(presentation.coverageMetrics).forEach { (bucket, metric) ->
              val selected = inspectedBucket.value == bucket.id
              val interactions = remember(bucket.id) { MutableInteractionSource() }
              val focused by interactions.collectIsFocusedAsState()
              Row(
                  Modifier.testTag("summary-legend-${bucket.id}")
                      .focusRequester(focusRequesters.getValue(bucket.id))
                      .onFocusChanged {
                        if (it.isFocused) focusedLegend.value = bucket.id
                        else if (focusedLegend.value == bucket.id && bucket.id in currentIds)
                            focusedLegend.value = null
                      }
                      .background(
                          if (selected) SelectionSurface else Color.Transparent,
                          MiniOrcaShapes.control)
                      .border(
                          1.dp,
                          if (focused) FocusAccent else Color.Transparent,
                          MiniOrcaShapes.control)
                      .semantics {
                        contentDescription =
                            "${metric.label}, ${bucket.count} ${if (bucket.count == 1) "file" else "files"}"
                        stateDescription = if (selected) "Inspecting" else "Not inspecting"
                      }
                      .selectable(
                          selected = selected,
                          role = Role.Tab,
                          interactionSource = interactions,
                          indication = null,
                          onClick = { inspectedBucket.value = if (selected) null else bucket.id })
                      .padding(horizontal = 8.dp, vertical = 6.dp),
                  verticalAlignment = Alignment.CenterVertically,
                  horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                    Box(
                        Modifier.size(10.dp)
                            .background(summaryMetricTint(metric.tone), MiniOrcaShapes.pill))
                    Text(
                        "${bucket.count} ${metric.label.lowercase()}",
                        color = summaryMetricTint(metric.tone),
                        style = IdeTypography.workspaceMetadata)
                  }
            }
          }
      coverage.buckets
          .firstOrNull { it.id == inspectedBucket.value }
          ?.let { bucket ->
            SummaryCoverageInspection(
                bucket,
                coverage.total,
                presentation.coverageMetrics[coverage.buckets.indexOf(bucket)].label)
          }
    }
  }
}

private fun summaryBucketMeaning(id: AnalysisCoverageBucket): String =
    when (id) {
      AnalysisCoverageBucket.UpToDate -> "Saved analysis is current for all applicable stages."
      AnalysisCoverageBucket.Outdated -> "Saved analysis is outdated."
      AnalysisCoverageBucket.NotAnalyzed -> "No saved analysis is available yet."
      AnalysisCoverageBucket.Running ->
          "Saved stages report running or pending; this is not run progress."
      AnalysisCoverageBucket.Failed -> "A saved analysis stage failed."
      AnalysisCoverageBucket.Incomplete -> "Saved analysis is incomplete."
      AnalysisCoverageBucket.Unavailable -> "Saved analysis status is unavailable."
    }

@Composable
private fun SummaryCoverageInspection(bucket: SummaryCoverageBucket, total: Int, label: String) {
  Column(
      Modifier.fillMaxWidth()
          .testTag("summary-coverage-inspection")
          .background(StrongSurface)
          .padding(12.dp),
      verticalArrangement = Arrangement.spacedBy(8.dp)) {
        Text(
            "$label · ${bucket.count} of $total selected files",
            color = PrimaryText,
            style = IdeTypography.workspaceBody)
        Text(
            summaryBucketMeaning(bucket.id),
            color = SecondaryText,
            style = IdeTypography.workspaceMetadata)
        Text(
            "Local inspection only · no analysis starts.",
            color = SecondaryText,
            style = IdeTypography.workspaceMetadata)
        when (val paths = bucket.paths) {
          SummaryCoveragePaths.Unavailable ->
              Text(
                  "File paths are unavailable for aggregate coverage. View analysis for file scope.",
                  color = SecondaryText,
                  style = IdeTypography.workspaceBody)
          is SummaryCoveragePaths.Selected -> {
            SelectionContainer {
              Column(
                  Modifier.fillMaxWidth()
                      .heightIn(max = 240.dp)
                      .verticalScroll(rememberScrollState())
                      .testTag("summary-inspection-paths"),
                  verticalArrangement = Arrangement.spacedBy(8.dp)) {
                    paths.rows
                        .sortedBy { it.file.path }
                        .forEach { row ->
                          Column(verticalArrangement = Arrangement.spacedBy(2.dp)) {
                            Text(
                                row.file.path,
                                color = PrimaryText,
                                style = IdeTypography.resultCode)
                            DiagnosticText(
                                "${row.status.label} · ${row.explanation}", color = SecondaryText)
                          }
                        }
                  }
            }
          }
        }
      }
}

@Composable
internal fun AnalysisCategoryIcon(
    type: AnalysisResultType,
    tint: Color,
    description: String = type.workspace.name,
    modifier: Modifier = Modifier,
) {
  val icon =
      when (type) {
        AnalysisResultType.Bugs -> DesktopIcon.Problems
        AnalysisResultType.Performance -> DesktopIcon.Performance
        AnalysisResultType.Security -> DesktopIcon.Security
      }
  DesktopLineIcon(icon, description, modifier = modifier, tint = tint)
}

internal data class SummaryModule(val name: String, val path: String?, val description: String)

internal fun summaryModule(value: String): SummaryModule {
  val match =
      Regex("^([^()]+)\\(([^()]+)\\)(.*)$", RegexOption.DOT_MATCHES_ALL).matchEntire(value.trim())
          ?: return SummaryModule(value, null, "")
  return SummaryModule(
      match.groupValues[2].trim(),
      match.groupValues[1].trim().trim('`'),
      match.groupValues[3].trim().trimStart(':', '-', '–', '—').trim())
}

@Composable
internal fun SummaryModules(values: List<String>) {
  Column(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(12.dp)) {
    values.forEachIndexed { index, value ->
      if (index > 0) IdeHorizontalSeparator()
      val module = remember(value) { summaryModule(value) }
      Box(Modifier.fillMaxWidth().padding(horizontal = 8.dp, vertical = 4.dp)) {
        val identity: @Composable (Modifier) -> Unit = { modifier ->
          Column(modifier, verticalArrangement = Arrangement.spacedBy(4.dp)) {
            Text(module.name, color = PrimaryText, style = IdeTypography.workspaceHeading)
            module.path?.let {
              androidx.compose.foundation.text.selection.SelectionContainer {
                Text(it, color = SecondaryText, style = IdeTypography.resultCode)
              }
            }
          }
        }
        if (module.description.isNotBlank()) {
          Row(
              horizontalArrangement = Arrangement.spacedBy(20.dp),
              verticalAlignment = Alignment.CenterVertically) {
                identity(Modifier.weight(0.58f))
                ModelResultContent(
                    module.description,
                    preview = false,
                    style = IdeTypography.workspaceBody,
                    modifier = Modifier.weight(0.42f))
              }
        } else {
          Column(verticalArrangement = Arrangement.spacedBy(4.dp)) { identity(Modifier) }
        }
      }
    }
  }
}
