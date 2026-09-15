package io.miniorca.desktop

import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.TooltipArea
import androidx.compose.foundation.background
import androidx.compose.foundation.focusable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
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
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

internal data class EditorChromeUiState(
    val title: String,
    val path: String,
    val breadcrumbSegments: List<EditorBreadcrumbSegment>,
    val accessibleDescription: String,
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

internal data class CandidateSummaryPresentation(
    val target: String,
    val stage: String,
    val changedLines: String,
    val reviewAvailable: Boolean,
)

internal fun candidateSummaryPresentation(
    draft: DeclarationDraft?,
    chrome: EditorChromeUiState,
): CandidateSummaryPresentation? {
  val currentDraft = draft ?: return null
  val changed =
      currentDraft.validation?.diff?.lines.orEmpty().count { it.kind in setOf("added", "removed") }
  return CandidateSummaryPresentation(
      target =
          listOf(currentDraft.targetPath, currentDraft.targetSymbol)
              .filter(String::isNotBlank)
              .joinToString(" · "),
      stage = chrome.stageLabel,
      changedLines = if (changed > 0) "$changed changed lines" else "Validate to compose a diff.",
      reviewAvailable = chrome.reviewAvailable,
  )
}

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
  val symbol = selectedSymbol?.name?.takeIf(String::isNotBlank)
  return EditorChromeUiState(
      title = title,
      path = path,
      breadcrumbSegments = editorBreadcrumbSegments(path, symbol),
      accessibleDescription =
          "$title. $path${symbol?.let { ". Selected declaration $it" }.orEmpty()}. Read-only ${activeSurface.name.lowercase()} surface. $stageLabel.",
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

@Composable
internal fun EditorWorkspace(
    chrome: EditorChromeUiState,
    draft: DeclarationDraft?,
    onSelectSurface: (EditorSurface) -> Unit,
    onCreateDeclaration: () -> Unit,
    canvas: @Composable () -> Unit,
    modifier: Modifier = Modifier,
) {
  Column(modifier.fillMaxSize().background(EditorCanvas)) {
    ActiveFileEditorChrome(chrome, onSelectSurface, onCreateDeclaration)
    Box(Modifier.fillMaxWidth().weight(1f)) { canvas() }
    candidateSummaryPresentation(draft, chrome)?.let { summary ->
      CandidateSummary(summary, onReview = { onSelectSurface(EditorSurface.Review) })
    }
  }
}

@Composable
private fun CandidateSummary(summary: CandidateSummaryPresentation, onReview: () -> Unit) {
  MiniOrcaPanel(
      Modifier.fillMaxWidth().padding(12.dp),
      contentPadding = androidx.compose.foundation.layout.PaddingValues(12.dp),
  ) {
    Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
      DesktopLineIcon(DesktopIcon.Editor, "Candidate", tint = SelectionText, iconSize = 16.dp)
      Spacer(Modifier.width(8.dp))
      Text("Candidate", color = PrimaryText, fontSize = 12.sp, modifier = Modifier.weight(1f))
      MiniOrcaButton(
          onClick = onReview,
          enabled = summary.reviewAvailable,
          tone = ActionTone.Neutral,
          density = ButtonDensity.Toolbar) {
            Text(
                if (summary.reviewAvailable) "Open review" else "Review unavailable",
                fontSize = 12.sp)
          }
    }
    Text(
        summary.target.ifBlank { "Current declaration draft" },
        color = PrimaryText,
        fontSize = 12.sp,
        maxLines = 1,
        overflow = TextOverflow.Ellipsis,
        modifier = Modifier.padding(top = 8.dp))
    Text(
        "${summary.stage} · ${summary.changedLines} · focused checks pending",
        color = if (summary.reviewAvailable) Success else SecondaryText,
        fontSize = 12.sp,
        modifier = Modifier.padding(top = 4.dp))
  }
}

@Composable
private fun ActiveFileEditorChrome(
    state: EditorChromeUiState,
    onSelectSurface: (EditorSurface) -> Unit,
    onCreateDeclaration: () -> Unit,
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
    BoxWithConstraints(Modifier.fillMaxWidth()) {
      val stackAction = maxWidth < 480.dp
      Column {
        Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
          Row(
              Modifier.weight(1f)
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
                  },
              verticalAlignment = Alignment.CenterVertically) {
                ChromeTab(
                    onClick = { onSelectSurface(EditorSurface.Source) },
                    selected = state.activeSurface == EditorSurface.Source,
                    focusHighlight = tabGroupHasFocus && focusedSurface == EditorSurface.Source,
                    modifier =
                        Modifier.weight(1f, fill = false).semantics {
                          contentDescription = "Source file · ${state.title}"
                        },
                ) {
                  DesktopLineIcon(
                      DesktopIcon.File, "Source file", tint = SelectionText, iconSize = 16.dp)
                  Spacer(Modifier.width(6.dp))
                  Text(
                      "Source · ${state.title}",
                      fontSize = 12.sp,
                      maxLines = 1,
                      overflow = TextOverflow.Ellipsis)
                }
                if (state.reviewAvailable) {
                  ChromeTab(
                      onClick = { onSelectSurface(EditorSurface.Review) },
                      selected = state.activeSurface == EditorSurface.Review,
                      focusHighlight = tabGroupHasFocus && focusedSurface == EditorSurface.Review) {
                        DesktopLineIcon(DesktopIcon.Check, "Review candidate", iconSize = 14.dp)
                        Spacer(Modifier.width(6.dp))
                        Text("Review candidate", fontSize = 12.sp)
                      }
                }
              }
          if (!stackAction)
              NewFunctionButton(
                  state.path,
                  state.creationBlockedReason,
                  onCreateDeclaration,
                  Modifier.padding(end = 8.dp))
        }
        if (stackAction)
            NewFunctionButton(
                state.path,
                state.creationBlockedReason,
                onCreateDeclaration,
                Modifier.padding(horizontal = 8.dp, vertical = 4.dp))
      }
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
      Text(
          if (state.reviewAvailable) state.stageLabel.replace('_', ' ') else "Read-only",
          color = if (state.reviewAvailable) Success else FaintText,
          fontSize = 11.sp)
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
  TooltipArea(tooltip = { IdeControlTooltip(fullPath) }) {
    Row(
        modifier.horizontalScroll(rememberScrollState()).semantics {
          contentDescription = "Project-relative path: $fullPath"
        },
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
    }
  }
}
