# AC63 source-deletion preservation evidence — 2026-10-21

The canonical refusal test now covers the missing attachment and permission
relations. Its expanded first run and both ordinary deletion controls pass
against the task-owned schema-737 database with race detection. No product
behavior changed.

## Criterion and coverage

AC63 (R37, primary owner P2):

> 删除被拒绝后，空间、共享任务、评论、附件及权限关系保持完整，不得先执行部分清理；没有本功能共享任务记录的空间仍按既有删除规则处理。

`TestWorkspaceWithCollaborationHistoryCannotBeDeleted` retains its `todo`,
`done`, and `cancelled` cases. Each case now builds:

- A source workspace with a real owner membership and collaboration issue,
  participant/session records, and a distinct external participant.
- A separate private execution workspace owned by that external participant,
  an actual offline agent/runtime, and a persisted enrolled runtime identity.
- A resource offer submitted by that owner, adopted by the source organizer,
  and explicitly consented by the owner through the existing handlers. The
  returned grant must correlate issue/target/owner/workspace/agent/runtime/binding
  and remain unrevoked. No provider executable is resolved or launched.
- A real encrypted private upload by the external participant, delivered by a
  normal collaboration comment. The delivered attachment must have its real
  issue and comment references; the encrypted row and storage bytes are not
  mocked.

Fixture assertions prove one source owner membership, one execution owner
membership, both active issue participants, and zero cross-workspace
memberships. The fixture therefore exercises source-member and external
participant read bases independently. The exact grant and delivered
attachment/reference checks run before the snapshot; all 15 relationship
arrays must be nonempty.

Before deletion, both the source owner and external participant read the exact
`controlled private bytes` through the protected attachment handler. The test
then calls the actual `DeleteWorkspace` handler and expects HTTP 409 with
`workspace_has_collaboration_records`. One ordered JSON snapshot must remain
byte-for-byte identical across the refusal:

| Contract | Snapshot / assertion |
| --- | --- |
| Source/execution spaces | Full `workspace` rows for both identities |
| Shared issue/history | Full issue, collaboration, session, comment, event and original-operation rows |
| Private attachments | Full encrypted attachment rows and all issue/comment reference rows; exact authorized bytes before/after |
| Source/participant permissions | Full user, source/execution member, issue participant and account-fence rows |
| Resource permission/identity | Full target, owner grant, runtime enrollment, agent, runtime and explicit offer rows |
| No partial cleanup | Exact before/after snapshot equality plus both independent attachment read bases after refusal |
| Existing ordinary deletion | `TestDeleteWorkspace_OwnerSucceeds` expects 204 and actual cleanup with no shared records |
| Other-workspace isolation | `TestDeleteWorkspace_PreservesOtherWorkspaceData` proves neighbor workspace/issue/comment/inbox/profile/usage/object rows survive ordinary deletion |

This closes the specific AC63 evidence gap described in
`ordinary-control-audit.md`. The coordinator owns acceptance-row promotion;
this report does not mutate the ledger.

## Change boundary and reuse

Only the body of
`server/internal/handler/collaboration_boundary_test.go::TestWorkspaceWithCollaborationHistoryCannotBeDeleted`
was edited. Existing bare comment/event setup was replaced with actual
upload/comment and offer/adopt/consent operations. The expanded assertions reuse
`dbfx`, `testutil.New`, `testutil.Call`, `collaborationFileHandler`,
`collaborationUploadedFile`, `collaborationContentRequest`,
`collaborationFileRequest` and `switchHTTPBody`. No new production abstraction,
dependency, migration or duplicate fixture helper was added.

All cleanup remains scoped to this test's issue/resources, including retained
target/channel/grant/history rows and configuration tombstones. Auth mode,
deployment ID and upload directory use `t.Setenv` and a test-owned temporary
directory. Production collaboration gates, managed rollout, browsers and native
runtime slots were untouched.

`ac63-deletion-baseline-20261021.json` retains the original function, source
hash and a hash of every out-of-scope byte. The final scope check proves all
bytes outside that function remain exactly unchanged.

## Commands and actual outcomes

Commands ran from
`/Volumes/artisan/code/2026/multica-0.4.37/server`, with Go 1.26.6 on
darwin/arm64, through the agent CLI guard and explicit task-owned `service_s5`
launcher:

```bash
GOTOOLCHAIN=go1.26.6 python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/with-db.py service_s5 \
  bash ../scripts/go-test-with-agent-cli-guard.sh \
  go test -json -race ./internal/handler \
  -run '^(TestWorkspaceWithCollaborationHistoryCannotBeDeleted|TestDeleteWorkspace_OwnerSucceeds|TestDeleteWorkspace_PreservesOtherWorkspaceData)$' \
  -p 1 -parallel 1 -count=1 -timeout=30m
```

The identical selector ran before and after the test extension:

| Run | Outcome | Evidence |
| --- | --- | --- |
| Existing narrower baseline | Exit 0; 3 top-level passes, 0 failures, 0 skips; package 2.733s | `ac63-deletion-baseline-race-20261021.jsonl` and `-command.json` / `-result.json` |
| Expanded first run | Exit 0; 3 top-level passes, all 3 refusal status subtests pass, 0 failures, 0 skips; package 2.956s | `ac63-deletion-expanded-race-first-20261021.jsonl` and `-command.json` / `-result.json` |

There was no product assertion failure and no retry. All stderr streams are
retained; the initial passing baseline is narrower coverage, not claimed as
the completed attachment/permission evidence. The expanded result contains
before/after source hashes and exact command/environment/timestamps. All six
recorded source hashes were unchanged during that run.

Static checks:

```bash
# From server/:
GOTOOLCHAIN=go1.26.6 go vet -p 1 ./internal/handler
# From repository root:
gofmt -d server/internal/handler/collaboration_boundary_test.go
```

Both exit 0 with empty diagnostics/diff. Vet took 7.304s and is retained in
`ac63-deletion-vet-20261021-result.json`, `.stdout` and `.stderr`. The expanded
first-run artifacts retain the empty gofmt streams.

The read-only provenance query in `ac63-deletion-database-20261021.json`
confirms PostgreSQL 17.11, database `multica_gap_service_s5`, migration
`737_collaboration_runtime_preparation`, and the actual
`agent_runtime.collaboration_capabilities` column. No connection credentials
were written to the repository or printed.

Source hashes for the expanded passing run:

| File | SHA-256 |
| --- | --- |
| `server/internal/handler/collaboration_boundary_test.go` | `6d5f6b6b7b111009d653b974a619b17c7315487bbaf0d143ba9daffadea15ebd` |
| `server/internal/handler/workspace.go` | `f3ff75784b9dfb0648ffac9277a5d2e24867efa4b841094293fe7ff33b3f37d7` |
| `server/internal/handler/workspace_test.go` | `b281eed61c6e0430d14a3dfb03a1a262b529a6bd99b427c5d8efe26dae28130a` |

## Limits and coordinator handoff

- The complete AC63 backend criterion is now represented by this current
  refusal snapshot plus the two ordinary deletion controls. The coordinator
  still owns independent review and the authoritative matrix/catalog update.
- Terminal issue statuses are persisted fixture prerequisites; this test does
  not claim to exercise the lifecycle complete/cancel UI. Those workflows have
  separate canonical tests and acceptance evidence.
- The encrypted blob uses real test-local storage. S3/external storage,
  packaged clients, real providers, other-OS containment and production rollout
  are outside this deletion criterion and were not tested here.
- No new full `make check` was run after this test-only change. No commit,
  archive, acceptance/spec/progress mutation or shared-source reset occurred.

Changed files: the canonical test function, this report and unique `ac63-*`
verification artifacts. Root retains ownership of all remaining S6 gaps and
any Lore slice commit.

## Appendix: standalone preservation regression for commit isolation

The coordinator requested an independently committable regression of the same
criterion. New
`server/internal/handler/collaboration_workspace_preservation_test.go` contains
`TestCollaborationWorkspaceDeletionPreservesPrivateAttachmentsAndPermissions`.
The original expanded offer/adopt-path test in
`collaboration_boundary_test.go` is preserved byte-for-byte.

The new test runs the same `todo`, `done`, and `cancelled` refusal scenarios,
using the existing `RegisterCollaborationTarget` and owner
`ConsentCollaborationTarget` handlers directly. Its dependencies already exist
in tracked HEAD; it uses no Offer/Adopt handler, offer table, or new product
module. AC63 verifies permission and data preservation, while the offer-path
acceptance has its separate test above.

Each scenario creates separate source/owner execution workspaces, real source
membership, external participation with no source membership, a private offline
agent/runtime with enrolled binding, exact registered target/owner consent,
and an actual encrypted local upload delivered by an external author's comment.
Both source owner and external participant can read the exact original bytes
before and after the HTTP 409 refusal. The grant must correlate its
issue/target/resource/owner/execution workspace/binding, and the attachment must
have exactly its issue and comment references.

The snapshot includes source/execution workspace, issue/collaboration/session,
full accounts and source/execution members, participants/account fences,
targets/grants/enrollment/agent/runtime, encrypted attachments and all references,
comments/events/original operations. All 14 relationship arrays are asserted
nonempty; source/participant and execution-owner relations are separately
asserted. It must remain exactly equal after refusal, preserving every seeded
record and permission identity. No product behavior or auth predicate changed.

The current-tree command, from `server/`, was:

```bash
GOTOOLCHAIN=go1.26.6 python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/with-db.py service_s5 \
  bash ../scripts/go-test-with-agent-cli-guard.sh \
  go test -json -race ./internal/handler \
  -run '^(TestCollaborationWorkspaceDeletionPreservesPrivateAttachmentsAndPermissions|TestDeleteWorkspace_OwnerSucceeds|TestDeleteWorkspace_PreservesOtherWorkspaceData)$' \
  -p 1 -parallel 1 -count=1 -timeout=30m
GOTOOLCHAIN=go1.26.6 go vet -p 1 ./internal/handler
```

The first race run passes 3 top-level identities, 6 total pass events including
the 3 refusal statuses, zero failures and zero skips, exit 0, package 2.902s.
It ran serially in the explicitly allocated task-owned `service_s5` slot and
released that slot immediately before report writing. Exact argument array,
timestamps/return code and before/after source hashes are in
`ac63-standalone-race-first-20261021-command.json` and `-result.json`; raw output
is `ac63-standalone-race-first-20261021.jsonl` and `.stderr`. No failed first
attempt or retry occurred.

Scoped vet passes with empty output in 3.398s; its exact command/hash/streams are
in `ac63-standalone-vet-20261021*`. From repository root,
`gofmt -d server/internal/handler/collaboration_workspace_preservation_test.go`
returns an empty diff.

Final new file SHA-256:
`389bd237027d1545ab12701e973f12b5fca699c66057fdf62bae5bb88cf1b879`.
`ac63-standalone-baseline-20261021.json` and the final-scope artifact confirm
all six existing product/helper files, including the original broad dirty
boundary test, remain unchanged. This implementation lane wrote only the new
test and this appendix/evidence; it did not commit, change a matrix/spec, run a
browser/provider, or alter an application environment/gate.

The coordinator owns the isolated tracked HEAD snapshot containing only this
new test, its independent check and any Lore commit. Until that check's own
artifacts exist, the implementer claims only the actual current-tree pass above.
The same local-storage/synthetic enrollment/terminal-fixture and other-platform
limits stated above remain; this does not introduce a new full product claim.

## Coordinator standalone HEAD proof

Exact HEAD `1918a41b9` was archived outside the shared checkout with only
`collaboration_workspace_preservation_test.go` added. The same real local blob,
direct target registration/owner consent and14 nonempty permission/history
relations plus ordinary owner deletion/neighbor controls pass there: **3 top-level
/6 total, zero failures/skips**, guarded race exit0. The original broad dirty
`collaboration_boundary_test.go` and new offer implementation were not copied.
`ac63-standalone-isolated-head-source.json` and
`ac63-standalone-isolated-head-race-result.json` retain the exact HEAD/archive/
source/selector. This independently verifies the commit slice without absorbing
shared uncommitted work. No product behavior changed; the earlier full offer-
path snapshot remains separate current integration evidence.
