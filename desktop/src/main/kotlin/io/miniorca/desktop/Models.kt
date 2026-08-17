package io.miniorca.desktop

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

@Serializable
data class ProjectAnalysis(
    @SerialName("project_id") val projectId: String,
    @SerialName("project_revision") val projectRevision: String,
    val name: String,
    val path: String,
    val type: String,
    @SerialName("build_file") val buildFile: String = "",
    @SerialName("file_count") val fileCount: Int,
    @SerialName("source_file_count") val sourceFileCount: Int,
    @SerialName("total_lines") val totalLines: Int,
    val languages: Map<String, Int> = emptyMap(),
    val files: List<String> = emptyList(),
    @SerialName("analysis_file") val analysisFile: String,
    val summary: String,
    @SerialName("ai_status") val aiStatus: String,
    @SerialName("analyzed_at") val analyzedAt: String,
)

@Serializable
data class ProjectFileInfo(
    val path: String,
    @SerialName("content_hash") val contentHash: String,
    val name: String,
    val extension: String = "",
    val language: String,
    @SerialName("size_bytes") val sizeBytes: Long,
    @SerialName("line_count") val lineCount: Int,
    @SerialName("modified_at") val modifiedAt: String,
    val binary: Boolean,
    val content: String = "",
)

@Serializable
data class GenerationResult(
    val output: String,
    val phase: String = "coding",
    val metadata: Map<String, String> = emptyMap(),
)
