# v10 category timeouts: read-only diagnosis

Captured 2026-11-02T21:11:41.876040+00:00. Candidate: `118fc53ea8eb8ebc53b4f9c1131e3e7910eacce9` at `/Volumes/artisan/code/2026/mc-collaboration-e2e-repair-20261103`. This report diagnoses the retained failures and grants no execution or acceptance clearance. No source, dependency, build, database, service or Git state was changed.

## Findings (fixed)

None. This dispatch permits diagnostic JSON/Markdown only.

## Findings (not fixed)

**CAT-V10-1 — production SSR registers and reads different auth-store modules.** The retained Web log repeats `Auth store not initialised — call registerAuthStore() first`, digest `3602458576`. Its stack is `Object.apply (.next/server/chunks/6491.js:5:32969)` → `g (.next/server/chunks/6491.js:7:75189)`. The caller is `AdminSessionBoundary`; the throw maps to `packages/core/auth/index.ts:35`. `apps/web/components/web-providers.tsx:115` mounts this boundary above all route content, explaining category, Agent and login-page failures together.

| Build | Root provider module | Registered auth module | Boundary module | Boundary auth module |
| --- | --- | --- | --- | --- |
| v9, `4288.js` | 37702 | 48286 | 60892 | 48286 |
| v10, `6491.js` | 5429 | 99423 | 19653 | 10100 |

The v10 WebProviders SSR entry81904 uses auth99423 and CoreProvider5429. Auth99423 retains its registration setter and a hook reading that registered instance. Boundary19653 reads10100, whose proxy was compiled to throw unconditionally with no retained setter. In v9 both paths read48286. This is the proven direct cause; no speculative source patch is proposed.

The v10 `.next/trace` records these **SSR** source edges:

- WebProviders in the frozen v10 checkout → `final4/packages/core/auth/index.ts` (span3185).
- WebProviders → `final4/packages/core/platform/core-provider.tsx` through its platform index (span3639).
- `final4/packages/views/admin/admin-session-boundary.tsx` → `final3/packages/core/auth/index.ts` (span3278).

Both auth paths also occur in the browser layer. The manual-route client-reference manifest binds the boundary under final4 and SSR id19653. Source paths are absolute in the JSON. These observations prove the mixed module identity, while the exact resolver/cache step that selected the two roots remains unisolated.

**Remaining mechanism hypothesis:** v10 borrows whole workspace node_modules directories via final4, which already borrows them from final3. v9 borrows directly from final3. Physical realpaths ultimately point to final3 in both cases, and inspected Next16.2.6, React19.2.3 and Playwright1.58.2 package bytes match. The intermediate workspace identity is therefore the targeted resolution hypothesis. The compiler trace and generated graph support it; no controlled rebuild or reproduction was authorized in this pass.

## Failure location and observed behavior

| Case | First navigation | Immediate control | v9 result |
| --- | --- | --- | --- |
| `agent-categories.spec.ts:10` | line35: `/<workspace>/agents/new/manual` | line36: click `展开分类列表` | pass, 8.042s, retry0 |
| `agent-category-grouping.spec.ts:23` | line57: `/<workspace>/agents/<id>?view=general` | line58: click `展开分类列表` | pass, 4.958s, retry0 |

Both v10 error contexts contain only the root **Something went wrong**, explanatory text and **Reload** button. Neither failure directory contains a trace ZIP, video or screenshot. The config has no trace/video/screenshot option and retries0. The live line reporter contains only the180000ms whole-test deadline, at runner-log lines114176 and114192, without an action stack. The exact awaiting locator is therefore **unrecorded**. The evidence bounds progress to the initial browser entry/dropdown action; it does not establish that any category control ran. No category screenshots or category mutation requests were retained. Both category spec files, fixtures and inspected boot-chain sources are byte-identical to v9.

The API log confirms fixture login/setup succeeds. Both browser starts receive fast200 responses for `/api/config` and `/api/me` (lines176–177 and219–220). Grouping setup creates its three Agents with201 before navigation. Production Web is build `XBQ1-kYxShqvrub7FJsc4` on13549, recorded commit118fc53ea. Its built rewrites and private check configuration both target API18629. The saved evidence shows neither an upstream port mismatch nor a login/config request failure explaining this root screen.

At the test deadlines, workspace cleanup returns500 with `context canceled` (API lines196–197 and237–238; ~31ms and30ms). Preserve those secondary cleanup failures and the failed database. They do not explain the initial auth-store exception, and this report does not claim cleanup succeeded. Test-local pageerror arrays are checked only at the end and not attached on failure; React's root boundary may catch errors before window pageerror.

## Recommended later reproduction and corrective scope

After the live439 run stops and the sole owner provides the next exclusive window, preserve the failed artifacts and prepare task-owned workspace dependency directories. Link every `@multica` workspace import to the same candidate packages while reusing the pinned external package cache. Keep the borrowed final3/final4 directories intact.

Build production Web once with a fresh owned output/cache, retain its compiler source identities and compare auth registration/accessor ids. A minimal `/login` SSR/browser navigation is the first discriminating reproduction; the two unchanged category cases can follow under the owner's schedule. Keep the auth guard, product assertions,180000ms timeout and zero retries. Whether that layout change closes the resolver mechanism remains to be verified. No application patch, test run, build, service change or clearance is authorized by this diagnostic report.

After validation, consider syncing the E2E environment/preflight spec with the invariant that borrowed node_modules must not introduce another workspace module identity. Candidate HEAD/source fingerprints alone do not inspect that runtime graph.

## Verification

- Read-only artifact/source/compiled graph inspection: completed.
- Lint: not run; this dispatch prohibits execution.
- TypeCheck: not run; this dispatch prohibits execution.
- Tests/reproduction/build: not run; the live suite remains exclusively owned by `01a19c0e-5cc4-77e0-b2d3-bd2842d39899`.
- Files written: this Markdown and adjacent diagnostic JSON only. No code simplifications or fixes.

The JSON contains exact paths, compiler span ids, source/dependency comparisons and sanitized HTTP facts. Each input artifact has a SHA256 and captured byte length. Mutable logs/trace are hashed as observed prefixes; later appends can be verified by hashing that captured prefix. No private raw logs, credentials or Provider prompts were copied.
