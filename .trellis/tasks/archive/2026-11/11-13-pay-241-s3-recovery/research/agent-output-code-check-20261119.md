# Agent output code check — 2026-11-19

Status: **PASS — CODE / FUNCTIONAL REVIEW AND FINITE HARNESS PREREQUISITES. Live Web/Desktop2top16 acceptance is not included.**

The initial independent binding matched all 31 owned server WORK paths and all seven client WORK paths to the implementation reports. Subsequent local assertion fixes and finite proof additions are recorded below. Source and final database-profile review are complete, with no remaining code or harness prerequisite blocker. Main owns runtime execution, integration and live acceptance; native/current-client evidence remains separate.

The current code audit and verification follow the retained review history below. The preceding planning contract report and all original failure/implementation reports remain unchanged. No S3 requirement or parent test case is closed here.

## Early fixed finding K01

The two direct-handler human/termination assertions expected 403, while the unchanged narrow bearer decoder returns 401. Only those two expected constants were corrected in private WORK. The immutable real-router 403 test is unchanged.

Before hash: `e36d8fcbaa405831bf0f90f3e2b1a6d0ce13b68fa73fe5509d25aca92e80f91a`. After hash: `d559c24776cc62f58c221ed580e49b905d5e62a6d433a4e7486eecf839fc161d`. Main must execute/freeze the corrected source for handler functional evidence; no database test was run by this reviewer.

## Early finding K02 and fresh client checks

The retained first handler profile also proves the grant-withdrawal byte request returns 409 `grant_required`, while its test expects 403. The following original upload-replay assertion has the same incorrect expectation and was not reached. This is reported to Main and awaits the existing WORK owner handoff before a local assertion fix. Product authority should not be changed to satisfy the fixture.

Independent reviewer whole-Core/Views type checks and scoped lint passed under fresh `output-review-*-r01` labels with zero source/config/tool drift. The existing 222-name client functional profiles remain source/hash checked; no database test was run by this reviewer.

K02 fixed after Main explicitly assigned the two direct-SQL grant-refusal assertions. Both now require 409 `grant_required`; plaintext and attachment-header guards remain. This does not assert the response of public RevokeGrant/capability withdrawal. The combined handler test hash is `e0d9566bf2b7d50400eb25e7809a7749c6633fa4f7a72acf030aa588d7613150`; first failure/source evidence is preserved, and a fresh Main-owned source-qualified handler run remains required.

## Finite proof additions K03

Main assigned the two previously reported O08 proof gaps. Existing service tops now contain `full-bound-original-replay` and `prepared-output-loses-to-outcome`: exact original receipt/capacity at32, and a real prepared ciphertext whose original outcome commits before the final output transaction, followed by exact loser cleanup and unchanged frozen refs/notification/public events. All28 new top-level names are unchanged; the expected service named denominator increases from37 to39. Neither new DB body has been run by this reviewer.

## Current code audit and source binding

Review covers the exact31 private Go paths and seven private client paths, the immutable Main route test, and their original behavior boundaries. The final31 Go paths plus the Main route test match `source/server-green-r03` byte-for-byte,32/32. All seven client paths remain unchanged after the client implementation report. ROOT product/spec/task/git, frozen sources, separate E2E sources and other WIP were read-only.

Frozen Go source contains5,089 records. `intake/server-green-source-r03.json` SHA256 is `32c998c8f19a00953bd9d75e976f3e49ccf720a0b0662b24f7ecb34a96fbcdde`; freeze record SHA256 is `5fe5c1ed7a4a856e0214c3fd7e65d4004a7ac0072d8b276797471a5c0749f361`. Main's exact fresh normal/race permit is `intake/output-package-execution-permit-r03.json`, SHA256 `070b05e872e37993a4e4c7e8fdd11af9d02dddc83415c6b66c89abed29a59125`. All paths are below `/Volumes/artisan/.multica-acceptance/pay241-s3-output-20261119-tm89sjgy`.

### C01 — All stored-outcome consumers and original ACK

The internal flat `collaborationStoredOutcome` in `collaboration_runtime_output.go:42` embeds the unchanged original request and adds only server-owned output IDs. Its decoder checks strict object shape, unknown/duplicate fields and end of input, and rejects null/duplicate/noncanonical/zero or more-than32 output IDs. Legacy absence remains absent; an explicit stored empty list remains distinct on read. No-output marshaling omits the extension.

Every actual stored consumer uses that decoder: both outcome replay/ACK branches, both records metadata/item paths, checkpoint/retention/seal, outcome-backed test-report logs and handoff result text. The incoming HTTP outcome decoder and Provider/Runner/ledger wire are unchanged and cannot accept caller output refs. Both ACK comparisons use the complete original `CollaborationRunOutcome`, including record/identity/attempt/body, while ignoring the server-owned ref extension. The original issued-capability/termination/attempt branch remains content-free after withdrawal/settlement. New or changed outcome requests still pass their separate current-authority/conflict gates.

### C02 — Comment identity, required writes and storage ownership

`collaboration_squad.go` has one saved input with an optional full fingerprint and one borrowed locked comment writer. Original `comment:<UUID>`, actual Agent author, squad delegation/continuation, wake/public-event/notification effects remain on that path. JSON has no file or invented attempt; no-file event bytes remain unchanged. JSON/multipart reuse conflicts in either order. Matching output replay compares full input, actual current attempt/row and original receipt metadata, including timestamp instants.

The output writer checks current mcb/admission/reservation and actual attempt before storage, prepares independent private ciphertext outside locks, then rechecks the same authority and attempt inside the final transaction. File row, permanent issue ref, comment, comment ref, event/receipt and existing side effects commit together. Caller fields supply no Agent/run/attempt/source/draft/key/digest authority; MIME is sniffed from actual bytes. The exact file fingerprint, body, parent and server attempt determine replay compatibility.

Object ownership resets at each internal callback. Only an inserted object with `collaborationRuntimeCommitError` is retained on uncertainty; known losers/replays/rollbacks delete only their own random object. Current human/resource/input/grant fences remain in `lockRuntimeBusiness` and the original source/group admission. The output cannot impersonate a human, change stage signatures, authorize formal acceptance or replace parent/resource consent. Original `author_type=agent` and `dispatch_eligible=false` are retained. No second writer, ledger, dependency or automatic fresh key was introduced.

### C03 — Real Agent rows through metadata and shared clients

Migration868 adds nullable Agent/run/attempt provenance and an exact human/Agent XOR/private delivery constraint. Existing human rules remain. Agent output has null human/draft/intended-upload fields, complete Agent/run/positive attempt, encrypted permanent issue delivery and no discard. There is no FK/cascade/index/dependency. Locked NOWAIT downgrade refuses retained or concurrently written outputs before restoring human constraints.

The shared stored validator, single public metadata DTO, sqlc attachment/delivery/preparation scanners and model carry the provenance together. Human extension fields are omitted; keys/digests/storage URLs remain private. Current authorized humans use existing issue/native capability reads, without machine credentials.

Core preserves human absence/null compatibility, accepts real nonzero Agent/run and canonical int64 attempt with null human identity, and rejects mixed/half provenance. Comment/result records retain bounded optional original output refs; old input refs keep their separate existing variant. The API's only change is the human-draft null guard: a wrong-kind response stays unavailable/unconfirmed and cannot throw or retire the original intent.

Views reuses `CollaborationCommentAttachments` for input/comment/result with original protected queries, previews and downloads. Input-specific labels stay input-specific. Existing issue-subtree realtime invalidation includes the attachment query; public comment reads already project the committed references and actual Agent summary. Legacy records without refs issue no file request. No new local state/store/platform fetch is introduced.

### C04 — Exact32 and original output/outcome ordering

Count/insert are scoped to the exact run/current attempt under the existing run lock; replay precedes the capacity check. Unique33 refuses and cleans its prepared loser. Issue history and earlier attempts are not globally capped. Outcome validates and freezes the complete committed current-attempt set ordered by `(created_at,id)`, with no LIMIT, later-file inference or truncation.

The writer checks business outcome before storage and after the final lock, including existing squad activity closure. Original replay remains subject to current authority. Terminal/forced/old-attempt gates remain unchanged; explicit new work retains old failures/results/files. The two added subcases explicitly prove full-bound replay and a prepared object losing to outcome commit in Main's forthcoming DB denominator.

### C05 — CLI containment and complete response correlation

Only the original narrow comment operation accepts one explicit file with its bound issue and original request UUID. Local validation precedes any context HTTP request. A pinned CWD `os.Root`, nonblocking open/regular-file Stat and bounded read refuse URL/multiple/empty/nonregular/oversized/escaping paths; contained symlinks remain supported. This proves invocation CWD containment, not immutable run-root or OS containment. Overrides and human profile fallback stay unavailable.

One mcb multipart POST carries original comment/content/parent/file. `GetBody=nil` and redirect refusal prevent automatic follow-up. The corrected3xx wrapper retains the original retry UUID. Typed success requires complete fields and exact comment/Agent/run/name/sniffed type/length plus canonical file/source/attempt/version/times and valid derived IDs. Unknown, truncated or mismatched success stays unconfirmed; no journal/new key/auto-retry is added. Built-in skill and source-map touch points are present.

### Reader and reference boundaries

The current mcb is checked before file resolution. Runtime reads allow exact current Agent/run/attempt output with same issue/source/version/permanent row, or an exact original frozen input manifest reference with matching metadata. They grant no whole-issue fallback; old/foreign files stay unavailable unless independently frozen as current input. No-preparation unfrozen/missing IDs share `attachment_not_found`; real query/malformed-manifest errors retain their error paths.

The original real content handler repeats authorization after storage/decryption, compares the full row, then sets exact run/file/version headers before bytes. Human/native read stamps and final checks remain. Ordinary/public private-prefix exclusion is unchanged. Records project original IDs/body, never raw event fingerprints, storage material or capabilities.

## Other owners' fixes reviewed

The designated Go owner fixed the original307 unconfirmed-error classification in `internal/cli/collaboration_output.go` (hash `bcb9f3f392e752519e39a4612cb9c372f63cba16ea7790b8d2259d26ede14ecf`). The current original redirect test now passes; the first failure remains retained.

The same owner changed only the explicit-new-run positive fixture to the existing supported host claim/permit/launch-termination/final-proof path in `collaboration_runtime_output_host_test.go` (hash `4d5b82859dfde15030701860688976c09c8b5e0f09cdd537815ab401db17822c`). Existing legacy unsupported-file refusals were not weakened. Its DB proof remains pending. This review attributes those fixes to their owner.

## Unaffected-control map

All eight original service control files are byte-identical to allocated BASE. Their53 unique tops plus17 new tops produce70 combined service tops. The JSON contains exact file hashes and each original name/line.

| Original service control file | Tops | Relevant original behavior |
| --- | ---: | --- |
| `collaboration_attachments_test.go` | 7 | Human encryption/scope/current authority, unknown/concurrent upload, atomic refs and retained command/comment files |
| `collaboration_checkpoints_test.go` | 3 | Recording bounds, outcome overlap, Stop/content-free ACK and internal event filtering |
| `collaboration_records_test.go` | 4 | Frozen paging/retention, current read authority, malformed/absent/unclaimed data |
| `collaboration_runtime_handoff_test.go` | 4 | Original assignment/wake sources and memory purge/base semantics |
| `collaboration_runtime_preparation_test.go` | 17 | Original manifests/versions/commit, legacy no-file/unsupported-file controls, configuration/revocation and histories beyond32 |
| `collaboration_runtime_test.go` | 4 | Batch/attempt/outcome/proof, Stop/Resume and authority removal |
| `collaboration_squad_test.go` | 10 | Original Agent comment delegation/capacity/authority, human joins/unread cancellation and activity/proof separation |
| `collaboration_test_report_test.go` | 4 | Actual claim/report/typed subject/member write fence and settled-success prerequisite for QA evidence |

Main's fresh normal/race runs must execute these controls. Prior greens are not substituted. Client controls add95 Core and24 Views names, already reconciled against their raw final JSON/current source.

## Verification

All reviewer commands use the recorded helper and closed no-DB/client configurations with new labels and no inherited ambient environment. Exact commands and hashes are in the JSON.

| Reviewer labels | Observed result |
| --- | --- |
| `output-review-gofmt-r01`, `output-review-format-check-r01` | Owned tests formatted/all owned Go files checked; exit0, empty output |
| `output-review-compile-r01` | Genuine `go test -c`, six affected packages, exit0; no DB/test body |
| `output-review-vet-r01` | Same six packages, exit0 |
| `output-review-internal-cli-r01` | 4 tops/41 named PASS; zero fail/skip/duplicate/unfinished |
| `output-review-cmd-cli-r01` | 3 tops/10 named PASS; zero fail/skip/duplicate/unfinished |
| `output-review-core-typecheck-r01`, `output-review-views-typecheck-r01` | Whole Core/Views, no emit/incremental cache, exit0 |
| `output-review-core-lint-r01`, `output-review-views-lint-r01` | Exact seven client paths, max warnings0, exit0 |

Every reviewer record has zero source/config/tool drift. Existing client raw JSON was independently reconciled:100 new Core schema/recovery +3 shared Views +95 Core controls +24 Views controls =222 named PASS, with no pending or duplicate case and matching hashes. The desktop download bridge remains synthetic.

- Lint: **pass**, scoped client files and owned Go formatting.
- TypeCheck/compile: **pass**, whole Core/Views and six Go packages.
- Static analysis: **pass**, Go vet.
- Pure tests: **pass**, reviewer51 Go CLI names and independently reconciled222 client names.
- Go database tests: **pending Main's final package/mode/source evidence**. Earlier partial greens and compilation are not functional acceptance.

## Findings (not fixed) and limits

No additional source violation remains. No further WORK mutation is planned. Main still owns fresh combined DB normal/race/route/migration results, full name sets, gate/catalog/history/connection cleanup, integration and actual Provider/production Web/Desktop evidence. A direct SQL grant409 control is not public credential withdrawal403; protocol messages/mock bridges are not native process/IPC proof.

After integration/acceptance, Main should sync the authoritative content/runtime-recovery and Core/Views provenance/record specs. ROOT spec/task/journal/git are outside this reviewer's ownership. This does not add a product decision or acceptance gate.

Original failures remain: first service16/17 tops36/37 names; first internal CLI3/4 tops39/41 names; first handler0/2 tops11/15 names, including actual401/409-vs403 receipts and its unreached replay assertion. Route RED, reader RED, old failed sources/databases and previously unconsumed races remain separate. No result is spliced across rounds.

Reviewer source changes are limited to the two recorded test files. The Markdown/JSON are the only ROOT writes. No production Go/client code, database, migration, runtime/Provider/browser, spec/task state, journal, Git or other WIP was changed by this reviewer. Earlier accepted subscriptions/human attachment/controlled runtime scopes retain their independent dispositions; R02/R09/full S3 remain open.

## Actual final router correction and readonly harness review

The final real router normal run returned401 `invalid_collaboration_capability` for anonymous/human credentials, rather than the original test assumption403 `business_capability_required`. Actual business routes are direct registrations; managed claim/permit middleware does not wrap them. The reviewer’s earlier middleware attribution was incorrect. Main’s original test remains read-only here; K04 asks its owner to correct the expectation and preserve both original404 RED and this actual401 failed round. Valid revoked mcb tokens still have their separate403 contract.

Fresh service normal/race each70tops/119names pass, including identical39new and80old-control names. Handler/migration each2tops/15names per mode and internal/cmd CLI4/3tops with41/10names per mode pass against exact permits/source/config/raw hashes. Router correction, its fresh normal/race and final resource audits remain required.

Readonly E2E review found H01: successful multipart writes are201 but observer/replay checks expected200, so intended ACK loss and winning receipt retention never occurred. Main assigned only output status fixes to the original owner. H04 likewise requires the actual wrong-credential401 contract in its human writer control. H02 recommends actual late UUID in the returned wire identity; H03 recommends checking response observation before capability expiry. No harness source/runtime was changed by this reviewer.

The two-platform plan is internally consistent:2tops/16cases/10physical Providers/12actual CLIs/10Agentrows/20issue+comment refs/6humanfiles/4server outcomes. SixStop/author/grant runs explicitly have no stored outcome/result refs. Existing-winner HTTP replays are labelled separately from actual CLI/Provider counts. Actual metadata/comment keys, frozen prior outputs on continuation, DESC3-command history, original public withdrawal and hashed PID/token/program/storage evidence were checked against source. This is fixture admission review, not runtime acceptance.

## Resumed final source and harness prerequisite reconciliation

Main’s route-only r04 assertion correction matches actual401/`invalid_collaboration_capability`; original r03 product remains unchanged. Fresh corrected BASE RED and router normal/race are still Main-owned. Original failed route rounds remain retained.

All H01–H04 harness corrections are now source-verified in readonly WORK R07, fixture SHA `f75c2f9ec631f311ef205d64efbc714751198264d8ee80b0b63ce067c0ff8e15`: multipart201, actual fresh late UUID, after-body observation before capability expiry and wrong-humanJWT401. Valid/revoked mcb403 stays unchanged. R06/R07 target TS/lint/list each3pass with raw/source/config/tool hashes checked. No new harness source blocker remains; final frozen copy binding is pending and the actual2top16case suite remains unexecuted.

Both service after-audits confirm119body/39new/80old names, restored original gate booleans epoch3,856ledger rows,663ready-valid indexes, zero output/comment/run/capability/reservation effects and connection0. The documented synthetic organization/handoff/operation/group/invalidation histories are retained under the coordinator’s accepted control scope; they are not classified as unexpected production output effects.

## Final independent disposition

**PASS: allocated Agent-output code, owning-layer functional evidence and finite harness prerequisites.** All prior pending/failed checkpoint entries above are retained history; this is the current disposition. The live2top16 suite was NOT_RUN at prerequisite handoff and no live output receipts are accepted by this report. Main may execute the already authorized finite run separately. R02/current-client/R09/full S3 remain open.

| Final owning package | Normal tops / names | Race tops / names |
| --- | ---: | ---: |
| Service, including53 original controls | 70 /119 | 70 /119 |
| Handler | 2 /15 | 2 /15 |
| Corrected actual router | 1 /3 | 1 /3 |
| Migration | 2 /15 | 2 /15 |
| Internal CLI | 4 /41 | 4 /41 |
| CLI command | 3 /10 | 3 /10 |
| Total | **82 /203** | **82 /203** |

All12 final profiles were independently checked against source, command, config, raw hashes, run/terminal inventory and expected top sets:164tops/406named PASS, zero fail/skip/duplicates/unfinished/build failure. Service includes the same39new and80old names in both modes. Existing client222 named passes and scoped lint/type/compile/vet evidence remain valid.

Five packages have literally identical raw mode lists. Internal CLI has exactly one absolute temporary external-file subtest per mode. The two exact raw names map to `TestCollaborationOutputO09FileReadUsesProcessCWD/absolute-external-file-from-TestTempDir`, pinned to unchanged source SHA `39bbc02ee135e1f25fd041779eba8db60637e24380fac4376347fe68ddff8160`, creation line41 and t.Run line66. Each exact raw name appears once. All other40 internal CLI names are literal matches. No broad normalization or test rerun was used; raw lists and the initial summary diagnostic remain untouched.

Main corrected its route test only to actual401/`invalid_collaboration_capability`. The same corrected test on BASE has the expected404 missing-route RED:1top/3named FAIL. Fresh corrected router normal/race each1top/3named PASS. Original403-assumption failure rounds remain preserved. Product code is still r03; r04 changes only the owned route expectation.

Final service audits preserve actual test-end flags, restore original booleans at epoch3 and close connections0. Handler, migration and corrected router final audits preserve original gate epoch2,856ledger/663ready-valid indexes and connection0. Migration public table counts are unchanged. Documented synthetic organization/handoff/operation/group/invalidation history is intentionally retained; output/file/comment/run/reservation/capability effects are clean. No failed evidence or retained history was erased.

Final frozen harness is `source/client-output-run-r04`,4136records, manifest `4de74f70a2a67c5f0ce34d1b86c7fcb213cddb91514935cf5cb361eda3af734e`. Its four owned source files match final WORK; four shared product bytes match validated WORK and production source. Fixture SHA is `f75c2f9ec631f311ef205d64efbc714751198264d8ee80b0b63ce067c0ff8e15`; Provider SHA is `dd97c9266823a1fe6617e760306d18e17165f67fd2f981ae8a93f7fa4aca2f89`. All H01–H04 corrections and R07 static target TS/lint/list evidence are source-verified, and the legacy runtime helper bodies remain unchanged.

The finite permit pins2tops/16cases/10Providers/12actual CLIs/10Agentrows/20issue+comment refs/6humanfiles/4server outcomes, with6Stop/withdrawal outcomes absent and recovery launches0. Its four executable hashes and closed config hashes were independently checked. API signing material is present only in the API config; other service/browser configs carry no signing value, and model credential values are empty. The fresh live admission is856ledger/663ready-valid indexes, empty business facts, closed execution/host gates epoch2 and connection0. Main subsequently began reversible owned API/Web/renderer readiness; this is not a live-test result.

No remaining code or finite harness prerequisite blocker is known. Reviewer source writes remain exactly the two scoped test files; Main’s route and original-owner CLI/host/harness fixes are attributed separately. Reports are the only reviewer ROOT writes. ROOT product/spec/task/journal/git and all unrelated WIP remain outside reviewer writes. No database or native Provider/browser test body was run by this reviewer. Main owns integration, spec sync and the separate whole live-output acceptance.
