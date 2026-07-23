# Mini-Orca Improvement Plan

This document outlines the improvement plan for the mini-orca project, focusing on enhancing its capabilities to support a more sophisticated orchestrator with multiple phases and a dashboard interface.

## Overview

The current mini-orca project is a local agents orchestrator for mini tasks. This improvement plan aims to transform it into a more robust system with:

1. A light frontend dashboard for execution and monitoring
2. A multi-phase orchestrator workflow
3. Configurable models for each phase
4. Future native app support with Kotlin

## Project Structure

```
plan/
├── README.md
├── 01-architecture.md
├── 02-phase-flow.md
├── 03-agent-skills.md
├── 04-frontend-dashboard.md
├── 05-ide-dashboard.md
├── 06-implementation-plan.md
└── kotlin-native.md
```

## Improvement Goals

1. **Multi-Phase Orchestrator** - Implement a structured workflow with planning, coding, testing, and review phases
2. **Configurable Models** - Allow each phase to use different AI models
3. **Dashboard Interface** - Create a frontend dashboard for monitoring and control
4. **Native App Support** - Plan for Kotlin-based native application development