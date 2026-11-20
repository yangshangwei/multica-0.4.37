# Official CLI direct execution harness

The user explicitly selected the official `codex exec` path instead of the
Multica app-server SDK path. This is a separate evidence cohort; a successful
CLI run will not prove the production app-server adapter works with the account.
No client identity is forged or rewritten.

`prepare_direct_cli_overlay.py` preserves production source files. It writes a
private generated copy of `server/pkg/agent/builtin_runtimes.go` with exactly one
branch at `ResolveBackend`: provider `codex` plus `SECREM_DIRECT_CLI=1` selects the
task's tagged `securityDirectCLIBackend`. All other providers and unmarked
execution use the original resolver. The injected backend exists only under
`agentintegration && !windows`; the generated copy, original source and exact
replacement string have separate hashes in private `build/direct-overlay/`.

The existing actual platform fixture still creates a fresh squad and human
owner, claims a server task, and invokes `RK1KnowledgeDriver.Run`. Production
`runTask` still prepares the isolated work directory, copies skills, writes the
complete AGENTS.md, generates the original prompt, assembles the original mat_
environment, starts the platform task, drains genuine native events, and reports
the terminal result. Only the provider backend execution/parser is replaced.
There is no app-server RPC translation layer in this cohort.

The direct backend executes the wrapper with exactly
`exec --json --skip-git-repo-check -`, writing the unmodified production prompt to
stdin. Explicit resume uses
`exec resume --json --skip-git-repo-check <session-id> -`; it is never silently
dropped. System-prompt projection, custom launch arguments, MCP configuration,
knowledge/lifecycle adapters and nondefault service tier are refused. Real mode
requires the original Go environment gate, the private inherited driver marker,
explicit `execution_mode=official-cli-direct` and a hash-bound private config.
The old fixture's full source/binary/admission checks remain in force.

Native JSONL is retained privately. Exactly one valid native thread UUID and one
native terminal event are required. Empty stdout, malformed events, duplicate or
conflicting thread identities, multiple terminals, native error/failure and a
nonzero process after `turn.completed` all refuse completion. Native final text
and native usage feed the existing report path; missing usage stays absent and
metadata records `usage_observed=false`. Platform comments remain subject to the
original source/task author checks; model text or process exit alone never
satisfies semantic acceptance.

## Verification

- `TestSecurityDirectCLI*`: 10 named runs passed, no skipped tests, through the
  normal agent CLI guard with test-created fake executables. Cases include exact
  args/stdin, native completion/usage, empty stream, unsuccessful process,
  conflicting and duplicate threads, malformed stream, failure, real entry
  guards and explicit resume.
- Actual `TestSecurityRemediationDirectP1Fake` passed against the allocated
  `secrem_v2_real_20261120_cli_fake_p1` database. The direct fake CLI performed real
  Multica API reads/writes with the actual claimed mat_; all nine platform
  assertions passed. No model request was sent.
- That run delivered an exact 620-byte production prompt and an 80,171-byte
  generated AGENTS.md. The instruction hash stayed unchanged. These are input
  delivery observations, not proof of model understanding.
- The fake's model has no discovered effort capability, so production resolution
  left the backend option empty; metadata states that fact. Real CLI effort is
  separately bound in the immutable transport configuration, and actual request
  evidence must verify the effective value.
- `gofmt`, generated overlay source identity, Python syntax and task-scoped
  whitespace checks passed.

The current compiled P1 harness is private
`/Volumes/artisan/code/2026/secrem-v2-cli-evidence-20261120/bin/direct-harness.test`.
Current source and binary hashes are in `direct-cli-harness-hashes-20261120.json`.
Root owns final admission and real calls. The direct backend result is mechanical;
P1 human incident semantics still require independent review of actual platform
comments and full workspace changes.

## Historical source-retention limitation

The first consumed CLI binary (a340c9...) and its original input/event/evidence hashes were retained. The untracked direct backend source with hash5fb8b93... was not copied before the later fixed-material successor edit; the overlay referred to that mutable task file. Its original bytes are not independently recoverable from the evidence directory. Do not infer full source reproducibility from the stored hash or reconstruct it after the fact. The unchanged direct entry source0dfb11... and original P1 helper were preserved, and the current successor source set is now copied byte-for-byte before further changes.
