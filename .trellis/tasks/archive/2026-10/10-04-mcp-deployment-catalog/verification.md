# MCP deployment catalog — final verification

## Result

Implemented and integrated into the original checkout on 2026-10-04. The isolated implementation used `codex/mcp-deployment-catalog`; only the task delta was copied back, with baseline byte comparisons and per-file concurrent-change checks. Other sessions' admin/onboarding and existing parameterized MCP work were preserved. On 2026-10-05 the user authorized commit. The standalone verified MCP snapshot and its required parameterized-recipe prerequisites were committed to main as `e7016ad4887583fd607009cef38b25ef47d78292`. Later intranet expansion, administration publishing and other WIP remained outside the commit. No remote push was performed.

## Delivered behavior

- `MULTICA_MCP_TEMPLATE_DIR/<key>/mcp.json`, bounded strict parsing, live scans, stable content versions and independently identified builtin/deployment sources.
- Explicit source persistence and old-client projection through `mcp_source_version=1`; rename preserves and custom replacement clears provenance.
- Private deployment configuration and typed env/arg/header inputs; directory/template errors have stable codes.
- Shared Web/Desktop source filters, visible 30s refresh, stale-form recovery, preserved saved instances/assignments and explicit assignment.
- Read-only compose mount, empty offline-bundle directory, two example manifests, bilingual instructions and updated built-in agent documentation/specs.

## Verification

| Check | Evidence / result |
| --- | --- |
| Merged core regressions | 9 files, 321 tests passed; `merged-core-tests.log` |
| Merged views regressions | 5 files, 87 tests passed; `merged-views-tests.log` |
| MCP service tests | Passed, including strict input keys, deep-copy isolation, symlinks and deterministic directory-to-FIFO regressions; `merged-service-tests.log` |
| Real PostgreSQL MCP handlers | 64 top-level passes, 1 existing builtin-HTTP skip; deployment HTTP lifecycle ran; `merged-handler-tests.log` |
| Repository typecheck | All 9 tasks passed; `merged-typecheck.log` |
| Repository lint | All 6 tasks passed, existing warnings retained; `merged-lint.log` |
| Go vet | service, handler and generated DB packages passed; `merged-go-vet.log` |
| Built-in publishing gate | `make check-mcp-catalog` passed; `merged-publishing-check.log` |
| Web + real Electron | All 6 tests passed in 1.9 minutes; `evidence/e2e-final.log` |
| Platform compilation | Service test binaries cross-compiled for Windows amd64 and Linux amd64 |
| Offline packaging | Existing script suite 13/13, bash -n, actual dry-run without image operations, Compose default/custom interpolation |
| Documentation | Example JSON parsed; both quickstart MDX files compiled |
| Visual review | 94/100 PASS against existing skill-catalog hierarchy; `evidence/visual-verdict.json` |
| Independent reviews | Spec and quality/security PASS after the fixes below |
| Static inventory | Knip findings exactly match pre-task original checkout; no additional finding |
| Whitespace | Task diff checks passed |

The real Web/Electron flow creates an HTTPS template, discovers it without API restart, saves it with a masked input, explicitly assigns it, detects a changed file through automatic polling, clears the stale input on reload, and removes the file while proving saved configuration/assignment retention. It then discovers/saves/assigns a deployment stdio template with no inputs. Existing builtin setup, agent reuse, partial assignment retry, member permissions, localization and responsive layouts also pass.

## Review regressions closed

1. Server input keys could pass file validation but fail the public client schema, poisoning a whole catalog. The parser now uses the exact public input-key grammar and isolates bad entries.
2. A valid `constructor` input key could read an inherited Object.prototype value. Rendering and required validation now use an own-property string accessor, with a red-green UI regression.
3. A directory replaced by FIFO between stat/open could block indefinitely. Both directory opens now preserve a literal `/.` under Go Root confinement; bounded child-process regressions demonstrated the old hang and passed after the change. No blocking test subprocess was left alive.

## Baseline findings and limits

- Knip still reports its pre-existing 16 unused paths, one dependency, one devDependency and configuration hint; the normalized report is identical to `knip-original.log`. No unrelated cleanup was performed.
- Three legacy claim fixtures fail in password mode because they omit PasswordSession. This was reproduced in the unmodified original checkout using the same isolated DB, then the full relevant handler group passed with explicit legacy auth. Production authorization was not loosened.
- The isolated baseline initially had five in-flight admin TypeScript errors; other work resolved them in the original checkout. The final integrated repository typecheck passes.
- Windows/Linux received compile validation, not execution on those systems. No third-party MCP programs or tools were executed. Docker images were not built/launched; ShellCheck was unavailable.
- Deployment users need migration 512 and the initial new backend/client release. Subsequent file changes require no release. Do not roll the backend back to a version lacking source projection while deployment instances exist; retaining the database column alone is insufficient.

## Resources and artifacts

Logs, screenshots, baseline backup, integration manifest and task patch are under `.omx/artifacts/mcp-deployment-catalog/` in the original checkout. Current task code is in the original checkout; the sibling worktree is retained as an implementation/evidence copy.

Task-owned API and Next processes were stopped. The isolated `multica_mcp_deployment_20261004` database was dropped after verification. Original databases and application processes were not changed. The stopped worktree environment file refers to the removed test DB and must be reprovisioned before rerunning runtime checks.

## Commit verification — 2026-10-05

The exact commit candidate was reconstructed from main plus the verified 64-file snapshot in a clean detached worktree. Independent scope review confirmed the three prerequisite test files and excluded later ResourcePublisher/admin-resource and intranet expansion code. After committing, every committed source blob matched the tested candidate SHA-256; all working-tree file bytes remained unchanged by the index-only staging operation.

Fresh checks: 321 core tests, 87 views tests, all 9 repository typecheck tasks, all 6 lint tasks, guarded MCP service tests, server build, Go vet and staged whitespace validation passed. Earlier Web/Electron and real-PostgreSQL acceptance evidence remains documented above; those runtime suites were not repeated for commit preparation.
