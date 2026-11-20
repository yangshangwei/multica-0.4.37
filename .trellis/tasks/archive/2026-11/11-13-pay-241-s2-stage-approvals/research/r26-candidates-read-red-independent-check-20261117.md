# R26 independent candidate-read RED boundary review

**Verdict: `ACCEPTED_RED_BOUNDARY`. Blocking RED-boundary findings: 0.**

Reviewed on 2026-11-17 by `/root/r26_red_boundary_check`, against the active PAY-241 S2 task and its sealed R25 read-coexistence plan. This accepts the genuine service-layer RED and the proposed boundary for a separate candidate-only repair. It does not accept an implementation, a GREEN run, the complete preview path, historical lock attribution, or S2 completion.

Write ownership was limited to this new Markdown/JSON pair and new private `red-independent-check` support. Product/test source, configuration, SQL, old evidence and history were read only. The checker did not execute tests, builds, SQL, API calls, process signals, Git operations or subagents.

## Evidence and source identity

- Sealed plan JSON: SHA256 `0cc6121e5e237b5352f02ce479e457352a0e782564ae01fcb3cb145545406a28`; Markdown: `212e552b0f519990de7de999370bbb31e9991da80e80bb444292f4026aa06519`.
- Implementation receipt: `bca51aea9f659d284856aeb65a3fdb00b4995a58fed843bf12ee8a7df5e1b8ef`. Its compile-only wording is retained; the actual RED is established by the subsequent raw stream and result.
- RED result: `f6ab208639390515e012d80349ed63649963c02d14a9b856914e03a0ad72aa93`; raw Go JSON: `24780be5f35dd181401534d9d3b7380ef401860d67b61836dee325dfa878a38c`.
- Exact command permit: `13be3d737b83a3934793ebf97f1dae2b70cf3ca9ca81e85af92de765609c0fa2`; actual child environment: `02d6bc6bfa34f965ff5993c61a77264c092182677b65d01edaca0327f31ab485`.

The independently rehashed source has **5,048 paths**: all **5,047 original paths/bytes/modes** match the R25 frozen source and `source-before.json`, with exactly one additional file, `server/internal/service/collaboration_completion_candidates_read_concurrency_test.go`. That file is 674 lines / 32,315 bytes / mode0644, SHA256 `086a559dcb420a41a88dcd5e499edae2f76325b49b4b0266bcf4704865a46ce9`. All **5,053 permit references** match their exact hashes. Existing fixtures, product, generated SQL and the twenty source-backed R25 boundary pairs are unchanged.

The captured command matches the permit exactly: `go test -json -count=1 -timeout=10m ./internal/service` with the sealed four-test selector. Its cwd is the frozen source's `server` directory; its environment was supplied as a complete mapping, without ambient inheritance. The URL database name, profile, allocation and actual before/after database all identify `multica_pay241_20261117_s2_m1read_red_r26`. API pool/publisher, password-mode and global test-root overrides are absent. The normal fixture pool and five-second actor read contexts remain in force; no SQL READ ONLY or substitute repeatable-read candidate transaction was introduced.

## Raw inventory and actual failure

Independent parsing of all 171 raw JSON lines found 15 unique named runs, exactly one terminal for each, and the same AST/plan/permit names: **4 top-level / 11 leaf / 15 named**, **9 PASS / 6 FAIL / 0 SKIP**. There are no parse/shape errors, pending, missing, extra or duplicate identities. The package terminal is FAIL and command exit is1. The two failing parents reflect their four failing leaves; they are not additional business cases.

| Leaf | Actual terminal | Concurrent backend pair or authority result |
| --- | --- | --- |
| `R4Coexistence/r4_first` | FAIL | R4 38898 held; candidate 38906 returned actual 55P03 / domain 409 |
| `R4Coexistence/candidates_first` | FAIL | Candidate 38908 held; R4 38915 returned actual 55P03 |
| `SiblingReads/same_creator` | FAIL | Candidate 38917 held; same-creator candidate 38924 returned actual 55P03 / domain 409 |
| `SiblingReads/different_current_managers` | FAIL | Creator 38926 held; current reviewer/manager 38933 returned actual 55P03 / domain 409 |
| `WriterFences/writer_first` | PASS | Actual default observer 38937 held; reader 38951 refused before release; fresh read recovered |
| `WriterFences/reader_first` | PASS | Candidate 38958 held; actual default observer 38972 refused at its upfront lifecycle fence |
| `CurrentAuthority/wrong_workspace_child` | PASS |404 `delivery_completion_not_found`; no partial page |
| `CurrentAuthority/outside_participant` | PASS |403 `delivery_completion_denied`; actual root participant has no source membership |
| `CurrentAuthority/disabled_account` | PASS |404 collaboration `not_found`; no partial page |
| `CurrentAuthority/expired_credential` | PASS |403 `delivery_completion_denied`; exact creator JWT context expired |
| `CurrentAuthority/revoked_artifact_manager` | PASS | Actual admin→member change; root/source read retained, private candidate set empty |

The full names have prefix `TestCollaborationCompletionCandidates`; the JSON/support receipt retains them without abbreviation.

The three candidate refusals and two writer-control refusals retain the first actual `*pgconn.PgError` before domain mapping: query `LockLifecycleIssue`, phase `row_scan`, SQLSTATE55P03, backend PID and UUID arguments. Canonical generated SQL SHA256 is `c895f24b83c5f5dcf48d463d8aa5703179d3c334d15ab42b99fe5cb2d867cf7e`, independently matched to `server/pkg/db/generated/lifecycle.sql.go`. The candidate domain result is409 `delivery_completion_conflict`. The reverse R4 leaf retains an actual unwrapped PostgreSQL55P03 error; it does not have a per-query trace record, so no exact SQL frame is claimed for that direction. These first failures do not prove that later exclusive child/principal/approval/execution acquisitions were reached.

## Methodology and preservation

The new test forwards canonical pgx SQL, arguments, rows, Scan destinations and errors. It supplies no fake rows, decisions or PgErrors. It calls the real `ListCollaborationDeliveryCandidatesInTx` with the actual borrowed transaction and actual `StageCodeEvidence` through the existing forwarding barrier. Every leaf first passed its real unheld candidate/R4 baseline; the different-manager case also passed the reviewer's own unheld candidate baseline. The baseline asserts exact current root/ordinary-child/workspace/project, numeric revisions, single selectable real artifact, canonical digests/receipt and live expiry.

The reused fixture creates real source/participant/member rows, a current code candidate and stage transition, and uses the actual packager/canonical spec. It seeds retained approval/execution rows through the established test fixture. This is a synthetic service authority fixture with finite JWT contexts, not proof of middleware authentication, the M1 approval HTTP journey, password-session/R38 acceptance or a remote repository/CI integration.

Held-before-terminal ordering is explicit in the frozen source. R4-first waits at the existing post-authority `ListCollaborationCandidateTestReportEvents` barrier, then returns and commits/rolls back the second reader before releasing/draining R4. Candidate-first and sibling-first leave their successful real transaction uncommitted until the second actual method returns. Six captured backend pairs are nonzero and distinct. Registered cleanup cancels, releases, drains and rolls back outstanding handles before the final snapshot.

Both controls use unchanged `EvaluateDeliveryEvidenceInTx`; no hand-written lock simulates a writer. Writer-first requires its actual observation to pass and hold the original fences. Reader-first proves its actual `LockLifecycleIssue` refusal before `LockArtifactUsers`; neither control passes private candidate/observation data through the refusal. Rollback/commit and fresh-read recovery then preserve the baseline.

All **22** retained per-read snapshot pairs have matching before/after digests. Each snapshot asserts complete rows and counts for **72 relations**, plus the existing stage-exit snapshot; the positive fixture includes real issue/participant/role/candidate/decision, notification/event/operation, task/runtime and artifact rows rather than only empty counters. Expected authority setup changes are made before each control's own no-write baseline. No completion authorization/terminal, evaluation, status, submission, attachment binding, queue, grant or execution side effect was observed within these assertions.

The retained database audit records zero other connections before and after, the identical853-entry ledger, and identical663 public index definitions, all valid/ready/live. It inventories211 public tables. Across the entire fixture lifetime, `collaboration_config_invalidation` grows21→43 and `collaboration_knowledge_invalidation` grows18→51. These preserved setup/cleanup histories are outside the72-relation per-read snapshot; **whole-database row equality is not claimed**. No history was removed to obtain a pass. The checker read the saved audits and did not reconnect to the database.

The fixture does not expose its archive output path. Matching canonical spec/receipt/output metadata and the existing real packager setup are not a new before/after tar-byte audit. No output path was guessed and no such audit is claimed.

## Findings (fixed)

None. This checker had no product/test/configuration write ownership. The review/support artifacts are the only additions.

## Findings (not fixed)

- **R26-C01: Real candidate reads exclude other authorized readers.** The three candidate refusals and reverse R4 refusal reproduce the read-class boundary on unchanged product code. The candidate root helper and nested child/artifact preparation retain exclusive acquisitions. Repair belongs to the implementation owner: initially limit product edits to `server/internal/service/collaboration_completion_preview.go` and `server/internal/service/delivery_completion_preview.go`, using the existing shared root/child/artifact helpers and an unexported marker created only by actual pure-read entries. Preserve every source/current-credential/role/canonical artifact/expiry/schema/policy/final-clock check and the actual unmarked writer defaults. This design-sensitive change was not made by the evidence reviewer.
- **Separate preview work remains open.** The sealed plan records additional preview/managed-submission/attachment/stage/R38 paths and the static `boundSubmission` initialization gap. This RED did not execute those branches. Give the managed preview its own real regression and bounded follow-up; do not bundle it or a broader R38/final-writer change into the candidate repair.

## Verification

- **Lint:** not executed under the read-only dispatch. The implementation receipt's gofmt-only exit0/empty output is retained; this is not a go-vet or complete lint claim.
- **TypeCheck:** not executed under the read-only dispatch. Retained compile-only `go test -c ./internal/service` exited0; its86,030,802-byte binary SHA256 is `6f57745cb014dc5fffe7ab54da08658c15bdd60fdac9da23a9052bce4177005a`. Binary bytes were independently rehashed. The compile did not execute TestMain; the subsequent RED command was `go test`, so execution of this particular compile-only binary is not claimed.
- **Tests:** original actual RED remains FAIL, as intended for the four read-coexistence leaves. The two writer and five authority leaves pass. The checker independently reconciled raw results and identities without rerunning the suite.
- **Independent evidence audit:** PASS, sealed private receipt `red-independent-check/audit-result-r01.json`, SHA256 `fd0fd0d939e2b53d003b89bbf6139418c8758a41ccf6f3913f0e7951650008b1`.

Candidate GREEN requires the unchanged15 names/test bytes on separate fresh normal/race allocations, original read/writer controls, source/command/environment/final resource checks and an independent implementation review. A complete old A/W/Q replay is not required merely to establish this candidate increment. Shared-helper callers must be checked because the two proposed files also contain pure preview entries; candidate success cannot stand in for preview acceptance. Any resulting spec clarification belongs to the main owner's later integration, preserving the distinction between shared preparation reads and the original transaction writers.

The original R24 M1 HTTP holder and R25 QA holder remain **UNPROVED**. This is independent reachable read-class evidence, with no retrospective assignment of either failure. Full preview/R38, original four scenarios, A01–A12, parent24, S3, real S4 and final S5 acceptance remain unchanged.
