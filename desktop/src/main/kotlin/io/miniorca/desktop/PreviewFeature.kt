package io.miniorca.desktop

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
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

internal fun previewFeatureIcon(feature: PreviewFeature): DesktopIcon =
    when (feature.label) {
      "New file" -> DesktopIcon.File
      "Branch actions" -> DesktopIcon.Branch
      "Content search" -> DesktopIcon.Search
      "Run / Debug" -> DesktopIcon.Run
      "Settings & Help" -> DesktopIcon.Settings
      else -> DesktopIcon.More
    }

@Composable
internal fun PreviewBadge(modifier: Modifier = Modifier) {
  Text(
      "Preview",
      color = SecondaryText,
      fontSize = 11.sp,
      modifier =
          modifier
              .background(StrongSurface, MiniOrcaShapes.small)
              .semantics { contentDescription = "Preview; local only" }
              .padding(horizontal = 6.dp, vertical = 2.dp),
  )
}

/** Keeps unsupported utilities discoverable without giving them primary toolbar space. */
@Composable
internal fun PreviewFeatureMenu(
    features: List<PreviewFeature>,
    modifier: Modifier = Modifier,
) {
  val triggerFocus = remember { FocusRequester() }
  var expanded by remember { mutableStateOf(false) }
  var activeFeature by remember { mutableStateOf<PreviewFeature?>(null) }
  var restoreFocus by remember { mutableStateOf(false) }
  Box(modifier) {
    ChromeButton(
        onClick = { expanded = true },
        modifier =
            Modifier.focusRequester(triggerFocus).semantics {
              contentDescription = "Preview tools menu; local only"
            }) {
          DesktopLineIcon(DesktopIcon.More, "Preview tools", iconSize = 18.dp)
          Spacer(Modifier.width(4.dp))
          Text("Preview", fontSize = 11.sp)
        }
    IdeDropdownMenu(
        expanded = expanded,
        onDismissRequest = {
          expanded = false
          restoreFocus = true
        }) {
          features.forEach { feature ->
            IdeDropdownMenuItem(
                label = feature.label,
                onClick = {
                  expanded = false
                  activeFeature = feature
                },
                icon = previewFeatureIcon(feature),
                status = { PreviewBadge() })
          }
        }
  }
  activeFeature?.let { feature ->
    PreviewFeatureDialog(feature) {
      activeFeature = null
      restoreFocus = true
    }
  }
  LaunchedEffect(restoreFocus) {
    if (restoreFocus) {
      triggerFocus.requestFocus()
      restoreFocus = false
    }
  }
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
  IdeDialog(
      onDismissRequest = onDismiss,
      title = {
        Row(verticalAlignment = Alignment.CenterVertically) {
          Text("${feature.label} · Preview", color = PrimaryText, fontWeight = FontWeight.SemiBold)
          Spacer(Modifier.width(MiniOrcaSpacing.standard))
          PreviewBadge()
        }
      },
      content = {
        Text(
            previewFeatureDescription(feature),
            color = SecondaryText,
            fontSize = 13.sp,
            modifier =
                Modifier.fillMaxWidth().semantics {
                  contentDescription = previewFeatureDescription(feature)
                })
      },
      actions = {
        MiniOrcaButton(onClick = onDismiss, tone = ActionTone.Primary) { Text("Close") }
      },
  )
}
