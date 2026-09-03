package io.miniorca.desktop

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

/** Bottom-pane inputs are a read-only view of the project-bound finding snapshot. */
internal data class ProblemsToolWindowState(
    val findings: List<UnifiedFinding>,
    val loading: Boolean,
)

@Composable
internal fun ProblemsToolWindow(
    state: ProblemsToolWindowState,
    actions: FindingActions,
    modifier: Modifier = Modifier,
) {
  val filters = rememberFindingsFilterState()
  val presentation = findingsPresentation(state.findings, filters.filters, state.loading)
  Column(modifier.fillMaxSize().padding(horizontal = 12.dp, vertical = 8.dp)) {
    FocusFlowPanel(Modifier.fillMaxWidth(), raised = true) {
      Text("PROBLEMS", color = PrimaryText, fontSize = 11.sp, fontWeight = FontWeight.Bold)
      FindingsFilterControls(
          filters, presentation, modifier = Modifier.fillMaxWidth().padding(top = 6.dp))
    }
    Spacer(Modifier.height(6.dp))
    LazyColumn(Modifier.fillMaxWidth().weight(1f)) {
      if (presentation.priorityGroups.isEmpty()) {
        item {
          Text(
              presentation.emptyMessage,
              color = SecondaryText,
              fontSize = 12.sp,
              modifier = Modifier.padding(top = 7.dp))
        }
      } else {
        presentation.priorityGroups.forEach { group ->
          item { SectionLabel(group.priority.sectionLabel, Modifier.padding(top = 5.dp)) }
          items(
              group.findings,
              key = { finding ->
                "${group.priority.name}:${finding.id}:${finding.location.path}:${finding.location.startLine}"
              }) { finding ->
                CompactProblemRow(finding, actions)
              }
        }
      }
    }
  }
}
