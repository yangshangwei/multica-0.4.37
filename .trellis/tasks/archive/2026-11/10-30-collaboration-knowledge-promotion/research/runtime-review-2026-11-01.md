# Independent C2/C3/C4 runtime review — 2026-11-01

Status: **changes required; G2 remains open**. One P2 finding is open. This is a read-only product review and retained-evidence audit, not a new execution of the acceptance suites.

## Scope

Reviewed approved PRD AC6–AC8 and design/implement K1–K5/F1–F4, focusing on C2 ordinary server negotiation/history/fences, C3 ordinary daemon consumption, and C4 collaboration consumption. A2 human/resolver and F3 invalidation have separate independent reviews; this pass checked their runtime integration contracts rather than repeating those suites.

Product files were not edited. No database, tests, Provider CLI, account, commit, or sub-agent execution was performed by this reviewer. Only this report and its JSON companion are reviewer-owned.

## Open finding

### RR1 — P2: raw Claude stderr bypasses the private-input log boundary

- `server/pkg/agent/claude.go:128` always creates `newStderrTail(newLogWriter(b.cfg.Logger, "[claude:stderr] "), ...)`.
- `server/pkg/agent/claude.go:1311` sends every nonempty stderr chunk verbatim to `Logger.Debug`.
- Ordinary runtime passes `d.logger` into the backend in `server/internal/daemon/daemon.go`. Its new guards suppress private stream messages and final error details, but they do not cover this shared stderr sink.
- Production collaboration construction sets `HostExecutor.Logger = d.logger` in `server/internal/daemon/collaboration.go`; C4 passes that logger through the same Claude backend. Its separate collaboration lifecycle intentionally has no ordinary `opts.Knowledge`, so an ordinary-only guard would leave C4 affected.

Reproduction logic: configure a Debug-enabled in-memory logger in each existing fake-executable knowledge fixture, have the fake read the approved private stdin sentinel and write it to stderr, then complete/fail normally. The shared `logWriter` necessarily persists that sentinel in generic daemon logs. Existing stdout/stream suppression assertions do not exercise this path. No real Provider is required.

Minimal fix: suppress the raw stderr **log sink** for both explicitly negotiated private-knowledge execution paths while retaining bounded stderr as authorized failure/transcript evidence. Add fake stderr-sentinel assertions for both ordinary and collaboration execution. Preserve useful content-free process diagnostics and the ordinary base-task policy. Owner fixes and source-bound regressions are pending; this finding must not be called resolved from the recommendation alone.

## Contracts checked without an additional finding

- C2 claim responses contain a source-free descriptor. `FinalizeTaskClaim` writes it inside the existing runtime/task transaction before response, including non-comment tasks, without importing the private source lock graph.
- Configuration namespace and actual invocation fingerprint remain distinct. C3 constructs task cwd, settings, model/thinking, custom/default args and MCP options before preparation; the leaf compares the fingerprint against actual invocation options. C2 explicitly rejects ordinary `--no-session-persistence` despite C4's shared leaf allowance.
- C2 exact-session history reads all task statuses, retains immutable old claim/session identities across reclaim and runtime rewrite, includes pending resume bindings, unions historical manifests, rejects malformed/truncated proof and distinguishes claimed/valid-empty history from private exposure. Legacy/unsupported claims also check the wider retained daemon/provider and owner/provider namespaces; unsafe v1 selectors cannot silently fall through to ordinary execution.
- Preparation and each launch authorization recheck current consumer/source/target eligibility. Original-key preparation reconstructs the same manifest, and exact history cannot be replaced by a smaller current source set. The shared resolver's contribution sort matches canonical manifest order.
- Runtime transactions use ordered user/credential and workspace/member fences, project/independent target locks, consumer/source/candidate locks, then late NOWAIT runtime/installation/agent/task locks. C4 prelocks commands, grants and group runs before its existing admission reader; the dedicated path does not import the old blocking runtime-first source helper.
- Trusted consumers come from task originator, actual runtime owner, immutable Chat creator and accepted input authors; Accountable/installer identity is not an authorization fallback. Collaboration audience is re-read under source state locks.
- Start, Pin, Complete and Fail check the original claim fence inside their writer transaction, including zero-native-ID reports and downgraded rows. Public completion JSON strips the fence. Native observations atomically bind exposure and pointer before publication; same-request replay preserves the original outcome. The daemon now decodes acknowledgments through strict `TaskKnowledgeObservationAck` and requires `accepted: true`.
- Ordinary first-launch refusal yields zero processes; negotiated accounting starts from actual callbacks. Later refusal retains usage and failed transcript evidence, blanks successful output and bypasses automatic fresh retry/requeue. The five knowledge failure classifications remain outside the automatic retry allowlist.
- C4 freezes MDT negotiation at claim; later downgrade/upgrade cannot reinterpret the run. Prepare/authorize require MDT, full original run identity, handoff, request and invocation binding. Stored events contain manifest/identity metadata rather than private body. Record/checkpoint/handoff SQL allowlists exclude internal knowledge events.
- C4 keeps `NewLaunchLifecycle`, business-token refresh/filtering, termination authority, original handoff/proof recovery and recording behavior. Every admitted run, even an empty set, has a distinct native namespace, does not save/reuse a native pointer, and includes documented `--no-session-persistence` in actual print argv before hashing. A missing/valued/reenable flag or retained option override refuses without a flag-removing fallback. Retained configuration is re-probed before disclosure and actual launch.
- C4's termination-only status endpoint accepts mct and returns identity, prepared state and original request key only. Gate/source loss does not remove this recovery path. Empty authorized sets create preparation state too.
- Ordinary metadata columns use generated `json:"-"` tags, including derived SQL task rows. Checked SQL JSON surfaces do not serialize task metadata through a raw row projection.

## Retained evidence and source reconciliation

The JSON companion contains source hashes, mismatches and raw Go event inventories. Counts below are top-level passes / pass events including subtests; they are not summed as unique tests across overlapping suites.

| Retained suite | Passes | Skips/failures |
| --- | --- | --- |
| C2 final complete service race | 37 / 60 | 0 / 0 |
| C2 final handler race | 6 / 21 | 0 / 0 |
| C2 protocol/taskfailure | 71 / 562 | 0 / 0 |
| C2 production router | 1 / 1 | 0 / 0 |
| C2 adjacent handler regressions | 98 / 129 | 0 / 0 |
| C2 adjacent service regressions | 29 / 41 | 2 Redis-dependent skips / 0 |
| C3 full daemon | 886 / 1668 | 2 platform skips / 0 |
| C3 focused race | 95 / 298 | 0 / 0 |
| Main strict observation ACK follow-up | 4 / 7 | 0 / 0 |
| C4 final agent/protocol/host | 61 / 218 | 0 / 0 |
| C4 service | 45 / 92 | 0 / 0 |
| C4 handler | 5 / 9 | 0 / 0 |
| C4 production routes | 2 / 2 | 0 / 0 |

At the reviewed snapshot, all 35 C2 owned/generated hashes and all five dependency hashes match the current files. All 24 hashes in C4's allowlist match. C3's original full-suite snapshot has three expected owned-file changes (`knowledge_context.go` and its two tests); all match main's later strict-ACK source-bound snapshot. The protocol dependency matches that same ACK snapshot, while the shared `knowledge_launch.go` dependency matches C4's documented persistence change. These updates are reconciled explicitly rather than treating the older C3 snapshot as current-source evidence.

C2's final `verification.json` is now present. Its six retained log hashes and pass-event counts were independently recomputed and match, with no failed event. Every C2 canonical service, handler, protocol, taskfailure and router test has a matching retained pass event. Earlier fixture/compile failures and the explicitly documented default-DB incident are not passing acceptance evidence.

## Remaining acceptance

RR1 remains an implementation finding requiring a fix and regressions. Separately, main still owns the real server + isolated database + daemon fake-executable composite integration, the final platform/source gate, and any separately authorized real Provider/native/account matrix. C3 used fake authorization transports; C2 used actual handlers/WS-to-batch bridge, not a live WS daemon transport; C4 combined its service/handler and host fixtures as separate suites. Those pass records do not establish a single end-to-end composition or model understanding. The production gate must remain closed until the required combined acceptance is complete.
