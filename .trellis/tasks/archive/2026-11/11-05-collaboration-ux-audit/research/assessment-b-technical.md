# Research: Collaboration technical assessment B

- Query: Audit every cross-workspace collaboration UI surface for accessibility, responsive behavior, implementation integrity, recovery, and meaningful regression coverage.
- Scope: internal; all production TSX in `packages/views/collaboration/` and `packages/views/admin/collaboration/`, their shared primitives and relevant Core queries. Source-workspace entry and account administration are included.
- Date: 2026-11-05
- Method: independent impeccable Assessment B, isolated from Assessment A. No Assessment A findings were read. Findings were withheld from the parent until the parent explicitly confirmed A was complete.
- Mode: Operate; preserve the incumbent design system and product contracts.

## Findings

### Implementation integrity verdict

**Pass with interaction/recovery gaps.** The shared account-owned workbench, protected issue identity, explicit human workflow, exact result acceptance, original-request recovery, and native shared navigation form a coherent product-specific system. This is not a candidate for aesthetic replacement. Local component decisions still leave gaps in search commitment, inline form focus, and auxiliary error recovery.

The CLI detector ran **once**, on `packages/views/collaboration` and `packages/views/admin/collaboration` together. It exited **0**, returning **0 findings**, raw JSON `[]`. There are no CLI rule names or file locations to enumerate. A clean deterministic scan does not prove functional accessibility.

The browser detector was successfully injected into three representative rendered states: desktop detail, narrow creation, and narrow list. Each reported one `OVERUSED FONT` warning for the incumbent Inter font. These are **false positives for this task**: the existing app font is intentional, shared across the product, and an Operate audit does not authorize replacing it. No overlay finding is an accepted defect.

### Audit health score

| Dimension | Score / 4 | Evidence and limit |
|---|---:|---|
| Accessibility | 2 | Strong labels, native disclosures, named groups and keyboard picker support; inline workflow focus and field-specific error feedback still need repair. |
| Performance | 3 | Paged records, lazy run/history bodies, debounced creation parent lookup; closed auxiliaries still mount polling readers. No timing/profile benchmark was run. |
| Theming | 3 | Semantic tokens throughout, no CLI color drift; light/dark narrow rendering inspected. Full painted focus-contrast matrix was not measured in this run. |
| Responsive design | 3 | No horizontal overflow in tested viewport/root geometry; local coarse-pointer omissions remain and were identified from source. Physical touch and 200% zoom were not run. |
| Implementation integrity | 3 | Coherent product contract; several local interaction/recovery failures despite clean detector. |
| **Total** | **14 / 20** | **Good; repair the weak interaction and recovery paths.** |

Eight accepted findings: **0 P0, 1 P1, 7 P2, 0 P3**. Some rely on complete source control flow rather than a live server error reproduction; evidence type is stated below.

### B1 — P1: Recipient search can commit an unrelated or stale account

- **Location:** `packages/views/collaboration/account-picker.tsx:86`, `:91`, `:103`, `:148`, `:196`; `packages/core/collaboration/recipient-sections.ts:18`; shared picker auto-highlight/Enter at `packages/views/issues/components/pickers/property-picker.tsx:183`, `:245`.
- **Category:** Accessibility / Implementation integrity / Error prevention.
- **Evidence:** `input` updates immediately but `query` updates 250 ms later. The Query uses `keepPreviousData`, and the list is always rendered without distinguishing the raw/debounced mismatch or placeholder result. A nonempty search flattens self, selected, recent, and matching results, although `collaborationRecipientSections` does not filter the pinned sections by the query. The generic picker highlights the first committed candidate after typing and Enter selects it. In creation, self is always first even for a search matching someone else. Old results also remain selectable while a new query is pending.
- **Impact:** A user typing a recipient and pressing Enter can choose self or an unrelated old account. For participant/transfer actions that is an identity and sharing error, not cosmetic uncertainty.
- **Recommendation:** Disable result commitment while raw input differs from the debounced query or data belongs to a previous query. Preserve authorized same-query rows on background errors, as existing tests require. Keep self/selected/recent browsing under clear groups, but ensure typed-result keyboard selection targets an actual match. Do not change exact lookup, access guards, IME, append-only paging, or full-ID labels.
- **Regression:** Actual primitives; type a query while an older list is visible, press Enter during debounce and delayed response, assert no selection. Then resolve the current response, press Enter and assert the intended user ID. Add a self-plus-different-search-match case and stale query/error case.
- **Suggested command:** `$impeccable harden`.
- **Evidence type:** Source-proven; no real invitation/transfer write was performed.

### B2 — P2: Opening a workflow form does not move focus to the form

- **Location:** `packages/views/collaboration/workflow.tsx:76`, `:98`, `:237`; result preparation `:67`; caller `packages/views/collaboration/collaboration-detail.tsx:120`.
- **Category:** Accessibility / Interaction.
- **Evidence:** Fresh live Chromium: focus Edit issue, press Enter; `document.activeElement` remains the Edit issue button outside the new form. Saved in `assessment-b-browser-evidence.json` as `workflowFocus`. Workflow form has no ref/focus effect or autofocus. It is mounted after all action controls, Participants, and notices. Preparing a result from a comment also mounts the result form near the page top without focus/scroll guidance.
- **Impact:** Keyboard/screen-reader users are left at the old control and must find the form; the comment-to-result path can open a form far above the current viewport without making that transition apparent.
- **Recommendation:** Reuse `useOpenedForm` from `packages/views/collaboration/focus.ts:28` or an equivalent local first-field focus pattern. Focus once on intentional open, preserve focus during edits/refetch, restore to an actual opener on dismissal. Capture the comment action as the opener for result preparation.
- **Regression:** Keyboard-open primary and More actions forms, assert focus in the labelled form; prepare a result from a lower comment and assert its form is scrolled into the real account-detail viewport. Closing restores the correct opener.
- **Suggested command:** `$impeccable harden`.

### B3 — P2: Initial shared-context failure becomes an empty disclosure

- **Location:** `packages/views/collaboration/memory-panel.tsx:139`–`:147`; enclosing disclosure at `packages/views/collaboration/collaboration-detail.tsx:130`.
- **Category:** Recovery / Implementation integrity.
- **Evidence:** Pending renders a loading panel. After an initial non-denial network/server failure, `memory` is absent and line 141 returns null. The `memoryQuery.isError` retry renderer at line 147 is reachable only if data already exists. Unsupported responses also return null.
- **Impact:** Opening Shared context after an initial failed read yields no content, no explanation, and no Retry action. A user cannot distinguish a read failure from an empty context.
- **Recommendation:** Keep the scoped panel heading and explicit failure/unavailable state. Offer the existing protected query's retry; preserve authoritative denial teardown and stop-polling behavior on unsupported servers.
- **Regression:** First read rejects, user sees scoped alert + Retry, retry succeeds without losing another open form. Separately cover unsupported and authoritative denial; do not restore revoked cached content.
- **Suggested command:** `$impeccable harden`.
- **Evidence type:** Source-proven; no fault injection performed in this baseline.

### B4 — P2: Oversized revision reason silently disables the action

- **Location:** `packages/views/collaboration/workflow.tsx:201`–`:209`, `:255`, `:259`–`:264`.
- **Category:** Accessibility / Form validation.
- **Evidence:** Return-for-revision sets `decisionReady=false` when its reason exceeds the 65,536-byte limit, but the note textarea has neither an associated error nor `aria-invalid` for that case. Submit simply disables. Result/basis do show a shared error but mark both fields invalid even if only one is too long, and the alert is not referenced by either field.
- **Impact:** The user cannot identify why a filled revision form cannot be submitted. Users writing non-ASCII text are particularly likely to hit byte limits sooner than character counts imply.
- **Recommendation:** Use the existing `collaborationCommentWithinByteLimit` / `collaborationTextByteLength` contract and field-specific invalid/describedby feedback for reason, result, and basis. Preserve text. KnowledgeByteLimit is an established pattern, but do not import knowledge-specific limits into delivery.
- **Regression:** Oversized multibyte revision reason exposes a local associated error and no write; shortening enables explicit submission. A result-only overflow marks only the result invalid.
- **Suggested command:** `$impeccable harden`.

### B5 — P2: Comment/record attachments disappear silently on unavailable metadata

- **Location:** `packages/views/collaboration/attachment-panel.tsx:133`–`:137`; consumers `comment-list.tsx:130`, `comment-history.tsx:24`, `command-history.tsx:68`, `run-records.tsx:194`.
- **Category:** Recovery / Implementation integrity.
- **Evidence:** `CollaborationCommentAttachments` returns null if attachment metadata is unavailable, including a failed initial read. A successful partial list silently filters out referenced IDs. Unlike `SubmissionFiles` in `delivery-panel.tsx:73`, it presents no loading/error/retry/partial-availability state.
- **Impact:** A discussion or recorded input that contains files can look like it never had attachments. Users lose the evidence path and a way to recover without visiting another panel.
- **Recommendation:** Keep a compact scoped file loading/error/retry state when IDs are known; distinguish omitted/unavailable IDs after successful metadata load. Reuse the existing protected query and list; preserve denial cleanup and do not reveal hidden file names.
- **Regression:** Known IDs plus rejected metadata request shows a retry; partial metadata reports unavailable references while rendering authorized files; denial shows no protected names.
- **Suggested command:** `$impeccable harden`.

### B6 — P2: Some secondary controls miss the project's coarse-pointer target floor

- **Location:** source list filter `packages/views/collaboration/source-filter.tsx:14`; knowledge summary/expiry `knowledge-proposal.tsx:97`, `:133`; knowledge digest/history summaries `knowledge-review.tsx:72`, `:80`; memory version link `memory-panel.tsx:221`; attachment/source-action dialog close controls through `attachment-panel.tsx:44`, `source-bridge.tsx:54` and primitive `packages/ui/components/ui/dialog.tsx:82`.
- **Category:** Responsive / Accessibility.
- **Evidence:** Source filter is explicitly 32×32 (28px high at md); knowledge inputs inherit 32px height; memory version action uses h-auto/p-0; secondary dialog close uses 28×28. Unlike creation, these surfaces do not add a local coarse-pointer override. The broad knowledge-panel selector applies to buttons, not inputs or summaries.
- **Impact:** These controls are harder to hit on coarse pointers and are inconsistent with the already audited 44px collaboration targets.
- **Recommendation:** Add scoped coarse-pointer minimum dimensions and wrapping to the identified controls, including portaled close buttons. Do not enlarge every desktop control or globally restyle shared primitives.
- **Regression:** Real browser coarse-pointer context measures rendered control rectangles after opening each relevant disclosure/modal; accessible target region is at least 44px as the local project specification requires. Keep 320px-wide dialogs in bounds.
- **Suggested command:** `$impeccable adapt`.
- **Evidence limit:** The main browser context used a fine pointer; its small-control counts are **not** coarse-pointer proof. 44px is the project's/skill's touch target criterion, not a claim that all sub-44px controls fail WCAG 2.2 AA.

### B7 — P2: Narrow English workbench header consumes substantial fixed vertical space

- **Location:** `packages/views/collaboration/account-workbench-page.tsx:40`–`:52`, `notifications.tsx:39`.
- **Category:** Responsive / Layout.
- **Evidence:** At 390×740 in English, the fixed shell scope row is 48px and the collaboration header wraps into separate title, tabs, New collaboration, and notifications rows. Detail starts around y=240; requirements are below the initial 740px viewport. See `assessment-b-narrow-detail.png` and `assessment-b-compact-detail.png`.
- **Impact:** On small windows much of the usable height is permanently taken by repeated navigation and header actions. The document has no horizontal overflow, yet actual task content is pushed down.
- **Recommendation:** Keep tabs and one compact action row with a compact, clearly named notification trigger; allow the main scroll area to receive more height. Preserve unread count, account scope, creation access, and the mobile back path. Coordinate with Assessment A's hierarchy decisions.
- **Regression:** English/Chinese at 320×568 and 390×740, both themes, coarse pointer. Capture actual header/content rectangles, not just body scrollWidth. Ensure all actions remain accessible.
- **Suggested command:** `$impeccable layout`.

### B8 — P2: Knowledge destination loading/error does not offer local recovery

- **Location:** `packages/views/collaboration/knowledge-proposal.tsx:28`–`:44`.
- **Category:** Recovery / Form feedback.
- **Evidence:** OtherTarget renders workspaces/projects as empty lists while pending and a generic error paragraph when either fails. No local retry control is rendered for either Query. Other primary candidate selectors already expose scoped loading and retry.
- **Impact:** Users cannot distinguish an empty destination catalog from a pending request and must change destination/reopen to recover, while preserving their proposal draft.
- **Recommendation:** Expose distinct loading, empty and failed destination states; provide a Retry action for the failed query. Keep the source workspace/project shortcuts independent of directory reads, preserve project identity, and clear effective authorization on denial.
- **Regression:** Held workspace read announces loading; failed project read retains summary/content/selected source identities and retries locally; revalidated destination must still be visible before submit.
- **Suggested command:** `$impeccable harden`.

### Performance observation, not a release defect

Closed native `<details>` preserve mounted children. `CollaborationDetail` mounts attachments, memory, knowledge and activity at lines 127–137. Their queries execute/poll even while closed (`memory-queries.ts:16`, `knowledge-queries.ts:21`, `content-queries.ts:7`). This is a bounded optimization opportunity for large active accounts, not proven latency harm. Do not simply conditionally unmount these children: pending intent, recovery attention, lease erasure, and saved human drafts must stay correct. If optimizing later, lazily fetch expensive bodies after first open while keeping lightweight recovery discovery active and retaining an opened form's lifecycle. No benchmark claim is made.

### Positive findings and constraints to preserve

- Account scope is independent of workspace directory availability; source and account views share actual business components and platform navigation.
- Workbench search explicitly says **Search loaded issues** / **搜索已加载的任务**, and no-results copy says loaded items. Do not report it as a false promise of global search. Broader discoverability may be a product improvement, but current copy is truthful.
- Selected list rows retain weight/foreground through hover; native detail disclosures and named status messages are widely used.
- Creation dialog has a stable header/source/actions and a scrollable body, input/portal focus treatment, 44px coarse targets, native validation, persistent draft/files, explicit original-key recovery, and reason-specific refusals.
- Account picker has full ID identity, stable option IDs, ARIA selected/disabled state, IME protection, exact fallback and pagination tests. Fix its search-commit hole without regressing those features.
- Result records distinguish exact submission/decision and unavailable historical evidence. No inference from done status to acceptance.
- Comment preflight captures current intent, ordinary comments send once, explicit AI targeting requires confirmation, stale preflight preserves input.
- Knowledge approval pins exact candidate/version/revision/hash/availability; human-authored drafts survive lease refresh. UTF-8 field limits and source selection are already modeled correctly.
- Resource consent/withdraw/switch forms use real current authority, observations, and original receipts; source/target IDs remain explicit where meaningful.
- Admin collaboration preserves narrow dedicated metadata authority, captured versions/successors, nonempty byte-bounded reason, and original-key recovery. No generic normal-workspace permissions are borrowed.
- Restricted RichContent, sandboxed HTML, PDF download instead of unsafe embedding, transient blob revocation, and scoped native-file capabilities are maintained.

## Files found and surface inventory

| Surface | Primary files | Status of inspection |
|---|---|---|
| Account list/detail shell, current/participating, search/sort/notifications | `account-workbench-page.tsx`, `notifications.tsx` | Source + live desktop/narrow light/dark; header B7 |
| Creation, source/parent/recipient/properties/attachments | `creation-form.tsx`, `account-picker.tsx`, `field-inputs.tsx`, `form-styles.ts`, `attachment-picker.tsx` | Source + live creation; B1, local B6 omissions outside creation |
| Source workspace entry/detail/actions/filter | `source-bridge.tsx`, `source-filter.tsx` | Source; B6 |
| Human workflow, participants, delivery/basis/acceptance | `workflow.tsx`, `participants.tsx`, `delivery-panel.tsx` | Source + keyboard-open live; B2/B4 |
| Discussion/reply/edit/delete/reactions/history | `comment-composer.tsx`, `comment-list.tsx`, `discussion-actions.tsx`, `comment-history.tsx` | Source and existing tests; B5 |
| Files, file delivery, previews/downloads | `attachment-panel.tsx`, `attachment-picker.tsx` | Source; B5/B6 |
| Shared context, versions, restore/purge | `memory-panel.tsx` | Source; B3/B6 |
| Knowledge proposal/review/source/target collections | `knowledge-panel.tsx`, `knowledge-proposal.tsx`, `knowledge-review.tsx`, `knowledge-source-picker.tsx`, `knowledge-controls.tsx` | Source; B6/B8 |
| AI execution, controls, records/history/handoff | `execution-panel.tsx`, `run-records.tsx`, `command-history.tsx`, `run-handoff.tsx`, `preparation-summary.tsx` | Source and existing suites; B5 file helper |
| Resource add/consent/revoke/offers/switch | `resource-panel.tsx`, `resource-form.tsx`, `resource-offers.tsx`, `switch-panel.tsx` | Source; preserve focus and authority patterns |
| Metadata, children, activity, query recovery | `field-summary.tsx`, `children.tsx`, `activity.tsx`, `query-state.tsx`, `focus.ts`, `labels.ts` | Source; reuse existing primitives |
| Limited admin list/detail/lifecycle/successor/memory purge/receipt | `admin/collaboration/collaboration-page.tsx`, `collaboration-form.tsx`, `collaboration-controls.tsx`, `collaboration-memory-controls.tsx`, `collaboration-receipt.tsx` | Source and existing tests only; no live privileged account |

Paths in the table are under `packages/views/collaboration/` unless the admin path is explicit. A complete test-name inventory is in `assessment-b-test-inventory.md`.

## Browser evidence and run notes

- Fresh isolated Playwright Chromium context/page; local package `@playwright/test` **1.58.2**, browser **HeadlessChrome 145.0.7632.6**.
- App URL `http://localhost:13493/account?view=participating&issue=01a1a9a7-53d0-777a-9305-01532d06cf17`; API `http://localhost:18573`.
- Test-owned user/workspace/issues were supplied by the parent through a private temporary credentials file. No credentials are copied into these research artifacts.
- **Local CORS workaround:** direct browser preflights lacked Access-Control-Allow-Origin, leaving a blank page. A context-local `route.fetch` forwards actual requests to the real API and adds response CORS headers; OPTIONS is fulfilled locally. No app code or server configuration was changed. This is rendered UI evidence, not proof of production cross-origin setup.
- No native browser MCP was available; installed Playwright fallback was used. Headless contexts are not presented human-visible tabs; no persistent human-visible overlay is claimed.
- Mutable injection preflight succeeded by changing title to `[Human] Impeccable audit` and appending/running a script. `/detect.js` injected successfully into 3 states; browser console summaries and overlays are saved.
- Temporary impeccable server was started with cwd inside this research directory, and stopped with `live-server stop --keep-inject`. Isolated Chromium contexts were closed. Research scripts/evidence remain as intentional artifacts.
- `.impeccable/critique/ignore.md` was absent. No prior assessment snapshot was consumed. Parent owns target slug and combined critique storage.
- Screenshots contain a Next development issue indicator. It is not product UI and is not an audit finding.

| Capture / measured state | Geometry result |
|---|---|
| `assessment-b-desktop-detail.png`, 1440×900 | Document no horizontal overflow; account-list 351/351 client/scroll width; detail 1088/1088 |
| `assessment-b-narrow-detail.png`, 390×740 | Document and detail 390/390; no horizontal overflow |
| `assessment-b-narrow-create.png`, 390×740 | Dialog in viewport; no document horizontal overflow |
| `assessment-b-narrow-list.png`, 390×740 | Document no overflow; list 389/389 |
| `assessment-b-narrow-dark-detail.png`, 390×740 | Document/detail no horizontal overflow |
| `assessment-b-compact-detail.png`, 320×568 | Document/detail 320/320; no horizontal overflow |

Raw geometry: `assessment-b-browser-evidence.json`. Overlay screenshots: `assessment-b-desktop-detail-overlay.png`, `assessment-b-narrow-create-overlay.png`, `assessment-b-narrow-list-overlay.png`. The geometry helper included some hidden/inert descendants in its small-control list; that count is diagnostic only, not accepted accessibility findings. It does not validate all touch targets or a spoken screen-reader experience.

## Regression strategy

Use existing shared component files for behavior tests; keep pure boundary matrices in Core. Extend actual primitives rather than replacing dropdowns/dialogs with div mocks.

1. `account-picker.test.tsx`: typed query ownership + stale/no-match commit and keyboard selection with self present (B1). Reuse existing IME/paging/full-identity assertions.
2. `workflow.test.tsx`, detail integration test: one focus lifecycle per inline action and result preparation; associated per-field revision/result/basis byte errors (B2/B4).
3. `memory-panel.test.tsx`: initial read failure, retry, unsupported response, denial (B3). Existing suite mainly covers anchors, stale edits, unknown receipt, restore and revocation.
4. `attachment-panel.test.tsx` / comment or command-history integration: metadata pending/error/partial denial without disappearing file existence (B5).
5. `knowledge-panel.test.tsx`: held/failed other-destination directories with preserved proposal and local retry (B8).
6. Existing production browser journeys: one batched EN/ZH, light/dark, 320/390/desktop check for header, creation, supporting controls. Coarse-pointer rectangles for B6; no additional broad helper-matrix mounts. Run relevant focused suites after implementation; do not rerun all existing provider/native acceptance suites for presentation-only edits.

The existing suite is extensive: real API and desktop journeys cover creation, recipient lookup, source editing, comment/mention workflows, field changes, resources/consent/offers/switch, execution control, run records, command history, memory/knowledge, original-request recovery, revocation, and administration. Many existing browser suites check wide/narrow geometry. This audit does **not** claim those tests passed today; it inventories their assertions.

## Related specs

- `CLAUDE.md`: shared package boundaries, server/client state, semantic tokens, one canonical test layer.
- `.trellis/workflow.md`: persisted research and scoped role protocol.
- `.trellis/spec/views/frontend/index.md` and `component-guidelines.md`.
- `.trellis/spec/views/frontend/collaboration-detail.md`: facts-first hierarchy, role actions, real viewport focus, enclosing disclosure recovery.
- `.trellis/spec/views/frontend/collaboration-creation.md`: current source identity, field/picker focus, fixed dialog actions and coarse targets.
- `.trellis/spec/views/frontend/collaboration-source-picker.md`: visible pending/retry, no stale parent commitment and current protected Query authority.
- `.trellis/spec/views/frontend/collaboration-knowledge.md`: exact live review, field bytes, source identity, original outcome recovery.
- `.trellis/spec/core/frontend/collaboration-workbench.md`: scoped access, original keys, wire validation, no inference from completion.
- `.trellis/spec/core/frontend/global-scope-navigation.md`: account independence, restored navigation, repeated explicit anchors.
- `.trellis/spec/server/collaboration-administration.md`: limited dedicated admin authority, captured revisions, byte-bounded reason.

## External references / versions

No external documentation was fetched for this source-centered audit. Local impeccable skill **4.5.1** and its `reference/audit.md` and Assessment B in `reference/critique.md` were followed. Shared UI uses `@base-ui/react` **^1.3.0** from the workspace manifest. This report does not certify full WCAG conformance or assign unmeasured contrast ratios.

## Recommended actions

1. **P1 `$impeccable harden`**: resolve current-query recipient commitment first.
2. **P2 `$impeccable harden`**: workflow focus, initial memory/file recovery, local field and destination feedback.
3. **P2 `$impeccable layout`**: compress narrow account header with Assessment A's direction.
4. **P2 `$impeccable adapt`**: scoped coarse-pointer sizes and portaled controls.
5. **`$impeccable polish`**: final bounded rendered verification after the behavior fixes.

## Caveats / Not Found

- No code, spec, config, migration, git or external application changes were made by Assessment B.
- Live evidence is limited to the real test-owned account workbench/detail/creation states described above. No resource consent, destructive lifecycle, invite, send, knowledge approval, admin operation, or file upload was performed.
- No physical-device gesture, spoken assistive technology, Electron Main, privileged admin-browser, production build, performance timing, comprehensive contrast, or 200% zoom acceptance was run in this audit.
- Existing E2E/proof artifacts from earlier tasks establish available test coverage, not current acceptance. This report keeps source and live evidence distinct.
- Assessment B does not own the combined critique questions, final product plan, or implementation; those belong to the parent task.
