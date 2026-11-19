# R22 runtime-capability GET / stage-evidence regression

Status: **GENUINE ORIGINAL-CODE RED ESTABLISHED; PRIVATE TWO-FILE GREEN READY FOR INDEPENDENT REVIEW AND TESTS.** ROOT source was not changed by this implementer. This implementer executed no Go test, build, list, database, API, browser or process command; the coordinator owns the actual observations recorded below.

## Genuine r03 RED and minimum private GREEN

The coordinator's fresh r03 result has the complete **4 top-level / 8 named / 7 leaf** inventory, exactly the two intended read coexistence failures, and passing original registration-writer plus all four authority controls. The held capability GET makes R4 fail with real user-lock `55P03`; the reverse original capability GET exceeds its five-second deadline while R4 holds the shared account fence. There are no skips, parser errors or source drift; required fixture tables are clean and other database connections are zero. Result SHA256 is `93de31afa3aa69fcb2520110bbabe59ff987dde19447b7815453611083b6b0a5`. The earlier setup-blocked `3716fbe0…` result and every source/profile remain retained.

The leader authorized the private account-only repair after that genuine RED. A separate `implement/source-green-r01` changes only:

1. `server/internal/service/managed_runtime.go`: the existing public `LockManagedRuntime` selects the constant-false writer branch of a shared private implementation. The private read-account branch retains password-session existence, Setup/Change restrictions, positive version and UUID parsing; it takes existing `LockArtifactUsers` for exactly one matching user, then repeats `CheckPasswordVersion` for that same current user/version/binding under the lock. Writer mode still calls the unchanged `auth.LockPasswordSession`.
2. `server/internal/service/collaboration_runtime_preparation_capabilities.go`: existing `lockPreparationRuntime` selects constant false for registration; **only** `RuntimeCapabilities` selects true. Original daemon authorization, namespace order, runtime/owner/binding checks, installation admission and `LockAgentRuntime` remain unchanged. The response, SQL, API, dependency and transaction/retry contracts are unchanged.

The original registration function and 15 unaffected existing function bodies are byte-identical. The managed namespace/runtime/binding/installation tail is byte-identical, and the credential-minting auth file is unchanged. The corrected test remains `d9535bfa…`; the other **5,045 files/modes** are unchanged. This proves a bounded source boundary, not a passing GREEN or the original R20 holder.

Current GREEN inputs:

- `evidence/green-source-manifest-r01.json`, 5,047 files: `97d31ea4942956cc3f1189788bb136af45012b40678ef94131e51d999bf64479`.
- `managed_runtime.go`: `325251ecb8a8adeaf3c8c3a97ef53302750fce9a8c163d579087e25f12288e96`.
- `collaboration_runtime_preparation_capabilities.go`: `e625a0286ff4e7f468f956145a149727b2c1836399d0afdeadc72320215f14b9`.
- Exact two-file diff: `evidence/green-two-file-diff-r01.patch`, SHA256 `abe29f0e74dba4213cf4b15ec47c5d66b5a3e412cf5f74a127af55fc359ac74f`.
- Writer/default/tail preservation: `evidence/green-writer-preservation-r01.json`, SHA256 `de7ee73a710272d8ab287dd790b18b23b90ecf9d570f267f295c1df8cc731cc2`.
- All unaffected function bodies: `evidence/green-existing-functions-preserved-r01.json`, SHA256 `e28ca2ac74f35b166446379a2d25dd6ff3a14b13c8ad4a032ec0fc4f445ca641`.
- Proposed exact follow-up: `evidence/green-test-plan-r01.json`, SHA256 `018b0d902c201fbcd2069e18470f62a0aab398d50adb2ecf381bfc859978656d`.

The proposed normal command selects the new **4/8/7** regression inventory. Combined race adds the preserved registration epoch/no-op, complete-summary/current-authority and unknown-commit controls, producing **7 top-level / 25 named / 22 leaf** records. Both retain `-json -count=1 -timeout=5m`; package vet follows. Main owns unused GREEN/RACE profiles and exact execution bindings. No 116-test rerun is requested, and overlapping observations do not change the business-case denominator.

Only `gofmt` and physical/source-function checks ran here. Independent review, actual GREEN/race/vet, guarded ROOT integration, matching consuming products, the original four cases and full S2 acceptance remain pending. The actual R20 lock holder remains unproved.

## Retained count-identity checkpoint: r03

The coordinator executed the r02 original-product command. Its complete 4 top-level / 8 named / 7 leaf inventory ended with all 8 named records failed during the fixture's side-effect count: `SQLSTATE 42703`, absent `collaboration_run.issue_id`. The raw caller anchors are test lines 104, 137, 171 and 215. None of the intended held read/writer assertions was reached. Result SHA256 is `3716fbe02686096cef2439c196c57b280ff157611fabc678dff6ff58eacfe431`; stdout is `805de1d17c7f24a73d933ce32c12da0bf84180f2ac6988b720899054e341d02a`. This is a preserved setup-blocked observation, not eight product defects or valid read-lock RED evidence.

The source DDL at `server/migrations/571_collaboration_schema.up.sql:82` defines runs by `session_id`. Production runtime queries also use that relationship; the actual creation/access fixture exposes its persisted `SessionID`. A new separate `implement/source-control-r03` changes only the count line to `SELECT count(*) FROM collaboration_run WHERE session_id=$1`, with `f.issue.SessionID`. The zero-run assertion and all 5,046 baseline files, modes, read/writer/authority assertions, PID checks, release ordering and five-second deadlines are unchanged. The valid 32→31 writer-summary correction remains.

Current frozen inputs:

- Corrected test SHA256: `d9535bfa9f9965517391fc3ba1255ed54970b6b249b049f5d0f79f67a4cc37ea`.
- `evidence/red-source-manifest-r03.json`, 5,047 files: `0798b58ba06ca14cc3fb7c3eab50b7c61fb4ea87069776c7c44a9668ff977c4f`.
- `evidence/red-test-inventory-r03.json`, unchanged 4/8/7 inventory: `46ee80a5cb6691106b79ed0fbf43b10676f5515e46e386f8c5bec78ccecb28e2`.
- `evidence/side-effect-count-correction-r03.patch`, exactly one corrected test line: `756169c6cce101313fbb8eb45a79fa87c7030db1120817151b765dd524a588f9`.
- Previous MD/JSON reports are retained verbatim as `evidence/pre-control-r03-implementation-report.{md,json}`; copy/retention receipt SHA256 is `7c3d2467b216b63ff53fce0d4f351a0bb0b3edf874f6437b8bc5af904ad56ef8`.

The coordinator has prepared the new explicit `multica_pay241_20261116_s2_capread_red_r22r03` profile. This implementer performed no allocation or database check. Execute it only after independent r03 boundary acceptance and a fresh command/source/environment binding; keep the used r02 profile, result, raw files and original sources. No GREEN is authorized by the setup-blocked r02 result.

## Corrected writer-control boundary: r02

The independent checker found that the original control's `FileCount++` changes 32 to 33. `protocol.preparationLimits` permits only 1–32, so that request would be refused before the real registration transaction and its user-lock barrier. The original frozen source and command were **NOT_ACCEPTED / NOT_EXECUTED**; there is no failed-run result to relabel.

A separate `implement/source-control-r02` copy changes only that test line to `FileCount--`, yielding a valid changed summary of 31. All original next-epoch, exact-summary, one-invalidation, read-refusal, PID, release and five-second assertions remain unchanged. Every one of the 5,046 existing R21 baseline files and all file modes remains identical; only the new test differs from the original test-writing checkpoint.

Current frozen inputs:

- Corrected 235-line test SHA256: `3af577c0c900da0ea2d457e1743ab9ef5cea4a109fa349e1db250988322a51f3`.
- `evidence/red-source-manifest-r02.json`, 5,047 files: `e71ad66bae0fa00454c1c71b450aa6244c9ae60d271ccb2ab34dc6cf5a0467f1`.
- `evidence/red-test-inventory-r02.json`, unchanged 4 top-level / 8 named / 7 leaf inventory: `1e7a17a6ee2c466966e26d964f62329a03d25049ad9a769828e8a67ec04a17f4`.
- `evidence/writer-control-correction-r02.patch`, exactly one changed test line: `8e919a74c738bbee8192a4bbcbaeeafc0638869567b2dc3ae655ba214a03c396`.
- Original MD/JSON reports are preserved verbatim in `evidence/pre-control-r02-implementation-report.{md,json}`. The copy/retention receipt is `evidence/control-r02-copy-and-report-retention.json`, SHA256 `23957d57dcba65ab008557f0d4868cac4319667477d10f70b2211773743c6896`.

The original `source-red-r01` test remains `d7965cbc…`; its source/inventory and unexecuted command are historical inputs. Main owns the new exact command binding and actual RED against the untouched selected profile. No product GREEN is prepared before a valid observed RED. The prospective repair remains limited to the GET's account fence; runtime, namespace, registration and other writer fences are outside that approved slice.

## Observable boundary

The real daemon `EnsureCapabilities` reads `GET /api/collaboration/runtime/capabilities` before deciding whether a capability registration is necessary. An identical summary returns without a POST. The GET itself calls `RuntimeCapabilities` → `lockPreparationRuntime` → `LockManagedRuntime` → `auth.LockPasswordSession` → `LockPasswordUser FOR UPDATE`. It performs no durable capability or cursor write.

An artifact-bound QA R4 reads that same current QA user through `LockArtifactUsers FOR SHARE NOWAIT`. The new counterexample tests these two real service reads in both orders. It does **not** assign the original R20 lock holder or certify a native Host process.

## Fixture and exact synchronization

The fixture reuses `newStageRuntimeFixture(true, stageArtifactReadFixture)` and `ownerSwitchNamespace` to create a persisted namespace, installation binding and daemon password session for the actual designated QA user. QA's real member role is made admin for the existing artifact-reader permission. A valid full capability summary is registered through the actual service, and fresh R4/capability reads prove the exact candidate, artifact, runtime, registration, epoch and account relationship before concurrency starts. The source issue has zero collaboration runs.

The namespace uses the existing managed cloud/Linux protocol fixture. No synthetic settled state or native execution is asserted. Capability/read authority is the same production GET path regardless of whether a process has already completed.

The existing passthrough `stageProjectionBarrierStarter` holds `LockAgentRuntime`, the actual next query after managed account authorization/user locking. The reverse read reuses `holdStageArtifactR4` with a local fixture copy whose reader context is the QA context. Both tests require different real PostgreSQL backend PIDs and finish the second request before releasing the first. Five-second contexts are unchanged; the second deadline starts first to avoid a false pass when the first read expires. Buffered result channels, `sync.Once` releases and cleanup drains preserve failure cleanup.

## New inventory and intended original RED

| Top-level test | Required behavior / intended old-code result |
| --- | --- |
| `TestCollaborationRuntimeCapabilitiesReadThenStageEvidence` | Held actual capability GET must allow the same QA's artifact-bound R4. Intended RED: real `55P03` at R4's current-user shared fence. |
| `TestCollaborationRuntimeCapabilitiesStageEvidenceThenRead` | Held actual R4 must allow the same QA's capability GET within five seconds. Intended RED: original capability GET blocks on its exclusive user lock until the second deadline. |
| `TestCollaborationRuntimeCapabilitiesReadOriginalWriterFence` | Actual changed-summary registration holds the original user fence; R4 must refuse with `55P03` and zero private projection. After release the writer must commit the exact next epoch/summary and one invalidation. Intended PASS. |
| `TestCollaborationRuntimeCapabilitiesReadCurrentAuthority` | Four leaves: actual password-version change, disabled account, revoked binding, and another real daemon's wrong binding. Each must refuse with `managed_runtime_required` and an empty capability response. Intended PASS. |

Static inventory is **4 top-level / 8 named / 7 leaf records**. The expected original RED has two failed coexistence leaves and five passing control leaves, plus the passing authority parent: 2 named FAIL / 6 named PASS. These are expectations, not observed test results.

## Original test-writing source checkpoint (retained; not accepted)

Private root: `/Volumes/artisan/.multica-acceptance/pay241-20261115-continuation/resume-r05/r22-runtime-capabilities-read-r01`.

- New `implement/source-red-r01/server/internal/service/collaboration_runtime_capabilities_read_concurrency_test.go`: 235 lines, SHA256 `d7965cbca8da0f034afa6527bde5cd2ec4b2594b01184fa8397876d482abaa11`.
- All **5,046 existing source files/modes** match the coordinator's copied R21 baseline. No existing file changed. The RED manifest contains **5,047 files**, SHA256 `f42869d3cde43ac80f091ab984c786161017eb3ebd3adefc29df59b57bb4884b`.
- `evidence/red-test-inventory-r01.json`: SHA256 `30160de581d174a2a87a9cd33e089d0ed4119c22dd0073eb189c93380f5f4837`, exact selector, named/leaf sets, intended terminals and barrier/deadline requirements.

Only `gofmt` ran. Static/source-byte inspection does not prove compilation or behavior. Main owns independent boundary review, exact fresh database/source/command binding and original RED execution. Any unexpected control or setup failure must remain preserved and be corrected in a new separately bound test-only source.

After a valid leader-observed RED, the proposed minimum repair is a private GET-only managed-account read fence and routing from `RuntimeCapabilities`. Reuse existing shared user queries and recheck the actual password/binding version under that lock. Preserve all password-session preconditions, namespace order, runtime/installation/binding checks, current authority and response shape. Actual capability registration/claim writers and credential minting keep their original exclusive fences. No repair is implemented or accepted here.

Full S2, the actual R20 holder, original four-case acceptance, A01–A12 and the parent ledger remain open.
