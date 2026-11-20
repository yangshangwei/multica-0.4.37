# Independent caller review, F3 step 7b

## Verdict and boundary

Reviewed the 43 frozen caller files against design §12.1–12.3 and §12.6–12.10. One actionable privacy defect was reproduced and fixed in the integration worktree after the coordinator widened ownership. No further actionable caller defect was found in this bounded review. This is caller acceptance, not acceptance of the complete F3 server/daemon feature.

- Frozen source: `/Volumes/artisan/code/2026/mc-f3-budget-7b-20261119` (`86fea651c` plus caller changes). All 43 SHA-256 values match `/tmp/f3-main-s7k/base-r2/manifest.txt` both before and after review. No source files were added or edited there.
- Fix destination: `/Volumes/artisan/code/2026/mc-f3-completion-20261120`.
- Only source edits by this reviewer: `packages/core/api/client.ts` and `packages/core/api/issue-budget-schemas.test.ts` in the fix destination.
- Scratch evidence and independent tests: `/tmp/f3-codex-7b-check/`.

## Findings (fixed)

**KB-01 — P2: Core logs private budget error text before schema redaction.**

- Location in the frozen source: `packages/core/api/client.ts:3270` (`getIssueBudget`), `:2655` (private path classification), `:2718` (error logging).
- The new method uses `fetch<unknown>`, which reaches `fetchRaw`. That shared path extracts `data.error` and logs it for non-2xx responses. `/api/issue-budgets/` was not classified as private, so `redact: true` in the later schema call never protects this path.
- Reproduction: a mocked HTTP 400/403/404/409/500/502 response with `{"error":"budget-secret-reason 73500000000","code":"internal_error"}` logs that exact reason and amount in `error` metadata. This requires a nonconforming response (e.g. server drift); conforming code-only error fixtures do not expose the bug. Client redaction must not depend on an error body obeying the server's contract.
- Independent test source: `core-overlay/packages/core/api/issue-budget-review.check7kb.test.ts` under scratch. Original caller source copied without modification: 6 failing privacy cases, 4 passing numeric boundary cases, all original 92 tests passing (`core-independent.log`).
- Fix: classify `/api/issue-budgets` for the shared error logger and emit `Budget request failed`. Preserve the `ApiError` status/body, authorization behavior, and the existing `SyntaxError` → `null` fallback. No dependency, schema, wire, or UI change.
- Regression in the integration tree: `issue-budget-schemas.test.ts:142` covers 400/401/403/404/409/500/502, verifies fixed log text and absence of private response content, and verifies status/body remain available to the caller. Before fix: 7 fail / 92 pass. After fix with the shared API-client suite: 221 pass / 0 fail.
- Final product anchors: `client.ts:2659` and `:2719`; exact two-file deltas from the frozen caller copy are `client.ts.completion.diff` and `issue-budget-schemas.test.ts.completion.diff` in scratch.

## Findings (not fixed)

None requiring another caller change. The main session still owns replacing the skill source-map's design reference with real handler/test references when server integration lands (previously accepted R7), and publishing the F3 executable server spec before archive.

## What was verified

- **Write identity and outcomes:** every write requires the operator's canonical nonzero key, reason, confirmation, and (after enable) positive revision. Both amount flags are required. POST disables redirects and `GetBody`, never supplies an idempotency header, and has no application retry. 201 or replayed 200 requires understood fields plus matching operation kind/root; known-code 4xx or 401 is refusal, everything else stays unknown.
- **Manual recovery:** independent `TestIssueBudgetReviewOriginalIntentRecovery` drives POST → lost/503 outcome → unknown by-key GET → explicit same-command replay. It asserts exactly those three requests, only two operator-invoked POSTs, byte-identical request bodies, original request key and revision. Passes on Go 1.27.0 and 1.26.9.
- **Permissions:** traced `budgetStart` → `requireHumanLocalCommand` / `newAPIClient` → `budgetHumanCredential`; every subcommand refuses task/daemon/cloud tokens, agent/task identity and task/collaboration context before sending. Workspace/header behavior is covered by fake-server assertions. Server authorization remains a separate integration gate.
- **Exact money/null:** decimal dollars convert by digits into int64 ticks, with no floats; overflow, signs and excessive precision fail. Measured `null` prints unknown, configured `null` prints not set, and sized zero stays zero. A/E/R/H, estimates outside occupancy, and pre-activation gaps remain distinct.
- **Wire compatibility/recovery reads:** wrapper fields reject missing required keys, invalid/null types and duplicate known keys; extra keys and new enum strings are accepted. JSON output preserves received bytes. Independent nested-null, int64 overflow and case-aliased duplicate cases pass; a single case alias is accepted as documented. Root-addressed reads validate root identity, and pagination values are escaped as query values.
- **Core:** the schema covers the five shared show fixtures, required nullable fields and decimal strings, allows new fields/enums, returns null for malformed 2xx with redacted diagnostics, and adds no pages/hooks/stores. Independent negative/fractional/above-2^53 counts are rejected; max-int64 decimal strings retain every digit.
- **Frozen fixtures:** all 26 files passed the independent design checker (shape/order/nullability/amount arithmetic); existing CLI and Core tests consume the same files. No fixture was changed.
- **Skills:** human-only wording and no task invocation are present in SKILL, reference and source map; the one added contract assertion is the sole tracked test change. General built-in skills are embedded and their manifests are computed from content (`builtin_skills.go`, `task.go:6796`), so no manual role-skill version or digest bump is needed. No role skill changed.
- **Accepted limits:** success receipt matching compares kind/root rather than rechecking all result values; no correct-server path to a mismatched result was established. Core validates fields independently and does not enforce cross-field accounting invariants or root identity. These existing documented readings are not newly reported defects.

## Verification

Every Go test below used `DATABASE_URL=postgres://invalid:invalid@127.0.0.1:1/f3_no_database?sslmode=disable`, a private scratch HOME, canonical TMPDIR, offline Go module settings, and the agent CLI guard. No handler/router TestMain or DB test suite ran. Counts are top-level / including subtests.

| Run | Result | Evidence under scratch |
| --- | --- | --- |
| Go 1.27.0 `gofmt -l` on caller Go files | pass, no output | `gofmt.log` |
| Go 1.27.0 `go -C server vet ./cmd/multica/...` | pass | `vet.log` |
| Go 1.27.0 `go -C server build ./...` | pass | `build.log` |
| Go 1.27.0 full `./cmd/multica/... -count=1 -v` under guard | 526/2161 pass, 0 fail/skip | `cli.log` |
| Go 1.27.0 `./cmd/multica/ -run '^TestIssueBudget' -race -count=1 -v` under guard | 36/875 pass, 0 fail/skip | `cli-budget-race.log` |
| Five named pure service skill tests under guard | 5/23 pass, 0 fail/skip | `service-skill.log` |
| Go 1.27.0 independent overlay tests | 2/6 pass, 0 fail/skip | `go-independent-default.log` |
| Go 1.26.9 independent overlay tests | 2/6 pass, 0 fail/skip | `go-independent-go126.log` |
| Existing frozen Core budget tests | 92 pass | `core-test.log` |
| Frozen Core typecheck / lint | both pass | `core-typecheck.log`, `core-lint.log` |
| Independent Core tests before proposal | 96 pass, 6 privacy failures | `core-independent.log` |
| Scratch proposal with original + independent Core tests | 102 pass | `core-proposal.log` |
| Integration regression before fix | 92 pass, 7 privacy failures | `completion-regression-red.log` |
| Integration `issue-budget-schemas.test.ts` + `client.test.ts` | 221 pass | `completion-tests-green.log` |
| Integration Core typecheck / lint | both pass | `completion-typecheck.log`, `completion-lint.log` |

The five service tests were explicitly `TestBuiltinSkillsConformToTemplate`, `TestBuiltinSkillsFrontmatterIsStrictYAML`, `TestWorkingOnIssuesSkillCoversIssueLoopContracts`, `TestBuildAgentSkillBundlesProducesRefsAndFullBundles`, and `TestBuildAgentSkillBundlesAssignsBuiltinID`. This selected suite differs from the older brief's 5/27 run; the 5/23 count is intentional.

Initial Go 1.26.9 selection through `GOTOOLCHAIN` failed before any test because `GOSUMDB=off` prevented toolchain verification (`cli-budget-go126.log`). The installed binary `/Users/artisan/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.9.darwin-arm64/bin/go` then ran the independent tests with `GOTOOLCHAIN=local`; no toolchain was downloaded. The full existing budget suite was not repeated on 1.26.9.

Final integration commands ran directly from `mc-f3-completion-20261120/packages/core`, using the already installed executable files in `mc-f3-budget-7b-20261119`: `node .../packages/core/node_modules/vitest/vitest.mjs run api/issue-budget-schemas.test.ts api/client.test.ts`, `node .../packages/core/node_modules/typescript/bin/tsc --noEmit`, and `node .../node_modules/eslint/bin/eslint.js .`. The Vitest output identifies the integration source directory.

Per coordinator request, retained gitignored dependency links in the integration tree are `node_modules` → `/Volumes/artisan/code/2026/mc-f3-budget-7b-20261119/node_modules` and `packages/core/node_modules` → `/Volumes/artisan/code/2026/mc-f3-budget-7b-20261119/packages/core/node_modules`. No dependency installation, package manifest or lockfile change was made. Baseline `pnpm` script invocations unexpectedly populated registry metadata inside scratch HOME, so that baseline is not proof of zero registry access; all later TypeScript checks used direct installed Node executables.

## Limits and remaining integration work

No DB, provider, agent CLI, real budget server, UI, deployment, billing reconciliation, enforcement, or full server acceptance was exercised. This review did not repeat the prior exhaustive mutation campaign or knip. CLI debug mode intentionally retains raw error details under the existing CLI convention; default output was checked. Source-map R7 and the F3 server spec remain the coordinator's integration tasks. The only report written outside scratch is this file.

STATUS: complete
