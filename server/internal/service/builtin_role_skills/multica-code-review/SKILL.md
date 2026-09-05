---
name: multica-code-review
description: "Use when reviewing a diff: what to look for, how to state a finding so it is actionable, and what not to report."
user-invocable: false
---

# Code review

## When to use

Any review of a change, whether requested on an issue, a pull request, or a
working tree.

## What to look for, in order

1. **Correctness** — does it do what the acceptance criteria say, for the inputs
   it will actually receive? Off-by-one, null and empty cases, wrong operator,
   inverted condition, error swallowed.
2. **Contracts** — does any caller, stored row, or installed client still parse
   what this now returns? A field that changed type or disappeared is a finding
   even when every test passes.
3. **Error and failure paths** — what happens when the call fails, the row is
   missing, the user lacks permission, or the operation runs twice.
4. **Concurrency and ordering** — two of these at once; a retry; a partial write.
5. **Coverage** — did the behaviour that changed get a test that would fail
   without the change?
6. **Reuse** — does this reimplement something the repository already has? Name
   the existing helper by path.
7. **Scope** — files changed that the issue did not require.

## How to write a finding

Each finding needs four things, and is not a finding without them:

```text
`<path>:<line>` — <severity: must-fix | consider>
<what is wrong, in one sentence>
Failure: <concrete input or state> → <wrong output or crash>
```

If you cannot fill in the `Failure:` line from the code, you have a suspicion,
not a finding. Investigate it or drop it.

Severity means consequence, not confidence:

- **must-fix** — produces a wrong result, loses data, breaks a contract, or opens
  a permission hole;
- **consider** — real improvement, no failure attached.

## What not to report

- Style the repository has not adopted, or a rewrite in your preferred idiom.
- Findings that require code you did not read.
- Restating what the diff obviously does.
- Padding an empty review. "No blocking findings" is the correct output when the
  change is fine.

## Do not

Edit the code, push a commit, apply your own suggestion, or change the issue's
status. The review is the whole deliverable.

## Stop and ask a human when

- The diff is too large to review honestly — ask for a split.
- The change is correct but conflicts with a recorded decision.
- You find a security-relevant defect: mark it must-fix and say it needs a
  security review before merge.
