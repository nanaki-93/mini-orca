package io.miniorca.desktop

import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.relocation.BringIntoViewRequester
import androidx.compose.foundation.relocation.bringIntoViewRequester
import androidx.compose.foundation.rememberScrollState
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.key
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.layout.onGloballyPositioned
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.stateDescription
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.launch

internal data class TerminalTabActions(
    val select: (Long) -> Unit,
    val create: () -> Unit,
    val close: (Long) -> Unit,
)

@Composable
internal fun TerminalTabs(
    state: TerminalWorkspaceState,
    actions: TerminalTabActions,
    modifier: Modifier = Modifier,
) {
  Row(modifier, verticalAlignment = Alignment.CenterVertically) {
    Row(Modifier.weight(1f, fill = false).horizontalScroll(rememberScrollState())) {
      state.tabs.forEach { tab ->
        key(tab.id) { TerminalTab(tab, state.activeTabId == tab.id, actions) }
      }
    }
    ChromeButton(
        onClick = actions.create,
        enabled = state.canCreate,
        accessibleName = "New shell",
        modifier = Modifier.padding(start = 4.dp),
    ) {
      DesktopLineIcon(DesktopIcon.Add, "New shell", iconSize = 16.dp)
    }
  }
}

@Composable
private fun TerminalTab(tab: TerminalTabState, active: Boolean, actions: TerminalTabActions) {
  val bringIntoView = remember { BringIntoViewRequester() }
  val scope = rememberCoroutineScope()
  var laidOut by remember { mutableStateOf(false) }
  LaunchedEffect(active, laidOut) { if (active && laidOut) bringIntoView.bringIntoView() }
  Row(
      Modifier.bringIntoViewRequester(bringIntoView).onGloballyPositioned { laidOut = true },
      verticalAlignment = Alignment.CenterVertically) {
        ChromeTab(
            onClick = { actions.select(tab.id) },
            selected = active,
            accessibleName = tab.title,
            modifier =
                Modifier.semantics {
                      selected = active
                      stateDescription = terminalSummary(tab.session)
                    }
                    .onFocusChanged {
                      if (it.isFocused) scope.launch { bringIntoView.bringIntoView() }
                    },
        ) {
          val label =
              if (tab.session.phase == TerminalSessionPhase.Running &&
                  tab.session.error == null &&
                  !tab.session.cleanupPending)
                  tab.title
              else "${tab.title} · ${terminalSummary(tab.session)}"
          Text(
              label,
              fontSize = 12.sp,
              color = if (tab.session.error != null) Warning else PrimaryText)
        }
        ChromeButton(
            onClick = { actions.close(tab.id) },
            enabled = !tab.session.cleanupPending,
            selected = active,
            accessibleName = "Close ${tab.title}",
            contentPadding = PaddingValues(horizontal = 4.dp, vertical = 4.dp),
            modifier =
                Modifier.onFocusChanged {
                  if (it.isFocused) scope.launch { bringIntoView.bringIntoView() }
                },
        ) {
          DesktopLineIcon(DesktopIcon.Close, "Close ${tab.title}", iconSize = 14.dp)
        }
      }
}
