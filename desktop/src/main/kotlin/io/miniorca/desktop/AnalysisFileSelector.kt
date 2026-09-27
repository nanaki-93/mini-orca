package io.miniorca.desktop

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
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
import androidx.compose.foundation.lazy.LazyListState
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateMapOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.runtime.snapshotFlow
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.layout.onSizeChanged
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.stateDescription
import androidx.compose.ui.semantics.testTag
import androidx.compose.ui.unit.dp
import kotlinx.coroutines.flow.collect
import kotlinx.coroutines.flow.distinctUntilChanged

internal class AnalysisFilesViewState {
  var expanded by mutableStateOf(true)
  var query by mutableStateOf("")
  var filter by mutableStateOf(AnalysisFileFilter.All)
  val listState = LazyListState()
  val expandedDetails = mutableStateMapOf<String, Boolean>()
  private var anchorPath: String? = null
  private var anchorOffset = 0
  private var previousPaths: List<String>? = null
  private var pendingAnchor: Pair<String?, Int>? = null

  fun preparePaths(paths: List<String>) {
    if (previousPaths != null && previousPaths != paths) {
      // Capture the old keyed layout before the new list is measured at the old numeric index.
      val visible = listState.layoutInfo.visibleItemsInfo.firstOrNull()?.key as? String
      pendingAnchor =
          if (visible != null && previousPaths?.contains(visible) == true)
              visible to listState.firstVisibleItemScrollOffset
          else anchorPath to anchorOffset
    }
  }

  fun retainInventory(paths: Set<String>) {
    expandedDetails.keys.retainAll(paths)
  }

  // An index alone would transfer the old row's position to a different file after refresh.
  suspend fun followPaths(paths: List<String>) {
    if (previousPaths != null && previousPaths != paths) {
      val (path, offset) = pendingAnchor ?: (anchorPath to anchorOffset)
      val index = path?.let(paths::indexOf)?.takeIf { it >= 0 } ?: 0
      listState.requestScrollToItem(index, if (path in paths) offset else 0)
    }
    pendingAnchor = null
    previousPaths = paths
    snapshotFlow { listState.firstVisibleItemIndex to listState.firstVisibleItemScrollOffset }
        .distinctUntilChanged()
        .collect { (visibleIndex, offset) ->
          anchorPath = paths.getOrNull(visibleIndex)
          anchorOffset = if (anchorPath == null) 0 else offset
        }
  }
}

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
    view: AnalysisFilesViewState = remember { AnalysisFilesViewState() },
) {
  var panelHeightPx by remember { mutableStateOf(0) }
  var tableHeightPx by remember { mutableStateOf(0) }
  LaunchedEffect(panelHeightPx, tableHeightPx) {
    if (panelHeightPx > 0) onChromeHeightChanged(panelHeightPx - tableHeightPx)
  }
  val state = analysis.fileSelection
  val selection = state.selection
  val expanded = view.expanded
  val query = view.query
  val filter = view.filter
  val ignored = selection?.excludedPaths.orEmpty().toSet()
  val rows =
      remember(selection, analysis.run) {
        selection?.let { analysisFileStatuses(it, analysis.run) }.orEmpty()
      }
  val eligible = selection?.files.orEmpty().filter { it.reason.isBlank() }
  val lockedByRun = analysis.run?.showsProgressOnSummary() == true
  val locked = selection?.editable == false || lockedByRun
  val editable = canEditAnalysisSelection(analysis)
  val selectedCount = eligible.count { it.path !in ignored }
  val excludedCount = rows.count { it.status == AnalysisFileSyncStatus.Excluded }
  val files = filteredAnalysisFiles(rows, query, filter)
  val paths = files.map { it.file.path }
  LaunchedEffect(view, selection?.files) {
    if (selection != null) view.retainInventory(selection.files.map { it.path }.toSet())
  }
  val panelModifier =
      Modifier.testTag("analysis-file-panel").onSizeChanged { panelHeightPx = it.height }
  WorkspaceSection(modifier = panelModifier) {
    Column(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(12.dp)) {
      FlowRow(
          Modifier.fillMaxWidth(),
          horizontalArrangement = Arrangement.spacedBy(16.dp),
          verticalArrangement = Arrangement.spacedBy(8.dp),
          itemVerticalAlignment = Alignment.CenterVertically) {
            ChromeButton(
                onClick = { view.expanded = !expanded },
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
                        Text("Files", color = PrimaryText, style = IdeTypography.workspaceHeading)
                        Text(
                            when {
                              selection != null ->
                                  "$selectedCount selected · $excludedCount excluded"
                              state.loading -> "Loading status…"
                              else -> "Not loaded"
                            } +
                                when (state.failure) {
                                  AnalysisSelectionFailure.Read -> " · Refresh failed"
                                  AnalysisSelectionFailure.Save -> " · Save failed"
                                  null ->
                                      when {
                                        state.saving -> " · Saving selection…"
                                        state.loading && selection != null -> " · Refreshing…"
                                        state.error != null -> " · Selection error"
                                        else -> ""
                                      }
                                },
                            color = if (state.error == null) SecondaryText else Error,
                            style = IdeTypography.workspaceMetadata)
                      }
                }
            if (state.error == null)
                MiniOrcaButton(
                    onClick = actions.refreshSelection,
                    enabled = !state.loading && !state.saving,
                    tone = ActionTone.Neutral) {
                      Text("Refresh files")
                    }
          }
      Text(
          "File selection is independent of the open Editor file and does not start analysis.",
          color = SecondaryText,
          style = IdeTypography.workspaceMetadata)
    }
    state.error?.let { error ->
      FlowRow(
          Modifier.fillMaxWidth(),
          horizontalArrangement = Arrangement.spacedBy(12.dp),
          verticalArrangement = Arrangement.spacedBy(4.dp),
          itemVerticalAlignment = Alignment.CenterVertically) {
            Text(
                when (state.failure) {
                  AnalysisSelectionFailure.Read -> "Could not refresh file selection"
                  AnalysisSelectionFailure.Save -> "Could not save file selection"
                  null -> "File selection unavailable"
                },
                color = Error,
                style = IdeTypography.workspaceMetadata)
            MiniOrcaButton(
                onClick = actions.refreshSelection,
                enabled = !state.loading && !state.saving,
                tone = ActionTone.Neutral) {
                  Text("Refresh files")
                }
          }
      Text(
          "${if (selection == null) "No confirmed selection is loaded." else "The last confirmed selection is still shown."} Refresh files reads the saved selection; it does not retry a failed change or start analysis.",
          color = SecondaryText,
          style = IdeTypography.workspaceMetadata)
      DiagnosticText(error, color = Error)
    }
    if (locked)
        Row(
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(6.dp),
        ) {
          DesktopLineIcon(DesktopIcon.Lock, "", iconSize = 15.dp, tint = SecondaryText)
          Text(
              if (lockedByRun) "Selection locked. Finish or cancel the current run to change files."
              else "Selection changes unavailable.",
              color = SecondaryText,
              style = IdeTypography.workspaceMetadata)
        }
    if (expanded) {
      AnalysisFileFilters(
          rows, query, { view.query = it }, filter, { view.filter = it }, selection != null)
      if (selection != null)
          Text(
              "${files.size} of ${rows.size} files match",
              color = SecondaryText,
              style = IdeTypography.workspaceMetadata)
      Text(
          "Select all and Exclude all affect every eligible file, regardless of search or filter matches.",
          color = SecondaryText,
          style = IdeTypography.workspaceMetadata)
      if (selection != null && eligible.isNotEmpty() && selectedCount == 0)
          Text(
              "All eligible files are excluded; no files are selected for analysis.",
              color = SecondaryText,
              style = IdeTypography.workspaceMetadata)
      remember(view, paths) { view.preparePaths(paths) }
      LaunchedEffect(view, paths) { view.followPaths(paths) }
      Column(Modifier.fillMaxWidth()) {
        Row(
            Modifier.fillMaxWidth()
                .background(HeaderSurface)
                .padding(horizontal = 12.dp, vertical = 8.dp),
            horizontalArrangement = Arrangement.spacedBy(AnalysisWideTableGrid.columnGap)) {
              Row(Modifier.weight(AnalysisWideTableGrid.fileWeight)) {
                Spacer(Modifier.width(AnalysisWideTableGrid.leadingControlsWidth))
                Text("File", color = SecondaryText, style = IdeTypography.workspaceMetadata)
              }
              Text(
                  "Analysis state",
                  Modifier.weight(AnalysisWideTableGrid.stateWeight)
                      .padding(start = AnalysisWideTableGrid.statusMarkerWidth),
                  color = SecondaryText,
                  style = IdeTypography.workspaceMetadata)
              Text(
                  "Details",
                  Modifier.weight(AnalysisWideTableGrid.detailsWeight),
                  color = SecondaryText,
                  style = IdeTypography.workspaceMetadata)
            }
        if (files.isEmpty())
            Text(
                when {
                  selection == null && state.loading -> "Loading file selection…"
                  selection == null -> "File status is not loaded. Refresh files to try again."
                  rows.isEmpty() -> "No files are available for analysis."
                  else -> "No matching files."
                },
                Modifier.padding(vertical = 12.dp),
                color = SecondaryText,
                style = IdeTypography.workspaceMetadata)
        LazyColumn(
            state = view.listState,
            modifier =
                Modifier.fillMaxWidth()
                    .height(tableHeight)
                    .testTag("analysis-file-table")
                    .onSizeChanged { tableHeightPx = it.height }) {
              itemsIndexed(files, key = { _, row -> row.file.path }) { _, row ->
                AnalysisFileRow(
                    row,
                    row.file.reason.isBlank() && row.file.path !in ignored,
                    editable && row.file.reason.isBlank(),
                    view) {
                      actions.saveSelection(
                          (if (row.file.path in ignored) ignored - row.file.path
                              else ignored + row.file.path)
                              .sorted())
                    }
                IdeHorizontalSeparator()
              }
            }
      }
      FlowRow(
          Modifier.fillMaxWidth().testTag("analysis-file-footer"),
          horizontalArrangement = Arrangement.spacedBy(12.dp),
          verticalArrangement = Arrangement.spacedBy(8.dp),
          itemVerticalAlignment = Alignment.CenterVertically) {
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
    setFilter: (AnalysisFileFilter) -> Unit,
    confirmed: Boolean
) {
  val counts = remember(rows, query) { analysisFileFilterCounts(rows, query) }
  FlowRow(
      horizontalArrangement = Arrangement.spacedBy(8.dp),
      verticalArrangement = Arrangement.spacedBy(8.dp)) {
        CompactSingleLineField(
            query,
            setQuery,
            "Filter files",
            Modifier.width(220.dp),
            placeholder = "Search file paths")
        AnalysisFileFilter.entries.forEach { choice ->
          val count = counts[choice] ?: 0
          val selected = filter == choice
          ChromeTab(
              selected = selected,
              onClick = { setFilter(choice) },
              accessibleName = choice.label,
              modifier = Modifier.semantics { this.selected = selected }) {
                Text(choice.label, style = IdeTypography.action)
                if (confirmed) {
                  Spacer(Modifier.width(6.dp))
                  Text(
                      "$count",
                      color = if (selected) PrimaryText else SecondaryText,
                      style = IdeTypography.compactBody)
                }
              }
        }
      }
}

private object AnalysisWideTableGrid {
  const val fileWeight = 0.44f
  const val stateWeight = 0.20f
  const val detailsWeight = 0.36f
  val checkboxWidth = 34.dp
  val documentWidth = 16.dp
  val identityGap = 8.dp
  val leadingControlsWidth = checkboxWidth + identityGap + documentWidth + identityGap
  val statusMarkerWidth = 14.dp
  val columnGap = 16.dp
}

@Composable
private fun AnalysisFileRow(
    row: AnalysisFileStatus,
    selected: Boolean,
    editable: Boolean,
    view: AnalysisFilesViewState,
    toggle: () -> Unit
) {
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
                IdeCheckbox(
                    checked = selected,
                    onCheckedChange = { toggle() },
                    accessibleName = "Analyze ${row.file.path}",
                    enabled = editable,
                    stateLabel =
                        if (selected) "Selected for analysis" else "Excluded from analysis")
                DesktopLineIcon(DesktopIcon.Document, "", iconSize = 16.dp, tint = SecondaryText)
                SelectionContainer {
                  Text(row.file.path, style = IdeTypography.workspaceMetadata, color = PrimaryText)
                }
              }
        }
        identity(Modifier.weight(AnalysisWideTableGrid.fileWeight))
        AnalysisFileStatusLabel(
            row.file.path, row.status, Modifier.weight(AnalysisWideTableGrid.stateWeight))
        AnalysisFileDetails(row, view, Modifier.weight(AnalysisWideTableGrid.detailsWeight))
      }
}

@Composable
private fun AnalysisFileStatusLabel(
    path: String,
    status: AnalysisFileSyncStatus,
    modifier: Modifier = Modifier,
) {
  Row(
      modifier,
      horizontalArrangement = Arrangement.spacedBy(6.dp),
      verticalAlignment = Alignment.CenterVertically) {
        Box(
            Modifier.size(8.dp)
                .clip(MiniOrcaShapes.indicator)
                .background(status.tint)
                .clearAndSetSemantics { testTag = "analysis-file-status-marker-$path" })
        Text(status.label, color = status.tint, style = IdeTypography.workspaceBody)
      }
}

@Composable
private fun AnalysisFileDetails(
    row: AnalysisFileStatus,
    view: AnalysisFilesViewState,
    modifier: Modifier = Modifier
) {
  val expanded = view.expandedDetails[row.file.path] == true
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
                  onClick = {
                    if (expanded) view.expandedDetails.remove(row.file.path)
                    else view.expandedDetails[row.file.path] = true
                  },
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
