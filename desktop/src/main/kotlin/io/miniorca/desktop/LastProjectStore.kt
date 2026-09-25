package io.miniorca.desktop

import java.util.prefs.Preferences

class LastProjectStore(
    private val preferences: Preferences =
        Preferences.userNodeForPackage(LastProjectStore::class.java),
) {
  fun load(): String? {
    // Preferences.get can silently return its default when a backing read fails. Sync first so
    // storage failures remain distinguishable from an unset last-project preference.
    preferences.sync()
    return preferences.get(LAST_PROJECT_PATH, null)?.takeIf { it.isNotBlank() }
  }

  fun save(path: String) {
    if (path.isBlank()) return
    preferences.put(LAST_PROJECT_PATH, path)
    preferences.flush()
  }

  private companion object {
    const val LAST_PROJECT_PATH = "last-project-path"
  }
}
