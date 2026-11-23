# Independent P0 review: shared observer and authority core

Date: 2026-11-11, Asia/Shanghai. Reviewer: native `trellis-check` /
`check_resume_evidence`. Compared with HEAD
`1ce4e81223be3397fe15e2649debfac40720ddd9`.

## Verdict

**PASS_P0_CODE_AND_RECORDED_REGRESSIONS.** No blocking product defect was found
in the eight-file P0 extraction. The current transaction-only source matches
the final handler/router normal and race records; the reviewer independently
recomputed log hashes, inventories and terminal counts without running tests.

This verdict covers the behavior-preserving service extraction and its recorded
endpoint regressions. It does not cover the concurrent client implementation,
future P1 authorization code, final whole-package commit checks or full
completion enforcement. P0 adds no grant, enforce, usable skip, migration or
status-writer interceptor. M1 and the six businesses remain incomplete.

## Findings (fixed)

No P0 code correction was required or made by the reviewer.

The main session updated the four owning specifications and the built-in
creating-agents source map to name the service implementation. The reviewer
reread those changes: they preserve current observe-only/non-authorizing
contracts and do not claim that extraction installs enforcement.

The P1 authorization draft contained an ambiguous suggestion to remove
conflicting workspace-selection headers. The main design now explicitly rejects
conflicts with the actual source. It also clarifies `target_position:null` as
no explicit override, preserving existing automatic column ordering. These are
future-stage contract clarifications; they did not require a P0 code change.

## Findings (not fixed)

No unresolved P0 source defect. The main session still owns final pure
contracts, sqlc/format checks, staged-source freeze and commit/integration.
Those later actions are not claimed by this review.

P1 must preserve the observer's strict caller prerequisites, including paired
project ID/revision and schema preparation. The extracted observer is not by
itself a complete completion gate or an issuer/consumer authorization model.
The authoritative design resolves the two future-stage draft clarifications
above; implementation must follow that design, not the superseded suggestions.

## Source assessment

- `service/artifact_authority.go` owns the single credential, authority, user
  and retained-spec implementation. Complete validators accept only `pgx.Tx`
  and build queries from it. Service does not read a substitute actor from
  context, inject authenticated contexts, depend on HTTP, or begin/commit/
  roll back/retry its caller's transaction. Query-level workspace/user lock
  helpers remain narrower helpers, not stand-alone admission gates.
- `service/delivery_evidence.go` retains the original observer's check order,
  reason/verdict precedence, producer/task/runtime/approver/claimant authority,
  database-clock expiry checks, four PR/link table locks and stored-PR provenance.
  After identifier/type renaming, the retained specification and canonical
  receipt validation bodies match the original algorithms.
- `service/delivery_policy.go` preserves virtual revision-0/off policy reads,
  response fields and exact readiness semantics. No mode or schema changes are
  introduced; the handler keeps policy management and transaction ownership.
- Handler translation preserves the established `artifactFailure` HTTP mapping.
  Checked service UUID parsing now refuses malformed internal values instead of
  panicking; real HTTP inputs still pass their unchanged strict decoders.
  Actor validation adds only fail-closed checks for empty user/invalid token ID.
- The controlled-artifact code from `CreateArtifactPackage` onward is unchanged
  from HEAD, including approval, claim, packaging and finalization. The observer
  HTTP decoder/entry/commit body is also unchanged. Private evaluation and hint
  files differ only in supplying the original transaction to policy reads.
- HTTP actor headers remain deny-only for human callers. Task credentials keep
  their existing typed identity. Explicit credential metadata is excluded from
  JSON; no private evaluation namespace, observation or response permission
  changes. RuntimeLookup retains the task attribution and transaction queries.
- `delegationAuthority` continues using the credential-only wrapper. It does not
  call the artifact-specific `Actor()` rejection of restricted task capability.
  The recorded `TestDurableDelegationReporterLocksTargetBeforeScopeRead` includes
  an actual permitted reporter comment and retained receipt before the scope
  race; this protects the important restricted-task compatibility boundary.
- One observer and one receipt validator remain. Thin handler wrappers adapt
  established callers and errors; they do not contain a second algorithm.

## Available evidence independently checked

| Current transaction-only final run | Named pass | Fail / skip | Database |
| --- | ---: | --- | --- |
| handler normal | 339 | 0 / 0 | `enf_p0_final_handler_normal01` |
| handler race | 339 | 0 / 0 | `enf_p0_final_handler_race01` |
| real router normal | 103 | 0 / 0 | `enf_p0_final_router_normal01` |
| real router race | 103 | 0 / 0 | `enf_p0_final_router_race01` |

Handler coverage is the existing 325 plus 14 named durable-delegation results;
normal/race are not summed. One existing strict-body subtest embeds a random
uppercase UUID in its name; that expected name variation is not lost coverage.
Each recorded started test has exactly one terminal, with no duplicate or
unterminated names. Raw log SHA-256, record inventories and counts all match.
All four final runs use different newly allocated database names in the recorded
task container. The reviewer did not connect to any database or inspect/start/
stop a container.

All eight P0 Go files are unchanged within those final runs and match current
bytes. The handler race record's broader source manifest changed only in the
concurrent `packages/core/issues/mutations.test.tsx`; that file is outside this
Go review. This is explicitly distinguished from P0 source stability in JSON.

Also inspected the pre-extraction baseline records (325/103), initial extraction
records (325/103), and build/vet 01/02 with their original raw logs. Earlier
extraction bytes differ from the final transaction-only interface and remain
intermediate evidence. Baseline records identify HEAD but have no per-file
source map; this review does not pretend their records bind current P0 bytes.
Current build02/vet02 both exit 0 and bind the eight reviewed Go files.

## Verification and scope

- Source comparison: passed, including normalized moved-function comparison and
  exact untouched handler sections.
- Go formatting: `gofmt -l` on the eight P0 files returned no files.
- Backend build/vet: current retained build02/vet02 records passed; independently
  rehashed, not rerun by this reviewer.
- Product tests: four final runs above passed according to independently parsed
  raw logs. No product/DB/provider tests were executed by this reviewer.
- TypeScript lint/typecheck and client tests: outside this P0 review; no result
  inferred from concurrent client work.
- Skimmed authorization and batch protocol stage contracts. No additional
  serious safety contradiction was found beyond the header/ordering clarifications
  resolved in the main design. This skim is not P1/P2 code or final protocol
  acceptance; exact issuer/consumer validation, original-operation recovery,
  state/terminal atomicity and real writer tests remain mandatory there.
- Reviewer writes are only this Markdown and its JSON companion. No product
  edits, resource allocations, task activation, commit, push, deployment or
  external message.

Exact source hashes and audited record/log identities are retained in
`p0-independent-review-20261111.json`. Later changes to any of the eight P0 files
require review of that change and appropriate updated evidence.
