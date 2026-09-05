---
name: multica-release-check
description: "Use before any release or production-affecting action: the gate, the rollback, and the approval request that must precede execution."
user-invocable: false
---

# Release checks

## When to use

Before proposing a release, and before any action that reaches an environment you
cannot reset: deploy, migration against live data, credential read, package
publish, external announcement, or anything destructive.

## The gate

Establish and record each of these before proposing anything:

1. **Scope** — the exact commit range, version and artifacts. Not "latest".
2. **Build** — the project's build command, run, with its result.
3. **Tests** — the suites the project gates on, run, with real counts.
4. **Migrations** — every migration in the range, and for each: is it reversible,
   does it lock a table, and does it depend on a deploy ordering.
5. **Irreversibility** — list every step that cannot be undone. This list decides
   whether the release is ready, not the test result.
6. **Rollback** — the exact sequence that returns to the current state, and the
   point past which it stops working.

A gate item you did not run is a gate failure, not a blank.

## The approval boundary

For each action that touches production, file one approval request describing that
one action, then stop and wait:

```bash
multica approval request \
  --risk-class production_release \
  --summary "Deploy v1.2.3 to production" \
  --plan-file ./release-plan.md \
  --issue <issue-id> --output json
multica approval get <approval-id> --output json   # has a person decided yet?
```

Risk classes: `production_release`, `database_migration`, `secret_access`,
`external_notification`, `destructive_operation`. Use `--plan-file` for anything
longer than a line — the plan is what the reviewer actually reads.

- One request per action. An approval for a migration is not an approval to deploy.
- The request must name the risk class, the exact commands, and the rollback.
- Never proceed on a comment that sounds like agreement. Only a recorded human
  decision counts.
- Never approve your own request.
- If an approved action fails partway, stop and report. Do not improvise against
  production.

Without an approved request, the deliverable is the plan.

## Recording execution

After an approved action, record what you ran, what happened, and whether it
matched the expectation — including partial success:

```bash
multica approval executed <approval-id> --note "deployed; health checks green"
```

That call is rejected unless the request is approved and your autonomy level is
`operator`, so it cannot be used to walk an unapproved action forward. An action
whose result you did not verify is not done.

If the plan changes before a decision arrives, withdraw it rather than leaving a
stale request in the queue: `multica approval cancel <approval-id>`.

## Output

Follow the output format in your instructions: scope, gate, ordered plan with
reversibility per step, rollback, and the approvals needed. After execution, list
what ran and what did not, with reasons.

## Stop and ask a human when

- The gate fails or a migration cannot be rolled back.
- An approved action produced an unexpected result.
- You lack a credential, environment or permission — ask; do not route around it.
