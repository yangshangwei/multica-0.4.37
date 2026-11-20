# S3 recovery follow-up: independent contract review

- Reviewer: `/root/s3_contract_check`.
- Explicit task: `.trellis/tasks/11-13-pay-241-s3-recovery`.
- Scope: preparation and public-contract review only. The coordinator continues S2; the shared server writer remains with its existing owner.
- Initial decision: **CHANGES REQUESTED for the v1 blocking contract.** The six composed recovery-test topics are suitable preparation, subject to rebinding to the accepted S2 candidate. No product defect or test failure is claimed by this review.
- The v1 research and its original evidence remain unchanged. A later decision on the author's v2 supplement will be appended below.

## Findings (fixed)

No product, test, specification, task-state, or existing planning file was changed. This reviewer owns only this report and its adjacent JSON record.

## Initial findings (not fixed in v1)

### S3P-C01 — blocked repair does not yet have a usable return path

Severity: **P1 planning blocker**. Source inspection, not an executed regression.

The v1 proposal requires a managed return into `blocked` to reactivate an exact complete open parent. It also rejects an ordinary managed blocked transfer and a generic status change that would pretend to resolve an active request. Its suggested repair is a new complete information request, but the existing writer saves the actual prior status and cannot manufacture the parent that a later return requires:

- `RequestInformation` in `server/internal/service/collaboration_handoff.go` only keeps a parent when the current handler is also the current open request's recipient. A creator replacing somebody else's request supersedes the chain and starts with no parent.
- `collaborationWorkflowTx.handoff` in `server/internal/service/collaboration_workflow.go` writes `PreviousStatus: before.Status`.
- `depart` can call `changeHandler`, which supersedes the information chain while leaving the issue status unchanged. This security removal must remain possible.
- `collaborationReturnStatus` currently accepts a selectable saved `blocked` key. The proposed parent-completeness restriction would refuse that return when the new request has no valid parent.

The contract must close all five sequences:

1. Complete managed request B1 → creator replaces the current recipient's request with B2.
2. B1 → current recipient is removed → chain is superseded while the root stays blocked → new B2.
3. Legacy standalone blocked root or incomplete old chain → stage policy becomes applicable → new B2.
4. A recipient nests complete B2 under an incomplete or no-longer-returnable legacy parent.
5. Original requester leaves or is disabled → return refuses → the proposed ordinary-transfer restriction also prevents the old recovery route.

Creating B2 in these states can leave `previous_status=blocked` with no usable return parent. Merely labeling the state `recovery_required` does not supply the missing transition. A full cancel/reopen is not silently substituted for ordinary information recovery.

Required correction: preserve every actual old status and handoff; define an explicit currently authorized repair with an original request key, current observations, an auditable restore source or an explicit human choice, and an actual committed transition out of blocked before a new root request is created. Never insert an invented historic nonblocked status into `previous_status`, silently reenroll an unavailable requester, or weaken a permission fence.

The existing pending-delivery case is also part of this correction: `collaboration_delivery_test.go` preserves the exact submission through information request/return and returns to `in_review`. Repair must not use a free human-selected target/status to bypass that pending submission, its creator, or accepted S2/M1 completion bindings.

### S3P-C02 — public read and runtime choices must be frozen before parallel implementation

Severity: **P2 contract blocker**. The v1 report explicitly leaves these as research recommendations; they are not a shipped behavior defect.

The implementation handoff needs one concrete choice for request presence rules, finite text bounds, timestamp precision, current/history shape, current applicability and runtime exposure. In particular:

- A nullable Go pointer alone collapses explicit JSON `null` into absence. New fields need boundary presence validation before old nil/`omitempty` serialization can preserve old hashes.
- PostgreSQL timestamps have microsecond precision. Validate the new estimate against the final transaction clock using the value that will actually be stored; preserve the original request string for its request identity.
- A complete historical record is different from an actionable current request. Old-server unknown metadata, a legacy row, malformed/unavailable metadata, an unavailable recipient and an unavailable restore target need distinct safe behavior.
- Creation options are currently workspace/search scoped. A managed-project restriction cannot silently remove blocked creation for all unmanaged projects; freeze how selected project context reaches the options request and final writer.
- Runtime handoff v1 recursively rejects unknown typed fields and validates `protocol_version == 1`. Its source text is the note; frozen snapshots strip content and later rehydrate using source kind/ID/version/hash. An unversioned extra text field or concatenation changes that contract. The bounded recommendation is to retain v1 and its note/source bytes, expose new structured facts to authorized human current/history readers, and make missing runtime metadata explicit. A new runtime version would require an explicit negotiated reader/source/budget/retention contract, not an implementation-time guess.

## Preparation assessment

The other six-topic recommendations match the inspected code boundaries:

- Upload metadata remains current-authority content; original upload keys and file fingerprints survive uncertainty. Exact runtime outcome ACKs and separate comment/attachment writes have distinct identities.
- Shared comment paging uses `limit=50,max_bytes=65536`, which selects `RecallComments`. The 55-comment denominator must include tombstones and complete cursor chains; the byte budget can require more than two pages. Real reconnect authentication is separate from GET convergence.
- Priority/handler races must preserve the current-authority-first outcome: a removed handler can lose with 403, while an authorized stale revision loses with 409. Ordinary child CAS remains explicitly optional for old callers.
- Typed mentions deduplicate people and Agent targets independently. Original notification recipients are frozen in the original transaction; `@all` edits do not expand old mentions to later joiners.
- Removal of one read basis is different from loss of every basis. Historical content-free own receipts do not authorize private record, file, runtime-source or knowledge reads.
- The archived timeout-proof is reused under its original source and execution limits. It does not prove Agent output upload or real model behavior, and no such new capability is inferred here.

No additional business task, parallel recovery system, new dependency, public fixture endpoint, account call, payment operation, or production deployment is needed for this preparation.

## Verification

- The full saved hook file was inspected for its scope; it injects adjacent S2 context. The explicit S3 dispatch is authoritative. S3 `check.jsonl` is empty; its PRD, design, implementation plan, v1 research and archived timeout reports were loaded directly.
- The v1 Markdown SHA matches its JSON binding. Its 147 final source entries were compared against the workspace: no missing path or byte drift at this review's initial audit.
- Actual source paths were inspected for information request/return, ordinary handler transfer/removal, lifecycle, creation, R38, request serialization, strict runtime snapshot/source decoding, comments, notifications and attachment boundaries. The 147 hashes do not claim a compiled or independently executed full source graph.
- Product lint: **NOT_RUN**, explicitly outside this research-only assignment.
- Product type-check: **NOT_RUN**, explicitly outside this assignment.
- Product tests, database/API/browser/Provider execution: **NOT_RUN / 0**.
- Parent matrix and all S2/S3 task statuses remain unchanged. Existing F2 failures, the unattributed S2 `55P03`, real S4 conditions and final-candidate S5 remain separate.

## V2 follow-up

Pending the author's bounded supplement and a fresh contract/source audit. This initial review remains part of the final report even if the identified planning gaps are closed.
