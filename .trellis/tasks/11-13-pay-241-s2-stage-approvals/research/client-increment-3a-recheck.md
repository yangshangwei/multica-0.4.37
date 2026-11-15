# S2 client 3a independent recheck

Completed: 2026-11-14T05:14:38.950053+00:00

**No open findings in the reviewed client increment. C01–C07 are resolved; the reviewer found and fixed C08.** The client 3a boundary is ready for eventual joint integration. This does not accept all S2 work, A01–A12, or the parent task.

Source: implementation report HEAD `8e824b3035c4f17a9a9dfa0504f1294fbcc9b20b` plus its overlay and the three reviewer-edited Views files below. No Git commands were run. Wire SHA-256: `ecd9ca49c34b25bf71815e3a7edf95b1ccfa7cfe327b25a24d9f6e6f8134d129`. Section 13.11 changes A5 credential expiry only; no extra client behavior was needed.

## Findings (fixed)

### C01–C07: implementation corrections independently verified

**C01 — resolved.** Full and recovery-only records share the existing prefix. Denial removes private body/prose. The strict action union retains exactly the fields needed by the same matcher, and no write body is reconstructed. Reduced objects and stale full handles cannot POST during denial; conflicting stored reduced records also block full-body overwrite. Denied detail exposes original-key GET404/GET200 recovery, with all identity/logout cleanup preserved.
Source: `stage-intent.ts:33,163; stage-schema.ts:442,678; stage-actions.ts:264,285; stage-panel.tsx:366; collaboration-detail.tsx:70`.

**C02 — resolved.** R4 source workspace is explicit in keys. A workspace-scoped generation is advanced before cancellation/erasure across same-viewer/session sibling and cross-issue R4 entries. Older success and error are fenced before they can restore evidence or deny issue access. Other workspaces and issue identity survive. Source-dependent forms unmount/remount without their old source snapshots or unsent review text; already-anonymous records preserve their existing read/error state.
Source: `stage-source-cache.ts:15,37; stage-queries.ts:128; stage-code-forms.tsx:35,47`.

**C03 — resolved.** Reason-only return derives independently from source-readable approval eligibility. Proper-role/current candidate returns accept null expiry, null evidence and no dependencies; optional hidden references and approval remain blocked. The withheld form exposes no private evidence controls.
Source: `stage-code-presentation.ts:28,45; stage-actions.ts:76; stage-code-forms.tsx:40,47`.

**C04 — resolved.** R1 discovery and R4 must agree on exact candidate identity and current state before authorizing controls. Discovery changes refresh selected R4. Revocation remains separately available for readable superseded, unconsumed decisions.
Source: `stage-code-presentation.ts:29; stage-queries.ts:136; stage-code-forms.tsx:121`.

**C05 — resolved.** The existing explicit-workspace member query supplies real workspace_id/user_id/role fields. The handler writes them and the schema preserves them; no fixture-only capability is assumed. The account-scoped wrapper retains one matching viewer role. Owner/admin and a successful non-fetching read are required; preparation rechecks workspace/account/role. No wire changes.
Source: `stage-queries.ts:154; stage-actions.ts:159; stage-panel.tsx:301; api/client.ts:5328; api/schemas.ts:3868; server/internal/handler/workspace.go:498`.

**C06 — resolved.** Required parent/R1, P1/authority and R3 failures disable new writes and resends while preserving input and GET recovery. Successful refresh re-enables the retained form. C08 below corrects an over-broad resend lock introduced by this change.
Source: `collaboration-detail.tsx:110; stage-panel.tsx:94,137,310; stage-forms.tsx:166,288`.

**C07 — resolved.** The parser rejects a valid decision with a terminal revocation, and validDecisions defensively excludes any non-null revocation.
Source: `stage-schema.ts:179; stage-code-presentation.ts:47`.

### C08: pending policy request could never be explicitly resent

**File:** `packages/views/collaboration/stage-panel.tsx:310`.

**Issue:** The pending-record condition correctly disabled a new policy request, but the same `locked` value was passed to the resend button. Every pending full intent therefore disabled its own resend, including after GET404 with a healthy current owner/admin read.

**Fix:** `sendDisabled` now contains only current read/authority/storage/busy restrictions and unavailable P1 data. `locked` adds pending-intent presence for new writes. Reduced records remain GET-only. The canonical test at `stage-panel.test.tsx:396` proves an explicit second PUT uses the same method/path/idempotency key/body, GET404 sends nothing automatically, the replacement write stays disabled, and a failed read disables resend while GET remains usable.

**Additional test repair:** `stage-recovery.test.tsx:30` now checks absence of the actual label `Send the same request again`. Its old `/Resend/` selector could pass without inspecting the real control.

## Findings (not fixed)

None in this bounded client review. No server, wire, spec, dependency, platform or unrelated source edits were made.

## Verification

- Input manifest: **26/26 source hashes** and **21/21 log hashes** matched before reviewer edits. The original check/fix reports remain immutable. Final source hashes and all new log hashes are recorded in `client-increment-3a-recheck.json`.
- Independent Core run: **12 suites, 292 passed**, no failures/skips. The implementation report’s earlier 291 count preceded its last source-cache test; the final source now has 292.
- Independent Views/detail/locales run: **6 suites, 127 passed**, no failures/skips, before C08. After the small Views correction, both affected suites passed **30 tests**, with no failures/skips. No combined all-Views count is inferred.
- Core/Views typecheck and scoped lint pass with zero code warnings. Final changed Views files were linted and typechecked again after the last assertion correction. Existing pnpm configuration warnings remain unchanged.
- C08’s first red attempt used an incorrect selector; it is retained but is not product-failure evidence. The corrected **red2** failed on the disabled button. The first green attempt reached the correct resend but compared an unrelated per-attempt `X-Request-ID`; that assertion was narrowed to operation identity. **green2** passes all 30 tests.
- No API, database, browser, model, platform launch or full build ran.

## Actual API-shape check

C05 follows `memberListOptions` → `ApiClient.listMembers` → `MemberWithUserSchema`. `ListMembersWithUser` actually returns `workspace_id`, `user_id`, and `role`; the wrapper does not depend on a mock-only field. The server’s stage-confirmation marshaler also emits explicit nulls for fresh A6 `parent_candidate_id` and returned A7 `expires_at`, matching the frozen client variants. These were source checks, not server runtime acceptance.

## Follow-up ownership

Root owns spec synchronization for recovery-only records, full account cleanup, workspace-scoped source erasure, and joining future private T4 queries to that fence. Client3b still owns M1 collaboration completion, generic completion blockers, activity/notifications, and T4 detail after the server freeze. Production Web/Desktop and full joint S2 acceptance remain pending.
