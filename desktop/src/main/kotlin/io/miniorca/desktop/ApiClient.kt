package io.miniorca.desktop

import java.net.URI
import java.net.URLEncoder
import java.net.http.HttpClient
import java.net.http.HttpRequest
import java.net.http.HttpResponse
import java.nio.charset.StandardCharsets
import java.time.Duration
import kotlinx.serialization.decodeFromString
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonNull
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.encodeToJsonElement
import kotlinx.serialization.json.put

data class TransportResponse(val status: Int, val body: String)
fun interface DaemonTransport { fun send(method: String, path: String, body: String?): TransportResponse }

class ApiClient(
    baseUrl: String = System.getenv("MINI_ORCA_URL") ?: "http://localhost:9090",
    private val transport: DaemonTransport = HttpDaemonTransport(baseUrl.trimEnd('/')),
) {
    private val endpoint = URI.create(baseUrl)
    private val json = Json {
        ignoreUnknownKeys = true
        coerceInputValues = true
    }

    fun importProject(path: String, confirmRemoteProvider: Boolean = false): ProjectAnalysis = decode(send("POST", "/api/projects/import", jsonBody("project_path" to path, "confirm_remote_provider" to confirmRemoteProvider)))
    fun restoreProject(path: String): ProjectAnalysis = decode(send("POST", "/api/projects/restore", jsonBody("project_path" to path)))
    fun index(): ProjectIndex = decode(send("GET", "/api/projects/current/index"))
    fun reindex(revision: String): ProjectIndex = decode(send("POST", "/api/projects/current/reindex", jsonBody("project_revision" to revision)))
    fun fileInfo(path: String): ProjectFileInfo = decode(send("GET", "/api/projects/current/files/info?path=${encode(path)}"))
    fun symbols(path: String): SymbolsResponse = decode(send("GET", "/api/projects/current/files/symbols?path=${encode(path)}"))
    fun analysis(path: String, revision: String): FileAnalysis = decode(send("GET", "/api/projects/current/files/analysis?path=${encode(path)}&project_revision=${encode(revision)}"))
    fun analyze(path: String, revision: String, refresh: Boolean = false, confirmRemoteProvider: Boolean = false): FileAnalysis = decode(send("POST", "/api/projects/current/files/analysis", jsonBody("path" to path, "project_revision" to revision, "refresh" to refresh, "confirm_remote_provider" to confirmRemoteProvider)))
    fun context(path: String, action: String = "fix"): ContextManifest = decode(send("GET", "/api/projects/current/context?path=${encode(path)}&action=${encode(action)}"))
    fun modelCatalog(): ModelCatalog = decode(send("GET", "/api/models/current"))
    fun status(): DaemonStatus = decode(send("GET", "/status"))
    fun impact(path: String, symbol: String = ""): ImpactPreview = decode(send("GET", "/api/projects/current/impact?path=${encode(path)}&symbol=${encode(symbol)}"))
    fun gitStatus(path: String): GitStatus = decode(send("GET", "/api/projects/current/git?path=${encode(path)}"))

    fun overview(revision: String): ProjectOverview = decode(send("GET", "/api/projects/current/overview?project_revision=${encode(revision)}"))

    fun findings(revision: String, filter: FindingFilter = FindingFilter()): FindingsResponse {
        val query = listOf(
            "project_revision" to revision,
            "source" to filter.source,
            "confidence" to filter.confidence,
            "severity" to filter.severity,
            "status" to filter.status,
            "freshness" to filter.freshness,
        ).filter { it.second.isNotBlank() }.joinToString("&") { "${it.first}=${encode(it.second)}" }
        return decode(send("GET", "/api/projects/current/findings?$query"))
    }

    fun updateFindingStatus(findingId: String, revision: String, status: String) {
        sendNoContent("PATCH", "/api/projects/current/findings/${encodePath(findingId)}", jsonBody("project_revision" to revision, "status" to status))
    }

    fun goScan(revision: String): GoScanReport? = decodeOptional(sendResponse("GET", "/api/projects/current/scan?project_revision=${encode(revision)}"))
    fun startGoScan(revision: String): GoScanReport = decode(send("POST", "/api/projects/current/scan", jsonBody("project_revision" to revision)))
    fun cancelGoScan(revision: String): GoScanReport = decode(send("DELETE", "/api/projects/current/scan?project_revision=${encode(revision)}"))

    fun analyzeAllJob(revision: String): AnalyzeAllJob? = decodeOptional(sendResponse("GET", "/api/projects/current/analysis-job?project_revision=${encode(revision)}"))
    fun startAnalyzeAll(revision: String, maxFiles: Int = 0, maxRetries: Int = 0, confirmRemoteProvider: Boolean = false): AnalyzeAllJob = decode(send("POST", "/api/projects/current/analysis-job", jsonBody("project_revision" to revision, "max_files" to maxFiles, "max_retries" to maxRetries, "confirm_remote_provider" to confirmRemoteProvider)))
    fun pauseAnalyzeAll(revision: String): AnalyzeAllJob = decode(send("POST", "/api/projects/current/analysis-job/pause?project_revision=${encode(revision)}"))
    fun resumeAnalyzeAll(revision: String, confirmRemoteProvider: Boolean = false): AnalyzeAllJob = decode(send("POST", "/api/projects/current/analysis-job/resume", jsonBody("project_revision" to revision, "confirm_remote_provider" to confirmRemoteProvider)))
    fun cancelAnalyzeAll(revision: String): AnalyzeAllJob = decode(send("POST", "/api/projects/current/analysis-job/cancel?project_revision=${encode(revision)}"))

    fun openChatSession(projectId: String, revision: String, baseFileHash: String, openPath: String, mode: String, targetSymbol: String, taskSpec: BugTaskSpec? = null): ChatSession = decode(send("POST", "/api/projects/current/chat/sessions", jsonBody("project_id" to projectId, "project_revision" to revision, "base_file_hash" to baseFileHash, "open_path" to openPath, "mode" to mode, "target_symbol" to targetSymbol, "task_spec" to taskSpec)))
    fun sendChatMessage(sessionId: String, message: String, parentDraftId: String = "", confirmRemoteProvider: Boolean = false, repair: Boolean = false): ChatDraftProposal = decode(send("POST", "/api/projects/current/chat/sessions/${encodePath(sessionId)}/messages", jsonBody("message" to message, "parent_draft_id" to parentDraftId, "confirm_remote_provider" to confirmRemoteProvider, "repair" to repair)))

    fun updateDraft(draftId: String, projectRevision: String, expectedRevision: Long, declaration: String, imports: List<String>): DeclarationDraft = decode(send("PATCH", "/api/projects/current/drafts/${encodePath(draftId)}", jsonBody("project_revision" to projectRevision, "expected_revision" to expectedRevision, "declaration" to declaration, "imports" to imports)))
    fun validateDraft(draftId: String, projectRevision: String, expectedRevision: Long): DeclarationDraft = decode(send("POST", "/api/projects/current/drafts/${encodePath(draftId)}/validate", jsonBody("project_revision" to projectRevision, "expected_revision" to expectedRevision)))
    fun checkDraft(draftId: String, projectRevision: String, expectedRevision: Long, expectedHash: String, runLint: Boolean = false, runTests: Boolean = false): DraftCheckReport = decode(send("POST", "/api/projects/current/drafts/${encodePath(draftId)}/checks", jsonBody("project_revision" to projectRevision, "expected_revision" to expectedRevision, "expected_hash" to expectedHash, "run_lint" to runLint, "run_tests" to runTests)))
    fun applyDraft(draft: DeclarationDraft): ApplyResult = decode(send("POST", "/api/projects/current/apply", jsonBody("draft_id" to draft.id, "draft_revision" to draft.revision, "draft_hash" to draft.hash, "project_id" to draft.projectId, "project_revision" to draft.projectRevision, "base_file_hash" to draft.baseFileHash, "confirm" to true)))

    fun endpointLocality(): String {
        val host = endpoint.host?.lowercase().orEmpty()
        return if (host == "localhost" || host == "127.0.0.1" || host == "::1") "Local endpoint" else "Remote endpoint"
    }

    fun undo(projectId: String, revision: String, postApplyHash: String): ApplyResult = decode(send("POST", "/api/projects/current/undo", jsonBody("project_id" to projectId, "project_revision" to revision, "post_apply_hash" to postApplyHash, "confirm" to true)))

    private inline fun <reified T> decode(body: String): T = json.decodeFromString(body)
    private inline fun <reified T> decodeOptional(response: TransportResponse): T? = if (response.status == 204 || response.body.isBlank()) null else decode(response.body)
    private fun send(method: String, path: String, body: String? = null): String = sendResponse(method, path, body).body
    private fun sendNoContent(method: String, path: String, body: String? = null) {
        sendResponse(method, path, body)
    }
    private fun sendResponse(method: String, path: String, body: String? = null): TransportResponse {
        val response = transport.send(method, path, body)
        if (response.status !in 200..299) {
            val error = runCatching { json.decodeFromString<ApiError>(response.body) }.getOrNull()
            throw ApiException(response.status, error, error?.userMessage?.ifBlank { error.message } ?: "Daemon returned ${response.status}")
        }
        return response
    }
    private fun encode(value: String) = URLEncoder.encode(value, StandardCharsets.UTF_8)
    private fun encodePath(value: String) = encode(value).replace("+", "%20")
    private fun jsonBody(vararg values: Pair<String, Any?>): String = buildJsonObject { values.forEach { (key, value) -> when (value) { is String -> put(key, value); is Boolean -> put(key, value); is Int -> put(key, value); is Long -> put(key, value); is BugTaskSpec -> put(key, json.encodeToJsonElement(value)); is List<*> -> put(key, kotlinx.serialization.json.JsonArray(value.map { kotlinx.serialization.json.JsonPrimitive(it as? String ?: error("unsupported JSON list value")) })); null -> put(key, JsonNull); else -> error("unsupported JSON value") } } }.toString()
}

class ApiException(val status: Int, val error: ApiError? = null, message: String) : IllegalStateException(message)

private class HttpDaemonTransport(private val root: String) : DaemonTransport {
    private val http = HttpClient.newBuilder().connectTimeout(Duration.ofSeconds(10)).build()
    override fun send(method: String, path: String, body: String?): TransportResponse {
        val builder = HttpRequest.newBuilder(URI.create(root + path)).timeout(Duration.ofMinutes(6)).header("Accept", "application/json")
        if (body == null && method == "GET") builder.GET() else if (body == null) builder.method(method, HttpRequest.BodyPublishers.noBody()) else builder.header("Content-Type", "application/json").method(method, HttpRequest.BodyPublishers.ofString(body))
        val response = http.send(builder.build(), HttpResponse.BodyHandlers.ofString())
        return TransportResponse(response.statusCode(), response.body())
    }
}
