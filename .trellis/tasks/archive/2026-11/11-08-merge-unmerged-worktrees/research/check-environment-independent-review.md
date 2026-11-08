# Independent review: complete-check Desktop/Collaboration environment

Reviewed at: 2026-11-08T12:32:51.304672+08:00

**Verdict: REQUEST CHANGES for the two concrete findings below.** The current focused run can finish on its frozen inputs because main confirmed its source env does not contain DESKTOP_RENDERER_PORT. Do not edit candidate files until main releases that freeze. No code edits, DB operations, browser sessions, builds or overlapping tests were performed by this reviewer.

## Findings (not fixed)

### P2 — copied source renderer port can override the allocated port

`prepare_check_environment` assigns `DESKTOP_RENDERER_PORT` from OFFSET before `bind_paths` and `save_manifest`, and uses that value in CORS and `COLLABORATION_ELECTRON_RENDERER_URL`. But it does not append the allocated `DESKTOP_RENDERER_PORT` to the copied check.env. Its final `load_env_file "$ENV_FILE"` therefore restores a stale source-file value, if present, while the registry and renderer URL still name the allocated value. Startup then binds or refuses the wrong port. This defeats the new explicit comment that inherited development renderer ports must be ignored.

Append `write_manifest_value DESKTOP_RENDERER_PORT "$DESKTOP_RENDERER_PORT"` to the task-only env assignments. Test an actual stale DESKTOP_RENDERER_PORT line in source.env, rather than only assigning the shell variable after loading source.env; require that the final loaded port equals `renderer_port_for_offset`, URL/origins/manifest agree, and source.env remains unchanged. Main and implementer were notified. This issue is absent from the currently active focused source environment, so it does not invalidate its original-failure reproduction coverage.

### P3 — prior spec section still describes a Web-only default

The existing “Optional native renderer origins in make check” contract and cases state that a run without optional renderer variables stays Web-only. The new implementation always builds Desktop and serves an allocated renderer, and the appended section correctly describes that behavior. Update the original section to describe allocated Web plus allocated renderer origins and any separately supplied optional extra origins. Also distinguish the source file's prior collaboration renderer URL from the task copy's actual newly allocated endpoint. Keep invalid-origin rejection and exact CORS/WS membership intact.

## Confirmed behavior and boundaries

- Existing registry allocation checks API, Web and renderer ports under the lock. The added renderer uses the same offset and unique check state directory; it refuses busy ports and uses strictPort rather than silently binding a different listener.
- The helper calls Vite preview with configFile:false and a prebuilt output directory. It serves actual production renderer files; it does not compile dev UI or execute production Electron Main. Vite is resolved through the already installed, directly declared electron-vite owner; no package manifest/lockfile/dependency was added.
- Desktop build precedes renderer startup and Playwright. The current checkout's preload/index.js and renderer/index.html must exist. Readiness compares served index bytes with the actual build file and proves listener ownership before recording its PID. Helper provenance records actual canonical renderer path, PID, origin and index hash.
- Cleanup flags cover renderer startup failures and later Web/browser failures. Renderer shutdown uses the existing per-component process-group/listener machinery. An unrelated busy listener is not adopted or killed; cleanup failure preserves a failed complete-check result. New shell regressions cover build/start failure order and cleanup; helper tests exercise actual HTTP/asset bytes, missing output, busy-port refusal and process exit.
- The task copy binds API alias, native WS URL, source root and output directory to this checkout/allocated API. Exact loopback renderer origin joins both CORS and WS allowlists before API startup. Source env is copied rather than mutated. The port finding above is the one missing persisted override.
- Native collaboration fixture consumes the allocated renderer URL and built preload while stubbing daemon start/restart to throw. Project native fixtures reuse the same newly built output through their own loopback proxy. No production Main/daemon is launched by the new helper.
- Supplying API aliases intentionally activates previously unconfigured human Collaboration suites and makes unconditional Knowledge cases executable. Password/admin/host/installed/provider cases retain separate explicit gates and binary/image inputs. No new test skips or weakened assertions were introduced. Real paid Agent opt-in remains outside this change.
- All candidate product/test sources must stay frozen while the complete or focused run is active. Built outputs are artifacts, not a source-code exception. Preserve known next-env generated import reconciliation separately as already reviewed.

## Verification

Independently ran `bash -n` on both shell scripts, `node --check` on both MJS files, and `git diff --check`: all pass. Did not run implementation-owned script/HTTP tests or main-owned DB/browser/build commands. Successful static review does not certify the active browser run.

## Exact reviewed scope

```json
{
  "scripts/check.sh": "e2fd949498656459d300b398b640bdd5ee2b4132714071cdf4fb5e38aa993dae",
  "scripts/check.test.sh": "499a9cb78a31849867f6e57a98dab85cf176976bd99e0f71a8835857439e8652",
  "scripts/check-desktop-renderer.mjs": "40ec1989a12c7bcb39ae460c1f8ffeefa7ec9e157ba9e0f903b43a05cb8528a7",
  "scripts/check-desktop-renderer.test.mjs": "66305c665a0b2f1a4712b38025a3b3cce9291d78c2f89b3c0fb7fbd46beb9de2",
  ".trellis/spec/web/frontend/e2e-run-environment.md": "5f0c2121f1466f486b20aaf0284aab4dd785824c35d7f6edb08dc8816956367e"
}
```

After the focused run ends, resolve the two findings, rerun the narrow script/HTTP tests, then re-review those exact changes before committing and starting a fresh complete gate. Preserve the failed full-check-02 and its copied artifacts.

## Follow-up: findings resolved and approved

Reviewed at: 2026-11-08T12:37:49.859107+08:00

**Updated verdict: APPROVE for the exact five-file environment patch below.** The initial findings remain above as historical evidence; both are resolved, and no other concrete blocking issue remains in this patch.

The allocated DESKTOP_RENDERER_PORT is now appended to the task copy before reload. Regression setup contains source-file19996 and separately inherited19997; it requires the allocated value, matching renderer URL/CORS/manifest and unchanged source-file19996. `check-env-port-before-02.log` records the exact pre-fix failure; `check-env-port-after-final.log` records script PASS plus all3 real-HTTP helper tests passing with zero fail/skip. Implementer recorded command exit0 in `check-env-final-hashes.json`. Reviewer read these logs and independently matched every final file hash to that manifest.

The original native-origin spec section now consistently states that the default check owns allocated Web and built Desktop origins, and that optional extra origins are validated. It records source-file port rebinding and keeps exact CORS/WS checks. No skip, test assertion or provider opt-in was weakened.

Reviewer independently reran bash/node syntax checks and git diff --check after these changes; all passed. No DB/browser/build/test command was duplicated. Main reports the27 previously failing E2E cases passed in retained e2e-repair-01; new e2e-activated-01 is separately testing the23 previously skipped paths identified below. Complete-gate approval still requires the new full run.

```json
{
  "scripts/check.sh": "7747c4c0ae432bc26d1b3b3b1f42eecdfd989778ea969bd54c95c21a8e2d76b4",
  "scripts/check.test.sh": "36f2555ed8ee3a3fbef2a913ebc228ab0ceac68f1f7f7becfaef5b5bbc76b0f1",
  "scripts/check-desktop-renderer.mjs": "40ec1989a12c7bcb39ae460c1f8ffeefa7ec9e157ba9e0f903b43a05cb8528a7",
  "scripts/check-desktop-renderer.test.mjs": "66305c665a0b2f1a4712b38025a3b3cce9291d78c2f89b3c0fb7fbd46beb9de2",
  ".trellis/spec/web/frontend/e2e-run-environment.md": "29dbde1f68f31961f39cd960050799000bb6deb1b97fe4ce7fa250a334240e68"
}
```

## Newly activated full02 skip inventory

Read the full02 actual skipped-case lines and each current readiness predicate. With only the newly supplied API/renderer/source-root bindings and the existing configured DATABASE_URL/NEXT_PUBLIC_API_URL,23 previous skips become executable across9 specs:

| Spec | Newly executable cases | Cases still separately gated |
| --- | ---: | --- |
| collaboration-account-desktop.spec.ts |9|2 require MULTICA_DEPLOYMENT_ID (lines277,590)|
| collaboration-boundaries-web.spec.ts |2 (lines164,451)|1 requires MULTICA_DEPLOYMENT_ID (line294)|
| collaboration-delivery-web.spec.ts |2|0|
| collaboration-editing-web.spec.ts |2|0|
| collaboration-machine-boundary.spec.ts |1|0; existing legacy lane matches|
| collaboration-memory-web.spec.ts |2|0|
| collaboration-records-web.spec.ts |1|0|
| collaboration-workbench-web.spec.ts |2|0|
| global-scope-navigation.spec.ts |2 (Web,Desktop)|0|

Precise recommendation sent to main: run these9 complete specs on the owned production Web/native environment with one worker and zero retries. Expected selected inventory is26 cases:23 execute and3 retain their existing deployment-specific skips. Do not introduce new skips if any execute and fail.

The source env has no MULTICA_DEPLOYMENT_ID, password/admin opt-ins, host/provider/installed executables/images or detail-provenance input. Mention/resource/preparation/preview/command-history cases continue requiring the missing deployment identity; control/host/installed/password cases keep their independent gates. The recipient-picker spec already executed using its existing NEXT_PUBLIC_API_URL fallback. Knowledge's8 cases were previous failures, not new skips, and are covered by the retained27-case repair run. No real paid Agent invocation is needed for this newly active human-browser inventory.
