# S3 blocking service R01 failure diagnosis — 2026-11-18

The original full service database round remains **NOT ACCEPTED**. Its 16 database top tests and 43 named identities ran once, with three failing top contracts and the failing `creator_replacement` leaf; no skip or source/configuration drift occurred. The database and raw command/output/audit remain immutable. This diagnosis corrects three newly authored tests in a separate R02 work source; it does not accept S3 or infer a production defect from an invalid prerequisite.

Owner: `/root/s3_block_fix_r02`, the sole server repair implementer after the first owner froze R01. Write source: `/Volumes/artisan/.multica-acceptance/pay241-s3-clients-20261118-x4c5mjep/source-block-work-r02`. The root workspace, current client runtime, R01 source and its database were not edited or executed by this owner. Public v2/r05 JSON, production code, SQL/scanners, migrations 866/867 and old runtime-v1 readers remain unchanged.

## Three actual causes and bounded repairs

| Original failing test | Actual source relationship | R02 correction and retained contract |
| --- | --- | --- |
| `TestCollaborationInformationCreationEffectiveProjectAndOriginalKeys` | The test's ordinary parent used `testutil.Fixture.Issue`, whose default number is `MAX(issue.number)+1`. That fixture insert does not advance `workspace.issue_counter`. The subsequent real collaboration writer called the canonical `AllocateIssueNumber`, which increments the stale counter and collided with the parent's number (`uq_issue_workspace_number`, SQLSTATE23505). The managed blocked refusal had already passed before reaching this unrelated fixture collision. | Create this ordinary parent through the existing `IssueService.Create` owner with the actual source member. Assert its workspace/project and next number, then read `GetWorkspace` to prove its counter matches. Preserve the managed inherited-project rejection, old rejected key after policy-off and independent fresh intent; assert exactly one fresh row, the true inherited parent/project/status and one next number/counter. No production counter repair or open-coded counter SQL is added. |
| `TestCollaborationInformationRuntimeV1SourcesAndRepairPreserveExecution` | `CreateCollaborationBusinessEvent` inserts `payload::jsonb`. PostgreSQL canonicalizes JSON object formatting, so returned persisted bytes need not equal the pre-insert `json.Marshal` buffer. The old envelope's rehydrated equality had already passed; the failing final test compared two different encoding stages. | Capture the actual inserted event before policy/repair. First prove its decoded JSON equals the supplied valid pre-marker v1 specimen. Then freeze the actual persisted payload bytes/hash and event ID/sequence/producer identity. After repair/replay/snapshot recovery, require exact equality with that database baseline. The existing full recovered-envelope/source equality and zero extra execution checks remain. Incoming JSON and persisted JSONB hashes are logged separately; old runtime bytes are not rewritten or relabelled. |
| `TestCollaborationInformationRepairRestoresReturnableContext/creator_replacement` | R01 B1 was H→creator. The creator was therefore the current open request's recipient and issue handler, with a complete chain and legal exit. Its new B2 was a legitimate normal nested request under the approved v2 contract, not a parentless replacement. A blanket production refusal would break the separately required normal creator-nesting control. | For this leaf only, B1 is H→QA, using the already invited real QA participant. Assert that the blocked issue's current handler is QA and the creator is a distinct actor with transfer authority. Its direct replacement request must now refuse `information_repair_required`. The explicit historical repair truthfully records QA→creator and prior status blocked. Other scenarios retain actual creator self-target facts. Old B1 supersession relation/count, independent later B2/return, original repair replay and zero new run/input/stage effects remain asserted. |

The three changes are confined to `collaboration_information_creation_test.go`, `collaboration_information_runtime_test.go` and `collaboration_information_test.go`. The finite top/leaf inventory is unchanged: 19 service top identities comprise three pure and 16 database tops; database names remain43, handler names3 and migration names5. The source used by production API/CLI/migrator compilation is unchanged, so the R01 migrator's binary binding may be reused only with that owning-compilation distinction recorded.

## Verification performed by this owner

The existing no-database wrapper and main-owned `configs/nodb-r01.json` selected the trusted HOME/TMP and explicit toolchain. A task-private recorder retained absolute argv/cwd, config/wrapper/runner hashes, raw stdout/stderr and complete 5,077-file source snapshots per command. Its outer environment inherited no ambient keys. No `DATABASE_URL`, TestMain or database-backed test body was selected.

Five command profiles passed under `verification/block-test-repair-r02-checks/`:

1. Format exactly the three changed tests.
2. `go test -c` for `./internal/service`, without executing the test binary.
3. `go vet ./internal/service`.
4. Exact no-DB filter for the three canonical pure information top tests: 3 top /8 named, one run and one pass each, no missing/unexpected/duplicate/unfinished/fail/skip.
5. `gofmt -l` for those same three tests, with empty output.

After formatting, every command's before/after file hashes and modes matched. Production paths, migrations, public declaration/wire references and all protected runtime snapshot/source/decoder files match R01 bytes/modes. No canonical sqlc regeneration was needed or run because no query or schema changed.

The private read-only intake parser initially had a syntax typo and one speculative missing-file lookup; those tool errors are retained as operational mistakes, not product RED. The successful diagnosis reads use actual fixture/SQL/model paths and the original complete JSON stream. No test was rerun to reconstruct the R01 failure.

## Main handoff and limits

`intake/block-server-freeze-r02.json` binds the repaired manifest, exact R01→R02 three-file delta, unchanged public wire, named inventory and fresh compile/static result. Main must provision new package/mode databases and run the complete 16-top/43-name service normal/race inventory on this candidate, preserving R01's database and result. Handler/migration and actual API/Web/Desktop/block/repair/history journeys remain separately required.

This owner claims **test repair and static/pure verification only**. Database GREEN, normal/race acceptance, independent check, current-client integration, signing-key rotation and full S3 are pending. TC-16 and the parent24 matrix are not changed by this diagnosis. No task status, archive, commit, push, dependency, real account/model, provider or production release action occurred.
