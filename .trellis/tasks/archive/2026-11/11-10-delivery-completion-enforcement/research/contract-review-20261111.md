# Independent contract review: P0 implementation readiness

Date: 2026-11-11, Asia/Shanghai. Reviewer: native `trellis-check` /
`check_resume_evidence`. Source HEAD: `1ce4e81223be3397fe15e2649debfac40720ddd9`.

## Verdict

**PASS_P0_IMPLEMENTATION_READY.** No blocking contract finding remains for the
behavior-preserving extraction of the existing observer and artifact-authority
core into service. The main session may activate the existing task, establish
the new owned baseline environment and dispatch P0 under the recorded scope.
No further user permission is required: explicit D1–D5 answers and the subsequent
recommendation delegation are recorded in the decision register.

This is readiness to implement P0, not a claim that P0, the enforcement task,
M1, M2/M3 or the six businesses are complete. Product implementation had not
started at review time. No enforce or grant capability is approved for premature
exposure by this verdict; P1–P4 retain their technical and acceptance gates.

## Findings (fixed by main session and rechecked)

- `implement.jsonl` and `check.jsonl` still described the decision register as
  unresolved. Main updated both reasons to the explicit confirmed/delegated
  contracts. Both still contain 15 existing, real context targets.
- P0 ownership omitted `handler/delivery_policy.go`, although it calls the
  shared artifact credential/fence/user helpers. Main added call-adapter-only
  ownership, explicitly keeping policy modes and schema unchanged.

The reviewer changed no main-owned planning document or product file. This
report is the only reviewer-owned addition.

## Findings (not fixed)

None blocking P0. Later exact grant DTOs, public error fields, SQL/index shapes,
typed integration proofs and Prepare/Finish interfaces are explicitly frozen
before their owning implementation stages. These are stage obligations, not
unanswered user choices and not permission requests.

The old research and per-answer history deliberately retain earlier pending
states. The current PRD/design/implementation plan and the final decision-register
section identify the later explicit authority and take precedence for this work.

## Contract assessment

- **P0 reuse is practical and bounded.** The existing observer already accepts
  the original transaction; its coupling is to HTTP/principal extraction and
  Handler metrics. Service already owns RuntimeLookup. Moving this one core
  there avoids a handler reverse dependency and a second observer. HTTP retains
  strict decoding, aliases, actual header denial and response/commit behavior;
  real principal/session/task-token values remain distinct from any future
  retained issuer facts. Metrics attribution and current credential checks must
  remain identical. P0 adds no grant, mode, migration or writer interception.
- **Behavior protection precedes movement.** The plan requires the existing real
  artifact/observer/evaluation/hint regressions on fresh owned baseline databases
  before edits, then normal/race checks on separate new databases. Preserve exact
  private evaluator lookup, immutable observations, schema readiness, DB time,
  lock graph, PR/link locks, actual tar bytes/SHA and failed-commit responses.
  This proves extraction compatibility, not new enforcement.
- **D1/D5 preserve distinct authority.** New ordinary-task delegation is explicit
  and current-authority bounded. Collaboration still requires its initiator;
  R38 keeps the real platform administrator. The explicit real ordinary child
  source has same parent/workspace/project and exact versions, while the artifact
  remains the child's. It does not fabricate an ordinary queue task for a shared
  run or expose source private evaluation/content to an outside consumer.
- **D2 does not manufacture a safe skip.** The current fixed observer has only
  hard identity, success, integrity, source and expiry checks. The delegated
  recommendation to expose an empty waivable list and reject skip is consistent
  with those constraints. D2 governance remains a future prerequisite; this
  version must not claim usable skip or full M1 delivery.
- **D3/D4 are coherent.** Creation resolves inherited project before rejecting
  enforce done. New completion is a non-done to done-category transition; later
  non-project edits retain only the frozen historical acceptance. Actual project
  migration still applies source and target policies, human owner/admin exit
  authority, reason/retention and separate reopen/migration/completion. Project
  deletion rechecks policy and retains the collaboration deletion guard.
- **D6 covers the actual consumers.** Ordered per-item results and non-2xx partial
  errors preserve committed prefixes; unknown is 503, not rejected. The plan
  includes required core parsing/cache and shared views feedback repairs,
  committed-prefix parent notifications and legacy count handling. There is no
  whole-batch atomicity, ledger, automatic retry or invented no-key recovery.
- **P1–P4 retain the final transaction contract.** Prepare does not write business
  state; Finish consumes under the same transaction and rechecks expiry/actual
  result. Self-result recording follows gate preparation; a later Finish failure
  rolls back rather than committing a normal refusal. Original-key recovery
  precedes fresh completion/done-edit paths. The VCS integration trust revision
  and real signature proof remain mandatory, distinct from system attribution.
- **Enforce remains last.** The public option opens only after every actual
  writer, project migration/deletion path, custom done and concurrency boundary
  has real acceptance evidence. Missing schema/dependency/unknown mode cannot
  fall back to off. Mixed old binaries are explicitly unsafe; policy rollback
  preserves new history. P0 readiness does not relax these restrictions.
- **Ownership is adequate.** Core extraction, ledger/schema, ordinary writers,
  clients, collaboration and VCS are separated by stage/file ownership. Shared
  types/generated SQL and route integration have a single coordinated owner;
  independent checks pause conflicting edits.

## Verification and limits

- Read the fresh PRD, design, implementation plan, task metadata, both context
  manifests, complete decision register and both 20261111 research reports.
  Loaded the context specifications and inspected the current authority,
  observer, policy and RuntimeLookup seams supporting P0.
- Task/JSONL parsing, context target existence, UTF-8, final LF and trailing
  whitespace checks passed. Both manifests contain 15 real entries. Tracked
  `git diff --check` passed; the two reported document corrections were reread.
- Product diff under `server`, `apps`, `packages`, `scripts` is empty at review
  time. No baseline or new acceptance was run by this reviewer.
- Lint/typecheck/product tests: **not run, not applicable to this planning-only
  check**. Their relevant commands and actual regression evidence remain P0
  implementation obligations; document checks are not product-test evidence.
- No DB/container/provider calls, task activation, runtime mutation, commit,
  integration, push, deployment or external message by this reviewer.
- A nonessential read search used absent future `service/delivery*.go` and
  `service/artifact*.go` globs; zsh reported no match. This was a read-only lookup
  failure, not product red. Existing source inspection and contract checks did
  not depend on that search; no state or historical evidence was changed.

The main session owns activation and baseline source freezing before the
implementer edits. P1–P4 exact interfaces and tests must remain in their owning
stage artifacts, without turning this P0 verdict into a full implementation or
completion claim.
