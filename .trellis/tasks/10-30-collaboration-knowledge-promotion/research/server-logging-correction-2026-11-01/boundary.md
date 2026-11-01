# RR2 server logging correction boundary

Only `service/task.go`'s successful failure warning, `handler/daemon.go`'s successful failure info record, and new focused server logging tests are owned by this correction. C2/C3/C4 source and evidence remain intact. No Provider, sub-agent, commit, default database access or shared logger-policy mutation is permitted.

The actual returned task row decides whether generic logs may include raw failure text. Any persisted negotiated/retained knowledge version or delivery metadata suppresses the raw field, even when the failure is an ordinary Provider failure or current negotiation/gate is absent. Task identity and persisted canonical failure reason remain. Task error storage and authorized transcript/comment evidence are unchanged. A request fence alone does not label a legacy task private.

Regression strategy: an external service-package test invokes a test-owned child test process on the explicit C2 database. The child drives actual preparation, service failure and authenticated handler failure and emits a content-free persistence receipt. The parent captures the child's normal stderr/stdout without changing `slog.Default`, process logger policy, or global output streams. Cases cover prepared, retained, refusal and stale-fence tasks plus ordinary legacy positives. Children and package runs are sequential.

The sole test database is `multica_knowledge_c2_20261101_7d95f46f`, loaded from its existing private environment file. RR2 remains a reviewer finding until main/reviewer recheck; this correction does not declare the gate closed.
