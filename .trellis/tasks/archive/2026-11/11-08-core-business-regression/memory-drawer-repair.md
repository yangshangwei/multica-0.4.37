# Two-runtime memory journey: modal resource lifecycle

Original `controlled/controlled-5/pw-22` failed before authorization because navigation to `#memory` automatically opened the Shared context modal. The error context contains the expected issue heading in an inaccessible background and the visible Shared context dialog. The test attempted the background Host consent control without dismissing the drawer.

Only `e2e/collaboration-memory-runtime.spec.ts` changed. The test now asserts that each anchored drawer really opens, closes it with Escape and waits for hidden before Host controls, opens Shared context through the visible Issue resources toolbar before editing, closes it before Resume, and reopens it for final attributed history assertions. No forced clicks, product behavior changes, fabricated state, timeout increases or assertion removal.

Exact file hashes: before `0a28c4248892aafe8cfca5ad1f99c37c3a8cdef2276c42239d072bcd9169d18c`; after `f4b7fb544330f429bbe9107b684b95ce94aad6bf45f2920593d5f0dd4db9110b`. Root and QA copies match. The delta is test-only and requires precise execution-source rebinding; compiled package inputs remain unchanged.

Complete focused replay on integrated `6201a5810`:

```sh
python3 /Users/artisan/.multica/qa/core-20261108/controlled/run.py \
  --execute --run-id memory-1 --ids pw-22 --host-ipv4 192.168.65.254
```

**1 PASS, 19.0 seconds, zero skipped/retried cases, exit 0.** Actual separate native runtime profiles and run-local CLI perform memory version 1 → 2 → 3, preserving startup versions, agent/user attribution, distinct workdir/session identities and final proof/settlement checks. Both daemon cleanup records and restored deployment gates remain asserted by the original test.

Private evidence: `/Users/artisan/.multica/qa/core-20261108/controlled/memory-1/pw-22/{results.json,case-inventory.json,artifacts/}`; parent `final.json` records `state=complete`, `original_api_restored=true`, no cleanup errors. Exec session 72466 was observed terminal, exit 0, before returning API ownership to root. Original failure remains under `controlled/controlled-5/pw-22/`; include the separately named `memory-1` path in final aggregate coverage.
