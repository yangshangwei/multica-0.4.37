# Research: Next frozen candidate and complete-check readiness

- Query: Fastest safe continuation after v11 exit2, preserving failed evidence; production Web/native renderer, dependencies, source provenance, Make and password7 setup.
- Scope: Internal filesystem/source/process inspection only. No Git mutation, service lifecycle, database command, install, test, browser, Provider or account operation.
- Date: 2026-11-03; underlying inspected records retain their original dates.

## Findings

### Actual v11 result and ownership

The retained candidate is `/Volumes/artisan/code/2026/mc-collaboration-final-owned-20261103`, pinned to `304f74dd3ba4ddea862c94917c5bb0770355a351`. Its `.omx/private/final-check-v11/make-check-v11.json` states `complete`, exit2, finished `2026-11-03T03:01:58.440776+00:00`; log SHA256 is `02d1f307edca1c06e701dd636116bcf1b85569a88317f5889826f3e45b8d16cb`. Existing run/clearance/claim files must remain untouched.

Read-only process observations during this research:

- Launcher14220 and Make14257 are absent.
- Recorded API launcher46864/listener46890 and Web launcher47283/listener47302 are absent; their registry JSON remains historical, not proof of live listeners. Recorded origins were API `http://localhost:19038`, production Web `http://localhost:13958`, build `q7FTQSwrhbDb2yPdr9cYU`.
- Renderer PID75467 is **still live** and listens on127.0.0.1:19846, serving the old candidate's `apps/desktop/out/renderer` through Python `http.server`. Preserve it unless root proves handoff and deliberately retires that exact owner.
- Docker's host listener on127.0.0.1:16422 is live. Historical proof assigns Redis container `9f84d2240700fd46f6241fc332eedfead5e8ebbef5a00fccae03ab33bc9ff58a` to the previous session. No current Redis client/database probe was performed.
- `.trellis/.runtime/sessions/codex_01a19ee4-40e8-7b70-b15c-5b5a13bd050e.json` still points at this task, `current_run=null`, last_seen `2026-11-03T00:58:47Z`. Several CLI Codex processes exist. Neither that stale pointer nor absent Make PIDs proves that another live session released19846/16422.
- Historical `v11-execution-handoff.json` transfers ownership to session `01a19ee4-40e8-7b70-b15c-5b5a13bd050e`, not this root session. No direct cross-session communication was sent.

Root can avoid waiting for historical-resource ownership by allocating **a fresh renderer port and fresh task Redis container/port**, then preserving19846/16422. Recheck the host's aggregate PostgreSQL budget before starting the full run; the last proof's100max/33idle belongs to the old window and cannot be reused as current evidence.

### Candidate scope choice

The planned final-HEAD claim favors a new frozen copy of the current shared checkout's committed HEAD (`084ea3c98` at the parent snapshot) plus only reviewed task-owned overlays. Do not mutate the retained v11 worktree to make it appear current.

A read-only byte comparison of the v11 tracked inventory against the current working checkout found102 changed/missing product/e2e paths, including the newly committed collaboration detail/result/manual-policy implementation and current picker/stage-label overlays. Newly added files outside the old inventory were not counted. Therefore v11 plus two E2E patches is a valid **historical closure scope** check only; it does not prove the current final-HEAD behavior. The current source manifest must include new tracked result/policy/migration files. Preserve unrelated dirty task/spec/evidence files in the shared checkout, and freeze reviewed source bytes in a separate checkout rather than keeping a long-running acceptance target in that changing shared directory.

### Reuse existing tooling with narrow input rebinding

There is no ready generic v12 launcher. The v11 launcher hard-pins root/SHA, v11 filenames, output paths, Redis16422 and the old renderer binding. Running it unmodified refuses new input and refuses overwriting existing output. Reuse the reviewed implementation by creating a **new numbered copy**, changing only candidate/run paths, SHA, fresh resource addresses and corresponding data bindings. Preserve parser, API/test environment separation, per-package DB adapter admission, default-closed flags, source/workspace/tool/artifact guards and original Make invocation. Independently review this narrow input rebinding, then run the existing fake boundary suite against that new copy. Avoid inventing a new launcher design.

Reusable paths:

| Path | Purpose |
| --- | --- |
| old candidate `.omx/private/final-check-v11/run-make-check-v11-provenance.py:68` | Full source/workspace/tool/CLI/desktop binding guard; root/SHA/output constants at15–17, main at147 |
| old candidate `.omx/private/final-check-v11/check-bin/go` | Reviewed per-package clone adapter, SHA256 `70ca24bb88b2a8df93ae4694e9fd9739c523e8f35755a10bb799b7561b934fce`; no hard-pinned candidate root |
| old candidate `.omx/private/final-check-v11/check-bin/pnpm` | Reviewed real Playwright API/renderer/CLI-guard wiring, SHA256 `acb022a829423fd10cde20c30b00ceb15c9ce4c38a1dd3327e8e0338bbce7ca2`; output filename still saysv9 but its directory is run-local |
| old candidate `.omx/private/final-check-v11/owned-bindings-v11-provenance.json` | Binding schema/template; **recompute values**, do not relabel old hashes |
| old candidate `.omx/private/final-check-v11/resolve-workspaces-v11.mjs` | Declared internal edges/actual exports graph collector |
| old candidate `.omx/private/final-check-v11/checkout-source-id-v11.cjs` | Dev HEAD/overlay identity; not canonical product SourceID |
| `.trellis/tasks/10-30-collaboration-context-handoff-runtime/research/real-model-rk1/source_manifest.py:16` | Canonical product ROOTS and sorted path NUL fileSHA LF digest collector |
| this task `research/continue-20261103/final-state-audit/final_state_audit.py:32` | Read-only final filesystem audit; currently hard-pinned to oldv11 paths/review, so make a separately reviewed numbered input rebind |
| knowledge-audit task `research/continuation-20261103/owned-candidate-preparation/build-current.py` | Retained exact offline CLI/desktop build process; create a new input-bound copy rather than overwriting old outputs |
| knowledge-audit task `research/continuation-20261103/owned-workspace-links/{seed-store.py,install-offline-v2.py,restore-electron-cache.py,probe-resolution.mjs,verify-integrity.py}` | Existing offline owned dependency installation and resolution/integrity workflow |

The source identities are distinct: canonical product manifest, development HEAD/overlay identity and full tracked file/mode fingerprint. Recompute all three from the new frozen candidate. Hash E2E source separately because canonical product ROOTS intentionally do not includee2e.

### Dependencies and artifact preparation

The retained Node is `/Users/artisan/.nvm/versions/node/v22.23.2/bin/node`. The copied pnpm10.28.2 package is old candidate `.omx/private/owned-workspace-links/pnpm-10.28.2/`. Cached Go1.26.6 is `/Users/artisan/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.6.darwin-arm64/bin/go`. Keep actual package-manager10.28.2 rather than resolving an ambient newer pnpm; the current parent root invocation already emitted new-pnpm ignored-settings warnings.

For a new checkout, use a candidate-owned package-manager/store copy with the retained install arguments:

```text
<pinned Node> <new candidate copied pnpm>/bin/pnpm.cjs install --offline --frozen-lockfile --ignore-scripts --package-import-method=copy --side-effects-cache=false --store-dir=<new private store> --cache-dir=<new private cache> --global-dir=<new private global> --reporter=append-only
```

Existing APFS `clonefile` cache-copy tooling in `seed-store.py` avoids hardlinks and preserves old stores. Do not point the new install at mutable old donor node_modules or the old store's project directory. A relink against one shared virtual store can resolve views/core/auth to a different physical source even when bytes match. Verify all declared workspace links and actual exports resolve inside the new candidate. Prior successful inventory had22 declared edges/66 exports/one physical auth source.

For faster same-lock preparation, root can instead APFS-clone the old candidate's **actual node_modules directories** with `cp -cR` into absent new-candidate directories (root plus every workspace importer), retaining relative symlinks and independent file inodes. Sampled existing links are relocatable: Web core/views use `../../../../packages/{core,views}`, and views core uses `../../../core`. Copy the candidate-owned pnpm SDK as well, then run `resolve-workspaces-v11.mjs` against the new candidate and require every exported realpath and auth physical identity to be owned there. Do not replace copied directories with symlinks or copy build outputs as new-source builds. This path avoids an install only if lock/package/config bytes match; inspect package graph and external dependency resolution after cloning.

No reusable binding-generation **factory** was found in persisted preparation code: `source-snapshot.py` collects tracked bytes/modes; `source_manifest.py` collects canonical product bytes; `probe-resolution.mjs`/`resolve-workspaces-v11.mjs` collect Node graph; `build-current.py` records CLI/desktop builds. `owned-bindings-v11-provenance.json` is the existing data schema/template whose values must be recomputed. Preserve the two adapter files byte-identically; no adapter factory is required.

Lifecycle scripts are intentionally disabled during offline install. Restore Electron39.8.7's cached dist/path.txt with the retained exact-file cache workflow, then build the new CLI and desktop renderer/preload/Main from new source. These are builds only; no installer/release scripts are needed for this task. `build-current.py` uses a byte-identical `.cts` copy of `apps/desktop/electron.vite.config.ts` under ignored app `node_modules/.cache/` to avoid an unignored timestamp ESM file. Update new build input paths/SHA/log names and keep the build policy unchanged. New artifacts must not be certified using old CLI/renderer hashes.

### Actual full command

The repository command stays:

```text
make check ENV_FILE=<new canonical0600 private environment>
```

Run it through the newly input-bound reviewed launcher. `scripts/check.sh:132` calls static lint/typecheck, TS tests, script regressions plus isolated race Go/vet, isolated API, production Web and original Playwright inventory; `:173` preserves one worker/zero retries. The reviewed adapters remove API-only pool/publisher settings in real Go test branches and allocate fresh migrated DB clones per package. Keep this behavior and its named inventory.

The launcher supplies a fresh private0700 HOME/registry/profiles/workspaces, explicit real tool/cache paths, a Git-external0700 TMPDIR, current-source CLI, missing absolute DSH path, no real Provider environment and default-closed knowledge/managed/admin gates. Never replace the full run by a selected-test replay or combine old successful phases into acceptance.

After completion, run the separately bound final auditor to a unique new report. Require Makeexit0 + auditstatuspass + complete original named inventories/provenance. The current v11 auditor can separately produce an identity-only report for the old failed run without authorizing success, but it writes only below its own auditor directory and cannot audit the new candidate unmodified.

### Production Web and native renderer

`scripts/dev-env.sh:786` owns Web identity/build/start. Production uses `pnpm --filter @multica/web build` followed by `pnpm --filter @multica/web exec next start --port <allocated>`. API and WS origins are build inputs. `scripts/dev-env.sh:803–811` requires source/config identity stable across build and a shared checkout build lock; two registries/ports do not isolate one `.next` directory. Use only the new candidate for the new lane.

The native renderer can use the established static launch shape with a **new free port** and new artifact path:

```text
/usr/bin/python3 -m http.server <new renderer port> --bind 127.0.0.1 --directory <new candidate>/apps/desktop/out/renderer
```

Record actual PID/cwd, HTTP index hash and full artifact manifest. Set both `COLLABORATION_ELECTRON_RENDERER_URL` and `CHANGELOG_ELECTRON_RENDERER_URL` to that origin; include it in API `CORS_ALLOWED_ORIGINS` and `ALLOWED_ORIGINS` before startup. The normal full-check source config's `check_allowed_origins` does this for the allocated Web plus declared renderer (`scripts/check.sh:15`).

### Password7 after full Make/audit

The existing exact inventory/setup is already documented in this task `research/continue-20261103/password-readiness/readiness.{md,json}`. Preserve it. Old `.omx/private/password-directed-20261103/preparation.json` is `prepared_not_started`, owns desired18249/13169/19846 and old SHA; those desired resources are not a live allocation for the new candidate.

Create a fresh source env/private registry/name/temp/database for the **same new frozen candidate**, after default full check/audit. Required password configuration is `MULTICA_AUTH_MODE=password`, `MULTICA_DEVICE_AUTH_ENABLED=false`, empty `MULTICA_CLOUD_URL`, `MULTICA_PASSWORD_LIMITER_MODE=single` for one API, signup/workspace creation enabled, `E2E_PASSWORD_AUTH=1`, and default-closed real execution/knowledge gates. Exact live API/WS/Web/renderer/database values must precede build and fixture import.

Use the retained production startup surface with new inputs:

```text
make up ENV_FILE=<new password source.env> C=api,web ARGS='--name <new task name> --web-mode production'
```

Run the original five-file selector under pinned Node/pnpm, one Chromium worker, zero retries, new JSON/artifact output:

```text
<pinned Node> <new copied pnpm>/bin/pnpm.cjs exec playwright test e2e/collaboration-handoff-chain-web.spec.ts e2e/collaboration-handoff-chain-desktop.spec.ts e2e/collaboration-offline-handoff.spec.ts e2e/collaboration-runtime-handoff-web.spec.ts e2e/collaboration-runtime-handoff-desktop.spec.ts --project=chromium --workers=1 --retries=0 --reporter=json --output=<new private artifacts>
```

Set `PLAYWRIGHT_JSON_OUTPUT_FILE=<new JSON path>`, `PLAYWRIGHT_BROWSERS_PATH` to the verified Chromium1208 cache (private HOME otherwise hides it), current native source root and renderer origin. Require exactly7passed/0skip/0retry plus original identities and attachments. These native tests use the source native fixture with built preload/renderer and do not prove packaged installer/Main or real Provider execution. No Docker image is required by this seven-case inventory.

## Related specs and external references

- `.trellis/spec/server/verification-environment.md`: reviewed bytes, data-only env parser, Make safety, API/test separation, exact package admission, before/after identities.
- `.trellis/spec/server/database-test-isolation.md`: owned DB/registry and aggregate connection budget.
- `.trellis/spec/web/frontend/e2e-run-environment.md`: production source/config identity, exact origins, native and password gate distinctions.
- No external documentation was necessary; retained installed tools and original repository scripts define this continuation.

## Caveats / Not Found

No new candidate, service, Redis/DB, dependency install, build, test/list command, browser/native process, Provider call or acceptance clearance was created. Current shared HEAD/overlays can change while other work proceeds; root must freeze once fixes settle and record the actual new SHA. Old resource ownership remains assigned to the previous session; absence of the old test process is insufficient to take it over. The reusable private tools have hard-pinned inputs; root must produce and independently review a small new input rebind rather than execute their historical versions against new source.
