package io.miniorca.desktop

import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.rememberScrollState
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.key
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.produceState
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.stateDescription
import androidx.compose.ui.unit.dp
import kotlinx.coroutines.CancellationException

internal class DiagramViewState {
  var showDiagram by mutableStateOf(false)
  var showSource by mutableStateOf(false)
  var zoom by mutableStateOf(1f)
}

internal data class SummaryDiagramInput(val source: String?, val prose: String)

internal fun summaryDiagramInput(value: String): SummaryDiagramInput {
  val fence = Regex("(?is)```[ \\t]*mermaid[ \\t]*\\r?\\n(.*?)\\r?\\n[ \\t]*```").find(value)
  if (fence != null)
      return SummaryDiagramInput(fence.groupValues[1].trim(), value.removeRange(fence.range).trim())
  if (Regex("^(flowchart|graph|sequenceDiagram)\\b").containsMatchIn(value.trim())) {
    return SummaryDiagramInput(value.trim(), "")
  }
  // Previously saved arrow chains remain usable without a new model request.
  if ('\n' !in value && '`' !in value && '<' !in value) {
    val nodes = value.split(Regex("\\s*(?:->|→|⟶)\\s*"))
    if (nodes.size in 2..8 && nodes.all { it.isNotBlank() }) {
      val declarations =
          nodes.mapIndexed { index, label ->
            "n$index[\"${label.replace("&", "&amp;").replace("\"", "&quot;")}\"]"
          }
      return SummaryDiagramInput("flowchart LR\n" + declarations.joinToString(" --> "), "")
    }
  }
  return SummaryDiagramInput(null, value)
}

private sealed interface DiagramState {
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
) {
  val input = remember(value) { summaryDiagramInput(value) }
  Column(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(8.dp)) {
    if (title == null && input.prose.isNotBlank()) ModelResultContent(input.prose, preview = false)
    key(ownerIdentity, input.source) {
      val localView = remember { DiagramViewState() }
      MermaidDiagramSource(
          input.source, label, title, input.prose.takeIf { title != null }, viewState ?: localView)
    }
  }
}

@Composable
@OptIn(ExperimentalLayoutApi::class)
private fun MermaidDiagramSource(
    source: String?,
    label: String,
    title: String?,
    prose: String?,
    view: DiagramViewState
) {
  val state by
      produceState<DiagramState>(
          if (source == null) DiagramState.Unavailable else DiagramState.Loading, source) {
            if (source != null)
                try {
                  value = DiagramState.Ready(renderMermaidImage(MermaidRenderer.render(source).svg))
                } catch (cancelled: CancellationException) {
                  throw cancelled
                } catch (exception: Exception) {
                  value =
                      DiagramState.Failed(
                          exception.message?.take(240) ?: "Unable to render diagram")
                }
          }
  var showDiagram by view::showDiagram
  var showSource by view::showSource
  var zoom by view::zoom
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
            onClick = { showDiagram = !showDiagram },
            enabled = source != null && state !is DiagramState.Failed,
            accessibleName = "${if (showDiagram) "Hide" else "Show"} $label diagram",
            modifier =
                Modifier.semantics {
                  stateDescription =
                      when (state) {
                        DiagramState.Unavailable -> "Diagram unavailable"
                        DiagramState.Loading -> "Rendering diagram"
                        is DiagramState.Failed -> "Diagram unavailable"
                        is DiagramState.Ready -> if (showDiagram) "Expanded" else "Collapsed"
                      }
                }) {
              DesktopLineIcon(
                  if (showDiagram) DesktopIcon.ChevronDown else DesktopIcon.ChevronRight,
                  "",
                  iconSize = 16.dp)
              Text(if (showDiagram) "Hide diagram" else "Show diagram")
            }
      }
  prose
      ?.takeIf { it.isNotBlank() }
      ?.let { ModelResultContent(it, preview = false, style = IdeTypography.workspaceBody) }
  if (showDiagram) {
    FlowRow(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.End) {
      ChromeButton(
          onClick = { zoom = (zoom - 0.25f).coerceAtLeast(0.75f) },
          enabled = zoom > 0.75f,
          accessibleName = "Zoom out $label") {
            Text("−")
          }
      ChromeButton(onClick = { zoom = 1f }, accessibleName = "Reset zoom $label") {
        Text("${(zoom * 100).toInt()}%")
      }
      ChromeButton(
          onClick = { zoom = (zoom + 0.25f).coerceAtMost(2f) },
          enabled = zoom < 2f,
          accessibleName = "Zoom in $label") {
            Text("+")
          }
      ChromeButton(
          onClick = { showSource = !showSource }, accessibleName = "Mermaid source for $label") {
            Text(if (showSource) "Hide Mermaid" else "Mermaid source")
          }
    }
  }
  when (val current = state) {
    DiagramState.Unavailable -> Unit
    DiagramState.Loading ->
        if (showDiagram)
            Text("Rendering diagram…", color = SecondaryText, style = IdeTypography.compactBody)
    is DiagramState.Failed -> {
      Text(
          "Diagram unavailable: ${current.message}",
          color = Warning,
          style = IdeTypography.compactBody)
      ModelResultContent(requireNotNull(source), preview = false)
    }
    is DiagramState.Ready ->
        if (showDiagram) {
          val scale = LocalDensity.current.fontScale * zoom
          Box(
              Modifier.fillMaxWidth()
                  .clip(MiniOrcaShapes.control)
                  .background(EditorCanvas)
                  .horizontalScroll(rememberScrollState())
                  .padding(8.dp),
              contentAlignment = Alignment.Center) {
                Image(
                    current.image.bitmap,
                    "$label diagram\n$source",
                    modifier =
                        Modifier.size(
                            (current.image.width * scale).dp, (current.image.height * scale).dp))
              }
        }
  }
  if (showDiagram && showSource) ModelResultContent("```mermaid\n$source\n```", preview = false)
}
