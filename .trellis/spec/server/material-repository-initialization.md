# Material repository initialization kernel

## Scope

`internal/materialrepo` is the deterministic, transport-free part of repository
initialization. The runtime dispatch, durable approval, service ready gate and
shared UI remain future integration work. Never report these pure tests as a
complete initialized repository workflow.

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
formatting but must match schema_version1, bundle_version1.0.0, stable system ID
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
