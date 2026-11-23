# AC43 acceptance harness

Smallest gap: existing real smoke uses a scripted server and one provider; no current four-scenario evidence proves runtime routing and author/grant preservation together.

Boundary: two new tagged test files in the private candidate server/cmd/server. Reuse existing fixture schema, real router and signed installation enrollment, production services and Runner/HostExecutor. No product or migration changes. The fixture creates three distinct humans and two native macOS configured execution contexts with independent installation, daemon, runtime, ledger and workspace identities. The OS user is shared and is not a security-isolation claim.

The user explicitly excluded external gateway account/quota reconciliation. Test results report runtime provenance only and preserve nullable provider usage without inferring billing ownership.

Files: ac43_fixture_test.go owns signed enrollment, three-user fixture, exact live epochs, resource consent and runtime promotion; ac43_real_provider_test.go owns opt-in, exact task database guard, named serial scenarios and private evidence.

Implementation complete in the isolated candidate. No real provider calls performed by implementer.

## Verification

- `go test -tags=agentintegration -c -o <private>/ac43.test ./cmd/server` passed for the initial harness; no database or provider was executed by compilation.
- `TestAC43FixtureContract` on coordinator-assigned `multica_ac43_20261105_c7fbafeb`: first run failed because a test-created executable named `provider` collided with RK1's provider directory. Preserved `fixture-contract-1.log`; changed the executable to `fake-provider`.
- Second exact fixture run passed: 1 named test, 0 failed/skipped, package 2.614s. `fixture-contract-2.log` preserved privately. This proves three actors, signed native installation enrollment, two runtime grants, actual queue promotion, ordinary transfer and cross-environment squad registration. It does not launch any provider.
- Independent review corrections: exact trimmed response marker, persisted issue creator/handler read and comparison, unchanged binding/resource epochs and invocation digest across transfer, and Skip when the task-specific database assignment is absent.
- Final compile/vet and real-run execution are coordinator/reviewer-owned.

## Entry

`TestAC43RealProviderProvenance` requires `MULTICA_RUN_REAL_AGENT_SMOKE=1`, exact `AC43_DATABASE_NAME` matching the explicit `DATABASE_URL`, `AC43_OUT` naming a new private directory, `AC43_CLI`, and `AC43_CLAUDE_EXECUTABLE`, `AC43_CLAUDE_MODEL`, `AC43_CODEX_EXECUTABLE`, `AC43_CODEX_MODEL`. All executable/output paths are absolute. Provider versions are actually probed after authorization and before model calls.

The seven planned real runs are two independent targets, one batch with two human authors, one post-transfer run, and squad leader/member/continuation. Runtime A is Claude; runtime B is Codex. All execution is serial. The test preserves individual per-run evidence before assertions, private managed ledger and host/HTTP diagnostics, plus a summary. Public record pagination is consumed completely. Test-only DB/PG/AC43/RK1 configuration is removed from the provider process environment and restored afterwards.

## Remaining boundaries

The harness directly invokes production human service methods for setup, submit, control and transfer; runtime execution uses real HTTP router endpoints. This is an integration acceptance, not browser or installed-daemon GUI acceptance. Two configured environments share the macOS user. The account/source metadata proves configured runtime routing; it does not establish upstream billing identity. External gateway account/quota reconciliation was explicitly excluded by the user. Nullable provider usage is retained when present. Root must freeze CLI/config/source hashes and inspect actual real run results before recording completion.


## Real1 harness correction

The coordinator's first real attempt stopped before provider launch: production OpenLedger creates its leaf directory only, and the harness had not created private-ledger. Host.Probe has the same parent-directory contract. Added ac43ExecutionPaths to create task-owned private-ledger and environment-N parents at0700, reject nonprivate/non-directory paths, then construct the ledger. Extended TestAC43FixtureContract to exercise actual Ledger.Lock/Close and Host.Probe with the fake executable and assert all four directories are0700. Targeted fixture3 passed1 test/0fail/0skip in2.300s. Original real1 remains retained. Compiled ac43-v2.test; source and binary hashes sent to coordinator/reviewer. No real provider call by implementer.

## Independent Claude continuation after real2

Coordinator reported real2 Claude A succeeded while Codex B returned gateway403 despite complete terminal proof. Preserve that failure. Added optional `AC43_SCENARIOS=claude_flow` to run only A baseline, mixed human authors and ordinary transfer, recording `passed_partial_runtime_provenance` plus explicit unrun B/cross-environment-squad entries. The default full selection still executes all four scenarios; B follows the independent Claude flow so a blocked B cannot discard those observations. Unknown selections fail before provider version probing. No gateway identity spoofing or product changes. Compiled `ac43-v3.test`; reviewer owns delta review and root owns real3.

## Explicit codex exec test transport

After the built-in Codex app-server path was rejected by the configured gateway, the user explicitly selected direct `codex exec`. Added only task-owned testdata/ac43-codex-exec-bridge.py plus credential-free test_ac43_codex_exec_bridge.py. No product source or HTTP client identity was changed. The wrapper accepts the minimal adapter RPC handshake using a clearly labeled local handle, then runs exactly one actual `codex exec --json --skip-git-repo-check --sandbox read-only --model <requested> -` child with unchanged text/stdin, cwd and task CODEX_HOME. The actual Codex thread ID is recorded separately. No bootstrap/model answer is synthesized; resume and unsupported RPC/events are rejected. Only the exact observed pre-turn disable_response_storage config warning is retained as an inert diagnostic; the same warning in an active turn fails. Completion requires actual stdout terminal, answer, valid usage, EOF and waited exit0. The child inherits the managed process group so group Stop reaches it. Raw output/error stays in private0700 evidence.

10 credential-free tests passed in3.364s: output/usage/ID/argv/environment, waited delayed exit, nonzero exit after terminal, missing terminal, unsupported side-effect event, private error detail, resume/unknown operation refusal, process-group stop, actual version forwarding and exact warning boundaries. This bridge is explicit test transport, not acceptance of the built-in app-server adapter, not OS isolation, and not a general Codex integration. Read-only sandbox narrows the B-agent acceptance to text responses. Final bridge SHA2564063d0d06ca4ec2aee9d45affe924f050ce2c1c0b47460f20a4c17f35c54dfac; test SHA256d2de97741c1cb3984637b46743f34b2551aa98368db71691b2a0d0905d51e075. Root owns actual model execution and final artifact review.

## Real4 diagnostic rendering correction

The official exec CLI renders a long configuration path with an ellipsis and no closing parenthesis. Real4 retained a pre-turn known disable_response_storage warning in that form and correctly refused before the initial narrow pattern was updated. Changed only the full-match path portion: fixed exact prefix/count1, one non-newline user path, fixed exact ignored-setting suffix; still pre-turn item.completed/error only. Added sanitized actual truncated-path regression and different-setting rejection. Existing active-turn rejection retained. 12 credential-free tests passed in2.882s. Final bridge SHA2565809e94bb129227e01ebb80cbf3577b6ac524d06b35a6a106f57b29bac318f0d; test SHA256f4dfde51fb86728a7d59d992c5b4f5e6169e20ff62a574919ef7c1fea0e998fb. No Go or product changes; previous real4 evidence preserved.
