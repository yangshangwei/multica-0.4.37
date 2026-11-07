# Installed Main recipient-picker harness review

Verdict: **PASS WITH NOTES** for the bounded picker adaptation. No blocking code issue was found in the reviewed change. This is not a successful full Main run, installer/upgrade acceptance, or release approval.

Review snapshot: `2026-11-02T01:40:20Z`. Product candidate: `7af9ab9905c98c77bb091223ebe687c639475bb4`. Verification checkout: `/Volumes/artisan/code/2026/mc-functional-7af9ab990-20261102`.

## Scope

Read the current account-picker/workflow implementation, the existing TypeScript recipient helper, both changed installed-Main test/controller files, the new JavaScript helper, the saved patch/binding, and the two recorded Linux full Main results. Only this review file was written. No tests, database connections, API calls, services, containers, native windows, credentials, or child agents were used by the reviewer.

## Findings

1. **The old locator is obsolete.** `packages/views/collaboration/workflow.tsx` renders `CollaborationAccountPicker` for invite, transfer and request-information actions. `account-picker.tsx` exposes a button labelled Recipient, a labelled listbox, filter input and an exact-lookup footer. It does not render the former Exact recipient input directly in the workflow form. The original `linux-full-main-1` failure is precisely the first invite waiting for that obsolete input; it remains failed.

2. **The new steps follow the real UI.** Both helpers open Recipient, fill the UUID filter, open the exact-lookup footer, submit the same UUID, observe the returned account name, and click Use this account. The exact footer requires a nonempty query; the preceding filter fill establishes that condition without sleeping or changing time budgets. The popup locator is scoped to the labelled listbox.

3. **The plain-Node helper matches existing behavior exactly.** The body of `e2e/fixtures/collaboration-recipient-picker.mjs` is byte-for-byte equal to the body of the existing `selectCollaborationRecipient` TypeScript helper after its signature. The new file's reviewed SHA-256 is `3ebcda800248387b4c596a4ee84d25a666209355df7f2fa598e0152a8ce1a986`. A small `.mjs` counterpart is reasonable for the packaged client's plain-Node controller, which cannot assume Playwright's TypeScript transform. It adds no dependency or product compatibility layer. A broader helper extraction is unnecessary for this repair; keep the two implementations aligned if selectors change later.

4. **Authority and resulting behavior remain asserted.** The changed calls retain the real POST, original idempotency key, expected collaboration/lifecycle revisions, directed status selection and subsequent participant/handler/state comparisons. The installed journey still compares Web/native projections, author identity, immutable history, paused/unconfirmed execution, information recipients and the six distinct workflow writes. No product code, native IPC handler, authentication flow, timeout or retry was changed by this picker patch.

## Nonblocking notes and integration conditions

- **N1 — Retain the selected-account display check.** The old three-line interaction included a form-level Selected account assertion; the generic helper currently verifies the exact result before selecting but does not repeat that post-selection display check. Authoritative handler/recipient checks later in the journey still protect the business outcome, so this is not a blocking coverage loss. For exact UI-assertion parity, use the existing `selectHandoffRecipient` wrapper on the TypeScript side, or add `expect(form.getByText(...)).toBeVisible()` after selection. Add the equivalent in the Node helper/callers as appropriate. Do not describe every old UI assertion as retained until that check is restored.

- **N2 — Bind the new helper bytes to actual runs.** The research binding records the helper hash, but `linux-full-main-2/run.json` lists only tracked dirty files and its `harness.patch` cannot contain the untracked new file. Preserve that result as recorded. Before the next acceptance claim, include the new helper's content/hash in that run's harness evidence, together with tracked patch and imported helper identity. Merely saying HEAD is unchanged is insufficient when the verification harness has local changes.

- **N3 — Full Main execution is still unresolved.** `linux-full-main-2` passed the initial Web invite/transfer interaction and reached packaged-client startup, then exited with `Installed Main container exited: code=1, signal=null, private stderr bytes=894`. It recorded **0 expected / 1 unexpected**, exit 1. The public evidence does not establish that container exit's exact cause. Resolve its prerequisite/runtime failure and run the complete original business assertions; do not classify the second run as a native pass or assume it proves the Node helper's later workflow paths.

- **N4 — Keep artifact and platform claims separate.** The selected artifact is an electron-builder directory package mounted into a Linux container. Equal product trees and pinned ASAR/CLI/executable bytes can justify testing it against this candidate, but neither that preflight nor this review proves package-manager installation, upgrade, signatures, a native Linux desktop session, Windows, or a real commercial Provider account. Historical package results remain historical.

## Conclusion

The picker change is a faithful adaptation to the current exact-account UI and does not justify weakening the retained business assertions. Apply N1/N2 when preparing the next recorded harness, investigate the separate container exit, and preserve both original failed runs. This review does not approve the old adapter-v5/RKI6 launcher, external operations, or paid Provider calls.
