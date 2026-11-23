# Verification evidence audit

2026-11-23. Read-only audit of retained source hashes, Git blobs, package JSON streams, receipts, resource state and remote status. No tests/builds/browser journeys or product edits. The reviewer authored the modal and dbid fixes earlier, so this report is an **evidence/claim audit**, not an independent review of its own implementation; the parent separately reviewed those product/test diffs.

## Confirmed results

### Backend inventory and source

Independently parsed all accepted `backend/packages/*/test.jsonl` streams and their `receipt.json`, compared raw stream SHA-256 against `test-summary.json`, and matched package/top-level declarations from `expected-tests.json`. `TestMain` declarations were correctly treated as harness functions rather than test cases.

- Exactly **86 expected / 86 recorded packages**, every receipt exit 0 and every package terminal successful (or the Go no-test package skip).
- **25,564 named PASS / 24 SKIP / 0 FAIL**, **11,741 top-level PASS / 23 SKIP / 0 FAIL**.
- No missing package/top-level test identity, malformed JSON, duplicate terminal, started-but-unfinished named case, nonzero accepted receipt, or stream-hash mismatch found.
- All **5,288 entries** in `source-initial.json` still exist. Current private bytes differ only at the approved `server/pkg/dbid/writes_test.go`; its final hash matches the owning fresh race/vet receipt.

These support `backend-results.md`'s reconciled-package claim. They do not turn the original wrapper invocation into a success: `commands.json` retains race exit 1 and old-toolchain vulnerability exit 3. The separate accepted package replay closes the AST failure; Go 1.26.11 scanning closes the current-patch scan finding while tests themselves remain Go 1.26.6.

Verified durable paths under `.omx/private/qa-work/main-readiness-20261123/backend/`:

- `expected-tests.json`, `test-summary.json`, `skip-inventory.json` and all 86 `packages/*/{receipt.json,test.jsonl,stderr.log}`.
- `source-initial.json`, `final-audit-cleanup.json`.
- `dbid-final-command.json`, `packages/bd354a329a15/{receipt.json,test.jsonl}`, `logs/dbid-final-vet.log` (present, empty, receipt exit 0).
- `vulnerability-current12611-command.json`, `logs/vulnerability-current12611.log`: exit 0, zero reachable/imported-package findings; seven uncalled module-level advisories remain disclosed.
- `redis-auth-supplement-command.json`: separate opt-in execution exit 0; it must not remove the original default skip from the default inventory.

No missing path was found among the report's audited result links. The actual final audit/cleanup filename is `final-audit-cleanup.json`; there is no separate generic `cleanup.json` or `inventory-audit.json`, and the final report does not incorrectly link those names.

### Backend resource cleanup

`pre-cleanup-ownership.json` records successful other-client inspection with an empty client list. `final-audit-cleanup.json` records exact owner-labelled PG/Redis IDs, stopped states and ports. Independent current `docker inspect` by those IDs confirmed both containers remain `exited` and their recorded owner label is present. Loopback probes independently confirmed **54733 and 54738 closed**. No container was started, stopped, pruned or modified by this audit.

### Modal fix source versus actual browser result

The current private hashes of the three product files match both `responsive-dialog-fix.md` and `core/postmodal-focus/source-start.json`:

```text
34a3b45cba6427eca6c3c2f8abb39c057a52fd833cd830a0f3602dcf52de7ff8  packages/ui/components/ui/sidebar.tsx
b18233ef94415de56efb20fc1aaaa2ae93b023ff83794891098a13d99fe75c20  packages/views/layout/app-sidebar.tsx
f9c7a6c3f7d1b7db5a5e7a8d85717a4e7adadd317bac26420a88b33c8dddbc79  packages/views/collaboration/creation-form.tsx
```

`postmodal-focus/playwright.json` is **5 passed / 2 failed**, not a fully passing seven-case run. Its built Web identity is `oN4ToRAYN_RiHTCCs392_`. Both extended Web/Desktop rail originals and the unchanged recipient creation original passed; the remaining two failures were retained workbench/geometry fixture problems. The earlier independent fixture audit documents those exact causes and source assertions. This supports actual responsive draft retention and compact dismissal/focus within those tested journeys, not packaged Main or every native creation-success variant.

## Image policy and branch facts

Recomputed offenders from committed Git blobs at `73a28ed90fdcfd9eda1ac5d2970df5af2c486f94` against `dd87e0b33c1d6a64d488f7ee9ecd0a65984a68ff` using the unchanged 307,200-byte new/growth rule. All **121 paths, blob OIDs and prior/current sizes** exactly match the earlier `image-material-inventory.json`: **52,151,039 bytes (49.74 MiB)**, 113 files under `.trellis` and 8 under `artifacts`, none under served app bitmap directories.

The existing inventory/disposition still labels source `cb192014b`; the set is demonstrably unchanged at 73, but those documents should be refreshed to the final delivery source when adopted. This is a provenance-label follow-up, not a changed offender count. The proposed exemption remains reviewable and valid in rationale: retain original acceptance captures without history rewriting. It is **not an approved exemption**, an actual PR-body entry, or proof of remote CI success. The current policy still rejects the set without an explicit reason-bearing PR exception. The mechanism is PR-wide and requires checking the entire final set, not a per-file allowlist.

Observed local references: HEAD and origin feature branch both `73a28ed90`; origin/main `dd87e0b33` is their ancestor, `0 behind / 863 ahead`. This confirms Git fast-forward compatibility at the observed refs; no merge occurred and a final fetch remains parent-owned.

Read-only GitHub queries at audit time returned **no PR** for this head branch and **no workflow runs** for it. Do not call this branch's remote CI green or rely on a local exemption simulation as a completed PR gate. Earlier `large-pr-filter-review.md` remains a separately scoped existing-filter risk.

## Remaining claim boundaries

- The full browser attempt's original 22 failures and post-modal two failures remain immutable evidence. At the end of this audit the newer fixture/password run terminal files had appeared, but their outcomes were **not included in this audit**; final identity and conditional-admission reconciliation is a separate step.
- No current report may infer that all 538 browser identities passed, that every default skip ran conditionally, or that an earlier source hash is the final post-repair runtime.
- No observed evidence misstatement requires reverting a fix. Required final reporting adjustments are the image source label, exact original **18** failing-file union (17 subsequent files plus the separately passed recipient original), explicit CI/exemption status, and continued separation of per-package/test replays from a single pristine all-green invocation.

This report leaves implementation files and live acceptance resources untouched.
