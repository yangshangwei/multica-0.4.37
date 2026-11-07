# Component browser verification matrix

Fixture owner: evidence_assessment. Product writes are owned by the implementer.

Build current shared Views/UI/Core sources, production CSS, and locales with the installed Vite/Tailwind toolchain. Query and upload responses are intercepted inside the fixture; draft persistence uses a per-page memory StorageAdapter. All non-fixture browser network requests are aborted. No full stack, real accounts, agent execution, real submissions, or new dependencies.

Commands after implementation-ready:

```sh
node .trellis/tasks/11-07-collaboration-create-composer/research/preview/build.mjs round-1
node .trellis/tasks/11-07-collaboration-create-composer/research/preview/verify.mjs round-1
```

The verifier starts an owned ephemeral 127.0.0.1 listener and fresh Chromium contexts. It records listener origin/PID, browser process identities, hashes of current product/CSS/locale sources, screenshot paths, DOM geometry, full-ID text ranges, painted focus indicators, mock request counts and final cleanup. It closes contexts/browser/listener in finally. No persistent preview server remains after the command exits normally.

## Matrix

- Desktop 1280×900 in light/dark: hierarchy, labels, recipient listbox/AX, focus contrast, advanced disclosure and shared picker compatibility.
- Coarse 390×740 and 320×568 in both themes: initial and scrolled footer/close reachability, >=44px audited target geometry, body-only scroll, no viewport overflow, full account identity and 24-step Tab traversal.
- Same six desktop/narrow/theme combinations with 32 selected attachments and very long filenames: every checkbox remains in the scrolling body, footer does not move, last file remains keyboard-reachable, body retains useful height.
- 320×568 light/dark: compact attachment entry keyboard opens chooser; empty selection sends no request; cancel an active mock upload; recover original upload; retry a lost response; verify visible focus and painted indicator at the entry after controls disappear.
- 320×568: configured advanced summary, invalid retained value expands disclosure.
- Native Enter after actual browser/CDP IME composition: composition confirmation sends zero create requests, subsequent plain Enter sends one payload with title/plain description. CDP composition is not physical OS IME evidence.
- Retained incumbent cases: delayed-create source lock, confirmed quota rejection (en/zh desktop/narrow), original-key unknown-result recovery (desktop/narrow), exact recipient identity, parent debounce, generic picker/dialog defaults.

## Sampling policy

Prepare only until the parent says implementation-ready. One whole initial batch, then one combined confirmation batch for real defects. Harness errors may be corrected against the same immutable build and rerun only affected scenarios; such calibration is explicitly distinguished from UI correction. Persist visual verdict in `.omx/state/collaboration-create-composer/ralph-progress.json` before requesting any product correction.

## Reference and limitations

Prior collaboration screenshots are in archived task `11-02-collaboration-creation-audit-fixes/research/browser/`; current source hashes differ, so they are stylistic/history references only. Ordinary composer source `packages/views/modals/create-issue.tsx` supplies breadcrumb, role-named title text, body scroll, compact controls and responsive footer reference; no fresh ordinary composer runtime is mounted by this fixture. Real backend/Next navigation, Electron Main, physical touch, spoken screen reader, and real OS chooser/IME remain outside this isolated component proof.
