package io.miniorca.desktop

import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.TooltipArea
import androidx.compose.foundation.background
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.IntrinsicSize
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.relocation.BringIntoViewRequester
import androidx.compose.foundation.relocation.bringIntoViewRequester
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.input.pointer.PointerEventPass
import androidx.compose.ui.input.pointer.PointerIcon
import androidx.compose.ui.input.pointer.PointerId
import androidx.compose.ui.input.pointer.pointerHoverIcon
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

enum class SourceLineEmphasis {
  None,
  FocusedLocation,
  SelectedSymbol,
  FocusedSelectedSymbol
}

internal enum class SourceGutterMarkerKind(val glyph: String) {
  FocusedLine("●"),
  SelectedDeclaration("▸"),
  Finding("◆"),
}

internal data class SourceGutterMarker(
    val line: Int,
    val kind: SourceGutterMarkerKind,
    val description: String,
)

/** Cached source-only work that does not change when focus or gutter markers move. */
internal data class SourceViewportRow(
    val line: Int,
    val highlightedText: AnnotatedString,
    val selection: SourceLineSelection?,
)

/**
 * SelectionContainer requires all source lines to stay composed for multi-line text selection.
 * Cache the expensive syntax and declaration mapping per loaded source instead of virtualizing
 * rows; focus and markers are derived separately as lightweight presentation state.
 */
internal fun sourceViewportRows(
    source: String,
    expandIndentation: Boolean,
    selectable: Boolean,
    symbols: List<SymbolInfo>,
): List<SourceViewportRow> =
    source.lines().mapIndexed { index, sourceLine ->
      val line = index + 1
      SourceViewportRow(
          line = line,
          highlightedText =
              highlightedCode(
                  if (expandIndentation) expandedEditorIndentation(sourceLine) else sourceLine),
          selection = if (selectable) sourceLineSelection(symbols, line) else null,
      )
    }

internal fun sourceGutterMarkers(
    activeFilePath: String?,
    selectedSymbol: SymbolInfo?,
    focusedLine: Int,
    findings: List<UnifiedFinding>,
    lineCount: Int,
): Map<Int, List<SourceGutterMarker>> {
  fun inRange(line: Int) = line in 1..lineCount
  val markers = mutableListOf<SourceGutterMarker>()
  if (inRange(focusedLine)) {
    markers +=
        SourceGutterMarker(
            focusedLine,
            SourceGutterMarkerKind.FocusedLine,
            "Focused line marker at line $focusedLine",
        )
  }
  selectedSymbol
      ?.takeIf { inRange(it.startLine) }
      ?.let { symbol ->
        markers +=
            SourceGutterMarker(
                symbol.startLine,
                SourceGutterMarkerKind.SelectedDeclaration,
                "Selected declaration ${symbol.name} marker at line ${symbol.startLine}",
            )
      }
  findings
      .asSequence()
      .filter { it.location.path == activeFilePath && inRange(it.location.startLine) }
      .sortedWith(compareBy<UnifiedFinding> { it.location.startLine }.thenBy { it.id })
      .forEach { finding ->
        val title = finding.title.ifBlank { finding.message }.ifBlank { "Known finding" }
        markers +=
            SourceGutterMarker(
                finding.location.startLine,
                SourceGutterMarkerKind.Finding,
                "Finding marker at line ${finding.location.startLine}: $title",
            )
      }
  return markers.groupBy { it.line }
}

fun sourceLineEmphasis(
    line: Int,
    selectedSymbol: SymbolInfo?,
    focusedLine: Int
): SourceLineEmphasis =
    when {
      selectedSymbol != null &&
          line in selectedSymbol.startLine..selectedSymbol.endLine &&
          line == focusedLine -> SourceLineEmphasis.FocusedSelectedSymbol
      selectedSymbol != null && line in selectedSymbol.startLine..selectedSymbol.endLine ->
          SourceLineEmphasis.SelectedSymbol
      line == focusedLine && focusedLine > 0 -> SourceLineEmphasis.FocusedLocation
      else -> SourceLineEmphasis.None
    }

internal fun sourceLineDescription(line: Int, emphasis: SourceLineEmphasis): String =
    when (emphasis) {
      SourceLineEmphasis.None -> "Line $line"
      SourceLineEmphasis.FocusedLocation -> "Line $line, focused location"
      SourceLineEmphasis.SelectedSymbol -> "Line $line, selected declaration"
      SourceLineEmphasis.FocusedSelectedSymbol ->
          "Line $line, focused location in selected declaration"
    }

internal fun sourceLineContentDescription(
    line: Int,
    emphasis: SourceLineEmphasis,
    declarationSymbol: SymbolInfo?,
): String =
    sourceLineDescription(line, emphasis) +
        declarationSymbol?.let { ", selectable declaration ${it.name}" }.orEmpty()

/**
 * Widens only a source line's leading whitespace for easier visual nesting in the read-only editor.
 */
internal fun expandedEditorIndentation(sourceLine: String): String {
  val indentationEnd = sourceLine.indexOfFirst { !it.isWhitespace() }
  if (indentationEnd <= 0) return sourceLine
  val indentation = sourceLine.take(indentationEnd)
  val visualIndentationWidth =
      indentation.count { it != '\t' } + indentation.count { it == '\t' } * 4
  val additionalSpaces = maxOf(1, visualIndentationWidth / 4)
  return buildString(sourceLine.length + indentationEnd) {
    indentation.forEach { character ->
      append(character)
      append(character)
    }
    repeat(additionalSpaces) { append(' ') }
    append(sourceLine.drop(indentationEnd))
  }
}

@Composable
@OptIn(ExperimentalFoundationApi::class)
internal fun SourceEditorPane(
    project: ProjectAnalysis?,
    selected: ProjectFileInfo?,
    symbols: List<SymbolInfo>,
    selectedSymbol: SymbolInfo?,
    focusedLine: Int,
    findings: List<UnifiedFinding>,
    onSourceLineSelected: (SourceLineSelection) -> Unit,
) {
  val source =
      when {
        selected != null ->
            if (selected.binary) "Binary file: source preview is unavailable." else selected.content
        project != null -> project.summary
        else ->
            "Select Import to analyze a project. Mini-Orca indexes only policy-eligible project files."
      }
  val canSelectSource = selected != null && !selected.binary
  val rows =
      remember(source, selected?.path, selected?.contentHash, canSelectSource, symbols) {
        sourceViewportRows(
            source = source,
            expandIndentation = selected != null,
            selectable = canSelectSource,
            symbols = symbols,
        )
      }
  val markersByLine =
      remember(selected?.path, selectedSymbol, focusedLine, findings, rows.size) {
        sourceGutterMarkers(selected?.path, selectedSymbol, focusedLine, findings, rows.size)
      }
  val focusLine = focusedLine.takeIf { it in 1..rows.size }
  val focusLineRequester = remember { BringIntoViewRequester() }
  LaunchedEffect(selected?.path, selected?.contentHash, focusLine) {
    if (focusLine != null) focusLineRequester.bringIntoView()
  }
  SelectionContainer {
    Column(Modifier.fillMaxSize().padding(vertical = 10.dp)) {
      if (focusedLine > 0) {
        Text(
            "Editor context · ${selectedSymbol?.name ?: "line $focusedLine"} · line $focusedLine",
            color = SecondaryText,
            fontSize = 11.sp,
            modifier = Modifier.padding(start = 18.dp, bottom = 8.dp),
        )
      }
      Row(Modifier.fillMaxSize().verticalScroll(rememberScrollState())) {
        SourceGutter(
            rows = rows,
            selectedSymbol = selectedSymbol,
            focusedLine = focusedLine,
            markersByLine = markersByLine,
            onSourceLineSelected = onSourceLineSelected,
            modifier = Modifier.width(62.dp),
        )
        Column(
            Modifier.weight(1f).horizontalScroll(rememberScrollState()).width(IntrinsicSize.Max)) {
              rows.forEach { row ->
                val emphasis = sourceLineEmphasis(row.line, selectedSymbol, focusedLine)
                val declarationSymbol = row.selection?.symbol
                Box(
                    Modifier.fillMaxWidth()
                        .height(20.dp)
                        .background(sourceLineBackground(emphasis))
                        .semantics {
                          contentDescription =
                              sourceLineContentDescription(row.line, emphasis, declarationSymbol)
                        }
                        .sourceLineSelectionTap(row.selection) {
                          onSourceLineSelected(requireNotNull(row.selection))
                        }
                        .then(
                            if (row.line == focusLine)
                                Modifier.bringIntoViewRequester(focusLineRequester)
                            else Modifier),
                ) {
                  Text(
                      text = row.highlightedText,
                      color = PrimaryText,
                      fontFamily =
                          if (selected != null) FontFamily.Monospace else FontFamily.Default,
                      fontSize = 13.sp,
                      lineHeight = 20.sp,
                      softWrap = false,
                  )
                }
              }
            }
      }
    }
  }
}

@Composable
@OptIn(ExperimentalFoundationApi::class)
private fun SourceGutter(
    rows: List<SourceViewportRow>,
    selectedSymbol: SymbolInfo?,
    focusedLine: Int,
    markersByLine: Map<Int, List<SourceGutterMarker>>,
    onSourceLineSelected: (SourceLineSelection) -> Unit,
    modifier: Modifier = Modifier,
) {
  Column(modifier.background(AppBackground)) {
    rows.forEach { row ->
      val emphasis = sourceLineEmphasis(row.line, selectedSymbol, focusedLine)
      Row(
          Modifier.fillMaxWidth()
              .height(20.dp)
              .background(sourceLineBackground(emphasis))
              .sourceLineSelectionTap(row.selection) {
                onSourceLineSelected(requireNotNull(row.selection))
              },
      ) {
        Text(
            row.line.toString().padStart(4),
            color = if (row.line == focusedLine) SelectionText else FaintText,
            fontFamily = FontFamily.Monospace,
            fontSize = 12.sp,
            lineHeight = 20.sp,
            modifier = Modifier.width(38.dp),
        )
        markersByLine[row.line].orEmpty().forEach { marker ->
          TooltipArea(tooltip = { GutterMarkerTooltip(marker.description) }) {
            Text(
                marker.kind.glyph,
                color = gutterMarkerColor(marker.kind),
                fontFamily = FontFamily.Monospace,
                fontSize = 10.sp,
                lineHeight = 20.sp,
                modifier = Modifier.semantics { contentDescription = marker.description },
            )
          }
        }
      }
    }
  }
}

@Composable
private fun GutterMarkerTooltip(description: String) {
  Text(
      description,
      color = PrimaryText,
      fontSize = 11.sp,
      modifier = Modifier.background(StrongSurface).padding(6.dp),
  )
}

private fun gutterMarkerColor(kind: SourceGutterMarkerKind): Color =
    when (kind) {
      SourceGutterMarkerKind.FocusedLine -> SelectionText
      SourceGutterMarkerKind.SelectedDeclaration -> FocusAccent
      SourceGutterMarkerKind.Finding -> Warning
    }

@Composable
internal fun EditorPane(
    project: ProjectAnalysis?,
    selected: ProjectFileInfo?,
    symbols: List<SymbolInfo>,
    selectedSymbol: SymbolInfo?,
    focusedLine: Int,
    findings: List<UnifiedFinding>,
    onSourceLineSelected: (SourceLineSelection) -> Unit,
) {
  Box(Modifier.fillMaxSize()) {
    SourceEditorPane(
        project, selected, symbols, selectedSymbol, focusedLine, findings, onSourceLineSelected)
  }
}

private fun sourceLineBackground(emphasis: SourceLineEmphasis): Color =
    when (emphasis) {
      SourceLineEmphasis.FocusedSelectedSymbol -> SelectionSurface.copy(alpha = 0.72f)
      SourceLineEmphasis.FocusedLocation -> SelectionSurface.copy(alpha = 0.52f)
      SourceLineEmphasis.SelectedSymbol,
      SourceLineEmphasis.None -> Color.Transparent
    }

internal fun sourceTapSelectsContext(dragged: Boolean): Boolean = !dragged

private fun Modifier.sourceLineSelectionTap(
    selection: SourceLineSelection?,
    onTap: () -> Unit
): Modifier =
    if (selection == null) this
    else
        sourceLineTap(onTap).let { modifier ->
          if (selection.symbol == null) modifier
          else modifier.pointerHoverIcon(PointerIcon.Hand, overrideDescendants = true)
        }

private fun Modifier.sourceLineTap(onTap: () -> Unit): Modifier =
    pointerInput(onTap) {
      awaitPointerEventScope {
        var activePointer: PointerId? = null
        var dragged = false
        while (true) {
          val event = awaitPointerEvent(PointerEventPass.Initial)
          event.changes.forEach { change ->
            if (activePointer == null && !change.previousPressed && change.pressed) {
              activePointer = change.id
              dragged = false
            }
            if (change.id == activePointer) {
              dragged = dragged || change.position != change.previousPosition
              if (change.previousPressed && !change.pressed) {
                if (sourceTapSelectsContext(dragged)) onTap()
                activePointer = null
              }
            }
          }
        }
      }
    }
