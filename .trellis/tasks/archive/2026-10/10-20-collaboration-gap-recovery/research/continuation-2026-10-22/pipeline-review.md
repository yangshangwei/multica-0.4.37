# Pipeline receipt review — 2026-10-22

The retained service union is **1153 distinct passing top-level tests**, with no
overlap, missing identity, or failure in the four final artifacts. The current
ledger remains **79 passed / 18 not verified**, with all 97 protected definitions
unchanged. This is a read-only receipt/provenance audit, not a new monolithic
`make check` or a new execution of the old suite.

Exact audit data, command outputs, PASS event lines, remaining skip reasons and
current listener/source identities are in [pipeline-audit.json](pipeline-audit.json).
The reproducible [audit script](pipeline-audit.py) writes only that manifest and
runs no tests, builds, clients, provider executables, or environment mutations.

## Findings (fixed)

No product or existing evidence file was changed. The new audit filters the
`ok <package> 0.731s` footer in `go-service-identities.txt`; that line is not a test
identity. The 1153 real `Test*` entries match the disjoint 109/354/690 selectors.

## Findings (not fixed)

1. **Package receipt coverage is 81 of the current 82 packages.** Read-only
   `go list ./...` returns 82; retained package PASS/no-test outcomes cover 81.
   `internal/daemonws` has no package execution receipt in these artifacts. Its
   eight tracked files already exist byte-identically in both product snapshots;
   this is an evidence omission, not a newly introduced product package. The
   coordinator has now run a guarded narrow race check:22 top-level/30 total
   PASS, zero fail/skip in `daemonws-race.jsonl`; original omission is retained.
2. **Two exact command receipts are missing.** Partition 1 and
   `go-other-packages.jsonl` contain complete real PASS/package events and match
   recorded catalog SHA-256, but no neighboring structured command manifest is
   retained. Their original race flags/database attribution comes from the
   report, not a independently replayable command receipt. The coordinator's
   fresh original handoff selector supplies current G1 command provenance.
3. **Redis timeout prose exceeds the command facts.** The three main partitions
   use `-timeout=30m`; the 18-service supplement actually uses `10m`, and the
   73-other supplement uses `15m`. Both supplements exit 0 with no failure/skip.
   Preserve their actual budgets when integrating the evidence catalog rather
   than restating all executions as 30-minute runs.
4. **Conditional/platform boundaries remain.** Nine observed packages have no
   tests and report package `skip`. After the 18+73 Redis and six actual Linux
   route supplements, 15 original test/subtest skip identities still lack PASS
   in this selected union. Their actual reasons include pg_bigm, Windows/root,
   a remaining Linux daemon-poller case, live GitHub, performance/export opt-ins,
   and documented old fixture limitations. They are listed verbatim in the JSON;
   none is promoted to passing evidence or treated as a product failure.
5. **Current compilation failures belong to the new owner/switch tests.** The
   coordinator's fresh handoff first attempt is retained in this directory.
   Ownership of the safe fixture fixes and any authority repair remains with
   its assigned implementation lane. Historical full-suite PASS does not imply
   the newly added tests compile or satisfy their clauses.

## Verified receipts

| Final raw artifact | Top-level PASS | All test PASS | Test skips |
| --- | ---: | ---: | ---: |
| `go-service-partition-1.jsonl` | 109 | 397 | 0 |
| `go-service-partition-2.jsonl` | 354 | 913 | 0 |
| `go-service-partition-3.jsonl` | 672 | 1335 | 18 |
| `go-service-redis.jsonl` | 18 | 18 | 0 |
| `go-other-packages.jsonl` | 6601 | 12333 | 80 |
| `go-remaining-packages.jsonl` | 1873 | 3593 | 14 |
| `go-other-redis.jsonl` | 73 | 73 | 0 |
| `linux-router.jsonl` | 6 | 6 | 0 |

All final artifacts have zero failure events, complete package outcomes and
matching recorded SHA-256. Historical first failures are excluded from the PASS
union; nested and repeated identities are never added to its 1153 count. Redis
uses the task-only loopback instance on 57244 and is separate from API Redis.
The actual Linux route supplement uses only copied test-built executables,
with no host/home mount or authenticated provider account. It is not race
instrumented; the portable suites are attributed to their recorded guard/race
commands.

The old monolithic Go service **ten-minute timeout is a package time-budget
boundary**. Its named `sql_coalesced` test subsequently passed alone and the
disjoint final service union completed. Do not call that timeout a product
failure or claim a new monolithic pipeline ran through every Playwright case.

## Current application and listener correspondence

The initial read-only observation preceded the owner-authority repair; the
reproducible audit completed after the coordinator synchronized that exact
file to both snapshots. Across7013 tracked/untracked files, both current
product snapshots matched application bytes at that later point. This does
not imply that the superseded running API binary already had the repair. The remaining snapshot
differences are **22 legacy / 17 admin Go test-only files**, listed with exact
SHA-256 in the JSON. No frontend source byte differs. The nullable history helper
and its extracted caller already match both snapshots. The later committed
plugin dispatcher gate was already present in the baseline snapshot bytes;
that commit alone does not create source drift.

| Task environment | API listener | Production Web listener/build | Renderer listener |
| --- | --- | --- | --- |
| Legacy | 18985 / 46097 | 13905 / 86200 / `SrpeaQzGK_I-BlKn9Vy0e` | 6079 / 44952 |
| Password/admin | 18996 / 68800 | 13916 / 76944 / `tP4r55FoDsuyXfi1EylGw` | 6090 / 36145 |

All six HTTP endpoints returned200. The table records the initial historical
observation. During the later audit, admin health already reported22414 while
the audit still loaded the superseded68800 receipt; the raw mismatch is retained.
`content-admin-preflight.json` establishes current22414 source/configuration,
and `environment-api-source-refresh.json` retains both new API launch receipts
(admin22414 and legacy23252). Process parents/groups/cwd identify their snapshots. Web processes are `next-server (v16.2.6)` with recorded build
IDs. Both renderer responses equal their built index SHA-256
`337c08a228217fb574b77bb5fb6942bcbc40f2f1ce42587ceec5264204f3b35c`.
The admin renderer's later restart PID is established by AC33 preflight, not
the superseded first renderer restart receipt.

Only whitelisted public configuration was recorded: legacy/password mode,
admin availability, managed fixture availability, local endpoints and flags.
Password/admin remains true only in its task environment, with managed support
enabled for the prior synthetic fixture. Private config contents were not
printed. No rollout or production creation/execution gate was changed.

This source comparison is point-in-time evidence. The coordinator refreshed
only authority.go, restarted the owned APIs and recorded the current preflight
before the new clients. The separate observations must never be collapsed
into one historical listener identity.

## Current versus historical client evidence

The finalized AC90/91/97 mention test sources still match their recorded hashes.
Real final raw Web **3/3** and actual Electron **3/3** have only `passed`, retry 0,
zero skip/unexpected/flaky. They cover H+A+B+S, A refusal/B queue/H one notice,
original-key deduplication, fresh offline account login, anchors/read/archive
and comment/body edit deltas. Login is API authentication plus actual client UI,
not proof of an interactive login form. Synthetic queued targets do not imply
provider execution.

AC95 finalized control source also remains current: Web **2/2**, Electron
**2/2**, zero skip/unexpected/flaky/retry. Genuine Claim/Permit withholding proves
the UI pending barrier; actual Linux route tests separately own real proof
arrival/fresh Resume. The initial `control-admin-review.md` control hashes are
historical; its three unchanged S5 source hashes remain applicable.

AC33's post-extraction combined Web/Electron run5 remains a real **1/1 PASS** in
20.150s, with two actual synthetic Linux namespaces, immutable batch/cancellation
identity and real daemon proof. Both current snapshots contain the successful
history extraction. Run4 is retained earlier evidence; use run5 for the extracted
source rather than inferring freshness from the old build commit alone.

Dedicated password/admin S5 final public-login/account-shell journeys remain
Web **1/1** and Electron **1/1**, zero skip/unexpected/flaky/retry. They combine
with existing canonical current service authority/candidate/atomicity tests;
they do not prove provider operation, full installed Main, installer/updater or
an old installed client. No native client was launched by this reviewer.

## Ledger and narrow next checks

`task.py validate` passes 15 implement / 16 check context entries. All **189**
catalog artifact references have current matching bytes. Protected definition
SHA remains `e29ead58ba9e2c45a4099a52611c55e12dcae3b735136e058aafc87674cf217e`.
All 97 original definitions and primary owners remain unchanged; no automatic
promotion was made. Pending exact rows remain AC09/20/21/22/31/32/34/40/43/49/57/
67/75/83/84/85/89/96.

Recommended fresh scopes within the existing plan:

- Finish the coordinator's original handoff selector against migration737
  `service_s5`, after the new-test compilation fixes, using its exact guarded
  `-race -p 1 -parallel 1 -count=1 -timeout=30m` receipt.
- Add narrow `./internal/daemonws` race evidence to close the package omission.
- Run the newly added owner-departure/public-switch/resource-owner tests with
  their precise identities and current product repair; do not rerun the old
  1153 suite just to increase a count.
- Keep the source-role current22/95 and isolated3/44, mention-admission8/23,
  outbound16/65 and isolated5/12 receipts attached to their exact later commits.
  Those additions are not identities in the old 1153 inventory.
- After the API repair preflight, run the existing lifecycle Web/Electron
  reconnect/pause/removal/proof slice. Browser network emulation alone cannot
  prove Electron Main IPC network failure.

## Verification and ownership

Command run here: `python3 .../continuation-2026-10-22/pipeline-audit.py`, exit0.
It records the actual read-only `go list`, task validation, JSONL/selector/hash,
HTTP and process identity commands/results. No service rerun, lint/TypeScript
suite, native launch, environment/build/gate mutation or commit ran in this
lane. Prior lint/typecheck results remain attributed to their original receipts;
current broad compilation is not claimed green while new fixture work is active.

Changed files: `pipeline-audit.py`, `pipeline-audit.json`, `pipeline-review.md`.
Simplification: evidence verification reuses raw receipts without redundant
suite execution; no product abstraction/dependency/interface changed. Remaining
risks are the concrete gaps above and the original provider/OS/install/rollout
boundaries. Child and parent stay `in_progress`.

## Fresh coordinator follow-up

The original handoff selector now passes112 top-level/406 total with
`-race -p 1 -parallel 1 -count=1 -timeout=30m`,211.522s, zero fail/skip.
Its exact receipt is `handoff-service-race-final-command.json`; failed initial
compilation remains separately retained. `daemonws-race-command.json` closes
the retained82nd-package omission with22/30 current PASS. Neither command
constitutes a new monolithic full pipeline or replaces all conditional skips.
