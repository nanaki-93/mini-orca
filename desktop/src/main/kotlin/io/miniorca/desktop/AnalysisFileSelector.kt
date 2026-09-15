package io.miniorca.desktop

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.testTag
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
  var filter by remember(selection?.projectId) { mutableStateOf(AnalysisFileFilter.All) }
  val ignored = selection?.excludedPaths.orEmpty().toSet()
  val rows =
      remember(selection) {
        selection?.files.orEmpty().map { analysisFileStatus(it, it.path in ignored) }
      }
  val eligible = selection?.files.orEmpty().filter { it.reason.isBlank() }
  val editable =
      selection?.editable == true &&
          !state.loading &&
          !state.saving &&
          analysis.action.isBlank() &&
          analysis.run?.isActive() != true
  val excludedCount = rows.count { it.status == AnalysisFileSyncStatus.Excluded }
  val selectedCount = eligible.count { it.path !in ignored }
  Column(Modifier.fillMaxWidth().testTag("analysis-file-panel")) {
    IdeDisclosureHeader(
        "Files",
        expanded,
        { expanded = !expanded },
        stateLabel =
            when {
              state.saving -> "Saving selection…"
              state.loading -> "Loading status…"
              selection == null -> "Not loaded"
              else -> "$selectedCount selected · $excludedCount excluded"
            },
        stateTint = if (state.error == null) Information else Error)
    state.error?.let { DiagnosticText(it, color = Error) }
    if (selection?.editable == false || analysis.run?.isActive() == true)
        Text(
            "Finish or cancel the current run to change selection.",
            Modifier.padding(horizontal = 8.dp, vertical = 4.dp),
            color = Warning,
            style = IdeTypography.compactBody)
    if (!expanded) return@Column
    Column(Modifier.padding(horizontal = 8.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
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
              Text("Exclude all")
            }
      }
      CompactSingleLineField(query, { query = it }, "Filter files", Modifier.fillMaxWidth())
      ResponsiveActionGroup(Modifier.fillMaxWidth()) {
        AnalysisFileFilter.entries.forEach { choice ->
          ChromeTab(
              selected = filter == choice,
              onClick = { filter = choice },
              accessibleName = choice.label) {
                Text(choice.label, style = IdeTypography.compactBody)
              }
        }
      }
    }
    val files = filteredAnalysisFiles(rows, query, filter)
    if (files.isEmpty() && !state.loading)
        Text(
            if (selection == null) "File status is not loaded. Refresh files to try again."
            else "No matching files.",
            Modifier.padding(8.dp),
            color = SecondaryText,
            style = IdeTypography.compactBody)
    LazyColumn(Modifier.fillMaxWidth().heightIn(max = 400.dp)) {
      itemsIndexed(files, key = { _, row -> row.file.path }) { index, row ->
        AnalysisFileRow(
            row,
            row.file.reason.isBlank() && row.file.path !in ignored,
            editable && row.file.reason.isBlank(),
            index % 2 == 0) {
              actions.saveSelection(
                  (if (row.file.path in ignored) ignored - row.file.path
                      else ignored + row.file.path)
                      .sorted())
            }
      }
    }
  }
}

@Composable
private fun AnalysisFileRow(
    row: AnalysisFileStatus,
    selected: Boolean,
    editable: Boolean,
    alternate: Boolean,
    toggle: () -> Unit
) {
  val tint = row.status.tint
  Row(
      Modifier.fillMaxWidth()
          .background(if (alternate) EditorCanvas else Panel)
          .padding(end = 8.dp),
      horizontalArrangement = Arrangement.spacedBy(8.dp)) {
        Box(Modifier.width(3.dp).heightIn(min = 40.dp).background(tint))
        AnalysisFileCheckbox(row.file.path, selected, editable, toggle)
        Column(
            Modifier.weight(1f).padding(vertical = 8.dp),
            verticalArrangement = Arrangement.spacedBy(4.dp)) {
              Row(
                  Modifier.fillMaxWidth(),
                  verticalAlignment = Alignment.Top,
                  horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    Text(
                        row.file.path,
                        Modifier.weight(1f),
                        style = IdeTypography.resultCode,
                        color = PrimaryText)
                    IdeLabelBadge(row.status.label, tint)
                  }
              if (row.status != AnalysisFileSyncStatus.Updated) {
                val reason =
                    if (row.status == AnalysisFileSyncStatus.Excluded) row.explanation
                    else
                        row.file.stages
                            .firstOrNull { it.status !in setOf("fresh", "skipped") }
                            ?.reason
                            ?.takeIf { it.isNotBlank() } ?: row.explanation
                Text(reason, color = SecondaryText, style = IdeTypography.compactBody)
              }
            }
      }
}

@Composable
private fun AnalysisFileCheckbox(
    path: String,
    selected: Boolean,
    enabled: Boolean,
    toggle: () -> Unit
) {
  ChromeButton(
      onClick = toggle,
      enabled = enabled,
      role = Role.Checkbox,
      accessibleName = "Analyze $path",
      contentPadding = PaddingValues(4.dp),
      modifier =
          Modifier.padding(top = 8.dp).semantics {
            toggleableState = ToggleableState(selected)
            stateDescription = if (selected) "Included in next run" else "Not included in next run"
          }) {
        Box(
            Modifier.size(18.dp)
                .background(if (selected) SelectionSurface else EditorCanvas)
                .border(1.dp, if (selected) SelectionText else ControlBorder),
            contentAlignment = Alignment.Center) {
              if (selected)
                  DesktopLineIcon(
                      DesktopIcon.Check, "Selected", Modifier.size(14.dp), tint = SelectionText)
            }
      }
}
