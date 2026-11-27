# Research: initialization bundle, templates and layout contract

- Query: What exact base files should initialization create, which formats/dependencies can implement them, and how can existing content, custom templates, version references and repeat execution stay safe?
- Scope: internal code/document research; design recommendations, no product code or remote writes.
- Date: 2026-11-27
- Task: `11-27-material-repository-initialization`

## Findings

### 1. Confirmed requirements and source evidence

The user approved the full logical layout and requested its initialization flow. The exact mechanics below are proposed implementation choices; they must be reconciled into the root agent's design, not described as existing behavior.

The version design preserved in `research/prior-product-release-design.md:18` distinguishes product versions from Git revisions; lines 41–52 place scope, test, acceptance and actual baseline in different files and avoid the self-commit cycle. Lines 60–66 bind a PRD and release range in one review and preserve stable paths on deferral. Lines 72–86 separate issue discovery, affected, planned-fix and verified-fix versions. Lines 102–104 require independently authorized initialization, preservation of existing files, and later on-demand business files.

Current code does not implement this bundle. Ordinary document production remains `materialFiles` in `server/internal/service/material_worker.go:118`: only `prd.md`, `design.md`, `plan.md`. `server/internal/materialgit/client.go:197` allows only these basenames below `requirements/<id>`, and `validatePublish` at211 limits each ordinary candidate to 1–3 files in one material directory. **Keep that narrow ordinary allowlist. Initialization needs a separate operation kind and an exact bundle-file allowlist, not a general expansion of PRD publication authority.**

### 2. Initial bundle: exactly 20 files when the selected root is empty

Use paths relative to the configured material root. With `root_path=docs/product`, the first file is `docs/product/README.md`; do not also write a repository-root AGENTS.md. With an empty root, paths are repository-root relative.

| # | Relative file | Purpose and required content |
|---|---|---|
| 1 | `README.md` | Human navigation; logical directory lifecycle; product-version vs Git-revision distinction; rule-maintenance and runtime flow; links to templates. |
| 2 | `AGENTS.md` | Short reading order; stable IDs/paths; read only task-authorized resources; ordinary candidate paths; human review ownership; root/rule files require independent maintenance; no credentials, runtime-state or fabricated evidence. |
| 3 | `material.yaml` | Repository format major version, template bundle version, stable system ID and fixed logical layout. No secrets, runtime identity or approval/status booleans. |
| 4 | `.material/schemas/material.schema.json` | Machine-readable structural contract for material.yaml; reusable local reference definitions if desired. |
| 5 | `.material/schemas/module.schema.json` | Module ID/name, logical repository identities and dependencies; no dynamic access grants. |
| 6 | `.material/schemas/requirement.schema.json` | PRD frontmatter identity/type/title/modules and bounded fixed-version source references. |
| 7 | `.material/schemas/issue.schema.json` | Problem identity, classification, discovery/affected release references, tested code/build and linked requirements. |
| 8 | `.material/schemas/release.schema.json` | Stable release identity/display version and requirements/planned-fix scope references. |
| 9 | `.material/schemas/baseline.schema.json` | Actual released document/code/build revisions and acceptance references. |
| 10 | `.material/templates/module.yaml` | Actual module responsibilities, logical repos and dependencies to fill when registering a module. |
| 11 | `.material/templates/prd.md` | Users/problem, goals, scope/exclusions, flow, boundaries, stable AC IDs, engineering evidence, unresolved questions. |
| 12 | `.material/templates/design.md` | Exact PRD revision, boundaries, data flow, alternatives, compatibility/rollback, open review matters. |
| 13 | `.material/templates/plan.md` | Exact PRD/design input, stable task IDs, goals/dependencies/module references, AC mapping and validation; no live assignees/status. |
| 14 | `.material/templates/requirement-test-plan.md` | Single requirement AC-to-scenario mapping, explicit input/expected result, environment and evidence collection. |
| 15 | `.material/templates/issue.md` | Original symptom, prerequisites/repro, expected/actual, discovery and confirmed affected versions, code/build/environment and evidence; unknowns explicit. |
| 16 | `.material/templates/resolution.md` | Verified cause, fix commits/builds, individual target versions, actual retests and evidence; no prefilled success. |
| 17 | `.material/templates/release.yaml` | Product release identity/goal, planned requirements/fixes and immutable references; no guessed latest release. |
| 18 | `.material/templates/release-test-plan.md` | Whole release and cross-module test coverage, version combination, scenario and evidence matrix. |
| 19 | `.material/templates/acceptance.md` | Actual tested combination, date, scope, outcomes/gaps and evidence references; no fabricated pass or signer. |
| 20 | `.material/templates/baseline.yaml` | Actual delivery set of already-existing material/code commits and build identifiers, acceptance reference and differences from scope. |

No initialization writes under `modules/`, `requirements/`, `issues/`, `releases/`, `contracts/`, `specs/`, `decisions/` or `knowledge/`. README/material.yaml document those logical roots. They appear when a real module/version/document is created; no `.gitkeep`, dummy REQ/ISS, sample v1.8.0, empty acceptance or fabricated ADR. Existing `uat/` is outside the initialization diff and remains byte-identical.

A template is intentionally an authoring aid. Use obvious textual placeholders such as `<material-id>` and `<existing-commit-sha>` plus clear `TEMPLATE — replace placeholders before publication` guidance. Do not let templates masquerade as valid production records: `.material/templates/` has its own scope and may contain incomplete values; ordinary candidate readiness rejects unresolved placeholders/absent evidence under the relevant business validator. Do not add created-at/current date/random IDs to static template bytes.

### 3. Bundle ownership and versioning

Implement a pure, embedded source package, suggested `server/internal/materialbootstrap`, containing static assets and deterministic render/inspect/diff functions. This is a naming recommendation, not a requirement to introduce a broader framework. The runtime and server can import one package. Keep transport, database mutations and human permissions outside it.

Existing compile-time asset patterns are `server/internal/service/builtin_skills.go:16` and `builtin_agent_templates.go:28`. The template precedent explicitly preserves team-edited copies and shows future diffs rather than silently replacing them (`builtin_agent_templates.go:20`). Embed the bundled bytes once; avoid separately hand-maintained frontend copies.

Use two independent versions:

- `schema_version: 1`: supported repository format/interpretation major version.
- `bundle_version: "1.0.0"`: immutable packaged file/template revision. This is unrelated to a product version such as v1.8.1.

The server registry defines exact supported schema versions and bundle versions. Do not assume any same-major semver is safe; a future unknown `1.99.0` remains unsupported unless the current binary includes its compatibility rule. Compare using a fixed catalog for the first implementation; no new semver dependency is needed.

Changes to template prose or JSON schema bytes increment bundle version and require a maintenance candidate. Breaking field/layout meaning increments schema major. A service upgrade never rewrites the repo automatically or discards a previously recorded verified revision; unsupported current content blocks new publication with an explicit reason.

Build a deterministic bundle digest from sorted `(relative_path, mode, content_sha256)` entries plus format/bundle identity. Store it in the frozen init/maintenance operation and readiness receipt. Do not hash a map in noncanonical order or inject timestamps. `material.Digest`/`JSONDigest` in `server/internal/material/types.go:181` already supply SHA-256 helpers; source a sorted struct slice before hashing. Candidate marker hashing already sorts paths (`materialgit/client.go:239`).

**No self-hash:** material.yaml does not store its own file digest, this operation's future merge SHA, or the aggregate digest that includes itself. Actual file hashes and merge SHA are external receipt facts. Bundle code stores asset hashes; generated metadata states versions and identity only.

### 4. material.yaml shape

Suggested v1 shape, using the stable system ID allocated/persisted before dispatch:

```yaml
schema_version: 1
bundle_version: "1.0.0"
system_id: "<server-owned-stable-system-id>"
layout:
  modules: modules
  requirements: requirements
  issues: issues
  releases: releases
  contracts: contracts
  specs: specs
  decisions: decisions
  knowledge: knowledge
```

Paths are fixed relative children of the configured root in v1. This is not an arbitrary mapping language or a script loader. The trusted project binding owns provider/repo/root/target/runtime; material.yaml cannot redirect an operation elsewhere. Root is excluded from the file so a repository-root-relative configuration is not mistaken for another external filesystem root.

For an existing recognized manifest, preserve its stable system ID and require binding reconciliation; do not regenerate one per project or silently claim an already-bound logical system. If the system ID disagrees with the frozen operation/binding, report conflict. Final system ID allocation/cross-project root uniqueness belongs to backend design.

### 5. Existing content: preservation and compatibility matrix

“File exists” and “initialization compatible” are separate facts. No model judgment or `approved: true` text should establish compatibility.

| Observed entry at exact immutable commit | Result | Write behavior |
|---|---|---|
| Missing required path, with complete parent-tree proof | `missing` | Generate add-only action using deterministic bundle bytes. |
| Exact current known bundle bytes (dynamic manifest identity validated) | `compatible` | Preserve unchanged. |
| Recognized older supported bundle | `maintenance_available` or current compatible according to explicit catalog | Preserve until separately approved maintenance; no automatic overwrite. |
| Existing ordinary UTF-8 README.md | `preserved` | Keep original bytes. A missing default README can be added, but existing documentation is not replaced to match a template. The other generated files must carry the necessary machine/Agent conventions independently. |
| Existing AGENTS.md equal to known bundle or an earlier independently approved adopted digest | `compatible` | Preserve exact bytes. |
| Existing custom AGENTS.md without prior adoption | `content_conflict` | Preserve; show it and recommended bundle content for explicit maintenance/adoption review. Never declare semantically compatible based on a marker or LLM statement. |
| Existing custom template at a managed path without prior adoption | `content_conflict` | Preserve; no overwrite or second parallel template with a guessed name. A maintenance review can explicitly adopt a fixed digest if it satisfies template structural requirements. |
| Existing schema differing from supported packaged schema | `schema_conflict` | Preserve; block. Arbitrary custom schema cannot relax host acceptance rules. New schema support requires a supported format/bundle implementation. |
| material.yaml with supported shape, matching identity and known version | `compatible` | Preserve bytes even if formatting/key order differs; parsed semantic identity is authoritative, exact observed bytes still frozen for approval/readback. |
| Invalid YAML/JSON, unknown schema/bundle, wrong identity/layout | `unsupported_format` / `manifest_conflict` | Preserve; no destructive repair or inferred migration. |
| Symlink, submodule, executable regular file, file in place of required directory, case-collision | `unsafe_entry` | Block without following or overwriting it. |
| Extra ordinary files/dirs including uat | `unmanaged` | Leave unchanged and exclude from mutation authorization. |
| Read failure / partial tree / unknown existence | `unavailable` | Block; never interpret an HTTP404 or missing output as confirmed absence. |

The explicit adoption option above is useful for custom prose/template compatibility, but is a **product choice to freeze in main design**, not permission for an Agent to auto-approve existing AGENTS instructions. A narrower initial implementation may only accept known bundle bytes and report custom managed files as conflicts. Either path must not silently replace them. An adopted digest is a record in the independent initialization/maintenance review, never a self-reported manifest trust claim.

If all required files already match a recognized compatible setup and its binding, readiness can follow verified readback with no PR. A freshly missing manifest plus custom root README is still initializable: preserve README, add the other 19 files. Custom AGENTS/templates cannot be ignored while calling the whole bundle “verified”.

When preserving an existing README, readiness proof includes its observed digest/mode but its arbitrary prose does not alter access rules. Existing AGENTS is read as untrusted repository content during inspection; do not load it as execution instructions before approved admission. Even an approved AGENTS file does not override Multica's task capability or human approval boundaries.

### 6. Formats and dependencies: no new package needed

`server/go.mod:38` already pins `gopkg.in/yaml.v3 v3.0.1`; Go stdlib supplies `encoding/json`, `embed`, `io/fs`, SHA-256 and UTF-8 validation. Use typed manifest decoding, strict field/version checks and bounded parsing. The existing `strictResourceFrontmatter` in `server/internal/service/resource_validation.go:224` is the useful policy precedent: cap bytes, reject NUL/invalid UTF-8, require one mapping, reject aliases/anchors/depth abuse, reject duplicate/non-string keys (lines238–264). Reuse its principles without importing service-specific resource errors into a pure initializer package.

Recommend exact one-document decode (`Decode` then require EOF), node scan for duplicate keys/aliases, typed `KnownFields(true)`, explicit enum and path checks. JSON schema assets should be parsed with encoding/json; validate local `$ref` targets/anchors and disallow network-resolving refs in tests. A repository-provided schema is documentation/format input, not executable authorization.

The repo lockfile contains transitive Ajv versions but no inspected owning package declares a JSON Schema validator. **Do not import a transitive package or add an engine dependency solely to validate initialization.** Runtime readiness only needs strict material.yaml validation, supported schema assets, template structural checks, and complete byte/mode proof. Do not claim that a Go handwritten manifest validator validates every JSON Schema instance or future release workflow.

Keep JSON schema content within an explicitly declared dialect and common structural keywords; schema choice and cross-tool compatibility must be tested if an external validator is later introduced. This research did not introduce or validate a new JSON Schema engine.

Recommended field discipline for six schemas:

| Schema | Essential definitions |
|---|---|
| material | schema_version, bundle_version, stable system_id, fixed layout keys; forbid secret/runtime/approval fields. |
| module | schema_version, id, name, bounded logical repository list, dependencies; refs do not grant access. |
| requirement | schema_version, id, type=requirement, title, affected_modules, source refs; actual prose/AC completeness remains domain validation. |
| issue | schema_version, id, type=issue, category, discovery/affected version and tested build/code refs, requirement refs. Planned fixes belong to release scope, actual verified fixes to resolution. |
| release | schema_version, stable release id, display version, goal, requirements/fixes array of fixed refs; no live status/assignee. |
| baseline | schema_version, release id, exact delivered refs, repository full SHAs, build/digest refs and acceptance evidence; do not pretend plan scope equals delivery. |

Templates should not invent a feature/runtime that is not implemented. Distinguish intended structure from existing automatic version-management behavior. Require actual evidence before acceptance/resolution can say passed/fixed; template sections state what to fill, not sample success results.

### 7. Stable references and no circular commits

Use a single documented `material_ref` shape in relevant schemas/templates:

```yaml
id: "<stable-material-id>"
path: "requirements/<stable-material-id>/prd.md"
content_sha256: "<64-lowercase-hex>"
# commit is optional only for a same-change reference; an already existing
# revision or a published baseline must pin an existing full commit.
commit: "<40-or-64-lowercase-hex>"
```

For new PRD+release-scope content in the same future commit, omit commit and bind the file using ID/path/digest plus the exact accepted changeset. Record actual merge SHA outside those bytes when the PR merges. Do not permit `main`, `HEAD`, an omitted digest or “latest” to stand in for a fixed accepted revision. A later baseline.yaml references already-existing material commits, code commits and build IDs; it does not contain its own future merge SHA.

Issue relationship facts must have one owner: discovery/affected versions in issue.md; planned repair in release.yaml; verified repair in resolution/evidence. Repeating all four mutable relationships in every file would create conflicting sources. Expose consolidated version views in Multica by reading those relationships.

Initialization includes these contracts/templates but **does not implement product version creation, scope-change transactions, release freezing, issue resolution validation or their UI**. Those are separate business features. This distinction prevents a bundle test from being reported as the whole releases workflow passing.

### 8. Pure API seam and frozen candidate model

Suggested functions for the dependency-independent package:

```text
Bundle(formatVersion, bundleVersion, stableSystemID) -> []File
Inspect(configuredRoot, immutableTreeSnapshot, supportedCatalog) -> Inspection
Plan(inspection, approvedMaintenanceInput?) -> FrozenChangeSet
Verify(immutableMergedTreeSnapshot, frozenChangeSet, preservedEntries) -> Verification
```

All are pure and deterministic. `File` includes relative path, regular mode100644, complete UTF-8 content and computed digest. Inspection consumes a trusted complete tree observation from the runtime receipt layer, not a caller-provided map that could omit collisions. For zero files/no changes, no candidate is generated. Missing or uncertain observation never becomes an empty tree.

The frozen change set includes observed base commit; target/root/system binding identity; schema/bundle version; sorted exact add/modify operations; before hashes for deliberate maintenance edits; after hashes; required preserved-entry digests; and the independent review ID/version. Maintenance actions may only modify exact permitted bundle files after explicit human review. Init actions are add-only. Every source file path must be rooted and validated before `path.Join`; reusing `material.ValidPath` (`types.go:184`) is necessary but not sufficient because provider path validation rejects additional controls/reserved encoding (`materialgit/client.go:165`).

Version-aware replay uses the persisted operation identity and frozen bytes, not a newly rendered bundle after a service upgrade. An upgrade may offer a new maintenance generation only after the prior attempt is settled. Treat product version labels independently from bundle version when constructing operation keys.

### 9. High-value tests

1. Empty compatible tree produces exactly20 files with byte-identical second render; no business examples, empty directories, timestamps, generated credentials or future self-SHA.
2. Existing README + unrelated uat leaves their bytes untouched and produces only19 missing files; applying candidate then reinspection is compatible/no-op.
3. Exact supported complete bundle reinspection produces no PR; equivalent YAML formatting is valid but its actual bytes are preserved in observation.
4. Custom AGENTS/custom template/schema, unknown format/bundle, wrong system ID or altered layout are preserved and yield named conflicts; no mutation escapes the exact init allowlist.
5. Symlink/submodule/executable file, case collision, file-as-parent, traversal/control-character path and partial/missing tree observations fail before writes.
6. Strict YAML rejects duplicate fields, unknown fields, multiple documents, aliases/anchors, invalid UTF-8/NUL and oversized/deep inputs. No parser panic or semantic downgrade.
7. Candidate contains only generated missing entries; unrelated files cannot be attached. Pre-existing entry appearing between observation and publication causes conflict, never overwrite.
8. Maintenance generation explicitly binds old and new bytes; the old approved candidate cannot apply after a base/config/bundle change. Identical replay retains operation and artifact identity.
9. After actual merge, altered required file, missing preserved entry, modified mode, wrong root or wrong HEAD prevents ready. A successful command or marker alone never suffices.
10. Markdown templates include correct source/baseline placeholders and all logical roots; no filled “passed”, fabricated reviewer, fake Git SHA or auto-execute instruction.
11. Schema files parse, known refs resolve within the six-file schema bundle, and generated valid material.yaml plus explicit malformed examples match the typed validator. Do not mirror generator strings as the only test.
12. Ordinary `ValidatePublish` still rejects AGENTS.md, material.yaml, .material files and broader initialization file counts. This regression must remain outside the separate initialization path.
13. Current physical PRD UUID paths and pre-existing unversioned test data remain untouched; no migration to illustrative REQ/ISS naming and no automatic latest-release assignment.

## Files found

- `server/internal/material/types.go` — SHA helpers, path validation and existing candidate readiness.
- `server/internal/materialgit/client.go` — narrow ordinary material path/file-count validation, sorted candidate hashing, exact file/change checks.
- `server/internal/materialgit/github.go` — complete tree/regular-file and bounded UTF-8 blob readback precedent.
- `server/internal/service/material_worker.go` — ordinary PRD/design/plan generation seam; initialization must remain distinct.
- `server/internal/service/material.go:448` — configured target/root cannot change once material history exists; initialization history must be accounted for by backend policy.
- `server/internal/service/resource_validation.go:224` — strict YAML frontmatter safety precedent.
- `server/internal/service/builtin_skills.go:16` — embedded product-owned file tree precedent.
- `server/internal/service/builtin_agent_templates.go:20` — preserve customized content; upgrades show differences rather than replacing blindly.
- `server/go.mod:38` — existing YAML dependency.
- `pnpm-lock.yaml` — transitive schema validators are present but are not an authorized new direct import surface.

## Related specs

- `.trellis/spec/server/git-materials.md`
- `.trellis/spec/server/security-boundaries.md`
- `.trellis/spec/guides/cross-layer-thinking-guide.md`
- `.trellis/spec/guides/code-reuse-thinking-guide.md`
- `apps/docs/content/docs/developers/conventions.mdx`
- This task's `research/prior-product-release-design.md`

## External references

None consulted. Findings and proposals are grounded in current local code, the user-approved layout and the preserved local version-design record. No new library/API behavior is asserted as externally validated.

## Caveats / Not Found

- There is no current repository initializer, bundle manifest, schema engine, custom-file adoption record or independent rule-maintenance workflow in inspected product code.
- Exact20 files is the empty-root base inventory; preserving existing compatible/user documentation changes the actual add-only diff count.
- Custom AGENTS/template adoption versus strict conflict is a compatibility decision to make explicit in the main design. Do not silently use LLM semantic approval as the answer.
- Empty Git repositories have no target branch to review against. Bootstrap-of-default-branch policy is separate from this file bundle; this package must not secretly create/approve a first branch.
- Runtime operation/attempt/receipt contracts are currently being designed in another task. The pure bundle package can be built independently, but dispatch/readiness claims must wait for real integration.
- No product code, specs, credentials, Git state or remote repository content were modified; only this research artifact was written.
