# Mini-Orca — Future: Kotlin Native App

## Overview

This document outlines the plan to create a **native Kotlin desktop application** for Mini-Orca. This would be a separate client that connects to the Go daemon via REST APIs.

**Note:** No authentication is needed — this is a local-only environment.

---

## 1. Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────┐
│                      KOTLIN NATIVE APP                               │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │                   Jetpack Compose UI                          │  │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────────┐  │  │
│  │  │Dashboard │  │ CodeView │  │ TestView │  │ ConfigView   │  │  │
│  │  └──────────┘  └──────────┘  └──────────┘  └──────────────┘  │  │
│  └───────────────────────────┬───────────────────────────────────┘  │
│                              │                                      │
│  ┌───────────────────────────▼───────────────────────────────────┐  │
│  │                   Presentation Layer                          │  │
│  │  - ViewModels (StateFlow)                                     │  │
│  │  - UI State management                                        │  │
│  └───────────────────────────┬───────────────────────────────────┘  │
│                              │                                      │
│  ┌───────────────────────────▼───────────────────────────────────┐  │
│  │                   Domain Layer                                │  │
│  │  - UseCases                                                   │  │
│  │  - Repository interfaces                                      │  │
│  └───────────────────────────┬───────────────────────────────────┘  │
│                              │                                      │
│  ┌───────────────────────────▼───────────────────────────────────┐  │
│  │                   Data Layer                                  │  │
│  │  - API Client (Ktor)                                          │  │
│  │  - Local storage (DataStore)                                  │  │
│  └───────────────────────────────────────────────────────────────┘  │
└───────────────────────────┬─────────────────────────────────────────┘
                            │ REST API (no auth)
┌───────────────────────────▼─────────────────────────────────────────┐
│                     MINI-ORCA DAEMON (Go)                            │
│                     (unauthenticated, local-only)                    │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 2. Technology Stack

### Desktop (Kotlin/JVM)

| Technology | Purpose |
|------------|---------|
| **Kotlin 1.9+** | Language |
| **Jetpack Compose for Desktop** | UI Framework |
| **Ktor Client** | HTTP client |
| **Koin** | Dependency Injection |
| **Kotlinx Serialization** | JSON parsing |
| **DataStore** | Local preferences storage |

---

## 3. Project Structure

```
mini-orca-client/
├── desktop/                    # Desktop app (JVM)
│   ├── src/main/kotlin/
│   │   └── com/orca/client/
│   │       ├── Main.kt         # Entry point
│   │       ├── app/
│   │       │   ├── OrcaApp.kt  # Root Compose component
│   │       │   └── theme/
│   │       │       ├── Theme.kt
│   │       │       └── Colors.kt
│   │       ├── ui/
│   │       │   ├── dashboard/
│   │       │   │   ├── DashboardScreen.kt
│   │       │   │   └── DashboardViewModel.kt
│   │       │   ├── session/
│   │       │   │   ├── SessionScreen.kt
│   │       │   │   └── SessionViewModel.kt
│   │       │   ├── code/
│   │       │   │   ├── CodeScreen.kt
│   │       │   │   └── CodeViewModel.kt
│   │       │   ├── tests/
│   │       │   │   ├── TestsScreen.kt
│   │       │   │   └── TestsViewModel.kt
│   │       │   └── settings/
│   │       │       ├── SettingsScreen.kt
│   │       │       └── SettingsViewModel.kt
│   │       ├── data/
│   │       │   ├── api/
│   │       │   │   ├── OrcaApi.kt
│   │       │   │   ├── SessionApi.kt
│   │       │   │   └── ConfigApi.kt
│   │       │   ├── repository/
│   │       │   │   ├── SessionRepository.kt
│   │       │   │   └── ConfigRepository.kt
│   │       │   └── local/
│   │       │       └── PreferencesStore.kt
│   │       ├── domain/
│   │       │   ├── model/
│   │       │   │   ├── Session.kt
│   │       │   │   ├── Plan.kt
│   │       │   │   ├── AtomicUnit.kt
│   │       │   │   ├── CodeOutput.kt
│   │       │   │   ├── TestReport.kt
│   │       │   │   └── ReviewReport.kt
│   │       │   └── usecase/
│   │       │       ├── CreateSessionUseCase.kt
│   │       │       ├── ApprovePlanUseCase.kt
│   │       │       └── GetModelConfigUseCase.kt
│   │       └── di/
│   │           └── Modules.kt   # Koin modules
│   └── build.gradle.kts
├── build.gradle.kts
└── settings.gradle.kts
```

---

## 4. Key Components

### 4.1 API Client (Ktor)

```kotlin
// desktop/src/main/kotlin/com/orca/client/data/api/OrcaApi.kt

import io.ktor.client.*
import io.ktor.client.engine.cio.*
import io.ktor.client.plugins.contentnegotiation.*
import io.ktor.serialization.kotlinx.json.*
import kotlinx.serialization.json.Json

class OrcaApi(private val baseUrl: String = "http://localhost:8080") {
    private val client = HttpClient(CIO) {
        install(ContentNegotiation) {
            json(Json {
                ignoreUnknownKeys = true
                prettyPrint = true
            })
        }
    }

    // No authentication needed - local environment
    
    suspend fun createSession(request: CreateSessionRequest): Session {
        return client.post("$baseUrl/api/sessions") {
            setBody(request)
        }
    }

    suspend fun getSession(sessionId: String): Session {
        return client.get("$baseUrl/api/sessions/$sessionId")
    }

    suspend fun listSessions(): List<Session> {
        return client.get("$baseUrl/api/sessions")
    }

    suspend fun approvePlan(sessionId: String, approved: Boolean, feedback: String = "") {
        client.post("$baseUrl/api/sessions/$sessionId/approve") {
            setBody(ApproveRequest(approved, feedback))
        }
    }

    suspend fun getModelConfig(): ModelConfig {
        return client.get("$baseUrl/api/config/models")
    }
}
```

### 4.2 Dashboard Screen (Compose)

```kotlin
// desktop/src/main/kotlin/com/orca/client/ui/dashboard/DashboardScreen.kt

import androidx.compose.foundation.layout.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import com.orca.client.domain.model.Session
import com.orca.client.ui.session.SessionViewModel

@Composable
fun DashboardScreen(
    viewModel: SessionViewModel = koinViewModel(),
    onSessionClick: (String) -> Unit
) {
    val sessions by viewModel.sessions.collectAsState()
    val isLoading by viewModel.isLoading.collectAsState()

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("🐋 Mini-Orca") }
            )
        }
    ) { padding ->
        if (isLoading) {
            Box(
                modifier = Modifier.fillMaxSize(),
                contentAlignment = Alignment.Center
            ) {
                CircularProgressIndicator()
            }
        } else {
            LazyColumn(
                modifier = Modifier
                    .fillMaxSize()
                    .padding(padding)
                    .padding(16.dp),
                verticalArrangement = Arrangement.spacedBy(8.dp)
            ) {
                items(sessions) { session ->
                    SessionCard(
                        session = session,
                        onClick = { onSessionClick(session.id) }
                    )
                }
            }
        }
    }
}
```

---

## 5. API Contract (for Desktop App)

### 5.1 Sessions

```kotlin
// Shared data models

@Serializable
data class Session(
    val id: String,
    val project_path: String,
    val project_goal: String,
    val current_phase: String,
    val created_at: String,
    val updated_at: String,
    val plan: Plan? = null,
    val current_unit: AtomicUnit? = null
)

@Serializable
data class Plan(
    val id: String,
    val atomic_units: List<AtomicUnit>,
    val architecture: String
)

@Serializable
data class AtomicUnit(
    val id: String,
    val type: String,        // "function" | "struct" | "class"
    val name: String,
    val target_file: String,
    val description: String,
    val status: String
)

@Serializable
data class CreateSessionRequest(
    val project_path: String,
    val project_goal: String
)

@Serializable
data class ApproveRequest(
    val approved: Boolean,
    val feedback: String = ""
)
```

### 5.2 Model Config

```kotlin
@Serializable
data class ModelConfig(
    val provider: String,
    val model: String,
    val temperature: Double,
    val max_tokens: Int
)

@Serializable
data class PhaseConfigs(
    val planning: ModelConfig,
    val coding: ModelConfig,
    val testing: ModelConfig,
    val review: ModelConfig
)
```

---

## 6. Build Configuration

```kotlin
// desktop/build.gradle.kts

plugins {
    kotlin("jvm") version "1.9.22"
    id("org.jetbrains.compose") version "1.5.11"
    id("org.jetbrains.kotlin.plugin.serialization") version "1.9.22"
}

dependencies {
    // Compose Desktop
    implementation(compose.runtime)
    implementation(compose.foundation)
    implementation(compose.material3)
    implementation(compose.ui)
    
    // Ktor
    implementation("io.ktor:ktor-client-core:2.3.7")
    implementation("io.ktor:ktor-client-cio:2.3.7")
    implementation("io.ktor:ktor-client-content-negotiation:2.3.7")
    implementation("io.ktor:ktor-serialization-kotlinx-json:2.3.7")
    
    // Koin
    implementation("io.insert-koin:koin-core:3.5.3")
    
    // Serialization
    implementation("org.jetbrains.kotlinx:kotlinx-serialization-json:1.6.2")
}

compose.desktop {
    application {
        mainClass = "com.orca.client.MainKt"
        
        nativeDistributions {
            targetFormats(TargetFormat.Dmg, Msi, Deb)
            packageName = "mini-orca"
            packageVersion = "1.0.0"
        }
    }
}
```

---

## 7. Development Roadmap

### Phase 1: Desktop MVP (6-8 weeks)
- [ ] Basic project setup
- [ ] Dashboard with session list
- [ ] Session detail view
- [ ] Phase tracking
- [ ] Code review screen
- [ ] Model configuration
- [ ] Packaging (DMG, MSI, DEB)

### Phase 2: Desktop Polish (2-4 weeks)
- [ ] Better code editor (integrated)
- [ ] Dark/light theme
- [ ] System tray integration
- [ ] Auto-update support

---

## 8. Comparison: HTMX Web vs Kotlin Native

| Feature | HTMX Web Dashboard | Kotlin Desktop |
|---------|-------------------|----------------|
| **Setup** | Browser only | Install required |
| **Performance** | Good | Better (native) |
| **File Access** | Limited | Full filesystem |
| **System Integration** | Limited | Better (tray, notifications) |
| **Development Speed** | **Faster** | Slower |
| **Cross-Platform** | Any browser | JVM-based |
| **Bundle Size** | Small | Larger (~100MB+) |
| **Maintenance** | Simple | More complex |

---

## 9. Recommendation

**Start with the HTMX Web Dashboard first** (Milestones 1-5 in the main plan).

Build the Kotlin desktop app **only if**:
1. The web dashboard meets all needs → stop there
2. System integration is needed (tray, notifications, file access) → build Kotlin Desktop

The Go daemon's API should be designed to support both clients from the start.
