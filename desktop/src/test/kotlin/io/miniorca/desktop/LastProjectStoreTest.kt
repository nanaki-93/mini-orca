package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull

class LastProjectStoreTest {
  @Test
  fun lastSuccessfulProjectPathRoundTripsAcrossStoreInstances() {
    withPreferences { preferences ->
      val first = LastProjectStore(preferences)
      assertNull(first.load())

      first.save("/tmp/mini-orca-project")

      assertEquals("/tmp/mini-orca-project", LastProjectStore(preferences).load())
      assertEquals(1, preferences.flushCount)
    }
  }

  @Test
  fun blankPathsDoNotReplaceTheRememberedProject() {
    withPreferences { preferences ->
      val store = LastProjectStore(preferences)
      store.save("/tmp/mini-orca-project")

      store.save("  ")

      assertEquals("/tmp/mini-orca-project", store.load())
      assertEquals(1, preferences.flushCount)
    }
  }

  private fun withPreferences(test: (InMemoryPreferences) -> Unit) = test(InMemoryPreferences())
}
