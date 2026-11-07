# Independent Mermaid code and content review

Reviewed current task diff on 2026-11-07. This reviewer is read-only for implementation and document sources; the coordinator owns live Electron/public rendering, screenshots, generated bundle and final integration.

## Findings (fixed)

No implementation files were edited by this reviewer.

## Findings resolved

### Deferred promotion is gated by more than its scheduled time

- File: `apps/docs/content/docs/tasks.zh.mdx`, new task-state graph and its adjacent status table.
- The diagram edge says `到达时间`, and its accessibility description says the deferred task enters the queue when due. This is incomplete for the current implementation.
- Evidence: `server/pkg/db/queries/agent.sql`, `PromoteDueDeferredTasksForRuntime` (around lines 2364–2414), requires `fire_at <= now()`, an online runtime with a fresh heartbeat, no occupied queued/dispatched slot for the same issue/agent, and at most one candidate per slot. A due task can legitimately remain deferred.
- Recommendation sent to the coordinator: label the edge `满足触发条件` and briefly state that being due still requires runtime availability and a free pending slot. Keep this outside the graph's main node labels to preserve legibility. Synchronize the nearby table's same claim.
- Resolved by the coordinator in the graph edge, accessibility description, adjacent explanation and status table; final rendering will use the corrected source.

## Verified code contracts

- Chinese authoring uses ordinary Mermaid fences. The generator preserves their text; Fumadocs uses the already installed `remarkMdxMermaid` transformer. No JSX whitelist expansion or new package dependency was introduced.
- Both public page entry points bind `Mermaid`; normal Shiki code continues through the default MDX map.
- `DocsContent` dispatches only the complete `mermaid` language token. Lookalikes (`mermaidx`, `mermaid+preview`) remain ordinary code, and the source is passed without interpreting Markdown inside the fence.
- `DocsDiagram` reuses the existing leaf and viewer, with the near-viewport `LazyRichBlock` mount gate. The initial SSR/client frame remains the lazy placeholder rather than a sessionStorage-dependent leaf render.
- Inline application rendering retains `securityLevel: strict`, `htmlLabels: false`, error suppression and empty iframe sandbox. Zoom/source/export remain host-controlled; no new script/same-origin allowance was added.
- Public rendering remains strict, adds caught dynamic-import/render failures, preserves source fallback and uses natural diagram width inside a horizontal scrolling region. It does not pull business views/i18n into the public docs package.
- Diagram styles are scoped to `.docs-diagram`; the whole reader is not wrapped in editor prose styles. Existing editor styling receives the same selectors as before.
- Shared theme variables now explicitly cover clusters, actors, signals and notes, and use the host's computed font family. This also changes existing editor diagram borders from the primary accent to the semantic border color; the coordinator should include this intended shared-leaf visual effect in its validation scope.
- Existing leaf/viewer tests remain canonical for sandbox, failure source retention, theme changes, source/export, keyboard and zoom behaviors. New tests exercise only the docs wiring and public transformer/bundle parity.

## Verified content contracts

- Inventory: 12 diagrams in 10 Chinese pages. Each has `accTitle` and `accDescr`; existing headings are retained and diagrams are paired with explanatory prose.
- Core objects keep projects optional, distinguish issue/task/config/runtime, and leave cross-workspace authorization to the dedicated explanation.
- Execution sequence distinguishes WebSocket wakeup/events from HTTP claim/report, and does not equate task completion with business acceptance.
- Architecture separates server persistence and local execution, keeps PostgreSQL authoritative, excludes CLI from a blanket realtime-client claim, and restricts Mobile sharing to types/pure functions.
- Task graph correctly permits `waiting_local_directory → running`, preserves old records on retry, and labels itself a main-path summary rather than an exhaustive state machine. Manual rerun follows the source task's agent through the existing invoke gate (`RerunIssue`).
- Squad chart starts the leader, checks per-target admission, and keeps member result return conditional; the restricted-reviewer exception remains in prose.
- Human collaboration separates submission, initiator acceptance and revision, with the self-handler completion exception stated separately.
- Shared AI graph preserves owner consent, explicit resume and send-preview confirmation as distinct operations.
- Knowledge chart separates candidate review/confirmation from current runtime/deployment/permission/expiry eligibility. Confirmation is not proof of truth or proof that a run already used the knowledge.
- MCP chart distinguishes saved configuration, explicit binding and an available supported runtime; saving does not install tools or change Access.
- Triage distinguishes ordinary acceptance from explicit accept-and-execute; failed execution does not undo acceptance.
- Iteration closure preserves historical facts and directs unfinished work without changing task status or stopping AI; ordinary close does not start the next iteration.

## Verification

- Scoped ESLint: passed for `docs/docs-content.tsx`, `docs/docs-diagram.tsx`, `docs/docs-content.test.tsx`, `editor/mermaid-diagram.tsx`.
- Docs TypeCheck: passed (`pnpm --filter @multica/docs typecheck`).
- Views TypeCheck: passed (`pnpm --filter @multica/views typecheck`).
- Diff whitespace check: passed for all task rendering/docs targets.
- Tests: implementation agent reported 85 focused Views tests and 64 Docs tests passed; not repeated here because the coordinator requested no duplicate heavy verification.
- Live rendering, theme/viewport inspection, bundle identity and final visual disposition: coordinator-owned and not claimed by this source review.

## Spec synchronization recommendation

Record the standard Mermaid fence contract, public `remarkMdxMermaid` binding at both routes, in-app leaf/lazy/style boundary, diagram accessibility descriptions and real-render validation requirement in the existing docs-bundle spec. Build/parse tests alone do not establish legible labels or successful browser SVG output.
