# Verification — curated MCP catalog publishing

Completed locally on 2026-10-02. Base: `aa0d3ed83`; implementation branch:
`codex/mcp-catalog-publishing`.

## Delivered

- Added keyless Microsoft Learn and DeepWiki Streamable HTTP recipes; retained
  the three original recipes and revisions. Official source and anonymous
  protocol evidence is in `research/official-sources.md`.
- Added English/Chinese documentation-category filtering and key-stable book /
  search icons, including renamed saved configurations.
- Added `make check-mcp-catalog` and `docs/mcp-catalog-publishing.md`: candidate
  review, metadata/configuration gate, recipe revision, backend release,
  withdrawal and rollback. Updated contributor entry point and built-in skill.
- Reused existing server-side resolution, storage and explicit assignment; no new
  dependencies, database schema, endpoint, global store or catalog-loader layer.

## Automated evidence

| Check | Result |
| --- | --- |
| New content test before implementation | Failed because both reviewed HTTP recipes were absent |
| New frontend regressions before implementation | Four expected failures for documentation filter / new icons |
| `make check-mcp-catalog` | Passed; service-only offline gate |
| Publishing-gate corruption experiment | Rejected version 0, missing explicit Chinese title and HTTP documentation URL; restored file and gate passed |
| Isolated migrated PostgreSQL handler suite | 25 test/subtest pass events, zero skips; both HTTP recipe lifecycle cases exercised actual save, no implicit binding, explicit assignment and runtime configuration resolution |
| Core MCP schema/client/mutation suites | 3 files, 15 tests passed |
| Views MCP/settings/locales suites | 5 files, 125 tests passed |
| Core/Views dependency lint + typecheck | 6 tasks successful; Views has 26 existing warnings, no errors |
| Changed-source lint + E2E TS static check | Passed |
| Desktop node/renderer typecheck | Passed |
| `go vet ./internal/service ./internal/handler` | Passed |
| Impeccable mechanical detector | No findings |
| Production Web and Electron builds | Passed |
| Production Web + native Electron E2E | Four tests passed across two runs; zero retries |
| Visual review | 96/100, pass; `visual-verdict.json` |

The handler suite uses an isolated new database, runs migrations, inspects JSON
pass/skip events and drops the database afterward. The existing handler TestMain
can exit 0 before executing tests if PostgreSQL is unavailable; the offline
publishing gate therefore uses service tests only.

## Browser and native evidence

Production verification used a detached worktree at
`/Volumes/artisan/code/2026/multica-mcp-catalog-verify` with the exact same 13
changed application/test files (byte-compared against the implementation
checkout). The isolated API was at localhost:18637, Web at localhost:13557.
`web.running.json` records production mode, source fingerprint and build ID;
`api.running.json` records API ownership. The existing checkout's older listeners
were not stopped or reused.

```
MULTICA_DEV_HOME=/tmp/multica-mcp-catalog-dev make up C=api,web ARGS="--name mcp-catalog-verify --ephemeral --ttl 12 --web-mode production"
pnpm --filter @multica/desktop exec electron-vite build
```

Using `dev-env.sh exec mcp-catalog-verify -- env E2E_PASSWORD_AUTH=1`:

1. Playwright `mcp-market.spec.ts` / `mcp-desktop.spec.ts`, grep `collection canvas|creates all five|retries a failed|native desktop MCP`: 3 passed in 15.9s. The recipe test's updated title did not match that grep and was run explicitly next.
2. Playwright `mcp-market.spec.ts`, grep `preserves source after rename`: 1 passed in 12.4s. This exercised all five recipes, explicit assignment, rename/icon provenance, reuse, long names and Chinese presentation.

Native validation launches a real Electron BrowserWindow with newly built
renderer/preload, an isolated profile and the existing native fixture. It saves
and assigns all five recipes, checks HTTP/stdio identity, public requirements,
category/search, narrow layouts, settings embed and agent discovery. It reuses
the same lockfile-version Electron binary from the original dependency install.
All temporary E2E users/workspaces/profiles were cleaned up by the fixtures.

Artifacts: `.omx/artifacts/mcp-catalog-publishing/` contains screenshots, logs,
source fingerprint records and the gate-failure evidence. No visual correction
was required after the batched review.

## Review and limits

An independent read-only review found no blocking defect and one stale three-
recipe description, which was updated to five. The added behavior is covered
without starting an agent CLI, executing provider tools or using credentials.
Official anonymous initialize/tools/list checks are separate point-in-time
source evidence, not ongoing availability tests.

The existing email-invitation member E2E was not rerun against the password-only
verification API; member/actor authorization remains covered by handler and
component suites. The scope is local implementation and publishing workflow:
no remote push, release tag or production deployment was performed. Remote HTTP
recipes still depend on outbound connectivity and provider availability.

## Resumed verification — 2026-10-02

Recovered the completed implementation at `c1c5f7f9a` on
`codex/mcp-catalog-publishing`. Application sources were clean; the task artifacts
had not yet been committed or archived. The implementation and scope decisions
above remain unchanged.

Fresh checks during this continuation:

- `make check-mcp-catalog`: passed with `-count=1`.
- Core MCP schema/client/mutation suites: 3 files, 15 tests passed.
- Views MCP/settings/locales suites: 4 files, 65 tests passed.
- Shared MCP server-row suite: 1 file, 4 tests passed.
- Core/Views lint and typecheck through Turbo: 6 successful cached tasks,
  including dependency typechecks; 0 errors, 2 existing Core lint warnings and
  26 existing Views lint warnings.
- `go vet ./internal/service ./internal/handler`: passed.
- `git diff --check`: passed.

The 84 TS tests above are this continuation's actual selection, separate from
the previous session's broader reported 140-test run. The earlier handler JSON
artifact was inspected: 25 test/subtest passes, 0 failures, 0 skips and a package
pass event. Production Web/native Electron, database lifecycle and anonymous
provider checks were not rerun in this continuation; their earlier evidence is
recorded above. The package manager also emitted its existing warning that
`package.json` pnpm settings are ignored by the installed pnpm version; this run
did not install or change dependencies.

### Review correction: unbraced input placeholders

The independent continuation review found that `$WORKSPACE_ROOT` and
`%WORKSPACE_ROOT%` bypassed the original placeholder pattern. Four temporary
catalog mutations reproduced the issue: a variable command, a POSIX variable
argument, a Windows variable argument and a variable documentation URL each
incorrectly passed the original gate. The original roster was restored after
each experiment; none of the five shipped recipes contains these placeholders.

Added 11 detector cases; the four new unbraced-variable cases failed before the
fix. Commands, arguments and URLs now use the same test-only pattern, retaining
the existing braced/angle forms and accepting concrete commands/package names,
URLs and literal percentage values. All four corruption experiments then failed
with the intended gate diagnostic; restoring the real roster passed the gate.
Evidence: `.omx/artifacts/mcp-catalog-publishing/placeholder-gate-regression.log`.
The publishing guide and MCP spec now record this invariant. This correction
changes only the offline test gate and its documentation, not runtime behavior.
The independent re-review found the issue resolved with no remaining blocker.
The correction was committed as `a041caebf`; the feature commit is `c1c5f7f9a`.
Go vet and whitespace checks also passed after the correction. All acceptance
criteria are met locally; the task can be archived without a remote release.
