# Independent Linux installer fixture review

Reviewed scope: the explicit Debian mode in `e2e/fixtures/collaboration-installed-main.ts`, its `.mjs` controller, `e2e/collaboration-host-installed.spec.ts`, and the seven Debian acceptance contracts in `.trellis/spec/web/frontend/e2e-run-environment.md`. The requirements are in `acceptance-installation-2026-10-26.md` and `research/installer-next-slice-2026-10-26.md`.

The final reviewed implementation passes scoped static checks and has no unresolved review findings. The main session owns actual Debian-installation and default directory-mode browser acceptance; this review does not substitute for those runs or add cases to the original 58-case inventory. No application source, historical evidence, services, gates, accounts or publication were changed by this reviewer.

## Findings resolved by the implementer

1. **Bind Debian version to accepted application bytes.** The initial code compared installed `dpkg` version only with `.deb` metadata and compared CLI only with Main. An incorrectly labeled package could satisfy those comparisons. The spec now compares `installation.version` with `main.version.replaceAll("-", "~")` and the CLI version, with only its optional leading `v` removed, with the unchanged Main version.

   The mapping is independently verified against installed `app-builder-lib@26.8.1`: `out/targets/FpmTarget.js:134–135` supplies `helper.getSanitizedVersion(target)` to `--version`; `out/targets/LinuxTargetHelper.js:63–70` implements the Debian branch as `version.replace(/-/g, "~")`. The accepted application version is `0.6.5-179-g1bfd169dc`; the raw bundled CLI reports `v0.6.5-179-g1bfd169dc`; the corresponding Debian version is `0.6.5~179~g1bfd169dc`. All three raw values remain in acceptance evidence.

   The first actual Debian full Main run truthfully failed on the CLI's leading `v`, before host fixture creation. The final assertion uses `boot.bundledCLI?.version.replace(/^v/, "")` only for comparison and preserves the raw boot object. This matches `bundle-cli.mjs:123–128`, which retains the git-describe prefix in its Go ldflag, and `package.mjs:140–142` / `app-version.ts:36`, which remove exactly one leading `v` for the application version. No suffix, commit, payload byte or product version was changed to obtain a pass. Boot evidence is assigned before provenance assertions so a future mismatch retains the actual values.

2. **Preserve the separate CLI command in the installed filesystem.** A package manifest alone cannot detect a maintainer-script-created link. The final controller creates a test-owned `/usr/bin/multica` sentinel before installation and requires identical bytes afterward, in addition to checking that the package file list does not claim that command. The installed Desktop launcher resolves independently from `/usr/bin/multica-desktop` to `/opt/Multica/multica-desktop`.

3. **Keep real renderer location assertions precise.** The final code verifies the `file:` protocol and the exact renderer pathname under the selected package root, preserving the original directory mode while admitting `/opt/Multica` only for Debian mode.

## Cleanup finding resolved by the implementer

The pre-existing `finally` in `e2e/collaboration-host-installed.spec.ts` awaited `fixture.daemon.close()` before calling `native.close()`. The daemon close implementation in `e2e/fixtures/collaboration-host-runtime.ts` can reject while reading/parsing its retained ledger or removing its owned directory, which could previously skip container close. The implementer fixed this with nested `try/finally`: daemon cleanup failure still reaches `native.close()`, and failure of that close still reaches the credential check and private evidence attachment. This reviewer verified the final control flow and reran narrow static checks after the fix. There are no unresolved findings in this reviewed slice.

The new **install/init failure** branch is safe with respect to an uninitialized Electron app: it occurs before `seedHostRuntime()`, the controller's `close` command guards `if (app)`, and a caught installation error leaves its input loop able to receive that command. The Docker invocation uses `--rm`; normal controller exit removes the container. The transport's existing close path ends stdin, waits, and stops only its own UUID-named container when required. This is code-path review, not a new injected-failure execution claim.

## Contract review

- Debian mode is explicit, rejects simultaneous directory/volume selection, mounts the selected `.deb` read-only and verifies its SHA-256 before installation. Three expected accepted payload hashes are required and checked against actual installed executable, ASAR and bundled CLI bytes.
- Installation runs real `dpkg --install` with default dependency validation, requires success and `install ok installed`, verifies architecture/version and package ownership, and retains private command output. There is no dependency-force option, network package installation or fallback directory mount in the controller.
- Installed desktop entry, protocol, icon paths, launcher and standalone CLI separation are checked. Main observes its actual installed executable and ASAR, and the original full Main control journey remains intact.
- Directory and named-volume paths retain `/opt/multica` and their original read-only mounts. The Debian layout is `/opt/Multica`; the opt-in does not change default fixtures.
- Docker arguments forward no password/token or host home override. Runtime endpoints and synthetic login credentials use the existing stdin protocol. Real password-form authentication, production native IPC, fresh-container home/userData, Stop, restart while paused, explicit Resume, authors, receipts and capacity assertions are retained.
- The new seven-part specification matches the implementation and keeps package-manager installation separate from directory execution, other architectures/distributions, real Provider accounts, upgrades, signatures and publication.

## Verification

All checks below were executed by this reviewer after the implementer reported the three files stable, then repeated after the final leading-`v` assertion and nested-cleanup corrections:

```sh
node node_modules/.pnpm/eslint@9.39.4_jiti@2.7.0/node_modules/eslint/bin/eslint.js --config .omx/reports/collaboration-progress-reaudit-20261026/eslint.installer.config.mjs e2e/fixtures/collaboration-installed-main.ts e2e/fixtures/collaboration-installed-main.mjs e2e/collaboration-host-installed.spec.ts
pnpm exec tsc --project .omx/reports/collaboration-progress-reaudit-20261026/tsconfig.installer.json --pretty false
node --check e2e/fixtures/collaboration-installed-main.mjs
git diff --check -- e2e/fixtures/collaboration-installed-main.ts e2e/fixtures/collaboration-installed-main.mjs e2e/collaboration-host-installed.spec.ts .trellis/spec/web/frontend/e2e-run-environment.md
```

ESLint, strict TypeScript, JavaScript syntax and whitespace checks passed with exit 0 on the final source. The temporary ESLint configuration reuses repository `base.js`, Node/browser globals for the cross-process controller, and the existing root/Desktop dependency declarations. The temporary TypeScript configuration checks the two TypeScript entry points and their imported source under strict mode, bundler resolution, DOM/JSX and the existing core aliases. TypeScript output contains only the existing pnpm configuration warning, with no compiler diagnostic. Logs/configurations are under `.omx/reports/collaboration-progress-reaudit-20261026/`; `installer-eslint-final2.log` and `installer-tsc-final2.log` retain the final rerun, and the earlier logs remain intact.

The first ESLint invocation used a nonexistent direct `apps/desktop/node_modules/eslint/bin/eslint.js` path. It failed before linting; that command failure is retained in `installer-eslint.log`. Module resolution identified the installed `.pnpm` binary used by the successful final check; `installer-eslint-final.log` is empty.

Reviewed source SHA-256:

| File | SHA-256 |
| --- | --- |
| `e2e/fixtures/collaboration-installed-main.ts` | `bb946e015a4ebbc497324426b10c9a8e531b87af51b3ef862fc6e1555389380e` |
| `e2e/fixtures/collaboration-installed-main.mjs` | `eefd9f7d249890c3fe0b8b81a2c31c35cb1d534ade7e347821222edcef59d055` |
| `e2e/collaboration-host-installed.spec.ts` | `819ebce437699c360a61e0515af6b8a29740f60464aaa684cac7de2979fca606` |

These final hashes include the cleanup fix and leading-`v` comparison correction. Any later source change requires a corresponding narrow recheck and correctly scoped runtime provenance. Full repository checks remain the earlier retained evidence and were not repeated for this fixture-only slice.
