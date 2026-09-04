package io.miniorca.desktop

import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.TooltipArea
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.focusable
import androidx.compose.foundation.gestures.detectDragGestures
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.AlertDialog
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.drawWithContent
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.input.key.KeyEventType
import androidx.compose.ui.input.key.key
import androidx.compose.ui.input.key.onPreviewKeyEvent
import androidx.compose.ui.input.key.type
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

@Composable
@OptIn(ExperimentalFoundationApi::class)
internal fun ToolWindowBar(
    activeToolWindow: LeftToolWindow,
    onSelect: (LeftToolWindow) -> Unit,
    modifier: Modifier = Modifier,
) {
  var focusedToolWindow by remember(activeToolWindow) { mutableStateOf(activeToolWindow) }
  var tabGroupHasFocus by remember { mutableStateOf(false) }
  val enlargedText = LocalDensity.current.fontScale > 1.15f
  Column(
      modifier
          .width(TOOL_WINDOW_BAR_WIDTH.dp)
          .fillMaxHeight()
          .background(Chrome)
          .padding(vertical = 8.dp)
          .verticalScroll(rememberScrollState())
          .onFocusChanged { tabGroupHasFocus = it.hasFocus }
          .focusable()
          .onPreviewKeyEvent { event ->
            if (event.type != KeyEventType.KeyDown) return@onPreviewKeyEvent false
            val interaction =
                tabGroupInteraction(
                    LeftToolWindow.entries.toList(), focusedToolWindow, tabGroupKey(event.key))
                    ?: return@onPreviewKeyEvent false
            focusedToolWindow = interaction.focused
            interaction.activate?.let(onSelect)
            true
          },
      horizontalAlignment = Alignment.CenterHorizontally,
  ) {
    LeftToolWindow.entries.forEach { toolWindow ->
      val label = leftToolWindowLabel(toolWindow)
      val selected = toolWindow == activeToolWindow
      TooltipArea(tooltip = { ToolWindowTooltip(label) }) {
        ChromeButton(
            onClick = { onSelect(toolWindow) },
            modifier =
                Modifier.fillMaxWidth()
                    .heightIn(min = 76.dp)
                    .drawWithContent {
                      drawContent()
                      if (selected)
                          drawLine(
                              SelectionAccent,
                              Offset(1.dp.toPx(), 0f),
                              Offset(1.dp.toPx(), size.height),
                              2.dp.toPx())
                    }
                    .semantics {
                      contentDescription =
                          toolWindowSemanticsLabel(
                              toolWindow,
                              selected,
                              focused = tabGroupHasFocus && toolWindow == focusedToolWindow)
                      this.selected = selected
                    },
            contentPadding = PaddingValues(horizontal = 4.dp, vertical = 8.dp),
            role = Role.Tab,
            selected = selected,
            focusHighlight = tabGroupHasFocus && toolWindow == focusedToolWindow,
        ) {
          Column(horizontalAlignment = Alignment.CenterHorizontally) {
            DesktopLineIcon(
                leftToolWindowIcon(toolWindow),
                label,
                iconSize = 24.dp,
                tint = if (selected) SelectionText else SecondaryText)
            Spacer(Modifier.height(6.dp))
            Text(
                if (toolWindow == LeftToolWindow.Performance && enlargedText) "Perf." else label,
                color = if (selected) SelectionText else SecondaryText,
                fontSize = 11.sp,
                lineHeight = 15.sp,
                textAlign = TextAlign.Center,
                maxLines = 2,
                overflow = TextOverflow.Ellipsis,
            )
          }
        }
      }
    }
  }
}

@Composable
private fun ToolWindowTooltip(label: String) {
  Text(
      label,
      color = PrimaryText,
      fontSize = 11.sp,
      modifier =
          Modifier.background(StrongSurface)
              .border(androidx.compose.foundation.BorderStroke(1.dp, Border))
              .padding(6.dp))
}

@Composable
internal fun DockedToolWindow(
    title: String,
    content: @Composable (Modifier) -> Unit,
    modifier: Modifier = Modifier,
) {
  Column(
      modifier
          .fillMaxHeight()
          .background(Panel)
          .border(androidx.compose.foundation.BorderStroke(1.dp, Border))
          .semantics { contentDescription = "$title tool window" },
  ) {
    Text(
        title,
        color = PrimaryText,
        fontSize = 12.sp,
        fontWeight = FontWeight.SemiBold,
        modifier =
            Modifier.fillMaxWidth().height(36.dp).padding(horizontal = 10.dp, vertical = 10.dp))
    content(Modifier.fillMaxWidth().weight(1f))
  }
}

@Composable
internal fun EditorArea(content: @Composable () -> Unit, modifier: Modifier = Modifier) {
  Box(
      modifier.fillMaxHeight().background(AppBackground).semantics {
        contentDescription = "Editor area"
      }) {
        content()
      }
}

@Composable
internal fun BottomToolWindowRegion(
    layout: DesktopLayoutState,
    availableToolWindows: List<BottomToolWindow>,
    summaries: Map<BottomToolWindow, BottomToolWindowSummary>,
    onSelect: (BottomToolWindow) -> Unit,
    onCollapse: () -> Unit,
    onHeightDelta: (Float) -> Unit,
    onHeightCommit: () -> Unit,
    content: @Composable (BottomToolWindow, Modifier) -> Unit,
    tabModifier: Modifier = Modifier,
    modifier: Modifier = Modifier,
) {
  if (availableToolWindows.isEmpty()) return
  val activeToolWindow =
      layout.activeBottomToolWindow.takeIf { it in availableToolWindows }
          ?: availableToolWindows.first()
  val visibleSummary = bottomToolWindowSummary(activeToolWindow, summaries)
  val collapsed = layout.bottomCollapsed
  Column(
      modifier
          .fillMaxWidth()
          .height(if (collapsed) 40.dp else layout.bottomHeight.dp)
          .background(Chrome)
          .border(androidx.compose.foundation.BorderStroke(1.dp, Border)),
  ) {
    if (!collapsed) HorizontalResizableDivider(onHeightDelta, onHeightCommit)
    Row(
        Modifier.fillMaxWidth().height(38.dp).padding(horizontal = 8.dp),
        verticalAlignment = Alignment.CenterVertically) {
          BottomToolWindowTabs(
              availableToolWindows, activeToolWindow, summaries, onSelect, tabModifier)
          Text(
              visibleSummary?.text.orEmpty(),
              color = if (visibleSummary?.attention == true) Warning else SecondaryText,
              fontSize = 11.sp,
              maxLines = 1,
              modifier = Modifier.weight(1f).padding(start = 8.dp))
          ChromeButton(
              onClick = if (collapsed) ({ onSelect(activeToolWindow) }) else onCollapse,
          ) {
            Text(if (collapsed) "Open" else "Collapse", fontSize = 11.sp)
          }
        }
    if (!collapsed) content(activeToolWindow, Modifier.fillMaxWidth().weight(1f))
  }
}

@Composable
internal fun NarrowBottomToolWindowSummary(
    layout: DesktopLayoutState,
    availableToolWindows: List<BottomToolWindow>,
    summaries: Map<BottomToolWindow, BottomToolWindowSummary>,
    onOpen: () -> Unit,
    modifier: Modifier = Modifier,
) {
  if (availableToolWindows.isEmpty()) return
  val activeToolWindow =
      layout.activeBottomToolWindow.takeIf { it in availableToolWindows }
          ?: availableToolWindows.first()
  val summary = bottomToolWindowSummary(activeToolWindow, summaries)
  Row(
      modifier
          .fillMaxWidth()
          .height(38.dp)
          .background(Chrome)
          .border(androidx.compose.foundation.BorderStroke(1.dp, Border))
          .padding(horizontal = 8.dp)
          .semantics {
            contentDescription =
                "Bottom tools summary. ${bottomToolWindowLabel(activeToolWindow)} selected. ${summary?.text.orEmpty()}"
          },
      verticalAlignment = Alignment.CenterVertically,
  ) {
    Text(bottomToolWindowLabel(activeToolWindow), color = PrimaryText, fontSize = 11.sp)
    Text(
        summary?.text.orEmpty(),
        color = if (summary?.attention == true) Warning else SecondaryText,
        fontSize = 11.sp,
        maxLines = 1,
        modifier = Modifier.weight(1f).padding(start = 8.dp),
    )
    MiniOrcaButton(
        onClick = onOpen, tone = ActionTone.Navigation, density = ButtonDensity.Toolbar) {
          Text("Open", fontSize = 11.sp)
        }
  }
}

@Composable
internal fun BottomToolWindowOverlay(
    layout: DesktopLayoutState,
    availableToolWindows: List<BottomToolWindow>,
    summaries: Map<BottomToolWindow, BottomToolWindowSummary>,
    onSelect: (BottomToolWindow) -> Unit,
    onDismiss: () -> Unit,
    content: @Composable (BottomToolWindow, Modifier) -> Unit,
    tabModifier: Modifier = Modifier,
) {
  if (availableToolWindows.isEmpty()) return
  val activeToolWindow =
      layout.activeBottomToolWindow.takeIf { it in availableToolWindows }
          ?: availableToolWindows.first()
  AlertDialog(
      onDismissRequest = onDismiss,
      title = { Text("Bottom tools · ${bottomToolWindowLabel(activeToolWindow)}") },
      text = {
        Column(Modifier.fillMaxWidth().semantics { contentDescription = "Bottom tools overlay" }) {
          BottomToolWindowTabs(
              availableToolWindows, activeToolWindow, summaries, onSelect, tabModifier)
          Box(Modifier.fillMaxWidth().heightIn(min = 180.dp, max = 360.dp)) {
            content(activeToolWindow, Modifier.fillMaxSize())
          }
        }
      },
      confirmButton = {
        MiniOrcaButton(onClick = onDismiss, tone = ActionTone.Primary) { Text("Close") }
      },
  )
}

@Composable
private fun BottomToolWindowTabs(
    availableToolWindows: List<BottomToolWindow>,
    activeToolWindow: BottomToolWindow,
    summaries: Map<BottomToolWindow, BottomToolWindowSummary>,
    onSelect: (BottomToolWindow) -> Unit,
    modifier: Modifier,
) {
  var focusedToolWindow by remember(activeToolWindow) { mutableStateOf(activeToolWindow) }
  var tabGroupHasFocus by remember { mutableStateOf(false) }
  Row(
      modifier
          .onFocusChanged { tabGroupHasFocus = it.hasFocus }
          .focusable()
          .onPreviewKeyEvent { event ->
            if (event.type != KeyEventType.KeyDown) return@onPreviewKeyEvent false
            val interaction =
                tabGroupInteraction(availableToolWindows, focusedToolWindow, tabGroupKey(event.key))
                    ?: return@onPreviewKeyEvent false
            focusedToolWindow = interaction.focused
            interaction.activate?.let(onSelect)
            true
          }) {
        availableToolWindows.forEach { toolWindow ->
          val selected = toolWindow == activeToolWindow
          ChromeTab(
              onClick = { onSelect(toolWindow) },
              selected = selected,
              focusHighlight = tabGroupHasFocus && toolWindow == focusedToolWindow,
              modifier =
                  Modifier.semantics {
                    contentDescription =
                        bottomToolWindowTabDescription(
                            toolWindow,
                            selected,
                            summaries[toolWindow]?.text,
                            focused = tabGroupHasFocus && toolWindow == focusedToolWindow)
                  },
          ) {
            DesktopLineIcon(
                bottomToolWindowIcon(toolWindow),
                bottomToolWindowLabel(toolWindow),
                iconSize = 16.dp)
            Spacer(Modifier.width(4.dp))
            Text(bottomToolWindowLabel(toolWindow), fontSize = 12.sp)
          }
        }
      }
}

internal fun bottomToolWindowLabel(toolWindow: BottomToolWindow): String =
    when (toolWindow) {
      BottomToolWindow.Problems -> "Bugs & Problems"
      BottomToolWindow.Checks -> "Checks"
      BottomToolWindow.Output -> "Output"
      BottomToolWindow.Terminal -> "Terminal · Preview"
    }

internal fun bottomToolWindowIcon(toolWindow: BottomToolWindow): DesktopIcon =
    when (toolWindow) {
      BottomToolWindow.Problems -> DesktopIcon.Problems
      BottomToolWindow.Checks -> DesktopIcon.Summary
      BottomToolWindow.Output -> DesktopIcon.Editor
      BottomToolWindow.Terminal -> DesktopIcon.Terminal
    }

internal fun bottomToolWindowTabDescription(
    toolWindow: BottomToolWindow,
    selected: Boolean,
    summary: String?,
    focused: Boolean = false,
): String =
    "${bottomToolWindowLabel(toolWindow)} tool window tab${summary?.let { ", $it" }.orEmpty()}, ${if (selected) "selected" else "not selected"}${if (focused) ", focused" else ""}"

/**
 * A failed background tab is visible in the collapsed summary without changing the selected tab.
 */
internal fun bottomToolWindowSummary(
    activeToolWindow: BottomToolWindow,
    summaries: Map<BottomToolWindow, BottomToolWindowSummary>,
): BottomToolWindowSummary? =
    summaries.values.firstOrNull { it.attention } ?: summaries[activeToolWindow]

internal fun leftToolWindowIcon(toolWindow: LeftToolWindow): DesktopIcon =
    when (toolWindow) {
      LeftToolWindow.Summary -> DesktopIcon.Summary
      LeftToolWindow.Analysis -> DesktopIcon.Analysis
      LeftToolWindow.Performance -> DesktopIcon.Performance
      LeftToolWindow.Problems -> DesktopIcon.Problems
      LeftToolWindow.Editor -> DesktopIcon.Editor
    }

internal fun toolWindowSemanticsLabel(
    toolWindow: LeftToolWindow,
    selected: Boolean,
    focused: Boolean = false,
): String =
    "${leftToolWindowLabel(toolWindow)} tool window${if (selected) ", selected" else ", not selected"}${if (focused) ", focused" else ""}"

@Composable
internal fun ResizableDivider(onDelta: (Float) -> Unit, onCommit: () -> Unit) {
  val density = LocalDensity.current
  val currentOnDelta by rememberUpdatedState(onDelta)
  val currentOnCommit by rememberUpdatedState(onCommit)
  Box(
      Modifier.fillMaxHeight().width(8.dp).pointerInput(Unit) {
        detectDragGestures(
            onDrag = { change, amount ->
              change.consume()
              currentOnDelta(with(density) { amount.x.toDp().value })
            },
            onDragEnd = currentOnCommit,
        )
      },
      contentAlignment = Alignment.Center,
  ) {
    Box(Modifier.fillMaxHeight().width(1.dp).background(Border))
  }
}

@Composable
internal fun HorizontalResizableDivider(onDelta: (Float) -> Unit, onCommit: () -> Unit) {
  val density = LocalDensity.current
  val currentOnDelta by rememberUpdatedState(onDelta)
  val currentOnCommit by rememberUpdatedState(onCommit)
  Box(
      Modifier.fillMaxWidth().height(6.dp).pointerInput(Unit) {
        detectDragGestures(
            onDrag = { change, amount ->
              change.consume()
              currentOnDelta(-with(density) { amount.y.toDp().value })
            },
            onDragEnd = currentOnCommit,
        )
      },
      contentAlignment = Alignment.Center,
  ) {
    Box(Modifier.fillMaxWidth().height(1.dp).background(Border))
  }
}
