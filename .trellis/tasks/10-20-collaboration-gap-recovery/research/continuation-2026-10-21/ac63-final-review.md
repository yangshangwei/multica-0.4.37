# AC63 deletion preservation — independent final review

Reviewed 2026-10-21. Verdict: **approve the full AC63 backend-primary criterion** based on the expanded canonical refusal test and both ordinary deletion controls in the same current guarded race run. No unresolved product defect or additional test requirement was found for this clause. Root owns matrix/catalog/spec updates and any Lore commit; this reviewer changed only this report.

AC63 (R37, primary owner P2) requires source deletion refusal to preserve workspace, shared issue, comments, attachments and permission relations before any partial cleanup; workspaces with no collaboration records must retain ordinary deletion behavior. The expanded test directly covers the missing attachment/permission preservation evidence in the previous ledger.

## Current code and owned scope

Canonical function: `server/internal/handler/collaboration_boundary_test.go:132`, `TestWorkspaceWithCollaborationHistoryCannotBeDeleted`. Current source SHA-256 `6d5f6b6b7b111009d653b974a619b17c7315487bbaf0d143ba9daffadea15ebd` matches the expanded race run before/after hash. All six source hashes in `ac63-deletion-expanded-race-first-20261021-result.json` still match at review.

The reviewer reconstructed the original file by replacing only this function with `function_before` from `ac63-deletion-baseline-20261021.json`. Its full-source SHA-256 matches the baseline `c34b1929285db1c68d9a96b6544128a3485e7e45015f0c7d5692fb8a5abbddf0`. The reconstructed unified diff is exactly equal to `ac63-deletion-owned-20261021.patch`, and every byte outside the function matches the retained SHA-256 `1bd70f71860656a85a2aec2c4e85da52fae2069abca4b923a670580d096756b2`. Imports, other tests and production code were not changed by this slice. This verification is stronger than assuming the whole dirty-file diff belongs to the slice.

## Contract review

The refusal is exercised for all three canonical statuses: `todo`, `done`, `cancelled`. Terminal statuses are deliberately persisted prerequisites; this does not claim a lifecycle UI journey.

Each case has a source owner and distinct external participant/resource owner. The latter owns a separate private execution workspace, runtime, agent and enrolled resource identity. Fixture assertions prove both intended owner memberships, exactly two active issue participants and zero cross-memberships. The organizer is not made a member of the private execution workspace.

The actual existing handlers create the resource offer, adoption and owner consent. The resulting grant is correlated to issue, target, external owner, execution workspace, agent, runtime and enrollment binding, and must be unrevoked before deletion. This is actual persisted consent/permission data, not an empty grant snapshot. Enrollment itself is a test-created persisted prerequisite with a synthetic key; no provider or managed-installation kernel execution is claimed or needed for deletion preservation.

`collaborationUploadedFile` calls the real attachment upload service using test-local storage, yielding encrypted persisted bytes; it does not stub storage or insert a fake attachment. The actual comment handler delivers the attachment. Assertions require encrypted delivered attachment metadata and exactly the active issue/comment references for the returned comment. Both source-member and external-participant callers read exact `controlled private bytes` through the protected content handler before deletion and again after refusal. Their request helper deliberately supplies an invalid ambient workspace header, so the account/source read bases are exercised independently of incidental workspace routing.

The snapshot captures complete source/execution workspace, issue, collaboration and session rows plus **15 nonempty relationship arrays**:

- accounts, members, participants and account fences;
- targets, grants, enrollment identities, agents, runtimes and offers;
- encrypted attachments, attachment references, comments, events and original operations.

The nonempty-array checks run before DELETE; therefore null/empty snapshots cannot falsely satisfy the preservation check. Arrays use deterministic primary-key/event order, and full `to_jsonb` rows retain all stored fields. After actual DELETE returns HTTP 409 `workspace_has_collaboration_records`, the entire ordered snapshot must be byte-for-byte equal. If a grant or attachment were removed, the corresponding child snapshot cannot silently vanish: its parent array is also retained in the equality check. Post-refusal byte reads additionally verify storage and current read authority.

Production `DeleteWorkspace` checks current source owner, takes the workspace transaction lock and queries `WorkspaceHasCollaborationRecords` before chat/object reads or any set-based relationship teardown. The guard covers all stored collaboration records regardless of terminal status. Earlier membership-cache invalidation is reversible cache eviction, not persisted permission removal; fresh protected reads still succeed. No cleanup is committed on refusal.

The same run passes `TestDeleteWorkspace_OwnerSucceeds`: with no shared records, the actual handler returns 204, deletes the workspace and validates normal relation/object cleanup. `TestDeleteWorkspace_PreservesOtherWorkspaceData` verifies a neighbor's workspace, issue, comment, inbox, runtime profile, usage and pending object rows stay intact. Those existing controls were read and their successful current events verified; no assertions were weakened or duplicated.

## Execution and fresh checks

The expanded first run used task-owned `service_s5`, PostgreSQL 17.11, schema through migration `737_collaboration_runtime_preparation`, with `agent_runtime.collaboration_capabilities` present. The retained database provenance is `ac63-deletion-database-20261021.json`. Credentials were not read or printed by this reviewer.

```sh
# From server/, executed by the preceding implementation lane:
GOTOOLCHAIN=go1.26.6 python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/with-db.py service_s5 \
  bash ../scripts/go-test-with-agent-cli-guard.sh \
  go test -json -race ./internal/handler \
  -run '^(TestWorkspaceWithCollaborationHistoryCannotBeDeleted|TestDeleteWorkspace_OwnerSucceeds|TestDeleteWorkspace_PreservesOtherWorkspaceData)$' \
  -p 1 -parallel 1 -count=1 -timeout=30m
```

Raw `ac63-deletion-expanded-race-first-20261021.jsonl` SHA-256 `df5c5b4231ca8ccce15a1e69df19cb6a0a76b29c1b0fd0a7d7434e65d9f612fe` reports **3 top-level passes / 6 including subtests / 0 failures / 0 skips**, package 2.956 s, exit 0, start `2026-10-21T09:07:23.567251Z`. The reviewer parsed actual Go events and confirmed all identities, rather than relying only on result counts. The narrower baseline also passed the same selector; its earlier smaller snapshot is retained as baseline, not complete attachment evidence. No red result or retry occurred in this slice.

Fresh reviewer commands:

```sh
# Repository root:
gofmt -d server/internal/handler/collaboration_boundary_test.go
# server/:
GOTOOLCHAIN=go1.26.6 go vet -p 1 ./internal/handler
```

Both exit 0 with empty output; fresh vet completed in 5.33 s. There is no TypeScript change, so TypeScript lint/typecheck is not applicable to this backend test-only increment. The source/patch/hash and artifact checks described above also pass. No product suite, browser/native run, API restart, environment mutation or full `make check` was repeated.

## Findings and remaining boundaries

No issues were fixed, and no unresolved issue prevents the full AC63 promotion. The previous matrix gap (explicit attachments/permissions plus ordinary control in the same current run) is now covered. Root should preserve the task's protected criterion/owner definitions and update only status/evidence/assessment fields through the audit lane.

The encrypted blob is test-local storage; external S3 fault behavior is untested. Terminal lifecycle UI, packaged clients, full Main startup, real providers/account quota and other-OS containment remain separate acceptance scopes. This report neither enables production collaboration gates nor authorizes a rollout, task/parent archive or full 97-item completion.

The owned patch is suitable input for root's slice-specific staging review, but the broader working file contains other shared changes. Do not stage unrelated hunks or untracked feature dependencies as an AC63 test-only commit. Any Lore `Tested` trail should cite the schema-737 guarded race and fresh vet/gofmt, with the local storage and UI/runtime limits recorded where relevant.

Changed file in this review: `research/continuation-2026-10-21/ac63-final-review.md` only. No production simplification, dependency, abstraction or code edit was made.
