# Acceptance environment and evidence design

## Source boundary
Use a detached test worktree at the exact target HEAD, retaining the source branch and initial clean status. Recheck target HEAD/status at closure. Test-only additions, if needed, are inventoried independently and do not import other worktree changes.

## Isolation
Use the existing dev-env/check launcher and a task-specific MULTICA_DEV_HOME, profiles/workspaces/uploads/temp roots. The launcher allocates and records ports, a fresh API database and separate Go database. Read only the local database endpoint from existing configuration; all account/crypto/auth settings are synthetic. Never stop an unknown listener.

## Modes
1. Legacy auth baseline through make check: production Web, static checks, TS tests, guarded Go race tests/vet and Chromium E2E with zero retries.
2. Password/admin/knowledge: separate task configuration and fresh database; validate /api/config and gate state before named suites. Password specs run in bounded batches respecting registration budgets.
3. Native Electron: matching build and task-owned home/renderer, no operator profile. Deterministic host fixtures explicitly cover protocol/control, not model behavior. Installed app/OS/provider prerequisites are separate verdicts.

## Observations
Named cases capture UI plus API/persistence assertions already provided by TestApiClient fixtures. Logs, JSON reporters, traces/screenshots where available, running.json/build ID/source fingerprints and sanitized configuration snapshots live under .omx/reports/branch-e2e-822c15785-20261101. Summary artifacts live in the Trellis task. Do not retain tokens/cookies/passwords in public evidence.

## Faults / recovery
Prefer existing bounded fault-injection cases: lost responses, duplicate intent, optimistic rollback, revoked authority, websocket reconnect, version conflict, stop/restart/resume. Distinguish mocks/HTTP integration from real UI and deterministic process tests.

## Verdict integrity
Keep every attempt under a unique run ID. A rerun adds evidence and never replaces a first failure. Aggregate per case and per actual execution; skips are not passes. Historical known defects are checked for ancestry and current reproducibility.

## Cleanup
Stop only task-owned services/containers, verify listener/process exit and gate cleanup. Retain failed databases where needed for diagnosis and report their identity. No deletion of unrelated databases, processes, worktrees or accepted evidence.
