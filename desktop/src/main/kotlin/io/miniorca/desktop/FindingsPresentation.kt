package io.miniorca.desktop

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

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
internal fun FindingActionButtons(
    finding: UnifiedFinding,
    actions: FindingActions,
) {
  ResponsiveActionGroup(Modifier.fillMaxWidth().padding(top = MiniOrcaSpacing.compact)) {
    MiniOrcaButton(
        onClick = { actions.prepareFix(finding) },
        enabled = findingCanPrepareFix(finding),
        tone = ActionTone.Navigation,
        density = ButtonDensity.Toolbar) {
          Text("Prepare fix", fontSize = 11.sp)
        }
    findingLifecycleActions(finding).forEach { action ->
      MiniOrcaButton(
          onClick = { actions.triage(finding, action) },
          tone = if (action.status == "dismissed") ActionTone.Destructive else ActionTone.Neutral,
          density = ButtonDensity.Toolbar) {
            Text(action.label, fontSize = 11.sp)
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
        finding.severity.ifBlank { "Unknown severity" },
        "",
        findingMaterialStateLabel(finding))

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
internal fun ResultRowContent(row: ResultRowPresentation) {
  Column(
      Modifier.fillMaxWidth(),
      verticalArrangement = androidx.compose.foundation.layout.Arrangement.spacedBy(4.dp)) {
        Text(row.title, color = PrimaryText, style = IdeTypography.resultHeading)
        ResponsiveActionGroup(Modifier.fillMaxWidth()) {
          IdeLabelBadge(
              row.severity.replaceFirstChar { it.uppercase() }, resultSeverityTint(row.severity))
          if (row.state.isNotBlank()) IdeLabelBadge(row.state, SecondaryText)
        }
        Text(row.location, color = SelectionText, style = IdeTypography.resultCode)
        if (row.source.isNotBlank())
            Text(row.source, color = SecondaryText, style = IdeTypography.compactBody)
        if (row.summary.isNotBlank())
            Text(
                row.summary,
                color = PrimaryText,
                style = IdeTypography.compactBody,
                maxLines = 2,
                overflow = androidx.compose.ui.text.style.TextOverflow.Ellipsis)
      }
}

/** Each list and detail pane owns one scroll area; narrow windows use an explicit Back action. */
@Composable
internal fun ResultListDetail(
    rows: List<ResultRowPresentation>,
    selectedKey: String?,
    onSelection: (String?) -> Unit,
    emptyMessage: String,
    modifier: Modifier = Modifier,
    detail: @Composable (String) -> Unit,
) {
  val selected = rows.firstOrNull { it.key == selectedKey }
  androidx.compose.foundation.layout.BoxWithConstraints(modifier) {
    val wide = maxWidth >= 900.dp
    Row(Modifier.fillMaxWidth()) {
      if (wide || selected == null) {
        androidx.compose.foundation.lazy.LazyColumn(
            Modifier.weight(if (wide) 0.42f else 1f).fillMaxHeight(),
            contentPadding = PaddingValues(vertical = 4.dp)) {
              if (rows.isEmpty())
                  item {
                    Text(
                        emptyMessage,
                        color = SecondaryText,
                        style = IdeTypography.body,
                        modifier = Modifier.padding(8.dp))
                  }
              items(rows.size, key = { rows[it].key }) { index ->
                val row = rows[index]
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
                                if (row.key == selectedKey) SelectionAccent
                                else ControlBorder.copy(alpha = 0.45f)),
                    selected = row.key == selectedKey,
                    accessibleName = "Inspect ${row.title}",
                    shape = MiniOrcaShapes.interactiveCard,
                    minimumHeight = 72.dp,
                    contentPadding = PaddingValues(10.dp),
                    modifier =
                        Modifier.fillMaxWidth()
                            .padding(horizontal = 8.dp, vertical = 4.dp)
                            .semantics { this.selected = row.key == selectedKey }) {
                      ResultRowContent(row)
                    }
              }
            }
      }
      if (wide) IdeVerticalSeparator(Modifier.fillMaxHeight())
      if (wide || selected != null) {
        Column(
            Modifier.weight(if (wide) 0.58f else 1f)
                .fillMaxHeight()
                .padding(8.dp)
                .verticalScroll(rememberScrollState())) {
              if (selected == null)
                  Text(
                      "Select a result to inspect its evidence.",
                      color = SecondaryText,
                      style = IdeTypography.body)
              else {
                if (!wide)
                    MiniOrcaButton(
                        onClick = { onSelection(null) },
                        tone = ActionTone.Neutral,
                        density = ButtonDensity.Toolbar) {
                          Text("Back to results")
                        }
                detail(selected.key)
              }
            }
      }
    }
  }
}

@Composable
internal fun ResultSectionHeader(
    page: AnalysisResultPageState,
    openAnalysis: () -> Unit,
) {
  val status = if (page.stale && page.run != null) "stale" else page.progress?.status
  Column(
      Modifier.fillMaxWidth().padding(8.dp),
      verticalArrangement = androidx.compose.foundation.layout.Arrangement.spacedBy(8.dp)) {
        Row(
            Modifier.fillMaxWidth(),
            horizontalArrangement = androidx.compose.foundation.layout.Arrangement.spacedBy(8.dp),
            verticalAlignment = Alignment.CenterVertically) {
              Column(Modifier.weight(1f)) {
                page.reportedCount?.let { count ->
                  Text(
                      "$count ${if (count == 1) "finding" else "findings"}",
                      color = PrimaryText,
                      style = IdeTypography.compactBody)
                }
                analysisResultStatusLabel(status)?.let {
                  Text(it, color = analysisStatusTint(status), style = IdeTypography.compactBody)
                }
              }
              MiniOrcaButton(
                  onClick = openAnalysis,
                  tone = ActionTone.Navigation,
                  density = ButtonDensity.Toolbar) {
                    Text("View analysis")
                  }
            }
        page.section.error?.let {
          Text(
              "Results could not be refreshed: $it",
              color = Error,
              style = IdeTypography.compactBody)
        }
        if (page.section.loading)
            Text("Loading results…", color = SecondaryText, style = IdeTypography.compactBody)
        if (page.stale && page.run != null)
            Text(
                "Retained results are out of date. Start a new analysis for current evidence.",
                color = Warning,
                style = IdeTypography.compactBody)
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
      Column(
          Modifier.fillMaxWidth()
              .padding(8.dp)
              .heightIn(max = 180.dp)
              .verticalScroll(rememberScrollState())) {
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
