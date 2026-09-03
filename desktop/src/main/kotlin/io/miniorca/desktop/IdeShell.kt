package io.miniorca.desktop

import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.TooltipArea
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.gestures.detectDragGestures
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

@Composable
@OptIn(ExperimentalFoundationApi::class)
internal fun ToolWindowBar(
    activeToolWindow: LeftToolWindow,
    onSelect: (LeftToolWindow) -> Unit,
    modifier: Modifier = Modifier,
) {
  Column(
      modifier
          .width(52.dp)
          .fillMaxHeight()
          .background(Panel)
          .border(androidx.compose.foundation.BorderStroke(1.dp, Border))
          .padding(vertical = 6.dp),
      horizontalAlignment = Alignment.CenterHorizontally,
  ) {
    LeftToolWindow.entries.forEach { toolWindow ->
      val label = leftToolWindowLabel(toolWindow)
      val selected = toolWindow == activeToolWindow
      TooltipArea(tooltip = { ToolWindowTooltip(label) }) {
        FocusFlowButton(
            onClick = { onSelect(toolWindow) },
            modifier =
                Modifier.padding(horizontal = 5.dp, vertical = 2.dp).semantics {
                  contentDescription = toolWindowSemanticsLabel(toolWindow, selected)
                  this.selected = selected
                },
            tone = ActionTone.Navigation,
            density = ButtonDensity.Toolbar,
            selected = selected,
        ) {
          Text(toolWindowGlyph(toolWindow), fontSize = 11.sp, fontWeight = FontWeight.Bold)
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
        fontSize = 11.sp,
        fontWeight = FontWeight.SemiBold,
        modifier =
            Modifier.fillMaxWidth().height(32.dp).padding(horizontal = 10.dp, vertical = 9.dp))
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
    summaries: Map<BottomToolWindow, String>,
    onSelect: (BottomToolWindow) -> Unit,
    onCollapse: () -> Unit,
    onHeightDelta: (Float) -> Unit,
    onHeightCommit: () -> Unit,
    content: @Composable (BottomToolWindow, Modifier) -> Unit,
    modifier: Modifier = Modifier,
) {
  if (availableToolWindows.isEmpty()) return
  val activeToolWindow =
      layout.activeBottomToolWindow.takeIf { it in availableToolWindows }
          ?: availableToolWindows.first()
  val collapsed = layout.bottomCollapsed
  Column(
      modifier
          .fillMaxWidth()
          .height(if (collapsed) 38.dp else layout.bottomHeight.dp)
          .background(Panel)
          .border(androidx.compose.foundation.BorderStroke(1.dp, Border)),
  ) {
    if (!collapsed) HorizontalResizableDivider(onHeightDelta, onHeightCommit)
    Row(
        Modifier.fillMaxWidth().height(34.dp).padding(horizontal = 8.dp),
        verticalAlignment = Alignment.CenterVertically) {
          availableToolWindows.forEach { toolWindow ->
            val selected = toolWindow == activeToolWindow
            FocusFlowButton(
                onClick = { onSelect(toolWindow) },
                tone = ActionTone.Navigation,
                density = ButtonDensity.Toolbar,
                selected = selected,
                modifier =
                    Modifier.semantics {
                      contentDescription =
                          bottomToolWindowTabDescription(
                              toolWindow, selected, summaries[toolWindow])
                    }) {
                  Text(bottomToolWindowLabel(toolWindow), fontSize = 11.sp)
                }
          }
          Text(
              summaries[activeToolWindow].orEmpty(),
              color = SecondaryText,
              fontSize = 11.sp,
              maxLines = 1,
              modifier = Modifier.weight(1f).padding(start = 8.dp))
          FocusFlowButton(
              onClick = if (collapsed) ({ onSelect(activeToolWindow) }) else onCollapse,
              tone = ActionTone.Neutral,
              density = ButtonDensity.Toolbar) {
                Text(if (collapsed) "Open" else "Collapse", fontSize = 11.sp)
              }
        }
    if (!collapsed) content(activeToolWindow, Modifier.fillMaxWidth().weight(1f))
  }
}

internal fun bottomToolWindowLabel(toolWindow: BottomToolWindow): String =
    when (toolWindow) {
      BottomToolWindow.Problems -> "Problems"
      BottomToolWindow.Checks -> "Checks"
      BottomToolWindow.Output -> "Output"
    }

internal fun bottomToolWindowTabDescription(
    toolWindow: BottomToolWindow,
    selected: Boolean,
    summary: String?,
): String =
    "${bottomToolWindowLabel(toolWindow)} tool window tab${summary?.let { ", $it" }.orEmpty()}, ${if (selected) "selected" else "not selected"}"

internal fun toolWindowGlyph(toolWindow: LeftToolWindow): String =
    when (toolWindow) {
      LeftToolWindow.Project -> "P"
      LeftToolWindow.Summary -> "S"
      LeftToolWindow.Analysis -> "A"
      LeftToolWindow.Problems -> "!"
      LeftToolWindow.Editor -> "E"
    }

internal fun toolWindowSemanticsLabel(toolWindow: LeftToolWindow, selected: Boolean): String =
    "${leftToolWindowLabel(toolWindow)} tool window${if (selected) ", selected" else ", not selected"}"

@Composable
internal fun ResizableDivider(onDelta: (Float) -> Unit, onCommit: () -> Unit) {
  val density = LocalDensity.current
  Box(
      Modifier.fillMaxHeight().width(8.dp).pointerInput(Unit) {
        detectDragGestures(
            onDrag = { change, amount ->
              change.consume()
              onDelta(with(density) { amount.x.toDp().value })
            },
            onDragEnd = onCommit,
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
  Box(
      Modifier.fillMaxWidth().height(6.dp).pointerInput(Unit) {
        detectDragGestures(
            onDrag = { change, amount ->
              change.consume()
              onDelta(-with(density) { amount.y.toDp().value })
            },
            onDragEnd = onCommit,
        )
      },
      contentAlignment = Alignment.Center,
  ) {
    Box(Modifier.fillMaxWidth().height(1.dp).background(Border))
  }
}

internal fun desktopStatusBarVisible(loading: Boolean, error: String?): Boolean =
    loading || error != null

@Composable
internal fun ShellStatusRegion(status: String, error: String?, loading: Boolean) {
  if (!desktopStatusBarVisible(loading, error)) return
  Row(
      Modifier.fillMaxWidth()
          .height(30.dp)
          .background(Panel)
          .border(androidx.compose.foundation.BorderStroke(1.dp, Border))
          .padding(horizontal = 12.dp),
      verticalAlignment = Alignment.CenterVertically) {
        Box(
            Modifier.size(7.dp)
                .background(
                    if (error == null) Warning else Error,
                    androidx.compose.foundation.shape.RoundedCornerShape(50)))
        Spacer(Modifier.width(7.dp))
        Text(
            error ?: status,
            color = if (error == null) SecondaryText else Error,
            fontSize = 11.sp,
            maxLines = 1)
      }
}
