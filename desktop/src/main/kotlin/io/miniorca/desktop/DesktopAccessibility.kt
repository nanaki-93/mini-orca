package io.miniorca.desktop

import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.withStyle

enum class DesktopShortcut {
    OpenFile,
    OpenSymbol,
    OpenAction,
    Generate,
    Cancel,
    NextTab,
}

fun desktopShortcut(key: String, primaryModifier: Boolean, shift: Boolean = false): DesktopShortcut? = when {
    key == "Escape" -> DesktopShortcut.Cancel
    primaryModifier && key == "P" -> DesktopShortcut.OpenFile
    primaryModifier && shift && key == "O" -> DesktopShortcut.OpenSymbol
    primaryModifier && key == "K" -> DesktopShortcut.OpenAction
    primaryModifier && key == "Enter" -> DesktopShortcut.Generate
    primaryModifier && key == "Tab" -> DesktopShortcut.NextTab
    else -> null
}

private val codeToken = Regex("//.*$|#.*$|\\\"(?:\\\\.|[^\\\"])*\\\"|\\b(?:class|data|fun|func|interface|package|import|return|if|else|for|while|when|val|var|type|struct|impl|pub|def|async|await)\\b", RegexOption.MULTILINE)

/** Applies lightweight, read-only highlighting without attempting to parse or edit source. */
fun highlightedCode(source: String): AnnotatedString = buildAnnotatedString {
    var cursor = 0
    codeToken.findAll(source).forEach { match ->
        append(source.substring(cursor, match.range.first))
        val token = match.value
        val color = when {
            token.startsWith("//") || token.startsWith("#") -> Color(0xFF8B949E)
            token.startsWith('"') -> Color(0xFFA5D6FF)
            else -> Color(0xFFD2A8FF)
        }
        withStyle(SpanStyle(color = color)) { append(token) }
        cursor = match.range.last + 1
    }
    append(source.substring(cursor))
}
