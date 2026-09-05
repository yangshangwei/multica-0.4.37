---
name: multica-documentation-change
description: "Use when a change needs documentation: deciding what to write, keeping it true to the code, and finding what the change made wrong."
user-invocable: false
---

# Documentation changes

## When to use

A change alters observable behaviour, a command, an API field, a configuration
key, a limit, or a rule other people follow.

## Decide what the change actually requires

Work from the diff, not from the issue's promises:

| The change | What it requires |
|---|---|
| user-visible behaviour | the user-facing page for that feature |
| API field or endpoint | the reference entry, plus its compatibility note |
| CLI command or flag | the command reference and any example using it |
| operational step | the runbook, at the step that changed |
| a rule others must follow | the conventions document |
| bug fix with no behaviour change | often nothing — say so |

"Nothing to document" is a valid, common result. Write it as a finding rather than
producing a page to have produced something.

## Keep it true

- Every statement must be traceable to code, a test, or a recorded decision. If
  you inferred it, verify it or leave it out.
- Read the tests. They usually state the real contract more precisely than the
  issue does.
- Use the project's glossary term, not a synonym. When the code and the docs
  disagree on a name, report the mismatch instead of adding a third name.
- Match the surrounding document: structure, depth, terminology and, for
  user-facing copy, its voice and language.

## Find what is now wrong

Search the documentation for the old behaviour — the old field name, flag,
default, or limit — and list every place that still describes it. A change that
invalidates a page and leaves it standing has not been documented. Fix what is in
scope; list the rest with the correction needed.

## Do not

- Change code, tests or configuration, including renaming something to match your
  text.
- Document behaviour that does not exist yet.
- Add a summary of the summary, a "conclusion", or an empty heading.

## Stop and ask a human when

- The code and the intended behaviour disagree — that is a defect report.
- Documenting it honestly would expose a security-sensitive detail.
- It needs a product decision about user-facing naming.
