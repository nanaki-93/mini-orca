package io.miniorca.desktop

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.key
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.unit.dp
import java.util.prefs.Preferences

/** Compact local disclosure for advisory prose already returned with its owner. */
@Composable
internal fun EngineeringInsightPanel(
    insight: EngineeringInsight?,
    stale: Boolean = false,
    scopeLabel: String = "",
    modifier: Modifier = Modifier,
) {
  val pieces = insight?.let(::engineeringInsightPieces).orEmpty()
  if (pieces.isEmpty()) return
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
      Column(Modifier.fillMaxWidth().padding(horizontal = 8.dp, vertical = 4.dp)) {
        pieces.forEach { piece ->
          Text(piece.label, color = ResultAccent, style = IdeTypography.resultLabel)
          ModelResultContent(piece.content, modifier = Modifier.padding(top = 4.dp, bottom = 12.dp))
        }
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

internal data class EngineeringInsightPiece(val label: String, val content: String)

internal data class SummaryEngineeringInsightPieces(
    val lead: List<EngineeringInsightPiece>,
    val additional: List<EngineeringInsightPiece>,
)

internal fun summaryEngineeringInsightPieces(
    pieces: List<EngineeringInsightPiece>
): SummaryEngineeringInsightPieces {
  val leadLabels = setOf("Mechanism", "Why it matters here")
  val preferred = pieces.filter { it.label in leadLabels }
  val available = preferred + pieces.filterNot { it.label in leadLabels }
  return SummaryEngineeringInsightPieces(available.take(2), available.drop(2))
}

/** Summary-local insight disclosure that resets with its project and returned content. */
@Composable
internal fun SummaryEngineeringInsightPanel(
    pieces: List<EngineeringInsightPiece>,
    stale: Boolean,
    ownerIdentity: Any,
    modifier: Modifier = Modifier,
) {
  if (pieces.isEmpty()) return
  key(ownerIdentity, pieces) {
    val presentation = summaryEngineeringInsightPieces(pieces)
    var expanded by remember { mutableStateOf(false) }
    val detailsFocus = remember { FocusRequester() }
    var restoreDetailsFocus by remember { mutableStateOf(false) }
    fun toggle() {
      expanded = !expanded
      if (!expanded) restoreDetailsFocus = true
    }
    Column(modifier.fillMaxWidth()) {
      Text(
          engineeringInsightStateLabel("", stale),
          color = if (stale) Warning else SecondaryText,
          style = IdeTypography.workspaceMetadata)
      presentation.lead.forEach { piece ->
        Text(piece.label, color = ResultAccent, style = IdeTypography.resultLabel)
        ModelResultContent(piece.content, preview = false, style = IdeTypography.workspaceBody)
      }
      presentation.additional
          .takeIf { it.isNotEmpty() }
          ?.let { additional ->
            IdeDisclosureHeader(
                title = "More insight",
                expanded = expanded,
                onToggle = ::toggle,
                modifier = Modifier.focusRequester(detailsFocus))
            if (expanded) {
              Column(Modifier.fillMaxWidth().padding(top = 4.dp)) {
                additional.forEach { piece ->
                  Text(piece.label, color = ResultAccent, style = IdeTypography.resultLabel)
                  ModelResultContent(
                      piece.content, preview = false, style = IdeTypography.workspaceBody)
                }
              }
            }
          }
    }
    LaunchedEffect(restoreDetailsFocus) {
      if (restoreDetailsFocus) {
        detailsFocus.requestFocus()
        restoreDetailsFocus = false
      }
    }
  }
}

internal fun engineeringInsightPieces(insight: EngineeringInsight): List<EngineeringInsightPiece> =
    listOf(
            EngineeringInsightPiece("Mechanism", insight.mechanism),
            EngineeringInsightPiece("Why it matters here", insight.whyItMattersHere),
            EngineeringInsightPiece("Trade-off or failure mode", insight.tradeoffOrFailureMode),
            EngineeringInsightPiece("Transferable lesson", insight.transferableLesson),
        )
        .mapNotNull { piece ->
          piece.content.trim().takeIf(String::isNotEmpty)?.let { piece.copy(content = it) }
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
