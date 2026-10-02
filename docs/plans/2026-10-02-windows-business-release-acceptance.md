# Windows business and release acceptance implementation plan

**Goal:** Execute the user's approved business integration, retained-state upgrade and release-readiness acceptance, recording actual passes and missing external prerequisites.

**Architecture:** Reuse hosted Windows and the existing accepted candidate. Run a disposable native PostgreSQL cluster and real Go server on that runner, then drive installed Electron with Playwright and a contained synthetic provider. Reuse existing artifact publication/verification against an isolated local Nginx service. No new application dependencies, real model calls or production publication.

**Choices:** Native Windows same-host backend avoids remote tunnels and Linux service-container assumptions. A mocked renderer cannot prove real daemon integration. The original macOS harness supplies protocol patterns but its POSIX filesystem/process checks must not be copied as Windows evidence.

## 1. Business integration

- Add Windows CI preparation under `apps/desktop/scripts/` for a task-owned PostgreSQL cluster, migrations and password/managed/platform server configuration.
- Add an installed-app harness with explicit executable, loopback API, disposable runner, isolated home/userData and native synthetic provider boundaries.
- Exercise real UI login, installation enrollment/workspace binding, successful task/result, administrator task cancellation and websocket reconnect. Preserve reports without credentials.
- Validate fixtures locally, run Windows CI, fix discovered issues and reject missing/skipped scenarios.

## 2. Retained state

- Published v0.5.1 predates password login and managed identity; do not claim it can generate this state.
- Use previously accepted 0.5.2-rc.11.1 (run36996436086, verified SHA256) as the feature-capable installed baseline, then upgrade to a new numerically higher candidate.
- Baseline login/configuration must be created by the actual installed app. Stop owned processes before upgrade; reopen the same profile and assert retained session, configuration, installation identity and owned workspace/task history.
- Keep stable v0.5.1 installer lifecycle evidence separate from feature-capable retained-state evidence. Pending task semantics must be tested explicitly or remain unverified.

## 3. Release rehearsal

- Publish exact verified candidate bytes only to ignored isolated storage using existing update-artifacts with prerelease opt-in.
- Start task-specific loopback Nginx, verify metadata, SHA512, HEAD/Range and container restart persistence, then stop the owned service and retain artifacts.
- Inspect signing/environment configuration by names only. Real signing/offline trust requires actual certificate/trust material; never replace it with self-signed evidence or mark unsigned acceptance as signed.
- Do not merge main, tag a release or deploy production as part of this acceptance.

## Verification and records

Run focused regression tests, lint/typecheck for affected code, native Windows job and inspect raw reports/artifact hashes. Update S07 evidence/checklist/handoff with exact tested source commit and unresolved gates. Subsequent documentation-only commits are not the candidate's tested SHA.
