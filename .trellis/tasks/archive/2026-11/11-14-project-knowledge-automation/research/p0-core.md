# P0 Core protocol regression baseline

Date: 2026-11-14. Owner: native `trellis-implement` child `p0_core_contracts`.

## Change boundary

The current Core reader only accepts knowledge protocol v1 and requires human
proposal/decision UUIDs. The approved v2 must preserve system provenance, exact
accepted-delivery identity and `approved` history independently of paused usage.
This P0 slice adds tests only in
`packages/core/collaboration/knowledge-automation-schema.test.ts`. Production
schemas, API methods, Views, database and runtime behavior are unchanged.

The v1 material/budget/projection matrix remains canonical in
`knowledge-schema.test.ts`; no UI or runtime matrices are duplicated here. New
fixtures use the shared wire names agreed with the server protocol implementer:

- delivery: `kind`, `issue_id`, `submission_id`, `delivery_event_seq`,
  `accepted_event_seq`, `sha256`;
- public candidate: `proposal_origin`, `decision_origin`, nullable human UUIDs,
  `automation: {job_id, policy_revision, rule_version}`, `usage_status`;
- runtime manifest validity is backend-owned and has no new Core implementation
  in this slice.

The schema assertions expose the intended v2 gap and retain fail-closed v1
boundaries. Public human proposals may cite delivery but cannot select system
identity. These are protocol checks, not AI semantics or product acceptance.

## Baseline

Before adding the new test file:

```sh
pnpm --filter @multica/core exec vitest run collaboration/knowledge-schema.test.ts
```

Result: 1 test file passed, 12 tests passed (Vitest 4.1.0; 2026-11-14 19:11:25
Asia/Shanghai). No database, provider, browser or real-account model call.

## P0 red results and P4 inventory

Command:

```sh
pnpm --filter @multica/core exec vitest run collaboration/knowledge-schema.test.ts collaboration/knowledge-automation-schema.test.ts
```

Result: exit 1; 4 expected v2-positive failures, 21 passes across 25 tests. All
12 pre-existing v1 tests still pass. The new file has 13 cases: four valid-v2
behaviors currently rejected, one v1 representation boundary and eight malformed
v2 cases. Raw output: `research/p0-core-red.log` (2026-11-14 19:12:37
Asia/Shanghai). The target-list positive assertion follows the source-list
assertion in one test and is therefore not reached in this red run; both become
required in the green run. The old type signatures compile; no absent API or
unimplemented symbol is imported to manufacture these failures.

The four expected failures are:

1. System proposal/decision with nullable human actor and automation provenance.
2. Exact accepted-delivery identity in source/target summaries.
3. `state: approved` retained with separate `usage_status: paused_review`.
4. Public human v2 proposal citing exact delivery, with system-identity fields
   still rejected by that public request schema.

Scoped ESLint on the added test file and `pnpm --filter @multica/core typecheck`
both passed (exit 0). No database/provider/browser check is claimed by this slice.

### Core integration inventory

| File | Current behavior and required later change |
| --- | --- |
| `packages/core/collaboration/knowledge-schema.ts` | Frozen v1 source/evidence/proposal/read/receipt schemas; add separate strict v2 shapes and dispatch by declared version. Preserve absent-protocol `null`, malformed-support errors, exact integer strings, private redaction and unavailable/restricted unions. |
| `packages/core/api/client.ts` | `fetchCollaborationKnowledge` centralizes deadline/auth/no-store checks but sends no knowledge negotiation header. Apply v2 negotiation consistently to source/detail/target/write; original-operation recovery must keep the original version. Add policy/job methods through the same protected current-authority boundary. |
| `packages/core/collaboration/knowledge-actions.ts` | `prepareKnowledgeProposal` and `prepareKnowledgeDecision` hardcode 1; intents preserve serialized bytes, while receipt matching currently checks IDs/action only. Store the chosen protocol with the original identity and compare it during receipt recovery; never upgrade existing uncertain request bytes. |
| `packages/core/collaboration/knowledge-recovery.ts` | Stored strict identity contains scope/candidate/request/action only. Legacy records require an explicit v1 interpretation; new identities carry protocol metadata without source, body, policy result or receipt. Keep existing key namespace and exact-request conflict semantics. |
| `packages/core/collaboration/knowledge-queries.ts` and `knowledge-cache.ts` | Keep projections in leased Query data; add policy/job query identities and appropriate invalidation without copying source-bearing data to local state/storage. |
| `packages/core/collaboration/knowledge-sources.ts` | Existing source builder supports memory/comment/run record only. Delivery requires its exact existing retained submission/decision API, not the newest comment or approximate timestamp; the v1 builder must not silently emit a new source shape. |

### Shared Views integration inventory

| File | Required later change |
| --- | --- |
| `packages/views/collaboration/knowledge-review.tsx` | Current `actorLabel` requires a human string and approval rendering is keyed by a non-null human UUID. Render explicit system provenance independently of UUIDs, show rule/accepted source, keep evidence claims attributed, and branch delivery links explicitly. Do not display unrestricted model reasons. |
| `packages/views/collaboration/knowledge-controls.tsx` | `KnowledgeState` only shows candidate state/expiry; add independently visible paused usage. `KnowledgeSourceLabel` currently assumes every non-run source has `version`, so it needs a delivery branch. |
| `packages/views/collaboration/delivery-panel.tsx` | Existing `#delivery` section is the accepted-result surface. Add job outcome/pending/configuration status adjacent to actual submission/acceptance; no-project/manual must not show fabricated running work. |
| `packages/views/projects/components/project-detail.tsx` | Already mounts `ManagedKnowledgePanel` with explicit workspace/project IDs. Add the authorized default/override policy control here using shared Core hooks; no new platform page. |
| `packages/views/collaboration/knowledge-panel.tsx` | Preserve leased projections, original-key recovery and current-authority controls while adding automation exception entry points. |

Existing source links use `accountPath`; the delivery panel already provides
`id="delivery"`, avoiding a new global route. Web and Desktop consume these
shared components; actual platform journeys remain a later P6 requirement.

### Follow-ups and limitations

- Final public `usage_status` vocabulary and public-policy/job DTOs are still
  implementation coordination details; the approved `paused_review` behavior
  and agreed delivery/provenance names are covered here.
- This slice does not verify server authorization, full-source comparison
  privacy, runtime validity, database races, LLM semantics or UI operation.
- The new tests deliberately remain red until the P1/P4 reader implementation.
- Two read-only inventory commands initially included guessed missing view paths;
  an `rg --files` lookup corrected the paths. No edit or execution depended on
  the missing files.
