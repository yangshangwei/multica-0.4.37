# C4 real server/DB/host composite evidence

The composite is implemented in
`server/cmd/server/collaboration_knowledge_host_composite_test.go`, behind the
`knowledgeintegration` tag. No production file was changed in this increment.

The fixture uses the real `NewRouterWithOptions`, Handler, managed bearer
authentication, signed installation enrollment/binding, service transactions,
SQL queries and migrated table definitions. Its knowledge gate is enabled only
on the test Handler instance. Runtime capability registration goes through the
real HTTP route before target registration and explicit owner consent. Source
creation, proposal and approval use the real A2 services. The reused schema
fixture's indexes/checks are accompanied by the actual migrated noninternal
trigger definitions, including capability epochs and immutable knowledge.

`Runner` and `HostExecutor` call the real server over HTTP. The observing
transport forwards every request and actual response; loss tests discard a
real successful response rather than fabricate service JSON. Only the Claude
executable is synthetic. It is an absolute, test-owned copy of the test binary,
and records actual stdin/argv while emitting controlled output/stderr.

## Passing behavior

Eight top-level tests, with sixteen total tests/subtests, pass both normally and
under `-race`. Both runs have zero skips and zero failures. Tagged Go vet also
passes. `verification.json` lists every actual test name, commands, versions,
database, source hashes and result files.

- Real MDT claim URL negotiation, frozen handoff, canonical project/workspace
  contributions, exact application-prompt authorization, actual physical
  process start/termination and accepted proof complete to a successful run.
  Both original input authors and their accepted text remain in stdin.
- Successive admitted runs use different actual directories/namespaces. Every
  knowledge invocation carries `--no-session-persistence` and no resume or
  continue selector. A later run contains its newly accepted input, without
  reclaiming the earlier run's executed input.
- Withdrawal, historical source purge, expiry and source-participant removal
  after preparation refuse the later run at the real knowledge authorization
  endpoint. Its proof settles as `never_launched_proven`, while the earlier
  successful run and its physical proof remain unchanged. Reopening the same
  stopped run does not claim, prepare or physically launch another input.
- Lost preparation and authorization replies replay the same request and return
  byte-equal actual responses, with one retained preparation/authorization fact
  and one physical launch. Withdrawal after a lost preparation reply makes its
  original-key replay return a content-free refusal instead of stored body.
- A frozen unnegotiated claim cannot prepare private knowledge or upgrade on
  Runner reclaim. An admitted knowledge claim refuses a downgraded claim and
  remains executable under its original negotiated identity.
- Termination-only metadata status remains readable after source/gate loss,
  without candidate IDs or private body. Raw fake stderr echoes approved stdin;
  it never appears in the supplied generic debug logger or stable CLAUDE.md.
  A failing fake retains the private diagnostic only in bounded authorized run
  outcome evidence while preserving physical termination and proof.

## Child and database safety

All execution used only
`multica_knowledge_host_composite_20261101_efb9efe6`. The private environment is
`/tmp/multica_knowledge_host_composite_20261101_efb9efe6.env`; its connection
settings were derived from the C4-owned template, never another lane's database
contents. Each execution command checked `current_database()` before testing.
Compile-only checks used `go test -c` and did not execute TestMain.

The fake dispatch uses a `COLLAB_` marker before TestMain. A test-owned executable
basename also exits before TestMain when that marker is absent/stripped. The
negative guard test clears DATABASE_URL as an independent safety backstop and
requires the basename guard's distinct exit code with zero output. Unrelated
existing fixture children are not intercepted.

An earlier failed fake used a reserved `MULTICA_` marker, which the host correctly
removed. That copied test binary could enter ordinary TestMain against this
same explicit host-composite database; the record is `happy-sixth.jsonl`. No
default/shared-database investigation or repair was attempted. The marker and
basename corrections are test-only and were verified before final acceptance.

## Failure history and source stability

All initial failures are retained. They were fixture issues: a public project
foreign key targeting a shadowed workspace, the dbfx issue counter, installation
`macos` versus Go's `darwin` label, omitted migrated triggers, the stripped child
marker, and attempting to purge the current memory version. The purge matrix now
uses the supported replacement-then-historical-purge flow. No product assertion
was weakened and no production fix was made.

`final-source-before.json` and `final-source-after.json` cover server Go/SQL
sources. During final checks only the ordinary-lane test files
`task_knowledge_config_test.go` and `task_knowledge_logging_test.go` changed; this new
composite and its production dependencies remained unchanged. The differences
are recorded explicitly in `verification.json`.

## Scope limits

This composite exercises built-in local Claude direct-agent runs with multiple
authors. Squad/member delegation and same-run repeated physical-attempt behavior
retain their existing service/host tests; they are not relabeled as new composite
coverage. The fake verifies application argv/input delivery and actual lifecycle
bookkeeping. It does not prove real-Claude acceptance, model understanding,
private Provider history behavior or the broader installed/OS/account matrix.
Main retains G2's combined review and rollout decision. No Provider/account
calls, commits, subagents or production gate changes were made.
