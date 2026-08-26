package io.miniorca.desktop

import androidx.compose.ui.graphics.Color

internal val AppBackground = Color(0xFF0D1117)
internal val Panel = Color(0xFF161B22)
internal val Card = Color(0xFF21262D)
internal val Border = Color(0xFF30363D)
internal val PrimaryText = Color(0xFFF0F6FC)
internal val SecondaryText = Color(0xFF8B949E)
internal val Accent = Color(0xFF2F81F7)
internal val Success = Color(0xFF3FB950)
internal val Warning = Color(0xFFD29922)
internal val Error = Color(0xFFF85149)

internal fun badgeColor(status: String) = when (status.lowercase()) {
    "fresh" -> Success
    "stale", "running" -> Warning
    "failed" -> Error
    else -> SecondaryText
}

internal fun formatBytes(bytes: Long): String = when {
    bytes < 1024 -> "$bytes B"
    bytes < 1024 * 1024 -> "${bytes / 1024} KB"
    else -> "${bytes / (1024 * 1024)} MB"
}
