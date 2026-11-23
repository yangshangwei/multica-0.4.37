# CCG v14 offline preparation

Candidate: `d0b132fff0c477eb2d744933e13db730240231aa` in `/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103`. Coordinator: `01a1a186-9a5c-7b00-96f8-0430b47edb4c`.

The thirteen prepared files are under the candidate's `.omx/private/knowledge-final-gates-v14`. They preserve the actual CCG1 normal/race and CCG2 OFF/ON commands and assertions. The v13 files remain unchanged. Four permission templates remain `ready_to_execute=false`; no CCG database, temporary execution directory, registry, binary, account, deployment or service has been allocated.

The latest preparation inventory is `preparation-v2.json`. `offline-refusal-v2/offline-verification.json` records four actual unready-template refusals and eighteen isolated admission counterexamples, including raw permit symlink refusals. Those checks denied networking, did not run any Go/Playwright/database/service command, and created no candidate result directory. True flags existed only in the local synthetic fixtures; actual permission templates stayed false.

## Changes and retained limitations

- Updated the candidate SHA, private directory, CCG temporary prefix, full-run path, final auditor/review paths and environment parser path. The worktree's literal `v12` directory component is unchanged.
- Rehashed eleven selected source files, four tools, the current final auditor and environment parser. The actual v14 independent review did not yet exist when prepared; its intended path is set and its hash is null. This explicitly prevents a successful later admission until the actual review is pinned.
- Replaced the forbidden full-check renderer origins with port 55132. Future CCG renderer resources must be separate.
- Added `MULTICA_PASSWORD_LIMITER_MODE=single` to both task-only source-value templates because current `server/internal/auth/password_config.go` rejects an unset mode and `dev-env.sh` does not fill it. Each OFF/ON deployment has one API process and its own registry. No running or production configuration changed.
- The first rebinding missed the ROOT-relative `PRIVATE` literal even though absolute paths were updated. Its unready invocation refused with `Permit outside owned directory`, before any external operation. `offline-initial-failure.json` and `initial-rebind-before-private-literal-fix/` retain that failure and source. V2 corrected only that relative literal and the affected unready permit hashes; all refusal checks then passed.
- `ccg-admission-template.py.txt` is preserved reference material. Its older resolution ordering differs from the actual executable runners. Do not regenerate a runner from that reference: review and execute `run-ccg1.py` / `run-ccg2.py`, which retain the original raw symlink checks.

## Mandatory gate before actual CCG allocation

Root must verify one actual v14 `make check` exit 0, the actual final audit pass, complete named and explicit-skip inventory, and the live source/API/database/process proof. Root must release the complete-check execution window and retain its databases, logs and original failures. Prepared files or a filesystem-only audit do not establish a successful complete check.

After the v14 independent review is final, pin its exact hash in `source-inputs.json`. Recheck all source/tool/auditor/parser hashes and refresh the four unready templates' `source_inputs_sha256` values. Any changed runner/template/input requires a new exact-byte CCG review. Record that final review before enabling a real permit. A null review hash is deliberately not accepted by the runners.

Root-owned registry manifests, database freshness, process startup flags, rendered Web API/WS configuration and binary build records remain required evidence. The runners check only their documented selected fields and HTTP facts; they do not independently reconstruct every root observation.

## Four fresh databases and two independent registries

Allocate CCG execution resources only after the preceding gate and separate dispatch:

1. Create a new Git-free mode0700 `/private/tmp/multica-ccg-v14-<nonce>` and explicit CCG-owned Go caches. Preserve the full-check caches and state.
2. Allocate and migrate two different CCG1 databases named `multica_knowledge_host_composite_20261101_v14n_<nonce>` and `multica_knowledge_host_composite_20261101_v14r_<nonce>`. The literal prefix is required by the existing Host fixture. Verify the database using the same explicit connection settings supplied to migration, and retain `database|764_collaboration_ai_policy` before each permit. Do not reuse a failed database or invoke migration with an absent URL.
3. Allocate separate fresh OFF and ON database/deployment identities. Use separate private registries under `deployments/off/registry` and `deployments/on/registry`, with separate profiles/workspaces and exact private `ENV_FILE` paths. `dev-env.sh` can reuse a checkout match within one registry despite a different `--name`; port fallback can also change the database name. Compare the actual manifest and database after startup, and refuse a reused or silently rewritten allocation.
4. Use a new CCG renderer and any explicitly required CCG Redis. Never reuse the full-check API, Web, renderer 55132 or Redis 55131. No existing full-check or business database may be dropped to obtain a fresh allocation.

## CCG1 normal then race

Create a new real permit from each unready template only after the root observations and exact review exist. Bind the candidate, runner/source-input hashes, full-check resource/inventory release, fresh DB/identity, CCG temp/cache paths, root permission record and `real_provider_allowed=false`. Use a new permit/result label for every actual attempt.

Invoke pinned Python 3.14 with `run-ccg1.py normal` or `race`, `--permit <new real permit> --label <new label>`. The retained Go command selects exactly:

```
./cmd/server -tags=knowledgeintegration
-run ^TestCollaborationKnowledgeHostCompositeCorrectionReplacesActualInputAndWithdrawalRefusesNextLaunch$
-p=1 -parallel=2 -count=1 -json -timeout=5m
```

Race additionally uses `-race`; both run through the repository Agent CLI guard. Acceptance requires one named run/pass, zero fail/skip and the unchanged physical-input fact: two starts, corrected version 2 in actual input, superseded input absent, third authorization refused, zero third start, original exposure and receipts retained. Keep normal and race as separate observations.

## Current fake binaries and CCG2 OFF then ON

After full-check release, compile the current daemon test binary with `go -C server test -c -o <CCG binaries>/host-daemon.test ./internal/daemon`. Build the current `e2e/fixtures/collaboration-host-provider.go` to `<CCG binaries>/host-provider`. Copy the audited v14 CLI from `.omx/private/final-check-v14/multica` into the CCG binary directory and bind its original successful build record/hash; do not select an installed user Agent CLI. Bind current source, tool, command, actual exit and output hash for each binary. Compilation alone is not deployed acceptance.

Materialize source-value templates through the reviewed data parser/canonical serializer, supplying private credentials, new matched ports and explicit origins. Keep password auth, managed-installation flags and `single` limiter mode. ON changes only its separately owned test deployment. Production remains default false.

Start OFF using explicit registry/configuration and `make up C=api,web` with production Web. Record actual API PID/commit/startup knowledge=false, password/managed configuration, production Web build and rendered API/WS values, new registry/database, CCG renderer bytes and fake binary/CLI identities. Bind and issue the OFF permit, then run `run-ccg2.py off` with a new label. Stop only owned OFF services and retain its database/evidence before separately preparing and running ON with knowledge=true.

Each lane retains the single `e2e/collaboration-knowledge-deployed.spec.ts`, one worker, zero retries and original timeouts/assertions. Require one pass and zero fail/skip/retry. Independently inspect the complete `deployed-knowledge-gate-evidence` attachment: exact account/deployment/run/contribution identity, no knowledge input/events OFF, exact approved input and one knowledge claim/preparation/authorization ON, one actual fake start and authorized/observed/reaped/sealed proof in both lanes.

## Closure boundary

After all four actual observations, independently verify raw results, attachment contents, source/binary identities and hashes. Task #7 can then close against AC1–AC9 and the 21 dispositions without waiting for #6 real models or release acceptance. This preparation does not advance the real-model ledger: knowledge allocation/calls remain 0/0 and injection remains default off.
