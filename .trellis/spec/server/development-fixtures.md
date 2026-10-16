# Persistent Desktop Development Fixtures

## 1. Scope / Trigger

Use `scripts/seed-rich-test-data.mjs` to populate an existing local workspace for manual backend/Desktop QA. E2E `TestApiClient` fixtures remain ephemeral; do not run their teardown against this retained dataset. No migration, new dependency, account credential, daemon registration or agent executable is involved.

## 2. Signatures

```sh
make seed ARGS="--workspace <slug> [--dry-run] [--reset]"
DATABASE_URL=<local-postgres-url> pnpm seed --workspace <slug>
pnpm test:seed
SEED_TEST_DATABASE_URL=<disposable-schema-db> pnpm test:seed
```

The Make target uses the checkout's exported environment. Direct `pnpm seed` requires an explicit environment `DATABASE_URL`. `--reset` deletes the fixture and does not reseed. Integration tests require database names beginning `multica_rich_seed_verify_` or `multica_seed_test_`; they never discover `.env` or use the live development database.

## 3. Contracts

- `FIXTURE_KEY = multica.rich-desktop.v1`; IDs derive from fixture version + workspace UUID + scenario key. Descriptions use `<!-- fixture:multica.rich-desktop.v1 -->`; JSON markers use `fixture_key`. Titles are display labels, never deletion authority.
- Seed reuses existing owner/member accounts. One transaction reserves unique issue numbers and inserts the complete dataset. Any existing fixture makes missing roots/children intentional deletions; reruns preserve edits, read/archive cursors and deletions.
- Missing triage and iteration settings mean enabled, matching the backend. Explicit disabled settings omit corresponding membership/intake samples without changing the settings. Reset always considers the full fixture identity set.
- Planned iteration participation and `planned_activity` events match `internal/iteration/membership.go`; never fabricate a completed iteration without its canonical frozen snapshot. Existing active/closed iterations are retained.
- Inbox types are `new_comment`, `mentioned`, `issue_assigned`, `status_changed`, `task_failed`; status details use `from`/`to`. Chat suggestions and agent starters use `label`/`prompt` objects.
- Synthetic tasks use an offline runtime with null daemon ID and exhausted automatic retry budget. Normal stale-runtime sweeping remains authoritative; a queued demonstration may become failed after reconnect grace. The script preserves that transition.

## 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Missing database, non-PostgreSQL URL, remote host or host/database query override | Refuse before connecting |
| Workspace or owner absent | Roll back; create no account/workspace |
| Dry run | Repeatable-read/read-only transaction; no number reservation |
| Fixture marker missing, runtime online/connected, agent rebound | Refuse and roll back |
| Non-fixture UUID, UUID-array, JSON/text reference into fixture | Refuse reset before deleting |
| Any mid-insert SQL error | Roll back reserved counter and all parents/children |

## 5. Good / Base / Bad Cases

Good: seed once, edit a title, read a notification, then repeat; all edits remain. Base: a new local workspace receives state/priority/relationship samples. Bad: identifying disposable data from a Chinese title prefix or deleting a seed project after a user attached their own issue.

## 6. Tests Required

`scripts/seed-rich-test-data.test.mjs` covers target validation, stable IDs, planned facts, inert executions, readonly counters, real transaction rollback, default-enabled triage, retained edits/deletions, external references and cleanup after feature toggles. For delivery, also read real API projections and load the current Desktop renderer in an isolated native profile; an inserted row alone does not prove the consumer contract.

## 7. Wrong vs Correct

Wrong: direct-insert arbitrary inbox type strings that pass PostgreSQL, or seed a completed iteration header without its snapshot. Correct: follow the shared API/UI enums and canonical iteration facts, then verify their real GET endpoints and native views. For native test harnesses, align origin/proxy behavior with the installed app and finish pending requests before changing routes.
