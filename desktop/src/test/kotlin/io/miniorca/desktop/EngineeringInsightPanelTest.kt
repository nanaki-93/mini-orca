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
}
