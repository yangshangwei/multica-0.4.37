# Research: repository initialization backend and runtime integration

- Query: Where should a reviewed, idempotent repository initialization/maintenance lifecycle attach to current materials, and what can be implemented independently while selected-runtime dispatch is still being designed?
- Scope: internal source and copied dependency-design analysis; no Git operations, database access, runtime/model calls or remote writes.
- Date: 2026-11-27
- Checkout: `/Users/artisan/.codex/worktrees/material-repository-initialization/multica-0.4.37`

## Findings

### 1. Current behavior and concrete integration seams

| File/anchor | Current behavior and consequence |
| --- | --- |
| `server/internal/material/types.go:21` | Settings contain repository/resource/base/root and legacy connection/plan-agent fields. No initialization status or root identity exists. Runtime work owns changing these fields. |
| `server/internal/service/material.go:91` | `begin` checks actual credentials, membership, workspace and project under transactions. Reuse this authorization/locking shape; initialization does not justify a new credential classifier. |
| `server/internal/service/material.go:155` | `human` rejects machine actors; `manager` at161 requires human owner/admin. Settings at416 already use this manager authority. This is the most conservative available initial maintenance-review role. |
| `server/internal/service/material.go:179` | Existing request IDs are scoped by workspace/project/actor and bind action+body hash. Same ID/different body conflicts. Reuse this mechanism for explicit initialization/retry/decision commands. |
| `server/internal/service/material.go:416` | `Configure` locks, validates CAS, freezes binding and saves settings, then commits. Persist an initialization inspection intent in this same transaction after a first valid selected-runtime binding; dispatch only after commit. Do not call Git synchronously here. |
| `server/internal/service/material.go:448` | Repository/root/provider/base/resource changes are denied when material history exists. Initialization history must join this test, otherwise a repository with only rule-review history remains freely retargetable. |
| `server/internal/service/material.go:491` | `Create` creates only a database material and allocates `root/requirements/<uuid>/prd.md`. Preserve draft creation while initialization is pending. |
| `server/internal/service/material.go:536` | `Revise` validates a candidate, freezes settings and creates a revision. `ready=true` at608/644 enqueues publishing. This is the first ordinary remote-publication readiness gate. Draft saves must still work. |
| `server/internal/service/material.go:662` | `Decide` binds current material version, revision, configured human reviewer, config version and remote HEAD. Reuse principles, not its `git_material_revision` identity for initialization. |
| `server/internal/service/material.go:794` | `Command` can re-enqueue publication/review or start planning. Gate this path too; initialization checks only in the form or `Revise` can be bypassed by old retries. |
| `server/internal/service/material_worker.go:47` | Current worker chooses only `plan` versus material Git work. `workerTx` at77 always loads real material/revision rows. It cannot safely run repository-only initialization today. |
| `server/internal/service/material_worker.go:118` | Ordinary files are exactly PRD or design+plan under one material directory. Preserve this narrow file construction. |
| `server/internal/service/material_worker.go:128` | Ordinary review validation binds deterministic branch/target, regular-file mode and only the expected material paths. Do not widen this checker for AGENTS/template paths. |
| `server/internal/service/material_worker.go:148` | Final bytes are compared at immutable commit. Initialization needs the same completeness principle for all new/managed and preserved foundation files. |
| `server/internal/service/material_worker.go:353` | Current requirement merge creates requirement baseline and enqueues plan. Initialization completion must not enter this branch or create a fake requirement/plan issue. |
| `server/internal/service/material_plan.go:126` | Running-plan token authorization consumes exact task/current baseline. Add repository readiness to current-authority checks in the eventual runtime implementation, not just initial dispatch. |
| `server/internal/service/material_plan.go:143` | `checkMaterialRunAuthority` rechecks config, authorizing member, manager, repository and invocation. Correct place for the reusable ready-binding predicate. |
| `server/internal/handler/material.go:37` | Access context and `Cache-Control: no-store`; strict request decoding at56. Use same handlers' conventions for project-level initialization endpoints. |
| `server/internal/handler/material_capability.go:16` | Existing task ceiling assumes plan issue/task. Runtime dependency already plans issue-less material operations; initialization must be explicitly recognized, never allowed via fallback when IssueID is absent. |
| `server/pkg/db/queries/git_material.sql:63` | Logical jobs require material_id+revision_id and deduplicate `(revision_id,kind)`; not yet a repository-initialization job abstraction. |
| `server/pkg/db/queries/git_material.sql:90` | Project history only sees `git_material`; workspace guard at98 has the same limitation. Add initialization/review/receipt history to retention checks. |
| `server/internal/handler/project.go:920`, `server/internal/handler/workspace.go:1150` | Existing delete guards preserve material history. Extend to initialization-only history; do not leave orphaned audit rows or delete a bound project with an initialization review. |

Current `checkMaterialBinding` (`material.go:355`) still resolves server-side App/VCS identity. That is being replaced by the other task. Do not introduce new initialization code depending on those legacy connection rows.

### 2. Safe independent implementation slice

Implement a new `server/internal/materialrepo/` package and embedded foundation assets, with no imports of service, handler, database, daemon, HTTP clients or runtime executors. It should own only a deterministic layout/catalog and pure inspection/proof rules. This avoids competing edits in the runtime task and provides concrete reusable code/tests now.

Suggested responsibilities (names are local API recommendations, not frozen cross-task wire fields):

1. `BuildCatalog(systemID, root)` returns a sorted immutable foundation bundle, file modes, per-file digests, catalog/version identity and overall digest. The caller supplies the stable system/root identity; the package does not allocate random IDs or timestamps on repeat calls.
2. `Inspect(catalog, snapshot)` validates a complete fixed-commit tree snapshot and relevant full file bytes, classifies every required path as missing/compatible/preserved/conflicting, and returns a deterministic create-only initialization plan.
3. `PlanDigest(plan)` binds target root, catalog version, system identity, source tree/commit, exact addition paths and bytes, preserved-file proof and operation kind. The same logical input has the same digest independent of input map order.
4. `VerifyPublication(plan, observation)` validates the exact initialization PR/MR identity expectation, full changed-file set, regular-file modes and source bytes. The caller verifies runtime/task/credential provenance.
5. `VerifyMerged(plan, observation)` proves actual merged state, expected candidate HEAD, allowed target/files, full merged bytes and preservation constraints. It returns immutable verification data, not a DB `ready` mutation.
6. Maintenance diff planning is separate from initialization: it needs an explicit previous approved catalog/manifest and exact before/after bytes for each managed path. Do not use an `overwrite=true` flag on initialization.

Use an internal visible asset directory (e.g. `catalog/`) with an explicit path map to logical `.material/…` files, or an embed form that intentionally includes hidden paths. Existing compile-time asset patterns are `builtin_skills.go:16` and `builtin_autopilot_templates.go:27`. Tests must assert the expected catalog list so a missing hidden asset cannot silently disappear from a build.

The approved proposed initial bundle contains **20 substantive files**: README, AGENTS, material.yaml; six schemas; eleven templates. Logical `requirements/`, `issues/`, `releases/`, modules/contracts/specs/decisions/knowledge are described in material.yaml/README and represented through templates, but do not need empty directories or `.gitkeep`.

The package may use standard Go JSON plus the existing `gopkg.in/yaml.v3` dependency if the manifest is YAML. Parse strictly: refuse duplicate keys, trailing documents, unknown required format version, unexpected fields affecting paths, NUL/invalid UTF-8, traversal, case/path collisions and a mismatched system identity. Do not claim full JSON Schema validation unless an actual validator is implemented; a syntactically valid schema JSON is not proof that its examples satisfy every keyword. Typed metadata validation and exhaustive contract fixtures are a no-new-dependency option.

The catalog covers release scope/reference, module, requirement, issue and published baseline contracts. It must not create a fake v1.8.0, requirement, issue, acceptance result or deployment artifact. Template placeholders are valid template content, not valid completed business material.

**Completion boundary:** the pure package can be implemented and verified now, but it is an independent deliverable, not a completed initialization feature. Keep the task active until configuration trigger, durable review, runtime execution, readback, readiness gate and shared UI are integrated.

### 3. Existing-file compatibility and preservation

Recommended conservative policy:

| Existing path | Initialization behavior |
| --- | --- |
| Missing regular-file slot with safe tree ancestors | Add the catalog file at that exact path. |
| README.md or AGENTS.md already exists as an ordinary readable UTF-8 file | Preserve exact bytes; show it as preserved, not an automatic edit. Do not pretend its text is the generated catalog. It never grants runtime authority or changes the server allowlist. |
| Known material.yaml at supported format, same stable system identity and directory contract | Reuse. A generated default must not replace it. |
| material.yaml with future/unknown format, different system ID or directory remap | Conflict; block automatic adoption and expose concrete incompatibility. |
| Schema/template exactly matches a recognized supported catalog or an explicitly approved maintained manifest | Reuse and pin digest. |
| Existing schema/template differs without recognized maintained provenance | Conflict; preserve bytes and require independent maintenance review. Do not decide that a same filename means compatibility. |
| Expected file is a directory, executable, symlink or submodule | Block this path; never follow it or silently replace it. |
| Any ancestor is a file/symlink/submodule or tree enumeration incomplete | Block; absence must be established by a complete fixed-tree observation. |
| Unrelated business files, README outside the selected root, uat data | Preserve unchanged. They are outside the initialization write allowlist. |

Preserving arbitrary README/AGENTS is compatible with the user's existing-content requirement; it is not a finding that all arbitrary rule text is semantically compatible. Product-owned runtime instructions and server authority remain authoritative. If generated navigation must be added to an existing README/AGENTS, propose a separate reviewed maintenance diff; initial automatic inspection must not append content secretly.

Build the complete repository candidate tree as source tree plus explicit additions. This both avoids loading all unrelated bytes and permits an exact structural preservation proof. Verify non-target entries by tree/blob identity, not only that the approved new files exist. A merge may legitimately include unrelated target-branch advances; preflight re-observation must distinguish those advances from extra changes in this candidate's complete diff. Do not blanket reject every future main commit after readiness, since normal demand publication itself advances main.

### 4. Initialization aggregate and durable state

A repository initialization is scoped to a project binding/root, not to a requirement. Recommended distinct aggregate:

- Current root record: workspace/project, stable root/system ID, normalized repository identity/host/resource fingerprint, base branch/root, format/catalog version, current initialization revision, state/blocker, immutable ready receipt reference, row version.
- Immutable initialization revision: ID, `kind=initialize|maintain`, config snapshot, catalog/version, source commit/tree, exact proposed files, before/after digests, preserved relevant files, plan digest, creator and created time.
- Append-only decision: actual human/member/credential snapshot, initialization revision, candidate HEAD/plan digest, approve/request_changes/withdraw, reason and timestamp.
- Logical execution jobs and attempt/receipt records: reuse the runtime task's queue and observation machinery with an explicit initialization target. Do not make a `git_material` called “初始化”, use `kind=plan` with rule content, or mint placeholder material/revision IDs.

The currently proposed runtime descriptor still names material+revision IDs and the existing SQL requires them. **Dependency contract must support an explicit target discriminator**, for example `target.kind=material_revision|repository_initialization`, a real root/init revision identity for the latter, plus an exact content allowlist/digest and operation kind. Actual field names should be frozen jointly with that task; do not copy its draft wire shape and silently invent another scheduler.

Suggested logical stages:

`inspection_pending → inspecting → publication_pending → in_review → approved → merge_pending → verifying → ready`

Additional durable results: `changes_requested`, `blocked`, `outcome_unknown`, `superseded`. “Already compatible” skips publication/review after a trusted observation proves the existing supported state; store a ready verification receipt with adoption provenance rather than fabricate a merged PR. Existence of a file named material.yaml is never enough.

First settings save persists one inspection intent in the same transaction. Repeated identical save/request returns original result. Reinspection/retry continues the existing intent until its outcome is settled; new candidate only for changed desired catalog, binding or deliberately revised maintenance content. One active attempt per logical job. Queue lease expiry must not create a second running initialization model task.

Ready proof should bind repository/root/system/catalog/managed hashes. Operation attempts additionally bind settings version and selected runtime/config authority. Toggling auto-start/reviewer settings should not erase verified Git history; it may invalidate an in-flight operation's authorization. Changing runtime requires fresh authorized observation before new dispatch but does not re-create existing foundation files. Changing target/root with any initialization history should follow the existing retained-target guard, not retarget old approvals.

### 5. Permissions and independent maintenance

Recommended initial authority is human workspace owner/admin, matching `Configure` and `materialTx.manager()`. The ordinary requirement reviewer list does not automatically grant write approval over AGENTS/templates/schema. A future explicit maintenance-reviewer role can be added as a separately designed capability; it is not already present in current code.

- Current human manager may configure binding, request maintenance and approve/return/withdraw the exact initialization revision.
- Agent/task credential may read only its admitted operation and request local observation; cannot configure, approve, write initialization state or post trusted receipt itself.
- Registered selected daemon provides independently gathered observation using exact task/runtime/claim/challenge binding from dependency protocol.
- Recheck current manager, originator, runtime owner/visibility and configuration before dispatch, claim, token finalization, start, preflight and receipt promotion.
- Revoke/return/withdraw or candidate HEAD/allowlist changes deny merge promotion and future stages. A remote merge performed outside approved flow remains an observed fact, not initialized=true.
- Only the exact approved maintenance paths may change. Ordinary material operation permissions remain requirements/<id>/{prd,design,plan}; do not widen `materialPath()` globally.

Known code constraint: `materialgit/provider.go:48` limits merge files to three and validates each with `materialPath`; `client.go:211` does the same for publishing and requires all files share a directory. Foundation initialization has 20 files across subdirectories. Reusing these methods unchanged is impossible. The selected runtime should execute the init stage; its independent observer needs a separate initialization expectation/checker, not `SkipValidation` or an unrestricted path list supplied by the model.

### 6. Backend/API suggestions and the readiness gate

Suggested project-level surface under `/api/projects/{id}/materials/initialization`:

- GET: current state, current candidate, observed/preserved/missing/conflicting file list, full bounded before/after content or authorized detail links, actual review URL/HEAD, decisions and capabilities.
- POST `/check`: explicit recheck/recovery with request_id + expected_version; initial inspection is automatically requested by settings save.
- POST `/revisions`: manager-maintenance candidate request against exact prior manifest/catalog, only when deliberate content/version changes are needed.
- POST `/decisions`: manager human decision with request_id, expected_version, initialization_revision_id, head_sha, plan_digest, decision and reason.
- POST `/retry`: resume same durable intent after current-authority validation; no automatic new mutation on reconnect.

This is a suggested naming seam, not existing routes. Runtime observation callbacks remain the dependency's daemon-only callback, not an initialization HTTP endpoint that accepts user `verified:true`.

Enforce a shared `requireRepositoryReady` predicate at:

1. `Revise` when `ready=true`, before revision/job creation.
2. Retry/review/merge intent dispatch, including old queued revisions.
3. Plan enqueue, claim/reclaim/token finalization/start, and plan candidate publication.
4. Runtime receipt promotion and final remote merge/ready authorization, not only initial UI availability.

Allow material draft creation/save and historical read independently. A blocked publish returns a specific `material_repository_not_initialized`/`material_repository_rules_changed` code without destroying draft input. Do not silently turn a ready submission into a draft, silently postpone it forever, or automatically replay a previously refused submit after initialization later succeeds.

Rule readiness cannot rely forever on one DB flag: before new runtime mutation, the read-only observer checks current required format/managed files against the approved ready manifest. If current material.yaml/templates changed outside approved maintenance, block and require reinspection. Unrelated business changes and new requirement files should not invalidate initialization.

### 7. Empty repository bootstrap

Existing adapters require an existing base branch: GitHub `github.go:201` and GitLab `gitlab.go:205` read it before candidate creation. The reviewed PR/MR flow cannot begin with no base/default branch. A provider 404 also cannot prove emptiness, because it can mean no access or an absent branch.

Safest first-delivery behavior: detect a truly empty repository using complete authorized remote facts and return `base_branch_required` with a clear instruction to establish its default branch, preserving no-write semantics. This satisfies honest capability handling but must be documented as a deliberate scope limit, not automatic empty-repo support.

If full empty-repo support is required, treat genesis as its own manager-authorized exception, visible before execution: create a minimal initial commit/default ref once with compare-absence semantics, never pretend it was reviewed through a nonexistent PR. Then run the actual foundation changes through the normal initialization PR. This is a product/authorization decision because it writes the default branch before the ordinary reviewed flow. It is not safe to infer from “initialize missing structure” or secretly fallback to direct default-branch writes.

The user's supplied test repository already has main/README, so this exception does not block that authorized acceptance scenario. The parent should state the empty-repo boundary explicitly in final planning rather than promise unimplemented genesis support.

### 8. Legacy data and compatibility

- No auto `ready` backfill from enabled settings, existing git_material history, existing baseline, `.git` presence or matching directory names.
- Old settings without selected runtime follow the dependency's repairable configuration-required state. Historical reviews/baselines remain readable.
- Existing pending publications do not resume under a newly created initialization gate until current runtime/root proof is ready; frozen old config/approval remains historical and may need deliberate reissue.
- Existing merged PRD/plan content stays at its current path and SHA. Initialization does not create version associations, convert UAT to production material or create a latest-version release.
- Project/workspace retention checks must include initialization revisions/decisions/receipts even when no normal requirement exists (`project.go:920`, `workspace.go:1150`).
- Additive schema only, no foreign keys/cascades. Every index in its own single-statement CREATE [UNIQUE] INDEX CONCURRENTLY migration. Coordinate migration numbers/sqlc ownership with runtime task; do not allocate overlapping numbers while that task owns generated models.

### 9. Meaningful tests

**Independent package (safe now, no DB/runtime):**

- Catalog contains precisely the intended 20 files and release/module/issue templates; schema references resolve; no fake completed business examples emitted as initialized files.
- Stable inputs yield same plan/file/bundle hashes regardless of snapshot ordering; invalid root/system ID cannot escape root or alter filenames.
- Existing arbitrary README/AGENTS bytes and UAT tree retained; missing slots added only once; second inspection has no additions.
- Truncated tree, duplicate/conflicting paths, invalid UTF-8/NUL, case collisions, symlink/submodule/executable/file-at-ancestor all rejected. Missing bytes differ from empty bytes.
- Unknown manifest version, wrong system ID, changed directory mapping and non-catalog schema/template drift block readiness.
- Candidate extra path/deletion/rename/mode change, wrong head/target, altered approved bytes and partial observation fail; exact merged proof succeeds.
- Proof checks preserved paths, so “new files are correct but old README removed” fails.
- A read-only fixed Git tree fixture using a test-local temporary repository validates real file modes/ancestors; do not use a user's ambient repository/credentials.

**Integration after dependency is available:**

- Settings transaction enqueues exactly one initialization inspection and zero normal publication. Repeated request/body identity and changed-body conflict covered.
- Fake selected runtime carries an actual issue-less task through inspect → candidate PR fixture → manager review → merge fixture → trusted observation → ready. No direct test insertion of final ready receipt.
- Initial no-change adoption creates no PR/task duplication; current same-root normal publication then succeeds.
- Normal reviewer and machine cannot approve initialization; owner/admin demotion after approval denies dispatch/promotion.
- Runtime/daemon/claim/config/root mismatch and forged model receipt rejected; ordinary tasks continue when init task blocked.
- Lost branch/PR/merge/receipt responses and worker restarts recover one logical operation; scheduler lease expiry cannot duplicate a live run.
- Readiness before/after mutation, unknown gates, externally changed rule files, extra init paths and preserved-content corruption tested.
- Legacy ready history never fabricated; initialization-only history prevents destructive parent deletion; runtime teardown retains records.
- Same repository root claimed by concurrent project bindings: prevent silent different system IDs, and serialize/verify remote creation through shared immutable identity. Cross-workspace reuse is an access decision, not granted by URL equality.

Use fresh task-owned DBs per DB-backed package and serialize package-global fixture suites per `.trellis/spec/server/database-test-isolation.md`. Default tests use fake runtime executables only. Real GitHub/GitLab runs remain separately authorized and retain evidence.

## Related specs and files found

- `.trellis/spec/server/git-materials.md`: immutable revisions, explicit recovery, exact bytes, human decisions and manual-plan hold.
- `.trellis/spec/server/security-boundaries.md`: human versus machine authority and originator propagation.
- `.trellis/spec/server/task-api-capabilities.md`: task token ceiling is not ambient shell credential isolation.
- `.trellis/spec/server/database-test-isolation.md`: no incidental DB fixture execution; package-owned DB identity.
- `.trellis/tasks/11-27-material-repository-initialization/research/dependency-runtime-dispatch-design.md`: selected-runtime queue, descriptor, daemon observer and capability plan, currently a proposal.
- `.trellis/tasks/11-27-material-repository-initialization/research/dependency-material-runtime-state-design.md`: current-authority receipt/state/lifecycle plan, currently a proposal.
- `.trellis/tasks/11-27-material-repository-initialization/research/prior-product-release-design.md`: releases/reference semantics and independent initialization maintenance intent.

## External references

No new external API or framework assumption is needed for the independent package. Source evidence above establishes current implementation limits. Before implementing automatic empty-repo genesis or new provider read shapes, validate the exact target GitHub/GitLab APIs using official provider documentation; this research did not contact external services or certify provider behavior.

## Caveats / Not Found

- No repository initialization aggregate, foundation catalog, ready gate, project-root operation target or trusted initialization receipt exists in this checkout.
- Runtime dispatch dependency is still design-only. Independent catalog/check/proof implementation is useful now, but end-to-end initialization cannot honestly be marked implemented until that dependency exposes and verifies the target/receipt seam.
- Initialization reviewer authority is not yet a separate product capability. Human owner/admin is the smallest existing conservative role; treating every ordinary requirement reviewer as a rule maintainer would expand permissions.
- Empty-repository genesis cannot fit the promised PR-before-merge path without a separate explicit exception. Default to a visible base-branch prerequisite unless parent resolves that product boundary.
- Arbitrary existing policy/template files cannot be proven semantically compatible by filename or JSON parse. Preservation, supported manifest recognition and explicit maintenance review must remain separate concepts.
- The selected runtime may possess ambient Git credentials. Server/task permission checks cannot guarantee it never performs an unauthorized external write; they can refuse dispatch, refuse trusted state promotion and preserve observed facts. Do not describe prompts as an OS/credential sandbox.
- No tests, database operations, remote writes or product changes were performed by this research task.
