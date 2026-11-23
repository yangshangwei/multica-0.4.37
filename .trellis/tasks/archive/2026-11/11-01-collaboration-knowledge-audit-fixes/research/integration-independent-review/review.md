# Combined knowledge/runtime independent review

Verdict: **pass-with-notes**, no blocking cross-lane product defect found. Reviewed `45efa093c1866c6ec988966769bd8f761daf7b50` on `codex/collaboration-knowledge-fixes-20261101`. Reviewer `/root/knowledge_web_review` is independent of the implementers and root merge operator. Only this review directory was written; no product, database, migration, Provider, installed Agent CLI or dependency operation ran.

## Merge preservation

The reviewed claim, launch, write/IW-1, web and runtime tips are all ancestors. A per-file blob comparison shows reviewed product bytes retained verbatim except two intentional overlaps: `knowledge_launch_test.go` contains both argument-selection and waited-termination regressions, and both locale files combine the knowledge changes with runtime `memoryLatest`. The admin fixed-clock test remains byte-identical to9c056ad68. The merged knowledge spec/operator guide preserves transient claim requeue, typed finalization settlement, complete history, original resume_unavailable replay, whole-transaction retry budget and lifecycle boundaries. No product implementation was silently replaced during spec reconciliation.

Exact source trees, the whole changed-product diff hash and file hashes are in `source-binding.json`, `full-product-binding.json` and `merge-provenance.json`.

## Interaction checks

- **Claim history → rebind → authorization:** Complete keyset/namespace history and the transient-versus-typed-refusal split coexist with first-disclosure fresh-generation rebinding. The rebind gate skips only the dropped native transcript, not current source authorization. Current locked history/manifest checks run again before disclosure; original receipt/hash replay remains exact. A typed finalization refusal does not spin in the requeue path. The strict per-task descriptor rejection marker cannot fall back to ordinary prompt execution.
- **Managed settings → actual physical invocation:** The settings pair remains before both filtered configured argument lists. Knowledge fingerprint and leaf admission still validate the same finalized default/custom args, settings path and persistence policy. Ordinary fresh/reusable sessions and collaboration's fresh/no-session-persistence namespace remain separate; the H2 prepared fresh identity reaches later authorization/observations.
- **Knowledge host → handoff/checkpoint/recall:** Knowledge preparation stays bound to the negotiated frozen handoff and exact prompt/permit. Recording retry/tail/termination recovery does not grant a new disclosure or replay an input. Actual fake host tests exercise approved knowledge plus handoff and recording-outage/proof recovery on this combined tree. Recall sources/comments remain capability-checked data; member display labels do not grant authority. Optional display names and live-only memory base metadata preserve old-response behavior and frozen snapshot encoding.
- **Human write → websocket hint → Query projection:** DB-clock budget/expiry and PAT revocation ordering coexist with transactional target hints. The fair drainer keeps stable hint identities; final-send authorization precedes write budgets. Frontend hints/lease loss erase private Query and rollback values before refetch; later-page reset obtains a new root authorization rather than renewing the old lease. Unknown recovery membership uses a current Core detail and cannot authorize a restricted candidate or close another draft. Original receipts remain content-free recovery identities.
- **Migration and defaults:** The three concurrent index migrations and cleanup registration survive the merge. Generated query files match reviewed branch bytes; coordinator records source-bound sqlc0-drift/build/vet at this tree. `.env.example` and Compose keep knowledgefalse and router admission requires the literaltrue. No production enablement occurred.

## Independent combined-source verification

| Check | Result |
| --- | --- |
| Guarded fake host knowledge/handoff/recording, daemon rebind/strict claim, agent settings and protocol selection |107 named passes,0 fail/skip;5 package exits0 |
| CLI recall/handoff/source/preview/optional attribution selection |58 named passes,0 fail/skip |
| Core knowledge action/lease/cache plus handoff parser |45 passes,0 fail/skip |
| Shared knowledge panel, handoff view and locale parity |104 passes,0 fail/skip |
| `git diff --check` |exit0 |

All commands, actual exits and raw-log hashes are in `review.json` and the per-command records. Tests use repository CLI guards, real-call gates removed and two-worker/two-parallel bounds. The CLI package had no matching named case in the first broad selection, so its58 named recall checks were separately executed and counted exactly once.

## Remaining gates

This is an integrated product/merge review, not final acceptance. The coordinator's current source-bound snapshot shows handler5279 pass/53skip and router731 pass/7skip with matching completed-log hashes; service remains running and is not counted passed. Current sqlc/build/vet records exit0. Full frontend and DB package outcomes, final make check, gated Web/native Electron, RK1 fake on the final framework/candidate, real models and platform/install/upgrade/account gates remain explicit coordinator work. Prior per-lane passes retain their original source scope; they were not combined into a fictitious final full pass.

No reviewer self-fix or product change was made.
