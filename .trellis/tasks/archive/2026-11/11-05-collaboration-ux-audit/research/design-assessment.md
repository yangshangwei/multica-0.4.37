# Research: Collaboration UX assessment A

- Query: Independently audit every existing cross-workspace collaboration surface for business UX, visual hierarchy, simplicity, states, actions, and localization; recommend bounded fixes.
- Scope: Internal; shared Web/Electron surfaces, their entry points, and native-mobile implementation inventory. Assessment A only; no detector output was run or read.
- Date: 2026-11-05
- Mode: Impeccable Operate / critique. Preserve the existing product identity and authority/recovery contracts.

## Findings

The collaboration model is sound: one source issue, one current handler, retained participation after handoff, explicit result submission and initiator acceptance, and separately authorized AI execution. The strongest parts are the exact-result decision flow and recovery of ambiguous writes. The most consequential weaknesses are presentation branches that hide failed auxiliary reads and the missing spatial transition from a comment to its prepared result. These can be fixed in shared Views without redesigning the workflow or changing access policy.

### Evidence and assessment provenance

- Read `CLAUDE.md`, `.trellis/workflow.md`, the Views frontend index, all four Views collaboration specs, the Core workbench contract, implementation files, incumbent focused tests, both collaboration dictionaries, and Web/Desktop route wiring.
- Read `/Users/artisan/.agents/skills/impeccable/SKILL.md` (4.5.1), `reference/critique.md`, and `reference/operate.md`. Ran `impeccable context --target packages/views/collaboration` once. No PRODUCT.md, DESIGN.md, or surface brief was found; the existing code and scoped specs supply the brief. No missing-document setup is required for this audit.
- Visually inspected archived production Web captures `web-manual-before-wide.png` and `web-accepted-zh-Hans-narrow.png` under `.trellis/tasks/archive/2026-11/11-02-collaboration-detail-optimization/evidence/web-final/`.
- Independently compared SHA-256 against that run's `detail-journey-evidence.json`: `collaboration-detail.tsx`, `workflow.tsx`, `delivery-panel.tsx`, `comment-list.tsx`, `comment-composer.tsx`, and both collaboration dictionaries match the current source. The old manifest contains no hash for the workbench or current tokens, so it is preliminary visual evidence, not a current full-render verification.
- Attempted an isolated Playwright Chromium context against `http://localhost:13493/account?view=participating&issue=01a1a9a7-53d0-777a-9305-01532d06cf17` using only the parent-provided test account. The first attempt timed out after 60 seconds waiting for the detail heading. One bounded diagnostic attempt returned the same URL with empty body text, no headers, and no pageerror. Saved blank wide/narrow diagnostic captures and observations under `research/design-evidence/`. These do not prove product layout or task completion. Browser contexts were closed; no server was started by this assessor.
- Before finalizing, visually inspected the parent's fresh 1440px capture `evidence/before/detail-wide.png`. It shows the real API fixture with three collaboration tasks, a self-handled detail, readable source/role metadata, a clear primary result action, empty result/discussion, and consistent restrained styling. The parent used a temporary Playwright response-header CORS shim because this checkout's API port did not allow the dev web origin. This is a real-data browser capture with a local transport shim, not an unmodified cross-origin deployment check. It contains no detector overlay or Assessment B findings.
- Incumbent tests were inspected, not executed by Assessment A. Parent reports its baseline: 30 files / 281 tests passed, log `/tmp/multica-ux-baseline-tests-scoped.log`. Treat that as parent-owned verification, not this audit's independent runtime result.

### Design specificity and strengths

**Specificity: product-grounded, with some operational jargon in secondary surfaces.** The source workspace, external handler, current permission context, result/basis, and exact acceptance belong to this product. Familiar controls and restrained neutral surfaces suit a coordination tool. Adding ornamental cards or a second design system would not improve the task.

1. **Roles and decisions are explicit.** `workflow.tsx:51` and `workflow.tsx:60` consume capabilities plus Core's role-derived action priorities, limiting the leading group to one primary and two secondary actions. `participants.tsx:21` explains the viewer's actual role. `delivery-panel.tsx:24` presents result, author, basis, and decision without inferring acceptance from status.
2. **Sensitive state is treated honestly.** Original requests survive uncertainty; source selection locks while pending, exact acceptance pins the displayed submission, knowledge review requires the exact current version, and private content uses restricted rendering. Existing confirmations for participant removal, memory purge, and resource authorization are substantive, not gratuitous.
3. **The basic composition is calm and resilient.** Archived wide/narrow captures show wrapping titles and body text, recognizable selected navigation, and no decorative noise. Creation has a stable action footer and independently scrolling body; auxiliary details avoid loading the main first screen with every advanced function.

### Priority issues and achievable fixes

#### A1 — P1: Shared context can fail into an empty disclosure, hiding recovery

- **Trigger:** the first memory request fails, returns unsupported/unavailable data, or has no current readable value while an original memory operation still needs attention.
- **Evidence:** `packages/views/collaboration/memory-panel.tsx:139` correctly handles denial and loading, then line 141 returns `null` for `!memory` or `CollaborationUnavailableError`. The ordinary error/retry UI at line 147 and pending operations at line 174 are after that return. `collaboration-detail.tsx:130` still renders the outer Shared context disclosure.
- **User consequence:** opening Shared context can show nothing and provide no retry, despite a visible feature entry. An unconfirmed save may have an opened outer disclosure but no recoverable action inside. This undermines the feature's otherwise careful original-request contract.
- **Fix:** retain a named Shared context region for unavailable/failed reads; show a localized reason plus explicit refresh. Render permissible original-operation recovery separately from the current-content gate, while preserving authoritative denial teardown. Do not display stale content or fabricate an empty version. Distinguish unsupported from empty successful content.
- **Verification:** add initial transport failure, successful unsupported/null response, retry-to-success, and pending-operation-plus-read-failure tests in `memory-panel.test.tsx`. Existing revoked-access and original-key tests must remain green. Use a browser-intercepted initial failure to verify the enclosing disclosure exposes the reason/action and a retry makes the region usable.

#### A2 — P1: Referenced comment files can silently disappear

- **Trigger:** a comment/history/run input references attachments and its protected metadata request fails, is still loading, or returns only some of the referenced IDs.
- **Evidence:** `packages/views/collaboration/attachment-panel.tsx:133` queries metadata, but line 136 returns `null` whenever data is absent; line 137 filters references without reporting missing IDs. Consumers include `comment-list.tsx:130`, `comment-history.tsx:24`, `command-history.tsx:68`, and `run-records.tsx:193`. By comparison, `delivery-panel.tsx:73` already exposes loading, error/retry, and partial evidence.
- **User consequence:** a discussion or recorded input can look as if it never had files. A collaborator can miss the evidence needed to work or review. A permission-safe missing file must not be visually indistinguishable from no reference.
- **Fix:** use the same factual loading/error/retry/missing-reference treatment as result evidence within the referenced-file component. Keep protected IDs as references; do not reveal names/bodies from cleared Query data and do not add ordinary-workspace file fallbacks. Show a concise partial-unavailability notice alongside available files.
- **Verification:** focused component cases for no-reference/no request, initial failed metadata, partial metadata, retry success, and denial clearing. Confirm all four consumers retain their existing restricted download/preview behavior.

#### A3 — P1: “Use in result” can open the result draft outside the user's viewport

- **Trigger:** scroll into a long discussion and click Use in result / 引用为结果.
- **Evidence:** `comment-list.tsx:133` forwards the selected comment; `collaboration-detail.tsx:120` stores it; `workflow.tsx:67` opens the result action by changing state. The WorkflowForm renders near the top at line 98 / line 237, above requirements and all comments. Neither this transition nor WorkflowForm mounts a focus/reveal action. The existing `focus.ts:31` has `useOpenedForm`, used by resource forms but not the workflow.
- **User consequence:** the explicit action has no local visible acknowledgement; keyboard focus stays on the comment and the next editable draft can be many screens earlier. A user may click repeatedly or assume the action failed. Archived narrow discussion evidence shows how far the result workflow can be from the discussion.
- **Fix:** on the explicit preparation transition, open any ancestor disclosure, reveal the mounted result form in the real `account-detail` scroll root, and focus its heading or first result field once. Preserve the user's editable result draft, require basis and formal submit, and restore focus to a sensible opener when dismissed. Do not scroll on refetch, unrelated text edits, or restored scope navigation. Reuse existing focus helpers where semantics fit; `recoverFocus` alone intentionally does nothing while the original live button retains focus.
- **Verification:** named regression using a mounted result form for focus/field contents; real browser long-discussion test for visible bounds after click. No submit request should be made by preparation. Preserve existing result-draft persistence and explicit submission tests.
- **Confidence:** high from source flow; the fresh live scenario was blocked by the blank local page, so do not claim a reproduced live click yet.

#### A4 — P2: List browsing exposes its partial-loading model without enough assistance

- **Trigger:** an account has more than one 50-item workbench page, or applies search while only part of the list is loaded.
- **Evidence:** `packages/core/collaboration/queries.ts:36` requests 50 items. `account-workbench-page.tsx:76` filters/sorts only loaded rows, line 85 displays the server total, line 95 uses the loaded-results empty state, and line 104 supplies a generic Load more. Search has no dedicated clear control. The dictionaries accurately say Search loaded issues / 搜索已加载的任务 and No loaded issues match, so this is **not** a false-empty or global-search correctness claim.
- **User consequence:** a count of 150 with 50 searchable rows is hard to reason about; alphabetical sorting changes as pages arrive, and users must discover that more loading can reveal a match. The empty current queue also has no direct participating action even though that is the most likely next step.
- **Fix within Views:** show loaded versus total count when incomplete, a concise partial-search hint and actionable Load more in the no-match state, and a labeled clear-search control. Normalize incidental outer whitespace. Give the current-list empty state a participating link. Keep query identity, scope restoration, explicit paging, and selected-detail independence. Full server search is a separate larger feature, not needed for this repair.
- **Verification:** two pages with a match only on the second; clearing restores the list; loaded/total and empty states remain truthful; switching scopes restores existing search/scroll semantics.

#### A5 — P2: Dates in one discussion use different locale conventions

- **Trigger:** select Chinese in an English-locale browser and read delivery plus comments/activity/history.
- **Evidence:** `comment-list.tsx:112`, `activity.tsx:24`, `comment-history.tsx:23`, and `command-history.tsx:64` / `:76` call `toLocaleString()` without the app locale. `delivery-panel.tsx:50`, `:65`, `memory-panel.tsx:47`, and `knowledge-review.tsx:46` correctly pass `useLocale()`. The inspected Chinese archived capture visibly mixes Chinese decision dates with an English `11/3/2026, 1:31:01 PM` comment timestamp.
- **User consequence:** date order and time notation vary inside a single audit trail; month/day ambiguity is avoidable friction in a collaboration product.
- **Fix:** pass the current app locale to discussion/activity/comment/command timestamps using the established locale hook. Do not change timezone semantics or reformat server IDs. Preserve machine-readable `<time dateTime>` values.
- **Verification:** both app locales against one deterministic instant; retain original ISO dateTime. A current Chinese capture should show consistent date conventions for result and discussion.

### Secondary observations, not reasons to redesign compliant flows

- **P2 candidate — narrow header density.** `account-workbench-page.tsx:40` contains a repeated title, two tabs, creation, admin entry when allowed, and notifications in one wrapping header; `scope-shell.tsx:47` adds the global scope strip. The archived 430px Chinese capture spends about 160px on persistent chrome. Preserve global scope access but consider a compact structured header on narrow widths, with one clear creation action and compact notification indicator. Verify fresh English/Chinese, 320/390px, zoom and coarse targets before changing it; the old capture's token/shell freshness is not fully proven.
- **P2 candidate — discussion control density.** Each owned comment may expose Reply, Use in result, Edit, Delete, Resolve, reaction, history, and permission disclosure (`comment-list.tsx:115`, `:131`, `:157`). The composer maps every participant and every AI target into visible buttons (`comment-composer.tsx:114`). At scale this becomes a control wall. A follow-up may group secondary comment operations and use an existing searchable mention affordance, while keeping explicit AI confirmation and recovery visible. This is not needed to fix A1–A5 and should not become a new mentions subsystem.
- **P2 candidate — notification chronology.** `notifications.tsx:46` shows task title and event kind with Mark read / Archive, but no event time. Repeated assignment/comment events are hard to distinguish. Add a formatted time only if the current protected notification schema supplies it; do not invent or fetch hidden fields.
- **P2 candidate — lifecycle confirmation clarity.** Generic workflow confirmation is labelled Confirm / Close (`workflow.tsx:321`), including leave/cancel, with less effect explanation than participant removal and resource withdrawal. Prefer action-specific confirm labels and a concise existing-semantic consequence when touching those flows; do not add unnecessary modal layers.
- **Do not remove full recipient IDs as “visual clutter.”** `account-picker.tsx:132` and the creation spec deliberately disambiguate equal names and shared UUID prefixes. Keep full identity while improving spacing.
- **Do not add issue bodies/titles to the admin view.** Its metadata-only surface is an authority boundary for disabled-creator recovery, not an incomplete everyday issue detail.
- **Do not make AI assisted policy silently resume work.** Policy, resource authorization and queue resume are distinct decisions. Existing text explains this and should remain.

### All-surface coverage and disposition

“Source” means implementation/spec/tests inspected. “Visual” means the archived captures above plus the parent's fresh wide account capture. The latter confirms the main workbench/detail appearance only, not its actions or auxiliary panels. No row claims end-to-end proof from a source read.

| Existing surface | Entry / files found | Coverage and disposition |
| --- | --- | --- |
| Global workspace / collaboration scope switch | `packages/views/layout/global-scope-navigation.tsx:16`; `apps/web/platform/scope-shell.tsx:14`; desktop `components/account-navigation.tsx` | Source; visible in archived captures. Coherent two-scope model; preserve scope-resume behavior. Narrow density observation only. |
| Account current/participating workbench, list/detail, empty/error/search | `packages/views/collaboration/account-workbench-page.tsx:21` | Source + archived wide detail. A4 partial list assistance; otherwise selected detail intentionally independent of list membership. |
| Web account route and guards | `apps/web/app/account/page.tsx`, `layout.tsx`; `apps/web/platform/account-workbench-route.tsx:10` | Source. Platform wrapper resolves account path and title; no UX-driven route rewrite needed. |
| Electron account route/tab entry | `apps/desktop/src/renderer/src/platform/account-workbench-route.tsx:10`; `components/tab-bar.tsx:227` | Source. Shared business UI, distinct persisted account tab and explicit workspace clearing. Native runtime not newly exercised. |
| Creation modal and standalone panel | `creation-form.tsx:24`, `:42`, `:193` | Source/spec/tests. Scoped drafts, stable footer, source locks, refusal feedback, native form semantics are valuable; retain. |
| Recipient and exact-account picker | `account-picker.tsx:53` | Source/spec/tests. Search, exact fallback, selected identity, IME/ARIA/full-ID coverage already deliberate; no independent release-blocking redesign finding. |
| Initial/edited fields, source/parent/iteration selection | `field-inputs.tsx:15`; `creation-form.tsx:223`; `workflow.tsx:302` | Source/spec/tests. Existing searchable parent and explicit source/revision rules remain. |
| Detail header / requirements / metadata | `collaboration-detail.tsx:80`, `:101`, `:141`; `field-summary.tsx:7` | Source + archived visual. Good hierarchy; preserve content-first design and container-based stacking. |
| Role-specific actions, transfer, information request/return, policy, lifecycle | `workflow.tsx:25`, `:112`; `participants.tsx:10` | Source/spec/tests. Exact capability/role gates and explicit forms are sound; A3 preparation transition; minor generic confirmation copy. |
| Participants / remove confirmation | `participants.tsx:42` | Source + archived expanded roster. Removal explains effects and pins revision; keep. |
| Result, basis, acceptance/revision, prior deliveries | `delivery-panel.tsx:14`, `:43`; `workflow.tsx:149` | Source + archived EN wide/ZH narrow. Strong exact decision presentation; file unavailability treatment is the pattern to reuse for A2. |
| Comments, reply/edit/delete, reactions, resolution, comment history | `comment-list.tsx:26`; `discussion-actions.tsx:10`; `comment-history.tsx:10` | Source + archived narrow comments. A2, A3, A5; otherwise sensible explicit write/recovery behavior. Secondary-action density is a follow-up. |
| Comment composition / human mentions / AI preview | `comment-composer.tsx:16` | Source/spec/tests. Single-send human flow and explicit AI confirmation are correct. Scale of inline mention buttons is a secondary observation. |
| Protected files / upload / preview / native download | `attachment-picker.tsx:22`; `attachment-panel.tsx:27`, `:87`, `:133` | Source/spec/tests. A2 missing referenced-file states; preserve protected download and sandbox/PDF rules. |
| AI execution, Stop/Resume, pending-input cancellation | `execution-panel.tsx:21` | Source/spec/tests. Authority and uncertainty are explicit; empty manual console omission is intentional. No live provider claim. |
| Resource register, consent/withdrawal, offers/adoption | `resource-panel.tsx:18`; `resource-form.tsx:34`, `:99`; `resource-offers.tsx:15` | Source/tests. Candidate/owner/runtime context and explicit consent are defensible. Reuse their inline-form focus pattern for A3. |
| Resource replacement proposals/consent/commit | `switch-panel.tsx:17` | Source/tests. Stop/consent prerequisites and original-operation recovery are present; no scope expansion needed. |
| Run records / preparation / handoff / retained inputs | `run-records.tsx:19`, `:63`; `preparation-summary.tsx:4`; `run-handoff.tsx:9`; `command-history.tsx:14` | Source/tests. Clear unavailable/missing facts and lazy records. A2 referenced files, A5 time consistency. IDs are audit facts, not a permission to add directory lookups. |
| Shared context current/edit/history/version/restore/purge | `memory-panel.tsx:31`, `:139`, `:178`, `:199` | Source/spec/tests. A1 is priority. Existing version/conflict/closed/purge contracts must survive. |
| Knowledge proposal/source/target selection/review/correction/derivation/recovery | `knowledge-panel.tsx:16`; `knowledge-proposal.tsx:47`; `knowledge-source-picker.tsx:10`; `knowledge-review.tsx:14` | Source/spec/tests. Exact-version approval, draft retention, byte limits and scope-aware recovery are sound. Complex but justified; no parallel knowledge model. |
| Project managed knowledge | `packages/views/projects/components/project-detail.tsx:541`; `knowledge-panel.tsx:26` | Source. Existing target-panel mount outside the ordinary description editor; preserve boundary. |
| Workspace settings managed knowledge | `packages/views/settings/components/workspace-tab.tsx:542`; `knowledge-panel.tsx:26` | Source. Member-gated panel is present; source restriction is factual. |
| Activity audit and linked comments | `activity.tsx:10` | Source/tests. Before/after and actor provenance available; A5 timestamps. |
| Ordinary children | `children.tsx:7`; detail sidebar at `collaboration-detail.tsx:149` | Source/tests. Direct source link only when `can_open` and source slug allow it; unavailable versus empty distinguished. |
| Notifications and deep links | `notifications.tsx:12` | Source/tests. Read/archive do not imply task completion; content-free removal retained. Secondary timestamp observation. |
| Source-workspace creation / external filter / row identity | `issues/components/issues-page.tsx:61`; `source-filter.tsx:7`; `source-bridge.tsx:18`, `:23`; list/table/board/Gantt/swimlane callers | Source. One shared issue identity across views. Coarse filter target should be checked during responsive verification; no duplicate issue model. |
| Source issue detail and action dialogs | `source-bridge.tsx:35`, `:50`; `issues/components/issue-detail.tsx:1169`; `issues/surface/issue-surface.tsx:290` | Source/tests. Reuses account detail/workflow and shared navigation; A1–A3 fixes should propagate here. |
| Super-admin collaboration list/detail/entry | `admin/collaboration/collaboration-page.tsx:15`, `:24`; `admin/admin-shell.tsx:79`; web `/admin/collaboration`; desktop account administration | Source/tests. Metadata-only disabled-creator administration is intentional; existing refresh/access states are clear. Preserve distinct route wrappers. |
| Admin lifecycle, successor choice, reason, receipt/retry | `admin/collaboration/collaboration-controls.tsx:13`; `collaboration-form.tsx:11`; `collaboration-receipt.tsx:8` | Source/tests. Explicit reasons, captured baseline, identity details, original receipt recovery. No independent mandatory change found. |
| Admin shared-context version disposal/recovery | `admin/collaboration/collaboration-memory-controls.tsx:18` | Source/tests. Keeps content out and requires reason/confirmation. Error has limited recovery guidance; evaluate a local versions retry in a future focused pass. |
| Native mobile | Repository-wide search of `apps/mobile/` for collaboration/cross-workspace terms; no feature routes, UI or query implementation found | Inventory only: not implemented. Web narrow captures are not native-mobile parity. Do not add a native feature during this repair. |

### Cognitive load assessment

| Criterion | Assessment |
| --- | --- |
| Clear primary decision | Pass for role-aware workflow. One primary plus up to two secondary actions; More is intentional progressive disclosure. |
| Decisions with more than four visible choices | Conditional failure in owned-comment operations; composer participant/AI button rows are unbounded. Knowledge applicability intentionally has five choices, but these are secondary controls inside an explicitly opened proposal form. |
| Recognition versus recall | Good exact recipient and source-parent browsing. Partial workbench paging and UUID-heavy AI target rows require more effort. Avoid adding lookups solely to make private identities prettier. |
| Context needed at decision | Strong: exact delivery rendered for acceptance, prior information request shown on return, resource owner/runtime shown at consent. A3 breaks spatial continuity when preparing a result from discussion. |
| Error and next step proximity | Mixed: strong creation refusal/footer and knowledge recovery; A1/A2 conceal initial failure. Generic workflow refusal is less actionable than memory/creation-specific feedback. |
| Hierarchy / noise floor | Main detail is calm. Nested disclosure then repeated heading/boxed panel adds weight to auxiliaries; narrow shell and per-comment controls consume vertical space. No strong evidence for a wholesale redesign. |
| Interruptions / work preservation | Creation/comment/result/original intents are deliberately scoped and persisted. Regular edit/knowledge text preservation differs by form; avoid promising autosave where only local component state exists. |

### Emotional journey and personas

- **First-time collaborator (Jordan):** creation explains source and AI participation, but a sparse account empty screen and the terms current handling / participating need an actionable bridge. A silent Shared context/file failure feels like missing work rather than a recoverable loading problem.
- **Keyboard/screen-reader user (Sam):** recipient search, dialogs, role labels and many recovery states are carefully designed. The prepared result transition leaves focus in the old location; this is the clearest accessibility flow gap. Verify painted focus and actual scroll bounds, not only DOM mounting.
- **Experienced coordinator (Alex):** role actions and linked history support efficient decisions. Partial list search and unbounded per-comment/mention controls slow larger accounts. Full server search and bulk notifications may be future product work, not prerequisites for the bounded fixes.
- **Journey peak:** exact result and decision make completion feel attributable and trustworthy.
- **Journey valley:** losing response confirmation is already stressful; hidden auxiliary recovery adds uncertainty at exactly that moment. Restore truthful states first.
- **Ending:** accepted/returned evidence stays readable after reopening; preserve this strength. Read/archive is notification housekeeping and should stay distinct from completion.

### Nielsen heuristic scores

All ten apply to this Operate surface. Scores are a source-based expert assessment supported by the limited archived visual evidence, not a measured usability study.

| Heuristic | Score / 4 | Evidence / implication |
| --- | --- | --- |
| 1. Visibility of system status | 2 | Rich ordinary pending/receipt states, but A1/A2 silently hide auxiliary failure. |
| 2. Match with the real world | 3 | Source, handler, result/basis and explicit acceptance are natural; execution/knowledge terms need domain familiarity. |
| 3. User control and freedom | 3 | Cancel/close, retained drafts, original request checking and explicit restore/purge. Search lacks clear reset assistance. |
| 4. Consistency and standards | 3 | Shared platform-agnostic business views, familiar controls/tokens. Date locale and inline focus behavior diverge. |
| 5. Error prevention | 3 | Exact observed versions, recipient identity, explicit consent and destructive confirmations. Some disabled forms use generic guidance. |
| 6. Recognition rather than recall | 2 | Good source/recipient browsing, but A3 and partial-list navigation demand recall and manual searching. |
| 7. Flexibility and efficiency | 2 | Search/sort, restored scope, anchors and keyboard primitives; partial loading and expanding controls impede larger workloads. |
| 8. Aesthetic and minimalist design | 3 | Restrained, clear main hierarchy. Header and discussion control density are the main rough edges. |
| 9. Error recognition and recovery | 2 | Many precise original-intent contracts, undermined by A1/A2 and generic workflow failure wording. |
| 10. Help and documentation | 2 | Contextual explanations for consent, policy and knowledge; no clear end-user help route within this surface. |
| **Total** | **25 / 40 (62.5%)** | **Acceptable foundation; fix specific recovery/discovery gaps before adding complexity.** |

### Ordered implementation plan

1. **Expose auxiliary recovery (A1–A2):** keep edits in memory/file shared components plus their named regression tests and necessary bilingual keys. Preserve all denial, epoch and original-key behavior.
2. **Repair explicit result preparation navigation (A3):** one intentional focus/reveal transition, not a polling-driven effect. Test form contents, no POST, focus, actual scroll root and dismissal return.
3. **Make workbench browsing and time presentation coherent (A4–A5):** loaded/total feedback, clear search and actionable empty current/no-match states; use the existing locale hook across timestamps. Preserve persisted search/sort and independent selected detail.
4. **One bounded visual refinement pass:** after the local page renders, inspect normal and long content at wide/narrow widths in both locales/themes. Compact the header only if fresh evidence confirms the density concern; keep existing tokens and action hierarchy. Avoid upgrading every compliant disclosure just to enlarge the diff.
5. **Verification:** focused Views suites and locale parity, scoped lint/typecheck, then parent-owned real Web/Electron checks appropriate to the final change. Confirm blank/error/loading/success/denied/partial-file/pending-original states. Source-workspace and account entry should exercise the same fixed components. No tests of protected server semantics should be duplicated as large DOM matrices.

### Related specs and references

- `.trellis/spec/views/frontend/collaboration-detail.md` — facts-first hierarchy, explicit result preparation, all-ancestor recovery, anchors and scroll proof.
- `.trellis/spec/views/frontend/collaboration-creation.md` — scoped completion, locked source, visible feedback, keyboard identity, constrained form.
- `.trellis/spec/views/frontend/collaboration-source-picker.md` — current protected options, browse/search/error/retry, explicit UUID selection.
- `.trellis/spec/views/frontend/collaboration-knowledge.md` — exact human review, separate source selection, protected projections and original recovery.
- `.trellis/spec/core/frontend/collaboration-workbench.md` — Core owns authority, operations, query identity, and original intent; absence is not authorization loss.
- `.trellis/spec/core/frontend/global-scope-navigation.md` — restored account/workspace scope and fresh anchor distinction.
- Local Impeccable 4.5.1 critique/Operate references — independent A/B assessment, heuristic scoring and preserve appropriate familiar product UI.
- Installed `@playwright/test` 1.58.2 — isolated browser attempt, not an external design authority. No network documentation/recommendation claims were used in this internal assessment.

## Caveats / Not Found

- The fresh local URL rendered a blank body in this assessor's two isolated attempts. `design-evidence/live-wide-zh.png`, `live-narrow-zh.png`, and `live-observations.json` record that environment limitation; do not count them as successful product screenshots. The parent subsequently identified local CORS misconfiguration and supplied a successful wide capture using the disclosed transport shim. The blank page is not classified as a collaboration UI defect.
- The archived captures have matching key detail/component/dictionary source hashes, but no workbench/token hash. Native Electron, current dark theme, live AI execution, platform-admin privilege, and knowledge decision flows were not newly exercised by Assessment A.
- No native mobile collaboration implementation was found; this task covers existing shared Web/Desktop UI.
- No detector findings or Assessment B report influenced the findings or scores.
- No product code, tests, specs, task state, or git state was changed by this assessor.
- Questions skipped: the user already authorized full audit and bounded repair, and the parent owns synthesis/execution. The design decisions above are actionable without another approval gate.
