# Codex ordinary and collaboration integration handoff

Date: 2026-11-05 Asia/Shanghai. Worktree: `/Volumes/artisan/code/2026/mc-knowledge-codex-20261104`.
Owner: B (`codex_integration`). No commits, installed Agent execution, user credential reads, schema migration, dependency or wire shape changes. Product and tests are ready for independent review and C's integrated framework. Source hashes are in `integration-source.json`.

## Behavior

- Current and retained ordinary claims/preparations admit builtin local Codex through the same provider support contract as the adapter. Provider/session identity remains exact; downgrade and complete-history guards remain active.
- Codex daemon configuration namespace uses the actual source home and the shared adapter's config-source digest. Existing Claude namespace serialization remains unchanged. Unreadable sources cannot negotiate. The helper currently hashes auth bytes: a token refresh changes its identity and fails closed/retires the old resume binding; this implementation does not infer stable account identity across credential refresh.
- Host claim and reauthorization require runtime provider to equal the frozen preparation provider, not merely belong to the supported set.
- Codex private Host execution sets `CodexEphemeral`; Claude keeps `--no-session-persistence`. Host fingerprints bind actual prepared environment/config sources both before disclosure and on each authorization. No private body is written into these descriptors. Preconfigured ephemeral options are rejected by Host Probe so the policy remains host-owned.
- Host's existing `Config.PrivateInput` is preserved for A's private diagnostics boundary. Existing pointer suppression and launch authorization remain authoritative.
- Existing host fake RPC fixture now supports explicit ephemeral success, persistent response and missing ephemeral response controls. Both failure controls prove zero private turn input and no second permit. Positive control proves actual authorized prompt hash and no reusable pointer.

## Files

- `server/internal/service/task_knowledge.go`
- `server/internal/service/task_knowledge_prepare.go`
- `server/internal/service/collaboration_knowledge_runtime.go`
- `server/internal/service/collaboration_knowledge_runtime_test.go`
- `server/internal/service/task_knowledge_codex_test.go`
- `server/internal/daemon/knowledge_context.go`
- `server/internal/daemon/knowledge_codex_configuration_test.go`
- `server/internal/daemon/collaborationrun/host_config.go`
- `server/internal/daemon/collaborationrun/host_knowledge.go`
- `server/internal/daemon/collaborationrun/host_runner.go`
- `server/internal/daemon/collaborationrun/host_knowledge_test.go`
- `server/internal/daemon/collaborationrun/host_handoff_codex_launch_test.go`
- `server/internal/daemon/collaborationrun/host_knowledge_codex_test.go`

## Verification

All execution commands used the existing `scripts/go-test-with-agent-cli-guard.sh`, except the initial two-test baseline (both tests use test-owned fake executables and no installed Agent).

- Baseline: `go test ./internal/daemon ./internal/daemon/collaborationrun -run '^(TestKnowledgeFailuresNeverEnterFreshReplay|TestHostKnowledgePersistenceIsMandatoryInExactInvocationFingerprint)$' -count=1` passed both packages.
- New Codex service cases plus selected existing claim/history/rebind regressions: normal and race each **16 named pass, 0 fail, 0 skip**, using separately allocated new databases `multica_kncodex_service_normal_20261104_318509c1` and `multica_kncodex_service_race_20261104_318509c1`. Exact identity verified through psql before either run. No database creation/migration/drop was performed by B. Service DB window released to main/C after both runs.
- Daemon namespace, Host knowledge, Codex ephemeral and existing nonknowledge fallback regressions: normal and race each **23 named pass, 0 fail, 0 skip**.
- `go vet ./internal/daemon ./internal/daemon/collaborationrun ./internal/service` passed.
- gofmt and `git diff --check` passed. After simplifying the internal host fingerprint environment argument to an explicit map, focused Host Codex ephemeral plus Claude fingerprint **race** rerun passed (5 named terminals).

Raw JSON and summaries: `.omx/private/knowledge-codex-integration/{service-normal-02,service-race-02,daemon-host-normal,daemon-host-race}.jsonl` plus corresponding `*-result.json`. An earlier service invocation failed at compile only during A's concurrent adapter edit; it ran no service fixture and its original log `service-normal.jsonl` is retained. New Host test development encountered an empty-capture helper mismatch and two compilation mistakes; those test-only issues were corrected, not product assertions relaxed.

## Remaining integration work

- C's full ordinary/Host Codex fake suite and main-owned real acceptance remain separate. This handoff makes no real-model success claim.
- A/main were notified to cover explicit Codex configuration/profile/session selectors before ordinary fallback. B already consults `KnowledgeSessionSelectorPresent` using the actual runtime provider and protects the complete retained namespace. Selector classification itself belongs to A; final review must verify its Codex policy.
- Source home and prepared home are intentionally different hash domains. The former binds claim namespace; the latter binds the actual physical invocation. Ambient operator configuration remains trusted per main's explicit decision; user global hooks are not silently changed.
- No frontend edits; no TypeScript check requested or run. Cross-layer Go checks above do not substitute for full repository acceptance.
