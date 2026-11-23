# Research: AC43 Codex gateway rejects the Multica integration

- Query: Explain why direct `codex exec` succeeds while the production HostExecutor/Codex app-server path receives an official-clients-only 403, and identify a legitimate unblock.
- Scope: local source and retained evidence, plus official OpenAI documentation and the matching upstream Codex 0.165.0 source. No provider calls, account/network configuration changes, client identity changes or product edits.
- Date: 2026-11-05

## Findings

### Conclusion

The observed blocker is an external gateway client-access restriction. The local Codex process and Multica's app-server initialization succeed; the gateway rejects the subsequent model request with `This account only allows Codex official clients`. Multica honestly identifies itself as `multica-agent-sdk`, while the direct official `codex exec` command identifies itself as `codex_exec`.

The name/identity difference is confirmed by local source, the failed run's actual Codex session metadata, and matching upstream code. That difference is the strongest supported explanation for the two paths' outcomes. The exact gateway rule and whether it matches an originator header, User-Agent, an upstream account restriction or another signal remain unobserved; do not claim a packet-level diagnosis or ownership of that policy.

**Recommended unblock:** the gateway/account administrator must permit this account to call Responses through the third-party Codex app-server integration identified as `multica-agent-sdk` / `Multica Agent SDK`, or supply an independently billed account/endpoint explicitly allowing that integration. Keep the real Multica identity and rerun the recorded failing scenario on a newly numbered attempt after the external change.

### Retained evidence

Private artifact root:

`/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-ac43-20261105-bi0bmk8x`

| Artifact | Observed fact |
| --- | --- |
| `connectivity/codex/summary.json` | Direct CLI probe exited 0 in 24.45 seconds, returned the expected marker, emitted `turn.completed`, and reported non-null usage |
| `real2/multiple_targets_B.json` | Provider `codex`, run `3ffd6b77-60df-4265-97d9-5f1aa4f5dd47`, business outcome `failure`, acceptance status `failed`, usage `null` |
| `real2/host-3ffd6b77-60df-4265-97d9-5f1aa4f5dd47.log` | Initialize was sent and completed in 351 ms; later model request received HTTP 403 with the official-clients-only message and `will_retry=false` |
| Same run proof in `multiple_targets_B.json` | One authorized and sealed attempt; proof `host_exit_observed`, original launch receipt and completed tracked process-group cleanup were retained |
| `real2/environment-2/workspaces/7b5e962ddd6c7caf62bea0463540104589746cf5dae6abde65b7a7850dd02221/codex-home/sessions/2026/11/05/rollout-2026-11-05T00-41-37-01a1a770-0136-77d1-8015-3f8b3314a8ff.jsonl` | The actual session metadata records originator `multica-agent-sdk`, CLI version `0.165.0`, provider `custom` |

The failure names `https://api666.zzzcoding.org/responses`, model `gpt-6-astra`, and gateway request ID `d0cef923-b8e8-4de4-9769-83d08b0ec2e1`. Those nonsecret facts are sufficient for the gateway operator to locate the rejection. No keys, auth headers or private prompt text are needed in a support request.

This is not an initialize timeout, unsupported binary, failed local process launch or Multica authorization rejection. It is also not evidence of exhausted quota: the gateway message states a client restriction. `usage=null` means no retained usage figure, not proof of zero charge.

### Code path comparison

The inspected Multica file in the isolated candidate is `source/server/pkg/agent/codex.go`:

- Lines 324–337: `codexBackend` always launches `codex app-server --listen stdio://` and communicates through JSON-RPC.
- Lines 1419–1429: initialize sends `clientInfo.name = multica-agent-sdk`, title `Multica Agent SDK`, version `0.2.0`, and `experimentalApi = true`.
- `server/pkg/agent/agent.go:428` resolves the Codex family to this backend; `builtin_runtimes.go:154` routes builtin/runtime family resolution to it. No `codex exec` transport option was found in this backend.

Official app-server documentation says the initialization metadata identifies the integrating application, and that initialization returns the upstream User-Agent. The documented Sign in with ChatGPT app-server integration states that the stable client name supplies request attribution and the name/version are incorporated in User-Agent. This supports retaining Multica's truthful identity. [App-server initialization](https://learn.chatgpt.com/docs/app-server), [official app-server integration](https://developers.openai.com/siwc/token-sharing-open-source/codex-app-server).

The matching upstream tag confirms the mechanism:

- `codex-rs/app-server/src/request_processors/initialize_processor.rs`, lines 123–125, 157–184: derives originator and User-Agent suffix from supplied client metadata and sets the process identity.
- `codex-rs/exec/src/lib.rs`, lines 258–260 and 705–722: direct exec uses originator/client name `codex_exec` and runs an in-process app-server client. Thus the successful direct path does not prove that third-party app-server clients are admitted by this gateway.

Primary source references: [Codex 0.165.0 initialize processor](https://github.com/openai/codex/blob/rust-v0.165.0/codex-rs/app-server/src/request_processors/initialize_processor.rs), [Codex 0.165.0 exec implementation](https://github.com/openai/codex/blob/rust-v0.165.0/codex-rs/exec/src/lib.rs).

### Legitimate fix options

1. **Prefer an external account-policy fix.** Ask the gateway administrator to enable third-party/app-server use for this account and the real client identity. Retain the configured account/model and current Multica backend so the subsequent run answers the original acceptance question. The exact administrative setting is gateway-specific and was not available in this research.
2. **Use an account/endpoint already authorized for this integration.** A different gateway account or official Responses-compatible account is viable if its operator permits the actual Multica usage and the user supplies/selects it. This is a new acceptance configuration and requires new before/after account/usage evidence; earlier direct probe figures cannot be reused as its baseline.
3. **A real `codex exec` adapter is a separate product change, not an acceptance shim.** It can be considered only if the gateway explicitly permits third-party automation through that official CLI entry point. It must run the actual official CLI without forging metadata and preserve Multica's launch permission, process tracking, cancellation, usage, retained records and session contracts. Merely translating app-server requests into an exec subprocess to get past the stated restriction would not establish supported access. No such adapter is currently present or verified in this codebase.

Do not rename `clientInfo`, override originator/User-Agent, substitute official-client headers, patch the binary or silently change the endpoint to bypass the restriction. A locally passing direct CLI probe cannot be relabeled as a successful Multica run.

### Acceptance implication

Keep `real2/multiple_targets_B` as a failed/incomplete external-provider lane with its original proof and error. Independent Claude scenarios may continue and establish their own positive results. Two-environment/mixed-provider AC43 remains incomplete until B can run through an allowed integration and both accounts have usable provenance and usage evidence.

## Related specs

- `.trellis/spec/server/collaboration-host-execution.md`: production Backend/HostExecutor lifecycle and truthful provider/configuration identity.
- `.trellis/spec/server/collaboration-records.md`: retained resource provenance is not a billing assertion; missing usage remains unknown.
- `.trellis/tasks/10-30-collaboration-context-handoff/release-gates-runbook-2026-11-01.md:106`: AC43 uses actual environment/account provenance and independent usage reconciliation.

## Caveats / Not Found

- No gateway policy source, administrator access or documented configuration toggle was inspected.
- No outgoing model HTTP capture was taken; exact request-header matching is an inference, not a measured fact.
- No provider retry was made by this researcher. No configuration, credentials, product code or acceptance verdict was changed.
- The report's alternative-account option is a proposed external unblock, not proof that another credential already exists locally or supports the selected model.
