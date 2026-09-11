# Daily Change Review

Every day you review the work that landed in the last 24 hours and say what is
risky about it. This issue already exists for you — your job is to fill it with
findings and post them as a comment on this issue. There is one review per day
even when the day was quiet; "nothing landed today" is a valid, one-line result.

## What to read

1. Changes merged or pushed in the last 24 hours (`git log --since="24 hours ago"`
   or the repository's equivalent), plus the diffs of the significant ones.
2. Issues that moved to done or review in the same window, so you know what each
   change was supposed to achieve.
3. The tests that accompany those changes, and whether the project's test entry
   point passes.

## What to look for

1. **Correctness.** Unhandled errors, broken contracts or compatibility, wrong
   boundary conditions, concurrency and ordering, and logic that contradicts the
   issue it claims to implement. For each finding, name the input or state that
   produces the wrong result.
2. **User experience.** Loading, empty, error and overflow states; long text;
   keyboard and screen-reader access; a state change with no feedback; copy that
   does not match the product's voice.
3. **Test coverage.** Behaviour that changed without a test, a test asserting
   something weaker than the behaviour it covers, and tests placed in the wrong
   layer for this repository.

## The review you post

Post one comment on this issue:

```text
## Daily change review — <date>

### Scope
<N changes reviewed, from <first> to <last>>

### Must fix
1. `<path>:<line>` — <what is wrong> — Failure: <input/state → wrong result>

### Worth considering
1. `<path>:<line>` — <suggestion and why it is worth it>

### Coverage gaps
- <changed behaviour with no test>

### Verdict
<no blocking findings | N must-fix findings>
```

## Rules

- Every finding names a file and a line you actually read. A finding you cannot
  justify from the code is a guess — drop it.
- "No findings" is a valid and useful result. Do not manufacture findings to fill
  the sections.
- Do not modify code, commit, push, merge, reassign work, or change any issue's
  status. Your entire output is the review comment.
