# Backend integration evidence — 2026-11-27

Work performed only in `/Users/artisan/.codex/worktrees/material-repository-initialization/multica-0.4.37`, based on merged runtime implementation `73e187139`. This is implementation-lane evidence; independent whole-scope review remains pending.

## Delivered

Public contract is frozen in contracts.md and `internal/material/repository.go`: initialization GET, exact request/version/settings-version check/retry/maintenance writes, owner/admin human decisions, candidate add/modify/preserve/conflict bytes, independent capabilities and retained previous-ready summaries. Current bundle maintenance is a no-op; unsupported version is refused. Runtime extension uses explicit repository target, real repository/revision IDs and original queue/carrier/claim/challenge/permit/receipt system. No fake material row, extra queue, App integration or server Git credential path was added.

Migrations 914–919 create root/revision/decision records and nullable typed job/operation targets with separate concurrent unique indexes. Down guards preserve root, revision, decision, job and operation history even if a root row is absent. Project/workspace delete guards now include initialization-only history.

First valid enabled configuration records one inspect intent. Complete compatible trees establish ready through actual claim/observation; missing files freeze independent create-only candidate, then selected runtime publication, manager review, audited decision synchronization, merge permit and exact merged-tree proof. New decisions on a previously synchronized candidate create a fresh decision-bound attempt. Recovery preserves the original operation/branch; absent publication/audit or an observed unmerged review resolves no_effect before a deliberate new write. Recheck cannot abandon an unresolved mutation. Settings, runtime, generation, latest approval and preflight details remain current-authority fences.

Ordinary drafts stay available. Ready revisions, queued operations, claim/start, permits, plans and current plan authority require current initialized binding. Ordinary runtime operations freeze accepted protected-rule proof and require current-target protected-file readback before model launch; normal business additions do not invalidate that proof. Complete rule drift commits a blocked root and immutable negative receipt, acknowledges replay safely, and leaves launch denied. Original ready history remains visible. The runtime owner handles local abort on the accepted negative observation.

## Files owned

New public domain types, `internal/service/material_repository.go`, `material_repository_operation.go`, `material_repository_test.go`, `internal/handler/material_repository.go`, `pkg/db/queries/material_repository.sql`, generated output, migrations 914–919 and owning migration regression. Existing material/service/plan/operation/claim/start/capability/router code gained the typed-target and readiness boundaries. Existing material fixtures now establish ready through service claim/readback instead of bypassing the new gate; ordinary regression assertions scope their own task history. Runtime/protocol/provider/kernel and shared UI remain owned by other lanes.

## Verification and environments

Fresh independently allocated service/router/migration databases are recorded in `/tmp/material-init-backend-fcesw4c9/{service,router,migrate}.json`. Database identity was checked before migrating. Private credentials remain outside the repository. No concurrent suites shared a package database. Failed run logs were retained, and narrow reruns used the same owning package database; these are targeted regressions, not full default acceptance.

| Run | Evidence | Result |
| --- | --- | --- |
| First integrated service suite | service-1.log | 24 failures retained: raw JSON digest instability, old fixture no-init assumptions, expected task counts |
| Second integrated suite | service-2.log | 3 failures retained: initialization history count, expected gate error, runtime change needs reinspection |
| Corrected normal material suite | service-3.log | 34 top-level passed |
| Expanded material + ordinary claim/batch/finalization/start race | service-race-1.log | 53 top-level passed |
| Additional initialization negative/authority/history race | repository-tests-4.log | 5 top-level passed |
| Router material host boundary + existing task capabilities race | router-race-1.log | 6 top-level passed |
| Actual migration upgrade / orphan-history downgrade refusal | migrate-1.log | 1 top-level passed |
| Final service race after recovery/approval and freshness corrections | service-final-race.log | Pending at handoff; coordinator must read completion |
| Final vet service/handler/router/migrate | vet.log | Pending at handoff; coordinator must read process completion |

SQL generation and service/handler/server compilation passed during implementation. Final `make sqlc`, vet, diff-check were launched together before handoff; their completion must be checked separately from the earlier pass.

## Acceptance coverage mapping

- A1: configure exactly one inspect, compatible readback ready, same-bundle maintenance creates no job, duplicate request/body refusal; full missing initialization and reapproval are tested.
- A2/A3: named conflicts/preserved bytes exposed, pure kernel remains authoritative; incomplete/symlink/foreign/stale receipt cannot establish ready. Existing compatible adoption uses actual queue claim/receipt.
- A4: manager-only commands and machine refusal, stale head/manifest/version/settings decisions, current runtime/generation/approval gates, original operation replay. Real router host challenge/task-token boundary retains its regression.
- A5: complete final publication/tree validation, provider merge timestamp and original permit/decision identity; exact publication and merge observation replay tested. Unknown mutation recovery code preserves original intent; independent checker should extend interrupted-init recovery adversarial cases.
- A6: same operation/queue/carrier and owner binding, no App/credential lookup, null ordinary material IDs for repository target.
- A7: delegated to existing materialrepo catalog tests and runtime kernel maintenance tests.
- A8: separate maintenance candidate with known before/after catalog changes, no arbitrary patch upload; ordinary exact path restrictions unchanged. Current supported target is exposed explicitly, future unknown bundles refuse.
- A9: Go evidence above; runtime composite, frontend, build, browser/Electron and independent final check have separate owners and are not claimed by this report.

No live model or remote repository write was performed by this lane. Final independent review and final source/build validation remain required before completion/merge.
