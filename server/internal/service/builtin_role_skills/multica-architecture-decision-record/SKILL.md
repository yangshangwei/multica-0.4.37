---
name: multica-architecture-decision-record
description: "Use when a technical decision will constrain later work: writes an ADR with context, the decision, the rejected alternatives and the consequences."
user-invocable: false
---

# Architecture decision records

## When to use

Write an ADR when the decision will still shape the code after the issue closes:
a boundary between modules, a data model, a contract other clients parse, a
dependency, or a rule the team must follow afterwards.

Do not write one for a choice contained entirely in one function, or for
restating a decision already recorded — link that one instead.

## Where it goes

Look for an existing ADR directory (`docs/adr/`, `docs/decisions/`,
`doc/arch/`) and match its numbering and filename convention exactly. If the
repository has none, propose the location in your comment and wait for a human to
confirm before creating a new documentation tree.

## Structure

```markdown
# <number>. <decision, as a statement>

Date: <YYYY-MM-DD>
Status: proposed

## Context

<the forces: what the system does today, the constraint that makes this a
decision rather than a preference, and what happens if nothing changes>

## Decision

<what we will do, in the present tense, naming real modules and paths>

## Consequences

<what becomes easier, what becomes harder, what we now have to maintain, and
what this forecloses>

## Alternatives considered

### <alternative>
<why it was plausible, and the specific trade-off that ruled it out>
```

Rules that matter more than the template:

- **Status starts at `proposed`.** Only a human moves it to `accepted`. An agent
  marking its own decision accepted is the thing this record exists to prevent.
- **Context must contain a constraint**, not a summary of the feature. If the
  context does not explain why a reasonable engineer might choose otherwise,
  there is no decision to record.
- **At least one real alternative.** "Do nothing" counts only when it was
  genuinely viable.
- **Consequences include the costs.** An ADR listing only benefits is marketing.

## Output

Add the file, then link it from your issue comment with the one-sentence decision
and the trade-off. Never leave the ADR as the only output — a reader on the issue
must see the decision without opening it.

## Stop and ask a human when

- The decision needs a new dependency, data store, or irreversible migration.
- It reverses or narrows an existing accepted ADR.
- Two alternatives differ mainly in cost or team taste rather than correctness.
