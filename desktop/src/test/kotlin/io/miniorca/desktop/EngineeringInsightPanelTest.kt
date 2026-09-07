package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class EngineeringInsightPanelTest {
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
