# C4 collaboration knowledge runtime integration

C4 now connects the A2 resolver to collaboration claims and actual host input.
The production gate remains closed. This is implementation and deterministic
evidence, not real Provider rollout or model-understanding evidence.

## Implemented contract

- Managed claim URL `knowledge_version=1` negotiates a source-free descriptor.
  The body remains compatible with the old strict claim decoder. The old
  handler checks only its named handoff/recording URL keys; a missing response
  descriptor means unsupported. Once a run is admitted, a legacy/downgraded
  claim cannot obtain that run; an already-frozen legacy claim cannot upgrade.
- `CollaborationService.KnowledgeContextEnabled` is an instance field, copied
  from `Handler.TaskService.KnowledgeContextEnabled`. No global mutable gate or
  independent production enabling switch was added.
- The two MDT-only POST endpoints are
  `/api/collaboration/runtime/runs/{runId}/knowledge-context/prepare` and
  `/api/collaboration/runtime/runs/{runId}/knowledge-context/authorize`.
  They accept the full real run identity, original request UUID, frozen handoff
  ID and exact invocation fingerprint. Authorization additionally binds the
  manifest/prompt hashes and existing permit sequence/nonce. No task ID is
  synthesized, and a business token cannot invoke either endpoint.
- Preparation constructs consumer facts from the frozen accepted input authors,
  current issue workspace/project, actual runtime owner and collaboration
  audience. It stores only the exact manifest and identities. Original-key
  replay reauthorizes and reconstructs that same manifest; it cannot substitute
  later approvals. Every physical Provider start rechecks current authority and
  its exact existing handoff binding before the local burn/start.
- The dedicated transaction does not invoke the old blocking runtime-source
  lock helper. It uses sorted user/workspace/member fences, A2 target/source
  NOWAIT locks, then explicit command/grant/group-run NOWAIT prelocks. Existing
  resource readers already use NOWAIT/try-lock. The original admission checker
  runs under those locks. The late-run contention regression proves bounded
  refusal rather than waiting behind the inverse runtime writer graph.
- Internal `knowledge_claim`, `knowledge_preparation` and
  `knowledge_authorization` events contain no body/summary. Existing SQL kind
  allowlists exclude them from records, checkpoints and handoff sources; the
  real record regression checks private IDs/body never enter that projection.
- Termination-only GET `.../knowledge-context/status` returns only run identity,
  prepared operation state and original request ID. Empty authorized sets also
  create preparation state, so this is not a private-contribution count oracle.
  Source/gate loss does not strand metadata recovery or original proof.

## Host and native-session policy

The implementation retains `NewLaunchLifecycle`, mcb business-token refresh,
mct termination authority, accepted inputs, delegation/handoff missing facts,
recording budgets and original-key recovery. Contributions are appended only to
the actual per-turn prompt. Stable CLAUDE/base files and the supervisor ledger
receive no knowledge body.

Initial support is the existing built-in local Claude host adapter. Custom
profile runtimes are rejected by the existing daemon host selector and server
runtime registration; another protocol family cannot claim Claude support.
Every admitted run, including an empty contribution set, has a distinct actual
host directory/session key and never reads or saves a reusable native pointer.

A unique cwd alone did not prove global native resume isolation. The final
implementation additionally puts Claude's documented `--no-session-persistence`
flag into every knowledge print invocation before computing its fingerprint.
The callback rejects a missing flag; valued, negated and reenable forms fail the
safe-argument parser. An older CLI's unknown-option refusal has no base-only or
flag-removing fallback. Account credentials and Provider home are not copied.

Vendor provenance is recorded in
`claude-session-persistence-reference.json`: the official CLI reference says the
print-only flag prevents sessions from being saved to disk or resumed. Fake argv
and lifecycle tests prove application delivery; the vendor contract supplies
native persistence semantics. No installed CLI, Provider account or quota was
accessed. C2 was notified and independently keeps this flag unsupported for its
ordinary reusable-session contract.

## Shared resolver repair

Main identified that A2 sorted manifest entries but left contributions in target
order. That made mixed project/workspace first disclosure differ from an exact
manifest replay. C4's narrowly allocated A2 change sorts contributions by
candidate ID too. The real database regression uses a high project UUID and low
workspace UUID and compares the actual first and repeated preparation response.
`canonical-order-red.jsonl` proves the prior failure; the final green log proves
the correction. Main and C2 received the resolver source hash.

## Verification and source provenance

All database execution used only `multica_knowledge_c4_20261101_fabcbe23`, created
from the A2 private template's connection settings. The private environment is
`/tmp/multica_knowledge_c4_20261101_fabcbe23.env`; no credentials are stored here.

`final-results.json` contains every actual test/subtest name:

| Final suite | Passed cases | Top-level tests | Skips |
| --- | ---: | ---: | ---: |
| Agent/protocol/host, including mandatory persistence | 218 | 61 | 0 |
| Service, resolver, host admission/handoff/record regressions | 92 | 45 | 0 |
| Handler boundaries and extension parsing | 9 | 5 | 0 |
| Production routes, including real MDT and closed gate | 2 | 2 | 0 |

Scoped Go vet, the full Go build and `git diff --check` passed. Failed fixture
diagnostics remain in the earlier logs; they are not acceptance evidence.
Source-bound red cases separately cover retained host option overrides,
canonical contribution order and missing native persistence protection.

`verification.json` records the command arrays and exit codes for the first full
pass. The final persistence pass is `final-agent-host.jsonl`, with
`final-source.json`, `persistence-green-source.json`, `files-changed.json` and
the final build/vet logs. Concurrent C2 changes were limited to its own ordinary
task/history files and are listed explicitly in `final-results.json`; C4-owned
product sources stayed stable during the relevant final checks.

## Ownership and remaining integration

`files-changed.json` is the review allowlist, distinguishing new C4 files,
allocated shared edits, and main-generated SQL output. No commit was created.
The router slot is released and the five-query SQL file is stable; main already
performed sole-owner sqlc generation.

Main retains the combined C2/C3/C4 review and enablement decision, formal G2
status, broader platform/installed-client acceptance, and final spec/built-in
guidance synchronization. The local Claude persistence behavior is documented
and fake-process verified; a real native/account/platform matrix was not run.
G2 must remain open until those combined gates pass.
