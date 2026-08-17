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
    @SerialName("generation_id") val generationId: String,
    @SerialName("project_id") val projectId: String,
    @SerialName("project_revision") val projectRevision: String,
    @SerialName("base_file_hash") val baseFileHash: String,
    @SerialName("target_path") val targetPath: String,
    @SerialName("target_symbol") val targetSymbol: String,
    @SerialName("scope_mode") val scopeMode: String,
    @SerialName("candidate_content") val candidateContent: String,
    @SerialName("candidate_hash") val candidateHash: String,
    val validation: GenerationValidation,
    @SerialName("context_manifest") val contextManifest: ContextManifest,
)

@Serializable data class ProjectIndex(@SerialName("project_id") val projectId: String, @SerialName("project_revision") val projectRevision: String, val files: List<IndexedFile> = emptyList())
@Serializable data class IndexedFile(val path: String, @SerialName("content_hash") val contentHash: String, val language: String, val binary: Boolean, @SerialName("analysis_status") val analysisStatus: String = "missing", val symbols: List<SymbolInfo> = emptyList())
@Serializable data class SymbolInfo(val name: String, val kind: String, val signature: String = "", @SerialName("start_line") val startLine: Int = 0, @SerialName("end_line") val endLine: Int = 0, val confidence: String, @SerialName("atomic_target") val atomicTarget: Boolean)
@Serializable data class SymbolsResponse(@SerialName("project_id") val projectId: String, @SerialName("project_revision") val projectRevision: String, val path: String, val symbols: List<SymbolInfo> = emptyList())
@Serializable data class FileAnalysis(val path: String, val status: String, val purpose: String = "", val responsibilities: List<String> = emptyList(), val dependencies: List<String> = emptyList(), @SerialName("side_effects") val sideEffects: List<String> = emptyList(), val risks: List<Finding> = emptyList(), val suggestions: List<Suggestion> = emptyList(), @SerialName("symbol_explanations") val symbolExplanations: Map<String, String> = emptyMap(), val failure: String = "", @SerialName("generated_at") val generatedAt: String = "")
@Serializable data class Finding(val severity: String, val summary: String)
@Serializable data class Suggestion(val title: String, val summary: String, @SerialName("target_symbol") val targetSymbol: String = "", val action: String = "")
@Serializable data class ContextManifest(val included: List<ContextFile> = emptyList(), val excluded: List<ContextDecision> = emptyList(), @SerialName("estimated_tokens") val estimatedTokens: Int = 0, @SerialName("byte_limit") val byteLimit: Int = 0, @SerialName("token_limit") val tokenLimit: Int = 0, val truncated: Boolean = false)
@Serializable data class ContextFile(val path: String, @SerialName("size_bytes") val sizeBytes: Long, val hash: String, @SerialName("estimated_tokens") val estimatedTokens: Int)
@Serializable data class ContextDecision(val path: String, val include: Boolean, val reason: String)
@Serializable data class GenerationValidation(val applicable: Boolean, @SerialName("scope_mode") val scopeMode: String, val diagnostics: List<GenerationFinding> = emptyList(), val diff: UnifiedDiff)
@Serializable data class GenerationFinding(val code: String, val message: String)
@Serializable data class UnifiedDiff(@SerialName("old_path") val oldPath: String, @SerialName("new_path") val newPath: String, val lines: List<DiffLine> = emptyList())
@Serializable data class DiffLine(val kind: String, @SerialName("old_line") val oldLine: Int = 0, @SerialName("new_line") val newLine: Int = 0, val text: String)
@Serializable data class CandidateCheckReport(@SerialName("target_path") val targetPath: String, val applicable: Boolean, val checks: List<CandidateCheck> = emptyList())
@Serializable data class CandidateCheck(val name: String, val required: Boolean = false, val state: String, val command: List<String> = emptyList(), val output: String = "")
@Serializable data class CandidateComparison(val left: CandidateComparisonItem, val right: CandidateComparisonItem)
@Serializable data class CandidateComparisonItem(@SerialName("generation_id") val generationId: String, @SerialName("candidate_hash") val candidateHash: String, @SerialName("target_path") val targetPath: String, @SerialName("target_symbol") val targetSymbol: String, val action: String, @SerialName("scope_mode") val scopeMode: String, @SerialName("diff_lines") val diffLines: Int, val applicable: Boolean, val checks: String, val model: String, @SerialName("user_note") val userNote: String = "")
@Serializable data class ReviewExport(val filename: String, val markdown: String)
@Serializable data class ApplyResult(@SerialName("project_revision") val projectRevision: String, @SerialName("post_apply_hash") val postApplyHash: String, @SerialName("undo_available") val undoAvailable: Boolean)
@Serializable data class AuditEntry(val action: String, @SerialName("target_path") val targetPath: String, val outcome: String, val timestamp: String)
@Serializable data class ImpactPreview(@SerialName("target_path") val targetPath: String, @SerialName("target_symbol") val targetSymbol: String = "", val references: List<ImpactReference> = emptyList())
@Serializable data class ImpactReference(val path: String, val symbol: String = "", val confidence: String, val reason: String)
@Serializable data class GitStatus(val available: Boolean, val branch: String = "", @SerialName("file_state") val fileState: String = "", @SerialName("diff_state") val diffState: String = "")
@Serializable data class ActivityEntry(val role: String, val content: String, val phase: String, val timestamp: String, @SerialName("target_file") val targetFile: String = "", @SerialName("target_symbol") val targetSymbol: String = "")
@Serializable data class EffectiveModel(val profile: String, val model: String, val timeout: String = "")
@Serializable data class ApiError(val message: String = "", @SerialName("user_message") val userMessage: String = "")
