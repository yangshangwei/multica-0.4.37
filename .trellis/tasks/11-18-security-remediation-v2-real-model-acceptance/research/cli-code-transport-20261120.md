# Direct official CLI code-role transport candidate

New `codex_exec_code_container.py` is independent from the P1/O1 scratch wrapper. It uses explicit transport_mode=official_cli_exec_code and validates the actual role, full clone root, current baseline commit and every pre-existing test hash before launch. It never makes an SDK/app-server call. A direct CLI successful terminal remains mechanical evidence, not a semantic acceptance verdict.

Filesystem modes:

- implementer: current source writable; complete tests subtree readonly; AGENTS/injected context/skills/config/requirements readonly; own exact .git writable for real commits.
- verifier: source root readonly; tests subtree writable for new reproduction files; each pre-existing test additionally mounted readonly; own .git writable for committing only tests.
- verifier_recheck, qa, reviewer: source, tests and Git readonly; only separate `.multica/comments` and per-run transport scratch writable.
- leader_scratch: actual is_leader_task, explicit leader_ignores_project_resources and prepared_repo_count=0; the production local_directoryAssignmentForTask deliberately returns nil for coordinators. Actual claim project resource count remains in metadata; validation of the separately prepared scratch set does not rewrite the claim. No project code is mounted.

All modes retain immutable managed network=false/approvalNever, internal supplier-control network, nonroot/cap-drop containers and original task-token CLI spool. Reserved instructions and all existing .multica material other than comments are protected. Exact source paths, manifest and immutable hashes enter final admission.

## Git trust boundary

`contained_git_role_handoff.py` is a new host launcher. ALL Git inspection and the complete existing export/import verifier execute inside a no-network container mounting only the owned full role clone and explicit bundle input/output. The existing verifier's staging clone is inside that container too. Host code never evaluates model-owned .git/config, includes, filters, fsmonitor or hooks. Export source is readonly; import can write only its receiver clone. Fresh output parent and leaf files must be regular/nonaliased before promotion.

The code wrapper's preflight Git reads also use that contained launcher, because a verifier recheck may see previously model-owned metadata. Post-run HEAD reads are manual nofollow/nonblocking regular-file reads with a size bound, not host Git; FIFO and symlink rejection prevent a model blocking evidence retention or reaching host paths. Protected tests and AGENTS are compared after the model exits.

## Verification

Final candidate r3 runs the actual official Codex exec wrapper against an in-container synthetic Responses peer, never a supplier. All five code modes passed their write matrix, Git commit/no-commit expectation, original task CLI read, live control-peer then denied shell socket, and rejected require_escalated. Implementer committed code only; verifier committed a new test only. Recheck/QA/reviewer could not commit. Existing tests and AGENTS remained unchanged. Separate actual leader_scratch r1 passed comment-file writing and immutable input denial while metadata retained claim_project_resource_count=1 and prepared_repo_count=0.

Five code-policy tests passed, including actual-role/head/protected binding, shared Git refusal, preserved claim counts, and nonblocking FIFO/symlink HEAD refusal. Four contained-Git tests passed: exact commit/tree/author-preserving bundle transfer, model fsmonitor could not touch a host sentinel, absolute host config include was invisible, and write-command/shared-clone inspections refused. Earlier /var tmp fixture mount failure was retained and corrected by using a canonical Docker-visible task fixture path; no assertion or security boundary was relaxed.

Evidence is outside the repository at `code-exec-r3/` and `code-leader-r1/` beneath `/Volumes/artisan/code/2026/secrem-v2-cli-evidence-20261120/`; source and result hashes are in cli-code-transport-r3-20261120.json. This does not admit full C1/D1: actual platform resource switching, role handoff, human gates and complete scenario oracles still require harness validation and independent admission.


## Bounded post-run maintenance after the first C1 attempt

The consumed code wrapper incorrectly tried to compare project test hashes in the leader's scratch directory even though production intentionally did not mount those tests. Its native CLI turn had ended normally, but that host evidence check failed. Preserve the original attempt and wrapper af830819 hash; the successor does not relabel it as success. Leader filesystem evidence now records `protected_tests_status=not_mounted` and `protected_tests_unchanged=null`, while retaining the project protected hash map as provenance and checking the actual AGENTS file.

Leader scratch also requires canonical `ignored_project_paths` from the actual claim and rejects same-directory, descendant or ancestor overlap with the prepared cwd. The actual claim resource count remains unchanged. Eight maintenance controls pass, including missing unmounted tests, non-overlap and invalid leader metadata.

A separate read-only `analysis` mode now binds only an actual product-analyst role. It adds no API credential authority, no writable code/tests/Git and no change to autonomy or template rules; only the existing comment scratch is writable. This permits a legitimate clarification role to be isolated if a future authorized harness uses it.

Actual official CLI with synthetic Responses proved the successor leader with nonempty project protected hash metadata but no mounted test tree completes its evidence safely (`code-leader-r2`). A separate product-analyst probe proved all code/new tests/baseline tests/AGENTS writes denied, no Git commit, original task read identity, offline shell and rejected escalation (`code-analysis-r1`). Both make zero supplier requests. This is bounded maintenance only: no real execution resumed, no blocked agent request was retried, and no continuation harness was implemented by this lane.
