# Mini-Orca v2.0 — Roadmap & Architecture Plan

## Overview

This folder contains the complete plan to evolve **mini-orca** from a basic single-agent orchestrator into a **multi-agent, multi-phase, configurable IDE-like orchestration platform** with a lightweight HTMX frontend.

---

## Current State (v1.0)

- Single agent calling LM Studio
- Basic state machine with 7 states
- Simple HTMX + Tailwind dashboard
- CLI fallback for approvals
- Hardcoded model (`qwen/qwen3-coder-30b`)
- File-based JSON state store
- Language-specific tools (Go, Gradle, Terraform)

---

## Target State (v2.0)

```
┌──────────────────────────────────────────────────────────────────────────────────┐
│                    IDE-LIKE HTMX FRONTEND                                        │
│                                                                                  │
│  ┌──────────────┐  ┌──────────────────────────────────────────────────────────┐  │
│  │  FILE TREE   │  │  MAIN WORKSPACE (IDE-like)                                │  │
│  │              │  │                                                           │  │
│  │  📁 src/     │  │  ┌─────────────────────────────────────────────────────┐  │  │
│  │  📁 auth/    │  │  │  Code Editor (single function OR full file)          │  │  │
│  │  📁 models/  │  │  │                                                     │  │  │
│  │  📄 main.go  │  │  │  func ValidateToken(token string) bool {            │  │  │
│  │  📄 user.go  │  │  │      // generated code...                           │  │  │
│  │  📄 ...      │  │  │  }                                                  │  │  │
│  │              │  │  │                                                     │  │  │
│  │  [Insert]    │  │  │  [Edit Function] [Edit Full File] [Submit]          │  │  │
│  │  [Write Fn]  │  │  └─────────────────────────────────────────────────────┘  │  │
│  └──────────────┘  │                                                           │  │
│                    │  ┌─────────────────────────────────────────────────────┐  │  │
│                    │  │  Phase Tracker: [Plan] → [Code] → [Test] → [Rev] → [Human] │  │
│                    │  └─────────────────────────────────────────────────────┘  │  │
│                    │                                                           │  │
│                    │  ┌─────────────────────────────────────────────────────┐  │  │
│                    │  │  Activity Log (scrollable)                          │  │  │
│                    │  │  [12:34] Planning started                           │  │  │
│                    │  │  [12:35] Plan generated                             │  │  │
│                    │  │  [12:36] Code generated: ValidateToken              │  │  │
│                    │  └─────────────────────────────────────────────────────┘  │  │
│                    └───────────────────────────────────────────────────────────┘  │
│  ┌──────────────────────────────────────────────────────────────────────────────┐  │
│  │  HEADER: Project Path | Goal | Model Config | [New Session]              │  │
│  └──────────────────────────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────────────────────┘
                            │ REST API
┌───────────────────────────▼───────────────────────────────────────────────────────┐
│                        ORCHESTRATOR DAEMON                                         │
│                                                                                  │
│  ┌────────────────────────────────────────────────────────────────────────────┐  │
│  │                    API Server (Go)                                          │  │
│  │  - REST endpoints for session management                                   │  │
│  │  - Server-side HTML rendering (HTMX templates)                             │  │
│  │  - No authentication (local environment)                                   │  │
│  └──────────────────────────────┬─────────────────────────────────────────────┘  │  │
│                                 │                                                 │  │
│  ┌──────────────────────────────▼─────────────────────────────────────────────┐  │
│  │               Orchestrator Engine                                          │  │
│  │  - Phase Router (manages A→B→C→D→E flow)                                  │  │  │
│  │  - Human Gate Manager (handles user approvals)                             │  │  │
│  │  - State Manager (persistent state store)                                  │  │  │
│  └──────────────────────────────┬─────────────────────────────────────────────┘  │  │
│                                 │                                                 │  │
│  ┌──────────────────────────────▼─────────────────────────────────────────────┐  │
│  │              Agent Registry                                                │  │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐                     │  │  │
│  │  │ Planner  │ │  Coder   │ │  Tester  │ │ Reviewer │                     │  │  │
│  │  │ +Skills  │ │ +Skills  │ │ +Skills  │ │ +Skills  │                     │  │  │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘                     │  │  │
│  └──────────────────────────────┬─────────────────────────────────────────────┘  │  │
│                                 │                                                 │  │
│  ┌──────────────────────────────▼─────────────────────────────────────────────┐  │
│  │              Model Router                                                  │  │
│  │  ┌────────────────────────────────────────────────────────────────────┐    │  │  │
│  │  │  Provider Interface (extensible)                                   │    │  │  │
│  │  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐         │    │  │  │
│  │  │  │ LM Studio│  │ Ollama   │  │ OpenAI   │  │ Anthropic│ (future)│    │  │  │
│  │  │  └──────────┘  └──────────┘  └──────────┘  └──────────┘         │    │  │  │
│  │  └────────────────────────────────────────────────────────────────────┘    │  │  │
│  └─────────────────────────────────────────────────────────────────────────────┘  │
│                                                                                  │
│  ┌────────────────────────────────────────────────────────────────────────────┐  │
│  │        LANGUAGE-AGNOSTIC TOOL EXECUTOR                                     │  │
│  │  - Auto-detect project type (Go, Kotlin, Java, Rust, TS, Python)          │  │  │
│  │  - Shell commands, file ops, build/test/run                               │  │  │
│  │  - Code formatting (per-language formatters)                              │  │  │
│  │  - Atomic modifications: ONE function/struct/class at a time              │  │  │
│  │  - Error display when LM Studio is unreachable                            │  │  │
│  └────────────────────────────────────────────────────────────────────────────┘  │
│                                                                                  │
│  ┌────────────────────────────────────────────────────────────────────────────┐  │
│  │              State Store                                                   │  │
│  │  - JSON file                                                               │  │
│  └────────────────────────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────────────────────┘
```

---

## Phase Flow (5 Stages)

```
User Input (Goal + Project Path)
    │
    ▼
┌──────────────────────────────────────────────────────────────────────────────────────┐
│  STAGE A — PLANNING                                                                  │
│  Agent: Planner (with Skills: architecture, task_breakdown, dependency_mapping)      │
│  Action: Analyze user specs + business logic file → generate plan with atomic units │
│  Gate:  User reviews and CONFIRMS the plan                                           │
│  ▼                                                                                   │
└──────────────────────────────────────────────────────────────────────────────────────┘
    │ (confirmed)
    ▼
┌──────────────────────────────────────────────────────────────────────────────────────┐
│  STAGE B — CODING                                                                    │
│  Agent: Coder (with Skills: function_generation, struct_design, class_creation)      │
│  Action: Write ONE function / struct / class based on the approved plan              │
│  Gate:  Auto-pass → Testing                                                          │
│  ▼                                                                                   │
└──────────────────────────────────────────────────────────────────────────────────────┘
    │
    ▼
┌──────────────────────────────────────────────────────────────────────────────────────┐
│  STAGE C — TESTING                                                                   │
│  Agent: Tester (with Skills: unit_testing, integration_testing, coverage_analysis)   │
│  Action: Run tests, analyze results, report issues                                   │
│  Gate:  Auto-pass or fail (loop back to Coding if fails)                             │
│  ▼                                                                                   │
└──────────────────────────────────────────────────────────────────────────────────────┘
    │ (tests pass)
    ▼
┌──────────────────────────────────────────────────────────────────────────────────────┐
│  STAGE D — REVIEW                                                                    │
│  Agent: Reviewer (with Skills: style_check, logic_review, security_audit)            │
│  Action: Code review — quality, style, correctness, SOLID, clean code                │
│  Gate:  Pass or fail (loop back to Coding if fails)                                  │
│  ▼                                                                                   │
└──────────────────────────────────────────────────────────────────────────────────────┘
    │ (review passes)
    ▼
┌──────────────────────────────────────────────────────────────────────────────────────┐
│  STAGE E — HUMAN REVIEW (IDE)                                                        │
│  Human: Developer                                                                    │
│  Action: Review final code in IDE editor, accept or reject                           │
│          - Accept → commit to file, continue to next unit                            │
│          - Reject → loop back to Coding (no feedback loop)                           │
│          - Edit → modify code, resubmit → Testing → Review → Human Review           │
│  ▼                                                                                   │
└──────────────────────────────────────────────────────────────────────────────────────┘
    │ (accepted)
    ▼
  Next atomic unit (loop B→C→D→E) or COMPLETED
```

---

## Key Features

| Feature | Description |
|---------|-------------|
| **IDE-like HTMX Frontend** | File tree + code editor + phase tracker — all via HTMX, no build step, zero client-side JS frameworks |
| **No Alpine.js** | All interactivity via HTMX + server-side rendering. Cleaner architecture, simpler debugging. |
| **Configurable Agent Skills** | Each agent has a unique, configurable set of skills (tools + knowledge) |
| **Skills Library** | Predefined skills: SOLID, Clean Code, KISS, No Repetition, Business Logic |
| **Configurable Models** | Each phase uses a different model (LM Studio first, interface for others) |
| **Multi-Agent Architecture** | Dedicated agents for each phase with specialized prompts |
| **LM Studio First** | Starts with LM Studio, interface built for easy provider extension |
| **Language-Agnostic Tools** | Auto-detects project type, runs appropriate commands |
| **Atomic Modifications** | Each agent modifies exactly ONE function, struct, or class per cycle |
| **Single Project** | One project at a time (no multi-session) |
| **No Authentication** | Local environment only |
| **Dark Theme** | Dark theme by default, extensible for more themes |
| **Formatted Code** | Code is auto-formatted by language-specific formatters |
| **Native Kotlin Future** | Architecture designed to support a Kotlin-native client |

---

## File Structure

```
plan/
├── README.md                  ← This file (overview)
├── 01-architecture.md         ← System architecture & component design
├── 02-phase-flow.md           ← Detailed phase flow & state machine
├── 03-agent-skills.md         ← NEW: Agent skills system (tools + knowledge)
├── 04-model-config.md         ← Model configuration system (LM Studio + extensible)
├── 05-ide-dashboard.md        ← REWRITTEN: IDE-like HTMX frontend
├── 06-implementation-plan.md  ← Updated: Step-by-step implementation roadmap
└── kotlin-native.md           ← Future: Kotlin native app plan
```

---

## Quick Start (After Implementation)

```bash
# 1. Start the daemon with model config
mini-orca daemon --config config.yaml

# 2. Open the IDE dashboard
open http://localhost:8080/ide

# 3. Enter your project goal → Review plan → Approve → Code in IDE → Accept/Reject
```
