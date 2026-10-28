# Frontend Development Guidelines

> Best practices for frontend development in this project.

---

## Overview

This directory contains guidelines for frontend development. Fill in each file with your project's specific conventions.

---

## Guidelines Index

| Guide | Description | Status |
|-------|-------------|--------|
| [Directory Structure](./directory-structure.md) | Module organization and file layout | To fill |
| [Component Guidelines](./component-guidelines.md) | Component patterns, props, composition | To fill |
| [Hook Guidelines](./hook-guidelines.md) | Custom hooks, data fetching patterns | To fill |
| [State Management](./state-management.md) | Local state, global state, server state | To fill |
| [Global scope navigation](./global-scope-navigation.md) | Fixed Workspace/Collaboration switch, identity-owned resume and explicit focus | Active |
| [Quality Guidelines](./quality-guidelines.md) | Code standards, forbidden patterns | To fill |
| [Type Safety](./type-safety.md) | Type patterns, validation | To fill |
| [ApiClient authEpoch](./api-client-auth-epoch.md) | Isolating a stale 401 from the current session | Active |
| [Collaboration execution protocol](../../server/collaboration-targets.md) | v5 channel/input/reader history, exact hold/control receipts, original-key/access-epoch recovery and gated execution | Direct/squad foundation; full product pending |
| [Collaboration recorded runs](../../server/collaboration-records.md) | Private record paging, exact usage, page-chain integrity, child hints and compatible run anchors | Current R51/R52 increment |
| [Collaboration trigger previews](../../server/collaboration-preview-wakeup.md) | Additive grouping facts, strict peer validation and current-draft preview identity | Focused preview increment |
| [Collaboration workbench](./collaboration-workbench.md) | Account reads, scoped creation/files, resource forms, schema boundaries and original-key recovery | Gated Web/Desktop workflow; full product pending |
| [Settings Recovery](../../views/frontend/settings-recovery.md) | Exact-Query denial and cancellation rollback, PAT parsing and one-time secret ownership | Active |
| [Connection Recovery](../../desktop/frontend/connection-recovery.md) | Recovery request deadlines/cancellation, transient auth metadata and workspace query errors | Active |
| [Iteration Operations](./iteration-operations.md) | Complete previews/activity, frozen progress and scope semantics, recovery, permission epochs and routing | Active |

---

Project execution defaults: see [the shared contract](../../server/project-execution-squad.md).

## How to Fill These Guidelines

For each guideline file:

1. Document your project's **actual conventions** (not ideals)
2. Include **code examples** from your codebase
3. List **forbidden patterns** and why
4. Add **common mistakes** your team has made

The goal is to help AI assistants and new team members understand how YOUR project works.

---

**Language**: All documentation should be written in **English**.
