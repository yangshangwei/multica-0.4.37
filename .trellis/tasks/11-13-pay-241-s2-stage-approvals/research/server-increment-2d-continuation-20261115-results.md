# S2 server increment 2d continuation — 2026-11-15

**Intermediate green implementation handoff only. S2 and A01–A12 are not accepted or complete. Independent checking is pending the root session. Project `activation_available` remains `false`.**

## Frozen input and safe LIVE merge

- Source: `/Users/artisan/.multica/acceptance/pay241-20261115-continuation/server-2d/frozen-green-r03/source`
- Manifest: `/Users/artisan/.multica/acceptance/pay241-20261115-continuation/server-2d/frozen-green-r03/manifest.json`
- Manifest SHA-256: `3e4bc0270a1e675ebb0a9cae4e479853393c6d796266d049f4e07f1728a3e7b3`; 4,954 files; final hash audit has zero mismatches.
- Use `frozen-green-r03` for matching builds/checks, not the preserved red `frozen-r01` or the `frozen-r02` test-draft intermediate.
- Inherited source: `/Users/artisan/.multica/acceptance/pay241-20261114-resume/s2-server-inc2d/source`. Checked 2c basis: `/Users/artisan/.multica/acceptance/pay241-20261114-resume/s2-server-inc2c-followup-check/source`.
- Original draft archive: `/Users/artisan/.multica/acceptance/pay241-20261115-continuation/server-2d/input/draft2d-before.tar`; SHA-256 `d2abae204c49700b45fba5a009ef71c20945e9ba015f620b83f12127f293cbc6`. Before-edit LIVE bytes and hashes are retained in `/Users/artisan/.multica/acceptance/pay241-20261115-continuation/server-2d/input/live-before` and `/Users/artisan/.multica/acceptance/pay241-20261115-continuation/server-2d/input/live-before-manifest.json`.
- LIVE contains all 42 explicit checked2c→draft2d `apply_patch` changes plus this batch's four focused product-file fixes and one owned regression-test file: 44 server paths total. No shared-hunk conflict occurred; merge exit is 0. Mixed `task.go`, `issue.go`, router and lifecycle peer content was not whole-file copied or replaced. Knowledge settings/routes/hooks, unrelated changes, and peer migrations remain.
- Minimal LIVE patch: `/Users/artisan/.multica/acceptance/pay241-20261115-continuation/server-2d/patches/live-final-green-r03.patch`; SHA-256 `bbcab9a1267df5c7e56c8bea710b1cef4d10f0c20441d8c42e820002a9308e0e`. Initial merge log: `/Users/artisan/.multica/acceptance/pay241-20261115-continuation/server-2d/logs/live-merge-r01.log`; SHA-256 `d9ef9af2f5dcbd9bf678992e0798cb890d04e7cdae270124b99729a2f5249ba2`.
- No Git commit/branch change, task-state edit, Core/Views/E2E edit, new task, migration execution, runtime/provider start, pool/timeout relaxation, or unrelated F2 cleanup change.

## Actual red → green

| Selection | Exit | Named pass / fail / skip | Raw JSONL SHA-256 |
| --- | ---: | --- | --- |
| `continuation-red-r01` | 1 | 8 / 3 / 0 | `7b8e57411989498f56466bd364b5486c3967f779adbbe234dc831c54d3812f24` |
| `continuation-green-r01` | 0 | 11 / 0 / 0 | `83f3f4f195f9221ce704ae9f180d1de174927401264505e64dfd614fa0d07930` |
| `completion-baseline-r02` | 0 | 16 / 0 / 0 | `a284464cc731426e450922bcbfc683833f7109bf753befcf0d8709e737d6770a` |
| `draft-admission-r01` | 1 | 3 / 2 / 0 | `0188dcebefebbcbd805a73182c7c7ee71e476c1951a5b799d16fad445cc40941` |

Named counts include parent tests and subtests. `completion-baseline-r02` includes the `continuation-green-r01` selection; 11 + 16 is **not** an independent-case denominator.

The three actual RED failures are now green:
- `TestStageCompletionGuardRequiresPreparedTransaction`: Reject nil actual transaction and invalid persisted issue identity; guard transaction type/comparability/reuse checks remain explicit.
- `TestStageCompletionGuardCannotBeOmittedForManagedRoot`: Recheck root policy when preparation is absent; an enabled root cannot silently pass. Invoke final stage checking only for successful submit/complete and actual admin complete, not unrelated/refused actions.
- `TestStageCompletionPolicyActivationRequiresExact864Ledger`: New enabling writes validate execution schema, submission schema and exact locked 864 ledger both before append and before commit; original-key recovery occurs before new-admission checks.

The current baseline also passes actual verified submit/exact-submission and self-result completion, one immutable binding, and original-key replay. The older draft admission `/false` failure (controlled deployment returned `unknown`, nil error) remains intact; the new pass does not establish that old fixture failure's cause. `/true` passed in the old attempt. No timeout, pool or bad-sample weakening was used.

### Exact successful suite commands

```sh
node /Users/artisan/.multica/acceptance/pay241-20261113-01/run.mjs /Users/artisan/.multica/acceptance/pay241-20261115-continuation/configs/service.json go -C /Users/artisan/.multica/acceptance/pay241-20261115-continuation/server-2d/working/source/server test ./internal/service -run '^TestStageCompletion(Guard|PolicyActivation|StoredBlockers|Activity)' -count=1 -json
node /Users/artisan/.multica/acceptance/pay241-20261113-01/run.mjs /Users/artisan/.multica/acceptance/pay241-20261115-continuation/configs/service.json go -C /Users/artisan/.multica/acceptance/pay241-20261115-continuation/server-2d/working/source/server test ./internal/service -run '^TestStageCompletion' -count=1 -json
```

- `continuation-green-r01`: `/Users/artisan/.multica/acceptance/pay241-20261115-continuation/server-2d/logs/continuation-green-r01.jsonl`; exit 0; elapsed 2.956s.
- Command SHA-256 `07d96f60a91b9e179a998ece710616b3476ea89fedb4db6273ac1ca83af40a2c`; log SHA-256 `83f3f4f195f9221ce704ae9f180d1de174927401264505e64dfd614fa0d07930`; exit-file SHA-256 `9a271f2a916b0b6ee6cecb2426f0b3206ef074578be55d9bc94f6f3fe3ab86aa`.
- `completion-baseline-r02`: `/Users/artisan/.multica/acceptance/pay241-20261115-continuation/server-2d/logs/completion-baseline-r02.jsonl`; exit 0; elapsed 14.272s.
- Command SHA-256 `2829b9ee52d61abf22418283f0edf7d0cdfc6555c0b99ffca6989c2a415182d6`; log SHA-256 `a284464cc731426e450922bcbfc683833f7109bf753befcf0d8709e737d6770a`; exit-file SHA-256 `9a271f2a916b0b6ee6cecb2426f0b3206ef074578be55d9bc94f6f3fe3ab86aa`.

These commands used the working path. The final freeze contains the unchanged passing product fixes and exactly the continuation-test bytes used by these commands (SHA-256 `dce3310e891db75d6cadaf336cb75849d81dd0d23e1366ec1f6429e1d967ea7c`). The unrun later test-only extension is saved privately in `/Users/artisan/.multica/acceptance/pay241-20261115-continuation/server-2d/input/not-run-extension-r02/manifest.json` (draft SHA-256 `35511be5bd5dedaad9e762ce542541b22edde2971437101bf27e81f78ab52014`) and is not part of the delivered Go tests. No product source was reverted, and no new W/Q matrix was run after the root requested this handoff.

## Build, static and isolation evidence

```sh
env -u DATABASE_URL -u TASK_DATABASE_NAME HOME=/Users/artisan/.multica/acceptance/pay241-20261115-continuation/server-2d/home TMPDIR=/Users/artisan/.multica/acceptance/pay241-20261115-continuation/server-2d/tmp TMP=/Users/artisan/.multica/acceptance/pay241-20261115-continuation/server-2d/tmp TEMP=/Users/artisan/.multica/acceptance/pay241-20261115-continuation/server-2d/tmp GOCACHE=/Volumes/artisan/dev-cache/go-build GOMODCACHE=/Users/artisan/go/pkg/mod GOPATH=/Users/artisan/go /opt/homebrew/bin/go -C /Users/artisan/.multica/acceptance/pay241-20261115-continuation/server-2d/frozen-green-r03/source/server build ./...
```

- Final no-DB `go build ./...`: exit 0; `/Users/artisan/.multica/acceptance/pay241-20261115-continuation/server-2d/logs/frozen-green-r03-build-all.log`; SHA-256 `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`.
- Scoped frozen `gofmt -l` and LIVE `git diff --check`: both exit 0 with empty output. Exact commands/logs are `/Users/artisan/.multica/acceptance/pay241-20261115-continuation/server-2d/logs/frozen-green-r03-gofmt.command.txt` / `/Users/artisan/.multica/acceptance/pay241-20261115-continuation/server-2d/logs/frozen-green-r03-gofmt.log` and `/Users/artisan/.multica/acceptance/pay241-20261115-continuation/server-2d/logs/live-green-r03-diff-check.command.txt` / `/Users/artisan/.multica/acceptance/pay241-20261115-continuation/server-2d/logs/live-green-r03-diff-check.log`.
- Static source/hash audit: `/Users/artisan/.multica/acceptance/pay241-20261115-continuation/server-2d/logs/frozen-green-r03-source-audit.json`; SHA-256 `85143f4d35f072e02719d1313d861ff20e623704dbae3a2cb734db0dd417f75f`. The 861/864 migration and submission query/generated digests match root's frozen r02 hashes. No SQL semantics or DDL were changed in this continuation; no new DDL is requested.
- Only `/Users/artisan/.multica/acceptance/pay241-20261115-continuation/configs/service.json` (SHA-256 `b4a13f261b5d9dcd0724a67e09354928d2d66bdbcec394d96e437a803d62efb3`) and the existing identity launcher were used for DB tests. HOME/TMP are private. Root's allocation record is `/Users/artisan/.multica/acceptance/pay241-20261115-continuation/evidence/database-allocations.json`. No old business, harness or `check_*` database was accessed.
- Root supplied r02 ledger/identity/retention results separately (835 ledger rows through 864, bare hex64, identity match, no other connections). These are not migrations executed by this sub-agent.
- Full service, handler/router normal, all races, `go vet`, global cleanup and runtime/E2E are **NOT_RUN in this batch**. Prior 2c or external root evidence is not substituted for new 2d coverage.

## Changed server paths

- `server/internal/handler/collaboration_completion_preview.go`
- `server/internal/service/collaboration_completion_preview.go`
- `server/internal/service/collaboration_stage_activity.go`
- `server/internal/service/collaboration_stage_completion.go`
- `server/internal/service/collaboration_stage_completion_test.go`
- `server/internal/service/collaboration_stage_project.go`
- `server/migrations/861_collaboration_stage_submission.down.sql`
- `server/migrations/861_collaboration_stage_submission.up.sql`
- `server/migrations/862_collaboration_stage_submission_id.down.sql`
- `server/migrations/862_collaboration_stage_submission_id.up.sql`
- `server/migrations/863_collaboration_stage_submission_issue.down.sql`
- `server/migrations/863_collaboration_stage_submission_issue.up.sql`
- `server/migrations/864_collaboration_stage_submission_ready.down.sql`
- `server/migrations/864_collaboration_stage_submission_ready.up.sql`
- `server/pkg/db/generated/collaboration_stage_submission.sql.go`
- `server/pkg/db/queries/collaboration_stage_submission.sql`
- `server/cmd/migrate/main.go`
- `server/cmd/server/router.go`
- `server/internal/handler/admin_collaboration.go`
- `server/internal/handler/collaboration_boundary_test.go`
- `server/internal/handler/collaboration_lifecycle.go`
- `server/internal/handler/collaboration_workflow.go`
- `server/internal/handler/github.go`
- `server/internal/handler/issue.go`
- `server/internal/handler/project.go`
- `server/internal/service/admin_collaboration.go`
- `server/internal/service/collaboration_activity.go`
- `server/internal/service/collaboration_edit_test.go`
- `server/internal/service/collaboration_fields_test.go`
- `server/internal/service/collaboration_handoff.go`
- `server/internal/service/collaboration_lifecycle.go`
- `server/internal/service/collaboration_stage_actions.go`
- `server/internal/service/collaboration_stage_execution_approvals.go`
- `server/internal/service/collaboration_stage_review.go`
- `server/internal/service/collaboration_workflow.go`
- `server/internal/service/collaboration_workflow_test.go`
- `server/internal/service/delivery_completion.go`
- `server/internal/service/delivery_completion_collaboration.go`
- `server/internal/service/delivery_project_change.go`
- `server/internal/service/issue_status_write.go`
- `server/internal/service/task.go`
- `server/pkg/db/generated/models.go`
- `server/internal/service/collaboration_stage_policy.go`
- `server/internal/service/collaboration_stage_completion_continuation_test.go`

Per-path checked2c/draft/before/LIVE/frozen digests are in the JSON report and source manifest. The two accompanying S2 research result files are the only task-artifact changes.

## Remaining W coverage

| ID | Status | Not yet covered |
| --- | --- | --- |
| W01 | PARTIAL | Unverified refusal and verified immutable binding in the default M1/off fixture pass; observe/enforce, attachment effects and final-clock races are not covered. |
| W02 | PARTIAL | Verified exact submission completion and same-key replay pass in the default M1/off fixture; enforce consumption and late/race/legacy bindings are not covered. |
| W03 | PARTIAL | Unverified refusal, verified self-result completion, one binding and replay pass in the default M1/off fixture; observe/enforce and attachment variants are not covered. |
| W04 | NOT_RUN | Wiring is merged; stage-enabled positive/negative admin, actual password/org, source issuer separation and audit/final-clock races remain. |
| W05 | NOT_RUN | Merged writer wiring has no new normal/race regression evidence in this batch. |
| W06 | NOT_RUN | Actual Move/root boundaries, ordinary child control and M1 combinations remain. |
| W07 | NOT_RUN | Committed-prefix, root refusal before side effects and repeated grant/key cases remain. |
| W08 | NOT_RUN | Real create paths and human/machine/done controls remain. |
| W09 | NOT_RUN | Actual signed/trusted webhook, ordinary child and stable root refusal controls remain. |
| W10 | NOT_RUN | No CLI, daemon, API process or provider was started. |
| W11 | NOT_RUN | Merged exit guard requires real M1-mode and contention tests. |
| W12 | NOT_RUN | Merged exit/delete wiring requires actual endpoint and policy contention controls. |
| W13 | NOT_RUN | Pre-side-effect boundary and ordinary-parent/root-child controls remain. |
| W14 | NOT_RUN | Existing 2c evidence is not reused as new 2d acceptance; actual stale-candidate consumption remains. |
| W15 | NOT_RUN | This batch does not rerun completed 2c admission work; both commit-order regressions remain for the combined gate. |
| W16 | NOT_RUN | The new final-stage invocation is restricted to successful submit/complete and admin complete; actual cancel/reopen compatibility remains. |
| W17 | NOT_RUN | F06 product/handler/router routes are merged and compile; readonly, exact source/root revisions, done-child, ACL and cursor/race tests remain. |
| W18 | PARTIAL | Closed safe metadata unit projections and stored stage-refusal GET/replay pass; actual HTTP/timeline/outbox, invalid retention and races remain. |

## Remaining Q coverage

| ID | Status | Not yet covered |
| --- | --- | --- |
| Q01 | PARTIAL | Only default M1/off submit/exact/self evidence; no full off/observe/enforce x R38 matrix. |
| Q02 | PARTIAL | Nil transaction/guard and omitted managed-root preparation now refuse; wrong transaction, reused/unfinalized guard, actor/operation exactness and allowed-after variants remain. |
| Q03 | NOT_RUN | Post-receipt final-clock, late issuer/caller/role/participant/account/source/trust changes and both commit orders remain. |
| Q04 | PARTIAL | A successful submit/self creates one immutable binding; missing/legacy/corrupt operation and semantic binding tests and exact row-lock enforcement remain. |
| Q05 | NOT_RUN | Actual P != E, outside-source consumer, revoked issuer/consumer, wrong admin org and privacy/control positives remain. |
| Q06 | NOT_RUN | Preview/attachment reads have no verified zero-write proof; done-child and exact root/source observation/binding remain. |
| Q07 | NOT_RUN | HTTP strict parsing/body bounds, candidate list/empty/cursor current-authority and race cases remain. |
| Q08 | PARTIAL | Original refused key survives policy disable, GET/replay preserves blockers without events; completed same-key replay passes. Commit-unknown/late failures/reopen and HTTP lost-response cases remain. |
| Q09 | PARTIAL | Enabling now checks execution/submission shape plus exact 864 ledger before write and before commit; missing-ledger seam refuses with no policy row. Missing table/index/trigger/partial migration, full writer readiness, final authority ordering and actual P2 activation remain. activation_available stays false. |
| Q10 | NOT_RUN | Completion/submit vs policy/project exit locks, physical target conflicts and independent ordinary child controls remain. |
| Q11 | NOT_RUN_LOCAL | Frozen r02 DDL/SQL digests match statically. Root-supplied migration/retention results are external evidence, not migrations run by this sub-agent. |
| Q12 | PARTIAL | Only the authorized new r02 service profile and launcher were used; original failures, before bytes/hashes and exact freeze are retained. Four runtime/E2E scenarios and full denominator remain separate. |

## Concrete next-batch boundaries

- Candidate head and immutable submission still use plain read queries in the inherited draft; precise owning-transaction row locks remain to implement.
- Bind the private final guard to the actual caller/participant/account snapshot and exact original operation/result identities independently of M1; corruption and late-change tests remain.
- Complete final evidence horizons including artifact/original issuer/rollback metric bounds and final current source/target checks; no late-clock suite was run in this batch.
- Stage x M1 all modes, R38 and P != E positive/negative controls; actual F06 list/preview/activity service/handler/router normal/race evidence.
- Keep transport activation_available false until full writer readiness and checks pass; this handoff must not enable P2.

There is no LIVE shared-hunk conflict or newly required DDL blocker. This is a bounded three-RED repair and safe inherited-2d integration, not completion of the final all-writer safety gate. Root owns the independent checker dispatch and the next precise test batch; this sub-agent does not recursively delegate or mark S2 complete.
