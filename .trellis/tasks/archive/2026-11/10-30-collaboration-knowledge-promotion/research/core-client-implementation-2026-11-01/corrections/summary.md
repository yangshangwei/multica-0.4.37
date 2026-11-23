# B1 correction evidence

Implementation is ready for main's independent recheck. This document does not change or close the review findings.

Changes:

- KF-01: target project/workspace knowledge routes now have a private logging classification independent of account collaboration header handling. Request/response logs remove query/cursor data; failed-response logs use a content-free error. Explicit workspace ID and cleared ambient slug remain intact.
- KF-02: a source-unavailable pending candidate may retain its server-granted rejection capability. The action preparer permits that content-free rejection, including expired pending candidates. Approval still requires available material plus its current server grant; restricted target-only rows cannot reject. Unavailable body/summary/hash/ref/quoted-reason suppression remains strict.
- A2 alignment: every v1 detail requires `history_truncated:boolean`; no missing-field default is used. Detail history remains bounded to 50 same-lineage/source/target rows. Source pages preserve every candidate version, including multiple versions of one lineage. The API seam documents this producer contract and the accepted unsealed pre-create error boundary.

Seven core files changed from the reviewed B1 baseline: `api/client.ts`, `api/collaboration-knowledge.test.ts`, `collaboration/knowledge-schema.ts`, `collaboration/knowledge-schema.test.ts`, `collaboration/knowledge-actions.ts`, `collaboration/knowledge-actions.test.ts`, and `collaboration/knowledge-test-fixtures.ts`. The API seam and correction evidence are the only additional task-document changes. No Go, view, router, provider, dependency, commit, or external model changes were made by this correction lane.

Evidence:

- Before product edits: **8 failing / 23 passing / zero skipped** across the three affected suites (`red.log`, `red-vitest.json`, `red-source-hashes.json`). Four failures show private target logging, three show unavailable rejection blocked, and one shows history completeness discarded.
- After the narrow fixes: **31 passing / zero failing or skipped** in the same three suites (`green.log`, `green-vitest.json`).
- Final affected run, including bounded-history/all-version and cache regressions: **40 passing across four suites**, zero failures/skips (`tests.log`, `vitest.json`, `test-inventory.json`). No unrelated broad test reruns.
- Full core typecheck, ESLint on the seven affected files, and `git diff --check -- packages/core` passed. Exact commands and exit codes are retained beside the logs.
- `source-manifest.json` in this directory contains the final hashes for all 23 B1 core files; `changed-source-hashes.json` identifies the seven corrections. The parent directory's original 207-test verification and source manifest are preserved as historical baseline evidence. `verification.json` records the current HEAD and scope.

Existing original-intent recovery behavior remains intact: elapsed proposal timestamps do not cause a new key or payload; unsealed API errors do not synthesize candidate state/receipts; ambiguous transport/commit outcomes retain the original recovery identity. Producer implementation and full HTTP/product acceptance remain later-batch work.
