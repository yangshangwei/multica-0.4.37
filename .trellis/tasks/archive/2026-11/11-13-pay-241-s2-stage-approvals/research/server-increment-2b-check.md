# S2 server increment 2b — independent implementation check

- Reviewer: `/root/s2_server_check`, 2026-11-14.
- Verdict: **changes requested** for one current-source authorization mismatch. No other concrete defect was found in this bounded review. This is not whole-S2 or A01–A12 acceptance.
- Source: 42 frozen 2b files over the previously certified 4,873-file 2a/follow-up reconstruction, producing 4,889 bound files. All overlay/base hashes and 45 implementation log/exit pairs matched. No live 2c source was copied.
- Ownership: only this report and JSON companion in the repository, plus private verification artifacts. No product, test, database, API, daemon, model or Git writes by the reviewer.

## Findings (fixed)

None in this review. The same server owner is actively implementing 2c, so the finding was routed for an owner-controlled correction and regression.

## Findings (not fixed)

### S2B-C01 — T4 and optional QA report references omit the artifact-source read gate (P1)

**Frozen locations:** `server/internal/service/collaboration_test_report_view.go:113` (`testReportReadRun`), `:163` (`testReportApplicability`), and `collaboration_stage_qa.go:15` (`stageQAReport`). Compare `collaboration_stage_review_view.go:162` and the existing M1 authority checks in `artifact_authority.go` / `delivery_evidence.go`.

R4 explicitly checks `checkStageArtifactCurrent` for an artifact-bound candidate. If the current viewer lacks the M1 source authority, it hides the complete source-evidence group, including reports. Workspace membership alone is insufficient for that check: `CheckArtifactAuthorityInTx` also requires the relevant artifact/agent manager or original initiator authority. A regular source member can therefore be denied R4 even while `SourceMember` remains true.

T4's list/detail entry gate checks only `SourceMember` and the run/issue/session association. Neither its report-log resolution nor its applicability calculation invokes the corresponding current artifact-source read check. The latter validates retained artifact identity, but not the viewer's M1 authority. As a result, that same source member can use a known report ID or list route to obtain the artifact-bound subject, cases and retained logs hidden by R4. Optional QA-return report references have the same mismatch: `stageQAReport` treats source membership as sufficient current read authority. Approval eventually runs an artifact check, but reasoned return with a supplied report reference does not.

This finding is grounded in the actual query/authority paths; the reviewer did not execute an artifact-backed runtime reproduction. The coordinator accepted the mismatch and assigned the concrete fixture/reproduction and fix to the sole server owner.

**Minimum correction:** share a current read-authority check for the report's **original frozen source**, including its exact artifact when present. Apply it before T4 projection and when optional report references are accepted. Do not substitute the latest candidate's source or make historical readability depend on the old candidate still being current/usable. Preserve source-only reports and the documented empty-list behavior. A reason-only return remains available without forcing access to private evidence.

The transaction shape matters: T4 currently uses `readCollaborationContent`, which starts `REPEATABLE READ READ ONLY`. The M1 observer uses authority row locks; R4 deliberately uses a repeatable-read transaction without the read-only restriction. Do not simply call a locking observer inside T4's current read-only transaction. Reuse the correct current-authority boundary with a compatible transaction, keeping content reads atomic and error projections content-free.

**Required regression:** an artifact-bound report with (1) authorized initiator/manager, (2) source member lacking artifact authority, and (3) authority revoked after an earlier read; exercise R4, T4 list/detail and optional QA report references. Preserve reason-only return and readable stale/failed history. The denied viewer receives no report/case/log/hidden count, and cannot use a cached report ID or cursor to regain access.

**State:** open/routed. No later fix or source proof is included in this frozen review.

## Verified implementation paths

### Input, claim and immutable reporting

- C1 parses a closed create-only reference, requires the actual typed target/version context and current human/source/stage authority before attaching content, and binds only successfully appended command IDs. The inherited content transaction fences the member row. Its post-append validation checks the original approval again after work that can wait.
- First claim derives inputs from the actual in-flight commands, checks each bound author/participant/account/lifecycle against those inputs, refuses conflicting bindings, and requires the exact existing Git preparation commit. It freezes one immutable subject event; an upload cannot choose the latest candidate or attach a new subject to a legacy run.
- T2 resolves the issued business capability and immutable attempt/nonce/identity. Original receipt lookup precedes current admission, and the original-key check repeats after acquiring runtime locks. A changed body/key cannot replace the sole report for that attempt. The write appends the report and content-free execution hint in one transaction without changing business outcome, comments, children, QA decisions or stage.
- Redaction precedes retained report hashing and another full protocol validation. Case-ID collisions or size violations introduced by normalization are therefore rejected. Referenced logs require the exact retained event, same run/issue/attempt/nonce, valid message index and redacted-body hash; absent or cross-attempt logs refuse.

### Attempts, QA and expiry

- `testReportApplicability` checks the report's attempt against `run.authorized_attempt_seq` before returning usable. An older report cannot borrow a later attempt's successful outcome/proof.
- The converse shortcut is also closed: the existing host permit path rejects a new attempt after `BusinessOutcome` has been recorded. `collaborationRunProvenEnded` additionally requires settled state and the appropriate final sealed-attempt proof. A latest report cannot legitimately combine with an earlier recorded business outcome through a newly authorized attempt.
- QA approval requires the exact report/candidate, passing cases, current successful settled execution and a current code-review approval. The original signer chain is carried into the final authority check after evidence observers. QA→release re-locks and checks the chosen report/run and retains the transitive code-review signer dependency.
- Failed/unsettled reports remain valid evidence to return when the viewer can read them; they do not become passing reports. Superseded candidates preserve their original reports and failure history. The artifact read-authority exception is the finding above.

### Salt, cursors and other readers

- The random salt is stored only inside the private `test_subject` run event. T1 exposes the typed subject, T3 the minimal ACK, and T4 dedicated typed projections; none exposes the salt or manifest digest.
- Legacy run-record/preparation/recording readers select closed event kinds and project explicit fields. Handoff and knowledge source readers likewise constrain event kinds/producer keys; the runtime recovery paths check the expected kind before decoding. No inspected existing read path returns the raw `test_subject` payload.
- T4 cursor MACs use a separate domain and bind issue/run/viewer plus the original sequence watermark. Existing authority checks still run before cursor consumption. The retained host test covers different-viewer rejection, a tampered watermark, later reports excluded from an older page sequence, and original-attempt receipt recovery.
- This is a code-path and retained-test assessment, not a claim that a full browser revocation campaign or every future reader has been tested.

### CLI and compatibility

- Root command registration and run-local allowlists include only the subject/submit/receipt shapes. Bounded stdin, strict protocol decoding, issue/run/request response matching, original-key uncertainty and no ambient human/daemon credential fallback are preserved.
- Builtin/source-map documentation and the host brief identify reports as evidence, not QA approval. The optional test-harness Git input is isolated to the authorized harness; its five-minute poller and existing execution/receipt defaults remain unchanged.
- Old run-record item variants remain closed. Policy activation is still unavailable, and no 2c execution fact or 2d final-writer behavior is inferred.

## Verification

| Check | Result |
| --- | --- |
| Source/log reconstruction | PASS: 4,873 certified base files, 42 overlay files, 4,889 final bound files and 45 raw logs/direct exit sidecars. No missing prerequisite surfaced. |
| Independent Go vet/type analysis | PASS on frozen service, handler, internal CLI, root CLI and protocol packages; empty output. |
| Independent formatting | PASS on the frozen overlay's Go files; empty output. |
| Independent protocol report tests | 28 named PASS, zero FAIL/SKIP. |
| Independent internal CLI uncertainty test | 1 named PASS, zero FAIL/SKIP. |
| Independent root CLI report tests | 3 named PASS, zero FAIL/SKIP. |
| Implementation DB-backed evidence | Retained logs verified, including final Stage108, report/QA/member/expiry race22 and HTTP19. The broader300 run and later six-test QA hint check remain separately identified by the implementation report. Not rerun against a shared database by this reviewer. |
| Original failures | All 45 implementation logs/exits remain unchanged, including earlier red/setup/compilation failures. |

Independent commands, inventories and hashes are under `/Users/artisan/.multica/acceptance/pay241-20261114-resume/s2-server-inc2b-check/` and in the JSON companion. CLI/protocol tests use private Git-external HOME/temp state and no database/account environment. Their local HTTP fakes are unit transports, not a launched product API.

The 2b report's complete wire hash is historical: the central working wire changed while the sole owner began 2c. This review uses the supplied test-report contract and the unchanged 2b §§8.7–8.9 obligations; the JSON records the historical and currently observed wire hashes separately. No live 2c product bytes enter the reconstruction.

## Remaining acceptance limits

- Actual owned Git materialization and the controlled CLI→handler→service producer journey have not run in this increment's acceptance environment. The service fixtures use synthetic source/CI facts.
- Production Web, source-matched Desktop, the complete A01–A12 matrix, all 2d writers and policy activation still require their assigned verification.
- The eight unrelated F2 migration-registration omissions and previously recorded unrelated failures are neither fixed nor relabeled here.
- Correct S2B-C01, retain its first reproduction and verify the affected source/return/read paths before declaring this server increment independently accepted. Real provider truth, payment and deployment remain separate S4 evidence.
