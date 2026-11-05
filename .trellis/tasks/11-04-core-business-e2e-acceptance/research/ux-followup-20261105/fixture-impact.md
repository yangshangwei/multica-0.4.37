# Research: UX follow-up fixture impact

- Query: Map product commit `5a7440952` to the smallest existing real-API human, administration, Web, native Desktop and installed-client acceptance journeys; identify fixture incompatibilities and evidence that need not be repeated.
- Scope: internal; static support audit only. Reads of source, specs, archived evidence and parent-supplied revision/dirty-tree snapshots. No test, build, browser, database, service or git operation was executed.
- Date: 2026-11-05
- Ownership: this file only. Campaign owner `01a1a731-b1e5-7b11-b83e-55efca5293d6` owns the original campaign and its dirty fixtures. Support owner `01a1a7a6-02dc-7722-a9d6-7d05385edb2d` owns this follow-up directory.

## Findings

### Decision and evidence boundary

Use focused existing collaboration journeys, with small additions only where an existing test does not exercise the changed behavior. The most concrete incompatibility is the newly duplicated **Attachments** region name; fix the fixture's intended scope before using that journey against the UX candidate. Do not weaken a product assertion or replace its original failure evidence.

The original `coverage.csv` entries sampled here all identify candidate `1a32ca36c2c4f4455a01bdd39d0e0ab4db20cccc`. At inspection, recipient/admin entries say PASS, detail entries BLOCKED, and installed-Main/retained-comment-file entries FAIL. These are historical campaign results, not results for `5a7440952`. The support inventory reports checkout candidate `2aa291e5d`; its `old_to_new_product_files` also includes separate server/agent knowledge changes. The UX follow-up is bounded by `ux-product.patch`, not that larger old-to-new list.

The audit's `evidence/source-provenance.json` records an earlier `head` (`b66d0ef8…`), the production build ID and 31 source hashes, plus `verified_work_commit: 5a744095279eec613a4c1b5b7d208f9932d47633`. Preserve those actual fields; do not relabel its original build as a later checkout build. Its accepted source-bound evidence is useful without claiming a new runtime execution.

### Files found

| File | Relevance |
|---|---|
| `research/ux-followup-20261105/scope.md` | Parent's authorization, ownership and no-runtime boundary. |
| `research/ux-followup-20261105/ux-product.patch` | Parent-captured UX revision diff: 28 Views implementation/test/locale paths in this supplied patch. |
| `research/ux-followup-20261105/inventory.json`, `existing-e2e-changes.patch` | Parent-captured candidate and ten pre-existing dirty E2E paths, owned by the core campaign. |
| `coverage.csv`, `conditional-matrix.csv` | Existing canonical test inventory and lane classification; read only. |
| `.trellis/tasks/archive/2026-11/11-05-collaboration-ux-audit/acceptance.md` | Accepted UX changes and explicit evidence limitations. |
| Same archive: `research/final-review.md`, `evidence/source-provenance.json` | Independent review and source/build/check identity. |
| Same archive: `evidence/after/browser-results.json`, `evidence/after-admin/admin-results.json`, `evidence/recipient-browser.json` | Actual browser check records; distinguish real API, injected fault and mocked admin projection. |
| `e2e/collaboration-recipient-picker-web.spec.ts` | Real self/recent/search discovery fixture; currently stops before distinct-account selection/submission. |
| `e2e/collaboration-detail-journey.spec.ts`, `e2e/fixtures/collaboration-detail-journey.ts` | Canonical manual cross-account delivery lifecycle in production Web and Electron, with strict provenance. |
| `e2e/collaboration-administration-{web,desktop}.spec.ts`, `e2e/fixtures/collaboration-administration.ts` | Real password/super-admin metadata and succession controls. |
| `e2e/collaboration-memory-web.spec.ts`, `e2e/fixtures/collaboration-memory.ts` | Human memory retry and exact retained write identity. |
| `e2e/collaboration-delivery-web.spec.ts`, `e2e/collaboration-retained-attachment.spec.ts` | Real file delivery/preview/revocation and broader native retained-file coverage. |
| `e2e/collaboration-knowledge.spec.ts`, `e2e/fixtures/collaboration-knowledge.ts` | Human knowledge proposal/review; direct source destinations currently dominate these journeys. |
| `e2e/collaboration-installed-main.spec.ts`, `e2e/fixtures/collaboration-installed-main.{ts,mjs}` | Packaged Linux full Main plus production Web; separate readiness and owned harness patches. |
| `e2e/fixtures/collaboration-recipient-picker.mjs`, `collaboration-disclosures.ts`, `collaboration-native.ts`, `lane-guards.ts` | Existing exact-account selection, disclosure scoping, native launch and environment guards. |

Bare `research/…` and ledger paths in this report are relative to `.trellis/tasks/11-04-core-business-e2e-acceptance/`; other paths are repository-relative.

### Minimal canonical execution mapping

The following is an execution handoff, **not permission to use the active campaign's environment**. Eight existing test identities cover the focused Web/human/admin and native manual/admin surfaces: R1 (one), D1 (two), M1 (one), F1 (one), K1 (one), A1 (two). Run only after the coordinator supplies an isolated allocation or releases the appropriate window. Native knowledge/files and installed full Main are explicit conditional additions below; their absence must remain visible in the result scope.

| ID / existing test identity | Fixtures and lane | Smallest useful assertions for this diff |
|---|---|---|
| **R1** — `e2e/collaboration-recipient-picker-web.spec.ts:19`: **Web recipient picker exposes self/recent/search results and preserves the outer dialog on Escape** | Legacy collaboration; real source owner + nonmember recent collaborator, source workspace and persisted collaboration history already created at lines 28–53. | Preserve self/recent browse, exact minimal account projection, Escape and narrow popup. Add one delayed search for the *distinct* collaborator: rapid Enter cannot commit the prior/self row; after the current response settles, select the exact account UUID and perform one explicit creation, checking the real handler UUID and absence of accidental creation. The current test only searches and closes; rerunning it unchanged does not close this gap. Exhaustive pointer/IME/query matrices stay in component tests. |
| **D1-Web** — `e2e/collaboration-detail-journey.spec.ts:37`: **Production Web manual detail: information, delivery, revision, resubmission and exact acceptance** | `legacy-manual-detail-permit`; separate creator/source owner and external handler, shared issue plus self-completion issue; `detailJourneyProvenance()` and `seedDetailJourney` fixture. | Preserve all request/receipt/epoch/submission/no-AI assertions. In `openJourneyAction`, assert the named form heading receives focus and is revealed **before** choosing status. At the existing comment-preparation step, retain text/empty basis/no-write assertions; one close/reopen verifies return to the actual opener. Existing explicit `basis.focus()` is valid after heading focus settles. |
| **D1-Desktop** — same spec: **Electron Desktop manual detail: information, delivery, revision, resubmission and exact acceptance** | Same manual gate and provenance; actual Electron harness using built renderer/preload and separate actor profiles. | Same human business steps, with native form focus/scroll capture and EN/ZH result presentation. This is native renderer/platform evidence; it is not installed full Main. Fold header readability/accessibility observations into existing captures rather than adding an unrelated shell suite. |
| **M1** — `e2e/collaboration-memory-web.spec.ts:143`: **shared context recovers a lost save response with its original request after reload** | `seedMemoryIssue()` supports legacy or password actors; use the isolated manual legacy allocation to avoid another lane. | Retain the exact original-key/body retry and final version assertion. The existing fault fulfills 599 **without forwarding the first PUT**; it proves explicit retry after uncertain transport, not a committed-first-write recovery. Add an initial/after-reload GET failure only if real failure integration is required: named memory region and original recovery remain reachable; no automatic PUT; explicit refresh/retry succeeds against the real API. Do not manually open ancestors when testing automatic attention. |
| **F1** — `e2e/collaboration-delivery-web.spec.ts:23`: **Web creates one private issue after lost upload/create responses and revokes an open preview through account WS** | Legacy collaboration; source owner + external recipient, real private upload/creation/comment files, real WS revocation. | Scope the issue-level file panel as described below. Preserve byte/range/download/authority assertions. A single held/failed attachment-metadata GET followed by explicit retry can verify new referenced-file feedback and restored actual file access; privacy/revocation matrices stay canonical in Core/component tests. |
| **K1** — `e2e/collaboration-knowledge.spec.ts:89`: **Web external comment proposal, exact human review, correction and workspace derivation** | Legacy human-only knowledge fixture; real owner, external handler, source project/workspace and source comment/memory. Suite gates creation on and execution/host off. | Preserve the external handler's no-private-directory assertions. Add the changed **Another visible destination** path under the owner, with an actually visible destination workspace/project: preserve summary/content/source selection through one failed directory read, explicit retry and exact project proposal/review. The current direct-source shortcut path does not exercise `OtherTarget`; an unchanged replay cannot claim destination-failure coverage. Do not retarget an unavailable project to its workspace. |
| **A1-Web** — `e2e/collaboration-administration-web.spec.ts:61`: **Web administers disabled-creator collaborations with explicit succession and original receipt recovery** | Independent `password-admin` lane; injected synthetic super-admin plus creator/handler/successor; real admin projection/API, no provider. | Add factual localized status assertions to list and detail for EN and zh-Hans, keyed to actual API status. Keep disabled-creator eligibility, ordinary-content refusal, real receipt and successor assertions. The existing fixture is English-only; do locale observations before restoring English for its existing control labels. |
| **A1-Desktop** — `e2e/collaboration-administration-desktop.spec.ts:11`: **Desktop administers disabled-creator collaborations in the account shell and recovers the retained receipt** | Same password-admin contract; native renderer/preload, own profile and account administration route. | Same localized metadata check in the native shell; preserve admin-only projection and complete/cancel/reopen/recovery assertions. Keep each independent spec's synthetic admin credentials separate to avoid exhausting real reauthentication limits. |

No general `core-business-*`, all-Views, whole-repository, updater, daemon-runtime or real-agent rerun follows from this UX-only diff. The archived 312-test run already covers the changed shared component matrices.

### Conditional native and installed coverage

| Existing identity | When it earns a place | Boundaries |
|---|---|---|
| `e2e/collaboration-knowledge.spec.ts:285`: **Desktop native source review and independent project/workspace contributions** | If the follow-up explicitly needs a native `OtherTarget` failure/retry observation, adapt that owner segment instead of replaying every knowledge case. | Existing direct-source project/workspace flow alone does not test the changed directory branch. Requires legacy lane plus explicit built renderer/source root; its suite changes/restores the deployment gate. No Provider or daemon execution. |
| `e2e/collaboration-retained-attachment.spec.ts:156`: **Web and Electron retain exact comment file bytes and immutable AI input after display removal, deletion, refresh and cross-client restart** | If native file/preview continuity is required beyond F1 and the already accepted component/privacy regressions. | Password collaboration/native lane. This fixture includes simulated/idle AI command state, not paid-model execution. The core owner already has a dirty helper and historical failure; coordinate its selected frozen harness before any replay. Avoid running both typed-creation variants solely to recheck the comment attachment change. |
| `e2e/collaboration-installed-main.spec.ts:107`: **installed Linux full Main and production Web hand off the same issue, preserve authority/history and reject unconfirmed Resume** | Only when a freshly identified package containing the UX source and an isolated packaged lane are available. This is the canonical installed handoff/workflow smoke, not `collaboration-installed-clients.spec.ts`'s cached CLI compatibility case. | Requires `COLLABORATION_INSTALLED_IMAGE`, `COLLABORATION_INSTALLED_APP_DIR` or `COLLABORATION_INSTALLED_APP_VOLUME`, explicit API/browser API/database/deployment ID, and `E2E_PASSWORD_AUTH=1`. Preserve full Main/profile/install identity, handoff, receipt/history, Stop/Resume/authority assertions. Add heading readiness before native entry if needed; exact-account helpers do not test the changed search path. Existing old package/campaign FAIL cannot be relabeled as new-candidate failure or pass. No installed execution is claimed by this report. |

### Gates and fixture incompatibilities

1. **Legacy manual and password administration are different allocations.** Recipient/knowledge fixtures call email-code `TestApiClient.login`; the detail provenance function explicitly requires `config.auth_mode === "legacy"` (`e2e/fixtures/collaboration-detail-journey.ts:185`). Administration requires real password auth, `E2E_PLATFORM_ADMIN=1`, `E2E_PASSWORD_AUTH=1`, `E2E_PLATFORM_ADMIN_USERNAME/PASSWORD`, explicit loopback API/database/browser API and a non-default database (`collaboration-administration.ts:33`). Do not switch the original campaign API's auth mode to combine them.
2. **Detail provenance cannot be transplanted.** Required inputs include absolute `COLLABORATION_DETAIL_PROVENANCE_FILE`, absolute `COLLABORATION_DETAIL_ALLOCATION_FILE`, `COLLABORATION_DETAIL_TASK`, `COLLABORATION_ELECTRON_SOURCE_ROOT`, renderer URL, API/browser API/Web origin, database and `MULTICA_DEV_VERIFICATION_CODE`. Task is constrained to `.trellis/tasks/<single-name>`: the nested `research/ux-followup-20261105` directory is not a valid task identifier (`collaboration-detail-journey.ts:112`). Use the canonical task identity with separately owned allocation/provenance files. Checks bind allocation hash, source hashes, current commit, listener PIDs, production build, renderer/preload hashes, real config and database. Gate must be `creation_enabled=true`, `enabled=false`, `host_execution_enabled=false` (lines 195–203). The owner-added task/allocation flexibility remains intact; do not bypass any check to reuse `1a32ca36c` evidence.
3. **Selected specs can write global fixture gates.** `collaboration-knowledge.spec.ts:18` runs `setCollaborationGate` in `beforeAll` and restores later; `lane-guards.ts:116` documents the deployment-wide mutation. Serial execution in a coordinator-owned lane is necessary even when the test body is human-only. A grep filter does not suppress these suite hooks.
4. **Native Electron is not installed Main.** `collaboration-native.ts:28` launches `collaboration-account-electron.mjs`; its actual renderer/preload/IPC evidence is valid for that harness, not packaged startup, installation or cross-OS behavior. The installed-Main case has separate package/profile/deployment requirements and existing owner changes.
5. **Shared API constants and authentication must match.** Production Web must use the same API origin as `COLLABORATION_E2E_API_URL`, with CORS/WS admitting its exact renderer origin. Password browser writes require both real login cookies, including CSRF. Do not route password actors through legacy localStorage-only authentication or log credentials in traces.

### Concrete selector adaptation

`CollaborationCommentAttachments` previously returned the attachment list directly. The UX commit wraps it in `<section aria-label={attachments.title}>` (`packages/views/collaboration/attachment-panel.tsx:135`). The issue-level panel already has that same region name (line 89). Comment, comment-history and command-history renderers reuse the component (`comment-list.tsx:131`, `comment-history.tsx:25`, `command-history.tsx:69`).

**Definite static conflict:** `e2e/collaboration-delivery-web.spec.ts:149` selects every region named Attachments immediately after creating a comment file. Its nested `Preview comment-preview.png` and `Download comment-preview.png` locators (lines 150 and 159) can match both the live comment and the opened issue panel. The server result remains valid; this is an ambiguous fixture selector, not proof of a product failure. No browser reproduction was attempted in this support lane.

The narrow fix belongs to the fixture owner: retain the `details` returned by the existing discovery helper and locate the panel inside it:

```ts
const disclosure = await revealCollaborationDisclosure(page, "Attachments");
const attachments = disclosure.getByRole("region", { name: "Attachments", exact: true });
```

`revealCollaborationDisclosure()` already returns that exact enclosing details element (`e2e/fixtures/collaboration-disclosures.ts:7`). Alternatively filter the region by its own Attachments heading; comment attachment sections do not carry that heading. Avoid `.first()` because it can silently exercise the wrong surface. Comment/history assertions should keep their existing comment ID/event/command container.

The same broad locator exists in `collaboration-retained-attachments.ts:97`, `:169`, `:355`, and `collaboration-account-desktop.spec.ts:360`, `:388`. It is a **conditional** collision in those cases: the typed-creation scenario has no comment references, and retained-file steps deliberately close comment/command history before some issue-panel operations. Do not invent a failure for those rows. Reuse the scoped helper when the owner makes the selected follow-up fixture adjustment.

### Other assertion adjustments and gaps

- **Recipient search:** use the actual accessible textbox name **Filter options**, in the popover containing listbox **Recipient** (`collaboration-recipient-picker-web.spec.ts:72`, `:91`). For no-match/loading states, assert the popup and status; an empty listbox can have zero height. Wait for current-query settlement before choosing an enabled exact-ID row. Existing `selectCollaborationRecipient` (`e2e/fixtures/collaboration-recipient-picker.mjs:4`) intentionally uses the separate full-identity lookup/Use this account path. It remains compatible, but cannot prove search debounce behavior. Preserve the selected-account sentence, exact IDs and Escape semantics.
- **Workflow focus:** `workflow.tsx:128` now uses `useOpenedForm({ opening, reveal: true })` on its heading (`:250`); `focus.ts:32` reveals ancestors and focuses once per explicit opening identity. Existing `openJourneyAction` (`e2e/fixtures/collaboration-detail-journey.ts:442`) checks form visibility then selects Status, but does not assert the new initial focus. Insert any heading focus/visible-in-scroll-root assertion *before* `selectOption`, which can move focus itself. Do not change the later explicit basis focus assertion or require focus to remain on the opener after opening. Editing/refetch must not refocus; component tests already own that matrix.
- **Memory:** failed initial reads must retain a named region and an explicit retry; successful empty content is a different state. Existing `#memory` locators remain correct. Tests of retained-request attention must not call `revealCollaborationDisclosure` to manufacture the behavior being tested. Original-key recovery remains independent of whether current memory loaded.
- **File failures:** distinguish zero references (no metadata query/UI), pending read, failed read and partially missing IDs. Derive previews from current files (`attachment-panel.tsx:30`); do not expect an old preview to remain after its reference disappears or protected data is withdrawn. The full metadata/loading/denial matrix belongs in `attachment-panel.test.tsx`.
- **Destination failures:** `knowledge-proposal.tsx:28` only publishes an effective project while successful current directory reads contain its workspace/project; line 103 rechecks target identity after async source preparation. Existing direct-source shortcuts are intentionally independent of these directory reads. A simple rerun of those shortcuts is not a new OtherTarget claim.
- **Notifications/header:** `notifications.tsx:26` preserves the accessible name **Notifications N unread** through an explicit `aria-label`; only its narrow visible text changes to icon/count. Existing role selectors using `/^Notifications/` remain valid. Do not replace them with text-only searches or assert that the mobile h1 is visually expanded (`account-workbench-page.tsx:42`). No broad notification/host replay is needed for this presentation-only delta.
- **Workbench counts:** `account-workbench-page.tsx:95` distinguishes unknown, loaded/total and local match count; search applies to loaded pages. Existing UI count assertions should compare the appropriate loaded and total values, never infer that search queried the whole server collection. The exact two-page/clear/selected-detail matrix already exists in `account-workbench-page.test.tsx:58`; no new large browser fixture is justified solely for that matrix.
- **Locales/admin:** activity, comment/history and command/cancellation dates now use app locale while retaining ISO `dateTime`. Keep timestamp matrices in the named component suites. `admin/collaboration/collaboration-page.tsx:44` uses existing collaboration status labels; assert **Done / 已完成**, **In progress / 进行中**, etc. from the actual API status, not raw keys such as `done`. Unknown status values retain fallback. The real admin fixture currently asserts operations and metadata authority, not bilingual status display.

### Existing evidence sufficient without repetition

The archived final review and `acceptance.md` accept 30 test files / 312 tests, Views/Web/Desktop typechecks, zero lint errors, a production Web build, and the scoped static detector. This audit inspected their artifact records; it did not rerun those checks.

| Changed behavior | Accepted canonical evidence | Limit for this follow-up |
|---|---|---|
| Recipient stale selection/filtering | `account-picker.test.tsx:140` keyboard/pointer delay cases and `:179` pinned-account filtering; `evidence/recipient-browser.json` records rapid no-match Enter plus settled exact-self-ID selection with no page errors. | The final recipient JSON/addendum supersedes the earlier supplement's locator timeout. It still does not establish distinct-account search-to-write behavior. |
| Workflow reveal/focus and byte feedback | `workflow.test.tsx:29`, byte-limit cases and `focus.test.tsx`; all four production browser variants assert heading focus, prepared-result no-write and close focus return. | Native or installed behavior remains a distinct scope. No need to duplicate all UTF-8 boundaries in E2E. |
| Memory read failure / retained recovery | `memory-panel.test.tsx:46`, `:74` and retained-operation cases; production browser explicitly injects memory failure and retries in all four variants. | These are UI/injected-read checks, separate from real server retained-write receipt behavior. |
| Referenced-file failures / current preview | `attachment-panel.test.tsx:29`, `:58` and missing-reference/denial cases; touch supplement covers controlled preview-close geometry. | File bytes/WS/native assertions remain in F1 or the conditional retained-file journey, with the selector scope corrected. |
| Other destination failure/removal | `knowledge-panel.test.tsx:193`, `:225`, plus pending/empty workspace cases. | No existing browser record proves the real normal-directory failure branch; K1's owner extension is the focused integration addition. |
| Counts/header/locales | Workbench page tests, notification tests, command/comment locale tests, and 54 production browser captures across 1440×1000 EN/light, 1024×768 EN/dark, 390×740 ZH/dark/coarse and 320×568 EN/light/coarse. | The records report 11 check groups, zero page errors/overflow. Root font size 200% is not native browser zoom or a physical-device test. |
| Admin status | Bilingual component list/detail tests and four browser admin list/detail checks. | `evidence/after-admin/admin-results.json` explicitly has mocked admin projections with real authentication; A1 is needed for real privileged metadata behavior. |

### Preserve the other owner's fixture work

The parent supplied the following current dirty paths in `inventory.json` and their captured diff in `existing-e2e-changes.patch`. They are not this UX task's product changes:

- `e2e/collaboration-host-installed.spec.ts`
- `e2e/collaboration-installed-main.spec.ts`
- `e2e/collaboration-leave-web.spec.ts`
- `e2e/collaboration-terminal-reopen-web.spec.ts`
- `e2e/fixtures/collaboration-control-gaps.ts`
- `e2e/fixtures/collaboration-detail-journey.ts`
- `e2e/fixtures/collaboration-installed-main.mjs`
- `e2e/fixtures/collaboration-lane-close-controls.ts`
- `e2e/fixtures/collaboration-offline-result-notifications.ts`
- `e2e/fixtures/collaboration-retained-attachments.ts`

The captured changes include dynamic detail task/allocation provenance, formal-result basis/submission assertions, disclosure discovery and packaged Main/managed-restart handling. Preserve their source bytes, business assertions and original failure artifacts. Any later adaptation above must be handed to the owner or applied in an explicitly separate frozen harness, with its new hash recorded. This researcher changed none of them and did not update task-wide progress, coverage or result ledgers.

### Related specs and versions

- `CLAUDE.md`, Testing and Verification: one canonical layer per behavior; E2E setup/teardown through TestApiClient; shared Views tests and platform wiring kept distinct.
- `.trellis/workflow.md:363`: persistent research artifacts; this agent's role remains research-only.
- `.trellis/spec/views/frontend/collaboration-creation.md`: current-query ownership, IME/exact identity and stale commitment rules.
- `.trellis/spec/views/frontend/collaboration-detail.md`: named memory/file recovery, all enclosing disclosures, intentional workflow focus, loaded-row counts and localized dates.
- `.trellis/spec/views/frontend/collaboration-knowledge.md:187`: OtherTarget directory states, preserved IDs/text and no implicit workspace fallback.
- `.trellis/spec/core/frontend/collaboration-workbench.md`: protected Query ownership, scoped original intent and exact delivery authority.
- `.trellis/spec/web/frontend/e2e-run-environment.md`: production Web only, exact origins/provenance, distinct legacy/password lanes, real CSRF login and honest evidence scopes.
- `.trellis/spec/desktop/frontend/account-navigation.md`: actual Electron/profile/routing evidence; native service stubs do not prove packaged full Main.
- External references: none needed for this internal delta; no external behavior or latest-version claim was used. Local declared tool versions are `@playwright/test ^1.58.2` (`package.json:61`), React `19.2.3` and Vitest `^4.1.0` (`pnpm-workspace.yaml:30`, `:41`), and Next `^16.2.5` (`apps/web/package.json:25`). These are repository declarations, not an installed-version probe.

## Caveats / Not Found

- No new runtime, test pass, real admin action, installed package acceptance or real-model result is established by this file. Paid model lanes remain deferred.
- The Attachments selector collision is source-derived and concrete for the described delivery fixture; it was not executed here. Other broad attachment locators have conditional exposure and are not automatically failures.
- The current recipient E2E has real distinct actors but does not select/create through search; the current knowledge E2E uses mostly direct-source destinations; the current admin fixtures use English control labels. Unchanged reruns cannot certify the new assertions described above.
- Source/commit ownership was taken from the parent-captured diff/inventory because the researcher role prohibits git operations. The original campaign is active, so ledger statuses and dirty files may advance after this snapshot; a new run must use its own frozen input identities.
- No source file, test file, existing evidence, spec, campaign ledger, environment or task root was written. Only this research file was added.
