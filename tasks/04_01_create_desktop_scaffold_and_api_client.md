# Task 4.1 — Create the Gradle/Compose scaffold and API client

**Phase:** 4 — Kotlin Compose Desktop client  
**Status:** Complete  
**Priority:** High  
**Risk:** Medium

## Goal

Create a minimal, independently buildable Kotlin Compose Desktop module that communicates with the Go daemon.

## Plan coverage

- Add a minimal Gradle/Compose Desktop application.
- Keep one backend and one simple desktop API client.

## Dependencies

- Task 2.5
- Task 3.1

## Files

- `desktop/settings.gradle.kts`
- `desktop/build.gradle.kts`
- `desktop/src/main/kotlin/io/miniorca/desktop/Models.kt`
- `desktop/src/main/kotlin/io/miniorca/desktop/ApiClient.kt`
- `.gitignore`

## Implementation steps

1. Configure Kotlin JVM, serialization, Compose, and the Compose compiler plugin.
2. Target Java 21 and declare only Compose Desktop, coroutines Swing, and JSON serialization dependencies.
3. Define serializable DTOs matching project analysis, file info, and generation responses.
4. Implement one `java.net.http.HttpClient` wrapper for import, file info, and generation.
5. Use `MINI_ORCA_URL` with `http://localhost:8080` as a simple default.
6. Apply connection and request deadlines and surface non-2xx responses as errors.
7. Ignore Gradle, Kotlin, and desktop build outputs.

## Acceptance criteria

- The desktop module resolves and compiles independently.
- DTO field names correctly map snake_case API JSON.
- All three required backend operations are represented.
- No Ktor, database, dependency injection framework, or extra service layer is introduced.

## Verification

```bash
gradle -p desktop test
```
