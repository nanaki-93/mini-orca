package io.miniorca.desktop

import androidx.compose.foundation.layout.Column
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
import androidx.compose.ui.Modifier
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

/** Explicit finding intents keep ordinary inspection separate from fix preparation. */
internal class FindingActions(
    private val openFinding: (UnifiedFinding) -> Unit,
    private val prepareFinding: (UnifiedFinding) -> Unit,
    private val triageFinding: (UnifiedFinding, FindingLifecycleAction) -> Unit,
) {
  fun select(finding: UnifiedFinding) = openFinding(finding)

  fun prepareFix(finding: UnifiedFinding) = prepareFinding(finding)

  fun triage(finding: UnifiedFinding, action: FindingLifecycleAction) =
      triageFinding(finding, action)
}

/** Local UI input only; the shared [FindingsPresentation] owns all filter interpretation. */
internal class FindingsFilterState {
  var query by mutableStateOf("")
  var source by mutableStateOf("")
  var severity by mutableStateOf("")
  var freshness by mutableStateOf("")
  var lifecycle by mutableStateOf("")
  var advancedFiltersVisible by mutableStateOf(false)

  val filters: BugsFilters
    get() = BugsFilters(query, source, severity, freshness, lifecycle)
}

@Composable
internal fun rememberFindingsFilterState(): FindingsFilterState = remember { FindingsFilterState() }

@Composable
internal fun FindingsFilterControls(
    state: FindingsFilterState,
    presentation: FindingsPresentation,
    modifier: Modifier = Modifier,
) {
  Column(modifier) {
    CompactSingleLineField(
        state.query,
        { state.query = it },
        label = "Search findings",
        showLabel = false,
        modifier = Modifier.fillMaxWidth())
    IdeDisclosureHeader(
        title = "Filters",
        expanded = state.advancedFiltersVisible,
        onToggle = { state.advancedFiltersVisible = !state.advancedFiltersVisible },
        stateLabel = findingsFilterStateLabel(presentation.activeFilters),
        modifier = Modifier.padding(top = 6.dp))
    if (state.advancedFiltersVisible) {
      ResponsiveFieldPair(
          modifier = Modifier.fillMaxWidth().padding(top = 6.dp),
          minimumHorizontalWidth = 520.dp,
          first = { fieldModifier ->
            CompactSingleLineField(
                state.source, { state.source = it }, label = "Source", modifier = fieldModifier)
          },
          second = { fieldModifier ->
            CompactSingleLineField(
                state.severity,
                { state.severity = it },
                label = "Severity",
                modifier = fieldModifier)
          },
      )
      ResponsiveFieldPair(
          modifier = Modifier.fillMaxWidth().padding(top = 6.dp),
          minimumHorizontalWidth = 520.dp,
          first = { fieldModifier ->
            CompactSingleLineField(
                state.freshness,
                { state.freshness = it },
                label = "Freshness",
                modifier = fieldModifier)
          },
          second = { fieldModifier ->
            CompactSingleLineField(
                state.lifecycle,
                { state.lifecycle = it },
                label = "Lifecycle",
                modifier = fieldModifier)
          },
      )
    }
  }
}

internal fun findingsFilterStateLabel(activeFilters: List<String>): String? =
    activeFilters.size.takeIf { it > 0 }?.let { "$it active" }

@Composable
internal fun CompactProblemRow(
    finding: UnifiedFinding,
    actions: FindingActions,
    onShowDetails: (() -> Unit)? = null,
) {
  Column(
      Modifier.fillMaxWidth().padding(8.dp).semantics {
        contentDescription = compactProblemRowDescription(finding)
      }) {
        ResultRowContent(semanticResultRow(finding))
        FindingActionButtons(finding, actions, onShowDetails)
        IdeHorizontalSeparator(Modifier.padding(top = 8.dp))
      }
}

@Composable
internal fun FindingActionButtons(
    finding: UnifiedFinding,
    actions: FindingActions,
    onShowDetails: (() -> Unit)? = null,
) {
  ResponsiveActionGroup(Modifier.fillMaxWidth().padding(top = MiniOrcaSpacing.compact)) {
    onShowDetails?.let { onDetails ->
      MiniOrcaButton(
          onClick = onDetails, tone = ActionTone.Neutral, density = ButtonDensity.Toolbar) {
            Text("Details", fontSize = 11.sp)
          }
    }
    MiniOrcaButton(
        onClick = { actions.select(finding) },
        enabled = finding.location.path.isNotBlank(),
        tone = ActionTone.Navigation,
        density = ButtonDensity.Toolbar) {
          Text("Open source", fontSize = 11.sp)
        }
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

internal fun compactProblemRowDescription(finding: UnifiedFinding): String =
    "${finding.severity.ifBlank { "unknown" }} problem, ${findingStatusLabel(finding)}, ${findingProvenanceLabel(finding)}, ${findingLocationLabel(finding)}. ${finding.title.ifBlank { finding.message.ifBlank { "Untitled finding" } }}. Open source navigates only."

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
        findingProvenanceLabel(finding),
        findingStatusLabel(finding))

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
          IdeLabelBadge(row.state, SecondaryText)
        }
        Text(row.location, color = SelectionText, style = IdeTypography.resultCode)
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
            Modifier.weight(if (wide) 0.42f else 1f).fillMaxHeight()) {
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
                Column {
                  ChromeButton(
                      onClick = { onSelection(row.key) },
                      selected = row.key == selectedKey,
                      accessibleName = "Inspect ${row.title}",
                      modifier = Modifier.fillMaxWidth()) {
                        ResultRowContent(row)
                      }
                  IdeHorizontalSeparator()
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
                MiniOrcaButton(
                    onClick = { onSelection(null) },
                    tone = ActionTone.Neutral,
                    density = ButtonDensity.Toolbar) {
                      Text(if (wide) "Clear selection" else "Back to results")
                    }
                detail(selected.key)
              }
            }
      }
    }
  }
}

@Composable
internal fun ResultSectionHeader(page: AnalysisResultPageState, openAnalysis: () -> Unit) {
  IdePaneHeader(
      title = analysisCategoryLabel(page.category),
      stateLabel = page.statusLabel,
      stateTint = if (page.stale) Warning else SecondaryText,
      actions = {
        MiniOrcaButton(
            onClick = openAnalysis, tone = ActionTone.Navigation, density = ButtonDensity.Toolbar) {
              Text("View analysis")
            }
      })
  Column(
      Modifier.fillMaxWidth().padding(horizontal = 8.dp),
      verticalArrangement = androidx.compose.foundation.layout.Arrangement.spacedBy(4.dp)) {
        Text(
            "Reported findings: ${page.reportedCount?.toString() ?: "Not available"}",
            style = IdeTypography.resultLabel,
            color = PrimaryText)
        Text(
            analysisCoverageLabel(page.progress?.coverage),
            style = IdeTypography.compactBody,
            color = SecondaryText)
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
internal fun ResultPathFilter(path: String, onPath: (String) -> Unit) {
  ResponsiveFieldPair(
      Modifier.fillMaxWidth().padding(8.dp),
      minimumHorizontalWidth = 600.dp,
      first = { field -> CompactSingleLineField(path, onPath, "Filter by file path", field) },
      second = { field ->
        MiniOrcaButton(
            onClick = { onPath("") },
            enabled = path.isNotBlank(),
            tone = ActionTone.Neutral,
            modifier = field) {
              Text("All project files")
            }
      })
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
