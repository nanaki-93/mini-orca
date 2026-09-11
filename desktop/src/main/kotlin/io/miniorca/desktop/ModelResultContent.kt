package io.miniorca.desktop

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.stateDescription
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.font.FontStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.text.withStyle

private const val MAX_FORMATTED_CHARACTERS = 32_768
private const val MAX_FORMATTED_LINES = 512
private const val PREVIEW_LINES = 8
private val codeFence = Regex("```[a-zA-Z0-9_+.-]*")
private val listItem = Regex("^(?:[-+*]|[0-9]{1,9}[.)]) +(.+)$")
private val unsupportedBlock = Regex("^(?:#{1,6} |>|\\||~~~| {4}|\\t|[-*_]{3,}\\s*$)")

// Delimiter runs and intraword underscores stay literal. This is a deliberately small text
// formatter, not a Markdown document engine; unsupported inline constructs stay untouched.
private val inlineMarkup =
    Regex(
        "(?<![\\\\`])`([^`\\n]+)`(?!`)" +
            "|(?<![\\\\*])\\*\\*([^*\\n]+)\\*\\*(?!\\*)" +
            "|(?<![\\\\*])\\*([^*\\n]+)\\*(?!\\*)" +
            "|(?<![\\\\_\\p{L}\\p{N}])__([^_\\n]+)__(?![_\\p{L}\\p{N}])" +
            "|(?<![\\\\_\\p{L}\\p{N}])_([^_\\n]+)_(?![_\\p{L}\\p{N}])")

/**
 * Selectable model prose with a local disclosure. The parent owns vertical scrolling; this
 * component neither requests a model response nor interprets links, HTML or executable content.
 */
@Composable
internal fun ModelResultContent(response: String, modifier: Modifier = Modifier) {
  val formatted = remember(response) { formatModelResult(response) }
  var expanded by remember(response) { mutableStateOf(false) }
  var overflowsPreview by remember(response) { mutableStateOf(false) }
  Column(modifier, verticalArrangement = Arrangement.spacedBy(MiniOrcaSpacing.compact)) {
    SelectionContainer {
      Text(
          formatted,
          modifier = Modifier.fillMaxWidth(),
          color = PrimaryText,
          style = IdeTypography.body,
          maxLines = if (expanded) Int.MAX_VALUE else PREVIEW_LINES,
          overflow = TextOverflow.Ellipsis,
          onTextLayout = { if (!expanded) overflowsPreview = it.hasVisualOverflow },
      )
    }
    if (expanded || overflowsPreview) {
      ChromeButton(
          onClick = { expanded = !expanded },
          accessibleName = if (expanded) "Collapse full response" else "Expand full response",
          modifier =
              Modifier.semantics { stateDescription = if (expanded) "Expanded" else "Collapsed" },
      ) {
        Text(
            if (expanded) "Show less" else "Show full response",
            color = SelectionText,
            style = IdeTypography.resultLabel)
      }
    }
  }
}

/** Large or unusually fragmented responses stay complete as plain, selectable text. */
internal fun formatModelResult(source: String): AnnotatedString {
  if (source.length > MAX_FORMATTED_CHARACTERS ||
      source.count { it == '\n' } >= MAX_FORMATTED_LINES) {
    return AnnotatedString(source)
  }
  val lines = source.replace("\r\n", "\n").split('\n')
  val result = AnnotatedString.Builder()
  var index = 0
  while (index < lines.size) {
    val line = lines[index]
    if (codeFence.matches(line)) {
      val end = (index + 1 until lines.size).firstOrNull { lines[it] == "```" }
      if (end == null) {
        // Preserve an unfinished fence exactly, including any markup within its body.
        result.append(lines.subList(index, lines.size).joinToString("\n"))
        break
      }
      result.withStyle(
          IdeTypography.resultCode
              .toSpanStyle()
              .copy(color = PrimaryText, background = EditorCanvas)) {
            append(lines.subList(index + 1, end).joinToString("\n"))
          }
      index = end
    } else if (unsupportedBlock.containsMatchIn(line) ||
        line.contains('<') ||
        line.contains('[') ||
        line.contains('\\') ||
        line.contains("``")) {
      result.append(line)
    } else {
      val item = listItem.matchEntire(line)
      if (item != null) {
        val marker = if (line.first().isDigit()) line.substringBefore(' ') else "•"
        result.withStyle(SpanStyle(color = ResultAccent, fontWeight = FontWeight.SemiBold)) {
          append("$marker ")
        }
        result.appendResultInline(item.groupValues[1])
        if (index < lines.lastIndex) result.append('\n')
        index++
        continue
      }
      result.appendResultInline(line)
    }
    if (index < lines.lastIndex) result.append('\n')
    index++
  }
  return result.toAnnotatedString()
}

private fun AnnotatedString.Builder.appendResultInline(line: String) {
  var start = 0
  inlineMarkup.findAll(line).forEach { match ->
    append(line.substring(start, match.range.first))
    val group = (1..5).first { match.groups[it] != null }
    val style =
        when (group) {
          1 ->
              IdeTypography.resultCode
                  .toSpanStyle()
                  .copy(color = SelectionText, background = SelectionSurface)
          2,
          4 -> SpanStyle(fontWeight = FontWeight.SemiBold)
          else -> SpanStyle(fontStyle = FontStyle.Italic)
        }
    val content = match.groupValues[group]
    if (group != 1 && (content.first().isWhitespace() || content.last().isWhitespace())) {
      // Arithmetic and spaced delimiters are prose, not emphasis.
      append(match.value)
    } else {
      withStyle(style) { append(content) }
    }
    start = match.range.last + 1
  }
  append(line.substring(start))
}
