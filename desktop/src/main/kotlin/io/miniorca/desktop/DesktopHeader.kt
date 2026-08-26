package io.miniorca.desktop

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.Button
import androidx.compose.material.ButtonDefaults
import androidx.compose.material.CircularProgressIndicator
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

@Composable
internal fun DesktopHeader(
    project: ProjectAnalysis?, busy: Boolean, connection: ConnectionState, onImport: () -> Unit, onReanalyze: () -> Unit, onPalette: () -> Unit,
    narrow: Boolean, onOpenExplorer: () -> Unit, onOpenAction: () -> Unit,
) {
    Row(
        modifier = Modifier.fillMaxWidth().height(56.dp).background(Panel).border(BorderStroke(1.dp, Border)).padding(horizontal = 16.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Box(Modifier.size(26.dp).background(Accent, RoundedCornerShape(7.dp)), contentAlignment = Alignment.Center) { Text("⚡", fontSize = 14.sp) }
        Spacer(Modifier.width(10.dp))
        Text("Mini-Orca", fontWeight = FontWeight.SemiBold)
        project?.let { Spacer(Modifier.width(14.dp)); Text("/  ${it.name}", color = SecondaryText, fontSize = 13.sp) }
        Spacer(Modifier.weight(1f))
        Text(if (connection.connected) "● ${connection.model} · ${connection.locality} ${connection.latency}" else "○ ${connection.label} · ${connection.locality}", color = if (connection.connected) Success else Warning, fontSize = 11.sp)
        if (busy) { Spacer(Modifier.width(12.dp)); CircularProgressIndicator(Modifier.size(18.dp), color = Accent, strokeWidth = 2.dp) }
        Spacer(Modifier.width(12.dp))
        if (narrow) {
            Button(onClick = onOpenExplorer) { Text("Files") }
            Spacer(Modifier.width(8.dp))
            Button(onClick = onOpenAction) { Text("Action") }
            Spacer(Modifier.width(8.dp))
        }
        Button(onClick = onPalette) { Text("⌘K") }
        Spacer(Modifier.width(8.dp))
        Button(onClick = onReanalyze, enabled = project != null) { Text("Re-analyze") }
        Spacer(Modifier.width(8.dp))
        Button(onClick = onImport, colors = ButtonDefaults.buttonColors(backgroundColor = Accent, contentColor = Color.White)) { Text("Import") }
    }
}
