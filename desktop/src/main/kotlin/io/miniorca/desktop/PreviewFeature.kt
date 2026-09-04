package io.miniorca.desktop

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.material.AlertDialog
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

/** Local-only metadata for a control that deliberately has no product implementation. */
internal data class PreviewFeature(val label: String, val description: String)

internal fun previewFeatureDescription(feature: PreviewFeature): String =
    "${feature.label} preview. ${feature.description} Local only; no project, provider, or workflow state changes."

@Composable
internal fun PreviewBadge(modifier: Modifier = Modifier) {
  Text(
      "Preview",
      color = FocusAccent,
      fontSize = 12.sp,
      fontWeight = FontWeight.SemiBold,
      modifier =
          modifier
              .background(FocusAccent.copy(alpha = 0.12f), MiniOrcaShapes.small)
              .border(
                  androidx.compose.foundation.BorderStroke(1.dp, FocusAccent.copy(alpha = 0.55f)),
                  MiniOrcaShapes.small)
              .semantics { contentDescription = "Preview; local only" }
              .padding(horizontal = 6.dp, vertical = 2.dp),
  )
}

/**
 * Owns only its ephemeral dialog state. It intentionally has no presenter callback, so a preview
 * cannot request a model, mutate a project, or affect review eligibility.
 */
@Composable
internal fun PreviewFeatureButton(
    feature: PreviewFeature,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    density: ButtonDensity = ButtonDensity.Toolbar,
) {
  val triggerFocus = remember { FocusRequester() }
  var visible by remember { mutableStateOf(false) }
  var restoreFocus by remember { mutableStateOf(false) }

  MiniOrcaButton(
      onClick = { visible = true },
      enabled = enabled,
      tone = ActionTone.Neutral,
      density = density,
      modifier =
          modifier.focusRequester(triggerFocus).semantics {
            contentDescription = previewFeatureDescription(feature)
          },
  ) {
    Text(feature.label, fontSize = 12.sp)
    Spacer(Modifier.width(MiniOrcaSpacing.compact))
    PreviewBadge()
  }

  if (visible) {
    PreviewFeatureDialog(
        feature = feature,
        onDismiss = {
          visible = false
          restoreFocus = true
        })
  }
  LaunchedEffect(restoreFocus) {
    if (restoreFocus) {
      triggerFocus.requestFocus()
      restoreFocus = false
    }
  }
}

@Composable
internal fun PreviewFeatureDialog(feature: PreviewFeature, onDismiss: () -> Unit) {
  AlertDialog(
      onDismissRequest = onDismiss,
      title = {
        Row(verticalAlignment = Alignment.CenterVertically) {
          Text("${feature.label} · Preview", color = PrimaryText, fontWeight = FontWeight.SemiBold)
          Spacer(Modifier.width(MiniOrcaSpacing.standard))
          PreviewBadge()
        }
      },
      text = {
        Text(
            previewFeatureDescription(feature),
            color = SecondaryText,
            fontSize = 13.sp,
            modifier =
                Modifier.fillMaxWidth().semantics {
                  contentDescription = previewFeatureDescription(feature)
                })
      },
      confirmButton = {
        MiniOrcaButton(onClick = onDismiss, tone = ActionTone.Primary) { Text("Close") }
      },
  )
}
