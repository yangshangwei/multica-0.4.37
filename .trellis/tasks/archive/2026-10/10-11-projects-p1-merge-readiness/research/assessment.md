# Assessment evidence

At source 7ab8d0389 and target 501f4b55f, merge-tree was clean and source was
81 commits ahead with no target-only commits. No open PR or latest-commit CI
checks existed. Inbox test correction was already committed and passed 43 tests.
Core/views tsc passed; projecthealth/triagecsv Go packages passed; offline script
suites reported 63 pass and 3 explicit Docker-smoke skips.

Two temporary regression probes reproduced `parseIssueReferences("123-1")`
and `parseIssueReferences("123-1 123-2")` returning null. Workspace prefixes
permit 1–10 ASCII alphanumerics. Backend identifierNumberRe also rejects digits
within prefixes and should be aligned while preserving ordinary text search.

A probe using the real Unix credential helpers published a hardlink and left
the staging name present. `checkInitializationFile` rejected the final inode
with `credentials_file_unsafe`; deleting the probe-owned staging link restored
validity. The fix must remove this interrupted-state window without weakening
the single-link/no-overwrite security checks. Existing x/sys exposes Linux
Renameat2/RENAME_NOREPLACE and Darwin RenameatxNp/RENAME_EXCL.

The bitmap gate found 13 added PNGs above 307200 bytes, all within .impeccable
audit/critique evidence. Preserve their paths and dimensions during compression.
The repository has installed Sharp through existing node dependencies; command
availability also includes sips and bun. Do not add a project dependency.
