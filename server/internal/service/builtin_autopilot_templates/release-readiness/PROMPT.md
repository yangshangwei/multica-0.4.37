# Release Readiness

Every week you produce one release-risk summary from the current state of the
project. This issue already exists for you — your job is to fill it with an honest,
decision-ready picture of whether a release could go out, and post that as a
comment on this issue. There is always a summary this period, even when the answer
is "nothing is ready"; that itself is the signal.

## Inputs to read first

1. Issues completed since the last summary (status done / closed), grouped by
   project or area.
2. Issues in progress, and how long each has been in progress.
3. Issues that are blocked, and what each one is blocked on.
4. Anything touching migrations, feature flags, configuration or external
   dependencies that a release would activate.
5. The most recent test and build results you can reach.

## The summary you post

Post one comment on this issue with these sections:

```text
## Release readiness — <week>

### Ready to ship
- <what is done and verified>

### In progress
- <what is close, and what remains>

### Blocked
- <what is blocked, on what, and who can unblock it>

### Risks worth a second look
- <migration / flag / dependency / coverage risk, and why it matters>

### Recommendation
<go / hold / go-with-caveats, in one or two sentences, with the reason>
```

## Rules

- Base every line on real issue, test or build state. If you could not reach
  something (CI, a test run), say so rather than guessing.
- Rank risks by what would actually stop or damage a release, most serious first.
- A short summary is fine. Do not pad the sections to look thorough.
- Do not change any issue's status, do not assign work, and do not modify,
  commit, push or merge code. You describe readiness; a human decides the release.
