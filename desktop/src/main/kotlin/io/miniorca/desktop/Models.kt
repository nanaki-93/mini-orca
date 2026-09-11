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
    @SerialName("engineering_insight") val engineeringInsight: EngineeringInsight? = null,
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
data class EngineeringInsight(
    val mechanism: String = "",
    @SerialName("why_it_matters_here") val whyItMattersHere: String = "",
    @SerialName("tradeoff_or_failure_mode") val tradeoffOrFailureMode: String = "",
    @SerialName("transferable_lesson") val transferableLesson: String = ""
)

@Serializable
data class Finding(
    val severity: String,
    val summary: String,
    @SerialName("task_spec") val taskSpec: BugTaskSpec? = null,
    @SerialName("engineering_insight") val engineeringInsight: EngineeringInsight? = null
)

@Serializable
data class Suggestion(
    val title: String,
    val summary: String,
    @SerialName("target_symbol") val targetSymbol: String = "",
    val action: String = "",
    @SerialName("engineering_insight") val engineeringInsight: EngineeringInsight? = null
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
data class DeclarationExplanation(
    val version: String,
    @SerialName("project_id") val projectId: String,
    @SerialName("project_revision") val projectRevision: String,
    @SerialName("base_file_hash") val baseFileHash: String,
    val anchor: DeclarationSourceAnchor,
    val summary: String,
    val behavior: List<String> = emptyList(),
    val inputs: List<String> = emptyList(),
    val outputs: List<String> = emptyList(),
    @SerialName("side_effects") val sideEffects: List<String> = emptyList(),
    @SerialName("error_behavior") val errorBehavior: List<String> = emptyList(),
    @SerialName("engineering_insight") val engineeringInsight: EngineeringInsight? = null,
    @SerialName("context_manifest") val contextManifest: ContextManifest,
)

@Serializable
data class DeclarationSourceAnchor(
    val path: String,
    val symbol: String,
    val signature: String,
    @SerialName("start_line") val startLine: Int,
    @SerialName("end_line") val endLine: Int,
)

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

/** Current-session permission for the fixed commands that execute project code. */
@Serializable
data class ExecutionTrust(
    @SerialName("project_id") val projectId: String = "",
    @SerialName("project_revision") val projectRevision: String = "",
    val trusted: Boolean = false,
    val commands: List<List<String>> = emptyList(),
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

@Serializable
data class ProjectAnalysisRisk(
    val severity: String = "",
    val summary: String = "",
    @SerialName("engineering_insight") val engineeringInsight: EngineeringInsight? = null
)

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
    @SerialName("engineering_insight") val engineeringInsight: EngineeringInsight? = null,
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
    @SerialName("finding_counts") val findingCounts: FindingCounts = FindingCounts(),
    @SerialName("analysis_run") val analysisRun: AnalysisRun? = null
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
    @SerialName("task_spec") val taskSpec: BugTaskSpec? = null,
    @SerialName("engineering_insight") val engineeringInsight: EngineeringInsight? = null,
    val category: String = ""
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
data class SecuritySourceAnchor(
    val path: String = "",
    @SerialName("start_line") val startLine: Int = 0,
    @SerialName("end_line") val endLine: Int = 0,
    val symbol: String = "",
)

@Serializable
data class SecurityFinding(
    val id: String = "",
    val rule: String = "",
    val category: String = "",
    val title: String = "",
    @SerialName("source_anchor") val anchor: SecuritySourceAnchor = SecuritySourceAnchor(),
    val severity: String = "",
    val confidence: String = "",
    @SerialName("evidence_kind") val evidenceKind: String = "",
    @SerialName("observed_condition") val observedCondition: String = "",
    @SerialName("preconditions_or_unknowns") val preconditions: String = "",
    val remediation: String = "",
    @SerialName("verification_idea") val verificationIdea: String = "",
    val cwe: String = "",
    val reference: String = "",
    val triage: String = "",
    @SerialName("verification_state") val verificationState: String = "",
    @SerialName("engineering_insight") val engineeringInsight: EngineeringInsight? = null,
)

@Serializable
data class SecurityFileReport(
    @SerialName("schema_version") val schemaVersion: String = "",
    @SerialName("project_id") val projectId: String = "",
    @SerialName("project_revision") val projectRevision: String = "",
    val path: String = "",
    @SerialName("content_hash") val contentHash: String = "",
    val status: String = "not_run",
    val source: String = "",
    @SerialName("rule_set_version") val ruleSetVersion: String = "",
    val findings: List<SecurityFinding> = emptyList(),
    val reason: String = "",
    val model: String = "",
    @SerialName("configured_model") val configuredModel: String = "",
    val profile: String = "",
    val scope: String = "",
    @SerialName("provider_origin") val providerOrigin: String = "",
    @SerialName("reasoning_effort") val reasoningEffort: String = "",
    @SerialName("prompt_version") val promptVersion: String = "",
    @SerialName("context_policy_version") val contextPolicyVersion: String = "",
    @SerialName("generated_at") val generatedAt: String = "",
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
data class PerformanceJobFile(
    val path: String = "",
    @SerialName("content_hash") val contentHash: String = "",
    val status: String = "",
    val attempts: Int = 0,
    val reason: String = "",
    val error: String = "",
)

@Serializable
data class PerformanceJob(
    val id: String = "",
    val generation: String = "",
    @SerialName("project_id") val projectId: String = "",
    @SerialName("project_revision") val projectRevision: String = "",
    val root: String = "",
    @SerialName("policy_fingerprint") val policyFingerprint: String = "",
    @SerialName("queue_id") val queueId: String = "",
    val status: String = "",
    @SerialName("max_files") val maxFiles: Int = 0,
    @SerialName("run_budget") val runBudget: Long = 0,
    val elapsed: Long = 0,
    val files: List<PerformanceJobFile> = emptyList(),
    @SerialName("created_at") val createdAt: String = "",
    @SerialName("updated_at") val updatedAt: String = "",
)

@Serializable
data class PerformanceQueuePreview(
    @SerialName("project_id") val projectId: String = "",
    @SerialName("project_revision") val projectRevision: String = "",
    @SerialName("policy_fingerprint") val policyFingerprint: String = "",
    @SerialName("queue_id") val queueId: String = "",
    @SerialName("max_files") val maxFiles: Int = 0,
    val files: List<PerformanceJobFile> = emptyList(),
    val excluded: Int = 0,
    val oversized: Int = 0,
    @SerialName("outside_limit") val outsideLimit: Int = 0,
    val provider: ScopedModel? = null,
)

@Serializable
data class PerformanceReport(
    @SerialName("project_id") val projectId: String = "",
    @SerialName("project_revision") val projectRevision: String = "",
    @SerialName("queue_id") val queueId: String = "",
    val status: String = "",
    val counts: Map<String, Int> = emptyMap(),
    val categories: Map<String, Int> = emptyMap(),
    val paths: Map<String, String> = emptyMap(),
    val findings: List<PerformanceFinding> = emptyList(),
)

@Serializable
data class PerformanceFinding(
    val id: String = "",
    val category: String = "",
    @SerialName("potential_impact") val potentialImpact: String = "",
    val confidence: String = "",
    val title: String = "",
    @SerialName("observed_pattern") val observedPattern: String = "",
    @SerialName("workload_conditions") val workloadConditions: String = "",
    val recommendation: String = "",
    val tradeoff: String = "",
    @SerialName("verification_plan") val verificationPlan: String = "",
    @SerialName("start_line") val startLine: Int = 0,
    @SerialName("end_line") val endLine: Int = 0,
    val symbol: String = "",
    @SerialName("engineering_insight") val engineeringInsight: EngineeringInsight? = null,
)

/** A completed or terminal PERF-02 comparison. Rendering this value never starts a benchmark. */
@Serializable
data class GoBenchmarkComparison(
    @SerialName("draft_id") val draftId: String = "",
    @SerialName("draft_revision") val draftRevision: Long = 0,
    @SerialName("draft_hash") val draftHash: String = "",
    @SerialName("project_id") val projectId: String = "",
    @SerialName("project_revision") val projectRevision: String = "",
    @SerialName("base_file_hash") val baseFileHash: String = "",
    @SerialName("target_path") val targetPath: String = "",
    val benchmark: String = "",
    val scope: String = "",
    val status: String = "",
    val reason: String = "",
    val command: List<String> = emptyList(),
    val base: GoBenchmarkMeasurement? = null,
    val candidate: GoBenchmarkMeasurement? = null,
)

@Serializable
data class GoBenchmarkCatalog(
    @SerialName("draft_id") val draftId: String = "",
    @SerialName("draft_revision") val draftRevision: Long = 0,
    @SerialName("draft_hash") val draftHash: String = "",
    @SerialName("project_id") val projectId: String = "",
    @SerialName("project_revision") val projectRevision: String = "",
    @SerialName("base_file_hash") val baseFileHash: String = "",
    @SerialName("target_path") val targetPath: String = "",
    val available: Boolean = false,
    val trusted: Boolean = false,
    val reason: String = "",
    val benchmarks: List<GoBenchmarkChoice> = emptyList(),
)

@Serializable
data class GoBenchmarkChoice(
    val name: String = "",
    val command: List<String> = emptyList(),
    val scope: String = "",
)

@Serializable data class GoBenchmarkMeasurement(val samples: List<GoBenchmarkSample> = emptyList())

@Serializable
data class GoBenchmarkSample(
    val iterations: Long = 0,
    @SerialName("ns_per_op") val nanosecondsPerOperation: Double = 0.0,
    @SerialName("bytes_per_op") val bytesPerOperation: Long? = null,
    @SerialName("allocs_per_op") val allocationsPerOperation: Long? = null,
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
    @SerialName("task_spec") val taskSpec: BugTaskSpec? = null,
    @SerialName("engineering_insight") val engineeringInsight: EngineeringInsight? = null
)

/**
 * Authoritative project-run wire contracts; unknown progress is never represented as zero findings.
 */
@Serializable
data class AnalysisRunLimits(
    @SerialName("batch_files") val batchFiles: Int,
    @SerialName("budget_seconds") val budgetSeconds: Int,
    @SerialName("max_attempts_per_stage") val maxAttemptsPerStage: Int,
)

@Serializable
data class AnalysisQueueIdentity(
    @SerialName("project_id") val projectId: String,
    @SerialName("project_revision") val projectRevision: String,
    @SerialName("policy_fingerprint") val policyFingerprint: String,
    @SerialName("provider_fingerprint") val providerFingerprint: String,
    @SerialName("queue_id") val queueId: String,
)

@Serializable
data class AnalysisRunIdentity(
    @SerialName("project_id") val projectId: String,
    @SerialName("project_revision") val projectRevision: String,
    @SerialName("policy_fingerprint") val policyFingerprint: String,
    @SerialName("provider_fingerprint") val providerFingerprint: String,
    @SerialName("queue_id") val queueId: String,
    val id: String,
    val generation: String,
) {
  fun queue() =
      AnalysisQueueIdentity(
          projectId, projectRevision, policyFingerprint, providerFingerprint, queueId)
}

@Serializable
data class AnalysisPreviewRequest(
    @SerialName("project_id") val projectId: String,
    @SerialName("project_revision") val projectRevision: String,
    val scope: String,
    val refresh: Boolean,
    val limits: AnalysisRunLimits,
    @SerialName("resume_run") val resumeRun: AnalysisRunIdentity? = null,
)

@Serializable
data class AnalysisEffectiveModel(
    val scope: String = "",
    val profile: String = "",
    val model: String = "",
    @SerialName("reasoning_effort") val reasoningEffort: String = "",
    @SerialName("provider_origin") val providerOrigin: String = "",
    @SerialName("remote_provider") val remoteProvider: Boolean = false,
    val timeout: Long = 0,
)

@Serializable
data class AnalysisProviderRequirement(
    val id: String,
    val stages: List<String>,
    val model: AnalysisEffectiveModel,
    @SerialName("remote_confirmation_required") val remoteConfirmationRequired: Boolean,
)

@Serializable
data class AnalysisStagePlan(
    val stage: String,
    val eligible: Boolean,
    val cached: Boolean,
    val reason: String = "",
    @SerialName("provider_id") val providerId: String = "",
    @SerialName("max_model_requests") val maxModelRequests: Int,
)

@Serializable
data class AnalysisPlannedFile(
    val path: String,
    @SerialName("content_hash") val contentHash: String,
    val language: String,
    @SerialName("size_bytes") val sizeBytes: Long,
    val stages: List<AnalysisStagePlan>,
)

@Serializable data class AnalysisExcludedFile(val path: String, val reason: String)

@Serializable
data class AnalysisRunPreview(
    @SerialName("schema_version") val schemaVersion: String,
    @SerialName("preview_id") val previewId: String,
    val identity: AnalysisQueueIdentity,
    val scope: String,
    val refresh: Boolean,
    val limits: AnalysisRunLimits,
    val files: List<AnalysisPlannedFile> = emptyList(),
    val excluded: List<AnalysisExcludedFile> = emptyList(),
    val providers: List<AnalysisProviderRequirement> = emptyList(),
    @SerialName("expected_model_requests") val expectedModelRequests: Int,
    @SerialName("max_model_requests") val maxModelRequests: Int,
    @SerialName("security_review_intent_required") val securityReviewIntentRequired: Boolean,
    @SerialName("compatibility_stage") val compatibilityStage: String = "",
)

@Serializable
data class AnalysisRunConfirmations(
    @SerialName("provider_ids") val providerIds: List<String>,
    @SerialName("security_review") val securityReview: Boolean,
)

@Serializable
data class AnalysisRunStartRequest(
    val identity: AnalysisQueueIdentity,
    @SerialName("preview_id") val previewId: String,
    val limits: AnalysisRunLimits,
    val refresh: Boolean,
    val confirmations: AnalysisRunConfirmations,
)

@Serializable
data class AnalysisRunControlRequest(
    val identity: AnalysisRunIdentity,
    val action: String,
    @SerialName("preview_id") val previewId: String = "",
    val confirmations: AnalysisRunConfirmations? = null,
)

@Serializable
data class AnalysisRunCoverage(
    val total: Int = 0,
    val pending: Int = 0,
    val running: Int = 0,
    val succeeded: Int = 0,
    val partial: Int = 0,
    val failed: Int = 0,
    val skipped: Int = 0,
    val unavailable: Int = 0,
)

@Serializable
data class AnalysisSectionProgress(
    val category: String,
    val status: String,
    val coverage: AnalysisRunCoverage,
    @SerialName("finding_count") val findingCount: Int? = null,
)

@Serializable
data class AnalysisStageProgress(
    val stage: String,
    val status: String,
    val attempts: Int,
    val cached: Boolean,
    @SerialName("finding_count") val findingCount: Int? = null,
    @SerialName("report_id") val reportId: String = "",
    val reason: String = "",
)

@Serializable
data class AnalysisRunFile(
    val path: String,
    @SerialName("content_hash") val contentHash: String,
    val language: String,
    val stages: List<AnalysisStageProgress>,
)

@Serializable
data class AnalysisRun(
    @SerialName("schema_version") val schemaVersion: String,
    val identity: AnalysisRunIdentity,
    val plan: AnalysisRunPreview,
    val status: String,
    val files: List<AnalysisRunFile> = emptyList(),
    val sections: List<AnalysisSectionProgress> = emptyList(),
    @SerialName("elapsed_seconds") val elapsedSeconds: Long = 0,
    @SerialName("window_elapsed_seconds") val windowElapsedSeconds: Long = 0,
    @SerialName("window_files_completed") val windowFilesCompleted: Int = 0,
    val reason: String = "",
    @SerialName("created_at") val createdAt: String = "",
    @SerialName("updated_at") val updatedAt: String = "",
) {
  fun isActive(): Boolean = status in setOf("queued", "running", "pausing", "canceling")
}

@Serializable
data class PerformanceFileReport(
    @SerialName("schema_version") val schemaVersion: String = "",
    @SerialName("project_id") val projectId: String,
    @SerialName("project_revision") val projectRevision: String,
    val path: String,
    @SerialName("content_hash") val contentHash: String,
    val status: String,
    val findings: List<PerformanceFinding> = emptyList(),
    val warning: String = "",
    val model: String = "",
    val profile: String = "",
    val scope: String = "",
    @SerialName("provider_origin") val providerOrigin: String = "",
    @SerialName("reasoning_effort") val reasoningEffort: String = "",
    @SerialName("prompt_version") val promptVersion: String = "",
    @SerialName("context_policy_version") val contextPolicyVersion: String = "",
    @SerialName("generated_at") val generatedAt: String = "",
)

@Serializable
data class AnalysisSectionResults(
    val identity: AnalysisRunIdentity,
    val progress: AnalysisSectionProgress,
    val path: String = "",
    val semantic: List<UnifiedFinding> = emptyList(),
    val performance: List<PerformanceFileReport> = emptyList(),
    val security: List<SecurityFileReport> = emptyList(),
    val unclassified: List<UnifiedFinding> = emptyList(),
)
