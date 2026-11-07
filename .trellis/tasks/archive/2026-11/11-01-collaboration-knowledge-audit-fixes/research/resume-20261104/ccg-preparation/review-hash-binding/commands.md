# CCG execution seams after the shared gate

These are source-backed future commands, not executed operations. The bound
independent review covers V15 admission; it does not supply the complete
result, final audit, inventory or release. Every actual CCG permission remains
false. Candidate is `d41573e3a4eb3bd6c36feed3f0d542a0e066ee0f`.

Use the recorded tools from `knowledge-final-gates-v15/source-inputs.json` and
the same candidate checkout. Its existing workspace dependencies are installed.
Keep the shared and CCG output namespaces distinct.

## 1. Complete and release the shared window

Root first verifies make exit 0, final audit pass, all named terminals and each
skip, plus actual resource ownership. Retain the completed run/config/log and
release only its owned API/Web/renderer/Redis window. Keep every old database.
Recheck the source/parser/auditor/review hashes before CCG admission. The actual
auditor requires `run.state=complete` and checks the entire registered binding;
neither its review hash nor a live browser observation supplies acceptance.

## 2. Allocate and run CCG1 normal, then race

For each invocation allocate a fresh migrated DB named with the literal prefix
`multica_knowledge_host_composite_20261101_`, and a distinct Git-free 0700 temp
under `/private/tmp/multica-ccg-v15-`. The current successful migrator is the
candidate's `final-check-v15/migrate`, SHA-256
`060c3a17ba03e9837092884a6fc789c067830629692d4ddec6f309c40f26a379`.
It may be reused against each explicitly selected new DB; never execute it
without that DB's private URL and identity proof.

The existing `v15-preparation/allocate-fresh-db-redis.py` contains the proven
explicit PGHOST/PGPORT/PGUSER/PGPASSWORD/PGDATABASE seam: verify absence through
the recorded server, create through the same endpoint, then verify the new DB.
Its hardcoded V15 seed and exclusive output names mean it cannot be rerun
unchanged for CCG. Use that narrow seam for the new allocation record; no
shared-cluster restart, default DB, old-history deletion or limit change.

After root binds the matching DB identity (`<name>|764_...`), current hashes,
temp/cache paths and shared release/inventory facts into a NEW actual permit:

```sh
python3 <CCG>/run-ccg1.py normal --permit <CCG>/permissions/<normal.json> --label <unique-normal>
python3 <CCG>/run-ccg1.py race --permit <CCG>/permissions/<race.json> --label <unique-race>
```

The unchanged runner selects only
`TestCollaborationKnowledgeHostCompositeCorrectionReplacesActualInputAndWithdrawalRefusesNextLaunch`
with `knowledgeintegration`, `-p=1 -parallel=2 -count=1 -json -timeout=5m`.
The race lane appends `-race`; its runtime must support CGO/race. Each result
requires exactly one run/pass, no fail/skip, and the actual two-input correction
and zero-third-launch fact. Preserve both raw outputs and failures.

## 3. Build the two explicit fake binaries and copy the audited CLI

The retained successful build record
`mc-collaboration-final3-20261102/.omx/private/deployed-probe-e6/build.json`
used default `go test -c ./internal/daemon` and a named-file stdlib provider
build. The current driver has no build tag: `init()` enters it only for the
single `__multica_collaboration_host_router_poller` argument. No test body or
account lookup is needed to compile it.

Run these proposed argv with the pinned Go, a new owned CCG cache/HOME/temp,
local module cache and an offline build profile:

```sh
<GO> -C server test -c -p=1 -mod=readonly -o <CCG>/binaries/host-daemon.test ./internal/daemon
<GO> build -p=1 -mod=readonly -o <CCG>/binaries/host-provider <ROOT>/e2e/fixtures/collaboration-host-provider.go
```

Copy the existing audited `final-check-v15/multica` into the new
`<CCG>/binaries/multica` only after verifying source SHA-256
`7e2b1e39e6f4b83a708cb67919239d3986960953d827e9d0c7e9963892d6701f`.
Keep destination absent before copying and preserve its executable mode. The
fake provider's `--fixture-kind` branch identifies it as
`multica-host-e2e-provider-v1`; the daemon binary is launched by the actual
browser fixture, not through its default test entry.

Fill the existing `binary-binding.json.template`: candidate, successful build
exit, the two exact source hashes, and all three absolute executable paths,
hashes and actual build/copy records. Both fake sources are already selected in
`source-inputs.json`. No installed provider CLI is involved.

## 4. Start the independent OFF deployment

Materialize `deployments/off/source-values.json.template` as a generated private
env, using a new DB, UUID, JWT, matched API/Web slot and a separate CCG renderer.
Preserve password mode, managed=true, limiter=single, empty model configuration
and knowledge=false. Set both `CORS_ALLOWED_ORIGINS` and `ALLOWED_ORIGINS` to the
exact Web and CCG renderer origins; these two required startup values are not
present in the source-values template. Retain the task-owned missing DSH path.
Parse/quote this generated data through the reviewed parser, with no dollar or
control characters. Source credentials stay only in private files/environment.

Use `scripts/dev-env.sh up`, which already owns DB verification/migration,
allocation locking, detached API startup, production Web build/start and
listener/source/configuration provenance:

```sh
bash scripts/dev-env.sh up --components api,web --name <unique-off> --ttl 0 --web-mode production
```

Its launch environment must explicitly select `ENV_FILE` and distinct
`MULTICA_DEV_HOME=<OFF>/registry`, `MULTICA_DEV_PROFILES_HOME=<OFF>/profiles`,
`MULTICA_DEV_WORKSPACES_PARENT=<OFF>/workspaces`,
`MULTICA_DEV_TMPDIR=<new CCG temp>` and task HOME. Give it pinned Node/Go and
owned pnpm command resolution; the pinned pnpm CJS can be exposed by a small
task-owned `pnpm` link. Use the direct pinned pnpm from the CCG runner for the
final Playwright command, bypassing the full-suite reporting wrapper.

`--ttl 0` marks the private registry entry as agent-owned without automatic
expiry. Slots require API `18080+offset`, Web `13000+offset`, and free reserved
renderer `5174+offset` (6000 maps to 6174). The separate CCG renderer can use its
own free port. If the slot is not valid/free, `up` rewrites the private env;
preserve that allocation and rebind its actual DB/ports/env hash before a
permission. Never retain a pre-allocation expected identity after a rewrite.

The current renderer need not be rebuilt. Reuse the ownership pattern in
`v15-preparation/start-renderer.py` with a NEW CCG record/port/HOME and existing
same-source `apps/desktop/out/renderer` bytes:

```sh
<PYTHON> -u -m http.server <CCG-port> --bind 127.0.0.1 --directory <ROOT>/apps/desktop/out/renderer
```

That helper's hardcoded final-check-v15 paths cannot be executed unchanged for
CCG. Retain the actual listener PID/group/cwd, HTTP index hash and installed
artifact identity in the CCG deployment proof.

## 5. Prove OFF identity and run the one deployed probe

Fill `deployments/off/deployment-proof.json.template` with the actual registry
manifest/hash, current DB/764 freshness, env hash, same candidate and
`distinct_from_complete_check_resources=true`. API proof needs actual owned
PID/commit and startup knowledge=false observed from its process, plus password
and managed config. Web proof needs production BUILD_ID and rendered
`apiBaseUrl`/`wsUrl`, not just proxy configuration. The existing
`v15-execution/live-proof/witness.py` demonstrates bounded ps/lsof/explicit-PG
identity inspection, secret-safe environment extraction and rendered HTML
configuration matching; use its narrow seams with CCG inputs, not its fixed
full-run watcher. Capture both actual CORS preflights and renderer bytes.

Root then fills a NEW false-template-derived OFF permit with current runner,
input, env, deployment and binary hashes and the verified fresh resource fields:

```sh
python3 <CCG>/run-ccg2.py off --permit <CCG>/permissions/<off.json> --label <unique-off>
```

The runner chooses only `e2e/collaboration-knowledge-deployed.spec.ts`, one
worker, zero retries, line+JSON reporters and a unique output. It requires one
pass, no errors/skip/flaky, and one actual `deployed-knowledge-gate-evidence`
attachment: real calls=0, section absent and all three knowledge event counts=0.
The fixture also checks original author/input/contribution identities and the
authorized, observed, reaped and proof-acknowledged one-launch ledger.

## 6. Release OFF and repeat independently for ON

Stop the exact OFF API/Web via its private registry:

```sh
bash scripts/dev-env.sh down <unique-off> --components api,web
```

Gracefully release only its separately recorded CCG renderer. Preserve DB,
profiles, registry, build provenance and all evidence. Then perform the same
sequence with distinct ON DB/registry/profiles/temp/ports/UUID/secret, setting
only this fake deployment's knowledge flag=true. Run `run-ccg2.py on` with a new
permit/label. It requires one pass and the corresponding actual section and
claim/preparation/authorization counts of 1, while real calls remain 0.

Serialize OFF and ON: they share the candidate's `.next` output. The existing
Web build lock is per checkout; different private registries do not make live
shared `.next` safe to overwrite. Only the registered Next import rewrite is
allowed in tracked source throughout. Independently reconcile all four CCG
results and current final source before #7 closure. Production knowledge
injection stays default false and no other model budget is transferred.
