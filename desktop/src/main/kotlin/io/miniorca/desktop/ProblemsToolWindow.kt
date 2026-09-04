package io.miniorca.desktop

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.material.Divider
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.style.TextOverflow
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
  var selectedKey by remember { mutableStateOf<String?>(null) }
  val findings = presentation.priorityGroups.flatMap { it.findings }
  Column(modifier.fillMaxSize().padding(horizontal = 12.dp, vertical = 8.dp)) {
    FindingsFilterControls(filters, presentation, Modifier.fillMaxWidth())
    Spacer(Modifier.height(8.dp))
    Row(Modifier.fillMaxWidth().padding(horizontal = 6.dp, vertical = 6.dp)) {
      ProblemTableCells(
          "Severity",
          "File / line",
          "Description / source",
          "Status",
          severityColor = SecondaryText,
          contentColor = SecondaryText)
    }
    Divider(color = Border)
    LazyColumn(Modifier.fillMaxWidth().weight(1f)) {
      if (findings.isEmpty()) {
        item {
          Text(
              presentation.emptyMessage,
              color = SecondaryText,
              fontSize = 12.sp,
              modifier = Modifier.padding(vertical = 12.dp))
        }
      }
      findings.forEach { finding ->
        val key = findingDisplayKey(finding)
        item(key = key) {
          ChromeButton(
              onClick = { selectedKey = if (selectedKey == key) null else key },
              selected = selectedKey == key,
              contentPadding = PaddingValues(horizontal = 6.dp, vertical = 8.dp),
              modifier =
                  Modifier.fillMaxWidth().semantics {
                    contentDescription =
                        compactProblemRowDescription(finding) + ". Select to show or hide details."
                  }) {
                ProblemTableCells(
                    finding.severity.ifBlank { "Unknown" }.replaceFirstChar { it.uppercase() },
                    findingLocationLabel(finding),
                    finding.title.ifBlank { finding.message.ifBlank { "Untitled finding" } },
                    findingStatusLabel(finding),
                    provenance = findingProvenanceLabel(finding),
                    severityColor =
                        when (findingPriority(finding)) {
                          FindingPriority.High -> Error
                          FindingPriority.Medium -> Warning
                          FindingPriority.Low -> SelectionText
                          FindingPriority.Other -> SecondaryText
                        })
              }
          Divider(color = Border.copy(alpha = 0.35f))
          if (selectedKey == key) CompactProblemRow(finding, actions)
        }
      }
    }
  }
}

@Composable
private fun RowScope.ProblemTableCells(
    severity: String,
    location: String,
    description: String,
    status: String,
    provenance: String? = null,
    severityColor: Color,
    contentColor: Color = PrimaryText,
) {
  Text(
      severity,
      color = severityColor,
      fontSize = 11.sp,
      maxLines = 1,
      overflow = TextOverflow.Ellipsis,
      modifier = Modifier.width(68.dp))
  Text(
      location,
      color = SecondaryText,
      fontSize = 11.sp,
      maxLines = 1,
      overflow = TextOverflow.Ellipsis,
      modifier = Modifier.weight(0.36f).padding(end = 12.dp))
  Column(Modifier.weight(0.64f).padding(end = 12.dp)) {
    Text(
        description,
        color = contentColor,
        fontSize = 11.sp,
        maxLines = 1,
        overflow = TextOverflow.Ellipsis)
    provenance?.let {
      Text(
          it,
          color = SecondaryText,
          fontSize = 10.sp,
          maxLines = 1,
          overflow = TextOverflow.Ellipsis,
          modifier = Modifier.padding(top = 3.dp))
    }
  }
  Text(
      status,
      color = SecondaryText,
      fontSize = 11.sp,
      maxLines = 1,
      overflow = TextOverflow.Ellipsis,
      modifier = Modifier.width(90.dp))
}
