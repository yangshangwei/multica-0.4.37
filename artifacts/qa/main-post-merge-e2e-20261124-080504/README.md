# Post-merge acceptance evidence

Tested candidate: `103e2bc329a9fb845d9f45c20b2a9bb3ebb54625`.
Campaign: `main-post-merge-e2e-20261124-080504`.
Acceptance work is complete; product verdict is **FAIL**, with no release approval.

The task report and defect list are under
`.trellis/tasks/11-24-main-post-merge-e2e-acceptance/`.
`coverage-audit/snapshot-24/` records all 25 domains and 11 configuration lanes.
The default run remains 400 PASS / 138 SKIP. Subsequent results for those 138
identities are 117 PASS / 21 FAIL; retaining every historical failure gives
114 PASS / 24 FAIL. Neither count hides the original skipped or failed attempts.

This Git delivery contains the task documents, offline accounting code, final
named ledgers, source digests and selected execution/cleanup receipts. Exact
paths, byte lengths and SHA256 digests are in `publication-manifest.json`.
Original evidence is copied into Git without rewriting its hashes or outcomes.
The manifest does not include itself.

The complete local campaign is approximately 892 MB. Raw application logs,
Playwright traces, screenshots, intermediate attempts, binaries, task-private
launchers and credentials remain at their original local paths. A referenced
path or digest in a committed receipt does not imply that its raw file is
included in this delivery. Absolute paths retain the original machine's
provenance and are not portable execution instructions.

The offline accounting regression suite can be checked from the repository root:

```sh
python3 -B -m unittest discover -s artifacts/qa/main-post-merge-e2e-20261124-080504/coverage-audit -p audit_selftest.py
```

Rebuilding a full campaign audit requires the retained local raw evidence.
Do not run historical launchers against another checkout, credentials or
database allocation. This evidence commit changes repository HEAD but does not
change the tested candidate recorded above.

Final current-resource cleanup passed: the campaign services are stopped,
182 service ports are closed, and owned processes and locks are absent.
Databases and volumes are retained. CLI03's earlier fallback volume cleanup
remains a historical failure despite the final absence of those resources.
