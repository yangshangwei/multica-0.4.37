# Image budget check

## 1. Scope / Trigger

Read before changing `scripts/check-image-budget.mjs`, its regression suite, or
the `image-budget` job. Large feature branches contain enough evidence paths to
exceed a synchronous child-process output buffer before any bitmap is checked.

## 2. Signatures

```sh
node scripts/check-image-budget.mjs --base <git-ref>
node --test scripts/check-image-budget.test.mjs
```

`--base` takes precedence over `GITHUB_BASE_REF` (resolved as `origin/<ref>`).
`PR_BODY` supplies the existing reason-bearing exemption line.

## 3. Contracts

- Compare tracked added/modified working-tree paths against the selected base;
  untracked files are outside the current command's contract.
- A bitmap above 300 KiB fails when new or larger than its base blob. Deletions,
  unchanged sizes and shrinking oversized images pass.
- Process Git's `--name-status --diff-filter=AM -z` output as streamed
  status/path records. Tabs, line breaks and Unicode belong to the pathname.
  Keep only the incomplete record between chunks and let the stream decoder
  preserve split UTF-8 characters.
- An explicit `Oversized image exemption: <reason>` in `PR_BODY` permits the
  reported offenders. This is a PR-wide exception, not a per-file allowlist;
  review the complete final offender inventory before applying it.
- A successful exception still reports every offender. Do not exempt archive
  directories implicitly or present a local exception simulation as remote CI.

## 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Valid diff without oversized growth | Exit 0 |
| New/grown bitmap above 300 KiB, no reason | Exit 1 with offenders |
| Same offenders with explicit reason | Exit 0 with offenders and exception message |
| Missing Git, invalid base, nonzero/signal Git exit | Nonzero; never a clean-budget message |
| Truncated status/path record | Nonzero |
| Consumer fails before diff finishes | Close the owned child and await settlement |

## 5. Good / Base / Bad Cases

- Good: a diff above 1 MiB reaches an oversized image at its end and rejects it.
- Base: a file shrinks from 400 KiB to 350 KiB and remains allowed.
- Bad: raising a fixed `maxBuffer`, splitting quoted path output on newlines, or
  hiding evidence directories to make a large branch pass.

## 6. Tests Required

`scripts/check-image-budget.test.mjs` uses real temporary Git repositories for
thresholds, new/grown/shrunk/deleted files, reason/empty reason, unusual paths,
Git startup/ref failure and a large diff whose final bitmap must be checked.
The image CI filter includes both script and test; Node 22 runs the tests before
the actual PR budget check. Preserve the original failing regression when
changing the subprocess path.

## 7. Wrong vs Correct

Wrong: `execFileSync("git", args).toString().split("\n")` buffers the entire
repository path list and misreads quoted paths.

Correct: stream `git diff ... -z`, parse complete NUL fields, check the child's
final result, and keep the unchanged size/exemption rules after enumeration.
