package io.miniorca.desktop

import androidx.compose.foundation.Image
import androidx.compose.foundation.ScrollState
import androidx.compose.foundation.background
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.key
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.ExperimentalComposeUiApi
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.ClipEntry
import androidx.compose.ui.platform.LocalClipboard
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.stateDescription
import androidx.compose.ui.unit.dp
import java.awt.datatransfer.StringSelection
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.launch

internal class DiagramViewState {
  var showDiagram by mutableStateOf(false)
  var showSource by mutableStateOf(false)
  var zoom by mutableStateOf(1f)
  var copyFeedback by mutableStateOf<DiagramCopyFeedback?>(null)
  // Keep inspection position with the diagram owner, not the transient dialog composition.
  val horizontalScroll = ScrollState(0)
  val verticalScroll = ScrollState(0)
  internal var renderState by mutableStateOf<DiagramState>(DiagramState.Unavailable)
    private set

  private var job: Job? = null
  private var generation = 0
  private var started = false

  internal fun start(
      scope: CoroutineScope,
      source: String?,
      render: suspend (String) -> MermaidImage,
  ) {
    if (started) return
    started = true
    if (source == null) return
    renderState = DiagramState.Loading
    val active = ++generation
    job =
        scope.launch {
          try {
            val image = render(source)
            if (generation == active) renderState = DiagramState.Ready(image)
          } catch (cancelled: CancellationException) {
            throw cancelled
          } catch (exception: Exception) {
            if (generation == active)
                renderState =
                    DiagramState.Failed(exception.message?.take(240) ?: "Unable to render diagram")
          }
        }
  }

  internal fun cancel() {
    generation++
    job?.cancel()
    job = null
  }
}

internal suspend fun renderSummaryDiagram(source: String): MermaidImage =
    renderMermaidImage(MermaidRenderer.render(source).svg)

internal data class SummaryDiagramInput(
    val source: String?,
    val prose: String,
    val original: String,
    val generatedFromArrowChain: Boolean = false,
)

internal fun summaryDiagramInput(value: String): SummaryDiagramInput {
  val fence = Regex("(?is)```[ \\t]*mermaid[ \\t]*\\r?\\n(.*?)\\r?\\n[ \\t]*```").find(value)
  if (fence != null)
      return SummaryDiagramInput(
          fence.groupValues[1].trim(), value.removeRange(fence.range).trim(), value)
  if (Regex("^(flowchart|graph|sequenceDiagram)\\b").containsMatchIn(value.trim())) {
    return SummaryDiagramInput(value.trim(), "", value)
  }
  // A declared but unsupported Mermaid type is an attempted diagram, not a prose report.
  val unsupported =
      Regex(
              "(?im)^[ \\t]*(?:[a-z][a-z0-9]*Diagram(?:-v[0-9]+)?|journey|gantt|" +
                  "pie(?:[ \\t]+(?:showData(?:[ \\t]+title[ \\t]+[^\\r\\n]+)?|" +
                  "title[ \\t]+[^\\r\\n]+))?|gitGraph|mindmap|timeline|quadrantChart|" +
                  "C4Context|zenuml|kanban|treemap|sankey-beta|xychart-beta|block-beta|" +
                  "packet-beta|architecture-beta|radar-beta)[ \\t]*(?=\\r?\\n|\\z)")
          .find(value)
  if (unsupported != null) {
    return SummaryDiagramInput(
        value.substring(unsupported.range.first).trim(),
        value.substring(0, unsupported.range.first).trim(),
        value)
  }
  // Previously saved arrow chains remain usable without a new model request.
  if ('\n' !in value && '`' !in value && '<' !in value) {
    val nodes = value.split(Regex("\\s*(?:->|→|⟶)\\s*"))
    if (nodes.size in 2..8 && nodes.all { it.isNotBlank() }) {
      val declarations =
          nodes.mapIndexed { index, label ->
            "n$index[\"${label.replace("&", "&amp;").replace("\"", "&quot;")}\"]"
          }
      return SummaryDiagramInput(
          "flowchart LR\n" + declarations.joinToString(" --> "),
          "",
          value,
          generatedFromArrowChain = true)
    }
  }
  return SummaryDiagramInput(null, value, value)
}

internal sealed interface DiagramCopyFeedback {
  data object Copied : DiagramCopyFeedback

  data class Failed(val message: String) : DiagramCopyFeedback
}

internal sealed interface DiagramState {
  data object Unavailable : DiagramState

  data object Loading : DiagramState

  data class Ready(val image: MermaidImage) : DiagramState

  data class Failed(val message: String) : DiagramState
}

@Composable
internal fun MermaidDiagram(
    value: String,
    label: String,
    title: String? = null,
    ownerIdentity: Any = Unit,
    viewState: DiagramViewState? = null,
    renderScope: CoroutineScope? = null,
    render: suspend (String) -> MermaidImage = ::renderSummaryDiagram,
) {
  val input = remember(value) { summaryDiagramInput(value) }
  val localScope = rememberCoroutineScope()
  Column(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(8.dp)) {
    if (title == null && input.prose.isNotBlank()) ModelResultContent(input.prose, preview = false)
    key(ownerIdentity, input.source) {
      val localView = remember { DiagramViewState() }
      if (viewState == null) DisposableEffect(localView) { onDispose { localView.cancel() } }
      MermaidDiagramSource(
          input,
          label,
          title,
          input.prose.takeIf { title != null },
          viewState ?: localView,
          renderScope ?: localScope,
          render)
      if (viewState == null && localView.showDiagram)
          MermaidDiagramViewer(input, label, title, localView)
    }
  }
}

@Composable
@OptIn(ExperimentalLayoutApi::class)
private fun MermaidDiagramSource(
    input: SummaryDiagramInput,
    label: String,
    title: String?,
    prose: String?,
    view: DiagramViewState,
    renderScope: CoroutineScope,
    render: suspend (String) -> MermaidImage,
) {
  val source = input.source
  LaunchedEffect(view, source) { view.start(renderScope, source, render) }
  val state = view.renderState
  val showDiagram = view.showDiagram
  Row(
      Modifier.fillMaxWidth(),
      horizontalArrangement = Arrangement.End,
      verticalAlignment = Alignment.CenterVertically) {
        title?.let {
          Text(
              it,
              color = ResultAccent,
              style = IdeTypography.workspaceHeading,
              modifier = Modifier.weight(1f).semantics { heading() })
        }
        ChromeButton(
            onClick = { view.showDiagram = true },
            enabled = source != null,
            accessibleName = "Expand $label diagram",
            modifier =
                Modifier.semantics {
                  stateDescription =
                      when (state) {
                        DiagramState.Unavailable -> "Diagram unavailable"
                        DiagramState.Loading -> "Rendering diagram"
                        is DiagramState.Failed -> "Diagram failed"
                        is DiagramState.Ready -> if (showDiagram) "Expanded" else "Preview"
                      }
                }) {
              DesktopLineIcon(DesktopIcon.ChevronRight, "", iconSize = 16.dp)
              Text("Expand diagram")
            }
      }
  prose
      ?.takeIf { it.isNotBlank() }
      ?.let { ModelResultContent(it, preview = false, style = IdeTypography.workspaceBody) }
  if (source != null || input.original.isNotBlank()) {
    Box(
        Modifier.fillMaxWidth()
            .height(180.dp)
            .clip(MiniOrcaShapes.control)
            .background(EditorCanvas)
            .testTag("diagram-preview"),
        contentAlignment = Alignment.Center) {
          when (val current = state) {
            DiagramState.Unavailable ->
                Text(
                    "No diagram in saved content",
                    color = SecondaryText,
                    style = IdeTypography.compactBody)
            DiagramState.Loading ->
                Text("Rendering diagram…", color = SecondaryText, style = IdeTypography.compactBody)
            is DiagramState.Failed ->
                Text(
                    "Diagram unavailable: ${current.message}",
                    color = Warning,
                    style = IdeTypography.compactBody,
                    modifier = Modifier.padding(8.dp))
            is DiagramState.Ready ->
                Image(
                    current.image.bitmap,
                    "$label diagram preview\n$source",
                    contentScale = ContentScale.Fit,
                    modifier = Modifier.fillMaxSize().padding(8.dp))
          }
        }
  }
  if (state is DiagramState.Failed) {
    Text("Original saved content", style = IdeTypography.compactBody)
    LiteralDiagramSource(input.original)
  }
}

@Composable
@OptIn(ExperimentalLayoutApi::class, ExperimentalComposeUiApi::class)
internal fun MermaidDiagramViewer(
    input: SummaryDiagramInput,
    label: String,
    title: String?,
    view: DiagramViewState,
) {
  var showSource by view::showSource
  var zoom by view::zoom
  val source = input.source
  val clipboard = LocalClipboard.current
  val copyScope = rememberCoroutineScope()
  IdeDialog(
      onDismissRequest = { view.showDiagram = false },
      title = {
        Text(
            "${title ?: label} diagram",
            style = IdeTypography.workspaceHeading,
            modifier = Modifier.semantics { heading() })
      },
      content = {
        FlowRow(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.End) {
          ChromeButton(
              onClick = { zoom = (zoom - 0.25f).coerceAtLeast(0.75f) },
              enabled = zoom > 0.75f,
              accessibleName = "Zoom out $label") {
                Text("−")
              }
          ChromeButton(
              onClick = { zoom = 1f },
              accessibleName = "Reset zoom $label",
              modifier =
                  Modifier.semantics { stateDescription = "Zoom ${(zoom * 100).toInt()}%" }) {
                Text("${(zoom * 100).toInt()}%")
              }
          ChromeButton(
              onClick = { zoom = (zoom + 0.25f).coerceAtMost(2f) },
              enabled = zoom < 2f,
              accessibleName = "Zoom in $label") {
                Text("+")
              }
          ChromeButton(
              onClick = { showSource = !showSource },
              accessibleName = "Mermaid source for $label") {
                Text(if (showSource) "Hide Mermaid" else "Mermaid source")
              }
        }
        when (val state = view.renderState) {
          DiagramState.Unavailable -> Text("No diagram in saved content")
          DiagramState.Loading -> Text("Rendering diagram…")
          is DiagramState.Failed -> Text("Diagram unavailable: ${state.message}", color = Warning)
          is DiagramState.Ready -> {
            val scale = LocalDensity.current.fontScale * zoom
            Box(
                Modifier.fillMaxWidth()
                    .height(300.dp)
                    .clip(MiniOrcaShapes.control)
                    .background(EditorCanvas)
                    .testTag("diagram-canvas")) {
                  Box(
                      Modifier.fillMaxSize()
                          .verticalScroll(view.verticalScroll)
                          .testTag("diagram-vertical-scroll")) {
                        Box(
                            Modifier.fillMaxWidth()
                                .horizontalScroll(view.horizontalScroll)
                                .testTag("diagram-horizontal-scroll")
                                .padding(8.dp)) {
                              Image(
                                  state.image.bitmap,
                                  "$label diagram\n$source",
                                  modifier =
                                      Modifier.size(
                                          (state.image.width * scale).dp,
                                          (state.image.height * scale).dp))
                            }
                      }
                }
          }
        }
        if (showSource) {
          Text(
              if (input.generatedFromArrowChain) "Original saved arrow chain"
              else "Original saved content",
              style = IdeTypography.compactBody)
          LiteralDiagramSource(input.original)
          ChromeButton(
              onClick = {
                view.copyFeedback = null
                copyScope.launch {
                  try {
                    clipboard.setClipEntry(ClipEntry(StringSelection(input.original)))
                    view.copyFeedback = DiagramCopyFeedback.Copied
                  } catch (cancelled: CancellationException) {
                    throw cancelled
                  } catch (exception: Exception) {
                    view.copyFeedback =
                        DiagramCopyFeedback.Failed(exception.message ?: "Clipboard unavailable")
                  }
                }
              },
              accessibleName = "Copy saved content for $label") {
                Text("Copy source")
              }
          when (val feedback = view.copyFeedback) {
            DiagramCopyFeedback.Copied -> Text("Saved content copied", color = SecondaryText)
            is DiagramCopyFeedback.Failed ->
                Text("Could not copy saved content: ${feedback.message}", color = Warning)
            null -> Unit
          }
          if (input.generatedFromArrowChain && source != null) {
            Text("Generated Mermaid for rendering (not saved)", style = IdeTypography.compactBody)
            LiteralDiagramSource(source)
          }
        }
      },
      actions = {
        ChromeButton(
            onClick = { view.showDiagram = false }, accessibleName = "Close $label diagram") {
              Text("Close")
            }
      })
}

@Composable
private fun LiteralDiagramSource(value: String) {
  Box(
      Modifier.fillMaxWidth()
          .heightIn(max = 200.dp)
          .clip(MiniOrcaShapes.control)
          .background(EditorCanvas)
          .verticalScroll(rememberScrollState())
          .testTag("diagram-source-scroll")) {
        SelectionContainer {
          Text(
              value,
              color = PrimaryText,
              style = IdeTypography.resultCode,
              softWrap = false,
              modifier =
                  Modifier.horizontalScroll(rememberScrollState())
                      .testTag("diagram-source-horizontal-scroll")
                      .padding(8.dp))
        }
      }
}
