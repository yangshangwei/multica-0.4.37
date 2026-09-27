# Upstream backport ledger

This fork preserves its own two-language product, template/copy semantics,
authorization, RCA/lifecycle handoffs, device identity and intranet releases.
Record selected upstream changes here rather than treating an upstream tag as
proof that all of its contents were imported.

## Batch B1

- Local base: `7845de31e6bc7d021bba6005bccdab44fc9958e6`.
- Reviewed upstream boundary: `0a51a6cc433a926548f40ad6b01275468874cdaa`.
- Execution branch: `sync/upstream-b1`.
- Status: verified and fast-forwarded into local `main` at `9ed9c8e018d332dd2493c95bba07ea1fbaf1e3bf`. No remote push, release tag or deployment.
- No database migrations, dependency changes, retired locale resources or backend API implementation changes.

| Feature | Upstream | Local commit | Import scope | Focused verification |
| --- | --- | --- | --- | --- |
| CLI skill labels | [#8368](https://github.com/multica-ai/multica/pull/8368), `904693bed94f9bd0cd93908014114a9ebbead8e0` | `e2c2865a214cf899298b8868d0e9c4f2efd94a81` | Six CLI implementation/test files; current en/zh CLI and importing-skill guidance; generated Chinese help | Missing Cobra routes reproduced; 42 guarded Skill/Label checks pass; independent review |
| CLI comment editing | [#7507](https://github.com/multica-ai/multica/pull/7507), `91ad87186437230303d7a6862a16089656b4fdbe` | `0edab269d779b48c101e8c2fa2ad4c82abc6c31c` | Two CLI files; current working-on-issues and CLI guidance; help conflict advice adapted from #8548; generated Chinese help | Missing route reproduced; 62 guarded comment/input checks pass; 403/409 do not retry; independent review |
| Cache hit rate | [#8798](https://github.com/multica-ai/multica/pull/8798), `470fd1fc0dcbbb8b86a8d13c6b4c43b6f2a10e1f` | `67d97e6126e0072429e17815a3663f158511b4f6` | Exact six-file views patch; pricing and persisted usage unchanged | Wrong rendered 100% for 72K read/28K write reproduced; 113 focused checks pass; views typecheck/scoped lint; independent review |

### Deliberate exclusions

- #8368's backend, API schema, view-store, skill-list and locale changes were not imported; the fork already has richer skill presentation and label behavior.
- No `multica-platform` directory was restored. Guidance was added to the current split skills and source maps.
- Comment changes use the existing revision/author/admin/invocation contracts, preserve attachments and never automatically retry a rejected revision.
- Cache fixes include cache writes in input totals; they do not backfill old usage or change prices.
- No remote push, release tag or production deployment is part of B1.

### Verification and rollback

Full evidence: `.trellis/tasks/archive/2026-09/09-27-upstream-backport-b1/verification.md`.

- Static lint/typecheck completed successfully across 9 packages; UI wildcard
  exports were clean. Existing lint warnings remain.
- Focused Go CLI tests passed; full `scripts/test-go.sh --race` and guarded
  `go vet -p 2 ./...` passed using an isolated, migrated verification database.
- TypeScript package suites passed: core 2,057, docs 62, views 5,446 (serialized
  rerun), web 261 and desktop 718 tests. The first parallel aggregate run had one
  failure in the unrelated issue-limit dialog Escape test (5,445/5,446 views
  passed); the focused test passed on both base and feature, and the complete
  views suite passed with one worker.
- Production Web built and served from commit `67d97e612`. The dedicated CLI
  integration suite passed 3/3 against both development and production Web/API.
- Full `make check` did not finish because its parallel views run hit the
  issue-limit dialog failure above. Its remaining Go, API and full Playwright
  phases were run separately as listed here, except the repository-wide
  Playwright suite; the B1 integration suite was run in both modes.

Each feature has its own local commit and can be reverted independently after
checking later dependencies. Reverting code does not undo intentional user
edits to comments or labels.

## Remaining candidates

B2 (mention boundary, known image type, menus, BorderBeam), B3 (malformed member
mentions, object-valued MCP summaries, Save View draft retention) and optional
plugin/mirror/config/native-image changes remain unimported. Module-scale settings,
status, steering/wakeup and attachment-viewer upgrades require a separate plan.

For future updates, compare new local/upstream changes against each row's actual
files and contracts. Preserve complete upstream and local SHAs, record follow-up
fixes/reverts, and never mark a partial import as a complete upstream merge.
