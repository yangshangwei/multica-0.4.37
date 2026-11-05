# M1-A1 independent implementation review

Reviewer: `trellis-check` / `least_privilege_plan_review`
Worktree: `/Volumes/artisan/code/2026/mc-agent-rnd-m1-20261105`
Decision: **Accept this bounded child implementation after the source-context fix.** No remaining blocking code findings were identified in the reviewed scope. This does not accept M1 as a whole or M2/M3.

## Findings (fixed)

### Allowed issue detail disclosed live changes from an unauthorized source issue

`GetIssue` includes `issueSourceContextDetail`. The initial capability implementation checked the target issue, then the existing detail helper read the current source issue/thread unconditionally. A source attachment added after the frozen capture exposed its new filename through `source_context.change_details.description_attachment_changes`. This was a new live resource read, not user-supplied frozen input.

The reviewer added `server/internal/handler/task_capability_review_test.go`. Actual failing run `check-a1-source-context-red02` recorded reviewer-other-issue and reporter-other-project failures, with reporter-same-project passing. The implementer then fixed `server/internal/handler/source_context.go`: both source issue and surviving anchor issue require capability scope before live projection or live author lookup. Outside scope returns the retained captured snapshot with current-source states unavailable and no live deltas.

Independent rerun under the race detector passed all three scenarios, with assertions that the frozen comment remains readable and the permitted same-project reporter retains live attachment changes. The initial `red01` failed during fixture setup because this table has no default UUID; it is retained and explicitly excluded from product regression proof.

### Independent production-router coverage added

`server/cmd/server/task_capability_review_test.go` exercises the actual production router with a valid scoped credential: current issue/comments/timeline reads, a current issue comment write, cross-issue denial, mutation denial, credential mint refusal, plugin bridge refusal, durable comment non-dispatch and a human write control. This complements the implementation tests' real single/batch claim endpoints and prevents an allow-all/deny-all router from satisfying the negative baseline alone.

## Findings (not fixed)

None blocking in this child. The following are explicit boundaries, not hidden passing claims:

- The policy is an API/platform grant ceiling, not a local filesystem, shell or arbitrary network sandbox. Human-supplied agent configuration and ambient host credentials remain outside it.
- Resource-specific attachment URLs already delivered retain their existing lifetime. A restricted request receives no wildcard CloudFront cookie; task revocation cannot retract previously delivered content or URLs.
- Restricted **comment-created** events carry existing `suppress_execution`; immediate/comment replay/plugin dispatch remains inert. Trusted configured `task:running/completed/failed` plugin subscriptions retain their existing lifecycle behavior. This change does not disable every plugin subscription.
- The new role versions intentionally do not automatically wake a leader or write issue status. Older saved roles/skills are not migrated. Human or existing authorized coordinator intervention remains necessary; no new done/acceptance semantics are introduced.
- Parent M1 controlled high-risk execution, evidence gates and full critical-path CI acceptance remain separate. No M2/M3 acceptance follows from this review.

## Verification

| Independent command/selection | Result | Retained summary |
| --- | --- | --- |
| Handler `^TestTaskCapability`, race, final source | 38 named pass, 0 fail, 0 skip | `review-evidence/check-a1-final-handler-race02-summary.json` |
| Production router `^TestTaskCapability` | 10 named pass, 0 fail, 0 skip | `review-evidence/check-a1-router01-summary.json` |
| Pure auth capability + middleware auth/CDN cases | 8 named pass, 0 fail, 0 skip | `review-evidence/check-a1-pure01-summary.json` |
| Service restricted/legacy failure recovery, durable dispatch and plugin mapping controls | 4 named pass, 0 fail, 0 skip | `review-evidence/check-a1-service01-summary.json` |
| `go build -p 1` auth, middleware, service, handler, cmd/server | pass | `review-evidence/check-a1-static-summary.json` |
| `go vet -p 1` same affected packages | pass | same summary |
| `git diff --check`, affected new Go files `gofmt -l` | pass, no output | executed after final source fix |

These named counts include parent tests/subtests; they are scoped inventory, not complete-package or repository coverage. The earlier independent handler race run had 34 passes before adding the source-context regression, and is retained separately without adding it to final coverage totals.

Database packages were run serially, using root-allocated, distinct handler/router/service databases in the task-owned PostgreSQL17/pgvector container. Test runners used GOMAXPROCS2/-p1 and private HOME outside Git. Reviewer created no databases, ran no migrations, and called no real providers/models or external hooks. Local Go is 1.27.0; hosted Go1.26 and browser/desktop/real-model behavior remain unverified here. No TypeScript production code was changed by this child, so frontend typechecking was not rerun. The coordinator separately supplied migration rollback/schema and sqlc verification evidence; this review inspected those contracts/source but does not relabel their runs as independent executions.

## Source review and evidence identity

Reviewed the private task-token context in both auth modes; both actual router Auth groups; strict capability parse/default denial/missing scope; current task generation/identity/status, live member/user/invoke checks; consistent read snapshots and transactional comment locks; profile provenance and old-version preservation; claim freezing and unsupported-tool/non-issue terminal refusal; both daemon tool backstops; durable dispatch suppression on immediate/reconcile/final-output/failure-recovery paths; wildcard-cookie denial; template/skill instruction alignment; migration down locks; and live secondary source reads.

Existing reporter list variants use `project_id` as an AND constraint in both SQL paths. Repeated query values and encoded paths are denied. Labels, reactions and attachment metadata directly attached to an allowed issue are part of that permitted resource; the review found no additional dynamic foreign issue read in those fields. Foreign source-context live deltas were the concrete exception and are now covered.

`review-evidence/review-source-manifest.json` binds the reviewed files; `check-a1-final-sources01.json` was captured immediately before the corrected race run. Product hashes were rechecked after all focused execution and were unchanged. Task docs/spec updates by the coordinator after this point must be reviewed as documentation changes; they do not invalidate the code test evidence. Raw failures and passes remain in the task-private directory recorded in `review-evidence/README.md`, with SHA256-bound summaries copied here. `source-context-red02.patch` records the product fix, and both reviewer regression files remain in the source tree.
