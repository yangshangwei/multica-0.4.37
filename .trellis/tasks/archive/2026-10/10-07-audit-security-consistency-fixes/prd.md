# Audit security and consistency fixes

## Goal

Fix the six confirmed audit findings without broadening the product or changing unrelated user work. Protect local files and machine-credential boundaries, and preserve users' edits across concurrency and project navigation.

## Background

The user requested remediation after a report-only audit and explicitly approved task creation and planning on 2026-10-07. The planning baseline is `4a9be02e5` on `codex/projects-p1`. Audit evidence is preserved in this task's research directory.

## Requirements

- R1 / P1: Untrusted HTML fences and attachments displayed by Desktop must not read local files. Sources: `apps/desktop/src/main/renderer-web-preferences.ts:37`, `packages/views/editor/code-block-iframe.tsx:48`. Preserve ordinary previews and interactive HTML where a verified, bounded isolation mechanism permits it.
- R2 / P1: Browser plugin bridge calls must not convert task or other machine credentials into human member authority. Sources: `server/internal/handler/plugin_action.go:127`, `server/cmd/server/router.go:1569`. Keep legitimate browser/PAT callers and dedicated plugin bearer/callback routes compatible.
- R3 / P2: A remotely refreshed iteration revision must not authorize overwriting fields from an older local edit baseline. Source: `packages/views/iterations/iteration-form.tsx:74`. Preserve user input and expose genuine conflicts.
- R4 / P2: Same-tab cached project A-to-B navigation must not retain A's composer, preview, cursor or correction target under B. Sources: `packages/views/projects/components/project-detail.tsx:601`, `project-progress.tsx:204`. Preserve each project's own draft.
- R5 / P2: Concurrent project-resource create/update requests must preserve one local directory per project/daemon. Source: `server/internal/handler/project_resource.go:515,622`. Conflicting requests must fail without creating ambiguous execution configuration.
- R6 / P2: Concurrent partial resource edits must not overwrite omitted fields using a stale server snapshot. Sources: `server/internal/handler/project_resource.go:590,730`, `server/pkg/db/queries/project_resource.sql:53`. Preserve old-client label/ref compatibility.

## Acceptance Criteria

- AC1 / R1: A fake known-path local-file probe is rejected in real isolated Electron using the production rendering path. Ordinary rendering and any retained script/network behavior have positive controls. Cover inline, attachment and full-page entry points.
- AC2 / R2: Authenticated task/machine tokens are rejected at all session bridge aliases before reads/writes or hook invocation. Human controls and dedicated plugin bearer/callback behavior still work in supported auth modes.
- AC3 / R3: Editing at revision N, receiving N+1 remotely, then saving cannot silently replace N+1 fields. Successful save advances the baseline without losing edits.
- AC4 / R4: Cached same-tab A-to-B navigation never previews/publishes A's body under B; returning to A restores its appropriate saved draft.
- AC5 / R5: Deterministic concurrent create/update tests permit at most one row per project/daemon, with a conflict response for the loser.
- AC6 / R6: Concurrent execution-ref change and label/position-only patch preserve both changes. Existing rename and execution-capability tests remain valid.
- AC7: Narrow regressions fail before fixes and pass afterward; relevant lint/typecheck, TypeScript tests, Go vet/race and real Electron security verification pass using isolated services and no real agent accounts.

## Scope and Constraints

- No new dependency, unrelated cleanup, production deployment, release packaging or edits to existing user modifications.
- No database foreign keys or cascading actions. Prefer current transaction/locking helpers. Any required index follows concurrent single-statement migration rules.
- Shared business behavior remains in shared packages; platform security stays in the appropriate platform boundary.
- Implementation follows the reviewed design and implement.md; continuation was authorized on 2026-10-07 after the user received the current planning status.

## Resolved Research

- Desktop uses a native session file-request boundary supported by isolated experiments. Retain interactive previews; validate the production path before claiming closure. A broad protocol migration is outside this task.
