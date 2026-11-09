# M1 controlled artifact execution

## Goal
Provide one narrowly scoped controlled high-risk executor for packaging a release candidate artifact. The server owns an approval-bound, one-time execution lease and receipt; a local synthetic adapter performs only deterministic packaging in an owned directory. This closes the difference between an approval row and proof that a specific approved action ran.

## User-confirmed boundary
This task follows the recommended first adapter: release-candidate artifact packaging. No deployment platform, production environment, remote push, publication, paid model or external notification is included. Existing human approval and agent identity remain authoritative.

## Acceptance criteria
- A canonical typed spec and pending approval freeze actor, resource, action, source revision, trusted staging manifest/input digest, target environment, adapter policy version and expiry in one transaction before human decision. Missing/unknown fields fail closed.
- Only a human with existing authority may approve. Approval cannot be reused, changed or transferred to another actor/resource/action/commit/artifact/environment/argv.
- Claim is atomic and one-time. Concurrent claims yield one execution row; expired/revoked/consumed permits cannot execute. Claim records unknown before the side effect and preserves one execution ID; legacy approval status is not marked executed early.
- Synthetic pack adapter validates exact operator-staged input files/digests and writes only an owned output directory; absent operator root disables it. It cannot push, deploy, read production credentials or execute arbitrary argv. Output digest is recorded in the receipt.
- Result states distinguish `succeeded`, `failed`, `unknown`; timeout/adapter crash leaves `unknown` and never auto-retries a side effect or permits a second run. Replaying the same request returns the existing receipt or a conflict; it never runs twice.
- Approval identity, claim, adapter result and receipt are queryable by authorized human/auditor; agent cannot approve or claim a different approval. Existing autonomy/operator gates remain an upper bound.
- Negative tests cover actor/resource/commit/artifact/env/argv mutation, expiry, revoke, concurrent claim, duplicate request, input drift, output collision, adapter failure/unknown, human approval control, agent denial and legacy approvals untouched.
- Parent remains in_progress; this child is not an M1 completion claim.

## Review-enforced constraints

The canonical typed package spec and approval are created in one transaction before a human decision; generic legacy approvals cannot gain controlled scope later. An execution receipt is separate from legacy approval status, is claimed once, writes unknown before side effects, and never auto-retries unknown/failed results. The local adapter is disabled without an operator staging root and rejects symlinks, traversal, special files, collisions and caller argv. Commit is a declared revision backed by a trusted staging manifest digest.

## Non-goals
No general shell sandbox, deployment adapter, artifact registry, database cleanup policy, budget system, M2 snapshots, M3 evaluation or remote CI enforcement.
