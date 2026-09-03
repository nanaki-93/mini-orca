package io.miniorca.desktop

import androidx.compose.foundation.background
import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.relocation.BringIntoViewRequester
import androidx.compose.foundation.relocation.bringIntoViewRequester
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.input.pointer.PointerIcon
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.input.pointer.PointerEventPass
import androidx.compose.ui.input.pointer.PointerId
import androidx.compose.ui.input.pointer.pointerHoverIcon
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

enum class SourceLineEmphasis { None, FocusedLocation, SelectedSymbol, FocusedSelectedSymbol }

fun sourceLineEmphasis(line: Int, selectedSymbol: SymbolInfo?, focusedLine: Int): SourceLineEmphasis = when {
    selectedSymbol != null && line in selectedSymbol.startLine..selectedSymbol.endLine && line == focusedLine -> SourceLineEmphasis.FocusedSelectedSymbol
    selectedSymbol != null && line in selectedSymbol.startLine..selectedSymbol.endLine -> SourceLineEmphasis.SelectedSymbol
    line == focusedLine && focusedLine > 0 -> SourceLineEmphasis.FocusedLocation
    else -> SourceLineEmphasis.None
}

internal fun sourceLineDescription(line: Int, emphasis: SourceLineEmphasis): String = when (emphasis) {
    SourceLineEmphasis.None -> "Line $line"
    SourceLineEmphasis.FocusedLocation -> "Line $line, focused location"
    SourceLineEmphasis.SelectedSymbol -> "Line $line, selected declaration"
    SourceLineEmphasis.FocusedSelectedSymbol -> "Line $line, focused location in selected declaration"
}

internal fun sourceLineContentDescription(
    line: Int,
    emphasis: SourceLineEmphasis,
    declarationSymbol: SymbolInfo?,
): String = sourceLineDescription(line, emphasis) +
    declarationSymbol?.let { ", selectable declaration ${it.name}" }.orEmpty()

/** Widens only a source line's leading whitespace for easier visual nesting in the read-only editor. */
internal fun expandedEditorIndentation(sourceLine: String): String {
    val indentationEnd = sourceLine.indexOfFirst { !it.isWhitespace() }
    if (indentationEnd <= 0) return sourceLine
    val indentation = sourceLine.take(indentationEnd)
    val visualIndentationWidth = indentation.count { it != '\t' } + indentation.count { it == '\t' } * 4
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
    onSourceLineSelected: (SourceLineSelection) -> Unit,
) {
    val source = when {
        selected != null -> if (selected.binary) "Binary file: source preview is unavailable." else selected.content
        project != null -> project.summary
        else -> "Select Import to analyze a project. Mini-Orca indexes only policy-eligible project files."
    }
    val canSelectSource = selected != null && !selected.binary
    val sourceLines = source.lines()
    val focusLine = focusedLine.takeIf { it in 1..sourceLines.size }
    val focusLineRequester = remember { BringIntoViewRequester() }
    LaunchedEffect(selected?.path, selected?.contentHash, focusLine) {
        if (focusLine != null) focusLineRequester.bringIntoView()
    }
    SelectionContainer {
        Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).horizontalScroll(rememberScrollState()).padding(18.dp)) {
            if (focusedLine > 0) {
                Text(
                    "Editor context · ${selectedSymbol?.name ?: "line $focusedLine"} · line $focusedLine",
                    color = SecondaryText,
                    fontSize = 11.sp,
                    modifier = Modifier.padding(bottom = 10.dp),
                )
            }
            sourceLines.forEachIndexed { index, sourceLine ->
                val lineNumber = index + 1
                val emphasis = sourceLineEmphasis(lineNumber, selectedSymbol, focusedLine)
                val sourceSelection = if (canSelectSource) sourceLineSelection(symbols, lineNumber) else null
                val declarationSymbol = sourceSelection?.symbol
                Row(
                    Modifier.fillMaxWidth()
                        .background(sourceLineBackground(emphasis, declarationSymbol != null))
                        .semantics {
                            contentDescription = sourceLineContentDescription(lineNumber, emphasis, declarationSymbol)
                        }
                        .sourceLineSelectionTap(sourceSelection) {
                            onSourceLineSelected(requireNotNull(sourceSelection))
                        }
                        .then(if (lineNumber == focusLine) Modifier.bringIntoViewRequester(focusLineRequester) else Modifier),
                ) {
                    Text(
                        lineNumber.toString().padStart(4),
                        color = if (declarationSymbol != null) CyanAccent else SecondaryText,
                        fontFamily = FontFamily.Monospace,
                        fontSize = 12.sp,
                        lineHeight = 20.sp,
                        modifier = Modifier.width(46.dp),
                    )
                    Text(
                        text = highlightedCode(if (selected != null) expandedEditorIndentation(sourceLine) else sourceLine),
                        color = PrimaryText,
                        fontFamily = if (selected != null) FontFamily.Monospace else FontFamily.Default,
                        fontSize = 13.sp,
                        lineHeight = 20.sp,
                    )
                }
            }
        }
    }
}

@Composable
internal fun EditorPane(
    project: ProjectAnalysis?,
    selected: ProjectFileInfo?,
    symbols: List<SymbolInfo>,
    selectedSymbol: SymbolInfo?,
    focusedLine: Int,
    onSourceLineSelected: (SourceLineSelection) -> Unit,
) {
    Box(Modifier.fillMaxSize()) {
        SourceEditorPane(project, selected, symbols, selectedSymbol, focusedLine, onSourceLineSelected)
    }
}

private fun sourceLineBackground(emphasis: SourceLineEmphasis, selectable: Boolean): Color = when (emphasis) {
    SourceLineEmphasis.FocusedSelectedSymbol -> Accent.copy(alpha = 0.30f)
    SourceLineEmphasis.SelectedSymbol -> Card
    SourceLineEmphasis.FocusedLocation -> Accent.copy(alpha = 0.18f)
    SourceLineEmphasis.None -> if (selectable) Accent.copy(alpha = 0.08f) else Color.Transparent
}

internal fun sourceTapSelectsContext(dragged: Boolean): Boolean = !dragged

private fun Modifier.sourceLineSelectionTap(selection: SourceLineSelection?, onTap: () -> Unit): Modifier =
    if (selection == null) this else sourceLineTap(onTap).let { modifier ->
        if (selection.symbol == null) modifier else modifier.pointerHoverIcon(PointerIcon.Hand, overrideDescendants = true)
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
