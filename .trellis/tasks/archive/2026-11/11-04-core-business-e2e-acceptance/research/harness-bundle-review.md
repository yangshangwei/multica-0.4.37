# Current UI harness independent review

**PASS for static harness review, zero open blockers. Live E2E acceptance remains pending.**

Candidate: `1a32ca36c2c4f4455a01bdd39d0e0ab4db20cccc`. Reviewed against the frozen checkout, not later product changes in the root workspace.

## Exact reviewed inputs

- Base12-file bundle SHA: `70fe66c6e8df1b7745b6bbccab00129419007c893841b4c42203be911045d2d9`.
- Required restart followup SHA: `8c2dfd0ead5d2449cecc9e57cf14a2f8454f2973c004f42eadd69e22846e3b83`.
- Final installed controller SHA: `aa0f1ff6375dc9ca462a1361e5c3324fde58835c1d75b952c39e33eda2b2e749`.
- The JSON companion records all12 final file hashes. The original bundle manifest's installed-main hash is superseded by this followup.

## Finding fixed during review

`collaboration-installed-main.mjs:308` still waited unconditionally for `stopped` inside `launch()`, even though metadata had been updated to expect `running` after managed login. `restart` calls `launch`, and the current product deliberately reconciles a managed session to running; with autoStop=false, the old check would time out before reaching metadata.

The author corrected this under parent authorization. Both launch and metadata now keep the initial unauthenticated stopped assertion, while an authenticated managed restart requires running and verifies the exact private profile, daemon PID file, server URL, deployment and user. The reviewer re-read the fixed path and verified final bytes. No product daemon semantics were changed.

## Preserved behavior

Result and basis now travel through the actual current forms. Receipts bind the exact submission; creator acceptance and the previous forbidden-role probes remain. The close/reopen cases first submit as the real handler and assert session/execution equality before acceptance, then retain in-flight proof, cancellation, Stop/Resume and history checks.

Disclosure helpers use actual More actions/Attachments controls. They do not force clicks or bypass business responses. The installed Web helper reuses the shared directed-form assertions instead of keeping a duplicate implementation. Managed daemon cleanup uses the real stop IPC only after verifying task ownership.

No assertion timeouts were increased, no automatic retry was introduced, and no new business mocks were added. Frozen status contains only the12 reviewed E2E files plus the expected generated Next route-types import; product sources remain unchanged.

## Verification and limits

Reviewer: node syntax check PASS; git diff --check PASS;11 TypeScript files transpile without syntax diagnostics;12 final hashes match. Writer evidence reviewed:35 actual-export provenance contract cases PASS,7 existing controller/layout cases PASS, and22 E2E cases across12 specs collect successfully.

Full ESLint and semantic TypeScript checks were not run during this bounded static review; root E2E files are outside the current package lint paths, and collection/transpilation are not a semantic type-check. No browser, database, Provider or service was started by this reviewer. The original failed E2E cases must be replayed against newly bound runtime provenance before being marked resolved.

The coordinator still owns current macOS package before/after hashes where the fixture itself only enforces all pins for Windows/DEB. After successful live verification, capture the initial-startup versus authenticated-managed-restart distinction in the E2E environment spec.
