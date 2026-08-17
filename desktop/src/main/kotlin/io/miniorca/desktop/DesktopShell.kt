package io.miniorca.desktop

import java.util.prefs.Preferences

data class ExplorerRow(
    val path: String,
    val name: String,
    val depth: Int,
    val directory: Boolean,
    val analysisStatus: String = "missing",
)

fun useNarrowLayout(widthDp: Float): Boolean = widthDp < 1000f

/** Builds a stable project-relative explorer without exposing filesystem paths. */
fun explorerRows(files: List<IndexedFile>, filter: String = ""): List<ExplorerRow> {
    val matching = files
        .filter { filter.isBlank() || it.path.contains(filter, ignoreCase = true) }
        .associateBy { it.path }
    if (matching.isEmpty()) return emptyList()

    val directories = sortedSetOf<String>()
    matching.keys.forEach { path ->
        path.substringBeforeLast('/', "").takeIf { it.isNotBlank() }?.let { parent ->
            parent.split('/').indices.forEach { index -> directories += parent.split('/').take(index + 1).joinToString("/") }
        }
    }
    val children = mutableMapOf<String, MutableList<String>>()
    directories.forEach { directory ->
        children.getOrPut(directory.substringBeforeLast('/', "")) { mutableListOf() } += directory
    }
    matching.keys.forEach { path ->
        children.getOrPut(path.substringBeforeLast('/', "")) { mutableListOf() } += path
    }

    val rows = mutableListOf<ExplorerRow>()
    fun appendChildren(parent: String) {
        children[parent].orEmpty()
            .sortedWith(compareBy<String> { it !in directories }.thenBy { it.substringAfterLast('/').lowercase() })
            .forEach { path ->
                val directory = path in directories
                rows += ExplorerRow(path, path.substringAfterLast('/'), path.count { it == '/' }, directory, matching[path]?.analysisStatus ?: "missing")
                if (directory) appendChildren(path)
            }
    }
    appendChildren("")
    return rows
}

fun explorerDirectories(files: List<IndexedFile>): Set<String> = explorerRows(files)
    .filter { it.directory }
    .mapTo(linkedSetOf()) { it.path }

fun visibleExplorerRows(files: List<IndexedFile>, filter: String, collapsedDirectories: Set<String>): List<ExplorerRow> {
    val rows = explorerRows(files, filter)
    if (filter.isNotBlank()) return rows
    return rows.filter { row -> collapsedDirectories.none { row.path.startsWith("$it/") } }
}

fun analysisBadge(status: String): String = when (status.lowercase()) {
    "fresh" -> "✓ Fresh"
    "stale" -> "● Stale"
    "failed" -> "! Failed"
    "running" -> "… Analyzing"
    else -> "○ Not analyzed"
}

data class PaneWidths(val explorer: Float = 270f, val action: Float = 390f) {
    fun withExplorer(value: Float) = copy(explorer = value.coerceIn(180f, 520f))
    fun withAction(value: Float) = copy(action = value.coerceIn(280f, 560f))
}

class PaneWidthStore(private val preferences: Preferences = Preferences.userNodeForPackage(PaneWidthStore::class.java)) {
    fun load(): PaneWidths = PaneWidths(
        explorer = preferences.getFloat("explorer-width", 270f).coerceIn(180f, 520f),
        action = preferences.getFloat("action-width", 390f).coerceIn(280f, 560f),
    )

    fun save(widths: PaneWidths) {
        preferences.putFloat("explorer-width", widths.explorer)
        preferences.putFloat("action-width", widths.action)
    }
}
