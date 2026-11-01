# Core client B1 boundary

The approved human knowledge API has no typed core client yet. Existing collaboration access epochs cover identity and issue denial, but do not impose a finite visibility lease on knowledge projections or consume target-only knowledge hints.

Implementation lives in `packages/core/collaboration/knowledge-*`: pure strict wire schemas; API request/recovery intent helpers; Query keys/options/hooks; a reusable monotonic authorization lease. `api/client.ts` adds the eight human endpoint families with existing authenticated transport and schema parsing. `types/events.ts`, `realtime/collaboration.ts`, and a focused knowledge realtime binding connect content-free invalidations. The collaboration barrel exports the seam. Colocated Node tests own schema, transport, intent and cache policy matrices; any hook test will cover wiring only.

Before refresh or authority change, advance the scope generation, cancel in-flight reads without rollback, and erase all knowledge projection data. Conservative scope-wide clearing covers target/source relations that may be hidden or still in-flight. Receipts remain content-free and recoverable without source authorization. Never copy response data into Zustand or storage. No dependencies or lockfile changes.

No Go, SQL, router, views, new platform Provider, runtime/daemon, existing planning artifacts or commits are in this slice. Full GET DTO field names not fixed by design §4 will be recorded here and coordinated with the protocol owner. Existing collaboration paths retain behavior; only additive knowledge event handling is changed and its old empty-workbench payload rule is regression-tested.

Validation: focused Node schema/transport/cache/action/realtime tests; core typecheck and changed-file ESLint; exact source hashes and command/test logs under this directory. Backend, UI and native browser acceptance belong to later batches.

Implementation clarification from main: “No Provider” meant no external model/account calls. Narrow existing React WSProvider and WSClient edits were authorized to expose immediate disconnect; no new provider was introduced. Client-generated content-free recovery identities are persisted through StorageAdapter before POST; full intent bytes stay in memory and no server state/body enters storage.
