# Research: AC43 real-provider provenance acceptance

- Query: Find the smallest trustworthy harness for two actual provider configurations, two managed runtime/installations, three human identities, and AC43's multi-target, squad, mixed-author and transfer scenarios.
- Scope: internal source inspection; no provider, account, database or product calls were made by this research agent.
- Date: 2026-11-05
- Source: root supplied isolated worktree at `/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-ac43-20261105-bi0bmk8x/source`, baseline `d862c5c4b`. Reads used the shared root; implementation must use the isolated candidate and inspect any differences.

## Findings

### Recommended smallest path

Add a dedicated `agentintegration && !windows` acceptance test to `server/cmd/server`, reusing real router fixture construction and the production `Runner` / `HostExecutor` / current `multica` CLI. Keep this separate from the registered RK1 model experiments: RK1 fixes one Claude configuration and one runtime, its own budget/series and its own outcomes; neither changing its launcher nor claiming its prior samples closes AC43.

Suggested files for the implement agent:

1. `server/cmd/server/ac43_real_provider_test.go`: explicit `MULTICA_RUN_REAL_AGENT_SMOKE=1` opt-in, absolute CLI/config/evidence paths, exact task database identity, source/config fingerprints, four serial scenarios, durable per-run/public-safe evidence.
2. `server/cmd/server/ac43_fixture_test.go`: small AC43 fixture composing existing helpers, real enrollment/consent, two provider configurations and two native runners, lifecycle/metadata collection. Split only if one file becomes unwieldy.

No product implementation or migration is required on the evidence inspected. If the real test reveals a product defect, preserve the failing attempt before proposing repair.

The test should create a proper collaboration issue using `CollaborationService.Create` or its real HTTP route. The legacy `newCollaborationRouteFixture` seeds an older source issue directly; its useful managed identity setup is not sufficient by itself for current workflow/transfer acceptance. Create three separate test users: initiator, collaborating handler, resource owner. Both execution resources may belong to the resource owner, while initiator and handler submit independent inputs. Transfer from the handler to the initiator preserves three distinct actors without introducing a fourth user.

### Existing harnesses and limitations

| File / symbol | Reuse | Limitation |
| --- | --- | --- |
| `server/internal/daemon/collaborationrun/host_agentintegration_test.go:32`, `TestHostProviderAccountSmoke` | First real account/adapter preflight, version normalization, current CLI build, managed termination and outcome assertions | Uses a scripted `httptest.Server`; explicitly not evidence of production authorization or AC43's multiple environments |
| `server/cmd/server/collaboration_single_agent_routes_test.go:87`, `newCollaborationRouteFixture` | Isolated migrated schema, real router, synthetic users, real signed installation enrollment/binding, scoped daemon token | Starts with Linux/codex legacy fixture and direct-seeded issue; adjust OS through new enrollment or exact test row repair before host capability/consent, then create current issue |
| `server/cmd/server/collaboration_hold_routes_test.go:34`, `secondCollaborationRouteTarget` | Demonstrates a second independent installation, binding, daemon, runtime and target within one router/schema | Hardcodes Linux/codex and registers/consents immediately; copy the enrollment sequence into the AC43 helper so host capabilities are registered before target consent |
| `server/cmd/server/rk1_fixture_test.go:68`, `newRK1Fixture` | Real `NewRouterWithOptions`, `CollaborationService.Create`, trigger cloning, host capability registration, claim request construction | One Claude runtime/provider and two humans; do not call its unchanged `register` for a two-runtime roster because it grants every resource with one binding |
| `server/cmd/server/rk1_run_test.go:147`, `rk1CloneTriggers` | Copies production triggers absent from `CREATE TABLE LIKE` | Required to preserve run identity, immutability and transition checks; keep same migrated schema semantics |
| `server/cmd/server/rk1_run_test.go:81`, `(*rk1Fixture).execute` | Production `Runner`, durable private ledger, HostExecutor, current CLI, awaited terminal state | Uses a Claude plan wrapper and per-run fresh work directory; AC43 can use actual CLIs directly and one stable per-environment work root |
| `server/cmd/server/rk1k_fixture_test.go:229`, `nextRunForIssue` | Runtime HTTP `List` promotes pending work, intersect eligible IDs with exact issue/session and unsettled rows | Do not substitute a direct DB run lookup: it does not promote; for AC43 also select the exact runtime |
| `server/cmd/server/rk1_trace_test.go:58`, `rk1Trace.ServeHTTP` | Records actual server runtime request/response pairs, scrubbed credentials, waits for handler completion on snapshot | Does not capture human workflow paths; capture those receipts separately; private raw claim includes credentials and must not be serialized |
| `e2e/fixtures/collaboration-host-runtime.ts:175`, `enrollHostRuntime` | Public signed challenge/enrollment/binding example | Hardcodes Claude `fixture-1` in registration |
| `e2e/fixtures/collaboration-host-runtime.ts:429`, `adoptHostTarget` | Public offer → adopt → per-resource owner consent, exact runtime/installation checks | Expects one resource owner; fine for AC43's two environments if that owner owns both |
| `e2e/fixtures/collaboration-host-squad.ts:22`, `seedHostSquad` | Public squad roster setup and three targets | Both agents use the same runtime; cannot be presented as two-environment evidence |
| `e2e/collaboration-host-squad.spec.ts:15` | Root → real CLI member delegation → member result → leader continuation ordering | Controlled fake provider callback; current `startHostDaemon` cannot simply be pointed at real Codex/Claude |

The existing native e2e poller is deliberately fixture-specific: `server/internal/daemon/collaboration_host_router_test.go:130` hardcodes provider `claude`, version `fixture-1` and account source `test-owned-empty-account`; `startHostDaemon` also copies a fake binary, probes `--fixture-kind`, strips the environment and depends on a callback socket. Adding real-provider support there would broaden the change more than a tagged router integration test.

### Configuration and execution boundary

- `server/internal/daemon/collaborationrun/host_config.go:23`: `HostConfig` includes provider, version, account-source selector, executable and default model. `Capabilities()` at line 107 hashes managed config/executable facts; it is not account/billing evidence.
- `server/internal/daemon/collaborationrun/host_config.go:146`: `HostExecutor.Probe` rejects prefilled model/session/CWD/tool/lifecycle options. Set the model on the frozen agent invocation or `HostConfig.DefaultModel`, and let the production runner populate CWD/session/lifecycle.
- `server/internal/daemon/collaborationrun/host_runner.go:167`: production `ResolveBackend` receives real provider name, actual executable, observed version and environment. Its managed lifecycle owns launch/terminal receipts.
- `server/internal/daemon/collaborationrun/host_preparation.go:298`: runtime business identity and temporary paths are injected; Codex additionally gets a task-scoped `CODEX_HOME`.
- `server/internal/daemon/execenv/collaboration.go:13`: `PrepareCollaborationCodexHome` follows the existing shared provider configuration path. The source home selection is process-global, so run providers serially and set configuration selectors before preparation. Do not change global home/config on disk, pretend a fake process is Codex, or invent billing IDs.
- Two directories or two runtime IDs under one macOS user provide distinct configured execution contexts, **not OS-level security isolation or proof of two independent billing accounts**. Report this boundary explicitly.

The root reported after initial inspection that **both** current CLIs use custom gateways. The earlier assumption of first-party Claude OAuth is superseded. A gateway URL, nonidentical secret fingerprint or different models does not establish independently billed upstream accounts. Final account attribution requires gateway/account usage evidence collected by the coordinator.

### Four scenarios and minimum observations

1. **Multiple targets, environments A and B**: distinct targets use their assigned `runtime_id`, `installation_id`, binding and provider. Execute through real HTTP list/claim/permit/outcome/proof for each. A/B must not accidentally share an environment configuration; compare prepared config fingerprints and observed CLI/backend before launch.
2. **Squad leader A, member B, continuation A**: freeze a two-runtime roster, consent each resource with its own binding, submit a real leader command that delegates through `multica issue comment`. Assert member `parent_run_id`, roster resource/grant, B runtime/installation, and continuation back on A. Preserve provider tool outputs plus router writes; a fixture-injected delegation is a weaker administrative test and must not be called real model delegation.
3. **Two human submitters on A**: queue one input each from initiator and handler before resume, or run two sequential commands. Assert all persisted prompt inputs retain the correct `author_user_id`; owner/agent metadata and A config/grant/account fingerprint remain stable.
4. **Ordinary transfer**: fetch current authority/revision, call real `/api/collaboration/issues/{id}/handoffs` with `purpose=transfer`, target human and new idempotency key, confirm actual handler changed, then let new handler submit to existing A. Assert original environment, resource grant and provider/account source remain unchanged; input author is the new handler.

Transfer references: `server/internal/service/collaboration_workflow.go:112` is the product mutation; request types are lines 26–39. `e2e/fixtures/collaboration-runtime-handoff.ts:23` shows the public HTTP body with current epochs/revision. Do not directly edit the handler DB field. Use current `write_context` after transfer rather than assuming participant epochs remain 1.

Mixed-runtime squad reference: `server/internal/service/collaboration_squad_test.go:24`, `newManagedSquadResourcesFixture(separateMemberNamespace=true)`, demonstrates correct per-member runtime/binding consent. It is lower-level evidence, not a substitute for real provider execution.

### Evidence already available from production

- `GET /api/collaboration/issues/{issue}/runs/{run}/records?execution_view_version=6&limit=100`, implemented at `server/internal/service/collaboration_records.go:144`.
- `CollaborationRunResource` (`collaboration_records.go:26`) exposes actual agent, agent owner, runtime, resource owner, execution workspace, grant ID, provider/version/model and `account_source`.
- `CollaborationRunRecordItem` (`collaboration_records.go:86`) exposes each immutable input's command ID, actual human author, body and attachments. Paginate through `next_cursor` rather than losing later inputs.
- `CollaborationRunRecordsPage` (`collaboration_records.go:105`) exposes exact nullable usage, recording completeness, preparation, run and records.
- Read the actual `db.CollaborationRun` (generated `models.go:1044`) and scrubbed claim to collect installation/binding, grant/resource epochs, run/group/parent identity, proof state, attempts and termination evidence that public records intentionally omit.
- `freezeResourceRecording` (`collaboration_records.go:48`) explicitly freezes **configuration provenance, not billing**. `account_source=runtime_configuration` must never be presented as the provider's account ID.
- Record issue creator separately from input author, executing agent/agent owner, resource owner and current handler. Hash only nonsecret account identity; credential fingerprints are config identifiers, not identity proof.
- Persist started/completed/failed attempts with UTC bounds, source/config/binary hashes and exact real provider request/response-derived usage. Preserve failure attempts rather than rerunning silently.

### Database allocation and commands

The coordinator must allocate one fresh DB exclusively for `server/cmd/server`, for example `multica_ac43_20261105_<suffix>`, migrate it fully using the isolated source, record its actual name/cluster and keep `DATABASE_URL` private. Do not reuse application, browser or another agent's databases. `cmd/server/integration_test.go:44` initializes real rows even when `-run` selects one tagged test; without `DATABASE_URL` it exits 2, and a connection failure can exit 0 with a skip message, so check actual test inventory as well as exit code.

Use the exact coordinator-allocated environment and check `SELECT current_database()` before any migration/test. Query aggregate PostgreSQL connection count before allocating an API or more test processes; do not raise shared limits or terminate others' sessions. A full standalone API/Web process is unnecessary for this router integration lane.

Existing preflight (safe only with the user's now-explicit authorization, one provider at a time):

```sh
# Run from server/; absolute executable comes from coordinator inspection.
MULTICA_RUN_REAL_AGENT_SMOKE=1 \
MULTICA_HOST_SMOKE_PROVIDER=codex \
MULTICA_HOST_SMOKE_EXECUTABLE="$AC43_CODEX_EXECUTABLE" \
go test -tags=agentintegration ./internal/daemon/collaborationrun \
  -run '^TestHostProviderAccountSmoke$' -count=1 -v
```

Repeat with `claude` and its absolute executable. A smoke pass is adapter evidence only.

Proposed future test entry (flags become valid only after implementation):

```sh
# Private coordinator environment already selects exact fresh migrated DB,
# two explicit provider configurations, built current multica CLI and output.
go test -tags=agentintegration ./cmd/server \
  -run '^TestAC43RealProviderProvenance$' -count=1 -timeout=30m -v
```

Compile first using `go test -tags=agentintegration -c -o "$AC43_TEST_BINARY" ./cmd/server`; this does not run package `TestMain`. Add fake-provider contract checks for wrong runtime selection, changed input author, transfer config drift and missing account/usage evidence before consuming real calls. These are harness correctness tests, not a broad new test architecture.

Refresh **only these two positive fixtures'** runtime heartbeats before each delayed real claim; `service.RuntimeClaimFreshnessSeconds` is 150 seconds. Real providers can take long enough that the second runtime becomes stale. Never alter the product timeout/freshness fence or use a global heartbeat that changes negative-test semantics.

## Related specs

- `.trellis/spec/server/collaboration-host-execution.md`: distinct input/owner identities, configuration consent, stable sessions and host limits.
- `.trellis/spec/server/collaboration-records.md`: public provenance vs private identity and nullable usage.
- `.trellis/spec/server/database-test-isolation.md`: task DB, package `TestMain`, migration identity and cluster budget.
- `.trellis/spec/server/collaboration-context-experiments.md`: real calls gated, finite named runs, durable failed attempts and observed vs configured model.
- `.trellis/tasks/10-30-collaboration-context-handoff/release-gates-runbook-2026-11-01.md:106`: G5 exact four-scenario acceptance and account usage reconciliation.

## External references

No external API implementation guidance was needed for this source-mapping task. The coordinator owns installed CLI versions, gateway documentation/usage APIs and account evidence. No credentials or account exports were inspected by this agent.

## Caveats / Not Found

- No existing dedicated AC43 real multi-account harness was found. Existing smoke, RK1 and e2e results cannot individually close it.
- No code-level ledger proves who the custom gateways bill. Missing gateway account/usage evidence must keep that portion explicitly incomplete even if all Multica routing and author/grant tests pass.
- Separate provider contexts on the same physical machine do not prove separate machines or users; choose and report the supported acceptance interpretation instead of quietly broadening claims.
- Actual OS-native Windows/installer acceptance remains outside AC43 and in the cross-platform task.
- Research did not run tests or providers. All proposed real-provider outcome assertions remain unverified until the implementation and coordinator run them.
