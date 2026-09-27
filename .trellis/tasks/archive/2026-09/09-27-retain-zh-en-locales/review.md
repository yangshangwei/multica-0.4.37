# Planning review

Date: 2026-09-27. Scope: planning artifacts only; no implementation or runtime-test approval is implied.

## Independent results

- **Backend/compatibility reviewer — APPROVE.** Reviewed UI/content language identifiers, old-client requests, new-client/old-server preference handling, null/omission behavior, no write-on-read, deferred squad JSON, content/identity preservation and isolated test execution. No blocking issue found.
- **Web/docs reviewer — APPROVE.** Verified redirect shape against installed Next behavior: basePath is added once, permanent redirects use 308, query parameters survive, and redirects run before middleware. Confirmed old-slug inventory, full destination verification, preference precedence, retained Chinese help and available scripts. No blocking issue found.

## Incorporated refinements

1. Tests explicitly compare database values before/after legacy-preference reads and unrelated profile updates omitting language; responses normalize to English without rewriting storage.
2. Chinese help generation precedes integration checks when Chinese source changes, preventing an intentional edit from failing the generated-bundle drift check.
3. The oversized workflow document is read directly before implementation instead of being truncated during context injection.

## Verification boundary

Planning validation covers JSON syntax, task status, context-file existence, Markdown links, whitespace, and current command/file references. The English/Chinese resource-key comparison covered 27 namespaces with no mismatch after plural normalization. Application tests, builds, migrations and deployments have not run for this task; their required coverage is specified in `test-spec.md` and `implement.md`.

This planning review preceded the user’s implementation authorization on 2026-09-27. Completed implementation evidence is in `verification.md`.
