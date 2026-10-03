package io.miniorca.desktop

import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.TooltipArea
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.focusable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
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
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.input.key.KeyEventType
import androidx.compose.ui.input.key.key
import androidx.compose.ui.input.key.onPreviewKeyEvent
import androidx.compose.ui.input.key.type
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.stateDescription
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

internal data class EditorChromeUiState(
    val title: String,
    val path: String,
    val breadcrumbSegments: List<EditorBreadcrumbSegment>,
    val accessibleDescription: String,
    val inspectionLabel: String,
    val retainedDraftLabel: String?,
    val activeSurface: EditorSurface,
    val reviewAvailable: Boolean,
    val stageLabel: String,
    val creationBlockedReason: String?,
)

internal fun declarationCreationBlockedReason(
    file: ProjectFileInfo?,
    busy: Boolean = false
): String? =
    when {
      file == null -> "Open a Go file to create a function or type."
      file.binary || !file.language.equals("Go", ignoreCase = true) ->
          "Function and type creation requires a Go source file."
      busy -> "Wait for the current generation or validation to finish."
      else -> null
    }

@Composable
internal fun NewFunctionButton(
    path: String,
    blockedReason: String?,
    onCreate: () -> Unit,
    modifier: Modifier = Modifier
) {
  MiniOrcaButton(
      onClick = onCreate,
      enabled = blockedReason == null,
      tone = ActionTone.Primary,
      density = ButtonDensity.Toolbar,
      modifier = modifier.semantics { contentDescription = "New function in $path" }) {
        Text("New function", style = IdeTypography.action)
      }
}

internal enum class EditorBreadcrumbKind(val icon: DesktopIcon?) {
  Folder(DesktopIcon.Folder),
  File(DesktopIcon.File),
  Symbol(DesktopIcon.Code),
  Collapsed(null),
  Placeholder(DesktopIcon.File),
}

internal data class EditorBreadcrumbSegment(
    val label: String,
    val kind: EditorBreadcrumbKind,
)

internal fun editorChromeUiState(
    file: ProjectFileInfo?,
    selectedSymbol: SymbolInfo?,
    requestedSurface: EditorSurface,
    progress: EditorProgressUiState,
    draft: DeclarationDraft?,
    creationInProgress: Boolean = false,
): EditorChromeUiState {
  val path = file?.path ?: "No file selected"
  val reviewAvailable =
      progress.progress == EditorProgress.Review && draft?.validation?.applicable == true
  val activeSurface =
      if (reviewAvailable && requestedSurface == EditorSurface.Review) EditorSurface.Review
      else EditorSurface.Source
  val title = file?.name ?: "No file open"
  val stageLabel =
      when {
        reviewAvailable && activeSurface == EditorSurface.Source -> "VALIDATED DRAFT"
        activeSurface == EditorSurface.Review -> "REVIEW CANDIDATE"
        draft != null && progress.progress == EditorProgress.Edit -> "DRAFT EDITED"
        else -> "SOURCE"
      }
  val symbol =
      (if (activeSurface == EditorSurface.Review) draft?.targetSymbol else selectedSymbol?.name)
          ?.takeIf(String::isNotBlank)
  val inspectionLabel =
      if (activeSurface == EditorSurface.Review && draft != null) {
        "Reviewing candidate: ${draft.targetSymbol} · revision ${draft.revision}"
      } else if (file != null && selectedSymbol != null && symbol != null) {
        val approximate =
            if (selectedSymbol.confidence.equals("exact", ignoreCase = true)) ""
            else " · Approximate · read-only"
        "Inspecting declaration: $symbol · ${symbolRangeLabel(selectedSymbol, file)}$approximate"
      } else if (file != null) "Inspecting file" else "No file selected"
  val retainedDraftLabel =
      progress.currentEditIdentity
          ?.takeIf { it.hasDraft && !it.matches(file, selectedSymbol) }
          ?.let {
            "Retained draft: ${it.targetPath} · ${it.targetSymbol} (not the inspected declaration)"
          }
  return EditorChromeUiState(
      title = title,
      path = path,
      breadcrumbSegments = editorBreadcrumbSegments(path, symbol),
      accessibleDescription =
          "$title. $path${symbol?.let { ". Selected declaration $it" }.orEmpty()}. Read-only ${activeSurface.name.lowercase()} surface. $stageLabel. $inspectionLabel.${retainedDraftLabel?.let { " $it." }.orEmpty()}",
      inspectionLabel = inspectionLabel,
      retainedDraftLabel = retainedDraftLabel,
      activeSurface = activeSurface,
      reviewAvailable = reviewAvailable,
      stageLabel = stageLabel,
      creationBlockedReason = declarationCreationBlockedReason(file, creationInProgress),
  )
}

internal fun editorBreadcrumbSegments(
    path: String,
    symbol: String? = null,
): List<EditorBreadcrumbSegment> {
  val pathSegments = path.split('/').filter(String::isNotBlank)
  if (pathSegments.isEmpty() || path == "No file selected")
      return listOf(EditorBreadcrumbSegment(path, EditorBreadcrumbKind.Placeholder))

  val compactPath =
      when {
        pathSegments.size <= 3 -> pathSegments
        else -> listOf(pathSegments.first(), "…", pathSegments.last())
      }
  return buildList {
    compactPath.forEachIndexed { index, label ->
      add(
          EditorBreadcrumbSegment(
              label,
              when {
                label == "…" -> EditorBreadcrumbKind.Collapsed
                index == compactPath.lastIndex -> EditorBreadcrumbKind.File
                else -> EditorBreadcrumbKind.Folder
              },
          ))
    }
    symbol?.takeIf(String::isNotBlank)?.let {
      add(EditorBreadcrumbSegment(it, EditorBreadcrumbKind.Symbol))
    }
  }
}

internal sealed interface FileReadUiState {
  data class Pending(val path: String) : FileReadUiState

  // Pathless failures cover local reads before a destination could be established.
  data class Failed(val path: String?, val message: String) : FileReadUiState
}

internal fun fileReadUiState(
    pendingPath: String?,
    failedPath: String?,
    error: String?,
): FileReadUiState? =
    when {
      pendingPath != null -> FileReadUiState.Pending(pendingPath)
      error != null -> FileReadUiState.Failed(failedPath, error)
      else -> null
    }

@Composable
internal fun FileReadFeedback(
    state: FileReadUiState,
    retainingFile: Boolean,
    onOpenFile: ((String) -> Unit)?,
    modifier: Modifier = Modifier,
    fallbackAction: (@Composable () -> Unit)? = null,
) {
  val failed = state as? FileReadUiState.Failed
  val path =
      when (state) {
        is FileReadUiState.Pending -> state.path
        is FileReadUiState.Failed -> state.path
      }
  SystemStateMessage(
      title = if (failed != null) "Could not open file" else "Opening file",
      message =
          if (failed != null)
              "Reading local file data failed. ${failed.message.ifBlank { "No details available." }}"
          else "Reading local file data.",
      accent = if (failed != null) Error else Information,
      modifier = modifier,
      action = {
        Column {
          if (path != null) {
            val label = if (failed != null) "Failed destination" else "Pending destination"
            Text(label, color = SecondaryText, style = IdeTypography.workspaceMetadata)
            Text(
                path,
                color = PrimaryText,
                style = IdeTypography.compactBody,
                softWrap = false,
                modifier =
                    Modifier.fillMaxWidth()
                        .horizontalScroll(rememberScrollState())
                        .testTag("file-read-path")
                        .semantics { contentDescription = "$label: $path" })
          }
          if (retainingFile)
              Text(
                  "The current file remains open.",
                  color = SecondaryText,
                  style = IdeTypography.compactBody)
          if (failed != null && path != null && onOpenFile != null) {
            MiniOrcaButton(
                onClick = { onOpenFile(path) },
                density = ButtonDensity.Toolbar,
                modifier = Modifier.semantics { contentDescription = "Retry opening $path" }) {
                  Text("Retry opening file", style = IdeTypography.action)
                }
          } else if (failed != null) fallbackAction?.invoke()
        }
      })
}

@Composable
internal fun EditorWorkspace(
    chrome: EditorChromeUiState,
    review: ReviewToolWindowState?,
    onSelectSurface: (EditorSurface) -> Unit,
    onCreateDeclaration: () -> Unit,
    canvas: @Composable () -> Unit,
    modifier: Modifier = Modifier,
    onEditDraft: (() -> Unit)? = null,
    fileRead: FileReadUiState? = null,
    onOpenFile: ((String) -> Unit)? = null,
) {
  BoxWithConstraints(modifier.fillMaxSize().background(EditorCanvas)) {
    // Retained work and recovery can outgrow short windows. Scroll only the chrome rather
    // than squeezing away the selectable source or replacing its composition on reflow.
    val textScale = LocalDensity.current.fontScale
    val canvasReserve = if (chrome.activeSurface == EditorSurface.Review) 200.dp else 120.dp
    val chromeHeight =
        (maxHeight - canvasReserve * textScale)
            .coerceAtLeast(56.dp * textScale)
            .coerceAtMost(maxHeight * 0.65f)
    Column(Modifier.fillMaxSize()) {
      Column(
          Modifier.fillMaxWidth()
              .heightIn(max = chromeHeight)
              .verticalScroll(rememberScrollState())
              .testTag("editor-chrome-scroll")) {
            ActiveFileEditorChrome(
                chrome,
                onSelectSurface,
                onCreateDeclaration,
                onEditDraft.takeIf { review?.draft != null })
            fileRead?.let {
              FileReadFeedback(
                  it,
                  retainingFile = chrome.path != "No file selected",
                  onOpenFile = onOpenFile,
                  modifier = Modifier.fillMaxWidth().padding(8.dp))
            }
            if (review?.draft != null) EditorReviewProgression(editorProgressionRows(review))
          }
      Box(Modifier.fillMaxWidth().weight(1f)) { canvas() }
    }
  }
}

@Composable
@OptIn(ExperimentalFoundationApi::class)
private fun EditorReviewProgression(rows: List<ReviewEvidenceRow>) {
  Row(
      Modifier.fillMaxWidth()
          .horizontalScroll(rememberScrollState())
          .padding(horizontal = 12.dp, vertical = 8.dp)
          .testTag("editor-progression"),
      horizontalArrangement = Arrangement.spacedBy(12.dp),
  ) {
    rows.forEachIndexed { index, row ->
      val label =
          when (row.label) {
            "Validation" -> "Validate"
            "Focused checks" -> "Checks"
            else -> row.label
          }
      val marker =
          when (row.status) {
            ReviewEvidenceStatus.Passed -> "✓"
            ReviewEvidenceStatus.Failed -> "×"
            ReviewEvidenceStatus.Canceled -> "–"
            ReviewEvidenceStatus.Stale -> "!"
            ReviewEvidenceStatus.Running -> "…"
            ReviewEvidenceStatus.Skipped -> "–"
            ReviewEvidenceStatus.Missing -> "${index + 1}"
          }
      val exceptional =
          row.status !in setOf(ReviewEvidenceStatus.Passed, ReviewEvidenceStatus.Missing)
      TooltipArea(
          tooltip = { IdeControlTooltip(row.detail) },
          modifier = Modifier.align(Alignment.CenterVertically)) {
            Row(
                Modifier.semantics(mergeDescendants = true) {
                  contentDescription = "${row.label}: ${row.detail}"
                  stateDescription = row.status.label
                },
                verticalAlignment = Alignment.CenterVertically) {
                  val passed = row.status == ReviewEvidenceStatus.Passed
                  Box(
                      Modifier.size(22.dp * LocalDensity.current.fontScale)
                          .background(if (passed) Success else EditorCanvas, MiniOrcaShapes.pill)
                          .border(1.dp, evidenceColor(row.status), MiniOrcaShapes.pill),
                      contentAlignment = Alignment.Center) {
                        Text(
                            marker,
                            color = if (passed) EditorCanvas else evidenceColor(row.status),
                            style = IdeTypography.workspaceMetadata)
                      }
                  Spacer(Modifier.width(6.dp))
                  Text(
                      if (exceptional) "$label · ${row.status.label}" else label,
                      color =
                          if (row.status == ReviewEvidenceStatus.Passed) PrimaryText
                          else evidenceColor(row.status),
                      style = IdeTypography.workspaceMetadata,
                      softWrap = false)
                }
          }
    }
  }
}

@Composable
@OptIn(ExperimentalLayoutApi::class)
private fun ActiveFileEditorChrome(
    state: EditorChromeUiState,
    onSelectSurface: (EditorSurface) -> Unit,
    onCreateDeclaration: () -> Unit,
    onEditDraft: (() -> Unit)?,
) {
  val surfaces = buildList {
    add(EditorSurface.Source)
    if (state.reviewAvailable) add(EditorSurface.Review)
  }
  var focusedSurface by
      remember(state.activeSurface, state.reviewAvailable) { mutableStateOf(state.activeSurface) }
  var tabGroupHasFocus by remember { mutableStateOf(false) }
  Column(
      Modifier.fillMaxWidth().background(EditorCanvas).semantics {
        contentDescription = state.accessibleDescription
      },
  ) {
    FlowRow(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
      Row(
          Modifier.horizontalScroll(rememberScrollState())
              .onFocusChanged { tabGroupHasFocus = it.hasFocus }
              .focusable()
              .onPreviewKeyEvent { event ->
                if (event.type != KeyEventType.KeyDown) return@onPreviewKeyEvent false
                val interaction =
                    tabGroupInteraction(surfaces, focusedSurface, tabGroupKey(event.key))
                        ?: return@onPreviewKeyEvent false
                focusedSurface = interaction.focused
                interaction.activate?.let(onSelectSurface)
                true
              }
              .testTag("editor-tabs"),
          verticalAlignment = Alignment.CenterVertically) {
            ChromeTab(
                onClick = { onSelectSurface(EditorSurface.Source) },
                selected = state.activeSurface == EditorSurface.Source,
                focusHighlight = tabGroupHasFocus && focusedSurface == EditorSurface.Source,
                modifier =
                    Modifier.semantics { contentDescription = "Source file · ${state.title}" },
            ) {
              DesktopLineIcon(
                  DesktopIcon.File, "Source file", tint = SelectionText, iconSize = 16.dp)
              Spacer(Modifier.width(6.dp))
              Text("Source", fontSize = 12.sp, maxLines = 1)
            }
            if (state.reviewAvailable) {
              ChromeTab(
                  onClick = { onSelectSurface(EditorSurface.Review) },
                  selected = state.activeSurface == EditorSurface.Review,
                  focusHighlight = tabGroupHasFocus && focusedSurface == EditorSurface.Review) {
                    DesktopLineIcon(DesktopIcon.Check, "Candidate diff", iconSize = 14.dp)
                    Spacer(Modifier.width(6.dp))
                    Text("Candidate diff", fontSize = 12.sp, softWrap = false)
                  }
            }
          }
      EditorDraftActions(state, onCreateDeclaration, onEditDraft)
    }
    state.creationBlockedReason?.let { reason ->
      Text(
          reason,
          color = SecondaryText,
          style = IdeTypography.compactBody,
          modifier = Modifier.padding(horizontal = 8.dp, vertical = 4.dp))
    }
    Row(
        Modifier.fillMaxWidth()
            .background(EditorCanvas)
            .padding(horizontal = 8.dp, vertical = 4.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
      EditorBreadcrumbs(
          state.breadcrumbSegments,
          state.path,
          modifier = Modifier.weight(1f),
      )
      Spacer(Modifier.width(8.dp))
      Text("Read-only", color = FaintText, fontSize = 11.sp, softWrap = false, maxLines = 1)
    }
    if (state.activeSurface == EditorSurface.Source) {
      Column(Modifier.fillMaxWidth().padding(horizontal = 8.dp, vertical = 4.dp)) {
        Text(
            state.inspectionLabel,
            color = SecondaryText,
            style = IdeTypography.workspaceMetadata,
            softWrap = false,
            modifier =
                Modifier.fillMaxWidth()
                    .horizontalScroll(rememberScrollState())
                    .testTag("editor-inspection"))
        state.retainedDraftLabel?.let { label ->
          Text(
              label,
              color = Warning,
              style = IdeTypography.workspaceMetadata,
              softWrap = false,
              modifier =
                  Modifier.fillMaxWidth()
                      .horizontalScroll(rememberScrollState())
                      .testTag("editor-retained-draft"))
        }
      }
    }
  }
}

@Composable
private fun EditorDraftActions(
    state: EditorChromeUiState,
    onCreate: () -> Unit,
    onEdit: (() -> Unit)?,
) {
  Row(
      Modifier.horizontalScroll(rememberScrollState())
          .padding(horizontal = 8.dp, vertical = 4.dp)
          .testTag("editor-draft-actions"),
      horizontalArrangement = Arrangement.spacedBy(8.dp),
      verticalAlignment = Alignment.CenterVertically) {
        NewFunctionButton(state.path, state.creationBlockedReason, onCreate)
        if (onEdit != null)
            MiniOrcaButton(
                onClick = onEdit, tone = ActionTone.Neutral, density = ButtonDensity.Toolbar) {
                  Text("Edit draft", style = IdeTypography.action)
                }
      }
}

@Composable
@OptIn(ExperimentalFoundationApi::class)
private fun EditorBreadcrumbs(
    segments: List<EditorBreadcrumbSegment>,
    fullPath: String,
    modifier: Modifier = Modifier,
) {
  TooltipArea(tooltip = { IdeControlTooltip(fullPath) }, modifier = modifier) {
    Row(
        Modifier.fillMaxWidth()
            .horizontalScroll(rememberScrollState())
            .testTag("editor-path")
            .semantics { contentDescription = "Project-relative path: $fullPath" },
        verticalAlignment = Alignment.CenterVertically,
    ) {
      segments.forEachIndexed { index, segment ->
        if (index > 0) {
          Text(
              "›",
              color = FaintText,
              fontSize = 12.sp,
              modifier = Modifier.padding(horizontal = 4.dp),
          )
        }
        segment.kind.icon?.let { icon ->
          DesktopLineIcon(icon, segment.kind.name.lowercase(), iconSize = 12.dp)
          Spacer(Modifier.width(4.dp))
        }
        Text(segment.label, color = SecondaryText, fontSize = 12.sp, maxLines = 1)
      }
      if (segments.any { it.kind == EditorBreadcrumbKind.Collapsed }) {
        Text(
            "›",
            color = FaintText,
            fontSize = 12.sp,
            modifier = Modifier.padding(horizontal = 4.dp))
        Text(fullPath, color = SecondaryText, fontSize = 12.sp, maxLines = 1)
      }
    }
  }
}
