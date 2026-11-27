# Implementation start — 2026-11-27

User explicitly approved existing PRD/design/contracts and requested native Trellis implement/check agents. Existing task activated; no new task.

Source HEAD: ba39ec44e; branch codex/git-engineering-material-workflow. Pre-existing tracked changes belong to parent task 11-10 (prd/design/implement/task.json). Current task directory was untracked. Other untracked tasks, .impeccable and QA/release artifacts are outside this change and must be preserved.

Contract v1 is frozen as contracts.md. Backend owns domain, SQL/generated, services, server handler/router and task admission. Runtime owns pkg/protocol/material.go (freeze first), daemon and material CLI. Frontend owns core/materials and shared material views/locales. Main owns task/specs/built-in skill and integrated verification. Shared files outside ownership require an explicit handoff.

All remote mutations remain selected runtime actions; daemon observer is read-only. No new dependencies or real model/provider writes in this phase.
