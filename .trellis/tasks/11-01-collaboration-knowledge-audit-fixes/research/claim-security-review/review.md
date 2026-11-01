# Claim security review

Verdict: **pass-with-notes** for product30edf3e36 including H3/M394e790d2d. Root reviewed the H3 implementation written by the claim child, independently of its implementation reasoning; the separate security child ended with429 and did not produce a completed review. No product edits, database commands or Provider calls were performed in this review.

- No new trust source or grant is introduced by complete metadata reads. Current keyset uses timestamp plus UUID in the same descending order; runtime selection checks selected row identity and missing/version-lost deliveries fail closed.
- Exact session decisions retain existing native/config/provider/runtime/agent/scope checks and revalidate the accumulated manifest before private preparation. Removing a count-based refusal does not turn unsupported options into positive knowledge admission.
- The runtime key pass can stop at the first exposure because that result refuses unsupported execution; skipping later decode in this branch cannot authorize a process. Corrupt prior metadata and lost selected rows retain terminal refusal.
- Legacy NULL-version rows are excluded only under the existing metadata CHECK that requires absent delivery; negotiated data is not filtered to only previously exposed rows.
- Three concurrent partial indexes contain metadata only; no FK/cascade, policy enlargement, source-content projection or new credential is added. Owner/provider namespace remains intentionally cross-workspace.
- Owned settings precede both filtered option lists; competing settings retain the existing removal rule. A malformed later required-value option can cause ordinary CLI parse failure but cannot consume the already-parsed owned pair. Installed CLI parsing was not tested.
- Complete runtime namespace keys use O(N) metadata memory and may scan a broad negotiated partial index. This is an explicitly accepted cost, not evidence of indexed production-size performance. Existing concurrency/fence semantics were not changed by H3; this review does not claim a new concurrency proof.

This is a static security perspective, not a replacement for named DB tests, the separate correctness/test-validity reviews or final candidate acceptance. Source hashes and exact scope are in review.json.
