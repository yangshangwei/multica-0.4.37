# S2 increment 2b — independent artifact-read ACL recheck

- Reviewer: `/root/s2_check_resume`, 2026-11-14.
- Verdict: **PASS for this correction; S2B-C01/P1 closed.** No open blocking finding in the reviewed ACL increment. This does not accept all of S2, enable policy activation, or certify unfinished2c/2d behavior.
- Source: the independently checked4,889-file2b baseline plus the explicit14-file correction/supplement overlay, reconstructed as4,894 files. Initial and final hashes match. No undeclared live source was copied.
- Review ownership: only this report, its JSON companion and private check artifacts. The implementation owner made the product/test corrections; this reviewer made no product, test, migration, configuration or Git changes.

## Findings (fixed)

### S2B-C01 — original artifact authority now protects T4 and optional QA references

The shared reader in `server/internal/service/collaboration_stage_evidence_read.go:15` keeps R4 and T4 on a repeatable-read transaction that allows the existing M1 permission locks. It does not introduce an alternative authority system. `testReportFrozenSourceRead` at line52 resolves the original frozen child, approval and artifact execution, calls `lockDeliveryArtifactInTx`, and verifies retained execution/spec/receipt/artifact identity. It does not require the historical candidate or PR head to remain current.

`collaboration_test_report_view.go:114` checks the run's original subject before sequence lookup, cursor validation and page filtering. This also closes the empty-page case raised during this review: an unreadable artifact-bound run cannot reveal whether it has reports through an empty result or cursor error. Authorized legacy runs with no frozen subject retain their empty-list behavior. Each returned report must also agree with the original frozen subject. T4 discards its entire result on an error.

`collaboration_stage_qa.go:15` applies the same current authority before accepting a report reference, including reasoned return. The actual borrowed transaction is forwarded through standing/final QA consumers. An authorized manager can cite failed evidence, while an ordinary source member or demoted manager receives `stage_evidence_unavailable`. A reason-only return remains available.

Independent service tests cover authorized initiator/manager reads, R4/T4 denial, positive and denied QA citations, readable history after candidate supersession and PR-head change, real two-attempt cursor recovery, artifact-bound empty pages and the unbound-run control. The concurrency tests use a real second connection and canonical `UpdateMemberRole`: an already held permission fence produces55P03; the demotion succeeds after the read/QA transaction finishes. When demotion commits after the RR access snapshot but before the authority lock, T4 produces40001 and no page/detail, followed by404 on a fresh read. The corrected QA seam must be hit exactly once.

The external `service_test` regression invokes the actual T4 handler adapters with fixture authentication context. It verifies allowed `no-store` responses and exact content-free `{code,error}`404 bodies for list/detail, including a demoted manager. This is not an actual router/auth/browser launch.

## Findings (not fixed)

No remaining ACL product finding was identified. Two boundaries are recorded without changing the frozen implementation report or widening this correction:

- **Evidence timezone erratum:** the implementation Markdown calls the inherited handler fixture's timestamp `14:38Z`. The actual filesystem mtime was `2026-11-14T06:38:23.783938Z`, which is14:38 in Asia/Shanghai. The fixture source hash is correct; this wording error does not change its provenance or the test outcome. The coordinator requested preserving the original report and recording the correction here.
- **Unfinished2c DTOs are not certified:** their explicit inclusion makes the current source reconstructable and compilable, and the legacy code-review/QA serialization test passes. The current draft R4 constructor leaves its new three arrays nil;2c must provide the frozen array/unknown/source-denied contract before its own acceptance. No execution ledger, target resolution, release/rollback consumption or public2c readiness is inferred from this ACL pass.

## Verification

All commands ran on the independent reconstructed source. The database launcher verified the exact coordinator-allocated package database before execution; suites ran serially. Named subtests overlap and must not be added as business acceptance cases.

| Independent check | Result |
| --- | --- |
| Historical baseline and evidence audit |4,889 source files and45 historical log/exit pairs matched |
| Correction reconstruction |14 declared overlay files;4,894 final files; zero hash differences before/after checks |
| Correction evidence audit |14 raw logs,14 direct exit files and referenced capture metadata matched; parsed named inventories match |
| Service ACL + report/QA/legacy request regressions |23 named PASS,0 FAIL,0 SKIP |
| Service artifact ACL `-race` |13 named PASS,0 FAIL,0 SKIP; includes real DB ordering assertions and actual handler adapters |
| Handler stage regressions |44 named PASS,0 FAIL,0 SKIP |
| `go vet ./internal/service ./internal/handler` |PASS, empty output |
| `go build ./...` |PASS, empty output |
| `gofmt -l` on the14-file overlay |PASS, empty output |

The implementation owner's broader final Stage125, handler44 and race25/44 evidence was hash-checked, not relabeled as additional independent cases. Original red-2 retains the five actual ACL failures. Red-1 and extended-1/2/3 retain their separate fixture/compilation/unhit-seam failures; the mistyped launcher attempt remains an infrastructure record with its uncaptured exit explicitly disclosed. No failed evidence was overwritten or changed into a pass.

Private evidence and exact command/source/log bindings are under:

`/Users/artisan/.multica/acceptance/pay241-20261114-resume/s2-server-inc2b-acl-check/`

The independent processes have ended and all granted package profiles are released. No standalone API, browser, Desktop, remote provider or model process was started by this reviewer. No migration or gate change was performed.

## Next dependency and remaining acceptance

The sole server implementation owner may continue the frozen2c contract and then2d. Preserve these ACL checks as shared report consumers evolve. The complete A01–A12 matrix, all completion/project-exit writers, legitimate policy activation, owned-Git/CLI journey, production Web and matching Desktop remain separate required acceptance. The eight existing unrelated F2 migration-registration gaps, real external integration NOT_RUN cases and unchanged parent matrix remain open.
