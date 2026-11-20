# Ordinary UI CAS E2E implementation — 2026-11-19

Disposition: **WORK R04 STATIC READY; current UI body NOT_RUN.** Strict
TypeScript, targeted ESLint with zero warnings, and discovery pass on WORK R04
after the bounded Assignee trigger repair and explicitly released old-assertion
contract sync. Main's original runtime RED R01 is
retained as NOT_ACCEPTED: both tops failed before any PUT capture, two subcases
failed and four remained unstarted. There is no actual missing-token RED proof.
This lane executed no test body, account setup, database audit, browser/native
process or API control. TC-16 and full S3 are not accepted here.

The companion [JSON record](ordinary-ui-cas-e2e-implementation-20261119.json)
contains exact commands, all thirteen static recorder results, initial/final source
manifests, executable/configuration hashes, original failures, source checks and
the complete NOT_RUN inventory.

## Source and ownership

The final owned source is
`/Volumes/artisan/.multica-acceptance/pay241-s3-ui-cas-20261119-u691amwe/source/client-ui-harness-work-r04`.
Main supplied the coherent BASE/WORK R02 after its read-only review found seven
stale and three missing companions in the initial mixed ROOT/OUT source. Its
original allocation, BASE/WORK R01, failed build and every R01 static result
remain retained. The exact three-file delta was copied only after the HOLD ACK,
as recorded in `intake/ui-harness-coherent-carry-r02.json` and
[allocation R02](ordinary-ui-cas-allocation-20261119-r02.json).

Main owns environment, runtime, integration, specs, task progress, acceptance and
Git. The separate product owner owns the eight product paths and ten regression
tests. This lane changed no product/coherence companion,
API environment/configuration, database or Git state, and spawned no agents.
Final manifest comparison finds **zero nonowned file or mode differences** from
coherent BASE; all ten supplied companions retain their recorded hashes.

| Owned harness file | Final SHA-256 |
| --- | --- |
| `e2e/collaboration-recovery-ordinary-ui.spec.ts` | `ce0a7178bb0bd815357c0dd92dbb3e56204658ff95172feb5196b7b9a5b3adde` |
| `e2e/fixtures/collaboration-recovery-ordinary-ui.ts` | `d8441e916aecc36a09725ed8e9f9108dbaceeef68b64c34f7a04df48197db977` |
| `e2e/fixtures/collaboration-recovery.ts` | `78786a384aabddab834131dea49ebdd6285dc072d621c28bec1c3049d057cacb` |

Through frozen WORK R03 the shared recovery fixture was byte-identical to BASE
after adding only `export ` to `openOrdinaryChild`: **export-only invariant TRUE
for R03**. WORK R04 additionally applies Main's exact released ordinary helper
contract sync, documented below. The original export and all explicit numeric
API CAS/hold/response assertions remain unchanged. Historical omission assertions
remain preserved in prior sources and accepted raw evidence. Initial
inspection found neither new owned E2E file, so no partial implementation was
erased. ROOT writes are only this report and its adjacent JSON.

## Authored proof

The spec persists the whole two-top/six-subcase inventory before setup. Each
subcase has its own evidence file; failed and unstarted entries survive a worker
replacement. It retains one worker, zero retries, the existing 15-second action
timeout and 360-second recovery-top timeout.

| Platform | Explicit subcase | Authored behavior | Actual status |
| --- | --- | --- | --- |
| Web | `priority-first` | Priority wins; assignee loses; explicit fresh clear | NOT_RUN |
| Web | `assignee-first` | Assignee clear wins; priority loses; explicit fresh High | NOT_RUN |
| Web | `versionlessAPI` | Separate direct PUT omits `expected_revision` | NOT_RUN |
| Desktop | `priority-first` | Same real UI order in matching native renderer | NOT_RUN |
| Desktop | `assignee-first` | Same opposite real UI order in matching native renderer | NOT_RUN |
| Desktop | `versionlessAPI` | Separate direct API compatibility control | NOT_RUN |

Each complete platform journey seeds one protected collaboration root and three
distinct ordinary children with low priority and a valid member assignee. Each
ordering reuses `withSessions` to separately authenticate two actual creator
sessions and the exported native/Web child navigation. `ChildTraffic` observes
actual child UUID/identifier GET responses and associates PUT responses with
their original Request objects. Both displayed baseline fields and both numeric
R observations are retained.

Two existing `HeldRequest` instances are installed before either real picker
choice. Low→High and member→Unassigned each produce a genuine PUT; both are
captured before forwarding and must contain the original numeric R. Baseline RED
will fail this required request assertion if product still omits the token.
There is no route/body/token injection or synthetic PUT used as UI proof.

The named winner must receive natural 200 and R+1. The loser must receive exact
409 `revision_conflict`, child resource UUID/type, expected R and actual R+1,
with the losing field preserved. Its existing exact localized conflict toast
must be visible, followed by an actual authoritative child GET at R+1 and the
winning displayed state. Only after that observation is its picker explicitly
reopened. A new genuine PUT must carry R+1, receive 200 and produce R+2 with high
priority and no assignee. Both UI field projections converge; the actual write
inventory allows only two original PUTs and one explicit fresh PUT per order.

Natural forwarding remains the shared helper's unchanged `route.continue()`
plus exact `response.request() === route.request()`. The new fixture never asks
for its lose-response mode, calls `route.fetch`/`fulfill`, or submits a
`page.evaluate`/synthetic field request. Its single direct Fetch PUT exists only
in the separately named `versionlessAPI` compatibility function and contributes
zero UI requests. It must succeed with an omitted token and advance its distinct
child once while retaining the member assignee.

All original/fresh holds close in `finally`; both picker promises and traffic
streams are collected on failure before session teardown. Cleanup failures
reject otherwise successful cases **after** `finally`, while wrapped primary
errors retain precedence, including a thrown undefined value. The existing
native zero-daemon-start and session cleanup guards are unchanged. Ten bounded
READ ONLY SQL inventories retain parent issue, collaboration, session,
events/operations/handoffs and zero command/group/run/dispatch rows. The parent
projection and complete protected rows must equal their prior snapshot after
each subcase. These audit statements were source-reviewed and authored only;
they have not contacted a database in this lane.

## Static verification and retained failures

Every executable check used `helpers/record-run.py`, a unique label, explicit
absolute Node/compiler/linter/Playwright paths and a closed static environment.
R01/R02 environments have no database URL, JWT or nonempty credential values;
both body gates are `0`, HOME is private, and no ambient environment is inherited.
Source `.env` and `.env.worktree` are absent. The unchanged R02 environment hash
is `d4e2f01d9a675773755d914874165bfca6432a6a41236448e058680cdabbf2c8`.

| Recorder label | Exit | Observed result |
| --- | --- | --- |
| `e2e-cas-ts-r01` | 2 | Three owned TS2349 Fetch `status()` errors at 325/327/332; preserved |
| `e2e-cas-lint-r01` | 1 | Four owned `no-unsafe-finally` errors at 204/309/356/357; preserved |
| `e2e-cas-discovery-r01` | 0 | Exactly two tests in one file; no body |
| `e2e-cas-ts-r02` | 2 | TS4058 inherited declaration-artifact error in old/new seed exports; preserved |
| `e2e-cas-lint-r02` | 0 | All three explicit harness targets, zero errors/warnings |
| `e2e-cas-discovery-r02` | 0 | Exactly the Web and Desktop tops, two tests in one file |
| `e2e-cas-ts-r02b` | 0 | Strict no-emit check of all three owned targets and imported graph |
| `e2e-cas-ts-r03` | 0 | Strict check after source-backed Assignee trigger/diagnostic repair |
| `e2e-cas-lint-r03` | 0 | All three explicit R03 harness targets, zero warnings |
| `e2e-cas-discovery-r03` | 0 | Exactly the same two tops; current body NOT_RUN |
| `e2e-cas-ts-r04` | 0 | Strict check after only the released old helper contract sync |
| `e2e-cas-lint-r04` | 0 | All three explicit R04 targets, zero warnings |
| `e2e-cas-discovery-r04` | 0 | Exactly the same two tops; current body NOT_RUN |

All thirteen commands report **no source drift, no environment/config drift and no
tool drift**. R02 fixes only the authorized Fetch property accesses and cleanup
control flow in the owned fixture. The subsequent TS4058 came from inheriting
library declaration generation for a private E2E `TestWorkspace` return type.
Fresh `ordinary-ui-cas-tsconfig-r02b.json` disables only declaration artifacts;
strict/no-unused/no-unchecked checks remain inherited. The original failing
R02 config and result are preserved. No unrelated helper/type workaround was
introduced.

Static configurations were added only under campaign `configs/`:
`ordinary-ui-cas-{tsconfig,eslint,playwright}-r01`, corresponding R02 configs, and
the final `ordinary-ui-cas-tsconfig-r02b.json`. The successful TypeScript config
hash is `42c3d88dd567a7a3457cab14854d1b6a31c42ac521924ed47960573f303b81c6`;
ESLint R02 is `10850d5e36b8331f811493598db1505e8d9212de1f18017e66385c6537f33659`;
Playwright R02 is `111bd54f8f82935a2ba74b1e0f0892b5365b0d45e2f5d8e3b8d91752e0f73bba`.
All exact arguments, paths, hashes and raw outputs are in the JSON record.

Pinned tools are Node 22.22.0, TypeScript 5.9.3, ESLint 9.39.4 and Playwright
1.58.2. Targeted lint explicitly includes the E2E files that the normal package
lint ignores and uses ESLint/TypeScript recommended rules with the repository's
compiler-owned unused-variable convention. Broader product checks and
independent review belong to Main; no full `pnpm lint`/package gate is claimed.

## Handoff and limits

Main can freeze the exact three-file R04 delta into its coherent production
baseline and execute a whole fresh RED round, then a fresh GREEN round with the
separately owned product candidate. Complete GREEN expects, per platform, four
separately authenticated UI sessions, four genuine original PUTs, two genuine
fresh PUTs and one separate versionless API PUT. Preserve the entire RED and any
failed GREEN round instead of combining passing subcases.

Actual picker locators, automatic GET refresh, native transport/current-client
convergence and SQL/no-execution assertions remain **NOT_RUN**. Main still owns
fresh database/config/build binding, exact child and protected historical
reconciliation, gate/PID/native/connection closure, the separately unreleased
old omission assertion, and independent acceptance. Static success does not
close TC-16, R09 or S3.

## Runtime RED R01 diagnosis and WORK R03 repair

The R02 report pair was preserved under campaign `intake/` before updating this
report: `ordinary-ui-cas-e2e-implementation-20261119-r02.md/json`, original hashes
`f42ccd1c9ca86551ac39356c0b5763aa15f780dd901d512927449e2a31606d28` and
`f08237a4b59fd1d1185fcd3f9bbd775ca472385a0704068a080fd07239c0232c`.
Main allocated fresh WORK R03 from the frozen exact R02 files, recorded in
`intake/ui-harness-runtime-r01-repair-allocation.json`.

Read-only inspection of `verification/ui-red-r01/stdout.log` and both
`*-priority-first.json` attachments confirms the same failure: the exact
Assignee owning-button accessible name `Offline creator P` was not found. Both
actual UI clients on each platform had received the intended child GET at
numeric R1 with valid Web cookie/CSRF or native bearer authentication. Their
retained traffic contains **zero PUTs** and no hold input; the later expected
missing-token assertion was never reached. This is a fixture-locator failure,
not product CAS evidence. The original round keeps both failed tops and the
2 FAILED / 4 NOT_RUN subcase denominator.

Source proof is the actual `AssigneePicker` default trigger: `ActorAvatar` plus
a sibling span containing `getActorName(...)`. A member with no avatar renders
its initials through the shared UI avatar, and the resolver computes `OC` for
`Offline creator P`. The nested profile/hover shells are spans inside the
PropertyPicker trigger; the picker button remains the actual control. The
retained Desktop screenshot visibly shows the `OC` avatar and member-name text
in the Assignee row. The failed R01 does not contain a complete accessibility
snapshot, so the exact combined accessible name is source-derived rather than
claimed as an observed DOM fact. The existing ordinary recovery option helper
already matches an exact member-name descendant to avoid avatar/name metadata.

WORK R03 changes only the new fixture's locator and diagnostics. It finds the
actual Assignee property row, matches its owning button by the exact rendered
member/unassigned label descendant, and requires exactly one matching button.
Priority retains its exact accessible-name match. Before field and picker
selector assertions, it records bounded actual row text/button counts/button
text/aria attributes/rendered spans and the row's `ariaSnapshot`. Those are
read-only DOM diagnostics, not cache/route/token changes. Request fields, exact
tokens/conflicts, protected snapshots, counts, holds/transport, fresh-intent
logic and all deadlines remain unchanged.

Fresh `e2e-cas-{ts,lint,discovery}-r03` all pass with no drift under the unchanged
closed `ui-harness-static-r03.json`, SHA
`2c10b646c8af4109ec80b627cd81d8172b7afaae72925d9ded7dd4432afa7c77`.
Explicit R03 static configs are added under campaign `configs/` while preserving
all earlier configs. Current R03 source is frozen for Main's new round; no body
was executed here.

Main's separate admission check had also failed before its original runtime
test, but newline shell sequencing still started that test. The retained
`audit/ui-red-r01-admission-sequencing-failure.json` records this condition and
the startup `sys_cron_executions` table drift. This lane made no admission,
database or runtime correction. Main's `ui-red-r01-owned-closure.json` records
all three owned services stopped, original gate/epoch2 unchanged, 856 migrations,
663 indexes and zero connections. Those are attributed Main artifacts, retained
with the full NOT_ACCEPTED round. New admission and test steps remain separate
tool calls under Main's control.

The old shared fixture's omission assertion remains unchanged in frozen R03.
Main separately authorized a current-contract synchronization in
`intake/ui-previous-omission-assertion-release-r01.json`, then explicitly held
R03 to allocate WORK R04. That released scope has not been written yet. The old
whole four-top field body and historical R06 evidence are not reexecuted or
retroactively accepted by this lane.

## WORK R04 exact old-assertion contract synchronization

Main allocated an exact copy of frozen R03 as WORK R04 and reissued the release
in `intake/ui-previous-omission-assertion-release-r02.json`. R03 source, configs,
static raw and reports remain retained. The R03 report pair was copied under
campaign `intake/ordinary-ui-cas-e2e-implementation-20261119-r03.md/json`, with
hashes `f27df97735a3732fd702f34defd28606dfd5a6bda3a1f0dcdf0542cb4fa4da4e` and
`4dc9387cd6408bcdca3ddbb480fce393132036f952bc62b105ae73a3cef2b224`.

Only the released ordinary helper region in `collaboration-recovery.ts` changes:

- `ordinaryUIUpdate` records `ORDINARY_UI_OBSERVED_NUMERIC_REVISION`, requires
  `input.body.expected_revision === child.revision`, then requires parsed result
  ID equal to the supplied child ID and revision equal to child revision +1.
- Its returned scope says the real UI retained its numeric observation and the
  separate new suite owns genuine two-client contention.
- `exerciseRecoveryOrdinary` names its UI contract accurately, renames the two
  locals to `observedPriority`/`observedAssignee`, and uses
  `UI_observed_numeric_revision` / `UI_observed_numeric_revision_requests` for
  evidence/result keys. The two genuine UI request counter stays **2**.

Prefix/suffix comparison proves **only this released region changed relative
to R03**. The existing export remains. Explicit API requests/holds/response
association/conflicts and all original request counters are unchanged. The new
spec and `collaboration-recovery-ordinary-ui.ts`, including the real property
locator/ARIA/count diagnostics and separate versionless API control, are
byte-identical to frozen R03. No product or unrelated fixture changed.

Fresh `e2e-cas-{ts,lint,discovery}-r04` pass under closed
`ui-harness-static-r04.json`: strict TSC, zero lint warnings and exactly two
discovered tops, all without source/config/tool drift. R04 explicit static
configs were added without replacing earlier configs. Exact hashes, the full
released region diff, source/old-report preservation and commands are recorded
in the JSON.

WORK R04 is frozen for Main's fresh BASE RED2 source/config/database. **The old
whole four-top field body was not reexecuted here; old accepted R06 raw/reports
were not modified.** This synchronization is a current test-contract update,
not new runtime/UI acceptance. Current R04 bodies remain NOT_RUN, and the prior
Main RED R01 retains its no-PUT failure and separate admission problem.
