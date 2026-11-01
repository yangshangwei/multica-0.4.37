# Ordinary knowledge composite acceptance

The ordinary server/database/daemon composition passes deterministic acceptance.
Production knowledge enablement remains closed and owned by main. This evidence
uses a test-created shell executable, not an installed Provider or real model.

## Boundary and ownership

The only new Go file is `server/internal/daemon/knowledge_server_integration_test.go`.
It is selected only by the `knowledgeintegration` build tag. No product file,
existing C2/C3/C4 test, dependency, or commit was changed by this lane.

The fixture uses public collaboration Create/UpdateMemory/ProposeKnowledge/
ApproveKnowledge services and `testutil` database rows. A real HTTP listener runs
`handler.New`, `middleware.DaemonAuth`, real claim/skill-resolution/start/private
knowledge/observation/terminal handlers and real service transactions. The daemon
uses its actual Client and `d.runTask`; the test-created executable captures its
actual argv, stream-json stdin and task credential in private temporary files.
The initial configured C3 executable helper is reused, but every composite API
request goes to the new real authenticated handler listener.

The live WS case also runs the actual daemon wakeup connection, server daemon
WebSocket Hub, heartbeat negotiation and DaemonRPCHandler. It rejects a silent
HTTP claim fallback, then executes the claimed task through the same real
HTTP/database/daemon path.

The later-command test uses the existing controlled two-command backend to ask
the real provider backend for a second physical invocation. Its preparation,
authorization, native/process observations and final failure all use the real
server/database. The second command is refused after actual source withdrawal;
only the original command starts, and its usage and failure evidence survive.
This is lifecycle composition evidence, not a claim that an ordinary successful
turn automatically requests two commands.

C4's real router/service/HostExecutor composite is assigned separately to the
original C4 owner in `cmd/server/collaboration_knowledge_host_composite_test.go`.
This lane did not create a service or host composite file.

## Database and invocation safety

- Database: `multica_knowledge_composite_20261101_c825abaf`.
- Private connection file: `/tmp/multica_knowledge_composite_20261101_c825abaf.env`.
- Created from C2's connection settings only; no C2/default database data was
  cloned, mutated, or used for test execution.
- Actual PostgreSQL identity/version and migration ledger are in
  `database-identity.json`. Current migrations were applied to the new database.
- The tagged fixture rejects missing configuration or a database outside its
  recorded owned prefix before connecting. Two bounded subprocess regressions
  verify both guards. `go list` evidence confirms default daemon tests exclude
  the new file; a package-wide DB-requiring TestMain was deliberately avoided.
- Every executable-bearing test run uses the repository's default-agent CLI
  guard. No provider lookup, installed CLI, account discovery or paid call ran.
- Raw captures remain in test-owned temporary directories and are removed by
  test cleanup. Retained logs contain source/prompt/manifest hashes and assertion
  results, not task/daemon credentials or raw private input.

## Final command and evidence

```sh
source /tmp/multica_knowledge_composite_20261101_c825abaf.env
bash scripts/go-test-with-agent-cli-guard.sh \
  go -C server test -race -tags=knowledgeintegration ./internal/daemon \
  -run '^TestKnowledgeServerDaemonComposite' -count=1 -json
```

`ordinary-final.jsonl` is the final raw run. `ordinary-verification.json` contains
all 27 actual passing test/subtest names (14 top-level tests), zero failures and
zero skips, the command, tool version and commit identity. Before/after source
hash inventories bind the tested snapshot; the selected sources stayed stable.
Scoped tagged Go vet, the daemon build, gofmt and whitespace checks passed.
Main owns the full repository suites and final source integration.

## AC6–AC8 coverage

All names below have the `TestKnowledgeServerDaemonComposite` prefix.

| Contract | Concrete evidence |
| --- | --- |
| AC6 ordinary entry points | `HappyPath`, `OrdinarySurfaces` cover issue, comment, private Chat, quick-create and autopilot using actual single/batch claims and daemon stdin. `LiveWebSocketClaim` proves actual WS negotiation/claim with no HTTP fallback. |
| AC6 trusted consumer | `AccountabilityIsNotOriginator` removes the originator while retaining accountability: the real resolver returns an empty bundle and actual stdin stays private-body-free. `SourceParticipationRevoked` begins with an authorized external private-Chat source, then actual Transfer/Leave removes only source Read while target membership survives; the next physical launch is refused. |
| AC7 disclosure/input binding | `HappyPath` has distinct source issues and both project/workspace targets, deliberately reverse-sorted candidate IDs, exact canonical contribution order, real prompt/manifest/invocation digest agreement, all three real process/native/termination observations, and matching persisted native pointer/exposure. It checks the actual task credential, unchanged database base context, metadata without body/summary and stable files without body/summary/private source IDs. |
| AC7 original-key recovery | `LostCommittedResponses` disconnects after real handlers commit preparation, authorization and observation. Retries keep byte-identical requests and receipts; first/replay mixed-target order is identical and only one physical command runs. `LostDisclosureThenWithdrawal` refuses the original-key replay after withdrawal with no new disclosure or launch. |
| AC7 current authority | `RefusesChangedSourceBeforeLaunch` covers withdrawal, actual source memory purge, real elapsed expiry and gate closure after disclosure. The next launch has zero native starts, no synthetic process facts and a non-retried terminal failure. |
| AC8 exact native history | `ExactResumeThenFreshReplacement` executes three real turns: new native, exact resume with the same server scope generation, then a fresh native after withdrawal. The old cumulative exposures remain persisted; the fresh prompt excludes withdrawn content and retains current approved content. `LegacyCannotReuseExposure` proves a nonnegotiating authenticated client cannot resume known managed history. |
| AC8 writer and cleanup fences | `ZeroNativeStaleFence` changes the server claim generation before StartTask; both actual start and zero-native failure writes are rejected, with no disclosure/process start and no replacement-row mutation. `GateClosureKeepsObservationCleanup` retains accepted process/native/termination cleanup and final pointer after the gate closes following authorization. |
| AC8 refusal/replay | The first-refusal matrix and `LaterLaunchRefusal` preserve typed reasons and original process usage/evidence, settle the task as failed, and show no extra physical command or task retry. |

## Red diagnostics and limits

Intermediate logs are retained. Their failures were fixture errors, not product
regressions: missing skill-resolution route, premature manual StartTask before
real preparation, a test model field typo, current-memory purge without first
advancing the memory version, an attempted immutable expiry edit, non-UTC expiry,
and using a workspace-output consumer for a cross-workspace source. They were
corrected using the actual route/state/UTC/private-Chat contracts; no product
assertion was relaxed. Final expiry uses a UTC expiry declared during proposal
and waits until it actually elapses. Final source revocation asserts that the
external contribution was initially disclosed before removing participation.
Fixture cleanup was aligned with the schema retention triggers before the final run.

The database is retained for inspection. Candidate history and pending
invalidation rows are intentionally retained by the schema's DELETE guards;
fixtures remove their ordinary task/source rows but do not bypass those guards.
`database-post-state.json` records the retained fixture counts. Every row belongs
to this newly created test database; no shared/default database cleanup ran.

No real model understanding, installed-client compatibility, non-POSIX platform,
C4 composite, general provider rollout or production enablement is claimed here.
