# Project Structure

## Overview
This document outlines the project structure for the mini-orca project with the enhanced requirements including dashboard, multi-agent workflow, and model configurability.

## Directory Structure
```
mini-orca/
├── src/
│   ├── orchestrator/
│   │   ├── index.js
│   │   ├── phases/
│   │   │   ├── planning.js
│   │   │   ├── coding.js
│   │   │   ├── testing.js
│   │   │   └── review.js
│   │   └── agents/
│   │       ├── planner.js
│   │       ├── coder.js
│   │       ├── tester.js
│   │       └── reviewer.js
│   ├── config/
│   │   ├── index.js
│   │   └── models.js
│   ├── dashboard/
│   │   ├── index.js
│   │   ├── components/
│   │   │   ├── StatusPanel.jsx
│   │   │   ├── ArtifactViewer.jsx
│   │   │   └── ConfigManager.jsx
│   │   └── api/
│   │       └── routes.js
│   ├── utils/
│   │   └── logger.js
│   └── index.js
├── config/
│   ├── default.json
│   └── models.json
├── tests/
│   └── orchestrator.test.js
├── package.json
└── README.md
```

## Core Components

### Orchestrator
The orchestrator is the main workflow controller that manages the execution of phases and agents.

### Agents
Each agent handles a specific aspect of the development workflow:
- Planner: Creates project plans based on requirements
- Coder: Generates specific code functions
- Tester: Executes tests and validates code quality
- Reviewer: Performs code review and quality checks

### Configuration System
The configuration system allows for flexible model selection and parameter management across all phases.

### Dashboard
A lightweight frontend interface to monitor workflow progress, view artifacts, and manage configurations.

## Configuration Files

### Default Configuration
The default configuration file (`config/default.json`) contains:
- Default model selections for each phase
- Phase execution parameters
- Dashboard UI settings

### Model Configuration
The model configuration file (`config/models.json`) contains:
- Available models for each phase (LLM options)
- Model parameters and settings
- Authentication credentials for external services

## Implementation Approach

### Phase Flow Control
1. Planning phase - Agent creates project plan
2. Coding phase - Agent generates specific function code
3. Testing phase - Agent executes tests on generated code
4. Review phase - Agent reviews code quality and security
5. User approval - Manual review and confirmation

### Model Configuration
Each phase can use different models with configurable parameters:
- Planning model (e.g., GPT-4, Claude)
- Coding model (e.g., CodeLlama, GPT-3.5)
- Testing model (e.g., GPT-4, CodeBERT)
- Review model (e.g., GPT-4, SonarQube)

### Dashboard Integration
The dashboard provides:
- Real-time status monitoring of workflow phases
- Artifact display with syntax highlighting
- Configuration management interface
- User approval workflow for code changes

### Future Native App Extension
The architecture is designed to support future native app development:
- API-first approach for dashboard backend services
- Component-based UI that can be ported to native frameworks
- Local storage capabilities for offline operation

## Technology Stack
### Backend
- Node.js with Express.js for API services
- JavaScript/TypeScript for core logic

### Frontend (Dashboard)
- React.js or Vue.js for component-based UI
- Material-UI or Tailwind CSS for styling
- WebSocket for real-time updates

### Native App (Future)
- Kotlin with Jetpack Compose for Android UI
- REST/GraphQL API for backend integration