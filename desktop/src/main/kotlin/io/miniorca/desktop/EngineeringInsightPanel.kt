package io.miniorca.desktop

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
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
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import java.util.prefs.Preferences

/** Compact local disclosure for advisory prose already returned with its owner. */
@Composable
internal fun EngineeringInsightPanel(
    insight: EngineeringInsight?,
    stale: Boolean = false,
    scopeLabel: String = "",
    modifier: Modifier = Modifier,
) {
  if (insight == null) return
  var expanded by remember { mutableStateOf(EngineeringInsightPreference.load()) }
  val headerFocus = remember { FocusRequester() }
  var restoreHeaderFocus by remember { mutableStateOf(false) }
  fun toggle() {
    expanded = !expanded
    EngineeringInsightPreference.save(expanded)
    if (!expanded) restoreHeaderFocus = true
  }
  Column(modifier.fillMaxWidth().padding(top = 8.dp)) {
    IdeDisclosureHeader(
        title = "Engineering insight",
        expanded = expanded,
        onToggle = ::toggle,
        modifier = Modifier.focusRequester(headerFocus),
        stateLabel = engineeringInsightStateLabel(scopeLabel, stale),
        stateTint = if (stale) Warning else SecondaryText)
    if (expanded) {
      val prose =
          listOf(
                  insight.mechanism,
                  insight.whyItMattersHere,
                  insight.tradeoffOrFailureMode,
                  insight.transferableLesson)
              .filter(String::isNotBlank)
              .joinToString(" ")
      Box(
          Modifier.fillMaxWidth()
              .heightIn(max = 240.dp)
              .verticalScroll(rememberScrollState())
              .padding(horizontal = 8.dp, vertical = 4.dp)) {
            Text(prose, color = PrimaryText, fontSize = 12.sp, lineHeight = 18.sp)
          }
    }
  }
  LaunchedEffect(restoreHeaderFocus) {
    if (restoreHeaderFocus) {
      headerFocus.requestFocus()
      restoreHeaderFocus = false
    }
  }
}

internal fun engineeringInsightStateLabel(scopeLabel: String, stale: Boolean): String =
    buildString {
      append("AI interpretation")
      scopeLabel.takeIf { it.isNotBlank() }?.let { append(" · ").append(it) }
      if (stale) append(" · stale")
    }

internal object EngineeringInsightPreference {
  private val preferences = Preferences.userNodeForPackage(EngineeringInsightPreference::class.java)

  fun load(): Boolean = preferences.getBoolean("engineering-insight-expanded", false)

  fun save(expanded: Boolean) = preferences.putBoolean("engineering-insight-expanded", expanded)
}
