# F3-S1 final local delivery — 2026-11-20

Delivered product commit `584846f95d30f70041b583e5b11eb12a859aa6df`; local integration commit `8126073f638a6db2636dc1c10575836e1bff46fc` combines it with shared parent `f2ea7d50d0e2ae412e1ee7961c47247390f20478`. Delivery branch: `codex/f3-completion-20261120`. Shared local branch `codex/cross-workspace-collaboration` was fast-forwarded to the identical verified product tree. No remote publication occurred.

The integrated tree is `695a4a84fc50a9223f56cda07fbfb2cf89936452`. All7916 recorded source hashes matched before and after the fast-forward.13200 unrelated dirty files were hashed immediately before and after; none changed. Original Claude/6a/6b/7b/review worktrees remain untouched. Ownership and reconciliation are in [takeover report](codex-takeover-20261120.md).

## Delivered behavior and changes

- One lifetime root per ordinary parent, immutable member attribution across moves/retries/deletion, atomic claim attempts and conservative reservation holds; quick-create uses verified execution identity and its stored context parent.
- USD integer ticks, frozen prices, actual/estimated/reserved/unknown separated; immutable original-byte batches and original-key receipts; no amount-based execution refusal.
- Human owner/admin API and `issue budget` CLI, no new Web/Desktop page. Daemon capability/journal and Core schema/redaction are integrated.
-213 feature files cover server budget/service/handler/daemon/CLI/protocol/SQL, Core API/schema and executable spec. Migrations869–885 preserve shared866–868. Built-in skill/source map points to the actual human-only boundary.
- Final cleanup removed stale documentation claims, clarified one retention-order test comment and used existing auth/writer/transaction seams. No dependency or speculative compatibility layer was added. Two F2 test request counters were synchronized without changing assertions or production behavior.

## Verified combined source

Each selected DB package and mode used a fresh task-owned database, sequentially, under the agent CLI guard. Every Go test had an explicit DATABASE_URL. The migration template had873 applied migrations through885, no workspace and no invalid index; isolation was read committed.

| Go suite | Normal named PASS | Race named PASS |
| --- | ---: | ---: |
|Service F3+B54+built-in contracts|393|393|
|Handler F3+B54|159|159|
|Router F3+B54|54|54|
|Migration catalog|4|4|
|Migration runner/readiness|6|6|
|Pure budget|112|112|
|CLI F3+B54|993|993|
|Middleware|8|8|
|Report snapshot|28|28|
|Daemon F3|298|298|
|Metrics full package|88|88|

All selected runs exit0, package terminal PASS, zero failure/skip/missing names. Counts include named subtests. The row binding covers62 S1 rows,142 cited names and571 expected top-level names. All91 canonical M1/F2 names also pass in pristine86fea baseline and the final combined normal/race streams. This is selected backend acceptance, not a full-repository backend claim.

Core:374 files/5355 tests. Views:619 files/7610 tests. Both full suites exit0. Go build/vet, Core lint/typecheck/knip, Views typecheck and F3 diff whitespace checks pass. Incoming shared historical PAY reports contain preexisting whitespace findings; their bytes were preserved and the separate failed check is retained. No assertions or per-test timeouts were relaxed. The earlier feature-only Core366/5172 and Views608/7441 runs remain separately identified.

Independent native checks verified original-tree integration, both final raw-log inventories, all7916 source hashes,34 SQL declarations/generated methods,15 single-statement concurrent indexes/cleanup/readiness registrations, source-map paths and deployment spec. No unresolved F3 functional blocker remained.

## Persistent evidence

All key evidence is in [codex-evidence-20261120](codex-evidence-20261120/README.md). Its manifest binds uncompressed bytes/hashes to each original source. Start with:

- `takeover-integrated-run-binding.json`: exact normal/race names, canonical B54 and log hashes.
- `takeover-integrated-db-summary.json`, `takeover-nodb-summary.json`, `takeover-integrated-frontend-summary.json`: commands, actual exits and raw results.
- `takeover-integrated-source-manifest.json.gz`, `takeover-shared-fast-forward-proof.json`: source identity and shared-branch delivery.
- `b54-canonical-before-after.json`, baseline raw JSONL and pristine verification: regression comparison.
- Original CLI fixture/race failures, Core privacy red/green, integration fixture failures and all Views dependency/cwd/concurrency failures are retained. Unfinished KD1 review tests remain historical material; no completed-review claim is made for them.

## Known limits and remaining roadmap

Production evidence contains only Claude normalizer1 fresh invocation scope and no trusted reported-cost row. Production actual therefore remains0; supported fresh usage can contribute estimates, and unsupported/resumed/ambiguous scopes remain unknown. Synthetic injected actual and pure settlement/correction tests are not provider billing evidence.

No production settlement/release/correction authority, enforce mode, amount-based cancellation or billing reconciliation. Seven-day journal expiry, account/profile changes and persistence failure can lose telemetry while server holds remain unknown. Lifecycle per-root deadlines and large-ledger replay can delay convergence; paginated manual reads may scan/sort many rows. Deployment requires read-committed defaults and all server instances upgraded before enable; both read-only gap probes and retained-history/off rollback are now in the spec.

No real Provider, user-installed agent CLI, POLA-1, shared project policy operation, push, PR, deployment, publication or Hosted CI was performed. F3-S2/E01–E06, F4, F5-A–E and M1-S2–S6 remain open; the parent roadmap remains in_progress. This child alone is archived at `.trellis/tasks/archive/2026-11/11-16-f3-parent-budget-root`; the parent remains active.

After evidence retention and zero-connection checks, the11 databases allocated by this takeover were removed. All earlier Claude/cx databases, original worktrees and failure evidence were preserved. The database named `multica` was not queried for cleanup or modified. See `takeover-cleanup.json`.
