# Verification evidence

## Final implementation checks

- Canonical projects-page tests plus locale parity: 90 passed after the final headline correction. The first test-first pass failed on the three intended missing-welcome assertions while 21 existing tests passed.
- All project component tests plus locale parity: 24 files, 277 tests passed before the display-only correction. The correction's affected page/parity tests were then rerun successfully.
- Scoped ESLint and views typecheck passed after the final `Trans`/responsive typography edit.
- Web typecheck and Desktop node/renderer typechecks passed. The final production Web build also compiled the corrected shared component successfully.
- Incremental Trellis review of `Trans`, responsive typography and the new spec found no issues. Owned whitespace checks passed.
- Impeccable detector ran once over the changed welcome TSX/CSS and projects page and returned `[]`.

## Visual evidence

`qa/project-welcome.browser.ts-1fc76-the-production-Web-renderer/` holds the final complete capture batch: English/Chinese, light/dark and 1440/390px, plus example detail views at narrow widths and current triage/iteration references. `layout-metrics.json` proves the following for all eight layout variants:

- no document or welcome horizontal overflow;
- primary target height of 44px;
- a 36px wide headline and a 24px narrow headline;
- sampled foreground text contrast meeting 4.5:1 (3:1 for large text).

The main agent visually inspected wide and narrow captures against the incumbent references, persisted a first 88/revise verdict, corrected only headline measure/word boundaries/font breakpoint in one batch, and confirmed a final 95/pass verdict. `qa-first/` preserves the first batch. No additional polishing loop was run.

The initial combined browser script completed its visual capture but failed on verification-harness assumptions: measuring colors before a shared button transition settled, expecting an English title from the Chinese bundled documentation, and treating Escape from the existing create-form editor as guaranteed dismissal. Corrected the harness to wait for finite transitions, read the deployment's actual docs title, and use the existing Cancel action. No product behavior was changed to satisfy these checks. Raw traces/snapshots remain in the task-owned runtime outside git.

The capture script now only owns visual assertions; the separate actions script owns flow verification. Do not repeat visual capture simply to duplicate action evidence.

## Actual actions

`project-welcome.actions.browser.ts` passed against the corrected production Web build (1 test, 4.0 seconds). `qa-actions/*/actions-evidence.json` records:

- keyboard activation opens the existing New Project modal;
- its existing Cancel action closes the modal;
- keyboard activation of the project guide reaches the existing in-app docs;
- browsing the example causes no project/issue/agent/squad writes;
- a real API-created project appears in the list and removes the welcome;
- no page errors.

## Runtime and limits

Renderer: Chromium against production Next.js, API `http://localhost:18573`, Web `http://localhost:13493`. Final build ID: `T_iEhuCL2S5_g0_rtnmZF`. API health recorded source commit `fed531eb4`, with the scoped dirty-source changes included in build provenance.

The task-only registry, env, database and auth are isolated from the repository's normal `.env`. Local startup required the already installed `/opt/homebrew/opt/libpq/bin` on PATH and disabling managed-installation settings only in the test env, following the existing `make check` legacy fixture convention. No repository runtime code or dependency was modified.

Native Electron visual/interaction acceptance was not run. The feature is in the shared Web/Desktop view and consumer typechecks passed. No backend code changed; Go tests were not rerun for this UI-only change. Existing pnpm config and jsdom fixture notices did not fail any check.
