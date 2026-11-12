# Codex 0.166.0 registration continuation

Date: 2026-11-05. Scope: test-only harness version binding. Parent authorized this bounded continuation; model remains `gpt-6-astra`, cap remains 15, actual calls remain 0. Previous accepted 0.165 fake evidence is retained unchanged.

## Local schema comparison

Read existing offline schema bundles under the main checkout `.omx/private/knowledge-codex-20261104/protocol-schema/v2` and `.omx/private/knowledge-codex-20261105/protocol-schema-0166/v2`. No installed CLI was invoked for this comparison.

- `ThreadStartParams` and `ThreadResumeParams` are identical.
- `Thread` and `Turn` definitions, exact thread/turn ID fields, ephemeral flag, model fields and notification correlation fields remain identical.
- `agentMessage` and `commandExecution` item variants are unchanged.
- Response schema differences are additive optional item timestamps, optional image-generation identifier and a new `policyCapacityUnavailable` error enum value. `TurnStartParams` additionally describes an optional name for additional untrusted context; the harness does not send that field.
- This confirms the schema surfaces used by the harness, not actual 0.166 runtime/model behavior.

## Patch

- `rk1k_codex_registration_test.go`: admit only exactly `0.165.0` and `0.166.0`; retain hash/expiry/account/source/budget checks. Resolve optional private version-binding metadata through the full hash-bound registration.
- `rk1k_fixture_test.go`: real fixture writes its registration reference into the private provider plan directory so metadata probes and host configuration agree on the registered version.
- `rk1k_codex_provider_test.go`: `--version` reports the resolved registered version, without starting the installed CLI. Fake-only fixtures retain `0.165.0`; malformed/stale real metadata refuses instead of silently reporting the fake version.
- `rk1k_codex_controls_test.go`: extend the existing registration controls to accept the reviewed second version; reject adjacent minor/patch, suffix and blank versions; verify bound metadata propagation and fake baseline preservation.

No new top-level tests, dependencies, product contracts, model substitutions, real executions or database operations.

## Verification status

Gofmt and `git diff --check` passed. Builds and test execution intentionally not run: main owns the shared verification window and will grant the compile/test permit. Proposed checks are compile-only binding followed by existing registration and RPC controls on a fresh explicitly allocated database; complete fake evidence for changed metadata is a main-session acceptance decision.

## Independent review V166-1

The missing-metadata branch now explicitly refuses when `MULTICA_RUN_REAL_AGENT_SMOKE=1`; it cannot silently fall back to the historical fake version after a real fixture metadata file disappears. The added regression creates and deletes that binding under the real gate and requires refusal; the fake-baseline control explicitly clears the gate. Gofmt/diff-check passed after the fix; execution is still pending the coordinator window.

## Authorized verification completed

The explicit ≤10-minute coordinated window permitted serial source-bound compilation, tagged cmd/server vet and fresh-database fake normal/race. Both compilation records have exit0 and identical stable full server manifests; tagged vet passed. Existing CLI was checked against SHA `22dd56e7f63decfeee03654fb02602a898bfa927d7c335f9cb8a590a8b7686ab` before use.

Both suites completed **exit0 / accepted / source_stable**, each **53 named passes, zero failures/skips/invalid JSON, exact eight top-level tests**. Durable summaries each contain15 measured fake starts +3 fake seeds +0 paid. Normal UTC05:36:09.343953–05:36:39.596094; race UTC05:37:14.617169–05:38:18.860863. No owned test processes remained atUTC05:38:31.642850, and the heavy window was released.

Evidence references and hashes: `version0166-fake-results.json`. New test binaries: normal SHA `aad059d0c57221deeca82ca39fad6012c4c7ad0745cbd6f2c245dc03d8490bb3`; race SHA `93fb026fd0c3b133dcf823402d75fcfca247415c89204ae5beff14d23ba0971a`. The registration tests validate both reviewed version strings and refusal controls; the fake executable deliberately preserves0.165 protocol baseline. This is not a real0.166 model execution. Main owns new native0.166 binding, final independent review, real account window and execution.
