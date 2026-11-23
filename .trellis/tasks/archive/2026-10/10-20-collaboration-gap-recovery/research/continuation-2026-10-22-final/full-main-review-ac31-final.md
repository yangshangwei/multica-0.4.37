# Final independent AC31 review

Active task: `.trellis/tasks/10-20-collaboration-gap-recovery`.
Reviewer: `full_main_check`, dispatched `trellis-check`, 2026-10-22.

Decision: **recommend AC31 passed at its original P6 owner**. The two concrete
findings in `full-main-review-ac31.md/json` are closed by the current six-form
journey and the complete corrected paired run. The earlier review and all failed
attempts remain unchanged. This final review supplements the prior aggregate
analysis; it does not reclassify the97 criteria or authorize providers/rollout.
Only these `full-main-review-*` files were written by the reviewer.

## Findings (fixed)

- **Missing information/acceptance UI submits: closed.** Current Web and complete
  packaged Main each submit Request information, Return information and Submit
  for acceptance through the existing shared form controls. The native fixture's
  `workflow` command only selects/clicks those real controls and observes their
  original HTTP POST. Web uses the same product forms. No TestApiClient mutation
  stands in for any of the six actions. Public password authentication gives the
  same installed handler a fresh Web session for its final acceptance submit.
- **Final paired rerun: closed.** `installed-pair-reviewed-final.stdout` is2/2,
  expected2 /unexpected0 /skipped0 /flaky0, both retry0,54.084971s. Its exact
  command/result/current source hashes match. The old relay input is explicit;
  the Main flow uses committed post-navigation baselines and completes all six
  forms plus original controls/history/restart. The earlier line139 assertion
  remains a source-frame failure, not an OS exit139.

No product fix or test mutation was made by this reviewer. The implementer/root
owns the reviewed fixture changes and final executions.

## Six exact human actions in the final paired run

All operate on the same issue
`01a163fa-3fd3-7524-b438-08de243aa624`. Creator C is
`9106c2d6-ec84-4de7-99fe-1c96de245838`; installed handler H is
`b09a85eb-93af-4eee-91f6-8190ec023857`.

| Client / form | Current handler before → after | Selected nonterminal status | Original request key |
| --- | --- | --- | --- |
| Electron Request information | H → C | blocked | `6d11cdb1-7c13-4b19-9264-2c1cf83a77f0` |
| Web Return information | C → H | todo | `601500ca-03de-45c8-a369-182128feff7b` |
| Electron Submit for acceptance | H → C | in_review | `6c37b811-d565-4560-a2e1-4aec74aebd6b` |
| Web Request information | C → H | blocked | `3a600ece-16fc-4add-a2e7-dacb63635fe0` |
| Electron Return information | H → C | in_review | `b89c61bd-f33d-4a5f-b49c-541fde72c330` |
| Web Submit for acceptance | H → C | in_review | `8fbfc313-a2d5-4a47-8099-a443ead78d58` |

The reviewer compared each retained before/after issue snapshot: same UUID and
creator, exact prescribed status, original captured collaboration/lifecycle
revision and current request identity, full session and execution equal. Both
request actions create the expected requester/recipient relation; the Web and
native return actions consume exact request IDs
`bf960691-016e-46d0-a1ce-74b745f958de` and
`3ca769d1-6b44-4da1-945a-9f4bc6dbc9ea`. Returns and acceptance clear the active
request. All six keys are unique. The activity contains exactly two each of
`information_requested`, `information_returned` and `acceptance_submitted`.
Acceptance remains `in_review`, never completed/cancelled.

Native waits for a real issue GET initiated after the fresh main-frame
navigation, verifies its200, then reads current JSON through the authenticated
renderer and confirms the rendered Handler field. Web likewise verifies the
new renderer GET before its authenticated current read and actual handler
display. This removes stale/retired CDP body observations without changing any
UI write, status/version expectation or product code. Every write remains a
real form submission against the current immutable operation contract.

## Exact current verification

- `full-main-directed-third`:1/1,0 unexpected/skipped/flaky, retry0;
 40.161599s suite /38.818s test /42.284s command.
 Raw SHA256 `9a03c940a8b016327ec3cf07743729a62c1207787a8377f1adaa8ee4d674125a`;
 safe journey SHA256
 `a531209dfa97c722850e623bfd83c8519fbc89c459047a7566d906cfd9490a36`.
- `installed-pair-reviewed-final`:2/2,0 unexpected/skipped/flaky, retry0;
 54.084971s suite /56.780s command. The full Main case itself is41.713s.
 Raw SHA256 `97b96230adaac5ea0ed36a3f3138f2f7e57b3825b5d351219cfccda2b6f3e756`;
 safe full Main journey SHA256
 `5c75199de167a879bb9497ae4ef336ae91039d20fa3a1ec0a97afd26c40c82a0`.
 Both command/results report no source change during execution. The old CLI
 companion is separately owned/reviewed under AC32; its pass is not provider
 evidence or an extra AC31 definition.
- Current strict typecheck, base ESLint3 files/0 errors/0 warnings, syntax,
 discovery and whitespace checks all exit0 in
 `full-main-fixture-directed-static-results.json`;
 SHA256 `05f62eb3fa7fe98915b4d8d789806ba7bc1ec94bb0cf9dd4ebc319697db77a09`.
 Reviewer source hash comparison matches all three files in both final commands
 and the static record:
 spec `fd3a0889d01b2336b62014c1dfc4d84bac6c30c2d28ef5bde76efe471cc76b86`;
 MJS `f5ba85f0a7bb56c1facf61d5fac871e1351e03e13bb7ac307eafc3e1d408e319`;
 TS `1d677378fe0aa34405c1ccba0bd8073ec14b4073135f4bad33b4f91418f027dc`.
 Retained pnpm configuration warnings are not compiler/lint failures.

## Original whole-clause conclusion

The previously reviewed aggregate covers current dual-client creation/private
files, resource offers/consent, comments/preview/partial refusals, offline
notifications, controls/removal/close/reopen, source and canonical shared UI
contracts. The concrete three-form branch gap is now closed in actual Web and
installed full Main. Cross-client continuation additionally preserves one UUID,
actual current handler and role-dependent capabilities, command/body/author/
cancellation history, queues/holds, original unconfirmed A barrier and restart.
The final native client has395 observed account requests, all without ambient
workspace headers, actual Linux/client-version write headers and zero page errors.
Former handler retains participant AI control and loses transfer; authorized
responsibility changes never derive from a client switch.

The exact packaged application remains unchanged across directed and paired
runs and restarts:

- executable `379334fb6707615062d32fcc481a8a8d3140938531378ccba14f3fa169665043`;
- ASAR `35690546d3148df7c702ae39a5589e684eeb379893e34e79b2438ada578b3ea5`;
- bundled CLI `d4f9091acf534ec4fff9f3110d063ac58ead160b4badedf0607a5b989edfc732`.

The reviewer again recomputed1578 packaged application-source hashes against
the current tree: zero mismatch. Production Main/preload/file renderer,
`app.isPackaged:true`, real password login, native daemon/settings/CLI bootstrap,
default isolated `/root`/userData and no native IPC/auth substitutes remain
proven. Real native preferences keep daemon stopped and updater disabled; these
are persisted settings, not fake services.

## Findings (not fixed) and limits

No remaining blocking AC31 finding in the inspected current evidence.
Unsigned Linuxarm64 directory packaging proves an executable packaged app;
system installer/signature/public distribution and fresh macOS/Windows full
Main evidence remain outside this proof. Original AC31 does not separately
require those release properties or every OS variant. Other slice harness
scopes remain accurately labelled; a full Main pass does not retroactively
convert their native service stubs into full startup evidence.

The remote A Claim/Permit still has no launched provider, outcome or supplied
termination proof. Resume correctly refuses409 `stop_unconfirmed`; partial
whole Resume releases B/S while preserving A; final Stop remains paused and
restart keeps the same original ending run. Separate actual Linux runtime
proofs remain their own evidence. Commercial account/version/quota matrices,
AC43, production creation/execution/rollout and task/parent archive remain closed.

Observed ledger at this review is95 passed /2 pending, AC31 still pending,
`full_acceptance_complete:false`. Root owns the independently reviewed ledger
promotion/spec/commit. Neither original97 definitions nor existing task plans
were changed by this reviewer. The prior conditional review is superseded for
AC31 status by this final current proof, while its historical facts are retained.
