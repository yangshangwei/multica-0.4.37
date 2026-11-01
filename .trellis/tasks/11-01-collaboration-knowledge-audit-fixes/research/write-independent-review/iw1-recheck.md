# IW-1 independent delta recheck

Verdict: **pass** for `46454019190720d0a95c61b1d87284ff7af75cf6` relative to `d4bc37a86`. IW-1 is closed for this product delta. No blocking issue found. This is the independent recheck of the previously identified target-budget clock gap, not a replacement for final combined-source validation.

The approval branch obtains the existing database timestamp unconditionally, including a candidate without its own expiry. The same timestamp now drives both candidate expiry and the existing active-target predicate. The target counter and candidate locks are unchanged; no schema, query, authorization, grant or dependency was changed. An application clock ahead can no longer hide a still-active contribution, and a lagging application clock no longer supplies the budget query's cutoff.

Budget refusal happens before target revision advance, superseding an approved predecessor or approval UPDATE. It returns the existing pending candidate receipt, which the normal operation transaction seals under the original request. The regression checks that only one contribution remains approved, and after restoring the application clock the original request returns the byte-equivalent refusal instead of recomputing a different outcome. Existing expiry, correction rollback and unexpired-NoRows semantics are preserved.

The actual red reproduction is `IW1-budget-red-r2.jsonl`: one named failure shows a second contribution incorrectly approved while the first64KiB contribution is still active on the database clock. The earlier zero-test compile fault is retained separately and is not counted as reproduction. Normal and race controls each report7 named passes,0 failure/skip. Their raw hashes and current source-file hashes match the manifests. Those DB tests were run by the coordinator, not rerun by this reviewer.

Independent current-source service build and vet exit0; `git diff --check` exits0. Exact commands, SHA/tree/file hashes and log hashes are in `iw1-recheck.json`, `iw1-evidence-check.json` and `iw1-{build,vet}.json`.

No product file, database, migration, Provider or installed Agent CLI was touched. The prior full service3262/18skip and migrations536 results keep their original product scope; final combined full suites must cover the new delta. No separate reviewer DB lagging-clock case is claimed.
