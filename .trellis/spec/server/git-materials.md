# Git requirement and plan materials

## Scope

An ordinary project may opt into publishing requirement and plan documents. Git owns the verified bytes; Multica owns comments, human decisions, job recovery, and task dispatch. A merged material is not authorization to run code or complete an issue.

## Signatures

- Project routes are under `/api/projects/{id}/materials`. Settings are read with GET and changed with PUT. Create materials with POST; send revisions, decisions, comments, retry and start-plan commands to the material's own route.
- `multica project material {list,get,create,revise}` reads or authors candidates, not decisions. `revise --file -` consumes one bounded JSON object. Retain its original `request_id` across retries.
- `multica.material.baseline()` is a read-only plugin action bound to the surface's plan issue. Installing a plugin is not approval to read every material in the workspace.
- Provider merging accepts `Merge(ctx, MergeExpectation{Number, HeadSHA, Branch, BaseBranch, Changes})`. The complete frozen expectation is checked before the remote write and again against the returned observation.

## Contracts

Mutations have a request identity; version-sensitive commands include `expected_version`. A decision binds the authenticated reviewer, revision and exact remote HEAD. A ready plan supplies both design and task breakdown plus its source baseline. The SDK never receives a Git credential. Ordinary chats and unready drafts create no publication intent.

The worker records facts in stages: a review URL is not approval, approval is not merge, and a successful merge request is not a verified baseline. Only exact bytes at the observed merge commit can establish that baseline. A queued plan is not a running agent. Keep the requirement baseline when approving its associated plan.

GitHub's `protected` branch flag includes rulesets. A classic protection endpoint returning 404 is insufficient proof that classic protection is absent. Ruleset-only support requires active rules plus an explicit, scoped GraphQL `branchProtectionRule: null`; missing fields, permission errors and unrecognized facts remain blocked. Provider APIs condition merging on the source HEAD, not an atomic snapshot of every target fact; post-merge byte verification remains required.

Shared material editors retain their mounted input and pinned revision during transient read failures. Authorization loss still clears protected state. Material mutations use `networkMode: "always"` and `retry: false`: a failed offline write retains its original body and request ID, and reconnect alone must not submit it.

Material plan enqueue, initial claim, singular/batch reclaim, token finalization and start all consume the exact recorded task, agent, originator and immutable baseline. The full handoff text must equal the baseline-derived prompt. Claims already own task locks: use the project/member/agent `SHARE NOWAIT` fences rather than waiting on a configuration writer that may need the task lock. Propagate database/lock failures; they are not permanent permission denial. A confirmed material denial cancels only the unstarted attempt, retains its history, and must not strand unrelated work behind it or roll back healthy rows in a reclaim batch.

An explicit human `start-plan` may restart a failed/cancelled/completed attempt only if no plan revision was submitted and no active attempt exists. Retain the same issue and previous queue rows; serialize the new attempt with the material version and original request identity. Background retry and re-enabling automatic planning never restart a terminal attempt. Each new approved requirement baseline gets its own plan issue; ordinary title deduplication is bypassed only for this internal creation, whose material/baseline identity already supplies uniqueness.

## Validation and errors

| Boundary | Required behavior |
| --- | --- |
| Reused request ID with a different command body, stale version or changed head | Refuse the mutation; do not silently retry as a new request. |
| Unknown native check, incomplete changed-file listing, foreign file or unreadable tree | No merge or baseline. A provider 404 is not an empty tree. |
| Timeout after a remote effect | Recover against the same intent and immutable bytes. An ambiguous GitLab branch owner requires operator review. |
| Action refused, project still readable | Keep the editor and draft. Revoked project or workspace access clears protected caches. |
| CLI receives an error page | Keep status classification; never echo response text that may contain a private document. |
| Target branch or complete changed-file set changes between observations | Refuse merge or refuse the final receipt; do not establish a baseline. |
| Detail/settings refresh fails temporarily | Keep current editable inputs and focus, show a retry action, and preserve the pinned reviewed version. |
| Approval fails offline, then connectivity returns | No automatic write; explicit retry reuses the exact persisted request. |
| Authority revoked after enqueue | No claim payload, token or start; settle the denied unstarted attempt without blocking ordinary queued work. |
| Project/member/agent lock is contended or a DB read fails | Roll back and retry; do not classify uncertainty as a permanent denial or cancel the attempt. |
| Human retries a terminal plan without an output revision | Same issue, preserved prior run, one new authorized attempt; replayed request returns its original result. |

## Examples

- Good: a human approves the visible SHA and the worker verifies the approved content after merge.
- Base: automatic planning is off. The verified baseline produces a durable todo; a manual start reuses that issue.
- Bad: an MR is merged elsewhere before an authorized merge intent. Do not treat it as an approved baseline merely because the file contents match.

## Verification

Use the local HTTP provider suites for each platform and an identity-checked PostgreSQL database for service, handler and router tests. The Web/Electron runner in `e2e/fixtures/material-run.md` exercises stored settings, review decisions, dispatch count and exact bytes with a loopback GitLab service. Keep this evidence separate from testing a real provider, model, or packaged desktop installation.

Keep provider regressions for both pre-write and final-receipt target/file-set drift and affirmative ruleset-only policy discovery. `project-materials.test.tsx` owns settings/review input retention and offline explicit retry; the shared browser journey verifies the focused review input survives an actual failed poll before recovering.

`material_test.go` owns revoked claim/reclaim/finalization, mixed ordinary queues, contention, superseded baselines and terminal retry. Assert absence of disclosed payload/tokens, preserved terminal history, the same issue ID and exactly one new attempt, then include existing ordinary claim/admission tests in the regression run.

## Wrong vs correct

Do not sort display timestamps to choose an approvable revision, copy the latest text into a new run without its baseline, or convert a lost merge reply into a successful handoff. Use server history order for display; the service verifies its current revision pointer and the actual Git baseline before acting.
