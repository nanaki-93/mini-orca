# Mini-Orca v2.0 — Roadmap & Architecture Plan

## Overview

This folder contains the complete plan to evolve **mini-orca** from a basic single-agent orchestrator into a **multi-agent, multi-phase, configurable orchestration platform** with a lightweight HTMX frontend.

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
┌─────────────────────────────────────────────────────────────────────┐
│                    LIGHT HTMX FRONTEND DASHBOARD                     │
│  (HTMX + Alpine.js + TailwindCSS)                                   │
│  - Real-time state visualization                                    │
│  - Per-phase code review UI                                         │
│  - Model configuration panel                                        │
│  - Session management & history                                     │
└────────────────────────┬────────────────────────────────────────────┘
                         │ REST API
┌────────────────────────▼────────────────────────────────────────────┐
│                        ORCHESTRATOR DAEMON                           │
│                                                                      │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐           │
│  │ PLANNER  │→│ CODER    │→│ TESTER   │→│ REVIEWER │           │
│  │ Agent    │  │ Agent    │  │ Agent    │  │ Agent    │           │
│  │ (config) │  │ (config) │  │ (config) │  │ (config) │           │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘           │
│         │              │              │              │              │
│         ▼              ▼              ▼              ▼              │
│  ┌──────────────────────────────────────────────────────────┐       │
│  │              HUMAN REVIEW GATE                           │       │
│  │        (Accept / Reject / Request Changes)               │       │
│  └──────────────────────────────────────────────────────────┘       │
│                                                                      │
│  ┌──────────────────────────────────────────────────────────┐       │
│  │         MODEL ROUTER (LM Studio first, extensible)       │       │
│  │  - Provider interface (ready for Ollama, OpenAI, etc.)   │       │
│  │  - Per-phase model, temperature, max_tokens              │       │
│  └──────────────────────────────────────────────────────────┘       │
│                                                                      │
│  ┌──────────────────────────────────────────────────────────┐       │
│  │         LANGUAGE-AGNOSTIC TOOL EXECUTOR                  │       │
│  │  - Detect project type automatically                     │       │
│  │  - Shell commands, file ops, build/test/run              │       │
│  └──────────────────────────────────────────────────────────┘       │
└─────────────────────────────────────────────────────────────────────┘
```

---

## Phase Flow (5 Stages)

```
User Input
    │
    ▼
┌─────────────────────────────────────────────────────────────┐
│  STAGE A — PLANNING                                         │
│  Agent: Planner                                               │
│  Action: Analyze user specs + project context → generate plan │
│  Gate:  User reviews & CONFIRMS the plan                     │
│  ▼                                                             │
└─────────────────────────────────────────────────────────────┘
    │ (confirmed)
    ▼
┌─────────────────────────────────────────────────────────────┐
│  STAGE B — CODING                                           │
│  Agent: Coder                                                 │
│  Action: Write ONE function / struct / class                 │
│  Gate:  Auto-pass (no user gate here)                        │
│  ▼                                                             │
└─────────────────────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────────────────┐
│  STAGE C — TESTING                                          │
│  Agent: Tester                                                │
│  Action: Run tests, analyze results, report issues            │
│  Gate:  Auto-pass or fail (loop back to Coding if fails)     │
│  ▼                                                             │
└─────────────────────────────────────────────────────────────┘
    │ (tests pass)
    ▼
┌─────────────────────────────────────────────────────────────┐
│  STAGE D — REVIEW                                           │
│  Agent: Reviewer (can be same as Planner)                    │
│  Action: Code review — quality, style, correctness           │
│  Gate:  Pass or fail (loop back to Coding if fails)         │
│  ▼                                                             │
└─────────────────────────────────────────────────────────────┘
    │ (review passes)
    ▼
┌─────────────────────────────────────────────────────────────┐
│  STAGE E — HUMAN REVIEW                                     │
│  Human: Developer                                             │
│  Action: Review final code, accept or reject                 │
│  Gate:  Accept → commit & continue; Reject → loop back      │
│  ▼                                                             │
└─────────────────────────────────────────────────────────────┘
    │ (accepted)
    ▼
  Next function/struct/class (loop B→C→D→E) or COMPLETED
```

---

## Key Features

| Feature | Description |
|---------|-------------|
| **HTMX Frontend** | Lightweight, no build step, server-rendered templates with HTMX for interactivity |
| **Configurable Models** | Each phase (Planner, Coder, Tester, Reviewer) can use a different model |
| **Multi-Agent Architecture** | Dedicated agents for each phase with specialized prompts |
| **LM Studio First** | Starts with LM Studio, but interface is built for easy provider extension |
| **Language-Agnostic Tools** | Tool executor detects project type and runs appropriate commands |
| **Atomic Modifications** | Each agent modifies exactly ONE function, struct, or class per cycle |
| **No Authentication** | Local-only environment, no auth overhead |
| **Native Kotlin Future** | Architecture designed to support a Kotlin-native client (see `kotlin-native.md`) |

---

## File Structure

```
plan/
├── README.md                  ← This file (overview)
├── 01-architecture.md         ← System architecture & component design
├── 02-phase-flow.md           ← Detailed phase flow & state machine
├── 03-model-config.md         ← Model configuration system (LM Studio + extensible interface)
├── 04-dashboard.md            ← HTMX frontend design
├── 05-implementation-plan.md  ← Step-by-step implementation roadmap
└── kotlin-native.md           ← Future: Kotlin native app plan
```

---

## Quick Start (After Implementation)

```bash
# 1. Start the daemon with model config
mini-orca daemon --config config.yaml

# 2. Open the dashboard
open http://localhost:8080/dashboard

# 3. Enter your project goal → Review plan → Approve → Watch agents work!
```
