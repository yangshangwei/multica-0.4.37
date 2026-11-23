# F3 final takeover and source reconciliation

Current owner: Codex session `01a1f915-42b9-7422-a8d8-aa97e9af8724`.
Code: `/Volumes/artisan/code/2026/mc-f3-completion-20261120`, branch `codex/f3-completion-20261120`.
Task identity is unchanged; no replacement task was created.

## Ownership proof

At 21:11 the original Claude PID31007 was still alive, with watch PID36626 and mutation runner PID48690 plus a Go child. Its transcript records user interruption at 21:12:10; subsequent process scans found none of those processes or a matching F3 test/watch process. Child transcripts stopped at 21:09–21:10. No original tree was terminated, overwritten or removed by this takeover.

The previous Codex session explicitly froze its own edits and all three subagents, then confirmed they were completed with no active command or pending write. Its Views r4 owner polled exec70123/PID46891: exit0,608 files/7441 tests,436.04s. The raw log is retained. Core's aligned typecheck also exited0. Reports that still say Views pending describe the earlier cut.

Final old-source manifest:213 files, zero SHA256 differences at takeover. Old r1:211 files, zero differences. The two additional executable files were the supplemental acceptance tests and F2 atomic-counter test repair; their later normal/race evidence is retained. Independent raw-log audit found565 canonical top-level names and142 names cited by the62-row map passing normal/race; B54 has91 exact names from14 files, each passing pristine baseline/current normal/current race.

## Original trees and final adjudication

Read-only native check compared actual source bytes across original root,6a,6b,7b,6kd2,verify5c1 and verify5d. No missing product fix was found. Root and verify5c1 share110 changed paths;6a contains the same110 plus the two integrated protocol fixtures. The34 migration files map866–882 to869–885 without SQL behavior drift. Frozen6b21-file and7b43-file manifests still match their source trees.

KD2's three regression files are included byte-for-byte under their final names. KD2-11 request-count synchronization and KD1-02 missing/zero private journal version checks are included. KD2-06 retains the seven-day cap: expiration-first may drop without a final send; direct-send-first may deliver. This conforms to design11.10 and neither order releases server hold. The residual test comment was corrected; no test assertion or product behavior changed.

Three unfinished KD1 review tests are retained as `historical-*check6kd1_test.go.txt` in the evidence directory. They are historical review material, not executed final acceptance. Original step6-check-daemon1/2 reports retain their unfinished/pending labels. The completed bounded Codex crosschecks and final independent review are the accepted review, not a claim that those original reviews finished.

Source-tree inventory digests (sorted `SHA256  relative_path\n`, then SHA256):

| Source | Paths | Digest |
| --- | ---: | --- |
| root / verify5c1 |110|1eb8a35bcd91c5a9ada94ddcc6e024984cd18315e64b163afeaff2dcd9f0ccfe|
|6a|112|6bfe129cc7ccea95d1483edfd4d801fab4d1dbfcc894881d70f268d89344f228|
|6b including unfinished review tests|24|6e45f35d61ef697cdba57dbc644e95a3c19f99e30f0d9377ecaecb7e21ab206a|
|7b|119|6c2ca60f2e1dbdada1198d88a3e45219045fb9381028dae0b9c25f65c074c05f|
|6kd2|25|bd5e4190874f8e3bb9a05569bfe46450225f6092187a40812abe8d44545adb0f|
|verify5d|97|c718dc16f2eb3fde3cae4975cd49c7e8ae9937a628d0ac484a40b347e962f49b|

## Spec and integration closeout

Added explicit READ COMMITTED deployment prerequisite, all-server-upgrade-before-enable rule, retained-history/off rollback, populated-down refusal, and both read-only missing-binding/tombstone probes. The tombstone detector excludes deleted workspaces because workspace teardown intentionally leaves all six audit tables untouched. Clarified exact POST-collection/GET-key management routes. Independent source review accepted these corrections.

Feature commit `584846f95` contains the213 owned product/spec/test files. Fresh Go build/vet passed; full metrics88 named tests passed normal/race with the agent CLI guard and explicit unreachable DATABASE_URL. No production behavior changed after the original accepted cut.

Shared parent `f2ea7d50d` merged without conflict into the feature tree. Nine overlapping files preserve both branches. Independent review confirms task reclaim preflight still reaches finalize, collaboration-root creation checks precede the same-transaction budget binding, budget middleware stays transparent to non-budget routes, and new Core collaboration APIs coexist with budget parsing/redaction. Shared migrations866–868 do not collide. Integrated verification is recorded separately; old isolated evidence alone does not prove this merge.

All old worktrees, failure logs, original review reports and databases remain available. No Provider/installed agent, POLA-1, shared-policy operation, push/PR, deployment or Hosted CI was invoked.
