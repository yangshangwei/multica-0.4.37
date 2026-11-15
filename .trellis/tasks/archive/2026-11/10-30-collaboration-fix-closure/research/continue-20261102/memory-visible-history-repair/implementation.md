# Memory visible-history interaction repair

The v9 raw failure and preserved DOM show restored memoryv4 in the closed/read-only state with the Version history trigger still collapsed; Purgev2 is in CollapsibleContent. The test must open that current UI before exercising the historical purge.

The owned worktree receives exactly two added statements after the existing read-only assertions: click the exact labelled Version history button, then assert no Restore actions before the existing Purgev2 interaction. All original contents, versions, control states, response/receipt checks and cleanup remain byte-for-byte unchanged. No forced interaction, sleep or selector weakening was introduced. The preexisting main-worktree fix belongs to the separate functional-regression task and was only read; it was not edited.

Targeted static validation parses the complete file using installed TypeScript5.9.3 with zero syntax diagnostics, verifies the exact two-statement insertion against the untouched source snapshot, checks only this file changed, and passes git diff --check. This does not claim lint/typecheck or browser replay. The original v9 report and three raw attachments are referenced and hashed in before-and-failure-identity.json. Root owns services/databases/actual replay and commits.
