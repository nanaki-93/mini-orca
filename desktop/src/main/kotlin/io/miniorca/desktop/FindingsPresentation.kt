package io.miniorca.desktop

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.key
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.input.key.KeyEvent
import androidx.compose.ui.input.key.KeyEventType
import androidx.compose.ui.input.key.key
import androidx.compose.ui.input.key.onPreviewKeyEvent
import androidx.compose.ui.input.key.type
import androidx.compose.ui.layout.Layout
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp

/** Explicit finding intents keep ordinary inspection separate from fix preparation. */
internal class FindingActions(
    private val prepareFinding: (UnifiedFinding) -> Unit,
    private val triageFinding: (UnifiedFinding, FindingLifecycleAction) -> Unit,
) {
  fun prepareFix(finding: UnifiedFinding) = prepareFinding(finding)

  fun triage(finding: UnifiedFinding, action: FindingLifecycleAction) =
      triageFinding(finding, action)
}

@Composable
@OptIn(ExperimentalLayoutApi::class)
internal fun FindingActionButtons(
    finding: UnifiedFinding,
    actions: FindingActions,
) {
  FlowRow(
      Modifier.fillMaxWidth().padding(top = MiniOrcaSpacing.compact),
      horizontalArrangement = Arrangement.spacedBy(8.dp),
      verticalArrangement = Arrangement.spacedBy(8.dp)) {
        MiniOrcaButton(
            onClick = { actions.prepareFix(finding) },
            enabled = findingCanPrepareFix(finding),
            tone = ActionTone.Primary,
            density = ButtonDensity.Toolbar) {
              Text("Prepare fix")
            }
        findingLifecycleActions(finding).forEach { action ->
          MiniOrcaButton(
              onClick = { actions.triage(finding, action) },
              tone =
                  if (action.status == "dismissed") ActionTone.Destructive else ActionTone.Neutral,
              density = ButtonDensity.Toolbar) {
                Text(action.label)
              }
        }
      }
}

internal fun findingLocationLabel(finding: UnifiedFinding): String {
  val location = finding.location
  if (location.path.isBlank()) return "project-wide"
  val line = if (location.startLine > 0) ":${location.startLine}" else ""
  val symbol = location.symbol.takeIf { it.isNotBlank() }?.let { " · $it" }.orEmpty()
  return "${location.path}$line$symbol"
}

/** Common scan order only; producer-specific evidence and actions stay with their result pages. */
internal data class ResultRowPresentation(
    val key: String,
    val title: String,
    val location: String,
    val summary: String,
    val severity: String,
    val source: String,
    val state: String,
)

internal fun semanticResultRow(finding: UnifiedFinding) =
    ResultRowPresentation(
        "semantic:${findingDisplayKey(finding)}",
        finding.title.ifBlank { "Untitled finding" },
        findingLocationLabel(finding),
        finding.message,
        resultFacetLabel(finding.severity),
        "",
        findingMaterialStateLabel(finding))

internal fun findingEvidenceIdentity(finding: UnifiedFinding): String =
    when (classifyFinding(finding)) {
      FindingClassification.Verified ->
          "Tool report${finding.source.takeIf { it.isNotBlank() }?.let { " · $it" }.orEmpty()}"
      FindingClassification.Suggested -> "Model suggestion"
      FindingClassification.Unclassified -> "Evidence origin unavailable"
    }

internal fun findingEvidenceTint(finding: UnifiedFinding) =
    when (classifyFinding(finding)) {
      FindingClassification.Verified -> Information
      FindingClassification.Suggested -> SelectionText
      FindingClassification.Unclassified -> SecondaryText
    }

internal fun findingMaterialStateLabel(finding: UnifiedFinding): String =
    buildList {
          finding.status
              .trim()
              .lowercase()
              .takeIf {
                it in setOf("failed", "partial", "unavailable", "canceled", "fixed", "dismissed")
              }
              ?.let { add(it.replaceFirstChar(Char::uppercase)) }
          if (finding.freshness.equals("stale", ignoreCase = true)) add("Stale")
        }
        .distinct()
        .joinToString(" · ")

internal fun resultSeverityTint(severity: String) =
    when (severity.lowercase()) {
      "critical",
      "high" -> Error
      "medium" -> Warning
      "low" -> SelectionText
      else -> SecondaryText
    }

@Composable
@OptIn(ExperimentalLayoutApi::class)
internal fun ResultRowContent(row: ResultRowPresentation) {
  Row(
      Modifier.fillMaxWidth()
          .drawBehind {
            drawRoundRect(
                color = resultSeverityTint(row.severity),
                size = androidx.compose.ui.geometry.Size(3.dp.toPx(), size.height),
                cornerRadius = androidx.compose.ui.geometry.CornerRadius(3.dp.toPx()))
          }
          .padding(start = 13.dp),
      horizontalArrangement = Arrangement.spacedBy(10.dp)) {
        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(6.dp)) {
          Text(
              row.title,
              color = PrimaryText,
              style = IdeTypography.resultHeading,
              maxLines = 2,
              overflow = TextOverflow.Ellipsis)
          FlowRow(
              horizontalArrangement = Arrangement.spacedBy(6.dp),
              verticalArrangement = Arrangement.spacedBy(4.dp)) {
                IdeLabelBadge(resultFacetLabel(row.severity), resultSeverityTint(row.severity))
                if (row.state.isNotBlank()) IdeLabelBadge(row.state, SecondaryText)
              }
          Text(row.location, color = SelectionText, style = IdeTypography.resultCode, maxLines = 1)
        }
      }
}

@Composable
internal fun ResultDetailHeader(row: ResultRowPresentation) {
  Column(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(6.dp)) {
    FlowRow(
        horizontalArrangement = Arrangement.spacedBy(6.dp),
        verticalArrangement = Arrangement.spacedBy(4.dp)) {
          IdeLabelBadge(resultFacetLabel(row.severity), resultSeverityTint(row.severity))
          if (row.state.isNotBlank()) IdeLabelBadge(row.state, SecondaryText)
        }
    Text(row.title, color = PrimaryText, style = IdeTypography.workspaceHeading)
    Text(row.location, color = SelectionText, style = IdeTypography.resultCode)
    if (row.source.isNotBlank())
        Text(row.source, color = SecondaryText, style = IdeTypography.workspaceMetadata)
  }
  IdeHorizontalSeparator(Modifier.padding(vertical = 4.dp))
}

@Composable
internal fun ResultEvidenceSection(title: String, text: String) {
  Column(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(4.dp)) {
    Text(
        title,
        color = SecondaryText,
        style = IdeTypography.workspaceMetadata,
        modifier = Modifier.semantics { heading() })
    ModelResultContent(text, style = IdeTypography.workspaceBody)
  }
}

@Composable
internal fun ResultEmptyState(
    presentation: AnalysisResultEmptyPresentation,
    modifier: Modifier = Modifier,
) {
  Box(
      modifier.verticalScroll(rememberScrollState()).testTag("result-empty"),
      contentAlignment = Alignment.Center) {
        WorkspaceSection(modifier = Modifier.fillMaxWidth()) {
          Text(presentation.message, color = PrimaryText, style = IdeTypography.workspaceBody)
          if (presentation.detail.isNotBlank())
              Text(presentation.detail, color = SecondaryText, style = IdeTypography.compactBody)
        }
      }
}

// Both columns need room for result metadata and evidence at the current text scale.
internal fun resultDetailStacked(width: androidx.compose.ui.unit.Dp, fontScale: Float): Boolean =
    width < (280.dp + 340.dp) * fontScale + 8.dp

/** Keep both scroll owners composed while changing only their measured placement. */
@Composable
internal fun ResultListDetail(
    rows: List<ResultRowPresentation>,
    browser: ResultBrowserState,
    onSelection: (String?) -> Unit,
    emptyMessage: String,
    modifier: Modifier = Modifier,
    detail: @Composable (String) -> Unit,
) {
  val selectedKey = browser.selectedKey
  val selected = rows.firstOrNull { it.key == selectedKey }
  val listState = browser.listState
  var keyboardFocusKey by remember(browser) { mutableStateOf<String?>(null) }
  LaunchedEffect(keyboardFocusKey, rows) {
    keyboardFocusKey?.let { key ->
      rows
          .indexOfFirst { it.key == key }
          .takeIf { it >= 0 }
          ?.let { index -> listState.scrollToItem(index) }
    }
  }
  val fontScale = LocalDensity.current.fontScale
  Layout(
      modifier = modifier.fillMaxSize(),
      content = {
        LazyColumn(
            Modifier.testTag("result-list"),
            state = listState,
            contentPadding = PaddingValues(bottom = 8.dp),
            verticalArrangement = Arrangement.spacedBy(8.dp)) {
              if (rows.isEmpty())
                  item {
                    WorkspaceSection {
                      Text(emptyMessage, color = SecondaryText, style = IdeTypography.workspaceBody)
                    }
                  }
              items(rows, key = { it.key }) { row ->
                val rowFocusRequester = remember(row.key) { FocusRequester() }
                IdeActionSurface(
                    onClick = { onSelection(row.key) },
                    colors =
                        IdeActionColors(
                            background = Panel,
                            hoveredBackground = ControlHover,
                            pressedBackground = SelectionSurface,
                            selectedBackground = SelectionSurface,
                            disabledBackground = Panel,
                            content = PrimaryText,
                            selectedContent = PrimaryText,
                            disabledContent = FaintText,
                            border =
                                if (row.key == selectedKey) SelectionAccent else PaneSeparator),
                    selected = row.key == selectedKey,
                    accessibleName = "Inspect ${row.title}",
                    tooltip = null,
                    shape = MiniOrcaShapes.interactiveCard,
                    minimumHeight = 64.dp,
                    contentPadding = PaddingValues(horizontal = 16.dp, vertical = 10.dp),
                    modifier =
                        Modifier.fillMaxWidth()
                            .focusRequester(rowFocusRequester)
                            .onPreviewKeyEvent { event: KeyEvent ->
                              if (event.type != KeyEventType.KeyDown) return@onPreviewKeyEvent false
                              val target =
                                  when (event.key) {
                                    Key.DirectionDown -> nextResultBrowserKey(rows, row.key, 1)
                                    Key.DirectionUp -> nextResultBrowserKey(rows, row.key, -1)
                                    else -> null
                                  }
                              when {
                                target != null -> {
                                  keyboardFocusKey = target
                                  true
                                }
                                event.key == Key.Enter || event.key == Key.Spacebar -> {
                                  onSelection(row.key)
                                  true
                                }
                                else -> false
                              }
                            }
                            .semantics { this.selected = row.key == selectedKey }) {
                      ResultRowContent(row)
                    }
                if (row.key == keyboardFocusKey)
                    LaunchedEffect(row.key, keyboardFocusKey) {
                      rowFocusRequester.requestFocus()
                      keyboardFocusKey = null
                    }
              }
            }
        val detailModifier =
            Modifier.clip(MiniOrcaShapes.interactiveCard)
                .background(Panel)
                .border(1.dp, PaneSeparator, MiniOrcaShapes.interactiveCard)
                .testTag("result-detail")
        if (selected == null) {
          Box(detailModifier.padding(24.dp), contentAlignment = Alignment.Center) {
            Column(
                horizontalAlignment = Alignment.CenterHorizontally,
                verticalArrangement = Arrangement.spacedBy(12.dp)) {
                  DesktopLineIcon(
                      DesktopIcon.Document, "Finding evidence", iconSize = 28.dp, tint = FaintText)
                  Text(
                      "Select a result to inspect its evidence.",
                      color = SecondaryText,
                      style = IdeTypography.workspaceBody,
                      textAlign = androidx.compose.ui.text.style.TextAlign.Center)
                }
          }
        } else {
          key(selected.key) {
            Column(
                detailModifier.verticalScroll(rememberScrollState()).padding(16.dp),
                verticalArrangement = Arrangement.spacedBy(12.dp)) {
                  detail(selected.key)
                }
          }
        }
      }) { measurables, constraints ->
        val width = constraints.maxWidth
        val height = constraints.maxHeight
        val gap = 8.dp.roundToPx()
        val stacked = resultDetailStacked(width.toDp(), fontScale)
        val (listWidth, listHeight, detailWidth, detailHeight, detailX, detailY) =
            if (stacked) {
              val verticalGap = gap.coerceAtMost(height)
              // Prioritize reading evidence in short windows; the lazy list stays independently
              // scrollable.
              val listHeight = ((height - verticalGap) * 0.3f).toInt()
              ResultRegionSizes(
                  width,
                  listHeight,
                  width,
                  height - verticalGap - listHeight,
                  0,
                  listHeight + verticalGap)
            } else {
              val horizontalGap = gap.coerceAtMost(width)
              val available = width - horizontalGap
              val minList = (280.dp * fontScale).roundToPx()
              val minDetail = (340.dp * fontScale).roundToPx()
              val listWidth = (available * 0.42f).toInt().coerceIn(minList, available - minDetail)
              ResultRegionSizes(
                  listWidth, height, available - listWidth, height, listWidth + horizontalGap, 0)
            }
        val list =
            measurables[0].measure(
                androidx.compose.ui.unit.Constraints.fixed(listWidth, listHeight))
        val selectedDetail =
            measurables[1].measure(
                androidx.compose.ui.unit.Constraints.fixed(detailWidth, detailHeight))
        layout(width, height) {
          list.placeRelative(0, 0)
          selectedDetail.placeRelative(detailX, detailY)
        }
      }
}

private data class ResultRegionSizes(
    val listWidth: Int,
    val listHeight: Int,
    val detailWidth: Int,
    val detailHeight: Int,
    val detailX: Int,
    val detailY: Int,
)

@Composable
@OptIn(ExperimentalLayoutApi::class)
internal fun ResultSectionHeader(
    page: AnalysisResultPageState,
    loadedCount: Int,
    openAnalysis: () -> Unit,
) {
  val status = if (page.stale && page.run != null) "stale" else page.progress?.status
  val statusText =
      if (status == "completed_empty" &&
          (loadedCount > 0 ||
              page.emptyPresentation(0).availability != AnalysisResultAvailability.CompletedEmpty))
          "Completed · details unconfirmed"
      else analysisResultStatusLabel(status)
  Column(
      Modifier.fillMaxWidth().testTag("result-header"),
      verticalArrangement = Arrangement.spacedBy(8.dp)) {
        Row(
            Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.spacedBy(12.dp),
            verticalAlignment = Alignment.CenterVertically) {
              Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(6.dp)) {
                Text(
                    page.type.workspace.name,
                    color = PrimaryText,
                    style = IdeTypography.workspaceHeading,
                    modifier = Modifier.semantics { heading() })
                FlowRow(
                    horizontalArrangement = Arrangement.spacedBy(8.dp),
                    verticalArrangement = Arrangement.spacedBy(4.dp),
                    itemVerticalAlignment = Alignment.CenterVertically) {
                      Text(
                          page.countLabel(loadedCount),
                          color = SecondaryText,
                          style = IdeTypography.workspaceMetadata)
                      statusText?.let { IdeLabelBadge(it, analysisStatusTint(status)) }
                      page.coverageLabel?.let {
                        Text(it, color = SecondaryText, style = IdeTypography.workspaceMetadata)
                      }
                      page.runTimeLabel?.let {
                        Text(it, color = SecondaryText, style = IdeTypography.workspaceMetadata)
                      }
                    }
              }
              ResultAnalysisAction(openAnalysis)
            }
        if (page.stale && page.run != null && loadedCount > 0)
            Text(
                "Retained results are out of date. Start a new analysis for current evidence.",
                color = Warning,
                style = IdeTypography.workspaceMetadata)
      }
}

/**
 * Saved-result reads remain visible above the browser, independently of filters and retained rows.
 */
@Composable
internal fun ResultReadFeedback(
    page: AnalysisResultPageState,
    hasRetainedRows: Boolean,
    modifier: Modifier = Modifier,
) {
  if (!page.section.loading && page.section.error == null) return
  Column(
      modifier.verticalScroll(rememberScrollState()).testTag("result-read-feedback"),
      verticalArrangement = Arrangement.spacedBy(6.dp)) {
        if (page.section.loading)
            WorkspaceSection {
              Text("Loading results…", color = PrimaryText, style = IdeTypography.workspaceBody)
              Text(
                  if (hasRetainedRows)
                      "Reading saved ${page.type.workspace.name} results. Previously loaded results remain available below."
                  else
                      "Reading saved ${page.type.workspace.name} results; no new analysis is being started.",
                  color = SecondaryText,
                  style = IdeTypography.workspaceMetadata)
            }
        page.section.error?.let { error ->
          WorkspaceSection {
            Text(
                "${page.type.workspace.name} · saved result read failed",
                color = Error,
                style = IdeTypography.workspaceBody)
            Text(
                "Results could not be refreshed: ${error.ifBlank { "The saved result read failed without a diagnostic." }}",
                color = Error,
                style = IdeTypography.workspaceMetadata)
            if (hasRetainedRows)
                Text(
                    "Previously loaded results remain available below.",
                    color = SecondaryText,
                    style = IdeTypography.workspaceMetadata)
          }
        }
      }
}

@Composable
private fun ResultAnalysisAction(openAnalysis: () -> Unit) {
  MiniOrcaButton(onClick = openAnalysis, tone = ActionTone.Navigation) {
    DesktopLineIcon(
        DesktopIcon.Analysis,
        "Analysis",
        modifier = Modifier.clearAndSetSemantics {},
        iconSize = 16.dp)
    androidx.compose.foundation.layout.Spacer(Modifier.size(6.dp))
    Text("View analysis", style = IdeTypography.action)
  }
}

@Composable
internal fun PreviousAnalysisDetails(findings: List<UnifiedFinding>) {
  if (findings.isEmpty()) return
  var expanded by remember { mutableStateOf(false) }
  IdeDisclosureHeader(
      "Previous analysis · unclassified",
      expanded,
      { expanded = !expanded },
      stateLabel = "Historical · excluded from current counts")
  if (expanded)
      Column(Modifier.fillMaxWidth().padding(8.dp)) {
        findings.forEach { finding ->
          Text(
              finding.title.ifBlank { "Previous risk" },
              style = IdeTypography.resultHeading,
              color = PrimaryText)
          Text(
              findingLocationLabel(finding),
              style = IdeTypography.resultCode,
              color = SelectionText)
          ModelResultContent(finding.message)
          IdeHorizontalSeparator()
        }
      }
}
