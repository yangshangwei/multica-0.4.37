# Independent B integration review

- Date: 2026-11-05 Asia/Shanghai.
- Worktree: `/Volumes/artisan/code/2026/mc-knowledge-codex-20261104`.
- Scope: the 13 files in `integration-source.json`; source-only inspection of their adapter interfaces and existing execution callers. No full adapter/harness review, database access, installed CLI execution, global configuration read, commit or product edit.
- Verdict: **PASS WITH NOTES for B integration. Zero blocking bugs identified within B's owned files. One actionable cross-lane configuration-binding issue remains open for A and blocks treating the combined Codex path as accepted.**

## Findings fixed

None. Only this review report was written. Writer-owned source remains unchanged.

## Findings not fixed

### CROSS-1 — Bind custom provider environment credential sources (A-owned, open)

`knowledge_context.go:93` correctly delegates namespace construction to the shared `agent.KnowledgeCodexConfigFingerprint`; Host fingerprints delegate to the same adapter implementation. At review time, `server/pkg/agent/codex_knowledge.go` hashes config/auth file bytes plus the fixed environment keys `OPENAI_API_KEY`, `CODEX_API_KEY` and `OPENAI_BASE_URL`. Its TOML traversal binds external instruction files but does not resolve the selected provider's `env_key` or `env_http_headers` values.

Concrete trigger: the configured provider uses `env_key = "CUSTOM_PROVIDER_KEY"`, and the daemon's inherited value of `CUSTOM_PROVIDER_KEY` changes while config bytes remain identical. The source namespace and current invocation digest can remain unchanged although Codex will authenticate with different credentials. This matters for the exact account/configuration namespace across native-session reuse. The repository already supports custom env-key providers (`execenv_test.go:3516` onwards); it is not an invented input type.

Required correction in A: hash the effective resolved values of selected provider credential/header environment references without exposing those values; reject unreadable or unsupported account-source resolution. Add controls proving unchanged values keep identity, changed/missing values change identity or refuse, and configured environment overrides take the same precedence as actual execution. Recheck B's existing namespace tests and final integrated fake evidence after A freezes. The main session was notified and has assigned the correction to A. No B source change is currently demonstrated as necessary.

### Evidence boundaries (nonblocking for B)

- The source namespace and prepared invocation use different homes intentionally: claim identity follows the source configuration, while actual launch binds prepared task/Host configuration. This review traced the ordinary prepared `CODEX_HOME` through daemon backend configuration and the Host prepared environment through prepare/authorize. Source credential-byte refresh changes the namespace and can retire a previously resumable binding; B documents this conservative behavior and does not claim stable identity across token refresh.
- The four retained raw test logs are genuine passing bounded evidence. They are not an independently rerun final integrated adapter/harness suite. The handoff reports a later five-terminal focused Host race rerun after the explicit fingerprint environment-argument simplification; that rerun's raw log was not present in the supplied B evidence directory, so this review does not independently certify that claim.
- `go vet` passing is reported by the writer; no saved vet command/output was supplied in B's evidence directory. This reviewer did not run vet or compile/test packages while writers and other test lanes were active.
- Installed Codex ephemeral behavior and real-model understanding remain separate runtime acceptance work. Fake response enforcement proves refusal/continuation logic, not vendor behavior.

## Reviewed contracts

### Provider exactness and retained metadata

- `task_knowledge.go` adds supported Codex to metadata decoding, invocation admission and finalization while retaining exact runtime/provider/session/configuration equality. The runtime support helper is used instead of inventing a second supported-provider list.
- `task_knowledge_prepare.go` rechecks the current runtime provider against the stored descriptor before disclosure/launch authorization. Agent/runtime configuration digest comparisons remain intact.
- `collaboration_knowledge_runtime.go` requires the current runtime provider to equal the frozen preparation provider both for initial positive negotiation and in the locked knowledge operation. It does not merely accept any member of the supported-provider set.
- Existing `system`/`result` native-observation source categories are unchanged; no new wire property or enum is introduced by B. There are no diffs under `server/pkg/protocol`, `server/pkg/db`, `server/migrations` or `packages/core` at this review point.

### Unsupported and legacy history

- `BuildTaskKnowledgeClaim` applies session/config selector detection using the actual provider before safe fallback. Selector classification belongs to A and remains a final integrated-review dependency.
- A currently negotiated task or any retained private exposure still rejects an unsupported client, including a blank chosen pointer. The complete runtime namespace search and history pagination logic are unchanged.
- Supported configuration changes retire the old native selection rather than relabel its exposure. `taskKnowledgeExposure` retains both server-side agent/runtime configuration digest and exact provider/runtime/agent/scope/configuration comparisons.
- Proven legacy work remains eligible for base execution; adding Codex does not globally refuse non-knowledge tasks.

### Host ownership and preparation

- `HostExecutor.Probe` rejects caller-supplied `CodexEphemeral` along with the existing forbidden option overrides; the Host runner owns the choice.
- A knowledge run clears resume expectations, selects Codex ephemeral mode or the existing Claude nonpersistence flag before fingerprinting, and retains zero reusable native pointers after completion.
- Codex prepare hashes the actual prepared configuration/environment. Authorization recomputes the same file-backed fingerprint and compares it with the prepared request before granting a physical launch. The actual callback rebuilds current scoped business credentials; credentials and process-temp paths are not falsely treated as source material.
- Existing before-launch source authorization, handoff binding, process proof, private-input flag and cleanup remain in place. The ordinary adapter's native-ID ACK and actual ephemeral enforcement are A-owned contracts consumed by B, not independently re-certified here.

## Source and evidence verification

All 13 current file SHA-256 values matched the supplied `integration-source.json` during review. No B file changed during that comparison.

| Saved run | Recounted named pass terminals | Fail / skip | Package terminals | Raw SHA-256 |
| --- | ---: | --- | --- | --- |
| `service-normal-02.jsonl` | 16 | 0 / 0 | service pass | `4dfcdcd310e646b7b8447c591313bcae847c7798d223784d17f0ab7ea332f164` |
| `service-race-02.jsonl` | 16 | 0 / 0 | service pass | `fde6d8b8ff28696e3c46c3663515811c21b9fe77868e82614db824c6f72f2976` |
| `daemon-host-normal.jsonl` | 23 | 0 / 0 | daemon + collaborationrun pass | `38e4db8767ad4f2e6fe6d6116753528e0cb446f043880d71f8cdd6b334d7a462` |
| `daemon-host-race.jsonl` | 23 | 0 / 0 | daemon + collaborationrun pass | `5ab03217bf955901ffa176e3ceba12a78a8c3c365f3052b69ea6bb356b23023d` |

Every raw line parsed as JSON; no race diagnostic was present. Counts include Go named test and subtest terminals, not a claim that all terminals are independent leaf scenarios. The saved service result records identify separate normal/race databases; database identity was not queried by this reviewer.

The test code covers Codex builtin/provider/profile admission; exact native exposure and withdrawal; legacy downgrade and changed configuration; collaboration provider change and revocation; source-config drift; ephemeral true/false/missing responses; approved actual prompt hash; no second permit on ephemeral rejection; and no reusable Host pointer. Selected existing nonknowledge Codex fallback, Claude fingerprint, history/rebind and knowledge lifecycle regressions also occur in the raw logs.

## Verification status

- Formatting/static whitespace: **pass**, fresh `gofmt -l` over all 13 files returned no paths; fresh `git diff --check` over those files returned 0.
- Go vet: writer-reported pass; not independently executed or certified from saved raw evidence.
- Type-check/compile: not rerun by this reviewer; saved Go test package passes establish compilation for those recorded runs, not a later all-lane snapshot.
- Tests: saved bounded normal/race evidence independently recounted as above; no fresh test or database invocation.
- Full combined readiness: **pending CROSS-1 and independent A/C review plus current integrated fake acceptance**. Real acceptance and rollout are not inferred.
