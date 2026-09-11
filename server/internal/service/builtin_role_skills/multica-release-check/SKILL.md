---
name: multica-release-check
description: "Use when preparing a release or a high-risk action: choose the applicable checks, prepare recovery, and obtain human approval before execution."
user-invocable: false
---

# Release checks

## When to use

When preparing a release or a high-risk action: deploy, migration against live
data, credential read, package publish, external announcement, or anything
destructive. The checks depend on the action; every high-risk action still needs
an Operator and its own recorded human approval.

## The release gate

For an actual release, establish and record the applicable items before asking
for approval:

1. **Scope** — the exact commit range, version and artifacts. Not "latest".
2. **Build** — the project's build command, run, with its result.
3. **Tests** — the suites the project gates on, run, with real counts.
4. **Migrations** — every migration in the range, and for each: is it reversible,
   does it lock a table, and does it depend on a deploy ordering.
5. **Irreversibility** — list every step that cannot be undone. This list decides
   whether the release is ready, not the test result.
6. **Rollback** — the exact sequence that returns to the current state, and the
   point past which it stops working.

Mark an item `N/A` only when it does not apply, and explain why — for example,
"Migrations: N/A — no schema or data changes in this release." An applicable
check you did not verify is a gate failure. Missing access or an unavailable test
environment does not make a check inapplicable.

## Checks for a standalone high-risk action

For an action outside a release, verify its exact target, scope, expected effect,
how the result will be checked, and its recovery or irreversibility. Use the
checks that establish readiness for that action:

- **Credential read** — establish which credential is needed, why, who may receive
  it, and how it will be kept out of logs and shared output. Do not read the
  credential while preparing the request.
- **External announcement** — prepare the exact content, recipients or channel,
  timing, and how delivery will be confirmed or corrected.
- **Live migration or destructive operation** — identify the affected data or
  resources, validate the commands in an appropriate test environment, and verify
  backups, recovery limits and any ordering constraints that apply.

A standalone credential read or announcement does not need an unrelated build,
test suite, commit range or migration review. If using the release checklist to
record it, mark those items `N/A` with reasons. Applicable checks still have to
pass before requesting execution approval.

## The approval boundary

Only an Operator may execute a high-risk action. As an Operator, file one approval
request for each action describing that action, then stop at its execution
boundary and wait:

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

Before approval, complete permitted preparation: read-only inspection, builds,
tests and reversible local work. This does not authorize a credential read or
another high-risk action needed by those checks; request approval for that action
separately. Without an approved request, deliver the plan and preparation results,
and leave the high-risk action unexecuted. Other roles hand the plan to a human
or an Operator.

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

Follow the output format in your instructions: scope, applicable checks and any
`N/A` reasons, ordered plan with reversibility per step, recovery limits, and the
approvals needed. After execution, list what ran and what did not, with reasons.

## Stop and ask a human when

- The gate fails or a migration cannot be rolled back.
- An approved action produced an unexpected result.
- You lack a credential, environment or permission — ask; do not route around it.
