# Password live-revoke fixture repair and package source binding

The first password live-revoke run (`password/password-1/pw-21`) failed at the positive assertion for still-authorized Agent B. The trace retained the Comment composer's `Mention` button with `aria-expanded=false` after reload. The test attempted to inspect the collapsed AI targets group before exposing it. This does not prove that B's authority was lost.

The root repaired only `e2e/collaboration-live-revoke-web.spec.ts`: reuse `revealCollaborationMentions(composer)` before asserting B and squad S visible and revoked A absent. The helper itself asserts that the participant/AI group is visible. Existing server catalogue, rejection, unchanged-other-lanes, stop/proof and explicit Resume assertions remain intact. No product source or timeout/geometry assertion changed.

Original evidence:

- `/Users/artisan/.multica/qa/core-20261108/password/password-1/pw-21/playwright.log`
- `/Users/artisan/.multica/qa/core-20261108/password/password-1/pw-21/results.json`
- `/Users/artisan/.multica/qa/core-20261108/password/password-1/pw-21/artifacts/collaboration-live-revoke--a83f9--proof-plus-explicit-Resume-chromium/trace.zip`

The extracted matching trace snapshots and trace hash are retained privately in `packaged/live-revoke-failed-disclosure.private.json`. Replay is root-owned at `password/password-2/pw-21`; inspect its actual terminal results rather than counting the repair itself as PASS.

## Post-build test-only source delta

The mac-3/Linux-2 directory packages bind `packaged/source-files-v2.json`, which remains byte-identical and is not rewritten. A new `source-files-v3.json` describes the current test checkout. Comparison of every captured source file and mode proves exactly one difference:

- Path: `e2e/collaboration-live-revoke-web.spec.ts`.
- Before SHA-256: `25edb2607a927196ba3d21315261eb3775e7b0446c9fa203d0e86186780d3594`.
- After SHA-256: `49db85d0cdb4c3f2e87643cf2776dc7463b2a89900a497ffb45dfc7262938718`.
- Mode remains 0644; all other **27,606** source files/modes are unchanged.
- No application source, build input or packaged payload changed; no rebuild is needed for this test-only repair.

`/Users/artisan/.multica/qa/core-20261108/packaged/source-binding-v3.json` links the unchanged v2 build manifest, v3 test manifest, exact patch hash and original package proofs. Package proofs continue to reference v2 because those are the actual compiled inputs. Controller ownership remains with the independent reviewer/root: this lane did not edit `packaged/run.py`. The owner must consume the exact reviewed v2→v3 binding rather than replacing the original build identity or allowing arbitrary test changes.
