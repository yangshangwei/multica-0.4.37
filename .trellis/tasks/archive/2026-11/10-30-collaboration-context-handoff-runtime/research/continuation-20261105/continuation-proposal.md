# Proposed continuation after the H3a timeout

Status: reviewable proposal only. The original registration remains stopped;
no model call or restart is authorized by this document.

## Evidence and unchanged limits

E0a, H1, H2 and H2b passed their original behavior criteria; H3a trial1 passed.
H3a trial2 consumed one real launch, reached the correct decision through a tool,
and then hit the fixed five-minute Provider limit without a final result.
Its original incomplete result,143 exit, all logs and12/25 total consumption
remain unchanged. No confirmed upstream root cause or product defect was found.
The remaining13 starts are unspent; unspent budget alone does not override the
original first-incomplete stopping instruction.

## Concrete proposed scope

Run only the originally unattempted slots, in their existing order:

| Scenario / slots | Maximum new paid starts |
| --- | ---: |
| H3a trial3 | 1 |
| H3b trials1–3 | 3 |
| H5 trials1–3 | 3 |
| H6 trials1–3, two launches each | 6 |
| Total | 13 |

Do not repeat H3a trial2 or any completed slot; do not change the model, prompts,
scoring, original five-minute Provider timeout, source or seventeen framework
files. No extra health call. Fake seed starts remain separately counted.

A new explicit continuation decision must acknowledge that the preceding timeout
cause is not proved fixed. It creates one narrow exception to the historic stop
for the remaining slots; it never turns the historic incomplete sample into pass.
The first new incomplete/runtime/environment/harness failure or H5 canary result
stops the continuation. H6 still needs its registered semantic review.

## Implementation before any paid execution

1. Record the user's exact continuation decision and reviewed predecessor hashes
   (all original public results, invocation ledger and consumed12) in a separate
   continuation registration. Keep the original stopped launcher and records
   immutable.
2. Implement a separate task-local continuation entry that reuses the same frozen
   scenario implementation. It must validate exact history hashes, cumulative
   cap25, remaining13, next-unattempted labels and one exclusive window. It can
   waive only the exact hash-bound `RK1-H3a-t2` incomplete record and its
   causally linked original H3a invocation exit1/stop record. Both hashes must
   match the reviewed predecessor pair; any other old/new incomplete result or
   failed invocation still refuses. Never modify or hide the predecessor to fool the old
   launcher. Explicitly preserve the old failure in all summary reporting.
3. Independently verify it with credential-free fake processes: wrong history,
   omitted debit, duplicate/replaced slot, exhausted cap, unrelated incomplete,
   new failure/canary, concurrent entry and changed source/config all refuse.
   Test actual result persistence and physical-start accounting, not merely flags.
4. Bind fresh full operator manifests, current exact CLI/Provider/tool bytes and
   new owned databases, then obtain each coordinated window. Retain the same
   preflight and post-run review procedure used for H1/H2/H2b.
5. Execute one scenario invocation at a time; review raw result and operator delta,
   account for every physical start and release resources before any successor.

## Decision still required

The user may authorize this bounded continuation, or retain the original stop and
record partial acceptance. The latter leaves the task in_progress and makes no
claim about H3a trial3, H3b, H5 or H6. No choice increases the25-start allowance.
