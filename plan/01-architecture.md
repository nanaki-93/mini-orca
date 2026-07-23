# Architecture Overview

## Current State
The current mini-orca project is a simple local agents orchestrator that handles basic tasks. It lacks:
- A structured multi-phase workflow
- Configurable models per phase
- A dashboard interface for monitoring and control
- Native app support capabilities

## Proposed Architecture

### Core Components
1. **Orchestrator Engine** - Manages the multi-phase workflow execution
2. **Agent Manager** - Handles different types of agents (planning, coding, testing, review)
3. **Model Configurator** - Allows configuration of models for each phase
4. **Dashboard Interface** - Web-based dashboard for monitoring and control
5. **State Manager** - Tracks project state across phases
6. **Task Queue** - Manages task execution order

### System Flow
```
User Input → Planning Phase → Coding Phase → Testing Phase → Review Phase → User Approval
      ↑                                                           ↓
    Configuration                                               Integration
```

### Technology Stack Considerations
- Go for backend services (existing)
- React/Vue.js for dashboard frontend (new)
- REST/GraphQL API for communication
- SQLite/PostgreSQL for state persistence (optional)

### Future Native App Support
- Kotlin Multiplatform for cross-platform mobile apps
- Jetpack Compose for modern UI development (Android)
- SwiftUI for iOS development (future consideration)