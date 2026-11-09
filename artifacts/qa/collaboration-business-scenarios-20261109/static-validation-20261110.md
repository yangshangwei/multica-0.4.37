# Business scenario QA: focused static validation — 2026-11-10

Executor: native `trellis-implement` child `qa_harness_fix`.
Working directory: `/Volumes/artisan/code/2026/multica-0.4.37`.

This records commands actually executed after the final S1 fixture changes. It is scoped verification of the two new business-scenario test files, not a repository-wide lint/typecheck result or a live business acceptance run. No tests were rerun to write this record. The main session owns environment synchronization and live Playwright runs.

## Files and final SHA-256

These hashes were read when writing this record. This executor made no source changes after the final static checks below.

| File | SHA-256 |
| --- | --- |
| `e2e/collaboration-business-scenarios.spec.ts` | `f58d0ab26ac93c9535ab7618354a59b6069b4f0e2869d05dfaa4dddd89e94cea` |
| `e2e/fixtures/collaboration-business-scenarios.ts` | `14d9be8fd6a6b60a85c8afd1d1b71aaa38744cf6b330c453631b60f98207e0d2` |

## Strict TypeScript

Exact command:

```sh
pnpm exec tsc --noEmit --strict --skipLibCheck --esModuleInterop --module esnext --moduleResolution bundler --target es2024 --jsx react-jsx e2e/collaboration-business-scenarios.spec.ts e2e/fixtures/collaboration-business-scenarios.ts
```

Result: **exit 0**, no TypeScript diagnostics. The command follows imports and skips checking dependency declaration files through the explicit `--skipLibCheck`; it does not execute the tests. The final invocation completed in 9.646 seconds. pnpm printed its existing notice that the root `package.json` `pnpm.onlyBuiltDependencies` and `pnpm.overrides` settings are ignored by the installed pnpm version; no dependency or configuration was changed.

## Focused repository ESLint

The initial direct base-config invocation was:

```sh
node packages/views/node_modules/eslint/bin/eslint.js --config packages/eslint-config/base.js e2e/collaboration-business-scenarios.spec.ts e2e/fixtures/collaboration-business-scenarios.ts
```

That invocation returned **exit 1**, with one finding at fixture line 3:

```text
'@playwright/test' should be listed in the project's dependencies, not devDependencies  import-x/no-extraneous-dependencies
```

The root package already declares `@playwright/test` as a devDependency. The shared rule's devDependency patterns include `*.spec.ts` and `test/`/`tests/`, but do not include this test-only `e2e/fixtures/*.ts` path. This is a fixture classification mismatch; it is not a missing dependency. It was not hidden by ignoring the file or disabling the rule.

For the focused check, the following exact command retained the repository base configuration and overrode only that rule's allowed devDependency paths for `e2e/**/*.ts`. No configuration file was written and no package was installed or moved into production dependencies.

```sh
node --input-type=module <<'JS'
import { ESLint } from './packages/views/node_modules/eslint/lib/api.js';
const lint = new ESLint({
  overrideConfigFile: 'packages/eslint-config/base.js',
  overrideConfig: [{ files: ['e2e/**/*.ts'], rules: {
    'import-x/no-extraneous-dependencies': ['error', { devDependencies: ['**/e2e/**'], peerDependencies: true }],
  } }],
});
const results = await lint.lintFiles(['e2e/collaboration-business-scenarios.spec.ts', 'e2e/fixtures/collaboration-business-scenarios.ts']);
const formatter = await lint.loadFormatter('stylish');
const rendered = formatter.format(results);
if (rendered) process.stdout.write(rendered);
console.log(JSON.stringify({files: results.length, errors: results.reduce((n, r) => n + r.errorCount, 0), warnings: results.reduce((n, r) => n + r.warningCount, 0)}));
process.exitCode = results.some((r) => r.errorCount || r.warningCount) ? 1 : 0;
JS
```

Final result: **exit 0**, completed in 2.711 seconds, output:

```json
{"files":2,"errors":0,"warnings":0}
```

## Playwright collection

Exact command:

```sh
pnpm exec playwright test e2e/collaboration-business-scenarios.spec.ts --list
```

Final result: **exit 0**, completed in 3.713 seconds. The root configuration collected these Chromium tests:

```text
collaboration-business-scenarios.spec.ts:100:1  S2 support ticket: manual intake becomes AI-assisted, files reach the agent, and a reopened ticket is reworked
collaboration-business-scenarios.spec.ts:251:1  S3 governance: live withdrawal really stops work, removal and cancellation fence AI, and the issue still closes
collaboration-business-scenarios.spec.ts:402:1  S1 outsourcing: client, vendor manager and workspace-less engineer close an issue with a vendor squad and reuse approved knowledge
Total: 3 tests in 1 file
```

Collection confirms that the spec loads and registers all three cases. It executes no browser journeys and is not evidence that any of the three business scenarios passed. Full repository lint, full repository TypeScript checks, Go tests, service/database operations and real model calls were not performed by this child.
