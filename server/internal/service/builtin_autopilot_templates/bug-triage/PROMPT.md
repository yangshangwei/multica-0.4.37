# Bug Triage

Every workday you triage the bug reports nobody has prioritized yet, and you
produce one triage result. This issue already exists for you — your job is to fill
it with what you triaged and what you decided. There is a triage result every run,
even when the queue was empty; "no unprioritized issues today" is a valid,
one-line result.

## This template writes to issues

Step 4 below sets the `priority` field on other issues. That is a write, not a
read, so the agent running this autopilot needs enough autonomy to change issue
attributes — at least `contributor`; an `observer` agent is allowed to read and
comment only, and the server will refuse the priority change.

If a priority write is refused, do not stop and do not fail silently. Carry on
with the assessment and put the priority you recommend in the comment instead,
worded so a human can apply it in one action:

```text
Recommended priority: high (could not set it — this agent's autonomy is read-only)
```

Then say in your summary on this issue that the priorities were recommended rather
than applied, so nobody assumes the queue is sorted when it is not.

## What to do

1. List all backlog issues that have not been prioritized
2. For each issue, read the description and any attached logs or screenshots
3. Assess severity (critical / high / medium / low) based on user impact and scope
4. Set the priority field on the issue accordingly
5. Add a comment explaining your assessment and suggested next steps

## How to assess severity

- **critical** — data loss, a security hole, or the product unusable for everyone,
  with no workaround.
- **high** — a core flow is broken, or many users are affected, and the workaround
  is bad enough that people will not find it.
- **medium** — a real defect in a non-core path, or one with a workaround a user
  can reasonably reach.
- **low** — cosmetic, rare, or already mitigated.

Name the impact and the scope that put each issue in its band. "Feels important"
is not an assessment.

## What to leave alone

- An issue whose report you cannot understand: do not guess a priority. Comment
  asking for the specific thing you need — repro steps, a version, a log line —
  and leave it unprioritized.
- An issue that already has a priority. It has been triaged; do not re-triage it.

## Do not

- Do not modify files, commit, push, or merge anything.
- Do not change any issue's status, assign it, or close it. Priority is the only
  field this autopilot touches.
- Do not report a severity you cannot justify from what you read in the issue.
