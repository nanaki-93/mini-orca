package io.miniorca.desktop

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class ModelResultContentTest {
  @Test
  fun proseKeepsParagraphsUnicodeAndWhitespaceWithDistinctInlineRoles() {
    val result =
        formatModelResult(
            "**Summary:** keeps *all* facts.\n\nRead `worker.go:42` and café 日本語.\n  ")
    assertEquals("Summary: keeps all facts.\n\nRead worker.go:42 and café 日本語.\n  ", result.text)
    assertEquals(
        "Summary:",
        result.spanStyles
            .single { it.item.fontWeight == FontWeight.SemiBold }
            .let { result.text.substring(it.start, it.end) })
    assertEquals(
        "all",
        result.spanStyles
            .single { it.item.fontStyle == FontStyle.Italic }
            .let { result.text.substring(it.start, it.end) })
    val code = result.spanStyles.single { it.item.fontFamily == FontFamily.Monospace }
    assertEquals("worker.go:42", result.text.substring(code.start, code.end))
    assertEquals(SelectionText, code.item.color)
    assertEquals(SelectionSurface, code.item.background)
  }

  @Test
  fun listsKeepOrderAndItemBoundariesWithDistinctMarkers() {
    val result =
        formatModelResult(
            "- **Bug:** a nil pointer\n+ Check `err`\n* Keep the guard\n\n3. First check\n8) Next check")
    assertEquals(
        "• Bug: a nil pointer\n• Check err\n• Keep the guard\n\n3. First check\n8) Next check",
        result.text)
    assertEquals(5, result.spanStyles.count { it.item.color == ResultAccent })
  }

  @Test
  fun codeFencesPreserveCodeAndDisableInlineInterpretation() {
    val code = "func Run() {\n\tprintln(\"**literal** <tag> [link]\")\n}"
    val result =
        formatModelResult(
            "Before\r\n\r\n```go\r\n${code.replace("\n", "\r\n")}\r\n```\r\n\r\nAfter")
    assertEquals("Before\n\n$code\n\nAfter", result.text)
    assertEquals(1, result.spanStyles.size)
    assertEquals(FontFamily.Monospace, result.spanStyles.single().item.fontFamily)
    assertEquals(EditorCanvas, result.spanStyles.single().item.background)
  }

  @Test
  fun unsupportedAndIncompleteMarkupRemainsLiteralAndNonInteractive() {
    listOf(
            "# **Heading**",
            "> quoted *text*",
            "| **table** |",
            "---",
            "    **indented code**",
            "[**link**](https://example.com)",
            "![image](https://example.com/a.png)",
            "<script>**text**</script>",
            "\\*escaped*",
            "``nested `code` delimiters``",
            "2 * 3 * 4",
            "** spaced **",
            "**unclosed",
            "*unclosed",
            "`unclosed",
            "***nested***",
            "snake_case_name",
            "```go\n**not prose**\nfinal line")
        .forEach { source ->
          val result = formatModelResult(source)
          assertEquals(source, result.text, source)
          assertTrue(result.spanStyles.isEmpty(), source)
          assertTrue(result.getStringAnnotations(0, result.length).isEmpty(), source)
        }
  }

  @Test
  fun underscoresOnlyEmphasizeWholeWords() {
    val result = formatModelResult("__Strong__ and _emphasis_, keep read_file_name and version_2.")
    assertEquals("Strong and emphasis, keep read_file_name and version_2.", result.text)
    assertEquals(2, result.spanStyles.size)
  }

  @Test
  fun largeOrFragmentedResponsesFallBackWithoutDroppingAnyContent() {
    listOf("**start**\n" + "x".repeat(40_000) + "\nlast café", "*item*\n".repeat(600)).forEach {
        source ->
      val result = formatModelResult(source)
      assertEquals(source, result.text)
      assertTrue(result.spanStyles.isEmpty())
    }
    assertEquals("", formatModelResult("").text)
    assertEquals("\n\n", formatModelResult("\n\n").text)
  }

  @Test
  fun productionContentExpandsByKeyboardAndResetsForANewResponseAtNarrowAndWideSizes() {
    listOf(360 to 1.5f, 720 to 1f).forEach { (width, scale) ->
      var response by
          mutableStateOf((1..12).joinToString("\n") { "Line $it: a readable explanation." })
      ComposeVisualFixture(width, 1100, scale) {
            ModelResultContent(response, Modifier.fillMaxWidth().padding(8.dp))
          }
          .use { fixture ->
            fixture.render("model-result-collapsed-$width-$scale")
            assertTrue(fixture.hasDescription("Expand full response"))
            assertEquals("Collapsed", fixture.stateDescription("Show full response"))
            assertFalse(fixture.hasScrollableContent())
            assertTrue(fixture.requestFocus("Show full response"))
            fixture.render("model-result-focused-$width-$scale")
            assertTrue(fixture.isFocused("Show full response"))
            fixture.pressKey(Key.Enter)
            fixture.render("model-result-expanded-$width-$scale")
            assertEquals("Expanded", fixture.stateDescription("Show less"))
            fixture.assertTextWrapsWithoutClipping(response)
            fixture.clickDescription("Collapse full response")
            fixture.render()
            assertEquals("Collapsed", fixture.stateDescription("Show full response"))
            fixture.clickDescription("Expand full response")
            fixture.render()
            response = "A short replacement."
            fixture.render()
            assertTrue(fixture.hasText(response))
            assertFalse(fixture.hasDescription("Collapse full response"))
            assertFalse(fixture.hasDescription("Expand full response"))
            fixture.assertTextFits(response)
          }
    }
  }

  @Test
  fun productionPrimitivesWrapHeadingsBadgesAndCodeAtLargeText() {
    val heading = "Why this function can fail when the input is missing"
    val badge = "High severity — requires attention before applying"
    val response =
        "**Summary:** validate the input before using it.\n\n- Check `worker.go:42`\n- Keep *errors* visible\n\n```go\nif err != nil {\n    return err\n}\n```"
    ComposeVisualFixture(360, 1000, 1.5f) {
          Column(Modifier.fillMaxWidth().padding(8.dp)) {
            IdePaneHeader(heading, stateLabel = "Explanation", stateTint = ResultAccent)
            IdeLabelBadge(badge, Warning)
            ModelResultContent(response)
          }
        }
        .use { fixture ->
          fixture.render("model-result-primitives-360-1.5")
          fixture.assertTextWrapsWithoutClipping(heading)
          fixture.assertTextWrapsWithoutClipping(badge)
          assertTrue(fixture.hasDescription(badge))
          fixture.clickDescription("Expand full response")
          fixture.render("model-result-primitives-expanded-360-1.5")
          fixture.assertTextWrapsWithoutClipping(formatModelResult(response).text)
        }
  }
}
