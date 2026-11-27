# Runtime integration — 2026-11-27

Worktree: `/Users/artisan/.codex/worktrees/material-repository-initialization/multica-0.4.37`.

## Implemented

- Explicit repository target/generation/revision with no dummy material row; new
  material-repository-v1 capability; optional fields preserve legacy descriptor
  hashes. Nested frozen JSON is canonicalized with integer precision before
  hashing so PostgreSQL jsonb normalization cannot invalidate current claims.
- Separate read-only full-tree observer: fixed branch commit, complete tree,
  regular modes, every preserved file SHA256, managed file UTF-8 bodies. Both
  providers refuse truncated/missing data. Limits remain 10,000 entries, 1 MiB
  per file, 32 MiB raw aggregate; observation/CLI/private replay bounded at
  200 MiB including JSON escape overhead. Init publication and merged snapshots
  are mutually exclusive payloads.
- GitHub recursive tree plus immutable blob reads; GitLab recursive paginated
  tree and RAW blob reads. Unmanaged binary files retain exact byte hashes.
- Init inspect runs without a model; publication/audit/merge use the normal
  selected runtime backend. Independent full readback supplies candidate-base,
  complete diff/head and actual merge tree for pure kernel validation.
- Ordinary operations verify the adopted rules before model launch and again
  during observations. Actual drift snapshot reaches the server so readiness
  can be revoked; an acknowledged negative receipt still aborts local launch.
- Pure maintenance changesets have explicit before/after identities, accepted
  known catalog lookup, independent publication/merged verification, and exact
  preserved tree validation. The production registry contains only 1.0.0;
  same-bundle maintenance is a no-op. Tests use explicit old/target catalog
  fixtures, never a fictitious production version or unapproved custom proof.

## Validation

Passed: full protocol/materialrepo/materialgit `go test -race`; focused
`TestMaterial|TestProjectMaterial` daemon/CLI `go test -race`; `go vet` on
protocol/materialrepo/materialgit/daemon/CLI and `git diff --check`.

Normal composite passed both providers on fresh owned
`multica_material_runtime_init_cb8937cb`, with allocation and raw events under
`/private/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-material-init-runtime-6sncut4f/`.
It consumed actual enqueue/claim/native executable/host readback/service receipt
for initialization approval/merge and subsequent ordinary material/manual retry.
No real models, accounts or remote Git writes occurred.

Final expanded race result is appended after execution. Prior failure records
are retained: first cross-layer runs exposed jsonb digest normalization and an
absent optional-payload EOF in backend consumption; both were fixed. A later
20-second fixture native timeout was too small for 20 files and hundreds of
race-instrumented fake CLI subprocess reads (the prior fixture had one file).
The composite-only native budget is now bounded at two minutes; product defaults
are unchanged. An older repeat was stopped as owned stale work, not counted.

## API references and limits

[GitLab repositories API](https://docs.gitlab.com/api/repositories/#retrieve-a-blob-from-a-repository)
explicitly notes that JSON blob contents can be transcoded for non-UTF8 text.
The [raw blob endpoint](https://docs.gitlab.com/api/repositories/#retrieve-raw-blob-content)
therefore supplies exact preserved bytes. The
[GitHub tree API](https://docs.github.com/en/rest/git/trees#get-a-tree) documents
recursive full paths and truncation; truncated replies are refused.

Ordinary rule reads currently conservatively scan/hash the complete bounded
repository each stage, including unmanaged files. This can be expensive for
large repositories and should later be narrowed through a distinct managed-rule
proof surface, without labeling a partial initialization tree complete.
GitHub Enterprise and GitLab URL-prefix hosting remain unsupported as in the
predecessor. Browser/Electron evidence is owned by the frontend lane. Full final
independent review remains the coordinator's gate.

## Final expanded race pass

`go -C server test -tags=materialintegration ./internal/daemon -run '^TestMaterialCompositeQueueRuntimeObservation$' -count=1 -race -json` passed both GitHub and GitLab on fresh owned `multica_material_runtime_init_b1eb7c6c`. Private allocation, raw JSONL and current runtime source hashes: `/private/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-material-init-runtime-i3_pitt9/`. No active writer remains.

The final matrix includes initialization inspect/publish/human decision/review sync/merge/readiness, preserved README and binary UAT contents, compatible recheck with zero native calls, same-bundle maintenance with zero new tasks, ordinary PRD publish/approve/merge, default-off manual plan start and terminal same-issue retry, then actual external rules drift. Drift invalidates readiness before a native launch. Its server-accepted response is intentionally lost once; exact private receipt replay is acknowledged and removed with no new model call. The native counter is exactly eight executions per provider (six authorized Git write stages and two manual plan runs); read-only checks and drift trigger none.

Named results:
```json
[
  {
    "Test": "TestMaterialCompositeQueueRuntimeObservation/github",
    "Action": "pass",
    "Elapsed": 103.82
  },
  {
    "Test": "TestMaterialCompositeQueueRuntimeObservation/gitlab",
    "Action": "pass",
    "Elapsed": 40.51
  },
  {
    "Test": "TestMaterialCompositeQueueRuntimeObservation",
    "Action": "pass",
    "Elapsed": 144.33
  }
]
```

## Supported maintenance bundle 1.0.1

The coordinator approved a real rule-content upgrade after independent review
found the maintenance branch unreachable with a single supported bundle.
Current default is now1.0.1. Its AGENTS guidance explicitly separates
repository_initialization from material_revision authority, requires independent
human maintenance approval, retains protected rules until verified merge, and
forbids self-approval/stage escalation. material.yaml records1.0.1.

The20 original1.0.0 assets were compared byte-for-byte to73e187139 and remain
untouched. A pinned SHA256 inventory prevents accidental edits. The1.0.1
overlay stores only AGENTS.md and material.yaml; the other18 files reuse the
immutable baseline. Version lookup accepts only known registered bundles; no
semver guess or custom rule adoption is introduced. Existing1.0.0 catalogs
remain compatible/adoptable, and already-frozen1.0.0 candidates retain their
original catalog identity. Current and historical proofs resolve the actual
accepted catalog digest. Same-version maintenance remains no-op.

Full materialrepo race tests and vet pass, including actual1.0.0→1.0.1
publication/merge validation with exact before/after bytes, two changed paths,
preserved user README/unrelated content and rejection of extra modified paths.
The new TestMaterialRepositoryMaintenanceUpgrade runs real queue claims, native
fake runtime, independent provider reads and server receipts for both providers.
Its first race pass used private multica_material_runtime_init_9811c66d at
`/private/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-material-init-runtime-14ldcwr1/`.
The final combined run additionally asserts retained old-proof history; result
is appended below. No real model/account/repository operation occurred.

Final combined race command passed:
`go -C server test -tags=materialintegration ./internal/daemon -run '^TestMaterial(CompositeQueueRuntimeObservation|RepositoryMaintenanceUpgrade)$' -count=1 -race -json`.
Fresh owned database: multica_material_runtime_init_a72deb64. Private allocation,
raw events and current kernel/composite source hashes are retained at
`/private/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-material-init-runtime-s81v80_a/`.
No active database writer remains. The main journey tests latest1.0.1 initialization
and protected ordinary stages. The upgrade journey adopts1.0.0 with zero model
calls, proposes exactly two modifications while keeping the supported old proof
usable, executes three native write stages after separate human approval, then
checks1.0.1 readiness, retained1.0.0 proof history and same-version no-op.

```json
[
  {
    "Test": "TestMaterialCompositeQueueRuntimeObservation/github",
    "Action": "pass",
    "Elapsed": 74.19
  },
  {
    "Test": "TestMaterialCompositeQueueRuntimeObservation/gitlab",
    "Action": "pass",
    "Elapsed": 38.08
  },
  {
    "Test": "TestMaterialCompositeQueueRuntimeObservation",
    "Action": "pass",
    "Elapsed": 112.28
  },
  {
    "Test": "TestMaterialRepositoryMaintenanceUpgrade/github",
    "Action": "pass",
    "Elapsed": 17.75
  },
  {
    "Test": "TestMaterialRepositoryMaintenanceUpgrade/gitlab",
    "Action": "pass",
    "Elapsed": 15.09
  },
  {
    "Test": "TestMaterialRepositoryMaintenanceUpgrade",
    "Action": "pass",
    "Elapsed": 32.84
  }
]
```
