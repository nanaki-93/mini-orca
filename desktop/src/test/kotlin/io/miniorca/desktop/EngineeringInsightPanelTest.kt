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
  fun summaryPiecesKeepLeadContentVisibleAndDeferAdditionalContent() {
    val pieces = summaryInsightPieces()

    val summary = summaryEngineeringInsightPieces(pieces)

    assertEquals(listOf("Mechanism", "Why it matters here"), summary.lead.map { it.label })
    assertEquals(
        listOf("Trade-off or failure mode", "Transferable lesson"),
        summary.additional.map { it.label },
    )
  }

  @Test
  fun summaryPiecesPromoteAvailableContentForEveryMissingLeadCase() {
    val pieces = summaryInsightPieces()

    val withoutMechanism = summaryEngineeringInsightPieces(pieces.drop(1))
    assertEquals(
        listOf("Why it matters here", "Trade-off or failure mode"),
        withoutMechanism.lead.map { it.label },
    )
    assertEquals(listOf("Transferable lesson"), withoutMechanism.additional.map { it.label })

    val withoutWhyItMatters =
        summaryEngineeringInsightPieces(pieces.filterNot { it.label == "Why it matters here" })
    assertEquals(
        listOf("Mechanism", "Trade-off or failure mode"),
        withoutWhyItMatters.lead.map { it.label },
    )
    assertEquals(listOf("Transferable lesson"), withoutWhyItMatters.additional.map { it.label })

    val withoutPreferredLeads = summaryEngineeringInsightPieces(pieces.drop(2))
    assertEquals(
        listOf("Trade-off or failure mode", "Transferable lesson"),
        withoutPreferredLeads.lead.map { it.label },
    )
    assertTrue(withoutPreferredLeads.additional.isEmpty())
  }

  @Test
  fun summaryInsightShowsFullLeadAndAdditionalContentWithoutNestedPreviews() {
    val longContent = "Insight content. ".repeat(20)
    ComposeVisualFixture(360, 650) {
          SummaryEngineeringInsightPanel(
              pieces =
                  listOf(
                      EngineeringInsightPiece("Mechanism", longContent),
                      EngineeringInsightPiece("Why it matters here", longContent),
                      EngineeringInsightPiece("Trade-off or failure mode", longContent)),
              stale = false,
              ownerIdentity = "project/revision")
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText(formatModelResult(longContent).text))
          assertFalse(fixture.hasText("Show full response"))
          assertTrue(fixture.tryClick("Expand More insight"))
          fixture.render()
          assertTrue(fixture.hasText("Trade-off or failure mode"))
          assertFalse(fixture.hasText("Show full response"))
        }
  }

  @Test
  fun summaryInsightCollapseRestoresDisclosureFocusAndKeepsLeadVisible() {
    ComposeVisualFixture(600, 650) {
          SummaryEngineeringInsightPanel(
              summaryInsightPieces(), stale = false, ownerIdentity = "one")
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.tryClick("Expand More insight"))
          fixture.render()
          assertTrue(fixture.hasText("Trade-off."))
          assertTrue(fixture.requestDescriptionFocus("Collapse More insight"))
          assertTrue(fixture.tryClick("Collapse More insight"))
          fixture.render()
          assertEquals("Collapsed", fixture.stateDescription("More insight"))
          assertTrue(fixture.isDescriptionFocused("Expand More insight"))
          assertFalse(fixture.hasText("Trade-off."))
          assertTrue(fixture.hasText("Mechanism."))
          assertTrue(fixture.hasText("Local impact."))
        }
  }

  private fun summaryInsightPieces(): List<EngineeringInsightPiece> =
      engineeringInsightPieces(
          EngineeringInsight(
              mechanism = "Mechanism.",
              whyItMattersHere = "Local impact.",
              tradeoffOrFailureMode = "Trade-off.",
              transferableLesson = "Lesson."))

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
