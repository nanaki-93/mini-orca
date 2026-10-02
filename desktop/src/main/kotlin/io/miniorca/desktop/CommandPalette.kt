package io.miniorca.desktop

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.runtime.withFrameNanos
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.input.key.KeyEvent
import androidx.compose.ui.input.key.KeyEventType
import androidx.compose.ui.input.key.key
import androidx.compose.ui.input.key.onPreviewKeyEvent
import androidx.compose.ui.input.key.type
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.style.TextOverflow
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
      PaletteMode.Files -> "Files · ⌘P"
      PaletteMode.Symbols -> "Symbols · active file · ⌘⇧O"
      PaletteMode.Actions -> "Commands"
    }

internal fun commandSearchFieldLabel(mode: PaletteMode): String =
    when (mode) {
      PaletteMode.Files -> "Filter files"
      PaletteMode.Symbols -> "Filter active-file symbols"
      PaletteMode.Actions -> "Filter commands"
    }

internal fun commandSearchModeLabel(mode: PaletteMode): String =
    when (mode) {
      PaletteMode.Files -> "Files"
      PaletteMode.Symbols -> "Symbols"
      PaletteMode.Actions -> "Commands"
    }

internal fun commandSearchHint(mode: PaletteMode): String =
    "↑↓ select · Enter activate · Space on result · Esc close"

internal fun commandSearchResults(
    mode: PaletteMode,
    query: String,
    files: List<IndexedFile>,
    symbols: List<SymbolInfo>,
    hasActiveFile: Boolean,
): List<CommandSearchResult> {
  val normalizedQuery = query.trim()
  return when (mode) {
    PaletteMode.Files ->
        files
            .asSequence()
            .filter { it.path.contains(normalizedQuery, ignoreCase = true) }
            .sortedBy { it.path.lowercase() }
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
        availableCommandActions(hasActiveFile)
            .asSequence()
            .filter { commandActionLabel(it).contains(normalizedQuery, ignoreCase = true) }
            .sortedBy(::commandActionLabel)
            .map { action ->
              CommandSearchResult(
                  type = CommandSearchResultType.Action,
                  label = commandActionLabel(action),
                  detail = commandActionDetail(action),
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
    onMode: (PaletteMode) -> Unit,
    files: List<IndexedFile>,
    symbols: List<SymbolInfo>,
    hasActiveFile: Boolean,
    onSelectFile: (String) -> Unit,
    onSelectSymbol: (SymbolInfo) -> Unit,
    onSelectAction: (String) -> Unit,
    onDismiss: () -> Unit,
    blockedReason: String? = null,
) {
  val results =
      remember(mode, query, files, symbols, hasActiveFile) {
        commandSearchResults(mode, query, files, symbols, hasActiveFile)
      }
  val resultFocus = remember(results) { results.map { FocusRequester() } }
  var pendingResultFocus by remember(results) { mutableStateOf<Int?>(null) }
  val filterFocusRequester = remember { FocusRequester() }
  val resultListState = rememberLazyListState()
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
  fun handleKey(event: KeyEvent, fromResult: Boolean = false): Boolean {
    if (event.type != KeyEventType.KeyDown) return false
    return when (event.key) {
      Key.DirectionDown -> {
        selectedIndex = nextCommandSearchSelection(selectedIndex, results.size, 1)
        if (fromResult) pendingResultFocus = selectedIndex
        true
      }
      Key.DirectionUp -> {
        selectedIndex = nextCommandSearchSelection(selectedIndex, results.size, -1)
        if (fromResult) pendingResultFocus = selectedIndex
        true
      }
      Key.Enter -> {
        if (fromResult) return false
        results.getOrNull(selectedIndex)?.let(::activate)
        true
      }
      else -> false
    }
  }
  IdeDialog(
      onDismissRequest = onDismiss,
      focusSafeActionOnOpen = false,
      title = { Text(commandSearchTitle(mode)) },
      content = {
        Column {
          Row(Modifier.fillMaxWidth()) {
            PaletteMode.entries.forEach { candidate ->
              ChromeTab(
                  onClick = {
                    onMode(candidate)
                    filterFocusRequester.requestFocus()
                  },
                  selected = mode == candidate,
                  modifier = Modifier.weight(1f),
                  accessibleName = "${commandSearchModeLabel(candidate)} search mode",
              ) {
                Text(
                    commandSearchModeLabel(candidate),
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
              }
            }
          }
          Spacer(Modifier.height(8.dp))
          CompactSingleLineField(
              value = query,
              onValueChange = onQuery,
              label = commandSearchFieldLabel(mode),
              showLabel = false,
              modifier =
                  Modifier.fillMaxWidth().focusRequester(filterFocusRequester).onPreviewKeyEvent {
                    handleKey(it)
                  })
          Text(
              commandSearchHint(mode),
              color = SecondaryText,
              fontSize = 10.sp,
              modifier = Modifier.padding(top = 5.dp))
          Spacer(Modifier.height(6.dp))
          blockedReason?.let {
            SystemStateMessage("Cannot prepare creation", it)
            Spacer(Modifier.height(6.dp))
          }
          if (results.isEmpty()) {
            SystemStateMessage(commandSearchEmptyTitle(mode), commandSearchEmptyDetail(mode))
          } else {
            Text(
                "${results.size} results",
                color = SecondaryText,
                style = IdeTypography.workspaceMetadata)
            Box(Modifier.fillMaxWidth().heightIn(max = COMMAND_RESULT_HEIGHT)) {
              LazyColumn(
                  Modifier.fillMaxWidth().testTag("palette-results"), state = resultListState) {
                    itemsIndexed(results) { index, result ->
                      CommandSearchEntry(
                          result = result,
                          selected = index == selectedIndex,
                          modifier =
                              Modifier.focusRequester(resultFocus[index])
                                  .onFocusChanged { if (it.isFocused) selectedIndex = index }
                                  .onPreviewKeyEvent { handleKey(it, fromResult = true) },
                          onClick = {
                            selectedIndex = index
                            activate(result)
                          },
                      )
                    }
                  }
            }
          }
        }
      },
      actions = {
        MiniOrcaButton(onClick = onDismiss, tone = ActionTone.Neutral) { Text("Close") }
      },
  )
  LaunchedEffect(mode) { filterFocusRequester.requestFocus() }
  LaunchedEffect(selectedIndex, results, pendingResultFocus) {
    if (selectedIndex in results.indices) {
      val visible =
          resultListState.layoutInfo.visibleItemsInfo.firstOrNull { it.index == selectedIndex }
      if (visible == null ||
          visible.offset < resultListState.layoutInfo.viewportStartOffset ||
          visible.offset + visible.size > resultListState.layoutInfo.viewportEndOffset) {
        resultListState.scrollToItem(selectedIndex)
      }
      if (pendingResultFocus == selectedIndex) {
        withFrameNanos {}
        resultFocus[selectedIndex].requestFocus()
        pendingResultFocus = null
      }
    }
  }
}

internal fun availableCommandActions(hasActiveFile: Boolean = true): List<String> = buildList {
  addAll(
      listOf("start_analysis", "open_analysis", "open_bugs", "open_performance", "open_security"))
  if (hasActiveFile) addAll(listOf("fix", "refactor", "document", "create_function", "create_type"))
}

internal fun commandActionWorkspace(action: String): Workspace? =
    when (action) {
      "start_analysis",
      "open_analysis" -> Workspace.Analysis
      "open_bugs" -> Workspace.Bugs
      "open_performance" -> Workspace.Performance
      "open_security" -> Workspace.Security
      "fix",
      "refactor",
      "document",
      "create_function",
      "create_type" -> Workspace.Editor
      else -> null
    }

internal fun commandActionLabel(action: String): String =
    when (action) {
      "start_analysis" -> "Start analysis"
      "open_analysis" -> "View analysis progress"
      "open_bugs" -> "View Bugs results"
      "open_performance" -> "View Performance results"
      "open_security" -> "View Security results"
      "create_function" -> "New Go function"
      "create_type" -> "New Go type"
      else -> action.replaceFirstChar { it.uppercase() }
    }

internal fun commandActionDetail(action: String): String =
    when (action) {
      "start_analysis" -> "Whole project · review scope and consent before starting"
      "open_analysis" -> "Current project run · navigation only"
      "open_bugs",
      "open_performance",
      "open_security" -> "Project results · navigation only"
      else -> "Current file · prepare an edit"
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
      PaletteMode.Actions -> "Change the filter to find project analysis, results or file actions."
    }

@Composable
private fun CommandSearchEntry(
    result: CommandSearchResult,
    selected: Boolean,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
  MiniOrcaButton(
      onClick = onClick,
      modifier =
          modifier.fillMaxWidth().padding(top = 3.dp).semantics {
            contentDescription =
                "${result.accessibleDescription}, ${if (selected) "selected" else "not selected"}"
          },
      tone = ActionTone.Navigation,
      selected = selected,
  ) {
    Column(Modifier.fillMaxWidth()) {
      Text(
          result.label,
          color = PrimaryText,
          style =
              if (result.type == CommandSearchResultType.File) IdeTypography.resultCode
              else IdeTypography.resultHeading)
      Text(
          "${result.type.label} · ${result.detail}",
          color = SecondaryText,
          style = IdeTypography.workspaceMetadata)
    }
  }
}

private val COMMAND_RESULT_HEIGHT = 288.dp
