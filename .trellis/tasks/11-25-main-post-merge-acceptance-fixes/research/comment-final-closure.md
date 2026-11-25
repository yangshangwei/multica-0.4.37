# Research: Final comment authority closure

- Query: Independently verify the final principal-bound comment fix with raw owning-package normal/race results and actual-issued credential controls.
- Scope: internal; defensive source and retained execution evidence review.
- Date: 2026-11-25

## Findings

**ACCEPT D01's bounded comment management/principal correction. No remaining code finding in this reviewed scope.** This does not change the separate generic upload contract or approve unrelated service/native failures.

The local commentMutationActor design/source was previously reviewed. Final source still confines the change to Update/Delete identity decisions and context-derived edit lineage; shared resolveActor, machine classifier and Auth adapters remain unchanged. The valid header-pair concern is resolved by verified task context or bound member identity, not ignored as a false positive. A raw client hint is never the positive author grant. Real task self and human self/admin controls remain; Cloud self uses its bound member identity without inheriting administrator rights.

| New run | Parsed named results | Exit |
| --- | --- | ---: |
| principal-handler-focused01 |31 PASS |0|
| valid-principal-green01 |1,081 PASS |0|
| principal-handler-normal01 |6,211 PASS /7 SKIP |0|
| principal-handler-race01 |6,211 PASS /7 SKIP |0|
| principal-router-normal01 |2,284 PASS /8 SKIP |0|
| principal-router-race01 |2,284 PASS /8 SKIP |0|

All six raw JSON streams parse cleanly, contain complete started/terminal inventories and successful package terminals, and match their retained counts/inventories. The original40m/p1/parallel2 and supported Go1.26.11 command remain. All six frozen file hashes/modes, environment hashes, tool hashes and source-manifest bindings independently match. Each final full-package run has its own database.

Both final router runs include134 named PASS terminals for each actual-claim legacy/password family and533 for the valid-hint family. The claim tests obtain mat_ from the real local daemon claim API, validate the persisted token binding and start through the API; the issued token is not replaced with a fixture row before the terminal cancellation control. These are genuine issuance/Auth/router checks with synthetic local fixtures, not a deployed Fleet or provider/model test.

The focused31 controls and both full handler runs include the new binding cases, direct backstop and legitimate autopilot edit-lineage test. A later source change after handler execution updates one explanatory comment only; current router final source includes it. No executable product difference invalidates the handler results. Updated security spec now describes the local verified-principal resolver rather than only the earlier role predicate.

After-resource records show database_connections=1 for each run. The preserved audit query includes its own connection; this means zero other clients at observation, not an orphan connection. All final package databases have zero workspaces/comments, with5 handler or3 router retained users as recorded. Databases remain parent-owned evidence; this review did not drop or mutate them.

## Files found / related specs

- `comment-final-closure-audit.json` — independent raw counts, exact named controls/skips, source/mode/config/tool comparisons and resource interpretation.
- Public comment `principal-*` and `valid-principal-green01` records; frozen external sources — exact executed evidence.
- `server/internal/handler/comment.go`, comment-authority tests, valid-hint router tests and autopilot fixture — reviewed bounded fix.
- `.trellis/spec/server/security-boundaries.md:383` — matching final authority contract.
- `comment-mutation-principal-review.md` — earlier focused source/design review retained.

## Caveats / Not Found

- No tests, services, credentials, external targets, SQL/Docker/Git or source edits executed by reviewer; research outputs only.
- Seven handler/eight router skips remain exact named exclusions in the audit; no skipped identity is counted as PASS. Actual package assets include examples, correcting the earlier incomplete-export run without rewriting its failure.
- Final global resource cleanup remains parent-owned. Generic UploadFile association and broad legacy actor attribution are outside this correction.
- No external references needed; do not sum normal/race/focused counts into unique product coverage.
