# Project Structure for Enhanced Orchestrator

## Overview
This document outlines the project structure for implementing an enhanced orchestrator with a light frontend dashboard, multi-phase workflow, and configurable models.

## Directory Structure

```
mini-orca/
├── src/
│   ├── main/
│   │   ├── java/
│   │   │   └── com/orca/
│   │   │       ├── orchestrator/
│   │   │       │   ├── WorkflowManager.java
│   │   │       │   ├── PhaseManager.java
│   │   │       │   └── ConfigurableModel.java
│   │   │       ├── dashboard/
│   │   │       │   ├── DashboardController.java
│   │   │       │   └── DashboardService.java
│   │   │       ├── models/
│   │   │       │   ├── ModelConfig.java
│   │   │       │   └── ModelRegistry.java
│   │   │       ├── planner/
│   │   │       │   ├── PlanningAgent.java
│   │   │       │   └── PlanValidator.java
│   │   │       ├── coder/
│   │   │       │   ├── CodingAgent.java
│   │   │       │   └── CodeGenerator.java
│   │   │       ├── tester/
│   │   │       │   ├── TestingAgent.java
│   │   │       │   └── TestRunner.java
│   │   │       ├── reviewer/
│   │   │       │   ├── ReviewAgent.java
│   │   │       │   └── CodeAnalyzer.java
│   │   │       └── ui/
│   │   │           ├── DashboardUI.java
│   │   │           └── StatusMonitor.java
│   │   └── resources/
│   │       ├── config/
│   │       │   ├── models.json
│   │       │   └── workflow.json
│   │       ├── templates/
│   │       │   ├── plan-template.md
│   │       │   └── code-template.java
│   │       └── static/
│   │           ├── css/
│   │           ├── js/
│   │           └── images/
│   └── test/
│       ├── java/
│       │   └── com/orca/
│       │       ├── orchestrator/
│       │       ├── dashboard/
│       │       └── models/
│       └── resources/
├── docs/
│   ├── design/
│   │   ├── frontend-design.md
│   │   ├── native-app-design.md
│   │   └── project-structure.md
│   └── user/
│       ├── getting-started.md
│       └── configuration-guide.md
├── scripts/
│   ├── setup.sh
│   └── run.sh
├── build.gradle
└── README.md
```

## Component Breakdown

### 1. Orchestrator Core
- **WorkflowManager**: Main orchestrator that coordinates all phases
- **PhaseManager**: Manages the execution flow between phases
- **ConfigurableModel**: Interface for model configuration and selection

### 2. Dashboard Module
- **DashboardController**: UI controller for dashboard interactions
- **DashboardService**: Service layer for dashboard data management
- **UI Components**: React-based or JavaFX components for the dashboard

### 3. Phase Agents
- **PlanningAgent**: Generates and manages execution plans
- **CodingAgent**: Writes specific functions based on plan
- **TestingAgent**: Executes tests for written code
- **ReviewAgent**: Analyzes and reviews generated code

### 4. Model Management
- **ModelConfig**: Configuration class for model parameters
- **ModelRegistry**: Central registry for available models

### 5. Configuration and Templates
- **models.json**: JSON configuration file for model settings
- **workflow.json**: Configuration for workflow phases and transitions
- **plan-template.md**: Template for generating plan documentation
- **code-template.java**: Template for generated code files

## Configuration Files

### models.json
```json
{
  "planningModel": {
    "name": "gpt-4",
    "temperature": 0.7,
    "maxTokens": 2048
  },
  "codingModel": {
    "name": "gpt-4",
    "temperature": 0.3,
    "maxTokens": 1024
  },
  "testingModel": {
    "name": "gpt-4",
    "temperature": 0.5,
    "maxTokens": 1024
  },
  "reviewModel": {
    "name": "gpt-4",
    "temperature": 0.2,
    "maxTokens": 1024
  }
}
```

### workflow.json
```json
{
  "phases": [
    {
      "name": "planning",
      "enabled": true,
      "model": "planningModel"
    },
    {
      "name": "coding",
      "enabled": true,
      "model": "codingModel"
    },
    {
      "name": "testing",
      "enabled": true,
      "model": "testingModel"
    },
    {
      "name": "review",
      "enabled": true,
      "model": "reviewModel"
    }
  ],
  "approvalRequired": {
    "planning": true,
    "coding": false,
    "testing": false,
    "review": false
  }
}
```

## Build and Deployment

### Gradle Configuration (build.gradle)
```gradle
plugins {
    id 'java'
    id 'application'
}

repositories {
    mavenCentral()
}

dependencies {
    implementation 'com.fasterxml.jackson.core:jackson-databind:2.15.2'
    implementation 'org.slf4j:slf4j-api:2.0.7'
    testImplementation 'org.junit.jupiter:junit-jupiter-api:5.9.2'
    testRuntimeOnly 'org.junit.jupiter:junit-jupiter-engine:5.9.2'
}

application {
    mainClass = 'com.orca.Main'
}
```

## Documentation Structure

### Design Documentation
1. **frontend-design.md**: Detailed UI design and component specifications
2. **native-app-design.md**: Native application architecture and implementation plan
3. **project-structure.md**: Project structure and component breakdown

### User Documentation
1. **getting-started.md**: Quick start guide for new users
2. **configuration-guide.md**: Detailed instructions for model and workflow configuration

## Development Workflow

### Phase 1: Core Implementation
1. Implement WorkflowManager and PhaseManager
2. Create basic dashboard UI with status indicators
3. Set up model configuration system

### Phase 2: Phase Agents Implementation
1. Develop PlanningAgent with plan generation capabilities
2. Implement CodingAgent for code generation
3. Create TestingAgent with test execution functionality
4. Build ReviewAgent for code analysis

### Phase 3: Dashboard Integration
1. Connect dashboard to workflow manager
2. Implement real-time status updates
3. Add configuration management interface

### Phase 4: Native App Development (Future)
1. Create Kotlin multiplatform project structure
2. Implement Android and desktop versions
3. Integrate with existing orchestrator core

This project structure provides a solid foundation for implementing the enhanced orchestrator with all requested features, including the configurable multi-phase workflow and future native app capabilities.