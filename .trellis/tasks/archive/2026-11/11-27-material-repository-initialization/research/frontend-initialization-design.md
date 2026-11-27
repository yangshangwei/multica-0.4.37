# Research: shared material repository initialization UI and API

- Query: Where should the initialization/maintenance flow attach to shared Web/Desktop material settings, how should its state and authorization be represented, and what gates ordinary publication without losing drafts or history?
- Scope: internal. Research-only inspection of the current checkout and the copied runtime-execution dependency design; no product edits, tests, database or remote operations.
- Date: 2026-11-27

## Findings

### 1. Existing shared seams and factual limits

The initialized layout and selected-runtime workflow do not exist in this checkout yet. The dependency document is a proposed runtime contract, not an available implementation. Frontend code should consume the final dependency contract rather than reintroducing a second runtime or App selector.

| File | Existing responsibility / relevant anchor |
|---|---|
| `packages/views/projects/components/project-materials.tsx:20` | Shared, viewer/project-keyed root; project denial hides it. The settings query is at34, route-selected material at43, settings/detail/list split at60, new-draft action at66. |
| `packages/views/projects/components/project-material-settings.tsx:20` | Existing repository/workflow form; currently connection and plan-Agent dependent. Runtime migration is owned by the other task. |
| `packages/views/projects/components/material-detail.tsx:52` | Current review snapshot pins revision ID/head/digest, independently of polling; baseline and plan actions appear around82–101. |
| `packages/views/projects/components/material-revision-editor.tsx:22` | Retained version-bound editor draft. `ready=false` defaults, and the ready checkbox at68 requests publication; no initialization gate exists. |
| `packages/views/projects/components/material-controls.tsx:25` | `useMaterialAction` persists the exact body/request ID before a write; ambiguous results lock the intent, and explicit retry reuses it. |
| `packages/core/materials/schema.ts:14` | Settings schema; detail/capability schema at44, commands at78, redacted scoped parsers at110, exact revision approval helper at141. |
| `packages/core/materials/queries.ts:7` | Workspace+project material key hierarchy and protected reads; retry:false/networkMode:always mutations at28,41,51. |
| `packages/core/api/client.ts:6557` | Existing material settings/list/detail/command transport, with captured workspace headers and schema parsing. |
| `packages/core/materials/draft-store.ts:10` | Editors and unknown pending requests only, keyed by viewer/workspace/project/subject; protected cleanup at32. |
| `packages/core/projects/access.ts:37` | Action-level 403 allowlist distinguishes a denied action from lost workspace/project access. Protected request wrapper at121 fences late responses. |
| `packages/views/settings/components/settings-query-feedback.tsx:13` | Authorized retained data survives temporary read failure; active access denial overrides it. |
| `packages/core/paths/paths.ts:46` | Shared project detail section and material URLs; no new platform page is needed. |
| `packages/views/agents/components/inspector/runtime-picker.tsx:43` | Explicit machine/runtime selector; unlike the create-Agent variant, it does not auto-select the first runtime. |
| `packages/views/editor/code-block-static.tsx:28` | Existing read-only source display usable for JSON/YAML/Markdown rule-file inspection, without executing embedded content. |

Package contract remains core=headless queries/types, views=shared UI, app=platform wiring. This task should not create a store in views or duplicate Web/Desktop components.

### 2. One initialization surface within Engineering materials

Add a shared `ProjectMaterialInitialization` section directly below the material page header and above the existing list/detail content. It remains visible for the selected detail as well, so a deep-linked editor can explain why publication is blocked. It is not a fake requirement row and must not be mixed into `MaterialDetail.revisions`.

The section has two compact modes:

1. **Summary:** repository/root, configured runtime label, verified current format/template bundle version, initialization state, last verified commit/time, current blocker and appropriate primary action.
2. **Candidate review:** initialization or maintenance candidate, full exact file list and content comparison, pinned HEAD/bundle digest, permitted decisions, and expandable immutable history.

Configuration is still one existing form. Once runtime settings are saved successfully, the server schedules a check in that same accepted configuration flow; frontend does not use an effect or mutation-success callback to create a second check job. The UI reads the returned/invalidated status. A reload must never be a remote-write trigger.

Suggested user-facing language:

- First configuration save hint: “保存后，所选运行时会检查物料仓库。缺少基础文件时将生成初始化变更，审核合并后才能发布需求。”
- Initial panel title: “物料仓库初始化”. Maintenance candidate title: “规则与模板维护”.
- Read-only member explanation: “仓库初始化需要维护权限；你仍可整理需求草稿。”
- Successful check with no diff: “基础结构已核对，无需创建初始化评审。”
- Successful verified merge: “初始化完成”，with the actual verified commit beneath it.
- A raw runtime/task completion must never render that completion label.

Do not invent setup-generated product releases, example business modules, or fake completed acceptance documents in the panel. Show required initial files and a brief description that releases/requirements/issues are created with real content later. Existing UAT directories are retained, not silently renamed into the schema layout.

### 3. Separate readiness, operation progress, and review history

A single status string cannot describe both an already usable repository and an optional maintenance candidate. Propose the following schema-level model; exact names must be frozen with the backend owner before implementation:

```ts
interface MaterialInitializationEnvelope {
  workspace_id: string;
  project_id: string;
  protocol_version: 1;
  version: number; // monotonic initialization aggregate/CAS version
  binding_generation: number; // repository/root/runtime binding generation
  status: string; // see known display states below
  blocker: string | null;
  readiness: {
    publish_allowed: boolean;
    verified_commit: string | null;
    manifest_sha256: string | null;
    format_version: number | null;
    bundle_version: string | null;
    verified_at: string | null;
  };
  operation: {
    id: string;
    kind: string; // initialization check/publish/merge/verify or maintenance
    status: string; // selected-runtime dependency's actual execution projection
    execution_runtime_id: string;
    task_id: string | null;
    blocker: string | null;
  } | null;
  candidate: InitializationCandidate | null;
  decisions: InitializationDecision[];
  capabilities: {
    can_check: boolean;
    can_review: boolean;
    can_retry: boolean;
    can_maintain: boolean;
  };
}

interface InitializationCandidate {
  id: string;
  kind: "initialize" | "maintenance";
  version: number;
  binding_generation: number;
  bundle_version: string;
  manifest_sha256: string;
  base_commit: string;
  head_sha: string | null;
  review_url: string | null;
  publication_status: string;
  files: Array<{
    path: string;
    action: string; // add | preserve | modify | conflict, fail closed otherwise
    before_sha256: string | null;
    after_sha256: string | null;
    before_content: string | null;
    after_content: string | null;
    explanation: string | null;
  }>;
}

interface InitializationDecision {
  id: string;
  candidate_id: string;
  actor_id: string;
  decision: string;
  reason: string;
  head_sha: string;
  manifest_sha256: string;
  created_at: string;
}
```

All file/text size limits must be authoritative and aligned with the template bundle; full initial bundle content is preferable to pagination for this bounded v1. If pagination becomes necessary, approval must remain disabled until the exact complete manifest and all selected review data are verified; do not approve only visible pages or silently truncate.

Known overall states and rendering:

| State | Display / action |
|---|---|
| unconfigured | Select repository/root/runtime in the existing settings form. |
| queued / checking | Show actual queued or checking fact and selected runtime. No new duplicate start button. |
| publishing | Runtime is preparing/publishing missing-file candidate; not yet reviewable. |
| in_review | Show complete candidate and permitted maintainer review actions. |
| changes_requested | Show reason/history and next correction/check action as returned by capabilities. |
| approved / merging | Approval exists; native gates or merge still pending. No “initialized” success. |
| verifying | Actual merge observed; contents/manifest still being verified. |
| ready | Verified binding and compatible bundle; ordinary publication may be allowed. |
| blocked / unknown | Show specific cause and permitted recheck/original-request recovery. Preserve prior facts as historical evidence. |
| unsupported/future value | Honest unavailable state; no authority-dependent operation. Historical data remains readable. |

`readiness.publish_allowed` is necessary but not sufficient for a fresh frontend publication action: require a recognized supported protocol, current matching binding generation, a valid verified proof (commit/digest/time/version), and ordinary material edit capability. The backend remains authoritative. Do not infer readiness from a non-null PR URL, `material.yaml` filename, task status, a previous unrelated repository's proof, or a default boolean.

A proposed maintenance update can retain the last accepted proof while awaiting review only if the server explicitly says the current bundle is still compatible and publish_allowed remains true. A binding change, unknown schema or invalid actual scaffold must make that false. This avoids frontend inventing whether all maintenance work blocks an otherwise usable repository; the aggregate exposes that policy explicitly.

### 4. Minimal HTTP contract and frontend integration

Keep initialization under the project material scope but outside individual material commands:

- `GET /api/projects/{id}/materials/initialization` → `MaterialInitializationEnvelope`; read-only.
- `POST /api/projects/{id}/materials/initialization/check` with `{request_id, expected_version, expected_settings_version}` → envelope. Used for explicit recheck/recovery or maintenance-version discovery; initial check is server-triggered by accepted settings save.
- `POST /api/projects/{id}/materials/initialization/decisions` with `{request_id, expected_version, candidate_id, candidate_version, binding_generation, head_sha, manifest_sha256, decision, reason}` → envelope.
- `POST /api/projects/{id}/materials/initialization/retry` with `{request_id, expected_version}` → envelope. Retries existing logical operation, not a new candidate/key.
- A deliberately requested bundle upgrade may need a separate `maintenance` command with an advertised supported target bundle version; it must reuse the same independent candidate/decision flow. Do not build an arbitrary file-upload or rule-code editor into this first UI.

The backend routing must register static initialization paths before a generic `{materialID}` loader. Bootstrap decision input never accepts actor ID, runtime ID, file content or repository identity from the browser. Those come from current authenticated actor and frozen server state.

Add `materialKeys.initialization(wsId, projectId)` underneath `materialKeys.all`, with `protectProjectRequest`, explicit workspace header, parsed schemas and retry:false. Poll only while known work is active or a displayed review requires new facts; use the existing detail interval (5s) if desired, without mounting a second observer just for labels. All mutations use `retry:false`, `networkMode:"always"`, monotonic cache replacement and project/session access fences.

Mutations invalidate initialization, settings and material details whose eligibility depends on initialization. Never optimistic-set ready or enqueue successor tasks in the UI.

Expose publication eligibility on the ordinary material detail as a **distinct** capability such as `can_publish` plus initialization/binding version, not by setting `can_edit=false`. Existing editor behavior must continue to save ready:false drafts. Once the initialization query fails or permission becomes unknown, draft editing can remain, while new ready:true submissions are prevented until safe current facts return.

### 5. Maintainer approval must be distinct from PRD reviewers

Current backend `materialTx.manager` at `server/internal/service/material.go:161` means authenticated human owner/admin. `Settings.CanConfigure` uses that at290. In contrast, `CanReview` at275 is membership in `reviewer_ids`, appropriate for PRD content. Reusing it would let a product reviewer approve AGENTS.md and executable agent rules without maintenance authority.

Smallest safe v1 recommendation: initialization/maintenance actions require current human workspace owner/admin **and** current project access, matching the existing configuration authority. The UI uses separately named capabilities returned by the new endpoint; it never derives initialization rights from `reviewer_ids` or merely from a local role string. A dedicated maintained-members list can be added later only if desired; it is not necessary to implement a new ACL in this task.

All project members with read access may see initialization state and the same repository file diff available within their project. Lack of maintenance capability hides/locks decision controls but leaves readable material drafts intact. On revoked project/workspace access, the existing project access boundary clears both server cache and material pending drafts.

Add an explicit action-level forbidden code such as `material_maintenance_permission_denied` to the command-refusal classification only when backend has already proved project readability. Otherwise a rejected maintainer operation would currently be interpreted as workspace access loss (`projects/access.ts:37–47`). Do not blanket-exempt all 403s: true revocation still clears protected content.

### 6. Pinned review, complete differences and retained inputs

Reuse the review-snapshot pattern from `material-detail.tsx:52–76`, with snapshot identity `(candidate_id, candidate_version, binding_generation, head_sha, manifest_sha256)`. Polling may update actual facts but must not silently move an open review or approve a newer candidate with the old reason. Changed snapshot requires an explicit “重新查看当前初始化变更” action before deciding.

Review layout can reuse existing primitives:

- Vertical file list with path, add/preserve/modify/conflict label and short explanation.
- Selected file's before/after source panes using `CodeBlockStatic`, with wrapping/scrolling and readable line boundaries. Pure additions show “新增文件” and full proposed content; preserved files clearly show no planned write.
- JSON/YAML/AGENTS/Markdown rules are shown as inert source, not an HTML preview or model-executable instruction surface. A normal Markdown preview can be optional for README, but raw text is the approval source.
- Summary counts correspond to the complete manifest. Existing unrelated files are summarized as retained, not imported into a second complete private repository browser.
- Approve button copy identifies initialization/maintenance, e.g. “批准本次初始化变更”; explain that this authorizes the selected runtime to merge only the shown changes after gates pass.
- History shows actual actor, candidate/head, decision, reason, time and the resulting verified commit separately.

Use `useMaterialAction` for one aggregate-level command slot keyed by `materialDraftKey(userId, wsId, projectId, "initialization")`. Persist only client command intent/reason and small pinned identity as needed; all candidate files, readiness and runtime facts remain in Query. Do not persist copied server manifests into Zustand.

Temporary GET errors must remain sibling feedback so the mounted reason textarea and current file stay intact. A mutation transport timeout/malformed success keeps exact original request ID/body locked. “重试原请求” resends only that; returning online is not a submission. “核对并恢复” is a distinct backend operation and cannot be offered as a way to skip unresolved client intent.

If protocol upgrade makes a persisted request undecodable, surface it as an incompatible pending operation; do not strip fields and silently resend under the same ID. This is already a risk in `useMaterialAction` because `decode(stored)===null` currently unlocks malformed legacy shapes. Coordinate the dependency's pending-request migration so initialization does not add a competing behavior.

### 7. Legacy and conditional rendering

Old servers do not provide this endpoint. Treat explicit404/405/501 as initialization unsupported, not “repository uninitialized and safe to start.” Preserve list/detail reads and authorized draft editing where possible; block initialization-dependent new publication in this new UI, with server version/update guidance. Do not restore the App prerequisite or silently assume an old successfully merged material proves scaffold readiness.

Prefer an optional initialization summary/protocol capability on the settings envelope to distinguish unsupported old server from supported-but-never-checked data. Missing new field defaults to unknown/unavailable, never true. Do not make the entire old settings payload unparsable solely because the initialization extension is absent; use a dedicated strict parser for the new endpoint.

Current settings/prd UI response schemas accept future string states but guard authority-dependent actions. Preserve that pattern for state evolution, while requiring real booleans, nonblank IDs, full SHA/digests, unique file paths, valid references and same workspace/project for initialization responses. An invalid partial file list is an error, never an empty no-change result.

For current-runtime migration: preserve the dependency's execution_runtime_id selection and missing-runtime repair state. Existing historical reviews/baselines stay readable. No automatic assignment of an existing repo's documents to a product release; no rewrite of existing UAT paths.

### 8. Minimal file ownership map

New files keep most initialization changes separate from the other task:

| File | Change |
|---|---|
| `packages/core/materials/initialization-schema.ts` | New scoped parser, input decoder, state/snapshot readiness helpers; re-export from index. |
| `packages/core/materials/initialization-queries.ts` | Protected query + original-request mutations under existing material keys. |
| `packages/core/api/client.ts` | Narrow initialization transport methods and schema imports. Coordinate same-file insertion with runtime owner. |
| `packages/core/materials/queries.ts` | Add initialization key/invalidation seam if kept in central key object. |
| `packages/core/materials/schema.ts` | Optional protocol summary and distinct normal can_publish capability. Coordinate runtime owner. |
| `packages/core/projects/access.ts` | Explicit maintenance action refusal classification, with tests. |
| `packages/views/projects/components/project-material-initialization.tsx` | Summary, operation/ready state, permissions, history and commands. |
| `packages/views/projects/components/material-initialization-review.tsx` | Bounded full file comparison and pinned maintenance review form. |
| `packages/views/projects/components/project-materials.tsx` | Shared panel placement; preserve list/detail/draft navigation. |
| `packages/views/projects/components/material-revision-editor.tsx` | Separate draft save from initialization-gated ready checkbox/submission. |
| `packages/views/projects/components/material-detail.tsx` | Honest blocker and plan-start eligibility with current init state. |
| `packages/views/projects/components/project-material-settings.tsx` | Save-and-check copy and optional status summary only after runtime-selection changes settle. |
| `packages/views/locales/en/projects.json`, `zh-Hans/projects.json` | Matching initialization/maintenance states, controls, errors and explanations. |

No new Zustand server-state store, platform route, dependency, public initializer CLI in the UI, or role framework is required. Add a shared path helper only if deep-linking an initialization review is needed; use `?section=materials&initialization=<candidate-id>` rather than Next/Electron-specific routing. Unknown/deleted candidate IDs should show current state/history without substituting authority silently.

### 9. Canonical tests and verification

Pure core tests (`initialization-schema.test.ts`, node environment):

- Wrong workspace/project, malformed booleans/digest/SHA, duplicate/invalid paths, candidate/decision reference mismatch, incomplete proof and unsupported protocol cannot enable publication or approval.
- Known ready proof/current binding versus stale binding; maintainable-but-ready versus incompatible maintenance state according to explicit server policy.
- Current review snapshot versus different candidate/version/head/manifest; unknown states readable but not actionable.
- API captures workspace header; unchanged original command bytes and request ID on explicit replay; schema diagnostics redact source text.
- Ordinary maintenance permission refusal preserves project data; actual access revocation removes init Query/mutation data and related client pending intent.

Shared component tests (`project-material-initialization.test.tsx` plus narrow existing-suite wiring cases):

- First settings save does not issue a second client-originated check; panel shows queued/checking result, not ready.
- Compatible no-diff ready response shows verified state and no PR; missing files show candidate and exact inert source.
- PRD reviewer without maintenance capability cannot approve; maintainer can approve exact snapshot.
- Pending/merging/verifying do not show initialized; unknown runtime/protocol/format blocks fresh writes.
- Ready:false draft save works while initialization incomplete; ready:true disabled; permission/eligibility change while editor mounted blocks submission without erasing text.
- A failing poll preserves actual reason textarea node/text/focus and file selection; denial clears protected content.
- Offline approval fails promptly; reconnect alone does nothing; explicit retry sends identical body; duplicate clicks only one write.
- New candidate/head arrives during review: previous text remains pinned and cannot authorize new version; explicit re-review required.
- Existing initialized repo with optional maintenance candidate reflects explicit server publication policy; current proof and maintenance candidate are displayed separately.
- English/Chinese locale parity and long file paths/content do not hide final actions.

One canonical Web/Electron journey with the dependency's deterministic fake runtime should prove settings save → one actual runtime check → missing-file candidate → maintainer review → real fixture merge → readback ready → ordinary publication allowed. A second fixture branch covers existing compatible bundle/no PR and same request recovery. Assert dispatch counts and remote file bytes, not only labels. Use real role/name locators for Base UI controls and review retained-input identity during failed polls. This is not an installed-desktop or real-account certification.

Appropriate checks once implemented: core/views typecheck+lint; owning schema/component suites and existing material/access regressions; shared route wiring; locale parity; production Web/Electron acceptance and required visual verdict. Do not duplicate the entire schema matrix through DOM mounts.

## Related specs

- `.trellis/spec/server/git-materials.md`: frozen revision approval, remote facts versus claims, explicit recovery, client input retention and protected reads.
- `.trellis/spec/views/frontend/settings-recovery.md`: retained exact Query state, access loss, awaited saves, focused input preservation and native accessibility verification.
- `.trellis/spec/views/frontend/component-guidelines.md`: shared routing and component boundaries.
- `.trellis/spec/core/frontend/index.md` and `.trellis/spec/views/frontend/index.md`: owning-layer test placement and state ownership.
- `CLAUDE.md`: React Query owns server state; core/views/app package boundaries; response drift parsing; canonical-layer tests; no new dependencies.

## External references

None required for this research. All recommendations derive from local code and the user-approved behavior; no unfamiliar third-party API is proposed. Existing React Query, Zod, Base UI and read-only source components are reused at repository versions.

## Caveats / Not Found

- The copied dependency design is still a proposal. It has not delivered execution_runtime_id or attempt/receipt contracts into this checkout. Final names and authority boundaries must align with its implementation before wiring UI.
- Current PRD does not name a dedicated engineering-maintainer role. Reusing human owner/admin is the least additional authority model; main should adopt it explicitly in design or resolve an intentional product policy difference.
- Product initialization status, candidate API and can_publish capability do not exist today. This document is a proposed implementable interface, not evidence they already work.
- Existing raw runtime picker has no material-capability proof by itself. Keep Git readiness and operation support in actual server/daemon results.
- Empty Git repositories need an explicit first-commit policy at backend/runtime boundary. UI must expose that preflight state rather than pretend a PR can already be created; do not add a hidden default-branch write.
- Full release management, arbitrary rule editing, old UAT conversion and schema migrations of existing custom content are outside this UI slice.
