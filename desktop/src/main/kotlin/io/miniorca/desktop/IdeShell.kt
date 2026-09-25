package io.miniorca.desktop

import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.TooltipArea
import androidx.compose.foundation.background
import androidx.compose.foundation.focusable
import androidx.compose.foundation.gestures.detectDragGestures
import androidx.compose.foundation.hoverable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.interaction.collectIsFocusedAsState
import androidx.compose.foundation.interaction.collectIsHoveredAsState
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.relocation.BringIntoViewRequester
import androidx.compose.foundation.relocation.bringIntoViewRequester
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.drawWithContent
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.input.key.KeyEventType
import androidx.compose.ui.input.key.key
import androidx.compose.ui.input.key.onPreviewKeyEvent
import androidx.compose.ui.input.key.type
import androidx.compose.ui.input.pointer.PointerIcon
import androidx.compose.ui.input.pointer.pointerHoverIcon
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.stateDescription
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import java.awt.Cursor

/** Shared shell geometry keeps the terminal below every pane and exposes each rounded perimeter. */
@Composable
internal fun WorkspaceFrame(
    rail: @Composable () -> Unit,
    panes: @Composable RowScope.() -> Unit,
    terminal: @Composable (Float) -> Unit,
    editorPanes: (@Composable (Float, Float) -> Unit)? = null,
    modifier: Modifier = Modifier,
) {
  Row(
      modifier
          .fillMaxWidth()
          .background(MiniOrcaPalette.editorCanvas)
          .padding(
              top = WORKSPACE_FRAME_INSET.dp,
              end = WORKSPACE_FRAME_INSET.dp,
              bottom = WORKSPACE_FRAME_INSET.dp)) {
        rail()
        Spacer(Modifier.width(WORKSPACE_FRAME_INSET.dp))
        BoxWithConstraints(Modifier.weight(1f).fillMaxHeight()) {
          val workspaceHeight = maxHeight.value
          Column(Modifier.fillMaxHeight()) {
            BoxWithConstraints(Modifier.weight(1f).fillMaxWidth()) {
              if (editorPanes == null) {
                Row(Modifier.fillMaxSize(), content = panes)
              } else {
                // Include the rail, its gap and the frame end inset in the resolver's allocation.
                val scale = maxOf(1f, LocalDensity.current.fontScale)
                editorPanes(
                    maxWidth.value + TOOL_WINDOW_BAR_WIDTH * scale + 2 * WORKSPACE_FRAME_INSET,
                    maxHeight.value)
              }
            }
            Spacer(Modifier.height(WORKSPACE_FRAME_INSET.dp))
            terminal(workspaceHeight)
          }
        }
      }
}

@Composable
@OptIn(ExperimentalFoundationApi::class)
internal fun ToolWindowBar(
    activeToolWindow: LeftToolWindow,
    onSelect: (LeftToolWindow) -> Unit,
    modifier: Modifier = Modifier,
    onOpenTerminal: () -> Unit,
    onOpenCommands: () -> Unit = {},
    commandsFocusRequester: FocusRequester? = null,
    onOpenModels: () -> Unit = {},
    modelsFocusRequester: FocusRequester? = null,
) {
  var utilityHasFocus by remember { mutableStateOf(false) }
  val terminalReveal = remember { BringIntoViewRequester() }
  val commandsReveal = remember { BringIntoViewRequester() }
  val modelsReveal = remember { BringIntoViewRequester() }
  var focusedUtility by remember { mutableStateOf("Terminal") }
  LaunchedEffect(utilityHasFocus, focusedUtility) {
    if (utilityHasFocus) {
      when (focusedUtility) {
        "Commands" -> commandsReveal.bringIntoView()
        "Models" -> modelsReveal.bringIntoView()
        else -> terminalReveal.bringIntoView()
      }
    }
  }
  var focusedToolWindow by remember(activeToolWindow) { mutableStateOf(activeToolWindow) }
  var tabGroupHasFocus by remember { mutableStateOf(false) }
  val railWidth = (TOOL_WINDOW_BAR_WIDTH * maxOf(1f, LocalDensity.current.fontScale)).dp
  Column(
      modifier
          .width(railWidth)
          .fillMaxHeight()
          .background(ActivityRail)
          .padding(vertical = 8.dp)
          .verticalScroll(rememberScrollState())
          .onFocusChanged { tabGroupHasFocus = it.isFocused }
          .focusable()
          .onPreviewKeyEvent { event ->
            if (utilityHasFocus || event.type != KeyEventType.KeyDown)
                return@onPreviewKeyEvent false
            val interaction =
                tabGroupInteraction(workspaceRailOrder, focusedToolWindow, tabGroupKey(event.key))
                    ?: return@onPreviewKeyEvent false
            focusedToolWindow = interaction.focused
            interaction.activate?.let(onSelect)
            true
          },
      horizontalAlignment = Alignment.CenterHorizontally,
  ) {
    workspaceRailOrder.forEach { toolWindow ->
      WorkspaceNavigationEntry(
          toolWindow,
          activeToolWindow == toolWindow,
          tabGroupHasFocus && toolWindow == focusedToolWindow,
          { onSelect(toolWindow) })
    }
    IdeHorizontalSeparator(Modifier.padding(horizontal = 10.dp, vertical = 8.dp))
    ChromeButton(
        onClick = onOpenTerminal,
        modifier =
            Modifier.fillMaxWidth()
                .padding(horizontal = 2.dp, vertical = 2.dp)
                .onFocusChanged {
                  utilityHasFocus = it.hasFocus
                  if (it.isFocused) focusedUtility = "Terminal"
                }
                .bringIntoViewRequester(terminalReveal),
        contentPadding = PaddingValues(horizontal = 2.dp, vertical = 8.dp),
        accessibleName = "Terminal · Open or focus; may start a local shell",
        tooltip = "Open or focus Terminal · may start a local shell") {
          Column(horizontalAlignment = Alignment.CenterHorizontally) {
            DesktopLineIcon(DesktopIcon.Terminal, "Terminal", iconSize = 20.dp)
            Spacer(Modifier.height(4.dp))
            Text("Terminal", style = IdeTypography.action, maxLines = 1, softWrap = false)
          }
        }
    ChromeButton(
        onClick = onOpenCommands,
        modifier =
            Modifier.fillMaxWidth()
                .padding(horizontal = 2.dp, vertical = 2.dp)
                .then(commandsFocusRequester?.let { Modifier.focusRequester(it) } ?: Modifier)
                .onFocusChanged {
                  utilityHasFocus = it.hasFocus
                  if (it.isFocused) focusedUtility = "Commands"
                }
                .bringIntoViewRequester(commandsReveal),
        contentPadding = PaddingValues(horizontal = 2.dp, vertical = 8.dp),
        accessibleName = "Commands · Open actions",
        tooltip = "Open commands") {
          Column(horizontalAlignment = Alignment.CenterHorizontally) {
            DesktopLineIcon(DesktopIcon.Search, "Commands", iconSize = 20.dp)
            Spacer(Modifier.height(4.dp))
            Text("Commands", style = IdeTypography.action, maxLines = 1, softWrap = false)
          }
        }
    ChromeButton(
        onClick = onOpenModels,
        modifier =
            Modifier.fillMaxWidth()
                .padding(horizontal = 2.dp, vertical = 2.dp)
                .then(modelsFocusRequester?.let { Modifier.focusRequester(it) } ?: Modifier)
                .onFocusChanged {
                  utilityHasFocus = it.hasFocus
                  if (it.isFocused) focusedUtility = "Models"
                }
                .bringIntoViewRequester(modelsReveal),
        contentPadding = PaddingValues(horizontal = 2.dp, vertical = 8.dp),
        accessibleName = "Models · Configured model details",
        tooltip = "Configured model details") {
          Column(horizontalAlignment = Alignment.CenterHorizontally) {
            DesktopLineIcon(DesktopIcon.Document, "Models", iconSize = 20.dp)
            Spacer(Modifier.height(4.dp))
            Text("Models", style = IdeTypography.action, maxLines = 1, softWrap = false)
          }
        }
  }
}

@Composable
@OptIn(ExperimentalFoundationApi::class)
private fun WorkspaceNavigationEntry(
    toolWindow: LeftToolWindow,
    selected: Boolean,
    focused: Boolean,
    onSelect: () -> Unit
) {
  val label = leftToolWindowLabel(toolWindow)
  val reveal = remember { BringIntoViewRequester() }
  LaunchedEffect(focused) { if (focused) reveal.bringIntoView() }
  TooltipArea(tooltip = { IdeControlTooltip(label) }) {
    ChromeButton(
        onClick = onSelect,
        modifier =
            Modifier.fillMaxWidth()
                .padding(horizontal = 2.dp, vertical = 2.dp)
                .bringIntoViewRequester(reveal)
                .drawWithContent {
                  drawContent()
                  if (selected)
                      drawLine(
                          SelectionAccent,
                          Offset(2.dp.toPx(), 12.dp.toPx()),
                          Offset(2.dp.toPx(), size.height - 12.dp.toPx()),
                          2.dp.toPx(),
                          cap = StrokeCap.Round)
                }
                .semantics {
                  contentDescription = toolWindowSemanticsLabel(toolWindow, selected, focused)
                  this.selected = selected
                },
        contentPadding = PaddingValues(horizontal = 2.dp, vertical = 8.dp),
        role = Role.Tab,
        selected = selected,
        focusHighlight = focused) {
          Column(horizontalAlignment = Alignment.CenterHorizontally) {
            DesktopLineIcon(
                leftToolWindowIcon(toolWindow),
                label,
                iconSize = 20.dp,
                tint = if (selected) MiniOrcaPalette.identityAccent else SecondaryText)
            Spacer(Modifier.height(4.dp))
            Text(label, style = IdeTypography.action, maxLines = 1, softWrap = false)
          }
        }
  }
}

@Composable
internal fun DockedToolWindow(
    title: String,
    content: @Composable (Modifier) -> Unit,
    modifier: Modifier = Modifier,
    onClose: (() -> Unit)? = null,
    showHeader: Boolean = true,
) {
  Column(
      modifier
          .fillMaxHeight()
          .clip(MiniOrcaShapes.workspace)
          .background(ToolWindowSurface)
          .semantics { contentDescription = "$title tool window" },
  ) {
    if (showHeader) ToolWindowHeader(title, onClose)
    content(Modifier.fillMaxWidth().weight(1f))
  }
}

@Composable
private fun ToolWindowHeader(title: String, onClose: (() -> Unit)?) {
  IdePaneHeader(
      title = title,
      actionsBelow = true,
      actions = {
        onClose?.let { close ->
          ChromeButton(
              onClick = close,
              contentPadding = PaddingValues(4.dp),
              accessibleName = "Close $title drawer",
          ) {
            DesktopLineIcon(DesktopIcon.Close, "Close $title drawer", iconSize = 16.dp)
          }
        }
      },
  )
}

@Composable
internal fun EditorArea(content: @Composable () -> Unit, modifier: Modifier = Modifier) {
  Box(
      modifier.fillMaxHeight().clip(MiniOrcaShapes.workspace).background(EditorCanvas).semantics {
        contentDescription = "Editor area"
      }) {
        content()
      }
}

@Composable
internal fun TerminalDock(
    layout: DesktopLayoutState,
    state: TerminalWorkspaceState,
    onOpen: () -> Unit,
    onCollapse: () -> Unit,
    tabActions: TerminalTabActions,
    onHeightDelta: (Float) -> Unit,
    onHeightCommit: () -> Unit,
    content: @Composable (Modifier) -> Unit,
    controlModifier: Modifier = Modifier,
    modifier: Modifier = Modifier,
    effectiveHeight: Float = layout.bottomHeight,
) {
  Column(
      modifier
          .fillMaxWidth()
          .then(if (layout.bottomCollapsed) Modifier else Modifier.height(effectiveHeight.dp))
          .clip(MiniOrcaShapes.workspace)
          .background(ToolWindowSurface)) {
        if (!layout.bottomCollapsed) HorizontalResizableDivider(onHeightDelta, onHeightCommit)
        TerminalBar(
            state,
            layout.bottomCollapsed,
            if (layout.bottomCollapsed) onOpen else onCollapse,
            tabActions = tabActions,
            controlModifier = controlModifier)
        if (!layout.bottomCollapsed) {
          // The native Swing terminal cannot inherit the Compose perimeter clip.
          content(
              Modifier.fillMaxWidth().weight(1f).padding(start = 8.dp, end = 8.dp, bottom = 8.dp))
        }
      }
}

@Composable
internal fun TerminalBar(
    state: TerminalWorkspaceState,
    collapsed: Boolean,
    onToggle: () -> Unit,
    tabActions: TerminalTabActions,
    modifier: Modifier = Modifier,
    controlModifier: Modifier = Modifier,
) {
  Column(modifier.fillMaxWidth().clip(MiniOrcaShapes.workspace).background(ToolWindowSurface)) {
    Row(
        Modifier.fillMaxWidth().heightIn(min = 38.dp).padding(horizontal = 8.dp),
        verticalAlignment = Alignment.CenterVertically) {
          ChromeButton(
              onClick = onToggle,
              selected = !collapsed,
              modifier =
                  controlModifier.semantics {
                    contentDescription =
                        if (collapsed) "Open terminal · Ctrl+Shift+T" else "Collapse terminal"
                    stateDescription = if (collapsed) "Collapsed" else "Expanded"
                  }) {
                DesktopLineIcon(
                    if (collapsed) DesktopIcon.ChevronRight else DesktopIcon.ChevronDown,
                    "Terminal",
                    iconSize = 16.dp)
                Spacer(Modifier.width(4.dp))
                Text("Terminal", fontSize = 12.sp)
              }
          if (!collapsed) TerminalTabs(state, tabActions, Modifier.weight(1f))
        }
  }
}

internal fun leftToolWindowIcon(toolWindow: LeftToolWindow): DesktopIcon =
    when (toolWindow) {
      LeftToolWindow.Summary -> DesktopIcon.Summary
      LeftToolWindow.Analysis -> DesktopIcon.Analysis
      LeftToolWindow.Performance -> DesktopIcon.Performance
      LeftToolWindow.Problems -> DesktopIcon.Problems
      LeftToolWindow.Security -> DesktopIcon.Security
      LeftToolWindow.Editor -> DesktopIcon.Editor
    }

internal fun toolWindowSemanticsLabel(
    toolWindow: LeftToolWindow,
    selected: Boolean,
    focused: Boolean = false,
): String =
    "${leftToolWindowLabel(toolWindow)} tool window${if (selected) ", selected" else ", not selected"}${if (focused) ", focused" else ""}"

internal const val KEYBOARD_SPLITTER_STEP = 12f

internal fun verticalSplitterKeyboardDelta(key: Key): Float? =
    when (key) {
      Key.DirectionLeft -> -KEYBOARD_SPLITTER_STEP
      Key.DirectionRight -> KEYBOARD_SPLITTER_STEP
      else -> null
    }

internal fun horizontalSplitterKeyboardDelta(key: Key): Float? =
    when (key) {
      Key.DirectionDown -> -KEYBOARD_SPLITTER_STEP
      Key.DirectionUp -> KEYBOARD_SPLITTER_STEP
      else -> null
    }

@Composable
internal fun ResizableDivider(
    onDelta: (Float) -> Unit,
    onCommit: () -> Unit,
    onFocusChanged: (Boolean) -> Unit = {},
) {
  val density = LocalDensity.current
  val interaction = remember { MutableInteractionSource() }
  val focused by interaction.collectIsFocusedAsState()
  val hovered by interaction.collectIsHoveredAsState()
  val currentOnDelta by rememberUpdatedState(onDelta)
  val currentOnCommit by rememberUpdatedState(onCommit)
  var commitPending by remember { mutableStateOf(false) }
  LaunchedEffect(commitPending) {
    if (commitPending) {
      currentOnCommit()
      commitPending = false
    }
  }
  Box(
      Modifier.fillMaxHeight()
          .width(RESIZE_DIVIDER_WIDTH.dp)
          .pointerHoverIcon(PointerIcon(Cursor.getPredefinedCursor(Cursor.E_RESIZE_CURSOR)))
          .semantics { contentDescription = "Resize adjacent panes. Use Left or Right Arrow." }
          .hoverable(interaction)
          .onFocusChanged { onFocusChanged(it.isFocused) }
          .focusable(interactionSource = interaction)
          .onPreviewKeyEvent { event ->
            if (event.type != KeyEventType.KeyDown) return@onPreviewKeyEvent false
            val delta = verticalSplitterKeyboardDelta(event.key) ?: return@onPreviewKeyEvent false
            currentOnDelta(delta)
            commitPending = true
            true
          }
          .pointerInput(Unit) {
            detectDragGestures(
                onDrag = { change, amount ->
                  change.consume()
                  currentOnDelta(with(density) { amount.x.toDp().value })
                },
                onDragEnd = { commitPending = true },
            )
          },
      contentAlignment = Alignment.Center,
  ) {
    Box(
        Modifier.width(2.dp)
            .height(24.dp)
            .background(
                if (focused) FocusAccent else if (hovered) SelectionAccent else ControlBorder,
                MiniOrcaShapes.pill))
  }
}

@Composable
internal fun HorizontalResizableDivider(onDelta: (Float) -> Unit, onCommit: () -> Unit) {
  val density = LocalDensity.current
  val interaction = remember { MutableInteractionSource() }
  val focused by interaction.collectIsFocusedAsState()
  val hovered by interaction.collectIsHoveredAsState()
  val currentOnDelta by rememberUpdatedState(onDelta)
  val currentOnCommit by rememberUpdatedState(onCommit)
  var commitPending by remember { mutableStateOf(false) }
  LaunchedEffect(commitPending) {
    if (commitPending) {
      currentOnCommit()
      commitPending = false
    }
  }
  Box(
      Modifier.fillMaxWidth()
          .height(RESIZE_DIVIDER_WIDTH.dp)
          .pointerHoverIcon(PointerIcon(Cursor.getPredefinedCursor(Cursor.N_RESIZE_CURSOR)))
          .semantics { contentDescription = "Resize bottom pane. Use Up or Down Arrow." }
          .hoverable(interaction)
          .focusable(interactionSource = interaction)
          .onPreviewKeyEvent { event ->
            if (event.type != KeyEventType.KeyDown) return@onPreviewKeyEvent false
            val delta = horizontalSplitterKeyboardDelta(event.key) ?: return@onPreviewKeyEvent false
            currentOnDelta(delta)
            commitPending = true
            true
          }
          .pointerInput(Unit) {
            detectDragGestures(
                onDrag = { change, amount ->
                  change.consume()
                  currentOnDelta(-with(density) { amount.y.toDp().value })
                },
                onDragEnd = { commitPending = true },
            )
          },
      contentAlignment = Alignment.Center,
  ) {
    Box(
        Modifier.height(2.dp)
            .width(24.dp)
            .background(
                if (focused) FocusAccent else if (hovered) SelectionAccent else ControlBorder,
                MiniOrcaShapes.pill))
  }
}
