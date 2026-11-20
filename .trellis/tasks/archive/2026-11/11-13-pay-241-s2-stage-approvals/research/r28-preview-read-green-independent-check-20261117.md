# R28 submission-preview GREEN independent check

Verdict: **ACCEPTED_R28_SUBMISSION_READ_GREEN_NORMAL_RACE** for the private
three-file repair and union of 26 named tests only. No additional defect was found in
this scope. Original completion controls, ROOT/runtime integration and full
preview/S2 acceptance remain separate. No checker test/build/DB/SQL/API/browser,
signals, Git or subagent execution occurred; no source/config/history was edited.

## Findings (fixed)

- The owner repaired the genuine R28 read/read contention without changing
  authority or writer defaults. Only `collaboration_completion_preview.go`,
  `delivery_completion_collaboration.go` and `collaboration_stage_completion.go`
  change. Four existing read dispatches share the exact value marker around
  the actual borrowed Tx, only for validated submission after current-version
  checks. The nested retained issuer/report/run/source chain keeps that marker.
- Intent, facts, final-callback facts and stage-state writer entries explicitly
  select constant false. One common body preserves their original predicates,
  callback/clock order and signatures. Canonical child and stage-task read
  branches use compatible fences; current task getter/virtual absent head and
  root/schema protections remain. No duplicated validator or public read flag.

## Findings (not fixed)

- The additional 7-top/58-named/38-leaf original completion/default controls and
  matching five-product builds are separate main-owned evidence. They are not
  claimed passed here, and this is not a current 116-test result.
- Nonempty attachment locks, self/admin/ordinary/advisory behavior remain on
  original dispatches. Complete read compatibility for those variants, ROOT
  integration, final runtime binding, original four/A01–A12/S2, parent and S3–S5
  remain open. Historical R24/R25 holders are not attributed by this regression.

## Verification

- **Source:** all 5,051 actual GREEN files/modes match; exactly 3 product files
  differ from RED, with **5,048 protected files unchanged**. Applying
  each recorded patch to its RED bytes reconstructs the complete GREEN file.
  Original 602-line test 318850, managed f1aa and admin 658b bytes are unchanged.
- **Caller/structure audit:** corrected real Go AST r02 agrees with manual
  review: original signatures and all unrelated bodies unchanged; only preview
  calls the new read entries. No SQL/generated/migration/dependency/type change.
  Original QA/access/observer and immutable submission/op/handoff checks stay.
- **Lint:** retained service vet/gofmt pass, no new checker execution.
- **TypeCheck:** retained single isolated service compilation exited 0, empty
  logs; binary `f9ec53f6fa624f06e05aec6788cae81cd7ba8cfb6c62bccab36d5665434eee3d` matches.
- **Tests:** same-source actual normal/race each **7 top-level / 19 leaves /
  26 named PASS**, exit 0; no FAIL/SKIP/missing/extra/pending/parse/drift/retry.
  These are new 18 plus retained binding/admin 8; profile/command/raw hashes agree.

| Read order | Normal held → second PID | Race held → second PID |
| --- | --- | --- |
| Standing R4 first | 15422 → 15445 | 18127 → 18155 |
| Preview first | 15449 → 15473 | 18160 → 18189 |
| Sibling previews | 15477 → 15499 | 18196 → 18222 |

Every second actual read succeeds while the first remains held. Standing R4
reaches its real QA/session/run/artifact chain; other first previews retain
active Tx status and commit only after the second terminal. Stable current
target/source/binding/observation/expiry comparisons pass. No fake ready or M1
grant is introduced. Preview read committed, R4 repeatable-read, five-second
actions, 250ms artifact locks and ten-minute suite budgets remain unchanged.

Real observer writer-first/reader-first and plain/marked original stage-state
UPDATE controls pass. Disabled account/password 404, manager 403, QA issuer
blocker, missing binding and actual stale stage controls preserve distinctions.
Retained human/admin binding and exact saved UTC microsecond expiry also pass;
all non-permission flags remain false and tested denials reveal no private data.

All 64 complete 72-relation/stage-exit comparisons match; M1 authorization,
terminal/evaluation counts 0, with no admin/adapter effect assertion failure.
Both final other-connection counts 0; ledger/indexes unchanged. Fixture cleanup
handoffs and configuration/knowledge history growth remain recorded, not erased
or represented as a whole-DB/archive-byte audit.

The first AST utility panicked on `*ast.FieldList`; its failed logs/bytes remain.
Only new utility r02 corrected that tooling failure, with no product/test edits.
Original RED/result/report e7ae remain sealed and unchanged. This reviewer wrote
only this report, JSON and new private `read-green-independent-check/` support.
