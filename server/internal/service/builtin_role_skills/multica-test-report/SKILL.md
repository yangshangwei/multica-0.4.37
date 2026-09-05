---
name: multica-test-report
description: "Use when reporting that code was verified: what ran, the real result, what each new test proves, and what was not covered."
user-invocable: false
---

# Test reports

## When to use

Any turn that claims a change works. The report is what makes the claim checkable
by someone who did not watch the run.

## Rules

1. **Report only runs you performed.** Never write a command you did not execute,
   and never describe an expected result as an observed one. "Tests pass" without
   a command is not a report.
2. **A new test must fail without the change.** Confirm it: revert or stub the
   change, watch the test fail, restore. A test that passes either way proves
   nothing, and this step is what catches it.
3. **Report failures.** A suite that fails goes in the report, with the failing
   names and the message. Removing a failing test, marking it skipped, or
   loosening an assertion to get a green run is falsification.
4. **Say what you did not run.** A skipped suite, an environment you could not
   reach, a platform you cannot test on — each is a line in the report, not an
   omission.
5. **Give real numbers.** Counts of tests run, passed, failed, skipped, taken from
   the runner's own output.

## Choosing the layer

Put each test where the repository already puts that kind of test, and give each
behaviour one canonical home:

- pure logic, parsing, state transitions, permission matrices → the unit test file
  next to the code;
- component and page behaviour → the component test, covering the happy path,
  wiring and named regressions;
- cross-process and end-to-end flows → the e2e suite, sparingly.

Do not re-run a helper's full matrix through a slower layer. Point at the
canonical test in a comment instead.

## Output

```text
## Test plan
- <behaviour> — <how it is checked> — <file>

## Results
- `<command>` — <passed>/<total> passed, <failed> failed, <skipped> skipped
  - FAILED <test name> — <message>

## Proof the new tests bite
- <test> — fails without the change: <how you confirmed>

## Not run
- <suite or platform> — <why>
```

## Stop and ask a human when

- A failure looks like data loss, a permission hole, or a security defect.
- Verifying the behaviour needs production data, real credentials, or a live
  external service.
- The acceptance criteria cannot be tested as written.
