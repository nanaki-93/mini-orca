package io.miniorca.desktop

import java.net.URI
import java.net.URLEncoder
import java.net.http.HttpClient
import java.net.http.HttpRequest
import java.net.http.HttpResponse
import java.nio.charset.StandardCharsets
import java.time.Duration
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.put

class ApiClient(
    baseUrl: String = System.getenv("MINI_ORCA_URL") ?: "http://localhost:8080",
) {
    private val root = baseUrl.trimEnd('/')
    private val http = HttpClient.newBuilder().connectTimeout(Duration.ofSeconds(10)).build()
    private val json = Json { ignoreUnknownKeys = true }

    fun importProject(path: String): ProjectAnalysis {
        val body = buildJsonObject { put("project_path", path) }.toString()
        return json.decodeFromString(send("/api/projects/import", "POST", body))
    }

    fun fileInfo(path: String): ProjectFileInfo {
        val encoded = URLEncoder.encode(path, StandardCharsets.UTF_8)
        return json.decodeFromString(send("/api/projects/current/files/info?path=$encoded"))
    }

    fun generate(message: String, filePath: String, targetSymbol: String): GenerationResult {
        val body = buildJsonObject {
            put("message", message)
            put("file_path", filePath)
            put("target_symbol", targetSymbol)
        }.toString()
        return json.decodeFromString(send("/api/chat/message", "POST", body))
    }

    private fun send(path: String, method: String = "GET", body: String? = null): String {
        val builder = HttpRequest.newBuilder(URI.create(root + path))
            .timeout(Duration.ofMinutes(6))
            .header("Accept", "application/json")
        if (body == null) {
            builder.GET()
        } else {
            builder.header("Content-Type", "application/json")
                .method(method, HttpRequest.BodyPublishers.ofString(body))
        }
        val response = http.send(builder.build(), HttpResponse.BodyHandlers.ofString())
        if (response.statusCode() !in 200..299) {
            throw IllegalStateException("Daemon returned ${response.statusCode()}: ${response.body()}")
        }
        return response.body()
    }
}
