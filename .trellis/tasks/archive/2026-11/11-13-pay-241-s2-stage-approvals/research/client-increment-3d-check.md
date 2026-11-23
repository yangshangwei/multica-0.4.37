# Client increment 3d — independent check

Reviewer: `/root/s2_client_check_resume`, 2026-11-14.

**APPROVE after two local transport corrections.** No open findings remain in the bounded client3d scope. Server2d, actual Web/Desktop/API acceptance and complete A01–A12 remain separate. No real API, database, browser, Provider or model account was used in this review.

## Findings (fixed)

### C3D-C01 / P2 — an absent new route incorrectly declared source-access loss

Files: `packages/core/api/client.ts`, `packages/core/collaboration/completion.test.ts`.

The new completion reader classified every404 as `denied`, including a plain old-router response without a structured code. The normal/admin completion Query consequently invoked the workspace source fence and removed still-readable R4/T4/completion observations. A405 from an unsupported route escaped as an exception rather than the unavailable capability result used by existing R4/T4 readers.

Independent API counterexamples reproduced both classifications. A further real Query/source-fence unit counterexample showed the previously stored R4 entry becoming undefined after a plain404; the structured `not_found` control correctly cleared it.

The reader now returns `{state:"unavailable",code:null}` for code-free404/405, preserving existing source observations. Explicit403/404 denials retain the current source fence. Unknown405 responses remain unavailable. No HTTP contract, authorization permission, request key or automatic retry changed. The earlier test expecting a plain404 to be denied was corrected alongside the independently failing cases.

### C3D-C02 / P2 — completion candidate pages ignored the requested bound and cursor progress

Files: the same two Core files.

The candidate reader checked root/source identities but accepted more items than requested and a `next_cursor` identical to the request cursor. The latter left the shared Next action with a repeated cursor to ignore, instead of reporting an incomplete page.

Independent responses with two candidates for `limit:1`, and with an empty page returning its original cursor, were both accepted before correction. The reader now requires the requested count bound and forward cursor progress. A bad page becomes unavailable; it is not silently truncated or treated as successful pagination. No new request is created.

## Findings (not fixed)

None within the agreed scope. The main session confirmed that this increment provides the **current completion consumer's review UI**, plus an existing-authorization reference mode. It does not add a separate source-issuer workbench. Issuer-versus-consumer separation uses the existing M1 authorization API and the consumer's reference flow; a complete cross-issuer UI creation journey is not claimed.

Two additional shared-UI controls verify that the reference flow needs no artifact read: a server-permitted externally issued grant is looked up, then used by the original workflow operation ID and confirmed by another consumed-grant GET; a server-denied reference cannot enable completion or send a completion POST. These are client unit fixtures. The server's actual cross-issuer/consumer authorization and wrong-consumer refusal remain required backend evidence.

## Review evidence

- **Source and ownership:** reconstructed the checked3c 2,526-file base plus the declared29-file S2 overlay into2,532 files and152 links. Every overlay/base/hash relationship and all25 author command/log/exit triples matched. No live knowledge-task or migration844–859 source entered the verification graph.
- **Closed targets:** ordinary, collaboration submission, self-result and administration variants are distinct. Root numeric issue revision comes from the candidate response; collaboration generations remain exact decimal strings. The ordinary source-equals-target invariant stays ordinary-only, and the ordinary submit helper refuses collaboration drafts.
- **Root/source observations:** candidate/preview requests name the actual root and selected ordinary child. Preview response parsing correlates root/source revisions, shared project, exact artifact and target. It cannot turn an ordinary-child completion preview into root authority. Stage-incomplete is non-authorizing in off/observe/enforce, with no binding or expiry.
- **Authorization and original request:** the shared form consumes a server-derived binding, freezes result/submission/admin intent, and creates the existing M1 authorization. Its operation ID becomes the final workflow/admin request key. Reload and unknown-result paths retain the original draft; bound workflow GET404 does not POST again. A new review after explicit revocation gets a new operation ID.
- **Two receipts:** normal completion first checks its original delivery confirmation, then checks the exact consumed authorization/operation/root/status/revision and the same returned submission ID. R38 checks its original organization/actor/admin result and its exact consumed authorization. A missing or contradictory second receipt leaves the final request unresolved. Ordinary receipts cannot substitute for collaboration receipts.
- **Current authority:** protected source generations isolate normal and R38 completion observations from late responses. Authority loss clears private observations while preserving minimal original authorization references. Scope checks surround request completion and draft mutations. R38 completion source URLs, cursors and error bodies retain the reviewed log-redaction boundary.
- **Activity:** stage metadata is a closed kind-specific union. Unexpected fields, private prose/hash/report payloads, invalid step values or unrelated details remove stage details rather than manufacture a decision. Execution/metric events remain safe hints. Existing activity identity, reference-only notifications and Query invalidation are reused; no new outbox or server-data Zustand store was introduced.

## Verification

All commands used the isolated declared graph and private HOME/TMP. Counts overlap across reruns and are not added as acceptance cases.

| Check | Result |
| --- | --- |
| Independent submitted-source Core regression |28 files,544 passed,0 failed/skipped |
| Independent submitted-source Views regression |11 files,110 passed,0 failed/skipped |
| First transport counterexamples |4 failed /9 existing passed; retained |
| Added Query/source-denial controls |5 failed /10 passed before correction; retained |
| Final Core after corrections |28 files,550 passed,0 failed/skipped |
| Final Views including reference controls |11 files,112 passed,0 failed/skipped |
| Final Core/Views typecheck |Both passed |
| Final changed-file Core/Views ESLint |Both passed, zero findings |
| Final frozen-source, link and ownership audit |Passed; three declared files changed |

The three reviewer changes are the API transport fix, its canonical Core regressions, and the shared reference-flow regression file. Product UI selectors and completion-reference semantics did not change; the E2E owner was notified.

## Live peer changes and final bindings

The live API file contains separate peer changes. Only the exact `readCollaborationCompletion` and `listCollaborationCompletionCandidates` span was patched. Reversing that span reproduced the captured live-before file byte-for-byte, proving the review edit preserved all other bytes at application time. The isolated S2 API hash intentionally differs from the live mixed API hash.

The live collaboration index and both collaboration locale files also received peer changes; the reviewer did not edit them. Their two added S2 exports and63 S2 locale values per language still match the checked source. The JSON includes the four live-versus-isolated differences. **Do not overwrite the workspace with whole frozen files.** Integrate the declared S2 patches/hunks and rebind the final combined candidate.

The final29-file S2 overlay is under `/Users/artisan/.multica/acceptance/pay241-20261114-resume/s2-client3d-check/checked-source/`. The JSON companion binds that overlay, the complete2,532-file verification source, three-file review patch, original red sources, commands and ownership audits. Original3d snapshots, mixed-graph failures and unrelated peer work remain unchanged.

## Remaining acceptance

The completed server2d contract and all real writer gates must still be integrated. The E2E owner must exercise the final shared normal/R38 M1 UI against source-matched API/CLI/production Web/Desktop, including original-key and consumed-receipt recovery. This review does not establish real artifact execution, external integration, A11/A12 or parent acceptance, and it performs no Git/index, server, E2E, spec, dependency or platform-route change.
