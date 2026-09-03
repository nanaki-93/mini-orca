package io.miniorca.desktop

import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.TooltipArea
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.material.AlertDialog
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

internal data class DesktopStatusProvider(
    val scope: ModelScope,
    val model: ScopedModel,
)

internal data class DesktopStatusBarState(
    val project: ProjectAnalysis?,
    val index: ProjectIndex?,
    val selectedFile: ProjectFileInfo?,
    val symbols: List<SymbolInfo>,
    val selectedSymbol: SymbolInfo?,
    val focusedLine: Int,
    val analysis: FileAnalysis?,
    val loading: Boolean,
    val operation: String,
    val error: String?,
    val provider: DesktopStatusProvider,
    val connection: ConnectionState,
)

internal enum class DesktopStatusSegmentType {
  Error,
  Operation,
  Project,
  Index,
  File,
  Symbol,
  Line,
  Analysis,
  Provider,
  Daemon,
}

internal data class DesktopStatusSegment(
    val type: DesktopStatusSegmentType,
    val label: String,
    val detail: String,
    val priority: Int,
    val actionable: Boolean = false,
    val attention: Boolean = false,
)

internal data class DesktopStatusBarPresentation(
    val segments: List<DesktopStatusSegment>,
)

internal fun desktopStatusBarVisible(project: ProjectAnalysis?): Boolean = project != null

internal fun desktopStatusBarState(
    appState: DesktopState,
    provider: DesktopStatusProvider,
): DesktopStatusBarState =
    DesktopStatusBarState(
        project = appState.project,
        index = appState.index,
        selectedFile = appState.selectedFile,
        symbols = appState.symbols,
        selectedSymbol = appState.selectedSymbol,
        focusedLine = appState.selection.focusedLine,
        analysis = appState.analysis,
        loading = appState.loading,
        operation = appState.status,
        error = appState.error,
        provider = provider,
        connection = appState.connection,
    )

internal fun desktopStatusBarPresentation(
    state: DesktopStatusBarState,
): DesktopStatusBarPresentation {
  val project = state.project
  val index = currentStatusIndex(project, state.index)
  val file = currentStatusFile(index, state.selectedFile)
  val symbol = currentStatusSymbol(file, state.symbols, state.selectedSymbol)
  val focusedLine =
      file?.lineCount?.let { lineCount -> state.focusedLine.takeIf { it in 1..lineCount } }
  val analysis = state.analysis?.takeIf { it.path == file?.path }
  val segments = buildList {
    state.error?.trim()?.takeIf(String::isNotBlank)?.let { error ->
      add(
          DesktopStatusSegment(
              DesktopStatusSegmentType.Error,
              "Error: $error",
              error,
              priority = CRITICAL_PRIORITY,
              actionable = true,
              attention = true,
          ))
    }
    state.operation.trim().takeIf(String::isNotBlank)?.let { operation ->
      val label = if (state.loading) "Working: $operation" else "Status: $operation"
      add(
          DesktopStatusSegment(
              DesktopStatusSegmentType.Operation,
              label,
              operation,
              priority = CRITICAL_PRIORITY,
              actionable = true,
              attention = state.loading,
          ))
    }
    project?.let {
      add(
          DesktopStatusSegment(
              DesktopStatusSegmentType.Project,
              "Project: ${it.name}",
              "Project ${it.name} · ${it.type}",
              priority = LOW_PRIORITY,
          ))
      add(
          DesktopStatusSegment(
              DesktopStatusSegmentType.Index,
              indexStatusLabel(index),
              indexStatusDetail(index),
              priority = LOW_PRIORITY,
              attention = index == null,
          ))
    }
    file?.let {
      add(
          DesktopStatusSegment(
              DesktopStatusSegmentType.File,
              "File: ${it.name} · ${it.language}",
              "Selected file ${it.path} · ${it.language}",
              priority = MEDIUM_PRIORITY,
          ))
    }
    symbol?.let {
      add(
          DesktopStatusSegment(
              DesktopStatusSegmentType.Symbol,
              "Symbol: ${it.name}${focusedLine?.let { value -> " · line $value" }.orEmpty()}",
              "Selected ${it.kind} ${it.name}${focusedLine?.let { value -> " at line $value" }.orEmpty()}",
              priority = SECONDARY_PRIORITY,
          ))
    }
    if (symbol == null)
        file?.let { selectedFile ->
          focusedLine?.let { line ->
            add(
                DesktopStatusSegment(
                    DesktopStatusSegmentType.Line,
                    "Line: $line",
                    "Focused line $line in ${selectedFile.path}",
                    priority = SECONDARY_PRIORITY,
                ))
          }
        }
    analysis?.let {
      val status = statusBadgeStyle(it.status).label
      add(
          DesktopStatusSegment(
              DesktopStatusSegmentType.Analysis,
              "File analysis: $status",
              "File analysis for ${it.path}: $status",
              priority = MEDIUM_PRIORITY,
              attention = status in setOf("Stale", "Failed"),
          ))
    }
    providerStatusSegment(state.provider)?.let(::add)
    add(daemonStatusSegment(state.connection))
  }
  return DesktopStatusBarPresentation(segments)
}

internal fun visibleDesktopStatusSegments(
    presentation: DesktopStatusBarPresentation,
    widthDp: Float,
): List<DesktopStatusSegment> {
  val maximumPriority =
      when {
        widthDp >= WIDE_STATUS_BAR_WIDTH -> LOW_PRIORITY
        widthDp >= COMPACT_STATUS_BAR_WIDTH -> SECONDARY_PRIORITY
        widthDp >= NARROW_STATUS_BAR_WIDTH -> MEDIUM_PRIORITY
        else -> DAEMON_PRIORITY
      }
  return presentation.segments.filter { it.priority <= maximumPriority }
}

internal fun desktopStatusBarDescription(segments: List<DesktopStatusSegment>): String =
    segments.joinToString(separator = ". ") { it.detail }

@Composable
@OptIn(ExperimentalFoundationApi::class)
internal fun PersistentStatusBar(
    presentation: DesktopStatusBarPresentation,
    widthDp: Float,
    onOpenDetails: () -> Unit,
    modifier: Modifier = Modifier,
) {
  val segments = visibleDesktopStatusSegments(presentation, widthDp)
  Row(
      modifier
          .fillMaxWidth()
          .height(30.dp)
          .background(Panel)
          .border(androidx.compose.foundation.BorderStroke(1.dp, Border))
          .padding(horizontal = 10.dp)
          .semantics { contentDescription = desktopStatusBarDescription(segments) },
      verticalAlignment = Alignment.CenterVertically,
  ) {
    segments.forEachIndexed { index, segment ->
      if (index > 0) Spacer(Modifier.width(8.dp))
      StatusBarSegment(
          segment = segment,
          onOpenDetails = onOpenDetails,
          modifier = Modifier.weight(1f, fill = false),
      )
    }
  }
}

@Composable
@OptIn(ExperimentalFoundationApi::class)
private fun StatusBarSegment(
    segment: DesktopStatusSegment,
    onOpenDetails: () -> Unit,
    modifier: Modifier,
) {
  TooltipArea(tooltip = { StatusBarTooltip(segment.detail) }) {
    if (segment.actionable) {
      FocusFlowButton(
          onClick = onOpenDetails,
          tone = if (segment.attention) ActionTone.Attention else ActionTone.Neutral,
          density = ButtonDensity.Toolbar,
          modifier = modifier.semantics { contentDescription = segment.detail },
      ) {
        Text(segment.label, fontSize = 10.sp, maxLines = 1, overflow = TextOverflow.Ellipsis)
      }
    } else {
      Text(
          segment.label,
          color = if (segment.attention) Warning else SecondaryText,
          fontSize = 10.sp,
          maxLines = 1,
          overflow = TextOverflow.Ellipsis,
          modifier = modifier.semantics { contentDescription = segment.detail },
      )
    }
  }
}

@Composable
private fun StatusBarTooltip(detail: String) {
  Text(
      detail,
      color = PrimaryText,
      fontSize = 11.sp,
      modifier =
          Modifier.background(StrongSurface)
              .border(androidx.compose.foundation.BorderStroke(1.dp, Border))
              .padding(6.dp),
  )
}

@Composable
internal fun DesktopStatusDetailsDialog(
    presentation: DesktopStatusBarPresentation,
    onDismiss: () -> Unit,
) {
  AlertDialog(
      onDismissRequest = onDismiss,
      title = { Text("Current status") },
      text = {
        Column {
          presentation.segments.forEach { segment ->
            Text(
                segment.detail,
                color = if (segment.attention) Warning else SecondaryText,
                fontSize = 11.sp,
                modifier = Modifier.padding(bottom = 6.dp),
            )
          }
        }
      },
      confirmButton = {
        FocusFlowButton(onClick = onDismiss, tone = ActionTone.Primary) { Text("Close") }
      },
  )
}

private fun currentStatusIndex(
    project: ProjectAnalysis?,
    index: ProjectIndex?,
): ProjectIndex? =
    index?.takeIf {
      project != null &&
          it.projectId == project.projectId &&
          it.projectRevision == project.projectRevision
    }

private fun currentStatusFile(index: ProjectIndex?, file: ProjectFileInfo?): ProjectFileInfo? =
    file?.takeIf { selected -> index?.files?.any { it.path == selected.path } == true }

private fun currentStatusSymbol(
    file: ProjectFileInfo?,
    symbols: List<SymbolInfo>,
    selected: SymbolInfo?,
): SymbolInfo? = selected?.takeIf { file != null && it in symbols }

private fun indexStatusLabel(index: ProjectIndex?): String =
    if (index == null) "Index: unavailable" else "Index: ${index.files.size} files"

private fun indexStatusDetail(index: ProjectIndex?): String =
    if (index == null) "Current project index is unavailable"
    else "Current index has ${index.files.size} files"

private fun providerStatusSegment(provider: DesktopStatusProvider): DesktopStatusSegment? {
  val model = provider.model
  if (model.model.isBlank() && model.profile.isBlank() && model.providerOrigin.isBlank())
      return null
  val destination = if (model.remoteProvider) "remote provider" else "local provider"
  val priority = if (model.remoteProvider) DAEMON_PRIORITY else SECONDARY_PRIORITY
  return DesktopStatusSegment(
      DesktopStatusSegmentType.Provider,
      "${provider.scope.label}: $destination",
      modelDestinationLabel(provider.scope, model),
      priority = priority,
      attention = model.remoteProvider,
  )
}

private fun daemonStatusSegment(connection: ConnectionState): DesktopStatusSegment {
  val presentation = connectionPresentation(connection)
  val locality = connection.locality.trim().takeIf(String::isNotBlank)
  val detail = "Daemon: ${presentation.label}${locality?.let { " · $it" }.orEmpty()}"
  return DesktopStatusSegment(
      DesktopStatusSegmentType.Daemon,
      "Daemon: ${presentation.label}",
      detail,
      priority = DAEMON_PRIORITY,
      attention = !connection.connected,
  )
}

private const val CRITICAL_PRIORITY = 0
private const val DAEMON_PRIORITY = 1
private const val MEDIUM_PRIORITY = 2
private const val SECONDARY_PRIORITY = 3
private const val LOW_PRIORITY = 4
private const val NARROW_STATUS_BAR_WIDTH = 760f
private const val COMPACT_STATUS_BAR_WIDTH = 1_000f
private const val WIDE_STATUS_BAR_WIDTH = 1_220f
