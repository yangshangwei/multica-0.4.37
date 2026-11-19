# S2 server increment 2b results

Status: implemented; focused normal/race/static verification passed. Independent review and the production Web/Desktop/controlled-provider journey remain pending. Policy activation stays disabled.

## Implemented behavior

- The human comment endpoint accepts the frozen create-only `test_subject` reference. It validates current source/role/scope/code-review authority before attachment or comment writes, binds only actual successfully appended commands, and checks the original approval again after target append work. Blocked targets acquire no binding.
- First claim freezes the original command subjects and the exact existing Git preparation. Missing Git, a mismatched commit, distinct candidates and legacy unbound runs refuse typed reporting. A private random salt keeps the public binding digest opaque; no manifest digest or salt is exposed.
- T1/T2/T3 use only the issued `mcb_` run capability. Actual attempt, nonce, Agent and immutable identity come from persisted authority. Each attempt gets one immutable `test_report` event. Exact original ACK lookup precedes current admission, including after Stop, a later attempt or settlement. Changed payloads and replacement keys cannot overwrite failure history.
- Case outcomes derive the reported result. Text is redacted before retention; record logs resolve exact retained checkpoint/outcome events from the same attempt. Missing, wrong-hash and cross-attempt references refuse. Report submission does not change business outcome, write a comment/child, sign QA or advance the stage.
- T4 and R4 project current human/source-authorized report evidence. Original failed cases/logs remain associated with their candidate. Passing cases from failed or unsettled execution cannot pass QA. T4 cursors authenticate issue/run/viewer and the original sequence watermark using a separate domain of the private run salt. Old run-record item variants are unchanged.
- QA approve/return and QA→release/rework reuse the existing workflow, decision and original-operation ledger. Approval requires the exact passing report plus current code-review approval and transitive original signer checks at the final clock. Return can cite failed/unsettled evidence and historical CI belonging to that candidate. Agent reports cannot replace the human decision.
- The existing CLI has run-local `collaboration test-report subject/submit/receipt` with bounded stdin, strict identity/response checks, original request handling and closed command/flag allowlists. Builtin instructions and host brief document one report per attempt and distinguish evidence from human approval.
- The root-authorized test harness now accepts the existing optional `HostGitConfig` before capabilities registration and emits only its safe capability Git summary. Existing five-minute poller, three-minute execution default, timeout override and receipt fault behavior are preserved. E2E owns actual local Git bytes and provider invocation.

## Files and evidence

The adjacent JSON binds **42 current server files** and **45 raw logs plus direct exit sidecars**. Exact copies are at:

`/Users/artisan/.multica/acceptance/pay241-20261114-resume/s2-server-inc2b-implementation-source`

This is an incremental overlay. Reconstruct it with the preceding 1b/2a and separately reviewed follow-up manifests named in the JSON; do not silently substitute live files for those prerequisites. The earlier nullable-digest fixture manifest omission and the independent first reconstruction failure remain recorded. The 2a source-member race fix has its own report and source freeze.

Changed groups: migrations 826–828 and cleanup/deletion registration; test-subject/report sqlc query/model output; protocol; comment/claim/runtime/report/QA services; HTTP/router; CLI; builtin documentation; host brief and the explicitly authorized test-only harness. No project dependency was added and this lane performed no git write.

## Verification

Counts include named subtests, overlap between selections, and must not be summed as acceptance cases.

| Check | Named pass / fail / skip | Exit |
|---|---:|---:|
| Final service `^TestStage` | 108 / 0 / 0 | 0 |
| Earlier broader stage + managed/preparation/host/checkpoint/recording/comment compatibility selection | 300 / 0 / 0 | 0 |
| Final key report/QA/member/expiry race selection | 22 / 0 / 0 | 0 |
| Final handler stage/comment selection | 19 / 0 / 0 | 0 |
| Root CLI collaboration selection | 95 / 0 / 0 | 0 |
| Internal CLI collaboration selection, including original-key uncertainty | 76 / 0 / 0 | 0 |
| Protocol collaboration selection | 336 / 0 / 0 | 0 |
| Host harness Git/timeout/receipt boundaries | 21 / 0 / 0 | 0 |
| Host brief tests | 2 / 0 / 0 | 0 |
| Concurrent-index cleanup inventory including 827 | 1 / 0 / 0 | 0 |
| Final QA/R4 blocker vocabulary recheck | 6 / 0 / 0 | 0 |
| `go vet` affected packages; `go build ./...` | static checks | 0 |

The broader 300-pass run preceded the final QA optional-evidence helper extraction and lost-commit regression. Final Stage108 and QA-hint6 cover those later changes; runtime/comment paths did not subsequently change. Host-brief tests execute explicit test-owned `os.Executable` provider fixtures; they are not merely static tests.

DB tests used the original exact-package `run.mjs` profiles. Later CLI/protocol/host checks used the coordinator's private per-package no-DB launchers. Root applied and verified migrations through828 on all four assigned databases; this lane did not invoke migration CLI. Migration records are linked in the JSON.

All initial failures remain: absent protocol/runtime/CLI/harness surfaces, fixture password/finite-JWT setup, unimplemented QA return, duplicate-subject HTTP error classification, the sqlc placeholder error, and separately reported 2a review regressions. Passing reruns do not replace those records.

## Remaining boundaries

The actual CLI→handler→service journey with an owned Git bundle and production Web/Desktop is assigned to the E2E lane and remains unexecuted here. Service fixtures prove server claim/permit/ledger behavior using synthetic source/CI facts, not host Git materialization, provider truthfulness, remote CI, payment or deployment. No standalone API/daemon or commercial model/account was launched by this lane.

Release/rollback decisions and controlled execution facts remain increment2c. Submission/completion/project-exit guards, all-writer verification and policy enabling remain2d. A01–A12 and the parent acceptance matrix remain open. The unrelated eight F2 index-cleanup gaps and previously identified Autopilot/report_snapshot failures were left intact.

The next step is coordinator review of this frozen increment, then the same server owner starts2c with the required wire/DTO amendment and original-request serialization regression.
