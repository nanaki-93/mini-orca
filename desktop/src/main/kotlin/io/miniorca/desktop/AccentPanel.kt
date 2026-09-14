package io.miniorca.desktop

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp

/** Shared tinted surface used for analysis summaries, file coverage and result sections. */
@Composable
internal fun AccentPanel(
    title: String,
    tint: Color,
    modifier: Modifier = Modifier,
    trailing: @Composable RowScope.() -> Unit = {},
    content: @Composable ColumnScope.() -> Unit,
) {
  Column(
      modifier
          .fillMaxWidth()
          .background(blendOver(tint.copy(alpha = 0.08f), Panel))
          .border(1.dp, tint.copy(alpha = 0.45f)),
      verticalArrangement = Arrangement.spacedBy(8.dp)) {
        Row(
            Modifier.fillMaxWidth()
                .background(blendOver(tint.copy(alpha = 0.16f), Panel))
                .padding(8.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(12.dp)) {
              Text(
                  title,
                  color = tint,
                  style = IdeTypography.section,
                  modifier = Modifier.semantics { heading() })
              trailing()
            }
        content()
      }
}
