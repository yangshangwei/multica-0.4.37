# Integrated repository workflow contract v1

Frozen for integration 2026-11-27. Public types are `server/internal/material/repository.go`.

GET `/api/projects/{id}/materials/initialization` returns `Initialization` (protocol_version 1). Response fields match frontend research except readiness uses `schema_version`, and root adds `settings_version` and `available_bundle_versions`. Missing configuration returns version/generation 0, unconfigured, publish_allowed false. Candidate kind is initialize or maintenance. Public before/after contents are bounded and protected by project access. Repository decisions belong to owner/admin maintenance authority, never PRD reviewer_ids.

POST suffixes `/check`, `/retry`, `/maintenance` accept `InitializationCommand`: request_id, expected_version, expected_settings_version; maintenance also target_bundle_version. POST `/decisions` accepts `InitializationDecisionInput`: those identities plus candidate_id, head_sha, manifest_sha256, decision (approve/request_changes/withdraw), reason. Every success returns complete Initialization. Same request/different body conflicts. Current supported bundle maintenance is an idempotent no-op without a new PR.

Durable root identity and immutable repository revisions/decisions are separate from git_material. Existing jobs/operations gain target_kind and real repository_id/repository_revision_id; ordinary material references become nullable for repository operations, never invented placeholder material rows. One existing queue, carrier, claim/challenge, task-token ceiling and observation transport remain.

Runtime flat fields: MaterialExecution.TargetKind (material_revision or repository_initialization), RepositoryID, RepositoryRevisionID, RepositoryGeneration, Repository *MaterialRepositoryExecution. Repository execution holds SystemID/RootPath/BundleVersion/CatalogDigest/BaseCommit strings and Candidate/Maintenance/ReadyProof JSON data. Observation RepositorySnapshot/RepositoryPublication/RulesSnapshot carry strict typed materialrepo JSON. inspect_repository is read-only; other actions publish/review_sync/merge/observe retain normal selected-runtime ownership. material-repository-v1 capability required for initialization or protected-rule checks.

Ordinary operations freeze accepted ready proof and require a fresh current-base RulesSnapshot in dispatched preparation before model launch; permits and terminal receipt recheck current proof identity. Drift blocks current publication eligibility. Initial inspect can adopt a complete compatible tree directly, or produce independent immutable create-only candidate and queue publication. Maintenance inspects exact supported prior/target catalogs and freezes explicit before/after changes; no arbitrary uploaded patch API.

## Readiness and recovery clarifications

A complete negative protected-rule readback is an immutable accepted receipt: persist blocked readiness and return its transport acknowledgment, then daemon stops before model execution. Its identical replay must work after readiness was invalidated. Missing/incomplete reads are not fabricated proof of drift. Successful observe/review_sync continues the original decision's merge; missing note or not-yet-merged open review may establish no_effect only under explicit read-only recovery and requires deliberate next retry. Recheck cannot abandon an unsettled mutation to create another PR.

Inspection conflicts are visible inert source records, not executable candidates: no approved head or nonempty manifest means no approval action. Ready proof history remains informational and cannot enable publication. Reconfiguration increments current identity while old tasks and proof records retain the original identity.

Repository JSON fields in descriptor digests are canonicalized with exact numeric values before hashing to survive database jsonb serialization. Omitted extension fields retain the exact old material v1 digest.

## Supported bundle maintenance

The original kernel's exact 1.0.0 catalog remains a supported immutable prior
bundle. The current 1.0.1 bundle adds the implemented initialization, protected
rule and independent maintenance authority instructions to AGENTS.md and updates
material.yaml's bundle identity. It is a real packaged content revision, not a
synthetic product release or fabricated acceptance result.

New repositories initialize to the current bundle. A complete supported 1.0.0
repository can still be independently adopted without creating a PR; its proof
records the actual old catalog. Deliberate maintenance to 1.0.1 freezes the exact
before/after files and follows the same separate human review, native permit and
merged-byte verification. While the old rules remain unchanged and supported,
the current proof stays usable until the verified replacement is adopted.
Unknown catalogs/versions remain blocked, current-version maintenance is a no-op,
and README/UAT/business files are never rewritten by the bundle upgrade.

## Explicit approval after configuration changes

Repository decisions persist server-owned settings_version and binding_generation.
A same-binding policy change, including disable/re-enable, refuses old attempt
permits and decisions. A fresh human decision under the current settings creates
a new execution attempt while retaining the existing candidate/review/branch and
all previous decision/task history. Prior tasks must be terminal before that new
attempt. Recheck never silently replaces an already published in-review candidate.
