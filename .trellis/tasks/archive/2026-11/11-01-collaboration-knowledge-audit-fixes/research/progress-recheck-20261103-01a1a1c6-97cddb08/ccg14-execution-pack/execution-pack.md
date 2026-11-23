# Research: CCG v14 concrete execution pack

- Query: Recover the exact supported commands, allocation order, input fields and independent root observations for CCG1 normal/race and CCG2 OFF/ON after the successful sole v14 gate.
- Scope: internal; current candidate `d0b132fff0c477eb2d744933e13db730240231aa` in `/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103`.
- Date: 2026-11-03

## Findings

This is a reusable execution pack, **not permission to execute**. The current four templates remain false. `source-inputs.json` SHA-256 is `e688795a758ae51478175bd1bf5a8cd99e1164545d522bab93235dddffe45589`; the final preparation review SHA-256 is `e4c4c5863908961ab9faf9efb49ac7e8ed7d5fab7dd32bf9564c90e79367d917`. Root must first inspect the actual v14 exit0, registered final audit, complete named/individual-skip inventory, live identity proof and release of the sole complete-check window. Record the current coordinator/one CCG execution claim while preserving the earlier owner/claim. Knowledge quota/calls remain0/0, real Provider entry remains closed, and default production injection remains false.

`required-fields.json` is the machine-readable companion: it binds all13 current preparation files, tools, parser/auditor/review, build/source pointers, exact argv arrays and each required field. No test, compilation, DB/network/service/container or git operation was performed in this research.

### Files and existing seams

| File | Purpose / relevant line |
| --- | --- |
| `.omx/private/knowledge-final-gates-v14/run-ccg1.py` | admission starts at29; one exact tagged test and physical-input fact at97 onward |
| `.omx/private/knowledge-final-gates-v14/run-ccg2.py` | same admission; current deployed proof/config/bytes at109 onward; actual attachment result at198 onward |
| `permissions/{ccg1-normal,ccg1-race,ccg2-off,ccg2-on}.json.template` | all remain false; copy to new uniquely labelled real permits only after evidence exists |
| `deployments/{off,on}/{source-values,deployment-proof}.json.template` | explicit new configuration, process/rendered identity and resource separation; never modify a live source env |
| `binary-binding.json.template` | current compiled fake daemon/provider plus exact audited CLI and build provenance |
| `Makefile:185` | `make up C=api,web ARGS=…`; `down` uses ARGS, not C |
| `scripts/dev-env.sh:30` | registry selector is `MULTICA_DEV_HOME`; profiles/workspaces/temp have separate supported selectors |
| `scripts/dev-env.sh:1380` | per-registry lock; checkout reuse/adoption and port/DB fallback before startup |
| `scripts/dev-env.sh:1451` | registry DB_NAME/DATABASE_URL/ports become authoritative for the children |
| `server/cmd/server/router.go:115`, `server/internal/realtime/hub.go:55` | exact HTTP/WS origins; add both Web and CCG renderer to new deployment data |
| `apps/web/app/layout.tsx:138`, `apps/web/config/runtime-urls.ts:98` | actual API/WS URLs passed to WebProviders during SSR |
| `e2e/collaboration-knowledge-deployed.spec.ts:15` | one180s case; exact true/false expectation and full actual-input/ledger assertions |
| `e2e/fixtures/collaboration-host-runtime.ts:82` | explicit absolute fake binaries, password/deployment/database requirements |

### 1. Root release and new CCG allocation

The order is: **inspect complete result/audit/inventory/live identity → record release/current sole CCG owner → validate new names and absent DBs/free matched ports → atomically create the named DB → materialize/hash exact private configuration → supported registry allocation/start → verify actual manifest/health/schema/process/rendering → independent materialized-input review → sign the new permit → exact one test.**

Create one new mode0700 Git-free `/private/tmp/multica-ccg-v14-<nonce>`; bind ownership, actual path and absence of an enclosing Git repository. Use the fixed CCG cache paths in all four templates (`PRIVATE/go-build-cache`, `PRIVATE/go-path`, `/Users/artisan/go/pkg/mod`), not complete-check writable caches. Pin Node/Go/psql/pnpm from `source-inputs.json`; use Python3.14 because retained helpers use `hashlib.file_digest`. Main PATH may lack psql; use its registered absolute binary.

The two CCG1 databases must have different names with literal fixture prefix:

- `multica_knowledge_host_composite_20261101_v14n_<nonce>`;
- `multica_knowledge_host_composite_20261101_v14r_<nonce>`.

CCG2 OFF/ON use two additional fresh task databases, e.g. `multica_ccg14_off_<nonce>` and `multica_ccg14_on_<nonce>`. All names are lowercase `[a-z0-9_]+`, at most63bytes, distinct from defaults, historical failed DBs and every other lane. Do not query/create through `docker exec`: connect to the same explicitly selected PostgreSQL host/port/user/SSL settings that each eventual DATABASE_URL uses.

Use an owned root claim and psql's quoted variable syntax over **stdin**, not interpolated SQL or `-c` variable expansion. Set connection parameters in a private subprocess environment, admin database `postgres` only for absence/create; never print password-bearing URLs.

```python
# Future root action; db is a newly generated/validated name, env is explicit.
args = [pinned_psql, '-X', '-v', 'ON_ERROR_STOP=1', '-v', 'ccg_db=' + db]
# Retain empty output (database absent), actual exit and timestamp.
subprocess.run(args, input="SELECT datname FROM pg_database WHERE datname=:'ccg_db';\n",
               env=admin_env, text=True, capture_output=True, check=True)
# No IF EXISTS/IF NOT EXISTS: a racing duplicate creation must fail.
subprocess.run(args, input='CREATE DATABASE :"ccg_db";\n',
               env=admin_env, text=True, capture_output=True, check=True)
```

Require absent→successful CREATE→explicit migration, not a hand-written freshness boolean. PostgreSQL does not provide a database creation timestamp. Record source settings through nonsecret fingerprints and actual connection identity.

For CCG1, run the current migration against **only** its new explicit URL. Reuse a current successfully built `migrate` only if its exact d0b source/build record/hash is verified; otherwise use the already-pinned tool/cache to run:

```text
<pinned Go> -C <candidate>/server run ./cmd/migrate up
```

Supply explicit DATABASE_URL, CCG temp/cache/HOME and minimal tool environment; preserve raw migration log/exit. The exact readonly query used by the runners is:

```sql
SELECT current_database(),max(version) FROM schema_migrations;
```

Require `<exact database>|764_collaboration_ai_policy` before permitting a case. For OFF/ON the supported `make up` performs the migration; inspect the actual DB only after it has completed migration/health, not at the empty pre-API stage.

### 2. Supported allocator controls and database identity

Use `MULTICA_DEV_HOME=<PRIVATE>/deployments/<lane>/registry` as the registry selector. `MULTICA_DEV_REGISTRY` is not that selector. Use separate **initially empty** OFF and ON registries, distinct `MULTICA_DEV_PROFILES_HOME`, `MULTICA_DEV_WORKSPACES_PARENT` and private process HOME; direct `MULTICA_DEV_TMPDIR` to an owned CCG path so builds do not escape. Avoid a global shell HOME reassignment: pass HOME only in the explicit child environment.

In a fresh registry the allocator preserves POSTGRES_DB/DATABASE_URL only when provided PORT/FRONTEND_PORT are a genuinely free matched slot:

- integer offset0–999;
- API `18080+offset`, Web `13000+offset`;
- reserve the allocator's renderer port `5174+offset` too (`6000` becomes6174).

The separate CCG static renderer can use another free owned high port; it must not equal fullcheck55132. Check all relevant listeners before allocation. The supported `make up` allocation lock is inside MULTICA_DEV_HOME; it atomically registers/adopts its slot. It does **not** reserve an unbound socket against unrelated processes in another registry. Final listener/manifest checks remain necessary; do not claim an advisory precheck is an atomic global port reservation.

Set `PORT`, `FRONTEND_PORT`, `POSTGRES_DB` and the URL's pathname to one proposed allocation in the new source-values JSON/env. If the slot is free and matched, `save_manifest` records `DB_NAME=POSTGRES_DB`, `DATABASE_URL` and those ports. If it falls back, it rewrites ports **and DB name/URL** to `multica_<checkout-basename-slug>_<offset>`. Do not guess that fallback name, pre-approve it, or change a permit to follow it. An existing checkout match in one registry is reused even when `--name` differs; hence two independent registries are mandatory.

Use argv, not a shell-generated command:

```python
up = ['make', 'up', 'C=api,web', 'ENV_FILE=' + str(source_env),
      'ARGS=--web-mode production --name ' + validated_lane_name + ' --ephemeral']
# cwd=candidate; minimal environment includes the explicit supported DEV selectors.
```

After actual startup, inspect `registry/envs/<actual NAME>/manifest.env` as data and compare DIR, ENV_FILE, DB_NAME, DATABASE_URL, BACKEND_PORT, FRONTEND_PORT, WEB_MODE, PROFILE and WORKSPACES_ROOT to the root proposal. Hash that actual manifest and canonical source env. Use actual health/config and listener provenance, not a retained registry alone.

If a port/database is rewritten or resource ownership differs: preserve source-values/env before and after, proposed DB and **actual** registered DB, manifest/log/exit and exact mismatch. Do not sign a permit or run Playwright. Stop only the new verified owned API/Web through its actual registry/name, retain both databases and failure evidence, and use a new DB/slot/env/JWT/deployment/label for a new separately recorded attempt. Never force-kill a conflicting listener or drop another DB to obtain the proposed slot.

### 3. Canonical private configuration and exact review

Copy the lane source-values template to a **new** actual private JSON; leave the template unchanged/false. Fill a distinct random JWT_SECRET and deployment UUID for each lane, explicit local DB credentials, matched ports and exact origins. Keep password mode, device false, managed installations true, platform admin true, signup true, E2E_PASSWORD_AUTH1 and `MULTICA_PASSWORD_LIMITER_MODE=single`. One API process supports explicitly empty REDIS_URL (in-memory hub/single limiter); if Redis is selected, allocate CCG-owned resources and never use fullcheck55131. Clear both Multica LLM keys and avoid inheriting ambient model/account/task variables into the deployment.

**Add CORS_ALLOWED_ORIGINS and ALLOWED_ORIGINS to the new materialized data** as exactly `<Web origin>,<CCG renderer origin>`. The current templates list only FRONTEND_ORIGIN; otherwise HTTP falls back to Web-only and the runner's actual renderer OPTIONS probe fails. This addition is only in the new isolated deployment, never current fullcheck or product defaults.

API origin is the same for COLLABORATION_E2E_API_URL and NEXT_PUBLIC_API_URL; WS is the exact loopback `ws://<API host>:<API port>/ws`. Web origin is FRONTEND_ORIGIN/PLAYWRIGHT_BASE_URL. Renderer origin is a newly owned credential-free loopback port outside the two forbidden55132 origins. ON sets true only for its new isolated deployment; OFF/default production stays false.

Reuse the registered `parse_generated_environment` from the pinned v14 launcher. Form `key=shlex.quote(value)+newline`, verify the parser decodes exactly the proposed dictionary, then write the canonical file exclusively0600. Reject any unresolved `ROOT_` placeholder, dollar sign, control or newline in values. Make include/export can interpret dollars even when shell-quoted; do not just source the old dotenv or concatenate shell text.

Hash and independently review the actual materialized JSON/env, source-input binding, deployment proof, binary-binding/build records and root ownership/permission record before creating a new true permit. Template or runner/source-input changes require a new exact-byte review and refreshed hashes. New actual CORS data/ports/DB identities also require explicit materialized-input review, even though old false templates and runner bytes stay unchanged. Root booleans are attestations of observed facts, not facts created by setting booleans.

### 4. CCG1 normal then race

Copy a false template to a new mode0600 `permissions/ccg1-<mode>-<label>.json`, never overwrite a template or earlier permit/result. Supply every common field listed in required-fields.json: exact runner/candidate/source-input hash, new DB URL/name and verified764 identity, owned0700 temp/fixed caches, recorded current coordinator/root permission and explicit Provider false. Set the release/inventory/freshness/ready flags only after matching actual evidence and independent exact review.

Run normal then race using two different fresh DBs and new labels:

```text
/opt/homebrew/opt/python@3.14/bin/python3.14 <PRIVATE>/run-ccg1.py normal --permit <new-normal-permit> --label <new-normal-label>
/opt/homebrew/opt/python@3.14/bin/python3.14 <PRIVATE>/run-ccg1.py race --permit <new-race-permit> --label <new-race-label>
```

The runner preserves the original CLI guard and exact `./cmd/server -tags=knowledgeintegration -run ^TestCollaborationKnowledgeHostCompositeCorrectionReplacesActualInputAndWithdrawalRefusesNextLaunch$ -p=1 -parallel=2 -count=1 -json -timeout=5m`; race appends-race. Require one run/pass,0fail/skip, exit0/no race, and the exact physical fact: two starts, corrected version2 alone in actual stdin, superseded stdin absent, third authorization refused, zero third process and original exposure/receipts retained. Preserve actual started/result/tests.jsonl and hashes. Normal/race are the same behavior under two modes, not two independent additional identities.

### 5. Current CCG2 fake binaries and isolated renderer

Only after the preceding gates, build the two current source-controlled fakes with the pinned Go and CCG temp/cache environment:

```text
<pinned Go> -C <candidate>/server test -c -o <PRIVATE>/binaries/host-daemon.test ./internal/daemon
<pinned Go> build -o <PRIVATE>/binaries/host-provider <candidate>/e2e/fixtures/collaboration-host-provider.go
```

`test -c` compiles and does not run TestMain. Copy exact audited `<candidate>/.omx/private/final-check-v14/multica` to CCG binaries, preserve executable mode, compare source/destination hash `ad532d8d34ea3ee8e6707e88b49207959dec7452733b83b6d631f59e83b13c2f`, and link `candidate-v14/candidate-setup/cli-build-v14.json` (actual successful d0b build/version/source). Do not select an installed user Agent CLI. Each binary needs its own command/source/tool/output/raw-log/exit record. The runner checks aggregate exit0/selected hashes, not each build_record's underlying evidence; root must inspect it.

Serve the current unchanged `apps/desktop/out/renderer` at a new CCG-owned loopback listener, e.g. with pinned Python `-m http.server <new-port> --bind127.0.0.1 --directory <current-renderer>`. Record actual PID/PGID/command/cwd/port and index bytes/hash equal to the audited artifact. One explicitly CCG-owned renderer may serve the two serial lanes; neither uses the fullcheck origin or old renderer ownership. This is a bytes/CORS prerequisite, not an installed/native Electron acceptance claim.

### 6. OFF then ON live proof, permit and one case

Start OFF through the supported argv and its new independent registry/configuration. Before signing its permit, independently retain:

- actual `/health` PID, commit d0b/full or9chars, started_at and listener/launcher PGID/cwd/provenance;
- actual process startup knowledge literal false, password/device/managed/single-limiter/deployment flags from the owned process/configuration hash; `/api/config` confirms auth/managed/signup/deployment, not knowledge by inference;
- exact actual DB/user/latest764 identity and absent/create/migration history, actual manifest/source-env/profile/workspace separation;
- actual production Web build exit/BUILD_ID, owned next-start listener and rendered SSR WebProviders API/WS values; an env value or provenance boolean alone does not prove rendering;
- actual successful OPTIONS/CORS from both Web and renderer, current served renderer index bytes and binary/build bindings.

When examining an owned process environment, capture in memory and emit/hash only selected nonsecret flags; never dump all ps/environment output or private credentials. `/health` and `/api/config` use explicit selected loopback origins. Their actual responses, relevant headers, selected provenance and timestamp form the root proof.

Fill a **new** per-lane deployment-proof JSON with those observations and root separation/freshness paths. Its source_environment_sha256 must be the final actual canonical env hash. Copy the OFF false permit to a new permit, add all six source-env/proof/binary paths+hashes and common fields, and perform the exact materialized-input review before flags become true.

```text
/opt/homebrew/opt/python@3.14/bin/python3.14 <PRIVATE>/run-ccg2.py off --permit <new-off-permit> --label <new-off-label>
```

This invokes only `e2e/collaboration-knowledge-deployed.spec.ts`, workers1/retries0, original180s budget, line+JSON reporting and an explicit actual false expectation. Require one expected pass with unexpected/flaky/skipped/retry/errors all0. Independently decode the sole complete `deployed-knowledge-gate-evidence` attachment; inspect exact account/deployment/input-command/batch/run/candidate/version/content/binary/source identities, not just counts. OFF requires one ordinary fake child, no knowledge section/contributions/no-session-persistence/events. Both lanes require settled success, launch_count1 and one original ledger with authorized/observed/reaped/terminal accepted, sealed namespace absence and proof_acknowledged/host_exit_observed.

Stop only verified OFF API/Web through the same MULTICA_DEV_HOME and actual manifest NAME:

```text
make down ENV_FILE=<OFF env> ARGS='--components api,web <actual OFF name>'
```

Keep the database, registry and raw evidence. Do not use `make stop`, destroy, forced port kills, or cluster cleanup. ON is then independently materialized/started/verified/reviewed on its **other** registry/DB/JWT/deployment/slot with knowledge true only there:

```text
/opt/homebrew/opt/python@3.14/bin/python3.14 <PRIVATE>/run-ccg2.py on --permit <new-on-permit> --label <new-on-label>
```

ON additionally requires the exact approved contribution in actual stdin, no-session-persistence=true and exactly1 each knowledge_claim/preparation/authorization. Stop only owned ON services, retain both lane DBs/proofs. Root/independent reviewer compare final source/tool/binary and complete raw attachment hashes; no self-written success or counter totals replace physical proof.

### Root observations versus runner checks

The runners actually validate current candidate/tool/selected source hashes, registered source auditor/review and its fresh wholecheck_passed result, explicit local DB/latest764, fixed caches/0700 Git-free temp/raw symlink rules, actual selected source-env/deployment/binary values and CCG2 health/config/CORS/renderer. They validate their documented result counts and selected input/event facts.

They do not independently reconstruct a complete named inventory/resource release, database creation history, process environment ownership, all registry/profile/workspace/source-rendering facts, each build record, every account/input/run/ledger attachment field, or the contents of root_permission_record/freshness-proof paths. Root must inspect/save those underlying observations, then an independent reviewer confirms them. The old `ccg-admission-template.py.txt` is historical reference; do not regenerate current runners from it.

### Historical patterns and related specs

- `10-30-collaboration-context-handoff/research/knowledge-crosschain-correction-20261102/{summary.json,db-permission.md}` retains the two fresh-DB/actual-input/third-refusal pattern.
- `10-30-collaboration-context-handoff/research/knowledge-deployed-probe-20261102/{root-deployment-manifest.json,root-deployed-results.json,root-ready.md}` retains actual independent OFF/ON source/configuration/build/ledger observations.
- `.trellis/spec/server/verification-environment.md`: data-safe environment parsing, process-specific settings, exact source/clearance/final identities.
- `.trellis/spec/server/database-test-isolation.md`: explicit owned new DB before any TestMain, no shared/default cleanup, compile-only semantics and named terminals.
- `.trellis/spec/web/frontend/e2e-run-environment.md`: production Web, exact password/single limiter startup, configured renderer HTTP/WS origins, source/working-tree provenance.
- External references were unnecessary: current repo source, recorded tool versions and accepted exact-byte review are the authority. No network research was performed.

## Caveats / Not Found

All actual allocation/build/live/test permissions are still pending the prior successful shared gate; this pack opens none. Old whole-chain CCG evidence cannot be rebound after current AI policy/router/queue/lock changes; it is a setup/assertion reference only. No supported single variable makes arbitrary independently selected ports immune to allocator rewrite; use a matched slot, separate empty registry and exact actual post-start checks. Port prechecks are not a global reservation. Missing/failed/rewritten allocation or test is retained under its original identity; subsequent attempts use new resources/labels after exact diagnosis/review, with no auto-retry or weakened assertion.
