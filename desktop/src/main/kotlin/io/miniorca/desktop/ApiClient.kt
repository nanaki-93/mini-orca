package io.miniorca.desktop

import java.net.URI
import java.net.URLEncoder
import java.net.http.HttpClient
import java.net.http.HttpRequest
import java.net.http.HttpResponse
import java.nio.charset.StandardCharsets
import java.time.Duration
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.decodeFromString
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json

data class TransportResponse(val status: Int, val body: String)

fun interface DaemonTransport {
  fun send(method: String, path: String, body: String?): TransportResponse
}

@Serializable
private data class ProjectImportRequest(
    @SerialName("project_path") val projectPath: String,
    @SerialName("confirm_remote_provider") val confirmRemoteProvider: Boolean,
)

@Serializable
private data class ProjectRestoreRequest(@SerialName("project_path") val projectPath: String)

@Serializable
private data class ProjectRevisionRequest(
    @SerialName("project_revision") val projectRevision: String
)

@Serializable
private data class FileAnalysisRequest(
    val path: String,
    @SerialName("project_revision") val projectRevision: String,
    val refresh: Boolean,
    @SerialName("confirm_remote_provider") val confirmRemoteProvider: Boolean,
)

@Serializable
private data class FindingStatusRequest(
    @SerialName("project_revision") val projectRevision: String,
    val status: String,
)

@Serializable
private data class AnalyzeAllStartRequest(
    @SerialName("project_revision") val projectRevision: String,
    @SerialName("max_files") val maxFiles: Int,
    @SerialName("max_retries") val maxRetries: Int,
    @SerialName("confirm_remote_provider") val confirmRemoteProvider: Boolean,
)

@Serializable
private data class AnalyzeAllResumeRequest(
    @SerialName("project_revision") val projectRevision: String,
    @SerialName("confirm_remote_provider") val confirmRemoteProvider: Boolean,
)

@Serializable
private data class ChatSessionRequest(
    @SerialName("project_id") val projectId: String,
    @SerialName("project_revision") val projectRevision: String,
    @SerialName("base_file_hash") val baseFileHash: String,
    @SerialName("open_path") val openPath: String,
    val mode: String,
    @SerialName("target_symbol") val targetSymbol: String,
    @SerialName("task_spec") val taskSpec: BugTaskSpec? = null,
)

@Serializable
private data class ChatMessageRequest(
    val message: String,
    @SerialName("parent_draft_id") val parentDraftId: String,
    @SerialName("confirm_remote_provider") val confirmRemoteProvider: Boolean,
    val repair: Boolean,
)

@Serializable
private data class DraftUpdateRequest(
    @SerialName("project_revision") val projectRevision: String,
    @SerialName("expected_revision") val expectedRevision: Long,
    val declaration: String,
    val imports: List<String>,
)

@Serializable
private data class DraftRevisionRequest(
    @SerialName("project_revision") val projectRevision: String,
    @SerialName("expected_revision") val expectedRevision: Long,
)

@Serializable
private data class DraftCheckRequest(
    @SerialName("project_revision") val projectRevision: String,
    @SerialName("expected_revision") val expectedRevision: Long,
    @SerialName("expected_hash") val expectedHash: String,
    @SerialName("run_lint") val runLint: Boolean,
    @SerialName("run_tests") val runTests: Boolean,
)

@Serializable
private data class DraftApplyRequest(
    @SerialName("draft_id") val draftId: String,
    @SerialName("draft_revision") val draftRevision: Long,
    @SerialName("draft_hash") val draftHash: String,
    @SerialName("project_id") val projectId: String,
    @SerialName("project_revision") val projectRevision: String,
    @SerialName("base_file_hash") val baseFileHash: String,
    val confirm: Boolean,
)

@Serializable
private data class DraftUndoRequest(
    @SerialName("project_id") val projectId: String,
    @SerialName("project_revision") val projectRevision: String,
    @SerialName("post_apply_hash") val postApplyHash: String,
    val confirm: Boolean,
)

class ApiClient(
    baseUrl: String = System.getenv("MINI_ORCA_URL") ?: "http://localhost:9090",
    private val transport: DaemonTransport = HttpDaemonTransport(baseUrl.trimEnd('/')),
) {
  private val endpoint = URI.create(baseUrl)
  private val json = Json {
    ignoreUnknownKeys = true
    coerceInputValues = true
  }

  // Project and indexed-file routes.
  fun importProject(path: String, confirmRemoteProvider: Boolean = false): ProjectAnalysis =
      decode(
          send(
              "POST",
              "/api/projects/import",
              requestBody(ProjectImportRequest(path, confirmRemoteProvider))))

  fun restoreProject(path: String): ProjectAnalysis =
      decode(send("POST", "/api/projects/restore", requestBody(ProjectRestoreRequest(path))))

  fun index(): ProjectIndex = decode(send("GET", "/api/projects/current/index"))

  fun reindex(revision: String): ProjectIndex =
      decode(
          send(
              "POST",
              "/api/projects/current/reindex",
              requestBody(ProjectRevisionRequest(revision))))

  fun fileInfo(path: String): ProjectFileInfo =
      decode(send("GET", "/api/projects/current/files/info?path=${encode(path)}"))

  fun symbols(path: String): SymbolsResponse =
      decode(send("GET", "/api/projects/current/files/symbols?path=${encode(path)}"))

  fun analysis(path: String, revision: String): FileAnalysis =
      decode(
          send(
              "GET",
              "/api/projects/current/files/analysis?path=${encode(path)}&project_revision=${encode(revision)}"))

  fun analyze(
      path: String,
      revision: String,
      refresh: Boolean = false,
      confirmRemoteProvider: Boolean = false
  ): FileAnalysis =
      decode(
          send(
              "POST",
              "/api/projects/current/files/analysis",
              requestBody(FileAnalysisRequest(path, revision, refresh, confirmRemoteProvider))))

  fun context(path: String, action: String = "fix"): ContextManifest =
      decode(
          send(
              "GET", "/api/projects/current/context?path=${encode(path)}&action=${encode(action)}"))

  fun impact(path: String, symbol: String = ""): ImpactPreview =
      decode(
          send("GET", "/api/projects/current/impact?path=${encode(path)}&symbol=${encode(symbol)}"))

  fun gitStatus(path: String): GitStatus =
      decode(send("GET", "/api/projects/current/git?path=${encode(path)}"))

  // Daemon health and project workspace routes.
  fun modelCatalog(): ModelCatalog = decode(send("GET", "/api/models/current"))

  fun status(): DaemonStatus = decode(send("GET", "/status"))

  fun overview(revision: String): ProjectOverview =
      decode(send("GET", "/api/projects/current/overview?project_revision=${encode(revision)}"))

  fun findings(revision: String, filter: FindingFilter = FindingFilter()): FindingsResponse {
    val query =
        listOf(
                "project_revision" to revision,
                "source" to filter.source,
                "confidence" to filter.confidence,
                "severity" to filter.severity,
                "status" to filter.status,
                "freshness" to filter.freshness,
            )
            .filter { it.second.isNotBlank() }
            .joinToString("&") { "${it.first}=${encode(it.second)}" }
    return decode(send("GET", "/api/projects/current/findings?$query"))
  }

  fun updateFindingStatus(findingId: String, revision: String, status: String) {
    sendNoContent(
        "PATCH",
        "/api/projects/current/findings/${encodePath(findingId)}",
        requestBody(FindingStatusRequest(revision, status)))
  }

  // Project-wide analysis routes.
  fun goScan(revision: String): GoScanReport? =
      decodeOptional(
          sendResponse("GET", "/api/projects/current/scan?project_revision=${encode(revision)}"))

  fun startGoScan(revision: String): GoScanReport =
      decode(
          send("POST", "/api/projects/current/scan", requestBody(ProjectRevisionRequest(revision))))

  fun cancelGoScan(revision: String): GoScanReport =
      decode(send("DELETE", "/api/projects/current/scan?project_revision=${encode(revision)}"))

  fun analyzeAllJob(revision: String): AnalyzeAllJob? =
      decodeOptional(
          sendResponse(
              "GET", "/api/projects/current/analysis-job?project_revision=${encode(revision)}"))

  fun startAnalyzeAll(
      revision: String,
      maxFiles: Int = 0,
      maxRetries: Int = 0,
      confirmRemoteProvider: Boolean = false
  ): AnalyzeAllJob =
      decode(
          send(
              "POST",
              "/api/projects/current/analysis-job",
              requestBody(
                  AnalyzeAllStartRequest(revision, maxFiles, maxRetries, confirmRemoteProvider))))

  fun pauseAnalyzeAll(revision: String): AnalyzeAllJob =
      decode(
          send(
              "POST",
              "/api/projects/current/analysis-job/pause?project_revision=${encode(revision)}"))

  fun resumeAnalyzeAll(revision: String, confirmRemoteProvider: Boolean = false): AnalyzeAllJob =
      decode(
          send(
              "POST",
              "/api/projects/current/analysis-job/resume",
              requestBody(AnalyzeAllResumeRequest(revision, confirmRemoteProvider))))

  fun cancelAnalyzeAll(revision: String): AnalyzeAllJob =
      decode(
          send(
              "POST",
              "/api/projects/current/analysis-job/cancel?project_revision=${encode(revision)}"))

  // File-scoped chat routes.
  fun openChatSession(
      projectId: String,
      revision: String,
      baseFileHash: String,
      openPath: String,
      mode: String,
      targetSymbol: String,
      taskSpec: BugTaskSpec? = null
  ): ChatSession =
      decode(
          send(
              "POST",
              "/api/projects/current/chat/sessions",
              requestBody(
                  ChatSessionRequest(
                      projectId, revision, baseFileHash, openPath, mode, targetSymbol, taskSpec))))

  fun sendChatMessage(
      sessionId: String,
      message: String,
      parentDraftId: String = "",
      confirmRemoteProvider: Boolean = false,
      repair: Boolean = false
  ): ChatDraftProposal =
      decode(
          send(
              "POST",
              "/api/projects/current/chat/sessions/${encodePath(sessionId)}/messages",
              requestBody(
                  ChatMessageRequest(message, parentDraftId, confirmRemoteProvider, repair))))

  // Draft review and guarded write routes.
  fun updateDraft(
      draftId: String,
      projectRevision: String,
      expectedRevision: Long,
      declaration: String,
      imports: List<String>
  ): DeclarationDraft =
      decode(
          send(
              "PATCH",
              "/api/projects/current/drafts/${encodePath(draftId)}",
              requestBody(
                  DraftUpdateRequest(projectRevision, expectedRevision, declaration, imports))))

  fun validateDraft(
      draftId: String,
      projectRevision: String,
      expectedRevision: Long
  ): DeclarationDraft =
      decode(
          send(
              "POST",
              "/api/projects/current/drafts/${encodePath(draftId)}/validate",
              requestBody(DraftRevisionRequest(projectRevision, expectedRevision))))

  fun checkDraft(
      draftId: String,
      projectRevision: String,
      expectedRevision: Long,
      expectedHash: String,
      runLint: Boolean = false,
      runTests: Boolean = false
  ): DraftCheckReport =
      decode(
          send(
              "POST",
              "/api/projects/current/drafts/${encodePath(draftId)}/checks",
              requestBody(
                  DraftCheckRequest(
                      projectRevision, expectedRevision, expectedHash, runLint, runTests))))

  fun applyDraft(draft: DeclarationDraft): ApplyResult =
      decode(
          send(
              "POST",
              "/api/projects/current/apply",
              requestBody(
                  DraftApplyRequest(
                      draft.id,
                      draft.revision,
                      draft.hash,
                      draft.projectId,
                      draft.projectRevision,
                      draft.baseFileHash,
                      true))))

  fun endpointLocality(): String {
    val host = endpoint.host?.lowercase().orEmpty()
    return if (host == "localhost" || host == "127.0.0.1" || host == "::1") "Local endpoint"
    else "Remote endpoint"
  }

  fun undo(projectId: String, revision: String, postApplyHash: String): ApplyResult =
      decode(
          send(
              "POST",
              "/api/projects/current/undo",
              requestBody(DraftUndoRequest(projectId, revision, postApplyHash, true))))

  private inline fun <reified T> decode(body: String): T = json.decodeFromString(body)

  private inline fun <reified T> decodeOptional(response: TransportResponse): T? =
      if (response.status == 204 || response.body.isBlank()) null else decode(response.body)

  private inline fun <reified T> requestBody(request: T): String = json.encodeToString(request)

  private fun send(method: String, path: String, body: String? = null): String =
      sendResponse(method, path, body).body

  private fun sendNoContent(method: String, path: String, body: String? = null) {
    sendResponse(method, path, body)
  }

  private fun sendResponse(method: String, path: String, body: String? = null): TransportResponse {
    val response = transport.send(method, path, body)
    if (response.status !in 200..299) {
      val error = decodeApiError(response.body)
      throw ApiException(
          response.status,
          error,
          error?.userMessage?.ifBlank { error.message } ?: "Daemon returned ${response.status}")
    }
    return response
  }

  private fun decodeApiError(body: String): ApiError? =
      try {
        json.decodeFromString(body)
      } catch (_: Exception) {
        null
      }

  private fun encode(value: String) = URLEncoder.encode(value, StandardCharsets.UTF_8)

  private fun encodePath(value: String) = encode(value).replace("+", "%20")
}

class ApiException(val status: Int, val error: ApiError? = null, message: String) :
    IllegalStateException(message)

private class HttpDaemonTransport(private val root: String) : DaemonTransport {
  private val http = HttpClient.newBuilder().connectTimeout(Duration.ofSeconds(10)).build()

  override fun send(method: String, path: String, body: String?): TransportResponse {
    val builder =
        HttpRequest.newBuilder(URI.create(root + path))
            .timeout(Duration.ofMinutes(6))
            .header("Accept", "application/json")
    if (body == null && method == "GET") builder.GET()
    else if (body == null) builder.method(method, HttpRequest.BodyPublishers.noBody())
    else
        builder
            .header("Content-Type", "application/json")
            .method(method, HttpRequest.BodyPublishers.ofString(body))
    val response = http.send(builder.build(), HttpResponse.BodyHandlers.ofString())
    return TransportResponse(response.statusCode(), response.body())
  }
}
