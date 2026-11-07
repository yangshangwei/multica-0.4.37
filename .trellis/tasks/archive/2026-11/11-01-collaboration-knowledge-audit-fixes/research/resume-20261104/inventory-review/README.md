# Independent complete-check inventory collector

`collect.py` only reads pinned evidence and writes new research reports. It never starts a test, database, service, browser, CLI Provider or model. Its source SHA-256 is `6acef959f12c095776aa9f8f8812c5d12273a2c22f01db404eee9f6f986b62aa`.

Run it after the sole executor has finished and supplied actual final paths/hashes:

```bash
python3 .trellis/tasks/11-01-collaboration-knowledge-audit-fixes/research/resume-20261104/inventory-review/collect.py INPUTS.json --label final-v15-01
```

The label must be new. It creates a mode-0700 directory exclusively below this research directory; all output files use exclusive creation. It preserves the original raw input files and every prior result.

## Input contract

```json
{
  "candidate_sha": "d41573e3a4eb3bd6c36feed3f0d542a0e066ee0f",
  "candidate_root": "/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103",
  "full_result": {"path": "ACTUAL_FINAL_V15_RESULT", "sha256": "ACTUAL_FINAL_SHA256"},
  "raw_log": {"path": "ACTUAL_FINAL_V15_RAW_LOG", "sha256": "ACTUAL_FINAL_SHA256"},
  "browser_report": {"path": "ACTUAL_FINAL_BROWSER_JSON", "sha256": "ACTUAL_FINAL_SHA256"},
  "go_directory": "ACTUAL_V15_PER_PACKAGE_DIRECTORY",
  "expected_go_package_count": 82,
  "expected_browser_case_count": 441,
  "skip_disposition_review": {"path": "ACTUAL_INDEPENDENT_SKIP_REVIEW", "sha256": "ACTUAL_REVIEW_SHA256"},
  "final_audit": {"path": "ACTUAL_FINAL_AUDIT_JSON", "sha256": "ACTUAL_AUDIT_SHA256"},
  "audit_selector": [],
  "live_proof": {"path": "ACTUAL_ROOT_LIVE_PROOF", "sha256": "ACTUAL_LIVE_SHA256"},
  "live_selector": []
}
```

Supply `expected_go_packages` as the frozen list of all package identities when available. The collector separately verifies one completed metadata/raw pair per package, distinct recorded per-package database names, actual raw hashes, all named run/terminal pairs and metadata counts. No-test-files packages retain their actual compile-only terminal; a silent package pass with zero named execution cannot pass inventory verification.

The current initial independent skip review is `../task7-contract-review/review.json`, SHA-256 `111f08f6bf423ea27e698acfc6fc0c5cdc4dfb038ff0c070ed3a5c8fca531b73`. It covers all 22 actual v14 Go and 91 browser skips. Reuse is allowed only for the same actual case/test identity and reason while the reviewed source and shared guard hashes still match. New or changed skips need independent dispositions. For new TS skips/todos, a supplied review can add `ts_skip_review.rows` with each `package`, `file`, `fullName`, `reason` and `sources` entries (`path`, `sha256`) identifying the exact reviewed declaration/guard.

`audit_selector` and `live_selector` select the actual top-level/nested audit object from their hash-pinned files. The audit object must explicitly report `status: pass` and `wholecheck_passed: true`. The root live object must explicitly report `status: pass` and the same candidate SHA. Missing or differently shaped proof cannot grant the shared gate; the named inventories are still reported separately. Adapt selectors to actual supplied evidence rather than generating approval facts.

## Output and decisions

The collector extracts only whole actual Turbo-prefixed Vitest JSON reports from the canonical five packages (`core`, `docs`, `views`, `web`, `desktop`). It retains the exact JSON substring and every actual `assertionResults` leaf with package/file/name/index/status. Argv/reporting markers, missing/truncated reports and multiple distinct same-package reports cannot become complete leaf evidence. Identical repeated log copies are recorded as occurrences and are counted once.

The installed Vitest 4.1 reporter defines `numTotalTestSuites` using nested suites. Actual file count is `testResults.length`; leaf count is the full assertion array sum. Passed/failed/skipped/pending/todo aggregates are reconciled individually. Actual tested file paths must resolve within the frozen candidate. Pending/queued leaves or nested suites block completion. JSON does not export retry counters, so the collector makes no fabricated TS zero-retry claim.

It writes actual TS files/leaves/skips, Go packages/leaves/skips, browser cases/skips and exact reused/unresolved explicit-skip dispositions as JSONL, plus one `summary.json`. Every browser case keeps its attempt statuses, final terminal, expected/report status, annotations and retry count. It checks all 441 identities and every aggregate; absent terminals, undeclared skips, retries/flaky outcomes, missing names or count differences remain blockers.

Source-reviewed explicit skip/todo is a complete independent terminal. It is never counted as pass and never automatically makes a permitted whole check fail. Unreviewed/changed/unexpected skips cannot grant `shared_gate_ready`.

`complete_named_inventory` reports structural completeness. `shared_gate_ready` additionally needs clean actual test/package results, every explicit skip disposition verified, actual make exit 0, final audit pass and root live proof pass. The script does not close #7: new CCG1 normal/race, CCG2 OFF/ON and independent final source review remain required.

## Verified preparation

- `synthetic-parser-controls-package-contract-v2-final/result.json`: fourteen finite format controls pass on this exact collector hash, including unexpected UI/foreign reporters and missing docs. These are synthetic parser controls, not product acceptance.
- `historical-v14-missing-ts-control-package-contract-v2/summary.json`: actual v14 input was independently read without re-running any test. It yields 82 Go packages, 20,963 named pass/22 skip, 441 browser cases (334 pass/16 fail/91 skip), and all 113 individual skip dispositions verified. It yields **zero TS leaves**, correctly marks complete inventory/shared acceptance false and exits 1 because the five actual reporter JSONs are absent. The original 1,069 TS file terminals/13,244 reported cases remain historical file-level evidence.

No new product test, service, database or Provider operation occurred. Knowledge allocation/calls remain 0/0 and production knowledge defaults remain false. Actual final whole v15 inputs are still pending. The completed TS phase is independently retained below.

## Corrected package contract and actual TS phase

The earlier collector expected UI and omitted docs. That package assumption was wrong. Its exact original bytes/hash, README and controls are retained in `versions/v1-777610239767e016/`; all original synthetic and negative reports remain at their original paths. Those synthetic checks establish parser behavior only and never established the real task package set.

`package-contract-repair-v2.json` reads all ten frozen workspace `package.json` files plus the root package/Turbo/workspace config. All current bytes match `d41573e3a4eb3bd6c36feed3f0d542a0e066ee0f`. The root excludes mobile, and the five implemented test scripts are exactly core/docs/views/web/desktop, each `vitest run`. UI has no test script. The collector accepts only this fixed set; an input override cannot replace it. Extra scoped/unscoped reporter packages are recognized and rejected. The canonical Vitest reporter/count semantics remain unchanged.

`actual-v15-ts-stage-package-contract-v2/summary.json` independently preserves the same sole v15's five actual byte spans and their original byte offsets/line numbers, with all five raw-object hashes equal to `v15-execution/TS-phase-observation.json`. The spans remain byte-identical in the original append-only log after collection.

Actual TS results: core 4,455/331 files; docs 62/9; views 7,165/593; web 290/34; desktop 1,272/102. Total **13,244 named pass / 1,069 files**, 0 failed/pending/todo. Every leaf name is copied from its real `assertionResults`, and every tested file resolves within the frozen candidate. `task7-canonical-ts-greens.json` also binds the eight actual M4/M5/L8/L9/L10/L11/L12/L13 canonical green leaves.

The TS-only extractor is `extract-ts-phase-v2.py --label <new-label>`. Its output always retains `wholecheck_accepted=false` and `shared_gate_ready=false`: Go, browser, final identity audit, live proof and CCG/final #7 review remain separate outstanding gates. It starts no product test, service or database and performs no Provider call.
