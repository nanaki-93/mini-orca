package io.miniorca.desktop

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.ui.Modifier
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class EngineeringInsightPanelTest {
  @Test
  fun expandedInsightUsesItsParentScrollAndRetainsTheFullResponse() {
    val original = EngineeringInsightPreference.load()
    val insight =
        EngineeringInsight(
            mechanism =
                "**Check** `cache.go`. " +
                    "Keep the evidence tied to the selected revision. ".repeat(30),
            transferableLesson = "Verify after changing the source.")
    try {
      EngineeringInsightPreference.save(false)
      ComposeVisualFixture(360, 650, 1.5f) {
            Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState())) {
              EngineeringInsightPanel(insight, stale = true, scopeLabel = "File")
            }
          }
          .use { fixture ->
            fixture.render()
            fixture.clickText("Engineering insight")
            fixture.render("insight-parent-scroll-360-1.5")
            assertEquals(1, fixture.scrollableContentCount())
            assertTrue(fixture.hasText("AI interpretation · File · stale"))
            fixture.clickText("Show full response")
            fixture.render()
            assertEquals("Expanded", fixture.stateDescription("Show less"))
            assertEquals(1, fixture.scrollableContentCount())
            assertTrue(fixture.hasText(formatModelResult(insight.mechanism.trim()).text))
            assertTrue(fixture.hasText(insight.transferableLesson))
          }
    } finally {
      EngineeringInsightPreference.save(original)
    }
  }

  @Test
  fun insightHeaderKeepsItsInterpretationScopeAndStalenessExplicit() {
    assertEquals("AI interpretation", engineeringInsightStateLabel("", stale = false))
    assertEquals("AI interpretation · File", engineeringInsightStateLabel("File", stale = false))
    assertEquals(
        "AI interpretation · Selected performance opportunity",
        engineeringInsightStateLabel("Selected performance opportunity", stale = false),
    )
    assertEquals(
        "AI interpretation · File · stale",
        engineeringInsightStateLabel("File", stale = true),
    )
  }

  @Test
  fun insightExpansionPreferencePersistsAndRestoresThePriorValue() {
    val original = EngineeringInsightPreference.load()

    try {
      EngineeringInsightPreference.save(true)
      assertTrue(EngineeringInsightPreference.load())
      EngineeringInsightPreference.save(false)
      assertFalse(EngineeringInsightPreference.load())
    } finally {
      EngineeringInsightPreference.save(original)
    }
  }

  @Test
  fun insightPieceLabelsAreStable() {
    val pieces =
        engineeringInsightPieces(
            EngineeringInsight(
                mechanism = "Mechanism.",
                whyItMattersHere = "Local evidence.",
                tradeoffOrFailureMode = "Trade-off.",
                transferableLesson = "Verification."))

    assertEquals(
        listOf(
            "Mechanism",
            "Why it matters here",
            "Trade-off or failure mode",
            "Transferable lesson",
        ),
        pieces.map { it.label },
    )
  }

  @Test
  fun insightPiecesOmitBlankOptionalFields() {
    val pieces =
        engineeringInsightPieces(
            EngineeringInsight(
                mechanism = "  Cache identity binds the evidence.  ",
                whyItMattersHere = "The selected file can change while work is in flight.",
                tradeoffOrFailureMode = "   ",
                transferableLesson = "Verify by changing the file before publication."))

    assertEquals(3, pieces.size)
    assertEquals("Cache identity binds the evidence.", pieces.first().content)
    assertFalse(pieces.any { it.content.isBlank() })
  }
}
