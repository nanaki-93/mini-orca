package io.miniorca.desktop

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.stateDescription
import androidx.compose.ui.unit.dp

private const val DIAGNOSTIC_PREVIEW_LIMIT = 4_096

/** Recorded diagnostics stay selectable without interpreting terminal control characters. */
@Composable
internal fun DiagnosticText(
    value: String,
    modifier: Modifier = Modifier,
    color: Color = PrimaryText,
) {
  val cleaned = remember(value) { sanitizeDiagnosticOutput(value) }
  var expanded by remember(value) { mutableStateOf(false) }
  val truncated = cleaned.length > DIAGNOSTIC_PREVIEW_LIMIT
  Column(modifier) {
    SelectionContainer {
      Text(
          if (expanded) cleaned else cleaned.take(DIAGNOSTIC_PREVIEW_LIMIT),
          modifier =
              Modifier.then(
                  if (expanded)
                      Modifier.heightIn(max = 240.dp)
                          .verticalScroll(rememberScrollState())
                          .testTag("diagnostic-output-scroll")
                  else Modifier),
          color = color,
          style = IdeTypography.resultCode)
    }
    if (truncated) {
      if (!expanded) {
        Text("… output truncated", color = color, style = IdeTypography.resultCode)
      }
      ChromeButton(
          onClick = { expanded = !expanded },
          accessibleName =
              if (expanded) "Collapse available diagnostic output"
              else "Expand available diagnostic output",
          modifier =
              Modifier.semantics { stateDescription = if (expanded) "Expanded" else "Collapsed" }) {
            Text(
                if (expanded) "Show preview" else "Show full available output",
                color = SelectionText,
                style = IdeTypography.resultLabel)
          }
    }
  }
}

internal fun sanitizedOutputText(value: String, limit: Int = DIAGNOSTIC_PREVIEW_LIMIT): String =
    previewDiagnosticOutput(sanitizeDiagnosticOutput(value), limit)

private fun sanitizeDiagnosticOutput(value: String): String =
    value.replace(DIAGNOSTIC_CONTROL_CHARACTERS, " ").trim()

private fun previewDiagnosticOutput(cleaned: String, limit: Int): String =
    if (cleaned.length <= limit) cleaned else "${cleaned.take(limit)}\n… output truncated"

private val DIAGNOSTIC_CONTROL_CHARACTERS =
    Regex("[\\u0000-\\u0008\\u000B\\u000C\\u000E-\\u001F\\u007F]")
