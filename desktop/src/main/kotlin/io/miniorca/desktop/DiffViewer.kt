package io.miniorca.desktop

import androidx.compose.foundation.ScrollState
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.IntrinsicSize
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.rememberTextMeasurer
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp

data class DiffCell(val lineNumber: Int?, val text: String, val change: String)

data class DiffRow(val before: DiffCell?, val proposed: DiffCell?)

internal fun diffLineBackground(change: String?): Color =
    when (change) {
      "added" -> DiffAddedBackground
      "removed" -> DiffRemovedBackground
      else -> Color.Transparent
    }

fun sideBySideDiffRows(diff: UnifiedDiff): List<DiffRow> {
  val rows = mutableListOf<DiffRow>()
  val removed = mutableListOf<DiffLine>()
  val added = mutableListOf<DiffLine>()
  fun flush() {
    repeat(maxOf(removed.size, added.size)) { index ->
      rows +=
          DiffRow(
              removed.getOrNull(index)?.let {
                DiffCell(it.oldLine.takeIf { n -> n > 0 }, it.text, it.kind)
              },
              added.getOrNull(index)?.let {
                DiffCell(it.newLine.takeIf { n -> n > 0 }, it.text, it.kind)
              })
    }
    removed.clear()
    added.clear()
  }
  diff.lines.forEach { line ->
    when (line.kind) {
      "removed" -> {
        if (added.isNotEmpty()) flush()
        removed += line
      }
      "added" -> added += line
      else -> {
        flush()
        rows +=
            DiffRow(
                DiffCell(line.oldLine.takeIf { it > 0 }, line.text, line.kind),
                DiffCell(line.newLine.takeIf { it > 0 }, line.text, line.kind))
      }
    }
  }
  flush()
  return rows
}

@Composable
internal fun DiffViewer(diff: UnifiedDiff?, modifier: Modifier = Modifier) {
  if (diff == null) {
    SystemStateMessage(
        "Composed diff unavailable",
        "Validate the latest draft to view the read-only composed diff.",
        modifier = modifier)
    return
  }
  BoxWithConstraints(
      modifier.fillMaxSize().semantics { contentDescription = "Read-only composed diff" }) {
        val readableWidth = maxWidth / LocalDensity.current.fontScale
        var preferredSideBySide by
            remember(diff.oldPath, diff.newPath) { mutableStateOf<Boolean?>(null) }
        val sideBySide = preferredSideBySide ?: (readableWidth >= 620.dp)
        val rows = remember(diff) { sideBySideDiffRows(diff) }
        val rowHeight = readOnlyCodeRowHeight()
        Column(Modifier.fillMaxSize()) {
          Row(
              Modifier.fillMaxWidth().padding(horizontal = 8.dp, vertical = 4.dp),
              horizontalArrangement = Arrangement.End) {
                ChromeTab(
                    selected = sideBySide,
                    onClick = { preferredSideBySide = true },
                    accessibleName = "Side-by-side diff") {
                      Text("Side-by-side", style = IdeTypography.compactBody)
                    }
                ChromeTab(
                    selected = !sideBySide,
                    onClick = { preferredSideBySide = false },
                    accessibleName = "Unified diff") {
                      Text("Unified", style = IdeTypography.compactBody)
                    }
              }
          val verticalScroll = rememberScrollState()
          val density = LocalDensity.current
          val largestLine = diff.lines.maxOfOrNull { maxOf(it.oldLine, it.newLine) } ?: 0
          val numberSize =
              rememberTextMeasurer()
                  .measure(
                      "9".repeat(maxOf(4, largestLine.toString().length)),
                      style = readOnlyCodeStyle)
                  .size
          val numberWidth = with(density) { numberSize.width.toDp() } + 12.dp
          SelectionContainer(Modifier.weight(1f)) {
            if (sideBySide) {
              Row(Modifier.fillMaxSize(), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                DiffColumn("Current", verticalScroll, Modifier.weight(1f)) {
                  rows.forEachIndexed { index, row ->
                    DiffCodeRow(
                        row.before, "Current", index, diffRowHeight(row, rowHeight), numberWidth)
                  }
                }
                DiffColumn("Candidate", verticalScroll, Modifier.weight(1f)) {
                  rows.forEachIndexed { index, row ->
                    DiffCodeRow(
                        row.proposed,
                        "Candidate",
                        index,
                        diffRowHeight(row, rowHeight),
                        numberWidth)
                  }
                }
              }
            } else {
              DiffColumn("Current → Candidate", verticalScroll, Modifier.fillMaxSize()) {
                diff.lines.forEachIndexed { index, line ->
                  DiffCodeRow(
                      DiffCell(
                          (if (line.kind == "added") line.newLine else line.oldLine).takeIf {
                            it > 0
                          },
                          line.text,
                          line.kind),
                      "Unified",
                      index,
                      rowHeight * (line.text.count { it == '\n' } + 1),
                      numberWidth)
                }
              }
            }
          }
        }
      }
}

private fun diffRowHeight(row: DiffRow, lineHeight: Dp): Dp =
    lineHeight *
        maxOf(
            row.before?.text?.count { it == '\n' }?.plus(1) ?: 1,
            row.proposed?.text?.count { it == '\n' }?.plus(1) ?: 1)

@Composable
private fun DiffColumn(
    label: String,
    verticalScroll: ScrollState,
    modifier: Modifier,
    content: @Composable () -> Unit
) {
  Column(
      modifier
          .fillMaxHeight()
          .clip(MiniOrcaShapes.interactiveCard)
          .background(EditorCanvas)
          .border(1.dp, PaneSeparator, MiniOrcaShapes.interactiveCard)
          .testTag("diff-$label-column")) {
        Text(
            label,
            Modifier.fillMaxWidth()
                .background(HeaderSurface)
                .padding(horizontal = 12.dp, vertical = 10.dp),
            color = PrimaryText,
            style = IdeTypography.workspaceMetadata,
            fontWeight = FontWeight.SemiBold)
        Box(
            Modifier.weight(1f)
                .fillMaxWidth()
                .verticalScroll(verticalScroll)
                .testTag("diff-$label-vertical")) {
              Column(
                  Modifier.fillMaxWidth()
                      .horizontalScroll(rememberScrollState())
                      .width(IntrinsicSize.Max)
                      .padding(vertical = 12.dp)
                      .testTag("diff-$label-horizontal")) {
                    content()
                  }
            }
      }
}

@Composable
private fun DiffCodeRow(cell: DiffCell?, side: String, index: Int, height: Dp, numberWidth: Dp) {
  val marker =
      when (cell?.change) {
        "added" -> "+"
        "removed" -> "−"
        else -> ""
      }
  val tint =
      when (cell?.change) {
        "added" -> Success
        "removed" -> Error
        else -> FaintText
      }
  Row(
      Modifier.fillMaxWidth()
          .height(height)
          .background(diffLineBackground(cell?.change))
          .testTag("diff-$side-row-$index")
          .semantics {
            contentDescription =
                cell?.let { "$side ${it.change} line ${it.lineNumber ?: "unknown"}" }
                    ?: "$side has no corresponding line"
          }) {
        Text(
            cell?.lineNumber?.toString().orEmpty(),
            Modifier.width(numberWidth).padding(end = 8.dp),
            color = FaintText,
            style = readOnlyCodeStyle,
            softWrap = false,
            textAlign = TextAlign.End)
        Text(
            marker,
            Modifier.width(20.dp * LocalDensity.current.fontScale),
            color = tint,
            style = readOnlyCodeStyle,
            softWrap = false)
        Text(
            remember(cell?.text) { highlightedCode(cell?.text.orEmpty()) },
            Modifier.padding(end = 12.dp),
            color = PrimaryText,
            style = readOnlyCodeStyle,
            softWrap = false)
      }
}
