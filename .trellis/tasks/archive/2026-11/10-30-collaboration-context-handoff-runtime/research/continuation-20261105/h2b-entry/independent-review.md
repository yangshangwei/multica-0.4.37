# H2b actual-result independent review

**Pass with notes: all three H2b trials pass.** Independent read-only verification
passed289 checks with zero failures. H2b's distinct source-read requirement is
met in every measured continuation. H3a is not authorized by this review.

Each member result is 5,782 bytes, while startup exposes only the 4,096-byte
prefix and does not contain the decisive code. The measured model makes exactly
one CLI `collaboration handoff source` request per trial with the exact member
source ID, run ID, SHA-256 and version. The successful source response returns
the full original content, its original hash and total byte count. The matching
Provider tool-result frame visibly contains the decisive code. Each answer then
matches the strict MEMBER_RUN / MEMBER_CODE / WOKEN_BY grammar, exact member
identity and recorded proof wake cause, with no decoy. This is source-read
evidence, not an answer-only pass.

All nine runs have complete lifecycle, checkpoint, termination, outcome, proof
and recording seal evidence. The three real continuations observed Claude CLI
2.1.318 and model `claude-opus-5-5`; fake seed roles remain separately labeled.
Ten actual CLI captures match the upfront binary and launch cwd/run IDs. Accounting
is **3 new paid + 6 fake seeds**, cumulative **10/25, 15 remaining**. Original
E0a/H1/H2 records remain unchanged. Two terminal `stop-intent` poll requests
returned503; mandatory writes/ACKs and no Runner errors remain complete. These
poll observations are retained without inventing an all-2xx requirement.

The operator delta is exactly two files: two marketplace `lastUpdated` values and
the builtin usage queue UUID. The directory catalog JSON bytes are identical
before/after in this invocation; installed plugin registry, all three reported
loaded plugin lists and all 1,970 loaded external-plugin leaves remain unchanged.
Full tree digest still changes, so no whole-tree stability claim is made.

Root may accept H2b's three-of-three source-read behavior and disposition this
specific timestamp/usage delta. Before H3a, rebind current operator/source/CLI/DB
and establish a new model window; no automatic continuation. H2b does not claim
full reader-matrix or results/causes pagination beyond32.

Exact source, tool-result and delta checks are in `independent-review.json`;
`verify_h2b_evidence.py` reproduces them from private artifacts. The preliminary
H2 reviewer report and its stricter poll assertion remain preserved in H2's
directory. No tests, Go, database or model calls were made by this reviewer.
