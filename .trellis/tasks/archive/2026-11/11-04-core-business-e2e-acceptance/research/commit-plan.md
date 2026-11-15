# Reviewable change boundary

Suggested work commit: `test(e2e): keep core acceptance aligned with current collaboration contracts`.

Owned test files are recorded in `owned-change-manifest.json`; reuse-only ROOT commits from other sessions are excluded. Raw evidence is ignored; private runners, credentials, traces and databases are not commit inputs. `database-test-isolation.md` has earlier parallel edits: only the new `Test state must not inherit a repository` section belongs to this task.

No commit was made: full acceptance remains failed and the current shared checkout contains unrelated concurrent work. The run is complete with explicit failed gates, and the Trellis task remains in progress for their closure.
