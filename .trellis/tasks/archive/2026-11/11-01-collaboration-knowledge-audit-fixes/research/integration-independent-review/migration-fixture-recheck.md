# Migration fixture delta independent recheck

Verdict: **pass** for the uncommitted canonical fixture delta based on `45efa093c`. Reviewed file SHA-256 `4aaa3b3e1038ed8ee4efe478d09f8450e375916eafab11d1493fa8aa50d6526e`; bind the eventual commit to these bytes. No production source/SQL changed. Reviewer `/root/knowledge_web_review` wrote only this report directory and ran no database, migration, Provider or installed Agent CLI operation.

The missing columns are preexisting production prerequisites, not columns manufactured to conceal a bad761 migration: `agent_id UUID` and `created_at TIMESTAMPTZ NOT NULL DEFAULT now()` belong to the initial agent_task_queue table in001; runtime_id UUID was added by004, long before747. The fixture intentionally omits irrelevant legacy constraints; its new UUID columns remain nullable so existing minimal legacy/test rows continue to exercise knowledge metadata behavior. No FK/cascade or dependency was added.

The existing knowledge migration glob now reaches761–763. The preserved red log contains12 missing-agent_id leaf failures plus an aggregate parent failure, yielding13 named failures. Each leaf failed while applying761; production/history index SQL does not need alteration. Both original log hashes and the new root-owned rerun hash were verified.

The added assertions use `pg_indexes` scoped to `current_schema()`, so same-named public indexes cannot make the fixture pass. A missing history index now fails explicitly, and the prior invalid concurrent-index check still runs. Exact index key/predicate correctness remains established by the unchanged previously reviewed migration SQL; the name check alone is not claimed as a drift detector. All existing fresh/legacy-session, reapply, protected-history down and empty-down cases are unchanged and still execute against the isolated schema.

Independent compile-only `go -C server test -c -o /dev/null ./internal/migrations` exits0 with DATABASE_URL removed; no TestMain/database operation ran. The coordinator's new explicitly allocated migration database rerun completed536 named passes,0 failures/skips,exit0. The original523pass/13fail attempt is retained separately. Exact SHA/delta/log bindings and command are in `migration-fixture-recheck.json`.

This is a test-only correction. Prior product checks keep their original45efa scope, and final make check/combined delivery gates remain open. No reviewer self-fix was applied.
