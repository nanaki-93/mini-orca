# Mini-Orca Kotlin Native Desktop App

> Plan for converting Mini-Orca to a native Kotlin desktop application

---

## Overview

This document outlines the plan for creating a native Kotlin desktop application for Mini-Orca, replacing the web-based frontend with a native UI.

---

## Architecture

```
┌─────────────────────────────────────────┐
│          Kotlin Desktop App             │
│  ┌───────────┐  ┌────────────────────┐  │
│  │  Jetpack  │  │  Compose Desktop   │  │
│  │   Compose │  │  (Multiplatform)   │  │
│  └───────────┘  └────────────────────┘  │
│                    │                     │
│  ┌─────────────────┼─────────────────┐  │
│  │   Mini-Orca Core (JVM)              │ │
│  │  - Orchestrator                     │ │
│  │  - Agent Registry                   │ │
│  │  - State Management                 │ │
│  │  - Tool Executor                    │ │
│  └─────────────────┼─────────────────┘  │
│                    │                     │
│  ┌─────────────────┼─────────────────┐  │
│  │  HTTP Client (Ktor)                 │ │
│  │  - Communicate with backend         │ │
│  └─────────────────┴─────────────────┘  │
└─────────────────────────────────────────┘
         │                    │
┌────────┴────────┐  ┌───────┴────────┐
│  Mini-Orca CLI  │  │  Mini-Orca     │
│  (Go Backend)   │  │  Backend       │
│  (Optional)     │  │  (Kotlin)      │
└─────────────────┘  └────────────────┘
```

---

## Phase 1: Research & Setup (2 weeks)

### 1.1 Technology Selection
- [x] Evaluate Jetpack Compose for Desktop
- [x] Evaluate Ktor for HTTP client
- [x] Evaluate serialization library (Kotlinx Serialization)
- [ ] Set up multiplatform project structure

### 1.2 Project Structure
```
mini-orca-desktop/
├── build.gradle.kts
├── settings.gradle.kts
├── gradle.properties
├── src/
│   └── commonMain/
│       └── kotlin/
│           ├── MiniOrcaApp.kt
│           ├── components/
│           │   ├── FileTree.kt
│           │   ├── PhaseTracker.kt
│           │   ├── ActivityLog.kt
│           │   └── CodeEditor.kt
│           ├── screens/
│           │   ├── DashboardScreen.kt
│           │   ├── PlanningScreen.kt
│           │   ├── CodingScreen.kt
│           │   └── ReviewScreen.kt
│           ├── state/
│           │   ├── SessionState.kt
│           │   └── ConfigState.kt
│           └── api/
│               ├── ApiClient.kt
│               └── Models.kt
```

### 1.3 Dependencies
```kotlin
dependencies {
    implementation("org.jetbrains.compose.material3:material3")
    implementation("org.jetbrains.compose.material:material")
    implementation("io.ktor:ktor-client-core:2.3.0")
    implementation("io.ktor:ktor-client-cio:2.3.0")
    implementation("io.ktor:ktor-client-content-negotiation:2.3.0")
    implementation("io.ktor:ktor-serialization-kotlinx-json:2.3.0")
    implementation("org.jetbrains.kotlinx:kotlinx-coroutines-core:1.7.3")
}
```

---

## Phase 2: Core UI Components (3 weeks)

### 2.1 Base Layout
- [ ] Main window with sidebar and content area
- [ ] Dark/light theme support
- [ ] Responsive layout (sidebar toggle)
- [ ] System tray integration

### 2.2 File Tree Component
- [ ] Recursive file tree with icons
- [ ] File selection and highlighting
- [ ] Context menu (open, rename, delete)
- [ ] Drag and drop support

### 2.3 Phase Tracker Component
- [ ] Horizontal progress bar
- [ ] Phase status indicators
- [ ] Click to jump to phase
- [ ] Animated transitions

### 2.4 Activity Log Component
- [ ] Scrollable log view
- [ ] Timestamp and severity indicators
- [ ] Search/filter functionality
- [ ] Auto-scroll to latest

### 2.5 Code Editor Component
- [ ] Syntax highlighting (Kotlinx Highlight)
- [ ] Line numbers
- [ ] Tab support
- [ ] Diff view (side-by-side)
- [ ] Auto-format integration

---

## Phase 3: Screen Implementation (3 weeks)

### 3.1 Dashboard Screen
- [ ] Session overview cards
- [ ] Quick actions (new session, resume)
- [ ] Recent sessions list
- [ ] System status indicators

### 3.2 Planning Screen
- [ ] Plan display with atomic units
- [ ] Expandable unit details
- [ ] Approve/reject buttons
- [ ] Change request modal

### 3.3 Coding Screen
- [ ] Current unit display
- [ ] Code editor with syntax highlighting
- [ ] Test output panel
- [ ] Agent activity indicators

### 3.4 Testing Screen
- [ ] Test results display
- [ ] Coverage visualization
- [ ] Test failure details
- [ ] Retry controls

### 3.5 Review Screen
- [ ] Code review with comments
- [ ] Diff viewer
- [ ] Approval workflow
- [ ] Change history

---

## Phase 4: Backend Integration (2 weeks)

### 4.1 API Client
- [ ] Ktor HTTP client setup
- [ ] JSON serialization/deserialization
- [ ] Error handling and retry logic
- [ ] WebSocket support (optional)

### 4.2 State Management
- [ ] Session state management
- [ ] Configuration persistence
- [ ] Offline support (optional)

### 4.3 Background Tasks
- [ ] Orchestrator process management
- [ ] Real-time log streaming
- [ ] Progress tracking

---

## Phase 5: Polish & Distribution (2 weeks)

### 5.1 Polish
- [ ] Keyboard shortcuts
- [ ] Accessibility improvements
- [ ] Localization support
- [ ] Performance optimization

### 5.2 Distribution
- [ ] Create installer (DMG, MSI)
- [ ] Code signing
- [ ] Auto-update mechanism
- [ ] App Store submission (optional)

---

## Alternative: WebView Approach

If native Compose proves too complex, consider a simpler approach:

```
┌─────────────────────────────────────────┐
│          Kotlin Desktop App             │
│  ┌───────────────────────────────────┐  │
│  │         WebView (CEF)             │  │
│  │  - HTMX + Alpine.js frontend      │  │
│  │  - Same as web version            │  │
│  └───────────────────────────────────┘  │
│                    │                     │
│  ┌─────────────────┼─────────────────┐  │
│  │  Kotlin Backend (JVM)               │ │
│  │  - Mini-Orca Core                   │ │
│  │  - Embedded HTTP Server             │ │
│  └─────────────────┴─────────────────┘  │
└─────────────────────────────────────────┘
```

**Advantages:**
- Reuse existing web frontend
- Faster development
- Simpler maintenance

**Disadvantages:**
- Larger binary size
- Less native feel

---

## Timeline Estimate

| Phase | Duration | Total |
|-------|----------|-------|
| 1. Research & Setup | 2 weeks | 2 weeks |
| 2. Core UI Components | 3 weeks | 5 weeks |
| 3. Screen Implementation | 3 weeks | 8 weeks |
| 4. Backend Integration | 2 weeks | 10 weeks |
| 5. Polish & Distribution | 2 weeks | 12 weeks |

**Total: ~12 weeks**

---

## Risk Assessment

| Risk | Impact | Mitigation |
|------|--------|------------|
| Jetpack Compose Desktop maturity | Medium | Use stable version, fallback to WebView |
| Performance with large files | Medium | Virtual scrolling, lazy loading |
| Cross-platform compatibility | Low | Test on all platforms early |
| Learning curve | High | Allocate time for Compose learning |

---

## Next Steps

1. **Week 1:** Set up project, create basic window
2. **Week 2:** Implement API client, connect to Go backend
3. **Week 3-5:** Build core UI components
4. **Week 6-8:** Implement screens
5. **Week 9-10:** Backend integration
6. **Week 11-12:** Polish and distribution

---

*Created: January 2024*
