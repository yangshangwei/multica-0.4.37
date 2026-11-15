# Verification design

Use a detached same-commit checkout with its own dependency resolution and production build. Original checkout product files remain unchanged. Each run has a unique directory and source/config fingerprint.

Root coordinates database allocation, verifies actual target identity and owns cleanup. API/browser database is separate from Go tests. DB-backed Go packages use fresh independently migrated/cloned databases, guarded against ambient agent CLI execution. Children may execute only the assigned package/database permit and cannot allocate or drop databases.

Run frontend static/unit and Mobile checks independently from read-only platform inventory. Bound heavy concurrency. Browser suites run sequentially in one production Web/API environment per auth/gate lane; distinct global resource catalogs require separate environments or serialization.

Use TestApiClient for setup, actual product actions for business writes, API and selected SQL for persisted assertions, second browser/native client for realtime. Keep zero automatic retries. Record setup failures separately from business assertion failures.

Capture initial/final candidate and dependency identities, API/Web PID/commit/Build ID/config and endpoint alignment, exact named inventory, JSON results, traces/screenshots and cleanup ownership. Never print tokens, source env values, cookies or credentials. Real-model account/model/call-limit selection remains an external prerequisite when unspecified.
