# Verification record

## Approval and boundary

- User approved the full implementation plan after deployment location and visual reuse were clarified.
- Task activated on 2026-10-13. Implementation split into publication, static page, and deployment/docs ownership using native Trellis implement agents; a separate check pass follows integration.
- Target product files were clean at start. The shared working tree contains unrelated changes; they are outside this task's commit and verification scope.
- Existing Next.js `/download` is presentation reference only. No change to the live Web route, production installer storage, authentication or workspace behavior is authorized by this implementation.

## Available verification environment

- Node v22.23.2; pnpm 10.28.2.
- Docker 29.2.1 with the repository-pinned Nginx image already loaded for amd64 and arm64.
- Playwright Chromium 145.0.7632.6 launched successfully in an isolated headless browser.
- Initial fixtures were prepared under `/tmp/multica-download-qa.f7hTUp`. Docker Desktop could not bind that path, so the isolated live store was moved to ignored `data/desktop-download-qa-f7htup/` under the shared checkout.
- QA Compose project: `multica-download-qa-f7htup`; loopback port 53807. This was independent of the existing installer service.

## Automated checks

- `pnpm --filter @multica/desktop exec vitest run scripts/update-artifacts.test.mjs scripts/verify-updates.test.mjs scripts/download-page.test.mjs`: 117 passed across 3 files.
- `node --test scripts/desktop-updates.test.mjs scripts/offline-build-changelog.test.mjs`: 49 passed, 0 failed/skipped.
- Total: 166 passing tests. Fixtures contain synthetic bytes; they are never executed.
- `pnpm --filter @multica/desktop typecheck`: both node/web configurations passed.
- `pnpm --filter @multica/desktop lint`: exit 0, no errors; pre-existing exhaustive-deps warning in `src/renderer/src/components/tab-content.tsx:54` remains outside this task.
- Inline HTML JavaScript parsed with `vm.Script` and passed existing ESLint rules with browser globals explicitly supplied. The initial lint invocation lacked that execution environment and reported browser globals as undefined; no code suppression was added.
- Shell syntax, targeted file syntax and diff whitespace checks passed in implementation lanes; final scoped whitespace verification follows the documentation update.
- Independent `trellis-check` reported lint/typecheck success, then its provider returned quota exhaustion. The primary agent completed the remaining tests, boundary review, browser checks and evidence collection directly.

## Nginx / browser integration

- HTML returned 200 with `text/html`; bare `/desktop` returned relative `308 /desktop/` without losing the public port.
- A real Chromium download against loopback Nginx matched the synthetic source SHA-512 and byte count.
- Publishing 1.0.0 then 1.0.1 and refreshing the already-open page updated the visible version and link; configuration bytes and old installers were preserved.
- Malformed JSON disabled links, retained the installation guide and independent business URL, and recovered through the retry button.
- Public-network requests were intercepted/denied: zero attempted external requests and zero page errors in the successful integration run.
- HTTP insecure-context copy worked through real legacy copy capability; a simulated copy denial retained selected text with honest manual-copy feedback.
- The reserved HTTP test hostname was not directly reachable in the browser environment (502). Insecure-context checks therefore relayed actual Nginx responses through Playwright interception. Native browser file-download evidence was collected separately against direct loopback Nginx, because a relayed download was canceled by the browser.
- Keyboard FAQ interaction, 390px viewport and a long server address passed with no horizontal overflow.
- The repository HTTP verifier checked full SHA-512, HEAD 200 and Range 206 against version 1.0.1. Unreferenced blockmap transport was verified without claiming a source checksum match.
- After restarting the isolated container, the HTML, version 1.0.1 and administrator URL remained available.
- Raw JSON evidence is in `evidence/`; screenshots are in ignored `dist/desktop-download-qa/`.

## Visual review

- Desktop and mobile screenshots preserve the approved dark serif hero and light row-based platform matrix. Visual verdict: 94/100, pass; no discretionary polish iteration was needed.
- Mechanical detection produced four warnings, manually checked against actual computed styles: the muted hero text is on `rgb(5, 7, 11)`, not white; headings are 80px/41.6px/16px versus 14px body, not uniformly 16px; FAQ summaries have 18px vertical padding; the 24-vertex clip-path is the incumbent geometric brand star, not an invented organic silhouette.
- These are detector false positives for this document, not ignored accessibility defects. Computed-style evidence is preserved alongside the detector output.

## Delivery and limitations

- Built `dist/desktop-download-qa/multica-desktop-download-service.tar.gz` (10 files, 46,998 bytes), verified every archive member against staged/current source, and wrote a SHA-256 file.
- The deployment archive includes the page, Nginx configuration, lifecycle script, example config and guides. It excludes Docker images, desktop installers, synthetic fixtures and Node publisher dependencies.
- Actual Windows installation, signup/workspace interaction inside a native Windows client, and production intranet deployment were not executed. Guide wording was checked against current registration/onboarding source and DOM content; these do not constitute native acceptance.

## Source control

- Implementation commit: `35cc68ca288cddd67b4a25c6b7d4b6c84fa3c7ad`. The service archive is pinned to this commit, including the pre-existing authentication guide, so it does not incorporate uncommitted parallel release work.
