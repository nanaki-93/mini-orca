package io.miniorca.desktop

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.layout.onSizeChanged
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.stateDescription
import androidx.compose.ui.semantics.toggleableState
import androidx.compose.ui.state.ToggleableState
import androidx.compose.ui.unit.dp

@Composable
@OptIn(ExperimentalLayoutApi::class)
internal fun AnalysisFileSelector(
    analysis: ProjectAnalysisRunState,
    actions: AnalysisWorkspaceActions
) = AnalysisFileSelector(analysis, actions, 320.dp)

@Composable
@OptIn(ExperimentalLayoutApi::class)
internal fun AnalysisFileSelector(
    analysis: ProjectAnalysisRunState,
    actions: AnalysisWorkspaceActions,
    tableHeight: androidx.compose.ui.unit.Dp,
    onChromeHeightChanged: (Int) -> Unit = {},
) {
  var panelHeightPx by remember { mutableStateOf(0) }
  var tableHeightPx by remember { mutableStateOf(0) }
  LaunchedEffect(panelHeightPx, tableHeightPx) {
    if (panelHeightPx > 0) onChromeHeightChanged(panelHeightPx - tableHeightPx)
  }
  val state = analysis.fileSelection
  val selection = state.selection
  var expanded by remember(selection?.projectId) { mutableStateOf(true) }
  var query by remember(selection?.projectId) { mutableStateOf("") }
  var filter by remember(selection?.projectId) { mutableStateOf(AnalysisFileFilter.All) }
  val ignored = selection?.excludedPaths.orEmpty().toSet()
  val rows =
      remember(selection, analysis.run) {
        selection?.let { analysisFileStatuses(it, analysis.run) }.orEmpty()
      }
  val eligible = selection?.files.orEmpty().filter { it.reason.isBlank() }
  val locked =
      selection?.editable == false ||
          analysis.run?.let { it.isActive() || it.status in setOf("paused", "interrupted") } == true
  val editable =
      selection?.editable == true &&
          !state.loading &&
          !state.saving &&
          analysis.action.isBlank() &&
          !locked
  val selectedCount = eligible.count { it.path !in ignored }
  val excludedCount = rows.count { it.status == AnalysisFileSyncStatus.Excluded }
  val files = filteredAnalysisFiles(rows, query, filter)
  val fontScale = LocalDensity.current.fontScale
  val panelModifier =
      Modifier.testTag("analysis-file-panel").onSizeChanged { panelHeightPx = it.height }
  WorkspaceSection(modifier = panelModifier) {
    BoxWithConstraints(Modifier.fillMaxWidth()) {
      val inline = maxWidth / fontScale >= 1050.dp
      Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
        Row(
            Modifier.fillMaxWidth(),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(16.dp)) {
              Box(Modifier.weight(1f)) {
                ChromeButton(
                    onClick = { expanded = !expanded },
                    accessibleName = "${if (expanded) "Collapse" else "Expand"} Files",
                    tooltip = null,
                    modifier =
                        Modifier.semantics {
                          stateDescription = if (expanded) "Expanded" else "Collapsed"
                        },
                    contentPadding = PaddingValues(0.dp)) {
                      DesktopLineIcon(
                          if (expanded) DesktopIcon.ChevronDown else DesktopIcon.ChevronRight,
                          "",
                          iconSize = 18.dp)
                      Column(
                          Modifier.padding(start = 12.dp),
                          verticalArrangement = Arrangement.spacedBy(4.dp)) {
                            Text(
                                "Files",
                                color = PrimaryText,
                                style = IdeTypography.workspaceHeading)
                            Text(
                                when {
                                  state.saving -> "Saving selection…"
                                  state.loading -> "Loading status…"
                                  selection == null -> "Not loaded"
                                  else -> "$selectedCount selected · $excludedCount excluded"
                                },
                                color = if (state.error == null) SecondaryText else Error,
                                style = IdeTypography.workspaceMetadata)
                          }
                    }
              }
              if (inline && expanded)
                  AnalysisFileFilters(rows, query, { query = it }, filter, { filter = it })
            }
        if (!inline && expanded)
            AnalysisFileFilters(rows, query, { query = it }, filter, { filter = it })
      }
    }
    state.error?.let { DiagnosticText(it, color = Error) }
    if (locked)
        Text(
            "Selection locked. Finish or cancel the current run to change files.",
            color = SecondaryText,
            style = IdeTypography.workspaceMetadata)
    if (expanded) {
      BoxWithConstraints(Modifier.fillMaxWidth()) {
        val wide = maxWidth / fontScale >= 720.dp
        Column {
          if (wide)
              Row(
                  Modifier.fillMaxWidth()
                      .background(HeaderSurface)
                      .padding(horizontal = 12.dp, vertical = 8.dp),
                  horizontalArrangement = Arrangement.spacedBy(AnalysisWideTableGrid.columnGap)) {
                    Row(Modifier.weight(AnalysisWideTableGrid.fileWeight)) {
                      Spacer(Modifier.width(AnalysisWideTableGrid.leadingControlsWidth))
                      Text(
                          "File",
                          color = SecondaryText,
                          style = IdeTypography.workspaceMetadata)
                    }
                    Text(
                        "Analysis state",
                        Modifier.weight(AnalysisWideTableGrid.stateWeight),
                        color = SecondaryText,
                        style = IdeTypography.workspaceMetadata)
                    Text(
                        "Details",
                        Modifier.weight(AnalysisWideTableGrid.detailsWeight),
                        color = SecondaryText,
                        style = IdeTypography.workspaceMetadata)
                  }
          if (files.isEmpty() && !state.loading)
              Text(
                  if (selection == null) "File status is not loaded. Refresh files to try again."
                  else "No matching files.",
                  Modifier.padding(vertical = 12.dp),
                  color = SecondaryText,
                  style = IdeTypography.workspaceMetadata)
          LazyColumn(
              Modifier.fillMaxWidth()
                  .height(tableHeight)
                  .testTag("analysis-file-table")
                  .onSizeChanged { tableHeightPx = it.height }) {
                itemsIndexed(files, key = { _, row -> row.file.path }) { _, row ->
                  AnalysisFileRow(
                      row,
                      row.file.reason.isBlank() && row.file.path !in ignored,
                      editable && row.file.reason.isBlank(),
                      wide) {
                        actions.saveSelection(
                            (if (row.file.path in ignored) ignored - row.file.path
                                else ignored + row.file.path)
                                .sorted())
                      }
                  IdeHorizontalSeparator()
                }
              }
        }
      }
      FlowRow(
          Modifier.fillMaxWidth().testTag("analysis-file-footer"),
          horizontalArrangement = Arrangement.spacedBy(12.dp),
          verticalArrangement = Arrangement.spacedBy(8.dp),
          itemVerticalAlignment = Alignment.CenterVertically) {
            if (selection != null)
                Text(
                    "${files.size} of ${rows.size} files match",
                    color = SecondaryText,
                    style = IdeTypography.workspaceMetadata)
            MiniOrcaButton(
                onClick = actions.refreshSelection,
                enabled = !state.loading && !state.saving,
                tone = ActionTone.Neutral) {
                  Text("Refresh files")
                }
            if (!locked) {
              MiniOrcaButton(
                  onClick = {
                    actions.saveSelection(ignored.minus(eligible.map { it.path }.toSet()).sorted())
                  },
                  enabled = editable,
                  tone = ActionTone.Neutral) {
                    Text("Select all")
                  }
              MiniOrcaButton(
                  onClick = {
                    actions.saveSelection((ignored + eligible.map { it.path }).sorted())
                  },
                  enabled = editable,
                  tone = ActionTone.Neutral) {
                    Text("Exclude all")
                  }
            }
          }
    }
  }
}

@Composable
@OptIn(ExperimentalLayoutApi::class)
private fun AnalysisFileFilters(
    rows: List<AnalysisFileStatus>,
    query: String,
    setQuery: (String) -> Unit,
    filter: AnalysisFileFilter,
    setFilter: (AnalysisFileFilter) -> Unit
) {
  val counts = remember(rows, query) { analysisFileFilterCounts(rows, query) }
  FlowRow(
      horizontalArrangement = Arrangement.spacedBy(8.dp),
      verticalArrangement = Arrangement.spacedBy(8.dp)) {
        CompactSingleLineField(query, setQuery, "Filter files", Modifier.width(220.dp))
        AnalysisFileFilter.entries.forEach { choice ->
          val count = counts[choice] ?: 0
          val selected = filter == choice
          ChromeTab(
              selected = selected,
              onClick = { setFilter(choice) },
              accessibleName = "${choice.label}, $count files") {
                Text(choice.label, style = IdeTypography.workspaceMetadata)
                Spacer(Modifier.width(6.dp))
                Text(
                    "$count",
                    color = if (selected) PrimaryText else SecondaryText,
                    style = IdeTypography.compactBody)
              }
        }
      }
}

private object AnalysisWideTableGrid {
  const val fileWeight = 0.44f
  const val stateWeight = 0.20f
  const val detailsWeight = 0.36f
  val checkboxWidth = 26.dp
  val documentWidth = 16.dp
  val identityGap = 8.dp
  val leadingControlsWidth = checkboxWidth + identityGap + documentWidth + identityGap
  val columnGap = 16.dp
}

@Composable
private fun AnalysisFileRow(
    row: AnalysisFileStatus,
    selected: Boolean,
    editable: Boolean,
    wide: Boolean,
    toggle: () -> Unit
) {
  val tint = row.status.tint
  Row(
      Modifier.fillMaxWidth()
          .testTag("analysis-file-row-${row.file.path}")
          .clip(MiniOrcaShapes.control)
          .background(
              if (row.savedStatus != null && row.status == AnalysisFileSyncStatus.Running)
                  SelectionSurface
              else Panel)
          .padding(horizontal = 12.dp, vertical = 4.dp),
      horizontalArrangement = Arrangement.spacedBy(AnalysisWideTableGrid.columnGap),
      verticalAlignment = Alignment.Top) {
        val identity: @Composable (Modifier) -> Unit = { modifier ->
          Row(
              modifier,
              horizontalArrangement = Arrangement.spacedBy(AnalysisWideTableGrid.identityGap),
              verticalAlignment = Alignment.Top) {
                AnalysisFileCheckbox(row.file.path, selected, editable, toggle)
                DesktopLineIcon(DesktopIcon.Document, "", iconSize = 16.dp, tint = SecondaryText)
                SelectionContainer {
                  Text(row.file.path, style = IdeTypography.workspaceBody, color = PrimaryText)
                }
              }
        }
        if (wide) {
          identity(Modifier.weight(AnalysisWideTableGrid.fileWeight))
          Text(
              row.status.label,
              Modifier.weight(AnalysisWideTableGrid.stateWeight),
              color = tint,
              style = IdeTypography.workspaceBody)
          AnalysisFileDetails(row, Modifier.weight(AnalysisWideTableGrid.detailsWeight))
        } else {
          Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
            identity(Modifier.fillMaxWidth())
            Text(row.status.label, color = tint, style = IdeTypography.workspaceBody)
            AnalysisFileDetails(row)
          }
        }
      }
}

@Composable
private fun AnalysisFileDetails(row: AnalysisFileStatus, modifier: Modifier = Modifier) {
  var expanded by remember(row.file.path, row.explanation) { mutableStateOf(false) }
  val summary = row.summary
  val detail =
      row.explanation +
          (row.savedStatus
              ?.let { "\nSaved analysis: ${it.label}\n" + analysisFileStatus(row.file).explanation }
              .orEmpty())
  val hasDetails =
      detail != summary &&
          row.status !in setOf(AnalysisFileSyncStatus.Updated, AnalysisFileSyncStatus.Excluded)
  Column(modifier, verticalArrangement = Arrangement.spacedBy(4.dp)) {
    Row(
        horizontalArrangement = Arrangement.spacedBy(8.dp),
        verticalAlignment = Alignment.CenterVertically) {
          SelectionContainer(Modifier.weight(1f)) {
            Text(
                sanitizedOutputText(summary),
                color =
                    if (row.status == AnalysisFileSyncStatus.Running) Information
                    else SecondaryText,
                style = IdeTypography.workspaceBody)
          }
          if (hasDetails)
              ChromeButton(
                  onClick = { expanded = !expanded },
                  accessibleName = "Analysis details for ${row.file.path}",
                  modifier =
                      Modifier.semantics {
                        stateDescription = if (expanded) "Expanded" else "Collapsed"
                      },
                  contentPadding = PaddingValues(horizontal = 4.dp)) {
                    Text(
                        if (expanded) "Hide details" else "Details",
                        style = IdeTypography.workspaceMetadata)
                  }
        }
    row.savedStatus
        ?.takeIf {
          it !in
              setOf(
                  AnalysisFileSyncStatus.Missing,
                  AnalysisFileSyncStatus.Updated,
                  AnalysisFileSyncStatus.Running,
                  AnalysisFileSyncStatus.Pending)
        }
        ?.let {
          Text(
              "Saved analysis: ${it.label.lowercase()}",
              color = Warning,
              style = IdeTypography.workspaceMetadata)
        }
    if (hasDetails && expanded) DiagnosticText(detail)
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
          Modifier.semantics {
            toggleableState = ToggleableState(selected)
            stateDescription = if (selected) "Selected for analysis" else "Excluded from analysis"
          }) {
        Box(
            Modifier.size(18.dp)
                .background(
                    if (selected) SelectionSurface else EditorCanvas, MiniOrcaShapes.indicator)
                .border(
                    1.dp, if (selected) SelectionText else ControlBorder, MiniOrcaShapes.indicator),
            contentAlignment = Alignment.Center) {
              if (selected)
                  DesktopLineIcon(
                      DesktopIcon.Check, "Selected", Modifier.size(14.dp), tint = SelectionText)
            }
      }
}
