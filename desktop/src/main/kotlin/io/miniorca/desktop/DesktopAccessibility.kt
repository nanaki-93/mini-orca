package io.miniorca.desktop

import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.withStyle

enum class DesktopShortcut {
  OpenProject,
  OpenFile,
  OpenSymbol,
  Generate,
  Cancel,
  NextTab,
  SummaryWorkspace,
  AnalysisWorkspace,
  BugsWorkspace,
  EditorWorkspace,
  FocusBugsFilters,
  FocusChat,
  FocusDraft,
  ValidateDraft,
  RunDraftChecks,
}

fun desktopShortcut(
    key: String,
    primaryModifier: Boolean,
    shift: Boolean = false
): DesktopShortcut? =
    when {
      key == "Escape" -> DesktopShortcut.Cancel
      primaryModifier && key == "1" -> DesktopShortcut.SummaryWorkspace
      primaryModifier && key == "2" -> DesktopShortcut.AnalysisWorkspace
      primaryModifier && key == "3" -> DesktopShortcut.BugsWorkspace
      primaryModifier && key == "4" -> DesktopShortcut.EditorWorkspace
      primaryModifier && key == "P" -> DesktopShortcut.OpenFile
      primaryModifier && shift && key == "O" -> DesktopShortcut.OpenSymbol
      primaryModifier && key == "O" -> DesktopShortcut.OpenProject
      primaryModifier && key == "K" -> DesktopShortcut.FocusChat
      primaryModifier && shift && key == "F" -> DesktopShortcut.FocusBugsFilters
      primaryModifier && shift && key == "D" -> DesktopShortcut.FocusDraft
      primaryModifier && shift && key == "V" -> DesktopShortcut.ValidateDraft
      primaryModifier && shift && key == "C" -> DesktopShortcut.RunDraftChecks
      primaryModifier && key == "Enter" -> DesktopShortcut.Generate
      primaryModifier && key == "Tab" -> DesktopShortcut.NextTab
      else -> null
    }

internal fun shortcutAvailable(shellMode: DesktopShellMode, shortcut: DesktopShortcut?): Boolean =
    shortcut != null &&
        (shellMode == DesktopShellMode.ProjectWorkspace || shortcut == DesktopShortcut.OpenProject)

private val codeToken =
    Regex(
        "//.*$|#.*$|\\\"(?:\\\\.|[^\\\"])*\\\"|\\b(?:class|data|fun|func|interface|package|import|return|if|else|for|while|when|val|var|type|struct|impl|pub|def|async|await)\\b",
        RegexOption.MULTILINE)

/** Applies lightweight, read-only highlighting without attempting to parse or edit source. */
fun highlightedCode(source: String): AnnotatedString = buildAnnotatedString {
  var cursor = 0
  codeToken.findAll(source).forEach { match ->
    append(source.substring(cursor, match.range.first))
    val token = match.value
    val color =
        when {
          token.startsWith("//") || token.startsWith("#") -> CodeComment
          token.startsWith('"') -> CodeString
          else -> CodeKeyword
        }
    withStyle(SpanStyle(color = color)) { append(token) }
    cursor = match.range.last + 1
  }
  append(source.substring(cursor))
}
