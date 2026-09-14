package io.miniorca.desktop

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.stateDescription
import androidx.compose.ui.semantics.toggleableState
import androidx.compose.ui.state.ToggleableState
import androidx.compose.ui.unit.dp

@Composable
internal fun AnalysisFileSelector(
    analysis: ProjectAnalysisRunState,
    actions: AnalysisWorkspaceActions
) {
  val state = analysis.fileSelection
  val selection = state.selection
  var expanded by remember(selection?.projectId) { mutableStateOf(false) }
  var query by remember(selection?.projectId) { mutableStateOf("") }
  val ignored = selection?.excludedPaths.orEmpty().toSet()
  val eligible = selection?.files.orEmpty().filter { it.reason.isBlank() }
  val selectedCount = eligible.count { it.path !in ignored }
  val editable =
      selection?.editable == true &&
          !state.loading &&
          !state.saving &&
          analysis.action.isBlank() &&
          analysis.run?.isActive() != true
  Column(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(4.dp)) {
    IdeDisclosureHeader(
        "Files to analyze",
        expanded,
        { expanded = !expanded },
        stateLabel =
            when {
              state.loading -> "Loading…"
              state.saving -> "Saving…"
              selection == null -> "Not loaded"
              else -> "$selectedCount selected · ${ignored.size} ignored"
            })
    state.error?.let { DiagnosticText(it, color = Error) }
    if (expanded) {
      Text(
          "Selections are saved for this project. New files are selected automatically. Build, dependency and metadata folders are omitted.",
          style = IdeTypography.compactBody,
          color = SecondaryText)
      if (selection?.editable == false || analysis.run?.isActive() == true)
          Text(
              "Finish or cancel the current run before changing its files.",
              style = IdeTypography.compactBody,
              color = Warning)
      ResponsiveActionGroup(Modifier.fillMaxWidth()) {
        MiniOrcaButton(
            onClick = actions.refreshSelection,
            enabled = !state.loading && !state.saving,
            tone = ActionTone.Neutral) {
              Text("Refresh files")
            }
        MiniOrcaButton(
            onClick = {
              actions.saveSelection(ignored.minus(eligible.map { it.path }.toSet()).sorted())
            },
            enabled = editable,
            tone = ActionTone.Neutral) {
              Text("Select all")
            }
        MiniOrcaButton(
            onClick = { actions.saveSelection((ignored + eligible.map { it.path }).sorted()) },
            enabled = editable,
            tone = ActionTone.Neutral) {
              Text("Ignore all")
            }
      }
      CompactSingleLineField(query, { query = it }, "Filter files", Modifier.fillMaxWidth())
      val files = selection?.files.orEmpty().filter { it.path.contains(query, ignoreCase = true) }
      if (files.isEmpty() && !state.loading)
          Text(
              if (selection == null) "Load files to choose the analysis scope."
              else "No matching files.",
              style = IdeTypography.compactBody,
              color = SecondaryText)
      LazyColumn(Modifier.fillMaxWidth().heightIn(max = 280.dp)) {
        items(files, key = { it.path }) { file ->
          val selected = file.reason.isBlank() && file.path !in ignored
          ChromeButton(
              onClick = {
                actions.saveSelection(
                    (if (selected) ignored + file.path else ignored - file.path).sorted())
              },
              enabled = editable && file.reason.isBlank(),
              selected = selected,
              role = Role.Checkbox,
              accessibleName = "Analyze ${file.path}",
              modifier =
                  Modifier.fillMaxWidth().semantics {
                    toggleableState = ToggleableState(selected)
                    stateDescription =
                        if (selected) "Selected"
                        else if (file.reason.isBlank()) "Ignored" else file.reason
                  }) {
                Text(
                    if (selected) "✓" else "□",
                    color = if (selected) SelectionText else SecondaryText)
                Spacer(Modifier.width(8.dp))
                Column(Modifier.weight(1f).padding(vertical = 4.dp)) {
                  Text(file.path, style = IdeTypography.resultCode, color = PrimaryText)
                  if (file.reason.isNotBlank())
                      Text(file.reason, style = IdeTypography.compactBody, color = SecondaryText)
                }
              }
        }
      }
    }
  }
}
