package io.miniorca.desktop

import java.util.prefs.AbstractPreferences
import java.util.prefs.BackingStoreException
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

  @Test
  fun blankPreferenceIsNotARememberedProject() {
    val preferences = ControlledProjectPreferences(read = { "  " })
    assertNull(LastProjectStore(preferences).load())
  }

  @Test
  fun preferenceReadFailureIsNotMistakenForAnEmptyPreference() {
    val preferences = ControlledProjectPreferences({ null }, syncFailure = "storage denied")
    kotlin.test.assertFailsWith<BackingStoreException> { LastProjectStore(preferences).load() }
  }

  private fun withPreferences(test: (InMemoryPreferences) -> Unit) = test(InMemoryPreferences())
}

/** Allows startup tests to hold or fail a preference read without touching the user's settings. */
internal class ControlledProjectPreferences(
    var read: () -> String?,
    var syncFailure: String? = null,
    var onSync: () -> Unit = {},
) : AbstractPreferences(null, "") {
  override fun getSpi(key: String): String? = read()

  override fun putSpi(key: String, value: String) = Unit

  override fun removeSpi(key: String) = Unit

  override fun removeNodeSpi() = Unit

  override fun keysSpi(): Array<String> = emptyArray()

  override fun childrenNamesSpi(): Array<String> = emptyArray()

  override fun childSpi(name: String): AbstractPreferences = error("Unexpected child preference")

  override fun syncSpi() {
    onSync()
    syncFailure?.let { throw BackingStoreException(it) }
  }

  override fun flushSpi() = Unit
}
