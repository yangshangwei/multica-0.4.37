# PAY-241 next-input contract — independent check

**ACCEPTED_REMAINING_INPUT_CONTRACT_DOCUMENTATION_ONLY; blocking findings 0.** The bounded research, next-input contract and parent-record reconciliation are accepted. This is a documentation increment, not a new product/runtime test or acceptance promotion. The parent stays `in_progress`: **9 PASS /11 PARTIAL /4 NOT_RUN**.

## Findings fixed

- `task.json` originally paired the new “latest completed TC-06” label with a retained TC-01 single-result hash and an older S3 quality gate. Main preserved the replaced values in a second appended history record, set the ambiguous single-result hash to null, and recorded the exact retained TC-06 normal/race receipt references separately. Current state and quality gate now explicitly describe documentation-only validation. Their pending labels can become accepted after this report.
- `research/next-input-contract-20261120.md` initially described all GitHub callbacks as refresh triggers. The implementation owner narrowed that sentence to **CI-class callbacks**. The actual `check_suite`/`check_run`/`status` owner has that contract; PR callbacks also maintain mirrored PR/link state. Final contract SHA-256: `ddae00bde9867181058795411635503563129558430d06f37998e4dbc0df9408`.

No product, test, dependency, migration or general specification edit was needed. Reviewer ownership was limited to this report pair and private audit files; Main and the implementation owner made the two corrections above.

## Independent assessment of the remaining clauses

The two research scopes jointly cover exactly the fifteen non-PASS parent rows. Their recommendation is supported by the current source and accepted evidence, without asserting that the whole repository lacks useful tests or defects.

| Cases | Why a new internal test is not the next useful increment |
| --- | --- |
| TC-01 / TC-03 | Creation validates source, handler, title and request identity while business prose/project/dates remain optional. Priority enums have no PAY P1 mapping. Creation-to-other-handler `assigned`, transfer-to-valid-participants `transferred`, and newly added body mentions are separate actual notification contracts. Existing positive creation/role/history evidence cannot decide the remaining completeness/P1/triage semantics. |
| TC-04 / TC-14 | Actual machine-authority refusals and controlled timeout/Stop/recovery/output paths are already accepted. Available real-provider smoke/provenance entrypoints do not supply a PAY quality oracle, and the current session excludes real-account calls. The packaged Main test covers controlled Stop/restart/Resume, not real-model timeout or analysis quality. Its availability does not itself justify another PAY acceptance test. |
| TC-06 / TC-07 / TC-12 | TC-06's two real writer orders and TC-12's actual local synthetic A→B repair filled the former internal gaps. Their remote repository/PR/CI/payment-source limitations remain explicit. Another local mock or revision chain cannot validate those external identities. |
| TC-09 / TC-10 / TC-11 | Sandbox payment/order/fulfillment ledgers, repeated real callback delivery, actual 45-second delay and consumer-failure recovery require the selected systems and permitted injection points. Platform comment or operation idempotency cannot replace domain observations. |
| TC-17 / TC-18 | `StageExecutionSources` is a trusted construction dependency; proof bytes have no public writer. The ordinary production constructor's no-source test explicitly rejects execution. Repeating a controlled injected executor/metric source does not validate an actual publishing or rollback boundary. |
| TC-19 / TC-20 | Accepted internal final-writer gates and actor/request/run/source auditing remain valid at their source. The residual delivery/audit requirement needs actual external facts and current human decisions, including business signoff. |
| TC-24 | S5 already has implemented and checked sampling tools plus a complete retained rehearsal. Its PRD and execution plan require required S2–S4 changes to stabilize before final-candidate sampling. No newly identified accounting defect justifies another rehearsal, and current-source rehearsal would not by itself establish final TC-24 PASS. |

The smallest next external slice is **S4 E01**. I02 repository/provider/instance/allowed branch or PR/connection reference, I03 complete CI/check/run/build identities, and the E01 subset of I09 roles can unlock it without waiting for payment or deployment inputs. Complete E01 also needs allowed replay, old-head-after-new-head and test-connection-replacement stimuli. Read-only identity inspection alone remains a partial E01 observation.

Direct source inspection confirms that CI callbacks enqueue API refreshes; GitHub snapshot acquisition preserves the full paginated check set and rejects a head change during pagination. Stage consumption requires a complete nonempty exact-head snapshot, valid observation horizon and every check explicitly successful. Generic VCS currently returns `unsupported` in that strict stage reader. The contract correctly requires selecting the real provider before deciding whether adaptation is needed; it introduces no parallel workflow or speculative adapter.

## Source and record verification

- The two research manifests contain **79 reference occurrences across 74 distinct paths**. Independent rehashing matches 75 occurrences against current files and four against Main's preserved intake copies of intentionally updated parent records. All **78 intake copies** match their retained hashes. These counts differ because Main's intake also includes parent record files and the semantic Markdown report is separately bound in its manifest.
- All 24 CSV rows and every cell are byte-identical to intake HEAD `fc51c498c9270758974c5bafd0cbca6c391a9891`; SHA-256 `b7cd01931c5851f81d9caa2d1b4488ac13c03f8aa921d1e1a83cb83c6fd992eb`. No case was promoted and no historical column changed.
- Eight prior acceptance/archive objects remain structurally exact. The original 17-entry progress-history prefix remains exact; two appended entries preserve all replaced current values. Current count, next-input pointer, S2 archive flag and parent task pointer agree with the ledger. Older named `rXX` fields remain historical rather than new validation claims.
- Both context manifests retain their original entries and append the new contract, for 30 valid file references each. Work Markdown links resolve; JSON/JSONL parse; there is no added trailing whitespace; `git diff --check` exits 0. The session link to this review resolves once the report is written.
- The final TC-06/TC-12 test hashes and accepted report JSON hashes match the handoff. Both retained TC-06 mode-result JSON files match their recorded hashes. None of this re-executes or reaccepts historical behavior at the current HEAD.
- The shared branch advanced only through the two observed peer security-task commits before this seal, to `624eb71b96de1f1945d269c47378d6a31005ccd9`. Their paths are outside PAY and the relevant runtime owners. All 27 intake peer tracked-dirty hashes remain unchanged. Commit and push remain Main's responsibility; their result is not preaccepted by this report.

Nine review-time parent documents are copied under private `review/record-snapshot/` and fingerprinted in the JSON to avoid a recursive review-hash cycle. Main may subsequently replace pending review labels with accepted, add this review hash/path and the actual work-commit identity, then record exact Git outcomes. Any substantive contract/source/CSV change requires renewed review; those small finalization edits can be checked in a private follow-up receipt without changing this sealed report.

The first private audit assumed exactly one newly appended history record while Main concurrently appended the second correction. That audit reported `old_progress_history_changed`; the original receipt remains. The corrected audit compares the original 17-entry prefix and both new snapshots, which are exact. This was a reviewer-script assumption, not a product or test failure, and no input/raw evidence was rewritten to fix it.

## Findings not fixed / remaining scope

No blocking documentation, source-mapping or record-consistency finding remains. Business-semantic decisions, real models, S4 inputs and execution, final S5 and real business signatures remain open. The ordinary-comment machine/admin authority issue remains with its existing independent task. No external input or account was discovered, no task was recreated, and PAY-241 is not complete.

## Verification

- **Documentation/static records: PASS.** Actual source-path review, 79 reference-occurrence reconciliation, 78 retained copies, complete CSV/history invariants, context/link/JSON checks and scoped diff check.
- **Lint / TypeCheck: not applicable, not run.** The change is task-local Markdown/JSON/JSONL only; no Go/TypeScript/configuration implementation changed. Documentation syntax/link/data checks are reported separately rather than called a code lint pass.
- **Product tests / builds: not run.** No new product stimulus was selected; historical normal/race/UI results keep their original identities. No TestMain, DB/API/browser/native/model/provider/payment/deployment/performance or service-control operation ran during review.
- **Resources:** reviewer used awaited foreground reads/parsers and wrote only its report/private audit files. No child agents, services or background product processes were started. Main must await its own Git/agent work and record final closure before ending the overall turn.

Private review evidence: `/Volumes/artisan/.multica-acceptance/pay241-next-input-20261120-sic2jfh_/review/independent-audit-r03.json`. The adjacent JSON binds this Markdown, all inspected research references, final work artifacts and the frozen review-time records.
