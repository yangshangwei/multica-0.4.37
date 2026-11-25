---
status: pass
blocking_findings_open: 0
scope: planning-artifacts-only
execution_clearance: not_granted
reviewer: acceptance_plan_review
date: 2026-11-24
---

# Independent acceptance-plan review

The final planning artifacts cover 25 business/risk domains and 11 environment lanes. The plan is ready for the coordinator's planning transition. This review does **not** admit a launcher, database, browser, native application or model account: their declared preflight and independent execution review remain required.

## Scope and evidence

- Read the complete saved native hook context at `/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/hook_outputs/01a20adb-fbd4-7fd1-8495-d079ae02a141/5da4a3b1-72f7-4fb5-8348-caaf0854536e.txt`, including the current server isolation, verification environment and Web E2E contracts.
- Reviewed `prd.md`, `test-plan.md`, `design.md`, `implement.md`, both curated manifests, and the Web/platform/backend research. The coordinator appended backend research to the manifests after the initial snapshot; the final manifest hashes below include that addition.
- Checked concrete existing Chat, cookie/CSRF and desktop startup code rather than inferring behavior from the matrix names. No product/test source was changed, and no suite, SQL, service, application or provider was executed.

## Findings resolved by the coordinator

### 1. Chat was missing from the named coverage ledger

The original T01–T24 matrix did not explicitly own Chat. Existing coverage is substantial and cannot be hidden under generic Agent or realtime rows: `e2e/chat-actor-picker.spec.ts:139`, `e2e/chat-attachments.spec.ts:82`, and `e2e/chat-recovery-branches.spec.ts:120,175,201,224,250` cover discovery, upload linkage, failed creation/send, duplicate pending submit and a late response after a session switch. Server-side cancellation/input ownership is separately covered by `server/internal/handler/chat_archive_cancel_test.go` and `chat_input_ownership_test.go`.

The coordinator added T25, synchronized PRD R03 and the final execution/report ledger, and included pending/retry, attachment failure, cancellation, session/archive/delete and draft isolation. Verified at `test-plan.md:43`, `prd.md:19`, and `implement.md:88`.

### 2. Cookie authentication and Origin refusal needed explicit assertions

Generic authority checks can report a fixture's absent CSRF cookie as a product authorization refusal. The actual router regression `server/cmd/server/platform_admin_auth_test.go:228–244` supplies missing, forged and another session's CSRF values and asserts no operation was written; `server/internal/auth/cookie_test.go:62–73` requires both auth and CSRF cookies. The E2E environment contract also requires the real login cookies and `X-CSRF-Token` before password control writes.

T02/T15 now explicitly distinguish successful cookie+CSRF and machine-credential paths, invalid/missing CSRF, invalid HTTP/WS Origin, and no protected side effects. The execution checklist requires those probes. Verified at `test-plan.md:20,33` and `implement.md:79`.

### 3. A private native HOME does not isolate macOS protocol registration

`apps/desktop/src/main/index.ts:648–668` redirects packaged home, then invokes `app.setAsDefaultProtocolClient`, then requests the single-instance lock. The system registration is not made private by HOME/userData. The current spec explicitly requires exact NSWorkspace default/set preservation and task-prefix-only registration cleanup (`.trellis/spec/web/frontend/e2e-run-environment.md:366–378`).

L7 now requires a pre-launch snapshot of the exact original default application URL and existing same-bundle registration URLs, a task-owned stable application path, cleanup limited to newly added task paths, and exact restoration verification. If that boundary cannot be established, the native subrun remains BLOCKED and must not start. Verified at `design.md:17`, `test-plan.md:87`, and `implement.md:71–72`.

## Remaining execution conditions and advisory

No blocking **planning** findings remain. The following are declared execution conditions rather than passed checks:

1. The new backend/overall launcher still needs its own source/tool/configuration/resource review and fresh clearance before any test or SQL. Standard `scripts/test-go.sh` serializes a shared URL, so the planned per-package fresh database adapter remains necessary. The newly curated backend research identifies the old simplified adapter's unvalidated `BACKEND_PACKAGE_CHILD=1` forwarding and incomplete BuildEvent shape checks; these must be addressed in that separate executable review, not grandfathered from an earlier green run.
2. L0 names both Go normal and race modes (`test-plan.md:49`), while the concrete entry in `implement.md:35` emphasizes `scripts/test-go.sh --race`. The coordinator should make the normal/race selection explicit in the new runner/allocation and reconcile each mode separately. A race result must not be labeled as a separately executed normal run.
3. The 538-case collector is inventory only. Every actual lane must collect again under its compatible configuration and reconcile named terminals, runtime skips and prerequisites. The current 70 declared skips are not accepted passes.
4. Runtime-output's exact archived-task identity, password/namespace/host compatibility, signed managed enrollment, browser cache binding, current desktop/CLI assets and final source/resource audit are explicit planned gates; none is established by this plan review.
5. Native iOS, Windows native installation, signed/updater phases, commercial providers and distributed scale retain their stated evidence boundaries. Recording BLOCKED/NOT_RUN is valid reporting; it does not make the corresponding product scope green.

## Review checks

| Check | Result |
| --- | --- |
| Goals, acceptance criteria and report semantics | PASS for the planning artifacts |
| Normal/refusal/boundary/concurrency/recovery breadth | PASS; T01–T25 own the identified domains |
| Auth/gate lane compatibility and evidence boundaries | PASS for the declared design |
| DB/HOME/temp/URL/resource/source identity requirements | PASS for the declared gates, not runtime proof |
| Curated manifests | `task.py validate` exited 0; final implement has 7 valid entries, check has 6 |
| Lint | NOT_RUN; explicitly outside this planning-only dispatch |
| TypeCheck | NOT_RUN; explicitly outside this planning-only dispatch |
| Tests / SQL / services / browser / native application | NOT_RUN; no runtime acceptance claim |

## Reviewed artifact snapshot

These SHA-256 values bind this planning review to the files observed after the coordinator's fixes. Later edits require the coordinator to assess whether they materially change the plan.

| Artifact | SHA-256 |
| --- | --- |
| prd.md | `8538a8c1aec527f18c5990e8c0d914434c61e036d7714653619b18e53ecf5ab6` |
| test-plan.md | `16833d4a4c30a81dff19bdb3641d98fea84e0543ae2ba0e7fff4d2cb187074a4` |
| design.md | `ffc4e4f51607d8084736682464d8055b96d9254320fe087019df90af6e87475b` |
| implement.md | `675e9c4ad54e3fc5c4ccb98a01979fd3fc65716422937e2b139b86ea3868f4da` |
| implement.jsonl | `6678c7a91e078d0912ee21c259c2b119d8f7a7ab4f0cb99fe9efc1713dc5f17d` |
| check.jsonl | `60555b735d29933f6f9f6ed1e416f220dad5d6db38a6232d2ea46c7d984dbdc8` |
