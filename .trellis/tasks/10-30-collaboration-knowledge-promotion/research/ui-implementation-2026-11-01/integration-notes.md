# B2 shared UI integration

Mode: Operate. This is an extension of incumbent collaboration panels and context/settings surfaces. The implementation introduces no application route, provider, dependency, generated asset, model call, or backend change.

## Entry points and behavior

- `CollaborationDetail` mounts `CollaborationKnowledgePanel` after shared context. It lists every candidate version through Core's explicit page boundary. Selecting a row opens inline, live `KnowledgeReview`; the page never copies candidate body into component state. The history disclosure respects required `history_truncated` and links older entries by candidate ID.
- `ProjectDetail` mounts `ManagedKnowledgePanel` after the human description area. `WorkspaceTab` mounts the same panel separately from its human context editor. Neither panel modifies or merges content into base editors.
- The source panel exposes **Propose knowledge** only for the server's `can_propose:true`. Destination defaults to the legally visible source workspace; the already-visible current project is another choice. **Another visible destination** opens the viewer's normal workspace list, then the chosen visible workspace's existing project query. The external-handler test proves current-source destinations make no workspace/project catalog calls.
- A selected source project is stored by ID. If the source relationship changes, the draft keeps its text and requires explicit destination selection; it never silently targets the new project.
- Proposal fields are human-written summary, Markdown conclusion, source checkboxes, optional typed test/commit evidence and verification claim, applicability, and optional expiry. Empty memory is not a prerequisite: **Comment** and **Run record** remain independent source choices. No JSON editor or AI summarizer exists.
- **Propose correction** creates a blank new proposal with the exact predecessor ID and fixed original target. **Propose workspace version** is available for an approved project candidate and creates a separate workspace proposal using the project's workspace plus `derived_from_candidate_id`. Neither path copies old body into retained form state or mutates the prior contribution.
- Approval requires the currently displayed full available candidate, server affordance and an explicit review checkbox. Review identity includes candidate ID, state revision, content version/hash and availability; replacement/unavailable projections unmount the decision controls and reset confirmation. Every action awaits Core's exact receipt. Known refusal has alert feedback, not success messaging.
- Source-unavailable pending rows retain permitted rejection without body/hash/refs. Restricted target-only rows render control identity/state and permitted withdrawal only: no source link, body, reject or approval.
- An uncertain action retains its immutable in-memory intent and offers **Check original result** / **Retry same request**. Core preserves only original client-generated recovery identity in StorageAdapter. After reload, only GET recovery is available; no private payload is reconstructed or automatically reposted. Missing results do not fabricate completion.
- Unsupported knowledge routes render a scoped unavailable message while the incumbent collaboration page remains intact. Unknown/malformed reads never become a fabricated empty list.

## Exact source reading

`buildKnowledgeSources` in Core resolves selected kind/issue/version or run-record ID using existing authenticated APIs, computes SHA-256 over the exact decoded text (no trimming/normalization), and returns typed refs only. It runs under the existing source/access guard and Query cancellation, with the shared 10-second request deadline through body parsing/hash completion.

- Memory uses `getCollaborationMemoryVersion`, including a lawful earlier version after latest advances; purged/null content refuses.
- Comment first reads the current authorized comment to reject tombstones, then uses current text or its retained exact history `after`/next-revision `before` text. The existing history SQL has no truncating LIMIT. Conflicting/missing history refuses rather than replacing it with latest text.
- Run records follow the existing paged reader to the exact immutable record ID. Cursor cycles and timeout refuse; there is no arbitrary page-count cutoff and no guessed hash. The server remains responsible for source restoratability/current authority again in the write transaction.

Memory choices expose exact historical versions. Comment and run choices retain their selected reference when the current list advances. Review source links use existing account memory-version, comment and run destinations; no new route contract was added. All text displayed by the picker comes from its active authorized queries. Component state retains IDs/selections and user input, not source bodies.

## Verification and main handoff

Pure builder contracts are tested once in Core. The shared UI suite covers real query/action wiring, keyboard confirmation, pending receipts, stale review reset, private-body clearing, reload recovery, external-handler comment/current-project proposal, correction/derivation identity, restricted withdrawal/refusal, source run selection, all-version paging, changed project selection and unsupported/history states. Existing shared page suites assert each mount's source/target identity; locale parity includes both new dictionaries.

Dependency setup reused the already-installed pnpm tree. Views/UI node_modules have local workspace aliases so Core imports resolve this worktree, while external package symlinks point at the existing installation. No package manifest/lockfile changed. The existing Core test-fixture entry point re-exports knowledge fixtures so shared tests respect package root boundaries.

Main owns the single mechanical Impeccable detector pass and independent screenshot review, production Web and native Desktop test setup, real HTTP/source-auth integration, and any final correction batch. This lane did not run DB tests or browser/platform acceptance. Approval/UI tests do not prove G2 runtime injection, session exposure, model understanding, or source fanout delivery. Existing project-detail lint hook warnings and its NavigationProvider test act warning are retained in logs; this change does not edit those hook/deletion paths.
