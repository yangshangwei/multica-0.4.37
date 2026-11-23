# AC34/57 current client evidence

Date: 2026-10-22. Existing task `10-20-collaboration-gap-recovery` stays
`in_progress`. This report proposes two whole-clause promotions for independent
review. It does not edit the acceptance matrix, source definitions, task status,
product gates, or shared fixtures.

## Changed files and boundary

- `e2e/collaboration-lane-close-controls.spec.ts`: one shared dual-client harness;
  two same-lane cases and four actual live complete/cancel cases.
- `e2e/fixtures/collaboration-lane-close-controls.ts`: public registration and
  owner-consent UI, typed squad comment, scope/hold assertions, actual lifecycle
  close UI and exact retained command/proof assertions. It imports the existing
  lifecycle, cross-client daemon and native helpers.
- `research/continuation-2026-10-22/lane-close-*`: immutable commands, raw results,
  extracted identity evidence, copied captures and diagnostic results.

No application, SQL, schema, router, locale or existing fixture changed. No new
dependency or test runtime was introduced. The same harness exercises Web and
Electron by swapping creator/handler roles rather than copying either journey.

## AC34 clause assessment

The public squad create/add-member API establishes S with real direct A as its
leader and creator-owned C as its member. Shared registration and each actual
resource-owner consent form create S in A's real channel. No target/channel row
is inserted in the database.

1. Direct A and typed squad S each have a waiting immutable input. Stop A's
   original receipt lists both targets and exactly one channel; their shared
   visible lane is paused and contains both typed resource IDs. Target Resume S
   resumes that lane and returns both aliases.
2. Independent B and C target/channel snapshots remain identical through the
   A/S Stop and Resume. B can Stop/Resume independently while A/S snapshots stay
   identical. The separate peer issue snapshot is identical at journey end.
3. Withdrawing S's member grant holds S only. Resume A releases the shared lane
   while S retains `grant_required`. S's public owner reconsent does not release
   its hold; the new explicit Resume S does. A's and B's independent grants and
   holds are preserved.
4. Whole Stop pauses the issue and every channel. A new D is publicly created,
   registered and owner-consented while paused. Target Resume D returns the
   original `issue_paused` refusal without execution writes. A newly posted D
   comment remains waiting with the exact preview reason `issue_paused`.
5. The other actual client reloads and attempts only Resume S. It receives
   `issue_paused`, displays the explicit-resume explanation and leaves execution
   identical. Only a fresh whole-issue Resume releases the issue/channels/holds.
   All three queued A/S/D commands remain waiting; no provider is launched.

Current final evidence: `lane-close-final-web-same-lane-identities.json` and
`lane-close-final-electron-same-lane-identities.json`, with two 0-retry PASS results
in the six-case `lane-close-final-summary.json`. Existing current canonical race results in
`lifecycle-service-canonical.jsonl` separately pass
`TestCollaborationTargetStopLeavesOtherTargetAndIssueActive`,
`TestCollaborationIssueStopFencesChangedAndLaterTargetSets`, and
`TestCollaborationResumeAliasesKeepIndependentHolds`. Actual descendant/partial
proof behavior remains covered by the existing current Linux and service
evidence; the offline alias UI case makes no containment claim.

## AC57 clause assessment

The four complete/cancel cases use two actual permitted Linux runs concurrently,
one in A and one in B. The creator and handler each post one remaining waiting
instruction to both lanes, making four waiting inputs from every collaborator.
The creator invokes the shared Complete issue/Cancel issue form in Web and, in
separate cases, actual Electron. Status selection and the exact lifecycle POST
are observed; no terminal state is seeded.

1. The public close receipt confirms completed/cancelled business status while
   both run projections retain `proof_state=unknown`, `settled_at=null`,
   `forced_disposition=stopped`, and `progress_state=ending`.
2. Both clients show Closed plus disabled Resume All, each original run's
   Termination unconfirmed/Waiting for termination, and its actual records
   message `Stop accepted; termination remains unconfirmed`.
3. Every waiting instruction is cancelled with the exact immutable body,
   author, target, batch, sequence, automatic cancellation actor, event UUID,
   event sequence, timestamp and `issue_completed`/`issue_cancelled` reason.
   Both clients show each history item and its real reason after reload.
4. The existing transport proxy withholds only the original natural sealed
   proof upload. Each run records HTTP503, then HTTP200 after release, with one
   SHA-256 for all its proof attempts. Provider sockets are never released by
   the test; the real server Stop and Linux supervisor produce the exit proof.
5. A's proof arrives while B remains unconfirmed; the issue stays closed and all
   cancelled history stays identical. After B's original proof, both runs are
   `exited_proven/settled` and consumed original inputs are Stopped. The issue
   remains terminal, every lane stays paused and cancelled instructions never
   revive. Exact history remains readable in both clients; the peer is unchanged.

Current final evidence:
`lane-close-final-{web,electron}-{complete,cancel}-identities.json`, four PASS
entries in `lane-close-final-summary.json`, two independent natural proof
identities per case. Canonical lifecycle/barrier race results separately pass
`TestCollaborationLifecycleCloseAndReopenPreservePermanentQueue` and
`TestCollaborationWholeLifecycleKeepsAllTargetBarriersOnReopen` in the current
15-top-level/35-including-subtests, 0-fail/0-skip run.

## Verification and retained attempts

- Strict scoped TypeScript (`--strict --noUncheckedIndexedAccess`), scoped ESLint
  and discovery pass at the final observed-helper source. Discovery finds six
  named cases. Commands and outputs are in
  `lane-close-static-public-{typecheck,lint,discovery}-result.json`.
- First full execution: 5 PASS, 1 failure, 0 skip/flaky/retry, 92.719 seconds.
  All four live complete/cancel cases pass; the first Web alias case fails a
  consecutive control observation. Raw SHA-256:
  `9bb504f867ce6701c4343199423638222c2e196d35acb6be66da14fbfc2e4587`
  (private original); public redacted raw:
  `2b398907e0535cc036c5b60083dd3822e9dee6a20ddd58aeeb23979d43b555cc`.
- The original retained Stop/Resume operations prove the exact refusal was
  `stale_control_version`409, not a grant or termination failure. The Stop
  advanced version5 while the visible Paused label remained identical.
  `lane-close-web-first-refusal-corrected.json` preserves these original keys and
  receipts. The earlier read-only query used nonexistent `http_status`; its
  diagnostic error is retained in `lane-close-web-first-refusal.json`.
- The fixture now explicitly reloads a current baseline before a new human
  control and asserts that the actual POST contains its exact target/control/
  lifecycle versions. It does not retry the refused operation, accept409 as a
  success or lower a server guard. Current alias execution: 2 PASS, 0 fail/skip/
  retry, 42.174 seconds. Raw SHA-256:
  `2b859bf58468641a7f8e798384b3efb550cb50e68f752764d995e1b8cf115642`
  (private original); public redacted raw:
  `e6ddbbc7e67f7497805a48857adda8ace07e3c7b3e0c1198277245fc588a626e`.
- Final six-case execution at one source revision: 6 PASS, 0 fail/skip/flaky/
  retry, 108.507 seconds, no source drift. All six original final attachments
  pass the decoded credential/key audit without rewriting. Raw SHA-256:
  `3bbcab7c52cb54937c6350bc4051234513ca966476258cf8d921b7a814356558`.
  Command/result and extracted cases are in `lane-close-clients-final-*` and
  `lane-close-final-summary.json`. The prior mixed result is preserved.

## Evidence projection correction

The first private evidence audit found that two same-lane registration summaries
included the fixture target's internal runtime object. That object carried a
synthetic managed credential. The public evidence projection now emits only the
target, resource, channel IDs and display name. All five original raw/extracted
files are preserved outside the repository in a mode0700 quarantine directory,
with original files mode0600; credential values were never printed by this
audit. The repository copies retain their real verdicts and public identities
after redaction. Result records distinguish original and redacted raw SHA-256.

`lane-close-artifact-redaction-repair.json` records that correction and the
original fingerprints. `lane-close-redaction-final-check.json` validates all
text artifacts, decodes raw Playwright JSON attachments and checks both token
patterns and credential/nonce fields; it passes with zero violations. Three
affected synthetic offline AC34 resources were deregistered through their own
public daemon API, all HTTP200. No live process, proof barrier or another lane's
runtime was altered. There is no public owner revoke API for those managed
credentials; no administrator authority was invented. The final six-case run
used new isolated identities and the corrected public projection; its original
raw bytes required no redaction rewrite.

## Environment and remaining boundaries

`lane-close-preflight.json` rechecks API listener22414, production Web build
`tP4r55FoDsuyXfi1EylGw`, matching renderer/index SHA-256
`337c08a228217fb574b77bb5fb6942bcbc40f2f1ce42587ceec5264204f3b35c`, and
the explicit test-owned Linux daemon/provider binaries. Parent
`content-admin-preflight.json` records all application/build/source provenance.
Scoped source fingerprints stayed unchanged during both executed runs.

The real executable is the test-created stdlib Linux provider. No real provider
account, model/quota, provider session discovery, production gate/rollout change,
installed full Main/installer, macOS/Windows execution containment or broad
Playwright inventory was exercised. Actual Electron runs use the existing
production renderer/native IPC harness with isolated profiles. The matrix and
spec updates, independent review and Lore commit remain the coordinator's work.
