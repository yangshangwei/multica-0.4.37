# Design

## Change boundary
The behavior gap spans copied workflow instructions, restricted reviewer result return, role/skill stop conditions and template upgrade documentation. Scope is the existing ordinary issue/squad workflow; account collaboration and child-stage acceptance are unchanged.

## Workflow
Upgrade the security squad and lead defaults to v3. Retain G1/G2 as automatic evidence checkpoints, not human gates. Leader selects the reproducer and next step from actual roster and complete result evidence. QA may produce local security regression fixtures before the separate implementer repairs; require independent QA and code/security review afterward. Limit retries and expose unresolved evidence explicitly.

## Backend
Restricted comments remain dispatch-ineligible; never remove suppression or give reviewer tokens new APIs. Add a platform-owned return to the original authorized security coordinator for genuine delegated outcomes, using persisted source/delegation identities and existing task enqueue/auth/deduplication patterns. Recheck current issue/squad/leader/originator authority. Scope automatic return to genuine built-in security squad/leader delegation and prevent stale/unrelated/duplicate completion from advancing. A return requests coordinator evaluation, never acceptance; failed/missing results cannot pass a checkpoint. Integration: use the existing task-service delegated failure return outbox, terminal task row lock, source-task coverage, bounded attempts and recovery sweep. Add completed reviewer outcomes as platform-owned system/progress signals; the inert original review is never reused as an executable trigger. Validate exact immutable squad/leader template keys, original delegation lineage, current assignment/roster/leader and originator/runtime invoke authority. Do not coalesce signals across unrelated contexts. Guest squad outcomes remain comment-only.

Creation provenance versions are not the saved instruction version. Do not require >=3 provenance for lifecycle return: a correctly delegated old security squad may receive a platform return while its v2 prompt still waits at its human gate. An explicit update of both saved instruction bodies can then enable autonomous workflow without rewriting provenance. Other squad templates and generic restricted role comments retain existing behavior. This preserves a usable upgrade path without new API/schema or silently replacing copies.

## Role and skill consistency
Update shared Diagnostician/QA and debugging/test-report instructions with an explicit exception for already-authorized local nonproduction security repair. No new permission granted by a skill; stop if required inputs/access exceed the boundary. Sync template/skill versions and related contracts. Keep owner and guest status rules distinct.

## Compatibility and saved instances
Templates are copied defaults, not live inheritance. Preserve saved/custom instances and existing API capability profiles. Document updating both squad and lead instructions and affected reused roles/skills; version identifiers alone must never silently overwrite user content. Inspect the requested instance only if the actual API/context can be identified safely; do not claim it upgraded unless verified.

## Validation
Meaningful backend integration tests exercise actual persisted results, permissions, delegation identity and queue behavior; scripted flow tests distinguish plumbing from real model behavior. Existing capability denial tests remain intact. Compile/vet and scoped unit/integration tests run with task-owned per-package databases, no real provider CLI invocations. Documentation bundle and affected frontend catalog tests validate displayed changes.
