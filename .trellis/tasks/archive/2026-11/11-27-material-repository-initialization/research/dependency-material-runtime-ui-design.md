# Research: Shared material UI with selected runtime execution

- Query: Replace the material GitHub/GitLab App/connection prerequisite with the user-selected underlying runtime, including publication, review synchronization, approved merge, observation and plan generation.
- Scope: internal; read-only product-code inspection, one research artifact.
- Date: 2026-11-27

## Findings

### 1. Current coupling is in the shared form and schema, not platform routing

`packages/views/projects/components/project-material-settings.tsx:38` loads the workspace Agent list; at40–41 it fetches GitHub installations/GitLab connections. The provider-dependent connection query joins the prerequisite gate at46–49. The selectable connection values come from installation/connection IDs at52–54, and valid saving requires `draft.connection_id` at62. The visible connection field at83 and empty-state copy at84 point the user to workspace integration setup. The form separately exposes `plan_agent_id` at92.

`packages/core/materials/schema.ts:14` currently requires `connection_id` and `plan_agent_id` in settings responses. The input schema is derived from that response shape at87, so deleting the UI control alone does not remove the wire dependency. `server/internal/material/types.go:21` defines the matching settings, at34 the input, and at199 validates a nonempty connection. The server binding/provider dependencies are covered by the other research lanes.

The whole feature already sits under shared ProjectDetail and uses shared paths, so this change requires no new Next.js or desktop route. Keep React Query as the owner of settings/material/execution facts; keep local settings edits and the existing scoped pending-request store as client state.

### 2. Select the runtime directly; use an internal carrier only where task plumbing needs one

The final recommendation follows the user's chosen product model and the parent coordinator's design decision: one public `execution_runtime_id` for **all material stages**. Remove both the connection selector and the planning-Agent selector. Do not replace them with two user-facing Agent/runtime pairs, and do not silently rebind a user-owned Agent.

Existing data distinguishes runtime from Agent:

- `packages/core/types/agent.ts:75`: RuntimeDevice identifies the execution instance, daemon/machine, provider, owner, visibility, mode and current status.
- `packages/core/api/schemas.ts:1694`: the parsed runtime requires identity, workspace, runtime mode/provider/status and owner; metadata is an open map. `online` is a liveness observation, not a Git-authentication assertion.
- `packages/core/api/schemas.ts:2179`: an Agent has its own `runtime_id`, instructions, model and invocation policy. An existing Agent picker would add a product requirement the user did not ask for.
- `server/internal/handler/agent_builder.go:53`: the existing direct-runtime Agent builder uses a hidden system Agent because ordinary task/chat plumbing is Agent-backed. It creates the carrier at107 with the selected runtime and optional model. This is a precedent for an internal material carrier, not a reason to show another Agent selector.

Use a material-specific internal carrier identity, separate from agent_builder identity. Runtime switch must create/bind the new material configuration without mutating shared Agent definitions. Freeze actual runtime identity/configuration into each operation attempt; the current project selection is not allowed to reroute an old in-flight attempt.

The planning stage still keeps its existing issue and auto/manual start semantics. The removed `plan_agent_id` is no longer an independent user choice: the internal carrier executing on `execution_runtime_id` performs plan generation too. The server may retain internal carrier/task identifiers for audit but does not expose them as setup requirements.

### 3. Reuse the existing explicit picker, not its auto-selection variant

Use `RuntimePicker` from `packages/views/agents/components/inspector/runtime-picker.tsx:43` with `variant="field"`, `compact`, `showLabel={false}`, the settings draft value and a purpose-specific outer label. This component groups instances by machine/provider and already renders read-only and online/offline state; it is already used inside a project configuration form at `project-squad-picker.tsx:142`.

Avoid the create-Agent picker from `agents/components/runtime-picker.tsx:32`: its effect at91 picks the first usable runtime when empty, and toggling Mine/All changes selection at102. That behavior would silently select an execution identity during material reconfiguration. The inspector variant's Mine/All filter is display-only and its `onChange` follows explicit runtime row activation.

Use `runtimeDisplayLabel`/existing machine labels, showing e.g. `Codex · <machine>` in configuration and current operation summaries. Preserve an unavailable/deleted selection as a named unavailable state until the user explicitly chooses a replacement; do not silently pick the next connected machine. Do not show the internal carrier name in product copy.

Runtime selection permission reuses `isRuntimeUsableForUser` (`packages/core/runtimes/access.ts:21`) and server `canUseRuntimeForAgent` semantics: private runtime is owner-only, even for workspace admins; public runtime requires workspace membership and a nonempty owner. `eligibleProjectRuntimes` (`packages/core/projects/execution-squad.ts:48`) adds current-user/workspace checks. Do not treat an unresolved viewer as write-ready just because the generic runtime access predicate permits display during auth loading.

Only apply project-local-directory machine restrictions when the actual material stage needs such a directory. A remotely associated material output repository is not automatically the project's code checkout. Do not exclude valid Git execution hosts merely because another resource points to a different local machine; backend preparation determines required resources and machine coverage.

### 4. Proposed minimum public settings contract

Continue using the existing `GET/PUT /api/projects/{id}/materials/settings` and versioned original-request writes. Proposed persisted/public shape:

```ts
type MaterialSettings = {
  version: number;
  enabled: boolean;
  provider: string; // github | gitlab when writing; future values display unavailable
  resource_id: string;
  repository: string;
  base_branch: string;
  root_path: string;
  reviewer_ids: string[];
  execution_runtime_id: string | null;
  auto_start_plan: boolean;
};
```

- Remove `connection_id` and `plan_agent_id` from new request construction, draft input decoding, form state and backend configuration/binding logic. No App, installation or GitLab connection list belongs in form prerequisites.
- Retain provider as Git platform selection, not Agent provider. Its GitLab instance/host is resolved from the selected project repository resource; do not add a token field. Runtime `provider` still means Codex/Claude/etc.
- Enabled runtime workflow requires an explicitly selected usable runtime ID. A disabled/incomplete configuration may represent null; enabling it must fail clearly until selected. Existing stored App-based settings with no runtime must read as `execution_runtime_id: null`, preserve history and require an explicit runtime selection. Do not infer a runtime from old `plan_agent_id` or auto-launch pending intents during migration.
- Allow a previously configured, currently offline runtime identity to remain saved. Saving configuration is not launching work. The server rechecks authority/capability at actual dispatch; the UI says “Offline—operations will wait” rather than “ready.” If backend chooses not to allow a newly selected offline runtime, this is a save-time availability rule, not permission to discard the previous choice.
- Retain `request_id`, `expected_version`, current-authorizer ownership, reviewer list and explicit `auto_start_plan=false`. Re-enabling planning must continue to leave old manual holds untouched.
- No model picker is necessary for this narrow change: the internal carrier can leave the model unset and use the selected runtime/provider default, as the builder request's optional model permits (`agent_builder.go:42`). Record the effective model/config on execution. Do not invent model IDs or import a shared user Agent's model. If an explicit override is later needed, reuse `runtimeModelsOptions` and existing model picker; failed/unsupported discovery must not manufacture an empty-but-valid catalog (`packages/core/runtimes/models.ts:68`, `builder-workspace.tsx:105`).

Installed-client compatibility needs a deliberate boundary. New client response parsing should tolerate an omitted `execution_runtime_id` as **unconfigured**, never as an automatically inferred execution target. An older server can therefore be read safely, but runtime writes require the new supported contract; expose a clear unsupported/update response instead of restoring the App fallback. If existing deployed clients require retired response keys, retain them only as inert response compatibility fields, not required configuration, authorization inputs or dual writes. Root decides whether such deployed-client compatibility is needed; the first material implementation was only locally committed, so no historical production usage is established by this inspection.

### 5. Availability and capability presentation

Use a small, canonical core helper only if multiple material views need the same derived state. Distinguish:

| State | Product behavior |
| --- | --- |
| Runtime inventory loading or failed | Keep current draft and selection; block fresh selection/save that depends on unknown inventory; retry the exact read |
| No runtime selected | Prompt to choose an execution runtime; no App setup CTA |
| Runtime deleted, foreign, private to another user or owner missing | Show unavailable; require explicit replacement; do not reroute old operations |
| Runtime offline/stale | Keep configured identity; show wait/blocker and no fake running state |
| Runtime online but material protocol unsupported | Show server-reported upgrade/capability blocker; do not assume provider name or CLI version proves support |
| Runtime busy / queued | Show actual queued operation, not “running” |
| Git CLI missing, host authentication missing, repository denied | Show operation failure/blocker with remediation on the selected machine; do not suggest GitHub App installation |
| Runtime changed after operation frozen | Display the old attempt's runtime; block/resume only by defined server policy, not a UI substitution |

The daemon capability metadata contains string capabilities (existing example `local-worktree-v1` in `packages/core/runtimes/cli-version.ts:153`). That helper explicitly treats metadata as a positive hint rather than a global UI gate because absent metadata may mean old server or old row. Follow that lesson: use a new server-enforced material capability contract and a precise operation blocker; do not infer support from arbitrary version strings. Git login and repository permissions must be checked by actual runtime operations and surfaced through sanitized results.

Current `MaterialDetail` only exposes material/plan states and issue ID (`packages/core/materials/schema.ts:40`), while publication/merge operations have no public actual-task identity. Add an optional read-only current `operation` projection if the backend is adding operation attempts: `{id, kind, status, task_id, execution_runtime_id, blocker}` with strict string/null parsing and safe unknown status rendering. A generic task “completed” state must not display “merged”; verified remote facts still determine material/baseline state. Preserve the existing plan issue link, and do not navigate a user to a hidden carrier Agent page.

### 6. Protected reads, replay and API drift

`materialSettingsOptions`/`materialDetailOptions` in `packages/core/materials/queries.ts:13` already use project access guards and workspace+project keys. The existing mutations at28/41/51 use `networkMode: "always"`, `retry: false`, await persistence and preserve original request IDs. Keep these contracts.

`runtimeListOptions` (`packages/core/runtimes/queries.ts:56`) currently does not use `withQueryAccessBoundary`, unlike member/Agent reads at `packages/core/workspace/queries.ts:49`. The new retained runtime picker must protect its exact Query. Prefer a narrowly scoped material runtime options wrapper using the canonical runtime key and protected query function; do not clear the entire project merely because the runtime inventory itself returns403. A transient error retains the mounted form, while permission loss removes protected inventory and disables its dependent controls.

The pending settings request is persisted as unknown JSON and decoded on remount (`materials/draft-store.ts:10`, `material-controls.tsx:25`). The new decoder must not strip an old `connection_id` body and silently resend it as a different runtime request using the original request ID. Retired/unsupported pending shapes need an explicit unavailable/reconcile message; preserve original bytes for inspection, refresh authoritative settings, and require a new deliberate configuration intent after certainty. Ordinary current-schema uncertain writes replay exactly as before.

`getSettingsQueryState` continues to retain the actual focused editor node through500 responses and to hide access-denied data. Version-conflict rendering at `project-material-settings.tsx:94` already demands reload; runtime identity changes must participate in the same CAS contract. A late settings response from an older version cannot overwrite the chosen runtime.

### 7. Minimal production file map

| File/area | Needed change |
| --- | --- |
| `packages/core/materials/schema.ts` | Runtime setting/input and optional operation projection; old/unknown schema fails safely |
| `packages/core/materials/queries.ts` | Runtime prerequisite read wrapper if needed, invalidation covers new execution projection; keep current write recovery |
| `packages/core/materials/draft-store.ts` / pending decoder | Only if schema-version-aware original-request handling needs a targeted migration; do not clear editor content broadly |
| `packages/views/projects/components/project-material-settings.tsx` | Remove App/VCS/Agent prerequisite queries and controls; use one explicit runtime picker; keep platform/resource/branch/root/reviewers/auto-start |
| `packages/views/projects/components/material-controls.tsx` | Runtime/tool/auth/unsupported blockers, retired request notice; no integration setup guidance |
| `packages/views/projects/components/material-detail.tsx` | If operation projection added, show actual selected runtime and current queued/running/result facts |
| `packages/views/locales/en/projects.json`, `zh-Hans/projects.json` | Runtime labels and machine-specific failure copy; remove connection/plan-Agent setup copy |
| Relevant server type/service/handler/SQL | Backend lanes own removing connection/plan_agent requirements and adding frozen runtime/carrier/operation binding |
| Built-in material skill/source-map | Describe runtime auth and current task receipt protocol; no App prerequisite |

No change is required in `packages/ui` or platform routers. Existing runtime administration/Agent editors and unrelated GitHub/App integrations remain independent; do not remove App support from unrelated product features to fix the material workflow.

### 8. Verification matrix and canonical owners

- `packages/core/materials/schema.test.ts`: both providers without connection_id; runtime identity/workspace checks; missing runtime degrades to unconfigured; malformed runtime/operation fields; unknown statuses cannot enable action; explicit false; exact scoped request contains runtime and omits retired keys; secrets never in parsed response.
- `packages/core/materials/draft-store.test.ts` or focused decode suite: old pending request is not silently transformed/replayed; new-schema original request remains byte/field equivalent; account/project cleanup still erases protected drafts.
- `packages/views/projects/components/project-materials.test.tsx`: only one runtime choice; no GitHub-installations/VCS-connections/Agent-list fetch prerequisite; same selected runtime on save; offline/removed/private/unknown-capability visible states; no automatic reselection when inventory changes; pending/failed inventory retains input;403 hides private candidates; explicit false survives refresh; current version conflict; duplicate submit guard; mounted focus and offline explicit retry regressions survive.
- Existing picker tests remain its canonical keyboard/grouping/access matrix; add one material wiring test with real picker semantics rather than duplicating all picker helper cases.
- Web/Desktop E2E: configure either GitHub/GitLab with no server App credentials or connection rows, choose a fake registered runtime in deterministic tests, observe frozen operation runtime and state, keep manual plan hold, verify approved merge result through backend receipt contract. Live selected-runtime execution is separately gated/recorded; a fake queue is not live acceptance.
- Existing backend capability tests must prove no runtime token can configure/review/self-approve and that unsupported/stale runtimes do not receive material operations. Root/backend lanes own these; UI hiding is not enforcement.

## Related Specs

- `.trellis/spec/views/frontend/settings-recovery.md`: retained authorized inputs, exact-query denial, write persistence and original intent.
- `.trellis/spec/server/project-execution-squad.md`: runtime binding versus effective machine, no shared-Agent rebinding, configuration is not liveness.
- `.trellis/spec/server/git-materials.md`: immutable versions, human review, actual baseline, manual hold and task-authority fences.
- `.trellis/spec/core/frontend/index.md`, `.trellis/spec/views/frontend/index.md`: React Query ownership, core/views boundaries and settings/access contracts.
- `CLAUDE.md`: schema parsing for installed clients, no new dependency, shared Web/Desktop business UI, real-agent test opt-in.

## External References

No new external API dependency was researched for this UI topic. The linked `research/runtime-git-direction.md` retains the already checked official GitHub CLI authentication/merge references. All file/line evidence above refers to the local checkout inspected on2026-11-27.

## Caveats / Not Found

- The exact material operation capability name and receipt schema do not exist yet. Do not implement UI “supported” from an invented metadata string before backend/daemon agreement.
- Hidden carrier creation is reusable as a pattern, not a drop-in copy of Agent builder lifecycle. `server/pkg/db/queries/runtime.sql:384` deletes system Agents on runtime removal; material audit history and operation references need explicit backend retention handling.
- Existing `resolveBuilderRuntime` comments mention owner/admin but the actual predicate and error at `agent_builder.go:365` enforce owner-only private runtime. Follow executable permission policy, not that stale comment.
- No tests, running applications, Git actions, credentials, external writes or product edits were performed. This file is the only written artifact.
