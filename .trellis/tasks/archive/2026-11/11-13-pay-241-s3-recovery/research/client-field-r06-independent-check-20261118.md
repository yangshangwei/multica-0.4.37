# S3 R06 current-client field increment independent check

**ACCEPTED_CURRENT_CLIENT_FIELD_AND_ORIGINAL_RECEIPT_INCREMENT_R06; blocking findings 0.** One fresh four-case run passes **4 tops / 8 ordering subcases**, one attempt each, retry0, fail0, skip0. The exact case ledgers match retained PostgreSQL fields, operations, events and original-request recovery. This accepts the bounded current password API/production Web/matching Electron increment. Keep **TC-16 PARTIAL** and full S3 in progress: ordinary field UI retains optional-version semantics, and batch/notification/upload/revocation/blocking remain separate requirements.

Active task: `.trellis/tasks/11-13-pay-241-s3-recovery`; reviewer `/root/s3_clients_check`. The reviewer used saved raw execution and main-owned SQL/process evidence, rehashed the frozen harness and ran two independent no-database static checks against that frozen harness. It changed no product/test source, old evidence, matrix, Git index or peer WIP; it performed no DB/API/browser/Provider/TestMain/build/process-control execution.

## Findings (fixed)

The exclusive implementer repaired the test harness; all earlier actual failures remain sealed.

- Original pending UI intent is recovered through the real Check original result action when present, matched to its original key/observations/result, and retired by the application owner. The restored refusal form closes through its actual control before a new intent. The exact visible button is found across the portaled dialog and shared section, without forcing a click or changing storage. In successful R06 normal refused requests retire directly, so the conditional second recovery branch is accurately recorded as **not required**; only the deliberately lost winning response requires the actual UI recovery.
- Safe request/response diagnostics retain origin, allowed header names/presence, canonical operation metadata and error codes; they do not retain raw Cookie/Bearer/CSRF/auth frames. Checkpoint payloads are cloned at capture, so later teardown does not rewrite an earlier lifecycle observation.
- The final `HeldRequest` forwards a nonlost request with `route.continue()` and reads the response belonging to that same browser request. Only deliberate ACK loss uses actual upstream `route.fetch()` followed by abort. No winning/refusing body or business header is fabricated. Every held request and response waiter reaches a recorded terminal/cleanup path.

The retained six-case unauthenticated `/health` transport probe explains this last change within the task harness: natural and continue retain GET200/PATCH405, whereas fetch/fulfill gives client status0 despite identical upstream/body bytes. It neither authenticates nor writes business data. This proves the local harness transport distinction, not a production CORS, permission or native-account defect. R01's masked primary exception remains unknown; R02–R05 failed runs are not retroactively passed.

## Complete actual run

| R06 case | Duration | Actual orderings |
| --- | --- | --- |
| Web priority/handler |11.338s|priority-first and transfer-first PASS |
| Web ordinary child numeric API CAS / UI compatibility |10.785s|priority-first and assignee-first PASS |
| Desktop priority/handler |19.962s|priority-first and transfer-first PASS |
| Desktop ordinary child numeric API CAS / UI compatibility |14.665s|priority-first and assignee-first PASS |

Reporter and per-case inventory agree: four expected tops, eight started/passed orderings, one result and retry0 per top, no run errors/flaky/skip. Runner exit0. Report elapsed58.800s is distinct from individual case durations. No passing case from the first five failed rounds contributes to this R06 acceptance.

Each collaboration pair captures two genuine UI requests at collaboration revision4 before either forward. Priority-first commits one edit200 and refuses the old transfer409 `stale_revision`; transfer-first commits one transfer200 and refuses the former noncreator handler edit403 `forbidden`. Only the winning field changes. The winner's actual response is lost and the exact original UI GET resolves it without POST. Both own GETs match the actual original POST's canonical code/status/revision.

A deliberate fresh authorized UI intent uses revision5 and a distinct key, producing final priorityhigh/current creator handler/collaboration revision6 and ordinary revision5. Both earlier success/refusal GETs remain exact after that new intent. Former-handler old POSTs still403, and another actor's own-operation GET404. Original same-priority key200 and changed-priority key409 are independently retained in priority-first. Exact two added business events are `issue_edited` and `transferred`; no extra execution is created.

Ordinary children use explicitly supplied **numeric API CAS**, both original requests at revision1: one200/one409 with exact `revision_conflict`, resource type/ID and numeric expected1/actual2. A fresh API request applies the remaining field at revision3. Two actual unmodified picker writes omit `expected_revision`; they end at revision5, prioritymedium and creator member assignee. Those UI writes prove compatibility and resulting projection, not mandatory UI CAS. Parent collaboration business/activity/execution snapshots are unchanged.

## Persistent identities and actual relationships

Main's corrected read-only snapshot contains **8 roots, 4 ordinary children, 8 collaboration/session rows, 36 workflow operations, 40 collaboration events, 12 handoffs and 20 ordinary activity rows**. The reviewer compared every ordering ledger with these actual rows:

- All original/fresh collaboration operation keys resolve to one completed actor/root operation with matching kind, code, status and revision. Operation IDs/payload hashes remain sealed; original result recovery is not recomputed after new authority.
- Final root/child field/revision/handler/parent identities match their DTOs. The exact added event IDs map to the same issue/kind and genuine handoff facts; ordinary children retain their own five activity rows (creation plus four accepted field updates).
- Eight sessions remain paused, control version1/target set1, and **run_count=0, command_count=0, dispatches=0** through the actual `collaboration_run.session_id` / session→issue and ordinary activity-log relationships. No parent-row-absence inference is used.

The first speculative audit query used nonexistent aliases and failed read-only; its failure remains saved. Main corrected it to actual `activity_log` and session joins without writes/replays. Reviewer parser formatting/attachment-selection errors likewise remain separate private failed receipts; corrected parsers only reread the original immutable JSON, never rerun tests.

## Source, configuration and resource verification

R06 uses the frozen R09 fixture SHA256 **`e11aa601afe3bb82d245a6af04c67a99587fbc5f0c4964f284392cd4852d74d5`** and unchanged spec SHA256 **`10cac566342deb24c68e90fd15cdcfd1c8a6aec50c9572116dfbb683c77b67ca`**. Every declared `harness-r06` file/mode matches its manifest; execution before/after source snapshots are equal. Actual browser configuration SHA256 `e4dfdac8a29796364d567bf63387da0208c7358fa706f156070ddf36009f74e1` matches command/result and raw stdout/stderr hashes. The private harness and the separately built production source remain distinct identities.

Independent strict TypeScript and explicit two-file ESLint checks ran against **harness-r06**, with trusted task HOME and only PATH/LANG; both exit0 and frozen before/after hashes remain equal. TypeScript includes the resolved graph without emit/incremental state. ESLint uses the existing Core React rules and documented root/Core/Views declaration owners; it is a real `--no-ignore` two-file check, not whole-ROOT lint. No dependency was installed or changed. Raw commands/configs/tools are bound in `independent-check/client-runtime-r06/static-r09/`.

Main stopped owned API **PID63612**, port64930 empty, connections6→0. Saved before/after shutdown snapshots retain every211 table count, every853 migration entry,663 indexes and original gate exactly: **creationtrue, executionfalse, hostfalse, epoch2**. All five product artifacts remain unchanged. Native case lifecycles record exit0 and completed close for all10 native apps; main's exact-fixture process inventory finds none. Production Web and renderer remain intentionally running for the next increment. This is scoped owned shutdown/retention proof, not a claim that legitimate testcase writes never occurred or every unrelated process stopped.

## Findings (not fixed)

No blocking defect remains within this field increment. Scope limits remain material:

- Ordinary old/UI requests omit optional aggregate versions; this increment does not enforce same-version conflict for every ordinary client. Therefore retain TC-16 PARTIAL while adding this exact current-client/API evidence.
- The run starts new empty execution lanes; preserved empty command/run state proves zero new execution here, not recovery of every older live/failed run. The accepted service CAS/timeout evidence keeps its own original scope.
- 55-comment/real reconnect, mixed notifications, original upload and revoked indirect reads, structured block/repair/history and full R01–R09 acceptance require their own actual increments. Native scope is real Electron with matching renderer/preload and synthetic password accounts, without installed Main or real-model conclusions. Real S4, final S5 and business signatures remain open.

## Verification

- **Current field tests: PASS**, exact4 tops/8 orderings/retry0; original5 failed rounds preserved and excluded from the passing denominator.
- **TypeCheck / targeted lint: PASS**, fresh independent frozen-harness checks; source/config/raw and persisted identities PASS.
- **Owned API/native/gate/connection restoration: PASS**, exact saved relationships and final process evidence; Web/renderer held.
- **TC-16: retain PARTIAL; full S3: not accepted by this report.** Main may commit this bounded increment and then proceed to separately allocated batch recovery.

Machine-readable audit and raw links are in sibling JSON and private `independent-check/client-runtime-r06/`. No technical review work remains in this field slice; main owns spec/matrix/commit integration.
