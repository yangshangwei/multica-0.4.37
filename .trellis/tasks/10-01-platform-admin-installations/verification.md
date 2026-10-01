# S02 implementation and integration evidence

Worktree: `/Volumes/artisan/code/2026/multica-platform-admin`, branch `feat/platform-admin-console`, committed baseline `fa28e28a2`. Updated 2026-10-02. This slice is implemented and verified, including the final client review fixes and refreshed production screenshot. S04/S06/S07 are not complete.

## Implemented contract

- Ed25519 enrollment/bind/renew signs the original canonical server bytes. Persistent hashed challenges, account-version locks, deployment/organization/workspace/daemon fences, encrypted lost-response credential recovery, and atomic audit prevent replay from becoming a second binding or a credential upgrade.
- A second same-authority profile receives its own scoped credential without replacing the binding or interrupting active work. Rebinding/reactivation preserves monotonically increasing namespace epochs. Revoked namespace history cannot fall back to PAT/legacy registration, heartbeat or claim.
- Canonical installation identity lives at `~/.multica/management/{deployment_id}/installation.json`; the user-specific daemon UUID is derived from a separate stable namespace. Global PAT is only discovery; bound HTTP/WS/task credentials remain workspace scoped. See the client handoff for private persistence, IPC, drain and recovery details.
- Signed heartbeat sequence/boot/account-version checks distinguish client activity from daemon reachability. Metadata proof uses a separate signing key and cannot authenticate a user; the business request still needs its ordinary credential. Only validated proof writes task submission identity. Manual Web retry has no installation origin; automatic continuation preserves its original origin.
- Enrollment returns at most100 binding hints and declares truncation. `GET /api/installations/{id}/binding?workspace_id=...&daemon_id=...` resolves an omitted namespace using full human password authentication, matching current installation proof and current membership. Only explicit `binding:null` means absence. Revoked history and old account versions remain available for recovery. At most100 concurrently desired workspaces remain the client handoff bound.
- Read-only installation list/detail/unassociated-runtime views use exact verified associations, controlled metadata, bounded signed cursors and independent client/daemon/readiness axes. Unknown, stale and unavailable are distinct.

## Verification

All Go database tests use `multica_platform_admin_s01_test`, isolated fixtures and the agent-CLI guard. No real provider executable or model account was used.

- Actual `NewRouter` HTTP scenario covers enrollment, scoped workspace discovery, binding, lost-response replay beyond consumed challenge expiry, same-authority multi-profile coexistence, renewal, and binding-bound MAT revocation. It also verifies full HTTP quick-create attribution: signed Desktop proof persists origin, ordinary Web and forged installation-ID/body fields stay null, and execution identity is not fabricated at submission.
- Targeted binding lookup tests cover JWT/proof requirement, PAT rejection, mismatched installation, a deliberately omitted binding beyond100 hints, revoked namespace epoch recovery and cross-workspace null response.
- Final parent race command: `go test -race ./cmd/server ./internal/installation ./internal/auth ./internal/service -run 'Test(Managed|Installation|Platform|Admin|Manual)' -count=1`. Passed: router26.856s, auth1.679s, service99.758s. The name filter matched no protocol-package tests; a separate full `go test -race ./internal/installation -count=1` passed1.413s.
- `go vet ./internal/installation ./internal/auth ./internal/service ./internal/handler ./cmd/server` passed. Runtime enforcement separately records handler/daemonws race coverage and the later revoked-history regression in [runtime-enforcement.md](runtime-enforcement.md).
- Parent current core admin/metadata tests:12 files/67 tests; views admin tests:9 files/17 tests. Core/views/Web typechecks passed. Client final pre-review checks:82 Desktop files/899 tests, both Desktop typechecks, scoped ESLint and affected Go race/vet passed; subsequent review fixes must be rechecked.
- `pnpm knip --reporter json` still exits1 on the same11 findings recorded from original main:9 unused files,1 dependency,1 devDependency. Each finding's file/name was compared with `/tmp/platform-admin-baseline-knip.log`; none belongs to this change.
- S02 production HTTP enrollment/bind/heartbeat/register/deregister/read UI smoke passed1 test in2.7s on source fingerprint `67dbc412bec52dd0282d75c6ddd6fab9e267a21ed35836ac1f9292b7fb5e8f1b`, Web build `8WNMN1kDY94T4HvL_O04V`. Visual inspection found mobile UUID overlap; the local fix and bounding-box regression await the refreshed build. This older snapshot does not prove later client fixes.

## Final review and production closure

- Generic managed recovery no longer enters the legacy exhausted-PID-deferral fallback. Main start/server-switch paths and CLI background/foreground reservation use conservative process evidence. Private profile locking serializes PID publication; cleanup cannot remove a live process or successor's PID. Reviewer rechecked these exact paths after the fixes and found no remaining concrete blocker.
- Final frozen-source Desktop:83 files/908 tests; scoped ESLint, Node typecheck and diff checks passed. Renderer typecheck had passed and subsequent changes were Main/CLI only. Full guarded Go race: daemon63.340s, CLI3.736s, internal/cli cached; affected vet passed. Actual manager and foreground/PID wiring regressions are included.
- Real mixed-language lock acceptance on macOS used4 Node and4 Go subprocesses on one private directory. All8 increments persisted, no overlapping critical section and no remaining ticket files,1.175s. Harness and result: `.omx/reports/platform-admin/mixed-lock-interop.{mjs,json}`.
- Final production source `2d7a14b1612dab58d77a70352a12d9f8ce7270a2a3d9af56ea47e9c02fb87905`, Web build `4HqC2LA83Zf-dnxnfTGP6`, API PID1467/Web PID2095. Both provenance records share that source; `/health` and `/api/config` verified task-owned password/managed mode. Build/provenance: `.omx/reports/platform-admin/final-s02-s03-s05/`.
- S02 read smoke passed1/1 in2.7s on this snapshot, including the mobile UUID text-vs-cell geometry assertion. Fresh desktop/mobile/detail screenshots were inspected: identifier wraps inside its cell without status overlap; visual verdict93/pass. S03/S05 were also rerun on the same snapshot:2/2 passed in7.84s, no retries/skips/failures.
- Migration package tests passed3.205s. Core/views ESLint passed; JSON locales are not linted by that ESLint configuration, so both were parsed and all236 leaf keys compared for parity.

## Remaining evidence boundaries

- The independent client review findings above are closed by source/wiring regressions and re-review. Full installed Electron→daemon lifecycle acceptance remains part of S07.
- Windows installed-app/ACL/process behavior, 1000-installation load, million-execution pagination, rollout and complete control/alert acceptance belong to S07; none is claimed here.
- Client evidence: `.omx/reports/platform-admin/client-s02-handoff.md`. Read model: [read-model-verification.md](read-model-verification.md). Production evidence: `.omx/reports/platform-admin-installations-read/`. Parent logs are preserved under `.omx/reports/platform-admin/batch-s02-s03-s05/final-checks/`.
