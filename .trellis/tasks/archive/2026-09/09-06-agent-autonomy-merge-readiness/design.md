# Design decisions

1. Reuse the staffing transaction for authorization reads. Prefer existing query-parameter or transaction-bound helper patterns; do not add timeouts or larger pools as a workaround.
2. Keep the unmerged-index check before automatic staging. Validate the final staged snapshot before commit, and identify actual conflict groups instead of matching free-form Git error text. Inspection errors must not silently allow delivery.
3. Decode batch field presence consistently with Go's accepted case-insensitive object merging. Preserve existing accepted requests and null semantics; avoid random first-match map iteration.
4. Use existing workspace/automation cleanup API paths where they cover the dependency graph. Keep cleanup scoped to unique test-owned resources and make assertions expose leaks.
5. Keep the ordinary API test suite bounded: add a Coordinator actor for ceiling coverage and strengthen only the review-identified assertions.
6. Reproduce on isolated local PostgreSQL databases and a backend built from this worktree. Existing development services are not the verification target.

## Ownership

- execenv lane: server/internal/daemon/execenv/local_worktree.go and its focused tests.
- handler lane: server/internal/handler/squad_template.go, transaction-aware access helpers if needed, issue.go, and focused handler tests.
- E2E lane: e2e/agent-autonomy-gates.spec.ts and narrowly required TestApiClient cleanup support.
- leader: task artifacts, integration environment, cross-lane review, documented autonomy claims, final validation and commits.

## Constraints

Tests precede behavior edits. Use dbfx and testutil.Call for new DB-backed handler cases. Default tests must not execute user-installed agent CLIs. No new dependencies or broad refactors. Existing ungraded-agent compatibility is required. Keep every lane inside its assigned write scope.
