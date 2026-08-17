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
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.put

data class TransportResponse(val status: Int, val body: String)
fun interface DaemonTransport { fun send(method: String, path: String, body: String?): TransportResponse }

class ApiClient(
    baseUrl: String = System.getenv("MINI_ORCA_URL") ?: "http://localhost:9090",
    private val transport: DaemonTransport = HttpDaemonTransport(baseUrl.trimEnd('/')),
) {
    private val json = Json { ignoreUnknownKeys = true }

    fun importProject(path: String): ProjectAnalysis = decode(send("POST", "/api/projects/import", jsonBody("project_path" to path)))
    fun index(): ProjectIndex = decode(send("GET", "/api/projects/current/index"))
    fun fileInfo(path: String): ProjectFileInfo = decode(send("GET", "/api/projects/current/files/info?path=${encode(path)}"))
    fun symbols(path: String): SymbolsResponse = decode(send("GET", "/api/projects/current/files/symbols?path=${encode(path)}"))
    fun analysis(path: String, revision: String): FileAnalysis = decode(send("GET", "/api/projects/current/files/analysis?path=${encode(path)}&project_revision=${encode(revision)}"))
    fun analyze(path: String, revision: String, refresh: Boolean = false): FileAnalysis = decode(send("POST", "/api/projects/current/files/analysis", jsonBody("path" to path, "project_revision" to revision, "refresh" to refresh)))
    fun context(path: String, action: String = "fix"): ContextManifest = decode(send("GET", "/api/projects/current/context?path=${encode(path)}&action=${encode(action)}"))
    fun effectiveModel(): EffectiveModel = decode(send("GET", "/api/models/current"))
    fun audit(revision: String): List<AuditEntry> = decode(send("GET", "/api/projects/current/audit?project_revision=${encode(revision)}"))

    fun generate(message: String, filePath: String, targetSymbol: String, projectId: String, projectRevision: String, baseFileHash: String, scopeMode: String = "strict_symbol"): GenerationResult = decode(send("POST", "/api/chat/message", jsonBody("message" to message, "file_path" to filePath, "target_symbol" to targetSymbol, "project_id" to projectId, "project_revision" to projectRevision, "base_file_hash" to baseFileHash, "scope_mode" to scopeMode)))
    fun checks(generationId: String, revision: String, runLint: Boolean = false, runTests: Boolean = false): CandidateCheckReport = decode(send("POST", "/api/projects/current/candidates/checks", jsonBody("generation_id" to generationId, "project_revision" to revision, "run_lint" to runLint, "run_tests" to runTests)))
    fun apply(generationId: String, projectId: String, revision: String, baseHash: String): ApplyResult = decode(send("POST", "/api/projects/current/apply", jsonBody("generation_id" to generationId, "project_id" to projectId, "project_revision" to revision, "base_file_hash" to baseHash, "confirm" to true)))
    fun undo(projectId: String, revision: String, postApplyHash: String): ApplyResult = decode(send("POST", "/api/projects/current/undo", jsonBody("project_id" to projectId, "project_revision" to revision, "post_apply_hash" to postApplyHash, "confirm" to true)))

    private inline fun <reified T> decode(body: String): T = json.decodeFromString(body)
    private fun send(method: String, path: String, body: String? = null): String {
        val response = transport.send(method, path, body)
        if (response.status !in 200..299) {
            val error = runCatching { json.decodeFromString<ApiError>(response.body) }.getOrNull()
            throw ApiException(response.status, error?.userMessage?.ifBlank { error.message } ?: "Daemon returned ${response.status}")
        }
        return response.body
    }
    private fun encode(value: String) = URLEncoder.encode(value, StandardCharsets.UTF_8)
    private fun jsonBody(vararg values: Pair<String, Any>): String = buildJsonObject { values.forEach { (key, value) -> when (value) { is String -> put(key, value); is Boolean -> put(key, value); else -> error("unsupported JSON value") } } }.toString()
}

class ApiException(val status: Int, message: String) : IllegalStateException(message)

private class HttpDaemonTransport(private val root: String) : DaemonTransport {
    private val http = HttpClient.newBuilder().connectTimeout(Duration.ofSeconds(10)).build()
    override fun send(method: String, path: String, body: String?): TransportResponse {
        val builder = HttpRequest.newBuilder(URI.create(root + path)).timeout(Duration.ofMinutes(6)).header("Accept", "application/json")
        if (body == null) builder.GET() else builder.header("Content-Type", "application/json").method(method, HttpRequest.BodyPublishers.ofString(body))
        val response = http.send(builder.build(), HttpResponse.BodyHandlers.ofString())
        return TransportResponse(response.statusCode(), response.body())
    }
}
