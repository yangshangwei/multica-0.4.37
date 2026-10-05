# Administration-managed resource publication

`MULTICA_RESOURCE_PUBLISH_DIR` opts into a separate persistent managed store.
Manual Skill/MCP roots remain read-only. `ResourcePublisher` owns strict package
validation, immutable revision bundles, an atomic scoped index and independently
retained operation receipts. Administration routes use `/api/admin/resources`;
consumer catalogs continue to expose their existing deployment source protocol.

Do not confuse mutation revision UUIDs with content hashes. Every state transition
gets a new CAS revision, even equal-content republish or withdrawal. Matching
operation replay returns the original response after later mutations; reuse by a
different actor/scope/payload is a conflict. The atomic index includes both the
resource state and receipt. It is bounded and never silently evicts receipts.

Lock order is filesystem lock, platform advisory lock, then sorted account row
locks. Reauthorize after waiting for the filesystem lock and hold the actor lock
through filesystem apply. Audit the request durably before apply, then its result;
filesystem and PostgreSQL commits are not a single transaction. Failure after
apply returns an explicit unknown outcome and operation ID for receipt recovery.

Reads may retain an opened old index across an atomic replacement; never reject
that legitimate snapshot merely because the pathname now points to a new inode.
Immutable revision reads still require confined, regular, no-follow files and
integrity validation. Sync files before rename and directories after commit; keep
the lock inode stable. Unsupported filesystem semantics fail explicitly.

Manual catalog reads retain names and byte/count budgets from the actual scanned
snapshot, including invalid entries. Check those reservations and live directory
collisions on consumption as well as publication. Managed/manual same-key entries
must never silently choose a winner. Configured managed-store failures propagate
through SkillTemplates and ListSkillTemplates as errors/503; an unset feature
retains the previous manual catalog behavior.

Multipart uploads have bounded size, parts, concurrency and a30-second read
deadline. Close the body before clearing its deadline so net/http cannot drain a
stalled body forever. Do not apply global server read timeouts to WebSockets.
Archive validation rejects all unsupported files rather than publishing partial
packages. Publishing never runs commands, probes URLs, installs dependencies or
rewrites saved workspace resources/bindings.

Canonical tests: `resource_*_test.go` in service, `admin_resource_test.go` in
handler/router, plus `e2e/admin-resource-publishing.spec.ts`. Deployment guide:
`docs/admin-resource-publishing.zh-CN.md`.
