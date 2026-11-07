# Current frontend checks

Frozen candidate `4c817dee9`: **FAIL**, with13,320 passing and4 failing tests across1,074 files in5 packages. These are separate package executions after Turbo fail-fast; no full command success is claimed.

| Package | Pass | Fail | Files |
| --- | ---: | ---: | ---: |
| core | 4460 | 1 | 331 |
| views | 7236 | 3 | 598 |
| docs | 62 | 0 | 9 |
| desktop | 1272 | 0 | 102 |
| web | 290 | 0 | 34 |

All15 non-Mobile lint/typecheck tasks passed with30 warnings/0errors. UI export checks passed. Verification/controller Node tests:101PASS/0FAIL (89 top-level and12 child tests), plus the separate Go test-wrapper shell regression PASS.

The diagnostic-route mismatch reproduced14PASS/1FAIL; Skill default/localization mismatch reproduced106PASS/3FAIL. These repeat executions are excluded from totals. See `defects-current.md` for exact source contracts and repair handoffs. The old admin filter clock issue passed its8-test file at the actual current date.

`units-first` was manually interrupted to prevent report output from entering the frozen source; no generated report was produced. That operational attempt remains in the ledger, followed by ordinary reporting to the private logs. Tests were not weakened or retried automatically. All raw command records and hashes are in `frontend-result.json`; raw outputs are under the ignored private directory.
