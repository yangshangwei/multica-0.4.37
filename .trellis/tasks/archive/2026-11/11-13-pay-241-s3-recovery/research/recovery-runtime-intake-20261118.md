# S3 Extras-B controlled runtime authoring intake — 2026-11-18

This is a bounded implementation handoff, not runtime acceptance. The native
implementer owns only `e2e/collaboration-recovery-runtime.spec.ts`,
`e2e/fixtures/collaboration-recovery-runtime.ts` and new reports under this task.
Main owns source integration, databases, configuration, builds, API/browser
execution, processes, independent check, matrix updates and commits. Existing
S2, Extras-A, blocking, F3, UI and security work and all failed evidence remain
outside this write scope.

The missing composed evidence is actual result-notification recovery and
withdrawal of an originally valid runtime business capability. These behaviors
belong to the existing public participant/grant APIs, ordinary host Provider
adapter, HostRunner, original run ledger, permit/outcome/proof routes and shared
Web/Desktop consumers. No product, protocol, gate default, Provider dependency
or parallel recovery mechanism is introduced.

## Selected execution lane and exact prerequisites

Use the existing native `host_process_supervision_v1` lane: protocol3,
preparation2, execution read6 and one real machine slot. The stdlib-only Provider
speaks ordinary Claude stream-json, reads context with the actual run-local CLI
and waits on its controlled callback. Host exit is managed-process evidence;
this lane does not assert Linux namespace containment, all-descendant cleanup,
a commercial model account or installed full Electron Main.

Main builds from its final frozen server source:

```text
go -C server test -c -o <absolute>/s3-host-daemon.test ./internal/daemon
go build -o <absolute>/s3-host-provider e2e/fixtures/collaboration-host-provider.go
go -C server build -o <absolute>/s3-host-cli ./cmd/multica
```

The daemon's actual guarded entry is
`[<binary>, "__multica_collaboration_host_router_poller"]`, implemented by the
`init` guard in `server/internal/daemon/collaboration_host_router_test.go`.
It receives issued managed configuration on bounded stdin. It is not a
`TestCollaborationHostAcceptanceHarness` body. The Provider must report
`multica-host-e2e-provider-v1` for `--fixture-kind`, and the existing
`--fixture-capabilities` must include `test_report`, `failure_release` and
`prepared_git_marker`. The CLI source entry is `server/cmd/multica/main.go`.
No installed Agent executable lookup is allowed. Optional no-DB boundary
verification remains main-owned:
`go -C server test ./internal/daemon -run '^TestCollaborationHostHarness(TimeoutBoundary|HoldsOnlyCommittedReceiptAcknowledgements)$' -count=1`.

Required closed runtime configuration includes
`PAY241_S3_RECOVERY_ACCEPTANCE=1`,
`PAY241_S3_RECOVERY_RUNTIME_ACCEPTANCE=1`, `E2E_PASSWORD_AUTH=1`, the exact
`TASK_DATABASE_NAME` / `DATABASE_URL`, matching `COLLABORATION_E2E_API_URL` /
`NEXT_PUBLIC_API_URL`, `PLAYWRIGHT_BASE_URL`, `MULTICA_DEPLOYMENT_ID`, matching
`COLLABORATION_ELECTRON_RENDERER_URL` / `COLLABORATION_ELECTRON_SOURCE_ROOT`,
absolute `COLLABORATION_HOST_DAEMON` / `COLLABORATION_HOST_PROVIDER` /
`COLLABORATION_HOST_CLI`, trusted private `HOME`, temporary roots and
`PAY241_S3_PROFILE_ROOT`. Main checks the full migration/index ledger, source,
build, configuration, deployment/listener identities and pool budget before
entry. Actual creation/execution/host flags are saved, explicitly opened only
in the fresh runtime allocation, then restored with a monotonic epoch.

Main's intended combined source is `source-block-integrated-r03`; its final
serverR04/clientR03 origin manifest must be supplied before any body. Existing
API64940/Web64941/renderer64942 and all earlier databases are retained. Actual
Extras-B bodies use another fresh database/API binding with that same source.
Author checks do not read a candidate before main creates it.

## Frozen composed inventory

Exactly Web/Desktop ×2 tops = **4 tops /10 named subcases**, workers1, retry0:

| Top per platform | Named subcases |
| --- | --- |
| Result notifications | `proven_success_result`, `proven_failed_or_incomplete_result`, `proven_stopped_result` |
| Runtime withdrawal | `queued_input_author_withdrawal`, `live_resource_grant_withdrawal` |

The failure result precedes the explicit success on the same issue so the latter
must preserve the old failed run, record, notification, input and receipt. Each
result subcase deliberately creates one new command/run/physical launch. The
queued-author subcase creates one live run plus a separately identified waiting
input from that same external participant, then removes the author. The live
grant subcase uses another issue and withdraws its actual owner grant while its
Provider is live. Thus both withdrawals have original-token200-before and
denial-after evidence. Expected intentional runs/launches are five per platform,
ten total; waiting inputs, comments, uploads, results, notifications and HTTP
replays have separate actual denominators. Recovery contributes zero launch.

## Transport and identity boundary

Compose `createHostActor`, `enrollHostRuntime`, `startHostDaemon`,
`adoptHostTarget`, `sendHostComment`, `hostControl` and the existing recovery
session/socket owner. Password actors, explicit separate workspaces,
installation enrollment, offer/adoption/consent and withdrawal use real APIs.
A small owner-local loopback HTTP transport observes the genuine permit and
forwards original request/response bytes unchanged. Only a natural proof request
barrier or a deliberate successful ACK loss changes transport timing; no fake
permit, proof, authoritative status, permission or result body is supplied.

The genuine `mcb_` business token remains private memory. Daemon management and
termination credentials are distinct and compared by hashes. Raw credentials,
auth frames, private paths and nonce values never enter public evidence. Original
outcome, proof, snapshot and memory-base requests retain bytes and authorization
in the private transport for exact replay. A new poller reuses its original
directory and ledger; no claim, permit, handoff refresh or Provider start may be
attributed to recovery. The Desktop UI fixture separately requires daemonStarts0.

Before withdrawal, read context, preparation, the real input file, comment page
and content, memory, live handoff, an original frozen handoff replay and genuine
source chunks with the issued token. Repeat the same paths/token after real
withdrawal and record exact current refusals. Original content-free outcome ACK
is checked where a natural outcome was actually recorded; changed or new late
outcomes must not write business effects. Runtime attachment routes stay
read-only; this does not claim Agent output upload.

## Author-only verification

Permitted checks are strict targeted TypeScript, targeted nonignored ESLint and
Playwright `--list`, using main's closed no-DB wrapper and a coherent private
harness graph. Every failure uses a newly numbered retained output directory.
No TestMain/database test, E2E body, build, Provider, browser or process control
is executed by this implementer. Complete runtime evidence and independent
acceptance remain required before updating any S3 checkbox or parent row.

## Later coordination disposition

Main's latest directive limits its current execution turn to the unique
Extras-A R05 owner. Coordination session
`01a1ee9f-bdfc-7f31-ac53-9d7e072a20d7` owns the parent/task plan and retained
R04 database audit. Extras-B authoring continues to its sealed static handoff;
no Extras-B build, provision or body was started. The intended combined
serverR04/clientR03 source remains a future binding. Actual author checks use
the existing `source-r01` client packages and a coherent snapshot of current
canonical E2E helpers. This does not accept the eventual combined candidate.
