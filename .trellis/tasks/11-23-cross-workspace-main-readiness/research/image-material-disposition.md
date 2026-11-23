# Image evidence disposition

Final observed candidate: `73a28ed90fdcfd9eda1ac5d2970df5af2c486f94`; base: `dd87e0b33c1d6a64d488f7ee9ecd0a65984a68ff`.

The initial `image-material-inventory.json` was independently rechecked against `73a28ed90`; all path/blob/byte entries remain identical (see `verification-evidence-audit.md`). It enumerates all 121 added/grown images above the existing 300 KiB budget, with Git blob IDs and prior/current byte sizes. Their combined current size is approximately 49.74 MiB. Every offender is an archived acceptance capture under `.trellis/tasks`, `.impeccable` or `artifacts`; none is a product bitmap served by the application.

## Proposed treatment

Preserve the original captures. They are referenced by historical verification reports and some evidence records bind their bytes. Lossy conversion, resized copies or replacement of their originals would weaken reproducibility while retaining the large original blobs in existing Git history. No history rewriting or deletion is proposed.

Use the repository's existing, explicit PR-body exception for this bounded historic set. This does not disable the checker or change the limit. The corrected script continues to report every offender and rejects the set without the exception. Future product images remain subject to the same limit.

Reviewable PR-body line:

> Oversized image exemption: Preserve the 121 original acceptance screenshots (49.74 MiB total, individually listed with Git blob IDs in .trellis/tasks/11-23-cross-workspace-main-readiness/research/image-material-inventory.json). They are historical test evidence, not shipped product assets; replacing or recompressing the originals would invalidate linked evidence without removing those blobs from existing Git history. No image-budget rule or directory exclusion is changed.

This is a proposed delivery note. No PR was created or updated, and this document does not claim an actual PR check passed. Recalculate the inventory against the final candidate/base before using the line. If the set changes, update both the inventory and the explanation.

## Archive size

The broader `.trellis/tasks` change includes approximately 1,649.87 MiB of current blobs at the initial review snapshot. Moving those materials out of Git history is a separate repository-retention decision, not a safe byproduct of this merge-readiness repair. This task preserves the existing history and adds compact evidence summaries rather than another copy of raw browser/Go output.
