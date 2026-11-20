# Daemon review resolution in continuation tree

> Final takeover update (2026-11-20): the previous pending full Views r4 run completed with exit0,608 files/7441 tests. Original report text below records its earlier review cut. The213-file source manifest was independently verified unchanged before the documentation/comment closeout; all selected normal/race evidence is retained. Final shared-branch integration has separate evidence in `codex-takeover-20261120.md` and the final delivery record.


Source: frozen 6b 21-file manifest `/tmp/f3-main-s6k/base-r2/manifest.txt`, checked before copy. Original implementation/reviewer worktrees remain untouched.

- KD2-01/02/03/04/05/07/08/09/12: adopted the independent reviewer’s three test files (`budget_usage_delivery_regression_test.go`, `budget_usage_timeout_test.go`, `budget_usage_begin_race_test.go`). They check HTTP timeout retention, exact server identity, status classes, long incomplete response, redirect isolation, deduplication, resend cadence, settled-record removal, and record-creation race. Existing implementation satisfies them.
- KD2-11: corrected the flaky stop test to count requests at client transport dispatch; delayed server reads after client cancellation are not new sends. No product behavior changed.
- KD2-06: design §11.10 retains records at most seven days, and does not promise one final send after retention. Kept existing retention behavior, corrected the overclaim in the production comment/test name, and adopted both deterministic ordering tests. Expiration first drops locally with a warning; direct send first can deliver. Neither releases server hold.
- KD2-10: same server/profile switching accounts may make a pending report get a final 404. S1 accepts this as a bounded coverage loss with retained unknown hold, consistent with its existing 404/7-day policy. No wrong-account ledger write or cross-server send occurs. Account-bound retention is a follow-up before completeness/settlement can be claimed.
- KD1-02: on-disk version equality is correct; add missing-version/zero-version regression during final review.

Verification: guarded targeted daemon suite `daemon-normal-01.jsonl`: 298 named passes, zero failures/skips. Race and final integrated verification still pending.
