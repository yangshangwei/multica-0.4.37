# R21 immutable workflow receipt/read concurrency

Status: corrected original-code RED proved the read/read gap; private GREEN passed targeted normal/race and package vet. Independent acceptance, guarded ROOT integration and actual four-scenario acceptance remain pending.

## Actual verification and ROOT integration handoff

The coordinator executed the frozen GREEN commands. This implementer rehashed the result, raw stdout/stderr and command bindings, reparsed every Go JSON line and confirmed the exact named terminal sets. Source snapshots showed no drift; both test databases had zero other connections and empty fixture tables afterward.

| Check | Actual result | Result SHA256 |
| --- | --- | --- |
| New regression normal | 4 top-level / 6 named / 5 leaf PASS; zero FAIL/SKIP/parse errors | `38e25a8719e2b3058776fde863fa561e38a85ff7f6eba02e8d2f0cef4194eddb` |
| Combined race with three existing receipt controls | 7 top-level / 9 named / 8 leaf PASS; zero FAIL/SKIP/parse errors | `4fccfdb967a827be7d31c58113bb39c4f644779510f68cad4e2e6a6ca9db1ecc` |
| `go vet ./internal/service` | exit 0, empty stdout/stderr; bound runtime build source before/after unchanged | `5759e0ecbca912cc5583dfb3db561abb03c2b7c6eb366565fc939b993688ecad` |

Normal and race used `implement/source-green-r01/server`, the exact selectors in the retained plan, `-json -count=1 -timeout=5m`, and separate fresh `receiptread_green_r21r02` / `receiptread_race_r21r02` databases. Race additionally used `-race`. Vet used `runtime/build-source-r01/server`, whose 5,046-file manifest is `43286f23…`; both owned workflow/test leaves match the canonical GREEN exactly. These are two observations with overlapping named tests, not an increased business-case denominator.

ROOT's exact `GetWorkflowOperation` function was read-only compared with the corrected RED and matches byte-for-byte. ROOT's new regression file is absent. The frozen integration patch changes only the single receipt-lock line and adds the accepted `0dfec7c3…` test, preserving every other ROOT hunk. Patch: private `evidence/root-integration-patch-r01.patch`, SHA256 `ba16cb5dc031a0f7b7ad50bad042e71a7003227ca4f3eef70227163208d18d94`; before-byte/mode preflight: `root-integration-preflight-r01.json`, SHA256 `e7f490e183b6014860dbe026820a61fa4527f09e2add06bd06079fcb18bb196c`.

The structured report contains actual argv/cwd, source hashes, complete named sets and raw references. No source changed or test/build/DB/API/Git command was run by this implementer in this handoff. Main owns independent acceptance and must recheck ROOT preflight bytes immediately before integration. This proves the targeted private receipt/read repair; the original R20 runtime holder remains unproved and full S2 remains open.

## Current RED/GREEN checkpoint

The first RED result `62039eaa…` preserved both expected read/read failures plus an invalid writer control. Its barrier named `GetCollaborationStageCodeCandidate`, while the real decision writer calls `GetCollaborationStageCodeHead`. A new `source-control-r02` changed only that needle and the unexpected-receipt diagnostic; assertions and deadlines stayed fixed.

The coordinator's second full RED, `database/profiles-r02/red-r02/result.json` SHA256 `7b2eb8e9927e909ef358279c1d5a04b83f30687b68eb78bcf1e2d9776c06769f`, executed all **4 top-level / 6 named / 5 leaf records** without skips or parsing/source/database drift. Receipt-first R4 failed at the real user shared fence with `55P03`; the reverse receipt read exceeded its five-second deadline. Real writer exclusion and both current-authority controls passed.

A new `source-green-r01` preserves that corrected RED and changes only `GetWorkflowOperation` from `q.LockCollaborationUsers` to existing `q.LockArtifactUsers`. The actor-row count/equality, current-account authorization, scope/request lookup, allowed operation kinds, decoding and commit remain byte-identical. Every other source file/mode and corrected test SHA `0dfec7c3…` is unchanged.

GREEN manifest SHA256 is `804a8cee566af25b23c4cbf9532643255babc85e4ac9e850fb9bf8ba7a199863`; workflow SHA256 is `51e7eb2b52462fa5e771014b2000173752f30eaa07997f5c66d170054d5f5d09`. The one-line diff is `evidence/green-one-line-diff-r01.patch`, SHA256 `2f741326ddc6ed675f467f78ed5dff0d57fd79c994567e911868c756454e111a`.

The frozen `evidence/green-test-plan-r01.json` SHA256 `d9ea880d486e645808445ef032de43f150c089ba151db578768bb21785a7a312` plans normal execution of the new **4/6/5** inventory, then race **7/9/8** including three existing controls: voluntary leave/minimal recovery, lost leave reply without object access, and stage review/original-key recovery. Package vet follows. These commands have not been run by this implementer; the coordinator owns exact permits and fresh profiles.

The confirmed service read/read gap remains distinct from the actual R20 runtime failure, whose holder is still unproved. No ROOT integration, UI unlock, retry, isolation change, other GET repair or SQL/writer change is claimed.

## Original test-writing checkpoint (retained)

The sections below preserve the initial test-writing state and its original hashes. The original first RED is retained despite the subsequently corrected writer barrier.

## Reachable contract under test

`GetWorkflowOperation` currently locks its one real actor through `LockCollaborationUsers` (`FOR UPDATE`), authorizes the account, then reads the actor-scoped immutable operation. Artifact-bound R4 evidence reads use shared user fences. A receipt GET therefore has a reachable read/read exclusion gap.

This regression targets that contract directly. It does not attribute the latest R20 runtime `LockArtifactUsers` failure to a receipt reader; that runtime's actual lock holder remains unproved.

Only a new private `server/internal/service/collaboration_workflow_read_concurrency_test.go` was added. All 5,044 pre-existing source files and modes match `evidence/source-before-r01.json`; the frozen RED has 5,045 files. Private `collaboration_workflow.go` and ROOT product files remain unchanged.

## Test inventory and synchronization

| Top-level test | Required behavior |
| --- | --- |
| `TestCollaborationWorkflowReceiptReadThenStageEvidence` | Actual same-reviewer receipt GET holds after account authorization; actual R4 on another backend completes with the exact artifact-bound candidate before release. Original minimal receipt remains identical. |
| `TestCollaborationWorkflowStageEvidenceThenReceiptRead` | Existing actual R4 barrier holds shared artifact/source fences; the same reviewer's original receipt GET completes before release. Both original receipt and source baseline are retained. |
| `TestCollaborationWorkflowReceiptReadOriginalWriterFence` | Real `DecideStage(return)` holds its original user locks before candidate lookup; R4 must refuse with `55P03` and zero projection. Releasing the writer allows its exact return to commit. |
| `TestCollaborationWorkflowReceiptReadCurrentAuthority` | `wrong_actor`: 404 `not_found`, empty receipt, original actor recovery unchanged. `disabled_account`: actual user disable yields 403 `forbidden` and an empty receipt. |

The first three tests use `newStageReviewFixture`, the existing real artifact fixture and `f.approved` to create the exact candidate, human decision and original operation key. Artifact binding is necessary to reach the shared user fence; plain PR-only evidence is insufficient for the concurrency claim.

The receipt barrier uses the existing passthrough `stageProjectionBarrierStarter` at `GetCollaborationOperation`, after real user locking and `authorizeWorkflowAccount`. The reverse order reuses `holdStageArtifactR4`. The writer control calls real `DecideStage`, holding at `GetCollaborationStageCodeCandidate` after original workflow user locks. No SQL result, permission or transaction return is substituted.

All requests keep a five-second budget. The second-request context starts before the held reader's context, so the old receipt lock cannot succeed merely when the held reader expires. Backend PID comparisons require distinct connections. `sync.Once` release functions, buffered result channels and `t.Cleanup` release/cancel/drain cover failures; existing R4 cleanup is reused unchanged.

Static expected inventory is **4 top-level / 6 named / 5 leaf records**. Expected original RED is two failing coexistence leaves and three passing control leaves; this is an expectation, not an observed test result.

```text
^(TestCollaborationWorkflowReceiptReadThenStageEvidence|TestCollaborationWorkflowStageEvidenceThenReceiptRead|TestCollaborationWorkflowReceiptReadOriginalWriterFence|TestCollaborationWorkflowReceiptReadCurrentAuthority)$
```

## Frozen evidence and next gate

- Test SHA256: `9048a66130440ac64eee2bf451a1308b328fbf744b7bf14569e5f4d8dad5f20e`.
- RED manifest: private `evidence/red-source-manifest-r01.json`, SHA256 `23d2a121e5e25504a79cfb088498eb0231a0ea1ce8abc703bc66b3f7524879e5`.
- Static inventory: private `evidence/red-test-inventory-r01.json`, SHA256 `4edc7fa17f1ba7c00d7051c2af9f3ee25bc1bc1b92920933fe413af642f59b00`.

Only `gofmt -w` executed, without compilation or tests. The structured companion records complete paths and reference hashes.

Next: independent static acceptance, exact coordinator permit for the original RED command, then inspect the real failure. Any later private repair is limited to reusing existing `LockArtifactUsers` in `GetWorkflowOperation`, with unchanged actor-row equality, account checks, actor/scope/request identity, decoding and transaction. Original writer SQL, other GET readers, source privacy, retries and deadlines stay outside that repair.
