---
name: multica-architecture-decision-record
description: "Use when a technical decision will constrain later work: drafts a proposed ADR in the issue comment for an Implementer or human to save, with context, the decision, rejected alternatives and consequences."
user-invocable: false
---

# Architecture decision records

## When to use

Draft an ADR when the decision will still shape the code after the issue closes:
a boundary between modules, a data model, a contract other clients parse, a
dependency, or a rule the team must follow afterwards.

Do not draft one for a choice contained entirely in one function, or for
restating a decision already recorded — link that one instead.

## Where it goes

Put the complete draft in the issue comment. Look for an existing ADR directory
(`docs/adr/`, `docs/decisions/`, `doc/arch/`) and suggest a destination matching its
numbering and filename convention. If the repository has none, propose a
location in the comment for a human to confirm.

The default Architect is an Observer: do not edit repository files or create
directories. Hand the draft and suggested destination to an Implementer or human
to save in the repository. A suggested path is not a link to an existing file.

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

Include the complete ADR draft with `Status: proposed` in your issue comment,
along with the one-sentence decision and the trade-off. State the suggested
destination and handoff to an Implementer or human to save it. Link an ADR only
when the file already exists; never claim the draft has been saved or accepted.

## Stop and ask a human when

- The decision needs a new dependency, data store, or irreversible migration.
- It reverses or narrows an existing accepted ADR.
- Two alternatives differ mainly in cost or team taste rather than correctness.
