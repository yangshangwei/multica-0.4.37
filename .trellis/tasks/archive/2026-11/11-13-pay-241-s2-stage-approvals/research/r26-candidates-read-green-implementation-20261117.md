# R26 candidate-read GREEN implementation

The authorized candidate-only repair is complete in the private GREEN source. Main's exact normal run passed15 named results/0FAIL/0SKIP. This implementation agent ran formatting, compile-only, vet and static source checks, and executed no database tests. Main race state at report creation: `PASS`, exact permit SHA256 `2548f3a2b8faacd258215fefaf806e5eb50c63073dc61afd3021459e182dd377`. Its recorded result SHA256 is `57e05bb390a6b662f5e8af687d1838b057e463a5be69a7db03e823917e784043`; 15PASS/0FAIL/0SKIP.

The prior original-product RED is retained:9PASS/6FAIL/0SKIP across15 named records, with four genuine read-coexistence failures and two corresponding parent failures. Both writer and all five authority leaves passed. Independent RED-boundary review is `ACCEPTED_RED_BOUNDARY` with zero blockers; historical R24/R25 holders remain unproved.

## Changed source

Private source: `/Volumes/artisan/.multica-acceptance/pay241-r24-continuation-20261117-32uud2uo/r26-m1-candidates-read-r01/source-green-r01`. Only these two original files changed:

| File | GREEN SHA256 | Bytes/mode |
| --- | --- | --- |
| `server/internal/service/collaboration_completion_preview.go` | `07637fee75dc894a50dd32eaa2beee3254784f015f0096fdd8cd3b44410fd0fe` | 14141 /0644 |
| `server/internal/service/delivery_completion_preview.go` | `2f9103c1bc3e4018cd6ec351b040085b7d76593faca25427e8a8774918353a75` | 16586 /0644 |

The full5048-entry manifest has SHA256 `11ad7f493d3d33ef411903b9d062855b8112c797aaef78491e902e35051d5a8d` and is saved at `/Volumes/artisan/.multica-acceptance/pay241-r24-continuation-20261117-32uud2uo/r26-m1-candidates-read-r01/evidence/green-implementation-source-r01.json`. All5046 other entries remain equal to Main's before manifest, including the unchanged674-line candidate regression SHA256 `086a559dcb420a41a88dcd5e499edae2f76325b49b4b0266bcf4704865a46ce9`. Original RED and ROOT product bytes/modes were preserved. No source path was added/removed and no mode changed.

## Five selected read dispatches

| Pure read preparation | Original acquisition | Existing shared acquisition |
| --- | --- | --- |
| Root issue | `LockLifecycleIssue` | `LockDeliveryEvidenceIssueRead` with the same ID/workspace |
| Root collaboration row | `LockIssueCollaboration` | `TryLockCollaborationStageRootShared` with the same root ID |
| Source child | `LockLifecycleIssue` | `LockDeliveryEvidenceIssueRead` with the same ID/workspace |
| Repeated ordinary-child preparation | `LockLifecycleIssue` | `LockDeliveryEvidenceIssueRead` with the same ID/workspace |
| Candidate artifact projection | `lockDeliveryArtifactInTx` | `readDeliveryArtifactInTx` |

The last dispatch selects the existing shared principal-child, approval and execution queries, covering the remaining three exclusive acquisitions. Root and child issue reads remain `FOR SHARE NOWAIT`; the collaboration helper locks the same row with `FOR SHARE NOWAIT`. Its returned lifecycle scalar is not used as a substitute access result: the original `ResolveIssueAccess`/administrator recheck still follows the actual root fence. Source workspace, parent/project/ordinary-child identity and all original current authority/credential checks remain unchanged.

Candidate enumeration reaches the existing explicit unexported artifact-read entry, whose mode is constant true. It does not traverse a marker-dependent stage report/signer dispatch. Consequently no new `collaborationStageEvidenceReadTx` wrapper or read flag is needed; the existing R4 marker remains unchanged. Actual `EvaluateDeliveryEvidenceInTx` and `lockDeliveryArtifactInTx` wrappers still pass constant false, including when given a marked transaction.

Actual Go AST inspection found only three changed function bodies: `lockCompletionPreviewRoot`, `lockDeliveryPreviewIssue`, and `ListDeliveryEvidenceCandidatesInTx`. All seven other functions, all signatures and non-function declarations in the two files are unchanged. Package-wide AST caller inspection found exactly the existing candidate GET and pure preview POST caller for each private issue/root helper. No actual completion/authorization writer calls either helper. The full preview's additional observer, stage, attachment, session, R38/advisory and managed `boundSubmission` paths are untouched and have no new PASS claim.

All workspace/user/member/PAT/password/invocation/canonical spec/request/receipt/generation/expiry, policy/schema readiness, malformed-cursor/frame, final-clock and source checks retain their existing implementations. The five-second reader,250ms artifact timeout, default transaction isolation and10m package budget are unchanged. There are no new dependencies, migrations, SQL/generated edits, public parameters or interface changes.

## Verification evidence

- `gofmt` for the two files: clean.
- Exact compile-only `/opt/homebrew/bin/go test -c -o /Volumes/artisan/.multica-acceptance/pay241-r24-continuation-20261117-32uud2uo/r26-m1-candidates-read-r01/evidence/service-green.test ./internal/service`: exit0 in 18.596s. Cwd was `/Volumes/artisan/.multica-acceptance/pay241-r24-continuation-20261117-32uud2uo/r26-m1-candidates-read-r01/source-green-r01/server` and environment exclusively the parsed Main GREEN profile, SHA256 `3aaca9025f52b8c46674e9071a12b36262d6ffd5632f0189987c33a12257f101`. No TestMain/database execution or inherited environment. Binary SHA256 `16fa5594a6b2b78b06f450806b28d9fb8f41a8d5320b4d6a161ac96b934bde22`, 86,047,314 bytes.
- `/opt/homebrew/bin/go vet ./internal/service`: exit0 in 2.231s under the same source/profile; no tests executed. Compile/vet stdout and stderr are empty.
- Actual Go AST diff and package caller inventory: accepted the exact three-function boundary and original constant writer modes.
- Full source bytes/modes/path audit:5048 inputs;5046 untouched; exactly the two permitted paths changed; test086a unchanged; original RED drift0.
- Main normal result: `/Volumes/artisan/.multica-acceptance/pay241-r24-continuation-20261117-32uud2uo/r26-m1-candidates-read-r01/verification/green-r01/result.json`, SHA256 `c36bf7b021d6df51d3d31ec8a8ce5432ebaefa30cfed1267968ec72117c9c284`,15PASS/0FAIL/0SKIP, exact4-top/11-leaf/15-named inventory, no missing/extra/pending/parse/source drift, no rerun and final DBconnections0 as recorded by Main.
- Main race state at report creation: `PASS`, exact permit SHA256 `2548f3a2b8faacd258215fefaf806e5eb50c63073dc61afd3021459e182dd377`. Its recorded result SHA256 is `57e05bb390a6b662f5e8af687d1838b057e463a5be69a7db03e823917e784043`; 15PASS/0FAIL/0SKIP.

The same original test proves the four read orders together with real writer and authority controls. Its per-read assertions cover complete rows/counts for72 named relations plus the original stage-exit snapshot. These assertions do not claim equality of all211 tables across fixture setup/cleanup; preserved configuration/knowledge invalidation history remains outside that per-read set. The artifact fixture creates real tar bytes but does not expose the archive path; database metadata equality is not a fresh filesystem-byte audit.

Raw command/result logs, AST/caller evidence, the full source manifest, exact diff and static SQL references are under `/Volumes/artisan/.multica-acceptance/pay241-r24-continuation-20261117-32uud2uo/r26-m1-candidates-read-r01/evidence` and hash-bound by the JSON report. Original RED and prior reports remain unchanged.

## Remaining scope

Source is frozen for Main's single race run and independent final candidate review. Existing read/writer controls are Main's bounded verification decision. No new database run, retry, source installation, API, model account, Git operation or configuration change was performed by this agent. Candidate acceptance does not accept the complete preview/R38 path, original four Web/Desktop scenarios, A01–A12, S2, parent24 or S3–S5. The separate managed-submission `boundSubmission` issue still needs its own real regression and bounded implementation.
