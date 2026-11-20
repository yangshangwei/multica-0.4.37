# Ordinary UI CAS resume readiness — 2026-11-19

**Disposition: READY FOR A FRESH ADMISSION REVIEW; RED2 HAS NOT RUN.** The
ordinary UI candidate is source- and tool-consistent, and the two negative
runner controls correctly refused to start a Playwright process. The current
campaign still lacks an accepted `ui-red-r02-admission.json`, so no RED2
business body may be classified as executed. The generated production Web and
Desktop builds are now both successful, but the Desktop output was produced
after the original GREEN source manifest and needs a new generated-output
binding in the admission record.

## Scope and sources checked

This review is limited to the current ordinary UI CAS campaign
`/Volumes/artisan/.multica-acceptance/pay241-s3-ui-cas-20261119-u691amwe` and
the reviewed plan, implementation report, code check and E2E fixture report.
No application service, database fixture, browser, native renderer, provider,
or test body was started by this review. Existing output and failure records
were read without rewriting them.

The current candidate remains bounded to:

- eight ordinary Core/Views product paths and ten owning-layer test paths;
- two complete tops (Web and matching Desktop), each with priority-first,
  assignee-first and a separate versionless API control;
- genuine picker PUT capture with the observed numeric revision, one natural
  `200`, one natural `409 revision_conflict`, and one explicit fresh `R+1`
  continuation; and
- the existing protected parent/history/run audit and zero automatic resend
  requirement.

The code reviewer accepted Core 64 plus Views 216 named cases, whole Core and
Views type checks, and scoped lint. The E2E harness report accepted strict type,
lint and discovery checks only. Those reports explicitly do not accept live
Web/Desktop UI CAS, TC-16, R09 or full S3.

## Evidence that is ready

The following bindings were re-read and their recorded hashes were recomputed:

| Binding | Result |
| --- | --- |
| `client-ui-harness-run-r02` manifest | 3,646 rows; exact current source match |
| `ui-code-check-candidate-source-r01` manifest | 3,645 rows; exact current source match |
| `ui-green-prod-source-r01` manifest | 3,645 recorded input rows; all recorded rows still match |
| browser/native resources manifest | 1,096 rows; exact current match |
| RED runtime permit | source manifest, artifact manifest, browser manifest, config, guard and all pinned tools match their recorded hashes |
| accepted negative controls | `ui-blocked-r01` and `ui-blocked-r02` both exited 1, with raw stdout/stderr hashes matching and no test `command.json` created |
| Core/Views functional GREEN | 64 and 216 assertions, respectively; all passed exactly once with no duplicate names |
| E2E static R04 | TypeScript, lint and discovery exited 0 with no source/config/tool drift |

The eight product paths and ten test paths have identical bytes in the coherent
implementation, independent checker, and GREEN product sources. The RED
product candidate still matches the frozen BASE on those paths, so it is a
valid baseline for a new missing-revision RED round. The current E2E source
matches the frozen R04 harness on all three owned files.

The campaign's `run-ui-permitted-r01.py` guard is correctly fail-closed for the
two observed pre-admission cases. It requires an accepted admission record,
matches the permit and config hashes, verifies each pinned tool and owned
service PID/PGID, confirms the task database identity, and then invokes the
closed-environment recorder. The recorder snapshots its source before and
after, records exact argv/config/tool hashes, inherits no ambient environment,
and records a complete source-drift result.

## Current RED2 state

The RED2 allocation exists as
`ui_red_r02-allocation.json` with database
`multica_pay241_s3ui_20261119_u691amwe_ui_red_r02`, the migrated template,
password-mode configuration, ordinary UI acceptance enabled, runtime
acceptance disabled, and no model credentials. The retained database before
snapshot and allocator receipt record the expected gate epoch and
schema/table/ledger identity. This review did not connect to that database;
its current state must be checked by Main before admission.

The matching RED production source is `client-ui-red-prod-r03`; its 1,573
record artifact manifest has zero byte-hash mismatches. Two generated files in
that historical manifest omit a `mode` field (`.next/BUILD_ID` and
`.next/required-server-files.json`), so their mode is not an asserted drift;
the bytes match. Preserve that limitation rather than presenting the manifest
as a complete mode proof.

There is **no** `audit/ui-red-r02-admission.json` and **no**
`verification/ui-red-r02/result.json`. Therefore the six RED2 subcases are all
`NOT_RUN`; no genuine request, omitted-token assertion, database mutation, or
failure can be promoted from this allocation. The rejected `ui-blocked-r02`
record is only a guard regression and cannot serve as RED2 evidence.

Before RED2, Main must create a new accepted admission record after checking
the actual API/Web/renderer readiness records, the fresh database identity,
gate state, source and production artifact manifests, and the complete
configuration/tool binding. Its RED product output already has a retained
manifest; the separate GREEN admission needs the generated Desktop inventory
described below. After independently observing the accepted RED2 admission,
invoke the permitted runner in a separate tool call:

```bash
python3 /Volumes/artisan/.multica-acceptance/pay241-s3-ui-cas-20261119-u691amwe/helpers/run-ui-permitted-r01.py ui-red-r02
```

The permit already binds `workers=1`, `retries=0`, `max-failures=0`, the exact
spec/config/source and JSON reporter. This is a conditional next command, not
a command executed by this review.

## GREEN build state and remaining binding

Both production builds now exit 0 with no input-source/config/tool drift:

- `verification/build-ui-green-web-r01/result.json` — Web production build,
  exit 0.
- `verification/build-ui-green-desktop-r01/result.json` — Desktop
  `electron-vite build`, exit 0.

The Desktop result records 3,645 input files before the build and 4,140 after;
all 495 additions are under `apps/desktop/out/`, with zero changes or removals
to the pre-existing input files. This is expected generated output, not a
product-source edit. It must nevertheless be registered as a generated
artifact manifest (including hashes and modes) and tied to the exact GREEN
source/config/tool build before a GREEN admission. The earlier
`ui-green-prod-source-r01.json` cannot, by itself, prove the post-build
Desktop output because it predates those additions.

The GREEN product input remains the same eight paths and ten tests as the
accepted code candidate. The build log has only the known CSS `::highlight`
warnings; build exit status is 0 and no new source warning was introduced by
the CAS increment.

## Runner and admission recommendations

1. Keep the two rejected controls and their raw logs immutable. Create a fresh
   `ui-red-r02-admission.json` only after all owned service readiness,
   database, gate, source, generated-artifact and native-resource checks pass.
2. Add a generated-output manifest for GREEN Desktop `apps/desktop/out/` and
   bind its hash to the successful Desktop build command/config. Do not amend
   the historical 3,645-row input manifest or splice its rows with a later
   runtime result.
3. The current permitted runner checks the permit/config/tools, service
   PID/PGID and database identity, but it does not independently revalidate
   the source, production artifact, browser-resource or generated-output
   manifests named in the permit. The admission step must perform those
   checks, or a separately versioned runner may add them before a permit is
   accepted. Do not silently broaden the existing runner in-place for this
   round; bind any changed guard hash in a new permit.
4. Keep successful admission and test invocation sequential, with the accepted
   admission result observed before a separate runner tool call. The retained
   R01 admission failure started a test because
   newline-separated shell commands did not stop after the prerequisite
   exited 1; the R02 negative controls prove the Python guard now refuses the
   same mistake. Never join a failed prerequisite and the test with unchecked
   newline shell sequencing.
5. After RED2, preserve the entire two-top/six-subcase inventory even if the
   intended omitted-token assertion fails early. Release every held request in
   `finally`, retain the original error and all unstarted entries, and perform
   the owned shutdown/connection/gate audit before deciding whether a GREEN
   round is warranted.
6. Only after the intended genuine-PUT omission RED2 baseline is retained
   should Main run the GREEN
   candidate with the same denominator and a new source/config/database/run
   label. A passing static check, direct versionless API control, or generated
   build alone cannot promote TC-16.

## Findings fixed in this review

The first generic manifest comparison incorrectly treated a missing `mode`
key as an actual mode mismatch for two RED artifact rows. A subsequent parser
assumed every row had that key and raised `KeyError` before its read-only
checks finished. The corrected read checks only recorded fields, confirms all
1,573 hashes and all 1,571 recorded modes match, and identifies both unrecorded
modes explicitly. Main received the correction immediately. No artifact or
manifest was changed and no mode-drift finding remains.

The initial snapshot had no GREEN Desktop build result; Main completed that
build concurrently. This report incorporates its later retained result and
the exact 495 generated additions. It does not infer a defect or rerun from
that expected concurrent output.

## Findings not fixed

- Positive RED2 admission and actual UI bodies remain Main-owned execution
  work. This reviewer is limited to filesystem reads and this report.
- Generated GREEN Desktop artifact binding and any versioned runner
  hardening remain Main-owned configuration decisions. No product repair is
  justified by these findings.
- The reused scoped Views lint retains its one documented BASE
  `react-hooks/exhaustive-deps` warning. No unrelated callback change or
  warning suppression is proposed.

## Verification

- Source/manifest/config/tool/hash reconciliation: **PASS within the stated
  campaign scope**.
- Negative admission controls: **PASS**; both refused before a test process.
- Core/Views type check, lint and functional artifact reconciliation: **PASS**
  per the independent code report; no body replayed here.
- Web/Desktop production build: **PASS**, both exit 0; Desktop generated
  outputs require a new manifest binding.
- RED2 admission: **NOT RUN / not accepted**.
- RED2 UI body, GREEN UI body, TC-16, R09 and full S3: **NOT ACCEPTED**.

