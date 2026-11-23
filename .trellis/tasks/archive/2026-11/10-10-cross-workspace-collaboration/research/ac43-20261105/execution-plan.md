# AC43 continuation — 2026-11-05

## Authorization and scope

The user authorized local Codex and Claude Code execution, then removed call and quota limits for this acceptance. The user subsequently explicitly excluded gateway usage/billing reconciliation: “可以不用关注用量，你调就可以”. This supersedes the earlier request for a numerical budget and external usage dashboard in this session. No new task is needed; this continues the existing AC43 work.

This run verifies runtime/configuration provenance through real Multica business flows. External provider billing identity, actual charged quota and independent billing-account reconciliation are excluded at the user's direction, not recorded as tested or passing. Provider-reported usage is supporting evidence when available.

## Frozen source and isolation

- Baseline: `d862c5c4bb55d11074d9f4c51643871764e3a9bb`.
- Independent detached checkout and private evidence paths: `environment.json`.
- One dedicated migrated `multica_ac43_20261105_*` database for `./cmd/server`; never use the application database.
- Three test-owned human principals; two independently enrolled runtime/installation identities and local provider configurations. Both runtimes execute on this macOS host; this is logical runtime/configuration separation, not proof across two machines or OS users.
- Local configuration remains unchanged. Credentials are never copied into tracked evidence; summaries use fingerprints and redacted configuration facts.

## Behavior gap and implementation boundary

The existing provider smoke uses an HTTP stub, while existing host E2E uses a synthetic provider. Neither proves real provider execution through the authoritative server's cross-account authorship, resource grants and handoff transitions.

Add only tagged acceptance fixtures/scenarios under `server/cmd/server/ac43*_test.go`, reusing the real router, production service, Runner, HostExecutor, Multica CLI and existing test helpers. No production behavior change or dependency addition is planned.

## Sequence and evidence

1. Probe both existing CLIs with inert prompts; retain exact versions, outcomes and reported usage. Completed: both returned the requested marker.
2. Compile the acceptance harness and review its authorization, identity assertions, output privacy and cleanup.
3. Run serially on the isolated database: two targets on A/B, squad leader A/member B, two authors using A, ordinary handler transfer followed by A.
4. Assert authoritative creator/input authors, agent/resource owners, grants/epochs, runtime/installation identities and complete managed termination proofs. Preserve original failures and distinguish harness faults, provider failures and product failures.
5. Independently review the actual result and record the tested scope versus the original AC43 billing clause. The parent is not archived automatically by this continuation.

## User-selected Codex exec continuation

The ordinary Codex app-server path completed initialization but the configured gateway rejected its real `multica-agent-sdk` client identity with HTTP403, while a direct official `codex exec` call succeeded. Preserve this as a failed ordinary-adapter result.

The user then explicitly selected “那就 直接 codex exec”. The acceptance may therefore use an explicitly identified task-only protocol bridge that starts the actual official `codex exec --json`, preserving production Multica claims, permissions, input authorship, execution recording and final managed-process proof. This is evidence for `codex_exec_test_bridge`, not a claim that the built-in app-server path passed or that this bridge ships as product support.

The bridge keeps its own logical session handle separate from the actual Codex `thread.started` ID, performs no bootstrap model call, rejects unsupported/resume requests, and must wait for the actual child plus a successful terminal event before reporting success. It must not replace or spoof HTTP client identity, fabricate model output/usage, or silently retry with another provider. Child processes remain in the managed wrapper process group. Fake-executable tests and independent review precede actual execution.

Meanwhile `real3` independently verified Claude baseline, mixed human authors and ordinary transfer, with exactly three successful managed runs and stable grant/runtime/configuration facts. Its summary explicitly remains partial.
