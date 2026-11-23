# Main merge continuation — 2026-11-24

The user explicitly requested continued integration into `main`. This resumes
the existing approved task and PR [#6](https://github.com/yangshangwei/multica-0.4.37/pull/6),
including its previously published image-evidence exemption. The earlier
READINESS report remains the historical local-verification record.

At resumption, local and remote source were
`b073f59616eb3db68ebf2e45c101c3ba8166d52b`; local and remote main were
`dd87e0b33c1d6a64d488f7ee9ecd0a65984a68ff`. Main was an ancestor, with 874
source commits. Tracked files were clean. Existing untracked evidence was
preserved; no unrelated branch or worktree was removed.

## Remote failure and bounded repairs

[CI run 45203610040](https://github.com/yangshangwei/multica-0.4.37/actions/runs/45203610040)
failed in backend tests. The other 17 check contexts, including Agent critical
path, frontend aggregate, SQLC, vulnerability scan, image budget, three native
platforms and installers, passed on the same source. The failed backend
aggregate reflected the actual test failure rather than an independent defect.

1. Private information-history cursors received the public development signing
   key in CI. The unchanged production refusal returned 503. CI and release
   verification now generate a job-only key before any Go process can cache
   it, register GitHub's log masker, then export it through `GITHUB_ENV`.
2. Snooze state and the first receipt held nanoseconds that PostgreSQL discarded.
   Canonicalize UTC microseconds before final-clock validation; retain equality
   across the first receipt, list, undo and source replay. Fixtures explicitly
   inject a 789 ns remainder to reproduce on every host.
3. UTC hosts exposed `time.Local` versus JSON `time.UTC` struct comparisons.
   Internal stage finalization now normalizes copied deadline representations
   while comparing every invocation field at full precision. A 1 ns deadline
   change and every other binding change still fail, and unconsumed entries
   remain unauthorized. Receipt-only recall/admin tests compare complete wire
   values while retaining decoder, authority, actor and replay assertions.
4. Preview fixture setup counted unrelated immutable delivery history globally.
   Scope only the initial zero counts to the fixture workspace, joining terminal
   rows through their authorization. Complete global before/after no-write
   snapshots remain. A new regression retains real consumed grants and proves
   unrelated history is accepted while fixture-owned rows are still detected.

No dependency, migration, model access, production authorization relaxation,
deadline increase or test skip was introduced.

## Verification before push

Original product failures and fixture failures were reproduced before repair.
All final focused runs used assigned databases on a newly created task-owned
PostgreSQL container, guarded Agent CLI lookup, generated signing keys and
`TZ=UTC`. Linux stage/preview ran on Go 1.26.6; local macOS runs used Go 1.27.0.

| Scope | Final result |
| --- | --- |
| Complete inboxstate, race | 22 named PASS, 0 FAIL/SKIP |
| Handler snooze regressions, race | 15 named PASS, 0 FAIL/SKIP |
| Recall/memory service regressions, race | 26 named PASS, 0 FAIL/SKIP |
| All stage execution, Linux race | 56 named PASS, 0 FAIL/SKIP |
| All completion preview, Linux race | 31 named PASS, 0 FAIL/SKIP |
| Information history service and handler, race | All original history cases PASS |
| Workflow/orchestration Node contracts | 80 PASS, 0 FAIL/SKIP |
| Static checks | Scoped ESLint/checkJs, actionlint, Go vet, gofmt and diff checks PASS |

An independent Trellis checker reviewed the full continuation diff, verified
the production bounds and retained assertions, and identified the key masking
issue before push. Both missing-initializer and missing-mask regressions were
observed failing before their corrections.

Private logs, command receipts, failure inventory and source hashes live under
`.omx/private/merge-main-20261124/`; raw logs and generated credentials are not
committed. Initial macOS artifact fixture setup failures from the symlinked
`/var` temp path are preserved separately; corrected canonical external temp
paths produced the real red/green evidence. These focused results do not claim
a new complete remote CI pass. Merge waits for the corrected PR's actual gates.


## Completed merge

Candidate `662d6694301842d2bab77cdfc8ead1a81d400328` passed all 19 PR checks:

- [Complete CI 45230509413](https://github.com/yangshangwei/multica-0.4.37/actions/runs/45230509413): success, including backend-tests and both strict aggregates.
- [Agent critical path 45230509837](https://github.com/yangshangwei/multica-0.4.37/actions/runs/45230509837): success.
- Focused source-matching Go race inventory reconciles to **167 named PASS**,
  zero failures/skips. The workflow contract set has 80 PASS.

PR #6 merged at `2026-11-23T22:29:51Z` as
`0302e4cb98a8e76ea74cd25ba27e69c8099f4180`. Its parents are the original main
and tested candidate, and its tree `a1a56a7696ac33c53e0502bd505c7b068e005214`
is identical to that candidate. Both parent ancestry checks passed. The initial
merge call returned HTTP 502; a read of PR state and the remote main reference
confirmed successful completion, without retrying the mutation.

Local main was fast-forwarded with an expected-old-value check and selected
without changing file contents. The feature branch and all unrelated untracked
artifacts remain. Only this continuation's exact labeled Go/PostgreSQL containers
were stopped after proving zero remaining database clients; databases and raw
logs were retained. No release tag or deployment was created.

The push-to-main workflow was separately queued as run 45280293517. The PR gate
above is complete; this later run was not misrepresented as completed in the
merge decision. Task archival and the session journal change documentation only.
Previously recorded conditional/browser/provider acceptance limits remain.
