# AC67/75 current creation and retained-file evidence

Existing S6 continued without task creation or replanning. Final scoped strict
TypeScript, ESLint and Playwright discovery pass. Production Web/actual Electron
passes **3/3**, no skipped, failed, flaky or retried tests, **28.064 seconds**.
Coordinator canonical service/handler race passes **13 top-level /19 including
subtests**, no failures/skips, **45.373 seconds** on schema737 `service_s6_other`.
Promotion and independent full-clause review remain coordinator responsibilities.

Changed files are `e2e/collaboration-retained-attachment.spec.ts`, the preserved
existing `e2e/fixtures/collaboration-retained-attachments.ts`, and exactly two
return-only authentication closures in `e2e/fixtures/collaboration-control-journey.ts`.
Existing `collaboration-delivery-web.spec.ts`, `collaboration-account-desktop.spec.ts`
and native/account IPC helpers were not edited. The dedicated spec reuses the
existing public enrollment/owner-consent fixture and real password cookie login;
it does not copy setup or introduce production code/dependencies.

## Exact exercised clauses

AC67 creates through the real shared form in Web and Electron with one body
containing a current human typed `mention://member/<UUID>` and the actual
creator-owned AI typed `mention://agent/<UUID>`. The AI has real public
registration/consent in the independent fixture; its runtime remains offline.
Each first upload is still a private, creation-intended draft. Own byte read is
exact; another actor receives404; metadata contains no public URL, storage key,
encryption key or digest. Creation correlates the original request, title/body,
source, two participants, member handler, first delivered file and session.
Original-key POST/GET replay finds the same sole source issue. Initial paused
queue/control, empty targets/channels/commands/groups/runs, no comments and zero
ordinary dispatch remain unchanged through replay, refresh and client restart.
The independent fixture issue's consented execution/control also stays identical.
Canonical Go tests own transaction rollback, uncertain commit recovery and
stored ciphertext; client observations do not substitute for database atomicity.
Earlier lost-upload/lost-create assertions remain intact in their existing spec
and receipts under `../continuation-2026-10-21/creation-delivery-current.md`.

AC75 uses a Web comment author and a zero-workspace Electron participant. The Web
UI submits the actual typed AI comment with a real private file, then removes its
display reference through Edit and deletes a second attached comment through
Delete/Confirm. The second comment stays snapshot-identical until its own
deletion; the frozen AI input's original text, file IDs, author and command/event
identity remain identical throughout. After both mutations every current comment
has no file reference, while both precise history events retain before/after IDs.
Neither control, queue nor ordinary dispatch changes; no provider process starts.

Each actor refreshes then actually closes/reopens its client. Both Web and
Electron independently preview exact retained bytes from the issue attachment
list, edited-comment history, deleted-comment history and frozen command history.
The issue list and both comment histories also download exact original bytes:
**12 correlated downloads** across two actors and two refresh/restart stages.
Electron downloads use the actual production `file:download-url` IPC registration
and Electron's `will-download`/`done` event with a private test output path.

The recipient keeps an open exact-byte preview through public participant
removal. Live access loss removes the dialog/private title/content. Old preview
and download capabilities both return404 **before expiry**, also after public
reinvite; old history, direct content and command URLs deny. Anonymous native
capability/read URLs deny rather than redirect or expose a public link. The
creator still reads exact bytes, and renewed participation only permits a fresh
authenticated byte request. Backend tests own the broader slow-read/source-only/
tampering/public-object permission matrices.

## Commands and provenance

The exact final client command is in
[content-clients-final-command.json](./content-clients-final-command.json), raw
JSON/stderr in `content-clients-final.json`/`.stderr`, and parsed statuses in
[content-clients-final-result.json](./content-clients-final-result.json). It uses
the existing private `admin-test.py` launcher, the dedicated spec,
`--workers=1 --retries=0 --reporter=json`, and a task-owned artifact output path.
Unique final identities are `content-clients-final-web-creation-identities.json`,
`content-clients-final-native-creation-identities.json` and
`content-clients-final-retained-cross-client-identities.json`.

Scoped static commands/results are `content-tsc-final*`, `content-eslint-final*`
and `content-discovery-final*`. TypeScript uses the existing strict temporary
E2E template with raw-core alias and noEmit; ESLint keeps repository rules and
explicitly treats E2E fixtures as development imports. Exact temporary config
contents/hashes are retained in `content-static-config.json`; no repository lint
or compiler configuration was changed. `git diff --check` also passes.

[content-admin-preflight.json](./content-admin-preflight.json) is the coordinator
source/ownership proof:4549 compared product files have zero drift, API18996 is
PID22414 with matching source/configuration, Web13916 is production
`tP4r55FoDsuyXfi1EylGw`/PID76944, and renderer6090 is PID36145/HTTP200 with exact
built index SHA337c08a228217fb574b77bb5fb6942bcbc40f2f1ce42587ceec5264204f3b35c.
Native Main harness matches the snapshot. Host native helper difference is only
the prior font/two-frame screenshot wait; its exact hash is recorded.
[content-source-final-manifest.json](./content-source-final-manifest.json)
confirms all three final cases use the same final source bytes. Initial hashes
and preservation boundary are in `content-boundary-20261022.json` and its
retained fixture text snapshot. No listener was started or restarted by this lane.

The exact Go command/source/result is `content-backend-current-command.json`,
`content-backend-current-result.json`, raw `.jsonl`/`.stderr`. It includes all
three AC75 canonical identities:
`TestCollaborationAttachmentCommandSnapshotOutlivesCommentDeletion`,
`TestCollaborationCommentAttachmentDisplayRemovalKeepsDeliveredFile`, and
`TestCollaborationAttachmentHTTPUsesScopedUploadAndRevocableNativeBytes`.

## Retained first attempts and boundaries

`content-tsc-first*` caught an unused pre-existing fixture type import, now
removed. `content-eslint-first*` caught a configuration scope mismatch: E2E
fixtures use the declared root devDependency; the final external config
allowlists those test imports while preserving all other lint rules.
`content-clients-first*` passed both typed creation cases but failed the new
fixture's edit-receipt assertion. The current edit wire legitimately omits an
empty `attachment_ids`; the corrected assertion verifies actual PATCH
`attachment_ids:[]` plus the authoritative comment GET. No product/schema change
or weakened display/reference/history invariant was made. The exact retained-file
rerun `content-retained-second*` passes1/1, followed by final current-source3/3.
No retry/flaky result is used as final evidence.

The actual Electron IPC harness uses production routing, preload, file download
and renderer with stubbed daemon/install lifecycle. It does not establish full
installed Main/installer or old-client startup. Resource registration is
synthetic; it never invokes a provider or spends account quota. Other OS
containment, full notification fanout, full Playwright inventory and a new
monolithic `make check` are separate evidence scopes. Production gates were not
changed; task and parent remain in progress. No commit or ledger/spec/archive
mutation was made by this implementation lane.
