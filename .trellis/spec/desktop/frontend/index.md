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
| [Quality Guidelines](./quality-guidelines.md) | Code standards, forbidden patterns | To fill |
| [Type Safety](./type-safety.md) | Type patterns, validation | To fill |
| [Intranet Updates](./intranet-updates.md) | Release publication, Nginx static download page, first-install data contracts and installed-client validation | Filled |
| [Close Behavior](./close-behavior.md) | Windows/Linux close button semantics: minimize-to-tray vs quit vs ask; prefs, tray, prompt, settings | Filled |
| [Renderer File Access](./renderer-file-access.md) | Native local-file denial for untrusted HTML and real Electron verification | Active |
| [Connection Recovery](./connection-recovery.md) | Bounded boot requests, truthful retry state, persistent server editing and child-window handoff | Active |
| [Daemon Diagnostics](./daemon-diagnostics.md) | Independent CLI/reconnect failures, safe IPC diagnostics and localized initial recovery steps | Active |
| [Desktop Core Interaction](../../views/frontend/desktop-core-accessibility.md) | Tab reorder alternatives/current destination, interrupted reduced-motion springs and native acceptance | Active |
| [Settings Recovery](../../views/frontend/settings-recovery.md) | Named desktop preference controls, stable update announcements and shared settings verification | Active |
| [Managed Installations](../../server/managed-installations.md) | Shared identity, scoped credentials and Main/daemon protocol boundaries | Active |

---

## How to Fill These Guidelines

For each guideline file:

1. Document your project's **actual conventions** (not ideals)
2. Include **code examples** from your codebase
3. List **forbidden patterns** and why
4. Add **common mistakes** your team has made

The goal is to help AI assistants and new team members understand how YOUR project works.

---

**Language**: All documentation should be written in **English**.
