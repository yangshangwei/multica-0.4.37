# Final scoped verification — 2026-11-27

The approved requirement-to-plan-review delivery is implemented and locally verified. This record supersedes the earlier partial resume notes; original failures remain available in the private evidence directory.

## Delivered behavior

Project-bound material settings, complete immutable requirement/design/task-plan revisions, GitHub/GitLab controlled publication and merge, human version-bound review, verified Git baselines, durable plan handoff and explicit recovery are wired through shared Web/Desktop views. CLI candidate authoring and the narrowly scoped plugin baseline reader are included. No automatic coding or production release is part of this delivery.

Review fixes cover target/file-set merge drift, ruleset-only GitHub protection, revoked claim/reclaim/token authority, isolated denial of queued plans, transient lock errors, new-baseline issue creation, terminal planner retries, retained editor input on read failure and explicit retry after offline writes. Existing helpers and shared components were reused; no dependency was added.

## Executed verification

| Check | Result |
| --- | --- |
| Full frontend suites, Turbo concurrency 1 / Vitest maxWorkers 2 | Core 383 files / 5,523 tests; docs 10 / 64; views 633 / 7,895; desktop 104 / 1,315; web 36 / 324 passed. |
| Final focused material checks after review edits | Core 11 tests; views 18 tests passed, including terminal human retry. The added final view test follows the full 7,895-test run. |
| Forced lint + typecheck across workspace | 17 tasks successful, no errors; existing warnings retained. |
| Provider HTTP contracts | Full materialgit suite and race suite passed; exact merge drift and GitHub ruleset proof regressions included. |
| Service/handler/router race regressions | 34 / 27 / 7 top-level tests passed. Two initially skipped Redis tests subsequently passed on dedicated Redis, zero remaining skips in selected set. See backend-review.md for commands/counts. |
| Material domain, CLI, plugin contracts | Full packages passed; go build ./... and affected-package go vet passed. |
| Database and generation | Fresh task databases migrated through 908; pinned make sqlc passed. 19 migration files audited, 16 independent concurrent index files, no foreign keys/cascades. |
| Local GitLab fixture | node --test e2e/fixtures/material-gitlab.node-test.mjs passed. |
| Production builds | Fresh isolated Next production build and Electron renderer/preload build passed. |
| Web + Electron journey | 2 passed in 1.1m: draft → review → exact-byte merge → persisted manual hold → one plan dispatch → design/task-plan review → approved with no coding dispatch. Both also retain the same focused review input across an injected 503 and explicit recovery. |
| Visual inspection | Compared the material view with prior screenshots; score 94, pass. See visual-verdict.json. |
| Final whitespace | git diff --check passed. |

Frontend raw logs are under `/private/tmp/multica-material-20261126/resume-*`. The initial full test invocation used an unsupported Vitest option and ran no tests; its corrected run caught the new regression tests before fixes. Corrected complete package runs and focused final passes are recorded separately.

The first two browser runs exposed test-observation defects (ambiguous alert selection, Web header ARIA semantics and Electron route observation), not a waived product failure. The final test asserts intercepted GET/fulfilled 503, retained DOM node/text/focus, explicit recovery, merge bytes and dispatch counts on both clients.

Final browser evidence: `/private/tmp/multica-material-20261126/resume-ui-2wnlLH/replay-SgXuQm/material-run-6PoPke`. Its database is `multica_gitmaterial_ui_20261127_f82df445`; API and proxy identities were checked before fixtures. Owned API/Web/Electron processes and dedicated Redis were stopped after verification. Test databases and private credentials remain in the private allocation directory for diagnosis, outside Git. `verified-source.json` binds product source hashes to this reviewed result.

## Acceptance mapping and limits

A1–A5 are covered by domain/provider/service refusal and recovery matrices plus human review E2E; A6–A8 by exact baseline/prompt and task-dispatch tests; A9 by shared view contracts, privacy/schema tests and both client journeys; A10 by the checks above. The parent task's cross-module coding/QA/release vision remains open.

This is affected-backend verification, not a complete Go repository test run. No live GitHub/GitLab deployment, real model output, installed desktop package, GitHub Enterprise or arbitrary self-hosted GitLab version is certified. Ambiguous GitLab branch creation still requires operator resolution; unknown provider rules fail closed. Provider merge APIs do not offer atomic target-head snapshots, so final byte checks remain required. The sidebar knowledge feature visible beside the material view was not part of this scoped acceptance.
