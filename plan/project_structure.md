# Project Structure Plan

## Overall Architecture

The project will be organized into multiple components to support the desired workflow and native application development:

```
mini-orca/
├── orchestrator/           # Main Go orchestrator
│   ├── core/              # Core workflow logic
│   ├── agents/            # Individual agent implementations
│   ├── models/            # Data models and domain objects
│   └── config/            # Configuration management
├── dashboard/             # Frontend dashboard application
│   ├── ui/                # UI components and views
│   ├── api/               # API client for backend communication
│   └── state/             # State management
├── native-app/            # Native application (Kotlin)
│   ├── common/            # Shared code between platforms
│   ├── android/           # Android-specific implementation
│   ├── ios/               # iOS-specific implementation
│   ├── desktop/           # Desktop application
│   └── web/               # Web application
├── docs/                  # Documentation
│   ├── workflow.md        # Workflow description
│   └── native_app.md      # Native app development plan
├── tests/                 # Test suite
└── README.md              # Project documentation
```

## Component Details

### 1. Orchestrator (Go)
- Main workflow engine that coordinates all phases
- Configuration management for different models
- Communication with frontend dashboard
- State persistence and tracking

### 2. Dashboard (Frontend)
- Web-based user interface
- Real-time status monitoring
- User approval workflows
- Configuration management UI

### 3. Native App (Kotlin)
- Multi-platform implementation using Kotlin Multiplatform
- Cross-platform UI components
- Platform-specific features and optimizations

## Implementation Priorities

### Phase 1: Core Workflow Engine
1. Implement orchestrator with configurable models
2. Create agent implementations for each phase:
   - Planning agent
   - Coding agent
   - Testing agent
   - Review agent
3. Implement approval workflow with user interaction

### Phase 2: Dashboard Development
1. Create frontend dashboard with:
   - Real-time status updates
   - Code display and editing capabilities
   - Approval/rejection interface
2. Implement API communication with orchestrator

### Phase 3: Native Application Development
1. Start multi-platform Kotlin project
2. Implement core workflow features in Kotlin
3. Create platform-specific UI components
4. Integrate with existing backend services

### Phase 4: Full Integration and Migration
1. Complete dashboard integration with orchestrator
2. Implement full native application functionality
3. Gradual migration from Go to Kotlin implementation

## Technology Stack

### Backend (Go)
- Go 1.19+ for main orchestrator
- Gin or Echo for HTTP routing
- Goroutines and channels for concurrency
- JSON serialization for data exchange

### Frontend (Dashboard)
- React.js or Vue.js for UI components
- WebSockets for real-time updates
- TypeScript for type safety
- Tailwind CSS or Material UI for styling

### Native App (Kotlin)
- Kotlin Multiplatform Mobile for cross-platform
- Ktor for networking
- Compose Multiplatform for UI
- Kotlin Coroutines for asynchronous operations

## Configuration Management

### Model Configuration
1. Configurable models per workflow phase:
   - Planning model (e.g., GPT-4, Claude)
   - Coding model (e.g., GPT-4, CodeLLaMA)
   - Testing model (e.g., GPT-4, CodeT5)
   - Review model (e.g., GPT-4, Claude)
2. Configuration via:
   - Environment variables
   - Configuration files (YAML/JSON)
   - Dashboard UI controls

### Workflow Settings
1. Phase timeout configurations
2. Approval thresholds and policies
3. Error handling and retry logic

## Data Flow

### Orchestrator to Dashboard
1. Workflow status updates via WebSocket
2. Code changes and test results
3. Approval requests to user interface

### Dashboard to Orchestrator
1. User approval/rejection messages
2. Configuration updates
3. Manual workflow triggers

### Native App to Orchestrator
1. Same data flow as dashboard
2. Additional platform-specific features

## Testing Strategy

### Unit Tests
- Individual agent implementations
- Configuration management
- Workflow state transitions

### Integration Tests
- Dashboard to orchestrator communication
- Native app backend integration
- Multi-platform component testing

### End-to-End Tests
- Complete workflow execution
- User approval and rejection scenarios
- Cross-platform functionality testing

## Deployment Considerations

### Backend Deployment
1. Containerized Go application (Docker)
2. Kubernetes orchestration (if needed)
3. Load balancing capabilities

### Frontend Deployment
1. Static site hosting (Netlify, Vercel)
2. Webpack or Vite build pipeline
3. CDN support for assets

### Native App Deployment
1. Android app store (Google Play)
2. iOS app store (Apple Store)
3. Desktop package distribution
4. Web application deployment

This project structure provides a clear roadmap for implementing the desired features while maintaining flexibility for future enhancements and platform-specific optimizations.