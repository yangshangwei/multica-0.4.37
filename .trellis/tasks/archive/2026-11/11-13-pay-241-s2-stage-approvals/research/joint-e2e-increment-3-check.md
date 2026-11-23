# S2 joint E2E authoring increment 3 — independent check

Reviewer: `/root/s2_client_check_resume`, 2026-11-14.

**APPROVE the bounded authoring increment after one fixture correction.** No open findings remain within this scope. Actual scenario bodies executed: **0**. This is not A01–A12, production Web/Desktop, real deployment, payment or whole-S2 acceptance. The explicitly deferred client3d M1 shared-form integration remains required.

## Findings (fixed)

### E3-C01 / P2 — USTAR numeric fields accepted a valid octal prefix with invalid trailing bytes

Files: `e2e/fixtures/collaboration-stage-execution.ts` and `e2e/fixtures/collaboration-stage-artifact.node-test.mjs`.

`verifyStageArtifactTar` decoded numeric fields with `parseInt` after truncating at NUL. An invalid mode field ending in `x` still produced the expected numeric value. Recomputing the header checksum made the malformed header pass the remaining checks. The original mutation tests changed bytes without restoring the checksum, so checksum rejection did not prove numeric syntax validation.

The independent pure-artifact counterexample failed with the submitted verifier: one test failed and the allocation test passed. Its source and raw log are retained under `octal-red-source/` and `octal-red-1.log`.

The corrected verifier checks every numeric field's full bytes using Latin-1, requires octal digits followed by NUL/space termination and padding, then decodes the validated number. Latin-1 preserves invalid high bytes rather than masking them. The regression changes mode, UID, GID, size, mtime and checksum fields while retaining a correct checksum where applicable; it also checks missing numeric termination. The independently generated valid USTAR archive still passes.

The parent authorized this local correction. No production packager, source file bytes, execution timeout, pool setting or actual build behavior changed. Only the two files above differ from the author's five-file `source-v2` freeze; that original freeze remains unchanged.

## Findings (not fixed)

None within this authoring review. The final M1 success currently uses authorization/lifecycle APIs after a Web submission, followed by Web/Desktop reads. It does not test the shared completion-authorization form. This dependency was explicitly declared before review and remains assigned to the E2E owner after client3d freezes.

## Review evidence

- **Frozen graph:** verified the author's 2,543-file checked3c/E2E reconstruction and all 20 raw log/exit pairs. The independent copy adds the preserved original scenario and two preserved Go provider source files, producing 2,546 bound files. No live3d or parallel knowledge-automation source was copied. The final TypeScript dependency list contains633 files and no original-workspace product-source escape; workspace links resolve into the independent copy.
- **Physical build authoring:** the fixture reads the exact Git object, writes its canonical staging manifest and input digest into a preallocated generation, creates and human-approves the existing artifact request, invokes the actual packager route, and compares physical tar bytes/digest against both its receipt and persisted execution. It seeds only ordinary producer/configuration prerequisites. No approved artifact, report, stage approval, execution fact or metric row is inserted by the new fixture.
- **Identity and authority:** R/Q/L obtain artifact-reader authority through actual member PATCHes. A regular source member remains the R1-readable/R4-T4-denied control. The QA report is produced through the prepared base Host and run-local CLI, then checked against the exact candidate, source revision and actual artifact. Public human approvals remain separate from private integration observations.
- **Private transport:** the flat closed action union matches the frozen transport. UUIDs, versions, UTF-8 native IDs, fields, duplicate reply keys, correlated request IDs and bounded frames are validated. Receipt lookup can reuse a completed original request ID, while concurrent duplicate sends and unconfirmed sends never replay prepare or invent a new key. E2E cannot send shutdown or start the harness. The socket is canonical, privately owned and bounded to103 path bytes.
- **Execution scenario:** the request identifies the selected approval/candidate/target, callback counts are compared to the configured baseline, and the callback's last intent/build/target is checked. Unknown intent and historical lookup cannot create a fact, advance or revoke consumed approval. Authenticated success is followed by an explicit A9. Equal-version contradictory metrics block the signed rollback with no extra call; a newer observation and new human approval bind the exact predecessor for one rollback.
- **Late-result scenario:** an actual short approval horizon is supplied from server time; the authored poll waits for its expired state before recording the late fact. The fact remains tied to the original approval. Both expired-approval advancement and a fresh approval attempting to use the old fact are explicitly refused; no second prepare is issued.
- **Completion/exit scenario:** submit/self-complete assertions cover M1 off/observe/enforce, including an issued exact M1 grant whose final completion is stage-blocked and remains unconsumed. Project exits use M1 off; an unmanaged project is the deletion control. The successful path binds the M1 operation ID to the lifecycle request key. R38 targets a separate unfinished root and occurs after the main root consumes its approvals, avoiding account-generation invalidation of that positive path.
- **Ownership and evidence:** eight generation directories are preallocated before the staging root becomes0500; the fixture changes only selected generations, not the pinned root. Case slots0–6 are distinct. Cleanup closes scenario resources and restores original gate booleans with monotonic epoch; the parent owns bridge/harness shutdown. Structured failure evidence avoids copying native paths. Original failed runs and complete case discovery remain retained.

The socket client retains its existing **20,000ms** response timeout. The server's30-second callback-entry horizon and the coordinator bridge's separate bounds are different mechanisms. All three scenario timeouts remain480,000ms. No deadline was increased.

## Verification

| Check | Result |
| --- | --- |
| Original five E2E2 files | Unchanged, all hashes matched before/after review |
| Submitted five files and 20 author log/exit pairs | All matched |
| Independent original pure fixture baseline | 11 passed, 0 failed/skipped |
| New malformed-octal counterexample | 1 failed / 1 passed; retained |
| Final strengthened pure fixture suite | 11 passed, 0 failed/skipped; additional assertions are not counted as new scenarios |
| Final isolated TypeScript | Passed against declared checked3c/E2E graph |
| Final scoped ESLint | Five files reported, 0 errors, 0 warnings |
| Playwright discovery only | Four tests in two files; zero bodies executed |
| Final source/dependency/link audit | Passed; only two declared fixture files changed |

Pure tests execute an independent Python USTAR writer and fake private Unix-socket fixtures. They do not run the product packager/API, actual Git fixture, deployment controller, daemon, browser, model or database. Discovery used a private config with only the two selected specs, one worker and zero retries, and no global setup or web server.

Exact commands, source/log hashes and final five-file overlay are recorded in the JSON companion and `/Users/artisan/.multica/acceptance/pay241-20261114-resume/s2-e2e3-check/checked-source/`. The original `source-v2`, initial source reconstruction and every author failure remain unchanged.

## Required follow-up

The same E2E owner should integrate the frozen client3d shared M1 form and obtain another bounded check. The coordinator must then bind the final full API/CLI/Git/base-Host/Web/Desktop source and configuration, execute all four authored scenarios plus the remaining A01–A12 evidence plan, retain failed runs and resource restoration evidence, and obtain full independent phase review. The parent matrix and real S4 NOT_RUN items remain open.
