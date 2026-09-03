package io.miniorca.desktop

import java.util.UUID
import java.util.prefs.Preferences
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
    }
  }

  @Test
  fun blankPathsDoNotReplaceTheRememberedProject() {
    withPreferences { preferences ->
      val store = LastProjectStore(preferences)
      store.save("/tmp/mini-orca-project")

      store.save("  ")

      assertEquals("/tmp/mini-orca-project", store.load())
    }
  }

  private fun withPreferences(test: (Preferences) -> Unit) {
    val preferences = Preferences.userRoot().node("/io/miniorca/desktop/tests/${UUID.randomUUID()}")
    try {
      test(preferences)
    } finally {
      preferences.removeNode()
    }
  }
}
