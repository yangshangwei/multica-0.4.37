# Direct source-role and iteration request boundaries — 2026-10-21

Status: implemented and current serial database race verified; independent review and isolated commit proof remain coordinator-owned.
This executes the four exact evidence gaps in the prior immutable
`source-iteration-final-review.md`; it does not change the existing task/plan
or promote acceptance.

## Change boundary

Only the new `server/internal/service/collaboration_source_roles_test.go` and
`server/internal/handler/collaboration_iteration_request_boundary_test.go`
are product-adjacent changes. Existing product code and fixtures are read-only.
The canonical service owns role/reason/business-state assertions; the handler
owns strict dedicated PUT wire rejection and legal PUT/privacy. No production
abstraction, dependency, scope authority or rollout change is introduced.

## Direct source roles

`TestCollaborationSourceStructureAndIterationRejectNonManagers` explicitly
exercises `EditIssue` with project/parent/stage (and a companion title) and
`ChangeIteration`, preserving their actual recorded operation kinds. Roles:
source-only ordinary member/owner, full external participant, full source
participant, external handler, departed source handler and departed source
creator. Each reader retains the expected independent read/participation basis;
none has source-editing authority. Typed minimal current references remain
available, with no private parent body, unselected source objects or routes.
Nonmanager FieldOptions is forbidden; external/departed managers get no
SourceOptions. Exact persisted source/business/execution snapshots remain
unchanged across each direct mutation refusal and identical key replay.

Snapshots include issue/collaboration/participants/members, source iterations/
membership/events, handoffs/notifications/inbox, ordinary tasks, batches,
immutable inputs/dependencies, plus tracked control/run/grant/proof data.
Operation rows are checked separately to retain the permitted refusal receipt
without treating it as a business write. The active-channel/in-flight run is
synthetic fixture data, not a provider process.

## Missing removal reason

`TestCollaborationIterationRemovalRequiresReasonAndOriginalRefusal` runs both
source creator and current noncreator source handler. Public ChangeIteration
first joins the period. Missing and whitespace-only removal reasons must refuse
without changing membership, history/control or retained AI rows. A changed
reason with the old key conflicts; one new explicit reasoned request removes
membership, records the actual actor/reason and advances only the prescribed
issue/iteration state. Original missing-reason keys and own GET recovery stay
identical after successful removal. Source membership does not grant or restore
AI control/resources.

## Dedicated iteration PUT

`TestCollaborationIterationPUTRejectsMetadataBeforeEffects` makes actual PUT
requests for `/api/collaboration/issues/{id}/iteration`, with extra title,
project_id, status and stage respectively. Each strict 400 invalid_request
must occur before issue/membership/operation/event/AI effects. A legal separate
PUT then commits once and exact key replay adds no history. Private replies are
no-store, strip ambient workspace headers and disclose no supplied private
body/title or account email/username. Current external detail retains only
minimal project/parent/iteration references.

## Verification so far

`source-role-static-first-20261021-{1,2,3}` retain the first actual compile-only
service, compile-only handler and `go vet -p 1 ./internal/service
./internal/handler` results. All return0 with empty diagnostics and no database
access. Later assertion coverage was strengthened to include immutable batch/
input/dependency snapshots and preserve expected membership activity timestamps.
Current gofmt is clean. The allocated database verification is now recorded below; no browser/build/provider
operation was run by this lane.

Focused selector:

```text
^(TestCollaborationSourceStructureAndIterationRejectNonManagers|TestCollaborationIterationRemovalRequiresReasonAndOriginalRefusal|TestCollaborationIterationPUTRejectsMetadataBeforeEffects)$
```

## Remaining bounds

The full AC80/81 judgment belongs to the coordinator after actual execution and
independent review; counts are not acceptance. AC85 name/avatar/external-marker/
Unknown and drag behavior remain separate P6 client evidence. This lane does
not establish new Web/Electron, provider, kernel execution or installation
coverage. The previous final review artifacts remain unchanged.

## Actual allocated database execution

The coordinator allocated exclusive task-owned `service_s5` (schema737) after
mention-admission released it. Both commands ran serially under Go1.26.6,
darwin/arm64, the repository agent CLI guard, `-race -p 1 -parallel 1
-count=1 -timeout=30m`. The slot was released immediately after adjacency.
There was no failed DB assertion, skip, fixture retry or product fix.

From `server/`:

```bash
GOTOOLCHAIN=go1.26.6 python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/with-db.py service_s5 \
  bash ../scripts/go-test-with-agent-cli-guard.sh \
  go test -json -race ./internal/service ./internal/handler \
  -run '^(TestCollaborationSourceStructureAndIterationRejectNonManagers|TestCollaborationIterationRemovalRequiresReasonAndOriginalRefusal|TestCollaborationIterationPUTRejectsMetadataBeforeEffects)$' \
  -p 1 -parallel 1 -count=1 -timeout=30m
```

`source-role-focused-first-20261021-{command,result}.json`, `.jsonl` and
`.stderr` retain the actual focused command/results. All three top-level
identities pass with44 pass events including subtests, no failure/skip; wall
31.390s (service4.903s, handler2.377s). All seven explicit roles pass each
project/parent/stage/iteration direct refusal, exact unchanged snapshot and
original operation-kind/refusal check. Both source creator and source handler
pass missing/blank removal reason, changed-key conflict, fresh reasoned removal
and unchanged original refusal/recovery after success. All four metadata keys
are rejected on actual dedicated PUT before side effects; legal PUT/replay
commits exactly one membership/history without unrelated source/AI mutation.

The current canonical adjacency command is recorded verbatim in
`source-role-adjacent-first-20261021-command.json`. It selects these three
identities plus the prior deterministic transfer/departure/target-revision races,
external iteration statistics/history, source fields/creation/iteration, strict
PATCH, source catalog and ordinary mixed-move/rollover controls. It passes all
22 top-level identities,95 pass events including subtests, no failure/skip;
wall23.399s (service11.488s, handler2.820s). Exact raw PASS identities/lines
and raw/result hashes are in `source-role-final-scope-20261021.json`.

Final `gofmt -d` and `go vet -p 1 ./internal/service ./internal/handler` pass
with empty output against the final hashes. Earlier compile-only commands
passed as well; current race package compilation is final-source type evidence.
All nine protected existing product/helper hashes and both prior immutable
review artifacts remain identical.

| Final new file | SHA-256 |
| --- | --- |
| `server/internal/service/collaboration_source_roles_test.go` | `f006430da597c8a62209f72b8f04c32d9104df2f13886b87e8f9b9ac69fe694d` |
| `server/internal/handler/collaboration_iteration_request_boundary_test.go` | `7a081b2d5a46b11299b343ca8fcbb6cc5a13372bc77013abb9098fd14b8923a0` |

## Clause handoff

All four residual clauses from the previous review now have their own direct
current executed assertions. The coordinator may combine them with the prior
AC80/81 canonical/race/external-handler evidence for independent whole-clause
review. This implementation lane does not set acceptance status or commit;
exact isolated HEAD+only-two-file verification and final Lore staging are
coordinator-owned.

The original review remains a truthful immutable record of its then-pending
clauses. AC85's nullable external iteration name/Unknown UI boundary,
Web/Electron/installer/provider/OS scope and synthetic run-fixture limitation
remain separate. No spec/interface/production behavior was changed here.
