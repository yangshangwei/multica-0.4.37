# Independent claim test review

Reviewer `/root/claim_test_review`. The delivered NEW_TASK payload is empty;
the exact bounded assignment is recovered from the visible parent spawn record.
Review target: `a4e444bdf90c84939189fc2981cdacab50c678de`, product
`30edf3e36` including H3/M3 `94e790d2d`, base `4e3b61500`.

Product is read-only. Only this review directory is owned. No database,
migration, installed Agent CLI, Provider or dependency operation is permitted.
Root's ongoing handler/migration checks and their evidence are untouched.

Scope: independent test validity for deep history, namespace completeness,
corruption/missing metadata, keyset boundaries, red/green provenance,
EXPLAIN scope, claim/finalize disposition and managed-argument construction.
No final verdict yet.

## Verified so far

- All inspected log hashes match the retained manifests. H3 production query,
  generated SQL, reader and migration hashes map exactly to94e790d2d; the old
  query/generated SQL are byte-identical at6aed33aad and the actual red3110295d0.
- Full service rerun JSON has3299 named run events:3281 pass,18 skip,0 fail,
  terminal package pass. The original600s package timeout and named successes
  remain preserved, not converted into a pass.
- Independent guarded pure managed-argument construction run:9 passes,0 fail/skip;
  DATABASE_URL and real-agent gate absent. A settings-last overlay mutation is
  running; production files remain untouched.

## Nonblocking coverage note CT-1

Current exact-session growth cases set the retained authoritative row one day
older and populate the260 newer rows as claimed-only. Although inserted rows
use date_trunc(second,now()), no decisive exposure/corrupt row is placed inside
an equal-timestamp group beyond page1. A timestamp-only cursor regression could
skip newer benign claimed rows yet still read the older decisive row, leaving
these assertions unchanged. Current production SQL correctly uses the tuple
(created_at,id); this is a regression-strength gap, not a found product bug.

Recommended follow-up: assign one fixed timestamp to all negotiated rows, first
prove valid resume, then corrupt a deterministically selected row below the
first128 IDs and require knowledge_context_changed. A timestamp-only cursor
mutation should fail. No DB run or mutation of production code was performed
for this note.

## Completed

Verdict:pass-with-notes. Independent managed construction9/9 passes; append-last overlay killed with7 failing subcases plus parent (expected exit1). No production file changed and no DB/Provider operation. CT-1 is a low nonblocking tie-boundary coverage recommendation; current tuple SQL is correct. Full report and machine record:review.md/review.json.
