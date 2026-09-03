package io.miniorca.desktop

import java.util.prefs.Preferences

class LastProjectStore(
    private val preferences: Preferences =
        Preferences.userNodeForPackage(LastProjectStore::class.java),
) {
  fun load(): String? = preferences.get(LAST_PROJECT_PATH, null)?.takeIf { it.isNotBlank() }

  fun save(path: String) {
    if (path.isBlank()) return
    preferences.put(LAST_PROJECT_PATH, path)
    preferences.flush()
  }

  private companion object {
    const val LAST_PROJECT_PATH = "last-project-path"
  }
}
