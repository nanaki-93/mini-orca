package io.miniorca.desktop

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.material.AlertDialog
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.input.key.KeyEvent
import androidx.compose.ui.input.key.KeyEventType
import androidx.compose.ui.input.key.key
import androidx.compose.ui.input.key.onPreviewKeyEvent
import androidx.compose.ui.input.key.type
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

internal enum class CommandSearchResultType(val label: String) {
  File("File"),
  Symbol("Symbol"),
  Action("Action"),
}

internal data class CommandSearchResult(
    val type: CommandSearchResultType,
    val label: String,
    val detail: String,
    val accessibleDescription: String,
    val path: String = "",
    val symbol: SymbolInfo? = null,
    val action: String = "",
)

internal sealed interface CommandSearchActivation {
  data class File(val path: String) : CommandSearchActivation

  data class Symbol(val symbol: SymbolInfo) : CommandSearchActivation

  data class Action(val action: String) : CommandSearchActivation
}

internal fun commandSearchTitle(mode: PaletteMode): String =
    when (mode) {
      PaletteMode.Files -> "Go to file · ⌘P"
      PaletteMode.Symbols -> "Go to symbol · ⌘⇧O"
      PaletteMode.Actions -> "Focused action · ⌘K"
    }

internal fun commandSearchHint(mode: PaletteMode): String =
    "${commandSearchTitle(mode)} · ↑↓ select · Enter activate · Esc close"

internal fun commandSearchResults(
    mode: PaletteMode,
    query: String,
    files: List<IndexedFile>,
    symbols: List<SymbolInfo>,
    analysis: FileAnalysis?,
    hasActiveFile: Boolean,
): List<CommandSearchResult> {
  val normalizedQuery = query.trim()
  return when (mode) {
    PaletteMode.Files ->
        files
            .asSequence()
            .filter { it.path.contains(normalizedQuery, ignoreCase = true) }
            .sortedBy { it.path.lowercase() }
            .take(MAX_COMMAND_RESULTS)
            .map { file ->
              CommandSearchResult(
                  type = CommandSearchResultType.File,
                  label = file.path,
                  detail = "Indexed relative path",
                  accessibleDescription = "File ${file.path}",
                  path = file.path,
              )
            }
            .toList()
    PaletteMode.Symbols ->
        symbols
            .asSequence()
            .filter { it.name.contains(normalizedQuery, ignoreCase = true) }
            .sortedWith(compareBy<SymbolInfo> { it.name.lowercase() }.thenBy { it.startLine })
            .take(MAX_COMMAND_RESULTS)
            .map { symbol ->
              CommandSearchResult(
                  type = CommandSearchResultType.Symbol,
                  label = symbol.name,
                  detail = "${symbol.kind} · line ${symbol.startLine}",
                  accessibleDescription = "Symbol ${symbol.kind} ${symbol.name}",
                  symbol = symbol,
              )
            }
            .toList()
    PaletteMode.Actions ->
        availableCommandActions(analysis, hasActiveFile)
            .asSequence()
            .filter { commandActionLabel(it).contains(normalizedQuery, ignoreCase = true) }
            .sortedBy(::commandActionLabel)
            .map { action ->
              CommandSearchResult(
                  type = CommandSearchResultType.Action,
                  label = commandActionLabel(action),
                  detail = "Current file scope only",
                  accessibleDescription = "Action ${commandActionLabel(action)}",
                  action = action,
              )
            }
            .toList()
  }
}

internal fun nextCommandSearchSelection(
    selectedIndex: Int,
    resultCount: Int,
    direction: Int,
): Int {
  if (resultCount == 0) return -1
  val current = selectedIndex.coerceIn(0, resultCount - 1)
  return (current + direction + resultCount) % resultCount
}

internal fun commandSearchActivation(result: CommandSearchResult): CommandSearchActivation? =
    when (result.type) {
      CommandSearchResultType.File ->
          result.path.takeIf(String::isNotBlank)?.let(CommandSearchActivation::File)
      CommandSearchResultType.Symbol -> result.symbol?.let(CommandSearchActivation::Symbol)
      CommandSearchResultType.Action ->
          result.action.takeIf(String::isNotBlank)?.let(CommandSearchActivation::Action)
    }

@Composable
internal fun CommandPaletteDialog(
    mode: PaletteMode,
    query: String,
    onQuery: (String) -> Unit,
    files: List<IndexedFile>,
    symbols: List<SymbolInfo>,
    analysis: FileAnalysis?,
    hasActiveFile: Boolean,
    onSelectFile: (String) -> Unit,
    onSelectSymbol: (SymbolInfo) -> Unit,
    onSelectAction: (String) -> Unit,
    onDismiss: () -> Unit,
) {
  val results = commandSearchResults(mode, query, files, symbols, analysis, hasActiveFile)
  val filterFocusRequester = FocusRequester()
  var selectedIndex by
      remember(mode, query, results.map(CommandSearchResult::label)) {
        mutableStateOf(if (results.isEmpty()) -1 else 0)
      }
  fun activate(result: CommandSearchResult) {
    when (val activation = commandSearchActivation(result)) {
      is CommandSearchActivation.File -> onSelectFile(activation.path)
      is CommandSearchActivation.Symbol -> onSelectSymbol(activation.symbol)
      is CommandSearchActivation.Action -> onSelectAction(activation.action)
      null -> Unit
    }
  }
  fun handleKey(event: KeyEvent): Boolean {
    if (event.type != KeyEventType.KeyDown) return false
    return when (event.key) {
      Key.DirectionDown -> {
        selectedIndex = nextCommandSearchSelection(selectedIndex, results.size, 1)
        true
      }
      Key.DirectionUp -> {
        selectedIndex = nextCommandSearchSelection(selectedIndex, results.size, -1)
        true
      }
      Key.Enter -> {
        results.getOrNull(selectedIndex)?.let(::activate)
        true
      }
      else -> false
    }
  }
  AlertDialog(
      onDismissRequest = onDismiss,
      title = { Text(commandSearchTitle(mode)) },
      text = {
        Column {
          CompactSingleLineField(
              value = query,
              onValueChange = onQuery,
              label = { Text("Filter ${mode.name.lowercase()}") },
              modifier =
                  Modifier.fillMaxWidth()
                      .focusRequester(filterFocusRequester)
                      .onPreviewKeyEvent(::handleKey))
          Text(
              commandSearchHint(mode),
              color = SecondaryText,
              fontSize = 10.sp,
              modifier = Modifier.padding(top = 5.dp))
          Spacer(Modifier.height(6.dp))
          if (results.isEmpty()) {
            SystemStateMessage(commandSearchEmptyTitle(mode), commandSearchEmptyDetail(mode))
          } else {
            results.forEachIndexed { index, result ->
              CommandSearchEntry(
                  result = result,
                  selected = index == selectedIndex,
                  onClick = {
                    selectedIndex = index
                    activate(result)
                  })
            }
          }
        }
      },
      confirmButton = {
        FocusFlowButton(onClick = onDismiss, tone = ActionTone.Neutral) { Text("Close") }
      },
  )
  LaunchedEffect(Unit) { filterFocusRequester.requestFocus() }
}

internal fun availableCommandActions(
    analysis: FileAnalysis?,
    hasActiveFile: Boolean = true,
): List<String> {
  if (!hasActiveFile) return listOf("open_performance")
  return buildList {
    add("open_performance")
    addAll(listOf("fix", "refactor", "document", "create_declaration"))
    if (analysis?.status.equals("fresh", ignoreCase = true)) add("refresh_file_analysis")
  }
}

internal fun commandActionLabel(action: String): String =
    when (action) {
      "create_declaration" -> "Create declaration"
      "refresh_file_analysis" -> "Refresh file analysis"
      "open_performance" -> "Open Performance workspace"
      else -> action.replaceFirstChar { it.uppercase() }
    }

internal fun commandSearchEmptyTitle(mode: PaletteMode): String =
    when (mode) {
      PaletteMode.Files -> "No indexed file matches"
      PaletteMode.Symbols -> "No active-file symbol matches"
      PaletteMode.Actions -> "No focused action is available"
    }

internal fun commandSearchEmptyDetail(mode: PaletteMode): String =
    when (mode) {
      PaletteMode.Files -> "Change the filter to search indexed relative paths."
      PaletteMode.Symbols -> "Open a file with indexed symbols or change the filter."
      PaletteMode.Actions -> "Open an indexed file before choosing a scoped action."
    }

@Composable
private fun CommandSearchEntry(
    result: CommandSearchResult,
    selected: Boolean,
    onClick: () -> Unit,
) {
  FocusFlowButton(
      onClick = onClick,
      modifier =
          Modifier.fillMaxWidth().padding(top = 3.dp).semantics {
            contentDescription =
                "${result.accessibleDescription}, ${if (selected) "selected" else "not selected"}"
          },
      tone = ActionTone.Navigation,
      selected = selected,
  ) {
    Text(
        "${result.type.label} · ${result.label} · ${result.detail}",
        fontFamily = FontFamily.Monospace,
        fontSize = 11.sp,
        maxLines = 1)
  }
}

private const val MAX_COMMAND_RESULTS = 12
