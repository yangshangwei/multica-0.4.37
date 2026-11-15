# P2 policy, accepted jobs and recovery

Implementation under approved plan2026-11-14-v1. The background runner is not
activated by this lane; main owns router/startup wiring and the default-use gate.

## Implemented contracts

- `collaboration_knowledge_policy.go`: absent override resolves to revision0,
  system-enabled, max3, rule `project-knowledge-v1`; no fictitious human row.
  Current human project lead or workspace owner/admin may read/change the
  policy. Actual account/credential/member/project authority is checked under
  existing locks. A project-scoped advisory fence covers a concurrently inserted
  first override, so missing-row default reads cannot race a disable through
  settlement. The fence is acquired before target/source locks.
- Policy updates use revision CAS and the retained `collaboration_operation`
  project namespace. Original actor recovery survives subsequent changes and
  role/project loss; it grants no current read or write authority.
- `collaboration_lifecycle.go`: after saving the actual `completed` event, only
  an exact accepted delivery, current project, assisted policy and enabled
  project policy produce a durable job in the same business transaction. The
  source's real acceptance actor, submission, delivery/accepted sequence and
  actual closed lifecycle are retained. No model call, target knowledge lock or
  business queue restart occurs in acceptance. Reopen only adds the existing
  source invalidation scan; consumption's synchronous fence is source-owner work.
- `collaboration_knowledge_automation_jobs.go`: bounded current-source-authorized
  status and original-key explicit retry. Retry additionally requires current
  target confirmation authority. Public status contains fixed reason codes and
  provider-returned model/usage metadata, never model free-text reasons, outputs
  or input closures. Usage queries load metadata only from the current execution.
- Policy and job GETs use the existing `KnowledgeProjection` fields, first-read
  server timestamps, current credential expiry clamp and end-of-read validation.
  They do not introduce an independent client lease. Receipts remain content-free.

## Per-execution policy binding

Migration860 preserves the immutable original acceptance-policy fields and adds
`execution_policy_revision`, `execution_rule_version` and
`execution_max_candidates`. A deterministic same-value backfill runs under the
migration's exclusive table lock. Only an explicitly authorized retry revision
can change these execution fields; model attempts and evaluations freeze the
actual execution policy. This lets a human explicitly recheck after policy
changes without inventing a new accepted job or rewriting old attempts.

Runtime validity must bind immutable evaluation/check-attempt policy facts,
not the job's current execution fields: a later retry or disabled policy cannot
invalidate earlier lawful approved history. The source/runtime owner integrated
that distinction. The original844 downgrade guard remains reviewer-owned.

## Public service interfaces

- `GetProjectKnowledgePolicy(ctx, projectID, workspaceID)`
- `UpdateProjectKnowledgePolicy(ctx, projectID, workspaceID, ProjectKnowledgePolicyRequest)`
- `GetProjectKnowledgePolicyOperation(ctx, projectID, requestID)`
- `ListKnowledgeAutomationJobs(ctx, issueID)`
- `RetryKnowledgeAutomationJob(ctx, issueID, jobID, KnowledgeAutomationRetryRequest)`
- `GetKnowledgeAutomationRetryOperation(ctx, issueID, jobID, requestID)`

Policy view: projection fields plus project/workspace IDs, revision, enabled,
max_candidates, rule_version, system/human origin, nullable changed_by/updated_at,
can_manage. Job view: projection fields plus issue/items/has_more; each item has
exact acceptance identity, execution revision, phase/fixed reason, source_current,
can_retry, truthful saved/restored/revised/pending/skipped counts and bounded
actual model/usage statuses. A failed job without any candidate does not invent
a pending-candidate count.

## Evidence

- `p2-policy-intent-7ad2e23e.json`: compile-only failure while another lane's model
  parser file was incomplete; zero tests ran. Preserved, not counted as behavior.
- `p2-policy-intent-replay-87f7da59.json`:6/6 named tests passed, zero failures/skips,
  stable source fingerprint and zero owned connections. Covers system defaults,
  owner/current lead/member boundaries, CAS, old-request recovery after later
  updates and role loss, unique formal acceptance intent, zero jobs before
  acceptance or for no-project/manual/disabled tasks, and retained closed queues.

No real model, personal CLI/account, deployment, commit or push occurred.
