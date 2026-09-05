---
name: multica-requirement-clarification
description: "Use when a request is too vague to build or accept: derives open questions, acceptance criteria, risks and a work split from what the issue already contains."
user-invocable: false
---

# Requirement clarification

## When to use

A request states a wish rather than an outcome; acceptance is undefined; or two
readers of the same issue would build different things.

Do not use this to interrogate a request that is already decidable. If the issue
and its links answer everything, say so and move on.

## Before asking anything

Read, in this order, and treat each as already answered:

1. the issue title, description and every comment;
2. everything the issue links to, including other issues;
3. the acceptance criteria of the two most similar recent issues in this
   workspace — they show the shape this team accepts;
4. the code or documents the request names.

A question whose answer is in one of those four is noise, and it teaches the team
that your questions can be ignored.

## Deriving the criteria

For each criterion, write the observation that settles it — something a reader
can check by looking at the result. Convert every adjective into a check:

| Requested as | Written as |
|---|---|
| "should be fast" | "the list renders within 200 ms for 1000 rows" |
| "handle errors" | "a 500 from the API shows the retry state, and the draft survives" |
| "make it configurable" | "the interval is read from `X`; an absent value falls back to 30s" |

Cover the empty state, the failure path and the permission-denied case, not only
the success path. If you cannot state a check, that is an open question, not a
criterion.

## Ranking open questions

Order questions by what they block, and say so explicitly: a question that blocks
the whole change comes before one that blocks a detail. Mark any question that is
a product or business decision — those go to a human, not to another agent.

## Output

Follow the output format in your instructions. Additional rules:

- Number the open questions so replies can reference them.
- Never invent a question to fill the section. "No open questions" is a result.
- Never answer a business question yourself, even when the answer seems obvious.

## Stop and ask a human when

- The request contradicts a decision the workspace already recorded.
- The requester and a stakeholder in the thread want different outcomes.
- Answering a blocking question needs access to a system you cannot read.
