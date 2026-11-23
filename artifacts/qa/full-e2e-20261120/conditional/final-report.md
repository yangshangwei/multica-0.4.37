# Conditional browser verification — 2026-11-20

## Result

**92 canonical browser cases passed in the latest current-run evidence, 0 failed, 0 skipped, 0 automatic retries/flaky results, across 51 spec files.** All were executed against fresh isolated password/admin services and the current production Web/Desktop builds. Earlier failed executions are retained and are not rewritten as original passes. Two diagnostic copies also passed but are excluded from canonical counts.

Coverage includes password registration/error recovery/migration/session/server switch; administrator initialization, account access and controls, installations, observability, execution, collaboration administration and resource publishing; MCP native and live deployment catalog; collaboration controls, grant withdrawal, source/CLI surfaces, handoffs, offline delivery, retained attachments, native pointer behavior, text-selection boundaries, creation and version history; Web/Desktop contention, CAS, 55-comment recovery, subscriptions, information repair and protected revocation; real compiled host daemon/CLI execution, squads, switching, shared context, multi-owner closure, timeout/recovery, order export, stage approvals, runtime withdrawal; support/governance/outsourcing business scenarios and approved knowledge input.

## Findings

No additional reproducible product defect remained in this lane. Root's status-options fix was served by the rebuilt clients for host/business and failure replays. Initial failures were retained with diagnosis and unchanged replays:

- Shared x86 PostgreSQL terminated with Rosetta exit133; the lane moved to a fresh independent arm64 PostgreSQL and reran administration, including initialization.
- Suite prerequisites differed: control journeys require execution open; recovery Extras require execution closed. The lane recorded exact task gate transitions and reran original assertions without weakening them.
- Source-surface required an explicit fresh CLI path; binding the built fixture resolved it.
- Native startup, selection, post-completion form, and provider startup deadlines failed under concurrent machine load. Diagnostic copies retained original assertions and recorded bounds, actual hit elements, focus, actor reads and request metadata. Exact canonical replays passed with unchanged budgets. Stage preparation timing changed from incomplete at30s to actual Git preparation at7s and test report at11s; deployed knowledge replay completed in6.4s.

## Evidence and scope

- `current-inventory.json`: per-case original/replay reports and latest results.
- `failure-disposition.json`: original failures, classifications and replay identities.
- `coverage-matrix.md`, `gates.json`: current-source prerequisite inventory.
- `web.running.json`, `web-fixed.running.json`, renderer and API provenance, source/binary hashes: environment identity.
- `stage-timing-comparison.json`: preserved preparation timing.
- Production Web switched from `keuaOQwPKy5xUzGKBrZiD` to fixed `ppJO9I3pKqFza7qn0WOdt` only after root rebuilt both clients and the conditional lane stopped between specs.

The host provider is a deterministic test executable. These tests exercise real daemon/CLI/API/DB processes without commercial provider accounts; they do not prove model reasoning quality. Native coverage is real Electron with built renderer/preload and isolated harness services, not packaged Main/installer/updater acceptance. Root owns additional packaged/cross-client/stage execution/performance coverage separately. Runtime-output remains tied to an older task's hard-coded database/storage prefix and was not repointed here.

## Cleanup

Owned API18575, Web13495, renderer5669 and arm64 PostgreSQL25432 are stopped. The task DB container, logs and built binaries remain for forensic inspection/read-only reuse. Gate state restored to execution=false, host=false, creation=true with monotonic epoch. No repository product code was edited by this lane.

## Independent review of root's changes

Reviewed Core status-options fix and5 new cases, iteration-history browser synchronization, installed-host stopped outcome expectations, and Go fixture clock adjustment plus2 regression tests. No blocking findings. The Core fix preserves authoritative access denial and epoch/auth fences while preventing a refused action from erasing independently readable detail. History testing now observes the actual address-only history commit. Installed-host assertions bind the expected403 to the stopped run and preserve successful termination/proof evidence. Go fixture observes the database clock without relaxing production future-proof rejection.

Fresh independent verification: focused Core ESLint PASS; Core typecheck PASS; Core status-options/access tests13/13 PASS; changed-file diff whitespace check PASS. Go full-suite/race verification and browser tests of root-owned changes remain evidenced by their owning lanes.
