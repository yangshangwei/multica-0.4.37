# Verification

Completed 2026-09-29. User selected `http://<api-host>:18080/desktop`.

## Implemented

- Runtime parser derives the default from the business hostname, including IPv6, independently of its protocol, port, path, query and fragment.
- Saving configuration persists the default. Existing files missing the field are backfilled atomically while preserving unknown fields. A migration write failure is logged without preventing use of valid runtime configuration; explicit save errors still propagate.
- Custom update sources survive server changes. A source equal to the old standard directory follows the new server hostname.
- Updater reads current configuration before checks. Initial unconfigured state never contacts GitHub. Source changes wait for old checks/downloads; same-source requests coalesce. Queued automatic work is discarded if the preference or current server changes.
- Installation documentation, runbooks, desktop spec and generated Chinese embedded page are synchronized.

## Evidence

- First regression run: 13 expected failures (missing defaults, missing disk migration, stale startup source), followed by 64 passing tests after implementation.
- Review regressions: reproduced migration write failure and old-source check reuse; both pass after correction.
- Queue regressions: reproduced 2 failures for disabling automatic updates or changing servers while waiting; both pass after correction.
- Final `pnpm -C apps/desktop test`: **65 files, 733 tests passed**.
- Final `pnpm -C apps/desktop typecheck`: **passed** (main and renderer).
- Final `pnpm -C apps/desktop lint`: **0 errors**, one existing `react-hooks/exhaustive-deps` warning in unchanged `tab-content.tsx:54`.
- `pnpm -C apps/docs test`: **9 files, 62 tests passed**.
- `go test ./internal/docs/...`: **passed**.
- Docs generator: **36 pages and 58 assets**; copied only the changed generated desktop page from a temporary bundle.
- `git diff --check`: **passed**.
- Independent review approved after the migration and queued-update edge cases were fixed; reviewer also exercised rejected old check/download promises in an isolated harness.

## Boundaries

No actual user desktop.json was changed. No installer build/publication, OS installation, live intranet availability, signing/elevation or cross-version installation was exercised. The installed v0.5.2 app needs a one-time manual updateUrl configuration or replacement with a fixed build. Backend docs handler/DB integration was not rerun; only embedded content changed.

Unrelated pre-existing workspace changes were left untouched.

## Workflow note

Use `task.py archive --no-commit` in a shared dirty checkout: automatic archival stages unrelated task directories as well. This run corrected that generated local commit with a mixed reset, preserving every working file, then committed only this task's archived records. Inline Lore commit paragraphs and the OmX co-author trailer are required by the local pre-tool hook.
