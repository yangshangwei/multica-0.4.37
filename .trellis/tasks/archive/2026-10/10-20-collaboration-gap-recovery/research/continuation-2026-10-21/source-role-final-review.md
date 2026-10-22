# Independent source-role final review — 2026-10-22

Recommend **AC80 and AC81 passed at their assigned P1 layer**. The new exact role/reason/request tests close all four residual evidence clauses from the previous immutable [source/iteration review](source-iteration-final-review.md), combined with current canonical creation/source-edit/race/external-handler evidence. No product defect or mechanical fix was found. This reviewer writes only this report and [JSON evidence](source-role-final-review.json); source, previous reviews, matrix/catalog/progress/specs/index and environments stay read-only.

## Exact rows and closed gaps

| Criterion / P1 | Whole-clause result |
| --- | --- |
| AC80 | Source-member creator initial project/parent fields and later source-member creator/current-handler edits pass, with source validation, history/replay and no AI mutation. Direct project/parent/stage requests now refuse ordinary source member/owner, full external/source nonmanager participant, external current handler, departed handler and departed creator. Minimal selected references remain without private parent body, unselected resource IDs, URLs/routes or source catalog. Real transfer/departure contention rechecks authority in both orders and loser leaves no partial source fields. **Passed.** |
| AC81 | Initial and subsequent source iteration join/switch/remove pass with issue/iteration revisions, current membership/history and explicit reasons. Seven roles now directly call ChangeIteration and are refused without partial business/control state or catalog leaks. Creator and source handler remove with missing/blank reason both refuse permanently; changed old-key reason conflicts and fresh reasoned removal commits actor/history only. Metadata PATCH containing iteration and actual dedicated iteration PUT containing title/project/status/stage both refuse before effects. Complete source-owned external-handler statistics/start/end preview/history/rollover and private-field redaction pass. Both transfer/departure and target-iteration-version commit orders retain exact history and unchanged AI fixture state. **Passed.** |

The parent original criterion text at acceptance.md:89/90, requirements R36/R49/R50 and P1 ownership were matched exactly. These decisions do not promote the matrix automatically or relax any clause. Per-clause source anchors, actual executed identities/event lines and artifact/source hashes are preserved in JSON.

The previous four missing clauses are specifically closed:

1. **AC80 direct structure-role/catalog refusal:** `TestCollaborationSourceStructureAndIterationRejectNonManagers` sends actual project/parent/stage fields with a companion title, enters persisted `edit_source_structure`, and checks 403 plus original operation kind/result and exact business/control snapshots. It does not substitute generic metadata or ordinary iteration endpoints.
2. **AC81 direct iteration-role/catalog refusal:** the same seven roles each send actual ChangeIteration, verify persisted `change_iteration` refusal and identical replay, compare source memberships/history/inputs/runs/permissions and check FieldOptions/minimal references.
3. **AC81 no-reason removal:** `TestCollaborationIterationRemovalRequiresReasonAndOriginalRefusal/creator` and `/source_handler` join, then separately remove with empty and whitespace-only reasons. Both exact422 refusals preserve membership/history/control/AI. Old key with changed reason conflicts; a fresh reasoned removal records actor/reason and only expected iteration/issue revisions/timestamps. Both old refused keys and own GET recover their original unchanged refusal after success.
4. **AC81 mixed iteration PUT:** `TestCollaborationIterationPUTRejectsMetadataBeforeEffects/title`, `/project_id`, `/status`, `/stage` use actual MethodPut and dedicated path. Strict400 occurs before issue/membership/operation/event/control/AI effects, ambient scope is stripped, private/no-store boundaries hold. A legal independent PUT commits once, replay leaves exact state unchanged, and external detail keeps only selected project/parent/iteration references.

## Source and assertion review

| New file | SHA-256 |
| --- | --- |
| `server/internal/service/collaboration_source_roles_test.go` | `f006430da597c8a62209f72b8f04c32d9104df2f13886b87e8f9b9ac69fe694d` |
| `server/internal/handler/collaboration_iteration_request_boundary_test.go` | `7a081b2d5a46b11299b343ca8fcbb6cc5a13372bc77013abb9098fd14b8923a0` |

Both hashes match current source, first focused/adjacent manifests and isolated copies. All9 protected source/helper hashes and all30 previously reviewed source hashes remain current. The prior Markdown/JSON hashes remain `1068caa9e8414901f803b14e89177657b73b91b59123775bfa0a08a89b8ab56c` / `01fd2b3c207db7ada14e008746a05e15d282f1fe6143559bd7f2966844561c8e`; neither previous review was rewritten.

The direct-role prerequisites separately assert current Read, source membership, Edit manager state and full nonmanager participant ControlAI. Thus a generic inaccessible object cannot make the authority matrix pass. Each selected project/parent/iteration reference is present; private parent body, unselected source identities, URLs/paths, directory collection keys and account fields are absent. Nonmanagers receive no FieldOptions; external/departed managers receive no SourceOptions. Production `EditIssue` chooses edit_source_structure for project/parent/stage; `workflowRoleAllowed` requires both locked source membership and reloaded EditSourceStructure. The tests check the actual persisted operation kinds.

Negative requests compare full business/source snapshots and complete controlStateSnapshot. Refused operation rows are separately compared, because a durable rejection receipt is legitimate while a business mutation is not. Successful reasoned removal normalizes only expected iteration membership, revision and activity timestamps when comparing the issue; responsibility/authority/lifecycle/control remain unchanged. Execution snapshot excludes only legitimate human events and session.event_seq, retaining target/channel/grants/input/run/proof facts. Batch/group-input/dependency rows are also retained. In-flight group/run data is synthetic fixture state; it is not a launched provider.

Canonical initial/source edits, explicit null clear, cycle/missing-source refusal, joins/switch/removal, version refusal/replay and completed-issue confirmation are actual current PASS. Previous creation selection/source atomic-refusal PASS and exact event lines were revalidated from retained current-task partition JSONL. Real race helpers pause an actual pgx writer immediately before commit and prove the other writer blocked through pg_blocking_pids against its exact backend PID. Transfer/departure/target-revision both-order results are current PASS, with no arbitrary sleep as evidence or partial source/AI writes.

## Current executed evidence

| Raw JSONL | Actual PASS / wall / package times | SHA-256 |
| --- | --- | --- |
| `source-role-focused-first-20261021.jsonl` | 3 top-level /44 total; wall31.390s; service4.903s, handler2.377s | `107e132d5cfa44c4af416ef8794cfea884534aeb2caef3fd484ce8141e946e74` |
| `source-role-adjacent-first-20261021.jsonl` | **22 top-level /95 total**; wall23.399s; service11.488s, handler2.820s | `f035c0a251fe1addbd0c317c9a286d1f2ca4458c131d8431ed6383be02a66cf9` |
| `source-role-isolated-head-race.jsonl` | 3 top-level /44 total; wall50.758s; service5.147s, handler2.411s | `7d14504beb0b0b1f9c204e4b10da0ead9d93d006ded6644a44c92760dadb72cd` |

Each has actual package PASS, returncode0, empty stderr, **zero failures/skips**. First new DB assertions passed; there was no DB fixture failure/retry or product repair. JSON retains full original command vectors, cwd, package results, source hashes and every exact PASS event line. The current focused/adjacent environment is guarded Go1.26.6 darwin/arm64, migration737 service_s5, `-race -p 1 -parallel 1 -count=1 -timeout=30m`. The isolated result correctly uses separate schema737 **service_s6_ordinary**, not service_s5. No DB test was executed by this reviewer.

All seven roles (`source_member`, `source_owner`, `participant`, `source_participant`, `external_handler`, `departed_handler`, `departed_creator`) have actual project/parent/stage/iteration PASS subtests. Both removal-role cases and all four mixed-PUT cases pass. The22 top-level adjacency also executes original creation/structure/iteration positives, both-order transfer/departure/version races, source catalog and strict PATCH, external HTTP statistics/history and ordinary move/rollover boundaries. Test-name matching is not used as completion evidence.

## Exact HEAD two-source commit isolation

`source-role-isolated-head-source.json` records HEAD `7794f57d969fb583ddf02c830f6d1ab72186595d` at `/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-source-roles-head-pxmywphi`. Independent `git archive <recorded HEAD> server scripts` reproduces recorded SHA:

`bb70831936bf724b89adc1b5db02ec35077069991a5f60f42ac5a97daad85fcc`

All **4064 tracked server files and66 tracked scripts (4130 total)** match extracted bytes exactly. The only two additional files in the snapshot are the new service and handler tests, with current/recorded/isolated hashes equal. No dirty product, generated SQL or new sibling test supplies a dependency. The isolated selected3/44 pass therefore supports a Lore commit with only these two source files plus owned evidence/spec record. Current HEAD has advanced through other verified slices; this comparison is explicitly against recorded7794f57d9, not a claim that current shared source equals it.

## External-handler privacy and remaining AC85 limit

`TestCollaborationExternalHandlerIterationStatisticsPreviewAndHistory` is an actual current adjacency PASS. With no source membership for the handler, source-authorized start/end previews remain complete, current/original counts equal1, history retains exact issue/assignee ID/type, end snapshot freezes destination and rollover count, events remain readable, and server-owned rollover preserves external responsibility. Every tested source/detail response is no-store and excludes private email, username and parent body. External actor remains403 on ordinary source iteration reads/previews, with only minimal collaboration references and no SourceOptions.

The assignee name stays nullable and Available=false under the existing source-member directory contract. This does not make the P1 planning/history unavailable, and no directory was widened. **AC85 remains separate**: name/avatar/external marker, no-Unknown UI and drag behavior across Web/Electron are not evidenced by these tests. This review does not promote or repair that P6 client boundary.

## Verification and findings

Reviewer `gofmt -d` on the two new files passes with zero output/diff. Implementer's final `go vet -p 1 ./internal/service ./internal/handler` exit0/empty diagnostics and source hashes were independently checked in `source-role-static-final-20261021-vet-result.json`. Final current-tree and isolated race compilation is Go type evidence. No frontend TypeScript changes or frontend check is claimed; no reviewer DB/native/build/gate/provider command ran.

Fixed findings: none. Unfixed product findings: none. Exact whole-clause gaps at P1: none after this follow-up. Changed files: only `source-role-final-review.md` and `.json`. Simplification: existing fixtures, workflow/source/membership and snapshot primitives are reused; no product abstraction/dependency/interface change. Root may append exact evidence scope to existing specs and promote these two rows after its integration. This reviewer does not touch the ledger/index or commit.

Synthetic retained AI rows do not establish service launch, natural process proof, provider account/quota, unsupported OS or installed Main/installer. New Web/Electron evidence and production rollout remain separate. Child and parent stay in_progress until all97 exact criteria have current evidence.
