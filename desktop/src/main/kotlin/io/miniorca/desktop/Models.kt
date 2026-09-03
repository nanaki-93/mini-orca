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
data class ProjectIndex(
    @SerialName("project_id") val projectId: String,
    @SerialName("project_revision") val projectRevision: String,
    @SerialName("schema_version") val schemaVersion: String = "",
    @SerialName("generated_at") val generatedAt: String = "",
    val files: List<IndexedFile> = emptyList()
)

@Serializable
data class IndexedFile(
    val path: String,
    @SerialName("content_hash") val contentHash: String,
    val language: String,
    val binary: Boolean,
    @SerialName("size_bytes") val sizeBytes: Long = 0,
    @SerialName("line_count") val lineCount: Int = 0,
    @SerialName("modified_at") val modifiedAt: String = "",
    val imports: List<String> = emptyList(),
    @SerialName("analysis_status") val analysisStatus: String = "missing",
    val symbols: List<SymbolInfo> = emptyList(),
    val diagnostics: List<IndexDiagnostic> = emptyList()
)

@Serializable data class IndexDiagnostic(val message: String, val line: Int = 0)

@Serializable
data class SymbolInfo(
    val name: String,
    val kind: String,
    val signature: String = "",
    @SerialName("start_line") val startLine: Int = 0,
    @SerialName("end_line") val endLine: Int = 0,
    val confidence: String,
    @SerialName("atomic_target") val atomicTarget: Boolean
)

@Serializable
data class SymbolsResponse(
    @SerialName("project_id") val projectId: String,
    @SerialName("project_revision") val projectRevision: String,
    val path: String,
    val symbols: List<SymbolInfo> = emptyList()
)

@Serializable
data class FileAnalysis(
    val path: String,
    val status: String,
    val purpose: String = "",
    val responsibilities: List<String> = emptyList(),
    val dependencies: List<String> = emptyList(),
    @SerialName("side_effects") val sideEffects: List<String> = emptyList(),
    val risks: List<Finding> = emptyList(),
    val suggestions: List<Suggestion> = emptyList(),
    @SerialName("symbol_explanations") val symbolExplanations: Map<String, String> = emptyMap(),
    val failure: String = "",
    @SerialName("generated_at") val generatedAt: String = ""
)

@Serializable
data class BugTaskSpec(
    @SerialName("schema_version") val schemaVersion: String = "",
    @SerialName("target_path") val targetPath: String = "",
    @SerialName("target_symbol") val targetSymbol: String = "",
    @SerialName("target_signature") val targetSignature: String = "",
    @SerialName("acceptance_criteria") val acceptanceCriteria: List<String> = emptyList(),
    @SerialName("non_goals") val nonGoals: List<String> = emptyList(),
    @SerialName("go_test_candidate") val goTestCandidate: GoTestCandidateSpec? = null
)

@Serializable data class GoTestCandidateSpec(val name: String = "", val content: String = "")

@Serializable
data class Finding(
    val severity: String,
    val summary: String,
    @SerialName("task_spec") val taskSpec: BugTaskSpec? = null
)

@Serializable
data class Suggestion(
    val title: String,
    val summary: String,
    @SerialName("target_symbol") val targetSymbol: String = "",
    val action: String = ""
)

@Serializable
data class ContextManifest(
    val included: List<ContextFile> = emptyList(),
    val excluded: List<ContextDecision> = emptyList(),
    @SerialName("estimated_tokens") val estimatedTokens: Int = 0,
    @SerialName("byte_limit") val byteLimit: Int = 0,
    @SerialName("token_limit") val tokenLimit: Int = 0,
    val truncated: Boolean = false,
    val scope: String = "",
    val model: String = "",
    @SerialName("provider_origin") val providerOrigin: String = "",
    @SerialName("remote_provider") val remoteProvider: Boolean = false
)

@Serializable
data class ContextFile(
    val path: String,
    @SerialName("size_bytes") val sizeBytes: Long,
    val hash: String,
    @SerialName("estimated_tokens") val estimatedTokens: Int
)

@Serializable
data class ContextDecision(val path: String, val include: Boolean, val reason: String)

@Serializable
data class DeclarationValidation(
    val applicable: Boolean,
    @SerialName("scope_mode") val scopeMode: String,
    val diagnostics: List<DeclarationFinding> = emptyList(),
    val diff: UnifiedDiff
)

@Serializable data class DeclarationFinding(val code: String, val message: String)

@Serializable
data class UnifiedDiff(
    @SerialName("old_path") val oldPath: String,
    @SerialName("new_path") val newPath: String,
    val lines: List<DiffLine> = emptyList()
)

@Serializable
data class DiffLine(
    val kind: String,
    @SerialName("old_line") val oldLine: Int = 0,
    @SerialName("new_line") val newLine: Int = 0,
    val text: String
)

@Serializable
data class DraftCheckReport(
    @SerialName("target_path") val targetPath: String,
    val applicable: Boolean,
    val checks: List<DraftCheck> = emptyList(),
    @SerialName("draft_id") val draftId: String = "",
    @SerialName("draft_revision") val draftRevision: Long = 0,
    @SerialName("draft_hash") val draftHash: String = "",
    @SerialName("candidate_hash") val compositionHash: String = "",
    @SerialName("project_id") val projectId: String = "",
    @SerialName("project_revision") val projectRevision: String = "",
    @SerialName("base_file_hash") val baseFileHash: String = ""
)

@Serializable
data class DraftCheck(
    val name: String,
    val required: Boolean = false,
    val state: String,
    val command: List<String> = emptyList(),
    val output: String = "",
    @SerialName("exit_code") val exitCode: Int = 0
)

@Serializable
data class ApplyResult(
    @SerialName("project_revision") val projectRevision: String,
    @SerialName("post_apply_hash") val postApplyHash: String,
    @SerialName("undo_available") val undoAvailable: Boolean,
    val audit: AuditEntry? = null,
    val index: ProjectIndex? = null
)

@Serializable
data class AuditEntry(
    val action: String,
    @SerialName("target_path") val targetPath: String,
    val outcome: String,
    val timestamp: String
)

@Serializable
data class ImpactPreview(
    @SerialName("target_path") val targetPath: String,
    @SerialName("target_symbol") val targetSymbol: String = "",
    val references: List<ImpactReference> = emptyList()
)

@Serializable
data class ImpactReference(
    val path: String,
    val symbol: String = "",
    val confidence: String,
    val reason: String
)

@Serializable
data class GitStatus(
    val available: Boolean,
    val branch: String = "",
    @SerialName("file_state") val fileState: String = "",
    @SerialName("diff_state") val diffState: String = ""
)

enum class ModelScope(val wireValue: String, val label: String) {
  Analyze("analyze", "Analyze"),
  Bug("bug", "Bugs"),
  Function("function", "Function edits"),
}

/** Safe metadata returned for one prompt destination. */
@Serializable
data class ScopedModel(
    val scope: String = "",
    val profile: String = "",
    val model: String = "",
    @SerialName("provider_origin") val providerOrigin: String = "",
    @SerialName("remote_provider") val remoteProvider: Boolean = false,
    val timeout: String = "",
    @SerialName("reasoning_effort") val reasoningEffort: String = "",
)

@Serializable
data class ModelCatalog(
    val scopes: Map<String, ScopedModel> = emptyMap(),
) {
  fun forScope(scope: ModelScope): ScopedModel =
      scopes[scope.wireValue] ?: ScopedModel(scope = scope.wireValue)
}

fun ModelCatalog.identity(): List<ScopedModel> = ModelScope.entries.map(::forScope)

data class ScopedConfirmationState(
    val analyze: Boolean = false,
    val bug: Boolean = false,
    val function: Boolean = false,
) {
  fun confirmed(scope: ModelScope): Boolean =
      when (scope) {
        ModelScope.Analyze -> analyze
        ModelScope.Bug -> bug
        ModelScope.Function -> function
      }

  fun withConfirmation(scope: ModelScope, confirmed: Boolean): ScopedConfirmationState =
      when (scope) {
        ModelScope.Analyze -> copy(analyze = confirmed)
        ModelScope.Bug -> copy(bug = confirmed)
        ModelScope.Function -> copy(function = confirmed)
      }
}

@Serializable
data class DaemonStatus(val status: String, val version: String, val workflow: String = "")

@Serializable
data class ApiError(
    val type: String = "",
    val message: String = "",
    @SerialName("user_message") val userMessage: String = ""
)

@Serializable data class ProjectAnalysisRisk(val severity: String = "", val summary: String = "")

@Serializable
data class StructuredProjectAnalysis(
    @SerialName("schema_version") val schemaVersion: String = "",
    @SerialName("project_id") val projectId: String = "",
    @SerialName("project_revision") val projectRevision: String = "",
    val purpose: String = "",
    val architecture: String = "",
    val components: List<String> = emptyList(),
    @SerialName("entry_points") val entryPoints: List<String> = emptyList(),
    val flows: List<String> = emptyList(),
    val risks: List<ProjectAnalysisRisk> = emptyList(),
    @SerialName("next_steps") val nextSteps: List<String> = emptyList(),
    val status: String = "",
    val failure: String = "",
    val model: String = "",
    val profile: String = "",
    @SerialName("prompt_version") val promptVersion: String = "",
    @SerialName("generated_at") val generatedAt: String = "",
)

@Serializable
data class ProjectMetrics(
    val type: String = "",
    @SerialName("build_file") val buildFile: String = "",
    @SerialName("file_count") val fileCount: Int = 0,
    @SerialName("source_file_count") val sourceFileCount: Int = 0,
    @SerialName("total_lines") val totalLines: Int = 0,
    val languages: Map<String, Int> = emptyMap()
)

@Serializable
data class AnalysisCoverage(
    val total: Int = 0,
    val fresh: Int = 0,
    val stale: Int = 0,
    val missing: Int = 0,
    val failed: Int = 0,
    val running: Int = 0
)

@Serializable
data class FindingCounts(
    val verified: Int = 0,
    @SerialName("ai_suggestions") val aiSuggestions: Int = 0
)

@Serializable
data class ProjectOverview(
    @SerialName("project_id") val projectId: String = "",
    @SerialName("project_revision") val projectRevision: String = "",
    val metrics: ProjectMetrics = ProjectMetrics(),
    val analysis: StructuredProjectAnalysis = StructuredProjectAnalysis(),
    @SerialName("analysis_coverage") val analysisCoverage: AnalysisCoverage = AnalysisCoverage(),
    @SerialName("finding_counts") val findingCounts: FindingCounts = FindingCounts()
)

@Serializable
data class FindingLocation(
    val path: String = "",
    @SerialName("start_line") val startLine: Int = 0,
    @SerialName("end_line") val endLine: Int = 0,
    val symbol: String = ""
)

@Serializable
data class UnifiedFinding(
    @SerialName("id") val id: String = "",
    val source: String = "",
    val confidence: String = "",
    val severity: String = "",
    val title: String = "",
    val message: String = "",
    val rule: String = "",
    @SerialName("project_id") val projectId: String = "",
    @SerialName("project_revision") val projectRevision: String = "",
    @SerialName("file_hash") val fileHash: String = "",
    val location: FindingLocation = FindingLocation(),
    val evidence: String = "",
    val status: String = "",
    val freshness: String = "",
    @SerialName("detected_at") val detectedAt: String = "",
    @SerialName("originating_analysis") val originatingAnalysis: String = "",
    @SerialName("task_spec") val taskSpec: BugTaskSpec? = null
)

@Serializable
data class FindingsResponse(
    @SerialName("project_id") val projectId: String = "",
    @SerialName("project_revision") val projectRevision: String = "",
    val findings: List<UnifiedFinding> = emptyList()
)

@Serializable
data class FindingFilter(
    val source: String = "",
    val confidence: String = "",
    val severity: String = "",
    val status: String = "",
    val freshness: String = ""
)

@Serializable
data class GoScanPhase(
    val name: String = "",
    val state: String = "",
    val command: List<String> = emptyList(),
    val output: String = "",
    @SerialName("exit_code") val exitCode: Int = 0
)

@Serializable
data class GoScanReport(
    @SerialName("project_id") val projectId: String = "",
    @SerialName("project_revision") val projectRevision: String = "",
    val status: String = "",
    @SerialName("started_at") val startedAt: String = "",
    @SerialName("updated_at") val updatedAt: String = "",
    @SerialName("completed_at") val completedAt: String = "",
    val phases: List<GoScanPhase> = emptyList()
)

@Serializable
data class AnalyzeAllFileJob(
    val path: String = "",
    val status: String = "",
    val attempts: Int = 0,
    val error: String = ""
)

@Serializable
data class AnalyzeAllJob(
    @SerialName("project_id") val projectId: String = "",
    @SerialName("project_revision") val projectRevision: String = "",
    val status: String = "",
    @SerialName("max_files") val maxFiles: Int = 0,
    @SerialName("max_retries") val maxRetries: Int = 0,
    val files: List<AnalyzeAllFileJob> = emptyList(),
    @SerialName("created_at") val createdAt: String = "",
    @SerialName("updated_at") val updatedAt: String = ""
)

@Serializable
data class ChatSession(
    @SerialName("id") val id: String = "",
    @SerialName("project_id") val projectId: String = "",
    @SerialName("project_revision") val projectRevision: String = "",
    @SerialName("base_file_hash") val baseFileHash: String = "",
    @SerialName("open_path") val openPath: String = "",
    val mode: String = "",
    @SerialName("target_symbol") val targetSymbol: String = "",
    val state: String = "",
    @SerialName("latest_draft_id") val latestDraftId: String = "",
    val messages: List<ChatSessionMessage> = emptyList(),
    @SerialName("created_at") val createdAt: String = "",
    @SerialName("updated_at") val updatedAt: String = "",
    @SerialName("task_spec") val taskSpec: BugTaskSpec? = null,
    @SerialName("repair_count") val repairCount: Int = 0
)

@Serializable
data class ChatSessionMessage(
    val role: String = "",
    val content: String = "",
    @SerialName("draft_id") val draftId: String = "",
    @SerialName("created_at") val createdAt: String = ""
)

@Serializable
data class ChatDraftProposal(
    @SerialName("session_id") val sessionId: String = "",
    val draft: DeclarationDraft = DeclarationDraft(),
    @SerialName("assistant_message")
    val assistantMessage: ChatSessionMessage = ChatSessionMessage(),
    @SerialName("context_manifest") val contextManifest: ContextManifest = ContextManifest()
)

@Serializable
data class DeclarationDraft(
    @SerialName("id") val id: String = "",
    @SerialName("project_id") val projectId: String = "",
    @SerialName("project_revision") val projectRevision: String = "",
    @SerialName("base_file_hash") val baseFileHash: String = "",
    @SerialName("target_path") val targetPath: String = "",
    val mode: String = "",
    @SerialName("target_symbol") val targetSymbol: String = "",
    val declaration: String = "",
    val imports: List<String> = emptyList(),
    val revision: Long = 0,
    val hash: String = "",
    @SerialName("candidate_hash") val compositionHash: String = "",
    @SerialName("parent_draft_id") val parentDraftId: String = "",
    @SerialName("previous_hash") val previousHash: String = "",
    val state: String = "",
    val validation: DeclarationValidation? = null,
    @SerialName("task_spec") val taskSpec: BugTaskSpec? = null
)
