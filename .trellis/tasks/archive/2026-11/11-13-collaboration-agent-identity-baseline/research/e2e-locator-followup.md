# B05 author badge locator repairs

2026-11-13. Scope: `e2e/collaboration-multi-account-closure.spec.ts` and this note. No product, shared fixture, validation checkout, environment, database or commit changes.

## Retained failure and diagnosis

The coordinator's actual B05 run 1 report remains at `/Users/artisan/.multica/acceptance/s1-agent-identity-20261113-01/b05-run1.json`. Its first `checkAgentAuthor` call failed at line 173 because `comment.getByText("Agent", { exact: true })` resolved to two spans: the new author badge and the existing permission-context author role. The exact API `author_agent` assertion was reached and passed before this locator failure; this failure does not establish a product projection defect.

Current `comment-list.tsx` places the author name, badge and timestamp in a direct child header div. The permission-context role is inside a sibling `details` element. The initial repair shared `commentAuthorHeader(comment)`, selecting `:scope > div:has(> time)`, for:

- The exact Agent name and visible badge assertions.
- The long-name author Range measurement.
- The existing Web and Electron human-name/no-Agent-badge assertions.

The reply badge remains scoped to the exact reply hint's parent container. No first/nth selector was introduced. The exact `Edit comment` / `Delete comment` assertions, three participants, three Agents, five controlled runs with 1/2/2 counts, native checks, long-name geometry limits, delivery decisions and cleanup assertions remain intact.

## Checks actually run

All commands ran in `/Volumes/artisan/code/2026/multica-0.4.37`.

- `pnpm exec tsc --noEmit --skipLibCheck --strict --esModuleInterop --module ESNext --moduleResolution bundler --target ESNext --jsx react-jsx --resolveJsonModule --allowImportingTsExtensions e2e/collaboration-multi-account-closure.spec.ts`: exit 0; only the existing pnpm manifest-location warning.
- `pnpm exec playwright test e2e/collaboration-multi-account-closure.spec.ts --list`: exit 0; exactly one Chromium test discovered. No test body executed.
- `git diff --check -- e2e/collaboration-multi-account-closure.spec.ts`: exit 0.

Root E2E files remain outside the workspace ESLint base paths. An ignored-file lint exit is not claimed as coverage. The coordinator owns the new numbered B05/B06 runtime execution and validation-source copy; static checks do not pass the retained run 1 or establish browser/native geometry success.

## Source identity

- Before this repair, spec SHA-256: `6395d75cf915a54329fc4676fb6f196e73d715f28ecba3e1fc8117faf4ec04c8`.
- After this repair, spec SHA-256: `dbe2d3674584016e7498906191781f5ab8966c0bdea2cb5bb32d604a353dd8d5`.

The structural locator depends on the current direct header timestamp. If that markup changes, update the header boundary while preserving the independent badge assertion; do not replace it with an arbitrary matching occurrence.

## Run 2 composition failure and verified XPath correction

The coordinator's actual B05 run 2 remains at `/Users/artisan/.multica/acceptance/s1-agent-identity-20261113-01/b05-run2.json`. It failed on the header's exact author name at line 174: Playwright's composed `:scope > div:has(> time)` selector found no elements. The earlier static checks did not exercise locator resolution, so they did not detect this test-harness defect. Run 2 is not a passed product journey.

The only subsequent spec change is:

```diff
-    const commentAuthorHeader = (comment: Locator) => comment.locator(":scope > div:has(> time)");
+    const commentAuthorHeader = (comment: Locator) => comment.locator("xpath=./div[time]");
```

Two local `node --input-type=module` probes used the installed `@playwright/test` Chromium (145.0.7632.6), with no application navigation or API/database access. Both used this DOM:

```html
<ul><li id="comment-check"><div><span>Named Agent</span><span>Agent</span><time>now</time></div><details><span>Agent</span></details></li></ul>
```

The first probe reproduced the original conditions: generic exact Agent count 2, composed CSS header count 0, simple direct-child div count 1. The second read the helper selector from the changed spec itself, required `xpath=./div[time]`, and asserted header count 1, exact name count 1, exact badge count 1, with both name and badge visible. The generic Agent count remained 2. Both probes exited 0 and closed their own browsers in `finally`.

After the one-line correction, the same strict TypeScript command passed again; Playwright discovery again reported exactly one Chromium test; `git diff --check` passed. Final spec SHA-256: `e2bbcda9340f930c20b49936b7f2b6d4a1a567d44314c101f11c50b5cb03db53`.

This proves locator resolution against the minimal markup. The coordinator still owns the new numbered full B05/B06 rerun, validation-source copy, native geometry and cleanup verification. Both original runtime failures remain retained.
