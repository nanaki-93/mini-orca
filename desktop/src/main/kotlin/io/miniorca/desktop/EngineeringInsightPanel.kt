package io.miniorca.desktop

import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
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
  val opener = remember { FocusRequester() }
  fun toggle() {
    expanded = !expanded
    EngineeringInsightPreference.save(expanded)
  }
  MiniOrcaPanel(modifier.fillMaxWidth().padding(top = 8.dp), raised = expanded) {
    MiniOrcaButton(
        onClick = ::toggle,
        tone = ActionTone.Navigation,
        density = ButtonDensity.Toolbar,
        modifier =
            Modifier.focusRequester(opener).semantics {
              contentDescription =
                  "Engineering insight, AI interpretation, ${if (expanded) "expanded" else "collapsed"}"
            }) {
          Text(if (expanded) "⌄ Engineering insight" else "> Engineering insight", fontSize = 11.sp)
        }
    Text(
        "AI interpretation${scopeLabel.takeIf { it.isNotBlank() }?.let { " · $it" }.orEmpty()}",
        color = SecondaryText,
        fontSize = 10.sp,
        modifier = Modifier.padding(top = 3.dp))
    if (stale)
        Text(
            "Outdated — source changed",
            color = Warning,
            fontSize = 10.sp,
            modifier = Modifier.padding(top = 3.dp))
    if (expanded) {
      val prose =
          listOf(
                  insight.mechanism,
                  insight.whyItMattersHere,
                  insight.tradeoffOrFailureMode,
                  insight.transferableLesson)
              .filter(String::isNotBlank)
              .joinToString(" ")
      Text(prose, color = PrimaryText, fontSize = 12.sp, modifier = Modifier.padding(top = 8.dp))
      MiniOrcaButton(
          onClick = {
            toggle()
            opener.requestFocus()
          },
          tone = ActionTone.Neutral,
          density = ButtonDensity.Toolbar,
          modifier = Modifier.padding(top = 8.dp)) {
            Text("Close insight", fontSize = 11.sp)
          }
    }
  }
}

private object EngineeringInsightPreference {
  private val preferences = Preferences.userNodeForPackage(EngineeringInsightPreference::class.java)

  fun load(): Boolean = preferences.getBoolean("engineering-insight-expanded", false)

  fun save(expanded: Boolean) = preferences.putBoolean("engineering-insight-expanded", expanded)
}
