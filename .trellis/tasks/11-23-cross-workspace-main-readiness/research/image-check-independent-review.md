# Independent image-check and generated-doc review

Date: 2026-11-23. Reviewer: native `standards_review` child, independent of implementation. Reviewed private checkout `.omx/private/qa-work/main-readiness-20261123/src`, source `cb192014b2f27e9bb64112db52073cbb31ce8f04`, base `dd87e0b33c1d6a64d488f7ee9ecd0a65984a68ff`, plus the seven file changes fingerprinted below. Node: v22.23.2, macOS. No product/source files, Git index, commits, root environment or database were changed by this review. This report is the reviewer's only retained edit.

## Verdict

No blocking defect found in the streaming repair, regression tests, CI wiring, or canonical generated-output synchronization. The checker now completes the actual candidate comparison instead of failing with `ENOBUFS`. The default image policy still fails for 121 oversized added/grown evidence images. An explicit PR-body exemption remains a separate delivery decision; a local simulation does not establish approval, a PR, or a successful remote check.

## Verified checks

| Check | Observed result |
| --- | --- |
| `node --test scripts/check-image-budget.test.mjs` | 11 passed, 0 failed/skipped; includes real diff exceeding 1 MiB with final oversized image detected, existing/new bitmap budget behavior, shrink/delete, explicit/empty exemption, whitespace/Unicode/tab/newline paths, invalid ref and Git launch failure. |
| `node --check scripts/check-image-budget.mjs` and `.test.mjs` | Both passed. |
| `git diff --check -- scripts/check-image-budget.mjs scripts/check-image-budget.test.mjs .github/workflows/ci.yml server/internal/docs/content/pages` | Passed. The new test is untracked at review time; its syntax and execution were checked directly. |
| Five additional temporary fake-Git subprocess probes | Passed: one-byte chunks splitting UTF-8 and NUL fields; incomplete path record rejection; exit 7 after valid output rejection; signal termination rejection; consumer `statSync` failure terminates and reaps the child (PID no longer exists). All temporary probe files were removed. |
| `PR_BODY='' node scripts/check-image-budget.mjs --base dd87e0b33c1d6a64d488f7ee9ecd0a65984a68ff` | Exit 1, all 121 offenders reported, no `ENOBUFS`; correct unresolved policy result. |
| Same command with the proposed disposition line supplied only through local `PR_BODY` | Exit 0 and the same 121 offenders still reported; existing explicit-exemption path works. No actual PR was created or updated. |
| Independent `git ls-tree -rl -z` / `git diff --name-only -z --diff-filter=AM` inventory comparison | Exact match for all paths, blob OIDs, prior/current byte sizes: 121 files / 52,151,039 bytes (49.74 MiB). 113 are under `.trellis`, 8 under `artifacts`; none under application asset directories. |
| Canonical `generateDocsBundle({outputRoot: temporaryDirectory})`, twice | 44 pages, 112 assets, 157 files including manifest. Both generated trees are byte/hash-identical to each other and to the candidate's entire `server/internal/docs/content`. Source and candidate content hashes were unchanged before/after. Temporary output removed. |

The docs check imported the existing exported generator from `scripts/generate-docs-bundle.mjs`; only `outputRoot` was overridden. This verifies canonical output and idempotence without deleting/recreating the live candidate bundle during concurrent backend compilation. Sorted relative-path/SHA-256 manifest digest: `4d46774fb2472da6cd964105d0394db0524559d3194d32b2d6b24bd4d1a1b33f`.

## Implementation and gate review

- Git diff output is streamed with NUL field boundaries, so its total length no longer enters a fixed `execFileSync` buffer. The UTF-8 decoder retains split multibyte characters, and `pending` retains a split field. Added/modified records remain pairs under `--diff-filter=AM`; removed and renamed statuses are excluded as before.
- Child spawn errors, nonzero/signal exits and truncated final records fail visibly. The promise resolves an error value, preventing an early rejected-promise race while stdout is consumed. The generator's `finally` terminates an unfinished child and waits for it after a consumer failure. This was exercised with the temporary missing-file probe.
- The 300 KiB threshold, case-insensitive bitmap extension matching, comparison against prior blob size, shrinking allowance and existing exemption regex were not changed. No archive exclusion or default pass was introduced.
- CI adds the test path to the existing image filter and runs the Node tests before the existing budget command under Node 22. PR/base conditions, full-history checkout and real `PR_BODY` source remain unchanged. Remote Actions was not executed by this reviewer.
- The four JSON page changes are exactly canonical output from already committed Chinese MDX. No generator, source Markdown, manifest or asset change was introduced by this synchronization. It satisfies `.trellis/spec/docs/frontend/docs-bundle.md`'s source/generated parity contract.

## Material disposition and existing debt

`image-material-disposition.md` correctly describes a proposal, retains originals/history, and requires recalculation against the final source/base. The independent inventory confirms its 121-item/49.74 MiB set. Its statement that future product images remain subject to the limit describes the unchanged policy generally: the existing exemption is PR-wide, not mechanically allowlisted to these 121 files. If more images enter this same candidate, the inventory and rationale must be reviewed again before that line is used. Do not claim per-file exemption enforcement.

Nonblocking source-doc follow-up, reported to the parent before this report and explicitly left outside the synchronization scope:

- `apps/docs/content/docs/desktop-app.zh.mdx:57` says clicking the Collaboration heading navigates/restores, but `packages/views/layout/app-sidebar.tsx:824` renders a disclosure trigger.
- `desktop-app.zh.mdx:59`, `feature-overview.zh.mdx:38`, and `tutorial.zh.mdx:114` still describe a **新建** menu. Current `app-sidebar.tsx:720` renders **开始工作** with direct **新建任务** / **发起协作** actions (and the compact overflow). The generated JSON faithfully carries these existing source inaccuracies. Canonical byte parity does not establish editorial correctness.

These are inherited source-document issues, not changes invented by the generator repair. No product or editorial scope was expanded. This bounded review does not replace the parent's Go, builds, browser regression, final source refresh, or merge recommendation. The pre-existing comment-administrator authorization debt remains in its separate product task.

## Reviewed file SHA-256 fingerprints

```text
5928e58770905257e47a8e9d714e0ec3e864612c2ded9444863908e392fec18f  scripts/check-image-budget.mjs
382e908b72447c5aa607f29e83bbc369fb2db2ac7d7637fdffc78fcf12a55123  scripts/check-image-budget.test.mjs
f7a043109f5a8fce3e583ea9ff0e49de8960ee0190f324712406a2a2b6446b90  .github/workflows/ci.yml
85d9e73036aded99f48d6c2b6a9f286581eb8cb893a8b5e0901ea4a780e8ab12  server/internal/docs/content/pages/collaboration.json
b4a17117f8c8810cc9f21598a969e69d784d82d5e15d7671d727f7a2c0f00442  server/internal/docs/content/pages/desktop-app.json
e9bb1e81dc0744fe6388072ac129ff830126e4aceb59135191af7ab764c7068b  server/internal/docs/content/pages/feature-overview.json
f613f21ec3bf93d009ac0c47e06478feea27a830153af48a427e74489f0475ad  server/internal/docs/content/pages/tutorial.json
```
