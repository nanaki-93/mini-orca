package io.miniorca.desktop

import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color

/** Recorded diagnostics stay selectable without interpreting terminal control characters. */
@Composable
internal fun DiagnosticText(
    value: String,
    modifier: Modifier = Modifier,
    color: Color = PrimaryText,
) {
  SelectionContainer {
    Text(sanitizedOutputText(value), modifier, color = color, style = IdeTypography.resultCode)
  }
}

internal fun sanitizedOutputText(value: String, limit: Int = 4_096): String {
  val cleaned = value.replace(DIAGNOSTIC_CONTROL_CHARACTERS, " ").trim()
  return if (cleaned.length <= limit) cleaned else "${cleaned.take(limit)}\n… output truncated"
}

private val DIAGNOSTIC_CONTROL_CHARACTERS =
    Regex("[\\u0000-\\u0008\\u000B\\u000C\\u000E-\\u001F\\u007F]")
