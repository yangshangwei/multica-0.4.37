# Material repository initialization and maintenance

## Scope

`internal/materialrepo` is the deterministic, transport-free part of repository
initialization. The project/root lifecycle integrates it with the existing
material task queue, selected runtime, independent daemon observation and shared
UI. Pure kernel checks alone never establish a ready repository or human approval.

## Signatures

- `BuildCatalog(Binding) (Catalog, error)` renders the supported20-file bundle.
- `ParseManifest(string) (Manifest, error)` validates known YAML metadata.
- `Inspect(Binding, Snapshot) (Inspection, error)` preserves existing content and
  returns a frozen create-only candidate only when there are no conflicts.
- `ValidatePublication(Candidate, expectedHead, Publication) error` checks exact
  base/head, full added-file diff and resulting tree.
- `VerifyMerged(Candidate, actualMergeCommit, Snapshot) (Proof, error)` checks
  merged bytes. The caller must independently establish the actual merge fact.
- `VerifyExisting(Binding, Snapshot) (Proof, error)` handles a compatible no-op.

## Contracts

Bindings supply stable system ID and relative root. Catalogs contain README,
AGENTS, material.yaml, six schemas and eleven templates, including release scope,
version testing, acceptance and pinned delivery baselines. No business records,
empty directories, random IDs, timestamp churn or credentials are generated.

`Snapshot` is a complete tree at a full immutable Git SHA, with explicit ancestor
directories. Every regular file has SHA-256; managed files need complete UTF-8
bytes. Runtime/task/repository/approval authority belongs to the trusted caller;
a model-produced `complete:true`, candidate digest or Proof is not authority.

Existing README stays unchanged. Known material.yaml may use equivalent YAML
formatting but must match schema_version1, a supported bundle version, stable system ID
and fixed layout. Unknown AGENTS/templates/schemas conflict without writes.

Canonical candidates include the original tree, exact additions and catalog
hash. Verification recomputes those inputs and refuses modifications, omitted
files or unrelated additions. Keep ordinary PRD publication restricted to its
own1–3 files; initialization requires a separate repository/root operation.

## Validation and errors

| Condition | Result |
| --- | --- |
| Unknown format, identity, layout or YAML field | Named refusal, no candidate |
| Duplicate YAML keys, custom tags, aliases, trailing docs | invalid_manifest |
| Incomplete tree, missing body/hash or ancestor | Refuse before publishing |
| Unsafe/colliding path or unsupported entry mode | Refuse without following or rewriting |
| Different managed rule content | Conflict; independent maintenance required |
| Existing compatible bundle | No candidate; read-only proof permitted |
| Wrong source HEAD/base or extra/modified diff file | Publication refused |
| Changed/missing preserved bytes or fake merge SHA | No merged proof |

## Cases

Good: existing README and UAT files retain their bytes; nineteen missing bundle
files form one deterministic candidate. Reinspection after exact application is
a no-op.

Base: a new root at an existing Git commit yields twenty substantive files.

Bad: a caller marks initialization ready after exit0, writes AGENTS through a
PRD candidate, treats custom rule text as compatible by filename, or uses the
latest branch text instead of the inspected fixed commit.

## Tests

Owning tests cover catalog inventory/determinism, scoped roots, strict YAML,
custom-content refusal, original input mutation, complete snapshot/preservation,
publication diff and merged exact bytes. Limits must be checked on both input
and projected candidate state. Schema patterns need real JavaScript regex
probes in addition to JSON parsing and local reference checks.

Pure checks: `go test ./internal/materialrepo -count=1`, race, and `go vet`.
No DB, user Git credentials or model subprocess is part of those tests.

## Wrong versus correct

Wrong: widen the ordinary material allowlist to permit initialization files.
Correct: integrate this package through a distinct repository-initialization
operation and independent maintainer review after the runtime protocol exists.

The initial implementation conservatively rejects nonregular entries anywhere
in the complete observed tree and requires the exact prior tree plus additions
at merge. Mixed-code repositories or unrelated concurrent target changes may
need reinspection or a future explicit opaque-entry/preservation extension;
never loosen checks silently to obtain ready.

## Integrated lifecycle signatures

- `GET /api/projects/{id}/materials/initialization` returns the versioned
  `material.Initialization` envelope. Public types live in
  `internal/material/repository.go`; historical proof metadata is informational.
- POST `check`, `retry`, `maintenance` use `request_id`, `expected_version`,
  `expected_settings_version`; maintenance selects an advertised
  `target_bundle_version`. Decisions additionally bind `candidate_id`,
  `head_sha`, `manifest_sha256`, `decision` and `reason`.
- `project_material_repository` owns current binding/generation/readiness;
  repository revision and decision rows retain the independent candidate/history.
  Existing jobs/operations use a `target_kind` and real repository IDs. No fake
  requirement row, second queue, foreign key or cascading cleanup is introduced.
- `MaterialExecution` adds optional repository target/execution fields.
  `material-repository-v1` capability is required for initialization and ordinary
  tasks that require protected-rule preparation. Legacy omitted fields retain
  their original descriptor digest and cannot silently acquire new authority.

## Integrated authority and readback contract

A first enabled configuration atomically schedules an inspection intent. Reads
and UI effects never schedule it. The registered selected daemon performs
`inspect_repository` without a model invocation. A compatible complete snapshot
can become ready without a fabricated PR/approval. Otherwise a create-only
candidate is published by the selected runtime and reviewed by a current human
workspace owner/admin. Ordinary requirement reviewers cannot approve rules.

Initialization and maintenance reuse the exact task/runtime/generation/challenge,
current configuration, native preflight and actual merge-time fences. A model's
JSON, CLI exit status or kernel Proof alone cannot promote state. Maintenance
uses explicit supported before/after catalogs and adopted hashes; the current
bundle is a no-op, unknown bundles and unadopted custom files refuse.

The exact shipped 1.0.0 assets remain immutable. The 1.0.1 overlay changes only
AGENTS.md and material.yaml to document the integrated execution and maintenance
boundaries. `AvailableBundleVersions` and `BuildCatalogVersion` select known
catalogs; `SupportedProofCatalog` resolves the actually adopted digest. Existing
1.0.0 repositories remain adoptable and usable while a deliberate 1.0.1 proposal
awaits review. Do not edit a shipped baseline or earlier overlay in place, or
require a proof to match only the newest catalog before allowing its upgrade.

The daemon enumerates a complete fixed-commit tree, rejects truncation and unsafe
entries, hashes unrelated regular-file bytes, and retains full text for managed
files. Kernel limits are 10,000 entries, 1 MiB per blob and 32 MiB total source
data. The authenticated observation transport has a separate bounded JSON limit
to accommodate escaping, never a global enlargement of ordinary API bodies.
Initialization publication and merge verify the exact preserved tree plus the
approved additions/modifications. Ordinary material preparation checks the
protected catalog at the current target commit while allowing business files
to evolve.

New raw repository JSON in descriptors must be canonicalized before hashing:
PostgreSQL jsonb can reorder keys and whitespace. Preserve exact numbers and
old typed v1 digest order. Readback records remain exact-id replayable, including
accepted negative drift facts; drift invalidates publication eligibility and
cannot launch the model after its receipt is acknowledged.

`ready=false` drafts remain writable before initialization. Ready submission,
queued execution, permits, final observations and new planning require current
proof/authority. Old histories remain readable and are never automatically
marked initialized. Reconfiguration invalidates old attempts; runtime deletion
retains audit history. These are platform authorization boundaries, not a host
shell sandbox.

Repository decisions freeze `settings_version` and `binding_generation`.
Changing policy on the same repository binding invalidates old permits and
approvals. A fresh explicit human decision may authorize a new attempt on the
same candidate, review and branch once the earlier task is terminal; it must
not rewrite an earlier descriptor or decision. Disabling then re-enabling the
flow follows the same rule. Read-only recovery can inspect the original intent
under current authority but cannot transfer an old approval. Rechecking an
in-review candidate retains it instead of silently creating another PR.

Idempotency starts before task dispatch: an existing pending inspection or
maintenance revision is already a logical intent even if no operation/task row
exists yet. Repeated check/maintenance must retain it. A failed pre-dispatch
inspection retries the same revision; an uncertain remote mutation requires
read-only reconciliation. Recovery of `inspect_repository` remains a read-only
base-tree observation. Malformed/incomplete snapshots return unverified errors;
only complete conflicting facts may persist a rule-drift blocker.

## Integrated validation matrix

| Condition | Required result |
| --- | --- |
| Repeated initial save or exact request replay | One logical inspection, original result |
| Same request ID with changed body/version | Conflict, no new task |
| Missing default branch or incomplete tree | Explicit blocker; no hidden first commit |
| Existing README/UAT or binary regular file | Preserve bytes/digest; never silently rewrite |
| Custom rules or conflicting catalog content | Visible path/reason, no executable candidate |
| Machine/non-maintainer, stale HEAD/config/generation | No decision/permit/ready transition |
| Current protected rules changed | Persist blocked proof eligibility; zero model launch |
| Lost publication/note/merge acknowledgment | Read original effects first; preserve original identity |
| Rules already at requested supported bundle | No redundant PR or rule mutation |
| Proposal to update compatible rules | Keep accepted supported proof until verified replacement; never adopt proposal early |

## Integrated regression owners

`material_repository_test.go` exercises configuration triggers, complete trusted
inspection, independent human decisions, draft/publish gates, recovery and
retention. `material_repository.go` in daemon and materialgit repository tests
exercise actual read-only tree/blob behavior, completeness and binary preservation.
The native composite test must execute a test-built runtime through real
claim/start/observation and then continue the ordinary material/manual-plan loop.
Shared schema/component tests cover supported states, pinned review, complete
inert file diffs, denied access and original request recovery. Production
Web/Desktop journeys cover both providers on fresh isolated databases. Live
accounts/models remain a separately stated evidence level.
