# S2 production frontend build preparation

2026-11-14. Production Web and Desktop native/preload/renderer builds pass from an isolated source graph. This is build evidence only; the actual API/Web/Desktop scenarios remain **0/4**. Desktop CLI and final combined API/CLI/Host builds await the completed 2d server source.

## Inputs and scope

- HEAD: `8e824b3035c4f17a9a9dfa0504f1294fbcc9b20b`.
- Shared Core/Views/E2E: the independently checked E2E4 graph, including checked client3d and E3-C01. All 2,552 input files matched their recorded hashes.
- Web/Desktop platform files and supporting build scripts: clean HEAD files, verified equal to the working tree before copying. No peer product changes were included.
- Combined frontend graph: 3,127 files and 1,961 dependency links. Workspace links resolve inside the isolated source; external links resolve to the installed pnpm package store. No dependency was added.
- Private root: `/Users/artisan/.multica/acceptance/pay241-s2-20261114-r01/frontend-build-r01`.
- Build commands use private HOME/TMP, explicit loopback API/Web/Desktop origins, production mode and disabled Next telemetry. They do not inherit account/database configuration.

## Results and retained failures

| Attempt | Result | Interpretation |
| --- | --- | --- |
| Web 1 | exit1; source unchanged | Google Fonts TLS connections failed. Original log retained. |
| Font network probe | direct and system-proxy requests both passed | The same Next fetch helper returned valid CSS on subsequent probes. No persistent cause was established; no proxy or product change was applied. |
| Web 2 | build exit0; audit exit2 | Next generated `next-env.d.ts` changed its route-types import from `.next/dev/types/routes.d.ts` to `.next/types/routes.d.ts`. This exact generated change was separately bound. |
| Web 3 | exit0; all bound files/links unchanged | Successful production build using the explicitly recorded generated declaration. Build ID `SaRnyLTvwkVOxOqh_KuHE`. |
| Desktop renderer 1 | exit0; all bound files/links unchanged | `electron-vite build` compiled native, preload and renderer. The matching CLI bundle is still pending. |

The original binding and runner are retained alongside the production binding. The live repository's `next-env.d.ts` was not changed. Existing CSS-minifier warnings concerning `::highlight` remain in the logs; they are not treated as errors or silently suppressed.

The output manifest hashes 1,586 build artifacts and 23 external traced files. The trace audit found zero live product-source escapes. Two package-directory entries were initially classified as missing by an `is_file` check; the follow-up audit records both existing directories and zero missing paths. This was a verifier classification correction, with the original audit preserved.

## Remaining

- Final 2d source, matching CLI and server/test-harness/base-Host binaries.
- Actual API identity, authentication, controlled Git/artifact/report/execution operations and four E2E scenarios.
- A01–A12 complete writer and transaction evidence, independent phase acceptance and resource restoration.

The allocated ports must be checked again before listeners start. No API, Web listener, Electron window, daemon or external provider was launched by this build preparation. The exact private bindings, raw commands/logs/exit codes and output manifests are referenced by the companion JSON.
