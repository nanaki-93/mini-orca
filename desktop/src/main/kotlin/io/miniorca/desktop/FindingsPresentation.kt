package io.miniorca.desktop

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
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
        label = { Text("Search findings") },
        modifier = Modifier.fillMaxWidth())
    MiniOrcaButton(
        onClick = { state.advancedFiltersVisible = !state.advancedFiltersVisible },
        tone = ActionTone.Neutral,
        selected = state.advancedFiltersVisible,
        modifier = Modifier.padding(top = 6.dp)) {
          Text(if (state.advancedFiltersVisible) "Hide filters" else "Filters")
        }
    if (presentation.activeFilters.isNotEmpty())
        Text(
            "Filters active: ${presentation.activeFilters.joinToString(" · ")}",
            color = SecondaryText,
            fontSize = 11.sp,
            modifier = Modifier.padding(top = 5.dp))
    if (state.advancedFiltersVisible) {
      ResponsiveFieldPair(
          modifier = Modifier.fillMaxWidth().padding(top = 6.dp),
          first = { fieldModifier ->
            CompactSingleLineField(
                state.source,
                { state.source = it },
                label = { Text("Source") },
                modifier = fieldModifier)
          },
          second = { fieldModifier ->
            CompactSingleLineField(
                state.severity,
                { state.severity = it },
                label = { Text("Severity") },
                modifier = fieldModifier)
          },
      )
      ResponsiveFieldPair(
          modifier = Modifier.fillMaxWidth().padding(top = 6.dp),
          first = { fieldModifier ->
            CompactSingleLineField(
                state.freshness,
                { state.freshness = it },
                label = { Text("Freshness") },
                modifier = fieldModifier)
          },
          second = { fieldModifier ->
            CompactSingleLineField(
                state.lifecycle,
                { state.lifecycle = it },
                label = { Text("Lifecycle") },
                modifier = fieldModifier)
          },
      )
    }
  }
}

@Composable
internal fun CompactProblemRow(
    finding: UnifiedFinding,
    actions: FindingActions,
    onShowDetails: (() -> Unit)? = null,
) {
  MiniOrcaPanel(
      Modifier.fillMaxWidth().padding(top = MiniOrcaSpacing.compact).semantics {
        contentDescription = compactProblemRowDescription(finding)
      },
      contentPadding = PaddingValues(MiniOrcaSpacing.standard)) {
        Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
          Text(
              finding.severity.ifBlank { "unknown" }.uppercase(),
              color = PrimaryText,
              fontSize = 11.sp,
              fontWeight = FontWeight.Bold)
          Text(" · ${findingStatusLabel(finding)}", color = SecondaryText, fontSize = 11.sp)
          Text(
              finding.title.ifBlank { "Untitled finding" },
              color = PrimaryText,
              fontSize = 12.sp,
              fontWeight = FontWeight.SemiBold,
              modifier = Modifier.weight(1f).padding(start = 8.dp))
        }
        Text(
            findingProvenanceLabel(finding),
            color = SecondaryText,
            fontSize = 10.sp,
            modifier = Modifier.padding(top = MiniOrcaSpacing.compact))
        Text(
            findingLocationLabel(finding),
            color = SecondaryText,
            fontSize = 10.sp,
            modifier = Modifier.padding(top = MiniOrcaSpacing.compact))
        Text(
            finding.message.ifBlank { "No summary supplied." },
            color = PrimaryText,
            fontSize = 11.sp,
            maxLines = 2,
            modifier = Modifier.padding(top = MiniOrcaSpacing.compact))
        FindingActionButtons(finding, actions, onShowDetails)
      }
}

@Composable
private fun FindingActionButtons(
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
