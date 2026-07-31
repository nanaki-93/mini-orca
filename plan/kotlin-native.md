# Kotlin Native App Plan

**Created**: 2024-07-31  
**Version**: 1.0.0  
**Target Platform**: Kotlin Multiplatform (iOS + Android)  
**Backend**: Mini-Orca v2 API (REST + HTMX-compatible)

---

## Table of Contents

1. [Architecture Overview](#1-architecture-overview)
2. [UI Component Mapping](#2-ui-component-mapping)
3. [State Management Approach](#3-state-management-approach)
4. [Networking Layer](#4-networking-layer)
5. [Testing Strategy](#5-testing-strategy)
6. [Estimated Timeline](#6-estimated-timeline)

---

## 1. Architecture Overview

### 1.1 High-Level Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      Kotlin Multiplatform                   │
├──────────────┬──────────────────────────────────────────────┤
│   iOS        │              Android                         │
│  (UIKit)     │           (Jetpack Compose)                  │
├──────────────┼──────────────────────────────────────────────┤
│              │    Shared Module (kotlin-multiplatform)      │
│              │  ┌────────────────────────────────────┐      │
│              │  │         Domain Layer               │      │
│              │  │  - Entities, UseCases, Interfaces  │      │
│              │  └────────────────────────────────────┘      │
│              │  ┌────────────────────────────────────┐      │
│              │  │         Data Layer                 │      │
│              │  │  - Repositories, DTOs, Mappers     │      │
│              │  └────────────────────────────────────┘      │
│              │  ┌────────────────────────────────────┐      │
│              │  │         Network Layer              │      │
│              │  │  - Ktor Client, Serialization      │      │
│              │  └────────────────────────────────────┘      │
│              │  ┌────────────────────────────────────┐      │
│              │  │         State Management           │      │
│              │  │  - Kotlin Flow + ViewModel         │      │
│              │  └────────────────────────────────────┘      │
└──────────────┴──────────────────────────────────────────────┘
```

### 1.2 Project Structure

```
kotlin-native-app/
├── build.gradle.kts                    # Root build configuration
├── settings.gradle.kts                 # Module settings
├── gradle.properties                   # Gradle properties
├── shared/                             # Shared module (common code)
│   ├── build.gradle.kts
│   └── src/
│       ├── commonMain/                 # Shared code
│       │   ├── kotlin/
│       │   │   ├── domain/
│       │   │   │   ├── model/          # Entities
│       │   │   │   │   ├── Session.kt
│       │   │   │   │   ├── Project.kt
│       │   │   │   │   ├── FileEntry.kt
│       │   │   │   │   ├── Gate.kt
│       │   │   │   │   └── Agent.kt
│       │   │   │   ├── repository/     # Repository interfaces
│       │   │   │   │   ├── SessionRepository.kt
│       │   │   │   │   ├── ProjectRepository.kt
│       │   │   │   │   └── FileRepository.kt
│       │   │   │   └── usecase/        # Business logic
│       │   │   │       ├── CreateSessionUseCase.kt
│       │   │   │       ├── StartSessionUseCase.kt
│       │   │   │       └── ListProjectsUseCase.kt
│       │   │   ├── data/
│       │   │   │   ├── repository/     # Repository implementations
│       │   │   │   ├── dto/            # Data Transfer Objects
│       │   │   │   │   ├── SessionDto.kt
│       │   │   │   │   ├── ProjectDto.kt
│       │   │   │   │   └── FileDto.kt
│       │   │   │   ├── mapper/         # DTO ↔ Entity mappers
│       │   │   │   └── local/          # Local storage (if any)
│       │   │   ├── network/
│       │   │   │   ├── api/            # API service definitions
│       │   │   │   │   ├── SessionApi.kt
│       │   │   │   │   ├── ProjectApi.kt
│       │   │   │   │   └── GateApi.kt
│       │   │   │   ├── client/         # HTTP client setup
│       │   │   │   │   └── ApiClient.kt
│       │   │   │   └── interceptor/    # Request/response interceptors
│       │   │   │       └── LoggingInterceptor.kt
│       │   │   ├── state/
│       │   │   │   ├── SessionState.kt
│       │   │   │   ├── ProjectState.kt
│       │   │   │   └── UiState.kt
│       │   │   └── util/               # Utilities
│       │   │       ├── Constants.kt
│       │   │       └── Extensions.kt
│       ├── androidMain/                # Android-specific code
│       │   └── kotlin/
│       │       └── platform/
│       │           └── NetworkPlatform.kt
│       └── iosMain/                    # iOS-specific code
│           └── kotlin/
│               └── platform/
│                   └── NetworkPlatform.kt
├── android/                            # Android app module
│   └── src/main/kotlin/
│       └── com/orca/app/
│           ├── MainActivity.kt
│           └── di/                     # Dependency injection
└── iosApp/                             # iOS app module
    └── iOSApp/
        └── iOSApp.swift
```

### 1.3 Dependency Injection

- **Koin** for dependency injection (multiplatform-compatible)
- Module-based DI with separate modules for domain, data, and network layers

```kotlin
// shared/src/commonMain/kotlin/di/AppModule.kt
val appModule = module {
    // Network
    single { ApiClient(get()) }
    single { SessionApi(get()) }
    single { ProjectApi(get()) }
    
    // Repository
    single<SessionRepository> { SessionRepositoryImpl(get()) }
    single<ProjectRepository> { ProjectRepositoryImpl(get()) }
    
    // UseCases
    single { CreateSessionUseCase(get()) }
    single { StartSessionUseCase(get()) }
}
```

---

## 2. UI Component Mapping

### 2.1 HTMX-to-Kotlin Mapping

The Mini-Orca v2 API serves an HTMX-based web IDE. The Kotlin app will replicate this UI natively:

| Mini-Orca Web (HTMX) | Android (Jetpack Compose) | iOS (SwiftUI/UIKit) |
|---------------------|---------------------------|---------------------|
| Main Page (`/`) | `MainActivity` + `MainScreen` | `iOSApp.swift` + `MainView` |
| Dashboard (`/api/render/dashboard`) | `DashboardScreen` | `DashboardView` |
| Phase Tracker (`/api/render/phase-tracker`) | `PhaseTrackerScreen` | `PhaseTrackerView` |
| Activity Log (`/api/render/activity-log`) | `ActivityLogScreen` | `ActivityLogView` |
| File Tree (`/api/render/file-tree`) | `FileTreeScreen` | `FileTreeView` |
| Phase View (`/api/render/phase/{name}`) | `PhaseScreen` | `PhaseView` |
| Session List | `SessionListScreen` | `SessionListView` |
| Session Detail | `SessionDetailScreen` | `SessionDetailView` |
| Project List | `ProjectListScreen` | `ProjectListView` |
| Project Files | `ProjectFilesScreen` | `ProjectFilesView` |
| File Editor | `FileEditorScreen` | `FileEditorView` |
| Gate Approval | `GateApprovalScreen` | `GateApprovalView` |

### 2.2 Android UI (Jetpack Compose)

```kotlin
// android/src/main/kotlin/com/orca/app/ui/MainScreen.kt
@Composable
fun MainScreen(
    viewModel: MainViewModel = hiltViewModel(),
    navController: NavHostController = rememberNavController()
) {
    MainTheme {
        NavHost(navController, startDestination = "dashboard") {
            composable("dashboard") { DashboardScreen(viewModel) }
            composable("sessions") { SessionListScreen(viewModel) }
            composable("session/{id}") { backStackEntry ->
                val sessionId = backStackEntry.arguments?.getString("id")!!
                SessionDetailScreen(viewModel, sessionId)
            }
            composable("projects") { ProjectListScreen(viewModel) }
            composable("project/{id}/files") { backStackEntry ->
                val projectId = backStackEntry.arguments?.getString("id")!!
                ProjectFilesScreen(viewModel, projectId)
            }
            composable("gate/{sessionId}") { backStackEntry ->
                val sessionId = backStackEntry.arguments?.getString("sessionId")!!
                GateApprovalScreen(viewModel, sessionId)
            }
        }
    }
}
```

### 2.3 iOS UI (SwiftUI)

```swift
// iosApp/iOSApp/MainView.swift
struct MainView: View {
    @StateObject private var viewModel = MainViewModel()
    
    var body: some View {
        TabView {
            DashboardView(viewModel: viewModel)
                .tabItem { Label("Dashboard", systemImage: "house") }
            
            SessionListView(viewModel: viewModel)
                .tabItem { Label("Sessions", systemImage: "play.circle") }
            
            ProjectListView(viewModel: viewModel)
                .tabItem { Label("Projects", systemImage: "folder") }
        }
    }
}
```

### 2.4 Shared UI Components (Optional)

For maximum code reuse, consider **Compose Multiplatform** for UI:

```kotlin
// shared/src/commonMain/kotlin/ui/components/SessionCard.kt
@Composable
fun SessionCard(session: Session, onClick: () -> Unit) {
    Card(
        modifier = Modifier
            .fillMaxWidth()
            .padding(8.dp)
            .clickable(onClick = onClick),
        elevation = CardDefaults.cardElevation(defaultElevation = 2.dp)
    ) {
        Column(modifier = Modifier.padding(16.dp)) {
            Text(text = session.goal, style = MaterialTheme.typography.titleMedium)
            Text(text = "Phase: ${session.currentPhase}", style = MaterialTheme.typography.bodySmall)
            Text(text = "Status: ${session.status}", style = MaterialTheme.typography.bodySmall)
        }
    }
}
```

**Decision**: Use native UI frameworks (Compose + SwiftUI) for best platform integration, with shared business logic. Consider Compose Multiplatform for future iterations.

---

## 3. State Management Approach

### 3.1 Architecture: MVVM with Kotlin Flow

```
UI Layer (Compose/SwiftUI)
    ↓ observes
ViewModel (Android) / Observable (iOS)
    ↓ uses
UseCases (shared)
    ↓ calls
Repositories (shared)
    ↓ fetches
Network/Local Data Sources (shared)
```

### 3.2 State Flow Design

```kotlin
// shared/src/commonMain/kotlin/state/SessionState.kt
sealed class SessionUiState {
    object Loading : SessionUiState()
    data class Success(val session: Session) : SessionUiState()
    data class Error(val message: String) : SessionUiState()
}

// shared/src/commonMain/kotlin/state/UiState.kt
data class UiState<T>(
    val data: T? = null,
    val isLoading: Boolean = false,
    val error: String? = null
) {
    fun copyWithLoading(isLoading: Boolean = true) = copy(isLoading = isLoading)
    fun copyWithError(error: String) = copy(isLoading = false, error = error)
    fun copyWithData(data: T) = copy(data = data, isLoading = false, error = null)
}
```

### 3.3 ViewModel Pattern (Android)

```kotlin
// android/src/main/kotlin/com/orca/app/viewmodel/SessionViewModel.kt
class SessionViewModel(
    private val createSessionUseCase: CreateSessionUseCase,
    private val startSessionUseCase: StartSessionUseCase,
    private val sessionRepository: SessionRepository
) : ViewModel() {
    
    private val _uiState = MutableStateFlow<UiState<Session>>(UiState())
    val uiState: StateFlow<UiState<Session>> = _uiState.asStateFlow()
    
    fun createSession(goal: String, projectPath: String, projectType: String? = null) {
        viewModelScope.launch {
            _uiState.value = _uiState.value.copyWithLoading()
            try {
                val session = createSessionUseCase.execute(
                    CreateSessionRequest(goal, projectPath, projectType)
                )
                _uiState.value = _uiState.value.copyWithData(session)
            } catch (e: Exception) {
                _uiState.value = _uiState.value.copyWithError(e.message ?: "Unknown error")
            }
        }
    }
    
    fun startSession(sessionId: String) {
        viewModelScope.launch {
            try {
                startSessionUseCase.execute(sessionId)
                refreshSession(sessionId)
            } catch (e: Exception) {
                _uiState.value = _uiState.value.copyWithError(e.message ?: "Failed to start session")
            }
        }
    }
}
```

### 3.4 State Management (iOS)

```swift
// iosApp/iOSApp/ViewModels/MainViewModel.swift
import Combine
import shared

class MainViewModel: ObservableObject {
    @Published var uiState: UiState<Session>?
    @Published var sessions: [Session] = []
    
    private var cancellables = Set<AnyCancellable>()
    private let sessionRepository: SessionRepository
    
    init(sessionRepository: SessionRepository) {
        self.sessionRepository = sessionRepository
    }
    
    func createSession(goal: String, projectPath: String) {
        Task {
            do {
                let session = try await sessionRepository.createSession(
                    goal: goal,
                    projectPath: projectPath
                )
                await MainActor.run {
                    self.uiState = UiState.data(session)
                }
            } catch {
                await MainActor.run {
                    self.uiState = UiState.error(error.localizedDescription)
                }
            }
        }
    }
}
```

### 3.5 Background Operations

- **Kotlin Coroutines** for async operations
- **WorkManager** (Android) for background session polling
- **BackgroundTasks** (iOS) for periodic refresh
- **WebSocket** (future) for real-time session updates

---

## 4. Networking Layer

### 4.1 HTTP Client: Ktor Client

```kotlin
// shared/src/commonMain/kotlin/network/client/ApiClient.kt
class ApiClient(
    private val httpClient: HttpClient
) {
    suspend fun getSessions(): List<SessionDto> {
        return httpClient.get("/api/sessions")
    }
    
    suspend fun createSession(request: SessionCreateRequest): SessionDto {
        return httpClient.post("/api/sessions") {
            setBody(request)
        }
    }
}
```

### 4.2 API Service Definitions

```kotlin
// shared/src/commonMain/kotlin/network/api/SessionApi.kt
interface SessionApi {
    suspend fun createSession(request: SessionCreateRequest): SessionDto
    suspend fun listSessions(): List<SessionDto>
    suspend fun getSessionStatus(sessionId: String): SessionDto
    suspend fun startSession(sessionId: String): SessionActionResponse
    suspend fun pauseSession(sessionId: String): SessionActionResponse
    suspend fun resumeSession(sessionId: String): SessionActionResponse
    suspend fun stopSession(sessionId: String): SessionActionResponse
}

// shared/src/commonMain/kotlin/network/api/ProjectApi.kt
interface ProjectApi {
    suspend fun listProjects(): List<ProjectDto>
    suspend fun createProject(request: ProjectCreateRequest): ProjectDto
    suspend fun listProjectFiles(projectId: String, path: String? = null): List<FileEntryDto>
    suspend fun getFileContent(projectId: String, filePath: String): FileContentDto
}

// shared/src/commonMain/kotlin/network/api/GateApi.kt
interface GateApi {
    suspend fun getGateStatus(sessionId: String): GateResponse
    suspend fun respondToGate(sessionId: String, request: GateResponseRequest): GateActionResponse
}
```

### 4.3 Serialization

- **Kotlinx Serialization** for JSON parsing
- Custom serializers for date-time handling
- Polymorphic serialization for sealed classes

```kotlin
// shared/src/commonMain/kotlin/data/dto/SessionDto.kt
@Serializable
data class SessionDto(
    val id: String,
    val goal: String,
    val projectPath: String,
    val projectType: String,
    val currentPhase: String,
    val status: String,
    val createdAt: Instant,
    val updatedAt: Instant,
    val error: String? = null
)

// Mapper
fun SessionDto.toDomain(): Session = Session(
    id = id,
    goal = goal,
    projectPath = projectPath,
    projectType = projectType,
    currentPhase = Phase.valueOf(currentPhase),
    status = SessionStatus.valueOf(status),
    createdAt = createdAt,
    updatedAt = updatedAt,
    error = error
)
```

### 4.4 Interceptors

```kotlin
// shared/src/commonMain/kotlin/network/interceptor/LoggingInterceptor.kt
class LoggingInterceptor : HttpClientPlugin<LoggingInterceptor.Config, LoggingInterceptor> {
    // Request/Response logging with sensitive data redaction
}

// shared/src/commonMain/kotlin/network/interceptor/ErrorInterceptor.kt
class ErrorInterceptor : HttpClientPlugin<ErrorInterceptor.Config, ErrorInterceptor> {
    // Global error handling, retry logic, token refresh
}
```

### 4.5 Retry & Error Handling

```kotlin
// Retry configuration
val httpClient = HttpClient {
    install(RetryInterceptor) {
        maxRetries = 3
        backoffDelay = 1000
        maxBackoffDelay = 30000
        retryOnTimeout = true
    }
    install(Logging) {
        logger = Logger.DEFAULT
        level = LogLevel.INFO
    }
    install(ContentNegotiation) {
        json(Json {
            ignoreUnknownKeys = true
            isLenient = true
        })
    }
}
```

---

## 5. Testing Strategy

### 5.1 Test Pyramid

```
        /\
       /  \
      / E2E \          ← Instrumented tests (Android) / UI tests (iOS)
     /──────\
    / Unit   \         ← Kotlin multiplatform unit tests
   /──────────\
  / Integration\       ← API integration tests (mock server)
 /──────────────\
```

### 5.2 Unit Tests (Shared Module)

```kotlin
// shared/src/commonTest/kotlin/domain/usecase/CreateSessionUseCaseTest.kt
class CreateSessionUseCaseTest {
    
    private val repository = mockk<SessionRepository>()
    private val useCase = CreateSessionUseCase(repository)
    
    @Test
    fun `should create session successfully`() = runTest {
        // Given
        val request = CreateSessionRequest("Goal", "/path", "go")
        val session = Session(id = "1", goal = "Goal", ...)
        coEvery { repository.createSession(any()) } returns session
        
        // When
        val result = useCase.execute(request)
        
        // Then
        assertEquals(session, result)
        coVerify { repository.createSession(request) }
    }
    
    @Test
    fun `should throw error when goal is empty`() = runTest {
        val request = CreateSessionRequest("", "/path", "go")
        
        assertThrows<IllegalArgumentException> {
            useCase.execute(request)
        }
    }
}
```

### 5.3 Repository Tests

```kotlin
// shared/src/commonTest/kotlin/data/repository/SessionRepositoryImplTest.kt
class SessionRepositoryImplTest {
    
    private val api = mockk<SessionApi>()
    private val repository = SessionRepositoryImpl(api)
    
    @Test
    fun `should map DTO to domain entity`() = runTest {
        coEvery { api.getSessionStatus("1") } returns SessionDto(
            id = "1",
            goal = "Test",
            currentPhase = "planning",
            status = "running",
            createdAt = Instant.now(),
            updatedAt = Instant.now()
        )
        
        val result = repository.getSessionStatus("1")
        
        assertNotNull(result)
        assertEquals("1", result.id)
        assertEquals(Phase.PLANNING, result.currentPhase)
    }
}
```

### 5.4 Android-Specific Tests

```kotlin
// android/src/androidTest/kotlin/com/orca/app/ui/SessionListScreenTest.kt
@OptIn(ExperimentalTestApi::class)
@LargeTest
class SessionListScreenTest {
    
    @get:Rule
    val composeTestRule = createComposeRule()
    
    @Test
    fun sessionList_showsSessions() {
        composeTestRule.setContent {
            SessionListScreen(viewModel = fakeViewModel)
        }
        
        composeTestRule.onNodeWithText("Implement auth").assertExists()
    }
}
```

### 5.5 iOS-Specific Tests

```swift
// iosApp/iOSAppTests/MainViewModelTests.swift
import XCTest
@testable import iOSApp
import shared

class MainViewModelTests: XCTestCase {
    
    func testCreateSession_success() async throws {
        let mockRepo = MockSessionRepository()
        let viewModel = MainViewModel(sessionRepository: mockRepo)
        
        viewModel.createSession(goal: "Test", projectPath: "/path")
        
        await MainActor.run {
            XCTAssertNotNil(viewModel.uiState)
            XCTAssertEqual(viewModel.uiState?.data?.goal, "Test")
        }
    }
}
```

### 5.6 Mock Server for API Testing

```kotlin
// shared/src/commonTest/kotlin/TestServer.kt
object TestServer {
    val mockSession = SessionDto(
        id = "test-1",
        goal = "Test Goal",
        projectPath = "/test",
        projectType = "go",
        currentPhase = "planning",
        status = "pending",
        createdAt = Instant.now(),
        updatedAt = Instant.now()
    )
    
    fun mockSessionApi(): SessionApi = mockk {
        coEvery { listSessions() } returns listOf(mockSession)
        coEvery { createSession(any()) } returns mockSession
        coEvery { getSessionStatus("test-1") } returns mockSession
    }
}
```

---

## 6. Estimated Timeline

### 6.1 Phase Breakdown

| Phase | Duration | Deliverables |
|-------|----------|--------------|
| **Phase 1: Setup & Foundation** | 2 weeks | Project structure, DI setup, networking layer |
| **Phase 2: Core Features** | 3 weeks | Session management, project browsing |
| **Phase 3: Advanced Features** | 2 weeks | File editor, gate approval, activity log |
| **Phase 4: Polish & Testing** | 2 weeks | UI polish, testing, performance optimization |
| **Phase 5: Release** | 1 week | App Store submission, documentation |

**Total Estimated Timeline**: **10 weeks**

### 6.2 Detailed Sprint Plan

#### Sprint 1-2: Setup & Foundation (Weeks 1-2)

- [x] Project setup (Gradle, Koin, Ktor)
- [x] Network layer with Ktor Client
- [x] Serialization setup (kotlinx.serialization)
- [x] DI module structure
- [x] Base repository implementations
- [x] CI/CD pipeline (GitHub Actions)

**Deliverable**: Working networking layer with mock data

#### Sprint 3-5: Core Features (Weeks 3-5)

- [ ] Session CRUD operations
- [ ] Session lifecycle (start, pause, resume, stop)
- [ ] Session list UI (Android + iOS)
- [ ] Project management (create, list)
- [ ] Project file browsing
- [ ] State management with Flow/Observable

**Deliverable**: Functional session and project management

#### Sprint 6-7: Advanced Features (Weeks 6-7)

- [ ] File content viewer/editor
- [ ] Human gate approval flow
- [ ] Activity log display
- [ ] Phase tracker visualization
- [ ] Pull-to-refresh
- [ ] Offline support (if applicable)

**Deliverable**: Complete feature parity with web IDE

#### Sprint 8-9: Polish & Testing (Weeks 8-9)

- [ ] Unit tests (80%+ coverage on shared module)
- [ ] Android instrumented tests
- [ ] iOS UI tests
- [ ] Performance optimization
- [ ] Error handling polish
- [ ] Accessibility improvements
- [ ] Dark mode support

**Deliverable**: Production-ready app with tests

#### Sprint 10: Release (Week 10)

- [ ] App Store submission (iOS)
- [ ] Play Store submission (Android)
- [ ] Crash reporting setup (Sentry/Firebase)
- [ ] Analytics setup
- [ ] Final documentation
- [ ] Release notes

**Deliverable**: Published apps on both stores

### 6.3 Milestones

| Milestone | Week | Status |
|-----------|------|--------|
| Networking layer complete | 2 | ⬜ Not Started |
| Session management complete | 5 | ⬜ Not Started |
| Feature complete | 7 | ⬜ Not Started |
| Test coverage target met | 9 | ⬜ Not Started |
| App Store submission | 10 | ⬜ Not Started |
| **Public Release** | **10** | ⬜ Not Started |

### 6.4 Risk Mitigation

| Risk | Impact | Mitigation |
|------|--------|------------|
| Ktor multiplatform compatibility issues | High | Use stable versions, fallback to native HTTP clients |
| UI code sharing complexity | Medium | Start with separate UI, migrate to Compose Multiplatform later |
| iOS build complexity | Medium | Use Xcode Cloud for CI, provide detailed setup docs |
| API changes in Mini-Orca v2 | Low | API contract tests, version detection |
| Performance on low-end devices | Medium | Lazy loading, pagination, image caching |

---

## Appendix

### A. Technology Stack

| Component | Technology | Version |
|-----------|------------|---------|
| Language | Kotlin | 2.0+ |
| Multiplatform | Kotlin Multiplatform | 1.6+ |
| UI (Android) | Jetpack Compose | 1.5+ |
| UI (iOS) | SwiftUI | 5+ |
| HTTP Client | Ktor Client | 2.3+ |
| Serialization | kotlinx.serialization | 1.6+ |
| DI | Koin | 3.5+ |
| Coroutines | Kotlin Coroutines | 1.7+ |
| Testing | Kotlin Test, JUnit | 1.10+ |
| CI/CD | GitHub Actions | - |

### B. Mini-Orca API Endpoints Used

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/health` | GET | Health check |
| `/status` | GET | Daemon status |
| `/api/sessions` | GET, POST | Session management |
| `/api/sessions/{id}` | GET | Session detail |
| `/api/sessions/{id}/start` | POST | Start session |
| `/api/sessions/{id}/pause` | POST | Pause session |
| `/api/sessions/{id}/resume` | POST | Resume session |
| `/api/sessions/{id}/stop` | POST | Stop session |
| `/api/sessions/{id}/gate` | GET, POST | Gate operations |
| `/api/projects` | GET, POST | Project management |
| `/api/projects/{id}/files` | GET | File listing |
| `/api/projects/files/{path}` | GET | File content |

### C. Dependencies

**Shared Module (`shared/build.gradle.kts`)**

```kotlin
kotlin {
    sourceSets {
        commonMain.dependencies {
            implementation("io.ktor:ktor-client-core:2.3.7")
            implementation("io.ktor:ktor-client-content-negotiation:2.3.7")
            implementation("io.ktor:ktor-serialization-kotlinx-json:2.3.7")
            implementation("org.jetbrains.kotlinx:kotlinx-coroutines-core:1.7.3")
            implementation("org.jetbrains.kotlinx:kotlinx-serialization-json:1.6.2")
            implementation("io.insert-koin:koin-core:3.5.3")
        }
    }
}
```

**Android Module (`android/build.gradle.kts`)**

```kotlin
dependencies {
    implementation("androidx.core:core-ktx:1.12.0")
    implementation("androidx.lifecycle:lifecycle-viewmodel-compose:2.7.0")
    implementation("androidx.compose.ui:ui:1.6.0")
    implementation("androidx.compose.material3:material3:1.2.0")
    implementation("io.insert-koin:koin-android:3.5.3")
    implementation(project(":shared"))
}
```

---

**End of Plan**
